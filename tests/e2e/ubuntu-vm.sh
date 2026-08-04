#!/usr/bin/env bash
set -Eeuo pipefail
umask 077

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
cache=${EZVPN_E2E_CACHE:-$HOME/.cache/ez-vpn-lego/e2e}
signing_key=${EZVPN_SIGNING_KEY:-$HOME/.config/ez-vpn-lego-signing/release-ed25519.pem}
version=v0.0.0-e2e
mkdir -p "$cache"
work=$(mktemp -d)
server_pid= qemu_pid=
cleanup() {
  [[ -z $qemu_pid ]] || kill "$qemu_pid" 2>/dev/null || true
  [[ -z $server_pid ]] || kill "$server_pid" 2>/dev/null || true
  rm -rf -- "$work"
}
diagnose() {
  if [[ -n $qemu_pid ]]; then
    ssh -i "$work/id_ed25519" -p 2222 -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o ConnectTimeout=2 ubuntu@127.0.0.1 \
      'sudo systemctl --no-pager --full status lego-vpn-helper lego-vpnd xray lego-vpn-bot 2>&1; sudo journalctl --no-pager -n 100 -u lego-vpn-helper -u lego-vpnd 2>&1' || true
  fi
}
trap cleanup EXIT
trap diagnose ERR

test -r "$signing_key"
VERSION=$version GOARCH=amd64 OUTPUT_DIR="$work/assets" "$root/scripts/build-release.sh"
(cd "$work/assets" && sha256sum *.tar.gz > SHA256SUMS)
openssl pkeyutl -sign -inkey "$signing_key" -rawin -in "$work/assets/SHA256SUMS" -out "$work/assets/SHA256SUMS.sig"

go run "$root/tests/e2e/mockserver" --dir "$work/assets" --listen 0.0.0.0:18080 >"$work/mockserver.log" 2>&1 &
server_pid=$!

image="$cache/noble-server-cloudimg-amd64.img"
if [[ ! -s $image ]]; then
  curl --fail --location --proto '=https' --tlsv1.2 \
    https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img --output "$image.tmp"
  mv "$image.tmp" "$image"
fi
cp --reflink=auto "$image" "$work/vm.qcow2"
qemu-img resize "$work/vm.qcow2" 12G >/dev/null
ssh-keygen -q -t ed25519 -N '' -f "$work/id_ed25519"
cat > "$work/user-data" <<EOF
#cloud-config
users:
  - name: ubuntu
    groups: [adm, sudo]
    shell: /bin/bash
    sudo: ALL=(ALL) NOPASSWD:ALL
    ssh_authorized_keys:
      - $(<"$work/id_ed25519.pub")
ssh_pwauth: false
package_update: false
EOF
printf 'instance-id: ez-vpn-lego-e2e\nlocal-hostname: ezvpn-e2e\n' > "$work/meta-data"
genisoimage -quiet -output "$work/seed.iso" -volid cidata -joliet -rock "$work/user-data" "$work/meta-data"

qemu-system-x86_64 -enable-kvm -machine accel=kvm -cpu host -m 2048 -smp 2 \
  -drive "file=$work/vm.qcow2,if=virtio,format=qcow2" -drive "file=$work/seed.iso,if=virtio,format=raw" \
  -device virtio-net-pci,netdev=net0 -netdev user,id=net0,hostfwd=tcp:127.0.0.1:2222-:22 \
  -display none -serial "file:$work/serial.log" -daemonize -pidfile "$work/qemu.pid"
qemu_pid=$(<"$work/qemu.pid")

ssh_args=(-i "$work/id_ed25519" -p 2222 -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o ConnectTimeout=3)
scp_args=(-i "$work/id_ed25519" -P 2222 -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o ConnectTimeout=3)
for attempt in $(seq 1 120); do
  if ssh "${ssh_args[@]}" ubuntu@127.0.0.1 cloud-init status --wait >/dev/null 2>&1; then break; fi
  if ((attempt == 120)); then tail -200 "$work/serial.log" >&2; exit 1; fi
  sleep 2
done
scp "${scp_args[@]}" "$root/install.sh" ubuntu@127.0.0.1:/tmp/install.sh
ssh "${ssh_args[@]}" ubuntu@127.0.0.1 \
  "sudo bash /tmp/install.sh --version $version --artifact-base http://10.0.2.2:18080 --telegram-api-url http://10.0.2.2:18080 --bot-token 123456:e2e-token-that-is-long-enough --public-address 203.0.113.10"

ssh "${ssh_args[@]}" ubuntu@127.0.0.1 'sudo systemctl is-active lego-vpn-helper lego-vpnd lego-vpn-bot xray postgresql'
ssh "${ssh_args[@]}" ubuntu@127.0.0.1 'sudo -u ezvpn bash -c "set -a; source /etc/ez-vpn-lego/ezvpn.env; set +a; lego-vpnctl status; lego-vpnctl users list"'
ssh "${ssh_args[@]}" ubuntu@127.0.0.1 'sudo ez-vpn-lego backup /var/lib/ez-vpn-lego/backups/e2e.tar.gz >/dev/null && sudo test -s /var/lib/ez-vpn-lego/backups/e2e.tar.gz'
ssh "${ssh_args[@]}" ubuntu@127.0.0.1 'sudo ez-vpn-lego restore /var/lib/ez-vpn-lego/backups/e2e.tar.gz'
ssh "${ssh_args[@]}" ubuntu@127.0.0.1 'sudo systemctl is-active lego-vpn-helper lego-vpnd lego-vpn-bot xray postgresql'
echo 'Ubuntu 24.04 VM E2E passed'

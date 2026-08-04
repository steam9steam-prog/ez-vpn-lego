#!/usr/bin/env bash
set -Eeuo pipefail
umask 077

repo=steam9steam-prog/ez-vpn-lego
version=latest
bot_token=
public_address=
reality_target=www.microsoft.com:443
server_name=www.microsoft.com
artifact_base=
telegram_api_url=https://api.telegram.org

usage() {
  echo 'usage: install.sh --bot-token TOKEN [--version VERSION] [--public-address IP]'
}

while (($#)); do
  case "$1" in
    --bot-token) bot_token=${2:-}; shift 2 ;;
    --version) version=${2:-}; shift 2 ;;
    --public-address) public_address=${2:-}; shift 2 ;;
    --artifact-base) artifact_base=${2:-}; shift 2 ;;
    --telegram-api-url) telegram_api_url=${2:-}; shift 2 ;;
    --help) usage; exit 0 ;;
    *) echo "unknown option: $1" >&2; usage >&2; exit 2 ;;
  esac
done

if ((EUID != 0)); then echo 'run this installer as root' >&2; exit 1; fi
if [[ ! -r /etc/os-release ]]; then echo 'unsupported operating system' >&2; exit 1; fi
. /etc/os-release
if [[ ${ID:-} != ubuntu || ${VERSION_ID:-} != 24.04 ]]; then
  echo 'EZ VPN Lego currently supports Ubuntu Server 24.04 LTS only' >&2
  exit 1
fi
case $(dpkg --print-architecture) in amd64|arm64) arch=$(dpkg --print-architecture) ;; *) echo 'unsupported CPU architecture' >&2; exit 1 ;; esac
if [[ ! -s /etc/ez-vpn-lego/secrets/telegram-token && -z $bot_token ]]; then
  read -r -s -p 'Telegram bot token from @BotFather: ' bot_token </dev/tty
  echo >/dev/tty
  [[ -n $bot_token ]] || { echo 'Telegram bot token is required' >&2; exit 1; }
fi

export DEBIAN_FRONTEND=noninteractive
apt-get update
apt-get install -y --no-install-recommends ca-certificates curl jq openssl postgresql qrencode

if [[ $version == latest ]]; then
  version=$(curl --fail --silent --show-error --proto '=https' --tlsv1.2 \
    "https://api.github.com/repos/$repo/releases/latest" | jq -er .tag_name)
fi
[[ $version =~ ^v[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$ ]] || { echo 'invalid release version' >&2; exit 1; }

work=$(mktemp -d)
trap 'rm -rf -- "$work"' EXIT
base=${artifact_base:-"https://github.com/$repo/releases/download/$version"}
asset="ez-vpn-lego_${version}_linux_${arch}.tar.gz"
release_curl_flags=(--fail --location)
if [[ $base == https://* ]]; then release_curl_flags+=(--proto '=https' --tlsv1.2); fi
for file in "$asset" SHA256SUMS SHA256SUMS.sig; do
  curl "${release_curl_flags[@]}" "$base/$file" --output "$work/$file"
done
cat > "$work/release-public.pem" <<'PUBLIC_KEY'
-----BEGIN PUBLIC KEY-----
MCowBQYDK2VwAyEA7pNcqdSslm/GjqGIQLds1Ljy37ZeqUS6o0mEtWyUBgA=
-----END PUBLIC KEY-----
PUBLIC_KEY
openssl pkeyutl -verify -pubin -inkey "$work/release-public.pem" -rawin \
  -in "$work/SHA256SUMS" -sigfile "$work/SHA256SUMS.sig" >/dev/null
(cd "$work" && grep "  $asset" SHA256SUMS | sha256sum --check --status)

release_dir="/usr/local/lib/ez-vpn-lego/releases/$version"
install -d -m 0755 /usr/local/lib/ez-vpn-lego /usr/local/lib/ez-vpn-lego/releases "$release_dir"
tar --extract --gzip --file "$work/$asset" --directory "$release_dir" --no-same-owner
chmod -R a+rX "$release_dir"
for binary in lego-vpnd lego-vpnctl lego-vpn-bot lego-vpn-helper ez-vpn-lego-maintain xray; do
  test -x "$release_dir/bin/$binary"
done

getent group ezvpn >/dev/null || groupadd --system ezvpn
id ezvpn >/dev/null 2>&1 || useradd --system --gid ezvpn --home-dir /var/lib/ez-vpn-lego --shell /usr/sbin/nologin ezvpn
getent group xray >/dev/null || groupadd --system xray
id xray >/dev/null 2>&1 || useradd --system --gid xray --home-dir /nonexistent --shell /usr/sbin/nologin xray

install -m 0644 "$release_dir/tmpfiles/ez-vpn-lego.conf" /usr/lib/tmpfiles.d/ez-vpn-lego.conf
systemd-tmpfiles --create /usr/lib/tmpfiles.d/ez-vpn-lego.conf
install -m 0644 "$release_dir/systemd/"*.service "$release_dir/systemd/"*.timer /etc/systemd/system/
ln -sfn "$release_dir/bin" /usr/local/lib/ez-vpn-lego/current.new
mv -Tf /usr/local/lib/ez-vpn-lego/current.new /usr/local/lib/ez-vpn-lego/current
ln -sfn /usr/local/lib/ez-vpn-lego/current/lego-vpnctl /usr/local/bin/lego-vpnctl
ln -sfn /usr/local/lib/ez-vpn-lego/current/ez-vpn-lego-maintain /usr/local/sbin/ez-vpn-lego

secret_dir=/etc/ez-vpn-lego/secrets
install_secret() { local path=$1 value=$2; if [[ ! -s $path ]]; then printf '%s\n' "$value" > "$path"; fi; chown root:ezvpn "$path"; chmod 0640 "$path"; }
install_secret "$secret_dir/api-token" "$(openssl rand -base64 48 | tr -d '\n')"
install_secret "$secret_dir/helper-token" "$(openssl rand -base64 48 | tr -d '\n')"
install_secret "$secret_dir/master-key" "$(openssl rand -base64 32 | tr '+/' '-_' | tr -d '=\n')"
if [[ ! -s $secret_dir/telegram-token ]]; then
  install_secret "$secret_dir/telegram-token" "$bot_token"
fi

cat > /etc/ez-vpn-lego/ezvpn.env <<'ENV'
EZVPN_DATABASE_URL=postgres:///ezvpn?host=/var/run/postgresql
EZVPN_SOCKET_PATH=/run/ez-vpn-lego/control.sock
EZVPN_HELPER_SOCKET_PATH=/run/ez-vpn-lego/helper.sock
EZVPN_CANDIDATE_DIR=/var/lib/ez-vpn-lego/candidates
EZVPN_API_TOKEN_FILE=/etc/ez-vpn-lego/secrets/api-token
EZVPN_HELPER_TOKEN_FILE=/etc/ez-vpn-lego/secrets/helper-token
EZVPN_MASTER_KEY_FILE=/etc/ez-vpn-lego/secrets/master-key
EZVPN_TELEGRAM_TOKEN_FILE=/etc/ez-vpn-lego/secrets/telegram-token
EZVPN_ADMIN_ID_FILE=/etc/ez-vpn-lego/admin-id
EZVPN_XRAY_BINARY=/usr/local/lib/ez-vpn-lego/current/xray
EZVPN_XRAY_CONFIG=/etc/xray/config.json
EZVPN_XRAY_UNIT=xray.service
ENV
if [[ $telegram_api_url != https://api.telegram.org ]]; then printf 'EZVPN_TELEGRAM_API_URL=%s\n' "$telegram_api_url" >> /etc/ez-vpn-lego/ezvpn.env; fi
chown root:ezvpn /etc/ez-vpn-lego/ezvpn.env
chmod 0640 /etc/ez-vpn-lego/ezvpn.env

systemctl enable --now postgresql
runuser -u postgres -- psql --set=ON_ERROR_STOP=1 --quiet <<'SQL'
DO $$ BEGIN IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'ezvpn') THEN CREATE ROLE ezvpn LOGIN; END IF; END $$;
SELECT 'CREATE DATABASE ezvpn OWNER ezvpn' WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'ezvpn')\gexec
SQL

systemctl daemon-reload
systemctl enable --now lego-vpn-helper.service
systemctl enable --now lego-vpnd.service
for attempt in $(seq 1 30); do
  [[ -S /run/ez-vpn-lego/control.sock ]] && break
  if ! systemctl is-active --quiet lego-vpnd.service; then
    systemctl --no-pager --full status lego-vpnd.service >&2 || true
    exit 1
  fi
  if ((attempt == 30)); then echo 'control plane did not become ready' >&2; exit 1; fi
  sleep 1
done

set -a
. /etc/ez-vpn-lego/ezvpn.env
set +a
if [[ ! -s /etc/ez-vpn-lego/admin-id ]]; then
  unset EZVPN_ADMIN_ID_FILE
  owner_json=$(runuser -u ezvpn -- /usr/local/bin/lego-vpnctl owner bootstrap)
  jq -er .id <<<"$owner_json" > /etc/ez-vpn-lego/admin-id
  chown root:ezvpn /etc/ez-vpn-lego/admin-id
  chmod 0640 /etc/ez-vpn-lego/admin-id
fi
export EZVPN_ADMIN_ID_FILE=/etc/ez-vpn-lego/admin-id

if [[ -z $public_address ]]; then
  public_address=$(curl --fail --silent --show-error --proto '=https' --tlsv1.2 https://api.ipify.org)
fi
if [[ ! -s /etc/xray/config.json ]]; then
  runuser -u ezvpn -- /usr/local/bin/lego-vpnctl vpn bootstrap "$public_address" "$reality_target" "$server_name" > /etc/ez-vpn-lego/bootstrap-access.json
  chmod 0600 /etc/ez-vpn-lego/bootstrap-access.json
fi
systemctl enable xray.service

token=$(<"$secret_dir/telegram-token")
curl_flags=(--fail --silent --show-error)
if [[ $telegram_api_url == https://* ]]; then curl_flags+=(--proto '=https' --tlsv1.2); fi
bot_username=$(curl "${curl_flags[@]}" "${telegram_api_url}/bot${token}/getMe" | jq -er '.result.username')
pairing_json=$(runuser -u ezvpn -- /usr/local/bin/lego-vpnctl telegram pairing "$bot_username")
pairing_url=$(jq -er .url <<<"$pairing_json")
systemctl enable --now lego-vpn-bot.service
systemctl enable --now ez-vpn-lego-backup.timer

if command -v ufw >/dev/null && ufw status | grep -q '^Status: active'; then ufw allow 443/tcp >/dev/null; fi

echo
echo 'EZ VPN Lego installed successfully.'
echo "Open this one-time Telegram link: $pairing_url"
qrencode -t UTF8 "$pairing_url"
echo 'The link expires in 15 minutes and can be used only once.'

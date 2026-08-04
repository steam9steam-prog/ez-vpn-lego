#!/usr/bin/env bash
set -Eeuo pipefail
trap 'status=$?; echo "::error title=Release build failed::line ${BASH_LINENO[0]}: ${BASH_COMMAND}"; exit "$status"' ERR

version=${VERSION:?VERSION is required}
goarch=${GOARCH:?GOARCH is required}
output=${OUTPUT_DIR:-dist}
xray_version=${XRAY_VERSION:-v26.3.27}

case "$goarch" in
  amd64) xray_arch=64 ;;
  arm64) xray_arch=arm64-v8a ;;
  *) echo "unsupported architecture: $goarch" >&2; exit 1 ;;
esac

stage=$(mktemp -d)
trap 'rm -rf -- "$stage"' EXIT
mkdir -p "$stage/bin" "$output"

for command in lego-vpnd lego-vpnctl lego-vpn-bot lego-vpn-helper; do
  CGO_ENABLED=0 GOOS=linux GOARCH="$goarch" go build \
    -trimpath -ldflags "-s -w -X github.com/steam9steam-prog/ez-vpn-lego/internal/buildinfo.Version=$version" \
    -o "$stage/bin/$command" "./cmd/$command"
done

xray_zip="$stage/xray.zip"
curl --fail --location --retry 5 --retry-all-errors --connect-timeout 15 --proto '=https' --tlsv1.2 \
  "https://github.com/XTLS/Xray-core/releases/download/${xray_version}/Xray-linux-${xray_arch}.zip" \
  --output "$xray_zip"
curl --fail --location --retry 5 --retry-all-errors --connect-timeout 15 --proto '=https' --tlsv1.2 \
  "https://github.com/XTLS/Xray-core/releases/download/${xray_version}/Xray-linux-${xray_arch}.zip.dgst" \
  --output "$stage/xray.zip.dgst"
expected=$(awk '$1 == "SHA2-256=" {print $2}' "$stage/xray.zip.dgst")
test -n "$expected"
printf '%s  %s\n' "$expected" "$xray_zip" | sha256sum --check --status
unzip -q "$xray_zip" xray -d "$stage/bin"

cp -a packaging/systemd "$stage/"
cp -a packaging/tmpfiles "$stage/"
install -m 0755 packaging/bin/ez-vpn-lego-maintain "$stage/bin/ez-vpn-lego-maintain"
cp packaging/release-public.pem "$stage/"
cp LICENSE README.md "$stage/"
printf '%s\n' "$version" > "$stage/VERSION"

archive="$output/ez-vpn-lego_${version}_linux_${goarch}.tar.gz"
tar --create --gzip --file "$archive" --directory "$stage" \
  --owner=0 --group=0 --numeric-owner \
  VERSION LICENSE README.md release-public.pem bin systemd tmpfiles
sha256sum "$archive" | sed "s#  .*/#  #" > "$archive.sha256"

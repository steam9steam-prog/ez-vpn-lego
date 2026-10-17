#!/usr/bin/env bash
set -Eeuo pipefail

version=${XRAY_VERSION:-v26.3.27}
arch=${GOARCH:-$(go env GOARCH)}
destination=${1:?usage: fetch-xray.sh OUTPUT_DIRECTORY}
[[ $version =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo 'invalid Xray version' >&2; exit 1; }
case "$arch" in
  amd64) upstream_arch=64 ;;
  arm64) upstream_arch=arm64-v8a ;;
  *) echo 'unsupported Xray architecture' >&2; exit 1 ;;
esac

work=$(mktemp -d)
trap 'rm -rf -- "$work"' EXIT
base="https://github.com/XTLS/Xray-core/releases/download/$version/Xray-linux-$upstream_arch.zip"
for suffix in '' .dgst; do
  curl --fail --location --retry 3 --retry-all-errors --connect-timeout 15 \
    --proto '=https' --tlsv1.2 "$base$suffix" --output "$work/xray.zip$suffix"
done
expected=$(awk '$1 == "SHA2-256=" {print $2}' "$work/xray.zip.dgst")
[[ $expected =~ ^[0-9a-fA-F]{64}$ ]] || { echo 'missing upstream SHA-256 checksum' >&2; exit 1; }
printf '%s  %s\n' "$expected" "$work/xray.zip" | sha256sum --check --status
unzip -q "$work/xray.zip" xray -d "$work"
install -d -m 0755 "$destination"
install -m 0755 "$work/xray" "$destination/xray"
realpath "$destination/xray"

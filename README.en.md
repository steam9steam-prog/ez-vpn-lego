# EZ VPN Lego

[Русский](README.md) | [English](README.en.md) | [简体中文](README.zh-CN.md)

Your VPN, on your servers. Install it, pair your Telegram account, share access.
No mandatory panel or dependency on a hosted Lego service.

The published version supports one VPS with VLESS REALITY Vision and Telegram
administration. A modular version is in development, with direct connections
and two-node routes through multiple entries and exits.

## Available now

- installation on a clean Ubuntu Server 24.04, `amd64` or `arm64`;
- Telegram administration with single-use owner pairing;
- separate access credentials for people and devices;
- configuration validation and rollback on apply failure;
- daily backups and signed updates;
- existing VPN connections keep working without the bot or controller.

## Install the published version

Get a clean VPS and a bot token from [@BotFather](https://t.me/BotFather).
Run this on the server:

```bash
curl -fsSL https://raw.githubusercontent.com/steam9steam-prog/ez-vpn-lego/main/install.sh | sudo bash
```

The installer asks for the token with hidden input. It then prints a single-use
Telegram pairing link and QR code. The link expires after 15 minutes.

You do not need your own domain for this mode. You need an available TCP 443
port reachable from the internet. Do not install over an existing service.

## Modular version in development

```text
User → VPN node → Internet
User → Entry node → Exit node → Internet
```

Any country can host an entry or exit. The controller can be described on an
entry, on an exit, or on a separate server. The model has no fixed node count.

This branch includes a topology planner, an Xray configuration renderer, and
an offline workbench. It preserves existing keys, prepares individual
subscriptions, and checks DNS. The planner is available through the shared
API. Telegram can display the profile catalog.

Remote node enrollment and full fleet administration through Telegram are not
ready yet. The public installer still installs the single-node version.
Rendering files does not apply them to servers.

- [Try the modular workbench](docs/en/modular.md)
- [Domains and certificates](docs/en/domains.md)
- [First modular release scope](docs/product-requirements.md)
- [Architecture](docs/architecture.md)

## Maintain a single-node installation

```bash
sudo -u ezvpn bash -c 'set -a; source /etc/ez-vpn-lego/ezvpn.env; set +a; lego-vpnctl status'
sudo ez-vpn-lego backup
sudo ez-vpn-lego update
sudo ez-vpn-lego restore /path/to/backup.tar.gz
```

Backups include private keys. Keep them private and copy them off the VPS.
The current backup archive is not encrypted by itself.

See [operations](docs/operations.md), [threat model](docs/threat-model.md),
and the [API contract](api/openapi.yaml).

## Development

```bash
make generate-check
make fmt vet test-race build
XRAY_BINARY=/absolute/path/to/xray go test -race ./drivers/xray/topology -v
```

The Xray test checks real direct and two-node traffic on loopback.
The installer has a separate disposable Ubuntu VM test:

```bash
./tests/e2e/ubuntu-vm.sh
```

## License

[GNU AGPL-3.0](LICENSE).

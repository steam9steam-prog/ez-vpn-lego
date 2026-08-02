# EZ VPN Lego

EZ VPN Lego turns a clean VPS into a personal VPN managed through Telegram.
The owner performs one installation, imports the generated bootstrap profile,
and can then manage access without returning to the shell.

## Project status

Architecture and contracts are being established. There is no installable
release yet. Do not use the repository on a production server.

## Product principles

- No domain required.
- Telegram is the primary interface, not a public administration panel.
- The VPN keeps working when Telegram or the control plane is unavailable.
- Every configuration change is validated and reversible.
- Releases are versioned, signed, tested, and explicitly installed.
- Components communicate through documented, versioned contracts.
- No billing, storefront, referral, or commercial subscription features.

## Initial platform

- Ubuntu Server 24.04 LTS (`amd64`, `arm64`)
- VLESS over TCP with REALITY and XTLS Vision
- Go control plane and adapters
- PostgreSQL over a local Unix socket
- systemd and nftables

## Components

- `lego-vpnd`: desired state, API, operations, and reconciliation
- `lego-vpnctl`: recovery and automation CLI
- `lego-vpn-bot`: Telegram adapter
- `lego-vpn-helper`: narrow privileged system interface

Read the [product requirements](docs/product-requirements.md),
[architecture](docs/architecture.md), [data model](docs/data-model.md), and
[threat model](docs/threat-model.md) before contributing.

## License

GNU Affero General Public License v3.0. See [LICENSE](LICENSE).

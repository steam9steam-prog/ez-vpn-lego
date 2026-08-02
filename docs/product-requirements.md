# Product requirements

Status: draft for review.

## Product statement

EZ VPN Lego lets a person who can purchase a VPS and paste one command create a
personal VPN for themselves, family, and friends. After installation, normal
administration happens in a Telegram bot.

## Primary user

The owner understands what a VPS and SSH are but does not want to administer
Linux, edit Xray JSON, manage certificates, or troubleshoot systemd.

## Required user journey

1. The owner starts with a supported clean VPS.
2. The owner runs one bootstrap command.
3. The installer validates the host and installs a pinned release.
4. The installer creates an owner VPN credential and displays its URI and QR.
5. The owner connects to the VPN, creates a Telegram bot when convenient, and
   supplies its token through a secret prompt.
6. A single-use link binds the first Telegram account as owner.
7. The owner creates, disables, rotates, and removes user devices in Telegram.
8. The owner can install a verified update and see its result in Telegram.
9. Failed configuration changes and updates roll back automatically.
10. Emergency recovery remains possible over SSH with the CLI.

## Required capabilities

- One owner and additional role-based administrators.
- Users containing independently revocable devices.
- One credential per device.
- VLESS, TCP, REALITY, and `xtls-rprx-vision` as the first protocol driver.
- Connection URI and QR export without a domain.
- Persistent operations with status and audit records.
- Desired-state rendering, validation, atomic apply, verification, and rollback.
- Encrypted backup and tested restoration.
- Signed releases with compatibility metadata.
- Health reporting and actionable Telegram notifications.
- Versioned local API used equally by bot and CLI.

## Explicit non-goals

- Payments, tariffs, storefronts, referrals, and promotional codes.
- A hosted multi-tenant service.
- A public web administration interface.
- Arbitrary command execution through Telegram or the API.
- Automatic installation of upstream `latest` builds.
- A runtime dependency on a central EZ VPN Lego server.

## Availability rules

- Xray data-plane availability must not depend on the bot, database, GitHub, or
  Telegram being reachable.
- A control-plane failure must not invalidate existing credentials.
- Reconciliation must preserve the last verified Xray configuration.
- Reboot, failed update, and database restore are release acceptance tests.

## Supported environment

- Ubuntu Server 24.04 LTS.
- `amd64` and `arm64`.
- Minimum target: 1 vCPU, 1 GiB RAM, 10 GiB free disk.
- Recommended target: 2 vCPU, 2 GiB RAM, 20 GiB free disk.
- A public IPv4 or IPv6 address with TCP 443 available.


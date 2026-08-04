# Operations guide

## Supported host

The initial release supports a clean Ubuntu Server 24.04 LTS host on `amd64`
or `arm64`. It owns TCP port 443. A domain, Proxmox, Docker, and a public web
panel are not required.

## Runtime layout

- immutable versions: `/usr/local/lib/ez-vpn-lego/releases/`
- active version symlink: `/usr/local/lib/ez-vpn-lego/current`
- secrets and environment: `/etc/ez-vpn-lego/`
- encrypted desired state and audit log: local PostgreSQL database `ezvpn`
- Xray configuration: `/etc/xray/config.json`
- candidates and backups: `/var/lib/ez-vpn-lego/`
- local APIs: `/run/ez-vpn-lego/control.sock` and `helper.sock`

The Xray data plane stays available if Telegram or the control daemon is down.

## Backup and restore

`sudo ez-vpn-lego backup` creates a root-readable archive containing a
consistent PostgreSQL dump, Xray configuration, and the encryption keys needed
to decrypt credentials. Treat the archive like a password: copy it to encrypted
offline storage. The daily timer keeps local backups in
`/var/lib/ez-vpn-lego/backups`.

Restore on the same supported operating system with:

```bash
sudo ez-vpn-lego restore /secure/path/ez-vpn-lego_TIMESTAMP.tar.gz
```

The restore command rejects unexpected archive paths and verifies every file
against the backup manifest before replacing state.

## Updates and rollback

`sudo ez-vpn-lego update` downloads the newest stable GitHub release. It checks
the Ed25519 signature and SHA-256 digest before extraction, creates a pre-update
backup, switches one symlink, and performs a 30-second health check. Failed
health checks switch the symlink back and restart the previous binaries.

Pin a version when required:

```bash
sudo ez-vpn-lego update v1.2.3
```

## Diagnostics

```bash
sudo systemctl status lego-vpn-helper lego-vpnd lego-vpn-bot xray
sudo journalctl -u lego-vpnd -u lego-vpn-helper -u lego-vpn-bot -u xray
sudo -u ezvpn lego-vpnctl status
```

Never paste Telegram tokens, master keys, API tokens, full VLESS links, or
backup archives into an issue.

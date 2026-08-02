# Threat model

Status: initial draft. Security claims must be backed by tests before release.

## Protected assets

- Reality private keys and device credentials.
- Telegram bot token and administrator identities.
- PostgreSQL contents and encrypted backups.
- The last verified data-plane configuration.
- Release signing trust root.

## Trust boundaries

- Telegram and its Bot API are external and untrusted transports.
- GitHub is a distribution channel, not the release trust root.
- Input from bot, CLI, manifests, drivers, and the network is untrusted.
- The privileged helper is a separate high-risk boundary.

## Primary threats and controls

| Threat | Required control |
| --- | --- |
| Stolen activation link | Random 256-bit token, hash at rest, short expiry, single use |
| Stolen bot token | Root-readable secret file, redaction, rotation and owner recovery |
| Unauthorized Telegram user | Explicit binding, RBAC, deny by default, audit record |
| Command injection | No shell-string API; typed helper operations and strict validation |
| Malicious release | Offline trust key, signature verification, pinned versions |
| Broken update | Backup, staged switch, health check, bounded automatic rollback |
| Invalid Xray config | Candidate validation before atomic replacement |
| Database compromise | Local socket only, least-privilege role, restricted filesystem |
| Secret leakage in logs | Structured fields, centralized redaction, regression tests |
| Telegram outage | Existing VPN remains independent; local recovery CLI remains usable |
| Brute force or replay | Rate limits, expiring tokens, idempotency keys, replay rejection |

## Privilege policy

- Bot and daemon run as distinct unprivileged system users.
- PostgreSQL is never exposed publicly by the installer.
- The helper has no generic command or path parameters.
- systemd sandboxing and filesystem allowlists are mandatory release criteria.
- Secrets are never supplied as process arguments or committed to Git.

## Out of scope

- Compromise of the VPS root account or host kernel.
- Compromise of the owner's Telegram account or client device.
- Protection from a malicious VPS provider.
- Guaranteed censorship resistance on every network.


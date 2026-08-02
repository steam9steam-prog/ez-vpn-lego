# Data model

The initial PostgreSQL schema is defined in `migrations/000001_initial.up.sql`.

## Identity

- `admins` contains authorization roles and lifecycle state.
- `admin_identities` binds an administrator to an external identity such as a
  Telegram user ID without coupling the core model to Telegram.
- Exactly one active owner is allowed.

## Access

- `users` represents a person known to the owner.
- `devices` represents independently revocable access owned by a user.
- `credentials` binds a device to a node and versioned protocol driver data.
- Sensitive credential payloads use `bytea`; application-layer envelope
  encryption will be specified before credentials are implemented.

## Reconciliation

- `nodes` represents managed data-plane nodes, including the local node.
- `config_revisions` stores immutable rendered configurations.
- At most one revision per node is marked verified.

## Reliable work

- `operations` is the persistent job state and idempotency boundary.
- `audit_events` is an append-only application audit stream.
- `outbox_events` reliably delivers domain events to adapters.

## Lifecycle

- `releases` records compatibility manifests and installation state.
- `backups` records encrypted artifacts and their verification state.

Application IDs are UUIDs generated before insertion. Database timestamps use
`timestamptz`. Physical deletion is not part of ordinary API operations.


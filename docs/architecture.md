# Architecture

Status: draft for review.

## Context

```text
Telegram ──► lego-vpn-bot ──┐
                            ├──► versioned API ──► lego-vpnd ──► PostgreSQL
SSH ──────► lego-vpnctl ────┘                         │
                                                     ▼
                                            privileged helper
                                                     │
                                                     ▼
                                                   Xray
```

All control-plane services are local to the VPS. The API listens on a Unix
socket by default. Xray is the only public data-plane service.

## Process boundaries

### `lego-vpnd`

Owns domain rules, desired state, operation state machines, reconciliation,
release metadata, audit records, and the versioned control API. It does not run
as root and does not contain Telegram-specific behavior.

### `lego-vpn-bot`

Translates Telegram updates into API calls and API results into localized
messages. It cannot edit Xray files or execute commands.

### `lego-vpnctl`

Uses the same API for ordinary commands and provides deliberately small local
recovery paths when the daemon or database is unavailable.

### `lego-vpn-helper`

Runs with the minimum required privileges. It accepts typed, authenticated
operations rather than shell strings. Initial operations are configuration
install, service reload, release switch, and backup restore.

## Core model

```text
Admin ──► Role
User ──► Device ──► Credential ──► Protocol Driver
Node ──► Desired Revision ──► Applied Revision
Operation ──► Events
Release ──► Components and compatibility constraints
Backup ──► State and secret material
```

Users and devices are separate so a lost device can be revoked without
revoking every credential belonging to a person. A node exists in the model
even while the first supported topology contains only one node.

## State changes

```text
API request
  → database transaction and queued operation
  → render candidate configuration
  → validate candidate
  → store immutable revision
  → atomically install candidate
  → reload Xray
  → verify locally
  → mark applied, or restore the previous verified revision
```

Long-running mutations return an operation identifier. Clients poll operation
state or consume events; they do not hold a request open during system work.

## Extension boundary

Protocol drivers expose capabilities, validation, planning, application,
verification, rollback, credential issuance, revocation, and export. Drivers
are compiled into official releases. A versioned out-of-process protocol may be
added later; Go runtime plugins are explicitly excluded.

## Data and events

PostgreSQL is the single supported database. SQL migrations are reviewed and
versioned. Domain changes, operations, audit entries, and outgoing events use
the same transaction. A persisted outbox prevents notification loss when
Telegram is unavailable.

## Compatibility

- HTTP paths are versioned from `/v1`.
- Stored driver payloads carry a schema version.
- Release manifests declare database and component compatibility.
- Breaking API changes require a new API version.
- Irreversible migrations require an explicit release gate and restore plan.


# ADR-0003: Versioned HTTP API over a Unix socket

- Status: accepted
- Date: 2026-08-02

## Decision

Official adapters communicate with the daemon through a versioned REST API
transported over a local Unix socket. OpenAPI 3.1 is the source contract.

## Rationale

HTTP provides broad tooling and language support without opening a network
listener. A documented contract keeps Telegram, CLI, and future community
adapters independent from domain implementation.

## Consequences

Socket filesystem permissions are part of authorization. Mutations use
idempotency keys. Long-running work returns persistent operation identifiers.


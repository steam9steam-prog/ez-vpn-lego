# ADR-0002: PostgreSQL as the only state database

- Status: accepted
- Date: 2026-08-02

## Decision

PostgreSQL is the only supported application database. It listens locally and
is installed and maintained by the product installer.

## Rationale

Persistent jobs, transactional outbox records, audit history, and concurrent
adapters benefit from PostgreSQL transactions and locking. Supporting both an
embedded database and PostgreSQL would double migration and recovery paths.

## Consequences

The installer owns secure local configuration, upgrades, backup, restoration,
and resource tuning for small VPS installations.


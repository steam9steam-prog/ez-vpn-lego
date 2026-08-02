# ADR-0001: Go binaries and native system services

- Status: accepted
- Date: 2026-08-02

## Decision

Control-plane components are implemented in Go 1.26 and distributed as signed
Linux binaries for `amd64` and `arm64`. Production services run under systemd.
Node.js, Python, containers, and configuration-management runtimes are not
required on an installed VPS.

## Rationale

The product needs small self-contained artifacts, predictable cross-compilation,
fast startup, explicit interfaces, and straightforward system integration.

## Consequences

The repository uses one application language. Web interfaces, if introduced,
may use a separate frontend toolchain but cannot own domain logic.


# Contributing

The project is in architecture bootstrap. Discuss changes to public APIs,
database schemas, driver contracts, privilege boundaries, and release behavior
before implementation.

All changes must:

- keep the data plane independent of the control plane;
- avoid generic privileged command execution;
- include tests for behavior and failure paths;
- include a migration and rollback analysis when state changes;
- update relevant architecture documentation.

Run `make fmt vet test build` before opening a pull request.


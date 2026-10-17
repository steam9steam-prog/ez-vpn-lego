# Product requirements

Status: agreed scope for the modular release. Implementation status is tracked
below. The published installer still provides the single-node release.

## Product

EZ VPN Lego lets an owner deploy an independent VPN installation on clean
servers, add nodes, share access, and maintain it through Telegram. It does not
require a hosted Lego account or a web panel.

Two traffic shapes are supported:

1. User to VPN node to internet.
2. User to entry node to exit node to internet.

There is no fixed entry or exit count. Route depth is limited to these two
shapes. Country is metadata. One controller can share an entry or exit host,
or run separately. An installation does not manage unrelated customer services.

## First modular release

- Install on supported clean operating systems with a small number of commands.
- Register additional nodes without manually editing Xray configuration.
- Support multiple entries and exits, including direct and cascade routes together.
- Use stable IDs and preserve existing credentials across topology changes.
- Keep working traffic independent from the bot and controller.
- Manage nodes, access, subscriptions, diagnostics, backup and updates in Telegram.
- Keep a local CLI for recovery through SSH.
- Provide persistent individual subscription URLs and independently revocable access.
- Validate candidates, apply dependencies safely, verify real traffic and roll back failures.
- Notify Telegram after confirmed failures and recovery, without automatic country changes.
- Update the application and network components explicitly, with verification and rollback.
- Treat operating system security updates as a separately configured schedule.
- Publish a maintained connection-profile catalog, with compatibility and dated evidence.
- Validate Happ, Throne and Shadowrocket versions before claiming compatibility.
- Support domain-free REALITY and owner-controlled domains for ordinary TLS and public HTTPS subscriptions.
- Keep RU, EN and Simplified Chinese entry guides aligned.

## Optional modules after the first release

- Access expiration, traffic quotas and connection limits, without HWID.
- Remnawave and Marzban adapters. One backend owns each node's configuration;
  installing a panel must not create two competing writers.
- Additional protocol engines and transports, based on evidence and maintainability.

These modules must use explicit capabilities. An unavailable adapter or
unsupported operation is an error, not a silent fallback to native ownership.

## Not in scope

- Arbitrary multi-hop chains.
- A central service linking independent customers.
- Billing, storefronts, referrals and payment integrations in the first release.
- Hardware identity or a claim that connection count equals physical device count.
- Seamless failover or an automatic change of exit country.
- Unattended replacement of a working profile based only on a catalog recommendation.
- A built-in public web panel.
- Automatic import over an existing unmanaged VPN installation.
- Claims that a transport works throughout Russia or China without field evidence.

## Implementation status

Implemented in the topology workbench:

- validated node roles, control placement and the two traffic shapes;
- native ownership and a backend capability boundary;
- deterministic Xray rendering for direct and cascade routes;
- profile catalog with REALITY Vision and experimental XHTTP profiles;
- stable server material, explicit per-device route grants and subscription tokens;
- access disable/enable in rendered generations;
- domain requirements and DNS checks, including stale AAAA records;
- local CLI, shared read-only planning API and Telegram profile catalog;
- real loopback traffic tests distinguishing direct and cascade egress.

Still required for the modular release:

- remote enrollment, authenticated agent transport and fleet reconciliation;
- durable fleet operations and dependency-aware rollback;
- certificate issuance, renewal and HTTPS subscription publishing;
- full node and route management in Telegram;
- runtime route checks and durable notifications;
- backup, restore and update across all nodes;
- disposable multi-VM installation tests, client checks and load measurements.

The workbench prepares files. It does not apply them to servers. A valid plan
or a generated subscription is not evidence of a deployed VPN.

## Acceptance scenarios

1. Install one traffic node with colocated control. Connect from a supported client.
2. Expand a clean test installation to one entry and three selectable exits.
3. Deploy the same topology with separate control.
4. Place control on an exit without adding a traffic hop.
5. Add another entry, grant selected routes, and preserve existing access.
6. Revoke an imported credential and verify that the credential itself stops working.
7. Stop the bot and controller; verify existing routes still carry traffic.
8. Break one exit; notify once after confirmation, keep other routes working,
   and do not fall back to direct egress.
9. Interrupt an apply and an update, restart control, and verify recovery.
10. Restore a backup onto clean hosts and preserve access and subscription identity.
11. Test domain-free REALITY, ordinary TLS with a domain, and both subscription port layouts.
12. Record client versions, node resources, concurrent load and measured throughput.

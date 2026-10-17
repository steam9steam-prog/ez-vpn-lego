# ADR-0004: Independent installations and modular topologies

- Status: accepted
- Date: 2026-10-17

## Decision

One installation has one controller and any number of entry and exit nodes.
A controller may share a traffic host or run separately. It is not implicitly
part of a traffic route.

A route is either direct or entry to exit. It cannot contain a third traffic
node. Geography is display metadata. Multiple routes can share an entry
listener when profile and domain match; credentials select the exit.

The native backend renders Xray configuration. Remnawave and Marzban are
reserved backend identities, not implemented adapters. A future adapter must
report capabilities and own its node configuration. Lego must reject missing
adapters and mixed ownership rather than overwrite panel state.

Profile definitions, topology planning, rendering, transport to nodes,
subscriptions and Telegram are separate concerns. Read-only planning does not
issue credentials or connect to a server. Render uses private material that is
stored separately from the public installation description.

## Domain policy

REALITY can use IP endpoints without a domain owned by the installer. Ordinary
TLS and the standard public HTTPS subscription flow use owner-controlled DNS.
An installation does not depend on a project-operated shared domain.

DNS checks inspect all address families. A hostname does not solve a conflict
between HTTPS and REALITY listeners on one IP and port. Subscription publishing
must support a separate port, IP, or controller, and eventually an XHTTP/TLS
gateway sharing routes by path.

## Availability and updates

Control outages do not revoke or interrupt existing traffic. A failed exit
must not silently become a direct route. Confirmed failures trigger notifications;
there is no automatic country change. Apply and update are explicit operations
with validation, verification, persisted progress and recovery.

## Staging

The initial implementation is an offline topology workbench and shared planning
API. It does not replace the current installer, persist fleet state, issue
certificates or apply remote configurations. The local private material file
is not the production controller store. Remote operations and Telegram fleet
management must be verified before a modular release is advertised as installable.

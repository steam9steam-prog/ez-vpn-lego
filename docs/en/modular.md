# Try the modular workbench

[Русский](../ru/modular.md) | [English](../en/modular.md) | [简体中文](../zh-CN/modular.md)

This is an offline preparation tool. It does not connect to a VPS or change
an existing VPN. Remote apply and full Telegram fleet management are next.

## Build

Use Go 1.26.9. From the repository root:

```bash
mkdir -p bin workbench
go build -o bin/lego-vpnctl ./cmd/lego-vpnctl
cp examples/cascade.json workbench/installation.json
bin/lego-vpnctl topology plan workbench/installation.json
```

The example has one entry and three exits. Its documentation addresses are not
usable server addresses. Other examples are `direct.json`,
`separate-control.json`, and `xhttp-tls.json` in `examples`.

## Nodes and routes

- `control` manages the installation. It is not a traffic hop by itself.
- `entry` accepts user connections.
- `exit` sends cascade traffic to the internet.

Roles can share a machine. Exactly one node has the control role. There is no
fixed entry or exit count. Country is display metadata, not a routing rule.

A route has an `entry_id` and an optional `exit_id`. Omitting the exit creates
a direct route. Providing an exit creates a two-node route. Longer chains are
not supported.

Routes may share an entry port when their profile and domain match. Separate
credentials select their exits. An unavailable exit does not cause a direct
fallback. Inter-node links use REALITY on TCP 10443 by default. An exit's
`relay_port` can override this port.

Optional `reality_target` (`host:port`) and `reality_server_name` must be set
together. The default is `www.microsoft.com:443`. Check target suitability
from your own network.

Use `owner: lego` today. `remnawave` and `marzban` are reserved ownership values.
Until an adapter exists, the planner rejects these nodes rather than silently
replacing a panel's configuration.

## Create keys and access

```bash
bin/lego-vpnctl topology init workbench/installation.json workbench/state.json
bin/lego-vpnctl topology grant workbench/installation.json workbench/state.json phone "My phone" uk us ca
bin/lego-vpnctl topology render workbench/installation.json workbench/state.json workbench/generation-1
```

Output contains node configurations in `generation-1/nodes`, subscription files
in `generation-1/subscriptions`, and checksums in `manifest.json`. Command output
does not print credentials. Each permitted route gets a separate credential.

`state.json` contains private keys. Its permissions are 0600; generation
directories use 0700. This is a local workbench, not the production controller's
store: the file is not encrypted. Do not publish it or serve the whole output
directory over HTTP. `workbench` is ignored by Git.

Subscription filenames are random and stable between generations. Only the
`subscriptions` directory may be published through a separately configured
HTTPS service, with listing and caching disabled. The renderer does not start
that service. Publishing `nodes` would expose private keys.

## Extend an installation

After editing the installation file:

```bash
bin/lego-vpnctl topology extend workbench/installation.json workbench/state.json
bin/lego-vpnctl topology render workbench/installation.json workbench/state.json workbench/generation-2
```

Existing keys are preserved. New routes are not automatically granted to
existing devices. Update the route set while preserving credentials for retained
routes and the subscription token:

```bash
bin/lego-vpnctl topology routes workbench/installation.json workbench/state.json phone uk us ca
```

Omitted routes are revoked in the next rendered generation. Reinitializing an existing state file is refused. Each render uses a new
output directory.

## Revoke or restore access

```bash
bin/lego-vpnctl topology device workbench/state.json phone disable
bin/lego-vpnctl topology render workbench/installation.json workbench/state.json workbench/generation-3
```

The new generation omits both the subscription and the entry credentials.
Previously imported credentials keep working until the server configuration is
actually replaced. Use `enable` and render again to restore the same credentials
and subscription token.

Removing a route drops it from the rendered configuration and exports. Old
material remains in the private state for an explicit operator rollback.

## Profiles and DNS

```bash
bin/lego-vpnctl profiles list
bin/lego-vpnctl topology dns workbench/installation.json
```

REALITY Vision is implemented. XHTTP REALITY and XHTTP TLS are experimental.
TLS needs a domain and separately provisioned certificates. See [domains](domains.md).

The shared control API exposes `GET /v1/profiles` and `POST /v1/topology/plan`.
Planning returns no secrets and applies no changes. After owner pairing,
Telegram can display the catalog through `/profiles`.

## Verification

Tests cover key preservation, revocation, conflicting ports, control placement,
and planning an installation with 1003 nodes. The last check validates the model,
not production capacity.

With `XRAY_BINARY` set, the integration test starts real Xray processes on
loopback and checks direct and cascade traffic. Different egress source addresses
catch an unintended direct fallback.

Field checks in Russia and China, installed mobile client versions, real VPS
runs, and load tests are still required. The catalog makes no country-wide
reachability claim.

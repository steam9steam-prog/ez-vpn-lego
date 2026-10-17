# Domains

[Русский](../ru/domains.md) | [English](../en/domains.md) | [简体中文](../zh-CN/domains.md)

## Can I run it without my own domain?

Yes. VLESS REALITY Vision and XHTTP REALITY can use the server IP address.
REALITY uses the name of an external TLS site; you do not need to own that
name. Check that the chosen target is reachable from the node.

This works for direct and two-node routes. Three exits behind one entry do
not require three domains. Inter-node REALITY links can also use IP addresses.

Without your own domain you can share a connection URI or QR code. Lego's
planned public HTTPS subscription setup uses a domain controlled by its owner.

## Ordinary TLS and subscriptions

Use a domain from any registrar, or a subdomain of one you already own.
For example: `vpn.example.com` and `sub.example.com`. Lego does not sell domains
or require a shared domain run by the project.

The VPN hostname points to the entry, not to an overseas exit. The subscription
hostname points to its publisher, which may run on a separate controller.

Create an A record for IPv4. Add AAAA only if IPv6 is configured. The basic TLS
profile expects direct DNS, without a CDN proxy. In Cloudflare this is DNS only.
Remove stale AAAA records pointing elsewhere.

A dynamic DNS name can work if its provider permits the required certificate
and the name stays reachable. No particular DNS provider is mandatory.

## Checks available now

```bash
lego-vpnctl topology plan installation.json
lego-vpnctl topology dns installation.json
```

The plan lists domain requirements. The DNS check compares all A and AAAA
answers with the node address. If the node uses both address families, its
`address` may be a hostname resolving to both expected addresses.

This does not issue a certificate or test a user's operator. In the current
workbench, provision certificates separately at
`/etc/ez-vpn-lego/certificates/NAME/fullchain.pem` and `privkey.pem`.
Allow Xray to read the key, without making it public. Automated issuance and
renewal belong to the next installer stage.

## When port 443 is occupied

Separate REALITY and ordinary HTTPS processes cannot both listen on the same
IP and TCP 443. A second hostname does not solve the port conflict.

For a combined server using REALITY on 443, the planned HTTPS subscription
publisher can use another port, such as 8443. Other options are another IP or
a separate controller with HTTPS on 443. XHTTP/TLS can share an HTTPS gateway
with subscriptions through separate paths; that gateway is not implemented yet.

The installer must explain the choice before applying changes and check port
availability. Existing websites and mail records must not be overwritten.

## Ownership and recovery

The owner keeps control of registration and DNS. Moving servers should preserve
personal subscription URLs by updating DNS or subscription content. Emergency
SSH export must remain possible. These are modular release requirements,
not completed automation in this preview.

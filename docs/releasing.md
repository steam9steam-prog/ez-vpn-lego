# Release procedure

Releases are built for `linux/amd64` and `linux/arm64`. The build script pins
Xray and verifies its upstream SHA-256 digest. GitHub Actions then signs one
checksum manifest and attaches the archives, manifest, and signature to an
immutable versioned release.

The Ed25519 private key must never enter the repository. Base64-encode the PEM
private key and store it as the repository Actions secret
`RELEASE_SIGNING_KEY_B64`. The corresponding public key is
`packaging/release-public.pem` and is embedded in the installer.

Before tagging:

1. Run `go test -race -p 1 ./...` against PostgreSQL.
2. Run `tests/e2e/ubuntu-vm.sh` on a KVM-capable Linux host.
3. Confirm the release notes and migration compatibility.
4. Create and push a signed annotated SemVer tag.

Rotating the release key requires a separately reviewed installer update and a
documented transition release signed by the old key.

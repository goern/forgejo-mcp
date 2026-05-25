# Pinned Tool Versions

This file is the single source of truth for all tool versions baked into the release-tools image.
Update this file, then regenerate `npm/package-lock.json` and rebuild the image.

## Base Image

| Image | Tag | Digest | Notes |
|---|---|---|---|
| `registry.access.redhat.com/hi/go` | `latest-builder` | `sha256:d8c8b702b8a54150e8fdca86753f581d98c551ab8a3fd429886d4ddd4e949894` | **TAG-FLOATING RISK**: Hummingbird has not published a specific `MAJOR.MINOR-builder` tag as of 2026-05-25; only `latest-builder` is available. Upgrade to a pinned `1.26-builder` or `1.26.3-builder` tag when Hummingbird publishes one. Track at https://catalog.redhat.com/software/containers/hi/go/. |

Go version shipped: `1.26.3` (from image label `org.opencontainers.image.version`)

## Tools Installed via Build Stage (compiled/fetched)

| Tool | Version | Source | Notes |
|---|---|---|---|
| `syft` | `v1.44.0` | curl install script from anchore releases | Binary at `/usr/local/bin/syft` |
| `goreleaser` | `v2.16.0` | `go install github.com/goreleaser/goreleaser/v2@v2.16.0` | Binary at `/usr/local/bin/goreleaser` |
| `cosign` | `v3.0.6` | curl prebuilt linux/amd64 binary from sigstore releases | Binary at `/usr/local/bin/cosign` |

## Tools Installed via dnf (final stage)

| Tool | Version | Source |
|---|---|---|
| `node` | 22.x (from RHEL/UBI stream) | `dnf install -y nodejs` |
| `npm` | (with nodejs) | `dnf install -y npm` |
| `jq` | (latest in stream) | `dnf install -y jq` |
| `curl` | (latest in stream) | `dnf install -y curl` |
| `ca-certificates` | (latest in stream) | `dnf install -y ca-certificates` |

## npm Package (@anthropic-ai/mcpb)

| Package | Version | npm Integrity Hash | Tarball SHA256 |
|---|---|---|---|
| `@anthropic-ai/mcpb` | `2.1.2` | see `npm/package-lock.json` | see `npm/package-lock.json` `integrity` field |

The tarball SHA256 is recorded in `npm/package-lock.json` under `integrity` (sha512 format).
A manual integrity check: `npm pack @anthropic-ai/mcpb@2.1.2 --dry-run` and compare hash.

## Version Bump Policy

| Bump | Triggers |
|---|---|
| MAJOR | base image swap, removed bundled tool, removed/moved binary path, breaking CLI change, shell removed from final stage |
| MINOR | new bundled tool, Hummingbird base MINOR bump, Go version bump within same major, bundled tool MINOR bump |
| PATCH | bundled tool PATCH bumps, security backports, rebuild with no observable contract change |

Renovate manages automated bump PRs. Go bumps are treated as MINOR (not MAJOR) because
goreleaser's CLI contract is unaffected by Go's own backward-compatibility guarantee.

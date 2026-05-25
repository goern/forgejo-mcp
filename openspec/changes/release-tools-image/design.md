## Context

Each of three failed release runs (v2.24.0/1/2) hit a bug rooted in installing tools at Step time. The release-tools image is the architectural exit from that pattern. This design freezes the image layout, build/publish pipelines, and isolation contract so the image is portable to a separate project later.

## Goals

- **Single image, all release tooling**, version-pinned, signed, SBOM-attached.
- **CEL-gated pipelines** so image work and forgejo-mcp Go work do not cross-trigger each other.
- **Hummingbird base** for hardened, supported foundation with bash + dnf available where needed.
- **Structural isolation**: `image/release-tools/` + `.tekton/release-tools/` are the only paths touched. Lift = `git mv` + registry path edit.

## Non-Goals

- Rewriting `goreleaser-release.yaml`, `cosign-sign-release.yaml`, `mcpb-pack.yaml` to consume the image — separate follow-up change.
- Multi-arch image. amd64-only on first cut; arm64 deferred.
- Konflux pipelines. PaC + Tekton only.

## Decisions

### D1: Hummingbird as base (`registry.access.redhat.com/hi/go:<ver>-builder`)

**Why:** maintainer directive. Hummingbird publishes hardened, Red Hat-backed minimal images with both runtime and `-builder` variants. The builder variant ships bash + dnf, which lets us install non-Go tools (jq, node, npm, ca-certificates) via dnf without dropping to a non-Hummingbird base. Go toolchain present.

**Alternatives rejected:**
- `docker.io/library/golang:1.25` — works, but unhardened upstream and pulls a Debian rootfs we don't audit.
- `chainguard/go` — close to Hummingbird's posture but pulls from a different vendor universe than the op1st-pipelines runtime; mixing vendors complicates supply-chain story.
- `cgr.io/chainguard/cosign-bash` etc. — multiple specialized images instead of one. Defeats the purpose of consolidating.

**Risk:** Hummingbird is young; `hi/go` tag cadence may lag upstream Go releases. Mitigation: Renovate-driven bumps, accept lag in exchange for hardened base.

### D2: Multi-stage Containerfile, runtime stage retains Go

GoReleaser invokes `go build` at release time → the consuming Tekton Task needs a full Go toolchain in the runtime image, not just the goreleaser binary. So the "runtime" stage is effectively `hi/go:<ver>-builder` with extra tools layered on, not a `hi/core-runtime` minimal variant.

Build stages compile/fetch:
- syft (curl install script, pinned version)
- goreleaser (`go install` from pinned tag, then copy binary)
- cosign (curl prebuilt linux/amd64 binary, pinned version)
- @anthropic-ai/mcpb (npm install, prefetched into npm cache)

Final stage: `hi/go:<ver>-builder` + `dnf install nodejs npm jq curl ca-certificates` + COPY of compiled binaries + warmed npm cache.

### D3: CEL gates on PR + tag pipelines

PaC supports `pipelinesascode.tekton.dev/on-cel-expression`. Two gates:

**PR build pipeline** — runs when any file under `image/release-tools/` or `.tekton/release-tools/` changes:

```yaml
pipelinesascode.tekton.dev/on-cel-expression: |
  event == "pull_request" && target_branch == "main" &&
  files.all.exists(p, p.matches("^(image/release-tools/|\\.tekton/release-tools/).*"))
```

**Tag publish pipeline** — runs only on tags scoped to release-tools:

```yaml
pipelinesascode.tekton.dev/on-cel-expression: |
  event == "push" && target_branch.matches("^refs/tags/release-tools/v.*")
```

This separation is load-bearing for isolation. A regular `v*` tag (forgejo-mcp release) MUST NOT trigger the image pipeline; a `release-tools/vX.Y.Z` tag MUST NOT trigger the forgejo-mcp release pipeline. Both annotations enforce this via prefix matching on `refs/tags/...`.

**Risk:** CEL `files.all` requires a recent enough PaC version (≥0.22.0). Op1st-pipelines version must be verified before merging.

### D4: Registry path and tag scheme

Working assumption: `quay.io/operate-first/release-tools:vX.Y.Z` (plus `:latest` floating for non-prod). Final path coordinated with op1st-emea-b4mad maintainers before publish pipeline lands.

Tag scheme: `release-tools/vMAJOR.MINOR.PATCH`. The `release-tools/` prefix is what the tag-publish pipeline matches on; same repo can carry both `v2.24.x` (forgejo-mcp) and `release-tools/v1.0.0` (image) without ambiguity.

Versioning policy: semver of the image itself, NOT the embedded Go version. A Go bump from 1.25 → 1.26 is a MAJOR (consumer rebuilds). A goreleaser patch bump is a PATCH.

### D5: Signing

Image manifest signed via `cosign sign <image-ref>` using the same `cosign-signing-key` Secret already provisioned in `op1st-pipelines` (the one the forgejo-mcp release pipeline uses). One keypair covers both release artifacts and release-tools image — operational simplicity outweighs the marginal blast-radius cost.

Public key location: same op1st-emea-b4mad path the README "Verifying Releases" chapter points to. Users wishing to verify the image use the same public key documented for forgejo-mcp releases.

### D6: SBOM

Build pipeline emits a CycloneDX SBOM via `syft <image-ref>` during the build Task, attached to the image manifest via `cosign attach sbom` or as an OCI referrer. Consumers can fetch SBOM via `cosign download sbom` or via referrers API.

### D7: Pipelines reuse `buildah` Task from op1st catalog

`op1st-pipelines/unprivileged-building-of-container-images-using-buildah` exists in op1st-emea-b4mad. Reuse via taskRef. If functionality gaps appear, fall back to inline buildah scripts before adding new local Tasks.

## Risks and mitigations

| Risk | Mitigation |
|---|---|
| Hummingbird `hi/go` lags upstream Go | Pin via Renovate, accept lag; can override base in a `LOCAL_BASE` ARG for emergencies |
| op1st-pipelines PaC version too old for `files.all` CEL | Verify before merge; fall back to `body.pull_request.changed_files` style match |
| Single keypair shared between artifact + image signing | Document key rotation procedure; if blast-radius becomes a concern later, split into separate keypairs |
| Image grows to >1 GB (Hummingbird + Go + node + binaries) | Multi-stage; final stage discards build caches; accept image size budget of ~800MB for now |
| Registry path not yet agreed with op1st maintainers | Document as a precondition in tasks.md; do NOT merge publish pipeline until path is confirmed |

## Migration Plan

This change does not migrate existing release Tasks. It produces an image and its build/publish pipelines. Migration of the consuming Tasks is a separate change (`release-pipeline-use-release-tools-image`), gated on:

1. A signed `release-tools:v1.0.0` exists in the agreed registry.
2. Local `podman run --rm release-tools:v1.0.0 goreleaser --version` (and `syft`, `cosign`, `npx -y @anthropic-ai/mcpb`) all succeed.
3. The image's cosign signature verifies with the public key documented in README.

## Open Questions

- Final registry path: `quay.io/operate-first/release-tools`, `ghcr.io/operate-first/...`, or `codeberg.org/operate-first/...`? Maintainer decision.
- Should the image also bake in `tkn` CLI for emergency operator use? Defer — adds 15 MB; can be a follow-up.
- Multi-arch (arm64) — defer until Hummingbird publishes `hi/go:arm64` and a buildah-with-qemu Task exists in op1st-pipelines.

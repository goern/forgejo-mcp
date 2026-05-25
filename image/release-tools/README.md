# release-tools image

A versioned, signed OCI image bundling the full toolchain needed to cut a forgejo-mcp release.

## Purpose

The op1st Tekton release pipeline historically installed tools at Step runtime, causing three
production failures (v2.24.0/1/2). This image bakes all tooling in once, version-pinned and
cosign-signed, so release Steps simply consume a known-good image.

**Bundled tools:** `go`, `goreleaser`, `syft`, `cosign`, `node`, `npm`, `jq`, `curl`,
`ca-certificates`, `@anthropic-ai/mcpb`

See [VERSIONS.md](VERSIONS.md) for pinned versions and digest.

## Image Tag Scheme

Tags follow `release-tools/vMAJOR.MINOR.PATCH` in this repository.
The published OCI tag is `vMAJOR.MINOR.PATCH` at the agreed registry.

| Bump | Triggers |
|---|---|
| MAJOR | base image swap, removed tool, moved binary path, breaking CLI change, shell removed |
| MINOR | new tool added, Hummingbird base MINOR bump, Go version bump within same major, tool MINOR bump |
| PATCH | tool PATCH bumps, security backports, rebuild with no contract change |

## Pulling the Image

```bash
podman pull quay.io/operate-first/release-tools:v1.0.0
# or by digest (preferred for reproducibility):
podman pull quay.io/operate-first/release-tools@sha256:<digest>
```

Consumers SHOULD pin by full `vMAJOR.MINOR.PATCH` tag or by digest for maximum stability.

## Verifying the Cosign Signature

The published image is signed with the op1st cosign key (same key used for forgejo-mcp release
artifacts). The public key is documented at the same location referenced in the forgejo-mcp
release verification guide.

```bash
cosign verify \
  --key https://raw.githubusercontent.com/operate-first/common/main/cosign.pub \
  quay.io/operate-first/release-tools:v1.0.0
```

Expected output: `Verification for quay.io/operate-first/release-tools:v1.0.0 -- The following checks were performed on each of these signatures: ...`

## Running Locally

```bash
# Run all tools version check
podman run --rm quay.io/operate-first/release-tools:v1.0.0 \
  sh -c 'go version && syft version && goreleaser --version && cosign version && jq --version && curl --version && node --version'

# Run goreleaser in release mode
podman run --rm \
  -v $(pwd):/workspace:z \
  -w /workspace \
  quay.io/operate-first/release-tools:v1.0.0 \
  goreleaser release --clean
```

## Building Locally

Use `build.sh` to reproduce the image build without pushing:

```bash
bash image/release-tools/build.sh
```

This mirrors the PR pipeline build step. Requires `podman` and `git`.

## Verifying Tool Versions (local image)

```bash
bash image/release-tools/verify.sh
```

Runs the locally built image and asserts all tool versions match the pins in VERSIONS.md.

## Bumping Versions

1. Edit [VERSIONS.md](VERSIONS.md) with the new version string(s).
2. If bumping `@anthropic-ai/mcpb`: regenerate the lockfile:
   ```bash
   cd image/release-tools/npm
   npm install --package-lock-only
   ```
3. Rebuild and verify locally:
   ```bash
   bash image/release-tools/build.sh && bash image/release-tools/verify.sh
   ```
4. Open a PR. The PR build pipeline fires automatically when files under `image/release-tools/`
   change (CEL-gated).

Renovate manages automated bump PRs. Manual review is required for all bumps — see `renovate.json`.

## Lift-Out to a Separate Project

The image source is structurally isolated so it can be moved to a dedicated repository via:

1. `git mv image/release-tools/ .tekton/release-tools/ <new-repo>/`
2. Register the new repository as a PaC `Repository` CR in `op1st-pipelines`.
3. Ensure `cosign-signing-key` Secret is accessible in the new namespace (extend the
   emberstack reflector ruleset or keep PipelineRuns in `op1st-pipelines`).
4. Update the registry path string in `on-tag-publish.yaml` if publishing to a different registry.
5. Update ADR cross-references in `docs/design/release-pipeline-migration.md` to point at
   the new location.

No Go code in `operation/`, `pkg/`, `cmd/`, or `main.go` is affected.

## Architecture

```
image/release-tools/
  Containerfile        # Multi-stage build (tools-builder + final stage)
  VERSIONS.md          # Single source of truth for all pinned versions
  renovate.json        # Renovate bump config
  .dockerignore        # Narrows build context to image/release-tools/ only
  build.sh             # Local build script (no push)
  verify.sh            # Local verification script
  npm/
    package.json       # @anthropic-ai/mcpb dependency declaration
    package-lock.json  # Lockfile with integrity hashes for all transitive deps
```

```
.tekton/release-tools/
  on-pull-request-build.yaml   # PR build pipeline (CEL-gated to image tree)
  on-tag-publish.yaml          # Tag publish pipeline (CEL-gated to release-tools/v* tags)
```

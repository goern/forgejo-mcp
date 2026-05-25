## Why

The op1st Tekton release pipeline (added in PR #150) installs syft, goreleaser, cosign, node, jq, and curl at runtime in each release Step. Three distinct hazards surfaced on the first three live runs (`v2.24.0`, `v2.24.1`, `v2.24.2`):

1. **Step-isolation:** Tekton Step containers in a Pod share workspaces but not `/usr/local/bin/`. Tools installed in `install-*` Steps disappear before the `release` Step starts → `goreleaser: command not found` (`v2.24.0`). Workaround: PR #153 collapses install+run into a single Step, losing log granularity.
2. **Workspace pollution:** `GOCACHE`/`GOMODCACHE` set to paths inside the workspace mount made `git status` dirty → `goreleaser` refused to publish (`v2.24.1`). Workaround: PR #154 moves caches to `/tmp` (per-Step, no cross-Step reuse).
3. **Distroless cosign image:** `ghcr.io/sigstore/cosign/cosign:vX` has no shell → Tekton `script:` blocks fail with `fork/exec /tekton/scripts/script-0-XXXX: no such file or directory` (`v2.24.2`, no `.sig` shipped).

All three are symptoms of the same root cause: each Task improvises its own toolchain in a stock public image. A dedicated, pre-built **release-tools** OCI image with all tooling baked in eliminates the runtime-install pattern outright. It also makes the image-build itself a first-class artifact that can be reviewed, signed, version-pinned, and Renovated independently of the Tekton Tasks that consume it.

Per the maintainer's direction, the image source SHALL be **structurally isolated** in this repo (under `image/release-tools/`) so it can be lifted into a dedicated project later without touching the forgejo-mcp code path.

## What Changes

- **New image source tree** under `image/release-tools/`:
  - `Containerfile` — multi-stage build based on [Project Hummingbird](https://hummingbird-project.io/) images (`registry.access.redhat.com/hi/go:<ver>-builder` for build stage, `registry.access.redhat.com/hi/go:<ver>-builder` or a thinner runtime variant for the final layer). Provides hardened, Red Hat-supported base with bash + dnf.
  - Pinned tool versions baked in: Go (matches `hi/go` tag), syft, goreleaser, cosign, Node 22, npm, jq, curl, ca-certificates.
  - SBOM emission via syft at build time, attached to the published image.
  - cosign-signed at build time using the same `cosign-signing-key` Secret in `op1st-pipelines` (signature attached to the image manifest).
  - `README.md`, `VERSIONS.md`, and `Renovate` config so tool bumps are automated.

- **New Tekton build pipeline** at `.tekton/release-tools/on-pull-request-build.yaml`:
  - PaC trigger via **CEL** expression so the pipeline fires ONLY when files under `image/release-tools/**` change (PRs touching forgejo-mcp Go code MUST NOT trigger image rebuilds).
  - Builds the image with `buildah` (no push), verifies it boots and reports tool versions, scans with `syft`, fails if any pinned binary is missing.

- **New Tekton release pipeline** at `.tekton/release-tools/on-tag-publish.yaml`:
  - PaC trigger on push of tags matching `refs/tags/release-tools/v*`. CEL-gated to that tag prefix.
  - Builds the image, signs with cosign, pushes to a registry path coordinated with op1st-emea-b4mad maintainers (working assumption: `quay.io/operate-first/release-tools:vX.Y.Z`).
  - Publishes SBOM + signature as registry artifacts.

- **No changes to forgejo-mcp release pipeline in this change.** A follow-up change (`release-pipeline-use-release-tools-image`, not in this proposal) rewrites `.tekton/tasks/goreleaser-release.yaml`, `cosign-sign-release.yaml`, and `mcpb-pack.yaml` to reference the published image and drop the runtime installs + `/tmp` GOCACHE workaround.

## Capabilities

### New Capabilities

- `release-tools-image`: a versioned, signed OCI image bundling the toolchain needed to cut a forgejo-mcp release. Owns the Containerfile, version-pinning policy, build and publish pipelines, and the contract with consuming Tekton Tasks.

### Modified Capabilities

None in this change. The existing release Tasks continue to install tools at runtime until the follow-up rewrite lands.

## Impact

- **Code**: new tree `image/release-tools/{Containerfile,README.md,VERSIONS.md,renovate.json}`. New tree `.tekton/release-tools/{on-pull-request-build.yaml,on-tag-publish.yaml,tasks/}`. No edits to `operation/`, `pkg/`, or `main.go`.
- **CI surface**: two additional PaC PipelineRuns registered against the same `codeberg-org-goern-forgejo-mcp` `Repository` CR. CEL gating ensures they remain dormant on Go-only PRs.
- **External services**: registry push permission needed on the target registry (working assumption `quay.io/operate-first/release-tools`). Cosign signing reuses the existing `cosign-signing-key` Secret.
- **Docs**: README points to the published image as the canonical tooling for reproducing a release locally. ADR `docs/design/release-pipeline-migration.md` gets an addendum noting the image as the dependency for hard cutover.
- **Movability**: every path introduced lives under `image/release-tools/` or `.tekton/release-tools/`. Lifting to a separate project = `git mv` of those two trees + updating the registry path and PaC `Repository` CR. No code coupling.
- **Out of scope** (deferred): rewriting the consuming Tekton Tasks (next change); image hardening beyond Hummingbird's defaults; multi-arch image (amd64-only first cut); Konflux integration.

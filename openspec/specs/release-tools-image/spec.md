# release-tools-image Specification

## Purpose

The release-tools-image capability publishes a signed, SBOM-attested container image bundling Go, goreleaser, syft, cosign, jq, curl, and Node tooling for use by the forgejo-mcp release pipeline and other tag-driven workflows. The image lives in its own source tree under `image/release-tools/` and is built and published via dedicated Tekton PipelineRuns under `.tekton/release-tools/`, isolating its lifecycle from the consuming forgejo-mcp Go release.
## Requirements
### Requirement: Source tree isolation

All artifacts introduced by this capability SHALL live under exactly two top-level paths:

1. `image/release-tools/` — Containerfile, build scripts, version manifest, README, Renovate config.
2. `.tekton/release-tools/` — PR-build and tag-publish PipelineRun definitions plus any release-tools-only Tasks.

No file under any other path SHALL be added, modified, or deleted by this capability. Migrating the capability to a separate repository MUST be achievable via `git mv` of exactly these two trees plus a registry-path string change.

#### Scenario: Lift-out to separate project

The lift is NOT a one-step `git mv`. The full operator procedure is:

- **WHEN** a maintainer `git mv`s `image/release-tools/` and `.tekton/release-tools/` to a new repository
- **AND** registers the new repository as a PaC `Repository` CR in the `op1st-pipelines` namespace (separate manifest in `op1st-emea-b4mad`)
- **AND** ensures the new namespace has read access to `cosign-signing-key` — either by keeping PipelineRuns in `op1st-pipelines` itself, or by extending the emberstack reflector ruleset to mirror the Secret into the new namespace
- **AND** updates the registry path string in `on-tag-publish.yaml` (if the new project ships to a different registry)
- **AND** edits ADR cross-references in this source repo's `docs/design/release-pipeline-migration.md` to point at the new location
- **THEN** the build and publish pipelines SHALL function unchanged, producing the same signed image at the new registry path

#### Scenario: Forgejo-mcp Go change does not touch image tree

- **WHEN** a PR modifies any file under `operation/`, `pkg/`, `cmd/`, or `main.go`
- **AND** does not modify any file under `image/release-tools/` or `.tekton/release-tools/`
- **THEN** the image PR build pipeline SHALL NOT trigger

### Requirement: Base image is Hummingbird

The Containerfile SHALL use `registry.access.redhat.com/hi/go:<pinned-version>-builder` as the base for both the build stage and the final stage. The pinned version SHALL be recorded in `image/release-tools/VERSIONS.md` and referenced via build ARG in the Containerfile.

The final stage MUST retain bash and dnf availability so that the consuming Tekton Tasks (which use `script:` blocks requiring `/bin/sh`) execute successfully.

Substituting a different base image (e.g. `docker.io/library/golang`, `chainguard/go`) constitutes a MAJOR version bump and requires explicit ADR amendment.

#### Scenario: Image carries a shell

- **WHEN** the image is run via `podman run --rm <image-ref> /bin/sh -c 'echo ok'`
- **THEN** the command SHALL exit 0 with stdout `ok`

#### Scenario: Image declares its Hummingbird lineage

- **WHEN** `podman inspect <image-ref>` is invoked
- **THEN** `LABEL org.opencontainers.image.base.name` SHALL equal the exact pinned Hummingbird tag recorded in `VERSIONS.md` (e.g. `registry.access.redhat.com/hi/go:1.25-builder`), not merely contain a substring — substring matching can be faked by a malicious base swap that retains the original label string

### Requirement: Bundled tools with pinned versions

The image SHALL provide the following tools on `$PATH` for any user, with versions pinned via build ARGs sourced from `VERSIONS.md`:

| Tool | Source | Use |
|---|---|---|
| `go` | Hummingbird `hi/go:<ver>-builder` | release-time `go build` invoked by goreleaser |
| `syft` | anchore release | per-archive CycloneDX SBOMs |
| `goreleaser` | `go install` from pinned tag | release orchestration |
| `cosign` | sigstore release | sign-blob + verify |
| `node` | dnf | runtime for `npx -y @anthropic-ai/mcpb` |
| `npm` | dnf (with node) | bootstrap mcpb |
| `jq` | dnf | Codeberg API JSON parsing |
| `curl` | dnf | downloads + Codeberg API uploads |
| `ca-certificates` | dnf | TLS trust |

The Containerfile MUST include a final-stage `RUN` step that exercises each tool's `--version` (or equivalent) command. Failure of that step SHALL fail the build.

#### Scenario: All bundled tools report a version

- **WHEN** `podman run --rm <image-ref> sh -c 'go version && syft version && goreleaser --version && cosign version && jq --version && curl --version && node --version'`
- **THEN** the command SHALL exit 0 with each tool printing a non-empty version string

#### Scenario: npx can resolve @anthropic-ai/mcpb without network

- **WHEN** `podman run --rm --network=none <image-ref> sh -c 'npx -y @anthropic-ai/mcpb --version'`
- **THEN** the command SHALL exit 0 (npm cache prewarmed in build stage)

#### Scenario: @anthropic-ai/mcpb installs from a pinned lockfile, not from the live npm registry

The build stage SHALL install `@anthropic-ai/mcpb` via `npm ci --ignore-scripts` against a committed `image/release-tools/npm/package-lock.json` that pins the package and every transitive dependency by integrity hash. The tarball SHA256 SHALL also be recorded in `VERSIONS.md` so a manual integrity check can be performed without running npm.

- **WHEN** an attacker publishes a malicious new version of `@anthropic-ai/mcpb` (or any of its transitive deps) to the npm registry
- **AND** the image is rebuilt from the same git revision
- **THEN** the build SHALL produce an image bit-identical to the previous build, because `npm ci` resolves only what the lockfile records
- **AND** uplift to the new package version SHALL require an explicit commit that updates the lockfile (Renovate-managed, human-reviewed)

### Requirement: Image is cosign-signed at publish time

The tag-publish pipeline SHALL sign the published image manifest with cosign using the `cosign-signing-key` Secret in the `op1st-pipelines` namespace. The signature SHALL be attached to the image registry such that `cosign verify --key <public-key-url> <image-ref>` succeeds for any consumer.

The public key URL referenced in the README SHALL be identical to the URL referenced for forgejo-mcp release artifact verification (single source of truth).

**Signing posture diverges from the forgejo-mcp release pipeline.** The release pipeline mounts `cosign-signing-key` with `optional: true` (fail-open: unsigned release still ships if the Secret is absent). The image-publish pipeline mounts the same Secret with `optional: false` (fail-closed: missing key → Failed PipelineRun → image not advertised). Rationale: an unsigned release-tools image is a supply-chain regression because downstream Tasks execute its contents with signing-key access; an unsigned `checksums.txt` for a forgejo-mcp release is a UX regression only. The asymmetry is intentional and recorded in the ADR addendum.

#### Scenario: Published image verifies against the documented key

- **WHEN** the tag-publish pipeline completes for `release-tools/v1.0.0`
- **AND** a consumer runs `cosign verify --key <op1st-pub-url> codeberg.org/operate-first/release-tools:v1.0.0`
- **THEN** the command SHALL exit 0 with `Verification for ... successful`

#### Scenario: Image without signature does not pass cutover

- **WHEN** a publish run completes but cosign-sign Step fails
- **THEN** the PipelineRun status SHALL be Failed
- **AND** the image SHALL NOT be advertised to consumers via tag promotion

#### Scenario: Sign-fail does not leave an unsigned tag in the registry (TOCTOU)

The publish pipeline SHALL push to the registry by digest only (no human-readable tag) BEFORE the cosign-sign Step. The human-readable `vMAJOR.MINOR.PATCH` tag SHALL be promoted (via `crane tag` or equivalent) ONLY after cosign-sign succeeds. Registry tag mutability SHOULD be disabled.

- **WHEN** the publish pipeline pushes a manifest by digest and the subsequent cosign-sign Step fails
- **THEN** no `vMAJOR.MINOR.PATCH` tag SHALL resolve to that digest in the registry
- **AND** the previous good `vMAJOR.MINOR.PATCH` (if any) SHALL still resolve to its prior signed digest

### Requirement: SBOM attached as registry artifact

The publish pipeline SHALL emit a CycloneDX SBOM via `syft <image-ref>` and bind it to the published image manifest as a **signed** in-toto attestation using `cosign attest --predicate <sbom> --type cyclonedx --key <cosign-key>`. The attestation SHALL be signed with the same cosign key used to sign the image manifest. The pipeline SHALL NOT use `cosign attach sbom`, which is deprecated (sigstore/cosign#2755) and pushes the SBOM unsigned. Consumers SHALL be able to verify and retrieve the SBOM via `cosign verify-attestation --type cyclonedx --key <cosign.pub> <image-ref>` (or `cosign download attestation <image-ref>` for the raw DSSE envelope), or via the OCI referrers API.

#### Scenario: Signed SBOM attestation verifiable alongside the image

- **WHEN** a consumer runs `cosign verify-attestation --type cyclonedx --key cosign-images.pub codeberg.org/operate-first/release-tools:v1.0.0`
- **THEN** the command SHALL succeed, confirming the attestation signature against the public key
- **AND** the verified attestation payload SHALL carry a CycloneDX 1.x JSON document as its predicate

#### Scenario: Deprecated unsigned attach path is not used

- **WHEN** the publish pipeline binds the SBOM to the published image
- **THEN** it SHALL use `cosign attest` (signed)
- **AND** it SHALL NOT use `cosign attach sbom` (deprecated, unsigned)

### Requirement: PR build pipeline CEL-gated to release-tools paths

The PaC PipelineRun at `.tekton/release-tools/on-pull-request-build.yaml` SHALL include an `on-cel-expression` annotation that fires when ANY changed file matches `^(image/release-tools/|\.tekton/release-tools/).*`. The expression SHALL use `files.any.exists`, NOT `files.all.exists` — the latter requires every changed file to match, so a hybrid PR (image change + unrelated typo fix) would silently fail to fire the build.

Isolation (image-tree changes MUST NOT be mixed with Go code changes in the same PR) is enforced by code review, not by the CEL gate. The CEL stays permissive about firing the build.

#### Scenario: PR touching forgejo-mcp Go code does not fire image build

- **WHEN** a PR modifies `operation/issues/create.go`
- **AND** does not modify any file under `image/release-tools/` or `.tekton/release-tools/`
- **THEN** no `on-pull-request-build` PipelineRun SHALL appear in `op1st-pipelines` for that PR — observed via `tkn pr list -n op1st-pipelines` filtered by repository and PR commit SHA, expected to return zero matching runs

#### Scenario: PR touching the Containerfile fires the image build

- **WHEN** a PR modifies `image/release-tools/Containerfile`
- **THEN** the `on-pull-request-build` PipelineRun SHALL execute for that PR

### Requirement: Tag-publish pipeline CEL-gated to release-tools tag prefix

The PaC PipelineRun at `.tekton/release-tools/on-tag-publish.yaml` SHALL trigger only on push of tags matching `refs/tags/release-tools/v*`. A regular forgejo-mcp release tag (`v2.X.Y`) MUST NOT cause this pipeline to fire.

#### Scenario: forgejo-mcp release tag does not fire image publish

- **WHEN** a tag `v2.24.3` is pushed to main
- **THEN** the `on-tag-publish` PipelineRun for release-tools SHALL NOT execute

#### Scenario: release-tools tag fires the publish pipeline

- **WHEN** a tag `release-tools/v1.0.0` is pushed
- **THEN** the `on-tag-publish` PipelineRun SHALL execute and produce a signed image at the agreed registry path

### Requirement: Forgejo-mcp release pipeline unchanged by this capability

This change SHALL NOT modify `.tekton/tasks/goreleaser-release.yaml`, `.tekton/tasks/cosign-sign-release.yaml`, `.tekton/tasks/mcpb-pack.yaml`, or `.tekton/on-tag-push-release.yaml`. Switching those Tasks to consume the published image is a separate change (`release-pipeline-use-release-tools-image`) and is out of scope here.

#### Scenario: Consuming Tasks remain untouched

- **WHEN** the PR introducing this capability is reviewed
- **THEN** `git diff` SHALL show no modifications under `.tekton/tasks/` or `.tekton/on-tag-push-release.yaml`


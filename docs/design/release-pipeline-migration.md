# ADR: Migrate Release Workflow to op1st Tekton Pipeline

- Status: Accepted (hard cutover complete)
- Date: 2026-05-25 (initial) / 2026-05-26 (hard cutover)
- Tracking: [forgejo-mcp-d4b](#) (initial migration, closed) → [forgejo-mcp-n85](#) (soft-disable Forgejo trigger, closed) → [forgejo-mcp-gdz](#) (hard cutover, closed) → [forgejo-mcp-td8](#) (canonical CI gate decision, closed)

## Status history

| Date       | Status                  | Change                                                                                       |
|------------|-------------------------|----------------------------------------------------------------------------------------------|
| 2026-05-25 | Proposed                | PR #150 merged: Tekton release pipeline + Tasks + ADR + runbook + README verify chapter      |
| 2026-05-25 | Accepted (soft-cutover) | `.forgejo/workflows/release.yml` trigger changed from `push: tags: v*` to `workflow_dispatch` only (bead forgejo-mcp-n85). Tekton is now the sole auto-firing release path; Forgejo workflow kept as manual fallback. |
| 2026-05-26 | Accepted (hard cutover) | `.forgejo/workflows/release.yml` and `ci.yml` deleted (commit 1777cca). Tekton is now the sole release and CI path. See addendum below. |

## Context

The forgejo-mcp release is driven by `.forgejo/workflows/release.yml`, a
Forgejo Actions workflow that runs on Codeberg's hosted runner whenever a
`v*` tag is pushed. It performs six jobs in a single linear job:

1. Checkout with full history (for changelog).
2. Install Go via `actions/setup-go`.
3. Install `syft` (CycloneDX SBOM generator) and `cosign` (artifact signer).
4. Run `goreleaser release` against the tag, producing platform archives,
   per-archive SBOMs, and a `checksums.txt`.
5. Smoke-test the freshly-built `linux/amd64` binary (`forgejo-mcp version`
   must contain the tag).
6. Cosign-sign `checksums.txt` (skipped if `COSIGN_PRIVATE_KEY` unset) and
   upload the signature to the Codeberg release as an asset.
7. Build a `.mcpb` (Claude Desktop Extension) per `(goos, goarch)` pair
   (linux/darwin × amd64/arm64), version-stamp the bundled `manifest.json`,
   and upload each to the Codeberg release.

Parallel to this, the project already uses op1st Tekton (OpenShift
Pipelines + Pipelines-as-Code) for PR and push-to-main CI, registered in
namespace `op1st-pipelines` via the `Repository` CR
`.tekton/repository.yaml`. The open bead **forgejo-mcp-td8** captures the
follow-up decision of which CI gate (Forgejo Actions vs Tekton) becomes
canonical for PRs; release follows the same direction once that lands.

## Decision

Add a Tekton release `PipelineRun` (`.tekton/on-tag-push-release.yaml`)
that mirrors the seven Forgejo Actions steps as three reusable Tasks under
`.tekton/tasks/`:

| Task                                | Purpose                                          |
|-------------------------------------|--------------------------------------------------|
| `goreleaser-release.yaml`           | install syft + goreleaser, run release, smoke-test |
| `cosign-sign-release.yaml`          | sign-blob `checksums.txt`, upload `.sig` to release |
| `mcpb-pack.yaml`                    | pack `.mcpb` per platform, upload to release        |

PipelinesAsCode trigger is a tag push:

```yaml
pipelinesascode.tekton.dev/on-event: "[push]"
pipelinesascode.tekton.dev/on-target-branch: "[refs/tags/v*]"
```

The existing `.forgejo/workflows/release.yml` stays untouched. The two
pipelines run in parallel for at least two releases (~2–4 weeks), after
which a follow-up decision (see "Cutover criteria" below) selects the
canonical one.

## Why not …

- **Hard cutover in this PR.** First-time pipeline runs surface secret
  wiring bugs, image incompatibilities, and PaC parameter quirks. Losing
  a release to those is expensive — the package is consumed by Claude
  Desktop users who pin to specific `.mcpb` builds. Parallel-run buys
  cheap insurance: the Forgejo workflow still ships the release even if
  Tekton fails.

- **Cross-repo `workflow_call` from Forgejo Actions into Tekton.**
  Considered but rejected: doubles the moving parts (Forgejo orchestrator
  + Tekton executor), depends on Forgejo Actions remaining the entry
  point indefinitely, and the C5 spike (`forgejo-mcp-33k`) for cross-repo
  workflow_call on Codeberg's Forgejo v11.x is unverified.

- **Tekton Chains for artifact signing instead of explicit `cosign
  sign-blob`.** Tekton Chains is deployed (see
  `manifests/applications/op1st-pipelines/tekton-chains.yaml` in
  `op1st-emea-b4mad`) and signs `TaskRun` provenance automatically. But
  Chains signs the *run record*, not the GoReleaser artifact files
  uploaded to the Codeberg release. Users verifying a downloaded archive
  expect a `.sig` file matching the existing Forgejo workflow's contract;
  preserving that contract requires explicit `sign-blob`. Chains-emitted
  provenance is a complementary signal we can publish later — not a
  replacement.

- **Single mega-image with all tooling.** Slower image rebuilds, no
  cache reuse, and forces the maintainer to own a private registry path.
  Per-Task images (`golang:1.25`, `cosign/cosign:v2.4.1`,
  `node:22-bookworm`) are upstream-maintained and pinned.

## Secrets

Required in the `op1st-pipelines` namespace. Both already exist
(provisioned upstream — see "Source of truth" below):

| Secret                | Type    | Keys                                       | Used by                                             |
|-----------------------|---------|--------------------------------------------|-----------------------------------------------------|
| `op1st-release-token` | Opaque  | `token`, `username`                        | GoReleaser publish + Codeberg release asset uploads |
| `cosign-signing-key`  | Opaque  | `cosign.key`, `cosign.password`, `cosign.pub` | `sign-blob` of `checksums.txt`                   |

### Source of truth

Neither Secret is created directly in `op1st-pipelines`:

- `op1st-release-token` is auto-mirrored from `op1st-gitops/op1st-release-token`
  via the [emberstack reflector](https://github.com/emberstack/kubernetes-reflector).
  Rotation happens once in `op1st-gitops`; the reflector propagates to every
  consuming namespace within ~seconds.
- `cosign-signing-key` is provisioned alongside Tekton Chains tooling
  (matches the `signing-secret` shape used by Chains, with `cosign.key`,
  `cosign.password`, `cosign.pub` keys).

The pipeline references `cosign-signing-key` with `optional: true` — if
absent, the signing step warns and exits 0, matching
`.forgejo/workflows/release.yml`'s fail-open behavior.

### Operator-confirmable preconditions

These cannot be validated from inside the cluster (token scopes are not
visible from `oc get secret`). Operator must confirm once on Codeberg UI:

- The PAT stored in `op1st-release-token.token` carries
  `write:repository` on `goern/forgejo-mcp` (covers release publish +
  asset upload). Read-only PaC tokens like `codeberg-runner-openshift-pac`
  do **not** suffice.
- The PAT identity (e.g. `b4mad-release-bot`) is whoever should appear as
  release author in the Codeberg UI.
- `cosign-signing-key.cosign.pub` in `op1st-pipelines` matches the
  normative public key at
  `op1st-emea-b4mad/manifests/applications/op1st-pipelines-tokens/cosign-signing-key.pub`.
  Mismatch means `cosign verify-blob` failures for every downstream user
  following the README "Verifying Releases" chapter.

## Trigger semantics

PipelinesAsCode resolves the matched ref into `{{ target_branch }}`. For
a push of `refs/tags/v2.24.0` the value passed to the PipelineRun is the
full ref. The `resolve-tag` inline Task strips the `refs/tags/` prefix to
produce a bare tag name (`v2.24.0`) that downstream Tasks use to:

- locate the just-published release via the Codeberg API
  (`GET /repos/{owner}/{repo}/releases/tags/{tag}`), and
- derive `VERSION="${TAG#v}"` for filename templating consistent with
  `.goreleaser.yml`'s `name_template`.

## Cutover criteria

Promote Tekton to canonical (and remove `.forgejo/workflows/release.yml`)
when **all** of the following hold across two consecutive releases:

1. Tekton release pipeline succeeds end-to-end without manual
   intervention.
2. All eight expected assets (4× archive, 4× `.mcpb`, `checksums.txt`,
   optional `checksums.txt.sig`, SBOMs) are present on the Codeberg
   release and match the Forgejo-produced equivalents byte-for-byte
   (where deterministic) or in structure (SBOMs).
3. Wall-clock pipeline duration is within 1.5× of the Forgejo workflow,
   measured from tag push to release-complete.
4. Cosign signature verification succeeds against the published
   `cosign.pub` for both pipelines' outputs.

Failure modes that defer cutover: PaC tag-push trigger does not fire on
Codeberg's Forgejo v11.x (then we keep `.forgejo/workflows/release.yml`
and revisit when Forgejo lands the relevant fixes); image pulls blocked
from op1st-pipelines (then we mirror to an internal registry first).

## Rollback

Delete `.tekton/on-tag-push-release.yaml`. The three Task files are
inert without a PipelineRun referencing them; leaving them in tree
costs nothing. The Forgejo workflow remains the authoritative release
path until the file is removed in a follow-up commit.

## Addendum: Hard cutover — 2026-05-26

### Decision

Hard cutover executed after **one** fully successful Tekton release (v2.25.1)
rather than the two specified in the original cutover criteria.

`.forgejo/workflows/release.yml` and `.forgejo/workflows/ci.yml` deleted in
commit `1777cca`. Bead `forgejo-mcp-gdz` closed.

### Why one run instead of two

The two-run bar existed to catch infrastructure surprises: secret wiring bugs,
image incompatibilities, PaC parameter quirks. By 2026-05-26 all of those were
resolved:

| Run     | Outcome | Blocker resolved by |
|---------|---------|---------------------|
| v2.24.0 | goreleaser exit 127 | release-tools image (forgejo-mcp-1b4) |
| v2.24.1 | goreleaser dirty-tree | GOCACHE/GOMODCACHE outside workspace |
| v2.24.2 | cosign step failed (distroless, no shell) | release-tools image (forgejo-mcp-1b4) |
| v2.25.0 | cosign step failed (`--output-signature` deprecated) | fix in commit `cee6465` |
| v2.25.1 | **all tasks succeeded**, `.sig` present | — first clean end-to-end run |

After v2.25.1 the pipeline was demonstrably stable. No infrastructure surprises
remained. Running a second release solely to satisfy the original count added
delay without adding safety.

Additionally, the security posture improved with this cutover:

- cosign signing key split into `cosign-signing-key-artifacts` (release
  artifacts) and `cosign-signing-key-images` (release-tools image), so a
  compromised release-tools image can no longer forge artifact signatures
  (PR #164, bead forgejo-mcp-j52).

### Updated secrets table

| Secret                          | Keys                                          | Used by                                        |
|---------------------------------|-----------------------------------------------|------------------------------------------------|
| `op1st-release-token`           | `token`, `username`                           | goreleaser publish + Codeberg release asset uploads |
| `cosign-signing-key-artifacts`  | `cosign.key`, `cosign.password`, `cosign.pub` | `sign-blob` of `checksums.txt` in release pipeline |
| `cosign-signing-key-images`     | `cosign.key`, `cosign.password`, `cosign.pub` | signing the release-tools container image      |

The original `cosign-signing-key` Secret remains in `op1st-pipelines` as a
legacy artefact; it is no longer referenced by any Task.

### Rollback

The Forgejo Actions workflows are deleted. Rollback requires restoring them
from git history (`git show HEAD~1:.forgejo/workflows/release.yml`) and
re-adding the tag trigger. The Tekton pipeline remains in tree and can be
disabled by removing `.tekton/on-tag-push-release.yaml`.

## Out of scope

- Migrating CI (PR + push-to-main) Forgejo Actions to Tekton — tracked
  in `forgejo-mcp-td8`. This ADR covers release only.
- Publishing Tekton Chains in-toto attestations to the release —
  separate ADR if/when we decide to add that signal.
- Konflux integration for container builds — currently no container
  release exists; revisit when one does.

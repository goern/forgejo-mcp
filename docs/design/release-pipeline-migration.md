# ADR: Migrate Release Workflow to op1st Tekton Pipeline

- Status: Proposed (parallel-run phase)
- Date: 2026-05-25
- Tracking: [forgejo-mcp-d4b](#) — depends on [forgejo-mcp-td8](#) (canonical CI gate decision)

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

Required in the `op1st-pipelines` namespace:

| Secret               | Keys                                       | Purpose                                            |
|----------------------|--------------------------------------------|----------------------------------------------------|
| `forgejo-release-token` | `token`                                  | Codeberg PAT with `write:repository` for asset upload + GoReleaser publish |
| `cosign-release-key` | `cosign.key`, `cosign.password`, `cosign.pub` | Cosign keypair for `sign-blob` of `checksums.txt` |

Provisioning commands (run by an op1st-pipelines admin):

```bash
# Forgejo PAT — reuses the token already configured for the Codeberg-runner
# integration if it carries write:repository.
oc -n op1st-pipelines create secret generic forgejo-release-token \
  --from-literal=token='<codeberg-pat>'

# Cosign keypair — generate with `cosign generate-key-pair` if not present;
# commit cosign.pub to the repo (already done for the Forgejo workflow).
oc -n op1st-pipelines create secret generic cosign-release-key \
  --from-file=cosign.key=cosign.key \
  --from-literal=cosign.password='<password>' \
  --from-file=cosign.pub=cosign.pub
```

The `cosign-release-key` Secret is referenced with `optional: true` in
the Task — if it is absent the pipeline still completes, just unsigned,
matching the Forgejo workflow's fail-open behavior.

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

## Out of scope

- Migrating CI (PR + push-to-main) Forgejo Actions to Tekton — tracked
  in `forgejo-mcp-td8`. This ADR covers release only.
- Publishing Tekton Chains in-toto attestations to the release —
  separate ADR if/when we decide to add that signal.
- Konflux integration for container builds — currently no container
  release exists; revisit when one does.

# Tekton Release Pipeline — Runbook

Operator-facing companion to
[docs/design/release-pipeline-migration.md](../design/release-pipeline-migration.md).
Covers one-time provisioning, cutting a release, observing a run, and
common failure modes.

## One-time provisioning

The pipeline runs in the `op1st-pipelines` OpenShift namespace, which
already hosts the PR + push CI pipelines via the
`codeberg-org-goern-forgejo-mcp` `Repository` CR. Two additional Secrets
are required before the first release tag:

```bash
oc -n op1st-pipelines create secret generic forgejo-release-token \
  --from-literal=token='<codeberg-pat-with-write-repository>'

oc -n op1st-pipelines create secret generic cosign-release-key \
  --from-file=cosign.key=cosign.key \
  --from-literal=cosign.password='<cosign-password>' \
  --from-file=cosign.pub=cosign.pub
```

If `cosign-release-key` is omitted, the pipeline still completes — the
signing step warns and skips, matching `.forgejo/workflows/release.yml`.

The PaC `Repository` CR does not need changes: it scopes by repo URL,
not by PipelineRun, so the new release pipeline is picked up
automatically.

## Cutting a release

Releases are cut by tagging `main` with a semver `v*` tag. The project
uses `b4mad/semantic-release` for this — see the `release-cut-command`
memory in `bd memories release` for the exact container invocation.

Once the tag is pushed, **both** pipelines run in parallel:

- Forgejo Actions: `.forgejo/workflows/release.yml` → Codeberg's hosted
  runner.
- Tekton (this pipeline): `.tekton/on-tag-push-release.yaml` →
  `op1st-pipelines` namespace.

Either one alone is sufficient to ship the release. Asset uploads to the
Codeberg release are idempotent at the API level by `name`; if both
pipelines try to upload the same asset, the second 409s but does not
fail the run.

## Observing a run

```bash
# List recent PipelineRuns for forgejo-mcp.
oc -n op1st-pipelines get pipelineruns \
  -l pipelinesascode.tekton.dev/repository=codeberg-org-goern-forgejo-mcp \
  --sort-by=.metadata.creationTimestamp

# Stream logs for the most recent run.
tkn -n op1st-pipelines pr logs --last -f

# Inspect a specific Task within a run.
tkn -n op1st-pipelines pr describe <pipelinerun-name>
```

The Tekton Dashboard (op1st-tekton-hub) shows the same data in a
browser. PaC also reports check-run status back to the Codeberg commit,
visible on the tag's commit page.

## Pipeline shape

```
fetch-source (git-clone)
   │
resolve-tag (strips refs/tags/ prefix)
   │
goreleaser-release (syft + goreleaser + smoke-test)
   │
   ├── cosign-sign-release (sign checksums + upload)
   └── mcpb-pack (build .mcpb per platform + upload)
```

`cosign-sign-release` and `mcpb-pack` run in parallel because both only
read from `dist/` produced by `goreleaser-release`.

## Common failure modes

### PaC trigger did not fire on tag push

Symptom: tag pushed, GitHub-style check appears on the commit, but no
`PipelineRun` shows in `op1st-pipelines`.

Cause: PaC `on-target-branch` semantics for tag refs varies by PaC
version. On Codeberg's Forgejo, the push event includes
`ref: refs/tags/vX.Y.Z` — PaC matches that against the
`[refs/tags/v*]` glob.

Check: `oc -n op1st-pipelines logs deploy/pipelines-as-code-controller`
for the relevant timestamp. If the controller logs show the push event
but no match, try simplifying the glob to `[v*]` and re-tag.

### `GORELEASER_FORCE_TOKEN=gitea` ignored, GoReleaser tries GitHub

Symptom: `goreleaser release` errors with `GITHUB_TOKEN` missing.

Cause: GoReleaser autodetects the provider from environment. On a
hosted Forgejo runner, `GITEA_ACTIONS=true` is set automatically; in
Tekton it is not, so we set `GORELEASER_FORCE_TOKEN=gitea` explicitly.

Check that the `release` step still passes that env var, and that
`.goreleaser.yml` has `gitea_urls` configured.

### Cosign step skipped despite Secret being provisioned

Symptom: warning `COSIGN_PRIVATE_KEY not set; skipping signature step.`

Cause: the Secret key name does not match `cosign.key`/`cosign.password`,
or the Secret is in a different namespace.

Verify:

```bash
oc -n op1st-pipelines get secret cosign-release-key -o json \
  | jq '.data | keys'
```

Expected: `["cosign.key", "cosign.password", "cosign.pub"]`.

### `.mcpb` upload fails with 422 Unprocessable Entity

Symptom: `curl` in `mcpb-pack` returns HTTP 422 from
`/releases/{id}/assets`.

Cause: usually a duplicate asset name from a prior pipeline run on the
same tag. The Forgejo Actions release uploaded it first.

Fix during parallel-run phase: tolerate the duplicate (current behavior
is to fail loud). After cutover this disappears.

If urgent: delete the duplicate asset via the Codeberg UI or
`DELETE /repos/{owner}/{repo}/releases/assets/{id}`, then re-run the
PipelineRun via `tkn pr restart <name>`.

## Promoting Tekton to canonical

Triggered by satisfying the cutover criteria in the ADR. Removal of
`.forgejo/workflows/release.yml` and any related secrets on the Forgejo
side happens in a single PR referencing this runbook and the ADR.

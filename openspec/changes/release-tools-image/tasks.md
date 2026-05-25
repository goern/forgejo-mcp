## 1. Source tree and pinning

- [ ] 1.1 Create `image/release-tools/` with `Containerfile`, `README.md`, `VERSIONS.md`, `renovate.json`, `.dockerignore`
- [ ] 1.2 `VERSIONS.md` records pinned versions for: Go (matches `hi/go` tag), syft, goreleaser, cosign, Node, @anthropic-ai/mcpb. Single source of truth referenced by Containerfile ARGs.
- [ ] 1.3 `renovate.json` configured to bump each pinned version with appropriate cadence (Go = manual review, others = auto-PR)
- [ ] 1.4 `README.md` documents: purpose, image:tag scheme, how to pull, how to verify cosign signature, how to bump versions, lift-out instructions for moving to a separate project

## 2. Containerfile

- [ ] 2.1 Multi-stage build. Stage `tools-builder` based on `registry.access.redhat.com/hi/go:<ver>-builder`. Compile/fetch syft, goreleaser, cosign. Prewarm `npm install -g @anthropic-ai/mcpb` into a cache dir.
- [ ] 2.2 Final stage on `registry.access.redhat.com/hi/go:<ver>-builder`. `dnf install -y nodejs npm jq curl ca-certificates && dnf clean all`. `COPY --from=tools-builder` for binaries + npm cache.
- [ ] 2.3 Final-stage `RUN` smoke checks: `go version && syft version && goreleaser --version && cosign version && jq --version && curl --version && node --version && npx -y @anthropic-ai/mcpb --version`. Fails build if any tool missing.
- [ ] 2.4 `LABEL org.opencontainers.image.*` set: source, version, vendor=operate-first, licenses, description.
- [ ] 2.5 `.dockerignore` excludes everything outside `image/release-tools/` from the build context (host repo bind-mount is whole repo; ignore narrows it).

## 3. PR build pipeline

- [ ] 3.1 `.tekton/release-tools/on-pull-request-build.yaml` — PipelineRun with PaC annotations:
  - `on-event: "[pull_request]"`
  - `on-target-branch: "[main]"`
  - `on-cel-expression: files.all.exists(p, p.matches("^(image/release-tools/|\\.tekton/release-tools/).*"))`
- [ ] 3.2 Pipeline tasks: `fetch-source` (git-clone) → `build-image` (buildah, no push) → `verify-tools` (runs the image, asserts versions) → `scan-sbom` (syft against the built image, attach as TaskRun artifact)
- [ ] 3.3 Verify PaC controller in op1st-pipelines is recent enough for `files.all` CEL (≥0.22.0). If not, escalate to op1st-emea-b4mad maintainers BEFORE merging.

## 4. Tag-publish pipeline

- [ ] 4.1 `.tekton/release-tools/on-tag-publish.yaml` — PipelineRun with PaC annotations:
  - `on-event: "[push]"`
  - `on-target-branch: "[refs/tags/release-tools/v*]"`
  - `on-cel-expression: target_branch.matches("^refs/tags/release-tools/v.*")` (belt-and-suspenders for older PaC)
- [ ] 4.2 Pipeline tasks: `fetch-source` → `resolve-tag` (strips `refs/tags/release-tools/` prefix → bare semver) → `build-and-push` (buildah push to agreed registry) → `cosign-sign-image` (uses `cosign-signing-key` Secret) → `attach-sbom` (syft + cosign attach)
- [ ] 4.3 Operator coordination: confirm registry path (`quay.io/operate-first/release-tools` working assumption) + write credentials with op1st-pipelines admin. Document required Secret + push permissions in pipeline file header comment.

## 5. Local validation harness

- [ ] 5.1 `image/release-tools/build.sh` — single-shot local build via podman, no push. Mirrors the PR pipeline so a contributor can reproduce locally.
- [ ] 5.2 `image/release-tools/verify.sh` — runs the built image and asserts the same tool versions the pipeline checks. Used both locally and in pipeline.
- [ ] 5.3 Document both in `image/release-tools/README.md`.

## 6. Documentation

- [ ] 6.1 ADR `docs/design/release-pipeline-migration.md`: add subsection "release-tools image" noting the image as the dependency for hard cutover (forgejo-mcp-gdz).
- [ ] 6.2 `image/release-tools/README.md`: full operator guide — pull, verify, run locally, bump versions, lift to separate project.
- [ ] 6.3 Bead `forgejo-mcp-gdz` notes updated to point at this change's published image as the precondition.

## 7. Out of scope (deferred to follow-up change `release-pipeline-use-release-tools-image`)

- [ ] 7.1 Rewrite `.tekton/tasks/goreleaser-release.yaml` to reference the published image; drop runtime installs.
- [ ] 7.2 Same for `cosign-sign-release.yaml` and `mcpb-pack.yaml`.
- [ ] 7.3 Drop `/tmp` GOCACHE workaround once the image fixes the root cause.
- [ ] 7.4 Cut a clean `v2.24.x+1` release as the first end-to-end-clean Tekton run.

## Acceptance criteria for this change

- A PR labeled `Kind/Feature` exists with all paths under `image/release-tools/` and `.tekton/release-tools/` (no edits outside those + ADR + README).
- `image/release-tools/build.sh` runs locally to completion on the maintainer's workstation and produces a tagged image.
- A `release-tools/v1.0.0` tag pushed to main fires the publish pipeline and produces a signed, SBOM-attached image at the agreed registry.
- `cosign verify <image-ref>` against the documented public key succeeds.
- `podman run --rm <image-ref> goreleaser --version` and equivalent for all bundled tools succeed.
- The forgejo-mcp release pipeline (PR #150 lineage) is UNCHANGED by this PR; touching its files is a separate change.

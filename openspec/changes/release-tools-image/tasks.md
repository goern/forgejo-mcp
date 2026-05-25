## 1. Source tree and pinning

- [ ] 1.1 Create `image/release-tools/` with `Containerfile`, `README.md`, `VERSIONS.md`, `renovate.json`, `.dockerignore`
- [ ] 1.2 `VERSIONS.md` records pinned versions for: Go (matches `hi/go` tag), syft, goreleaser, cosign, Node, @anthropic-ai/mcpb. Single source of truth referenced by Containerfile ARGs.
- [ ] 1.3 `renovate.json` configured to bump each pinned version with appropriate cadence (Go = manual review, others = auto-PR)
- [ ] 1.4 `README.md` documents: purpose, image:tag scheme, how to pull, how to verify cosign signature, how to bump versions, lift-out instructions for moving to a separate project

## 2. Containerfile

- [ ] 2.1 Multi-stage build. Stage `tools-builder` based on `registry.access.redhat.com/hi/go:<ver>-builder`. Compile/fetch syft, goreleaser, cosign. For `@anthropic-ai/mcpb`: commit `image/release-tools/npm/{package.json,package-lock.json}` pinning the package + transitive deps by integrity hash; install via `npm ci --ignore-scripts` (NOT `npm install -g`); record the resolved tarball SHA256 in `VERSIONS.md`.
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
- [ ] 3.3 Verify PaC controller in op1st-pipelines is recent enough for `files.any` CEL (≥0.22.0). If not, escalate to op1st-emea-b4mad maintainers BEFORE merging.

- [ ] 3.4 Confirm `refs/tags/v*` glob in existing `.tekton/on-tag-push-release.yaml` does NOT match `refs/tags/release-tools/v1.0.0` under PaC's matcher semantics. If PaC matches anyway, add a defensive CEL `target_branch.matches('^refs/tags/v[0-9]')` (or `!target_branch.startsWith('refs/tags/release-tools/')`) to the existing pipeline's annotations to prevent the forgejo-mcp release pipeline from double-firing on a `release-tools/vX.Y.Z` tag push.

## 4. Tag-publish pipeline

- [ ] 4.1 `.tekton/release-tools/on-tag-publish.yaml` — PipelineRun with PaC annotations:
  - `on-event: "[push]"`
  - `on-target-branch: "[refs/tags/release-tools/v*]"`
  - `on-cel-expression: target_branch.matches("^refs/tags/release-tools/v.*")` (belt-and-suspenders for older PaC)
- [ ] 4.2 Pipeline tasks (push-by-digest-first to avoid the TOCTOU advertisement window): `fetch-source` → `resolve-tag` (strips `refs/tags/release-tools/` prefix → bare semver) → `build` (buildah build, no push) → `push-by-digest` (buildah push to registry, manifest only, NO human-readable tag) → `cosign-sign-by-digest` (uses `cosign-signing-key` Secret with `optional: false`) → `attach-sbom` (syft + cosign attach by digest) → `promote-tag` (crane tag or buildah manifest push: only now does `vMAJOR.MINOR.PATCH` resolve to the signed digest). Registry SHOULD be configured for immutable tags.
- [ ] 4.3 Operator coordination: confirm registry path (`codeberg.org/operate-first/release-tools` working assumption) + write credentials with op1st-pipelines admin. Document required Secret + push permissions in pipeline file header comment.

## 5. Local validation harness

- [ ] 5.1 `image/release-tools/build.sh` — single-shot local build via podman, no push. Mirrors the PR pipeline so a contributor can reproduce locally.
- [ ] 5.2 `image/release-tools/verify.sh` — runs the built image and asserts the same tool versions the pipeline checks. Used both locally and in pipeline.
- [ ] 5.3 Document both in `image/release-tools/README.md`.

## 6. Documentation

- [ ] 6.1 ADR `docs/design/release-pipeline-migration.md`: amend the "Why not …" section so the "Single mega-image with all tooling" rejection is marked **Superseded by `release-tools-image` change** with a one-paragraph note that the three v2.24.x failures (step isolation, GOCACHE-in-workspace, distroless cosign) invalidated the per-Task-image assumption. Add a new "release-tools image" subsection identifying the image as a precondition for `forgejo-mcp-gdz` cutover, and document the signing-posture asymmetry (image: `optional: false`; release artifacts: `optional: true`).
- [ ] 6.2 `image/release-tools/README.md`: full operator guide — pull, verify, run locally, bump versions, lift to separate project.
- [ ] 6.3 Bead `forgejo-mcp-gdz` notes updated to point at this change's published image as the precondition.

## 7. Out of scope (deferred to follow-up change `release-pipeline-use-release-tools-image`)

- [ ] 7.1 Rewrite `.tekton/tasks/goreleaser-release.yaml` to reference the published image; drop runtime installs.
- [ ] 7.2 Same for `cosign-sign-release.yaml` and `mcpb-pack.yaml`.
- [ ] 7.3 Drop `/tmp` GOCACHE workaround once the image fixes the root cause.
- [ ] 7.4 Cut a clean `v2.24.x+1` release as the first end-to-end-clean Tekton run.

## Acceptance criteria for this change

- A PR labeled `Kind/Feature` exists with all paths under `image/release-tools/` and `.tekton/release-tools/` (no edits outside those + ADR + README).
- `bash image/release-tools/build.sh` exits 0 in a clean clone with only `podman` + `git` installed and produces a tagged image. (Reproducibility is contract; not bound to a specific maintainer workstation.)
- A `release-tools/v1.0.0` tag pushed to main fires the publish pipeline and produces a signed, SBOM-attached image at the agreed registry.
- `cosign verify <image-ref>` against the documented public key succeeds.
- `podman run --rm <image-ref> goreleaser --version` and equivalent for all bundled tools succeed.
- The forgejo-mcp release pipeline (PR #150 lineage) is UNCHANGED by this PR; touching its files is a separate change.
- The five future-work beads (Hummingbird CVE cadence, transitive pinning, key-split, SLSA provenance, CVE rescan) MUST be filed under `Status/Blocked` BEFORE this PR merges, so they cannot be silently dropped post-merge.

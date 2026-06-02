## Why

The `release-tools-image` spec (Requirement: "SBOM attached as registry artifact") mandates that consumers retrieve the image SBOM via `cosign download sbom <image-ref>`. That contract is now wrong on two counts:

1. **The command is gone.** `cosign attach sbom` / `cosign download sbom` are deprecated and removed from current cosign (sigstore/cosign#2755). The retrieval scenario the spec asserts no longer runs.
2. **The SBOM was unsigned.** `attach sbom` pushed the SBOM as a bare OCI referrer with no signature — a supply-chain gap. A consumer could not prove the SBOM came from the pipeline.

The implementation already moved on: commit `b2619fc` (forgejo-mcp-aa6) migrated `.tekton/release-tools/tasks/cosign-attach-sbom.yaml` from `cosign attach sbom` to `cosign attest --predicate <sbom> --type cyclonedx --key`, producing a **signed** in-toto CycloneDX attestation signed with the same cosign key as the image manifest. README was updated to the `cosign verify-attestation` retrieval path in the same commit. This change brings the normative spec back in line with that reality.

## What Changes

- **Modify** the `release-tools-image` requirement "SBOM attached as registry artifact":
  - The publish pipeline binds the CycloneDX SBOM to the image as a **signed** in-toto attestation via `cosign attest --type cyclonedx`, signed with the image-signing cosign key.
  - The consumer retrieval contract becomes `cosign verify-attestation --type cyclonedx --key <cosign.pub> <image-ref>` (or `cosign download attestation` for the raw DSSE envelope), or the OCI referrers API.
  - The pipeline MUST NOT use the deprecated `cosign attach sbom`.
- **No code changes.** The Tekton task and README already match the new contract (landed under aa6). This change is spec-only: it updates the requirement + scenarios so the published spec stops asserting a removed, unsigned command.

## Capabilities

### Modified Capabilities

- `release-tools-image`: the SBOM requirement now describes a signed cosign attestation and a `verify-attestation` retrieval contract, replacing the deprecated unsigned `attach sbom` / `download sbom` path. No other requirement in the capability changes.

## Impact

- **Spec**: `openspec/specs/release-tools-image/spec.md` — one requirement + its scenarios rewritten on archive/sync.
- **Code**: none in this change. Already landed: `.tekton/release-tools/tasks/cosign-attach-sbom.yaml`, `README.md` (commit `b2619fc`, forgejo-mcp-aa6).
- **Consumers**: anyone scripting `cosign download sbom` against the published image must switch to `cosign verify-attestation --type cyclonedx`. README already documents this.
- **Out of scope**: changing the signing key or registry path; multi-arch; adding SLSA provenance to the SBOM (the image manifest already carries a separate SLSA provenance attestation).

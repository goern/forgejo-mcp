## 1. Spec delta

- [x] 1.1 Add `specs/release-tools-image/spec.md` MODIFIED delta for requirement "SBOM attached as registry artifact": describe a signed cosign attestation (`cosign attest --type cyclonedx`) and a `cosign verify-attestation` retrieval contract; forbid deprecated `cosign attach sbom`.

## 2. Implementation alignment (already landed under forgejo-mcp-aa6)

- [x] 2.1 `.tekton/release-tools/tasks/cosign-attach-sbom.yaml` uses `cosign attest --predicate <sbom> --type cyclonedx --key` (commit `b2619fc`).
- [x] 2.2 `README.md` consumer block uses `cosign verify-attestation --type cyclonedx` (commit `b2619fc`).

## 3. Validate & archive

- [x] 3.1 `openspec validate signed-sbom-attestation --strict` passes.
- [ ] 3.2 After merge: archive the change and sync `openspec/specs/release-tools-image/spec.md`.

## MODIFIED Requirements

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

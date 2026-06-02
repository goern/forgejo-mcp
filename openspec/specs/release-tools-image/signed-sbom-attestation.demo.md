# Signed SBOM attestation (release-tools-image)

*2026-06-02T13:12:23Z by Showboat 0.6.1*
<!-- showboat-id: e2ed831c-a5c6-485c-be1b-f6b795173611 -->

*Captured: 2026-06-02 via Showboat 0.6.1*
<!-- captured-for: PR #193 -->
<!-- captured-at: 2026-06-02 -->
<!-- captured-against: 7b85ba4 (openspec/archive-signed-sbom-attestation) -->

Proves the OpenSpec change [`signed-sbom-attestation`](../../changes/archive/2026-06-02-signed-sbom-attestation/) — spec [`release-tools-image/spec.md`](./spec.md), requirement **"SBOM attached as registry artifact"**. Tracks issue `forgejo-mcp-3y1`; code landed under `forgejo-mcp-aa6` (commit `b2619fc`).

The pipeline command under proof lives in [`.tekton/release-tools/tasks/cosign-attach-sbom.yaml`](../../../.tekton/release-tools/tasks/cosign-attach-sbom.yaml):
`syft <ref> --output cyclonedx-json` then `cosign attest --predicate <sbom> --type=cyclonedx --key <key>`.

## Replay setup

```bash
# Tooling: cosign, syft, podman, python3 (all real, no project binary needed).
# Roundtrip blocks run against an ephemeral ttl.sh registry with a THROWAWAY
# cosign key — they prove the *mechanism* the pipeline runs, not the prod key.
# State (image ref, keypair, SBOM) persists in the workdir between blocks.
export DEMO_WORK="${DEMO_WORK:-/tmp/sbom-demo-work}"   # roundtrip blocks
export SHOWBOAT_REPO="${SHOWBOAT_REPO:-$(git rev-parse --show-toplevel)}"  # scenario-B block
```

> **Why a throwaway key + ttl.sh, not the published image?** No tagged
> release-tools image has been published *since* the `attach sbom` →
> `attest` migration (aa6) landed, so the registry carries no CycloneDX
> attestation to verify yet (see the probe at the end). The roundtrip below
> runs the **identical cosign/syft commands** the pipeline runs, so it proves
> the contract the spec asserts. First post-aa6 tag will reproduce it against
> the real key `cosign-signing-key-images.pub`.

## Scenario: Signed SBOM attestation verifiable alongside the image

> **WHEN** a consumer runs `cosign verify-attestation --type cyclonedx --key <pub> <image-ref>`
> **THEN** the command succeeds, confirming the attestation signature against the key
> **AND** the verified payload carries a CycloneDX 1.x JSON document as its predicate

### Setup: push a small image to an ephemeral registry + mint a throwaway key

```bash
set -e
podman pull -q alpine:3.20 >/dev/null
REF="ttl.sh/fjmcp-sbom-demo-${RANDOM}${RANDOM}:1h"
echo "$REF" > ref.txt
podman tag alpine:3.20 "$REF"
podman push -q "$REF" >/dev/null
COSIGN_PASSWORD="" cosign generate-key-pair >/dev/null
echo "image ref : $REF"
echo "keypair   : $(ls cosign.key cosign.pub | tr "\n" " ")"
```

```output
Private key written to cosign.key
Public key written to cosign.pub
image ref : ttl.sh/fjmcp-sbom-demo-298194484:1h
keypair   : cosign.key cosign.pub 
```

### Step 1 — generate the CycloneDX SBOM (same as the pipeline `syft` step)

```bash
REF=$(cat ref.txt)
syft -q "$REF" --output cyclonedx-json=sbom.cdx.json
python3 -c "import json;d=json.load(open(\"sbom.cdx.json\"));print(\"bomFormat   :\",d[\"bomFormat\"]);print(\"specVersion :\",d[\"specVersion\"]);print(\"components  :\",len(d.get(\"components\",[])))"
```

```output
bomFormat   : CycloneDX
specVersion : 1.6
components  : 15
```

### Step 2 — bind the SBOM as a **signed** in-toto attestation (`cosign attest --type=cyclonedx`)

```bash
REF=$(cat ref.txt)
COSIGN_PASSWORD="" cosign attest --predicate=sbom.cdx.json --type=cyclonedx --key=cosign.key --yes "$REF" 2>&1
```

```output
WARNING: Image reference ttl.sh/fjmcp-sbom-demo-298194484:1h uses a tag, not a digest, to identify the image to sign.
    This can lead you to sign a different image than the intended one. Please use a
    digest (example.com/ubuntu@sha256:abc123...) rather than tag
    (example.com/ubuntu:latest) for the input to cosign. The ability to refer to
    images by tag will be removed in a future release.

Using payload from: sbom.cdx.json
Signing artifact...
```

### Step 3 — consumer verifies the attestation (the spec scenario)

```bash
REF=$(cat ref.txt)
COSIGN_PASSWORD="" cosign verify-attestation --type cyclonedx --key cosign.pub "$REF" > att.json 2>verify.txt
cat verify.txt
echo "verify-attestation exit: $?"
```

```output

Verification for ttl.sh/fjmcp-sbom-demo-298194484:1h --
The following checks were performed on each of these signatures:
  - The cosign claims were validated
  - Existence of the claims in the transparency log was verified offline
  - The signatures were verified against the specified public key
verify-attestation exit: 0
```

### Step 4 — confirm the verified predicate IS a CycloneDX document

```bash
python3 -c "import json,base64;e=json.load(open(\"att.json\"));s=json.loads(base64.b64decode(e[\"payload\"]));print(\"predicateType :\",s[\"predicateType\"]);print(\"predicate.bomFormat :\",s[\"predicate\"][\"bomFormat\"]);print(\"predicate.specVersion :\",s[\"predicate\"][\"specVersion\"])"
```

```output
predicateType : https://cyclonedx.org/bom
predicate.bomFormat : CycloneDX
predicate.specVersion : 1.6
```

### Published-image state (current)

The image-signing key the spec names — `cosign-signing-key-images.pub` — is published in the GitOps repo (`op1st-pipelines-tokens`), distinct from the artifact-signing key. The currently published `release-tools:latest` predates both the key-split and the `attest` migration: it verifies against the legacy **artifacts** key and still carries only the **old tag-based `.sbom`** referrer, **no CycloneDX attestation**. This block documents that gap — the one piece the spec scenario cannot yet show against prod until the first post-migration tag is cut (then it reproduces against `cosign-signing-key-images.pub`).

```bash
BASE=https://codeberg.org/operate-first/op1st-emea-b4mad/raw/branch/main/manifests/applications/op1st-pipelines-tokens
curl -sSfL --retry 6 --retry-all-errors --retry-delay 2 -o images.pub  "$BASE/cosign-signing-key-images.pub"
curl -sSfL --retry 6 --retry-all-errors --retry-delay 2 -o artifacts.pub "$BASE/cosign-signing-key-artifacts.pub"
echo "== the image-signing key the spec names is published =="
printf "cosign-signing-key-images.pub: %s bytes  (distinct from artifacts key: %s)\n" \
  "$(wc -c < images.pub)" "$(cmp -s images.pub artifacts.pub && echo SAME || echo DIFFERENT)"
echo
echo "== current latest verifies with the legacy artifacts key (signed pre-split) =="
cosign verify --key artifacts.pub codeberg.org/operate-first/release-tools:latest 2>&1 | grep -iE "claims|transparency|verified against"
echo
echo "== referrers present today (cosign tree): signature + old tag-based .sbom, no cyclonedx attestation =="
cosign tree codeberg.org/operate-first/release-tools:latest 2>&1 | sed "s/sha256-[0-9a-f]\{12\}[0-9a-f]*/sha256-<digest>/g"
```

```output
== the image-signing key the spec names is published ==
cosign-signing-key-images.pub: 178 bytes  (distinct from artifacts key: DIFFERENT)

== current latest verifies with the legacy artifacts key (signed pre-split) ==
  - The cosign claims were validated
  - Existence of the claims in the transparency log was verified offline
  - The signatures were verified against the specified public key

== referrers present today (cosign tree): signature + old tag-based .sbom, no cyclonedx attestation ==
📦 Supply Chain Security Related artifacts for an image: codeberg.org/operate-first/release-tools:latest
└── 🔐 Signatures for an image tag: codeberg.org/operate-first/release-tools:sha256-<digest>.sig
   └── 🍒 sha256:6cad25d7569532c30ea9cd5eda16e44d34329ce64292ffd25ebe8dbc77d6f355
└── 📦 SBOMs for an image tag: codeberg.org/operate-first/release-tools:sha256-<digest>.sbom
   └── 🍒 sha256:ff4be5015dbb4fb6f89dc57ed5218555b84ee0fe4fc1134162e4b0ec92dfe54a
```

## Scenario: Deprecated unsigned attach path is not used

> **WHEN** the publish pipeline binds the SBOM to the published image
> **THEN** it SHALL use `cosign attest` (signed)
> **AND** it SHALL NOT use `cosign attach sbom` (deprecated, unsigned)

```bash
TASK="${SHOWBOAT_REPO:-$(git rev-parse --show-toplevel)}/.tekton/release-tools/tasks/cosign-attach-sbom.yaml"
echo "== active command in the Tekton task =="
grep -nE "cosign attest" "$TASK"
echo
echo "== any ACTIVE (non-comment) use of the deprecated path? =="
if grep -nE "^[[:space:]]*cosign attach sbom" "$TASK"; then echo "FOUND active attach sbom (FAIL)"; else echo "none — only historical references in comments:"; grep -nE "attach sbom" "$TASK" | sed "s/^/  /"; fi
```

```output
== active command in the Tekton task ==
6:    tekton.dev/displayName: "syft scan + cosign attest (signed CycloneDX SBOM) for release-tools image"
10:    manifest as a SIGNED in-toto attestation using `cosign attest --type
46:        path. cosign attest pushes the signed SBOM attestation as an OCI
113:        # `cosign attest` pushes the signed SBOM attestation as an OCI referrer,
162:        cosign attest \

== any ACTIVE (non-comment) use of the deprecated path? ==
none — only historical references in comments:
  15:    Migrated from `cosign attach sbom` (forgejo-mcp-aa6): `attach sbom` is
  160:        # attestation OCI referrer. Replaces deprecated `cosign attach sbom`,
```

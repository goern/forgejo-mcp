## Adversarial Review — 2026-05-25

Round-1-and-done debate. `devils-advocate` produced 8 critiques; supply-chain lens (`general-purpose` agent in research-lens framing) added 6. `proponent` responded with 0 outright defends, 9 concede-patches, 5 concede-future, 1 stalemate. No defends → no Round 2. Lead applied all 10 patches (incl. resolving A8 stalemate) and filed the 5 future-work items as beads.

### Lens used

Supply-chain security — picked because the proposal makes load-bearing claims about cosign signing, SBOM attachment, Hummingbird hardening, and a shared keypair that all needed adversarial pressure on the threat model.

### Patches applied (9 + 1 stalemate-resolution)

| # | Critique | File:section edited |
|---|---|---|
| A1 | Tag-isolation glob may double-fire | `tasks.md` new item 3.4 (defensive CEL probe on existing release pipeline) |
| A2 | ADR contradicts "Single mega-image" rejection | `tasks.md` item 6.1 strengthened — explicit ADR amendment, not just "subsection added" |
| A3 | Sign fail-open (artifact) vs fail-closed (image) on same Secret | `spec.md` Requirement: Image cosign-signed — added asymmetry paragraph + ADR note |
| A4 | `files.all.exists` inverts gate for mixed PRs | `design.md` D3 + `spec.md` PR-build Requirement — switched to `files.any.exists`, added rationale |
| A5 | `git mv` lift-out claim too clean | `spec.md` Scenario: Lift-out — enumerated 5 operator actions, not 1 |
| A7 | Vacuous/untestable scenarios | `spec.md` Hummingbird-lineage scenario (substring → exact-equal), Go-code-PR scenario (observation point named), `tasks.md` acceptance (reframed reproducibility) |
| A8 | Stalemate: versioning treadmill | `design.md` D4 — **resolved**: adopted image-API semver (MAJOR = base swap / removed tool / broken CLI / removed path; MINOR = added tool / Hummingbird base MINOR / Go minor bump / tool MINOR; PATCH = tool patches + security backports). Rationale: image's "interface" is bundled tools at known paths with stable CLI contracts; Go's compat guarantee makes Go-minor invisible to goreleaser CLI consumers. |
| L1 | Push-before-sign TOCTOU window | `tasks.md` item 4.2 rewritten — push-by-digest → sign → promote-tag; `spec.md` new TOCTOU scenario |
| L3 | npm prefetch ≠ vendor-lock | `tasks.md` item 2.1 + `spec.md` new lockfile scenario — `npm ci --ignore-scripts` against committed `package-lock.json` + SHA256 in `VERSIONS.md` |

### Future-work beads filed (5)

| Origin | Bead | Why deferred |
|---|---|---|
| A6, L6 | `forgejo-mcp-aun` — Hummingbird base health monitoring SLO + scheduled CVE rescan | Needs one CVE-cycle of operational data before SLO is meaningful |
| L2 | `forgejo-mcp-3h2` — Tighten pinning: vendor goreleaser source, SHA256-verify curl downloads | Separate vendoring pass; not blocking initial image cut |
| L4 | `forgejo-mcp-j52` — Key-split: release-tools image signing keypair ≠ release artifact signing keypair | op1st-pipelines secret-provisioning change, coordinate with op1st-emea-b4mad maintainers |
| L5 | `forgejo-mcp-46j` — SLSA provenance via Tekton Chains | Chains output is complementary; cosign signature is sufficient for v1.0.0 |

All four beads depend on `forgejo-mcp-1b4` (this change's parent) → automatically appear under `bd blocked` until the image ships, then transition to `bd ready` for follow-up work.

All filed under `Status/Blocked` per acceptance criteria — they cannot be silently dropped post-merge.

### Surviving critiques

None. All 14 either patched in place or filed as tracked follow-ups.

### Adversary's own culled list (not load-bearing)

- Image size budget ~800 MB — watch item, not a failure
- `tkn` CLI omission — design preference, already an Open Question
- arm64 deferral — acknowledged out of scope
- `npm` offline scenario as written — testable
- SBOM format check — fine
- Shared-keypair blast-radius (lens covered this under L4) — escalated, not duplicated

### Notes for next reviewer

The proposal took on more supply-chain debt than the "Why" advertised. The five future-work beads are not cosmetic — they are the gap between "we built a signed image" and "we built an attestable release-tools image with full supply-chain provenance." That gap is real and tracked, not silenced.

The A8 referee call (image-API semver, not bundled-tool semver) deserves a second look once the first 6 months of Renovate-driven bumps land. If MINOR bumps cause unexpected consumer breakage in practice, revisit the policy.

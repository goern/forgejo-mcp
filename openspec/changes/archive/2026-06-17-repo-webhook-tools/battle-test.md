# Battle Test — repo-webhook-tools

**Date:** 2026-06-17
**Targets:** proposal.md, design.md, specs/repo-webhook-tools/spec.md
**Team:** adversary (devils-advocate/opus), defender (proponent/opus)

## Verdict

**Patch first.**

7 critiques raised, 6 conceded as patches, 1 deferred to future work, 0 defended. No structural rework is needed — the approach is sound — but three patches are blockers before `/opsx:apply`: the invented `limit` ceiling (C2) creates two inconsistent bounding stories for the same data, the secret-masking mechanism (C4) conflates struct-field omission with map-key exclusion and leaves a real leak path, and the secret scenario (C5) is unfalsifiable as written. Apply all patches in the spec and design before implementing.

## Surviving Critiques

### C1 — URI scheme inconsistent across artifacts
**Severity:** major
**Failure mode:** `proposal.md` line 15 declares `forgejo://repo/{owner}/{repo}/hooks` (no query suffix); `design.md` Goals and `spec.md` both use `{?page,limit}`. Archive conflates two contracts for the same template.
**Hits:** `proposal.md#What Changes` vs `design.md#Goals` vs `spec.md#Hook collection resource template`
**Canonical conflict:** AGENTS.md resource table (labels row canonical form includes `{?page,limit}`)
**Falsifier:** `grep -n 'hooks' proposal.md design.md specs/repo-webhook-tools/spec.md` — strings do not match
**Disposition:** concede-patch

### C2 — Tool `limit` ceiling of 50 is invented and conflicts with resource cap of 30
**Severity:** blocker
**Failure mode:** `spec.md` mandates `list_repo_hooks` ceiling 50; resource path caps at `EmbeddedListCap=30`. Same data, two ceilings depending on access path. No ceiling-enforcement precedent exists in the codebase (`list_branch_protections` enforces none). The "50" has no rationale in `design.md`.
**Hits:** `spec.md#List repository hooks` (line 4); `design.md#D5`
**Canonical conflict:** `docs/design/output-bounding.md` sub-rule 1; `operation/resource/bound.go` `EmbeddedListCap=30`
**Falsifier:** Read `operation/branchprotection/branchprotection.go:108-142` — no ceiling clamp exists to copy
**Disposition:** concede-patch

### C3 — Truncation scenario unverifiable: sentinel reflects cap+1, not true count
**Severity:** major
**Failure mode:** Scenario asserts truncation fires "WHEN repository has more hooks than `limit`" — implies the sentinel is informative about total magnitude. The copied `limit+1` fetch pattern means `Bounded` reports "30 of 31" regardless of whether 31 or 3100 hooks exist. The scenario cannot be falsified against actual behavior.
**Hits:** `spec.md#List repository hooks` Scenario "List hooks truncation sentinel"; `design.md#D5`
**Canonical conflict:** `docs/design/output-bounding.md` sub-rule 3
**Falsifier:** Create 32 hooks, call `limit=30` — sentinel reports "30 of 31", not "30 of 32"
**Disposition:** concede-patch

### C4 — Secret masking: struct-omission ≠ map-key-deletion; claim rests on unverified SDK behavior
**Severity:** blocker
**Failure mode:** `design.md#D3` relies on "Forgejo masks the secret server-side" (unverified, SDK not vendored) and on "not projecting the `secret` field" — but `Hook.Config` is `map[string]string`. Copying the map wholesale carries `Config["secret"]` regardless of struct tags. The mitigation as written does not close the leak path.
**Hits:** `design.md#D3`; `spec.md#Create repository hook` Scenario "Secret not echoed"
**Canonical conflict:** none explicit — sole security-critical requirement
**Falsifier:** Vendor SDK; create hook with `secret=foo`; GET it; inspect `Config["secret"]`
**Disposition:** concede-patch

### C5 — "MUST NOT echo secret" scenario is unfalsifiable
**Severity:** blocker
**Failure mode:** All secret scenarios assert the `secret` *field* is absent — a test that never sets a secret value passes trivially and proves nothing. The security property (the *value* cannot appear in output) requires a value-tracing assertion.
**Hits:** `spec.md#Create repository hook` Scenario "Secret not echoed"
**Canonical conflict:** spec falsifiability quality bar
**Falsifier:** Write test: create hook with `secret="SENTINEL123"`, serialize all tool+resource responses, assert `!strings.Contains(output, "SENTINEL123")` — this test is not derivable from the current spec
**Disposition:** concede-patch

### C6 — `test_repo_hook` is server-initiated outbound traffic with no abuse scenario
**Severity:** watch
**Failure mode:** `design.md#D4` and Risk line 62 admit `test_repo_hook` causes Forgejo to fire a live HTTP POST and state "no mitigation in code needed." An agent can loop the call and cause Forgejo to hammer the registered URL with no MCP-side throttle. The spec has zero abuse-shape or confirmation scenario.
**Hits:** `design.md#D4` and Risks; `spec.md#Test repository hook`
**Canonical conflict:** none — but no other tool triggers server-initiated outbound traffic
**Falsifier:** Register hook to request-bin URL; loop `test_repo_hook`; observe unlimited deliveries
**Disposition:** concede-future (rate/confirmation guard) + concede-patch (doc: soften "no mitigation needed", add tool-description warning)

### C7 — Single-hook resource missing malformed-id scenario
**Severity:** major
**Failure mode:** `spec.md#Single hook resource template` only covers valid id and unknown id. No scenario for non-numeric id (`hook/abc`). Without it, an implementer who forwards `abc` to the SDK gets a 404 mapped as "not found" (-32003) rather than "invalid params" (-32602), silently miscategorizing a client error. Two conformant implementations can disagree on the error code.
**Hits:** `spec.md#Single hook resource template`; `design.md` Risk line 63
**Canonical conflict:** `operation/resource/errors.go` (-32602 vs -32003); `ParseLabel` precedent in `parse.go`
**Falsifier:** Read `forgejo://repo/o/r/hook/abc` — spec gives no expected error code
**Disposition:** concede-patch

## Conceded Patches

All patches are spec/design text edits — no features added.

1. **proposal.md:15** — append `{?page,limit}` to the collection URI: `forgejo://repo/{owner}/{repo}/hooks{?page,limit}`. Also confirm line 16 uses singular `hook/{id}` for the single-entity template.

2. **spec.md#List repository hooks** — drop "ceiling 50". New text: "`limit` (default 30)". **design.md#D5** — add one sentence: "The tool is the unbounded enumeration path (mirrors `list_branch_protections`); only the resource caps at `EmbeddedListCap`. The resource description names the tool as the escape hatch for >30 items."

3. **spec.md#List repository hooks Scenario "truncation sentinel"** — rewrite: "WHEN the repository has more hooks than the requested `limit` THEN the response includes `truncated: true` and a `list_tool: "list_repo_hooks"` sentinel. The sentinel signals that more results exist but does NOT report the total repository-wide count." **design.md#D5** — add: "Sentinel total reflects the fetched window (cap+1 probe), not the repository-wide count."

4. **design.md#D3** — replace "trust Forgejo's response" with a concrete allowlist rule: "The MCP payload struct MUST enumerate an explicit allowlist of config keys (`url`, `content_type`, `http_method`, `branch_filter`) copied individually from `Hook.Config`. It MUST NOT embed the raw `Config map[string]string`. Secret masking by Forgejo is defense-in-depth, not the primary control."

5. **spec.md#Create repository hook Scenario "Secret not echoed"** — rewrite: "WHEN a hook is created with `secret="SENTINEL"` and then retrieved via every tool and both resource templates THEN the substring `SENTINEL` does NOT appear anywhere in any serialized response (including inside config maps, error messages, or markdown rendering)."

6. **design.md#D4 and Risks line 62** — soften "no mitigation in code needed" to: "This operates against an already-registered URL; loop-abuse mitigation is deferred (see Future Work). The tool description MUST warn that each call triggers a live delivery from the Forgejo server." **spec.md#Test repository hook** — add to requirement text: "The tool description SHALL warn callers that each invocation triggers a live HTTP delivery from the Forgejo server."

7. **spec.md#Single hook resource template** — add Scenario after line 111: "**Scenario: Read single hook resource with malformed id** — WHEN a client reads `forgejo://repo/{owner}/{repo}/hook/abc` THEN the resource returns an invalid-params error (-32602) from the URI parser, NOT a not-found error (-32003)."

## Future Work

- File follow-up issue: "rate/confirmation guard for server-initiated webhook deliveries (`test_repo_hook`)" — covers loop-behavior throttle and any per-call confirmation. Cross-cutting concern; not a blocker for this change.
- Verify SDK `Hook.Config` masking behavior once the Go module is vendored (add a comment in the implementation PR confirming the empirical check was done).

## Lead Recommendation

Apply patches C1–C7 to `proposal.md`, `design.md`, and `spec.md` before running `/opsx:apply`. The three blockers are C2 (kill ceiling-50, pick one bounding story), C4 (config-key allowlist instead of raw map), and C5 (value-tracing secret test). Patches are all text edits in the planning artifacts — no implementation work needed first. Once patched, the change is implementable as-is; no rework of the overall approach is needed.

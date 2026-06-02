## Context

Forgejo exposes branch protection at `/repos/{owner}/{repo}/branch_protections`, and the vendored `forgejo-sdk/forgejo/v3` already binds it: `Client.ListBranchProtections`, `GetBranchProtection`, `CreateBranchProtection`, `EditBranchProtection`, `DeleteBranchProtection`, with `BranchProtection`, `CreateBranchProtectionOption`, `EditBranchProtectionOption`, and `ListBranchProtectionsOptions{ListOptions}`. No new dependency is needed. The work is purely a new MCP surface following the project's existing tool/resource conventions:

- CRUD tools cluster as a self-contained package (precedent: `operation/org/`).
- `forgejo://` resource-templates parse via `operation/resource` helpers (`ParseXxx`, `MapForgejoError`) and bound embedded lists via `resource.Bounded`/`EmbeddedListCap` (precedent: `operation/repo/resources_status.go`).
- Every data-proportional output obeys `docs/design/output-bounding.md`.

The motivating incident (forgejo-mcp-f6h): a repo with no protection let Renovate automerge before the required commit status was green. The load-bearing fields are therefore `enable_status_check` + `status_check_contexts` + `required_approvals`.

## Goals / Non-Goals

**Goals:**
- Read protection state (list + get tools, collection + single resources).
- Create/enforce protection with correct `status_check_contexts` round-tripping.
- Edit with true PATCH semantics: only fields the caller passes change.
- List tool + collection resource are caller-bounded and resumable.

**Non-Goals:**
- Exposing all ~22 `BranchProtection` fields as tool params — a focused, documented subset.
- Event-driven enforcement, auto-remediation, tag/ruleset protection.
- A delta against `mcp-resources-core` (not a live spec yet; this change is self-contained).

## Decisions

**D1 — Self-contained `branch-protection` capability, no `mcp-resources-core` delta.**
`mcp-resources-core` is still owned by the unarchived `mcp-resource-templates` change, so a delta against it would create a cross-change archive-ordering hazard (per the `collection-resource-precedent` note). Instead this capability states its own bounding + URI requirements, citing `output-bounding.md` as the invariant. Alternative (delta on core) rejected: couples this P0 to an unrelated unarchived change.

**D2 — New package `operation/branchprotection/`.**
Five tools + two resources are a cohesive CRUD cluster; a dedicated package mirrors `operation/org/` and keeps `operation/repo/` from growing a second entity. Alternative (fold into `operation/repo/`) rejected: repo package would mix repo-overview and protection concerns. URI parsing still centralizes in `operation/resource/parse.go` (`ParseBranchProtections` for the collection, `ParseBranchProtection` for one rule).

**D3 — Focused, documented param surface.**
`create_branch_protection`: `owner`, `repo`, `branch_name`, `rule_name` (optional; Forgejo defaults it to `branch_name`), `enable_push`, `enable_status_check`, `status_check_contexts` (array), `required_approvals`, `block_on_outdated_branch`, `require_signed_commits`, `dismiss_stale_approvals`. These cover the motivating bug and common hardening; the remaining whitelist/file-pattern fields are out of scope for v1 and can be added later without breaking the surface.

**D4 — Edit uses pointer PATCH semantics.**
`EditBranchProtectionOption` fields are `*bool`/`*int64`/`*string`. The handler sets a pointer (`pkg/ptr.To`) only when the caller supplied that argument, so omitted fields are left unchanged server-side. Slice fields (`status_check_contexts`) are sent only when provided. Alternative (always send all fields) rejected: silently resets fields the caller didn't mention.

**D5 — Rule identity in the URI.**
The single-entity URI is `forgejo://repo/{owner}/{repo}/branch_protection/{rule}` where `{rule}` is the rule name (the `{name}` path segment the API uses). The collection URI is the plural keyless `…/branch_protections` (collection-resource-precedent: plural for collections, singular `…/branch_protection/{rule}` for one entity).

**D6 — Bounding shape.**
List tool: `page` + `limit` params (the "list of entities" row of output-bounding.md), response carries the page echo for resumability. Collection resource: request `EmbeddedListCap + 1`, cap with `resource.Bounded(..., "list_branch_protections")`, emit the truncation sentinel and `list_tool` field when over cap.

**D7 — `status_check_contexts` as an array param.**
Declared with `mcp.WithArray("status_check_contexts", items=string)`; the handler coerces `[]any` → `[]string`. A dedicated test asserts the round-trip (request body contains the exact contexts; response echoes them) because this is the field the motivating bug hinged on.

## Risks / Trade-offs

- **`status_check_contexts` array coercion** (MCP sends `[]interface{}`) → explicit `[]string` coercion helper + a round-trip test.
- **`create` identity ambiguity** (`branch_name` vs `rule_name`) → require `branch_name`; treat `rule_name` as optional and let Forgejo default it; document both. → covered by a scenario.
- **Server version gate**: the SDK requires Forgejo ≥ 1.12 for these endpoints; tests construct the client with `SetForgejoVersion("7.0.0")` to pass the gate, matching existing tests.
- **Partial param surface** → callers needing whitelist/file-pattern fields can't set them in v1. Mitigation: documented Non-Goal; additive follow-up keeps the surface stable.
- **Destructive `delete`** → returns a clear confirmation payload; no force semantics beyond the API.

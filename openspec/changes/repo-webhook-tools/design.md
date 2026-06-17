## Context

The Forgejo/Gitea webhook API has been stable for years. The SDK (`codeberg.org/mvdkleijn/forgejo-sdk`) already exposes `CreateRepoHook`, `ListRepoHooks`, `GetRepoHook`, `EditRepoHook`, `DeleteRepoHook`, and `TestRepoHook` — so implementation is a thin wrapper, not net-new API work.

The project already has patterns for:
- Tool CRUD (see `operation/branchprotection/`, `operation/release/`)
- Resource templates with bounded lists (see `operation/issue/resources_label.go`)
- URI parsing via `operation/resource.Parse*` helpers
- Error mapping via `operation/resource.MapForgejoError`

Hooks live at the repository level only in this change (org-level and admin-level hooks are deferred).

## Goals / Non-Goals

**Goals:**
- Six MCP tools: `list_repo_hooks`, `get_repo_hook`, `create_repo_hook`, `edit_repo_hook`, `delete_repo_hook`, `test_repo_hook`
- Two resource templates: `forgejo://repo/{owner}/{repo}/hooks{?page,limit}` (bounded list) and `forgejo://repo/{owner}/{repo}/hook/{id}` (single)
- `secret` field never echoed — rely on Forgejo's masked server response; the MCP layer must not add it back
- Output bounding on `list_repo_hooks` and the hooks list resource (per `docs/design/output-bounding.md`)

**Non-Goals:**
- Org-level hooks (`/orgs/{org}/hooks`) — follow-up
- System/admin hooks (`/admin/hooks`) — follow-up
- Webhook delivery log / re-delivery — follow-up
- Validating that the hook URL is reachable from the Forgejo instance

## Decisions

### D1: New domain package `operation/hook/`

**Decision**: Place all handler files under `operation/hook/` (tools in `hook.go`, resources in `resources_hook.go`).

**Rationale**: Follows the one-domain-one-directory convention (`operation/branchprotection/`, `operation/release/`). Alternatives: adding to `operation/repo/` (too broad) or `operation/issue/` (wrong domain).

### D2: URI scheme — singular `hook` not `hooks` for single-entity template

**Decision**: Use `forgejo://repo/{owner}/{repo}/hook/{id}` (singular) for the single-entity resource and `forgejo://repo/{owner}/{repo}/hooks` (plural, keyless) for the collection, consistent with the project's `label` / `labels` precedent.

**Rationale**: Matches the normative `forgejo://` URI scheme established in `mcp-resources-core` (singular keyless segment for collection, singular + `/{id}` for entity).

### D3: Secret masking — explicit config-key allowlist

**Decision**: The MCP payload struct MUST define an explicit allowlist of config keys (`url`, `content_type`, `http_method`, `branch_filter`) copied individually from `Hook.Config`. It MUST NOT embed the raw `Config map[string]string`. This is the primary control. Secret masking by Forgejo server-side is defense-in-depth, not the primary control.

**Rationale**: `Hook.Config` is `map[string]string` — copying it wholesale carries `Config["secret"]` into the response regardless of Go struct tags or JSON omitempty. Struct-field omission does not remove a map key. The allowlist approach closes the leak path deterministically and is immune to future SDK changes that might surface the secret more prominently.

### D4: `test_repo_hook` — fire-and-forget

**Decision**: `test_repo_hook` calls the SDK's `TestRepoHook` and returns a simple success/error response with no body (Forgejo returns 204).

**Rationale**: The Forgejo API returns no meaningful body for test delivery. Returning `{"triggered": true}` is sufficient.

### D5: Output bounding strategy for list

**Decision**: The `list_repo_hooks` tool is the unbounded enumeration path (mirrors `list_branch_protections`: no ceiling clamp, default limit 30). The `forgejo://repo/{owner}/{repo}/hooks` resource is the bounded path, capping at `resource.EmbeddedListCap` (30) using `resource.Bounded`, with a truncation sentinel naming `list_repo_hooks` as the escape hatch for >30 items.

**Rationale**: Consistent with the two-path model established by `resources_label.go` (resource description: "Use list_repo_labels tool for the unbounded enumeration path"). Tool and resource serve different caller needs; they must not have conflicting ceilings on the same data. Sentinel total reflects the fetched window (cap+1 probe), not the repository-wide count — it signals "more exist", not how many.

## Risks / Trade-offs

- [Risk] SDK `Hook.Config` is `map[string]string` — future SDK versions might expose `secret` more prominently. → Mitigation: explicit exclusion in the payload struct, not relying on JSON tag omitempty alone.
- [Risk] `test_repo_hook` triggers a live HTTP request from the Forgejo server to the hook URL. This operates against an already-registered URL (authorized by a prior `create_repo_hook` call), so it is not arbitrary SSRF-to-anywhere. Loop-abuse mitigation (rate limiting, confirmation guard) is deferred to a follow-up. The tool description MUST warn that each call triggers a live delivery from the Forgejo server.
- [Risk] Hook IDs are `int64` but the URI path segment is a string. → Mitigation: `strconv.ParseInt` with error mapping in the URI parser.

## Open Questions

*(none — all decisions made)*

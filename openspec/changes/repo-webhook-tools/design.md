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

### D3: Secret masking — trust Forgejo's response

**Decision**: Do not project the `secret` field from the SDK struct into any payload struct. The SDK's `Hook` type includes `Config map[string]string` where `"secret"` may appear. Exclude it explicitly from the MCP-visible payload.

**Rationale**: Forgejo already masks the secret server-side (returns `"secret": ""` or omits it). We add a second layer by not mapping it at all — prevents accidental future leakage if SDK behaviour changes.

### D4: `test_repo_hook` — fire-and-forget

**Decision**: `test_repo_hook` calls the SDK's `TestRepoHook` and returns a simple success/error response with no body (Forgejo returns 204).

**Rationale**: The Forgejo API returns no meaningful body for test delivery. Returning `{"triggered": true}` is sufficient.

### D5: Output bounding strategy for list

**Decision**: Follow the exact pattern from `operation/issue/resources_label.go` — `page` and `limit` params, `resource.Bounded`, cap at `resource.EmbeddedListCap` (30), truncation sentinel naming `list_repo_hooks`.

**Rationale**: Consistent with every other bounded list in the project.

## Risks / Trade-offs

- [Risk] SDK `Hook.Config` is `map[string]string` — future SDK versions might expose `secret` more prominently. → Mitigation: explicit exclusion in the payload struct, not relying on JSON tag omitempty alone.
- [Risk] `test_repo_hook` triggers a live HTTP request from the Forgejo server to the hook URL. → Mitigation: document in tool description; no mitigation in code needed.
- [Risk] Hook IDs are `int64` but the URI path segment is a string. → Mitigation: `strconv.ParseInt` with error mapping in the URI parser.

## Open Questions

*(none — all decisions made)*

## Context

Forgejo separates label storage into two scopes:

- Repository labels — `GET /repos/{owner}/{repo}/labels` — exposed today via `list_repo_labels` (added by the in-flight `list-milestones-labels` change).
- Organization labels — `GET /orgs/{org}/labels` — usable on issues in any repo owned by the org, but not exposed by the MCP at all.

`forgejo-sdk/forgejo/v3@v3.0.0` provides bindings for the repo endpoint only. There is no `ListOrgLabels` function on the SDK client. The codebase has already established a pattern for working around SDK gaps: `pkg/forgejo/raw_http.go` exposes `DoJSON`, `DoJSONList`, `DoMultipart`, and `DoRaw` helpers that share auth, user-agent, and logging with the SDK client. The attachment tools use `DoJSONList` to call endpoints not bound by the SDK.

The reporter (Codeberg #125) explicitly asks that org labels be merged into `list_repo_labels` so the model does not need two separate calls.

## Goals / Non-Goals

**Goals**

- Expose org-level labels through the MCP for both discovery (`list_org_labels`) and combined listing (`list_repo_labels` with merge).
- Avoid SDK upgrades — use the existing raw-HTTP helper.
- Preserve current `list_repo_labels` semantics for callers that opt out of the merge.
- Make scope unambiguous: the merged response must let a caller tell whether a given label ID is repo- or org-scoped.

**Non-Goals**

- Creating, editing, or deleting org labels (write-side org-label tools).
- Caching label results across calls.
- Cross-org listing or label-search tools.
- SDK contributions to upstream `forgejo-sdk` (tracked separately).

## Decisions

### D1: Use `pkg/forgejo.DoJSONList` rather than vendoring the SDK

Two alternatives considered:

1. Fork/vendor the SDK and add `ListOrgLabels` — high friction, ongoing maintenance burden, breaks our "track upstream SDK" stance.
2. Use the existing `DoJSONList` raw-HTTP helper — already used for attachments, share auth/UA/logging, no new code paths.

Choose (2). When the SDK gains `ListOrgLabels`, swap the call in one place.

### D2: Two tools, not one

Alternatives:

1. Single tool `list_repo_labels` with `include_org_labels` flag, no standalone org tool.
2. Standalone `list_org_labels` tool, no merge in `list_repo_labels`.
3. Both — standalone `list_org_labels` plus opt-in merge in `list_repo_labels`. (chosen)

(3) covers the reporter's "one call" need while still giving callers that already know the org name a direct path that doesn't require a repo. Single-responsibility wins for the standalone tool; ergonomics win for the merged form.

### D3: `include_org_labels` defaults to `true`

Default-on matches the reporter's expectation ("no reason for the model to make 2 separate requests") and matches what the model would naturally want when discovering valid IDs. Callers wanting strict repo-only behavior pass `false` explicitly.

### D4: Stamp each label with `scope: "repo" | "org"`

Repo labels and org labels live in disjoint ID spaces in Forgejo, but a caller looking at a merged list needs to know which scope each ID belongs to — for documentation, for filtering, and to surface the distinction to the LLM. The `scope` field is added by the handler, not returned by the underlying API. We define a small wrapper type rather than mutating SDK structs.

### D5: Skip the org call when owner is not an org

`list_repo_labels` already has the owner string. We could:

1. Always attempt `/orgs/{owner}/labels` and rely on 404 → empty (`DoJSONList` handles this).
2. Pre-check via `/orgs/{owner}` and skip the second call for user-owned repos.

Choose (1). One unconditional GET is simpler than a conditional probe-then-fetch and the helper already maps 404 to an empty list with no error. The trade-off is a wasted round-trip for user-owned repos, which is acceptable given the call is light and rare in practice.

### D6: Pagination is shared

`include_org_labels` reuses the `page` and `limit` params. We do not split them, because most repos and orgs have small (≤100) label counts and split params double the parameter surface for marginal benefit. Documented behavior: pagination applies to each scope independently — repo labels paginated by `page`/`limit`, org labels paginated by the same `page`/`limit`. Callers who care can call `list_org_labels` directly.

## Risks / Trade-offs

- [Org-label call adds latency to `list_repo_labels` for org-owned repos] → One extra GET only when merge is enabled (default), and the helper is already tuned for low-overhead calls. Caller can disable via `include_org_labels=false`.
- [Wasted GET for user-owned repos] → Mitigated by the helper's 404→empty mapping; observable in logs but harmless. Future optimization could cache `is_org` per owner.
- [Label name collision between repo and org scope] → Distinct IDs in Forgejo + the `scope` field in the response disambiguate. Documented in the spec.
- [`list-milestones-labels` change has not yet archived, so there is no `list_repo_labels` capability spec to delta] → This change ships `list_org_labels` as a new capability immediately; the `list_repo_labels` modified-capability delta lands once the parent change archives. Implementation can proceed in parallel; only the spec sync is order-dependent.
- [SDK gains `ListOrgLabels` later] → Single call site in our handler; swap is trivial. No public contract change.

## Migration Plan

Additive only — no migration. New tool plus a new optional parameter on an existing tool.

Rollout:

1. Land `list-milestones-labels` change (already in flight).
2. Land this change. New tool registers; existing callers of `list_repo_labels` continue to work because `include_org_labels` defaults to `true` but the `scope` field is purely additive on the wire.
3. Update `docs/PROMPTING.md` examples if any reference label IDs.

Rollback: revert the commit. Tool de-registers cleanly; no persistent state.

## Open Questions

- Should `scope` be present on results from `list_org_labels` too? Probably yes for consistency — every label out of the system carries scope, regardless of which tool returned it. Decided: yes, scope is always present.
- Do we want a `scope` filter on `list_repo_labels` (e.g. `scope=org` to return only org labels)? Out of scope for this change; revisit if requested.

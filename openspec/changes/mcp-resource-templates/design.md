## Context

`forgejo-mcp` exposes Forgejo entities exclusively through MCP **tools** today. `operation/operation.go` constructs the server with `server.WithLogging()` only — no `server.WithResourceCapabilities(...)` — and every `RegisterXTool` function calls `s.AddTool(...)`. The fourteen registered tool domains in `operation/` cover the same entities this change wants to model as resources (issue, pr, repo, comment, …) plus mutating actions.

Two upstream facts shape the design:

1. **mcp-go v0.44.0 is in `go.mod`**. It provides `mcp.NewResourceTemplate(uriTemplate, name, opts...)` (RFC 6570 templates), `server.AddResourceTemplate(template, handler)`, and `server.WithResourceCapabilities(subscribe, listChanged bool)`. Handlers return `[]mcp.ResourceContents` (text or blob variants), so each resource fetch can return one or more content blocks.
2. **Output bounding is a hard rule** (`docs/design/output-bounding.md`). Any output proportional to repository state must expose a caller-controlled bound and a way to fetch the remainder. Resource handlers must respect this even though MCP `resources/read` has no native `limit`/`page` parameter — bounding has to happen *inside* the embedded payload.

Stakeholders: MCP client authors (Claude Code/Desktop, Codex, Cursor) consuming the server; this project's tool surface (must not regress); the Forgejo API rate limiter (resource reads become an additional traffic source).

## Goals / Non-Goals

**Goals:**

- A single coherent URI scheme (`forgejo://…`) covering all seven entities so client UX is uniform.
- Per-domain `RegisterResources(s *server.MCPServer)` entry points that mirror the existing `RegisterXTool` pattern, keeping the codebase navigable.
- Resources coexist with tools: clients without resource-template support keep working unchanged.
- Embedded lists inside resources (e.g. comments on an issue) follow the existing output-bounding contract.
- Cacheability hints set correctly: commit + commit-status are immutable per-sha and surfaced as such; issue/pr/comment are mutable and not flagged cacheable.

**Non-Goals:**

- **Implementation.** This change ships proposal + design + specs + tasks only; code lands under a follow-up change.
- **Mutating resources.** No write-through. Mutations stay on tools.
- **Subscriptions.** `server.WithResourceCapabilities(subscribe=false, listChanged=false)` for v1. No `resources/subscribe` handler.
- **Wiki / projects resources.** Blocked upstream (see `docs/plans/wiki-support.md`, `docs/plans/projects-support.md`).
- **Webhook / event resources.** Out of scope.
- **Replacing existing read tools.** `get_issue_by_index`, `get_pull_request_by_index`, etc. remain. Resources duplicate the read surface for clients that prefer URI addressing; the duplication is intentional during the migration window.

## Decisions

### D1: URI scheme — `forgejo://` (custom) over `https://…/api/v1/…`

`forgejo://repo/{owner}/{repo}/commit/{sha}` rather than the upstream REST URL.

Rationale: clients should not have to know which Forgejo instance the server is talking to (Codeberg vs self-hosted). The scheme also gives a clean discriminator between "this is an MCP resource URI" and "this is a web link the agent should open" — important because Forgejo `https://` URLs appear constantly in PR bodies and would otherwise collide with the resource namespace.

**Alternatives considered:**
- `https://<instance>/<owner>/<repo>/commit/<sha>`: tempting because users paste these. Rejected because (a) it leaks instance hostname into URIs (multi-instance deployments break), (b) creates ambiguity with link-fetching behavior, (c) requires URI parsers to special-case which `https://` URLs are MCP resources.
- `forgejo+resource://`: more explicit but ugly and unnecessary; the scheme already identifies the server.

### D2: Path shape — `forgejo://repo/{owner}/{repo}/<entity>/<key>` (entity nested under repo)

All repo-scoped entities sit under `repo/{owner}/{repo}/`. Owner is a top-level peer: `forgejo://owner/{name}`.

Rationale: every entity except owner is repo-scoped in Forgejo; the path mirrors the API. Makes URIs predictable and templates easy to write as RFC 6570.

**Alternative considered:** flatter `forgejo://issue/{owner}/{repo}/{index}`. Rejected because it splits the natural "this lives in a repo" parent across siblings; harder to scan.

### D3: Comment URIs discriminate on parent kind

`forgejo://repo/{owner}/{repo}/{kind}/{index}/comment/{id}` where `{kind}` ∈ `issue` | `pr`.

Rationale: Forgejo's API uses one comment table internally but the parent context matters for the agent (permissions, rendering, references). Embedding `kind` makes the URI self-describing — no need to fetch the parent to know whether a comment lives on an issue or PR.

**Alternative considered:** `forgejo://repo/{owner}/{repo}/comment/{id}`. Rejected because IDs collide across kinds in unhelpful ways and the agent loses context.

### D4: Combined status URI — `…/commit/{sha}/status` (singular)

`forgejo://repo/{owner}/{repo}/commit/{sha}/status` returns the *combined* status (aggregate state + per-context entries). The per-context list view stays a tool (`get_commit_statuses`) to keep the resource a single addressable thing.

Rationale: aggregate is what callers actually want when they ask "is this sha green?". The per-context list is bounded data that benefits from `page`/`limit` parameters tools support natively but resources don't.

### D5: Resource capability declaration — `WithResourceCapabilities(false, false)`

Subscribe = false, listChanged = false.

Rationale: v1 is read-on-demand only. `listChanged` would force the server to track template changes at runtime; since templates are static at startup, false is correct. Subscribe is explicitly punted — see Non-Goals.

### D6: Per-domain `RegisterResources(s)` entry point

Each domain under `operation/{domain}/` that owns one or more resources adds a `RegisterResources(s *server.MCPServer)` function alongside its existing `RegisterTool`. `operation/operation.go` gains a parallel list of `RegisterXResources(s)` calls.

Rationale: minimises diff against current layout. A reader who knows where the issue *tools* live finds the issue *resources* in the same package. No new top-level directory.

**Alternative considered:** a single `operation/resources/` package with all handlers. Rejected because it collects unrelated entity code under one roof, defeating the per-domain locality the rest of the codebase relies on.

### D7: Shared URI parser in `operation/resource/`

A new package (note singular) holding: RFC 6570 template registration helpers, URI parsing into typed structs (e.g. `parseCommitURI(req.Params.URI) (owner, repo, sha string, err error)`), and embedded-list bounding helpers. Each domain handler depends on this package, not the other way round.

Rationale: avoids duplicating URI parsing across seven domains. Keeps the per-domain handlers tiny — they go straight from typed params to a `pkg/forgejo` SDK call.

### D8: Embedded-list bounding — sentinel + index tool reference

When a resource embeds a variable-size list (issue comments on the issue resource, statuses on a commit-status resource), the embedded payload is capped at **N items** (default 30, exposed as a server-side constant; revisit if telemetry shows truncation hitting frequently) and ends with a sentinel block:

```
[truncated: N of M items shown. Use list_<entity> tool with page=2 to fetch more.]
```

This satisfies `docs/design/output-bounding.md` sub-rules 1 (no silent truncation) and 3 (always resumable) without inventing a new query parameter on `resources/read`. The sentinel names the existing list tool, completing the contract.

**Alternative considered:** parameterise via URI query (`?limit=50`). Rejected because RFC 6570 template matching makes optional query params painful and most MCP clients don't surface them in URIs the LLM constructs.

### D9: MIME types — `application/json` for entity bodies, `text/markdown` for rendered text fields

Each resource handler returns one or more `mcp.ResourceContents`. Primary content is the entity as JSON (`application/json`). Long markdown fields (issue body, PR description, commit message body) MAY additionally be returned as a sibling `text/markdown` content block, letting clients pick the form they want.

Rationale: JSON is the lingua franca for structured agent reasoning. Markdown sidecars cost little and dramatically improve LLM comprehension of formatted bodies.

### D10: Caching hints — annotations on immutable resources

`mcp.WithTemplateAnnotations` on `mcp-resource-commit` and `mcp-resource-status` carries `audience: [Role.Agent]` and `priority: 1.0` plus a description noting immutability. mcp-go does not currently surface an explicit cache header in the resource result, so the signal lives in the human-readable description until upstream adds one. We do NOT roll our own cache headers.

**Risk noted under Risks.**

### D11: Auth & errors — reuse `pkg/forgejo` singleton + map `403`/`404` to MCP errors

Resource handlers go through the same client used by tools. On `403` return MCP error code `-32002` (custom — match what existing tools return for access denial); on `404` return code `-32003`. Both with descriptive messages.

Rationale: identical semantics to today's tool errors. Clients that already handle tool-error patterns get the same UX for resources.

### D12: Client compatibility — design for `resources/templates/list` consumers; gracefully degrade for tool-only clients

Targets clients that implement MCP `resources/templates/list` and `resources/read` (Claude Code, Claude Desktop, Codex, current Cursor). Tool-only clients keep working: every entity remains reachable via existing tools, so absence of resource support costs nothing.

Compatibility matrix to publish in README:

| Client            | `resources/templates/list` | `resources/read` | Notes |
|-------------------|----------------------------|------------------|-------|
| Claude Code       | ✅                          | ✅                | Primary target. |
| Claude Desktop    | ✅                          | ✅                | URL-pasted `forgejo://` URIs auto-resolve. |
| Codex             | ✅                          | ✅                | Same as Claude Code. |
| Cursor (current)  | ✅                          | ✅                | Verify in implementation. |
| Older / minimal   | ⚠️ Tools only               | ⚠️ Tools only     | No regression — tools unchanged. |

### D13: Rollout order — `mcp-resources-core` + `mcp-resource-commit` as the thin slice

Implementation slices land in this order, each as an independent PR:

1. `mcp-resources-core` (framework + URI parser + capability registration)
2. `mcp-resource-commit` (highest value — replaces curl-for-statuses workflows)
3. `mcp-resource-status`
4. `mcp-resource-repo` + `mcp-resource-owner`
5. `mcp-resource-issue` + `mcp-resource-comment`
6. `mcp-resource-pr`

Rationale: slice 1+2 delivers real value (commit/status fetching by sha) on its own. Each subsequent slice is independently shippable behind its own `RegisterXResources` call.

## Risks / Trade-offs

- **[Inconsistent client behavior on resource templates]** → Pin compatibility matrix in README; verify each target client during implementation slice 1; keep tools as the fallback contract so non-supporting clients never lose functionality.

- **[Caching not enforceable from server side in mcp-go v0.44.0]** → Annotate immutable resources via description; revisit when upstream exposes explicit cache hints. Document the gap in README so client implementers don't assume server-side caching.

- **[Duplicate read surface (tool + resource) confuses agents and inflates LLM context]** → Specs explicitly call out coexistence; README publishes a single recommendation table ("prefer resource when you have a sha/index in hand; prefer tool when listing or searching"). If telemetry shows agents picking the wrong one, revisit by deprecating overlap rather than removing it.

- **[Embedded-list truncation pointing at a wrong list tool]** → Sentinel format is centralised in `operation/resource/`; each entity test asserts the sentinel names a tool that actually exists. Catches drift at compile + test time.

- **[Forgejo API rate-limit pressure from resource reads]** → Embedded lists are bounded (D8); resource handlers do not pre-fetch related entities; commit + status resources are inherently cacheable so heavy callers can dedupe by sha at the client layer.

- **[URI-scheme collision with future Forgejo features]** → `forgejo://` is owned by this server's contract. Document the namespace in README; new entities must extend it explicitly via a new change. No catch-all path.

- **[`pull_request_target`-style security concerns from agents auto-resolving URIs]** → Resources are read-only and route through the same auth as tools. No new attack surface beyond existing tool reads. Documented in README security section.

## Migration Plan

Strictly additive. No rollback strategy needed for existing tools.

Per-slice rollout (D13). Each slice ships with:
1. New `RegisterXResources(s)` registration in `operation/operation.go`.
2. Per-entity handler + tests.
3. README update appending to the resource table.

Rollback for any slice: revert the registration call in `operation/operation.go`. Resources disappear from `resources/templates/list`; tools remain.

## Open Questions

- **Default embedded-list cap (D8) — 30 the right number?** Pulled from existing list-tool defaults. Should validate against typical issue/PR comment thread depth on Codeberg before locking in. Possibly raise per-entity (e.g. PRs have more review comments than issues).
- **Should `forgejo://owner/{name}` distinguish users vs orgs in the URI?** Current proposal does not. If clients want to know without fetching, we'd add `forgejo://user/{name}` and `forgejo://org/{name}` instead. Defer until a real use case surfaces.
- **Markdown sidecar (D9) — return always, or only when the field is non-trivial (> N chars)?** Default to always for simplicity; revisit if context bloat becomes a problem.
- **Compatibility matrix verification (D12)** — needs hands-on testing against each target client in implementation slice 1. Spec acceptance for `mcp-resources-core` should include a test plan covering this.

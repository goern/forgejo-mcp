## Context

Every issue-listing tool on this server is repo-scoped. When an agent asked for an
organization's open issues ([#452](https://git.b4mad.industries/agentic-forges/forgejo-mcp/issues/452)),
it reached for the closest match — `list_repo_issues` — and left `repo` empty. That value
flowed unvalidated into `client.ListRepoIssues` (`operation/issue/issue.go:302`), whose path
guard `escapeValidatePathSegments` (forgejo-sdk v3 `issue.go:179`) returned
`path segment [1] is empty`. The tool wrapped it as `get issues list err: …` and the agent
had no way to learn that the capability simply does not exist.

The upstream capability does exist. `client.ListIssues(opt)` (forgejo-sdk v3 `issue.go:157`)
issues `GET /repos/issues/search`, and `ListIssueOption.QueryEncode` already emits an `owner`
query parameter (`issue.go:146-147`). Nothing needs to be added to the SDK or vendored.

Constraints that shape the design:

- `docs/design/output-bounding.md` is an architectural invariant: any tool whose output size
  depends on data MUST expose a caller-controlled bound **and** a way to fetch the remainder.
  A list of entities maps to the `page` / `limit` row of its sub-rule 2 table.
- The SDK does not parse `X-Total-Count` for issue searches — grep for `TotalCount` across
  forgejo-sdk v3 finds it only in `action.go`, `repo_tree.go`, and `status.go`. There is no
  honest total to report.
- The SDK-backed list tools in `operation/issue` return bare JSON arrays via
  `to.TextResult(issues)`. The wiki list tools already return objects carrying `page` and
  `has_next` (`openspec/specs/wiki-tools/spec.md:24-38`), and that spec already settles how
  `has_next` is derived.
- `ListOptions.getURLQuery` (forgejo-sdk v3 `list_options.go:26-32`) writes `PageSize` raw and
  `setDefaults` (`list_options.go:37-44`) touches only `Page`. Nothing between the tool and
  the server clamps `limit`; the SDK's own comment at `list_options.go:22` notes the ceiling
  depends on the instance's `MAX_RESPONSE_ITEMS`.

## Goals / Non-Goals

**Goals:**

- Answer "which issues are open in owner X?" in one tool call, across repositories.
- Satisfy the output-bounding contract fully — bound, resumability, and documentation.
- Turn the `path segment [1] is empty` dead end into an error that names the missing
  parameter and points at the tool that can serve the request.
- Reuse the existing parameter vocabulary so agents do not learn a second dialect.

**Non-Goals:**

- Changing `list_repo_issues`' response shape or its behavior for well-formed calls.
- Retrofitting the other list tools to the envelope shape. That is the `#124` umbrella.
- Exposing a `team` filter (see Decision 5).
- Cross-owner or instance-wide search. `owner` is required; unscoped search would be
  unbounded by anything the caller controls.

## Decisions

### 1. A new tool, not an optional `repo` on `list_repo_issues`

**Chosen:** add `search_issues`; leave `list_repo_issues` signature intact.

`list_repo_issues` declares `repo` as `mcp.Required()`. Relaxing it would mean one tool with
two upstream endpoints, two result shapes, and a required-parameter contract that changes
meaning depending on another argument's presence — the kind of conditional surface that
agents reliably get wrong. The two endpoints also differ in capability: `/repos/issues/search`
supports `q`, `created_by`, `assigned_by`, and `mentioned_by`, which the repo-scoped endpoint
does not. Folding them together would leave those filters silently inert half the time.

*Alternative considered:* make `repo` optional and branch internally. Rejected — cheaper to
implement, but it hides a real capability difference behind one name and makes the tool
description self-contradictory.

### 2. Response envelope instead of a bare array

**Chosen:** return `{issues, page, limit, count, has_next}`.

Sub-rule 3 of the bounding contract requires a continuation signal in the response. A bare
JSON array has nowhere to put one. This adopts the envelope shape `list_wiki_pages` already
uses (`openspec/specs/wiki-tools/spec.md:24-38`), so the server has one `has_next` dialect
rather than two. The bare-array shape in `operation/issue` is the outlier, and the `#124`
retrofit is already heading this way.

*Alternative considered:* bare array plus a trailing sentinel string, mirroring
`operation/resource.Bounded`. Rejected — `Bounded` exists because `resources/read` has no
caller-controlled range parameter and must smuggle the signal into content. A tool with real
`page` / `limit` parameters has no such constraint, and a sentinel inside a JSON array
corrupts the array's type.

### 3. Instance ceiling is discovered, not guessed; `has_next` follows from that

**Superseded 2026-08-06.** The paragraph below this note is what shipped in draft PR #458 and
is kept, struck through in spirit, so the reasoning failure is visible rather than quietly
edited away. [chris420](https://git.b4mad.industries/agentic-forges/forgejo-mcp/pulls/458#issuecomment-5533),
who reported the original issue (#452), reviewed #458 and falsified its central premise: the
"Alternative considered" paragraph below claims the `MAX_RESPONSE_ITEMS` ceiling "can only
guess at an admin-set value". That is false. `GET /api/v1/settings/api` returns
`{"max_response_items": N, ...}` **unauthenticated**, and forgejo-sdk v3 already wraps it as
`(*Client).GetGlobalAPISettings()` (`settings.go`). We conceded the point publicly
([issuecomment-5535](https://git.b4mad.industries/agentic-forges/forgejo-mcp/pulls/458#issuecomment-5535))
rather than defend it.

**Original text (now superseded):**

> **Chosen:** empty page → false; otherwise request `page+1` at the **same** `limit` and set
> `has_next` from whether the probe returned rows. Never `limit+1`.
>
> With no parsed `X-Total-Count`, a `total_count` field could only be fabricated.
>
> The obvious cheap rule — `has_next = (count == limit)` — is **unsound here**, and this is the
> central correctness decision of the change. Because nothing clamps `limit` client-side (see
> Context), an instance with `MAX_RESPONSE_ITEMS=50` answers a `limit=100` request with 50
> issues. `count != limit`, so the cheap rule reports `has_next` false *while more data exists*
> and the caller stops paging. That is a false **negative** — silent truncation, which
> `docs/design/output-bounding.md` sub-rule 1 exists to forbid.
>
> *Alternative considered:* clamp `limit` client-side to a documented ceiling and keep the
> count-based rule. Rejected — the ceiling can only guess at an admin-set value, and any guess
> above the real `MAX_RESPONSE_ITEMS` reintroduces the same false negative.

**Chosen (revised):** fetch and cache the instance's `max_response_items` via
`GetGlobalAPISettings()`, keyed on the instance base URL (not on any particular `*forgejo.Client`
value — `pkg/forgejo.Client(ctx)` hands out a fresh ephemeral client whenever a token is present
in the context, so a per-client cache would refetch on every call). When the ceiling is known:

- A caller `limit` above the ceiling is **rejected before the upstream call**, with an error
  naming both the requested limit and the ceiling (e.g. "limit 100 exceeds this instance's
  maximum of 50 (max_response_items); retry with limit <= 50"). The number must be in the
  message — the audience is an LLM agent, and the retry has to be self-correcting.
- Because the rejection guarantees the *effective* limit sent upstream never exceeds the
  ceiling, the request is always honored in full, and the count-based rule this decision
  previously rejected becomes sound again: `has_next = (count == effectiveLimit)`. It was only
  unsound because nothing clamped `limit` client-side; something now does, by refusing rather
  than silently truncating.
- The probe survives as a **fallback for when the ceiling is unknown** (settings endpoint
  unreachable, non-2xx, or too old to have it) — that question is open with chris420, and a
  restrictive instance is exactly the case the probe still has to cover. It keeps its original
  rules where it runs: request `page+1` at the **same** `limit`, never `limit+1` (Forgejo
  derives page offsets from page size, so changing it makes later pages skip rows), and
  `has_next` false on an empty page.
- A failed ceiling lookup MUST be treated as "unknown", never as a ceiling of `0` — a zero
  ceiling would reject every request.

The response also now reports the limit **actually sent upstream** (`effectiveLimit`), sourced
from the same variable used to build the SDK call, rather than recomputing the requested value
independently at the point the envelope is built. Before this fix, `issue.go:433` set
`Limit: int(limit)` from the raw requested value: a `limit=100` call against a `50`-ceiling
instance got 50 rows back but reported `limit: 100`, violating the "response reports the value
actually used" requirement in `specs/search-issues/spec.md`. That specific mismatch cannot
occur post-fix, since a `limit=100` request against a known `50` ceiling is now rejected before
it reaches the SDK at all.

*Alternative considered:* parse the `X-Total-Count` header off the SDK's `*Response`.
Deferred — reaching past the SDK's typed surface into raw headers for one field is a
different change with a different blast radius, and it is only worth it once the `#124`
retrofit wants exact totals everywhere.

### 4. Validation guard before the SDK call

**Chosen:** check `owner` and `repo` for emptiness in `ListRepoIssuesFn` and return a
purpose-written error; do not let the SDK's path guard be the validator.

The empty-`repo` error must name `repo` and mention `search_issues`. That makes the failure
self-correcting: an agent that hits it learns both what it got wrong and which tool to call
instead. The spec asserts the absence of the phrase `path segment` so a future refactor
cannot silently regress to leaking SDK internals.

### 5. No `team` filter

`ListIssueOption.QueryEncode` sends `query.Add("team", opt.MentionedBy)` (forgejo-sdk v3
`issue.go:150`) — the `team` key carries the `MentionedBy` value. This is an upstream bug.
Exposing a `team` parameter would present it as ours: callers would filter by team and get
mention-filtered results with no error. Omitted until fixed upstream; file separately against
forgejo-sdk.

Draft issue text is at `openspec/changes/add-search-issues-tool/upstream-sdk-issue.md`. It is
not filed — a human files it.

### 6. Owner is required

Making `owner` optional would allow an unscoped instance-wide search whose result size no
caller-controlled parameter meaningfully bounds — `limit` caps the page, not the work.

The SDK comment at v3 `issue.go:156` describing this endpoint as returning "all issues
assigned the authenticated user" is stale: `QueryEncode` emits no assignment filter unless the
caller sets `AssignedBy` (`issue.go:140-142`). An unscoped call searches every repo the token
can read, which strengthens rather than weakens the bounding argument above.

## Risks / Trade-offs

| Risk | Mitigation |
|---|---|
| The probe costs one extra upstream request on every non-empty call | Accepted deliberately: it buys a `has_next` that cannot under-report. The count-based alternative is free but ships silent data loss under a documented server setting |
| Envelope shape differs from the sibling tools in `operation/issue`; agents may mis-parse | Tool description states the shape explicitly; it matches `list_wiki_pages`, so the server has one dialect, and it is the direction of travel for `#124` |
| `owner` accepted but not verified to exist — a typo returns empty, not an error | Upstream returns 200 with no results; an empty set with `has_next` false is the honest answer, and verifying the owner would cost an extra round trip on every call |
| Result set spans repos with mixed visibility | Upstream filters by token scope; no client-side filtering needed or attempted |
| Wide `limit` with large issue bodies still fills context | Same exposure as every list tool; `limit` is the caller's knob, per the contract |

## Migration Plan

Additive. No tool is removed, no signature changes, no persisted state, no config.
Deploy is a normal binary release; rollback is redeploying the prior binary. Clients that
never call `search_issues` see one behavior difference: `list_repo_issues` with an empty
`repo` now returns a clear validation error instead of the SDK path-segment message — a
call that was already failing.

## Open Questions

- Should `search_issues` gain a sibling `search_pull_requests`, or is `type=pulls` enough?
  Leaning on `type`, matching how `list_repo_issues` handles the same split.
- Once `#124` retrofits the other list tools, should they adopt this envelope? Worth deciding
  as one cross-tool call rather than per tool.

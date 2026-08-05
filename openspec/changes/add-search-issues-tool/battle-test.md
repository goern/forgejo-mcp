# Battle Test — add-search-issues-tool

**Date:** 2026-08-04
**Targets:** proposal.md, design.md
**Team:** adversary, defender

## Verdict

**Patch first.**

Six critiques, six concessions, zero defends. The lead independently re-verified every
load-bearing claim against the vendored SDK and archived specs rather than accepting the
capitulation at face value — all six hold. The change's *shape* survives intact: a dedicated
`search_issues` tool, owner-required, envelope response, with a validation fix on
`list_repo_issues`. What does not survive is the `has_next` derivation, which is not merely
suboptimal but **inverted**: it can under-report the end of data, which is precisely the
silent truncation `docs/design/output-bounding.md` exists to forbid. The design also claimed
novelty for an envelope shape that archived canon already mandates, and cited three SDK
locations that were off by one to three lines. Patch, then proceed — no rework of the
approach itself.

## Surviving Critiques

### `has_next = (count == limit)` under-reports when the instance clamps `limit`
**Severity:** blocker
**Failure mode:** `ListOptions.getURLQuery` (v3 `list_options.go:26-32`) writes `PageSize`
raw, and `setDefaults` (`list_options.go:37-44`) touches only `Page` — nothing between the
tool and the server clamps `limit`. The SDK's own comment at `list_options.go:22` states
"The highest valid value depends on the server config MAX_RESPONSE_ITEMS". So a `limit=100`
request against an instance clamping at 50 returns 50 issues, `count != limit`, and
`has_next` reports **false while more data exists**. The caller stops paging and silently
loses the remainder.
**Hits:** design.md#Decisions.3, design.md:119 (risk table), spec.md:69-82, proposal.md:24-28
**Canonical conflict:** `docs/design/output-bounding.md` sub-rule 1 ("Never silently drop
bytes") and sub-rule 3 ("Always resumable"). This is the exact defect the document was
written to prevent.
**Falsifier:** Call `search_issues` with `limit=100` against an instance with
`MAX_RESPONSE_ITEMS=50` and an owner having >50 matching issues. Design predicts `has_next`
true; it returns false.
**Disposition:** patch
**Note:** design.md:82 asserts "a false positive, never a false negative — the caller is
never told the data ends when it does not." That is exactly backwards under clamping.

### Envelope shape and probe rule already exist in archived canon
**Severity:** blocker
**Failure mode:** design.md:23 states "Existing list tools return bare JSON arrays" and
Decision 2 claims `search_issues` is "the first list tool here to return an object". Both are
false. `openspec/specs/wiki-tools/spec.md:26-38` is archived normative canon: `list_wiki_pages`
already returns `page` + `has_next`, already derives `has_next` by a pagination-preserving
next-page probe, and already **forbids** `limit+1` because "Forgejo derives page offsets from
the page size, and changing it causes later pages to skip rows." Decision 2 argues at length
for a position canon already holds; Decision 3 silently diverges from a rule canon already
settled, without acknowledging it exists.
**Hits:** design.md:23, design.md#Decisions.2, design.md#Decisions.3, design.md:120
**Canonical conflict:** `openspec/specs/wiki-tools/spec.md:26-38`
**Falsifier:** Read that spec section. (Verified by lead.)
**Disposition:** patch
**Defender's extension, accepted:** the wiki rule triggers the probe only on a *full* page,
so it inherits the same clamping defect — it escapes in practice because wiki `limit` defaults
to the server page size and is never raised past the clamp. `search_issues` advertises
`limit=100`, so it needs a **strict superset**: probe on any non-empty page, and say why it
widens the inherited rule.

### `type` cannot be suppressed; the spec requires the impossible
**Severity:** major
**Failure mode:** spec.md:98 requires "Omitted filters SHALL NOT be sent upstream." But
`QueryEncode` emits `query.Add("type", string(opt.Type))` at v3 `issue.go:124`, outside any
length guard — unlike every neighbouring filter. `page` and `limit` are likewise emitted
unconditionally. The requirement is unsatisfiable without hand-rolling the query string, and
tasks.md 4.6 instructs an implementer to write a test asserting it.
**Hits:** spec.md:98, tasks.md 4.6
**Falsifier:** Read v3 `issue.go:124` and compare to the guarded emits at 120-122 and 126-128.
(Verified by lead.)
**Disposition:** patch

### A `list_repo_issues` requirement is filed under the `search-issues` capability
**Severity:** major
**Failure mode:** proposal.md:46-49 declares no modified capabilities and says the validation
fix is "captured as a task, not a spec delta". But spec.md:136-161 is a full
`### Requirement:` with three scenarios and MUST-language governing `list_repo_issues` —
sitting inside the `search-issues` capability file. The proposal is right that nothing is
*modified* (no spec in `openspec/specs/` governs `list_repo_issues`; only `cli-mode/spec.md:48`
mentions it, incidentally). It is wrong that nothing is *specified*. At archive time a
requirement about one tool lands permanently under a capability named for a different tool.
**Hits:** proposal.md:40-49, spec.md:136-161
**Falsifier:** `rg 'list_repo_issues' openspec/specs/` returns only the `cli-mode` incidental
mention, confirming no existing capability owns it. (Verified by lead.)
**Disposition:** patch

### Decision 6 rests partly on a stale SDK comment
**Severity:** watch
**Failure mode:** design.md#Decisions.6 claims a bare `/repos/issues/search` means "issues
assigned to me". It does not — `QueryEncode` emits nothing assignment-scoping unless the
caller sets `AssignedBy` (v3 `issue.go:140-142`). The claim traces to the SDK's own stale
doc comment at `issue.go:156`. The decision's other half — that an unscoped search is bounded
by nothing the caller controls — is sound and load-bearing on its own.
**Hits:** design.md#Decisions.6
**Falsifier:** Read `QueryEncode` end to end; no assignment filter is unconditional.
(Verified by lead.)
**Disposition:** patch

### Three SDK line citations are wrong
**Severity:** watch
**Failure mode:** The `team`/`MentionedBy` defect is at v3@v3.0.0 `issue.go:150`, not 148.
The `owner` emit is at 146-147, not 145. tasks.md 5.3 would have carried the wrong line into
a public upstream bug report.
**Hits:** design.md:13, design.md#Decisions.5, proposal.md:35, tasks.md 5.3
**Falsifier:** Read lines 144-152. (Verified by lead.)
**Disposition:** **already patched** — corrected in place as a factual typo, version pinned
in tasks.md 5.3.

## Conceded Patches

Applied already:

- design.md:13, design.md#Decisions.5, proposal.md:35, tasks.md 5.3 — SDK line numbers
  corrected to `issue.go:150` / `issue.go:146-147`; upstream report pinned to v3 v3.0.0.

Awaiting user approval (structural):

**`has_next` rewrite — the blocker.** One unified edit resolves both blockers:

- **spec.md:69-71** — replace the `has_next` paragraph: derive by pagination-preserving
  next-page probe. `count == 0` → false. Otherwise request `page+1` at the **same** `limit`;
  `has_next` follows whether the probe returned rows. MUST NOT request `limit+1`. State that
  `count == limit` is rejected *because* the instance may clamp `limit` at
  `MAX_RESPONSE_ITEMS`. Cite `openspec/specs/wiki-tools/spec.md:31-38` as the inherited rule
  and name the widening from "probe on a full page" to "probe on any non-empty page".
- **spec.md:73-82** — rewrite both scenarios to the probe.
- **spec.md:47-49** — append "**AND** the count may be lower if the instance clamps `limit`
  at `MAX_RESPONSE_ITEMS`" (the current "up to 100" wording is satisfiable but misleading).
- **design.md#Decisions.3** — retitle to "`has_next` by same-limit next-page probe; no
  `total_count`" and rewrite the body, naming the clamp as the reason for widening.
- **design.md:23** — replace with: SDK-backed list tools in `operation/issue` return bare
  arrays; the wiki list tools already return objects with `page` and `has_next`
  (`openspec/specs/wiki-tools/spec.md:24-38`).
- **design.md#Decisions.2** — drop "first list tool here to return an object" and the
  "deliberate inconsistency" framing; replace with adopting the envelope `list_wiki_pages`
  already uses, so the server has one `has_next` dialect.
- **design.md:119** — replace the wasted-call risk row with the probe's real cost: one extra
  upstream request per non-empty call, buying a `has_next` that cannot under-report.
- **proposal.md:24-28, 62-64** — restate `has_next` as probe-derived; drop "a departure from
  the bare-array shape".
- **tasks.md 2.5, 4.3** — rewrite to the probe. Add **4.9**: "Test that a `limit=100` request
  answered with 50 issues still reports `has_next` true when a further page exists."

**`type` filter:**

- **spec.md:98** — replace with: omitted filters SHALL be left unset on `ListIssueOption`;
  note the SDK emits `type` unconditionally (v3 `issue.go:124`), so an omitted `type` reaches
  the wire as empty `type=`, which upstream treats as no filter — the one filter the tool
  cannot suppress.
- **tasks.md 4.6** — assert omitted filters carry no value, **except** `type`, asserted
  present and empty.

**Capability split:**

- Move **spec.md:136-161** verbatim into a new delta file
  `specs/issue-listing-validation/spec.md` under `## ADDED Requirements`.
- **proposal.md:40-49** — add `issue-listing-validation` as a second **New Capability**
  ("input validation for repo-scoped issue listing, including the redirect to
  `search_issues`"); correct the Modified Capabilities note to say the validation fix is its
  own delta, not a task-only change.

**Stale claim:**

- **design.md#Decisions.6** — delete the "issues assigned to me" sentence and its trailing
  clause; keep the bounding argument. Optionally note the SDK comment at `issue.go:156` is
  stale.

## Future Work

- **`X-Total-Count` parsing.** Would replace the probe with an exact total and remove the
  extra round trip. Requires reaching past the SDK's typed surface into `*Response` headers —
  correctly deferred to the `#124` umbrella, where it can be decided once for all list tools.
- **Upstream SDK bug.** `QueryEncode` sending `opt.MentionedBy` under the `team` key
  (v3@v3.0.0 `issue.go:150`). Affects every consumer of the SDK, not just this repo.
  tasks.md 5.3 owns filing it.
- **Stale SDK doc comment** at `issue.go:156` describing `/repos/issues/search` as
  assignment-scoped. Worth folding into the same upstream report.
- **CLAUDE.md drift** (raised by defender, out of scope here): the architecture summary says
  `pkg/params/`, but the package is `operation/params/`.
- **Envelope adoption across list tools** — already an open question in design.md; the wiki
  precedent strengthens the case for standardizing rather than treating this as novel.

## Lead Recommendation

Apply the patch set above before `/opsx:apply`. The two blockers are not stylistic: the
`has_next` defect ships silent data loss under a documented, admin-controlled server setting,
and the canon miss means the change would have introduced a second `has_next` dialect into a
server that already settled the question.

One decision is genuinely the user's, not the team's. The probe costs **one extra upstream
round trip on every non-empty call** — defender raised this unprompted and recommended paying
it. The cheaper alternative is clamping `limit` client-side to a documented ceiling, but that
only guesses at an admin-set value and reintroduces the same false negative whenever the guess
is high. Lead concurs with defender: take the probe, and record the cost honestly in the risk
table rather than hiding it.

Note on process: all four artifacts under review were authored by the lead in this same
session, so `defender` was defending the lead's reasoning and the lead was mediating its own
design. Every critique was therefore re-verified against primary sources — the vendored SDK
and the archived specs — before being accepted here. The zero-defend outcome reflects genuine
defects in the original artifacts, not deference.

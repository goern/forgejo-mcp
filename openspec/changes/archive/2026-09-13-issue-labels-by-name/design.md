# SPDX-License-Identifier: GPL-3.0-or-later

## Context

Label assignment in `operation/issue/issue.go` predates the label tooling around it. `AddIssueLabelsFn` and `RemoveIssueLabelsFn` split their `labels` CSV and call `strconv.ParseInt` on every token, so only numeric ids reach the API. `CreateIssueFn` builds a `CreateIssueOption` with `Title` and `Body` and nothing else. `UpdateIssueFn` already handles `assignees`, `milestone`, and the mutually exclusive `due_date` / `clear_due_date` pair, so the argument shapes this change adds to `create_issue` are copies, not inventions.

Three archived or in-flight changes deliberately left this alone. `list-milestones-labels` added `list_repo_labels` so that an agent could map names to ids itself; `add-org-label-support` extended that to org labels for the same reason; `label-crud` states in both its proposal and its Non-Goals that the list and assignment tools are unchanged. This change is the follow-up none of them made.

Behaviour of the pinned Forgejo (16.0.3) was measured, not assumed, before the design was fixed. The measurements are in `proposal.md` § Why.

## Goals / Non-Goals

**Goals**

- A name the caller can read in the web UI is accepted everywhere a label id is accepted today.
- One call creates a fully formed issue: title, body, labels, assignees, milestone.
- A whole label set can be replaced, and cleared, without deleting labels one at a time.
- A typo fails loudly, before any write.

**Non-Goals**

- New tools, or a single combined create-or-update tool. One verb per tool is the house shape.
- Forwarding names to Forgejo and letting the server resolve them (D2).
- Any change to label listing, label CRUD, or the `forgejo://` label resources.
- Caching the label catalogue between calls.

## Decisions

### D1: Resolve in the server, fail closed

`resolveIssueLabelIDs` turns the caller's CSV into `[]int64` before any write, and returns an error if it cannot. An unknown token is an error naming the token; it is never dropped.

This is the whole point of the change rather than a detail of it. Forgejo's own name path (`prepareForReplaceOrAdd` → `GetLabelIDsInRepoByNames`) silently ignores names that do not exist, and returns 200. A caller that asks for `dogfood,frction` gets one label applied, HTTP success, and no signal that half the request vanished. For an agent that is worse than a failure, because the wrong state looks like the right one. `GetLabelIDsInRepoByNames` is documented upstream as "it silently ignores label names that do not belong to the repository", so this is the API's contract, not a bug to report.

The same reasoning rejects partial success inside one call: resolution completes for every token, or nothing is sent.

### D2: Send ids on every path, never names

Forgejo accepts names on `POST` and `PUT /issues/{index}/labels` and on `DELETE /issues/{index}/labels/{identifier}`, but not on issue creation, where `CreateIssueOption.Labels` is `[]int64` and a string is a 422. Mixing ids and names in one `IssueLabelsOption` body is a 400.

Sending ids everywhere means one code path, one failure mode, and no branch that behaves differently depending on whether the caller happened to type a name. It also keeps every call on the pinned SDK — `AddIssueLabels`, `ReplaceIssueLabels`, `DeleteIssueLabel`, and `ClearIssueLabels` all take `[]int64` — so the change adds no raw-HTTP endpoint and no second transport to review.

The cost is a catalogue read per mutating call that mentions a label. That is the price of D1: to report an unknown name the server has to know which names exist.

### D3: The resolver enumerates both scopes in full, and does not reuse `list_repo_labels`

`ListRepoLabelsFn` pages repo and org labels with a single shared `page` / `limit`, because it is a browsing tool. A resolver cannot share that: a label on the second page would resolve as "unknown", which under D1 is a hard error. So `listAssignableLabels` pages each scope independently until a short page arrives.

Enumeration is capped at `maxLabelCatalogPages` (20 pages of 100, so 2000 labels per scope). Beyond the cap the resolver errors and tells the caller to pass numeric ids, which need no catalogue. An unbounded `for` loop over a server that ignores paging would hang the tool; a cap turns that into a message. The cap is internal — it bounds a read the caller never sees, so the output-bounding contract in `docs/design/output-bounding.md` does not apply (that contract governs response size, and every tool here responds with one issue).

Org labels come from the existing `fetchOrgLabels`, which already maps 404 to empty for user-owned repositories and surfaces 401/403 as `forgejo.ErrUnauthorized`. A user-owned repository therefore resolves against repo labels alone, with no special case in the resolver.

### D4: Names win over ids, and an ambiguous name is an error

A token is matched as a name first. Only if no label carries that name, and the token parses as an integer, and a label with that id exists in the catalogue, is it treated as an id. A label named `123` is reachable; an id typed as `123` still works. The alternative — try integer first — would make a label named `123` unreachable through a tool whose documented input is names.

Requiring the numeric id to be present in the catalogue is deliberate. It costs nothing (the catalogue is already loaded), and it turns a stale id into the same clear error as a stale name instead of a 404 from the API, or worse, a successful call against a label that belongs to a different repository.

If a name matches in both the repo and the org scope, the resolver errors and names both scopes and both ids. Repo and org labels share an integer space but are distinct rows, and `list_repo_labels` already returns a `scope` field precisely because the two can collide. Guessing one would silently apply the wrong label; asking for the id is unambiguous and the caller can get it from `list_repo_labels`.

Duplicate tokens collapse to one id, so `bug,bug` sends `[7]`. Forgejo's `In("id", ids)` would collapse them anyway; doing it here keeps the request body honest about what was asked.

### D5: A provided-but-empty label argument is an error, except for `set_labels`

The key's presence is what switches the feature on, following `assigneesProvided` in `UpdateIssueFn`. A `labels` key that yields no tokens after trimming is an error with no write: the caller asked for an operation and named nothing, and silently doing nothing is the failure mode D1 exists to prevent.

`set_labels` is the documented exception: empty means "clear every label", which is a real, explicit operation. This mirrors `update_issue`'s own `assignees` ("Pass an empty string to clear all assignees") and `set_repo_topics` from the `repo-topics` change. Clearing takes `ClearIssueLabels` (`DELETE /issues/{index}/labels`) and reads no catalogue, since there is nothing to resolve.

### D6: `set_labels` lives on `update_issue`, and is a second request

`EditIssueOption` has no `labels` field. A `labels` key on the PATCH is accepted and ignored by Forgejo, which is exactly the silent no-op an agent cannot detect. So `set_labels` is `PUT /issues/{index}/labels` via `ReplaceIssueLabels`, issued after the PATCH.

It belongs on `update_issue` rather than a new `replace_issue_labels` tool because replacing the label set is updating the issue, and because the tool list is already long enough that some clients truncate it. Incremental add and remove keep their own tools; `update_issue` deliberately does not grow `add_labels` / `remove_labels` aliases for operations that already have a home.

Ordering and the returned object:

- If any PATCH field is present, `EditIssue` runs first, then the label mutation, then `GetIssue`, so the returned issue shows the new labels rather than the pre-mutation set `EditIssue` returned.
- If `set_labels` is the only argument, no PATCH is sent. An empty PATCH is a wasted round trip, and `updated_at` should not move because a caller changed labels.
- With no `set_labels` at all, the handler does exactly what it does today: one PATCH, return its response. An `update_issue` call that names no label argument must not read the catalogue or issue a second request.

### D7: `create_issue` mirrors `update_issue`'s argument shapes

`labels` and `assignees` are CSV strings; `milestone` is a numeric id as a string, parsed with `strconv.ParseInt` and rejected with the same message `update_issue` uses. Mirroring is worth more than improving the shapes here: an agent that learned `update_issue` should not have to learn a second convention, and a divergence would be a permanent trap.

Labels resolve before the POST, so an unknown name means no issue is created. The alternative (create, then apply labels) would leave a real, unlabelled issue behind after a typo, and would need a second call for the common case.

## Risks / Trade-offs

- **An extra read per labelled call.** Two GETs (repo, org) precede an add, remove, replace, or labelled create. Accepted: it is what D1 buys, it only happens when the caller mentions a label, and the catalogue is not fetched when no label argument is present.
- **A 2000-label repository cannot resolve names.** Accepted, and the error says to pass ids. No real repository is near the cap, and the alternative is an unbounded loop.
- **Ambiguity is an error rather than a preference.** A repo label and an org label with the same name force the caller to pass an id. Accepted: silently preferring repo scope would apply a different label than the one the caller read in the org settings, and no error would ever be seen.
- **Error text changes for id-only callers.** `add_issue_labels` with `not-a-number` used to say "labels must be numeric IDs"; it now says the label is unknown, which is true and more useful. Numeric behaviour is unchanged.

## Migration Plan

Additive. Every existing call keeps its meaning: numeric CSVs resolve to the same ids, `create_issue` without the new keys sends the same body, and `update_issue` without `set_labels` sends the same single PATCH. Clients that never pass a name never notice, and nothing is removed.

## Open Questions

None. The three behaviours this design turns on (names accepted on assignment, ids only on create, `labels` ignored on PATCH) were each confirmed against the pinned Forgejo 16.0.3 before the design was written, and the resolution rules above are decided rather than deferred.

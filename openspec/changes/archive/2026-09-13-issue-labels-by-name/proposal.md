# SPDX-License-Identifier: GPL-3.0-or-later

## Why

An agent that knows a label as `dogfood` cannot apply it. `add_issue_labels` and `remove_issue_labels` parse their `labels` CSV with `strconv.ParseInt` and reject anything else, so the caller must first run `list_repo_labels`, map names to numeric ids, and only then mutate. The `add_issue_labels` description says "Labels to add (comma-separated)" while the handler demands ids, so the failure arrives as `invalid label ID 'dogfood'` after the call.

`create_issue` accepts only `title` and `body`. Labelling a new issue therefore costs three calls (list, create, add) and leaves a window where the issue exists unlabelled. `update_issue` already accepts `assignees` and `milestone`, but `create_issue` accepts neither, even though `CreateIssueOption` carries both.

There is no replace-all for labels. `EditIssueOption` has no `labels` field, so a `labels` key on an `update_issue` PATCH is silently ignored by Forgejo; the SDK's `ReplaceIssueLabels` (`PUT /issues/{index}/labels`) is not exposed by any tool.

Verified against Forgejo 16.0.3: `POST|PUT /repos/{owner}/{repo}/issues/{index}/labels` accept names or ids but **not both in one body** (400), and unknown names are dropped silently with 200. `POST /repos/{owner}/{repo}/issues` accepts label ids only — a string name is 422 `cannot unmarshal string into CreateIssueOption.labels of type int64`. A `labels` key on `PATCH /issues/{index}` is accepted and ignored.

## What Changes

- `create_issue` gains optional `labels` (CSV of names or ids), `assignees` (CSV), and `milestone` (numeric id), mirroring the argument shapes `update_issue` already uses.
- `add_issue_labels` and `remove_issue_labels` accept label **names** as well as numeric ids; their descriptions stop promising a format the handler rejects.
- `update_issue` gains `set_labels`: replace the issue's whole label set. An empty CSV clears every label, matching the `assignees` and `set_repo_topics` precedent.
- Names are resolved to ids in the server, not forwarded to Forgejo. An unknown name is an error with no upstream write, rather than Forgejo's silent drop. A name that exists in both the repo and the org scope is an error naming both, because the two id spaces are distinct.
- No new tools. No change to `list_repo_labels`, `list_org_labels`, or the label CRUD family.

## Capabilities

### New Capabilities

- `issue-labels-by-name`: name-or-id resolution for issue label assignment, the `labels`, `assignees`, and `milestone` arguments on `create_issue`, and `set_labels` on `update_issue`.

### Modified Capabilities

- None. `list-milestones-labels`, `list_org_labels`, and `label-crud` each scoped themselves to listing and to label lifecycle and declared the assignment tools unchanged; this change is the follow-up that makes their "list first to get ids" guidance optional rather than mandatory. No archived requirement is restated or weakened.

## Impact

- **Affected code**: `operation/issue/issue.go` (tool schemas and the four handlers, plus a resolver), `operation/issue/issue_test.go`, `README.md`, `AGENTS.md`, `extension/manifest.json`.
- **APIs / SDK**: existing pinned `forgejo-sdk/v3` methods only — `CreateIssue`, `AddIssueLabels`, `DeleteIssueLabel`, `ReplaceIssueLabels`, `ClearIssueLabels`, `ListRepoLabels`; org labels through the existing `fetchOrgLabels` raw-HTTP helper. No new dependency, no new raw-HTTP endpoint.
- **Output bounding**: exempt. Every tool here returns one issue, a fixed-shape object whose size does not grow with repository data. The label catalogue the resolver reads is internal and fully enumerated by design (see `design.md` D3); it is never returned to the caller.
- **Back-compat**: numeric ids keep working everywhere they worked before. The only behaviour change for an id-only caller is the error text when a token is neither a known name nor a known id.

## Out of Scope

- `due_date` on `create_issue`. `CreateIssueOption` carries `Deadline`, but the pair `due_date` / `clear_due_date` on `update_issue` has semantics worth mirroring deliberately in their own change.
- Milestone by name. `update_issue` takes a numeric milestone id today; `create_issue` matching it keeps the two consistent. Name resolution for milestones is a separate change.
- Creating labels that do not exist. `create_repo_label` and `create_org_label` already do that, and a typo must not silently create a label.
- The deprecated singular `assignee` on `create_issue`. `update_issue` carries it for compatibility; a new argument list should not add a form the API documents as deprecated.
- Issue types, pinning, and `exclusive` / `is_archived` label fields.

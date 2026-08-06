# Draft upstream issue — forgejo-sdk v3

**Status:** draft only. Not filed. A human files this against the upstream
`forgejo-sdk` tracker; no automated tool was used to create it.

**Target:** `codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3` `v3.0.0`

## Title

`ListIssueOption.QueryEncode` sends `MentionedBy` under the `team` query key

## Body

`ListIssueOption.QueryEncode` (`issue.go:150`) contains:

```go
if len(opt.Team) > 0 {
	query.Add("team", opt.MentionedBy)
}
```

This is guarded by `opt.Team` but writes `opt.MentionedBy`'s value under the
`team` query key. Any caller who sets `Team` to filter issues by team gets
`mentioned_by`-filtered results instead, silently, with no error.

Separately, `MentionedBy` is also correctly emitted under its own
`mentioned_by` key a few lines above (`issue.go:143-145`), so setting
`MentionedBy` without `Team` works as documented. The bug only fires when a
caller sets `Team`.

Also worth folding into the same report: the doc comment on `ListIssues`
(`issue.go:156`) — "ListIssues returns all issues assigned the authenticated
user" — is stale. `QueryEncode` emits no assignment-scoping filter unless the
caller explicitly sets `AssignedBy` (`issue.go:140-142`); a bare call searches
every repo the token can read.

**Suggested fix:** `query.Add("team", opt.Team)`.

**Discovered while:** implementing `search_issues` in `forgejo-mcp`
(`goern/forgejo-mcp` `add-search-issues-tool`). The tool deliberately does not
expose a `team` parameter until this is fixed upstream — see
`openspec/changes/add-search-issues-tool/design.md` Decision 5.

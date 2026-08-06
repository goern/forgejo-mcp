# Repo-scoped issue listing validation (list_repo_issues)

*2026-08-05T13:20:27Z by Showboat dev*
<!-- showboat-id: 7c004978-bd83-4de1-8e59-8c55d9963c90 -->

*Captured: 2026-08-05 via Showboat dev*
<!-- captured-for: PR pending -->
<!-- captured-at: 2026-08-05 -->
<!-- captured-against: worktree-x8v (72ed65cf33fc7e7c99842e8fca09ee9423e212ca) -->

Proves the `issue-listing-validation` capability — spec [`spec.md`](./spec.md) (change `add-search-issues-tool`), issue [`#452`](https://git.b4mad.industries/agentic-forges/forgejo-mcp/issues/452): an empty `repo` used to reach the SDK path guard and surface `path segment [1] is empty`, an opaque dead end. This tightens `list_repo_issues` to validate before calling upstream.

Captured live against `https://git.b4mad.industries/`, using the `--cli` structured-output surface. Read-only: every command below is a `list_repo_issues` call; no issue, PR, repo, or webhook is created, edited, or deleted.

## Replay setup

```bash
export FORGEJO_MCP_BIN="${FORGEJO_MCP_BIN:-./forgejo-mcp}"   # local build of this branch, run `make build` first
# FORGEJO_URL and FORGEJO_ACCESS_TOKEN must be exported in the shell; a read-only token is enough.
```

## Scenario: Empty repo argument

Spec: *`list_repo_issues` called with a non-empty `owner` and an empty `repo`* → *the error names `repo` as required, mentions `search_issues` as the tool for owner-wide listing, and does NOT contain the phrase `path segment`.* This is the exact case from issue `#452`: an agent reached for `list_repo_issues` with an org name in `owner` and nothing in `repo`.

```bash
echo "error message:"
${FORGEJO_MCP_BIN:-./forgejo-mcp} --cli list_repo_issues --args '{"owner":"agentic-forges","repo":""}' 2>&1 | grep -o "repo is required.*" | head -1
echo "path segment occurrences: $(${FORGEJO_MCP_BIN:-./forgejo-mcp} --cli list_repo_issues --args '{"owner":"agentic-forges","repo":""}' 2>&1 | grep -c "path segment")"

```

```output
error message:
repo is required; for owner-wide listing across repositories use search_issues instead
path segment occurrences: 0
```

## Scenario: Empty owner argument

Spec: *`list_repo_issues` called with an empty `owner`* → *the error names `owner` as required, and no upstream request is made.*

```bash
${FORGEJO_MCP_BIN:-./forgejo-mcp} --cli list_repo_issues --args '{"owner":"","repo":"forgejo-mcp"}' 2>&1 | grep -o "owner is required.*" | head -1

```

```output
owner is required
```

## Scenario: Well-formed call unaffected

Spec: *`list_repo_issues` called with non-empty `owner` and `repo`* → *it behaves exactly as before this change.* Still returns the pre-existing bare JSON array (no envelope) — the shape `search-issues.demo.md` deliberately does not adopt for this tool.

```bash
${FORGEJO_MCP_BIN:-./forgejo-mcp} --cli list_repo_issues --args '{"owner":"agentic-forges","repo":"forgejo-mcp","limit":3}' --output json 2>/dev/null \
  | jq -r '.[0].text | fromjson | .Result | (type), (length), (.[] | {number, title})'

```

```output
array
3
{
  "number": 457,
  "title": "chore(deps): update registry.access.redhat.com/hi/go docker tag to v1.26.5"
}
{
  "number": 434,
  "title": "[Feature Request] Add ability to whitelist tools"
}
{
  "number": 432,
  "title": "Dependency Dashboard"
}
```

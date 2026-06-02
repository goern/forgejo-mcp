# Branch protection management (uc6)

*2026-06-02T14:30:58Z by Showboat 0.6.1*
<!-- showboat-id: 7b514f84-82f9-4e11-a050-68eb72df0e9d -->

*Captured: 2026-06-02 via Showboat 0.6.1*
<!-- captured-for: PR #196 -->
<!-- captured-at: 2026-06-02 -->
<!-- captured-against: 0826172 (feat/branch-protection-impl-uc6) -->

Proves the `branch-protection` capability — change [`branch-protection-management`](./specs/branch-protection/spec.md), issue `forgejo-mcp-uc6` (discovered from `forgejo-mcp-f6h`: a repo with no protection let Renovate automerge before CI was green).

> **Token-free demo.** Reading or writing real branch protection requires a repo-admin token, and the server, like any client, must not leak it. This demo deliberately uses **no token and no live instance**: it proves the surface through the CLI tool registry and the validation path, and proves behaviour through the test suite, which exercises the exact Forgejo HTTP round-trips against an in-process `httptest` server. No secrets appear anywhere in this file.

## Replay setup

```bash
export FORGEJO_MCP_BIN="${FORGEJO_MCP_BIN:-./forgejo-mcp}"   # local build of this branch
# All commands below run from the repo root and need NO token / NO network.
```

## The five tools are registered

The server exposes the branch-protection CRUD tools (token-free introspection via the CLI tool registry).

```bash
"${FORGEJO_MCP_BIN:-./forgejo-mcp}" --cli list 2>/dev/null | grep branch_protection
```

```output
  create_branch_protection                 Create a branch protection rule (e.g. require status checks before merge)
  delete_branch_protection                 Delete a branch protection rule by name
  edit_branch_protection                   Edit a branch protection rule. Only fields you pass are changed; omitted fields are left untouched.
  get_branch_protection                    Get a single branch protection rule by name
  list_branch_protections                  List a repository's branch protection rules (bounded by page/limit)
```

## Scenario: Create requires a branch name

Spec: *create_branch_protection ... SHALL return an error result and SHALL NOT call Forgejo* when `branch_name` is missing. The guard runs before any client/network call, so this is reproducible with no token and no URL.

```bash
"${FORGEJO_MCP_BIN:-./forgejo-mcp}" -debug=false --cli create_branch_protection --args '{"owner":"goern","repo":"forgejo-mcp"}' 2>&1 | grep -o "branch_name is required" | head -1
```

```output
branch_name is required
```

## Scenario coverage via the executable spec

Each spec scenario is a test against an in-process `httptest` Forgejo (real request/response, no network, no token). The names map 1:1 to the spec: `status_check_contexts` round-trip, edit PATCH null-safety, list/collection bounding + truncation sentinel, get/collection 404 → resource error, single-resource happy path, malformed URI → invalid-params, and the slash-glob rule URI.

```bash
go test -v -run "BranchProtection|ParseBranchProtection|SplitContexts" ./operation/branchprotection/ ./operation/resource/ 2>&1 | grep -E "^(=== RUN|--- PASS|--- FAIL|PASS|FAIL|ok)" | sed -E "s#\t# #g"
```

```output
=== RUN   TestListBranchProtectionsFn
--- PASS: TestListBranchProtectionsFn (0.00s)
=== RUN   TestGetBranchProtectionFn_OK
--- PASS: TestGetBranchProtectionFn_OK (0.00s)
=== RUN   TestGetBranchProtectionFn_NotFound
--- PASS: TestGetBranchProtectionFn_NotFound (0.00s)
=== RUN   TestCreateBranchProtectionFn_StatusCheckRoundTrip
--- PASS: TestCreateBranchProtectionFn_StatusCheckRoundTrip (0.00s)
=== RUN   TestCreateBranchProtectionFn_MissingBranchName
--- PASS: TestCreateBranchProtectionFn_MissingBranchName (0.00s)
=== RUN   TestEditBranchProtectionFn_OnlyPassedFields
--- PASS: TestEditBranchProtectionFn_OnlyPassedFields (0.00s)
=== RUN   TestEditBranchProtectionFn_ContextsRoundTrip
--- PASS: TestEditBranchProtectionFn_ContextsRoundTrip (0.00s)
=== RUN   TestDeleteBranchProtectionFn_OK
--- PASS: TestDeleteBranchProtectionFn_OK (0.00s)
=== RUN   TestSplitContexts
--- PASS: TestSplitContexts (0.00s)
=== RUN   TestBranchProtectionsResource_HappyPath
--- PASS: TestBranchProtectionsResource_HappyPath (0.00s)
=== RUN   TestBranchProtectionsResource_Truncation
--- PASS: TestBranchProtectionsResource_Truncation (0.00s)
=== RUN   TestBranchProtectionsResource_NotFound
--- PASS: TestBranchProtectionsResource_NotFound (0.00s)
=== RUN   TestBranchProtectionResource_HappyPath
--- PASS: TestBranchProtectionResource_HappyPath (0.00s)
=== RUN   TestBranchProtectionResource_MalformedURI
--- PASS: TestBranchProtectionResource_MalformedURI (0.00s)
PASS
ok   codeberg.org/goern/forgejo-mcp/v2/operation/branchprotection (cached)
=== RUN   TestParseBranchProtections
--- PASS: TestParseBranchProtections (0.00s)
=== RUN   TestParseBranchProtections_Invalid
--- PASS: TestParseBranchProtections_Invalid (0.00s)
=== RUN   TestParseBranchProtection
--- PASS: TestParseBranchProtection (0.00s)
=== RUN   TestParseBranchProtection_GlobRuleWithSlash
--- PASS: TestParseBranchProtection_GlobRuleWithSlash (0.00s)
=== RUN   TestParseBranchProtection_Invalid
--- PASS: TestParseBranchProtection_Invalid (0.00s)
PASS
ok   codeberg.org/goern/forgejo-mcp/v2/operation/resource (cached)
```

## What this demo does not show (by design)

A live `create_branch_protection` / `list_branch_protections` against a real repo is omitted: it needs a repo-admin token, and showing it risks leaking that token in CLI debug output. The `httptest` suite above drives the identical SDK calls and asserts the exact request bodies (e.g. `enable_status_check: true` + the `status_check_contexts` list) and responses, so the contract is proven without a secret. To run it live yourself: `forgejo-mcp --cli list_branch_protections --args '{"owner":"...","repo":"..."}'` with `-url`/`-token` set in your shell — never on the command line of a shared session.

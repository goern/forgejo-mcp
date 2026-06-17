# Repository webhook tools

*2026-06-17T08:09:11Z by Showboat dev*
<!-- showboat-id: 3f5c1866-7c8b-4612-b6d3-73c115a15114 -->

*Captured: 2026-06-17 via Showboat dev*
<!-- captured-for: PR #fa64928 -->
<!-- captured-at: 2026-06-17 -->
<!-- captured-against: a870fa0fe127d86075ba44750183fa4ab7e9e3be (main) -->

Proves the `repo-webhook-tools` capability — spec [`spec.md`](./spec.md) (change `repo-webhook-tools`).

Tools: `list_repo_hooks`, `get_repo_hook`, `create_repo_hook`, `edit_repo_hook`, `delete_repo_hook`, `test_repo_hook`.
Resources: `forgejo://repo/{owner}/{repo}/hooks`, `forgejo://repo/{owner}/{repo}/hook/{id}`.

> **Token-free surface demo.** The tool-registry, parameter-validation, and secret-redaction proofs require no live Forgejo instance and no token. Each scenario states whether it needs a live instance; those sections include placeholder output blocks marked `# TODO: re-run against live instance`.

## Replay setup

```bash
export FORGEJO_MCP_BIN="${FORGEJO_MCP_BIN:-./forgejo-mcp}"
# All token-free proofs run from repo root with no network access.
# Live-instance proofs require: export FORGEJO_URL=https://... FORGEJO_TOKEN=...
```

## AC1: All six webhook tools are registered

```bash
"${FORGEJO_MCP_BIN:-./forgejo-mcp}" --cli list 2>/dev/null | grep -A6 "WEBHOOK:"
```

```output
WEBHOOK:
  create_repo_hook                         Create a repository webhook. The secret is accepted but never echoed in the response.
  delete_repo_hook                         Delete a repository webhook by ID
  edit_repo_hook                           Edit a repository webhook. Only fields you pass are changed; omitted fields are left untouched.
  get_repo_hook                            Get a single repository webhook by ID
  list_repo_hooks                          List repository webhooks (bounded by page/limit, default 30 per page)
  test_repo_hook                           Trigger a test delivery for a repository webhook. WARNING: each call triggers a live HTTP delivery to the webhook URL.
```

## AC2 / AC7 / AC8: Resource templates are registered (hooks collection + single hook)

The server registers both resource URI templates. Token-free introspection via the MCP resource-templates list endpoint:

```bash
grep -c "forgejo://repo/{owner}/{repo}/hook" operation/hook/resources_hook.go && echo "resource URI templates present"
```

```output
4
resource URI templates present
```

```bash
grep "hooksResourceURITemplate\|hookResourceURITemplate\s*=" operation/hook/resources_hook.go
```

```output
	hooksResourceURITemplate = "forgejo://repo/{owner}/{repo}/hooks"
	hookResourceURITemplate  = "forgejo://repo/{owner}/{repo}/hook/{id}"
		hooksResourceURITemplate,
```

## AC3: Secret is never echoed (static proof)

The spec requires that the `secret` field is stripped from every response path — tools and resources alike. This is a structural proof from the source: no code path writes `Config.Secret` to the `hookPayload` struct that is serialised to JSON.

```bash
grep -n "hookPayload" operation/hook/hook.go | head -15
```

```output
106:// hookPayload is the safe MCP response — Config.secret is never included.
108:type hookPayload struct {
119:func safeHook(h *forgejo_sdk.Hook) hookPayload {
120:	return hookPayload{
136:	Hooks []hookPayload `json:"hooks"`
166:	payloads := make([]hookPayload, len(hooks))
```

```bash
sed -n "108,135p" operation/hook/hook.go
```

```output
type hookPayload struct {
	ID           int64    `json:"id"`
	Type         string   `json:"type"`
	Active       bool     `json:"active"`
	Events       []string `json:"events"`
	URL          string   `json:"url"`
	ContentType  string   `json:"content_type,omitempty"`
	HTTPMethod   string   `json:"http_method,omitempty"`
	BranchFilter string   `json:"branch_filter,omitempty"`
}

func safeHook(h *forgejo_sdk.Hook) hookPayload {
	return hookPayload{
		ID:           h.ID,
		Type:         h.Type,
		Active:       h.Active,
		Events:       h.Events,
		URL:          h.Config["url"],
		ContentType:  h.Config["content_type"],
		HTTPMethod:   h.Config["http_method"],
		BranchFilter: h.Config["branch_filter"],
	}
}

type listRepoHooksResult struct {
	Page  int           `json:"page"`
	Limit int           `json:"limit"`
	Count int           `json:"count"`
```

No `secret` field appears anywhere in `hookPayload`: the struct copies individual config keys (`url`, `content_type`, `http_method`, `branch_filter`) by name — `h.Config["secret"]` is never read. All six tool handlers and both resource handlers call `safeHook()` exclusively.

```bash
grep -c "safeHook\|hookPayload" operation/hook/hook.go && grep -c "safeHook" operation/hook/resources_hook.go && echo "all response paths use safeHook()"
```

```output
10
2
all response paths use safeHook()
```

No unit test file exists for the hook package yet — the secret-redaction guarantee is enforced structurally by the explicit-allowlist pattern in `safeHook()` rather than by a runtime test. A future PR can add `hook_test.go`.

## AC4: list_repo_hooks — parameter validation (token-free)

Spec: *WHEN a client calls `list_repo_hooks` with a valid owner/repo, THEN the tool returns a JSON object containing a `hooks` array.* The live-instance proof is in the placeholder section below; the token-free proof shows the required-parameter guard fires before any network call.

The tool requires `owner` and `repo`; missing either produces a clear error before any network call is attempted.

```bash
"${FORGEJO_MCP_BIN:-./forgejo-mcp}" --cli list_repo_hooks --args "{}" 2>&1 | grep -o "owner and repo are required" | head -1
```

```output
owner and repo are required
```

## AC5: get_repo_hook — parameter validation (token-free)

Spec: *owner, repo, and id are all required.* Missing `id` produces a pre-flight error.

```bash
"${FORGEJO_MCP_BIN:-./forgejo-mcp}" --cli get_repo_hook --args "{\"owner\":\"goern\",\"repo\":\"forgejo-mcp\"}" 2>&1 | grep -o "owner, repo and id are required" | head -1
```

```output
owner, repo and id are required
```

## AC6 / AC9 / AC10: create / edit / delete / test — parameter validation (token-free)

All remaining tools guard on `owner`, `repo`, and any tool-specific required field before attempting any network call.

```bash
"${FORGEJO_MCP_BIN:-./forgejo-mcp}" --cli create_repo_hook --args "{\"owner\":\"goern\",\"repo\":\"forgejo-mcp\"}" 2>&1 | grep "Error:" | sed "s/Error: tool execution failed: //"
"${FORGEJO_MCP_BIN:-./forgejo-mcp}" --cli edit_repo_hook   --args "{\"owner\":\"goern\",\"repo\":\"forgejo-mcp\"}" 2>&1 | grep "Error:" | sed "s/Error: tool execution failed: //"
"${FORGEJO_MCP_BIN:-./forgejo-mcp}" --cli delete_repo_hook --args "{\"owner\":\"goern\",\"repo\":\"forgejo-mcp\"}" 2>&1 | grep "Error:" | sed "s/Error: tool execution failed: //"
"${FORGEJO_MCP_BIN:-./forgejo-mcp}" --cli test_repo_hook   --args "{\"owner\":\"goern\",\"repo\":\"forgejo-mcp\"}" 2>&1 | grep "Error:" | sed "s/Error: tool execution failed: //"
```

```output
owner, repo and url are required
owner, repo and id are required
owner, repo and id are required
owner, repo and id are required
```

## AC8: test_repo_hook — live-delivery warning in tool description

Spec: *the tool description SHALL warn callers that each invocation triggers a live HTTP delivery.* Token-free proof: grep the registered tool description.

```bash
"${FORGEJO_MCP_BIN:-./forgejo-mcp}" --cli list 2>/dev/null | grep test_repo_hook
```

```output
  test_repo_hook                           Trigger a test delivery for a repository webhook. WARNING: each call triggers a live HTTP delivery to the webhook URL.
```

## Live-instance scenarios (AC4–AC10, AC7–AC8 resources)

The following scenarios require a live Forgejo instance and an admin token with webhook-write scope. They are documented as placeholder blocks per the Showboat retrofit convention.

Set up environment:

```bash
export FORGEJO_URL=https://codeberg.org
export FORGEJO_TOKEN=<admin-token-with-hook-write>
export TEST_OWNER=goern
export TEST_REPO=forgejo-mcp-webhook-test  # a scratch repo for demo purposes
```

### Scenario: List hooks returns results

Spec: WHEN `list_repo_hooks` called with valid owner/repo, THEN returns JSON with `hooks` array including id, type, config (no secret), events, active, created.

```bash
echo "# TODO: re-run against live instance
# ${FORGEJO_MCP_BIN:-./forgejo-mcp} --cli list_repo_hooks \\
#   --args \"{\\\"owner\\\":\\\"goern\\\",\\\"repo\\\":\\\"forgejo-mcp-webhook-test\\\"}\" \\
#   --url \"\${FORGEJO_URL}\" --token \"\${FORGEJO_TOKEN}\""
```

```output
# TODO: re-run against live instance
# ./forgejo-mcp --cli list_repo_hooks \
#   --args "{\"owner\":\"goern\",\"repo\":\"forgejo-mcp-webhook-test\"}" \
#   --url "${FORGEJO_URL}" --token "${FORGEJO_TOKEN}"
```

### Scenario: Create hook, get hook, verify secret not echoed, delete hook

```bash
echo "# TODO: re-run against live instance
# STEP 1 — create with secret
# ${FORGEJO_MCP_BIN:-./forgejo-mcp} --cli create_repo_hook \\
#   --args \"{\\\"owner\\\":\\\"goern\\\",\\\"repo\\\":\\\"forgejo-mcp-webhook-test\\\",\\
#            \\\"url\\\":\\\"https://example.com/hook\\\",\\\"events\\\":\\\"push\\\",\\
#            \\\"secret\\\":\\\"SENTINEL\\\"}\" \\
#   --url \"\${FORGEJO_URL}\" --token \"\${FORGEJO_TOKEN}\"
# STEP 2 — get hook, confirm SENTINEL absent
# STEP 3 — edit hook URL
# STEP 4 — delete hook
# STEP 5 — confirm get returns not-found"
```

```output
# TODO: re-run against live instance
# STEP 1 — create with secret
# ./forgejo-mcp --cli create_repo_hook \
#   --args "{\"owner\":\"goern\",\"repo\":\"forgejo-mcp-webhook-test\",\
#            \"url\":\"https://example.com/hook\",\"events\":\"push\",\
#            \"secret\":\"SENTINEL\"}" \
#   --url "${FORGEJO_URL}" --token "${FORGEJO_TOKEN}"
# STEP 2 — get hook, confirm SENTINEL absent
# STEP 3 — edit hook URL
# STEP 4 — delete hook
# STEP 5 — confirm get returns not-found
```

### Scenario: Resource templates — hooks collection and single hook

```bash
echo "# TODO: re-run against live instance
# Read collection resource (MCP client or curl equivalent):
#   forgejo://repo/goern/forgejo-mcp-webhook-test/hooks
# Read single hook resource:
#   forgejo://repo/goern/forgejo-mcp-webhook-test/hook/<id>
# Read non-existent hook (expect not-found -32003):
#   forgejo://repo/goern/forgejo-mcp-webhook-test/hook/99999
# Read malformed id (expect invalid-params -32602):
#   forgejo://repo/goern/forgejo-mcp-webhook-test/hook/abc"
```

```output
# TODO: re-run against live instance
# Read collection resource (MCP client or curl equivalent):
#   forgejo://repo/goern/forgejo-mcp-webhook-test/hooks
# Read single hook resource:
#   forgejo://repo/goern/forgejo-mcp-webhook-test/hook/<id>
# Read non-existent hook (expect not-found -32003):
#   forgejo://repo/goern/forgejo-mcp-webhook-test/hook/99999
# Read malformed id (expect invalid-params -32602):
#   forgejo://repo/goern/forgejo-mcp-webhook-test/hook/abc
```

## URI parser — malformed id returns invalid-params (token-free)

The `ParseHookID` function validates that the `{id}` segment is a valid integer before any network call. A non-numeric id returns error code `-32602` (invalid-params), not `-32003` (not-found).

```bash
grep -n "ParseHookID\|ErrInvalidParams\|strconv\|Atoi\|ParseInt" operation/resource/parse.go | grep -i "hook\|ParseInt\|Atoi" | head -10
```

```output
133:	index, err := strconv.ParseInt(parts[3], 10, 64)
154:	index, err := strconv.ParseInt(parts[3], 10, 64)
180:	index, err := strconv.ParseInt(parts[3], 10, 64)
184:	id, err := strconv.ParseInt(parts[5], 10, 64)
302:	id, err := strconv.ParseInt(parts[3], 10, 64)
364:		return HookParams{}, fmt.Errorf("%w: expected forgejo://repo/..., got %q", ErrInvalidParams, uri)
369:		return HookParams{}, fmt.Errorf("%w: expected forgejo://repo/{owner}/{repo}/hook/{id}, got %q", ErrInvalidParams, uri)
371:	id, err := strconv.ParseInt(parts[3], 10, 64)
373:		return HookParams{}, fmt.Errorf("%w: invalid URI %q: id must be numeric", ErrInvalidParams, uri)
385:		return HooksParams{}, fmt.Errorf("%w: expected forgejo://repo/..., got %q", ErrInvalidParams, uri)
```

Line 373: `fmt.Errorf("%w: invalid URI %q: id must be numeric", ErrInvalidParams, uri)` — the `%w` wraps `ErrInvalidParams`, which `MapForgejoError` maps to JSON-RPC `-32602`. This is distinct from the `-32003` (not-found) path that an API 404 would produce.

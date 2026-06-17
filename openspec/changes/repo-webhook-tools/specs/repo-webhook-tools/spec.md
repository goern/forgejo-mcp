## ADDED Requirements

### Requirement: List repository hooks
The server SHALL expose a `list_repo_hooks` MCP tool that returns a paginated list of all webhooks registered on a repository. The tool MUST accept `owner`, `repo`, `page` (default 1), and `limit` (default 30) parameters with no server-imposed ceiling (the tool is the unbounded enumeration path). The response MUST include a truncation sentinel naming the `list_repo_hooks` tool only when used from the resource path; the tool itself returns whatever the SDK page returns.

#### Scenario: List hooks returns results
- **WHEN** a client calls `list_repo_hooks` with a valid `owner`/`repo`
- **THEN** the tool returns a JSON object containing a `hooks` array with each hook's `id`, `type`, `config` (without `secret`), `events`, `active`, and `created` fields

#### Scenario: List hooks respects page and limit
- **WHEN** a client calls `list_repo_hooks` with `page=2` and `limit=10`
- **THEN** the tool returns at most 10 hooks from the second page

#### Scenario: List hooks truncation sentinel
- **WHEN** the repository has more hooks than the requested `limit`
- **THEN** the response includes `truncated: true` and a `list_tool: "list_repo_hooks"` sentinel signalling that more results exist; the sentinel does NOT report the total repository-wide hook count (it reflects the fetched window only)

#### Scenario: List hooks on repo with no hooks
- **WHEN** a client calls `list_repo_hooks` on a repository with zero hooks
- **THEN** the tool returns an empty `hooks` array and no truncation sentinel

---

### Requirement: Get single repository hook
The server SHALL expose a `get_repo_hook` MCP tool that returns a single webhook by its numeric `id`. The response MUST NOT include the `secret` field.

#### Scenario: Get hook by id
- **WHEN** a client calls `get_repo_hook` with a valid `owner`, `repo`, and `id`
- **THEN** the tool returns the hook's `id`, `type`, `config` (without `secret`), `events`, `active`, `branch_filter`, and `created` fields

#### Scenario: Get hook with unknown id
- **WHEN** a client calls `get_repo_hook` with an `id` that does not exist
- **THEN** the tool returns an MCP error with a not-found message

---

### Requirement: Create repository hook
The server SHALL expose a `create_repo_hook` MCP tool that registers a new webhook on a repository. The `secret` parameter MUST be accepted but MUST NOT be echoed in the response.

#### Scenario: Create hook with minimal params
- **WHEN** a client calls `create_repo_hook` with `owner`, `repo`, `url`, `type` (`forgejo`), and `events` (`["push"]`)
- **THEN** the tool creates the hook and returns the new hook object including its numeric `id`

#### Scenario: Create hook with all params
- **WHEN** a client calls `create_repo_hook` with all optional fields (`content_type`, `secret`, `http_method`, `active`, `branch_filter`)
- **THEN** the tool creates the hook with those settings and returns the hook object without the `secret` field

#### Scenario: Secret not echoed
- **WHEN** a hook is created with `secret="SENTINEL"` and then retrieved via every tool and both resource templates
- **THEN** the substring `SENTINEL` does NOT appear anywhere in any serialized response, including inside config maps, error messages, or any nested field

---

### Requirement: Edit repository hook
The server SHALL expose an `edit_repo_hook` MCP tool that partially updates an existing webhook. All patch fields are optional.

#### Scenario: Edit hook URL
- **WHEN** a client calls `edit_repo_hook` with a new `url`
- **THEN** the tool updates the hook URL and returns the updated hook object

#### Scenario: Edit hook events
- **WHEN** a client calls `edit_repo_hook` with a new `events` list
- **THEN** the tool updates the subscribed events and returns the updated hook object

---

### Requirement: Delete repository hook
The server SHALL expose a `delete_repo_hook` MCP tool that removes a webhook by `id`.

#### Scenario: Delete existing hook
- **WHEN** a client calls `delete_repo_hook` with a valid `id`
- **THEN** the tool deletes the hook and returns a success response

#### Scenario: Delete non-existent hook
- **WHEN** a client calls `delete_repo_hook` with an `id` that does not exist
- **THEN** the tool returns an MCP error with a not-found message

---

### Requirement: Test repository hook
The server SHALL expose a `test_repo_hook` MCP tool that triggers a test delivery for a webhook. The tool description SHALL warn callers that each invocation triggers a live HTTP delivery from the Forgejo server to the registered hook URL.

#### Scenario: Trigger test delivery
- **WHEN** a client calls `test_repo_hook` with a valid `owner`, `repo`, and `id`
- **THEN** the tool triggers a test delivery and returns `{"triggered": true}`

---

### Requirement: Hook collection resource template
The server SHALL register a `forgejo://repo/{owner}/{repo}/hooks{?page,limit}` resource template returning a bounded embedded list of hooks (cap `EmbeddedListCap` = 30). The resource MUST use `operation/resource.Bounded` for the truncation sentinel and MUST NOT include the `secret` field.

#### Scenario: Read hooks collection resource
- **WHEN** a client reads `forgejo://repo/{owner}/{repo}/hooks`
- **THEN** the resource returns a JSON object with a `hooks` array (up to 30 items) and optional truncation sentinel naming `list_repo_hooks`

#### Scenario: Read hooks collection with pagination params
- **WHEN** a client reads `forgejo://repo/{owner}/{repo}/hooks?page=2&limit=10`
- **THEN** the resource returns at most 10 hooks from the second page

---

### Requirement: Single hook resource template
The server SHALL register a `forgejo://repo/{owner}/{repo}/hook/{id}` resource template returning a single hook. The resource MUST NOT include the `secret` field and MUST use `operation/resource.MapForgejoError` for error mapping.

#### Scenario: Read single hook resource
- **WHEN** a client reads `forgejo://repo/{owner}/{repo}/hook/42`
- **THEN** the resource returns the hook JSON for hook id 42 without the `secret` field

#### Scenario: Read single hook resource with unknown id
- **WHEN** a client reads `forgejo://repo/{owner}/{repo}/hook/99999` and the hook does not exist
- **THEN** the resource returns an MCP not-found error via `MapForgejoError`

#### Scenario: Read single hook resource with malformed id
- **WHEN** a client reads `forgejo://repo/{owner}/{repo}/hook/abc` (non-numeric id)
- **THEN** the resource returns an invalid-params error (-32602) from the URI parser, NOT a not-found error (-32003)

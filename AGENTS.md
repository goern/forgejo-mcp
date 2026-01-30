# AGENTS.md

This file provides guidance to AI coding assistants (Claude Code, Cursor, etc.) when working with this repository.

For detailed developer documentation, see [DEVELOPER.md](DEVELOPER.md).

## Quick Reference

```bash
make build          # Build the binary (outputs ./forgejo-mcp)
make vendor         # Tidy and verify Go module dependencies
```

## Architecture Summary

```
main.go → cmd/cmd.go (CLI parsing) → operation/operation.go (tool registration) → operation/{domain}/*.go (tool handlers)
```

Key directories:
- `operation/` - MCP tool definitions and handlers by domain
- `pkg/forgejo/` - Singleton Forgejo SDK client wrapper
- `pkg/to/` - Response formatting helpers
- `pkg/params/` - Shared parameter descriptions

## Adding a New Tool

1. Create or modify a file in `operation/{domain}/`
2. Define tool with `mcp.NewTool()` and implement handler function
3. Register in the domain's `RegisterTool(s *server.MCPServer)` function
4. If new domain, import and call in `operation/operation.go`

See [DEVELOPER.md](DEVELOPER.md) for complete code examples and patterns.

## Blocked Features

Some features are blocked on upstream API/SDK support. See `docs/plans/` for:

- `wiki-support.md` - Wiki API (blocked on forgejo-sdk)
- `projects-support.md` - Projects/Kanban API (blocked on Gitea 1.26.0)

## Repository Labels

Labels for goern/forgejo-mcp on Codeberg:

| ID | Name | Color | Description |
|----|------|-------|-------------|
| 335058 | Kind/Feature | 0288d1 | New functionality |
| 335061 | Kind/Enhancement | 84b6eb | Improve existing functionality |
| 335091 | Status/Blocked | 880e4f | Something is blocking this issue or pull request |
| 335103 | Priority/Medium | e64a19 | The priority is medium |

### Usage with Codeberg MCP

When adding labels via the `mcp__codeberg__add_issue_labels` tool, use the numeric ID:

```
mcp__codeberg__add_issue_labels(
  owner: "goern",
  repo: "forgejo-mcp",
  index: <issue_number>,
  labels: "<label_id>"  # e.g., "335091" for Status/Blocked
)
```

## Landing the Plane (Session Completion)

**When ending a work session**, you MUST complete ALL steps below. Work is NOT complete until `git push` succeeds.

**MANDATORY WORKFLOW:**

1. **File issues for remaining work** - Create issues for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **PUSH TO REMOTE** - This is MANDATORY:
   ```bash
   git pull --rebase
   bd sync
   git push
   git status  # MUST show "up to date with origin"
   ```
5. **Clean up** - Clear stashes, prune remote branches
6. **Verify** - All changes committed AND pushed
7. **Hand off** - Provide context for next session

**CRITICAL RULES:**
- Work is NOT complete until `git push` succeeds
- NEVER stop before pushing - that leaves work stranded locally
- NEVER say "ready to push when you are" - YOU must push
- If push fails, resolve and retry until it succeeds

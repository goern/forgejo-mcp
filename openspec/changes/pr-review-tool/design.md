## Context

forgejo-mcp exposes PR review tools that are read-only: `list_pull_reviews`, `get_pull_review`, `list_pull_review_comments`. The forgejo-sdk v2 provides full write-side review methods (`CreatePullReview`, `SubmitPullReview`, `DismissPullReview`, `DeletePullReview`, `CreateReviewRequests`, `DeleteReviewRequests`) that have no corresponding MCP tools yet.

All existing tools live in `operation/pull/pull.go` (284 lines). The file handles both PR CRUD and review reads.

## Goals / Non-Goals

**Goals:**
- Expose all write-side PR review SDK methods as MCP tools
- Follow existing code patterns exactly (tool definition → handler → registration)
- Keep the codebase organized as the pull domain grows

**Non-Goals:**
- Modifying existing read-only review tools
- Adding review diff/patch viewing (not available in SDK)
- Implementing `UnDismissPullReview` (niche, can add later if requested)

## Decisions

### 1. Separate file for review-write tools

**Decision**: Add new tools in `operation/pull/review.go` rather than appending to `pull.go`.

**Rationale**: `pull.go` is already 284 lines with 7 tools. Adding 6 more tools (~180 lines of definitions + handlers) would push it past 450 lines. A separate file keeps the domain organized while staying in the same package — no import or registration changes needed beyond calling the new registration function from `pull.RegisterTool()` or adding a second call in `operation.go`.

**Alternative considered**: Append to `pull.go` — simpler but grows the file significantly. Since Go packages can span multiple files, splitting costs nothing.

### 2. Registration via a second function called from operation.go

**Decision**: Add `RegisterReviewTools(s *server.MCPServer)` in `review.go` and call it from `operation/operation.go` alongside the existing `pull.RegisterTool(s)`.

**Rationale**: Keeps registration explicit and follows the existing pattern where each domain registers its own tools. Avoids modifying `pull.RegisterTool()` which would blur the boundary.

### 3. CreatePullReview inline comments as JSON string parameter

**Decision**: Accept inline review comments as a JSON-encoded string parameter rather than a structured array.

**Rationale**: The MCP tool schema (`mcp.NewTool`) uses flat string/number/boolean parameters. There is no `mcp.WithArray()` or `mcp.WithObject()` helper. To pass `CreatePullReviewComment` structs (path, body, old/new line numbers), we accept a JSON string and deserialize it in the handler. This matches how other MCP servers handle complex nested inputs.

**Alternative considered**: Multiple separate parameters per comment field — doesn't support multiple comments per review.

### 4. Review state as string enum

**Decision**: Accept review state as a string parameter with values `APPROVED`, `REQUEST_CHANGES`, `COMMENT`.

**Rationale**: Maps directly to `forgejo_sdk.ReviewStateType`. The MCP tool description documents valid values. No conversion layer needed.

## Risks / Trade-offs

- **JSON comments parameter is less discoverable** → Mitigated by clear tool description documenting the expected JSON shape with an example.
- **No input validation beyond SDK errors** → Consistent with all existing tools in the codebase. The SDK returns clear error messages for invalid inputs.
- **File split adds a second registration call** → Trivial one-line addition in `operation.go`. Worth it for maintainability.

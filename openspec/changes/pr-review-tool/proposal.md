## Why

The forgejo-mcp server currently has read-only PR review tools (`list_pull_reviews`, `get_pull_review`, `list_pull_review_comments`) but no way to **submit, create, or manage** reviews. This was requested in [Codeberg issue #59](https://codeberg.org/goern/forgejo-mcp/issues/59) — users need the full review lifecycle so MCP clients can approve, request changes, and leave inline comments on pull requests. The forgejo-sdk v2 already exposes all the necessary methods; we just need tool wrappers.

## What Changes

- Add `create_pull_review` tool — create a new review with body, state (approve/request-changes/comment), and optional inline comments
- Add `submit_pull_review` tool — submit a pending review with a verdict
- Add `dismiss_pull_review` tool — dismiss a review with a reason
- Add `delete_pull_review` tool — delete a pending review
- Add `create_review_requests` tool — request reviews from specific users/teams
- Add `delete_review_requests` tool — cancel pending review requests

All tools are additive. No existing tools or behavior are modified.

## Capabilities

### New Capabilities
- `pr-review-write`: Write-side PR review operations (create, submit, dismiss, delete reviews; manage review requests). Wraps the forgejo-sdk methods `CreatePullReview`, `SubmitPullReview`, `DismissPullReview`, `DeletePullReview`, `CreateReviewRequests`, `DeleteReviewRequests`.

### Modified Capabilities
_(none — existing read-only review tools remain unchanged)_

## Impact

- **Code**: New tool definitions and handlers added to `operation/pull/pull.go` (or a new `operation/pull/review.go` file to keep the file manageable).
- **Params**: May add shared parameter descriptions to `pkg/params/params.go` (e.g., review state, review ID).
- **APIs**: Uses existing forgejo-sdk v2 methods — no SDK changes needed.
- **Risk**: Low. Purely additive, no breaking changes. All new tools follow the established handler pattern.

# Reply to Review Comment — Implementation Plan

**Issue**: [#98](https://codeberg.org/goern/forgejo-mcp/issues/98)
**Branch**: `feature/reply-to-review-comment`

## Summary

Add a `reply_to_review_comment` MCP tool that posts a threaded reply to a specific inline review comment on a pull request. This enables AI-assisted code review workflows where the PR author responds directly to each inline reviewer comment.

## Current State

- **forgejo-mcp** has 6 review tools in `operation/pull/review.go`: create, submit, dismiss, delete reviews; create/delete review requests
- **forgejo-sdk v2.2.1** has `ListPullReviewComments` (GET) but **no method to POST a reply** to a review comment
- The Forgejo REST API supports this via:
  ```
  POST /repos/{owner}/{repo}/pulls/{index}/reviews/{id}/comments
  ```
  with a JSON body containing `body` and (undocumented in SDK) a mechanism to thread replies

## Key Design Decision: SDK Gap

The forgejo-sdk does not expose a method for creating review comment replies. Two options:

### Option A: Raw HTTP call via SDK client (recommended)

Use the SDK client's underlying HTTP transport to make a direct API call. The SDK's `Client` type is based on a standard HTTP client, so we can construct the request ourselves while reusing auth and base URL.

**Pros**: No upstream dependency, ships immediately
**Cons**: Bypasses SDK validation; couples to raw API shape

### Option B: Upstream SDK contribution

Contribute a `CreatePullReviewCommentReply` method to forgejo-sdk, then use it.

**Pros**: Clean, idiomatic
**Cons**: Blocks on upstream review cycle (1-2 weeks minimum)

**Recommendation**: Start with Option A (raw HTTP) so the feature ships. File an upstream SDK issue in parallel. When the SDK adds native support, refactor to use it.

## Target State

### New Tool: `reply_to_review_comment`

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `owner` | string | yes | Repository owner |
| `repo` | string | yes | Repository name |
| `index` | number | yes | Pull request index |
| `review_id` | number | yes | Review ID |
| `comment_id` | number | yes | ID of the comment to reply to |
| `body` | string | yes | Reply body text |

**Returns**: The created `PullReviewComment` as JSON.

### Forgejo API Call

```
POST /repos/{owner}/{repo}/pulls/{index}/reviews/{review_id}/comments
Content-Type: application/json

{
  "body": "...",
  "reply_to": <comment_id>   // threads the reply under the original comment
}
```

> **Note**: The exact field name (`reply_to` vs `in_reply_to`) must be verified against the Forgejo API spec or by inspecting the Forgejo source. The Gitea API uses `in_reply_to`.

## Implementation Steps

### Part 1: Verify API Contract

1. **Check the Forgejo API spec** for `POST /repos/{owner}/{repo}/pulls/{index}/reviews/{id}/comments` — confirm the request body schema and the field name for threading replies
2. **Test manually** with `curl` against a live Forgejo instance to confirm behavior

### Part 2: Implement the Tool

#### 1. Add parameter descriptions in `pkg/params/params.go`

```go
CommentID = "The ID of the review comment to reply to"
```

(`ReviewID` and `ReviewBody` already exist.)

#### 2. Add tool definition and handler in `operation/pull/review.go`

```go
const ReplyToReviewCommentToolName = "reply_to_review_comment"

var ReplyToReviewCommentTool = mcp.NewTool(
    ReplyToReviewCommentToolName,
    mcp.WithDescription("Reply to an inline review comment on a pull request"),
    mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
    mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
    mcp.WithNumber("index", mcp.Required(), mcp.Description(params.PRIndex)),
    mcp.WithNumber("review_id", mcp.Required(), mcp.Description(params.ReviewID)),
    mcp.WithNumber("comment_id", mcp.Required(), mcp.Description(params.CommentID)),
    mcp.WithString("body", mcp.Required(), mcp.Description(params.ReviewBody)),
)
```

#### 3. Implement handler with raw HTTP call

Since the SDK lacks this method, the handler will need to make a direct HTTP POST. The implementation should:

- Construct the URL: `/repos/{owner}/{repo}/pulls/{index}/reviews/{review_id}/comments`
- Send JSON body: `{"body": "...", "in_reply_to": <comment_id>}`
- Parse the response as a `PullReviewComment`
- Use the SDK client's token and base URL for auth

The exact mechanism depends on what the SDK's `Client` type exposes. If it doesn't expose raw request methods, we may need a small helper in `pkg/forgejo/` that wraps `http.Client` with the same auth config.

#### 4. Register the tool

In `RegisterReviewTools()`:

```go
s.AddTool(ReplyToReviewCommentTool, ReplyToReviewCommentFn)
```

### Part 3: Documentation

- Add the new tool to the review tools table in `README.md`

## Open Questions

1. **Exact API field name**: Is it `reply_to` or `in_reply_to`? Must verify against Forgejo API spec.
2. **SDK raw HTTP access**: Does the SDK client expose a way to make arbitrary authenticated requests, or do we need to build our own `http.Request` with the token header?
3. **Response shape**: Does the POST return the created comment, or just a status? This determines the return value formatting.

## Risk Assessment

- **Low risk**: The tool is additive — no existing behavior changes
- **Medium risk**: Raw HTTP call bypasses SDK versioning checks; if Forgejo API changes, this breaks silently. Mitigated by eventually moving to SDK method.

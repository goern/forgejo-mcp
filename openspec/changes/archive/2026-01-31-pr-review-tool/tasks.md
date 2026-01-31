## 1. Shared Parameters

- [x] 1.1 Add shared parameter descriptions to `pkg/params/params.go` (review ID, review state enum, dismissal message)

## 2. Review Write Tools

- [x] 2.1 Create `operation/pull/review.go` with `RegisterReviewTools(s *server.MCPServer)` function
- [x] 2.2 Implement `create_pull_review` tool — define tool with owner/repo/index/body/state/comments params, handler parses JSON comments string into `[]CreatePullReviewComment`, calls `CreatePullReview`
- [x] 2.3 Implement `submit_pull_review` tool — define tool with owner/repo/index/review-id/body/state params, handler calls `SubmitPullReview`
- [x] 2.4 Implement `dismiss_pull_review` tool — define tool with owner/repo/index/review-id/message params, handler calls `DismissPullReview`
- [x] 2.5 Implement `delete_pull_review` tool — define tool with owner/repo/index/review-id params, handler calls `DeletePullReview`

## 3. Review Request Tools

- [x] 3.1 Implement `create_review_requests` tool — define tool with owner/repo/index/reviewers/team-reviewers params, handler calls `CreateReviewRequests`
- [x] 3.2 Implement `delete_review_requests` tool — define tool with owner/repo/index/reviewers/team-reviewers params, handler calls `DeleteReviewRequests`

## 4. Registration and Build

- [x] 4.1 Add `pull.RegisterReviewTools(s)` call in `operation/operation.go`
- [x] 4.2 Run `make build` and verify compilation succeeds

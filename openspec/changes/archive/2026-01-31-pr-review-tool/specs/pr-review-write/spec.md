## ADDED Requirements

### Requirement: Create a pull review
The system SHALL expose a `create_pull_review` MCP tool that creates a new review on a pull request. The tool SHALL accept owner, repo, PR index, body, and state (`APPROVED`, `REQUEST_CHANGES`, `COMMENT`) as parameters. The tool SHALL optionally accept a JSON-encoded string of inline comments, where each comment specifies a file path, body, and optionally old/new line numbers. The tool SHALL return the created review formatted via the standard response helpers.

#### Scenario: Create an approval review
- **WHEN** the tool is called with owner="org", repo="project", index=1, body="LGTM", state="APPROVED"
- **THEN** the system creates an approval review on PR #1 and returns the review details

#### Scenario: Create a review with inline comments
- **WHEN** the tool is called with state="REQUEST_CHANGES", body="Needs fixes", and comments=`[{"path":"main.go","body":"Fix this","newLineNum":10}]`
- **THEN** the system creates a review with the inline comment attached to line 10 of main.go

#### Scenario: Invalid state value
- **WHEN** the tool is called with state="INVALID"
- **THEN** the SDK returns an error and the tool propagates it to the caller

### Requirement: Submit a pending pull review
The system SHALL expose a `submit_pull_review` MCP tool that submits an existing pending review. The tool SHALL accept owner, repo, PR index, review ID, body, and state as parameters. The tool SHALL return the submitted review formatted via the standard response helpers.

#### Scenario: Submit a pending review as approved
- **WHEN** the tool is called with a valid review ID and state="APPROVED"
- **THEN** the system submits the pending review as an approval and returns the review details

### Requirement: Dismiss a pull review
The system SHALL expose a `dismiss_pull_review` MCP tool that dismisses an existing review. The tool SHALL accept owner, repo, PR index, review ID, and a dismissal message as parameters. The tool SHALL return the dismissed review formatted via the standard response helpers.

#### Scenario: Dismiss a review with reason
- **WHEN** the tool is called with a valid review ID and message="Outdated feedback"
- **THEN** the system dismisses the review with the given reason and returns the updated review details

### Requirement: Delete a pull review
The system SHALL expose a `delete_pull_review` MCP tool that deletes a pending review. The tool SHALL accept owner, repo, PR index, and review ID as parameters. The tool SHALL return a success confirmation.

#### Scenario: Delete a pending review
- **WHEN** the tool is called with a valid pending review ID
- **THEN** the system deletes the review and returns a success message

#### Scenario: Delete a non-pending review
- **WHEN** the tool is called with a review ID that is not in pending state
- **THEN** the SDK returns an error and the tool propagates it to the caller

### Requirement: Create review requests
The system SHALL expose a `create_review_requests` MCP tool that requests reviews from specific users and/or teams. The tool SHALL accept owner, repo, PR index, and lists of reviewer usernames and team names as parameters. The tool SHALL return the updated pull request details.

#### Scenario: Request reviews from users
- **WHEN** the tool is called with reviewers=["alice","bob"]
- **THEN** the system adds alice and bob as requested reviewers on the pull request

#### Scenario: Request reviews from a team
- **WHEN** the tool is called with team_reviewers=["backend-team"]
- **THEN** the system adds backend-team as a requested reviewer team on the pull request

### Requirement: Delete review requests
The system SHALL expose a `delete_review_requests` MCP tool that cancels pending review requests. The tool SHALL accept owner, repo, PR index, and lists of reviewer usernames and team names as parameters. The tool SHALL return the updated pull request details.

#### Scenario: Cancel review request for a user
- **WHEN** the tool is called with reviewers=["alice"]
- **THEN** the system removes alice from the requested reviewers on the pull request

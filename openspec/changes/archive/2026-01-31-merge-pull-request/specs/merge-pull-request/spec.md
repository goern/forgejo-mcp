## ADDED Requirements

### Requirement: Merge a pull request
The system SHALL expose a `merge_pull_request` MCP tool that merges a pull request. The tool SHALL accept owner, repo, and PR index as required parameters. The tool SHALL accept a required `style` parameter with values `merge`, `rebase`, `rebase-merge`, or `squash`. The tool SHALL accept optional parameters: `title` (merge commit title), `message` (merge commit message), `delete_branch_after_merge` (boolean), `force_merge` (boolean), and `merge_when_checks_succeed` (boolean). The tool SHALL call the Forgejo SDK `MergePullRequest` method and return a success message.

#### Scenario: Merge with default merge style
- **WHEN** the tool is called with owner="org", repo="project", index=1, style="merge"
- **THEN** the system merges PR #1 using a merge commit and returns a success message

#### Scenario: Squash merge with custom title and message
- **WHEN** the tool is called with style="squash", title="feat: add feature", message="Squashed commit details"
- **THEN** the system squash-merges the PR using the provided title and message

#### Scenario: Rebase merge
- **WHEN** the tool is called with style="rebase"
- **THEN** the system rebases the PR commits onto the base branch

#### Scenario: Merge and delete source branch
- **WHEN** the tool is called with style="merge", delete_branch_after_merge=true
- **THEN** the system merges the PR and deletes the head branch afterward

#### Scenario: Force merge bypassing checks
- **WHEN** the tool is called with style="merge", force_merge=true
- **THEN** the system merges the PR even if status checks have not passed

#### Scenario: Merge when checks succeed
- **WHEN** the tool is called with style="merge", merge_when_checks_succeed=true
- **THEN** the system schedules the PR to be merged once all status checks pass and returns a success message indicating the merge is scheduled

#### Scenario: Merge fails due to conflicts
- **WHEN** the tool is called on a PR that has merge conflicts
- **THEN** the SDK returns an error and the tool propagates it to the caller

#### Scenario: Merge an already-merged PR
- **WHEN** the tool is called on a PR that is already merged or closed
- **THEN** the SDK returns an error and the tool propagates it to the caller

#### Scenario: Invalid merge style
- **WHEN** the tool is called with style="invalid"
- **THEN** the SDK returns a validation error and the tool propagates it to the caller

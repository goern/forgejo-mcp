package params

// Shared parameter descriptions for MCP tools
// Following Anthropic's guideline: "be frugal with their use of tokens"
// Patterns follow official MCP server conventions (GitHub, MCP spec examples)
const (
	// Common repository parameters
	Owner = "Repository owner"
	Repo  = "Repository name"

	// Issue/PR parameters
	Index      = "Issue/PR index"
	IssueIndex = "Issue index"
	PRIndex    = "PR index"
	CommentID  = "Comment ID"
	Body       = "Content body"
	Title      = "Title"
	State      = "State"
	Labels     = "Label IDs"
	Milestone  = "Milestone ID"

	// Branch parameters
	Branch    = "Branch name"
	OldBranch = "Source branch"
	Head      = "Head branch"
	Base      = "Base branch"

	// File parameters
	FilePath      = "File path"
	Content       = "Content (plain text, will be base64-encoded automatically)"
	Message       = "Commit message"
	BranchName    = "Branch name"
	NewBranchName = "New branch name"
	Ref           = "Ref (branch/tag/commit)"
	SHA           = "SHA"

	// Pagination parameters
	Page  = "Page number (1-based)"
	Limit = "Page size"

	// Time parameters
	Since  = "After time (RFC3339)"
	Before = "Before time (RFC3339)"

	// Sort/filter parameters
	Sort    = "Sort order"
	Order   = "Order direction"
	Keyword = "Search keyword"

	// User/org parameters
	User = "Username"
	Org  = "Organization name"

	// Wiki parameters
	WikiTitle   = "Wiki page title"
	WikiContent = "Wiki page content"
	WikiPage    = "Wiki page name"

	// Review parameters
	ReviewID       = "Review ID"
	ReviewState    = "Review state (APPROVED, REQUEST_CHANGES, COMMENT)"
	ReviewBody     = "Review body/message"
	Reviewers      = "Reviewer usernames (comma-separated)"
	TeamReviewers  = "Team reviewer names (comma-separated)"
	DismissMessage = "Dismissal message"
	ReviewComments = `Inline comments as JSON array, e.g. [{"path":"file.go","body":"Fix this","new_position":10}]`

	// Actions parameters
	Workflow    = "Workflow file or ID (e.g. main.yml)"
	Inputs      = `Workflow inputs as JSON object (e.g. {"key": "value"})`
	Event       = "Filter by event type (e.g. push, pull_request, workflow_dispatch)"
	RunNumber   = "Filter by run number"
	HeadSHA     = "Filter by HEAD SHA"
	RunID       = "Run ID"
	JobID       = "Workflow job ID"
	Attempt     = "1-based job attempt; omit for the latest attempt"
	LogOffset   = "0-based byte offset; omit to return the tail of the log"
	LogMaxBytes = "Maximum log bytes to return (default 32768, maximum 262144)"
	Status      = "Filter by status (e.g. waiting, running, success, failure, cancelled)"

	// Misc parameters
	Description = "Description"
	Private     = "Private repo"

	// Attachment parameters
	AttachmentID       = "Attachment ID"
	AttachmentName     = "New name for the attachment"
	AttachmentContent  = "Base64-encoded file bytes to upload"
	AttachmentFilename = `Filename to associate with the uploaded attachment (e.g. "requirements.pdf")`
	AttachmentMIME     = "MIME type hint for uploaded file (optional; inferred from filename if omitted)"

	// Branch protection parameters
	BPRule                    = "Branch protection rule name (the rule_name; Forgejo defaults it to branch_name)"
	BPRuleName                = "Rule name (optional; defaults to branch_name if omitted)"
	BPEnablePush              = "Allow direct pushes to the protected branch"
	BPEnableStatusCheck       = "Require status checks to pass before merging"
	BPStatusCheckContexts     = `Required status check contexts (comma-separated, e.g. "ci/build,ci/test")`
	BPRequiredApprovals       = "Number of required approving reviews before merge"
	BPBlockOnOutdatedBranch   = "Block merge when the branch is behind its base"
	BPRequireSignedCommits    = "Require commits on the protected branch to be signed"
	BPDismissStaleApprovals   = "Dismiss approvals when new commits are pushed"
	BPEnablePushWhitelist     = "Restrict direct pushes to the push whitelist (users/teams) instead of all writers"
	BPPushWhitelistUsers      = `Usernames allowed to push to the protected branch (comma-separated, e.g. "alice,bot"). Each must be a collaborator with write access. Replaces the existing list.`
	BPEnableMergeWhitelist    = "Restrict who may merge pull requests to the merge whitelist"
	BPMergeWhitelistUsers     = "Usernames allowed to merge pull requests (comma-separated). Replaces the existing list."
	BPEnableApprovalsWl       = "Restrict whose reviews count toward required approvals to the approvals whitelist"
	BPApprovalsWhitelistUsers = "Usernames whose approving reviews count toward required approvals (comma-separated). Replaces the existing list."

	// Release parameters
	ReleaseID              = "Release ID"
	ReleaseTag             = "Existing release tag name"
	ReleaseTagName         = "Tag name for the release (created if it does not exist)"
	ReleaseTargetCommitish = "Branch or commit SHA the tag points at (only used when the tag does not yet exist)"
	ReleaseDraft           = "Whether the release is a draft"
	ReleasePrerelease      = "Whether the release is a prerelease"
	ReleaseState           = "Filter by state: all|draft|prerelease|published. Filter is applied client-side after pagination, so result size may be smaller than limit."

	// Time tracking parameters
	TimeID         = "Tracked time entry ID"
	TimeSeconds    = "Time in seconds to log (positive integer). Provide exactly one of seconds or duration."
	TimeDuration   = `Time as a duration string, e.g. "15m", "1h30m", "45s". Provide exactly one of seconds or duration.`
	TimeCreatedAt  = "Optional RFC3339 timestamp for when the work happened (defaults to server time)"
	TimeUserName   = "Optional username to log time on behalf of (requires admin; omit for self)"
	TimeUserFilter = "Filter results to this username"
)

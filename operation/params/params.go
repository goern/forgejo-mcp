package params

// Shared parameter descriptions for MCP tools
// Following Anthropic's guideline: "be frugal with their use of tokens"
// Patterns follow official MCP server conventions (GitHub, MCP spec examples)
const (
	// Common repository parameters
	Owner = "Repository owner"
	Repo  = "Repository name"

	// Issue/PR parameters
	Index        = "Issue/PR index"
	IssueIndex   = "Issue index"
	PRIndex      = "PR index"
	CommentID    = "Comment ID"
	Body         = "Content body"
	Title        = "Title"
	State        = "State"
	Labels       = "Label IDs"
	Milestone    = "Milestone ID"

	// Branch parameters
	Branch       = "Branch name"
	OldBranch    = "Source branch"
	Head         = "Head branch"
	Base         = "Base branch"

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
	ReviewID    = "Review ID"
	ReviewState = "Review state (APPROVED, REQUEST_CHANGES, COMMENT)"
	ReviewBody  = "Review body/message"
	Reviewers   = "Reviewer usernames (comma-separated)"
	TeamReviewers = "Team reviewer names (comma-separated)"
	DismissMessage = "Dismissal message"
	ReviewComments = `Inline comments as JSON array, e.g. [{"path":"file.go","body":"Fix this","new_position":10}]`

	// Actions parameters
	Workflow  = "Workflow file or ID (e.g. main.yml)"
	Inputs    = `Workflow inputs as JSON object (e.g. {"key": "value"})`
	Event     = "Filter by event type (e.g. push, pull_request, workflow_dispatch)"
	RunNumber = "Filter by run number"
	HeadSHA   = "Filter by HEAD SHA"
	RunID     = "Run ID"
	Status    = "Filter by status (e.g. waiting, running, success, failure, cancelled)"

	// Misc parameters
	Description = "Description"
	Private     = "Private repo"

	// Attachment parameters
	AttachmentID       = "Attachment ID"
	AttachmentName     = "New name for the attachment"
	AttachmentContent  = "Base64-encoded file bytes to upload"
	AttachmentFilename = `Filename to associate with the uploaded attachment (e.g. "requirements.pdf")`
	AttachmentMIME     = "MIME type hint for uploaded file (optional; inferred from filename if omitted)"

	// Time tracking parameters
	TimeID         = "Tracked time entry ID"
	TimeSeconds    = "Time in seconds to log (positive integer). Provide exactly one of seconds or duration."
	TimeDuration   = `Time as a duration string, e.g. "15m", "1h30m", "45s". Provide exactly one of seconds or duration.`
	TimeCreatedAt  = "Optional RFC3339 timestamp for when the work happened (defaults to server time)"
	TimeUserName   = "Optional username to log time on behalf of (requires admin; omit for self)"
	TimeUserFilter = "Filter results to this username"
)

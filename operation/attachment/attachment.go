// Package attachment registers MCP tools for issue and issue-comment
// attachments. It uses pkg/forgejo's raw-HTTP helper because forgejo-sdk/v3
// has no methods for these endpoints; see docs/plans/issue-attachments.md.
package attachment

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"time"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/params"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forgejo"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/log"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/to"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/upload"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Upload timeout. Multiple agents reported
// create_*_attachment (issue and comment attachments) failing with "illegal
// base64 data" at a variable byte
// offset (~1.5KB-4.9KB observed) and, once, hanging a caller for ~30
// minutes. A size-sweep repro against the real stdio transport (see
// test/transport/sweep_test.go) found no corruption or hang in this server
// for payloads from 1KB to 100KB — the root cause is upstream of this
// process (the calling MCP client/harness), not in this server's read,
// decode, or upload path. This guard exists regardless, as defense in
// depth: it bounds worst-case wall-clock time for a stuck network path so a
// broken upload always fails fast with a clear, actionable error instead of
// hanging.
//
// This timeout is scoped to create_issue_attachment / create_comment_attachment
// only — it is NOT applied to create_release_attachment. Release assets can
// legitimately be much larger than an issue/comment attachment (build
// artifacts, archives, container images) and may need more than 45s of
// upload headroom; create_release_attachment relies on the underlying HTTP
// client's own (longer) timeout instead. The base64 `content` size cap and
// received-length decode diagnostic, by contrast, live in pkg/upload.Open
// and apply uniformly to all three attachment-creating tools, since
// create_release_attachment accepts base64 `content` exactly like the
// issue/comment tools do (see operation/release/release.go).
const (
	// defaultAttachmentUploadTimeout bounds the multipart upload call so a
	// stuck network path fails with a clear, specific error well inside the
	// caller's own patience, instead of running to the underlying HTTP
	// client's 60s timeout (or beyond, if something upstream never returns
	// control at all).
	defaultAttachmentUploadTimeout = 45 * time.Second
)

// attachmentUploadTimeout is a var (not const) so tests can shrink it to
// exercise the timeout path without a real 45s wait; production code never
// reassigns it, so it always behaves as the 45s constant above.
var attachmentUploadTimeout = defaultAttachmentUploadTimeout

const (
	// Issue-scoped tool names
	ListIssueAttachmentsToolName    = "list_issue_attachments"
	GetIssueAttachmentToolName      = "get_issue_attachment"
	DownloadIssueAttachmentToolName = "download_issue_attachment"
	CreateIssueAttachmentToolName   = "create_issue_attachment"
	EditIssueAttachmentToolName     = "edit_issue_attachment"
	DeleteIssueAttachmentToolName   = "delete_issue_attachment"

	// Comment-scoped tool names
	ListCommentAttachmentsToolName    = "list_comment_attachments"
	GetCommentAttachmentToolName      = "get_comment_attachment"
	DownloadCommentAttachmentToolName = "download_comment_attachment"
	CreateCommentAttachmentToolName   = "create_comment_attachment"
	EditCommentAttachmentToolName     = "edit_comment_attachment"
	DeleteCommentAttachmentToolName   = "delete_comment_attachment"

	multipartFieldName = "attachment"
)

// downloadResult is the shape returned by download_*_attachment when bytes
// are inlined. The Blob field carries the embedded resource separately;
// this struct is only used for the metadata-only path.
type downloadResult struct {
	Attachment    *forgejo_sdk.Attachment `json:"attachment"`
	Inline        bool                    `json:"inline"`
	Reason        string                  `json:"reason,omitempty"`
	BytesIncluded int64                   `json:"bytes_included,omitempty"`
}

var (
	ListIssueAttachmentsTool = mcp.NewTool(
		ListIssueAttachmentsToolName,
		mcp.WithDescription("List attachments on an issue or pull request."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.Index)),
	)

	GetIssueAttachmentTool = mcp.NewTool(
		GetIssueAttachmentToolName,
		mcp.WithDescription("Get metadata for a single issue/PR attachment."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.Index)),
		mcp.WithNumber("attachment_id", mcp.Required(), mcp.Description(params.AttachmentID)),
	)

	DownloadIssueAttachmentTool = mcp.NewTool(
		DownloadIssueAttachmentToolName,
		mcp.WithDescription("Download an issue/PR attachment. Files at or above the inline cap return metadata + browser_download_url only; the caller is expected to fetch that URL with the same auth token."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.Index)),
		mcp.WithNumber("attachment_id", mcp.Required(), mcp.Description(params.AttachmentID)),
	)

	CreateIssueAttachmentTool = mcp.NewTool(
		CreateIssueAttachmentToolName,
		mcp.WithDescription("Upload a new attachment to an issue or pull request from exactly one of base64 content or a file path on the forgejo-mcp host."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.Index)),
		mcp.WithString("content", mcp.Description(params.AttachmentContent)),
		mcp.WithString("file_path", mcp.Description(params.AttachmentFilePath)),
		mcp.WithString("filename", mcp.Description(params.AttachmentFilename)),
		mcp.WithString("mime_type", mcp.Description(params.AttachmentMIME)),
	)

	EditIssueAttachmentTool = mcp.NewTool(
		EditIssueAttachmentToolName,
		mcp.WithDescription("Rename an issue/PR attachment."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.Index)),
		mcp.WithNumber("attachment_id", mcp.Required(), mcp.Description(params.AttachmentID)),
		mcp.WithString("name", mcp.Required(), mcp.Description(params.AttachmentName)),
	)

	DeleteIssueAttachmentTool = mcp.NewTool(
		DeleteIssueAttachmentToolName,
		mcp.WithDescription("Delete an issue/PR attachment."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.Index)),
		mcp.WithNumber("attachment_id", mcp.Required(), mcp.Description(params.AttachmentID)),
	)

	ListCommentAttachmentsTool = mcp.NewTool(
		ListCommentAttachmentsToolName,
		mcp.WithDescription("List attachments on an issue/PR comment."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("comment_id", mcp.Required(), mcp.Description(params.CommentID)),
	)

	GetCommentAttachmentTool = mcp.NewTool(
		GetCommentAttachmentToolName,
		mcp.WithDescription("Get metadata for a single comment attachment."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("comment_id", mcp.Required(), mcp.Description(params.CommentID)),
		mcp.WithNumber("attachment_id", mcp.Required(), mcp.Description(params.AttachmentID)),
	)

	DownloadCommentAttachmentTool = mcp.NewTool(
		DownloadCommentAttachmentToolName,
		mcp.WithDescription("Download a comment attachment. Files at or above the inline cap return metadata + browser_download_url only; the caller is expected to fetch that URL with the same auth token."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("comment_id", mcp.Required(), mcp.Description(params.CommentID)),
		mcp.WithNumber("attachment_id", mcp.Required(), mcp.Description(params.AttachmentID)),
	)

	CreateCommentAttachmentTool = mcp.NewTool(
		CreateCommentAttachmentToolName,
		mcp.WithDescription("Upload a new attachment to an issue/PR comment from exactly one of base64 content or a file path on the forgejo-mcp host."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("comment_id", mcp.Required(), mcp.Description(params.CommentID)),
		mcp.WithString("content", mcp.Description(params.AttachmentContent)),
		mcp.WithString("file_path", mcp.Description(params.AttachmentFilePath)),
		mcp.WithString("filename", mcp.Description(params.AttachmentFilename)),
		mcp.WithString("mime_type", mcp.Description(params.AttachmentMIME)),
	)

	EditCommentAttachmentTool = mcp.NewTool(
		EditCommentAttachmentToolName,
		mcp.WithDescription("Rename a comment attachment."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("comment_id", mcp.Required(), mcp.Description(params.CommentID)),
		mcp.WithNumber("attachment_id", mcp.Required(), mcp.Description(params.AttachmentID)),
		mcp.WithString("name", mcp.Required(), mcp.Description(params.AttachmentName)),
	)

	DeleteCommentAttachmentTool = mcp.NewTool(
		DeleteCommentAttachmentToolName,
		mcp.WithDescription("Delete a comment attachment."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("comment_id", mcp.Required(), mcp.Description(params.CommentID)),
		mcp.WithNumber("attachment_id", mcp.Required(), mcp.Description(params.AttachmentID)),
	)
)

// RegisterTool registers all 12 attachment tools with the MCP server.
func RegisterTool(s *server.MCPServer) {
	s.AddTool(ListIssueAttachmentsTool, ListIssueAttachmentsFn)
	s.AddTool(GetIssueAttachmentTool, GetIssueAttachmentFn)
	s.AddTool(DownloadIssueAttachmentTool, DownloadIssueAttachmentFn)
	s.AddTool(CreateIssueAttachmentTool, CreateIssueAttachmentFn)
	s.AddTool(EditIssueAttachmentTool, EditIssueAttachmentFn)
	s.AddTool(DeleteIssueAttachmentTool, DeleteIssueAttachmentFn)

	s.AddTool(ListCommentAttachmentsTool, ListCommentAttachmentsFn)
	s.AddTool(GetCommentAttachmentTool, GetCommentAttachmentFn)
	s.AddTool(DownloadCommentAttachmentTool, DownloadCommentAttachmentFn)
	s.AddTool(CreateCommentAttachmentTool, CreateCommentAttachmentFn)
	s.AddTool(EditCommentAttachmentTool, EditCommentAttachmentFn)
	s.AddTool(DeleteCommentAttachmentTool, DeleteCommentAttachmentFn)
}

// --- Issue-scoped handlers --------------------------------------------------

func ListIssueAttachmentsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListIssueAttachmentsFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	index, err := to.Float64(req.GetArguments()["index"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("index: %w", err))
	}

	var out []*forgejo_sdk.Attachment
	path := forgejo.APIPath("repos", owner, repo, "issues", int64(index), "assets")
	if err := forgejo.DoJSONList(ctx, http.MethodGet, path, &out); err != nil {
		return to.ErrorResult(fmt.Errorf("list issue attachments err: %w", err))
	}
	if out == nil {
		out = []*forgejo_sdk.Attachment{}
	}
	return to.TextResult(out)
}

func GetIssueAttachmentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetIssueAttachmentFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	index, err := to.Float64(req.GetArguments()["index"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("index: %w", err))
	}
	aid, err := to.Float64(req.GetArguments()["attachment_id"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("attachment_id: %w", err))
	}

	att, err := getIssueAttachment(ctx, owner, repo, int64(index), int64(aid))
	if err != nil {
		return to.ErrorResult(fmt.Errorf("get issue attachment err: %w", err))
	}
	return to.TextResult(att)
}

func DownloadIssueAttachmentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DownloadIssueAttachmentFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	index, err := to.Float64(req.GetArguments()["index"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("index: %w", err))
	}
	aid, err := to.Float64(req.GetArguments()["attachment_id"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("attachment_id: %w", err))
	}

	att, err := getIssueAttachment(ctx, owner, repo, int64(index), int64(aid))
	if err != nil {
		return to.ErrorResult(fmt.Errorf("download issue attachment (metadata) err: %w", err))
	}
	return downloadResultFor(ctx, att)
}

func CreateIssueAttachmentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called CreateIssueAttachmentFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	index, err := to.Float64(args["index"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("index: %w", err))
	}
	mimeType, _ := args["mime_type"].(string)
	reader, filename, err := upload.Open(upload.SourceFromArguments(args))
	if err != nil {
		return to.ErrorResult(err)
	}
	defer reader.Close()

	uploadCtx, cancel := context.WithTimeout(ctx, attachmentUploadTimeout)
	defer cancel()

	var att forgejo_sdk.Attachment
	path := forgejo.APIPath("repos", owner, repo, "issues", int64(index), "assets")
	if err := forgejo.DoMultipart(uploadCtx, http.MethodPost, path, multipartFieldName, filename, mimeType, reader, &att); err != nil {
		return to.ErrorResult(fmt.Errorf("create issue attachment err: %w", wrapUploadTimeout(ctx, uploadCtx, err)))
	}
	return to.TextResult(att)
}

func EditIssueAttachmentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called EditIssueAttachmentFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	index, err := to.Float64(req.GetArguments()["index"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("index: %w", err))
	}
	aid, err := to.Float64(req.GetArguments()["attachment_id"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("attachment_id: %w", err))
	}
	name, _ := req.GetArguments()["name"].(string)
	if name == "" {
		return to.ErrorResult(fmt.Errorf("name is required"))
	}

	var att forgejo_sdk.Attachment
	body := map[string]string{"name": name}
	path := forgejo.APIPath("repos", owner, repo, "issues", int64(index), "assets", int64(aid))
	if err := forgejo.DoJSON(ctx, http.MethodPatch, path, body, &att); err != nil {
		return to.ErrorResult(fmt.Errorf("edit issue attachment err: %w", err))
	}
	return to.TextResult(att)
}

func DeleteIssueAttachmentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DeleteIssueAttachmentFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	index, err := to.Float64(req.GetArguments()["index"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("index: %w", err))
	}
	aid, err := to.Float64(req.GetArguments()["attachment_id"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("attachment_id: %w", err))
	}
	path := forgejo.APIPath("repos", owner, repo, "issues", int64(index), "assets", int64(aid))
	if err := forgejo.DoJSON(ctx, http.MethodDelete, path, nil, nil); err != nil {
		return to.ErrorResult(fmt.Errorf("delete issue attachment err: %w", err))
	}
	return to.TextResult(map[string]string{"status": "deleted"})
}

// --- Comment-scoped handlers ------------------------------------------------

func ListCommentAttachmentsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListCommentAttachmentsFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	cid, err := to.Float64(req.GetArguments()["comment_id"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("comment_id: %w", err))
	}

	var out []*forgejo_sdk.Attachment
	path := forgejo.APIPath("repos", owner, repo, "issues", "comments", int64(cid), "assets")
	if err := forgejo.DoJSONList(ctx, http.MethodGet, path, &out); err != nil {
		return to.ErrorResult(fmt.Errorf("list comment attachments err: %w", err))
	}
	if out == nil {
		out = []*forgejo_sdk.Attachment{}
	}
	return to.TextResult(out)
}

func GetCommentAttachmentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetCommentAttachmentFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	cid, err := to.Float64(req.GetArguments()["comment_id"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("comment_id: %w", err))
	}
	aid, err := to.Float64(req.GetArguments()["attachment_id"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("attachment_id: %w", err))
	}

	att, err := getCommentAttachment(ctx, owner, repo, int64(cid), int64(aid))
	if err != nil {
		return to.ErrorResult(fmt.Errorf("get comment attachment err: %w", err))
	}
	return to.TextResult(att)
}

func DownloadCommentAttachmentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DownloadCommentAttachmentFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	cid, err := to.Float64(req.GetArguments()["comment_id"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("comment_id: %w", err))
	}
	aid, err := to.Float64(req.GetArguments()["attachment_id"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("attachment_id: %w", err))
	}
	att, err := getCommentAttachment(ctx, owner, repo, int64(cid), int64(aid))
	if err != nil {
		return to.ErrorResult(fmt.Errorf("download comment attachment (metadata) err: %w", err))
	}
	return downloadResultFor(ctx, att)
}

func CreateCommentAttachmentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called CreateCommentAttachmentFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	cid, err := to.Float64(args["comment_id"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("comment_id: %w", err))
	}
	mimeType, _ := args["mime_type"].(string)
	reader, filename, err := upload.Open(upload.SourceFromArguments(args))
	if err != nil {
		return to.ErrorResult(err)
	}
	defer reader.Close()

	uploadCtx, cancel := context.WithTimeout(ctx, attachmentUploadTimeout)
	defer cancel()

	var att forgejo_sdk.Attachment
	path := forgejo.APIPath("repos", owner, repo, "issues", "comments", int64(cid), "assets")
	if err := forgejo.DoMultipart(uploadCtx, http.MethodPost, path, multipartFieldName, filename, mimeType, reader, &att); err != nil {
		return to.ErrorResult(fmt.Errorf("create comment attachment err: %w", wrapUploadTimeout(ctx, uploadCtx, err)))
	}
	return to.TextResult(att)
}

func EditCommentAttachmentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called EditCommentAttachmentFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	cid, err := to.Float64(req.GetArguments()["comment_id"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("comment_id: %w", err))
	}
	aid, err := to.Float64(req.GetArguments()["attachment_id"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("attachment_id: %w", err))
	}
	name, _ := req.GetArguments()["name"].(string)
	if name == "" {
		return to.ErrorResult(fmt.Errorf("name is required"))
	}

	var att forgejo_sdk.Attachment
	body := map[string]string{"name": name}
	path := forgejo.APIPath("repos", owner, repo, "issues", "comments", int64(cid), "assets", int64(aid))
	if err := forgejo.DoJSON(ctx, http.MethodPatch, path, body, &att); err != nil {
		return to.ErrorResult(fmt.Errorf("edit comment attachment err: %w", err))
	}
	return to.TextResult(att)
}

func DeleteCommentAttachmentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DeleteCommentAttachmentFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	cid, err := to.Float64(req.GetArguments()["comment_id"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("comment_id: %w", err))
	}
	aid, err := to.Float64(req.GetArguments()["attachment_id"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("attachment_id: %w", err))
	}
	path := forgejo.APIPath("repos", owner, repo, "issues", "comments", int64(cid), "assets", int64(aid))
	if err := forgejo.DoJSON(ctx, http.MethodDelete, path, nil, nil); err != nil {
		return to.ErrorResult(fmt.Errorf("delete comment attachment err: %w", err))
	}
	return to.TextResult(map[string]string{"status": "deleted"})
}

// wrapUploadTimeout annotates err with a clear, specific message when the
// upload's own context deadline (attachmentUploadTimeout) is what ended the
// call, so a caller sees "upload timed out after 45s" instead of a bare
// "context deadline exceeded" buried inside an HTTP transport error.
//
// parent is the caller's original context (before attachmentUploadTimeout
// was applied) and uploadCtx is the child context actually passed to the
// upload call. Both must be checked: if parent's own deadline is what
// elapsed — e.g. an overall request timeout imposed by the MCP host, unaware
// of and unrelated to attachmentUploadTimeout — parent.Err() is already
// DeadlineExceeded by the time the upload call returns (context.WithTimeout
// derives uploadCtx from parent, so a parent-caused expiry propagates the
// same error down), and the message must not claim "upload timed out after
// 45s" for a deadline this function's own budget had nothing to do with.
func wrapUploadTimeout(parent, uploadCtx context.Context, err error) error {
	if !errors.Is(uploadCtx.Err(), context.DeadlineExceeded) {
		return err
	}
	if errors.Is(parent.Err(), context.DeadlineExceeded) {
		return err
	}
	return fmt.Errorf("upload timed out after %s: %w", attachmentUploadTimeout, err)
}

// --- shared helpers ---------------------------------------------------------

func getIssueAttachment(ctx context.Context, owner, repo string, index, aid int64) (*forgejo_sdk.Attachment, error) {
	var att forgejo_sdk.Attachment
	path := forgejo.APIPath("repos", owner, repo, "issues", index, "assets", aid)
	if err := forgejo.DoJSON(ctx, http.MethodGet, path, nil, &att); err != nil {
		return nil, err
	}
	return &att, nil
}

func getCommentAttachment(ctx context.Context, owner, repo string, cid, aid int64) (*forgejo_sdk.Attachment, error) {
	var att forgejo_sdk.Attachment
	path := forgejo.APIPath("repos", owner, repo, "issues", "comments", cid, "assets", aid)
	if err := forgejo.DoJSON(ctx, http.MethodGet, path, nil, &att); err != nil {
		return nil, err
	}
	return &att, nil
}

// downloadResultFor fetches inline bytes when the attachment is under the
// inline cap; otherwise returns metadata + URL only and instructs the caller
// to fetch via curl. Always includes browser_download_url.
func downloadResultFor(ctx context.Context, att *forgejo_sdk.Attachment) (*mcp.CallToolResult, error) {
	res := &downloadResult{Attachment: att}

	if att.Size >= forgejo.MaxInlineDownloadBytes {
		res.Inline = false
		res.Reason = fmt.Sprintf("size %d bytes >= inline cap %d; fetch browser_download_url with Authorization: token <TOKEN>", att.Size, forgejo.MaxInlineDownloadBytes)
		return to.TextResult(res)
	}

	body, ct, err := forgejo.DoRaw(ctx, att.DownloadURL)
	if err != nil {
		// Defensive: if the size advertised was under cap but the actual bytes
		// blow past it, treat that the same as "too big" rather than failing.
		if errors.Is(err, forgejo.ErrPayloadTooLarge) {
			res.Inline = false
			res.Reason = "body exceeded inline cap during fetch; fetch browser_download_url with Authorization: token <TOKEN>"
			return to.TextResult(res)
		}
		return to.ErrorResult(fmt.Errorf("download body err: %w", err))
	}
	res.Inline = true
	res.BytesIncluded = int64(len(body))

	uri := att.DownloadURL
	mimeType := ct
	encoded := base64.StdEncoding.EncodeToString(body)

	// NewToolResultResource gives us a CallToolResult containing both a
	// text content (the metadata-as-JSON) and an embedded BlobResourceContents.
	// MCP clients that don't know about embedded resources still see the JSON.
	textPart := to.SafeJSONMarshal(res)
	return mcp.NewToolResultResource(textPart, mcp.BlobResourceContents{
		URI:      uri,
		MIMEType: mimeType,
		Blob:     encoded,
	}), nil
}

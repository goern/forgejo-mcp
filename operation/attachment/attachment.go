// Package attachment registers MCP tools for issue and issue-comment
// attachments. It uses pkg/forgejo's raw-HTTP helper because forgejo-sdk/v3
// has no methods for these endpoints; see docs/plans/issue-attachments.md.
package attachment

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"

	"codeberg.org/goern/forgejo-mcp/v2/operation/params"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/forgejo"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/log"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/to"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

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
		mcp.WithDescription("Upload a new attachment to an issue or pull request."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.Index)),
		mcp.WithString("content", mcp.Required(), mcp.Description(params.AttachmentContent)),
		mcp.WithString("filename", mcp.Required(), mcp.Description(params.AttachmentFilename)),
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
		mcp.WithDescription("Upload a new attachment to an issue/PR comment."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("comment_id", mcp.Required(), mcp.Description(params.CommentID)),
		mcp.WithString("content", mcp.Required(), mcp.Description(params.AttachmentContent)),
		mcp.WithString("filename", mcp.Required(), mcp.Description(params.AttachmentFilename)),
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
		return to.ErrorResult(fmt.Errorf("index: %v", err))
	}

	var out []*forgejo_sdk.Attachment
	path := fmt.Sprintf("/repos/%s/%s/issues/%d/assets", owner, repo, int64(index))
	if err := forgejo.DoJSONList(ctx, http.MethodGet, path, &out); err != nil {
		return to.ErrorResult(fmt.Errorf("list issue attachments err: %v", err))
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
		return to.ErrorResult(fmt.Errorf("index: %v", err))
	}
	aid, err := to.Float64(req.GetArguments()["attachment_id"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("attachment_id: %v", err))
	}

	att, err := getIssueAttachment(ctx, owner, repo, int64(index), int64(aid))
	if err != nil {
		return to.ErrorResult(fmt.Errorf("get issue attachment err: %v", err))
	}
	return to.TextResult(att)
}

func DownloadIssueAttachmentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DownloadIssueAttachmentFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	index, err := to.Float64(req.GetArguments()["index"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("index: %v", err))
	}
	aid, err := to.Float64(req.GetArguments()["attachment_id"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("attachment_id: %v", err))
	}

	att, err := getIssueAttachment(ctx, owner, repo, int64(index), int64(aid))
	if err != nil {
		return to.ErrorResult(fmt.Errorf("download issue attachment (metadata) err: %v", err))
	}
	return downloadResultFor(ctx, att)
}

func CreateIssueAttachmentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called CreateIssueAttachmentFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	index, err := to.Float64(req.GetArguments()["index"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("index: %v", err))
	}
	content, _ := req.GetArguments()["content"].(string)
	filename, _ := req.GetArguments()["filename"].(string)
	mimeType, _ := req.GetArguments()["mime_type"].(string)

	if filename == "" {
		return to.ErrorResult(fmt.Errorf("filename is required"))
	}
	raw, err := base64.StdEncoding.DecodeString(content)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("content must be base64-encoded: %v", err))
	}

	var att forgejo_sdk.Attachment
	path := fmt.Sprintf("/repos/%s/%s/issues/%d/assets", owner, repo, int64(index))
	if err := forgejo.DoMultipart(ctx, http.MethodPost, path, multipartFieldName, filename, mimeType, bytes.NewReader(raw), &att); err != nil {
		return to.ErrorResult(fmt.Errorf("create issue attachment err: %v", err))
	}
	return to.TextResult(att)
}

func EditIssueAttachmentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called EditIssueAttachmentFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	index, err := to.Float64(req.GetArguments()["index"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("index: %v", err))
	}
	aid, err := to.Float64(req.GetArguments()["attachment_id"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("attachment_id: %v", err))
	}
	name, _ := req.GetArguments()["name"].(string)
	if name == "" {
		return to.ErrorResult(fmt.Errorf("name is required"))
	}

	var att forgejo_sdk.Attachment
	body := map[string]string{"name": name}
	path := fmt.Sprintf("/repos/%s/%s/issues/%d/assets/%d", owner, repo, int64(index), int64(aid))
	if err := forgejo.DoJSON(ctx, http.MethodPatch, path, body, &att); err != nil {
		return to.ErrorResult(fmt.Errorf("edit issue attachment err: %v", err))
	}
	return to.TextResult(att)
}

func DeleteIssueAttachmentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DeleteIssueAttachmentFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	index, err := to.Float64(req.GetArguments()["index"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("index: %v", err))
	}
	aid, err := to.Float64(req.GetArguments()["attachment_id"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("attachment_id: %v", err))
	}
	path := fmt.Sprintf("/repos/%s/%s/issues/%d/assets/%d", owner, repo, int64(index), int64(aid))
	if err := forgejo.DoJSON(ctx, http.MethodDelete, path, nil, nil); err != nil {
		return to.ErrorResult(fmt.Errorf("delete issue attachment err: %v", err))
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
		return to.ErrorResult(fmt.Errorf("comment_id: %v", err))
	}

	var out []*forgejo_sdk.Attachment
	path := fmt.Sprintf("/repos/%s/%s/issues/comments/%d/assets", owner, repo, int64(cid))
	if err := forgejo.DoJSONList(ctx, http.MethodGet, path, &out); err != nil {
		return to.ErrorResult(fmt.Errorf("list comment attachments err: %v", err))
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
		return to.ErrorResult(fmt.Errorf("comment_id: %v", err))
	}
	aid, err := to.Float64(req.GetArguments()["attachment_id"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("attachment_id: %v", err))
	}

	att, err := getCommentAttachment(ctx, owner, repo, int64(cid), int64(aid))
	if err != nil {
		return to.ErrorResult(fmt.Errorf("get comment attachment err: %v", err))
	}
	return to.TextResult(att)
}

func DownloadCommentAttachmentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DownloadCommentAttachmentFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	cid, err := to.Float64(req.GetArguments()["comment_id"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("comment_id: %v", err))
	}
	aid, err := to.Float64(req.GetArguments()["attachment_id"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("attachment_id: %v", err))
	}
	att, err := getCommentAttachment(ctx, owner, repo, int64(cid), int64(aid))
	if err != nil {
		return to.ErrorResult(fmt.Errorf("download comment attachment (metadata) err: %v", err))
	}
	return downloadResultFor(ctx, att)
}

func CreateCommentAttachmentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called CreateCommentAttachmentFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	cid, err := to.Float64(req.GetArguments()["comment_id"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("comment_id: %v", err))
	}
	content, _ := req.GetArguments()["content"].(string)
	filename, _ := req.GetArguments()["filename"].(string)
	mimeType, _ := req.GetArguments()["mime_type"].(string)
	if filename == "" {
		return to.ErrorResult(fmt.Errorf("filename is required"))
	}
	raw, err := base64.StdEncoding.DecodeString(content)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("content must be base64-encoded: %v", err))
	}

	var att forgejo_sdk.Attachment
	path := fmt.Sprintf("/repos/%s/%s/issues/comments/%d/assets", owner, repo, int64(cid))
	if err := forgejo.DoMultipart(ctx, http.MethodPost, path, multipartFieldName, filename, mimeType, bytes.NewReader(raw), &att); err != nil {
		return to.ErrorResult(fmt.Errorf("create comment attachment err: %v", err))
	}
	return to.TextResult(att)
}

func EditCommentAttachmentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called EditCommentAttachmentFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	cid, err := to.Float64(req.GetArguments()["comment_id"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("comment_id: %v", err))
	}
	aid, err := to.Float64(req.GetArguments()["attachment_id"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("attachment_id: %v", err))
	}
	name, _ := req.GetArguments()["name"].(string)
	if name == "" {
		return to.ErrorResult(fmt.Errorf("name is required"))
	}

	var att forgejo_sdk.Attachment
	body := map[string]string{"name": name}
	path := fmt.Sprintf("/repos/%s/%s/issues/comments/%d/assets/%d", owner, repo, int64(cid), int64(aid))
	if err := forgejo.DoJSON(ctx, http.MethodPatch, path, body, &att); err != nil {
		return to.ErrorResult(fmt.Errorf("edit comment attachment err: %v", err))
	}
	return to.TextResult(att)
}

func DeleteCommentAttachmentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DeleteCommentAttachmentFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	cid, err := to.Float64(req.GetArguments()["comment_id"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("comment_id: %v", err))
	}
	aid, err := to.Float64(req.GetArguments()["attachment_id"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("attachment_id: %v", err))
	}
	path := fmt.Sprintf("/repos/%s/%s/issues/comments/%d/assets/%d", owner, repo, int64(cid), int64(aid))
	if err := forgejo.DoJSON(ctx, http.MethodDelete, path, nil, nil); err != nil {
		return to.ErrorResult(fmt.Errorf("delete comment attachment err: %v", err))
	}
	return to.TextResult(map[string]string{"status": "deleted"})
}

// --- shared helpers ---------------------------------------------------------

func getIssueAttachment(ctx context.Context, owner, repo string, index, aid int64) (*forgejo_sdk.Attachment, error) {
	var att forgejo_sdk.Attachment
	path := fmt.Sprintf("/repos/%s/%s/issues/%d/assets/%d", owner, repo, index, aid)
	if err := forgejo.DoJSON(ctx, http.MethodGet, path, nil, &att); err != nil {
		return nil, err
	}
	return &att, nil
}

func getCommentAttachment(ctx context.Context, owner, repo string, cid, aid int64) (*forgejo_sdk.Attachment, error) {
	var att forgejo_sdk.Attachment
	path := fmt.Sprintf("/repos/%s/%s/issues/comments/%d/assets/%d", owner, repo, cid, aid)
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
			res.Reason = fmt.Sprintf("body exceeded inline cap during fetch; fetch browser_download_url with Authorization: token <TOKEN>")
			return to.TextResult(res)
		}
		return to.ErrorResult(fmt.Errorf("download body err: %v", err))
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

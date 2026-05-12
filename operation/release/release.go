// Package release registers MCP tools for Forgejo releases and their
// attachments. The Forgejo SDK provides every endpoint we wrap, so this
// package uses forgejo.Client() directly (no raw HTTP fallback) — except
// for download_release_attachment, which fetches the browser_download_url
// through forgejo.DoRaw to share the inline-size cap with the issue/comment
// attachment download path.
package release

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"

	"codeberg.org/goern/forgejo-mcp/v2/operation/params"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/forgejo"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/log"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/to"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	ListReleasesToolName       = "list_releases"
	GetReleaseByIDToolName     = "get_release_by_id"
	GetReleaseByTagToolName    = "get_release_by_tag"
	GetLatestReleaseToolName   = "get_latest_release"
	CreateReleaseToolName      = "create_release"
	EditReleaseToolName        = "edit_release"
	DeleteReleaseToolName      = "delete_release"
	DeleteReleaseByTagToolName = "delete_release_by_tag"

	ListReleaseAttachmentsToolName    = "list_release_attachments"
	GetReleaseAttachmentToolName      = "get_release_attachment"
	DownloadReleaseAttachmentToolName = "download_release_attachment"
	CreateReleaseAttachmentToolName   = "create_release_attachment"
	EditReleaseAttachmentToolName     = "edit_release_attachment"
	DeleteReleaseAttachmentToolName   = "delete_release_attachment"

	// State filter values for list_releases.
	stateAll        = "all"
	stateDraft      = "draft"
	statePrerelease = "prerelease"
	statePublished  = "published"
)

// downloadResult is the metadata-only / over-cap shape returned by
// download_release_attachment. Mirrors operation/attachment.downloadResult
// so MCP clients see a consistent contract across attachment domains.
type downloadResult struct {
	Attachment    *forgejo_sdk.Attachment `json:"attachment"`
	Inline        bool                    `json:"inline"`
	Reason        string                  `json:"reason,omitempty"`
	BytesIncluded int64                   `json:"bytes_included,omitempty"`
}

var (
	ListReleasesTool = mcp.NewTool(
		ListReleasesToolName,
		mcp.WithDescription("List releases for a repository. The state filter is applied client-side after pagination, so result size may be smaller than limit even when more matches exist on later pages."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(20)),
		mcp.WithString("state", mcp.Description(params.ReleaseState), mcp.DefaultString(stateAll)),
	)

	GetReleaseByIDTool = mcp.NewTool(
		GetReleaseByIDToolName,
		mcp.WithDescription("Get a release by numeric ID."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("release_id", mcp.Required(), mcp.Description(params.ReleaseID)),
	)

	GetReleaseByTagTool = mcp.NewTool(
		GetReleaseByTagToolName,
		mcp.WithDescription("Get a release by tag name."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("tag", mcp.Required(), mcp.Description(params.ReleaseTag)),
	)

	GetLatestReleaseTool = mcp.NewTool(
		GetLatestReleaseToolName,
		mcp.WithDescription("Get the latest non-draft, non-prerelease release."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
	)

	CreateReleaseTool = mcp.NewTool(
		CreateReleaseToolName,
		mcp.WithDescription("Create a release. If the tag does not yet exist, pass target_commitish so Forgejo creates the tag."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("tag_name", mcp.Required(), mcp.Description(params.ReleaseTagName)),
		mcp.WithString("target_commitish", mcp.Description(params.ReleaseTargetCommitish)),
		mcp.WithString("name", mcp.Description(params.Title)),
		mcp.WithString("body", mcp.Description(params.Body)),
		mcp.WithBoolean("draft", mcp.Description(params.ReleaseDraft)),
		mcp.WithBoolean("prerelease", mcp.Description(params.ReleasePrerelease)),
	)

	EditReleaseTool = mcp.NewTool(
		EditReleaseToolName,
		mcp.WithDescription("Edit an existing release. Only fields supplied by the caller are sent to Forgejo."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("release_id", mcp.Required(), mcp.Description(params.ReleaseID)),
		mcp.WithString("tag_name", mcp.Description(params.ReleaseTagName)),
		mcp.WithString("target_commitish", mcp.Description(params.ReleaseTargetCommitish)),
		mcp.WithString("name", mcp.Description(params.Title)),
		mcp.WithString("body", mcp.Description(params.Body)),
		mcp.WithBoolean("draft", mcp.Description(params.ReleaseDraft)),
		mcp.WithBoolean("prerelease", mcp.Description(params.ReleasePrerelease)),
	)

	DeleteReleaseTool = mcp.NewTool(
		DeleteReleaseToolName,
		mcp.WithDescription("Delete a release by numeric ID. Destructive."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("release_id", mcp.Required(), mcp.Description(params.ReleaseID)),
	)

	DeleteReleaseByTagTool = mcp.NewTool(
		DeleteReleaseByTagToolName,
		mcp.WithDescription("Delete a release by tag name. Destructive — verify tag before calling."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("tag", mcp.Required(), mcp.Description(params.ReleaseTag)),
	)

	ListReleaseAttachmentsTool = mcp.NewTool(
		ListReleaseAttachmentsToolName,
		mcp.WithDescription("List attachments on a release. The Forgejo API does not paginate this endpoint server-side, so the response is fetched in full and then sliced client-side; large attachment sets are still fully transferred from Forgejo before slicing."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("release_id", mcp.Required(), mcp.Description(params.ReleaseID)),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(20)),
	)

	GetReleaseAttachmentTool = mcp.NewTool(
		GetReleaseAttachmentToolName,
		mcp.WithDescription("Get metadata for a single release attachment."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("release_id", mcp.Required(), mcp.Description(params.ReleaseID)),
		mcp.WithNumber("attachment_id", mcp.Required(), mcp.Description(params.AttachmentID)),
	)

	DownloadReleaseAttachmentTool = mcp.NewTool(
		DownloadReleaseAttachmentToolName,
		mcp.WithDescription("Download a release attachment. Files at or above the inline cap return metadata + browser_download_url only; the caller is expected to fetch that URL with the same auth token."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("release_id", mcp.Required(), mcp.Description(params.ReleaseID)),
		mcp.WithNumber("attachment_id", mcp.Required(), mcp.Description(params.AttachmentID)),
	)

	CreateReleaseAttachmentTool = mcp.NewTool(
		CreateReleaseAttachmentToolName,
		mcp.WithDescription("Upload an attachment to a release."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("release_id", mcp.Required(), mcp.Description(params.ReleaseID)),
		mcp.WithString("content", mcp.Required(), mcp.Description(params.AttachmentContent)),
		mcp.WithString("filename", mcp.Required(), mcp.Description(params.AttachmentFilename)),
		mcp.WithString("mime_type", mcp.Description(params.AttachmentMIME)),
	)

	EditReleaseAttachmentTool = mcp.NewTool(
		EditReleaseAttachmentToolName,
		mcp.WithDescription("Rename a release attachment."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("release_id", mcp.Required(), mcp.Description(params.ReleaseID)),
		mcp.WithNumber("attachment_id", mcp.Required(), mcp.Description(params.AttachmentID)),
		mcp.WithString("name", mcp.Required(), mcp.Description(params.AttachmentName)),
	)

	DeleteReleaseAttachmentTool = mcp.NewTool(
		DeleteReleaseAttachmentToolName,
		mcp.WithDescription("Delete a release attachment. Destructive."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("release_id", mcp.Required(), mcp.Description(params.ReleaseID)),
		mcp.WithNumber("attachment_id", mcp.Required(), mcp.Description(params.AttachmentID)),
	)
)

// RegisterTool registers all 14 release tools with the MCP server.
func RegisterTool(s *server.MCPServer) {
	s.AddTool(ListReleasesTool, ListReleasesFn)
	s.AddTool(GetReleaseByIDTool, GetReleaseByIDFn)
	s.AddTool(GetReleaseByTagTool, GetReleaseByTagFn)
	s.AddTool(GetLatestReleaseTool, GetLatestReleaseFn)
	s.AddTool(CreateReleaseTool, CreateReleaseFn)
	s.AddTool(EditReleaseTool, EditReleaseFn)
	s.AddTool(DeleteReleaseTool, DeleteReleaseFn)
	s.AddTool(DeleteReleaseByTagTool, DeleteReleaseByTagFn)

	s.AddTool(ListReleaseAttachmentsTool, ListReleaseAttachmentsFn)
	s.AddTool(GetReleaseAttachmentTool, GetReleaseAttachmentFn)
	s.AddTool(DownloadReleaseAttachmentTool, DownloadReleaseAttachmentFn)
	s.AddTool(CreateReleaseAttachmentTool, CreateReleaseAttachmentFn)
	s.AddTool(EditReleaseAttachmentTool, EditReleaseAttachmentFn)
	s.AddTool(DeleteReleaseAttachmentTool, DeleteReleaseAttachmentFn)
}

// --- Release tools ----------------------------------------------------------

func ListReleasesFn(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListReleasesFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	page, _ := to.Float64(args["page"])
	if page == 0 {
		page = 1
	}
	limit, _ := to.Float64(args["limit"])
	if limit == 0 {
		limit = 20
	}
	state, ok := args["state"].(string)
	if !ok || state == "" {
		state = stateAll
	}
	switch state {
	case stateAll, stateDraft, statePrerelease, statePublished:
	default:
		return to.ErrorResult(fmt.Errorf("invalid state %q: must be one of all|draft|prerelease|published", state))
	}

	opt := forgejo_sdk.ListReleasesOptions{
		ListOptions: forgejo_sdk.ListOptions{
			Page:     int(page),
			PageSize: int(limit),
		},
	}
	rels, _, err := forgejo.Client().ListReleases(owner, repo, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("list releases err: %v", err))
	}
	filtered := filterReleasesByState(rels, state)
	if filtered == nil {
		filtered = []*forgejo_sdk.Release{}
	}
	return to.TextResult(filtered)
}

func filterReleasesByState(rels []*forgejo_sdk.Release, state string) []*forgejo_sdk.Release {
	if state == stateAll {
		return rels
	}
	out := make([]*forgejo_sdk.Release, 0, len(rels))
	for _, r := range rels {
		if r == nil {
			continue
		}
		switch state {
		case stateDraft:
			if r.IsDraft {
				out = append(out, r)
			}
		case statePrerelease:
			if !r.IsDraft && r.IsPrerelease {
				out = append(out, r)
			}
		case statePublished:
			if !r.IsDraft && !r.IsPrerelease {
				out = append(out, r)
			}
		}
	}
	return out
}

func GetReleaseByIDFn(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetReleaseByIDFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	rid, err := to.Float64(args["release_id"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("release_id: %v", err))
	}
	rel, _, err := forgejo.Client().GetRelease(owner, repo, int64(rid))
	if err != nil {
		return to.ErrorResult(fmt.Errorf("get release err: %v", err))
	}
	return to.TextResult(rel)
}

func GetReleaseByTagFn(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetReleaseByTagFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	tag, _ := args["tag"].(string)
	if tag == "" {
		return to.ErrorResult(fmt.Errorf("tag is required"))
	}
	rel, _, err := forgejo.Client().GetReleaseByTag(owner, repo, tag)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("get release by tag err: %v", err))
	}
	return to.TextResult(rel)
}

func GetLatestReleaseFn(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetLatestReleaseFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	rel, _, err := forgejo.Client().GetLatestRelease(owner, repo)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("get latest release err: %v", err))
	}
	return to.TextResult(rel)
}

func CreateReleaseFn(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called CreateReleaseFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	tagName, _ := args["tag_name"].(string)
	if tagName == "" {
		return to.ErrorResult(fmt.Errorf("tag_name is required"))
	}
	target, _ := args["target_commitish"].(string)
	name, _ := args["name"].(string)
	body, _ := args["body"].(string)
	draft, _ := args["draft"].(bool)
	prerelease, _ := args["prerelease"].(bool)

	// SDK's CreateReleaseOption.Validate rejects empty Title. Default it to
	// tag_name so callers can omit name without tripping the SDK's check.
	if name == "" {
		name = tagName
	}

	opt := forgejo_sdk.CreateReleaseOption{
		TagName:      tagName,
		Target:       target,
		Title:        name,
		Note:         body,
		IsDraft:      draft,
		IsPrerelease: prerelease,
	}
	rel, _, err := forgejo.Client().CreateRelease(owner, repo, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("create release err: %v", err))
	}
	return to.TextResult(rel)
}

func EditReleaseFn(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called EditReleaseFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	rid, err := to.Float64(args["release_id"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("release_id: %v", err))
	}

	opt := forgejo_sdk.EditReleaseOption{}
	if v, ok := args["tag_name"].(string); ok {
		opt.TagName = v
	}
	if v, ok := args["target_commitish"].(string); ok {
		opt.Target = v
	}
	if v, ok := args["name"].(string); ok {
		opt.Title = v
	}
	if v, ok := args["body"].(string); ok {
		opt.Note = v
	}
	if v, ok := args["draft"].(bool); ok {
		opt.IsDraft = &v
	}
	if v, ok := args["prerelease"].(bool); ok {
		opt.IsPrerelease = &v
	}

	rel, _, err := forgejo.Client().EditRelease(owner, repo, int64(rid), opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("edit release err: %v", err))
	}
	return to.TextResult(rel)
}

func DeleteReleaseFn(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DeleteReleaseFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	rid, err := to.Float64(args["release_id"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("release_id: %v", err))
	}
	if _, err := forgejo.Client().DeleteRelease(owner, repo, int64(rid)); err != nil {
		return to.ErrorResult(fmt.Errorf("delete release err: %v", err))
	}
	return to.TextResult(map[string]string{"status": "deleted"})
}

func DeleteReleaseByTagFn(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DeleteReleaseByTagFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	tag, _ := args["tag"].(string)
	if tag == "" {
		return to.ErrorResult(fmt.Errorf("tag is required"))
	}
	if _, err := forgejo.Client().DeleteReleaseByTag(owner, repo, tag); err != nil {
		return to.ErrorResult(fmt.Errorf("delete release by tag err: %v", err))
	}
	return to.TextResult(map[string]string{"status": "deleted"})
}

// --- Release attachment tools ----------------------------------------------

func ListReleaseAttachmentsFn(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListReleaseAttachmentsFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	rid, err := to.Float64(args["release_id"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("release_id: %v", err))
	}
	page, _ := to.Float64(args["page"])
	if page == 0 {
		page = 1
	}
	limit, _ := to.Float64(args["limit"])
	if limit == 0 {
		limit = 20
	}

	all, _, err := forgejo.Client().ListReleaseAttachments(owner, repo, int64(rid))
	if err != nil {
		return to.ErrorResult(fmt.Errorf("list release attachments err: %v", err))
	}
	sliced := sliceAttachments(all, int(page), int(limit))
	return to.TextResult(sliced)
}

func sliceAttachments(all []*forgejo_sdk.Attachment, page, limit int) []*forgejo_sdk.Attachment {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	offset := (page - 1) * limit
	if offset >= len(all) {
		return []*forgejo_sdk.Attachment{}
	}
	end := offset + limit
	if end > len(all) {
		end = len(all)
	}
	return all[offset:end]
}

func GetReleaseAttachmentFn(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetReleaseAttachmentFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	rid, err := to.Float64(args["release_id"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("release_id: %v", err))
	}
	aid, err := to.Float64(args["attachment_id"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("attachment_id: %v", err))
	}
	att, _, err := forgejo.Client().GetReleaseAttachment(owner, repo, int64(rid), int64(aid))
	if err != nil {
		return to.ErrorResult(fmt.Errorf("get release attachment err: %v", err))
	}
	return to.TextResult(att)
}

func DownloadReleaseAttachmentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DownloadReleaseAttachmentFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	rid, err := to.Float64(args["release_id"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("release_id: %v", err))
	}
	aid, err := to.Float64(args["attachment_id"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("attachment_id: %v", err))
	}
	att, _, err := forgejo.Client().GetReleaseAttachment(owner, repo, int64(rid), int64(aid))
	if err != nil {
		return to.ErrorResult(fmt.Errorf("download release attachment (metadata) err: %v", err))
	}
	return downloadResultFor(ctx, att)
}

func CreateReleaseAttachmentFn(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called CreateReleaseAttachmentFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	rid, err := to.Float64(args["release_id"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("release_id: %v", err))
	}
	content, _ := args["content"].(string)
	filename, _ := args["filename"].(string)
	// mime_type is accepted for parity with issue/comment attachments, but
	// the SDK's CreateReleaseAttachment derives content type from the
	// filename and does not forward an explicit hint.
	_, _ = args["mime_type"].(string)

	if filename == "" {
		return to.ErrorResult(fmt.Errorf("filename is required"))
	}
	raw, err := base64.StdEncoding.DecodeString(content)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("content must be base64-encoded: %v", err))
	}

	att, _, err := forgejo.Client().CreateReleaseAttachment(owner, repo, int64(rid), bytes.NewReader(raw), filename)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("create release attachment err: %v", err))
	}
	return to.TextResult(att)
}

func EditReleaseAttachmentFn(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called EditReleaseAttachmentFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	rid, err := to.Float64(args["release_id"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("release_id: %v", err))
	}
	aid, err := to.Float64(args["attachment_id"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("attachment_id: %v", err))
	}
	name, _ := args["name"].(string)
	if name == "" {
		return to.ErrorResult(fmt.Errorf("name is required"))
	}
	att, _, err := forgejo.Client().EditReleaseAttachment(owner, repo, int64(rid), int64(aid), forgejo_sdk.EditAttachmentOptions{Name: name})
	if err != nil {
		return to.ErrorResult(fmt.Errorf("edit release attachment err: %v", err))
	}
	return to.TextResult(att)
}

func DeleteReleaseAttachmentFn(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DeleteReleaseAttachmentFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	rid, err := to.Float64(args["release_id"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("release_id: %v", err))
	}
	aid, err := to.Float64(args["attachment_id"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("attachment_id: %v", err))
	}
	if _, err := forgejo.Client().DeleteReleaseAttachment(owner, repo, int64(rid), int64(aid)); err != nil {
		return to.ErrorResult(fmt.Errorf("delete release attachment err: %v", err))
	}
	return to.TextResult(map[string]string{"status": "deleted"})
}

// downloadResultFor mirrors operation/attachment.downloadResultFor: inline
// bytes when under the cap; metadata + browser_download_url otherwise.
func downloadResultFor(ctx context.Context, att *forgejo_sdk.Attachment) (*mcp.CallToolResult, error) {
	res := &downloadResult{Attachment: att}

	if att.Size >= forgejo.MaxInlineDownloadBytes {
		res.Inline = false
		res.Reason = fmt.Sprintf("size %d bytes >= inline cap %d; fetch browser_download_url with Authorization: token <TOKEN>", att.Size, forgejo.MaxInlineDownloadBytes)
		return to.TextResult(res)
	}

	body, ct, err := forgejo.DoRaw(ctx, att.DownloadURL)
	if err != nil {
		if errors.Is(err, forgejo.ErrPayloadTooLarge) {
			res.Inline = false
			res.Reason = "body exceeded inline cap during fetch; fetch browser_download_url with Authorization: token <TOKEN>"
			return to.TextResult(res)
		}
		return to.ErrorResult(fmt.Errorf("download body err: %v", err))
	}
	res.Inline = true
	res.BytesIncluded = int64(len(body))

	textPart := to.SafeJSONMarshal(res)
	return mcp.NewToolResultResource(textPart, mcp.BlobResourceContents{
		URI:      att.DownloadURL,
		MIMEType: ct,
		Blob:     base64.StdEncoding.EncodeToString(body),
	}), nil
}

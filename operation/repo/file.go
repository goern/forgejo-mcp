package repo

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"codeberg.org/goern/forgejo-mcp/v2/operation/params"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/forgejo"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/log"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/to"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
)

const (
	GetFileToolName    = "get_file_content"
	CreateFileToolName = "create_file"
	UpdateFileToolName = "update_file"
	DeleteFileToolName = "delete_file"
)

var (
	GetFileContentTool = mcp.NewTool(
		GetFileToolName,
		mcp.WithDescription("Get file content as plain text by default. Set `with_metadata=true` for binary files, or when you need the SHA/encoding/links from the full `ContentsResponse` (e.g. before a follow-up `update_file` call). Optional `start_line` and `end_line` request a 1-indexed inclusive line range; out-of-range values clamp to the file extent. Range parameters are ignored when `with_metadata=true`."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("ref", mcp.Required(), mcp.Description(params.Ref)),
		mcp.WithString("filePath", mcp.Required(), mcp.Description(params.FilePath)),
		mcp.WithBoolean("with_metadata", mcp.Description("Return the full ContentsResponse (sha, encoding, links, type, size, base64 content) instead of plain text.")),
		mcp.WithNumber("start_line", mcp.Description("Optional 1-indexed first line of the slice (inclusive). Defaults to 1 when only end_line is set.")),
		mcp.WithNumber("end_line", mcp.Description("Optional 1-indexed last line of the slice (inclusive). Defaults to the file's last line when only start_line is set.")),
	)

	CreateFileTool = mcp.NewTool(
		CreateFileToolName,
		mcp.WithDescription("Create file"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("filePath", mcp.Required(), mcp.Description(params.FilePath)),
		mcp.WithString("content", mcp.Required(), mcp.Description(params.Content)),
		mcp.WithString("message", mcp.Required(), mcp.Description(params.Message)),
		mcp.WithString("branch_name", mcp.Required(), mcp.Description(params.BranchName)),
		mcp.WithString("new_branch_name", mcp.Description(params.NewBranchName)),
	)

	UpdateFileTool = mcp.NewTool(
		UpdateFileToolName,
		mcp.WithDescription("Update file"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("filePath", mcp.Required(), mcp.Description(params.FilePath)),
		mcp.WithString("content", mcp.Required(), mcp.Description(params.Content)),
		mcp.WithString("message", mcp.Required(), mcp.Description(params.Message)),
		mcp.WithString("branch_name", mcp.Required(), mcp.Description(params.BranchName)),
		mcp.WithString("sha", mcp.Required(), mcp.Description(params.SHA)),
		mcp.WithString("new_branch_name", mcp.Description(params.NewBranchName)),
	)

	DeleteFileTool = mcp.NewTool(
		DeleteFileToolName,
		mcp.WithDescription("Delete file"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("filePath", mcp.Required(), mcp.Description(params.FilePath)),
		mcp.WithString("message", mcp.Required(), mcp.Description(params.Message)),
		mcp.WithString("branch_name", mcp.Required(), mcp.Description(params.BranchName)),
		mcp.WithString("sha", mcp.Required(), mcp.Description(params.SHA)),
		mcp.WithString("new_branch_name", mcp.Description(params.NewBranchName)),
	)
)

func GetFileContentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetFileFn")
	args := req.GetArguments()
	owner, ok := args["owner"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("owner is required"))
	}
	repo, ok := args["repo"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("repo is required"))
	}
	ref, _ := args["ref"].(string)
	filePath, ok := args["filePath"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("filePath is required"))
	}
	withMetadata, _ := args["with_metadata"].(bool)

	if withMetadata {
		client, err := forgejo.Client(ctx)
		if err != nil {
			return to.ErrorResult(err)
		}
		content, _, err := client.GetContents(owner, repo, ref, filePath)
		if err != nil {
			return to.ErrorResult(fmt.Errorf("get file err: %w", err))
		}
		return to.TextResult(content)
	}

	// Default: plain text via GetFile (SDK /raw/ endpoint, no base64).
	// GetFile returns []byte; binary files are returned as-is without detection.
	client, err := forgejo.Client(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	rawBytes, _, err := client.GetFile(owner, repo, ref, filePath)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("get file err: %w", err))
	}

	startF, _ := to.Float64(args["start_line"])
	endF, _ := to.Float64(args["end_line"])
	if startF == 0 && endF == 0 {
		return to.TextResult(string(rawBytes))
	}

	sliced, err := sliceLines(string(rawBytes), int(startF), int(endF))
	if err != nil {
		return to.ErrorResult(err)
	}
	return to.TextResult(sliced)
}

// sliceLines returns the 1-indexed inclusive [start, end] line range
// of content. start=0 means "from line 1"; end=0 means "to the last
// line". Out-of-range values clamp; an inverted range after clamping
// is reported as an error.
func sliceLines(content string, start, end int) (string, error) {
	lines := strings.Split(content, "\n")
	count := len(lines)

	if start <= 0 {
		start = 1
	}
	if end <= 0 {
		end = count
	}
	if end > count {
		end = count
	}
	if start > end {
		return "", fmt.Errorf("start_line (%d) is after end_line (%d) for a file with %d lines", start, end, count)
	}

	return strings.Join(lines[start-1:end], "\n"), nil
}

func CreateFileFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called CreateFileFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	filePath, _ := req.GetArguments()["filePath"].(string)
	content, _ := req.GetArguments()["content"].(string)
	message, _ := req.GetArguments()["message"].(string)
	branchName, _ := req.GetArguments()["branch_name"].(string)
	newBranchName, ok := req.GetArguments()["new_branch_name"].(string)
	if !ok || newBranchName == "" {
		newBranchName = ""
	}
	opt := forgejo_sdk.CreateFileOptions{
		FileOptions: forgejo_sdk.FileOptions{
			Message:       message,
			BranchName:    branchName,
			NewBranchName: newBranchName,
		},
		Content: base64.StdEncoding.EncodeToString([]byte(content)),
	}
	client, err := forgejo.Client(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	fileResp, _, err := client.CreateFile(owner, repo, filePath, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("create file error: %w", err))
	}
	return to.TextResult(fileResp)
}

func UpdateFileFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called UpdateFileFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	filePath, _ := req.GetArguments()["filePath"].(string)
	content, _ := req.GetArguments()["content"].(string)
	message, _ := req.GetArguments()["message"].(string)
	branchName, _ := req.GetArguments()["branch_name"].(string)
	sha, _ := req.GetArguments()["sha"].(string)
	newBranchName, ok := req.GetArguments()["new_branch_name"].(string)
	if !ok || newBranchName == "" {
		newBranchName = ""
	}
	opt := forgejo_sdk.UpdateFileOptions{
		FileOptions: forgejo_sdk.FileOptions{
			Message:       message,
			BranchName:    branchName,
			NewBranchName: newBranchName,
		},
		SHA:     sha,
		Content: base64.StdEncoding.EncodeToString([]byte(content)),
	}
	client, err := forgejo.Client(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	fileResp, _, err := client.UpdateFile(owner, repo, filePath, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("update file error: %w", err))
	}
	return to.TextResult(fileResp)
}

func DeleteFileFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DeleteFileFn")
	owner, ok := req.GetArguments()["owner"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("owner is required"))
	}
	repo, ok := req.GetArguments()["repo"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("repo is required"))
	}
	filePath, ok := req.GetArguments()["filePath"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("filePath is required"))
	}
	message, _ := req.GetArguments()["message"].(string)
	branchName, _ := req.GetArguments()["branch_name"].(string)
	sha, ok := req.GetArguments()["sha"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("sha is required"))
	}
	newBranchName, _ := req.GetArguments()["new_branch_name"].(string)
	opt := forgejo_sdk.DeleteFileOptions{
		FileOptions: forgejo_sdk.FileOptions{
			Message:       message,
			BranchName:    branchName,
			NewBranchName: newBranchName,
		},
		SHA: sha,
	}
	client, err := forgejo.Client(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	_, err = client.DeleteFile(owner, repo, filePath, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("delete file err: %w", err))
	}
	return to.TextResult("Delete file success")
}

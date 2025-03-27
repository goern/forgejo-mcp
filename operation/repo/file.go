package repo

import (
	"context"
	"encoding/base64"
	"fmt"

	"gitea.com/gitea/gitea-mcp/pkg/gitea"
	"gitea.com/gitea/gitea-mcp/pkg/log"
	"gitea.com/gitea/gitea-mcp/pkg/to"

	gitea_sdk "code.gitea.io/sdk/gitea"
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
		mcp.WithDescription("Get file Content and Metadata"),
		mcp.WithString("owner", mcp.Required(), mcp.Description("repository owner")),
		mcp.WithString("repo", mcp.Required(), mcp.Description("repository name")),
		mcp.WithString("ref", mcp.Required(), mcp.Description("ref can be branch/tag/commit")),
		mcp.WithString("filePath", mcp.Required(), mcp.Description("file path")),
	)

	CreateFileTool = mcp.NewTool(
		CreateFileToolName,
		mcp.WithDescription("Create file"),
		mcp.WithString("owner", mcp.Required(), mcp.Description("repository owner")),
		mcp.WithString("repo", mcp.Required(), mcp.Description("repository name")),
		mcp.WithString("filePath", mcp.Required(), mcp.Description("file path")),
		mcp.WithString("content", mcp.Required(), mcp.Description("file content")),
		mcp.WithString("message", mcp.Required(), mcp.Description("commit message")),
		mcp.WithString("branch_name", mcp.Required(), mcp.Description("branch name")),
		mcp.WithString("new_branch_name", mcp.Description("new branch name")),
	)

	UpdateFileTool = mcp.NewTool(
		UpdateFileToolName,
		mcp.WithDescription("Update file"),
		mcp.WithString("owner", mcp.Required(), mcp.Description("repository owner")),
		mcp.WithString("repo", mcp.Required(), mcp.Description("repository name")),
		mcp.WithString("filePath", mcp.Required(), mcp.Description("file path")),
		mcp.WithString("sha", mcp.Required(), mcp.Description("sha is the SHA for the file that already exists")),
		mcp.WithString("content", mcp.Required(), mcp.Description("file content, base64 encoded")),
		mcp.WithString("message", mcp.Required(), mcp.Description("commit message")),
		mcp.WithString("branch_name", mcp.Required(), mcp.Description("branch name")),
	)

	DeleteFileTool = mcp.NewTool(
		DeleteFileToolName,
		mcp.WithDescription("Delete file"),
		mcp.WithString("owner", mcp.Required(), mcp.Description("repository owner")),
		mcp.WithString("repo", mcp.Required(), mcp.Description("repository name")),
		mcp.WithString("filePath", mcp.Required(), mcp.Description("file path")),
		mcp.WithString("message", mcp.Required(), mcp.Description("commit message")),
		mcp.WithString("branch_name", mcp.Required(), mcp.Description("branch name")),
		mcp.WithString("sha", mcp.Description("sha")),
	)
)

func GetFileContentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetFileFn")
	owner, ok := req.Params.Arguments["owner"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("owner is required"))
	}
	repo, ok := req.Params.Arguments["repo"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("repo is required"))
	}
	ref, _ := req.Params.Arguments["ref"].(string)
	filePath, ok := req.Params.Arguments["filePath"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("filePath is required"))
	}
	content, _, err := gitea.Client().GetContents(owner, repo, ref, filePath)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("get file err: %v", err))
	}
	return to.TextResult(content)
}

func CreateFileFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called CreateFileFn")
	owner, ok := req.Params.Arguments["owner"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("owner is required"))
	}
	repo, ok := req.Params.Arguments["repo"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("repo is required"))
	}
	filePath, ok := req.Params.Arguments["filePath"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("filePath is required"))
	}
	content, _ := req.Params.Arguments["content"].(string)
	message, _ := req.Params.Arguments["message"].(string)
	branchName, _ := req.Params.Arguments["branch_name"].(string)
	opt := gitea_sdk.CreateFileOptions{
		Content: base64.StdEncoding.EncodeToString([]byte(content)),
		FileOptions: gitea_sdk.FileOptions{
			Message:    message,
			BranchName: branchName,
		},
	}

	_, _, err := gitea.Client().CreateFile(owner, repo, filePath, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("create file err: %v", err))
	}
	return to.TextResult("Create file success")
}

func UpdateFileFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called UpdateFileFn")
	owner, ok := req.Params.Arguments["owner"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("owner is required"))
	}
	repo, ok := req.Params.Arguments["repo"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("repo is required"))
	}
	filePath, ok := req.Params.Arguments["filePath"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("filePath is required"))
	}
	sha, ok := req.Params.Arguments["sha"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("sha is required"))
	}
	content, _ := req.Params.Arguments["content"].(string)
	message, _ := req.Params.Arguments["message"].(string)
	branchName, _ := req.Params.Arguments["branch_name"].(string)

	opt := gitea_sdk.UpdateFileOptions{
		SHA:     sha,
		Content: content,
		FileOptions: gitea_sdk.FileOptions{
			Message:    message,
			BranchName: branchName,
		},
	}
	_, _, err := gitea.Client().UpdateFile(owner, repo, filePath, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("update file err: %v", err))
	}
	return to.TextResult("Update file success")
}

func DeleteFileFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DeleteFileFn")
	owner, ok := req.Params.Arguments["owner"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("owner is required"))
	}
	repo, ok := req.Params.Arguments["repo"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("repo is required"))
	}
	filePath, ok := req.Params.Arguments["filePath"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("filePath is required"))
	}
	message, _ := req.Params.Arguments["message"].(string)
	branchName, _ := req.Params.Arguments["branch_name"].(string)
	sha, ok := req.Params.Arguments["sha"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("sha is required"))
	}
	opt := gitea_sdk.DeleteFileOptions{
		FileOptions: gitea_sdk.FileOptions{
			Message:    message,
			BranchName: branchName,
		},
		SHA: sha,
	}
	_, err := gitea.Client().DeleteFile(owner, repo, filePath, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("delete file err: %v", err))
	}
	return to.TextResult("Delete file success")
}

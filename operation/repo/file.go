package repo

import (
	"context"
	"fmt"

	"forgejo.com/forgejo/forgejo-mcp/pkg/forgejo"
	"forgejo.com/forgejo/forgejo-mcp/pkg/log"
	"forgejo.com/forgejo/forgejo-mcp/pkg/to"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v2"
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
		mcp.WithString("content", mcp.Required(), mcp.Description("file content")),
		mcp.WithString("message", mcp.Required(), mcp.Description("commit message")),
		mcp.WithString("branch_name", mcp.Required(), mcp.Description("branch name")),
		mcp.WithString("sha", mcp.Required(), mcp.Description("file sha")),
		mcp.WithString("new_branch_name", mcp.Description("new branch name")),
	)

	DeleteFileTool = mcp.NewTool(
		DeleteFileToolName,
		mcp.WithDescription("Delete file"),
		mcp.WithString("owner", mcp.Required(), mcp.Description("repository owner")),
		mcp.WithString("repo", mcp.Required(), mcp.Description("repository name")),
		mcp.WithString("filePath", mcp.Required(), mcp.Description("file path")),
		mcp.WithString("message", mcp.Required(), mcp.Description("commit message")),
		mcp.WithString("branch_name", mcp.Required(), mcp.Description("branch name")),
		mcp.WithString("sha", mcp.Required(), mcp.Description("file sha")),
		mcp.WithString("new_branch_name", mcp.Description("new branch name")),
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
	content, _, err := forgejo.Client().GetContents(owner, repo, ref, filePath)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("get file err: %v", err))
	}
	return to.TextResult(content)
}

func CreateFileFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called CreateFileFn")
	owner, _ := req.Params.Arguments["owner"].(string)
	repo, _ := req.Params.Arguments["repo"].(string)
	filePath, _ := req.Params.Arguments["filePath"].(string)
	content, _ := req.Params.Arguments["content"].(string)
	message, _ := req.Params.Arguments["message"].(string)
	branchName, _ := req.Params.Arguments["branch_name"].(string)
	newBranchName, ok := req.Params.Arguments["new_branch_name"].(string)
	if !ok || newBranchName == "" {
		newBranchName = ""
	}
	opt := forgejo_sdk.CreateFileOptions{
		FileOptions: forgejo_sdk.FileOptions{
			Message:       message,
			BranchName:    branchName,
			NewBranchName: newBranchName,
		},
		Content: content,
	}
	fileResp, _, err := forgejo.Client().CreateFile(owner, repo, filePath, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("create file error: %v", err))
	}
	return to.TextResult(fileResp)
}

func UpdateFileFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called UpdateFileFn")
	owner, _ := req.Params.Arguments["owner"].(string)
	repo, _ := req.Params.Arguments["repo"].(string)
	filePath, _ := req.Params.Arguments["filePath"].(string)
	content, _ := req.Params.Arguments["content"].(string)
	message, _ := req.Params.Arguments["message"].(string)
	branchName, _ := req.Params.Arguments["branch_name"].(string)
	sha, _ := req.Params.Arguments["sha"].(string)
	newBranchName, ok := req.Params.Arguments["new_branch_name"].(string)
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
		Content: content,
	}
	fileResp, _, err := forgejo.Client().UpdateFile(owner, repo, filePath, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("update file error: %v", err))
	}
	return to.TextResult(fileResp)
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
	opt := forgejo_sdk.DeleteFileOptions{
		FileOptions: forgejo_sdk.FileOptions{
			Message:    message,
			BranchName: branchName,
		},
		SHA: sha,
	}
	_, err := forgejo.Client().DeleteFile(owner, repo, filePath, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("delete file err: %v", err))
	}
	return to.TextResult("Delete file success")
}

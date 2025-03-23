package repo

import (
	"context"
	"fmt"

	"gitea.com/gitea/gitea-mcp/pkg/gitea"
	"gitea.com/gitea/gitea-mcp/pkg/log"
	"gitea.com/gitea/gitea-mcp/pkg/to"

	gitea_sdk "code.gitea.io/sdk/gitea"
	"github.com/mark3labs/mcp-go/mcp"
)

const (
	GetFileToolName    = "get_file"
	CreateFileToolName = "create_file"
	UpdateFileToolName = "update_file"
	DeleteFileToolName = "delete_file"
)

var (
	GetFileTool = mcp.NewTool(
		GetFileToolName,
		mcp.WithDescription("Get file"),
		mcp.WithString("owner", mcp.Required(), mcp.Description("repository owner"), mcp.DefaultString("")),
		mcp.WithString("repo", mcp.Required(), mcp.Description("repository name"), mcp.DefaultString("")),
		mcp.WithString("ref", mcp.Required(), mcp.Description("ref"), mcp.DefaultString("")),
		mcp.WithString("filePath", mcp.Required(), mcp.Description("file path"), mcp.DefaultString("")),
	)

	CreateFileTool = mcp.NewTool(
		CreateFileToolName,
		mcp.WithDescription("Create file"),
		mcp.WithString("owner", mcp.Required(), mcp.Description("repository owner"), mcp.DefaultString("")),
		mcp.WithString("repo", mcp.Required(), mcp.Description("repository name"), mcp.DefaultString("")),
		mcp.WithString("filePath", mcp.Required(), mcp.Description("file path"), mcp.DefaultString("")),
		mcp.WithString("content", mcp.Required(), mcp.Description("file content"), mcp.DefaultString("")),
		mcp.WithString("message", mcp.Required(), mcp.Description("commit message"), mcp.DefaultString("")),
		mcp.WithString("branch_name", mcp.Required(), mcp.Description("branch name"), mcp.DefaultString("")),
		mcp.WithString("new_branch_name", mcp.Description("new branch name"), mcp.DefaultString("")),
	)

	UpdateFileTool = mcp.NewTool(
		UpdateFileToolName,
		mcp.WithDescription("Update file"),
		mcp.WithString("owner", mcp.Required(), mcp.Description("repository owner"), mcp.DefaultString("")),
		mcp.WithString("repo", mcp.Required(), mcp.Description("repository name"), mcp.DefaultString("")),
		mcp.WithString("filePath", mcp.Required(), mcp.Description("file path"), mcp.DefaultString("")),
		mcp.WithString("content", mcp.Required(), mcp.Description("file content"), mcp.DefaultString("")),
		mcp.WithString("message", mcp.Required(), mcp.Description("commit message"), mcp.DefaultString("")),
		mcp.WithString("branch_name", mcp.Required(), mcp.Description("branch name"), mcp.DefaultString("")),
		mcp.WithString("new_branch_name", mcp.Description("new branch name"), mcp.DefaultString("")),
		mcp.WithString("from_path", mcp.Description("from path"), mcp.DefaultString("")),
		mcp.WithString("sha", mcp.Description("sha"), mcp.DefaultString("")),
	)

	DeleteFileTool = mcp.NewTool(
		DeleteFileToolName,
		mcp.WithDescription("Delete file"),
		mcp.WithString("owner", mcp.Required(), mcp.Description("repository owner"), mcp.DefaultString("")),
		mcp.WithString("repo", mcp.Required(), mcp.Description("repository name"), mcp.DefaultString("")),
		mcp.WithString("filePath", mcp.Required(), mcp.Description("file path"), mcp.DefaultString("")),
		mcp.WithString("message", mcp.Required(), mcp.Description("commit message"), mcp.DefaultString("")),
		mcp.WithString("branch_name", mcp.Required(), mcp.Description("branch name"), mcp.DefaultString("")),
		mcp.WithString("sha", mcp.Description("sha"), mcp.DefaultString("")),
	)
)

func GetFileFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetFileFn")
	owner := req.Params.Arguments["owner"].(string)
	repo := req.Params.Arguments["repo"].(string)
	ref := req.Params.Arguments["ref"].(string)
	filePath := req.Params.Arguments["filePath"].(string)
	file, _, err := gitea.Client().GetFile(owner, repo, ref, filePath)
	if err != nil {
		return nil, fmt.Errorf("get file err: %v", err)
	}
	return to.TextResult(file)
}

func CreateFileFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called CreateFileFn")
	owner := req.Params.Arguments["owner"].(string)
	repo := req.Params.Arguments["repo"].(string)
	filePath := req.Params.Arguments["filePath"].(string)
	opt := gitea_sdk.CreateFileOptions{
		Content: req.Params.Arguments["content"].(string),
		FileOptions: gitea_sdk.FileOptions{
			Message:       req.Params.Arguments["message"].(string),
			BranchName:    req.Params.Arguments["branch_name"].(string),
			NewBranchName: req.Params.Arguments["new_branch_name"].(string),
		},
	}

	_, _, err := gitea.Client().CreateFile(owner, repo, filePath, opt)
	if err != nil {
		return nil, fmt.Errorf("create file err: %v", err)
	}
	return to.TextResult("Create file success")
}

func UpdateFileFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called UpdateFileFn")
	owner := req.Params.Arguments["owner"].(string)
	repo := req.Params.Arguments["repo"].(string)
	filePath := req.Params.Arguments["filePath"].(string)
	opt := gitea_sdk.UpdateFileOptions{
		Content:  req.Params.Arguments["content"].(string),
		FromPath: req.Params.Arguments["from_path"].(string),
		SHA:      req.Params.Arguments["sha"].(string),
		FileOptions: gitea_sdk.FileOptions{
			Message:       req.Params.Arguments["message"].(string),
			BranchName:    req.Params.Arguments["branch_name"].(string),
			NewBranchName: req.Params.Arguments["new_branch_name"].(string),
		},
	}
	_, _, err := gitea.Client().UpdateFile(owner, repo, filePath, opt)
	if err != nil {
		return nil, fmt.Errorf("update file err: %v", err)
	}
	return to.TextResult("Update file success")
}

func DeleteFileFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DeleteFileFn")
	owner := req.Params.Arguments["owner"].(string)
	repo := req.Params.Arguments["repo"].(string)
	filePath := req.Params.Arguments["filePath"].(string)
	opt := gitea_sdk.DeleteFileOptions{
		FileOptions: gitea_sdk.FileOptions{
			Message:    req.Params.Arguments["message"].(string),
			BranchName: req.Params.Arguments["branch_name"].(string),
		},
		SHA: req.Params.Arguments["sha"].(string),
	}
	_, err := gitea.Client().DeleteFile(owner, repo, filePath, opt)
	if err != nil {
		return nil, fmt.Errorf("delete file err: %v", err)
	}
	return to.TextResult("Delete file success")
}

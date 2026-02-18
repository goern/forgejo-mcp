package actions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v2"
	"codeberg.org/goern/forgejo-mcp/v2/operation/params"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/forgejo"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/log"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/to"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	DispatchWorkflowToolName = "dispatch_workflow"
)

var (
	DispatchWorkflowTool = mcp.NewTool(
		DispatchWorkflowToolName,
		mcp.WithDescription("Trigger a workflow run"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("workflow", mcp.Required(), mcp.Description(params.Workflow)),
		mcp.WithString("ref", mcp.Required(), mcp.Description(params.Ref)),
		mcp.WithString("inputs", mcp.Description(params.Inputs)),
	)
)

func RegisterTool(s *server.MCPServer) {
	s.AddTool(DispatchWorkflowTool, DispatchWorkflowFn)
	s.AddTool(ListWorkflowRunsTool, ListWorkflowRunsFn)
	s.AddTool(GetWorkflowRunTool, GetWorkflowRunFn)
}

func DispatchWorkflowFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DispatchWorkflowFn")

	owner, ok := req.GetArguments()["owner"].(string)
	if !ok || owner == "" {
		return to.ErrorResult(errors.New("owner is required"))
	}
	repo, ok := req.GetArguments()["repo"].(string)
	if !ok || repo == "" {
		return to.ErrorResult(errors.New("repo is required"))
	}
	workflow, ok := req.GetArguments()["workflow"].(string)
	if !ok || workflow == "" {
		return to.ErrorResult(errors.New("workflow is required"))
	}
	ref, ok := req.GetArguments()["ref"].(string)
	if !ok || ref == "" {
		return to.ErrorResult(errors.New("ref is required"))
	}

	// Parse optional inputs JSON
	var inputs map[string]string
	if inputsJSON, ok := req.GetArguments()["inputs"].(string); ok && inputsJSON != "" {
		if err := json.Unmarshal([]byte(inputsJSON), &inputs); err != nil {
			return to.ErrorResult(fmt.Errorf(`invalid inputs JSON: %w (expected format: {"key": "value"})`, err))
		}
	}

	opt := forgejo_sdk.DispatchWorkflowOption{
		Ref:    ref,
		Inputs: inputs,
	}

	client := forgejo.Client()
	_, _, err := client.DispatchRepoWorkflow(owner, repo, workflow, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("failed to dispatch workflow: %w", err))
	}

	result := fmt.Sprintf("Workflow dispatched successfully!\n  Workflow: %s\n  Ref: %s\n  URL: %s/%s/%s/actions",
		workflow, ref, forgejo.GetBaseURL(), owner, repo)

	return to.TextResult(result)
}

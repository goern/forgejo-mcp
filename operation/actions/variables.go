// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"context"
	"errors"
	"fmt"
	"strings"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/params"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forgejo"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/log"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/to"

	"github.com/mark3labs/mcp-go/mcp"
)

const (
	ListActionVariablesToolName  = "list_action_variables"
	GetActionVariableToolName    = "get_action_variable"
	CreateActionVariableToolName = "create_action_variable"
	UpdateActionVariableToolName = "update_action_variable"
	DeleteActionVariableToolName = "delete_action_variable"

	defaultActionVariablesLimit = 30
	maxActionVariablesLimit     = 50

	// noGlobalActionVariables is the one sentence every variable tool carries:
	// Forgejo resolves a global variable inside a job, and the REST API has
	// no route that lists or reads it.
	noGlobalActionVariables = "Forgejo has no REST API for instance-global action variables. A job can still see one. This tool does not return it."
)

// actionVariableSummary has no Data field. A list page must not carry values.
type actionVariableSummary struct {
	Name    string `json:"name"`
	OwnerID int64  `json:"owner_id"`
	RepoID  int64  `json:"repo_id"`
}

type actionVariable struct {
	Name    string `json:"name"`
	Data    string `json:"data"`
	OwnerID int64  `json:"owner_id"`
	RepoID  int64  `json:"repo_id"`
}

type listActionVariablesResult struct {
	Scope      string                  `json:"scope"`
	Org        string                  `json:"org,omitempty"`
	Owner      string                  `json:"owner,omitempty"`
	Repo       string                  `json:"repo,omitempty"`
	Variables  []actionVariableSummary `json:"variables"`
	Page       int                     `json:"page"`
	Limit      int                     `json:"limit"`
	Count      int                     `json:"count"`
	TotalCount *int                    `json:"total_count,omitempty"`
}

type actionVariableMutationResult struct {
	Scope  string `json:"scope"`
	Org    string `json:"org,omitempty"`
	Owner  string `json:"owner,omitempty"`
	Repo   string `json:"repo,omitempty"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

type actionScopeCoords struct {
	scope string
	org   string
	owner string
	repo  string
}

var (
	ListActionVariablesTool = mcp.NewTool(
		ListActionVariablesToolName,
		mcp.WithDescription("List action variables for scope user, org, or repo. Each row is name, owner_id, and repo_id; values are not included. page defaults to 1, limit defaults to 30 and is at most 50. The envelope is {variables, scope, page, limit, count, total_count?}; total_count is present only when Forgejo sends X-Total-Count. Fetch the next page by calling again with page+1. "+noGlobalActionVariables),
		mcp.WithString("scope", mcp.Required(), mcp.Description(params.ActionScope)),
		mcp.WithString("org", mcp.Description(params.Org)),
		mcp.WithString("owner", mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Description(params.Repo)),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1), mcp.Min(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(defaultActionVariablesLimit), mcp.Min(1), mcp.Max(maxActionVariablesLimit)),
	)

	GetActionVariableTool = mcp.NewTool(
		GetActionVariableToolName,
		mcp.WithDescription("Get one action variable, including its value, for scope user, org, or repo. A missing name is an error, not an empty object. "+noGlobalActionVariables),
		mcp.WithString("scope", mcp.Required(), mcp.Description(params.ActionScope)),
		mcp.WithString("org", mcp.Description(params.Org)),
		mcp.WithString("owner", mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Description(params.Repo)),
		mcp.WithString("name", mcp.Required(), mcp.Description(params.VariableName)),
	)

	CreateActionVariableTool = mcp.NewTool(
		CreateActionVariableToolName,
		mcp.WithDescription("Create an action variable for scope user, org, or repo. Forgejo POST; a name that already exists is an error. data is the value and is not echoed back. "+noGlobalActionVariables),
		mcp.WithString("scope", mcp.Required(), mcp.Description(params.ActionScope)),
		mcp.WithString("org", mcp.Description(params.Org)),
		mcp.WithString("owner", mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Description(params.Repo)),
		mcp.WithString("name", mcp.Required(), mcp.Description(params.VariableName)),
		mcp.WithString("data", mcp.Required(), mcp.Description(params.VariableData)),
	)

	UpdateActionVariableTool = mcp.NewTool(
		UpdateActionVariableToolName,
		mcp.WithDescription("Update an action variable for scope user, org, or repo. Forgejo PUT; a missing name is an error. data is the value and is not echoed back. "+noGlobalActionVariables),
		mcp.WithString("scope", mcp.Required(), mcp.Description(params.ActionScope)),
		mcp.WithString("org", mcp.Description(params.Org)),
		mcp.WithString("owner", mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Description(params.Repo)),
		mcp.WithString("name", mcp.Required(), mcp.Description(params.VariableName)),
		mcp.WithString("data", mcp.Required(), mcp.Description(params.VariableData)),
	)

	DeleteActionVariableTool = mcp.NewTool(
		DeleteActionVariableToolName,
		mcp.WithDescription("Delete an action variable for scope user, org, or repo. A missing name is an error. "+noGlobalActionVariables),
		mcp.WithString("scope", mcp.Required(), mcp.Description(params.ActionScope)),
		mcp.WithString("org", mcp.Description(params.Org)),
		mcp.WithString("owner", mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Description(params.Repo)),
		mcp.WithString("name", mcp.Required(), mcp.Description(params.VariableName)),
	)
)

func ListActionVariablesFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListActionVariablesFn")
	coords, page, limit, err := listArgs(req.GetArguments())
	if err != nil {
		return to.ErrorResult(err)
	}

	client, err := forgejo.Client(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	opt := forgejo_sdk.ListOptions{Page: page, PageSize: limit}
	variables, resp, err := listVariables(client, coords, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("list action variables: %w", err))
	}

	rows := make([]actionVariableSummary, 0, len(variables))
	for _, variable := range variables {
		if variable == nil {
			continue
		}
		rows = append(rows, actionVariableSummary{
			Name:    variable.Name,
			OwnerID: variable.OwnerID,
			RepoID:  variable.RepoID,
		})
	}
	result := listActionVariablesResult{
		Scope:     coords.scope,
		Org:       coords.org,
		Owner:     coords.owner,
		Repo:      coords.repo,
		Variables: rows,
		Page:      page,
		Limit:     limit,
		Count:     len(rows),
	}
	if resp != nil && resp.Response != nil {
		result.TotalCount = forgejo.TotalCountPtr(resp.Header)
	}
	return to.TextResult(result)
}

func GetActionVariableFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetActionVariableFn")
	coords, name, err := namedArgs(req.GetArguments())
	if err != nil {
		return to.ErrorResult(err)
	}

	client, err := forgejo.Client(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	variable, err := getVariable(client, coords, name)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("get action variable: %w", err))
	}
	if variable == nil || variable.Name == "" {
		return to.ErrorResult(errors.New("action variable not found"))
	}
	return to.TextResult(actionVariable{
		Name:    variable.Name,
		Data:    variable.Data,
		OwnerID: variable.OwnerID,
		RepoID:  variable.RepoID,
	})
}

func CreateActionVariableFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called CreateActionVariableFn")
	coords, name, data, err := mutationArgs(req.GetArguments())
	if err != nil {
		return to.ErrorResult(err)
	}

	client, err := forgejo.Client(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	opt := forgejo_sdk.CreateVariableOption{Name: name, Data: data}
	if _, err = createVariable(client, coords, opt); err != nil {
		return to.ErrorResult(fmt.Errorf("create action variable: %w", err))
	}
	return to.TextResult(mutationResult(coords, name, "created"))
}

func UpdateActionVariableFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called UpdateActionVariableFn")
	coords, name, data, err := mutationArgs(req.GetArguments())
	if err != nil {
		return to.ErrorResult(err)
	}

	client, err := forgejo.Client(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	opt := forgejo_sdk.CreateVariableOption{Name: name, Data: data}
	if _, err = updateVariable(client, coords, name, opt); err != nil {
		return to.ErrorResult(fmt.Errorf("update action variable: %w", err))
	}
	return to.TextResult(mutationResult(coords, name, "updated"))
}

func DeleteActionVariableFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DeleteActionVariableFn")
	coords, name, err := namedArgs(req.GetArguments())
	if err != nil {
		return to.ErrorResult(err)
	}

	client, err := forgejo.Client(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	if _, err = deleteVariable(client, coords, name); err != nil {
		return to.ErrorResult(fmt.Errorf("delete action variable: %w", err))
	}
	return to.TextResult(mutationResult(coords, name, "deleted"))
}

func listVariables(client *forgejo_sdk.Client, coords actionScopeCoords, opt forgejo_sdk.ListOptions) ([]*forgejo_sdk.ActionVariable, *forgejo_sdk.Response, error) {
	switch coords.scope {
	case "user":
		return client.ListUserActionVariables(opt)
	case "org":
		return client.ListOrgActionVariables(coords.org, opt)
	case "repo":
		return client.ListRepoActionVariables(coords.owner, coords.repo, opt)
	default:
		return nil, nil, fmt.Errorf("unknown scope %q", coords.scope)
	}
}

func getVariable(client *forgejo_sdk.Client, coords actionScopeCoords, name string) (*forgejo_sdk.ActionVariable, error) {
	var (
		variable *forgejo_sdk.ActionVariable
		err      error
	)
	switch coords.scope {
	case "user":
		variable, _, err = client.GetUserActionVariable(name)
	case "org":
		variable, _, err = client.GetOrgActionVariable(coords.org, name)
	case "repo":
		variable, _, err = client.GetRepoActionVariable(coords.owner, coords.repo, name)
	default:
		err = fmt.Errorf("unknown scope %q", coords.scope)
	}
	return variable, err
}

func createVariable(client *forgejo_sdk.Client, coords actionScopeCoords, opt forgejo_sdk.CreateVariableOption) (*forgejo_sdk.Response, error) {
	switch coords.scope {
	case "user":
		return client.CreateUserActionVariable(opt)
	case "org":
		return client.CreateOrgActionVariable(coords.org, opt)
	case "repo":
		return client.CreateRepoActionVariable(coords.owner, coords.repo, opt)
	default:
		return nil, fmt.Errorf("unknown scope %q", coords.scope)
	}
}

func updateVariable(client *forgejo_sdk.Client, coords actionScopeCoords, name string, opt forgejo_sdk.CreateVariableOption) (*forgejo_sdk.Response, error) {
	switch coords.scope {
	case "user":
		return client.UpdateUserActionVariable(name, opt)
	case "org":
		return client.UpdateOrgActionVariable(coords.org, name, opt)
	case "repo":
		return client.UpdateRepoActionVariable(coords.owner, coords.repo, name, opt)
	default:
		return nil, fmt.Errorf("unknown scope %q", coords.scope)
	}
}

func deleteVariable(client *forgejo_sdk.Client, coords actionScopeCoords, name string) (*forgejo_sdk.Response, error) {
	switch coords.scope {
	case "user":
		return client.DeleteUserActionVariable(name)
	case "org":
		return client.DeleteOrgActionVariable(coords.org, name)
	case "repo":
		return client.DeleteRepoActionVariable(coords.owner, coords.repo, name)
	default:
		return nil, fmt.Errorf("unknown scope %q", coords.scope)
	}
}

func mutationResult(coords actionScopeCoords, name, status string) actionVariableMutationResult {
	return actionVariableMutationResult{
		Scope:  coords.scope,
		Org:    coords.org,
		Owner:  coords.owner,
		Repo:   coords.repo,
		Name:   name,
		Status: status,
	}
}

func listArgs(args map[string]any) (actionScopeCoords, int, int, error) {
	coords, err := actionCoords(args)
	if err != nil {
		return actionScopeCoords{}, 0, 0, err
	}
	page, err := boundedIntegerArg(args, "page", 1, 1, 1<<31-1)
	if err != nil {
		return actionScopeCoords{}, 0, 0, err
	}
	limit, err := boundedIntegerArg(args, "limit", defaultActionVariablesLimit, 1, maxActionVariablesLimit)
	if err != nil {
		return actionScopeCoords{}, 0, 0, err
	}
	return coords, page, limit, nil
}

func namedArgs(args map[string]any) (actionScopeCoords, string, error) {
	coords, err := actionCoords(args)
	if err != nil {
		return actionScopeCoords{}, "", err
	}
	name, err := requiredVariableName(args)
	if err != nil {
		return actionScopeCoords{}, "", err
	}
	return coords, name, nil
}

func mutationArgs(args map[string]any) (actionScopeCoords, string, string, error) {
	coords, name, err := namedArgs(args)
	if err != nil {
		return actionScopeCoords{}, "", "", err
	}
	data, _ := args["data"].(string)
	return coords, name, data, nil
}

func actionCoords(args map[string]any) (actionScopeCoords, error) {
	scope, _ := args["scope"].(string)
	switch scope {
	case "user":
		if err := rejectCoords(args, "org", "owner", "repo"); err != nil {
			return actionScopeCoords{}, err
		}
		return actionScopeCoords{scope: scope}, nil
	case "org":
		org, err := requiredOrg(args)
		if err != nil {
			return actionScopeCoords{}, err
		}
		if err := rejectCoords(args, "owner", "repo"); err != nil {
			return actionScopeCoords{}, err
		}
		return actionScopeCoords{scope: scope, org: org}, nil
	case "repo":
		owner, repo, err := requiredRepo(args)
		if err != nil {
			return actionScopeCoords{}, err
		}
		if err := rejectCoords(args, "org"); err != nil {
			return actionScopeCoords{}, err
		}
		return actionScopeCoords{scope: scope, owner: owner, repo: repo}, nil
	default:
		if scope == "" {
			return actionScopeCoords{}, errors.New("scope is required")
		}
		return actionScopeCoords{}, fmt.Errorf("scope must be user, org, or repo, got %q", scope)
	}
}

func rejectCoords(args map[string]any, keys ...string) error {
	for _, key := range keys {
		value, _ := args[key].(string)
		if value != "" {
			return fmt.Errorf("%s is not used for this scope", key)
		}
	}
	return nil
}

func requiredVariableName(args map[string]any) (string, error) {
	name, _ := args["name"].(string)
	if name == "" {
		return "", errors.New("name is required")
	}
	// SDK v3.0.0 CreateOrgActionVariable and CreateRepoActionVariable put
	// opt.Name into the path without escapeValidatePathSegments. A slash,
	// question mark, or hash would retarget that request. User create does
	// escape, and so do get, update, and delete. One rejection covers all.
	if strings.ContainsAny(name, "/?#") {
		return "", fmt.Errorf("name must be a single path segment, got %q", name)
	}
	return name, nil
}

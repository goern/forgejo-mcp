package repo

import (
	"context"
	"fmt"
	"regexp"

	"codeberg.org/goern/forgejo-mcp/v2/operation/params"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/forgejo"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/log"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/to"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v2"
	"github.com/mark3labs/mcp-go/mcp"
)

const (
	ListRepoLabelsToolName = "list_repo_labels"
	CreateLabelToolName    = "create_label"
	EditLabelToolName      = "edit_label"
)

var (
	ListRepoLabelsTool = mcp.NewTool(
		ListRepoLabelsToolName,
		mcp.WithDescription("List all repository labels"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(50)),
	)

	CreateLabelTool = mcp.NewTool(
		CreateLabelToolName,
		mcp.WithDescription("Create a new repository label"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("name", mcp.Required(), mcp.Description("Label name")),
		mcp.WithString("color", mcp.Required(), mcp.Description("Hex color (#RRGGBB)")),
		mcp.WithString("description", mcp.Description("Label description")),
	)

	EditLabelTool = mcp.NewTool(
		EditLabelToolName,
		mcp.WithDescription("Edit an existing label"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("id", mcp.Required(), mcp.Description("Label ID")),
		mcp.WithString("name", mcp.Description("New label name")),
		mcp.WithString("color", mcp.Description("New hex color (#RRGGBB)")),
		mcp.WithString("description", mcp.Description("New description")),
	)
)

// isValidHexColor validates that a color string is in #RRGGBB format
func isValidHexColor(color string) bool {
	matched, _ := regexp.MatchString("^#[0-9A-Fa-f]{6}$", color)
	return matched
}

func ListRepoLabelsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListRepoLabelsFn")
	owner, err := req.RequireString("owner")
	if err != nil {
		return to.ErrorResult(err)
	}
	repo, err := req.RequireString("repo")
	if err != nil {
		return to.ErrorResult(err)
	}
	page := req.GetFloat("page", 1)
	limit := req.GetFloat("limit", 50)

	opt := forgejo_sdk.ListLabelsOptions{
		ListOptions: forgejo_sdk.ListOptions{
			Page:     int(page),
			PageSize: int(limit),
		},
	}

	labels, _, err := forgejo.Client().ListRepoLabels(owner, repo, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("list repo labels err: %v", err))
	}
	return to.TextResult(labels)
}

func CreateLabelFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called CreateLabelFn")
	owner, err := req.RequireString("owner")
	if err != nil {
		return to.ErrorResult(err)
	}
	repo, err := req.RequireString("repo")
	if err != nil {
		return to.ErrorResult(err)
	}
	name, err := req.RequireString("name")
	if err != nil {
		return to.ErrorResult(err)
	}
	color, err := req.RequireString("color")
	if err != nil {
		return to.ErrorResult(err)
	}
	description := req.GetString("description", "")

	// Validate color format (#RRGGBB)
	if !isValidHexColor(color) {
		return to.ErrorResult(fmt.Errorf("invalid color format '%s': must be #RRGGBB", color))
	}

	opt := forgejo_sdk.CreateLabelOption{
		Name:        name,
		Color:       color,
		Description: description,
	}
	label, _, err := forgejo.Client().CreateLabel(owner, repo, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("create label err: %v", err))
	}
	return to.TextResult(label)
}

func EditLabelFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called EditLabelFn")
	owner, err := req.RequireString("owner")
	if err != nil {
		return to.ErrorResult(err)
	}
	repo, err := req.RequireString("repo")
	if err != nil {
		return to.ErrorResult(err)
	}
	id, err := req.RequireFloat("id")
	if err != nil {
		return to.ErrorResult(err)
	}
	name := req.GetString("name", "")
	color := req.GetString("color", "")
	description := req.GetString("description", "")

	// Validate at least one field is provided
	if name == "" && color == "" && description == "" {
		return to.ErrorResult(fmt.Errorf("at least one of name, color, or description must be provided"))
	}

	// Validate color format if provided
	if color != "" && !isValidHexColor(color) {
		return to.ErrorResult(fmt.Errorf("invalid color format '%s': must be #RRGGBB", color))
	}

	opt := forgejo_sdk.EditLabelOption{}

	// Only set fields that were provided (use pointers)
	if name != "" {
		opt.Name = &name
	}
	if color != "" {
		opt.Color = &color
	}
	if description != "" {
		opt.Description = &description
	}

	label, _, err := forgejo.Client().EditLabel(owner, repo, int64(id), opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("edit label err: %v", err))
	}
	return to.TextResult(label)
}

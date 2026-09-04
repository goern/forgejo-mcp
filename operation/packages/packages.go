// SPDX-License-Identifier: GPL-3.0-or-later

package packages

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/params"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forgejo"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/log"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/to"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	ListPackagesToolName     = "list_packages"
	GetPackageToolName       = "get_package"
	DeletePackageToolName    = "delete_package"
	ListPackageFilesToolName = "list_package_files"

	defaultPackagesLimit = 30
	maxPackagesLimit     = 50
)

// packageVersionAPI is the Forgejo package JSON we accept. Extra fields
// (nested User, Repository objects, creator) are dropped on unmarshal.
type packageVersionAPI struct {
	ID         int64  `json:"id"`
	Type       string `json:"type"`
	Name       string `json:"name"`
	Version    string `json:"version"`
	HTMLURL    string `json:"html_url"`
	CreatedAt  string `json:"created_at"`
	Repository *struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
}

type packageVersion struct {
	ID         int64  `json:"id"`
	Type       string `json:"type"`
	Name       string `json:"name"`
	Version    string `json:"version"`
	HTMLURL    string `json:"html_url,omitempty"`
	CreatedAt  string `json:"created_at,omitempty"`
	Repository string `json:"repository,omitempty"`
}

type packageFile struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256,omitempty"`
}

type listPackagesResult struct {
	Packages   []packageVersion `json:"packages"`
	Page       int              `json:"page"`
	Limit      int              `json:"limit"`
	Count      int              `json:"count"`
	HasNext    bool             `json:"has_next"`
	TotalCount *int             `json:"total_count,omitempty"`
}

type listPackageFilesResult struct {
	Files      []packageFile `json:"files"`
	Page       int           `json:"page"`
	Limit      int           `json:"limit"`
	Count      int           `json:"count"`
	HasNext    bool          `json:"has_next"`
	TotalCount int           `json:"total_count"`
}

type deletePackageResult struct {
	Owner   string `json:"owner"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Version string `json:"version"`
	Status  string `json:"status"`
}

var (
	GetPackageTool = mcp.NewTool(
		GetPackageToolName,
		mcp.WithDescription("Get one package version (id, type, name, version, html_url, created_at, repository full_name). Does not embed owner or creator users."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("type", mcp.Required(), mcp.Description(params.PackageType)),
		mcp.WithString("name", mcp.Required(), mcp.Description(params.PackageName)),
		mcp.WithString("version", mcp.Required(), mcp.Description(params.PackageVersion)),
	)

	DeletePackageTool = mcp.NewTool(
		DeletePackageToolName,
		mcp.WithDescription("Delete one package version. Removes that version only, not every version of the name. No preflight GET. HTTP 204 returns {owner, type, name, version, status: \"deleted\"}; 4xx/5xx stay errors."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("type", mcp.Required(), mcp.Description(params.PackageType)),
		mcp.WithString("name", mcp.Required(), mcp.Description(params.PackageName)),
		mcp.WithString("version", mcp.Required(), mcp.Description(params.PackageVersion)),
	)
)

func RegisterTool(s *server.MCPServer) {
	s.AddTool(ListPackagesTool, ListPackagesFn)
	s.AddTool(GetPackageTool, GetPackageFn)
	s.AddTool(DeletePackageTool, DeletePackageFn)
	s.AddTool(ListPackageFilesTool, ListPackageFilesFn)
}

func GetPackageFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetPackageFn")
	args := req.GetArguments()

	owner, packageType, name, version, err := requiredPackageCoords(args)
	if err != nil {
		return to.ErrorResult(err)
	}

	path := forgejo.APIPath("packages", owner, packageType, name, version)
	var src packageVersionAPI
	if err := forgejo.DoJSON(ctx, http.MethodGet, path, nil, &src); err != nil {
		return to.ErrorResult(fmt.Errorf("get package: %w", err))
	}
	return to.TextResult(projectPackageVersion(src))
}

func DeletePackageFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DeletePackageFn")
	args := req.GetArguments()

	owner, packageType, name, version, err := requiredPackageCoords(args)
	if err != nil {
		return to.ErrorResult(err)
	}

	path := forgejo.APIPath("packages", owner, packageType, name, version)
	if err := forgejo.DoJSON(ctx, http.MethodDelete, path, nil, nil); err != nil {
		return to.ErrorResult(fmt.Errorf("delete package: %w", err))
	}
	return to.TextResult(deletePackageResult{
		Owner:   owner,
		Type:    packageType,
		Name:    name,
		Version: version,
		Status:  "deleted",
	})
}

func projectPackageVersion(src packageVersionAPI) packageVersion {
	row := packageVersion{
		ID:        src.ID,
		Type:      src.Type,
		Name:      src.Name,
		Version:   src.Version,
		HTMLURL:   src.HTMLURL,
		CreatedAt: src.CreatedAt,
	}
	if src.Repository != nil {
		row.Repository = src.Repository.FullName
	}
	return row
}

func requiredOwner(args map[string]any) (string, error) {
	owner, ok := args["owner"].(string)
	if !ok || owner == "" {
		return "", errors.New("owner is required")
	}
	return owner, nil
}

func requiredPackageCoords(args map[string]any) (owner, packageType, name, version string, err error) {
	owner, err = requiredOwner(args)
	if err != nil {
		return "", "", "", "", err
	}
	packageType, _ = args["type"].(string)
	if packageType == "" {
		return "", "", "", "", errors.New("type is required")
	}
	name, _ = args["name"].(string)
	if name == "" {
		return "", "", "", "", errors.New("name is required")
	}
	version, _ = args["version"].(string)
	if version == "" {
		return "", "", "", "", errors.New("version is required")
	}
	return owner, packageType, name, version, nil
}

func boundedIntegerArg(args map[string]any, name string, defaultValue, minimum, maximum int) (int, error) {
	value, exists := args[name]
	if !exists || value == nil || value == "" {
		return defaultValue, nil
	}
	number, err := to.Float64(value)
	if err != nil || math.Trunc(number) != number || number < float64(minimum) || number > float64(maximum) {
		return 0, fmt.Errorf("%s must be an integer between %d and %d", name, minimum, maximum)
	}
	return int(number), nil
}

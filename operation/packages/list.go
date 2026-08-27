// SPDX-License-Identifier: GPL-3.0-or-later

package packages

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/params"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forgejo"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/log"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/to"

	"github.com/mark3labs/mcp-go/mcp"
)

var ListPackagesTool = mcp.NewTool(
	ListPackagesToolName,
	mcp.WithDescription("List package versions for a user or org (Forgejo SearchVersions: one row per version, not one per name). Optional type and q filter; page (default 1) and limit (default 30, maximum 50) are sent as query parameters. Returns {packages, page, limit, count} and total_count when Forgejo sets X-Total-Count. A missing owner is an error, not an empty list."),
	mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
	mcp.WithString("type", mcp.Description(params.PackageType)),
	mcp.WithString("q", mcp.Description(params.PackageQ)),
	mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1), mcp.Min(1)),
	mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(defaultPackagesLimit), mcp.Min(1), mcp.Max(maxPackagesLimit)),
)

func ListPackagesFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListPackagesFn")
	args := req.GetArguments()

	owner, err := requiredOwner(args)
	if err != nil {
		return to.ErrorResult(err)
	}
	page, err := boundedIntegerArg(args, "page", 1, 1, math.MaxInt32)
	if err != nil {
		return to.ErrorResult(err)
	}
	limit, err := boundedIntegerArg(args, "limit", defaultPackagesLimit, 1, maxPackagesLimit)
	if err != nil {
		return to.ErrorResult(err)
	}
	packageType, _ := args["type"].(string)
	q, _ := args["q"].(string)

	query := url.Values{}
	query.Set("page", strconv.Itoa(page))
	query.Set("limit", strconv.Itoa(limit))
	if packageType != "" {
		query.Set("type", packageType)
	}
	if q != "" {
		query.Set("q", q)
	}
	path := forgejo.APIPath("packages", owner) + "?" + query.Encode()

	rows := make([]packageVersionAPI, 0)
	header, err := forgejo.DoJSONWithHeader(ctx, http.MethodGet, path, nil, &rows)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("list packages: %w", err))
	}
	// A JSON null body unmarshals onto a nil slice and overwrites the make above.
	if rows == nil {
		rows = []packageVersionAPI{}
	}

	packages := make([]packageVersion, 0, len(rows))
	for _, row := range rows {
		packages = append(packages, projectPackageVersion(row))
	}

	return to.TextResult(listPackagesResult{
		Packages:   packages,
		Page:       page,
		Limit:      limit,
		Count:      len(packages),
		TotalCount: forgejo.TotalCountPtr(header),
	})
}

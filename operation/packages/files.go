// SPDX-License-Identifier: GPL-3.0-or-later

package packages

import (
	"context"
	"fmt"
	"math"
	"net/http"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/params"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forgejo"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/log"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/to"

	"github.com/mark3labs/mcp-go/mcp"
)

var ListPackageFilesTool = mcp.NewTool(
	ListPackageFilesToolName,
	mcp.WithDescription("List files of one package version. Forgejo returns the full file list with no paging; page (default 1) and limit (default 30, maximum 50) slice it client-side. Returns {files, page, limit, count, has_next, total_count}. total_count is the fetched list length (the files endpoint has no X-Total-Count). Each file is id, name, size, sha256."),
	mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
	mcp.WithString("type", mcp.Required(), mcp.Description(params.PackageType)),
	mcp.WithString("name", mcp.Required(), mcp.Description(params.PackageName)),
	mcp.WithString("version", mcp.Required(), mcp.Description(params.PackageVersion)),
	mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1), mcp.Min(1)),
	mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(defaultPackagesLimit), mcp.Min(1), mcp.Max(maxPackagesLimit)),
)

func ListPackageFilesFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListPackageFilesFn")
	args := req.GetArguments()

	owner, packageType, name, version, err := requiredPackageCoords(args)
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

	path := forgejo.APIPath("packages", owner, packageType, name, version, "files")
	allFiles := make([]packageFile, 0)
	if err := forgejo.DoJSON(ctx, http.MethodGet, path, nil, &allFiles); err != nil {
		return to.ErrorResult(fmt.Errorf("list package files: %w", err))
	}
	// A JSON null body unmarshals onto a nil slice and overwrites the make above.
	if allFiles == nil {
		allFiles = []packageFile{}
	}

	start64 := int64(page-1) * int64(limit)
	start := len(allFiles)
	if start64 < int64(len(allFiles)) {
		start = int(start64)
	}
	end := min(start+limit, len(allFiles))
	files := allFiles[start:end]
	hasNext := end < len(allFiles)

	return to.TextResult(listPackageFilesResult{
		Files:      files,
		Page:       page,
		Limit:      limit,
		Count:      len(files),
		HasNext:    hasNext,
		TotalCount: len(allFiles),
	})
}

// SPDX-License-Identifier: GPL-3.0-or-later

package actions

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

const (
	ListActionRunArtifactsToolName = "list_action_run_artifacts"
	GetActionArtifactToolName      = "get_action_artifact"

	defaultActionArtifactsLimit = 30
	maxActionArtifactsLimit     = 50
)

type actionArtifact struct {
	ID                 int64  `json:"id"`
	Name               string `json:"name"`
	RunID              int64  `json:"run_id"`
	SizeInBytes        int64  `json:"size_in_bytes"`
	Expired            bool   `json:"expired"`
	ExpiresAt          string `json:"expires_at,omitempty"`
	CreatedAt          string `json:"created_at,omitempty"`
	UpdatedAt          string `json:"updated_at,omitempty"`
	ArchiveDownloadURL string `json:"archive_download_url,omitempty"`
}

type listActionRunArtifactsResult struct {
	RunID      int64            `json:"run_id"`
	Artifacts  []actionArtifact `json:"artifacts"`
	Page       int              `json:"page"`
	Limit      int              `json:"limit"`
	Count      int              `json:"count"`
	TotalCount *int             `json:"total_count,omitempty"`
}

var (
	ListActionRunArtifactsTool = mcp.NewTool(
		ListActionRunArtifactsToolName,
		mcp.WithDescription("List artifacts of a workflow run. Server-paged: page (default 1) and limit (default 30, maximum 50) are sent as query parameters, optional name filters by artifact name. Returns {run_id, artifacts, page, limit, count} and total_count when Forgejo sets X-Total-Count. A missing run is an error, not an empty list. Metadata only — does not download zips."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("run_id", mcp.Required(), mcp.Description(params.RunID), mcp.Min(1)),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1), mcp.Min(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(defaultActionArtifactsLimit), mcp.Min(1), mcp.Max(maxActionArtifactsLimit)),
		mcp.WithString("name", mcp.Description(params.ArtifactName)),
	)

	GetActionArtifactTool = mcp.NewTool(
		GetActionArtifactToolName,
		mcp.WithDescription("Get metadata for one Actions artifact (id, name, run_id, size_in_bytes, expired, timestamps, archive_download_url). Does not download the zip."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("artifact_id", mcp.Required(), mcp.Description(params.ArtifactID), mcp.Min(1)),
	)
)

func ListActionRunArtifactsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListActionRunArtifactsFn")
	args := req.GetArguments()

	owner, repo, err := requiredRepo(args)
	if err != nil {
		return to.ErrorResult(err)
	}
	runID, err := positiveIntegerArg(args, "run_id", true)
	if err != nil {
		return to.ErrorResult(err)
	}
	page, err := boundedIntegerArg(args, "page", 1, 1, math.MaxInt32)
	if err != nil {
		return to.ErrorResult(err)
	}
	limit, err := boundedIntegerArg(args, "limit", defaultActionArtifactsLimit, 1, maxActionArtifactsLimit)
	if err != nil {
		return to.ErrorResult(err)
	}
	name, _ := args["name"].(string)

	query := url.Values{}
	query.Set("page", strconv.Itoa(page))
	query.Set("limit", strconv.Itoa(limit))
	if name != "" {
		query.Set("name", name)
	}
	path := forgejo.APIPath("repos", owner, repo, "actions", "runs", runID, "artifacts") + "?" + query.Encode()

	artifacts := make([]actionArtifact, 0)
	header, err := forgejo.DoJSONWithHeader(ctx, http.MethodGet, path, nil, &artifacts)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("list action run artifacts: %w", err))
	}
	// A JSON null body unmarshals onto a nil slice and overwrites the make above.
	if artifacts == nil {
		artifacts = []actionArtifact{}
	}

	return to.TextResult(listActionRunArtifactsResult{
		RunID:      runID,
		Artifacts:  artifacts,
		Page:       page,
		Limit:      limit,
		Count:      len(artifacts),
		TotalCount: forgejo.TotalCountPtr(header),
	})
}

func GetActionArtifactFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetActionArtifactFn")
	args := req.GetArguments()

	owner, repo, err := requiredRepo(args)
	if err != nil {
		return to.ErrorResult(err)
	}
	artifactID, err := positiveIntegerArg(args, "artifact_id", true)
	if err != nil {
		return to.ErrorResult(err)
	}

	path := forgejo.APIPath("repos", owner, repo, "actions", "artifacts", artifactID)
	var artifact actionArtifact
	if err := forgejo.DoJSON(ctx, http.MethodGet, path, nil, &artifact); err != nil {
		return to.ErrorResult(fmt.Errorf("get action artifact: %w", err))
	}
	return to.TextResult(artifact)
}

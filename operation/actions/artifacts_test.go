// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v2/pkg/forgejo"
	"github.com/mark3labs/mcp-go/mcp"
)

func TestListActionRunArtifactsFn_ServerPagedEnvelope(t *testing.T) {
	_, capture := setupActionAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set(forgejo.TotalCountHeader, "4")
		_, _ = w.Write([]byte(`[
			{"id":1,"name":"dist","run_id":42,"size_in_bytes":10,"expired":false,"archive_download_url":"https://example/1"}
		]`))
	})

	result, err := ListActionRunArtifactsFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"owner": "o", "repo": "r", "run_id": float64(42), "page": float64(2), "limit": float64(30), "name": "dist",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	decoded := decodeActionResult[listActionRunArtifactsResult](t, result)
	if capture.path != "/api/v1/repos/o/r/actions/runs/42/artifacts" {
		t.Fatalf("path: %s", capture.path)
	}
	if capture.rawQuery != "limit=30&name=dist&page=2" {
		t.Fatalf("query: %s", capture.rawQuery)
	}
	if decoded.RunID != 42 || decoded.Page != 2 || decoded.Limit != 30 || decoded.Count != 1 {
		t.Fatalf("envelope: %+v", decoded)
	}
	if decoded.TotalCount == nil || *decoded.TotalCount != 4 {
		t.Fatalf("total_count: %+v", decoded.TotalCount)
	}
	if len(decoded.Artifacts) != 1 || decoded.Artifacts[0].Name != "dist" {
		t.Fatalf("artifacts: %+v", decoded.Artifacts)
	}
}

func TestListActionRunArtifactsFn_OmitsTotalCountWhenHeaderAbsent(t *testing.T) {
	setupActionAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	})

	result, err := ListActionRunArtifactsFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"owner": "o", "repo": "r", "run_id": float64(42),
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := result.Content[0].(mcp.TextContent).Text
	if strings.Contains(text, "total_count") {
		t.Fatalf("total_count must be omitted when the header is absent: %s", text)
	}
	decoded := decodeActionResult[listActionRunArtifactsResult](t, result)
	if decoded.Count != 0 || decoded.Artifacts == nil {
		t.Fatalf("empty page: %+v", decoded)
	}
}

func TestListActionRunArtifactsFn_MissingRunIsError(t *testing.T) {
	setupActionAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"run not found"}`))
	})

	result, err := ListActionRunArtifactsFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"owner": "o", "repo": "r", "run_id": float64(99),
	}))
	if err == nil {
		t.Fatal("404 run must be an error, not an empty list")
	}
	if result != nil {
		t.Fatalf("expected nil result, got %+v", result)
	}
}

func TestGetActionArtifactFn_MetadataOnly(t *testing.T) {
	_, capture := setupActionAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": 9,
			"name": "dist",
			"run_id": 42,
			"size_in_bytes": 128,
			"expired": false,
			"expires_at": "2026-09-01T00:00:00Z",
			"created_at": "2026-08-24T00:00:00Z",
			"updated_at": "2026-08-24T00:00:00Z",
			"archive_download_url": "https://example/artifacts/9/zip"
		}`))
	})

	result, err := GetActionArtifactFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"owner": "o", "repo": "r", "artifact_id": float64(9),
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capture.path != "/api/v1/repos/o/r/actions/artifacts/9" {
		t.Fatalf("path: %s", capture.path)
	}
	if strings.HasSuffix(capture.path, "/zip") {
		t.Fatal("must not fetch the zip")
	}
	decoded := decodeActionResult[actionArtifact](t, result)
	if decoded.ID != 9 || decoded.Name != "dist" || decoded.RunID != 42 || decoded.SizeInBytes != 128 {
		t.Fatalf("artifact: %+v", decoded)
	}
	if decoded.ArchiveDownloadURL == "" || decoded.Expired {
		t.Fatalf("metadata: %+v", decoded)
	}
}

func TestGetActionArtifactFn_MissingArgsSendNoRequest(t *testing.T) {
	_, capture := setupActionAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("no request should be sent")
	})

	if _, err := GetActionArtifactFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"owner": "o", "repo": "r",
	})); err == nil {
		t.Fatal("expected error for missing artifact_id")
	}
	if capture.count != 0 {
		t.Fatal("request sent")
	}
}

func TestListActionRunArtifactsFn_OwnerSlashDoesNotRetarget(t *testing.T) {
	_, capture := setupActionAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	})

	_, err := ListActionRunArtifactsFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"owner": "o/x", "repo": "r", "run_id": float64(42),
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capture.escapedPath != "/api/v1/repos/o%2Fx/r/actions/runs/42/artifacts" {
		t.Fatalf("path retargeted: path=%s escaped=%s", capture.path, capture.escapedPath)
	}
}

func TestListActionRunArtifactsFn_JSONHasCountNotLocalTotal(t *testing.T) {
	setupActionAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set(forgejo.TotalCountHeader, "4")
		_, _ = w.Write([]byte(`[{"id":1,"name":"a","run_id":42}]`))
	})

	result, err := ListActionRunArtifactsFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"owner": "o", "repo": "r", "run_id": float64(42),
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := result.Content[0].(mcp.TextContent).Text
	var envelope struct {
		Result map[string]json.RawMessage `json:"Result"`
	}
	if err := json.Unmarshal([]byte(text), &envelope); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := envelope.Result["count"]; !ok {
		t.Fatalf("missing count: %s", text)
	}
	if string(envelope.Result["total_count"]) != "4" {
		t.Fatalf("total_count should be the server grand total, got %s", envelope.Result["total_count"])
	}
}

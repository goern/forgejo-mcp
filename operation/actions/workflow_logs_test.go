// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"codeberg.org/goern/forgejo-mcp/v2/pkg/flag"

	"github.com/mark3labs/mcp-go/mcp"
)

type actionRequestCapture struct {
	path      string
	rawQuery  string
	accept    string
	byteRange string
}

func setupActionAPIServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *actionRequestCapture) {
	t.Helper()
	capture := &actionRequestCapture{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capture.path = r.URL.Path
		capture.rawQuery = r.URL.RawQuery
		capture.accept = r.Header.Get("Accept")
		capture.byteRange = r.Header.Get("Range")
		handler(w, r)
	}))
	t.Cleanup(server.Close)
	flag.URL = server.URL
	flag.Token = "test-token"
	flag.UserAgent = "forgejo-mcp-test/0.0.1"
	return server, capture
}

func decodeActionResult[T any](t *testing.T, result *mcp.CallToolResult) T {
	t.Helper()
	if result == nil || len(result.Content) == 0 {
		t.Fatal("tool returned no content")
	}
	text, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected text content, got %T", result.Content[0])
	}
	var envelope struct {
		Result T `json:"Result"`
	}
	if err := json.Unmarshal([]byte(text.Text), &envelope); err != nil {
		t.Fatalf("decode result: %v; body=%s", err, text.Text)
	}
	return envelope.Result
}

func TestListActionRunJobsFn_PaginatesJobs(t *testing.T) {
	_, capture := setupActionAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"id":11,"run_id":42,"attempt":1,"name":"lint","status":"success","runs_on":["docker"]},
			{"id":12,"run_id":42,"attempt":2,"name":"test","status":"failure","task_id":99},
			{"id":13,"run_id":42,"attempt":1,"name":"build","status":"waiting"}
		]`))
	})

	result, err := ListActionRunJobsFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"owner": "o", "repo": "r", "run_id": float64(42), "page": float64(2), "limit": float64(2),
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	decoded := decodeActionResult[actionRunJobsResult](t, result)
	if capture.path != "/api/v1/repos/o/r/actions/runs/42/jobs" {
		t.Fatalf("path: %s", capture.path)
	}
	if decoded.TotalCount != 3 || len(decoded.Jobs) != 1 || decoded.Jobs[0].ID != 13 {
		t.Fatalf("unexpected page: %+v", decoded)
	}
	if decoded.HasNext || decoded.NextPage != 0 {
		t.Fatalf("last page advertised continuation: %+v", decoded)
	}
}

func TestListActionRunJobsFn_AdvertisesNextPage(t *testing.T) {
	setupActionAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"id":11,"run_id":42,"name":"lint"},
			{"id":12,"run_id":42,"name":"test"},
			{"id":13,"run_id":42,"name":"build"}
		]`))
	})

	result, err := ListActionRunJobsFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"owner": "o", "repo": "r", "run_id": float64(42), "page": float64(1), "limit": float64(2),
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	decoded := decodeActionResult[actionRunJobsResult](t, result)
	if !decoded.HasNext || decoded.NextPage != 2 || len(decoded.Jobs) != 2 {
		t.Fatalf("unexpected continuation metadata: %+v", decoded)
	}
}

func TestGetActionJobLogsFn_DefaultsToTailAndPassesAttempt(t *testing.T) {
	_, capture := setupActionAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Content-Range", "bytes 68-99/100")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte(strings.Repeat("x", 32)))
	})

	result, err := GetActionJobLogsFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"owner": "o", "repo": "r", "job_id": float64(7), "attempt": float64(2), "max_bytes": float64(32),
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	decoded := decodeActionResult[actionJobLogResult](t, result)
	if capture.path != "/api/v1/repos/o/r/actions/jobs/7/logs" || capture.rawQuery != "attempt=2" {
		t.Fatalf("request target: %s?%s", capture.path, capture.rawQuery)
	}
	if capture.accept != "text/plain" || capture.byteRange != "bytes=-32" {
		t.Fatalf("headers: accept=%q range=%q", capture.accept, capture.byteRange)
	}
	if !decoded.TruncatedBefore || decoded.TruncatedAfter || decoded.PreviousOffset == nil || *decoded.PreviousOffset != 36 {
		t.Fatalf("unexpected continuation metadata: %+v", decoded)
	}
	if decoded.BytesReturned != 32 || decoded.TotalBytes != 100 || decoded.Attempt != 2 {
		t.Fatalf("unexpected result metadata: %+v", decoded)
	}
}

func TestGetActionJobLogsFn_OffsetProvidesNextChunk(t *testing.T) {
	_, capture := setupActionAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Content-Range", "bytes 10-14/30")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte("error"))
	})

	result, err := GetActionJobLogsFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"owner": "o", "repo": "r", "job_id": float64(8), "offset": float64(10), "max_bytes": float64(5),
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	decoded := decodeActionResult[actionJobLogResult](t, result)
	if capture.byteRange != "bytes=10-14" {
		t.Fatalf("range: %s", capture.byteRange)
	}
	if decoded.Content != "error" || decoded.NextOffset == nil || *decoded.NextOffset != 15 {
		t.Fatalf("unexpected result: %+v", decoded)
	}
	if decoded.PreviousOffset == nil || *decoded.PreviousOffset != 5 {
		t.Fatalf("missing previous offset: %+v", decoded)
	}
}

func TestGetActionJobLogsFn_HandlesEmptyLog(t *testing.T) {
	setupActionAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
	})

	result, err := GetActionJobLogsFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"owner": "o", "repo": "r", "job_id": float64(8),
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	decoded := decodeActionResult[actionJobLogResult](t, result)
	if decoded.Content != "" || decoded.StartByte != 0 || decoded.EndByte != -1 || decoded.TotalBytes != 0 {
		t.Fatalf("unexpected empty log metadata: %+v", decoded)
	}
	if decoded.TruncatedBefore || decoded.TruncatedAfter || decoded.PreviousOffset != nil || decoded.NextOffset != nil {
		t.Fatalf("empty log advertised continuation: %+v", decoded)
	}
}

func TestGetActionJobLogsFn_RejectsInvalidBounds(t *testing.T) {
	for _, args := range []map[string]interface{}{
		{"owner": "o", "repo": "r", "job_id": float64(0)},
		{"owner": "o", "repo": "r", "job_id": math.Pow(2, 63)},
		{"owner": "o", "repo": "r", "job_id": float64(1), "attempt": float64(1.5)},
		{"owner": "o", "repo": "r", "job_id": float64(1), "offset": float64(-1)},
		{"owner": "o", "repo": "r", "job_id": float64(1), "offset": math.Pow(2, 63)},
		{"owner": "o", "repo": "r", "job_id": float64(1), "max_bytes": float64(maxActionLogBytes + 1)},
	} {
		if _, err := GetActionJobLogsFn(context.Background(), newCallToolRequest(args)); err == nil {
			t.Fatalf("expected validation error for %+v", args)
		}
	}
}

func TestGetActionJobLogsFn_RejectsInconsistentContentRange(t *testing.T) {
	setupActionAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Range", "bytes 0-99/100")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte("short"))
	})

	_, err := GetActionJobLogsFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"owner": "o", "repo": "r", "job_id": float64(1),
	}))
	if err == nil || !strings.Contains(err.Error(), "inconsistent Content-Range") {
		t.Fatalf("expected Content-Range error, got %v", err)
	}
}

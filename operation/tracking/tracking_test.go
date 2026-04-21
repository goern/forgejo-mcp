package tracking

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"codeberg.org/goern/forgejo-mcp/v2/pkg/forgejo"
	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
)

func newCallToolRequest(args map[string]any) mcp.CallToolRequest {
	return mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: args}}
}

type capturedRequest struct {
	method string
	path   string // URL path only
	query  string // raw query string (empty if absent)
	body   string
}

// mockServer records each incoming request and serves canned responses keyed
// by (method, pathPrefix). Tests assert against captured() afterwards.
func mockServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *[]capturedRequest) {
	t.Helper()
	captured := make([]capturedRequest, 0, 4)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		captured = append(captured, capturedRequest{
			method: r.Method,
			path:   r.URL.Path,
			query:  r.URL.RawQuery,
			body:   string(body),
		})
		w.Header().Set("Content-Type", "application/json")
		handler(w, r)
	}))
	client, err := forgejo_sdk.NewClient(srv.URL, forgejo_sdk.SetForgejoVersion("7.0.0"))
	if err != nil {
		t.Fatalf("creating test client: %v", err)
	}
	forgejo.SetClientForTesting(client)
	return srv, &captured
}

// --- resolveSeconds ----------------------------------------------------------

func TestResolveSeconds_Table(t *testing.T) {
	tests := []struct {
		name    string
		args    map[string]any
		want    int64
		wantErr string
	}{
		{name: "seconds_only", args: map[string]any{"seconds": 900.0}, want: 900},
		{name: "duration_only_15m", args: map[string]any{"duration": "15m"}, want: 900},
		{name: "duration_1h30m", args: map[string]any{"duration": "1h30m"}, want: 5400},
		{name: "both_rejected", args: map[string]any{"seconds": 60.0, "duration": "1m"}, wantErr: "exactly one"},
		{name: "neither_rejected", args: map[string]any{}, wantErr: "required"},
		{name: "negative_seconds", args: map[string]any{"seconds": -5.0}, wantErr: "positive"},
		{name: "zero_seconds", args: map[string]any{"seconds": 0.0}, wantErr: "positive"},
		{name: "negative_duration", args: map[string]any{"duration": "-30s"}, wantErr: "positive"},
		{name: "malformed_duration", args: map[string]any{"duration": "not a duration"}, wantErr: "invalid duration"},
		{name: "empty_duration_treated_as_absent", args: map[string]any{"duration": ""}, wantErr: "required"},
		{name: "string_seconds_parsed", args: map[string]any{"seconds": "120"}, want: 120},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveSeconds(tc.args)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("want error containing %q, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %d seconds, want %d", got, tc.want)
			}
		})
	}
}

// --- AddIssueTimeFn ---------------------------------------------------------

func TestAddIssueTimeFn_DurationInput(t *testing.T) {
	srv, captured := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/times") {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(forgejo_sdk.TrackedTime{ID: 42, Time: 900, UserName: "alice"})
	})
	defer srv.Close()

	req := newCallToolRequest(map[string]any{
		"owner":    "goern",
		"repo":     "forgejo-mcp",
		"index":    106.0,
		"duration": "15m",
	})
	res, err := AddIssueTimeFn(context.Background(), req)
	if err != nil {
		t.Fatalf("AddIssueTimeFn error: %v", err)
	}
	if res.IsError {
		t.Fatalf("expected success, got error result: %+v", res)
	}
	if len(*captured) != 1 {
		t.Fatalf("expected 1 API call, got %d", len(*captured))
	}
	got := (*captured)[0]
	if got.method != http.MethodPost {
		t.Errorf("method = %s, want POST", got.method)
	}
	if !strings.Contains(got.path, "/repos/goern/forgejo-mcp/issues/106/times") {
		t.Errorf("path = %s, want issues/106/times", got.path)
	}
	if !strings.Contains(got.body, `"time":900`) {
		t.Errorf("body = %s, want time:900", got.body)
	}
}

func TestAddIssueTimeFn_InvalidDuration_ReturnsStructuredError(t *testing.T) {
	// No mock server needed — should fail before any API call.
	srv, captured := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("handler should not be called on invalid input; got %s %s", r.Method, r.URL.Path)
	})
	defer srv.Close()

	req := newCallToolRequest(map[string]any{
		"owner":    "goern",
		"repo":     "forgejo-mcp",
		"index":    106.0,
		"duration": "bananas",
	})
	_, err := AddIssueTimeFn(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for malformed duration, got nil")
	}
	if !strings.Contains(err.Error(), "invalid duration") {
		t.Errorf("unexpected error: %v", err)
	}
	if len(*captured) != 0 {
		t.Errorf("should not hit API on validation failure, captured %d requests", len(*captured))
	}
}

func TestAddIssueTimeFn_CreatedAtRFC3339(t *testing.T) {
	srv, captured := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(forgejo_sdk.TrackedTime{ID: 1, Time: 60})
	})
	defer srv.Close()

	req := newCallToolRequest(map[string]any{
		"owner":      "o",
		"repo":       "r",
		"index":      1.0,
		"seconds":    60.0,
		"created_at": "2026-04-21T20:00:00Z",
	})
	if _, err := AddIssueTimeFn(context.Background(), req); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !strings.Contains((*captured)[0].body, `"created":"2026-04-21T20:00:00Z"`) {
		t.Errorf("expected created timestamp in body, got: %s", (*captured)[0].body)
	}
}

func TestAddIssueTimeFn_InvalidCreatedAt(t *testing.T) {
	srv, _ := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("handler should not be called; got %s %s", r.Method, r.URL.Path)
	})
	defer srv.Close()

	req := newCallToolRequest(map[string]any{
		"owner":      "o",
		"repo":       "r",
		"index":      1.0,
		"seconds":    60.0,
		"created_at": "yesterday afternoon",
	})
	_, err := AddIssueTimeFn(context.Background(), req)
	if err == nil || !strings.Contains(err.Error(), "invalid created_at") {
		t.Fatalf("expected invalid created_at error, got %v", err)
	}
}

// --- ListIssueTrackedTimesFn ------------------------------------------------

func TestListIssueTrackedTimesFn_FiltersPassed(t *testing.T) {
	srv, captured := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]forgejo_sdk.TrackedTime{{ID: 1, Time: 60}})
	})
	defer srv.Close()

	req := newCallToolRequest(map[string]any{
		"owner":  "o",
		"repo":   "r",
		"index":  42.0,
		"since":  "2026-01-01T00:00:00Z",
		"before": "2026-12-31T23:59:59Z",
	})
	if _, err := ListIssueTrackedTimesFn(context.Background(), req); err != nil {
		t.Fatalf("err: %v", err)
	}
	got := (*captured)[0].query
	if !strings.Contains(got, "since=2026-01-01") || !strings.Contains(got, "before=2026-12-31") {
		t.Errorf("query filters missing in URL query: %s", got)
	}
}

func TestListIssueTrackedTimesFn_InvalidSince(t *testing.T) {
	srv, _ := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("handler should not be called")
	})
	defer srv.Close()

	req := newCallToolRequest(map[string]any{
		"owner": "o",
		"repo":  "r",
		"index": 1.0,
		"since": "not-rfc3339",
	})
	if _, err := ListIssueTrackedTimesFn(context.Background(), req); err == nil {
		t.Fatal("expected error for invalid since")
	}
}

// --- ListRepoTrackedTimesFn -------------------------------------------------

func TestListRepoTrackedTimesFn_UserFilterPassed(t *testing.T) {
	srv, captured := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]forgejo_sdk.TrackedTime{})
	})
	defer srv.Close()

	req := newCallToolRequest(map[string]any{
		"owner": "o",
		"repo":  "r",
		"user":  "alice",
	})
	if _, err := ListRepoTrackedTimesFn(context.Background(), req); err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains((*captured)[0].query, "user=alice") {
		t.Errorf("expected user filter in URL query, got: %s", (*captured)[0].query)
	}
}

// --- ResetIssueTimeFn / DeleteIssueTimeEntryFn ------------------------------

func TestResetIssueTimeFn(t *testing.T) {
	srv, captured := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	defer srv.Close()

	req := newCallToolRequest(map[string]any{"owner": "o", "repo": "r", "index": 7.0})
	if _, err := ResetIssueTimeFn(context.Background(), req); err != nil {
		t.Fatalf("err: %v", err)
	}
	got := (*captured)[0]
	if got.method != http.MethodDelete || !strings.HasSuffix(got.path, "/issues/7/times") {
		t.Errorf("unexpected request: %+v", got)
	}
}

func TestDeleteIssueTimeEntryFn(t *testing.T) {
	srv, captured := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	defer srv.Close()

	req := newCallToolRequest(map[string]any{"owner": "o", "repo": "r", "index": 7.0, "time_id": 99.0})
	if _, err := DeleteIssueTimeEntryFn(context.Background(), req); err != nil {
		t.Fatalf("err: %v", err)
	}
	got := (*captured)[0]
	if got.method != http.MethodDelete || !strings.HasSuffix(got.path, "/issues/7/times/99") {
		t.Errorf("unexpected request: %+v", got)
	}
}

// --- Stopwatches ------------------------------------------------------------

func TestStartStopCancelStopwatch(t *testing.T) {
	srv, captured := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
	defer srv.Close()

	req := newCallToolRequest(map[string]any{"owner": "o", "repo": "r", "index": 3.0})

	if _, err := StartIssueStopwatchFn(context.Background(), req); err != nil {
		t.Fatalf("start err: %v", err)
	}
	if _, err := StopIssueStopwatchFn(context.Background(), req); err != nil {
		t.Fatalf("stop err: %v", err)
	}
	if _, err := CancelIssueStopwatchFn(context.Background(), req); err != nil {
		t.Fatalf("cancel err: %v", err)
	}

	if len(*captured) != 3 {
		t.Fatalf("expected 3 calls, got %d", len(*captured))
	}
	expectSuffixes := []string{"/issues/3/stopwatch/start", "/issues/3/stopwatch/stop", "/issues/3/stopwatch/delete"}
	for i, want := range expectSuffixes {
		if !strings.HasSuffix((*captured)[i].path, want) {
			t.Errorf("call %d: path %s, want suffix %s", i, (*captured)[i].path, want)
		}
	}
}

func TestListMyStopwatchesFn(t *testing.T) {
	srv, captured := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]forgejo_sdk.StopWatch{{Seconds: 123, IssueIndex: 4, RepoName: "r"}})
	})
	defer srv.Close()

	req := newCallToolRequest(map[string]any{})
	res, err := ListMyStopwatchesFn(context.Background(), req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.IsError {
		t.Fatalf("expected success")
	}
	if !strings.HasSuffix((*captured)[0].path, "/user/stopwatches") {
		t.Errorf("unexpected path: %s", (*captured)[0].path)
	}
}

// --- ListMyTrackedTimesFn ---------------------------------------------------

func TestListMyTrackedTimesFn(t *testing.T) {
	srv, captured := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]forgejo_sdk.TrackedTime{{ID: 1, Time: 60}})
	})
	defer srv.Close()

	req := newCallToolRequest(map[string]any{})
	if _, err := ListMyTrackedTimesFn(context.Background(), req); err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.HasSuffix((*captured)[0].path, "/user/times") {
		t.Errorf("unexpected path: %s", (*captured)[0].path)
	}
}

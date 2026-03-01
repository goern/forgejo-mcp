package actions

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v2"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/forgejo"
	"github.com/mark3labs/mcp-go/mcp"
)

func newCallToolRequest(args map[string]interface{}) mcp.CallToolRequest {
	return mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: args,
		},
	}
}

// setupDispatchMockServer creates a mock server that expects a workflow dispatch POST
// and returns the captured request body and recorded path.
func setupDispatchMockServer(t *testing.T, statusCode int) (*httptest.Server, *string, *[]byte) {
	t.Helper()
	var capturedPath string
	var capturedBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		if r.Body != nil {
			defer r.Body.Close()
			buf := make([]byte, 1024)
			n, _ := r.Body.Read(buf)
			capturedBody = buf[:n]
		}
		w.WriteHeader(statusCode)
	}))

	client, err := forgejo_sdk.NewClient(srv.URL, forgejo_sdk.SetForgejoVersion("7.0.0"))
	if err != nil {
		t.Fatalf("creating test client: %v", err)
	}
	forgejo.SetClientForTesting(client)

	return srv, &capturedPath, &capturedBody
}

func TestDispatchWorkflowFn_Success(t *testing.T) {
	srv, capturedPath, _ := setupDispatchMockServer(t, http.StatusNoContent)
	defer srv.Close()

	req := newCallToolRequest(map[string]interface{}{
		"owner":    "testowner",
		"repo":     "testrepo",
		"workflow": "ci.yml",
		"ref":      "main",
	})

	result, err := DispatchWorkflowFn(context.Background(), req)
	if err != nil {
		t.Fatalf("DispatchWorkflowFn returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("DispatchWorkflowFn returned tool error")
	}

	expectedPath := "/api/v1/repos/testowner/testrepo/actions/workflows/ci.yml/dispatches"
	if *capturedPath != expectedPath {
		t.Errorf("wrong path: got %q, want %q", *capturedPath, expectedPath)
	}
}

func TestDispatchWorkflowFn_WithInputs(t *testing.T) {
	srv, _, capturedBody := setupDispatchMockServer(t, http.StatusNoContent)
	defer srv.Close()

	req := newCallToolRequest(map[string]interface{}{
		"owner":    "testowner",
		"repo":     "testrepo",
		"workflow": "deploy.yml",
		"ref":      "v1.0.0",
		"inputs":   `{"environment": "production", "debug": "false"}`,
	})

	result, err := DispatchWorkflowFn(context.Background(), req)
	if err != nil {
		t.Fatalf("DispatchWorkflowFn returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("DispatchWorkflowFn returned tool error")
	}

	var body map[string]interface{}
	if err := json.Unmarshal(*capturedBody, &body); err != nil {
		t.Fatalf("unmarshaling captured body: %v", err)
	}

	inputs, ok := body["inputs"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected inputs in request body, got: %v", body)
	}
	if inputs["environment"] != "production" {
		t.Errorf("expected environment=production, got %v", inputs["environment"])
	}
}

func TestDispatchWorkflowFn_InvalidInputsJSON(t *testing.T) {
	srv, _, _ := setupDispatchMockServer(t, http.StatusNoContent)
	defer srv.Close()

	req := newCallToolRequest(map[string]interface{}{
		"owner":    "testowner",
		"repo":     "testrepo",
		"workflow": "ci.yml",
		"ref":      "main",
		"inputs":   `not-valid-json`,
	})

	_, err := DispatchWorkflowFn(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for invalid inputs JSON, got nil")
	}
}

func TestDispatchWorkflowFn_MissingOwner(t *testing.T) {
	req := newCallToolRequest(map[string]interface{}{
		"repo":     "testrepo",
		"workflow": "ci.yml",
		"ref":      "main",
	})

	_, err := DispatchWorkflowFn(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for missing owner, got nil")
	}
}

func TestDispatchWorkflowFn_MissingWorkflow(t *testing.T) {
	req := newCallToolRequest(map[string]interface{}{
		"owner": "testowner",
		"repo":  "testrepo",
		"ref":   "main",
	})

	_, err := DispatchWorkflowFn(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for missing workflow, got nil")
	}
}

// setupListRunsMockServer creates a mock server that returns a list of workflow runs.
func setupListRunsMockServer(t *testing.T, response interface{}, statusCode int) (*httptest.Server, *string) {
	t.Helper()
	var capturedPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		if response != nil {
			json.NewEncoder(w).Encode(response)
		}
	}))

	client, err := forgejo_sdk.NewClient(srv.URL, forgejo_sdk.SetForgejoVersion("7.0.0"))
	if err != nil {
		t.Fatalf("creating test client: %v", err)
	}
	forgejo.SetClientForTesting(client)

	return srv, &capturedPath
}

func TestListWorkflowRunsFn_ReturnsRuns(t *testing.T) {
	mockResponse := map[string]interface{}{
		"total_count": 2,
		"workflow_runs": []map[string]interface{}{
			{
				"id":         1,
				"title":      "Run CI",
				"status":     "success",
				"event":      "push",
				"commit_sha": "abc1234567890",
				"html_url":   "https://example.com/runs/1",
			},
			{
				"id":         2,
				"title":      "Deploy",
				"status":     "running",
				"event":      "workflow_dispatch",
				"commit_sha": "def9876543210",
				"html_url":   "https://example.com/runs/2",
			},
		},
	}

	srv, capturedPath := setupListRunsMockServer(t, mockResponse, http.StatusOK)
	defer srv.Close()

	req := newCallToolRequest(map[string]interface{}{
		"owner": "testowner",
		"repo":  "testrepo",
	})

	result, err := ListWorkflowRunsFn(context.Background(), req)
	if err != nil {
		t.Fatalf("ListWorkflowRunsFn returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("ListWorkflowRunsFn returned tool error")
	}

	expectedPath := "/api/v1/repos/testowner/testrepo/actions/runs"
	if *capturedPath != expectedPath {
		t.Errorf("wrong path: got %q, want %q", *capturedPath, expectedPath)
	}
}

func TestListWorkflowRunsFn_EmptyResult(t *testing.T) {
	mockResponse := map[string]interface{}{
		"total_count":   0,
		"workflow_runs": []interface{}{},
	}

	srv, _ := setupListRunsMockServer(t, mockResponse, http.StatusOK)
	defer srv.Close()

	req := newCallToolRequest(map[string]interface{}{
		"owner": "testowner",
		"repo":  "testrepo",
	})

	result, err := ListWorkflowRunsFn(context.Background(), req)
	if err != nil {
		t.Fatalf("ListWorkflowRunsFn returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("ListWorkflowRunsFn returned tool error")
	}
}

func TestListWorkflowRunsFn_MissingRepo(t *testing.T) {
	req := newCallToolRequest(map[string]interface{}{
		"owner": "testowner",
	})

	_, err := ListWorkflowRunsFn(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for missing repo, got nil")
	}
}

func TestGetWorkflowRunFn_ReturnsRun(t *testing.T) {
	mockResponse := map[string]interface{}{
		"id":         42,
		"title":      "CI Pipeline",
		"status":     "success",
		"event":      "push",
		"commit_sha": "deadbeef",
		"prettyref":  "refs/heads/main",
		"html_url":   "https://example.com/runs/42",
		"trigger_user": map[string]interface{}{
			"login":     "testuser",
			"full_name": "Test User",
		},
	}

	srv, capturedPath := setupListRunsMockServer(t, mockResponse, http.StatusOK)
	defer srv.Close()

	req := newCallToolRequest(map[string]interface{}{
		"owner":  "testowner",
		"repo":   "testrepo",
		"run_id": float64(42),
	})

	result, err := GetWorkflowRunFn(context.Background(), req)
	if err != nil {
		t.Fatalf("GetWorkflowRunFn returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("GetWorkflowRunFn returned tool error")
	}

	expectedPath := "/api/v1/repos/testowner/testrepo/actions/runs/42"
	if *capturedPath != expectedPath {
		t.Errorf("wrong path: got %q, want %q", *capturedPath, expectedPath)
	}
}

func TestGetWorkflowRunFn_InvalidRunID(t *testing.T) {
	req := newCallToolRequest(map[string]interface{}{
		"owner":  "testowner",
		"repo":   "testrepo",
		"run_id": float64(-1),
	})

	_, err := GetWorkflowRunFn(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for invalid run_id, got nil")
	}
}

func TestGetWorkflowRunFn_MissingRunID(t *testing.T) {
	req := newCallToolRequest(map[string]interface{}{
		"owner": "testowner",
		"repo":  "testrepo",
	})

	_, err := GetWorkflowRunFn(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for missing run_id, got nil")
	}
}

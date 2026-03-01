package repo

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"codeberg.org/goern/forgejo-mcp/v2/pkg/forgejo"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v2"
	"github.com/mark3labs/mcp-go/mcp"
)

// apiFileRequest mirrors the Forgejo API request body for create/update file.
type apiFileRequest struct {
	Content string `json:"content"`
}

func newCallToolRequest(args map[string]interface{}) mcp.CallToolRequest {
	return mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: args,
		},
	}
}

// setupMockServer creates an httptest server that captures the request body
// and returns a minimal valid FileResponse. It returns the server and a
// pointer to the captured request body bytes.
func setupMockServer(t *testing.T) (*httptest.Server, *[]byte) {
	t.Helper()
	var captured []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("reading request body: %v", err)
		}
		captured = body

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"content": map[string]interface{}{
				"name": "test.txt",
				"path": "test.txt",
				"sha":  "abc123",
			},
		})
	}))

	client, err := forgejo_sdk.NewClient(srv.URL, forgejo_sdk.SetForgejoVersion("7.0.0"))
	if err != nil {
		t.Fatalf("creating test client: %v", err)
	}
	forgejo.SetClientForTesting(client)

	return srv, &captured
}

func TestCreateFileFn_Base64EncodesContent(t *testing.T) {
	srv, captured := setupMockServer(t)
	defer srv.Close()

	plainText := "Hello, World!\nThis is a test file."

	req := newCallToolRequest(map[string]interface{}{
		"owner":       "testowner",
		"repo":        "testrepo",
		"filePath":    "test.txt",
		"content":     plainText,
		"message":     "add test file",
		"branch_name": "main",
	})

	result, err := CreateFileFn(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateFileFn returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("CreateFileFn returned tool error")
	}

	var body apiFileRequest
	if err := json.Unmarshal(*captured, &body); err != nil {
		t.Fatalf("unmarshaling captured body: %v", err)
	}

	expected := base64.StdEncoding.EncodeToString([]byte(plainText))
	if body.Content != expected {
		decoded, _ := base64.StdEncoding.DecodeString(body.Content)
		t.Errorf("content sent to API is not correctly base64-encoded\n  got decoded: %q\n  want:        %q", string(decoded), plainText)
	}
}

func TestUpdateFileFn_Base64EncodesContent(t *testing.T) {
	srv, captured := setupMockServer(t)
	defer srv.Close()

	plainText := "Updated content with special chars: <>&\"\n\ttabs too"

	req := newCallToolRequest(map[string]interface{}{
		"owner":       "testowner",
		"repo":        "testrepo",
		"filePath":    "test.txt",
		"content":     plainText,
		"message":     "update test file",
		"branch_name": "main",
		"sha":         "abc123",
	})

	result, err := UpdateFileFn(context.Background(), req)
	if err != nil {
		t.Fatalf("UpdateFileFn returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("UpdateFileFn returned tool error")
	}

	var body apiFileRequest
	if err := json.Unmarshal(*captured, &body); err != nil {
		t.Fatalf("unmarshaling captured body: %v", err)
	}

	expected := base64.StdEncoding.EncodeToString([]byte(plainText))
	if body.Content != expected {
		decoded, _ := base64.StdEncoding.DecodeString(body.Content)
		t.Errorf("content sent to API is not correctly base64-encoded\n  got decoded: %q\n  want:        %q", string(decoded), plainText)
	}
}

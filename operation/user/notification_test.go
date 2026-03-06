package user

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"codeberg.org/goern/forgejo-mcp/v2/pkg/forgejo"
	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v2"
	"github.com/mark3labs/mcp-go/mcp"
)

func newCallToolRequest(args map[string]interface{}) mcp.CallToolRequest {
	return mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: args,
		},
	}
}

// mockNotificationServer sets up a test HTTP server that responds with
// a predetermined set of notifications for testing CheckNotificationsFn.
func mockNotificationServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Basic validation of the request
		if r.URL.Path != "/api/v1/notifications" {
			t.Errorf("expected path /api/v1/notifications, got %s", r.URL.Path)
		}

		// Ensure only unread notifications are returned by default unless "all" is set in URL query
		// We can test this by looking at r.URL.Query().Get("status-types")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		notifications := []map[string]interface{}{
			{
				"id":         1,
				"unread":     true,
				"updated_at": time.Now().Format(time.RFC3339),
				"subject": map[string]interface{}{
					"title": "New Issue",
					"type":  "Issue",
				},
				"repository": map[string]interface{}{
					"full_name": "synapse/neural-net",
				},
			},
			{
				"id":         2,
				"unread":     false,
				"updated_at": time.Now().Format(time.RFC3339),
				"subject": map[string]interface{}{
					"title": "Fix bug",
					"type":  "Pull",
				},
				"repository": map[string]interface{}{
					"full_name": "synapse/yggdrasil",
				},
			},
		}
		_ = json.NewEncoder(w).Encode(notifications)
	}))

	client, err := forgejo_sdk.NewClient(srv.URL, forgejo_sdk.SetForgejoVersion("7.0.0"))
	if err != nil {
		t.Fatalf("creating test client: %v", err)
	}
	forgejo.SetClientForTesting(client)

	return srv
}

func TestCheckNotificationsFn(t *testing.T) {
	srv := mockNotificationServer(t)
	defer srv.Close()

	req := newCallToolRequest(map[string]interface{}{
		"all":   true,
		"limit": 10.0,
		"page":  1.0,
	})

	result, err := CheckNotificationsFn(context.Background(), req)
	if err != nil {
		t.Fatalf("CheckNotificationsFn returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("CheckNotificationsFn returned tool error")
	}

	// We expect the result text to contain our notifications in some JSON/stringified form
	if len(result.Content) == 0 {
		t.Fatalf("expected some content in result, got none")
	}

	textResult, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}

	if textResult.Text == "" {
		t.Errorf("expected non-empty text result")
	}
}

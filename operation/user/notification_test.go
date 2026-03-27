package user

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"codeberg.org/goern/forgejo-mcp/v2/pkg/forgejo"
	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
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
// a predetermined set of notifications for testing all notification endpoints.
func mockNotificationServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/api/v1/notifications":
			if r.Method == "GET" {
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
				}
				_ = json.NewEncoder(w).Encode(notifications)
			} else if r.Method == "PUT" {
				w.WriteHeader(http.StatusResetContent)
				w.Write([]byte("[]"))
			}

		case strings.HasPrefix(r.URL.Path, "/api/v1/notifications/threads/"):
			if r.Method == "GET" {
				w.WriteHeader(http.StatusOK)
				thread := map[string]interface{}{
					"id":         1,
					"unread":     true,
					"updated_at": time.Now().Format(time.RFC3339),
				}
				_ = json.NewEncoder(w).Encode(thread)
			} else if r.Method == "PATCH" {
				w.WriteHeader(http.StatusResetContent)
				w.Write([]byte("{}"))
			}

		case strings.HasPrefix(r.URL.Path, "/api/v1/repos/") && strings.HasSuffix(r.URL.Path, "/notifications"):
			if r.Method == "GET" {
				w.WriteHeader(http.StatusOK)
				notifications := []map[string]interface{}{
					{
						"id":     3,
						"unread": true,
					},
				}
				_ = json.NewEncoder(w).Encode(notifications)
			} else if r.Method == "PUT" {
				w.WriteHeader(http.StatusResetContent)
				w.Write([]byte("[]"))
			}

		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
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

	if len(result.Content) == 0 {
		t.Fatalf("expected some content in result, got none")
	}
}

func TestGetNotificationThreadFn(t *testing.T) {
	srv := mockNotificationServer(t)
	defer srv.Close()

	req := newCallToolRequest(map[string]interface{}{
		"id": 1.0,
	})

	result, err := GetNotificationThreadFn(context.Background(), req)
	if err != nil {
		t.Fatalf("GetNotificationThreadFn returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("GetNotificationThreadFn returned tool error")
	}
}

func TestMarkNotificationReadFn(t *testing.T) {
	srv := mockNotificationServer(t)
	defer srv.Close()

	req := newCallToolRequest(map[string]interface{}{
		"id": 1.0,
	})

	result, err := MarkNotificationReadFn(context.Background(), req)
	if err != nil {
		t.Fatalf("MarkNotificationReadFn returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("MarkNotificationReadFn returned tool error")
	}
}

func TestMarkAllNotificationsReadFn(t *testing.T) {
	srv := mockNotificationServer(t)
	defer srv.Close()

	req := newCallToolRequest(map[string]interface{}{
		"last_read_at": time.Now().Format(time.RFC3339),
	})

	result, err := MarkAllNotificationsReadFn(context.Background(), req)
	if err != nil {
		t.Fatalf("MarkAllNotificationsReadFn returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("MarkAllNotificationsReadFn returned tool error")
	}
}

func TestListRepoNotificationsFn(t *testing.T) {
	srv := mockNotificationServer(t)
	defer srv.Close()

	req := newCallToolRequest(map[string]interface{}{
		"owner": "synapse",
		"repo":  "neural-net",
		"limit": 10.0,
		"page":  1.0,
	})

	result, err := ListRepoNotificationsFn(context.Background(), req)
	if err != nil {
		t.Fatalf("ListRepoNotificationsFn returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("ListRepoNotificationsFn returned tool error")
	}
}

func TestMarkRepoNotificationsReadFn(t *testing.T) {
	srv := mockNotificationServer(t)
	defer srv.Close()

	req := newCallToolRequest(map[string]interface{}{
		"owner": "synapse",
		"repo":  "neural-net",
	})

	result, err := MarkRepoNotificationsReadFn(context.Background(), req)
	if err != nil {
		t.Fatalf("MarkRepoNotificationsReadFn returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("MarkRepoNotificationsReadFn returned tool error")
	}
}

// Package race_test reproduces the "concurrent map writes" panic from
// https://codeberg.org/goern/forgejo-mcp/issues/76
//
// Run:  go test -race -count=10 -timeout 120s ./test/race/
package race_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"codeberg.org/goern/forgejo-mcp/v2/operation"
	flagPkg "codeberg.org/goern/forgejo-mcp/v2/pkg/flag"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/forgejo"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// fakeAPI is a package-level test server so the forgejo.Client() singleton
// (initialized via sync.Once) always points to a live server.
var (
	fakeAPI  *httptest.Server
	setupMu  sync.Once
	mcpSrv   *server.MCPServer
	allTools []string
)

func setup(t *testing.T) {
	t.Helper()
	setupMu.Do(func() {
		fakeAPI = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")

			// Return realistic responses for common endpoints.
			switch {
			case r.URL.Path == "/api/v1/user":
				json.NewEncoder(w).Encode(map[string]interface{}{
					"id": 1, "login": "testuser", "full_name": "Test User",
					"email": "test@example.com", "avatar_url": "",
				})
			case r.URL.Path == "/api/v1/version":
				json.NewEncoder(w).Encode(map[string]interface{}{
					"version": "1.21.0",
				})
			case strings.Contains(r.URL.Path, "/issues/") && r.Method == http.MethodGet:
				json.NewEncoder(w).Encode(map[string]interface{}{
					"id": 1, "number": 1, "title": "test issue",
					"state": "open", "body": "body",
					"user": map[string]interface{}{"id": 1, "login": "testuser"},
				})
			case strings.Contains(r.URL.Path, "/pulls/") && r.Method == http.MethodGet:
				json.NewEncoder(w).Encode(map[string]interface{}{
					"id": 1, "number": 1, "title": "test pr",
					"state": "open", "body": "body",
					"user": map[string]interface{}{"id": 1, "login": "testuser"},
				})
			default:
				if r.Method == http.MethodGet {
					json.NewEncoder(w).Encode([]interface{}{})
				} else {
					json.NewEncoder(w).Encode(map[string]interface{}{
						"id": 1, "number": 1, "title": "test",
						"state": "open", "body": "test body",
						"user": map[string]interface{}{"id": 1, "login": "testuser"},
					})
				}
			}
		}))

		flagPkg.URL = fakeAPI.URL
		flagPkg.Token = "fake-token-for-testing"
		flagPkg.Debug = false

		// Force the forgejo client singleton to initialize against our fake server.
		_ = forgejo.Client()

		mcpSrv = server.NewMCPServer("forgejo-mcp", "test", server.WithLogging())
		operation.RegisterTool(mcpSrv)

		tools := mcpSrv.ListTools()
		allTools = make([]string, 0, len(tools))
		for name := range tools {
			allTools = append(allTools, name)
		}
	})
}

// minimalArgs returns the minimum required arguments for a tool.
func minimalArgs(toolName string) map[string]any {
	base := map[string]any{
		"owner": "testowner",
		"repo":  "testrepo",
	}
	switch toolName {
	case "get_issue_by_index", "update_issue", "issue_state_change", "add_issue_labels":
		base["index"] = float64(1)
	case "create_issue":
		base["title"] = "test issue"
	case "create_issue_comment":
		base["index"] = float64(1)
		base["body"] = "test comment"
	case "list_issue_comments":
		base["index"] = float64(1)
	case "get_issue_comment", "edit_issue_comment", "delete_issue_comment":
		base["comment_id"] = float64(1)
	case "get_pull_request", "merge_pull_request":
		base["index"] = float64(1)
	case "create_pull_request":
		base["title"] = "test pr"
		base["head"] = "feature"
		base["base"] = "main"
	case "get_pull_request_diff":
		base["index"] = float64(1)
	case "create_pull_request_review":
		base["index"] = float64(1)
		base["event"] = "COMMENT"
		base["body"] = "looks good"
	case "list_pull_request_reviews":
		base["index"] = float64(1)
	case "dismiss_pull_request_review":
		base["index"] = float64(1)
		base["review_id"] = float64(1)
	case "submit_pull_request_review":
		base["index"] = float64(1)
		base["review_id"] = float64(1)
		base["event"] = "COMMENT"
	case "get_file_content":
		base["filepath"] = "README.md"
	case "search_repos", "search_issues", "search_users":
		base["keyword"] = "test"
		delete(base, "owner")
		delete(base, "repo")
	case "search_org_teams":
		base["org"] = "testorg"
		delete(base, "owner")
		delete(base, "repo")
	case "get_my_user_info", "get_forgejo_version":
		delete(base, "owner")
		delete(base, "repo")
	case "create_branch":
		base["branch"] = "new-branch"
	case "fork_repo":
		// needs owner + repo (already set)
	}
	return base
}

// TestConcurrentToolCalls invokes all registered MCP tool handlers
// concurrently from multiple goroutines, simulating the mcp-go worker pool.
func TestConcurrentToolCalls(t *testing.T) {
	setup(t)
	t.Logf("registered %d tools", len(allTools))

	const concurrency = 20
	const iterations = 3

	for iter := 0; iter < iterations; iter++ {
		var wg sync.WaitGroup
		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for _, toolName := range allTools {
					st := mcpSrv.GetTool(toolName)
					if st == nil {
						continue
					}
					req := mcp.CallToolRequest{
						Params: mcp.CallToolParams{
							Name:      toolName,
							Arguments: minimalArgs(toolName),
						},
					}
					// We don't care about errors — only panics / races.
					_, _ = st.Handler(context.Background(), req)
				}
			}()
		}
		wg.Wait()
	}
}

// TestConcurrentSameToolRepeated hammers each tool individually from many goroutines.
func TestConcurrentSameToolRepeated(t *testing.T) {
	setup(t)

	for _, name := range allTools {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			st := mcpSrv.GetTool(name)
			if st == nil {
				t.Skip("tool not found")
			}

			var wg sync.WaitGroup
			for i := 0; i < 50; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for j := 0; j < 10; j++ {
						req := mcp.CallToolRequest{
							Params: mcp.CallToolParams{
								Name:      name,
								Arguments: minimalArgs(name),
							},
						}
						_, _ = st.Handler(context.Background(), req)
					}
				}()
			}
			wg.Wait()
		})
	}
}

// TestConcurrentListAndRegister tests for races between listing tools
// and registering tools concurrently.
func TestConcurrentListAndRegister(t *testing.T) {
	srv := server.NewMCPServer("forgejo-mcp", "test", server.WithLogging())

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		operation.RegisterTool(srv)
	}()

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = srv.ListTools()
			}
		}()
	}
	wg.Wait()
}

// TestInitFlagParseBug documents that cmd.init() calls flag.Parse() on the
// global flag.CommandLine, preventing `go test ./cmd/` from working.
func TestInitFlagParseBug(t *testing.T) {
	t.Log("cmd.init() calls flag.Parse() on global CommandLine, " +
		"preventing 'go test ./cmd/' from running. " +
		"This test documents the issue (it lives in test/race/ to avoid it).")
}

// TestNilResponseDeref documents a nil-pointer bug in tool handlers.
// Many handlers access resp.StatusCode BEFORE checking err != nil.
// When the forgejo client returns (nil, nil, err), this panics.
// Example: operation/user/user.go:44
//
//	user, resp, err := forgejo.Client().GetMyUserInfo()
//	forgejo.LogAPICall(ctx, "GET", "/user", duration, resp.StatusCode, err) // CRASH if resp==nil
//	if err != nil { ... }
//
// This is a separate bug but was discovered while investigating #76.
func TestNilResponseDeref(t *testing.T) {
	t.Log("Many tool handlers access resp.StatusCode before checking err. " +
		"If the API call returns (nil, nil, err), resp.StatusCode panics. " +
		"Fix: check err before accessing resp, or guard with 'if resp != nil'.")
}

package pull

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"codeberg.org/goern/forgejo-mcp/v2/pkg/flag"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/forgejo"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
)

const multiFileDiff = `diff --git a/cmd/main.go b/cmd/main.go
index 1234..5678 100644
--- a/cmd/main.go
+++ b/cmd/main.go
@@ -1,3 +1,4 @@
 package main

+import "fmt"
 func main() {}
diff --git a/README.md b/README.md
index abcd..ef01 100644
--- a/README.md
+++ b/README.md
@@ -10,2 +10,3 @@
 line ten
+line eleven
 line twelve`

func newDiffBackend(t *testing.T, diff string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/version", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":"11.0.0+gitea-1.22.0"}`))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// SDK calls /repos/{owner}/{repo}/pulls/{index}.diff
		if strings.HasSuffix(r.URL.Path, ".diff") {
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte(diff))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	flag.URL = srv.URL
	flag.Token = "tkn"
	flag.UserAgent = "test"

	c, err := forgejo_sdk.NewClient(srv.URL,
		forgejo_sdk.SetToken("tkn"),
		forgejo_sdk.SetUserAgent("test"),
	)
	if err != nil {
		t.Fatalf("failed to build SDK client: %v", err)
	}
	forgejo.SetClientForTesting(c)
	return srv
}

func makeReq(args map[string]any) mcp.CallToolRequest {
	return mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: args}}
}

func textOf(res *mcp.CallToolResult) string {
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}

func TestGetPullRequestDiff_NoFilePathReturnsFullDiff(t *testing.T) {
	_ = newDiffBackend(t, multiFileDiff)
	res, err := GetPullRequestDiffFn(context.Background(), makeReq(map[string]any{
		"owner": "goern", "repo": "forgejo-mcp", "index": float64(42),
	}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("err=%v res=%+v", err, res)
	}
	if textOf(res) != multiFileDiff {
		t.Fatalf("expected full diff back, got:\n%s", textOf(res))
	}
}

func TestGetPullRequestDiff_FilePathReturnsSlice(t *testing.T) {
	_ = newDiffBackend(t, multiFileDiff)
	res, err := GetPullRequestDiffFn(context.Background(), makeReq(map[string]any{
		"owner": "goern", "repo": "forgejo-mcp", "index": float64(42),
		"file_path": "cmd/main.go",
	}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("err=%v res=%+v", err, res)
	}
	out := textOf(res)
	if !strings.Contains(out, "diff --git a/cmd/main.go b/cmd/main.go") {
		t.Fatalf("expected slice header, got:\n%s", out)
	}
	if strings.Contains(out, "README.md") {
		t.Fatalf("slice should not contain other files:\n%s", out)
	}
}

func TestGetPullRequestDiff_FilePathNotFound(t *testing.T) {
	_ = newDiffBackend(t, multiFileDiff)
	_, err := GetPullRequestDiffFn(context.Background(), makeReq(map[string]any{
		"owner": "goern", "repo": "forgejo-mcp", "index": float64(42),
		"file_path": "does/not/exist.go",
	}))
	if err == nil {
		t.Fatal("expected error for missing file_path, got nil")
	}
	if !strings.Contains(err.Error(), "does/not/exist.go") {
		t.Fatalf("expected error to name the missing path, got %v", err)
	}
}

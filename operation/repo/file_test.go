package repo

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"codeberg.org/goern/forgejo-mcp/v2/pkg/forgejo"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
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

// setupRawMockServer creates an httptest server that serves raw file content,
// simulating the Forgejo /raw/ endpoint used by GetFile.
func setupRawMockServer(t *testing.T, responseBody string, statusCode int) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(statusCode)
		fmt.Fprint(w, responseBody)
	}))

	client, err := forgejo_sdk.NewClient(srv.URL, forgejo_sdk.SetForgejoVersion("7.0.0"))
	if err != nil {
		t.Fatalf("creating test client: %v", err)
	}
	forgejo.SetClientForTesting(client)

	return srv
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

// setupContentsResponseMockServer creates an httptest server that returns a
// proper Forgejo ContentsResponse (for the GetContents/with_metadata path).
func setupContentsResponseMockServer(t *testing.T) *httptest.Server {
	t.Helper()
	encodedContent := base64.StdEncoding.EncodeToString([]byte("binary-like content"))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"type":"file","name":"README.md","path":"README.md","sha":"abc123","content":"%s","encoding":"base64"}`, encodedContent)
	}))

	client, err := forgejo_sdk.NewClient(srv.URL, forgejo_sdk.SetForgejoVersion("7.0.0"))
	if err != nil {
		t.Fatalf("creating test client: %v", err)
	}
	forgejo.SetClientForTesting(client)

	return srv
}

func TestGetFileContentFn_ReturnsPlainText(t *testing.T) {
	plainText := "Hello from Forgejo!\nSecond line.\n"
	srv := setupRawMockServer(t, plainText, http.StatusOK)
	defer srv.Close()

	req := newCallToolRequest(map[string]interface{}{
		"owner":    "testowner",
		"repo":     "testrepo",
		"ref":      "main",
		"filePath": "README.md",
	})

	result, err := GetFileContentFn(context.Background(), req)
	if err != nil {
		t.Fatalf("GetFileContentFn returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("GetFileContentFn returned tool error")
	}

	// Result is wrapped as {"Result":"<text>"} — unmarshal to compare string value.
	text := result.Content[0].(mcp.TextContent).Text
	var wrapper struct{ Result string }
	if err := json.Unmarshal([]byte(text), &wrapper); err != nil {
		t.Fatalf("response is not valid JSON: %v\n  got: %q", err, text)
	}
	if wrapper.Result != plainText {
		t.Errorf("plain text mismatch\n  got:  %q\n  want: %q", wrapper.Result, plainText)
	}
}

func TestGetFileContentFn_WithMetadataReturnsContentsResponse(t *testing.T) {
	// with_metadata=true must route through GetContents and return a JSON ContentsResponse,
	// not plain text. We verify the response is valid JSON with a "Result" wrapper.
	srv := setupContentsResponseMockServer(t)
	defer srv.Close()

	req := newCallToolRequest(map[string]interface{}{
		"owner":         "testowner",
		"repo":          "testrepo",
		"ref":           "main",
		"filePath":      "README.md",
		"with_metadata": true,
	})

	result, err := GetFileContentFn(context.Background(), req)
	if err != nil {
		t.Fatalf("GetFileContentFn (with_metadata=true) returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("GetFileContentFn (with_metadata=true) returned tool error")
	}

	text := result.Content[0].(mcp.TextContent).Text
	var wrapper map[string]interface{}
	if err := json.Unmarshal([]byte(text), &wrapper); err != nil {
		t.Errorf("with_metadata=true response is not valid JSON: %v\n  got: %q", err, text)
	}
	if _, ok := wrapper["Result"]; !ok {
		t.Errorf("with_metadata=true response missing 'Result' key\n  got: %q", text)
	}
}

// fileContentText pulls the plain-text payload out of the
// {"Result":"..."} wrapper that to.TextResult emits, so range
// assertions can compare raw line content.
func fileContentText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	text := result.Content[0].(mcp.TextContent).Text
	var wrapper struct{ Result string }
	if err := json.Unmarshal([]byte(text), &wrapper); err != nil {
		t.Fatalf("response is not valid JSON: %v\n  got: %q", err, text)
	}
	return wrapper.Result
}

func TestGetFileContentFn_LineRange_InRange(t *testing.T) {
	body := "one\ntwo\nthree\nfour\nfive\n"
	srv := setupRawMockServer(t, body, http.StatusOK)
	defer srv.Close()

	req := newCallToolRequest(map[string]interface{}{
		"owner": "o", "repo": "r", "ref": "main", "filePath": "f.txt",
		"start_line": float64(2), "end_line": float64(4),
	})
	result, err := GetFileContentFn(context.Background(), req)
	if err != nil || result.IsError {
		t.Fatalf("err=%v isError=%v", err, result != nil && result.IsError)
	}
	if got, want := fileContentText(t, result), "two\nthree\nfour"; got != want {
		t.Errorf("range 2-4 mismatch:\n  got:  %q\n  want: %q", got, want)
	}
}

func TestGetFileContentFn_LineRange_StartOmittedDefaultsToOne(t *testing.T) {
	body := "one\ntwo\nthree\nfour"
	srv := setupRawMockServer(t, body, http.StatusOK)
	defer srv.Close()

	req := newCallToolRequest(map[string]interface{}{
		"owner": "o", "repo": "r", "ref": "main", "filePath": "f.txt",
		"end_line": float64(2),
	})
	result, _ := GetFileContentFn(context.Background(), req)
	if got, want := fileContentText(t, result), "one\ntwo"; got != want {
		t.Errorf("end-only mismatch:\n  got:  %q\n  want: %q", got, want)
	}
}

func TestGetFileContentFn_LineRange_EndOmittedDefaultsToCount(t *testing.T) {
	body := "one\ntwo\nthree\nfour"
	srv := setupRawMockServer(t, body, http.StatusOK)
	defer srv.Close()

	req := newCallToolRequest(map[string]interface{}{
		"owner": "o", "repo": "r", "ref": "main", "filePath": "f.txt",
		"start_line": float64(3),
	})
	result, _ := GetFileContentFn(context.Background(), req)
	if got, want := fileContentText(t, result), "three\nfour"; got != want {
		t.Errorf("start-only mismatch:\n  got:  %q\n  want: %q", got, want)
	}
}

func TestGetFileContentFn_LineRange_EndBeyondClamps(t *testing.T) {
	body := "one\ntwo\nthree"
	srv := setupRawMockServer(t, body, http.StatusOK)
	defer srv.Close()

	req := newCallToolRequest(map[string]interface{}{
		"owner": "o", "repo": "r", "ref": "main", "filePath": "f.txt",
		"start_line": float64(2), "end_line": float64(999),
	})
	result, err := GetFileContentFn(context.Background(), req)
	if err != nil || result.IsError {
		t.Fatalf("err=%v isError=%v", err, result != nil && result.IsError)
	}
	if got, want := fileContentText(t, result), "two\nthree"; got != want {
		t.Errorf("clamped range mismatch:\n  got:  %q\n  want: %q", got, want)
	}
}

func TestGetFileContentFn_LineRange_StartBelowOneClamps(t *testing.T) {
	body := "one\ntwo\nthree"
	srv := setupRawMockServer(t, body, http.StatusOK)
	defer srv.Close()

	req := newCallToolRequest(map[string]interface{}{
		"owner": "o", "repo": "r", "ref": "main", "filePath": "f.txt",
		"start_line": float64(-5), "end_line": float64(2),
	})
	result, _ := GetFileContentFn(context.Background(), req)
	if got, want := fileContentText(t, result), "one\ntwo"; got != want {
		t.Errorf("start clamp mismatch:\n  got:  %q\n  want: %q", got, want)
	}
}

func TestGetFileContentFn_LineRange_InvertedAfterClamping(t *testing.T) {
	body := "one\ntwo\nthree"
	srv := setupRawMockServer(t, body, http.StatusOK)
	defer srv.Close()

	req := newCallToolRequest(map[string]interface{}{
		"owner": "o", "repo": "r", "ref": "main", "filePath": "f.txt",
		"start_line": float64(20), "end_line": float64(10),
	})
	_, err := GetFileContentFn(context.Background(), req)
	if err == nil {
		t.Fatal("expected error on inverted range")
	}
}

func TestGetFileContentFn_LineRange_BothOmittedReturnsFull(t *testing.T) {
	body := "one\ntwo\nthree\n"
	srv := setupRawMockServer(t, body, http.StatusOK)
	defer srv.Close()

	req := newCallToolRequest(map[string]interface{}{
		"owner": "o", "repo": "r", "ref": "main", "filePath": "f.txt",
	})
	result, _ := GetFileContentFn(context.Background(), req)
	if got := fileContentText(t, result); got != body {
		t.Errorf("expected full body, got %q want %q", got, body)
	}
}

func TestGetFileContentFn_LineRange_IgnoredWithMetadata(t *testing.T) {
	// with_metadata=true takes the GetContents branch and must not apply
	// start_line/end_line — the response carries base64 content whose
	// semantics line-slicing would break.
	srv := setupContentsResponseMockServer(t)
	defer srv.Close()

	req := newCallToolRequest(map[string]interface{}{
		"owner": "o", "repo": "r", "ref": "main", "filePath": "README.md",
		"with_metadata": true,
		"start_line":    float64(5), "end_line": float64(10),
	})
	result, err := GetFileContentFn(context.Background(), req)
	if err != nil || result.IsError {
		t.Fatalf("err=%v isError=%v", err, result != nil && result.IsError)
	}
	text := result.Content[0].(mcp.TextContent).Text
	if !contains(text, `"sha":"abc123"`) {
		t.Fatalf("expected ContentsResponse with sha when with_metadata=true, got %q", text)
	}
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

func TestGetFileContentFn_DefaultIsPlainText(t *testing.T) {
	// Omitting with_metadata must behave the same as with_metadata=false (plain text default).
	plainText := "package main\n\nfunc main() {}\n"
	srv := setupRawMockServer(t, plainText, http.StatusOK)
	defer srv.Close()

	req := newCallToolRequest(map[string]interface{}{
		"owner":    "testowner",
		"repo":     "testrepo",
		"ref":      "main",
		"filePath": "main.go",
		// with_metadata intentionally omitted
	})

	result, err := GetFileContentFn(context.Background(), req)
	if err != nil {
		t.Fatalf("GetFileContentFn returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("GetFileContentFn returned tool error")
	}

	text := result.Content[0].(mcp.TextContent).Text
	var wrapper struct{ Result string }
	if err := json.Unmarshal([]byte(text), &wrapper); err != nil {
		t.Fatalf("response is not valid JSON: %v\n  got: %q", err, text)
	}
	if wrapper.Result != plainText {
		t.Errorf("default (no with_metadata) did not return plain text\n  got:  %q\n  want: %q", wrapper.Result, plainText)
	}
}

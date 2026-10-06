// SPDX-License-Identifier: GPL-3.0-or-later

package repo

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/flag"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forgejo"
)

func TestChangeFilesFnSendsAtomicBatch(t *testing.T) {
	var captured []byte
	var capturedPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		var err error
		captured, err = io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"sha":"commit123"}`))
	}))
	defer srv.Close()

	flag.URL = srv.URL
	flag.Token = "test"
	flag.UserAgent = "forgejo-mcp-test/0.0.1"

	client, err := forgejo_sdk.NewClient(
		srv.URL,
		forgejo_sdk.SetForgejoVersion("7.0.0"),
	)
	if err != nil {
		t.Fatalf("creating client: %v", err)
	}
	forgejo.SetClientForTesting(client)

	req := newCallToolRequest(map[string]interface{}{
		"owner":       "testowner",
		"repo":        "testrepo",
		"message":     "atomic change",
		"branch_name": "main",
		"files": []any{
			map[string]any{
				"operation": "create",
				"path":      "new.txt",
				"content":   "hello",
			},
			map[string]any{
				"operation":      "update",
				"path":           "binary.dat",
				"content_base64": "AP8=",
				"sha":            "oldsha",
			},
			map[string]any{
				"operation": "delete",
				"path":      "old.txt",
				"sha":       "deletesha",
			},
		},
	})

	result, err := ChangeFilesFn(context.Background(), req)
	if err != nil {
		t.Fatalf("ChangeFilesFn returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("ChangeFilesFn returned tool error: %v", result.Content)
	}

	if capturedPath != "/api/v1/repos/testowner/testrepo/contents" {
		t.Fatalf("unexpected request path: %q", capturedPath)
	}

	var body changeFilesOptions
	if err := json.Unmarshal(captured, &body); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}

	if body.Message != "atomic change" || body.BranchName != "main" {
		t.Fatalf("unexpected request envelope: %+v", body)
	}
	if len(body.Files) != 3 {
		t.Fatalf("expected 3 file operations, got %d", len(body.Files))
	}
	if body.Files[0].Content == nil || *body.Files[0].Content != "aGVsbG8=" {
		t.Fatalf("plaintext create was not encoded correctly: %+v", body.Files[0])
	}
	if body.Files[1].Content == nil || *body.Files[1].Content != "AP8=" {
		t.Fatalf("binary update changed supplied base64: %+v", body.Files[1])
	}
	if body.Files[2].Content != nil {
		t.Fatalf("delete unexpectedly contains content: %+v", body.Files[2])
	}
}

func TestParseChangeFilesArgumentsRejectsInvalidBatches(t *testing.T) {
	base := func(files []any) map[string]any {
		return map[string]any{
			"owner":       "o",
			"repo":        "r",
			"message":     "m",
			"branch_name": "main",
			"files":       files,
		}
	}

	tests := []struct {
		name string
		args map[string]any
		want string
	}{
		{
			name: "empty batch",
			args: base([]any{}),
			want: "at least one",
		},
		{
			name: "duplicate path",
			args: base([]any{
				map[string]any{"operation": "create", "path": "a", "content": "x"},
				map[string]any{"operation": "create", "path": "a", "content": "y"},
			}),
			want: "duplicate target path",
		},
		{
			name: "create with sha",
			args: base([]any{
				map[string]any{
					"operation": "create",
					"path":      "a",
					"content":   "x",
					"sha":       "abc",
				},
			}),
			want: "sha must be omitted",
		},
		{
			name: "update without sha",
			args: base([]any{
				map[string]any{
					"operation": "update",
					"path":      "a",
					"content":   "x",
				},
			}),
			want: "sha is required",
		},
		{
			name: "delete without sha",
			args: base([]any{
				map[string]any{"operation": "delete", "path": "a"},
			}),
			want: "sha is required",
		},
		{
			name: "delete with content",
			args: base([]any{
				map[string]any{
					"operation": "delete",
					"path":      "a",
					"sha":       "abc",
					"content":   "",
				},
			}),
			want: "content must be omitted",
		},
		{
			name: "invalid operation",
			args: base([]any{
				map[string]any{"operation": "rename", "path": "a"},
			}),
			want: "unsupported operation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, _, err := parseChangeFilesArguments(tt.args)
			if err == nil {
				t.Fatal("expected validation error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error %q does not contain %q", err, tt.want)
			}
		})
	}
}

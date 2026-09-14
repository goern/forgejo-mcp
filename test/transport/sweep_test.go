// SPDX-License-Identifier: GPL-3.0-or-later

// Package transport_test drives the *actual built binary* over the real
// stdio JSON-RPC transport (not the in-process handler path exercised by
// test/race, and not the --cli path used by test/e2e — both of those
// bypass the stdio wire entirely). Multiple agents reported comment/issue
// attachment uploads failing with "illegal base64 data" at a
// variable byte offset near the end of the payload, above thresholds that
// ranged from ~1.5KB to ~4.9KB across different agent sessions, and one
// session reported a 30-minute hang.
//
// This test sweeps payload sizes from 1KB to 100KB through the real stdio
// pipe using mcp-go's own client transport (a spec-compliant JSON-RPC/stdio
// client, structurally analogous to what any MCP host — including
// Claude Code's — does: marshal the call, write one line, read one line
// back) and verifies the server-side HTTP handler receives the exact bytes
// that were sent, byte for byte (via length + sha256).
//
// Run:  go test ./test/transport/ -run TestAttachmentSizeSweep -v
//
// The binary under test is built fresh by TestMain into a temp directory —
// this package is self-contained on a clean checkout and always exercises
// the current tree, never a stale prebuilt ./forgejo-mcp left over from a
// previous `make build`.
package transport_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
)

// builtBinary is set by TestMain to the path of the freshly built
// forgejo-mcp binary; tests in this package read it rather than locating a
// pre-existing build on disk.
var builtBinary string

// TestMain builds the current tree into a temp binary before running any
// test in this package, and removes it afterward. This makes the package
// self-contained on a clean checkout (no `make build` prerequisite) and
// guarantees the test always exercises the current source, not whatever
// ./forgejo-mcp happened to be left on disk from an earlier build.
func TestMain(m *testing.M) {
	tmpDir, err := os.MkdirTemp("", "forgejo-mcp-sweep-test-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "TestMain: create temp dir: %v\n", err)
		os.Exit(1)
	}
	// Note: os.Exit below does not run deferred functions, so the cleanup is
	// invoked explicitly around m.Run() rather than deferred.

	bin := filepath.Join(tmpDir, "forgejo-mcp")
	cmd := exec.Command("go", "build", "-o", bin, "../..")
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "TestMain: go build -o %s ../..: %v\n%s\n", bin, err, out)
		_ = os.RemoveAll(tmpDir)
		os.Exit(1)
	}

	builtBinary = bin
	code := m.Run()
	_ = os.RemoveAll(tmpDir)
	os.Exit(code)
}

// captured records what the fake Forgejo API actually received for one
// upload, read directly off the multipart body server-side.
type captured struct {
	size int64
	sha  string
}

func TestAttachmentSizeSweep(t *testing.T) {
	bin := builtBinary

	results := make(chan captured, 1)
	fakeAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Startup connectivity check (operation.testConnection) hits this
		// before the server will even start serving stdio; must succeed or
		// the subprocess os.Exit(1)s before our tool call ever reaches it.
		if r.Method == http.MethodGet && r.URL.Path == "/api/v1/version" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"version": "9.0.0+sweep-test"})
			return
		}
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if err := r.ParseMultipartForm(200 << 20); err != nil {
			t.Errorf("server: ParseMultipartForm: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		file, _, err := r.FormFile("attachment")
		if err != nil {
			t.Errorf("server: FormFile: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		defer file.Close()
		h := sha256.New()
		n, err := io.Copy(h, file)
		if err != nil {
			t.Errorf("server: read multipart file: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		results <- captured{size: n, sha: fmt.Sprintf("%x", h.Sum(nil))}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": 1, "name": "sweep.bin", "size": n,
			"browser_download_url": r.Host + "/attachments/fake",
		})
	}))
	defer fakeAPI.Close()

	env := []string{
		"FORGEJO_URL=" + fakeAPI.URL,
		"FORGEJO_ACCESS_TOKEN=sweep-test-token",
	}

	stdioTransport := transport.NewStdio(bin, env, "--transport", "stdio")
	c := client.NewClient(stdioTransport)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := c.Start(ctx); err != nil {
		t.Fatalf("start client/subprocess: %v", err)
	}
	defer c.Close()

	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcp.Implementation{Name: "sweep-test", Version: "0.0.0"}
	if _, err := c.Initialize(ctx, initReq); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	sizes := []int{
		1 * 1024,   // 1KB
		1536,       // ~1.5KB — the lowest reported failure threshold
		2 * 1024,   // 2KB
		4 * 1024,   // 4KB
		4300,       // ~4.3KB — reported ceiling
		4952,       // exact failing payload size from the original bug report
		8 * 1024,   // 8KB
		16 * 1024,  // 16KB
		32 * 1024,  // 32KB
		64 * 1024,  // 64KB
		100 * 1024, // 100KB
	}

	rng := rand.New(rand.NewSource(42))

	for _, size := range sizes {
		size := size
		t.Run(fmt.Sprintf("%dB", size), func(t *testing.T) {
			raw := make([]byte, size)
			if _, err := rng.Read(raw); err != nil {
				t.Fatalf("gen random payload: %v", err)
			}
			wantSum := sha256.Sum256(raw)
			b64 := base64.StdEncoding.EncodeToString(raw)

			callCtx, callCancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer callCancel()

			callReq := mcp.CallToolRequest{}
			callReq.Params.Name = "create_comment_attachment"
			callReq.Params.Arguments = map[string]any{
				"owner":      "sweep",
				"repo":       "sweep",
				"comment_id": float64(1),
				"content":    b64,
				"filename":   "sweep.bin",
				"mime_type":  "application/octet-stream",
			}

			callDone := make(chan error, 1)
			var res *mcp.CallToolResult
			go func() {
				var err error
				res, err = c.CallTool(callCtx, callReq)
				callDone <- err
			}()

			select {
			case err := <-callDone:
				if err != nil {
					t.Fatalf("CallTool transport error at %d bytes raw (%d base64): %v", size, len(b64), err)
				}
			case <-callCtx.Done():
				t.Fatalf("CallTool HUNG past 20s timeout at %d bytes raw (%d base64) — this is exactly the ~30-minute hang failure mode originally reported", size, len(b64))
			}

			if res != nil && res.IsError {
				var msg string
				if len(res.Content) > 0 {
					if tc, ok := mcp.AsTextContent(res.Content[0]); ok {
						msg = tc.Text
					}
				}
				t.Fatalf("tool returned error at %d bytes raw (%d base64): %s", size, len(b64), msg)
			}

			select {
			case got := <-results:
				if got.size != int64(size) {
					t.Errorf("server received %d bytes, want %d (raw), base64 len=%d", got.size, size, len(b64))
				}
				wantHex := fmt.Sprintf("%x", wantSum)
				if got.sha != wantHex {
					t.Errorf("server-side sha256 mismatch at %d bytes: got %s want %s — payload was corrupted in transit", size, got.sha, wantHex)
				}
			case <-time.After(5 * time.Second):
				t.Fatalf("fake API never received a multipart upload for %d-byte payload", size)
			}
		})
	}
}

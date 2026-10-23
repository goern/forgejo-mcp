// SPDX-License-Identifier: GPL-3.0-or-later

package operation

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/server"
)

// heartbeatTestServer is the smallest server the transports accept.
func heartbeatTestServer() *server.MCPServer {
	return server.NewMCPServer("heartbeat-test", "0.0.0")
}

// awaitPing reads an SSE stream and reports whether a JSON-RPC ping arrives
// before the stream ends. Reading the wire instead of inspecting options keeps
// the test honest: it fails if the heartbeat is configured but never written.
func awaitPing(r io.Reader) bool {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		payload, ok := strings.CutPrefix(scanner.Text(), "data:")
		if !ok {
			continue
		}
		var msg struct {
			Method string `json:"method"`
		}
		if err := json.Unmarshal([]byte(strings.TrimSpace(payload)), &msg); err != nil {
			continue
		}
		if msg.Method == "ping" {
			return true
		}
	}
	return false
}

// openNotificationStream initialises a session on the streamable HTTP handler
// and returns the GET stream that carries server-initiated messages.
func openNotificationStream(t *testing.T, srv *httptest.Server, within time.Duration) io.ReadCloser {
	t.Helper()

	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":` +
		`{"protocolVersion":"2025-06-18","capabilities":{},` +
		`"clientInfo":{"name":"heartbeat-test","version":"1"}}}`
	req, err := http.NewRequest(http.MethodPost, srv.URL+streamableHTTPEndpointPath, strings.NewReader(body))
	if err != nil {
		t.Fatalf("build initialize request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("initialize: %v", err)
	}
	// The initialize response is not under test; drain it so the connection can
	// be reused.
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("initialize: status %d", resp.StatusCode)
	}
	sessionID := resp.Header.Get("Mcp-Session-Id")
	if sessionID == "" {
		t.Fatal("initialize: no Mcp-Session-Id header")
	}

	ctx, cancel := context.WithTimeout(context.Background(), within)
	t.Cleanup(cancel)
	req, err = http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+streamableHTTPEndpointPath, nil)
	if err != nil {
		t.Fatalf("build GET request: %v", err)
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Mcp-Session-Id", sessionID)

	resp, err = srv.Client().Do(req)
	if err != nil {
		t.Fatalf("open GET stream: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("open GET stream: status %d", resp.StatusCode)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp.Body
}

func TestStreamableHTTPHeartbeat(t *testing.T) {
	t.Parallel()

	interval := 50 * time.Millisecond
	srv := httptest.NewServer(newStreamableHTTPHandler(heartbeatTestServer(), interval))
	// Closing the stream must happen before closing the server: Close waits for
	// the handler, which only returns once its client has gone.
	t.Cleanup(srv.Close)

	stream := openNotificationStream(t, srv, 5*time.Second)
	if !awaitPing(stream) {
		t.Fatalf("no ping on the notification stream within 5s with a %s heartbeat", interval)
	}
}

func TestStreamableHTTPHeartbeatDisabled(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(newStreamableHTTPHandler(heartbeatTestServer(), 0))
	t.Cleanup(srv.Close)

	// Long enough for many heartbeats at the interval used above, so a stream
	// that stays silent here stays silent because the heartbeat is off.
	stream := openNotificationStream(t, srv, 500*time.Millisecond)
	if awaitPing(stream) {
		t.Fatal("ping arrived although the heartbeat is disabled")
	}
}

func TestSSEHeartbeat(t *testing.T) {
	t.Parallel()

	interval := 50 * time.Millisecond
	srv := httptest.NewServer(newSSEHandler(heartbeatTestServer(), interval))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/sse", nil)
	if err != nil {
		t.Fatalf("build SSE request: %v", err)
	}
	req.Header.Set("Accept", "text/event-stream")

	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("open SSE stream: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("open SSE stream: status %d", resp.StatusCode)
	}

	if !awaitPing(resp.Body) {
		t.Fatalf("no ping on the SSE stream within 5s with a %s heartbeat", interval)
	}
}

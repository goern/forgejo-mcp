// SPDX-License-Identifier: GPL-3.0-or-later

package operation

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/flag"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forgejo"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/to"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"go.uber.org/zap"
)

const (
	rejectedToken = "not-a-real-token-echo-canary"
	acceptedToken = "accepted-token"
	scopedToken   = "token-without-the-scope"
)

// rejectingForge is a Forgejo that answers like the real one: an unknown
// token gets 401 on every path, /api/v1/version included, with the token
// repeated in the message. acceptedToken works, scopedToken gets 403.
func rejectingForge(t *testing.T) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "token ")
		switch {
		case token == "" && r.URL.Path == "/api/v1/version",
			token == acceptedToken && r.URL.Path == "/api/v1/version",
			token == scopedToken && r.URL.Path == "/api/v1/version":
			_, _ = w.Write([]byte(`{"version":"16.0.3"}`))
		case token == acceptedToken:
			_, _ = w.Write([]byte(`{"id":1,"login":"synapse"}`))
		case token == scopedToken:
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"message":"token does not have at least one of required scope(s): [read:user]"}`))
		default:
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = fmt.Fprintf(w, `{"message":"access token does not exist [sha: %s]\ntask with token %q: resource does not exist"}`, token, token)
		}
	}))
	t.Cleanup(srv.Close)
	saved := flag.URL
	flag.URL = srv.URL
	t.Cleanup(func() { flag.URL = saved })
}

// rejectionTestServer is an MCP server with the production middleware and a
// few probe tools that reach Forgejo the way real tools do.
func rejectionTestServer() *server.MCPServer {
	s := newMCPServer("rejection-test")
	whoami := func(ctx context.Context) (*mcp.CallToolResult, error) {
		client, err := forgejo.Client(ctx)
		if err != nil {
			return to.ErrorResult(err)
		}
		user, _, err := client.GetMyUserInfo()
		if err != nil {
			return to.ErrorResult(fmt.Errorf("get user: %w", err))
		}
		return to.TextResult(user)
	}
	s.AddTool(mcp.NewTool("whoami"), func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return whoami(ctx)
	})
	// swallow treats any failure as "nothing there", as DoJSONList does for a
	// 404. A refused credential must still fail the call.
	s.AddTool(mcp.NewTool("swallow"), func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var user map[string]any
		_ = forgejo.DoJSON(ctx, http.MethodGet, forgejo.APIPath("user"), nil, &user)
		return mcp.NewToolResultText("nothing found"), nil
	})
	// progress_then_whoami sends a notification first, which makes the
	// streamable transport commit status 200 and switch to an event stream.
	s.AddTool(mcp.NewTool("progress_then_whoami"), func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		_ = server.ServerFromContext(ctx).SendNotificationToClient(ctx, "notifications/message",
			map[string]any{"level": "info", "data": "working"})
		time.Sleep(50 * time.Millisecond) // let the transport flush the notification
		return whoami(ctx)
	})
	s.AddResourceTemplate(mcp.NewResourceTemplate("test-probe://probe/{x}", "probe"),
		func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			var user map[string]any
			if err := forgejo.DoJSON(ctx, http.MethodGet, forgejo.APIPath("user"), nil, &user); err != nil {
				return nil, err
			}
			return []mcp.ResourceContents{mcp.TextResourceContents{URI: request.Params.URI, Text: "ok"}}, nil
		})
	return s
}

// rpcReply is the part of a JSON-RPC response these tests look at.
type rpcReply struct {
	ID     any             `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// mcpSession speaks streamable HTTP to one endpoint.
type mcpSession struct {
	url, host, authorization, sessionID string
}

// post sends one JSON-RPC message and returns the response with the reply
// for id, read from a JSON body or from an event stream.
func (c *mcpSession) post(t *testing.T, id int, method, params string) (*http.Response, rpcReply) {
	t.Helper()
	body := fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":%q,"params":%s}`, id, method, params)
	req, err := http.NewRequest(http.MethodPost, c.url, strings.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if c.host != "" {
		req.Host = c.host
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if c.authorization != "" {
		req.Header.Set("Authorization", c.authorization)
	}
	if c.sessionID != "" {
		req.Header.Set("Mcp-Session-Id", c.sessionID)
	}
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		t.Fatalf("%s: %v", method, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var reply rpcReply
	if strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		scanner := bufio.NewScanner(strings.NewReader(string(raw)))
		for scanner.Scan() {
			payload, ok := strings.CutPrefix(scanner.Text(), "data:")
			if !ok {
				continue
			}
			var msg rpcReply
			if json.Unmarshal([]byte(strings.TrimSpace(payload)), &msg) == nil && fmt.Sprint(msg.ID) == fmt.Sprint(id) {
				reply = msg
			}
		}
	} else if len(raw) > 0 {
		if err := json.Unmarshal(raw, &reply); err != nil {
			t.Fatalf("%s: decode %q: %v", method, raw, err)
		}
	}
	if strings.Contains(string(raw), rejectedToken) {
		t.Fatalf("%s: the response carries the rejected token: %s", method, raw)
	}
	return resp, reply
}

func (c *mcpSession) initialize(t *testing.T) {
	t.Helper()
	resp, reply := c.post(t, 1, "initialize",
		`{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"rejection-test","version":"1"}}`)
	if resp.StatusCode != http.StatusOK || reply.Error != nil {
		t.Fatalf("initialize: status %d, error %+v", resp.StatusCode, reply.Error)
	}
	c.sessionID = resp.Header.Get("Mcp-Session-Id")
}

// passthroughSession serves the streamable HTTP handler in passthrough mode
// and initialises a session that presents token.
func passthroughSession(t *testing.T, token string) *mcpSession {
	t.Helper()
	rejectingForge(t)
	srv := httptest.NewServer(newStreamableHTTPHandler(rejectionTestServer(), 0))
	t.Cleanup(srv.Close)
	c := &mcpSession{url: srv.URL + streamableHTTPEndpointPath, authorization: "token " + token}
	c.initialize(t)
	return c
}

func callTool(name string) string { return fmt.Sprintf(`{"name":%q,"arguments":{}}`, name) }

// Spec stateless-http-auth, scenarios "A revoked token fails with -32001" and
// "A revoked token yields 401 with a challenge".
func TestARevokedTokenYields401WithAChallenge(t *testing.T) {
	c := passthroughSession(t, rejectedToken)
	logs := captureLogs(t, zap.DebugLevel)

	resp, reply := c.post(t, 2, "tools/call", callTool("whoami"))
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
	if got := resp.Header.Get("WWW-Authenticate"); got != `Bearer error="invalid_token"` {
		t.Fatalf("WWW-Authenticate = %q", got)
	}
	if reply.Error == nil || reply.Error.Code != credentialRejectedCode || reply.Error.Message != forgejo.RejectedCredentialMessage {
		t.Fatalf("reply error = %+v, want -32001 with the fixed message", reply.Error)
	}
	for _, e := range logs.All() {
		if line := e.Message + fmt.Sprint(e.ContextMap()); strings.Contains(line, rejectedToken) {
			t.Fatalf("log entry carries the token: %s", line)
		}
	}
}

// The rejection also holds when the server version is known up front, so the
// first Forgejo request is the tool's own call, not the SDK's version probe.
func TestARevokedTokenIsRecognisedAfterTheVersionIsKnown(t *testing.T) {
	forgejo.SetServerVersion("16.0.3")
	t.Cleanup(func() { forgejo.SetServerVersion("") })
	c := passthroughSession(t, rejectedToken)

	resp, reply := c.post(t, 2, "tools/call", callTool("whoami"))
	if resp.StatusCode != http.StatusUnauthorized || reply.Error == nil || reply.Error.Code != credentialRejectedCode {
		t.Fatalf("status %d, error %+v; want 401 and -32001", resp.StatusCode, reply.Error)
	}
}

// A handler that swallows the failure must not turn a refused credential into
// a successful answer.
func TestASwallowedRejectionStillFails(t *testing.T) {
	c := passthroughSession(t, rejectedToken)

	resp, reply := c.post(t, 2, "tools/call", callTool("swallow"))
	if resp.StatusCode != http.StatusUnauthorized || reply.Error == nil || reply.Error.Code != credentialRejectedCode {
		t.Fatalf("status %d, error %+v; want 401 and -32001", resp.StatusCode, reply.Error)
	}
}

// Spec stateless-http-auth, scenario "A resource read with a revoked token
// fails with -32001".
func TestAResourceReadWithARevokedTokenFails(t *testing.T) {
	c := passthroughSession(t, rejectedToken)

	resp, reply := c.post(t, 2, "resources/read", `{"uri":"test-probe://probe/x"}`)
	if resp.StatusCode != http.StatusUnauthorized || reply.Error == nil || reply.Error.Code != credentialRejectedCode {
		t.Fatalf("status %d, error %+v; want 401 and -32001", resp.StatusCode, reply.Error)
	}
}

// Spec stateless-http-auth, scenario "Insufficient scope is not a rejection".
func TestInsufficientScopeIsNotARejection(t *testing.T) {
	c := passthroughSession(t, scopedToken)

	resp, reply := c.post(t, 2, "tools/call", callTool("whoami"))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if reply.Error == nil || reply.Error.Code == credentialRejectedCode {
		t.Fatalf("reply error = %+v, want an ordinary error", reply.Error)
	}
}

func TestAnAcceptedTokenIsUntouched(t *testing.T) {
	c := passthroughSession(t, acceptedToken)

	resp, reply := c.post(t, 2, "tools/call", callTool("whoami"))
	if resp.StatusCode != http.StatusOK || reply.Error != nil {
		t.Fatalf("status %d, error %+v; want 200 and a result", resp.StatusCode, reply.Error)
	}
}

// Spec stateless-http-auth, scenario "A request that never reaches Forgejo is
// not challenged".
func TestARequestThatNeverReachesForgejoIsNotChallenged(t *testing.T) {
	c := passthroughSession(t, rejectedToken)

	resp, reply := c.post(t, 2, "tools/list", `{}`)
	if resp.StatusCode != http.StatusOK || reply.Error != nil {
		t.Fatalf("status %d, error %+v; want 200 and a result", resp.StatusCode, reply.Error)
	}
}

// Spec stateless-http-auth, scenario "A response already being streamed keeps
// its status".
func TestAResponseAlreadyBeingStreamedKeepsItsStatus(t *testing.T) {
	c := passthroughSession(t, rejectedToken)

	resp, reply := c.post(t, 2, "tools/call", callTool("progress_then_whoami"))
	if !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("the response was not streamed (Content-Type %q); the scenario needs a committed status",
			resp.Header.Get("Content-Type"))
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want the 200 already sent", resp.StatusCode)
	}
	if reply.Error == nil || reply.Error.Code != credentialRejectedCode {
		t.Fatalf("reply error = %+v, want -32001", reply.Error)
	}
}

// Spec stateless-http-auth, scenario "The SSE transport reports the code".
func TestTheSSETransportReportsTheCode(t *testing.T) {
	rejectingForge(t)
	srv := httptest.NewServer(newSSEHandler(rejectionTestServer(), 0))
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/sse", nil)
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Authorization", "token "+rejectedToken)
	stream, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("open SSE stream: %v", err)
	}
	t.Cleanup(func() { _ = stream.Body.Close() })
	events := bufio.NewScanner(stream.Body)

	var endpoint string
	for endpoint == "" && events.Scan() {
		endpoint, _ = strings.CutPrefix(events.Text(), "data: ")
		if !strings.Contains(endpoint, "sessionId=") {
			endpoint = ""
		}
	}
	postSSE := func(body string) {
		t.Helper()
		r, _ := http.NewRequest(http.MethodPost, srv.URL+strings.TrimPrefix(endpoint, srv.URL), strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Authorization", "token "+rejectedToken)
		resp, err := srv.Client().Do(r)
		if err != nil {
			t.Fatalf("post: %v", err)
		}
		_ = resp.Body.Close()
	}
	postSSE(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`)
	postSSE(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":` + callTool("whoami") + `}`)

	for events.Scan() {
		payload, ok := strings.CutPrefix(events.Text(), "data: ")
		if !ok {
			continue
		}
		if strings.Contains(payload, rejectedToken) {
			t.Fatalf("the stream carries the rejected token: %s", payload)
		}
		var reply rpcReply
		if json.Unmarshal([]byte(payload), &reply) != nil || fmt.Sprint(reply.ID) != "2" {
			continue
		}
		if reply.Error == nil || reply.Error.Code != credentialRejectedCode {
			t.Fatalf("reply error = %+v, want -32001", reply.Error)
		}
		return
	}
	t.Fatal("no reply to the tool call on the SSE stream")
}

// Spec stateless-http-auth, scenario "stdio is unchanged".
func TestStdioIsUnchanged(t *testing.T) {
	rejectingForge(t)
	savedToken, require := flag.Token, forgejo.RequireRequestToken()
	flag.Token = rejectedToken
	forgejo.SetRequireRequestToken(false)
	t.Cleanup(func() {
		flag.Token = savedToken
		forgejo.SetRequireRequestToken(require)
		forgejo.ResetClientForTesting()
	})
	forgejo.ResetClientForTesting()
	s := rejectionTestServer()

	reply := s.HandleMessage(context.Background(), []byte(
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":`+callTool("whoami")+`}`))
	raw, _ := json.Marshal(reply)
	if strings.Contains(string(raw), rejectedToken) {
		t.Fatalf("the reply carries the token: %s", raw)
	}
	var decoded rpcReply
	_ = json.Unmarshal(raw, &decoded)
	if decoded.Error == nil || decoded.Error.Code == credentialRejectedCode {
		t.Fatalf("reply error = %+v, want an ordinary error without -32001", decoded.Error)
	}
	if !strings.Contains(decoded.Error.Message, forgejo.RejectedCredentialMessage) {
		t.Fatalf("reply error = %q, want the fixed message", decoded.Error.Message)
	}
}

// The wrapper must keep the transport's ability to stream: without
// http.Flusher, mcp-go refuses subscriptions/listen.
func TestTheRejectionWriterKeepsStreaming(t *testing.T) {
	rec := httptest.NewRecorder()
	var sawFlusher, sawUnwrap bool
	withRejectionChallenge(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, sawFlusher = w.(http.Flusher)
		_, sawUnwrap = w.(interface{ Unwrap() http.ResponseWriter })
		w.(http.Flusher).Flush()
	})).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/mcp", nil))
	if !sawFlusher || !sawUnwrap {
		t.Fatalf("Flusher %v, Unwrap %v; want both", sawFlusher, sawUnwrap)
	}
	if !rec.Flushed || rec.Code != http.StatusOK {
		t.Fatalf("flushed %v, status %d", rec.Flushed, rec.Code)
	}
}

// requestTokenContextFunc attaches a Rejection only together with a
// per-request credential, and reuses the one the HTTP layer created so that
// the response status can follow it.
func TestARejectionTravelsOnlyWithAPerRequestCredential(t *testing.T) {
	saved := flag.AuthMode
	t.Cleanup(func() { flag.AuthMode = saved })

	fromHTTP := &forgejo.Rejection{}
	httpRequest := func(header string, extra context.Context) *http.Request {
		r := httptest.NewRequest(http.MethodPost, "/mcp", nil)
		if header != "" {
			r.Header.Set("Authorization", header)
		}
		return r.WithContext(forgejo.WithRejection(extra, fromHTTP))
	}

	flag.AuthMode = AuthModePassthrough
	ctx := requestTokenContextFunc(context.Background(), httpRequest("token "+acceptedToken, context.Background()))
	if forgejo.RejectionFrom(ctx) != fromHTTP {
		t.Fatal("passthrough: the handler context does not carry the HTTP layer's Rejection")
	}
	ctx = requestTokenContextFunc(context.Background(), httpRequest("", context.Background()))
	if forgejo.RejectionFrom(ctx) != nil {
		t.Fatal("passthrough: a request without a credential (operator fallback) carries a Rejection")
	}

	// The sse transport has no HTTP-layer Rejection; a fresh one is attached.
	r := httptest.NewRequest(http.MethodPost, "/message", nil)
	r.Header.Set("Authorization", "token "+acceptedToken)
	if forgejo.RejectionFrom(requestTokenContextFunc(context.Background(), r)) == nil {
		t.Fatal("sse: no Rejection with a per-request credential")
	}

	flag.AuthMode = AuthModeResourceServer
	minted := context.WithValue(context.Background(), mintedTokenKey{}, "minted-jwt")
	ctx = requestTokenContextFunc(context.Background(), httpRequest("Bearer inbound", minted))
	if forgejo.RejectionFrom(ctx) != fromHTTP {
		t.Fatal("resource-server: the minted credential does not carry the HTTP layer's Rejection")
	}
	ctx = requestTokenContextFunc(context.Background(), httpRequest("Bearer inbound", context.Background()))
	if forgejo.RejectionFrom(ctx) != nil {
		t.Fatal("resource-server: a Rejection without a minted credential")
	}
}

// Spec forgejo-jwt-issuer, scenarios "The integration was removed", "The
// operator learns of the rejection" and "The echoed JWT never reaches the
// caller".
func TestARejectedMintedJWTIsAServerSideFailure(t *testing.T) {
	env := startResourceServerWith(t, func() http.Handler {
		return newStreamableHTTPHandler(rejectionTestServer(), 0)
	})
	env.fixture.rejectUser.Store(true)
	logs := captureLogs(t, zap.DebugLevel)
	c := &mcpSession{url: "http://" + env.addr + streamableHTTPEndpointPath, host: rsHost,
		authorization: "Bearer " + env.accessToken(t, nil)}
	c.initialize(t)

	resp, reply := c.post(t, 2, "tools/call", callTool("whoami"))
	if resp.StatusCode == http.StatusUnauthorized || resp.Header.Get("WWW-Authenticate") != "" {
		t.Fatalf("status %d, WWW-Authenticate %q; a rejected minted JWT must not challenge",
			resp.StatusCode, resp.Header.Get("WWW-Authenticate"))
	}
	if reply.Error == nil || reply.Error.Code != mcp.INTERNAL_ERROR || reply.Error.Message != mintedCredentialRejectedMessage {
		t.Fatalf("reply error = %+v, want -32603 with the server-side message", reply.Error)
	}

	var minted string
	for _, req := range env.fixture.requests() {
		if m, ok := strings.CutPrefix(req, "/api/v1/user token "); ok {
			minted = m
		}
	}
	if minted == "" {
		t.Fatal("Forgejo never saw the minted JWT; the scenario did not run")
	}
	signature := strings.Split(minted, ".")[2]
	warned := false
	for _, e := range logs.All() {
		line := e.Message + fmt.Sprint(e.ContextMap())
		if strings.Contains(line, minted) || strings.Contains(line, signature) {
			t.Fatalf("a log entry carries the minted JWT: %s", line)
		}
		if e.Level >= zap.WarnLevel && strings.Contains(e.Message, "Forgejo rejected the JWT minted") &&
			strings.Contains(fmt.Sprint(e.ContextMap()), "whoami") {
			warned = true
		}
	}
	if !warned {
		t.Fatal("no warning naming the tool was logged for the operator")
	}
	// The response itself was checked for the rejected token by post; check
	// the minted JWT too.
	if strings.Contains(reply.Error.Message, signature) {
		t.Fatal("the reply carries the minted JWT")
	}
}

// A subscriptions/listen stream still opens and delivers through the
// rejection writer; mcp-go refuses it when the writer cannot flush.
func TestAListenStreamWorksThroughTheRejectionWriter(t *testing.T) {
	s := rejectionTestServer()
	srv := httptest.NewServer(newStreamableHTTPHandler(s, 0))
	t.Cleanup(srv.Close)

	body, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 7, "method": mcp.MethodSubscriptionsListen,
		"params": map[string]any{
			"notifications": map[string]any{"toolsListChanged": true},
			"_meta": map[string]any{
				mcp.MetaKeyProtocolVersion:    mcp.ProtocolVersion20260728,
				mcp.MetaKeyClientInfo:         map[string]any{"name": "rejection-test", "version": "1"},
				mcp.MetaKeyClientCapabilities: map[string]any{},
			},
		},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, srv.URL+streamableHTTPEndpointPath, strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set(mcp.HeaderProtocolVersion, mcp.ProtocolVersion20260728)
	req.Header.Set(mcp.HeaderMethod, string(mcp.MethodSubscriptionsListen))
	req.Header.Set("Authorization", "token "+acceptedToken)
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("open listen stream: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	if resp.StatusCode != http.StatusOK || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		t.Fatalf("listen stream refused: status %d, Content-Type %q, body %q", resp.StatusCode, resp.Header.Get("Content-Type"), raw)
	}

	arrived := make(chan struct{})
	go func() {
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			if strings.Contains(scanner.Text(), "notifications/tools/list_changed") {
				close(arrived)
				return
			}
		}
	}()
	time.Sleep(100 * time.Millisecond) // let the stream register before broadcasting
	s.AddTool(mcp.NewTool("added_later"), func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultText("ok"), nil
	})
	select {
	case <-arrived:
	case <-time.After(5 * time.Second):
		t.Fatal("no list_changed notification arrived on the listen stream")
	}
}

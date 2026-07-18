// SPDX-License-Identifier: GPL-3.0-or-later

package operation

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"codeberg.org/goern/forgejo-mcp/v2/pkg/flag"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func TestCoreResourceDispatchRoutesWikiTemplate(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/wiki/page/Home"):
			content := base64.StdEncoding.EncodeToString([]byte("# Dispatched\n"))
			_, _ = w.Write([]byte(`{"title":"Home","sub_url":"Home","content_base64":"` + content + `","last_commit":{"sha":"abc"}}`))
		case strings.Contains(r.URL.Path, "/wiki/revisions/Home"):
			_, _ = w.Write([]byte(`{"commits":[],"count":0}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(api.Close)
	flag.URL = api.URL
	flag.Token = "test"

	s := server.NewMCPServer("forgejo-mcp", "test")
	RegisterCoreResources(s)
	response := s.HandleMessage(context.Background(), []byte(`{
		"jsonrpc":"2.0",
		"id":1,
		"method":"resources/read",
		"params":{"uri":"forgejo://repo/o/r/wiki/Home"}
	}`))
	rpc, ok := response.(mcp.JSONRPCResponse)
	if !ok {
		t.Fatalf("wiki resource was not dispatched: %#v", response)
	}
	result, ok := rpc.Result.(mcp.ReadResourceResult)
	if !ok || len(result.Contents) != 2 {
		t.Fatalf("unexpected wiki resource response: %#v", rpc.Result)
	}
	markdown, ok := result.Contents[1].(mcp.TextResourceContents)
	if !ok || markdown.Text != "# Dispatched\n" {
		t.Fatalf("wiki handler did not produce markdown sidecar: %#v", result.Contents)
	}
}

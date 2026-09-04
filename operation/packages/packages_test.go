// SPDX-License-Identifier: GPL-3.0-or-later

package packages

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/flag"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forgejo"

	"github.com/mark3labs/mcp-go/mcp"
)

const fatPackageJSON = `{
	"id": 1,
	"type": "container",
	"name": "core",
	"version": "1.0.0",
	"html_url": "https://example/packages/1",
	"created_at": "2026-08-01T00:00:00Z",
	"owner": {"id": 2, "login": "alice", "full_name": "Alice", "email": "a@b.c"},
	"creator": {"id": 2, "login": "alice"},
	"repository": {"id": 9, "name": "core", "full_name": "alice/core", "owner": {"login": "alice"}}
}`

type packageRequestCapture struct {
	method      string
	path        string
	escapedPath string
	rawQuery    string
	count       int
}

func setupPackageAPIServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *packageRequestCapture) {
	t.Helper()
	capture := &packageRequestCapture{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capture.method = r.Method
		capture.path = r.URL.Path
		capture.escapedPath = r.URL.EscapedPath()
		capture.rawQuery = r.URL.RawQuery
		capture.count++
		handler(w, r)
	}))
	t.Cleanup(server.Close)
	flag.URL = server.URL
	flag.Token = "test-token"
	flag.UserAgent = "forgejo-mcp-test/0.0.1"
	return server, capture
}

func newCallToolRequest(args map[string]interface{}) mcp.CallToolRequest {
	return mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: args,
		},
	}
}

func decodePackageResult[T any](t *testing.T, result *mcp.CallToolResult) T {
	t.Helper()
	if result == nil || len(result.Content) == 0 {
		t.Fatal("tool returned no content")
	}
	text, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected text content, got %T", result.Content[0])
	}
	var envelope struct {
		Result T `json:"Result"`
	}
	if err := json.Unmarshal([]byte(text.Text), &envelope); err != nil {
		t.Fatalf("decode result: %v; body=%s", err, text.Text)
	}
	return envelope.Result
}

func resultText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	if result == nil || len(result.Content) == 0 {
		t.Fatal("tool returned no content")
	}
	text, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected text content, got %T", result.Content[0])
	}
	return text.Text
}

func TestListPackagesFn_QueryAndEnvelope(t *testing.T) {
	_, capture := setupPackageAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set(forgejo.TotalCountHeader, "4")
		_, _ = w.Write([]byte("[" + fatPackageJSON + "]"))
	})

	result, err := ListPackagesFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"owner": "o", "type": "container", "q": "core", "page": float64(2), "limit": float64(30),
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capture.path != "/api/v1/packages/o" {
		t.Fatalf("path: %s", capture.path)
	}
	if capture.rawQuery != "limit=30&page=2&q=core&type=container" {
		t.Fatalf("query: %s", capture.rawQuery)
	}
	decoded := decodePackageResult[listPackagesResult](t, result)
	if decoded.Page != 2 || decoded.Limit != 30 || decoded.Count != 1 {
		t.Fatalf("envelope: %+v", decoded)
	}
	if decoded.TotalCount == nil || *decoded.TotalCount != 4 {
		t.Fatalf("total_count: %+v", decoded.TotalCount)
	}
	if decoded.HasNext {
		t.Fatalf("has_next without Link: %+v", decoded)
	}
	if len(decoded.Packages) != 1 || decoded.Packages[0].Name != "core" || decoded.Packages[0].HTMLURL == "" {
		t.Fatalf("packages: %+v", decoded.Packages)
	}
	if decoded.Packages[0].Repository != "alice/core" {
		t.Fatalf("repository: %+v", decoded.Packages[0])
	}
}

func TestListPackagesFn_OmitsTotalCountWhenHeaderAbsent(t *testing.T) {
	setupPackageAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	})

	result, err := ListPackagesFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"owner": "o",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := resultText(t, result)
	if strings.Contains(text, "total_count") {
		t.Fatalf("total_count must be omitted when the header is absent: %s", text)
	}
	decoded := decodePackageResult[listPackagesResult](t, result)
	if decoded.Count != 0 || decoded.Packages == nil {
		t.Fatalf("empty page: %+v", decoded)
	}
	if decoded.HasNext {
		t.Fatalf("has_next without Link: %+v", decoded)
	}
}

func TestListPackagesFn_MissingOwnerIsError(t *testing.T) {
	setupPackageAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"user does not exist"}`))
	})

	result, err := ListPackagesFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"owner": "missing",
	}))
	if err == nil {
		t.Fatal("404 owner must be an error, not an empty list")
	}
	if result != nil {
		t.Fatalf("expected nil result, got %+v", result)
	}
}

func TestListPackagesFn_JSONNullIsEmptySlice(t *testing.T) {
	setupPackageAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`null`))
	})

	result, err := ListPackagesFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"owner": "o",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := resultText(t, result)
	if strings.Contains(text, `"packages":null`) {
		t.Fatalf("packages must be [] not null: %s", text)
	}
	decoded := decodePackageResult[listPackagesResult](t, result)
	if decoded.Packages == nil || len(decoded.Packages) != 0 {
		t.Fatalf("packages: %+v", decoded.Packages)
	}
}

func TestListPackagesFn_ProjectsAwayNestedUser(t *testing.T) {
	setupPackageAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("[" + fatPackageJSON + "]"))
	})

	result, err := ListPackagesFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"owner": "o",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := resultText(t, result)
	if !strings.Contains(text, `"html_url"`) {
		t.Fatalf("expected html_url: %s", text)
	}
	if strings.Contains(text, `"login"`) {
		t.Fatalf("nested user leaked: %s", text)
	}
}

func TestGetPackageFn_EscapesSlashInName(t *testing.T) {
	_, capture := setupPackageAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fatPackageJSON))
	})

	result, err := GetPackageFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"owner": "o", "type": "container", "name": "youscore/core", "version": "1.0.0",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capture.escapedPath != "/api/v1/packages/o/container/youscore%2Fcore/1.0.0" {
		t.Fatalf("path retargeted: path=%s escaped=%s", capture.path, capture.escapedPath)
	}
	decoded := decodePackageResult[packageVersion](t, result)
	if decoded.HTMLURL == "" || decoded.Repository != "alice/core" {
		t.Fatalf("projection: %+v", decoded)
	}
	text := resultText(t, result)
	if strings.Contains(text, `"login"`) {
		t.Fatalf("nested user leaked: %s", text)
	}
}

func TestDeletePackageFn_Success(t *testing.T) {
	_, capture := setupPackageAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	result, err := DeletePackageFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"owner": "o", "type": "generic", "name": "dist", "version": "1.2.3",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capture.method != http.MethodDelete {
		t.Fatalf("method: %s", capture.method)
	}
	if capture.path != "/api/v1/packages/o/generic/dist/1.2.3" {
		t.Fatalf("path: %s", capture.path)
	}
	decoded := decodePackageResult[deletePackageResult](t, result)
	if decoded.Status != "deleted" || decoded.Owner != "o" || decoded.Name != "dist" || decoded.Version != "1.2.3" {
		t.Fatalf("result: %+v", decoded)
	}
}

func TestDeletePackageFn_NotFoundIsError(t *testing.T) {
	_, capture := setupPackageAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"package does not exist"}`))
	})

	result, err := DeletePackageFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"owner": "o", "type": "generic", "name": "dist", "version": "9.9.9",
	}))
	if err == nil {
		t.Fatal("404 delete must be an MCP error")
	}
	if result != nil {
		t.Fatalf("expected nil result, got %+v", result)
	}
	if capture.method != http.MethodDelete {
		t.Fatalf("DELETE must still be sent, got %s", capture.method)
	}
}

func TestListPackageFilesFn_ClientSliceHasNext(t *testing.T) {
	setupPackageAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"id":1,"name":"a.bin","size":10,"sha256":"aa","md5":"m","sha1":"s1","sha512":"s5"},
			{"id":2,"name":"b.bin","size":20,"sha256":"bb","md5":"m2"}
		]`))
	})

	page1, err := ListPackageFilesFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"owner": "o", "type": "generic", "name": "dist", "version": "1.0.0",
		"page": float64(1), "limit": float64(1),
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	decoded := decodePackageResult[listPackageFilesResult](t, page1)
	if decoded.Count != 1 || !decoded.HasNext || len(decoded.Files) != 1 || decoded.Files[0].Name != "a.bin" {
		t.Fatalf("page 1: %+v", decoded)
	}
	if decoded.TotalCount != 2 {
		t.Fatalf("total_count: %d", decoded.TotalCount)
	}
	text := resultText(t, page1)
	if strings.Contains(text, `"md5"`) || strings.Contains(text, `"sha1"`) || strings.Contains(text, `"sha512"`) {
		t.Fatalf("extra hashes leaked: %s", text)
	}

	page2, err := ListPackageFilesFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"owner": "o", "type": "generic", "name": "dist", "version": "1.0.0",
		"page": float64(2), "limit": float64(1),
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	decoded2 := decodePackageResult[listPackageFilesResult](t, page2)
	if decoded2.Count != 1 || decoded2.HasNext || decoded2.Files[0].Name != "b.bin" {
		t.Fatalf("page 2: %+v", decoded2)
	}
	if decoded2.TotalCount != 2 {
		t.Fatalf("page 2 total_count: %d", decoded2.TotalCount)
	}
}

func TestListPackageFilesFn_EmptyListTotalCountZero(t *testing.T) {
	setupPackageAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	})

	result, err := ListPackageFilesFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"owner": "o", "type": "generic", "name": "dist", "version": "1.0.0",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := resultText(t, result)
	if !strings.Contains(text, `"total_count":0`) {
		t.Fatalf("empty list must emit total_count 0: %s", text)
	}
	decoded := decodePackageResult[listPackageFilesResult](t, result)
	if decoded.TotalCount != 0 || decoded.HasNext || decoded.Count != 0 || decoded.Files == nil {
		t.Fatalf("empty list: %+v", decoded)
	}
}

func TestLinkHasNext(t *testing.T) {
	cases := []struct {
		name string
		link string
		want bool
	}{
		{name: "absent", link: "", want: false},
		{name: "quoted", link: `</api/v1/packages/o?page=2>; rel="next"`, want: true},
		{name: "unquoted", link: `</api/v1/packages/o?page=2>; rel=next`, want: true},
		{name: "comma", link: `</api/v1/packages/o?page=1>; rel="prev", </api/v1/packages/o?page=3>; rel="next"`, want: true},
		{name: "prev only", link: `</api/v1/packages/o?page=1>; rel="prev"`, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			header := http.Header{}
			if tc.link != "" {
				header.Set("Link", tc.link)
			}
			if got := linkHasNext(header); got != tc.want {
				t.Fatalf("linkHasNext(%q) = %v, want %v", tc.link, got, tc.want)
			}
		})
	}
}

func TestListPackagesFn_HasNextFromLink(t *testing.T) {
	setupPackageAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Link", `</api/v1/packages/o?page=2&limit=30>; rel="next"`)
		_, _ = w.Write([]byte("[" + fatPackageJSON + "]"))
	})

	result, err := ListPackagesFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"owner": "o",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	decoded := decodePackageResult[listPackagesResult](t, result)
	if !decoded.HasNext {
		t.Fatalf("has_next: %+v", decoded)
	}
	text := resultText(t, result)
	if strings.Contains(text, "total_count") {
		t.Fatalf("total_count must stay omitted without the header: %s", text)
	}
}

func TestGetPackageFn_MissingArgsSendNoRequest(t *testing.T) {
	_, capture := setupPackageAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("no request should be sent")
	})

	if _, err := GetPackageFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"type": "container", "name": "core", "version": "1.0.0",
	})); err == nil {
		t.Fatal("expected error for missing owner")
	}
	if _, err := GetPackageFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"owner": "o", "type": "", "name": "core", "version": "1.0.0",
	})); err == nil {
		t.Fatal("expected error for empty type")
	}
	if capture.count != 0 {
		t.Fatal("request sent")
	}
}

func TestListPackagesFn_OwnerSlashDoesNotRetarget(t *testing.T) {
	_, capture := setupPackageAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	})

	_, err := ListPackagesFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"owner": "o/x",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capture.escapedPath != "/api/v1/packages/o%2Fx" {
		t.Fatalf("path retargeted: path=%s escaped=%s", capture.path, capture.escapedPath)
	}
}

func TestListPackagesFn_PageZeroIsError(t *testing.T) {
	_, capture := setupPackageAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("no request should be sent")
	})

	if _, err := ListPackagesFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"owner": "o", "page": float64(0),
	})); err == nil {
		t.Fatal("page=0 must be an error")
	}
	if capture.count != 0 {
		t.Fatal("request sent")
	}
}

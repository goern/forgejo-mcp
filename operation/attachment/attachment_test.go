package attachment

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"codeberg.org/goern/forgejo-mcp/v2/pkg/flag"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/forgejo"

	"github.com/mark3labs/mcp-go/mcp"
)

// recordedReq stores per-call detail for assertions.
type recordedReq struct {
	method  string
	path    string
	rawBody []byte
	ctype   string
}

// fakeBackend wires httptest.Server with a mux of canned responses keyed by
// "METHOD /path/prefix". The first matching handler wins. Each call appends
// to *records.
type fakeBackend struct {
	srv     *httptest.Server
	records *[]recordedReq
}

type route struct {
	method     string
	pathPrefix string
	handler    http.HandlerFunc
}

func newBackend(t *testing.T, routes ...route) *fakeBackend {
	t.Helper()
	records := make([]recordedReq, 0, 4)
	mux := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		records = append(records, recordedReq{
			method:  r.Method,
			path:    r.URL.Path,
			rawBody: body,
			ctype:   r.Header.Get("Content-Type"),
		})
		// Reset body for inner handlers if they need it.
		r.Body = io.NopCloser(strings.NewReader(string(body)))
		for _, ro := range routes {
			if ro.method == r.Method && strings.HasPrefix(r.URL.Path, ro.pathPrefix) {
				ro.handler(w, r)
				return
			}
		}
		http.NotFound(w, r)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	flag.URL = srv.URL
	flag.Token = "tkn"
	flag.UserAgent = "test"
	return &fakeBackend{srv: srv, records: &records}
}

// req builds an mcp.CallToolRequest from a map.
func req(args map[string]any) mcp.CallToolRequest {
	return mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: args}}
}

// extractTextContent finds the TextContent in a CallToolResult.
func extractTextContent(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	if res == nil {
		t.Fatalf("nil result")
	}
	for _, c := range res.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			return tc.Text
		}
	}
	t.Fatalf("no TextContent in result; content: %+v", res.Content)
	return ""
}

func extractBlobResource(res *mcp.CallToolResult) (mcp.BlobResourceContents, bool) {
	for _, c := range res.Content {
		if er, ok := c.(mcp.EmbeddedResource); ok {
			if br, ok := er.Resource.(mcp.BlobResourceContents); ok {
				return br, true
			}
		}
	}
	return mcp.BlobResourceContents{}, false
}

// --- Issue tools ------------------------------------------------------------

func TestListIssueAttachmentsFn_HappyPath(t *testing.T) {
	b := newBackend(t, route{
		method:     http.MethodGet,
		pathPrefix: "/api/v1/repos/o/r/issues/3/assets",
		handler: func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"id":1,"name":"a.txt","size":3,"uuid":"u1","browser_download_url":"` + flag.URL + `/attachments/u1"}]`))
		},
	})
	res, err := ListIssueAttachmentsFn(context.Background(), req(map[string]any{
		"owner": "o", "repo": "r", "index": 3.0,
	}))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	body := extractTextContent(t, res)
	if !strings.Contains(body, `"a.txt"`) {
		t.Fatalf("missing attachment in body: %s", body)
	}
	if got := (*b.records)[0].path; got != "/api/v1/repos/o/r/issues/3/assets" {
		t.Fatalf("called path: %s", got)
	}
}

func TestListIssueAttachmentsFn_404IsEmptyArray(t *testing.T) {
	newBackend(t, route{
		method:     http.MethodGet,
		pathPrefix: "/api/v1/repos/o/r/issues/9/assets",
		handler: func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		},
	})
	res, err := ListIssueAttachmentsFn(context.Background(), req(map[string]any{
		"owner": "o", "repo": "r", "index": 9.0,
	}))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	body := extractTextContent(t, res)
	// Body wraps in {"Result":[]}; assert it contains "[]".
	if !strings.Contains(body, "[]") {
		t.Fatalf("expected empty array, got: %s", body)
	}
}

func TestGetIssueAttachmentFn_HappyPath(t *testing.T) {
	newBackend(t, route{
		method:     http.MethodGet,
		pathPrefix: "/api/v1/repos/o/r/issues/3/assets/42",
		handler: func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"id":42,"name":"f","size":2,"uuid":"u","browser_download_url":"x"}`))
		},
	})
	res, err := GetIssueAttachmentFn(context.Background(), req(map[string]any{
		"owner": "o", "repo": "r", "index": 3.0, "attachment_id": 42.0,
	}))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	body := extractTextContent(t, res)
	if !strings.Contains(body, `"id":42`) {
		t.Fatalf("body: %s", body)
	}
}

func TestDownloadIssueAttachmentFn_UnderCap_ReturnsBlob(t *testing.T) {
	const payload = "hello pdf"
	b := newBackend(t,
		route{
			method:     http.MethodGet,
			pathPrefix: "/api/v1/repos/o/r/issues/3/assets/42",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				resp := map[string]any{
					"id": 42, "name": "f.pdf", "size": len(payload), "uuid": "u",
					"browser_download_url": flag.URL + "/attachments/u",
				}
				_ = json.NewEncoder(w).Encode(resp)
			},
		},
		route{
			method:     http.MethodGet,
			pathPrefix: "/attachments/u",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/pdf")
				_, _ = w.Write([]byte(payload))
			},
		},
	)
	res, err := DownloadIssueAttachmentFn(context.Background(), req(map[string]any{
		"owner": "o", "repo": "r", "index": 3.0, "attachment_id": 42.0,
	}))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	br, ok := extractBlobResource(res)
	if !ok {
		t.Fatalf("expected BlobResourceContents in result content")
	}
	if br.MIMEType != "application/pdf" {
		t.Fatalf("mime: %s", br.MIMEType)
	}
	decoded, err := base64.StdEncoding.DecodeString(br.Blob)
	if err != nil {
		t.Fatalf("blob not base64: %v", err)
	}
	if string(decoded) != payload {
		t.Fatalf("blob payload: %q", decoded)
	}
	if !strings.Contains(extractTextContent(t, res), `"inline":true`) {
		t.Fatalf("text part should advertise inline:true; got %s", extractTextContent(t, res))
	}
	// 1 metadata fetch + 1 download.
	if n := len(*b.records); n != 2 {
		t.Fatalf("expected 2 backend calls (metadata + bytes), got %d", n)
	}
}

func TestDownloadIssueAttachmentFn_OverCap_NoBlob(t *testing.T) {
	bigSize := forgejo.MaxInlineDownloadBytes + 1
	newBackend(t,
		route{
			method:     http.MethodGet,
			pathPrefix: "/api/v1/repos/o/r/issues/3/assets/99",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				resp := map[string]any{
					"id": 99, "name": "big.bin", "size": bigSize, "uuid": "u-big",
					"browser_download_url": flag.URL + "/attachments/u-big",
				}
				_ = json.NewEncoder(w).Encode(resp)
			},
		},
		// If the over-cap branch is buggy and tries to fetch, this 500 surfaces
		// the bug as a test failure.
		route{
			method:     http.MethodGet,
			pathPrefix: "/attachments/u-big",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
		},
	)
	res, err := DownloadIssueAttachmentFn(context.Background(), req(map[string]any{
		"owner": "o", "repo": "r", "index": 3.0, "attachment_id": 99.0,
	}))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if _, ok := extractBlobResource(res); ok {
		t.Fatalf("expected no blob for over-cap file")
	}
	text := extractTextContent(t, res)
	if !strings.Contains(text, `"inline":false`) {
		t.Fatalf("text should say inline:false, got: %s", text)
	}
	if !strings.Contains(text, "browser_download_url") {
		t.Fatalf("text should still surface browser_download_url, got: %s", text)
	}
}

func TestDownloadIssueAttachmentFn_BodyExceedsCap_GracefulFallback(t *testing.T) {
	// Metadata claims size 10, but server actually serves > cap. The handler
	// must fall back to metadata-only rather than fail.
	newBackend(t,
		route{
			method:     http.MethodGet,
			pathPrefix: "/api/v1/repos/o/r/issues/3/assets/77",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				resp := map[string]any{
					"id": 77, "name": "lying.bin", "size": 10, "uuid": "u-lie",
					"browser_download_url": flag.URL + "/attachments/u-lie",
				}
				_ = json.NewEncoder(w).Encode(resp)
			},
		},
		route{
			method:     http.MethodGet,
			pathPrefix: "/attachments/u-lie",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				big := make([]byte, forgejo.MaxInlineDownloadBytes+100)
				_, _ = w.Write(big)
			},
		},
	)
	res, err := DownloadIssueAttachmentFn(context.Background(), req(map[string]any{
		"owner": "o", "repo": "r", "index": 3.0, "attachment_id": 77.0,
	}))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if _, ok := extractBlobResource(res); ok {
		t.Fatalf("over-cap body should not produce blob")
	}
	if !strings.Contains(extractTextContent(t, res), `"inline":false`) {
		t.Fatalf("expected inline:false fallback")
	}
}

func TestCreateIssueAttachmentFn_DecodesBase64AndUsesMultipart(t *testing.T) {
	const raw = "payload bytes"
	encoded := base64.StdEncoding.EncodeToString([]byte(raw))
	b := newBackend(t,
		route{
			method:     http.MethodPost,
			pathPrefix: "/api/v1/repos/o/r/issues/3/assets",
			handler: func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(`{"id":111,"name":"f.bin","size":13,"uuid":"u","browser_download_url":"x"}`))
			},
		},
	)
	res, err := CreateIssueAttachmentFn(context.Background(), req(map[string]any{
		"owner": "o", "repo": "r", "index": 3.0,
		"content": encoded, "filename": "f.bin", "mime_type": "application/octet-stream",
	}))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(extractTextContent(t, res), `"id":111`) {
		t.Fatalf("body: %s", extractTextContent(t, res))
	}
	rec := (*b.records)[0]
	mt, ps, err := mime.ParseMediaType(rec.ctype)
	if err != nil || mt != "multipart/form-data" {
		t.Fatalf("expected multipart, got %q", rec.ctype)
	}
	mr := multipart.NewReader(strings.NewReader(string(rec.rawBody)), ps["boundary"])
	part, err := mr.NextPart()
	if err != nil {
		t.Fatalf("next part: %v", err)
	}
	if part.FormName() != "attachment" || part.FileName() != "f.bin" {
		t.Fatalf("part fields: name=%s filename=%s", part.FormName(), part.FileName())
	}
	got, _ := io.ReadAll(part)
	if string(got) != raw {
		t.Fatalf("payload: %q", got)
	}
}

func TestCreateIssueAttachmentFn_RejectsNonBase64(t *testing.T) {
	newBackend(t)
	_, err := CreateIssueAttachmentFn(context.Background(), req(map[string]any{
		"owner": "o", "repo": "r", "index": 3.0,
		"content": "not base64!!!", "filename": "f.bin",
	}))
	if err == nil {
		t.Fatalf("expected error for non-base64 content")
	}
}

func TestCreateIssueAttachmentFn_RequiresFilename(t *testing.T) {
	newBackend(t)
	_, err := CreateIssueAttachmentFn(context.Background(), req(map[string]any{
		"owner": "o", "repo": "r", "index": 3.0,
		"content": base64.StdEncoding.EncodeToString([]byte("x")),
	}))
	if err == nil {
		t.Fatalf("expected error when filename missing")
	}
}

func TestEditIssueAttachmentFn_PatchBody(t *testing.T) {
	b := newBackend(t,
		route{
			method:     http.MethodPatch,
			pathPrefix: "/api/v1/repos/o/r/issues/3/assets/42",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{"id":42,"name":"renamed.txt"}`))
			},
		},
	)
	res, err := EditIssueAttachmentFn(context.Background(), req(map[string]any{
		"owner": "o", "repo": "r", "index": 3.0, "attachment_id": 42.0,
		"name": "renamed.txt",
	}))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(extractTextContent(t, res), "renamed.txt") {
		t.Fatalf("body: %s", extractTextContent(t, res))
	}
	rec := (*b.records)[0]
	var body map[string]string
	if err := json.Unmarshal(rec.rawBody, &body); err != nil {
		t.Fatalf("body not JSON: %v", err)
	}
	if body["name"] != "renamed.txt" {
		t.Fatalf("name in body: %s", body["name"])
	}
}

func TestEditIssueAttachmentFn_RequiresName(t *testing.T) {
	newBackend(t)
	_, err := EditIssueAttachmentFn(context.Background(), req(map[string]any{
		"owner": "o", "repo": "r", "index": 3.0, "attachment_id": 42.0,
	}))
	if err == nil {
		t.Fatalf("expected error when name missing")
	}
}

func TestDeleteIssueAttachmentFn_204(t *testing.T) {
	newBackend(t,
		route{
			method:     http.MethodDelete,
			pathPrefix: "/api/v1/repos/o/r/issues/3/assets/42",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			},
		},
	)
	res, err := DeleteIssueAttachmentFn(context.Background(), req(map[string]any{
		"owner": "o", "repo": "r", "index": 3.0, "attachment_id": 42.0,
	}))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(extractTextContent(t, res), `"deleted"`) {
		t.Fatalf("body: %s", extractTextContent(t, res))
	}
}

// --- Comment tools (smoke; same machinery as issue) -------------------------

func TestCommentAttachmentLifecycle_Smoke(t *testing.T) {
	const raw = "comment payload"
	encoded := base64.StdEncoding.EncodeToString([]byte(raw))

	listCalled := false
	createCalled := false
	editCalled := false
	deleteCalled := false
	getCalled := false
	downloadByteFetch := false

	b := newBackend(t,
		route{
			method:     http.MethodGet,
			pathPrefix: "/api/v1/repos/o/r/issues/comments/55/assets/9",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				getCalled = true
				_, _ = w.Write([]byte(fmt.Sprintf(`{"id":9,"name":"x","size":15,"uuid":"u","browser_download_url":"%s/attachments/u"}`, flag.URL)))
			},
		},
		route{
			method:     http.MethodGet,
			pathPrefix: "/api/v1/repos/o/r/issues/comments/55/assets",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				listCalled = true
				_, _ = w.Write([]byte(`[{"id":9,"name":"x","size":15,"uuid":"u","browser_download_url":"x"}]`))
			},
		},
		route{
			method:     http.MethodPost,
			pathPrefix: "/api/v1/repos/o/r/issues/comments/55/assets",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				createCalled = true
				_, _ = w.Write([]byte(`{"id":10,"name":"x.txt"}`))
			},
		},
		route{
			method:     http.MethodPatch,
			pathPrefix: "/api/v1/repos/o/r/issues/comments/55/assets/9",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				editCalled = true
				_, _ = w.Write([]byte(`{"id":9,"name":"new"}`))
			},
		},
		route{
			method:     http.MethodDelete,
			pathPrefix: "/api/v1/repos/o/r/issues/comments/55/assets/9",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				deleteCalled = true
				w.WriteHeader(http.StatusNoContent)
			},
		},
		route{
			method:     http.MethodGet,
			pathPrefix: "/attachments/u",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				downloadByteFetch = true
				w.Header().Set("Content-Type", "text/plain")
				_, _ = w.Write([]byte("xxxxxxxxxxxxxxx"))
			},
		},
	)
	_ = b

	args := map[string]any{"owner": "o", "repo": "r", "comment_id": 55.0}

	if _, err := ListCommentAttachmentsFn(context.Background(), req(args)); err != nil {
		t.Fatalf("list: %v", err)
	}
	if _, err := CreateCommentAttachmentFn(context.Background(), req(merge(args, map[string]any{
		"content": encoded, "filename": "x.txt",
	}))); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := GetCommentAttachmentFn(context.Background(), req(merge(args, map[string]any{
		"attachment_id": 9.0,
	}))); err != nil {
		t.Fatalf("get: %v", err)
	}
	if _, err := DownloadCommentAttachmentFn(context.Background(), req(merge(args, map[string]any{
		"attachment_id": 9.0,
	}))); err != nil {
		t.Fatalf("download: %v", err)
	}
	if _, err := EditCommentAttachmentFn(context.Background(), req(merge(args, map[string]any{
		"attachment_id": 9.0, "name": "new",
	}))); err != nil {
		t.Fatalf("edit: %v", err)
	}
	if _, err := DeleteCommentAttachmentFn(context.Background(), req(merge(args, map[string]any{
		"attachment_id": 9.0,
	}))); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if !listCalled || !createCalled || !getCalled || !editCalled || !deleteCalled || !downloadByteFetch {
		t.Fatalf("not all comment endpoints exercised: list=%v create=%v get=%v edit=%v delete=%v download=%v",
			listCalled, createCalled, getCalled, editCalled, deleteCalled, downloadByteFetch)
	}
}

func merge(a, b map[string]any) map[string]any {
	out := make(map[string]any, len(a)+len(b))
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}

// --- argument validation edge cases -----------------------------------------

func TestHandlers_RejectMissingNumericArgs(t *testing.T) {
	newBackend(t)
	_, err := GetIssueAttachmentFn(context.Background(), req(map[string]any{
		"owner": "o", "repo": "r", // missing index, attachment_id
	}))
	if err == nil {
		t.Fatalf("expected error when numeric args missing")
	}
}

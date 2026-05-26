package release

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"codeberg.org/goern/forgejo-mcp/v2/pkg/flag"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/forgejo"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
)

// recordedReq stores per-call detail for assertions.
type recordedReq struct {
	method  string
	path    string
	query   string
	rawBody []byte
	ctype   string
}

type route struct {
	method     string
	pathPrefix string
	handler    http.HandlerFunc
}

// newBackend mirrors operation/attachment's helper: an httptest server that
// records requests and matches the first prefix that fits. It also wires the
// SDK singleton at flag.URL so handlers using forgejo.Client(ctx) hit it.
func newBackend(t *testing.T, routes ...route) *[]recordedReq {
	t.Helper()
	records := make([]recordedReq, 0, 4)
	mux := http.NewServeMux()
	// Forgejo SDK probes /version on first use (lazy at call sites that ask
	// for server version). Provide a canned response so any such probe works.
	mux.HandleFunc("/api/v1/version", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":"11.0.0+gitea-1.22.0"}`))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		records = append(records, recordedReq{
			method:  r.Method,
			path:    r.URL.Path,
			query:   r.URL.RawQuery,
			rawBody: body,
			ctype:   r.Header.Get("Content-Type"),
		})
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

	c, err := forgejo_sdk.NewClient(srv.URL,
		forgejo_sdk.SetToken("tkn"),
		forgejo_sdk.SetUserAgent("test"),
	)
	if err != nil {
		t.Fatalf("failed to build SDK client for test: %v", err)
	}
	forgejo.SetClientForTesting(c)
	return &records
}

func req(args map[string]any) mcp.CallToolRequest {
	return mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: args}}
}

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

// --- list_releases ----------------------------------------------------------

func TestListReleasesFn_DefaultPagination(t *testing.T) {
	records := newBackend(t, route{
		method:     http.MethodGet,
		pathPrefix: "/api/v1/repos/o/r/releases",
		handler: func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`[{"id":1,"tag_name":"v1","name":"v1","draft":false,"prerelease":false}]`))
		},
	})
	res, err := ListReleasesFn(context.Background(), req(map[string]any{
		"owner": "o", "repo": "r",
	}))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(extractTextContent(t, res), `"v1"`) {
		t.Fatalf("expected v1 in result; got %s", extractTextContent(t, res))
	}
	if q := (*records)[0].query; !strings.Contains(q, "page=1") || !strings.Contains(q, "limit=20") {
		t.Fatalf("expected default page=1 limit=20 in query, got %q", q)
	}
}

func TestListReleasesFn_CustomPagination(t *testing.T) {
	records := newBackend(t, route{
		method:     http.MethodGet,
		pathPrefix: "/api/v1/repos/o/r/releases",
		handler: func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`[]`))
		},
	})
	if _, err := ListReleasesFn(context.Background(), req(map[string]any{
		"owner": "o", "repo": "r", "page": 2.0, "limit": 5.0,
	})); err != nil {
		t.Fatalf("err: %v", err)
	}
	if q := (*records)[0].query; !strings.Contains(q, "page=2") || !strings.Contains(q, "limit=5") {
		t.Fatalf("expected page=2 limit=5 in query, got %q", q)
	}
}

func TestListReleasesFn_StateFilterPublishedExcludesDraftsAndPrereleases(t *testing.T) {
	newBackend(t, route{
		method:     http.MethodGet,
		pathPrefix: "/api/v1/repos/o/r/releases",
		handler: func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`[
				{"id":1,"tag_name":"v1","name":"v1","draft":false,"prerelease":false},
				{"id":2,"tag_name":"v2-pre","name":"v2-pre","draft":false,"prerelease":true},
				{"id":3,"tag_name":"v3-draft","name":"v3-draft","draft":true,"prerelease":false}
			]`))
		},
	})
	res, err := ListReleasesFn(context.Background(), req(map[string]any{
		"owner": "o", "repo": "r", "state": "published",
	}))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	body := extractTextContent(t, res)
	if !strings.Contains(body, `"v1"`) {
		t.Fatalf("expected v1 in filtered output: %s", body)
	}
	if strings.Contains(body, `"v2-pre"`) || strings.Contains(body, `"v3-draft"`) {
		t.Fatalf("published filter should exclude prereleases and drafts: %s", body)
	}
}

func TestListReleasesFn_InvalidStateRejectedBeforeSDK(t *testing.T) {
	called := false
	newBackend(t, route{
		method:     http.MethodGet,
		pathPrefix: "/api/v1/repos/o/r/releases",
		handler: func(w http.ResponseWriter, _ *http.Request) {
			called = true
		},
	})
	_, err := ListReleasesFn(context.Background(), req(map[string]any{
		"owner": "o", "repo": "r", "state": "foo",
	}))
	if err == nil {
		t.Fatalf("expected error for invalid state")
	}
	if called {
		t.Fatalf("SDK should not be called for invalid state")
	}
}

// --- get_release_by_id / by_tag / latest ------------------------------------

func TestGetReleaseByIDFn_NotFound(t *testing.T) {
	newBackend(t, route{
		method:     http.MethodGet,
		pathPrefix: "/api/v1/repos/o/r/releases/999",
		handler: func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"Not Found"}`))
		},
	})
	_, err := GetReleaseByIDFn(context.Background(), req(map[string]any{
		"owner": "o", "repo": "r", "release_id": 999.0,
	}))
	if err == nil {
		t.Fatalf("expected not-found error from SDK")
	}
}

func TestGetReleaseByTagFn_HappyPath(t *testing.T) {
	newBackend(t, route{
		method:     http.MethodGet,
		pathPrefix: "/api/v1/repos/o/r/releases/tags/v1",
		handler: func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"id":1,"tag_name":"v1","name":"v1"}`))
		},
	})
	res, err := GetReleaseByTagFn(context.Background(), req(map[string]any{
		"owner": "o", "repo": "r", "tag": "v1",
	}))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(extractTextContent(t, res), `"v1"`) {
		t.Fatalf("body: %s", extractTextContent(t, res))
	}
}

func TestGetLatestReleaseFn_HappyPath(t *testing.T) {
	newBackend(t, route{
		method:     http.MethodGet,
		pathPrefix: "/api/v1/repos/o/r/releases/latest",
		handler: func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"id":7,"tag_name":"v7","name":"v7"}`))
		},
	})
	res, err := GetLatestReleaseFn(context.Background(), req(map[string]any{
		"owner": "o", "repo": "r",
	}))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(extractTextContent(t, res), `"v7"`) {
		t.Fatalf("body: %s", extractTextContent(t, res))
	}
}

// --- create / edit / delete -------------------------------------------------

func TestCreateReleaseFn_WithTargetCommitish(t *testing.T) {
	records := newBackend(t, route{
		method:     http.MethodPost,
		pathPrefix: "/api/v1/repos/o/r/releases",
		handler: func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"id":11,"tag_name":"v9","name":"v9","target_commitish":"main"}`))
		},
	})
	res, err := CreateReleaseFn(context.Background(), req(map[string]any{
		"owner": "o", "repo": "r",
		"tag_name":         "v9",
		"target_commitish": "main",
		"name":             "v9",
	}))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(extractTextContent(t, res), `"v9"`) {
		t.Fatalf("body: %s", extractTextContent(t, res))
	}
	var body map[string]any
	if err := json.Unmarshal((*records)[0].rawBody, &body); err != nil {
		t.Fatalf("body parse: %v", err)
	}
	if body["target_commitish"] != "main" {
		t.Fatalf("expected target_commitish=main in body, got %v", body)
	}
	if body["tag_name"] != "v9" {
		t.Fatalf("expected tag_name=v9 in body, got %v", body)
	}
}

func TestEditReleaseFn_PartialUpdateBodyOnly(t *testing.T) {
	records := newBackend(t, route{
		method:     http.MethodPatch,
		pathPrefix: "/api/v1/repos/o/r/releases/42",
		handler: func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"id":42,"name":"v1","body":"updated notes"}`))
		},
	})
	if _, err := EditReleaseFn(context.Background(), req(map[string]any{
		"owner": "o", "repo": "r",
		"release_id": 42.0,
		"body":       "updated notes",
	})); err != nil {
		t.Fatalf("err: %v", err)
	}
	var body map[string]any
	if err := json.Unmarshal((*records)[0].rawBody, &body); err != nil {
		t.Fatalf("body parse: %v", err)
	}
	if body["body"] != "updated notes" {
		t.Fatalf("expected body in PATCH, got %v", body)
	}
	// draft / prerelease are *bool — when unset they marshal to null. The
	// SDK omits them as JSON null; the important guarantee is we didn't pin
	// them to false.
	if v, ok := body["draft"]; ok && v != nil {
		t.Fatalf("draft should not be sent as a concrete value when caller did not provide it, got %v", v)
	}
}

func TestDeleteReleaseFn_HappyPath(t *testing.T) {
	newBackend(t, route{
		method:     http.MethodDelete,
		pathPrefix: "/api/v1/repos/o/r/releases/5",
		handler: func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		},
	})
	res, err := DeleteReleaseFn(context.Background(), req(map[string]any{
		"owner": "o", "repo": "r", "release_id": 5.0,
	}))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(extractTextContent(t, res), `"deleted"`) {
		t.Fatalf("body: %s", extractTextContent(t, res))
	}
}

func TestDeleteReleaseByTagFn_HappyPath(t *testing.T) {
	newBackend(t, route{
		method:     http.MethodDelete,
		pathPrefix: "/api/v1/repos/o/r/releases/tags/v1",
		handler: func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		},
	})
	res, err := DeleteReleaseByTagFn(context.Background(), req(map[string]any{
		"owner": "o", "repo": "r", "tag": "v1",
	}))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(extractTextContent(t, res), `"deleted"`) {
		t.Fatalf("body: %s", extractTextContent(t, res))
	}
}

// --- release-attachment tools -----------------------------------------------

func TestCreateReleaseAttachmentFn_RejectsNonBase64(t *testing.T) {
	called := false
	newBackend(t, route{
		method:     http.MethodPost,
		pathPrefix: "/api/v1/repos/o/r/releases/1/assets",
		handler: func(w http.ResponseWriter, _ *http.Request) {
			called = true
		},
	})
	_, err := CreateReleaseAttachmentFn(context.Background(), req(map[string]any{
		"owner": "o", "repo": "r", "release_id": 1.0,
		"content": "not base64!!!", "filename": "f.bin",
	}))
	if err == nil {
		t.Fatalf("expected error for non-base64 content")
	}
	if called {
		t.Fatalf("SDK should not be called when base64 decode fails")
	}
}

func TestCreateReleaseAttachmentFn_HappyPath(t *testing.T) {
	const raw = "release payload bytes"
	encoded := base64.StdEncoding.EncodeToString([]byte(raw))
	records := newBackend(t, route{
		method:     http.MethodPost,
		pathPrefix: "/api/v1/repos/o/r/releases/1/assets",
		handler: func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"id":99,"name":"f.bin","size":21,"uuid":"u","browser_download_url":"x"}`))
		},
	})
	if _, err := CreateReleaseAttachmentFn(context.Background(), req(map[string]any{
		"owner": "o", "repo": "r", "release_id": 1.0,
		"content": encoded, "filename": "f.bin",
	})); err != nil {
		t.Fatalf("err: %v", err)
	}
	rec := (*records)[0]
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

func TestListReleaseAttachmentsFn_ClientSideSlicing(t *testing.T) {
	// SDK fetches the full slice; we slice client-side. Backend always
	// returns 5 attachments regardless of pagination params.
	newBackend(t, route{
		method:     http.MethodGet,
		pathPrefix: "/api/v1/repos/o/r/releases/1/assets",
		handler: func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`[
				{"id":1,"name":"a"},
				{"id":2,"name":"b"},
				{"id":3,"name":"c"},
				{"id":4,"name":"d"},
				{"id":5,"name":"e"}
			]`))
		},
	})

	// page 1, limit 2 → a, b
	res, err := ListReleaseAttachmentsFn(context.Background(), req(map[string]any{
		"owner": "o", "repo": "r", "release_id": 1.0, "page": 1.0, "limit": 2.0,
	}))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	body := extractTextContent(t, res)
	if !strings.Contains(body, `"a"`) || !strings.Contains(body, `"b"`) || strings.Contains(body, `"c"`) {
		t.Fatalf("page=1 limit=2 should yield [a,b]: %s", body)
	}

	// page 3, limit 2 → e only (boundary)
	res, err = ListReleaseAttachmentsFn(context.Background(), req(map[string]any{
		"owner": "o", "repo": "r", "release_id": 1.0, "page": 3.0, "limit": 2.0,
	}))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	body = extractTextContent(t, res)
	if !strings.Contains(body, `"e"`) || strings.Contains(body, `"d"`) {
		t.Fatalf("page=3 limit=2 should yield [e]: %s", body)
	}

	// page 4, limit 2 → empty (past end)
	res, err = ListReleaseAttachmentsFn(context.Background(), req(map[string]any{
		"owner": "o", "repo": "r", "release_id": 1.0, "page": 4.0, "limit": 2.0,
	}))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	body = extractTextContent(t, res)
	if !strings.Contains(body, "[]") {
		t.Fatalf("page=4 should yield empty array: %s", body)
	}
}

func TestDownloadReleaseAttachmentFn_UnderCap_ReturnsBlob(t *testing.T) {
	const payload = "small release asset"
	newBackend(t,
		route{
			method:     http.MethodGet,
			pathPrefix: "/api/v1/repos/o/r/releases/1/assets/42",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				resp := map[string]any{
					"id": 42, "name": "f.bin", "size": len(payload), "uuid": "u",
					"browser_download_url": flag.URL + "/attachments/u",
				}
				_ = json.NewEncoder(w).Encode(resp)
			},
		},
		route{
			method:     http.MethodGet,
			pathPrefix: "/attachments/u",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/octet-stream")
				_, _ = w.Write([]byte(payload))
			},
		},
	)
	res, err := DownloadReleaseAttachmentFn(context.Background(), req(map[string]any{
		"owner": "o", "repo": "r", "release_id": 1.0, "attachment_id": 42.0,
	}))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	br, ok := extractBlobResource(res)
	if !ok {
		t.Fatalf("expected BlobResourceContents in result")
	}
	decoded, err := base64.StdEncoding.DecodeString(br.Blob)
	if err != nil {
		t.Fatalf("blob not base64: %v", err)
	}
	if string(decoded) != payload {
		t.Fatalf("payload mismatch: %q", decoded)
	}
	if !strings.Contains(extractTextContent(t, res), `"inline":true`) {
		t.Fatalf("expected inline:true in text part")
	}
}

func TestDownloadReleaseAttachmentFn_AtCap_NoBlob(t *testing.T) {
	bigSize := forgejo.MaxInlineDownloadBytes
	newBackend(t,
		route{
			method:     http.MethodGet,
			pathPrefix: "/api/v1/repos/o/r/releases/1/assets/99",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				resp := map[string]any{
					"id": 99, "name": "big.bin", "size": bigSize, "uuid": "u-big",
					"browser_download_url": flag.URL + "/attachments/u-big",
				}
				_ = json.NewEncoder(w).Encode(resp)
			},
		},
		// Should not be reached for over-cap; surface a 500 if it is.
		route{
			method:     http.MethodGet,
			pathPrefix: "/attachments/u-big",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
		},
	)
	res, err := DownloadReleaseAttachmentFn(context.Background(), req(map[string]any{
		"owner": "o", "repo": "r", "release_id": 1.0, "attachment_id": 99.0,
	}))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if _, ok := extractBlobResource(res); ok {
		t.Fatalf("expected no blob for at-cap file")
	}
	text := extractTextContent(t, res)
	if !strings.Contains(text, `"inline":false`) {
		t.Fatalf("expected inline:false, got %s", text)
	}
	if !strings.Contains(text, `"reason"`) {
		t.Fatalf("expected reason field populated, got %s", text)
	}
}

func TestDownloadReleaseAttachmentFn_UnknownID_ReturnsError(t *testing.T) {
	newBackend(t, route{
		method:     http.MethodGet,
		pathPrefix: "/api/v1/repos/o/r/releases/1/assets/404",
		handler: func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"Not Found"}`))
		},
	})
	_, err := DownloadReleaseAttachmentFn(context.Background(), req(map[string]any{
		"owner": "o", "repo": "r", "release_id": 1.0, "attachment_id": 404.0,
	}))
	if err == nil {
		t.Fatalf("expected error for unknown attachment id")
	}
}

func TestEditReleaseAttachmentFn_HappyPath(t *testing.T) {
	records := newBackend(t, route{
		method:     http.MethodPatch,
		pathPrefix: "/api/v1/repos/o/r/releases/1/assets/9",
		handler: func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"id":9,"name":"renamed.bin"}`))
		},
	})
	if _, err := EditReleaseAttachmentFn(context.Background(), req(map[string]any{
		"owner": "o", "repo": "r", "release_id": 1.0, "attachment_id": 9.0,
		"name": "renamed.bin",
	})); err != nil {
		t.Fatalf("err: %v", err)
	}
	var body map[string]string
	if err := json.Unmarshal((*records)[0].rawBody, &body); err != nil {
		t.Fatalf("body parse: %v", err)
	}
	if body["name"] != "renamed.bin" {
		t.Fatalf("expected name in PATCH body, got %v", body)
	}
}

func TestDeleteReleaseAttachmentFn_HappyPath(t *testing.T) {
	newBackend(t, route{
		method:     http.MethodDelete,
		pathPrefix: "/api/v1/repos/o/r/releases/1/assets/9",
		handler: func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		},
	})
	res, err := DeleteReleaseAttachmentFn(context.Background(), req(map[string]any{
		"owner": "o", "repo": "r", "release_id": 1.0, "attachment_id": 9.0,
	}))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(extractTextContent(t, res), `"deleted"`) {
		t.Fatalf("body: %s", extractTextContent(t, res))
	}
}

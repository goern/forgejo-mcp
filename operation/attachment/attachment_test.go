package attachment

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/flag"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forgejo"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/upload"

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

func TestCreateIssueAttachmentFn_UsesFilePathAndBasename(t *testing.T) {
	t.Setenv(upload.AllowFilePathEnv, "1")
	path := filepath.Join(t.TempDir(), "from-path.bin")
	if err := os.WriteFile(path, []byte("path payload"), 0o600); err != nil {
		t.Fatal(err)
	}
	records := newBackend(t, route{
		method:     http.MethodPost,
		pathPrefix: "/api/v1/repos/o/r/issues/3/assets",
		handler: func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"id":112,"name":"from-path.bin"}`))
		},
	})
	if _, err := CreateIssueAttachmentFn(context.Background(), req(map[string]any{
		"owner": "o", "repo": "r", "index": 3.0, "file_path": path,
	})); err != nil {
		t.Fatalf("err: %v", err)
	}

	_, params, err := mime.ParseMediaType((*records.records)[0].ctype)
	if err != nil {
		t.Fatal(err)
	}
	part, err := multipart.NewReader(strings.NewReader(string((*records.records)[0].rawBody)), params["boundary"]).NextPart()
	if err != nil {
		t.Fatal(err)
	}
	payload, _ := io.ReadAll(part)
	if part.FileName() != "from-path.bin" || string(payload) != "path payload" {
		t.Fatalf("filename=%q payload=%q", part.FileName(), payload)
	}
}

func TestCreateIssueAttachmentFn_RejectsBothSources(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	backend := newBackend(t)
	_, err := CreateIssueAttachmentFn(context.Background(), req(map[string]any{
		"owner": "o", "repo": "r", "index": 3.0,
		"content": "", "file_path": path, "filename": "empty.txt",
	}))
	if err == nil || !strings.Contains(err.Error(), "exactly one") {
		t.Fatalf("expected exclusive-source error, got %v", err)
	}
	if len(*backend.records) != 0 {
		t.Fatalf("backend called for invalid input")
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

// TestCreateIssueAttachmentFn_RejectsNonBase64_ReportsReceivedLength covers
// the received-length diagnostic improvement: a decode failure must report
// how many bytes THIS SERVER received, so a caller can tell at a glance
// whether truncation happened upstream of this process.
func TestCreateIssueAttachmentFn_RejectsNonBase64_ReportsReceivedLength(t *testing.T) {
	newBackend(t)
	content := "not base64!!!"
	_, err := CreateIssueAttachmentFn(context.Background(), req(map[string]any{
		"owner": "o", "repo": "r", "index": 3.0,
		"content": content, "filename": "f.bin",
	}))
	if err == nil {
		t.Fatalf("expected error for non-base64 content")
	}
	wantFragment := fmt.Sprintf("received %d bytes", len(content))
	if !strings.Contains(err.Error(), wantFragment) {
		t.Fatalf("error %q does not report received length (want fragment %q)", err.Error(), wantFragment)
	}
}

// TestCreateCommentAttachmentFn_RejectsOversizedContent covers the
// defense-in-depth guard: a runaway/malformed content
// argument must be rejected immediately, before decode or upload is
// attempted, with a clear size-limit error.
func TestCreateCommentAttachmentFn_RejectsOversizedContent(t *testing.T) {
	newBackend(t)
	huge := strings.Repeat("A", upload.MaxContentB64Bytes+1)
	_, err := CreateCommentAttachmentFn(context.Background(), req(map[string]any{
		"owner": "o", "repo": "r", "comment_id": 1.0,
		"content": huge, "filename": "f.bin",
	}))
	if err == nil {
		t.Fatalf("expected error for oversized content")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Fatalf("expected a size-limit error, got: %v", err)
	}
}

// withShortUploadTimeout shrinks attachmentUploadTimeout for the duration of
// a test, restoring the production 45s value on cleanup. A stuck upload was
// once reported to hang a caller for ~30 minutes; the fix wraps
// create_*_attachment uploads in a context timeout so a stuck
// network path can never again present as an unbounded hang; these tests
// prove that path actually fires and bounds wall-clock time, rather than
// just asserting the constant exists.
func withShortUploadTimeout(t *testing.T, d time.Duration) {
	t.Helper()
	orig := attachmentUploadTimeout
	attachmentUploadTimeout = d
	t.Cleanup(func() { attachmentUploadTimeout = orig })
}

// hangingHandler never writes a response; it blocks until the request
// context is canceled (i.e. until the client-side upload timeout fires),
// simulating a stuck network path / unresponsive Codeberg backend.
func hangingHandler(w http.ResponseWriter, r *http.Request) {
	<-r.Context().Done()
}

func TestCreateIssueAttachmentFn_UploadTimeout(t *testing.T) {
	withShortUploadTimeout(t, 100*time.Millisecond)
	newBackend(t, route{
		method:     http.MethodPost,
		pathPrefix: "/api/v1/repos/o/r/issues/3/assets",
		handler:    hangingHandler,
	})

	start := time.Now()
	_, err := CreateIssueAttachmentFn(context.Background(), req(map[string]any{
		"owner": "o", "repo": "r", "index": 3.0,
		"content": base64.StdEncoding.EncodeToString([]byte("hang me")), "filename": "f.bin",
	}))
	elapsed := time.Since(start)

	if err == nil {
		t.Fatalf("expected a timeout error, got nil (upload should not hang forever)")
	}
	if !strings.Contains(err.Error(), "upload timed out after") {
		t.Fatalf("expected a clear upload-timeout error, got: %v", err)
	}
	// Generous bound (10x the injected timeout) to absorb scheduler jitter
	// while still proving the call returned promptly rather than hanging
	// for the production 45s (or indefinitely, as in the original report).
	if elapsed > time.Second {
		t.Fatalf("upload took %s to time out; want well under the injected 100ms bound (bounded, not hung)", elapsed)
	}
}

func TestCreateCommentAttachmentFn_UploadTimeout(t *testing.T) {
	withShortUploadTimeout(t, 100*time.Millisecond)
	newBackend(t, route{
		method:     http.MethodPost,
		pathPrefix: "/api/v1/repos/o/r/issues/comments/1/assets",
		handler:    hangingHandler,
	})

	start := time.Now()
	_, err := CreateCommentAttachmentFn(context.Background(), req(map[string]any{
		"owner": "o", "repo": "r", "comment_id": 1.0,
		"content": base64.StdEncoding.EncodeToString([]byte("hang me")), "filename": "f.bin",
	}))
	elapsed := time.Since(start)

	if err == nil {
		t.Fatalf("expected a timeout error, got nil (upload should not hang forever)")
	}
	if !strings.Contains(err.Error(), "upload timed out after") {
		t.Fatalf("expected a clear upload-timeout error, got: %v", err)
	}
	if elapsed > time.Second {
		t.Fatalf("upload took %s to time out; want well under the injected 100ms bound (bounded, not hung)", elapsed)
	}
}

// TestWrapUploadTimeout_ParentDeadlineNotMisattributed is a fast, direct unit
// test of wrapUploadTimeout's context bookkeeping (no real HTTP round trip):
// when the parent context's own deadline already elapsed, the function must
// not claim the upload's 45s budget is what fired.
func TestWrapUploadTimeout_ParentDeadlineNotMisattributed(t *testing.T) {
	parentCtx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	<-parentCtx.Done() // ensure the parent's own deadline has actually elapsed

	uploadCtx, uploadCancel := context.WithTimeout(parentCtx, attachmentUploadTimeout)
	defer uploadCancel()
	<-uploadCtx.Done()

	got := wrapUploadTimeout(parentCtx, uploadCtx, errors.New("context deadline exceeded"))
	if strings.Contains(got.Error(), "upload timed out after") {
		t.Fatalf("wrapUploadTimeout misattributed a parent-deadline expiry to attachmentUploadTimeout: %v", got)
	}
}

// TestWrapUploadTimeout_OwnBudgetStillReported is the companion case: when
// the parent context has no deadline of its own (or has not yet elapsed),
// the upload's own attachmentUploadTimeout budget elapsing must still be
// reported as "upload timed out after <duration>".
func TestWrapUploadTimeout_OwnBudgetStillReported(t *testing.T) {
	parentCtx := context.Background()
	uploadCtx, uploadCancel := context.WithTimeout(parentCtx, time.Nanosecond)
	defer uploadCancel()
	<-uploadCtx.Done()

	got := wrapUploadTimeout(parentCtx, uploadCtx, errors.New("context deadline exceeded"))
	if !strings.Contains(got.Error(), "upload timed out after") {
		t.Fatalf("expected wrapUploadTimeout to attribute the timeout to attachmentUploadTimeout, got: %v", got)
	}
}

// TestCreateIssueAttachmentFn_ParentDeadlineNotMisattributed covers the
// wrapUploadTimeout fix: when the CALLER's own context deadline is what ends
// the call — not attachmentUploadTimeout — the error must not claim "upload
// timed out after 45s". The parent deadline here (20ms) fires well before
// attachmentUploadTimeout (shrunk to 5s, itself far longer than the parent
// budget), so any "upload timed out after 45s"/injected-timeout wording in
// the error would be a lie about which budget actually expired.
func TestCreateIssueAttachmentFn_ParentDeadlineNotMisattributed(t *testing.T) {
	withShortUploadTimeout(t, 5*time.Second)
	newBackend(t, route{
		method:     http.MethodPost,
		pathPrefix: "/api/v1/repos/o/r/issues/3/assets",
		handler:    hangingHandler,
	})

	parentCtx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := CreateIssueAttachmentFn(parentCtx, req(map[string]any{
		"owner": "o", "repo": "r", "index": 3.0,
		"content": base64.StdEncoding.EncodeToString([]byte("hang me")), "filename": "f.bin",
	}))
	elapsed := time.Since(start)

	if err == nil {
		t.Fatalf("expected an error when the caller's own context deadline elapses")
	}
	if strings.Contains(err.Error(), "upload timed out after") {
		t.Fatalf("error wrongly attributes a caller-deadline expiry to attachmentUploadTimeout: %v", err)
	}
	// The call must still return promptly — bounded by the parent's 20ms
	// deadline, not by the (much longer) 5s attachmentUploadTimeout.
	if elapsed > time.Second {
		t.Fatalf("upload took %s; want well under the injected 5s upload-timeout bound (parent deadline should have fired first)", elapsed)
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
				_, _ = fmt.Fprintf(w, `{"id":9,"name":"x","size":15,"uuid":"u","browser_download_url":"%s/attachments/u"}`, flag.URL)
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

func TestCreateCommentAttachmentFn_UsesFilePath(t *testing.T) {
	t.Setenv(upload.AllowFilePathEnv, "1")
	path := filepath.Join(t.TempDir(), "comment.txt")
	if err := os.WriteFile(path, []byte("comment file"), 0o600); err != nil {
		t.Fatal(err)
	}
	backend := newBackend(t, route{
		method:     http.MethodPost,
		pathPrefix: "/api/v1/repos/o/r/issues/comments/55/assets",
		handler: func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"id":11,"name":"comment.txt"}`))
		},
	})
	if _, err := CreateCommentAttachmentFn(context.Background(), req(map[string]any{
		"owner": "o", "repo": "r", "comment_id": 55.0, "file_path": path,
	})); err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(*backend.records) != 1 || !strings.Contains(string((*backend.records)[0].rawBody), "comment file") {
		t.Fatalf("file payload was not uploaded")
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

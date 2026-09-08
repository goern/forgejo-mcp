package forgejo

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/flag"
)

type capturedReq struct {
	method      string
	path        string
	auth        string
	ua          string
	ctype       string
	accept      string
	rangeHeader string
	body        []byte
}

func newCaptureServer(t *testing.T, handler func(w http.ResponseWriter, r *http.Request, c *capturedReq)) (*httptest.Server, *capturedReq) {
	t.Helper()
	c := &capturedReq{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.method = r.Method
		c.path = r.URL.Path
		c.auth = r.Header.Get("Authorization")
		c.ua = r.Header.Get("User-Agent")
		c.ctype = r.Header.Get("Content-Type")
		c.accept = r.Header.Get("Accept")
		c.rangeHeader = r.Header.Get("Range")
		c.body, _ = io.ReadAll(r.Body)
		handler(w, r, c)
	}))
	t.Cleanup(srv.Close)
	flag.URL = srv.URL
	flag.Token = "test-token"
	flag.UserAgent = "forgejo-mcp-test/0.0.1"
	return srv, c
}

func TestDoJSON_GetSuccess_HeadersAndDecode(t *testing.T) {
	_, c := newCaptureServer(t, func(w http.ResponseWriter, r *http.Request, _ *capturedReq) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":42,"name":"hello"}`))
	})

	var out struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	if err := DoJSON(context.Background(), http.MethodGet, "/repos/o/r/issues/1/assets", nil, &out); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if out.ID != 42 || out.Name != "hello" {
		t.Fatalf("decode mismatch: %+v", out)
	}
	if c.path != "/api/v1/repos/o/r/issues/1/assets" {
		t.Fatalf("path: got %q", c.path)
	}
	if c.auth != "token test-token" {
		t.Fatalf("auth header: got %q", c.auth)
	}
	if c.ua != "forgejo-mcp-test/0.0.1" {
		t.Fatalf("user-agent: got %q", c.ua)
	}
}

func TestDoJSON_PostBodySerialised(t *testing.T) {
	_, c := newCaptureServer(t, func(w http.ResponseWriter, r *http.Request, _ *capturedReq) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})

	body := map[string]string{"name": "renamed.txt"}
	if err := DoJSON(context.Background(), http.MethodPatch, "/x", body, nil); err != nil {
		t.Fatalf("err: %v", err)
	}
	if c.method != http.MethodPatch {
		t.Fatalf("method: %s", c.method)
	}
	if c.ctype != "application/json" {
		t.Fatalf("content-type: %s", c.ctype)
	}
	var got map[string]string
	if err := json.Unmarshal(c.body, &got); err != nil {
		t.Fatalf("body not JSON: %v body=%s", err, c.body)
	}
	if got["name"] != "renamed.txt" {
		t.Fatalf("body: %v", got)
	}
}

func TestDoJSON_204NoContent(t *testing.T) {
	newCaptureServer(t, func(w http.ResponseWriter, r *http.Request, _ *capturedReq) {
		w.WriteHeader(http.StatusNoContent)
	})
	if err := DoJSON(context.Background(), http.MethodDelete, "/x/1", nil, nil); err != nil {
		t.Fatalf("expected no error on 204, got: %v", err)
	}
}

func TestDoJSON_4xxErrorTypes(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		want   error
	}{
		{"401", http.StatusUnauthorized, ErrUnauthorized},
		{"403", http.StatusForbidden, ErrUnauthorized},
		{"404", http.StatusNotFound, ErrNotFound},
		{"500", http.StatusInternalServerError, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			newCaptureServer(t, func(w http.ResponseWriter, r *http.Request, _ *capturedReq) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(`{"message":"nope"}`))
			})
			err := DoJSON(context.Background(), http.MethodGet, "/x", nil, nil)
			if err == nil {
				t.Fatalf("expected error for %d", tc.status)
			}
			var he *HTTPError
			if !errors.As(err, &he) {
				t.Fatalf("expected *HTTPError, got %T", err)
			}
			if he.StatusCode != tc.status {
				t.Fatalf("status: got %d want %d", he.StatusCode, tc.status)
			}
			if tc.want != nil && !errors.Is(err, tc.want) {
				t.Fatalf("expected wrapped %v, got %v", tc.want, err)
			}
			if !strings.Contains(he.Body, "nope") {
				t.Fatalf("body snippet missing payload: %q", he.Body)
			}
		})
	}
}

func TestDoJSONList_404IsEmpty(t *testing.T) {
	newCaptureServer(t, func(w http.ResponseWriter, r *http.Request, _ *capturedReq) {
		w.WriteHeader(http.StatusNotFound)
	})
	var out []map[string]any
	if err := DoJSONList(context.Background(), http.MethodGet, "/x", &out); err != nil {
		t.Fatalf("404 on list should be nil err, got: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected empty list, got %d", len(out))
	}
}

func TestDoJSONWithHeader_404IsError(t *testing.T) {
	newCaptureServer(t, func(w http.ResponseWriter, r *http.Request, _ *capturedReq) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"run not found"}`))
	})
	var out []map[string]any
	_, err := DoJSONWithHeader(context.Background(), http.MethodGet, "/x", nil, &out)
	if err == nil {
		t.Fatal("404 must remain an error")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("out should stay empty on error, got %d", len(out))
	}
}

func TestDoJSONWithHeader_ReturnsHeaders(t *testing.T) {
	newCaptureServer(t, func(w http.ResponseWriter, r *http.Request, _ *capturedReq) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set(TotalCountHeader, "4")
		_, _ = w.Write([]byte(`[{"id":1}]`))
	})
	var out []map[string]any
	header, err := DoJSONWithHeader(context.Background(), http.MethodGet, "/x", nil, &out)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if header.Get(TotalCountHeader) != "4" {
		t.Fatalf("X-Total-Count: %q", header.Get(TotalCountHeader))
	}
	if len(out) != 1 || out[0]["id"] != float64(1) {
		t.Fatalf("decode mismatch: %+v", out)
	}
}

func TestDoAPIRaw_RangedResponse(t *testing.T) {
	_, captured := newCaptureServer(t, func(w http.ResponseWriter, r *http.Request, _ *capturedReq) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Content-Range", "bytes 90-99/100")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte("last bytes"))
	})

	resp, err := DoAPIRaw(
		context.Background(),
		"/repos/o/r/actions/jobs/7/logs?attempt=2",
		"text/plain",
		"bytes=-10",
		10,
	)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if captured.path != "/api/v1/repos/o/r/actions/jobs/7/logs" {
		t.Fatalf("path: %s", captured.path)
	}
	if captured.accept != "text/plain" {
		t.Fatalf("accept: %s", captured.accept)
	}
	if captured.rangeHeader != "bytes=-10" {
		t.Fatalf("range: %s", captured.rangeHeader)
	}
	if resp.StatusCode != http.StatusPartialContent {
		t.Fatalf("status: %d", resp.StatusCode)
	}
	if resp.ContentRange != "bytes 90-99/100" {
		t.Fatalf("content range: %s", resp.ContentRange)
	}
	if string(resp.Body) != "last bytes" {
		t.Fatalf("body: %q", resp.Body)
	}
}

func TestDoAPIRaw_RejectsBodyOverCallerBound(t *testing.T) {
	newCaptureServer(t, func(w http.ResponseWriter, r *http.Request, _ *capturedReq) {
		_, _ = w.Write([]byte("sixteen bytes!!!"))
	})

	_, err := DoAPIRaw(context.Background(), "/x", "text/plain", "bytes=0-7", 8)
	if !errors.Is(err, ErrPayloadTooLarge) {
		t.Fatalf("expected ErrPayloadTooLarge, got %v", err)
	}
}

func TestDoMultipart_RoundTrip(t *testing.T) {
	srv, c := newCaptureServer(t, func(w http.ResponseWriter, r *http.Request, _ *capturedReq) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":7,"name":"f.txt"}`))
	})
	_ = srv

	var out struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	const payload = "hello world"
	err := DoMultipart(context.Background(), http.MethodPost, "/repos/o/r/issues/1/assets",
		"attachment", "f.txt", "text/plain", strings.NewReader(payload), &out)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out.ID != 7 || out.Name != "f.txt" {
		t.Fatalf("decoded: %+v", out)
	}

	mt, ps, err := mime.ParseMediaType(c.ctype)
	if err != nil {
		t.Fatalf("parse media type: %v", err)
	}
	if mt != "multipart/form-data" {
		t.Fatalf("media type: %s", mt)
	}
	mr := multipart.NewReader(strings.NewReader(string(c.body)), ps["boundary"])
	part, err := mr.NextPart()
	if err != nil {
		t.Fatalf("next part: %v", err)
	}
	if part.FormName() != "attachment" {
		t.Fatalf("form field name: %s", part.FormName())
	}
	if part.FileName() != "f.txt" {
		t.Fatalf("filename: %s", part.FileName())
	}
	if got := part.Header.Get("Content-Type"); got != "text/plain" {
		t.Fatalf("part content-type: %s", got)
	}
	got, _ := io.ReadAll(part)
	if string(got) != payload {
		t.Fatalf("payload mismatch: %q", got)
	}
}

func TestDoMultipart_DefaultMimeOctetStream(t *testing.T) {
	_, c := newCaptureServer(t, func(w http.ResponseWriter, r *http.Request, _ *capturedReq) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})
	if err := DoMultipart(context.Background(), http.MethodPost, "/x", "attachment", "x.bin", "", strings.NewReader("a"), nil); err != nil {
		t.Fatalf("err: %v", err)
	}
	_, ps, _ := mime.ParseMediaType(c.ctype)
	mr := multipart.NewReader(strings.NewReader(string(c.body)), ps["boundary"])
	part, err := mr.NextPart()
	if err != nil {
		t.Fatalf("next part: %v", err)
	}
	if got := part.Header.Get("Content-Type"); got != "application/octet-stream" {
		t.Fatalf("expected default octet-stream, got %s", got)
	}
}

type gatedReader struct {
	started <-chan struct{}
	reader  io.Reader
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) {
	return 0, errors.New("read failed")
}

type contextReader struct {
	ctx context.Context
}

func (r contextReader) Read([]byte) (int, error) {
	<-r.ctx.Done()
	return 0, r.ctx.Err()
}

func (r *gatedReader) Read(p []byte) (int, error) {
	<-r.started
	return r.reader.Read(p)
}

func TestDoMultipartStreamsAfterRequestStarts(t *testing.T) {
	requestStarted := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(requestStarted)
		_, _ = io.Copy(io.Discard, r.Body)
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)
	flag.URL = srv.URL

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	reader := &gatedReader{started: requestStarted, reader: strings.NewReader("streamed")}
	if err := DoMultipart(ctx, http.MethodPost, "/x", "attachment", "x.bin", "", reader, nil); err != nil {
		t.Fatalf("streaming upload failed: %v", err)
	}
}

func TestDoMultipartPropagatesReaderError(t *testing.T) {
	newCaptureServer(t, func(w http.ResponseWriter, r *http.Request, _ *capturedReq) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})
	err := DoMultipart(context.Background(), http.MethodPost, "/x", "attachment", "x.bin", "", failingReader{}, nil)
	if err == nil || !strings.Contains(err.Error(), "read failed") {
		t.Fatalf("expected reader error, got %v", err)
	}
}

func TestDoMultipartHonorsContextCancellation(t *testing.T) {
	newCaptureServer(t, func(w http.ResponseWriter, r *http.Request, _ *capturedReq) {
		w.WriteHeader(http.StatusOK)
	})
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(20*time.Millisecond, cancel)
	err := DoMultipart(ctx, http.MethodPost, "/x", "attachment", "x.bin", "", contextReader{ctx: ctx}, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
}

func TestDoRaw_SuccessUnderCap(t *testing.T) {
	srv, c := newCaptureServer(t, func(w http.ResponseWriter, r *http.Request, _ *capturedReq) {
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write([]byte("PDFBYTES"))
	})
	_ = srv
	body, ct, err := DoRaw(context.Background(), "/attachments/uuid-1")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if string(body) != "PDFBYTES" {
		t.Fatalf("body: %q", body)
	}
	if ct != "application/pdf" {
		t.Fatalf("content-type: %s", ct)
	}
	// DoRaw must NOT prepend /api/v1.
	if c.path != "/attachments/uuid-1" {
		t.Fatalf("path: %s", c.path)
	}
	if c.auth != "token test-token" {
		t.Fatalf("auth header missing on download: %s", c.auth)
	}
}

func TestDoRaw_ExceedsCap(t *testing.T) {
	bigPayload := make([]byte, MaxInlineDownloadBytes+1024)
	for i := range bigPayload {
		bigPayload[i] = 'a'
	}
	newCaptureServer(t, func(w http.ResponseWriter, r *http.Request, _ *capturedReq) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(bigPayload)
	})
	_, _, err := DoRaw(context.Background(), "/attachments/big")
	if !errors.Is(err, ErrPayloadTooLarge) {
		t.Fatalf("expected ErrPayloadTooLarge, got %v", err)
	}
}

func TestDoRaw_AbsoluteURLPassThrough(t *testing.T) {
	srv, c := newCaptureServer(t, func(w http.ResponseWriter, r *http.Request, _ *capturedReq) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("hi"))
	})
	body, _, err := DoRaw(context.Background(), srv.URL+"/attachments/abs-uuid")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if string(body) != "hi" {
		t.Fatalf("body: %s", body)
	}
	if c.path != "/attachments/abs-uuid" {
		t.Fatalf("path: %s", c.path)
	}
}

func TestResolveURL_BaseRequired(t *testing.T) {
	flag.URL = ""
	defer func() { flag.URL = "http://x" }()
	if _, err := resolveURL("/x"); err == nil {
		t.Fatalf("expected error when flag.URL empty")
	}
}

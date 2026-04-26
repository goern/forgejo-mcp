package forgejo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"
	"time"

	"codeberg.org/goern/forgejo-mcp/v2/pkg/flag"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/log"
)

// MaxInlineDownloadBytes caps inline base64 attachment payloads.
// Files at or above this size return metadata only; the caller is expected
// to fetch browser_download_url directly with the same auth token.
// See docs/plans/issue-attachments.md and Codeberg issue #106.
const MaxInlineDownloadBytes = 1 * 1024 * 1024

// Error sentinels callers can match with errors.Is.
var (
	ErrUnauthorized = errors.New("unauthorized")
	ErrNotFound     = errors.New("not found")
	ErrPayloadTooLarge = errors.New("payload exceeds inline cap")
)

// HTTPError carries the response status and a sanitised body snippet.
// It wraps one of the sentinels above when the status maps to one.
type HTTPError struct {
	StatusCode int
	Status     string
	Body       string
	Method     string
	URL        string
	wrapped    error
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("%s %s: %s: %s", e.Method, e.URL, e.Status, e.Body)
}

func (e *HTTPError) Unwrap() error { return e.wrapped }

// rawHTTPClient is package-level so tests can swap timeouts; a single
// shared client lets keep-alives work across tool calls.
var rawHTTPClient = &http.Client{Timeout: 60 * time.Second}

// userAgent returns the configured UA, falling back to forgejo-mcp/<version>.
func userAgent() string {
	if flag.UserAgent != "" {
		return flag.UserAgent
	}
	return "forgejo-mcp/" + flag.Version
}

// resolveURL turns a path or absolute URL into an absolute URL string.
// API paths (e.g. "/repos/x/y/issues/1/assets") are prefixed with
// flag.URL + "/api/v1". Absolute URLs are returned verbatim.
func resolveURL(pathOrURL string) (string, error) {
	if strings.HasPrefix(pathOrURL, "http://") || strings.HasPrefix(pathOrURL, "https://") {
		return pathOrURL, nil
	}
	base := strings.TrimRight(flag.URL, "/")
	if base == "" {
		return "", fmt.Errorf("flag.URL is empty; raw-HTTP helper needs a configured base URL")
	}
	if !strings.HasPrefix(pathOrURL, "/") {
		pathOrURL = "/" + pathOrURL
	}
	// Forgejo REST API root.
	return base + "/api/v1" + pathOrURL, nil
}

// resolveSameOriginURL is like resolveURL but for asset/download URLs that
// live outside the /api/v1 prefix (e.g. /attachments/{uuid}). Absolute URLs
// pass through; relative URLs hang off flag.URL with no /api/v1.
func resolveSameOriginURL(pathOrURL string) (string, error) {
	if strings.HasPrefix(pathOrURL, "http://") || strings.HasPrefix(pathOrURL, "https://") {
		return pathOrURL, nil
	}
	base := strings.TrimRight(flag.URL, "/")
	if base == "" {
		return "", fmt.Errorf("flag.URL is empty; raw-HTTP helper needs a configured base URL")
	}
	if !strings.HasPrefix(pathOrURL, "/") {
		pathOrURL = "/" + pathOrURL
	}
	return base + pathOrURL, nil
}

func setCommonHeaders(req *http.Request) {
	req.Header.Set("Authorization", "token "+flag.Token)
	req.Header.Set("User-Agent", userAgent())
	req.Header.Set("Accept", "application/json")
}

// doRequest sends req, returns the response, mapping common HTTP errors to
// the sentinels above. Caller owns response body close.
func doRequest(ctx context.Context, req *http.Request) (*http.Response, error) {
	start := time.Now()
	resp, err := rawHTTPClient.Do(req)
	duration := time.Since(start)
	endpoint := req.URL.Path
	if req.URL.RawQuery != "" {
		endpoint += "?" + req.URL.RawQuery
	}
	if err != nil {
		LogAPICall(ctx, req.Method, endpoint, duration, 0, err)
		return nil, fmt.Errorf("%s %s: %w", req.Method, req.URL.String(), err)
	}
	LogAPICall(ctx, req.Method, endpoint, duration, resp.StatusCode, nil)
	return resp, nil
}

// readBodySnippet reads up to 1 KiB of the body for inclusion in errors.
// It does not close the body.
func readBodySnippet(r io.Reader) string {
	buf := make([]byte, 1024)
	n, _ := io.ReadFull(io.LimitReader(r, 1024), buf)
	return string(buf[:n])
}

// httpErrorFromResponse builds an HTTPError mapping status to a sentinel.
func httpErrorFromResponse(req *http.Request, resp *http.Response) *HTTPError {
	body := readBodySnippet(resp.Body)
	e := &HTTPError{
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		Body:       body,
		Method:     req.Method,
		URL:        req.URL.String(),
	}
	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		e.wrapped = ErrUnauthorized
	case resp.StatusCode == http.StatusNotFound:
		e.wrapped = ErrNotFound
	}
	return e
}

// DoJSON performs an authenticated JSON request. Encodes body as JSON if
// non-nil; decodes 2xx response into out if non-nil. 204 responses are
// always success-with-no-body. 4xx/5xx return *HTTPError.
//
// The boolean isList signals "list endpoint": for those, 404 is treated
// as an empty list (not an error), matching Forgejo's habit of 404ing
// list endpoints when the parent entity has no children.
func DoJSON(ctx context.Context, method, pathOrURL string, body, out any) error {
	full, err := resolveURL(pathOrURL)
	if err != nil {
		return err
	}
	var bodyReader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, full, bodyReader)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	setCommonHeaders(req)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := doRequest(ctx, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return httpErrorFromResponse(req, resp)
	}
	if resp.StatusCode == http.StatusNoContent || out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

// DoJSONList is like DoJSON but treats 404 as "empty list" (no error,
// out left at its zero value).
func DoJSONList(ctx context.Context, method, pathOrURL string, out any) error {
	err := DoJSON(ctx, method, pathOrURL, nil, out)
	var he *HTTPError
	if errors.As(err, &he) && he.StatusCode == http.StatusNotFound {
		return nil
	}
	return err
}

// quoteEscaper mirrors mime/multipart's internal escaper for filenames.
var quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"")

// DoMultipart uploads a single file part via multipart/form-data and
// decodes the JSON response into out (if non-nil).
func DoMultipart(ctx context.Context, method, pathOrURL, fieldName, filename, mimeType string, r io.Reader, out any) error {
	full, err := resolveURL(pathOrURL)
	if err != nil {
		return err
	}

	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)

	h := textproto.MIMEHeader{}
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`,
		quoteEscaper.Replace(fieldName), quoteEscaper.Replace(filename)))
	if mimeType != "" {
		h.Set("Content-Type", mimeType)
	} else {
		h.Set("Content-Type", "application/octet-stream")
	}
	part, err := mw.CreatePart(h)
	if err != nil {
		return fmt.Errorf("create multipart part: %w", err)
	}
	if _, err := io.Copy(part, r); err != nil {
		return fmt.Errorf("copy file into part: %w", err)
	}
	if err := mw.Close(); err != nil {
		return fmt.Errorf("close multipart writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, full, body)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	setCommonHeaders(req)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := doRequest(ctx, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return httpErrorFromResponse(req, resp)
	}
	if resp.StatusCode == http.StatusNoContent || out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

// DoRaw fetches bytes from a URL (absolute or relative-to-flag.URL with no
// /api/v1 prefix), adding the configured auth header. Caps the response at
// MaxInlineDownloadBytes; ErrPayloadTooLarge is returned if the body would
// exceed the cap. Returns body bytes + content type.
func DoRaw(ctx context.Context, pathOrURL string) ([]byte, string, error) {
	full, err := resolveSameOriginURL(pathOrURL)
	if err != nil {
		return nil, "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, full, nil)
	if err != nil {
		return nil, "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "token "+flag.Token)
	req.Header.Set("User-Agent", userAgent())
	// Don't constrain Accept here — the asset endpoint is binary.

	resp, err := doRequest(ctx, req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", httpErrorFromResponse(req, resp)
	}

	// Read up to cap+1 to detect overflow.
	limited := io.LimitReader(resp.Body, MaxInlineDownloadBytes+1)
	buf, err := io.ReadAll(limited)
	if err != nil {
		return nil, "", fmt.Errorf("read body: %w", err)
	}
	if int64(len(buf)) > MaxInlineDownloadBytes {
		return nil, "", ErrPayloadTooLarge
	}
	ct := resp.Header.Get("Content-Type")
	return buf, ct, nil
}

// helper used by tests to validate URL construction directly.
func init() {
	// Validate at init that net/url accepts our base format if set.
	if flag.URL != "" {
		if _, err := url.Parse(flag.URL); err != nil {
			log.Errorf("flag.URL is not a valid URL: %v", err)
		}
	}
}

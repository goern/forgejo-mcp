// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v2/operation/resource"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v2/pkg/flag"

	"github.com/mark3labs/mcp-go/mcp"
)

// The label resources page. Until this file existed, nothing tested that they
// page *correctly*: resources_label_pagelimit_test.go proves pageLimit parses
// the query string, and the rest of the label tests use mocks that return the
// same canned body whatever page is asked for. Both are blind to the defect
// these tests guard — requesting limit+1 rows as a truncation probe while
// showing only limit of them, so that upstream's (page-1)*PageSize offset makes
// page N+1 begin one row past the last row page N showed. That row is returned
// by no page a client can ask for.

// labelRow is the shape the paginating mock serves for both label endpoints.
func labelRow(i int) map[string]interface{} {
	return map[string]interface{}{
		"id":          i,
		"name":        fmt.Sprintf("label-%d", i),
		"color":       "aabbcc",
		"description": "",
		"url":         fmt.Sprintf("https://example.test/labels/%d", i),
	}
}

// setupLabelPagingServer wires a paginating backend for both the SDK path
// (repo labels) and the raw-HTTP path (org labels, which needs flag.URL).
func setupLabelPagingServer(t *testing.T, total int) *httptest.Server {
	t.Helper()
	srv := setupPaginatingServer(t, &paginatingHandler{
		total:   total,
		pathHas: "/labels",
		row:     labelRow,
	})
	flag.URL = srv.URL
	flag.Token = "tkn"
	flag.UserAgent = "test"
	return srv
}

type labelResourceReader func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error)

func readLabelPayload(t *testing.T, read labelResourceReader, uri string) map[string]interface{} {
	t.Helper()
	contents, err := read(context.Background(), readReq(uri))
	if err != nil {
		t.Fatalf("read %s: %v", uri, err)
	}
	if len(contents) == 0 {
		t.Fatalf("read %s: no contents", uri)
	}
	text, ok := contents[0].(mcp.TextResourceContents)
	if !ok {
		t.Fatalf("read %s: first content block is not text: %#v", uri, contents[0])
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(text.Text), &payload); err != nil {
		t.Fatalf("read %s: payload is not JSON: %v\n%s", uri, err, text.Text)
	}
	return payload
}

// labelIDs returns the ids in payload order, so a caller can assert on what a
// page actually showed rather than merely how many rows it counted.
func labelIDs(t *testing.T, payload map[string]interface{}) []int {
	t.Helper()
	rows, ok := payload["labels"].([]interface{})
	if !ok {
		t.Fatalf("payload has no labels array: %#v", payload)
	}
	ids := make([]int, 0, len(rows))
	for _, r := range rows {
		row, ok := r.(map[string]interface{})
		if !ok {
			t.Fatalf("label row is not an object: %#v", r)
		}
		id, _ := row["id"].(float64)
		ids = append(ids, int(id))
	}
	return ids
}

// walkLabelPages reads pages 1..pages and returns every id the client saw, in
// order. A row skipped at a page boundary simply never appears.
func walkLabelPages(t *testing.T, read labelResourceReader, uriFmt string, pages, limit int) []int {
	t.Helper()
	var seen []int
	for page := 1; page <= pages; page++ {
		payload := readLabelPayload(t, read, fmt.Sprintf(uriFmt, page, limit))
		seen = append(seen, labelIDs(t, payload)...)
	}
	return seen
}

// ---- the property that matters: successive pages cover every row exactly once ----

func TestRepoLabelsResource_PagesCoverEveryRow(t *testing.T) {
	setupLabelPagingServer(t, 100)

	const limit = 30
	seen := walkLabelPages(t, repoLabelsResourceHandler,
		"forgejo://repo/o/r/labels?page=%d&limit=%d", 3, limit)
	assertPagesCoverCorpus(t, seen, 3, limit)
}

func TestOrgLabelsResource_PagesCoverEveryRow(t *testing.T) {
	setupLabelPagingServer(t, 100)

	const limit = 30
	seen := walkLabelPages(t, orgLabelsResourceHandler,
		"forgejo://org/o/labels?page=%d&limit=%d", 3, limit)
	assertPagesCoverCorpus(t, seen, 3, limit)
}

// A non-default limit exercises the boundary arithmetic at a different stride,
// where an off-by-one is a different absolute row number.
func TestRepoLabelsResource_PagesCoverEveryRowAtSmallLimit(t *testing.T) {
	setupLabelPagingServer(t, 100)

	const limit = 7
	seen := walkLabelPages(t, repoLabelsResourceHandler,
		"forgejo://repo/o/r/labels?page=%d&limit=%d", 4, limit)
	assertPagesCoverCorpus(t, seen, 4, limit)
}

// ---- truncation is the server's answer, not an item count ----

func TestRepoLabelsResource_TruncationComesFromTheHeader(t *testing.T) {
	setupLabelPagingServer(t, 100)

	payload := readLabelPayload(t, repoLabelsResourceHandler,
		"forgejo://repo/o/r/labels?page=1&limit=30")

	if got := len(labelIDs(t, payload)); got != 30 {
		t.Fatalf("expected a full page of 30 rows, got %d", got)
	}
	// Nothing in the row count says the corpus continues — only the Link header does.
	if truncated, _ := payload["truncated"].(bool); !truncated {
		t.Fatalf("a full page with rel=\"next\" must report truncated (rows=%d, sentinel=%q)",
			len(labelIDs(t, payload)), payload["sentinel"])
	}
	sentinel, _ := payload["sentinel"].(string)
	if !strings.Contains(sentinel, "30 of 100 items shown") {
		t.Fatalf("sentinel should carry the server's total from X-Total-Count, got %q", sentinel)
	}
	if tool, _ := payload["list_tool"].(string); tool != ListRepoLabelsToolName {
		t.Fatalf("truncated payload must name the enumeration tool, got %q", tool)
	}
}

func TestOrgLabelsResource_TruncationComesFromTheHeader(t *testing.T) {
	setupLabelPagingServer(t, 100)

	payload := readLabelPayload(t, orgLabelsResourceHandler,
		"forgejo://org/o/labels?page=1&limit=30")

	if got := len(labelIDs(t, payload)); got != 30 {
		t.Fatalf("expected a full page of 30 rows, got %d", got)
	}
	if truncated, _ := payload["truncated"].(bool); !truncated {
		t.Fatalf("a full page with rel=\"next\" must report truncated (rows=%d, sentinel=%q)",
			len(labelIDs(t, payload)), payload["sentinel"])
	}
	sentinel, _ := payload["sentinel"].(string)
	if !strings.Contains(sentinel, "30 of 100 items shown") {
		t.Fatalf("sentinel should carry the server's total from X-Total-Count, got %q", sentinel)
	}
	if tool, _ := payload["list_tool"].(string); tool != ListOrgLabelsToolName {
		t.Fatalf("truncated payload must name the enumeration tool, got %q", tool)
	}
}

// The other half: no rel="next", so no sentinel. Without this, "always
// truncated" would pass the tests above.

func TestRepoLabelsResource_LastPageIsNotTruncated(t *testing.T) {
	setupLabelPagingServer(t, 30)

	payload := readLabelPayload(t, repoLabelsResourceHandler,
		"forgejo://repo/o/r/labels?page=1&limit=30")

	if got := len(labelIDs(t, payload)); got != 30 {
		t.Fatalf("expected all 30 rows, got %d", got)
	}
	if truncated, _ := payload["truncated"].(bool); truncated {
		t.Fatalf("a corpus that ends exactly at the page boundary is not truncated (sentinel=%q)",
			payload["sentinel"])
	}
	if sentinel, _ := payload["sentinel"].(string); sentinel != "" {
		t.Fatalf("expected no sentinel, got %q", sentinel)
	}
}

func TestOrgLabelsResource_LastPageIsNotTruncated(t *testing.T) {
	setupLabelPagingServer(t, 30)

	payload := readLabelPayload(t, orgLabelsResourceHandler,
		"forgejo://org/o/labels?page=1&limit=30")

	if got := len(labelIDs(t, payload)); got != 30 {
		t.Fatalf("expected all 30 rows, got %d", got)
	}
	if truncated, _ := payload["truncated"].(bool); truncated {
		t.Fatalf("a corpus that ends exactly at the page boundary is not truncated (sentinel=%q)",
			payload["sentinel"])
	}
	if sentinel, _ := payload["sentinel"].(string); sentinel != "" {
		t.Fatalf("expected no sentinel, got %q", sentinel)
	}
}

// An instance that advertises a next page but sends no X-Total-Count must not
// have a total invented for it.
func TestRepoLabelsResource_TruncationWithoutTotalHeader(t *testing.T) {
	srv := httptest.NewServer(&noTotalPaginatingHandler{total: 100, pathHas: "/labels", row: labelRow})
	t.Cleanup(srv.Close)
	setupSDKClientFor(t, srv.URL)

	payload := readLabelPayload(t, repoLabelsResourceHandler,
		"forgejo://repo/o/r/labels?page=1&limit=30")

	if truncated, _ := payload["truncated"].(bool); !truncated {
		t.Fatalf("rel=\"next\" alone must still report truncated (rows=%d, sentinel=%q)",
			len(labelIDs(t, payload)), payload["sentinel"])
	}
	sentinel, _ := payload["sentinel"].(string)
	if !strings.Contains(sentinel, "30 items shown, more remain") {
		t.Fatalf("without X-Total-Count the sentinel must not claim a total, got %q", sentinel)
	}
}

// The caller's limit is what reaches the API — no over-fetch. This is the
// assertion the old contract got backwards.
func TestRepoLabelsResource_RequestsExactlyTheCallersLimit(t *testing.T) {
	rec := &limitRecordingHandler{}
	srv := httptest.NewServer(rec)
	t.Cleanup(srv.Close)
	setupSDKClientFor(t, srv.URL)

	readLabelPayload(t, repoLabelsResourceHandler, "forgejo://repo/o/r/labels?page=2&limit=5")

	if rec.limit != "5" {
		t.Fatalf("page size must equal the caller's limit exactly, got limit=%q (an over-fetch "+
			"makes page N+1 start past the last row page N showed)", rec.limit)
	}
	if rec.page != "2" {
		t.Fatalf("page must reach the API, got page=%q", rec.page)
	}
}

func TestRepoLabelsResource_ClampsLimitToCap(t *testing.T) {
	rec := &limitRecordingHandler{}
	srv := httptest.NewServer(rec)
	t.Cleanup(srv.Close)
	setupSDKClientFor(t, srv.URL)

	readLabelPayload(t, repoLabelsResourceHandler, "forgejo://repo/o/r/labels?limit=999")

	if want := fmt.Sprint(resource.EmbeddedListCap); rec.limit != want {
		t.Fatalf("a limit above the ceiling must clamp to %s, got %q", want, rec.limit)
	}
}

// Forgejo answers 404 rather than [] when an org has no labels at all, and the
// raw-HTTP list helper maps that to an empty list. Reading the paging headers
// meant threading them out of that helper alongside the error, so the 404 path
// is the one place the refactor could plausibly have turned "no labels" back
// into a failed read. Nothing else in the suite exercises it.
func TestOrgLabelsResource_NotFoundMeansEmptyList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"message":"label does not exist"}`, http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)
	setupSDKClientFor(t, srv.URL)

	payload := readLabelPayload(t, orgLabelsResourceHandler, "forgejo://org/o/labels?limit=5")

	if got := len(labelIDs(t, payload)); got != 0 {
		t.Fatalf("a 404 must read as an empty label list, got %d rows", got)
	}
	if truncated, _ := payload["truncated"].(bool); truncated {
		t.Fatalf("an empty list is not truncated (sentinel=%q)", payload["sentinel"])
	}
	if org, _ := payload["org"].(string); org != "o" {
		t.Fatalf("payload lost the org it was asked about: %#v", payload["org"])
	}
}

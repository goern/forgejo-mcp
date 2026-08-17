// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v2/operation/resource"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v2/pkg/forgejo"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
)

// listRoutingHandler serves the repo-issues list and the issue-comments list, and
// records the query string it was called with so the tests can assert that
// client-controlled bounds and filters actually reach the API.
type listRoutingHandler struct {
	issuesStatus   int
	issuesBody     interface{}
	commentsStatus int
	commentsBody   interface{}

	lastIssuesQuery   string
	lastCommentsQuery string
}

func (h *listRoutingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch {
	case strings.Contains(r.URL.Path, "/comments"):
		h.lastCommentsQuery = r.URL.RawQuery
		w.WriteHeader(h.commentsStatus)
		if h.commentsBody != nil {
			_ = json.NewEncoder(w).Encode(h.commentsBody)
		}
	case strings.HasSuffix(r.URL.Path, "/issues"):
		h.lastIssuesQuery = r.URL.RawQuery
		w.WriteHeader(h.issuesStatus)
		if h.issuesBody != nil {
			_ = json.NewEncoder(w).Encode(h.issuesBody)
		}
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

// paginatingHandler serves a corpus of `total` rows with the real upstream
// offset semantics — offset = (page-1)*PageSize — plus the `Link` and
// `X-Total-Count` headers Forgejo actually sends.
//
// listRoutingHandler above cannot see a paging bug, because it returns the same
// canned body whatever page is asked for. That is precisely how requesting
// limit+1 rows per page while showing only limit of them went unnoticed: every
// page boundary silently skipped a row, and no test that ignores `page` can
// tell.
type paginatingHandler struct {
	total   int
	pathHas string
	row     func(i int) map[string]interface{}
}

func (h *paginatingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if !strings.Contains(r.URL.Path, h.pathHas) {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	size, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if size < 1 {
		size = 10
	}

	offset := (page - 1) * size
	rows := make([]interface{}, 0, size)
	for i := offset + 1; i <= offset+size && i <= h.total; i++ {
		rows = append(rows, h.row(i))
	}

	w.Header().Set("X-Total-Count", strconv.Itoa(h.total))
	if offset+len(rows) < h.total {
		w.Header().Set("Link", fmt.Sprintf(`<%s?page=%d&limit=%d>; rel="next"`, r.URL.Path, page+1, size))
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(rows)
}

func setupPaginatingServer(t *testing.T, h *paginatingHandler) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	client, err := forgejo_sdk.NewClient(srv.URL, forgejo_sdk.SetForgejoVersion("7.0.0"))
	if err != nil {
		t.Fatalf("creating test client: %v", err)
	}
	forgejo.SetClientForTesting(client)
	return srv
}

// assertPagesCoverCorpus walks pages 1..pages and fails if the union of what
// the client saw is not exactly the contiguous run it should be. A row that no
// page returns is the defect this guards; a row two pages both return would be
// the opposite mistake.
func assertPagesCoverCorpus(t *testing.T, seen []int, pages, limit int) {
	t.Helper()
	want := pages * limit
	if len(seen) != want {
		t.Fatalf("expected %d rows across %d pages of %d, got %d: %v", want, pages, limit, len(seen), seen)
	}
	for i, got := range seen {
		if got != i+1 {
			t.Fatalf("page boundary lost or repeated a row at position %d: want %d, got %d\nfull sequence: %v",
				i, i+1, got, seen)
		}
	}
}

func setupListMockServer(t *testing.T, h *listRoutingHandler) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(h)
	client, err := forgejo_sdk.NewClient(srv.URL, forgejo_sdk.SetForgejoVersion("7.0.0"))
	if err != nil {
		t.Fatalf("creating test client: %v", err)
	}
	forgejo.SetClientForTesting(client)
	return srv
}

func fakeIssueRow(number int, title string) map[string]interface{} {
	return map[string]interface{}{
		"id":     number,
		"number": number,
		"title":  title,
		"body":   strings.Repeat("a long body that must not appear in a list row. ", 20),
		"state":  "open",
		"user":   map[string]interface{}{"login": "alice"},
		"labels": []interface{}{
			map[string]interface{}{"id": 1, "name": "security"},
		},
		"assignees":  []interface{}{map[string]interface{}{"login": "bob"}},
		"comments":   3,
		"created_at": "2026-08-01T10:00:00Z",
		"updated_at": "2026-08-02T11:00:00Z",
		"due_date":   "2026-09-01T00:00:00Z",
	}
}

func fakeIssueRows(n int) []interface{} {
	rows := make([]interface{}, 0, n)
	for i := 1; i <= n; i++ {
		rows = append(rows, fakeIssueRow(i, fmt.Sprintf("issue %d", i)))
	}
	return rows
}

func fakeCommentRow(id int, body string) map[string]interface{} {
	return map[string]interface{}{
		"id":         id,
		"body":       body,
		"user":       map[string]interface{}{"login": "carol"},
		"created_at": "2026-08-03T09:00:00Z",
		"updated_at": "2026-08-03T09:00:00Z",
	}
}

func readListPayload(t *testing.T, contents []mcp.ResourceContents, into interface{}) {
	t.Helper()
	if len(contents) == 0 {
		t.Fatal("no resource contents returned")
	}
	text, ok := contents[0].(mcp.TextResourceContents)
	if !ok {
		t.Fatalf("expected TextResourceContents, got %T", contents[0])
	}
	if text.MIMEType != "application/json" {
		t.Fatalf("expected application/json, got %q", text.MIMEType)
	}
	if err := json.Unmarshal([]byte(text.Text), into); err != nil {
		t.Fatalf("unmarshal payload: %v\npayload was: %s", err, text.Text)
	}
}

// ---- repo issues list ----

func TestRepoIssuesResource_RowsCarryNoBodies(t *testing.T) {
	h := &listRoutingHandler{issuesStatus: http.StatusOK, issuesBody: fakeIssueRows(2)}
	srv := setupListMockServer(t, h)
	defer srv.Close()

	contents, err := repoIssuesResourceHandler(context.Background(), readReq("forgejo://repo/o/r/issues"))
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}

	text := contents[0].(mcp.TextResourceContents).Text
	// The whole point of this resource: rows, not prose.
	if strings.Contains(text, "a long body that must not appear") {
		t.Fatal("issue body leaked into the list payload")
	}

	var payload issuesListPayload
	readListPayload(t, contents, &payload)
	if len(payload.Issues) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(payload.Issues))
	}
	row := payload.Issues[0]
	if row.Index != 1 || row.Title != "issue 1" || row.State != "open" {
		t.Fatalf("row not mapped: %+v", row)
	}
	if row.Author != "alice" {
		t.Fatalf("author not flattened to a login: %q", row.Author)
	}
	if len(row.Labels) != 1 || row.Labels[0] != "security" {
		t.Fatalf("labels not flattened to names: %+v", row.Labels)
	}
	if len(row.Assignees) != 1 || row.Assignees[0] != "bob" {
		t.Fatalf("assignees not flattened to logins: %+v", row.Assignees)
	}
	if row.CommentCount != 3 {
		t.Fatalf("comment_count not carried: %d", row.CommentCount)
	}
	if row.DueDate == "" {
		t.Fatal("due_date dropped")
	}
}

func TestRepoIssuesResource_DefaultsToOpen(t *testing.T) {
	h := &listRoutingHandler{issuesStatus: http.StatusOK, issuesBody: fakeIssueRows(1)}
	srv := setupListMockServer(t, h)
	defer srv.Close()

	if _, err := repoIssuesResourceHandler(context.Background(), readReq("forgejo://repo/o/r/issues")); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if !strings.Contains(h.lastIssuesQuery, "state=open") {
		t.Fatalf("expected state=open by default, query was %q", h.lastIssuesQuery)
	}
}

func TestRepoIssuesResource_FiltersReachTheAPI(t *testing.T) {
	h := &listRoutingHandler{issuesStatus: http.StatusOK, issuesBody: fakeIssueRows(1)}
	srv := setupListMockServer(t, h)
	defer srv.Close()

	req := readReq("forgejo://repo/o/r/issues?state=all&labels=security,%20for:coordination&page=2&limit=5")
	contents, err := repoIssuesResourceHandler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	q := h.lastIssuesQuery
	if !strings.Contains(q, "state=all") {
		t.Fatalf("state filter not forwarded: %q", q)
	}
	if !strings.Contains(q, "security") || !strings.Contains(q, "for%3Acoordination") {
		t.Fatalf("labels filter not forwarded (whitespace should be trimmed): %q", q)
	}
	if !strings.Contains(q, "page=2") {
		t.Fatalf("page not forwarded: %q", q)
	}
	// The page size is the caller's limit exactly. Asking for limit+1 to probe
	// for truncation would desynchronise the upstream offset from the rows
	// shown and skip a row at every page boundary — see
	// TestRepoIssuesResource_PagesCoverEveryRow.
	if !strings.Contains(q, "limit=5") {
		t.Fatalf("expected the caller's limit (5) to be requested verbatim, query was %q", q)
	}

	var payload issuesListPayload
	readListPayload(t, contents, &payload)
	if payload.State != "all" || payload.Page != 2 || payload.Limit != 5 {
		t.Fatalf("echoed bounds wrong: %+v", payload)
	}
	if len(payload.Labels) != 2 || payload.Labels[1] != "for:coordination" {
		t.Fatalf("labels not trimmed/echoed: %+v", payload.Labels)
	}
}

func TestRepoIssuesResource_RejectsUnknownState(t *testing.T) {
	h := &listRoutingHandler{issuesStatus: http.StatusOK, issuesBody: fakeIssueRows(1)}
	srv := setupListMockServer(t, h)
	defer srv.Close()

	if _, err := repoIssuesResourceHandler(context.Background(), readReq("forgejo://repo/o/r/issues?state=banana")); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if !strings.Contains(h.lastIssuesQuery, "state=open") {
		t.Fatalf("unknown state should fall back to open, query was %q", h.lastIssuesQuery)
	}
}

func TestRepoIssuesResource_TruncatesWithSentinel(t *testing.T) {
	// Ask for 2, hand back 3: over cap by one, which is what Bounded detects.
	h := &listRoutingHandler{issuesStatus: http.StatusOK, issuesBody: fakeIssueRows(3)}
	srv := setupListMockServer(t, h)
	defer srv.Close()

	contents, err := repoIssuesResourceHandler(context.Background(), readReq("forgejo://repo/o/r/issues?limit=2"))
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	var payload issuesListPayload
	readListPayload(t, contents, &payload)
	if !payload.Truncated {
		t.Fatal("expected truncated=true when more rows exist than the limit")
	}
	if len(payload.Issues) != 2 {
		t.Fatalf("expected the payload trimmed to the limit, got %d rows", len(payload.Issues))
	}
	if payload.ListTool != ListRepoIssuesToolName {
		t.Fatalf("expected the fallback tool named, got %q", payload.ListTool)
	}
	if payload.Sentinel == "" {
		t.Fatal("expected a truncation sentinel — silent truncation is the failure this guards")
	}
}

func TestRepoIssuesResource_CapCeiling(t *testing.T) {
	h := &listRoutingHandler{issuesStatus: http.StatusOK, issuesBody: fakeIssueRows(1)}
	srv := setupListMockServer(t, h)
	defer srv.Close()

	uri := fmt.Sprintf("forgejo://repo/o/r/issues?limit=%d", resource.EmbeddedListCap*10)
	if _, err := repoIssuesResourceHandler(context.Background(), readReq(uri)); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	want := fmt.Sprintf("limit=%d", resource.EmbeddedListCap)
	if !strings.Contains(h.lastIssuesQuery, want) {
		t.Fatalf("a caller-supplied limit above the ceiling must clamp (%s), query was %q", want, h.lastIssuesQuery)
	}
}

// TestRepoIssuesResource_PagesCoverEveryRow is the property that matters:
// walking the pages a client can actually ask for must show every row exactly
// once. It replaces the old "limit+1 was requested" assertion, which pinned the
// mechanism rather than the outcome — and pinned it to a mechanism that skipped
// a row at every page boundary.
func TestRepoIssuesResource_PagesCoverEveryRow(t *testing.T) {
	const (
		corpus = 100
		limit  = 30
		pages  = 3
	)
	setupPaginatingServer(t, &paginatingHandler{
		total:   corpus,
		pathHas: "/issues",
		row:     func(i int) map[string]interface{} { return fakeIssueRow(i, fmt.Sprintf("issue %d", i)) },
	})

	var seen []int
	for page := 1; page <= pages; page++ {
		uri := fmt.Sprintf("forgejo://repo/o/r/issues?page=%d&limit=%d", page, limit)
		contents, err := repoIssuesResourceHandler(context.Background(), readReq(uri))
		if err != nil {
			t.Fatalf("page %d: handler error: %v", page, err)
		}
		var payload issuesListPayload
		readListPayload(t, contents, &payload)
		if !payload.Truncated {
			t.Fatalf("page %d of a %d-row corpus must report more remaining", page, corpus)
		}
		for _, row := range payload.Issues {
			seen = append(seen, int(row.Index))
		}
	}
	assertPagesCoverCorpus(t, seen, pages, limit)
}

func TestIssueCommentsResource_PagesCoverEveryRow(t *testing.T) {
	const (
		corpus = 100
		limit  = 30
		pages  = 3
	)
	setupPaginatingServer(t, &paginatingHandler{
		total:   corpus,
		pathHas: "/comments",
		row:     func(i int) map[string]interface{} { return fakeCommentRow(i, fmt.Sprintf("comment %d", i)) },
	})

	var seen []int
	for page := 1; page <= pages; page++ {
		uri := fmt.Sprintf("forgejo://repo/o/r/issue/42/comments?page=%d&limit=%d", page, limit)
		contents, err := issueCommentsResourceHandler(context.Background(), readReq(uri))
		if err != nil {
			t.Fatalf("page %d: handler error: %v", page, err)
		}
		var payload commentsListPayload
		readListPayload(t, contents, &payload)
		if !payload.Truncated {
			t.Fatalf("page %d of a %d-row corpus must report more remaining", page, corpus)
		}
		for _, c := range payload.Comments {
			seen = append(seen, int(c.ID))
		}
	}
	assertPagesCoverCorpus(t, seen, pages, limit)
}

// TestRepoIssuesResource_TruncationComesFromTheHeader pins where "more exists"
// is now read from. The page is full and exactly the size asked for, so nothing
// in the item count says the corpus continues — only the Link header does. The
// sentinel carries the real total from X-Total-Count rather than repeating the
// page size back at the caller.
func TestRepoIssuesResource_TruncationComesFromTheHeader(t *testing.T) {
	setupPaginatingServer(t, &paginatingHandler{
		total:   471,
		pathHas: "/issues",
		row:     func(i int) map[string]interface{} { return fakeIssueRow(i, fmt.Sprintf("issue %d", i)) },
	})

	contents, err := repoIssuesResourceHandler(context.Background(), readReq("forgejo://repo/o/r/issues?limit=30"))
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	var payload issuesListPayload
	readListPayload(t, contents, &payload)

	if len(payload.Issues) != 30 {
		t.Fatalf("expected exactly the limit in rows, got %d", len(payload.Issues))
	}
	if !payload.Truncated {
		t.Fatal("a full page with a rel=next link must report truncation")
	}
	if payload.ListTool != ListRepoIssuesToolName {
		t.Fatalf("expected the fallback tool named, got %q", payload.ListTool)
	}
	if !strings.Contains(payload.Sentinel, "30 of 471") {
		t.Fatalf("sentinel should carry the server's total, got %q", payload.Sentinel)
	}
}

// TestRepoIssuesResource_LastPageIsNotTruncated is the other half: the final
// page has no rel=next link, so it must not claim more remains.
func TestRepoIssuesResource_LastPageIsNotTruncated(t *testing.T) {
	setupPaginatingServer(t, &paginatingHandler{
		total:   50,
		pathHas: "/issues",
		row:     func(i int) map[string]interface{} { return fakeIssueRow(i, fmt.Sprintf("issue %d", i)) },
	})

	contents, err := repoIssuesResourceHandler(context.Background(), readReq("forgejo://repo/o/r/issues?page=2&limit=30"))
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	var payload issuesListPayload
	readListPayload(t, contents, &payload)

	if len(payload.Issues) != 20 {
		t.Fatalf("expected the 20-row tail of a 50-row corpus, got %d", len(payload.Issues))
	}
	if payload.Truncated {
		t.Fatal("the last page must not report more remaining")
	}
	if payload.Sentinel != "" {
		t.Fatalf("no sentinel on the last page, got %q", payload.Sentinel)
	}
}

// TestRepoIssuesResource_TruncationWithoutTotalHeader covers an instance that
// sends rel=next but no X-Total-Count: truncation is still reported, and the
// sentinel says so without inventing a total.
func TestRepoIssuesResource_TruncationWithoutTotalHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Link", `<`+r.URL.Path+`?page=2&limit=2>; rel="next"`)
		_ = json.NewEncoder(w).Encode(fakeIssueRows(2))
	}))
	defer srv.Close()
	client, err := forgejo_sdk.NewClient(srv.URL, forgejo_sdk.SetForgejoVersion("7.0.0"))
	if err != nil {
		t.Fatalf("creating test client: %v", err)
	}
	forgejo.SetClientForTesting(client)

	contents, err := repoIssuesResourceHandler(context.Background(), readReq("forgejo://repo/o/r/issues?limit=2"))
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	var payload issuesListPayload
	readListPayload(t, contents, &payload)
	if !payload.Truncated {
		t.Fatal("rel=next alone must be enough to report truncation")
	}
	if strings.Contains(payload.Sentinel, " of ") {
		t.Fatalf("sentinel must not claim a total the server never sent, got %q", payload.Sentinel)
	}
	if !strings.Contains(payload.Sentinel, ListRepoIssuesToolName) {
		t.Fatalf("sentinel should still name the fallback tool, got %q", payload.Sentinel)
	}
}

func TestRepoIssuesResource_BadURI(t *testing.T) {
	if _, err := repoIssuesResourceHandler(context.Background(), readReq("forgejo://repo/o/r/issue")); err == nil {
		t.Fatal("expected an error for a URI that is not the issues list")
	}
}

func TestRepoIssuesResource_APIErrorIsMapped(t *testing.T) {
	h := &listRoutingHandler{issuesStatus: http.StatusForbidden, issuesBody: map[string]string{"message": "nope"}}
	srv := setupListMockServer(t, h)
	defer srv.Close()

	if _, err := repoIssuesResourceHandler(context.Background(), readReq("forgejo://repo/o/r/issues")); err == nil {
		t.Fatal("expected the 403 to surface as an error")
	}
}

// ---- comment thread ----

func TestIssueCommentsResource_FullBodies(t *testing.T) {
	long := strings.Repeat("the full comment body must survive. ", 30)
	h := &listRoutingHandler{
		commentsStatus: http.StatusOK,
		commentsBody:   []interface{}{fakeCommentRow(1, long), fakeCommentRow(2, "short")},
	}
	srv := setupListMockServer(t, h)
	defer srv.Close()

	contents, err := issueCommentsResourceHandler(context.Background(), readReq("forgejo://repo/o/r/issue/42/comments"))
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	var payload commentsListPayload
	readListPayload(t, contents, &payload)
	if len(payload.Comments) != 2 {
		t.Fatalf("expected 2 comments, got %d", len(payload.Comments))
	}
	// This is the whole difference from the single-issue resource, which excerpts
	// comment bodies at 200 characters.
	if payload.Comments[0].Body != long {
		t.Fatalf("comment body was truncated: %d chars kept of %d", len(payload.Comments[0].Body), len(long))
	}
	if payload.Comments[0].Author != "carol" {
		t.Fatalf("author not flattened: %q", payload.Comments[0].Author)
	}
	if payload.Kind != "issue" || payload.Index != 42 {
		t.Fatalf("kind/index not echoed: %+v", payload)
	}
}

func TestIssueCommentsResource_PRKindAccepted(t *testing.T) {
	h := &listRoutingHandler{commentsStatus: http.StatusOK, commentsBody: []interface{}{fakeCommentRow(1, "on a PR")}}
	srv := setupListMockServer(t, h)
	defer srv.Close()

	contents, err := issueCommentsResourceHandler(context.Background(), readReq("forgejo://repo/o/r/pr/7/comments"))
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	var payload commentsListPayload
	readListPayload(t, contents, &payload)
	if payload.Kind != "pr" || payload.Index != 7 {
		t.Fatalf("pr kind not carried: %+v", payload)
	}
}

func TestIssueCommentsResource_TruncatesWithSentinel(t *testing.T) {
	h := &listRoutingHandler{
		commentsStatus: http.StatusOK,
		commentsBody: []interface{}{
			fakeCommentRow(1, "one"), fakeCommentRow(2, "two"), fakeCommentRow(3, "three"),
		},
	}
	srv := setupListMockServer(t, h)
	defer srv.Close()

	contents, err := issueCommentsResourceHandler(context.Background(), readReq("forgejo://repo/o/r/issue/42/comments?limit=2"))
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	var payload commentsListPayload
	readListPayload(t, contents, &payload)
	if !payload.Truncated || len(payload.Comments) != 2 {
		t.Fatalf("expected 2 comments and truncated=true, got %d / %v", len(payload.Comments), payload.Truncated)
	}
	if payload.ListTool != ListIssueCommentsToolName || payload.Sentinel == "" {
		t.Fatalf("expected the fallback tool and sentinel, got %q / %q", payload.ListTool, payload.Sentinel)
	}
}

func TestIssueCommentsResource_PageForwarded(t *testing.T) {
	h := &listRoutingHandler{commentsStatus: http.StatusOK, commentsBody: []interface{}{fakeCommentRow(1, "x")}}
	srv := setupListMockServer(t, h)
	defer srv.Close()

	if _, err := issueCommentsResourceHandler(context.Background(), readReq("forgejo://repo/o/r/issue/42/comments?page=3&limit=4")); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if !strings.Contains(h.lastCommentsQuery, "page=3") || !strings.Contains(h.lastCommentsQuery, "limit=4") {
		t.Fatalf("page and the caller's limit verbatim not forwarded: %q", h.lastCommentsQuery)
	}
}

// setupBoundsIgnoringServer reproduces how Forgejo's issue-comments endpoint
// actually behaves: it returns the entire thread whatever `page` and `limit`
// say, and sends X-Total-Count but no Link header. Verified live against a
// 69-comment thread — `?limit=5` returned all 69, and `?page=2` returned the
// same rows as page 1.
func setupBoundsIgnoringServer(t *testing.T, total int) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		rows := make([]interface{}, 0, total)
		for i := 1; i <= total; i++ {
			rows = append(rows, fakeCommentRow(i, fmt.Sprintf("comment %d", i)))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Total-Count", strconv.Itoa(total))
		_ = json.NewEncoder(w).Encode(rows)
	}))
	t.Cleanup(srv.Close)
	client, err := forgejo_sdk.NewClient(srv.URL, forgejo_sdk.SetForgejoVersion("7.0.0"))
	if err != nil {
		t.Fatalf("creating test client: %v", err)
	}
	forgejo.SetClientForTesting(client)
}

// TestIssueCommentsResource_PagesWhenServerIgnoresBounds covers the endpoint as
// it really is. Without client-side windowing the resource advertises `page`,
// echoes it back, and silently serves page 1's rows for every page — which is
// worse than refusing, because the payload looks right.
func TestIssueCommentsResource_PagesWhenServerIgnoresBounds(t *testing.T) {
	const (
		corpus = 69
		limit  = 30
	)
	setupBoundsIgnoringServer(t, corpus)

	var seen []int
	for page := 1; page <= 3; page++ {
		uri := fmt.Sprintf("forgejo://repo/o/r/issue/42/comments?page=%d&limit=%d", page, limit)
		contents, err := issueCommentsResourceHandler(context.Background(), readReq(uri))
		if err != nil {
			t.Fatalf("page %d: handler error: %v", page, err)
		}
		var payload commentsListPayload
		readListPayload(t, contents, &payload)

		wantRows := limit
		wantTruncated := true
		if page == 3 {
			wantRows = corpus - 2*limit // 9
			wantTruncated = false
		}
		if len(payload.Comments) != wantRows {
			t.Fatalf("page %d: expected %d comments, got %d", page, wantRows, len(payload.Comments))
		}
		if payload.Truncated != wantTruncated {
			t.Fatalf("page %d: truncated=%v, want %v", page, payload.Truncated, wantTruncated)
		}
		if wantTruncated && !strings.Contains(payload.Sentinel, fmt.Sprintf("of %d", corpus)) {
			t.Fatalf("page %d: sentinel should carry the true total %d, got %q", page, corpus, payload.Sentinel)
		}
		for _, c := range payload.Comments {
			seen = append(seen, int(c.ID))
		}
	}

	if len(seen) != corpus {
		t.Fatalf("expected the whole %d-comment thread across 3 pages, got %d: %v", corpus, len(seen), seen)
	}
	for i, got := range seen {
		if got != i+1 {
			t.Fatalf("the thread repeated or skipped a comment at position %d: want %d, got %d\nfull sequence: %v",
				i, i+1, got, seen)
		}
	}
}

// TestIssueCommentsResource_PastTheEndIsEmpty guards the clamp: a page beyond
// the thread must be empty and untruncated, not a panic and not a wrapped slice.
func TestIssueCommentsResource_PastTheEndIsEmpty(t *testing.T) {
	setupBoundsIgnoringServer(t, 69)

	contents, err := issueCommentsResourceHandler(context.Background(),
		readReq("forgejo://repo/o/r/issue/42/comments?page=9&limit=30"))
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	var payload commentsListPayload
	readListPayload(t, contents, &payload)
	if len(payload.Comments) != 0 {
		t.Fatalf("expected no comments past the end of the thread, got %d", len(payload.Comments))
	}
	if payload.Truncated {
		t.Fatal("a page past the end must not claim more remains")
	}
}

// TestIssueCommentsResource_ServerHonouringBoundsIsNotSlicedTwice is the
// forward-compatibility half: if Forgejo starts honouring page/limit, the rows
// already are the requested page and re-applying the offset here would return
// nothing for page 2.
func TestIssueCommentsResource_ServerHonouringBoundsIsNotSlicedTwice(t *testing.T) {
	setupPaginatingServer(t, &paginatingHandler{
		total:   69,
		pathHas: "/comments",
		row:     func(i int) map[string]interface{} { return fakeCommentRow(i, fmt.Sprintf("comment %d", i)) },
	})

	contents, err := issueCommentsResourceHandler(context.Background(),
		readReq("forgejo://repo/o/r/issue/42/comments?page=2&limit=30"))
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	var payload commentsListPayload
	readListPayload(t, contents, &payload)
	if len(payload.Comments) != 30 {
		t.Fatalf("expected the server's own page 2 of 30 rows, got %d", len(payload.Comments))
	}
	if payload.Comments[0].ID != 31 {
		t.Fatalf("server-paged rows were sliced a second time: page 2 starts at %d, want 31", payload.Comments[0].ID)
	}
}

// TestIssueCommentsResource_ThreadExactlyLimitLongPagesToEmpty is the case a
// row-count-only guard cannot see. A bounds-ignoring server returns the whole
// thread, and when the thread is exactly `limit` long that is indistinguishable
// by count from a full page served by a server that paged properly — so a guard
// of "more rows than asked for" declines to apply the offset and hands page 1's
// rows back for page 2. X-Total-Count is what separates the two.
func TestIssueCommentsResource_ThreadExactlyLimitLongPagesToEmpty(t *testing.T) {
	const limit = resource.EmbeddedListCap // 30, the default limit
	setupBoundsIgnoringServer(t, limit)

	first, err := issueCommentsResourceHandler(context.Background(),
		readReq(fmt.Sprintf("forgejo://repo/o/r/issue/42/comments?page=1&limit=%d", limit)))
	if err != nil {
		t.Fatalf("page 1: handler error: %v", err)
	}
	var page1 commentsListPayload
	readListPayload(t, first, &page1)
	if len(page1.Comments) != limit {
		t.Fatalf("page 1 should carry the whole %d-comment thread, got %d", limit, len(page1.Comments))
	}
	if page1.Truncated {
		t.Fatal("a thread of exactly the limit fits on page 1 — nothing remains")
	}

	second, err := issueCommentsResourceHandler(context.Background(),
		readReq(fmt.Sprintf("forgejo://repo/o/r/issue/42/comments?page=2&limit=%d", limit)))
	if err != nil {
		t.Fatalf("page 2: handler error: %v", err)
	}
	var page2 commentsListPayload
	readListPayload(t, second, &page2)
	if len(page2.Comments) != 0 {
		t.Fatalf("page 2 of a %d-comment thread must be empty, got %d comments starting at id %d",
			limit, len(page2.Comments), page2.Comments[0].ID)
	}
	if page2.Truncated {
		t.Fatal("an empty page past the end must not claim more remains")
	}
}

// TestIssueCommentsResource_ShortThreadPagesToEmpty is the same defect one step
// further from the boundary: a thread shorter than the limit was also never
// offset, so page 2 repeated page 1.
func TestIssueCommentsResource_ShortThreadPagesToEmpty(t *testing.T) {
	setupBoundsIgnoringServer(t, 3)

	contents, err := issueCommentsResourceHandler(context.Background(),
		readReq("forgejo://repo/o/r/issue/42/comments?page=2&limit=30"))
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	var payload commentsListPayload
	readListPayload(t, contents, &payload)
	if len(payload.Comments) != 0 {
		t.Fatalf("page 2 of a 3-comment thread must be empty, got %d", len(payload.Comments))
	}
}

// TestIssueCommentsResource_HonouringServerFullPageIsNotSlicedTwice is the
// other direction, and the reason the guard is not simply ">=": a server that
// pages properly returns exactly `limit` rows for every full page. Those rows
// already are the requested page, and re-applying the offset would empty it.
// Here the last page is full too (60 rows, limit 30), so no page in the corpus
// can be told apart from a whole thread by row count alone.
func TestIssueCommentsResource_HonouringServerFullPageIsNotSlicedTwice(t *testing.T) {
	const (
		corpus = 60
		limit  = 30
	)
	setupPaginatingServer(t, &paginatingHandler{
		total:   corpus,
		pathHas: "/comments",
		row:     func(i int) map[string]interface{} { return fakeCommentRow(i, fmt.Sprintf("comment %d", i)) },
	})

	var seen []int
	for page := 1; page <= 2; page++ {
		uri := fmt.Sprintf("forgejo://repo/o/r/issue/42/comments?page=%d&limit=%d", page, limit)
		contents, err := issueCommentsResourceHandler(context.Background(), readReq(uri))
		if err != nil {
			t.Fatalf("page %d: handler error: %v", page, err)
		}
		var payload commentsListPayload
		readListPayload(t, contents, &payload)
		if len(payload.Comments) != limit {
			t.Fatalf("page %d: expected the server's own full page of %d rows, got %d",
				page, limit, len(payload.Comments))
		}
		for _, c := range payload.Comments {
			seen = append(seen, int(c.ID))
		}
	}
	assertPagesCoverCorpus(t, seen, 2, limit)
}

func TestIssueCommentsResource_BadKind(t *testing.T) {
	if _, err := issueCommentsResourceHandler(context.Background(), readReq("forgejo://repo/o/r/wiki/42/comments")); err == nil {
		t.Fatal("expected an error for a kind that is neither issue nor pr")
	}
}

func TestIssueCommentsResource_NonNumericIndex(t *testing.T) {
	if _, err := issueCommentsResourceHandler(context.Background(), readReq("forgejo://repo/o/r/issue/abc/comments")); err == nil {
		t.Fatal("expected an error for a non-numeric index")
	}
}

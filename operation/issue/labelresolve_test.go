// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/flag"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forgejo"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// catalogLabel is one row of the label catalogue the resolver enumerates.
type catalogLabel struct {
	ID   int64
	Name string
}

// resolveBackendOptions configures the fake forge used by resolver tests.
type resolveBackendOptions struct {
	repoLabels []catalogLabel
	orgLabels  []catalogLabel
	// orgStatus overrides the organization label response code. Zero serves
	// orgLabels normally; 404 is the user-owned-repository case that
	// fetchOrgLabels maps to an empty list.
	orgStatus int
	// maxPageSize clamps the requested limit the way Forgejo's
	// MAX_RESPONSE_ITEMS (default 50) does. Zero means no server-side cap.
	maxPageSize int
	// pagingHeaders makes the catalogue reads carry X-Total-Count and
	// Link rel="next", as Forgejo does. Off, a page is just a bare array.
	pagingHeaders bool
	// next handles everything that is not a catalogue read.
	next func(w http.ResponseWriter, r *http.Request, records *[]recordedReq)
}

// newResolveBackend serves the two catalogue reads the resolver makes — repo
// labels and org labels — and hands every other request to opts.next.
//
// Catalogue reads are counted rather than recorded, the same way the version
// and settings probes are excluded in newQueryBackendWithSettings: they are
// resolver plumbing, so *records stays the sequence of requests a test is
// actually asserting on, and *catalogReads proves whether the catalogue was
// consulted at all.
//
// Both endpoints honour page and limit, so a test can place a label on the
// second page and prove enumeration does not stop at the first.
func newResolveBackend(t *testing.T, opts resolveBackendOptions) (records *[]recordedReq, catalogReads *int) {
	t.Helper()
	recs := make([]recordedReq, 0, 4)
	reads := 0

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/version", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":"11.0.0+gitea-1.22.0"}`))
	})
	mux.HandleFunc("/api/v1/settings/api", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"not found"}`))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")

		switch {
		case isRepoLabelCatalogRequest(r):
			reads++
			writeLabelPage(w, opts.repoLabels, r, opts)
			return
		case isOrgLabelCatalogRequest(r):
			reads++
			if opts.orgStatus != 0 {
				w.WriteHeader(opts.orgStatus)
				_, _ = w.Write([]byte(`{"message":"no"}`))
				return
			}
			writeLabelPage(w, opts.orgLabels, r, opts)
			return
		}

		recs = append(recs, recordedReq{method: r.Method, path: r.URL.Path, query: r.URL.RawQuery, rawBody: body})
		if opts.next != nil {
			opts.next(w, r, &recs)
			return
		}
		_, _ = w.Write([]byte(`{"id":1,"number":1}`))
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
	return &recs, &reads
}

// isRepoLabelCatalogRequest matches /repos/{owner}/{repo}/labels but not
// /repos/{owner}/{repo}/issues/{index}/labels — both end in /labels, and the
// second is the request under test in an assignment test.
func isRepoLabelCatalogRequest(r *http.Request) bool {
	return r.Method == http.MethodGet &&
		strings.HasPrefix(r.URL.Path, "/api/v1/repos/") &&
		strings.HasSuffix(r.URL.Path, "/labels") &&
		!strings.Contains(r.URL.Path, "/issues/")
}

func isOrgLabelCatalogRequest(r *http.Request) bool {
	return r.Method == http.MethodGet &&
		strings.HasPrefix(r.URL.Path, "/api/v1/orgs/") &&
		strings.HasSuffix(r.URL.Path, "/labels")
}

// writeLabelPage serves the slice of labels the requested page/limit selects,
// so enumeration terminates on a short page exactly as it would upstream.
func writeLabelPage(w http.ResponseWriter, labels []catalogLabel, r *http.Request, opts resolveBackendOptions) {
	page, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil || page < 1 {
		page = 1
	}
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 {
		limit = labelCatalogPageSize
	}
	if opts.maxPageSize > 0 && limit > opts.maxPageSize {
		limit = opts.maxPageSize
	}

	start := (page - 1) * limit
	if start > len(labels) {
		start = len(labels)
	}
	end := start + limit
	if end > len(labels) {
		end = len(labels)
	}

	if opts.pagingHeaders {
		w.Header().Set("X-Total-Count", strconv.Itoa(len(labels)))
		if end < len(labels) {
			w.Header().Set("Link", fmt.Sprintf(`<%s?page=%d&limit=%d>; rel="next"`, r.URL.Path, page+1, limit))
		}
	}

	parts := make([]string, 0, end-start)
	for _, l := range labels[start:end] {
		parts = append(parts, fmt.Sprintf(`{"id":%d,"name":%q,"color":"ffffff"}`, l.ID, l.Name))
	}
	_, _ = w.Write([]byte("[" + strings.Join(parts, ",") + "]"))
}

// labelsFromBody reads the "labels" array out of a recorded request body and
// insists the entries are numbers, which is the point of resolving names: the
// wire must carry IDs even when the caller typed names.
func labelsFromBody(t *testing.T, body []byte) []int64 {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("invalid JSON body: %v\nbody: %s", err, body)
	}
	raw, ok := payload["labels"]
	if !ok || raw == nil {
		t.Fatalf("no labels field in body: %s", body)
	}
	entries, ok := raw.([]any)
	if !ok {
		t.Fatalf("labels is %T, expected an array: %s", raw, body)
	}
	ids := make([]int64, 0, len(entries))
	for _, e := range entries {
		n, ok := e.(float64)
		if !ok {
			t.Fatalf("label entry %v is %T, expected a numeric ID: %s", e, e, body)
		}
		ids = append(ids, int64(n))
	}
	return ids
}

func equalInt64s(got, want []int64) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func TestResolveIssueLabelIDs_RepoName(t *testing.T) {
	newResolveBackend(t, resolveBackendOptions{
		repoLabels: []catalogLabel{{ID: 5, Name: "bug"}, {ID: 6, Name: "triage"}},
		orgStatus:  http.StatusNotFound,
	})

	ids, err := resolveIssueLabelIDs(context.Background(), "goern", "forgejo-mcp", "bug, triage")
	if err != nil {
		t.Fatalf("resolveIssueLabelIDs err: %v", err)
	}
	if len(ids) != 2 || ids[0] != 5 || ids[1] != 6 {
		t.Fatalf("expected [5 6], got %v", ids)
	}
}

func TestResolveIssueLabelIDs_OrgName(t *testing.T) {
	newResolveBackend(t, resolveBackendOptions{
		repoLabels: []catalogLabel{{ID: 5, Name: "bug"}},
		orgLabels:  []catalogLabel{{ID: 91, Name: "dogfood"}},
	})

	ids, err := resolveIssueLabelIDs(context.Background(), "agentic-forges", "forgejo-mcp", "dogfood")
	if err != nil {
		t.Fatalf("resolveIssueLabelIDs err: %v", err)
	}
	if len(ids) != 1 || ids[0] != 91 {
		t.Fatalf("expected [91], got %v", ids)
	}
}

func TestResolveIssueLabelIDs_NumericTokenStillWorks(t *testing.T) {
	newResolveBackend(t, resolveBackendOptions{
		repoLabels: []catalogLabel{{ID: 5, Name: "bug"}, {ID: 6, Name: "triage"}},
		orgStatus:  http.StatusNotFound,
	})

	ids, err := resolveIssueLabelIDs(context.Background(), "goern", "forgejo-mcp", "5,6")
	if err != nil {
		t.Fatalf("resolveIssueLabelIDs err: %v", err)
	}
	if len(ids) != 2 || ids[0] != 5 || ids[1] != 6 {
		t.Fatalf("expected [5 6], got %v", ids)
	}
}

func TestResolveIssueLabelIDs_NameBeatsID(t *testing.T) {
	// A label literally named "123" must win over the label whose ID is 123,
	// otherwise it is unreachable through an argument documented to take names.
	newResolveBackend(t, resolveBackendOptions{
		repoLabels: []catalogLabel{{ID: 7, Name: "123"}, {ID: 123, Name: "other"}},
		orgStatus:  http.StatusNotFound,
	})

	ids, err := resolveIssueLabelIDs(context.Background(), "goern", "forgejo-mcp", "123")
	if err != nil {
		t.Fatalf("resolveIssueLabelIDs err: %v", err)
	}
	if len(ids) != 1 || ids[0] != 7 {
		t.Fatalf("expected the label named \"123\" (id 7), got %v", ids)
	}
}

func TestResolveIssueLabelIDs_SecondPage(t *testing.T) {
	// A full first page plus one more row: the label under test only appears
	// once enumeration asks for page two.
	repoLabels := make([]catalogLabel, 0, labelCatalogPageSize+1)
	for i := range labelCatalogPageSize {
		repoLabels = append(repoLabels, catalogLabel{ID: int64(i + 1), Name: fmt.Sprintf("filler-%d", i)})
	}
	repoLabels = append(repoLabels, catalogLabel{ID: 999, Name: "late"})

	newResolveBackend(t, resolveBackendOptions{repoLabels: repoLabels, orgStatus: http.StatusNotFound})

	ids, err := resolveIssueLabelIDs(context.Background(), "goern", "forgejo-mcp", "late")
	if err != nil {
		t.Fatalf("resolveIssueLabelIDs err: %v", err)
	}
	if len(ids) != 1 || ids[0] != 999 {
		t.Fatalf("expected [999] from the second page, got %v", ids)
	}
}

func TestResolveIssueLabelIDs_DuplicatesCollapse(t *testing.T) {
	newResolveBackend(t, resolveBackendOptions{
		repoLabels: []catalogLabel{{ID: 7, Name: "bug"}},
		orgStatus:  http.StatusNotFound,
	})

	ids, err := resolveIssueLabelIDs(context.Background(), "goern", "forgejo-mcp", "bug,bug,7")
	if err != nil {
		t.Fatalf("resolveIssueLabelIDs err: %v", err)
	}
	if len(ids) != 1 || ids[0] != 7 {
		t.Fatalf("expected [7], got %v", ids)
	}
}

func TestResolveIssueLabelIDs_UnknownName(t *testing.T) {
	newResolveBackend(t, resolveBackendOptions{
		repoLabels: []catalogLabel{{ID: 5, Name: "bug"}},
		orgStatus:  http.StatusNotFound,
	})

	_, err := resolveIssueLabelIDs(context.Background(), "goern", "forgejo-mcp", "bug,frction")
	if err == nil {
		t.Fatal("expected an error for an unknown label name")
	}
	if !strings.Contains(err.Error(), "frction") {
		t.Fatalf("error should name the unresolved token, got: %v", err)
	}
}

func TestResolveIssueLabelIDs_UnknownNumericID(t *testing.T) {
	// A numeric token is honoured only when the catalogue has that ID, so a
	// stale ID fails here rather than 404-ing mid-write.
	newResolveBackend(t, resolveBackendOptions{
		repoLabels: []catalogLabel{{ID: 5, Name: "bug"}},
		orgStatus:  http.StatusNotFound,
	})

	_, err := resolveIssueLabelIDs(context.Background(), "goern", "forgejo-mcp", "4242")
	if err == nil {
		t.Fatal("expected an error for an ID that is not in the catalogue")
	}
	if !strings.Contains(err.Error(), "4242") {
		t.Fatalf("error should name the unresolved token, got: %v", err)
	}
}

func TestResolveIssueLabelIDs_AmbiguousAcrossScopes(t *testing.T) {
	newResolveBackend(t, resolveBackendOptions{
		repoLabels: []catalogLabel{{ID: 7, Name: "bug"}},
		orgLabels:  []catalogLabel{{ID: 12, Name: "bug"}},
	})

	_, err := resolveIssueLabelIDs(context.Background(), "agentic-forges", "forgejo-mcp", "bug")
	if err == nil {
		t.Fatal("expected an error when a name matches in two scopes")
	}
	msg := err.Error()
	if !strings.Contains(msg, "repository label 7") || !strings.Contains(msg, "organization label 12") {
		t.Fatalf("error should report both scopes and IDs, got: %v", err)
	}
}

func TestResolveIssueLabelIDs_EmptyIsAnError(t *testing.T) {
	_, catalogReads := newResolveBackend(t, resolveBackendOptions{
		repoLabels: []catalogLabel{{ID: 5, Name: "bug"}},
		orgStatus:  http.StatusNotFound,
	})

	if _, err := resolveIssueLabelIDs(context.Background(), "goern", "forgejo-mcp", "  ,  "); err == nil {
		t.Fatal("expected an error when the argument names no label")
	}
	if *catalogReads != 0 {
		t.Fatalf("expected no catalogue read before rejecting an empty argument, got %d", *catalogReads)
	}
}

func TestResolveIssueLabelIDs_OrgLabelsUnauthorized(t *testing.T) {
	// 403 on the org endpoint would narrow the catalogue silently, turning a
	// real org label into "unknown label". It has to fail loudly instead.
	newResolveBackend(t, resolveBackendOptions{
		repoLabels: []catalogLabel{{ID: 5, Name: "bug"}},
		orgStatus:  http.StatusForbidden,
	})

	if _, err := resolveIssueLabelIDs(context.Background(), "agentic-forges", "forgejo-mcp", "bug"); err == nil {
		t.Fatal("expected an error when organization labels cannot be read")
	}
}

// numberedLabels returns n filler labels with IDs 1..n, then target as ID
// n+1, so the target sits past any first page the server will serve.
func numberedLabels(n int, target string) []catalogLabel {
	out := make([]catalogLabel, 0, n+1)
	for i := range n {
		out = append(out, catalogLabel{ID: int64(i + 1), Name: fmt.Sprintf("filler-%d", i)})
	}
	return append(out, catalogLabel{ID: int64(n + 1), Name: target})
}

// TestResolveIssueLabelIDs_ServerPageCap pins the Forgejo behaviour behind
// forgejo-mcp-grzt: the server clamps limit to MAX_RESPONSE_ITEMS (50), so a
// 50-row page is not a short page and must not end enumeration. Both header
// modes are covered: Forgejo sends X-Total-Count/Link, but a proxy may strip
// them, and enumeration must still reach the end.
func TestResolveIssueLabelIDs_ServerPageCap(t *testing.T) {
	for _, headers := range []bool{true, false} {
		for _, scope := range []string{"repo", "org"} {
			for _, by := range []string{"name", "id"} {
				name := fmt.Sprintf("%s/%s/headers=%t", scope, by, headers)
				t.Run(name, func(t *testing.T) {
					const total = 120
					labels := numberedLabels(total, "merge")
					opts := resolveBackendOptions{maxPageSize: 50, pagingHeaders: headers}
					if scope == "repo" {
						opts.repoLabels = labels
						opts.orgStatus = http.StatusNotFound
					} else {
						opts.repoLabels = []catalogLabel{{ID: 5000, Name: "bug"}}
						opts.orgLabels = labels
					}
					newResolveBackend(t, opts)

					token := "merge"
					if by == "id" {
						token = strconv.Itoa(total + 1)
					}
					ids, err := resolveIssueLabelIDs(context.Background(), "agentic-forges", "forgejo-mcp", token)
					if err != nil {
						t.Fatalf("resolveIssueLabelIDs(%q) err: %v", token, err)
					}
					if len(ids) != 1 || ids[0] != total+1 {
						t.Fatalf("expected [%d], got %v", total+1, ids)
					}
				})
			}
		}
	}
}

// serveLabelsByID answers the single-label reads the resolver falls back to
// once the catalogue is too large to enumerate. status, when set, replaces every
// answer.
func serveLabelsByID(repoIDs, orgIDs []int64, status int) func(http.ResponseWriter, *http.Request, *[]recordedReq) {
	return func(w http.ResponseWriter, r *http.Request, _ *[]recordedReq) {
		if status != 0 {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"message":"no"}`))
			return
		}
		known := repoIDs
		if strings.HasPrefix(r.URL.Path, "/api/v1/orgs/") {
			known = orgIDs
		}
		id, err := strconv.ParseInt(r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:], 10, 64)
		if err == nil && slices.Contains(known, id) {
			_, _ = fmt.Fprintf(w, `{"id":%d,"name":"found","color":"ffffff"}`, id)
			return
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"not found"}`))
	}
}

// TestResolveIssueLabelIDs_PastCatalogueCap pins the promise in the cap's own
// error message: with too many labels to enumerate, "pass numeric label IDs
// instead of names" must actually work. Refs #635.
func TestResolveIssueLabelIDs_PastCatalogueCap(t *testing.T) {
	const serverCap = 50
	tooMany := numberedLabels(maxLabelCatalogPages*serverCap+10, "merge")
	const repoLabel, orgLabel, staleID = 7, 90001, 424242

	tests := []struct {
		name      string
		overOrg   bool
		status    int
		token     string
		want      []int64
		wantErr   string
		wantReads []string
	}{
		{
			name: "repo id", token: "7", want: []int64{repoLabel},
			wantReads: []string{"/api/v1/repos/agentic-forges/forgejo-mcp/labels/7"},
		},
		{
			name: "org id", token: "90001", want: []int64{orgLabel},
			wantReads: []string{
				"/api/v1/repos/agentic-forges/forgejo-mcp/labels/90001",
				"/api/v1/orgs/agentic-forges/labels/90001",
			},
		},
		{
			name: "org catalogue over the cap", overOrg: true, token: "90001", want: []int64{orgLabel},
			wantReads: []string{
				"/api/v1/repos/agentic-forges/forgejo-mcp/labels/90001",
				"/api/v1/orgs/agentic-forges/labels/90001",
			},
		},
		{
			name: "duplicate ids collapse", token: "7,7", want: []int64{repoLabel},
			wantReads: []string{
				"/api/v1/repos/agentic-forges/forgejo-mcp/labels/7",
				"/api/v1/repos/agentic-forges/forgejo-mcp/labels/7",
			},
		},
		{
			name: "stale id", token: strconv.Itoa(staleID), wantErr: "neither a label name nor the ID",
			wantReads: []string{
				"/api/v1/repos/agentic-forges/forgejo-mcp/labels/424242",
				"/api/v1/orgs/agentic-forges/labels/424242",
			},
		},
		{name: "name", token: "merge", wantErr: "pass numeric label IDs"},
		{
			name: "id then name", token: "7,merge", wantErr: "pass numeric label IDs",
			wantReads: []string{"/api/v1/repos/agentic-forges/forgejo-mcp/labels/7"},
		},
		{
			name: "forbidden is not unknown", status: http.StatusForbidden, token: "7", wantErr: "get repo label 7",
			wantReads: []string{"/api/v1/repos/agentic-forges/forgejo-mcp/labels/7"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := resolveBackendOptions{
				maxPageSize:   serverCap,
				pagingHeaders: true,
				repoLabels:    tooMany,
				next:          serveLabelsByID([]int64{repoLabel}, []int64{orgLabel}, tt.status),
			}
			if tt.overOrg {
				opts.repoLabels = []catalogLabel{{ID: repoLabel, Name: "bug"}}
				opts.orgLabels = tooMany
			}
			records, _ := newResolveBackend(t, opts)

			ids, err := resolveIssueLabelIDs(context.Background(), "agentic-forges", "forgejo-mcp", tt.token)

			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected an error containing %q, got ids=%v err=%v", tt.wantErr, ids, err)
				}
			} else {
				if err != nil {
					t.Fatalf("resolveIssueLabelIDs(%q) err: %v", tt.token, err)
				}
				if !equalInt64s(ids, tt.want) {
					t.Fatalf("expected %v, got %v", tt.want, ids)
				}
			}

			got := make([]string, 0, len(*records))
			for _, rec := range *records {
				got = append(got, rec.path)
			}
			if !slices.Equal(got, tt.wantReads) {
				t.Fatalf("single-label reads:\n got %v\nwant %v", got, tt.wantReads)
			}
		})
	}
}

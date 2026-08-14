// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v2/pkg/flag"
)

func TestAPIPath_EscapesSegments(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{
			name: "plain segments are untouched",
			got:  APIPath("repos", "goern", "forgejo-mcp", "issues", int64(7), "assets"),
			want: "/repos/goern/forgejo-mcp/issues/7/assets",
		},
		{
			name: "slashes cannot break out of their segment",
			got:  APIPath("repos", "o/x/../../admin", "r", "issues"),
			want: "/repos/o%2Fx%2F..%2F..%2Fadmin/r/issues",
		},
		{
			name: "a question mark cannot start a query string",
			got:  APIPath("repos", "o", "r?state=all", "issues"),
			want: "/repos/o/r%3Fstate=all/issues",
		},
		{
			name: "int and int64 ids format naturally",
			got:  APIPath("orgs", "acme", "labels", 12) + APIPath("x", int64(34)),
			want: "/orgs/acme/labels/12/x/34",
		},
		{
			name: "spaces and unicode are escaped",
			got:  APIPath("repos", "o", "my repo", "wiki", "Seite Ü"),
			want: "/repos/o/my%20repo/wiki/Seite%20%C3%9C",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Fatalf("APIPath = %q, want %q", tt.got, tt.want)
			}
		})
	}
}

// TestAPIPath_HostileOwnerStaysOnEndpoint is the regression test for the
// raw-HTTP path-injection defect: an owner containing a traversal and a repo
// containing "?" must not move the request off /repos/{owner}/{repo}/issues.
func TestAPIPath_HostileOwnerStaysOnEndpoint(t *testing.T) {
	var requestURI string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestURI = r.RequestURI
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	t.Cleanup(srv.Close)
	flag.URL = srv.URL
	flag.Token = "test-token"
	flag.UserAgent = "forgejo-mcp-test/0.0.1"

	const owner = "o/x/../../admin"
	const repo = "r?state=all&"
	path := APIPath("repos", owner, repo, "issues") + fmt.Sprintf("?page=%d&limit=%d", 1, 20)

	var out []any
	if err := DoJSONList(context.Background(), http.MethodGet, path, &out); err != nil {
		t.Fatalf("request failed: %v", err)
	}

	// The traversal must arrive escaped, not collapsed by the URL parser.
	if !strings.Contains(requestURI, "%2F") {
		t.Fatalf("owner slashes were not escaped, got RequestURI %q", requestURI)
	}
	// The literal "?" in repo must not have terminated the path and pushed
	// "/issues" out of it.
	if !strings.HasSuffix(strings.SplitN(requestURI, "?", 2)[0], "/issues") {
		t.Fatalf("request left the /issues endpoint, got RequestURI %q", requestURI)
	}
	if strings.Contains(requestURI, "/repos/admin/") {
		t.Fatalf("traversal collapsed to another owner, got RequestURI %q", requestURI)
	}
	if want := "/api/v1/repos/o%2Fx%2F..%2F..%2Fadmin/r%3Fstate=all&/issues?page=1&limit=20"; requestURI != want {
		t.Fatalf("RequestURI = %q, want %q", requestURI, want)
	}
}

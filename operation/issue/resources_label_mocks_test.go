// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v2/pkg/flag"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v2/pkg/forgejo"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// setupSDKClientFor points both the SDK client and the raw-HTTP helper at srv.
// The label resources use one path each — repo labels through the SDK, org
// labels through the raw-HTTP helper — so a test that switches backends has to
// move both.
func setupSDKClientFor(t *testing.T, url string) {
	t.Helper()
	flag.URL = url
	flag.Token = "tkn"
	flag.UserAgent = "test"
	client, err := forgejo_sdk.NewClient(url, forgejo_sdk.SetForgejoVersion("7.0.0"))
	if err != nil {
		t.Fatalf("creating test client: %v", err)
	}
	forgejo.SetClientForTesting(client)
}

// noTotalPaginatingHandler pages like paginatingHandler but deliberately omits
// X-Total-Count, reproducing an instance that advertises a next page without
// saying how many rows exist. The sentinel must then decline to name a total
// rather than invent one.
type noTotalPaginatingHandler struct {
	total   int
	pathHas string
	row     func(i int) map[string]interface{}
}

func (h *noTotalPaginatingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
	if offset+len(rows) < h.total {
		w.Header().Set("Link", fmt.Sprintf(`<%s?page=%d&limit=%d>; rel="next"`, r.URL.Path, page+1, size))
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(rows)
}

// limitRecordingHandler records the page and limit the handler actually asked
// the API for. Asserting on the request rather than the response is what proves
// the page size equals the caller's limit instead of limit+1 — the response
// alone cannot distinguish the two on a single page.
type limitRecordingHandler struct {
	page  string
	limit string
}

func (h *limitRecordingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	q := r.URL.Query()
	h.page = q.Get("page")
	h.limit = q.Get("limit")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`[]`))
}

// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/flag"
)

// versionServer serves /api/v1/version and counts requests to it and requests
// that carried an Authorization header.
type versionServer struct {
	versionRequests atomic.Int64
	authorized      atomic.Int64
	status          int
	version         string
}

func newVersionServer(t *testing.T, version string, status int) *versionServer {
	t.Helper()
	vs := &versionServer{version: version, status: status}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			vs.authorized.Add(1)
		}
		if r.URL.Path != "/api/v1/version" {
			http.NotFound(w, r)
			return
		}
		vs.versionRequests.Add(1)
		w.WriteHeader(vs.status)
		_ = json.NewEncoder(w).Encode(map[string]string{"version": vs.version})
	}))
	t.Cleanup(srv.Close)

	prevURL, prevToken := flag.URL, flag.Token
	flag.URL = srv.URL
	t.Cleanup(func() {
		flag.URL, flag.Token = prevURL, prevToken
		SetServerVersion("")
	})
	return vs
}

func TestProbeServerVersionSendsNoCredential(t *testing.T) {
	vs := newVersionServer(t, "16.0.3", http.StatusOK)
	// A configured operator value must not leak into the probe either.
	flag.Token = decoyOperatorCredential

	got, err := ProbeServerVersion(context.Background())
	if err != nil {
		t.Fatalf("ProbeServerVersion: %v", err)
	}
	if got != "16.0.3" {
		t.Fatalf("version = %q, want 16.0.3", got)
	}
	if vs.authorized.Load() != 0 {
		t.Fatal("the version probe sent an Authorization header")
	}
}

func TestProbeServerVersionRefusesAnErrorStatus(t *testing.T) {
	newVersionServer(t, "16.0.3", http.StatusBadGateway)
	if _, err := ProbeServerVersion(context.Background()); err == nil {
		t.Fatal("ProbeServerVersion accepted a 502")
	}
}

func TestMajorVersion(t *testing.T) {
	cases := map[string]int{
		"16.0.3":                               16,
		"16.0.0-dev-741-6f391573+gitea-1.22.0": 16,
		"15.0.3":                               15,
		"v16.1":                                16,
		"16":                                   16,
	}
	for in, want := range cases {
		got, err := MajorVersion(in)
		if err != nil || got != want {
			t.Errorf("MajorVersion(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "dev", "x.1.2"} {
		if _, err := MajorVersion(bad); err == nil {
			t.Errorf("MajorVersion(%q) accepted a version without a numeric major", bad)
		}
	}
}

// Once the version is recorded at startup, building a client per request must
// not ask Forgejo for its version again.
func TestRecordedServerVersionSkipsThePerClientVersionRequest(t *testing.T) {
	vs := newVersionServer(t, "16.0.3", http.StatusOK)
	ctx := WithToken(context.Background(), "per-request-value")

	SetServerVersion("16.0.3")
	for range 2 {
		if _, err := Client(ctx); err != nil {
			t.Fatalf("Client: %v", err)
		}
	}
	if got := vs.versionRequests.Load(); got != 0 {
		t.Fatalf("version requests with a recorded version = %d, want 0", got)
	}

	// The control: without a recorded version the SDK asks on construction,
	// which is the request this change removes.
	SetServerVersion("")
	if _, err := Client(ctx); err != nil {
		t.Fatalf("Client: %v", err)
	}
	if got := vs.versionRequests.Load(); got != 1 {
		t.Fatalf("version requests without a recorded version = %d, want 1", got)
	}
}

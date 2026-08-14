// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v2/pkg/flag"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// TestMaxResponseItems_CachesAcrossCalls asserts that a second call for the
// same instance does not re-hit the settings endpoint.
func TestMaxResponseItems_CachesAcrossCalls(t *testing.T) {
	var requests int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/version":
			_, _ = w.Write([]byte(`{"version":"1.22.0"}`))
		case "/api/v1/settings/api":
			atomic.AddInt32(&requests, 1)
			_, _ = w.Write([]byte(`{"max_response_items":50,"default_paging_num":30,"default_git_trees_per_page":1000,"default_max_blob_size":10485760}`))
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()

	flag.URL = srv.URL
	flag.Token = "global-token"
	client = nil
	resetSettingsCacheForTesting()

	ctx := context.Background()

	max1, ok1 := MaxResponseItems(ctx)
	if !ok1 {
		t.Fatalf("expected ok=true on first call")
	}
	if max1 != 50 {
		t.Fatalf("expected max_response_items=50, got %d", max1)
	}

	max2, ok2 := MaxResponseItems(ctx)
	if !ok2 || max2 != 50 {
		t.Fatalf("expected cached value 50/true, got %d/%v", max2, ok2)
	}

	if got := atomic.LoadInt32(&requests); got != 1 {
		t.Fatalf("expected exactly 1 settings request (cache hit on 2nd call), got %d", got)
	}
}

// TestMaxResponseItems_FailureIsUnknownNotZero asserts that a non-2xx
// response from the settings endpoint (e.g. 403, or the endpoint missing on
// older/restricted instances) surfaces as ok=false ("unknown"), never as a
// ceiling of 0 — a zero ceiling would reject every request.
func TestMaxResponseItems_FailureIsUnknownNotZero(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/version":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"1.22.0"}`))
		case "/api/v1/settings/api":
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"message":"forbidden"}`))
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()

	flag.URL = srv.URL
	flag.Token = "global-token"
	client = nil
	resetSettingsCacheForTesting()

	ctx := context.Background()

	max, ok := MaxResponseItems(ctx)
	if ok {
		t.Fatalf("expected ok=false on 403, got ok=true with max=%d", max)
	}
	if max != 0 {
		// max is meaningless when ok is false, but pin it down anyway so a
		// future change doesn't silently start returning a bogus non-zero
		// "ceiling" alongside ok=false.
		t.Fatalf("expected max=0 (unused) when ok=false, got %d", max)
	}
}

// TestSetClientForTesting_ResetsSettingsCache asserts that SetClientForTesting
// clears the cached ceiling, so one test's cached value cannot leak into the
// next test that reuses the same package-level state.
func TestSetClientForTesting_ResetsSettingsCache(t *testing.T) {
	var requests int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/version":
			_, _ = w.Write([]byte(`{"version":"1.22.0"}`))
		case "/api/v1/settings/api":
			atomic.AddInt32(&requests, 1)
			_, _ = w.Write([]byte(`{"max_response_items":50,"default_paging_num":30,"default_git_trees_per_page":1000,"default_max_blob_size":10485760}`))
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()

	flag.URL = srv.URL
	flag.Token = "global-token"
	client = nil
	resetSettingsCacheForTesting()

	ctx := context.Background()
	if _, ok := MaxResponseItems(ctx); !ok {
		t.Fatalf("expected ok=true on first call")
	}
	if got := atomic.LoadInt32(&requests); got != 1 {
		t.Fatalf("expected 1 request before reset, got %d", got)
	}

	c, err := forgejo_sdk.NewClient(srv.URL, forgejo_sdk.SetToken("global-token"))
	if err != nil {
		t.Fatalf("failed to build test client: %v", err)
	}
	SetClientForTesting(c)

	if _, ok := MaxResponseItems(ctx); !ok {
		t.Fatalf("expected ok=true after reset+refetch")
	}
	if got := atomic.LoadInt32(&requests); got != 2 {
		t.Fatalf("expected a fresh settings request after SetClientForTesting reset the cache, got %d total requests", got)
	}
}

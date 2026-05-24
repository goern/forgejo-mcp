package forgejo

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"codeberg.org/goern/forgejo-mcp/v2/pkg/flag"
)

func TestClient_WithContextToken(t *testing.T) {
	// Need a real httptest server because forgejo.NewClient makes a test request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":"1.22.0"}`))
	}))
	defer srv.Close()

	// Setup flag defaults
	flag.URL = srv.URL
	flag.Token = "global-token"
	// Reset the singleton
	client = nil
	clientOnce = sync.Once{}

	// 1. Get client without context token
	ctx := context.Background()
	c1, err := Client(ctx)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if c1 == nil {
		t.Fatalf("expected non-nil client")
	}

	// 2. Get client with context token
	ctx2 := WithToken(context.Background(), "request-token")
	c2, err := Client(ctx2)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if c2 == nil {
		t.Fatalf("expected non-nil client")
	}

	if c1 == c2 {
		t.Fatalf("expected a new ephemeral client instance when token is present, got the same singleton")
	}
}

func TestClient_AuthorizationHeader(t *testing.T) {
	tokensSeen := make(map[string]bool)
	var mu sync.Mutex

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "" {
			mu.Lock()
			tokensSeen[auth] = true
			mu.Unlock()
		}
		w.Header().Set("Content-Type", "application/json")
		// Mock responses for both /version and /user
		if r.URL.Path == "/api/v1/version" {
			_, _ = w.Write([]byte(`{"version":"1.22.0"}`))
		} else {
			_, _ = w.Write([]byte(`{"login":"test"}`))
		}
	}))
	defer srv.Close()

	flag.URL = srv.URL
	flag.Token = "global-token"
	client = nil
	clientOnce = sync.Once{}

	// Test concurrent requests with different tokens
	var wg sync.WaitGroup
	numRequests := 10
	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			token := fmt.Sprintf("token-%d", id)
			ctx := WithToken(context.Background(), token)
			c, err := Client(ctx)
			if err != nil {
				t.Errorf("failed to get client: %v", err)
				return
			}
			_, _, _ = c.GetMyUserInfo()
		}(i)
	}
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	for i := 0; i < numRequests; i++ {
		expected := fmt.Sprintf("token token-%d", i)
		if !tokensSeen[expected] {
			t.Errorf("expected to see token %q, but didn't", expected)
		}
	}
}

package forgejo

import (
	"context"
	"testing"
	"net/http"
	"net/http/httptest"
	"sync"
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
	// Reset the singleton just in case it was modified by other tests in this package
	client = nil
	clientOnce = sync.Once{}

	// 1. Get client without context token
	ctx := context.Background()
	c1 := Client(ctx)
	
	if token, ok := ctx.Value(TokenContextKey).(string); ok {
		t.Fatalf("expected no token in base context, got %q", token)
	}
	
	if c1 == nil {
		t.Fatalf("expected non-nil client")
	}

	// 2. Get client with context token
	ctx2 := WithToken(context.Background(), "request-token")
	token2, ok := ctx2.Value(TokenContextKey).(string)
	if !ok || token2 != "request-token" {
		t.Fatalf("expected 'request-token' in context, got %v", token2)
	}

	c2 := Client(ctx2)
	if c2 == nil {
		t.Fatalf("expected non-nil client")
	}

	if c1 == c2 {
		t.Fatalf("expected a new ephemeral client instance when token is present, got the same singleton")
	}
}
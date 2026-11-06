// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/flag"
)

const rejectedToken = "not-a-real-token-for-redaction-tests"

// rejectingForge answers every request the way Forgejo answers a token it
// does not accept: 401, with the token repeated in the message.
func rejectingForge(t *testing.T) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "token ")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"access token does not exist [sha: ` + token + `]"}`))
	}))
	t.Cleanup(srv.Close)
	savedURL, savedToken := flag.URL, flag.Token
	flag.URL, flag.Token = srv.URL, ""
	client = nil
	t.Cleanup(func() { flag.URL, flag.Token = savedURL, savedToken })
}

func TestClientRedactsARejectedToken(t *testing.T) {
	rejectingForge(t)

	_, err := Client(WithToken(context.Background(), rejectedToken))
	if err == nil {
		t.Fatal("expected an error for a rejected token")
	}
	if strings.Contains(err.Error(), rejectedToken) {
		t.Fatalf("error carries the token: %v", err)
	}
	// The rejection transport replaces Forgejo's message before redaction
	// ever sees it; redactToken stays as a second line of defence.
	if !strings.Contains(err.Error(), RejectedCredentialMessage) {
		t.Fatalf("error lost the rejection message: %v", err)
	}
}

func TestDoJSONRedactsARejectedToken(t *testing.T) {
	rejectingForge(t)

	err := DoJSON(WithToken(context.Background(), rejectedToken), http.MethodGet, APIPath("user"), nil, nil)
	if err == nil {
		t.Fatal("expected an error for a rejected token")
	}
	if strings.Contains(err.Error(), rejectedToken) {
		t.Fatalf("error carries the token: %v", err)
	}
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("redaction dropped the ErrUnauthorized sentinel: %v", err)
	}
}

func TestRedactTokenKeepsCleanErrors(t *testing.T) {
	clean := errors.New("not found")
	if got := redactToken(clean, rejectedToken); !errors.Is(got, clean) {
		t.Fatalf("a clean error was replaced: %v", got)
	}
	if got := redactToken(clean, ""); !errors.Is(got, clean) {
		t.Fatalf("an empty token changed the error: %v", got)
	}
	if redactToken(nil, rejectedToken) != nil {
		t.Fatal("nil error became non-nil")
	}
}

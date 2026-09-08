// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/flag"
)

// The decoy below is never a real credential and never reaches a network: these
// tests assert which value is selected, not that it authenticates.
const decoyOperatorCredential = "decoy-operator-value-not-a-credential"

func withPolicy(t *testing.T, require bool, operatorValue string) {
	t.Helper()
	prevRequire, prevToken := RequireRequestToken(), flag.Token
	SetRequireRequestToken(require)
	flag.Token = operatorValue
	t.Cleanup(func() {
		SetRequireRequestToken(prevRequire)
		flag.Token = prevToken
	})
}

func TestRequestCredentialAlwaysWins(t *testing.T) {
	withPolicy(t, true, decoyOperatorCredential)
	ctx := WithToken(context.Background(), "per-request-value")
	got, err := tokenForRequest(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "per-request-value" {
		t.Fatalf("got %q, want the per-request value", got)
	}
}

func TestOperatorCredentialStandsInWhenTheFallbackIsAllowed(t *testing.T) {
	// This is the stdio behaviour, and the loopback-listener behaviour. It is
	// the control: if this fails, the change has broken the supported case
	// rather than the attack.
	withPolicy(t, false, decoyOperatorCredential)
	got, err := tokenForRequest(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != decoyOperatorCredential {
		t.Fatalf("got %q, want the operator value", got)
	}
}

func TestAnonymousRequestIsRefusedWhenTheFallbackIsOff(t *testing.T) {
	withPolicy(t, true, decoyOperatorCredential)
	got, err := tokenForRequest(context.Background())
	if !errors.Is(err, ErrNoRequestToken) {
		t.Fatalf("got (%q, %v), want ErrNoRequestToken", got, err)
	}
	if got != "" {
		t.Fatalf("a refused request must yield no credential, got %q", got)
	}
}

func TestEmptyRequestCredentialIsNotAnIdentity(t *testing.T) {
	// An Authorization header present but empty must not be treated as an
	// identity that then falls through to the operator's own value.
	withPolicy(t, true, decoyOperatorCredential)
	ctx := WithToken(context.Background(), "")
	if _, err := tokenForRequest(ctx); !errors.Is(err, ErrNoRequestToken) {
		t.Fatalf("empty per-request credential was accepted: %v", err)
	}
}

func TestRawHTTPHelperRefusesRatherThanSendingTheOperatorCredential(t *testing.T) {
	// Pre-mortem 3: the refusal must live at the fallback site itself, so a
	// caller that never passed through the transport check cannot walk around
	// it. No server is involved here at all.
	withPolicy(t, true, decoyOperatorCredential)

	req, err := http.NewRequest(http.MethodGet, "https://example.invalid/api/v1/version", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if err := setCommonHeaders(context.Background(), req); !errors.Is(err, ErrNoRequestToken) {
		t.Fatalf("setCommonHeaders did not refuse: %v", err)
	}
	if got := req.Header.Get("Authorization"); got != "" {
		t.Fatalf("a refused request must carry no Authorization header, got %q", got)
	}
}

func TestClientFactoryRefusesRatherThanReturningTheOperatorClient(t *testing.T) {
	withPolicy(t, true, decoyOperatorCredential)
	c, err := Client(context.Background())
	if !errors.Is(err, ErrNoRequestToken) {
		t.Fatalf("Client did not refuse: %v", err)
	}
	if c != nil {
		t.Fatal("a refused request must yield no client")
	}
}

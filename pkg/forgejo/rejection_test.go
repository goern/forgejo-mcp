// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/flag"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/log"
)

const echoedCredential = "not-a-real-credential-echo-canary"

// echoingForge answers like Forgejo does for a credential it refuses: 401,
// with the credential repeated in the message. Requests without a credential
// to /api/v1/version get a version, so a test can pick whether the SDK's
// version probe sees the credential. status overrides the 401 for requests
// that carry one, to check that other statuses pass through untouched.
func echoingForge(t *testing.T, status int) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		auth := r.Header.Get("Authorization")
		if auth == "" && r.URL.Path == "/api/v1/version" {
			_, _ = w.Write([]byte(`{"version":"16.0.3"}`))
			return
		}
		token := strings.TrimPrefix(auth, "token ")
		w.WriteHeader(status)
		_, _ = fmt.Fprintf(w, `{"message":"token is malformed\naccess token does not exist [sha: %s]\ntask with token %q: resource does not exist"}`, token, token)
	}))
	t.Cleanup(srv.Close)
	savedURL, savedToken := flag.URL, flag.Token
	flag.URL, flag.Token = srv.URL, ""
	client = nil
	t.Cleanup(func() {
		flag.URL, flag.Token = savedURL, savedToken
		client = nil
		SetServerVersion("")
	})
}

// observeLogs routes the package logger into an observer for the test.
func observeLogs(t *testing.T) *observer.ObservedLogs {
	t.Helper()
	prev := log.Default()
	core, logs := observer.New(zapcore.DebugLevel)
	log.SetDefault(zap.New(core))
	t.Cleanup(func() { log.SetDefault(prev) })
	return logs
}

// assertNoCredential fails if err or any observed log entry mentions the
// echoed credential.
func assertNoCredential(t *testing.T, err error, logs *observer.ObservedLogs) {
	t.Helper()
	if err != nil && strings.Contains(err.Error(), echoedCredential) {
		t.Fatalf("error carries the credential: %v", err)
	}
	for _, e := range logs.All() {
		line := e.Message + fmt.Sprint(e.ContextMap())
		if strings.Contains(line, echoedCredential) {
			t.Fatalf("log entry carries the credential: %s", line)
		}
	}
}

// Spec stateless-http-auth, scenario "Rejection on an SDK call after the
// client was built".
func TestRejectionOnAnSDKCallAfterTheClientWasBuilt(t *testing.T) {
	echoingForge(t, http.StatusUnauthorized)
	SetServerVersion("16.0.3") // no per-client probe: the first real call is rejected
	logs := observeLogs(t)
	rejection := &Rejection{}
	ctx := WithRejection(WithToken(context.Background(), echoedCredential), rejection)

	c, err := Client(ctx)
	if err != nil {
		t.Fatalf("Client: %v", err)
	}
	_, _, err = c.GetMyUserInfo()
	if err == nil {
		t.Fatal("expected the call to fail")
	}
	if err.Error() != RejectedCredentialMessage {
		t.Fatalf("error = %q, want %q", err, RejectedCredentialMessage)
	}
	if !rejection.Rejected() {
		t.Fatal("the rejection was not recorded")
	}
	assertNoCredential(t, err, logs)
}

// Spec stateless-http-auth, scenario "Rejection while the per-request client
// is built".
func TestRejectionWhileThePerRequestClientIsBuilt(t *testing.T) {
	echoingForge(t, http.StatusUnauthorized)
	logs := observeLogs(t)
	rejection := &Rejection{}
	ctx := WithRejection(WithToken(context.Background(), echoedCredential), rejection)

	_, err := Client(ctx)
	if err == nil {
		t.Fatal("expected construction to fail")
	}
	if !strings.Contains(err.Error(), RejectedCredentialMessage) {
		t.Fatalf("error lost the fixed message: %v", err)
	}
	if !rejection.Rejected() {
		t.Fatal("the rejection was not recorded")
	}
	assertNoCredential(t, err, logs)
}

// Spec stateless-http-auth, scenario "Rejection on a raw-HTTP helper call".
func TestRejectionOnARawHTTPHelperCall(t *testing.T) {
	echoingForge(t, http.StatusUnauthorized)
	logs := observeLogs(t)
	rejection := &Rejection{}
	ctx := WithRejection(WithToken(context.Background(), echoedCredential), rejection)

	err := DoJSON(ctx, http.MethodGet, APIPath("user"), nil, nil)
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("err = %v, want ErrUnauthorized", err)
	}
	if !strings.Contains(err.Error(), RejectedCredentialMessage) {
		t.Fatalf("error lost the fixed message: %v", err)
	}
	if !rejection.Rejected() {
		t.Fatal("the rejection was not recorded")
	}
	assertNoCredential(t, err, logs)
}

// Spec stateless-http-auth, scenario "The operator token is rejected at
// startup".
func TestTheOperatorTokenIsRejectedAtStartup(t *testing.T) {
	echoingForge(t, http.StatusUnauthorized)
	flag.Token = echoedCredential
	logs := observeLogs(t)

	err := VerifyConnection()
	if err == nil {
		t.Fatal("expected the connection test to fail")
	}
	assertNoCredential(t, err, logs)
}

// Spec stateless-http-auth, scenario "Insufficient scope is not a rejection":
// statuses other than 401 keep Forgejo's body and do not mark the request.
func TestOtherStatusesPassThrough(t *testing.T) {
	for _, status := range []int{http.StatusForbidden, http.StatusNotFound} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			echoingForge(t, status)
			rejection := &Rejection{}
			ctx := WithRejection(WithToken(context.Background(), "scoped-token"), rejection)

			err := DoJSON(ctx, http.MethodGet, APIPath("user"), nil, nil)
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), "access token does not exist") {
				t.Fatalf("Forgejo's body was replaced for %d: %v", status, err)
			}
			if rejection.Rejected() {
				t.Fatalf("a %d marked the request as rejected", status)
			}
		})
	}
}

// A 401 to a request that carried no credential is left alone: there is no
// credential to echo, and nothing the caller presented was refused.
func TestUnauthenticatedRequestIsNotMarked(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"token is required"}`))
	}))
	t.Cleanup(srv.Close)
	rejection := &Rejection{}
	req, _ := http.NewRequestWithContext(WithRejection(context.Background(), rejection), http.MethodGet, srv.URL, nil)

	resp, err := (&http.Client{Transport: &rejectionTransport{}}).Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()
	if rejection.Rejected() {
		t.Fatal("a 401 without a credential marked the request")
	}
}

func TestRejectionWithoutAMarkerStillReplacesTheBody(t *testing.T) {
	echoingForge(t, http.StatusUnauthorized)
	err := DoJSON(WithToken(context.Background(), echoedCredential), http.MethodGet, APIPath("user"), nil, nil)
	if err == nil || strings.Contains(err.Error(), echoedCredential) {
		t.Fatalf("err = %v, want a rejection without the credential", err)
	}
	if (*Rejection)(nil).Rejected() {
		t.Fatal("a nil Rejection reports rejected")
	}
}

// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
)

// RejectedCredentialMessage is the only text a Forgejo 401 leaves behind.
// Forgejo repeats the credential it refused in its message ("access token
// does not exist [sha: <token>]"), so its body is replaced with this before
// the SDK or the raw-HTTP helpers read it, and no error, log line or
// response built from it can carry the credential.
const RejectedCredentialMessage = "Forgejo rejected the access token"

// rejectedBody is the JSON body a Forgejo 401 is replaced with. The SDK turns
// its "message" into the error text, and the raw helpers quote it.
var rejectedBody = `{"message":` + strconv.Quote(RejectedCredentialMessage) + `}`

// maxDrainedRejection bounds how much of Forgejo's own 401 body is read and
// discarded before the connection is reused.
const maxDrainedRejection = 64 << 10

// Rejection records that Forgejo refused the per-request credential at least
// once while one MCP request was served. It is shared by every Forgejo call
// that request makes, so a single 401 marks the whole request.
type Rejection struct {
	rejected atomic.Bool
}

// Rejected reports whether Forgejo refused the request's credential.
func (r *Rejection) Rejected() bool { return r != nil && r.rejected.Load() }

func (r *Rejection) mark() {
	if r != nil {
		r.rejected.Store(true)
	}
}

type rejectionContextKey struct{}

// WithRejection attaches r to ctx. Only a context that carries a per-request
// credential should carry a Rejection: a refused operator token is not
// something the caller can fix by signing in again.
func WithRejection(ctx context.Context, r *Rejection) context.Context {
	return context.WithValue(ctx, rejectionContextKey{}, r)
}

// RejectionFrom returns the Rejection attached to ctx, or nil.
func RejectionFrom(ctx context.Context) *Rejection {
	r, _ := ctx.Value(rejectionContextKey{}).(*Rejection)
	return r
}

// rejectionTransport wraps the transport of every HTTP client that talks to
// Forgejo. On a 401 to a request that carried a credential it replaces the
// body with rejectedBody and marks the request context's Rejection, if any.
type rejectionTransport struct {
	base http.RoundTripper
}

func (t *rejectionTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	resp, err := base.RoundTrip(req)
	if err != nil || resp.StatusCode != http.StatusUnauthorized || req.Header.Get("Authorization") == "" {
		return resp, err
	}
	RejectionFrom(req.Context()).mark()

	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxDrainedRejection))
	_ = resp.Body.Close()
	resp.Body = io.NopCloser(strings.NewReader(rejectedBody))
	resp.ContentLength = int64(len(rejectedBody))
	resp.Header = resp.Header.Clone()
	resp.Header.Set("Content-Type", "application/json")
	resp.Header.Set("Content-Length", strconv.Itoa(len(rejectedBody)))
	resp.Header.Del("Content-Encoding")
	return resp, nil
}

// sdkHTTPClient is the HTTP client every SDK client is built with. It is
// shared so that keep-alive connections survive across per-request clients.
var sdkHTTPClient = &http.Client{Transport: &rejectionTransport{}, CheckRedirect: checkRedirect}

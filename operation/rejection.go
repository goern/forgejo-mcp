// SPDX-License-Identifier: GPL-3.0-or-later

package operation

import (
	"context"
	"net/http"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forgejo"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/log"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// credentialRejectedCode is the JSON-RPC error code of a call during which
// Forgejo refused the caller's own credential. It is taken from the
// implementation-defined range -32000..-32019 and avoids the codes MCP SDKs
// already give a meaning: -32000 (connection closed) and -32001 (request
// timeout) in the TypeScript and Python SDKs, -32002 (resource not found) in
// mcp-go, and -32003, which operation/resource uses for a 404.
const credentialRejectedCode = -32004

// mintedCredentialRejectedMessage is what a caller sees in resource-server mode
// when Forgejo refuses the JWT this server minted. The caller's sign-in is
// valid, so the message points at the server side rather than at the caller.
const mintedCredentialRejectedMessage = "Forgejo rejected the credential this server issued for you; " +
	"the operator has to check the Authorized Integration on Forgejo. Signing in again will not help"

// mintedCredentialRejectedError is the error of a call in resource-server mode
// whose minted JWT Forgejo refused. mcp-go reports it as -32603.
type mintedCredentialRejectedError struct{}

func (mintedCredentialRejectedError) Error() string { return mintedCredentialRejectedMessage }

// credentialRejectedError is the typed JSON-RPC error of a rejected
// per-request credential in passthrough mode.
type credentialRejectedError struct{}

func (credentialRejectedError) Error() string { return forgejo.RejectedCredentialMessage }

func (credentialRejectedError) JSONRPCError() mcp.JSONRPCError {
	return mcp.JSONRPCError{
		JSONRPC: mcp.JSONRPC_VERSION,
		Error:   mcp.NewJSONRPCErrorDetails(credentialRejectedCode, forgejo.RejectedCredentialMessage, nil),
	}
}

// rejectionOutcome is the error a call ends with when Forgejo refused its
// per-request credential, whatever the handler itself returned. A handler
// that turned the refusal into an empty result must not hide it. what names
// the tool or resource for the operator's log line.
func rejectionOutcome(ctx context.Context, what string) error {
	if resourceServerMode() {
		log.WarnCtx(ctx, "Forgejo rejected the JWT minted for the caller; check the caller's Authorized Integration and the published key set",
			log.StringField("call", what),
		)
		return mintedCredentialRejectedError{}
	}
	return server.NewHandlerError(credentialRejectedError{})
}

// rejectionToolMiddleware maps a tool call whose credential Forgejo refused to
// the rejection outcome.
//
// The Rejection it reads belongs to the HTTP request, not to the call. The two
// coincide because the streamable transport refuses JSON-RPC batches, so one
// request carries one call; TestABatchIsRefusedSoARejectionCoversOneCall
// guards that. With batches, one refused call would fail its siblings too,
// including one that never reached Forgejo.
func rejectionToolMiddleware(next server.ToolHandlerFunc) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		result, err := next(ctx, request)
		if forgejo.RejectionFrom(ctx).Rejected() {
			return nil, rejectionOutcome(ctx, "tool "+request.Params.Name)
		}
		return result, err
	}
}

// rejectionResourceMiddleware is rejectionToolMiddleware for resource reads.
func rejectionResourceMiddleware(next server.ResourceHandlerFunc) server.ResourceHandlerFunc {
	return func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		contents, err := next(ctx, request)
		if forgejo.RejectionFrom(ctx).Rejected() {
			return nil, rejectionOutcome(ctx, "resource "+request.Params.URI)
		}
		return contents, err
	}
}

// withRejectionChallenge gives every request a fresh Rejection, so that the
// Forgejo calls made while serving it can record a refused credential. In
// passthrough mode it then answers such a request with 401 and a challenge
// instead of 200, as long as the status has not been sent yet. In
// resource-server mode the caller's sign-in was valid, so the status is left
// alone: a challenge would only start a sign-in that cannot fix the cause.
func withRejectionChallenge(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rejection := &forgejo.Rejection{}
		rw := &rejectionResponseWriter{
			ResponseWriter: w,
			rejection:      rejection,
			challenge:      !resourceServerMode(),
		}
		next.ServeHTTP(rw, r.WithContext(forgejo.WithRejection(r.Context(), rejection)))
	})
}

// rejectionResponseWriter turns the 200 of a request whose credential Forgejo
// refused into a 401. It keeps http.Flusher, without which the transport
// refuses to stream, and exposes Unwrap for http.ResponseController.
type rejectionResponseWriter struct {
	http.ResponseWriter
	rejection   *forgejo.Rejection
	challenge   bool
	wroteHeader bool
}

func (w *rejectionResponseWriter) WriteHeader(code int) {
	if w.wroteHeader {
		w.ResponseWriter.WriteHeader(code)
		return
	}
	w.wroteHeader = true
	if code == http.StatusOK && w.challenge && w.rejection.Rejected() {
		w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
		code = http.StatusUnauthorized
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *rejectionResponseWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

func (w *rejectionResponseWriter) Flush() {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *rejectionResponseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

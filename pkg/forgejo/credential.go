// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo

import (
	"context"
	"errors"
	"sync/atomic"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/flag"
)

// ErrNoRequestToken is returned when a request carries no credential of its own
// and this server does not fall back to the operator's configured credential.
//
// The fallback exists for the stdio transport, where the client launched this
// process and there is no per-request identity to carry — the change that
// introduced per-request authentication describes it as "preserving backward
// compatibility for --stdio mode". On a network transport the same fallback
// serves an anonymous request with the operator's own credential, so the
// network transports switch it off.
var ErrNoRequestToken = errors.New(
	"request carries no Authorization header and this server does not fall back to its own credential; " +
		"send a per-request token",
)

// requireRequestToken is false by default, which is the stdio behaviour and the
// behaviour of every release before this one. The network transports set it in
// operation.serveMCPOverHTTP before they begin serving.
var requireRequestToken atomic.Bool

// SetRequireRequestToken selects whether an absent per-request credential is an
// error (true) or falls back to the operator's configured credential (false).
//
// It is deliberately NOT derived from the address the listener bound. Keying it
// to a loopback bind — on the reasoning that the reachable set is then the same
// as stdio's — looks safe and is not, in two ways:
//
//   - A loopback TCP port is reachable by every local user account and every
//     sandboxed process on the machine. stdio is reachable only by the process
//     that spawned the server. They are not the same boundary.
//   - A reverse proxy in front of a loopback listener makes the listener's own
//     address say nothing about who can reach it. nginx and Apache both rewrite
//     the Host header to the proxied target by default, so the request arrives
//     on a loopback socket carrying a loopback Host — every observable the
//     server can check looks local, and the request came from the internet.
//
// The second point has the worse shape: a proxy configured to preserve the
// original Host is refused and gets fixed, while the default configuration
// silently works and is the insecure one. A control whose failure mode selects
// for the vulnerable deployment is not a control.
func SetRequireRequestToken(require bool) { requireRequestToken.Store(require) }

// RequireRequestToken reports the current policy.
func RequireRequestToken() bool { return requireRequestToken.Load() }

// tokenForRequest resolves the credential to use for ctx.
//
// This is the single place the fallback lives. Both the typed client factory
// and the raw-HTTP helper call it, so the two cannot drift apart and no future
// call path can reach a fallback that skipped the policy.
func tokenForRequest(ctx context.Context) (string, error) {
	if token, ok := ctx.Value(TokenContextKey).(string); ok && token != "" {
		return token, nil
	}
	if requireRequestToken.Load() {
		return "", ErrNoRequestToken
	}
	return flag.Token, nil
}

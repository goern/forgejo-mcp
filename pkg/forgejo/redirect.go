// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo

import (
	"errors"
	"fmt"
	"net/http"
)

// maxRedirects is net/http's default limit, which a custom CheckRedirect
// replaces and so has to enforce itself.
const maxRedirects = 10

// ErrRedirectRefused reports that a redirect was not followed because it
// would have changed the request method.
var ErrRedirectRefused = errors.New("redirect refused")

// checkRedirect is the redirect policy of every HTTP client that talks to
// Forgejo. net/http turns any non-GET request into a GET when it follows a
// 301, 302 or 303, so a write answered with one of those would come back as
// the successful result of a read, and the write would be lost. A renamed
// repository is enough to trigger it: Forgejo answers 301, and POST .../forks
// becomes GET .../forks, which lists forks instead of creating one.
//
// Redirects that keep the method are followed: all of them for GET and HEAD,
// and 307/308 for anything else, which net/http replays with the body. A
// request whose body cannot be replayed is not redirected at all; the caller
// gets the 307/308 itself.
func checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= maxRedirects {
		return fmt.Errorf("stopped after %d redirects", maxRedirects)
	}
	// net/http keeps Authorization across a redirect to the same host on
	// another port, to a subdomain, and from https to http. None of those is
	// the Forgejo instance the token belongs to, so the token is dropped on
	// any hop that leaves it. The redirect itself is still followed: object
	// storage serving attachments directly answers with a presigned URL on
	// another host, which needs no token and is refused when given one.
	if req.Header.Get("Authorization") != "" && !withinInstanceURL(req.URL) {
		req.Header.Del("Authorization")
	}
	previous := via[len(via)-1]
	if req.Method == previous.Method {
		return nil
	}
	// The write is reported as unconfirmed rather than not applied: a 303
	// conventionally follows a write that succeeded.
	return fmt.Errorf("%w: %s %s answered %s with location %s; following it would turn the %s into a %s, so the write is unconfirmed; if the repository or owner was renamed, retry with the new name, otherwise check the configured Forgejo URL",
		ErrRedirectRefused,
		previous.Method, previous.URL.Redacted(),
		req.Response.Status, req.URL.Redacted(),
		previous.Method, req.Method)
}

// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo

import (
	"errors"
	"strings"
)

// redactedToken replaces a request token in error text.
const redactedToken = "[redacted]"

// redactToken returns err with every occurrence of token in its message
// replaced. Forgejo repeats a token it does not accept in its error
// ("access token does not exist [sha: <token>]"), and the error of a tool call
// is logged and returned to the client, so without this the token of a
// mistyped, revoked or wrong-instance request would be written to the log in
// full. err is returned unchanged when it does not contain the token.
func redactToken(err error, token string) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	if clean := redactTokenString(msg, token); clean != msg {
		return errors.New(clean)
	}
	return err
}

// redactTokenString replaces every occurrence of token in s.
func redactTokenString(s, token string) string {
	if token == "" {
		return s
	}
	return strings.ReplaceAll(s, token, redactedToken)
}

// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo

import (
	"fmt"
	"net/url"
	"strings"
)

// APIPath builds a REST API path from segments, escaping every one of them.
//
// It exists because the raw-HTTP helpers in this package take a path string
// and concatenate it onto the base URL verbatim, while the SDK client escapes
// user-supplied segments for us (escapeValidatePathSegments). Building a path
// with a bare fmt.Sprintf therefore lets a tool argument containing "/" or "?"
// escape its segment and retarget the request at a different endpoint — which
// matters most for the DELETE/PATCH call sites.
//
// String segments are url.PathEscape'd; anything else is formatted with %v
// first (so int/int64 IDs read naturally at the call site) and escaped too,
// which is a no-op for digits. Every segment is prefixed with "/", so
//
//	APIPath("repos", owner, repo, "issues", index, "assets")
//
// yields "/repos/{owner}/{repo}/issues/{index}/assets". Query strings are not
// this function's job: append them to the result, escaping values yourself.
func APIPath(segments ...any) string {
	var b strings.Builder
	for _, seg := range segments {
		s, ok := seg.(string)
		if !ok {
			s = fmt.Sprintf("%v", seg)
		}
		b.WriteByte('/')
		b.WriteString(url.PathEscape(s))
	}
	return b.String()
}

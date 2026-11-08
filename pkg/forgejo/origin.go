// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/flag"
)

// ErrCrossOrigin reports that a URL lies outside the configured Forgejo
// instance, so the raw-HTTP helpers refused to send the token to it.
var ErrCrossOrigin = errors.New("URL is outside the configured Forgejo instance")

// WithinInstance reports whether rawURL addresses the configured Forgejo
// instance (flag.URL): same scheme, host and port, and a path at or under
// flag.URL's path. It is the only check standing between a server-supplied
// URL — an attachment's browser_download_url can be an external link anyone
// with release write access plants — and the operator's token.
//
// A relative URL (no scheme, no host) is within the instance by definition:
// the helpers resolve it against flag.URL.
func WithinInstance(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	if u.Scheme == "" && u.Host == "" {
		return true
	}
	return withinInstanceURL(u)
}

// withinInstanceURL is WithinInstance for an absolute, parsed URL.
func withinInstanceURL(u *url.URL) bool {
	base, err := url.Parse(strings.TrimRight(flag.URL, "/"))
	if err != nil || base.Host == "" {
		return false
	}
	if u.User != nil {
		return false
	}
	// url.Parse lowercases the scheme; compare anyway in case u was built by hand.
	if !strings.EqualFold(u.Scheme, base.Scheme) {
		return false
	}
	if !strings.EqualFold(u.Hostname(), base.Hostname()) {
		return false
	}
	if effectivePort(u) != effectivePort(base) {
		return false
	}
	return withinBasePath(u.Path, base.Path)
}

// effectivePort returns u's port, filling in the scheme's default so that
// https://h and https://h:443 compare equal.
func effectivePort(u *url.URL) string {
	if p := u.Port(); p != "" {
		return p
	}
	switch strings.ToLower(u.Scheme) {
	case "http":
		return "80"
	case "https":
		return "443"
	}
	return ""
}

// withinBasePath reports whether p is basePath or below it. A Forgejo served
// under a ROOT_URL sub-path shares its host with whatever else is mounted
// there, so the host alone does not identify it. A dot segment is refused
// outright: "/forgejo/../other" has the prefix and is not under it, and the
// server, not net/url, decides how to normalise it.
func withinBasePath(p, basePath string) bool {
	for _, seg := range strings.Split(p, "/") {
		if seg == "." || seg == ".." {
			return false
		}
	}
	basePath = strings.TrimRight(basePath, "/")
	if basePath == "" {
		return true
	}
	return p == basePath || strings.HasPrefix(p, basePath+"/")
}

// checkAbsoluteWithinInstance returns nil when s is relative, or absolute and
// within the instance; otherwise an error wrapping ErrCrossOrigin. It never
// fails for a string net/url cannot parse — that is left to the request
// builder, which reports it with better context.
func checkAbsoluteWithinInstance(s string) (absolute bool, err error) {
	u, perr := url.Parse(s)
	if perr != nil || (u.Scheme == "" && u.Host == "") {
		return false, nil
	}
	if !withinInstanceURL(u) {
		return true, fmt.Errorf("%w: %s (configured: %s)", ErrCrossOrigin, u.Redacted(), instanceOrigin())
	}
	return true, nil
}

// instanceOrigin is flag.URL with any userinfo redacted, for messages.
func instanceOrigin() string {
	base, err := url.Parse(strings.TrimRight(flag.URL, "/"))
	if err != nil {
		return "<unparseable>"
	}
	return base.Redacted()
}

// CrossOriginDownloadReason is the reason a download tool reports when an
// attachment's browser_download_url lies outside the configured instance and
// its content was therefore not fetched.
const CrossOriginDownloadReason = "browser_download_url is outside the configured Forgejo instance, so the content was not fetched: " +
	"this server sends its Forgejo token only to that instance. Do not send the token to this URL; " +
	"if it is a trusted external link, fetch it without credentials"

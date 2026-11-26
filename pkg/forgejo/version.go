// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	goversion "github.com/hashicorp/go-version"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/log"
)

// maxVersionBody bounds the /version response read at startup.
const maxVersionBody = 64 << 10

// serverVersion holds the Forgejo version read at startup, or nothing.
var serverVersion atomic.Value

// SetServerVersion records the Forgejo version read at startup. Every SDK
// client built afterwards is told it, and so skips the version request the SDK
// otherwise sends each time a client is constructed.
func SetServerVersion(v string) { serverVersion.Store(v) }

// KnownServerVersion returns the version recorded with SetServerVersion, or
// "" when none is.
func KnownServerVersion() string { return knownServerVersion() }

func knownServerVersion() string {
	v, _ := serverVersion.Load().(string)
	return v
}

// clientOptions appends the recorded server version, when there is one, to the
// options an SDK client is built with.
func clientOptions(opts ...sdk.ClientOption) []sdk.ClientOption {
	if v := knownServerVersion(); v != "" {
		opts = append(opts, sdk.SetForgejoVersion(v))
	}
	return opts
}

// ErrSignInRequired reports that Forgejo refused the anonymous version
// request with a 401 or 403 API error and that /swagger.v1.json yielded no
// version either: the instance is there, but it requires sign-in to say which
// version it is.
var ErrSignInRequired = errors.New("sign-in required to read the Forgejo version")

// errVersionRefused reports that /api/v1/version answered 401 or 403 with a
// Forgejo API error. That is what an instance with REQUIRE_SIGNIN_VIEW does,
// and it proves the URL reaches Forgejo.
var errVersionRefused = errors.New("anonymous version request refused by Forgejo")

// maxSwaggerBody bounds how much of /swagger.v1.json the fallback reads. The
// document is about 850 KB; info sits near its start.
const maxSwaggerBody = 4 << 20

// probeTimeout bounds one startup probe, both of its requests together, so a
// stalled instance cannot hold startup for rawHTTPClient's full timeout.
var probeTimeout = 15 * time.Second

// ProbeServerVersion asks the configured Forgejo instance for its version
// without sending any credential. It requests /api/v1/version, which needs no
// credential on a default instance. An instance with REQUIRE_SIGNIN_VIEW
// answers that with a 401 or 403 API error but still publishes
// /swagger.v1.json, whose info.version carries the same value, so in that case
// the probe reads that instead. When neither yields a version, the error wraps
// ErrSignInRequired. Any other failure -- no response at all, another status,
// or a 401/403 whose body is not a Forgejo API error -- is returned as is,
// without trying swagger, so a wrong URL never looks like a closed instance.
func ProbeServerVersion(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	version, err := probeVersionEndpoint(ctx)
	if !errors.Is(err, errVersionRefused) {
		return version, err
	}
	version, swaggerErr := probeSwaggerVersion(ctx)
	if swaggerErr != nil {
		return "", fmt.Errorf("%w: %w; %w", ErrSignInRequired, err, swaggerErr)
	}
	return version, nil
}

// CheckReachable checks, without a credential, that the configured URL
// reaches a Forgejo instance. It is the startup check when the operator
// states the version, so it does not fall back to swagger: a 401 or 403 API
// error from /api/v1/version counts as reachable and returns an empty version.
// A 200 returns the version Forgejo reports, for comparing with the stated one.
func CheckReachable(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	version, err := probeVersionEndpoint(ctx)
	if errors.Is(err, errVersionRefused) {
		return "", nil
	}
	return version, err
}

// probeVersionEndpoint requests /api/v1/version anonymously. A 401 or 403
// carrying a Forgejo API error returns an error wrapping errVersionRefused.
// The body is drained and closed before it returns, so a following request
// can reuse the connection.
func probeVersionEndpoint(ctx context.Context) (string, error) {
	u, err := resolveURL("/version")
	if err != nil {
		return "", err
	}
	safe := log.SanitizeURL(u)
	resp, err := anonymousGet(ctx, u)
	if err != nil {
		return "", err
	}
	defer func() {
		drain(resp.Body, maxVersionBody)
		_ = resp.Body.Close()
	}()
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized, http.StatusForbidden:
		if !isForgejoAPIError(resp.Body) {
			return "", fmt.Errorf("GET %s: status %s, and the body is not a Forgejo API error; does the URL point at Forgejo?", safe, resp.Status)
		}
		return "", fmt.Errorf("%w: GET %s: status %s", errVersionRefused, safe, resp.Status)
	default:
		return "", fmt.Errorf("GET %s: status %s", safe, resp.Status)
	}
	var body struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxVersionBody)).Decode(&body); err != nil {
		return "", fmt.Errorf("GET %s: %w", safe, err)
	}
	if body.Version == "" {
		return "", fmt.Errorf("GET %s: response carries no version", safe)
	}
	return body.Version, nil
}

// isForgejoAPIError reports whether body is a Forgejo API error: a JSON
// object with a non-empty message. A proxy, a WAF or another service that
// refuses the request answers with something else.
func isForgejoAPIError(body io.Reader) bool {
	var apiErr struct {
		Message string `json:"message"`
	}
	err := json.NewDecoder(io.LimitReader(body, maxVersionBody)).Decode(&apiErr)
	return err == nil && apiErr.Message != ""
}

// drain reads and discards up to limit bytes of body, so that once it is
// closed the connection can be reused for the next request.
func drain(body io.Reader, limit int64) {
	_, _ = io.Copy(io.Discard, io.LimitReader(body, limit))
}

// anonymousGet sends a GET with no credential through the shared raw client,
// so the redirect policy and the rejection handling apply. Its errors name the
// URL without user information.
func anonymousGet(ctx context.Context, u string) (*http.Response, error) {
	safe := log.SanitizeURL(u)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("build request for %s: %w", safe, unwrapURLError(err))
	}
	req.Header.Set("User-Agent", userAgent())
	req.Header.Set("Accept", "application/json")
	resp, err := rawHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", safe, unwrapURLError(err))
	}
	return resp, nil
}

// unwrapURLError drops the *url.Error wrapper, whose message repeats the
// request URL with its user name, and keeps the cause.
func unwrapURLError(err error) error {
	var uerr *url.Error
	if errors.As(err, &uerr) {
		return uerr.Err
	}
	return err
}

// probeSwaggerVersion reads info.version from the instance's
// /swagger.v1.json. It walks the top-level object as a token stream and stops
// once info is decoded, reading at most maxSwaggerBody bytes. A document
// reached through a redirect to outside the instance is not trusted.
func probeSwaggerVersion(ctx context.Context) (string, error) {
	u, err := resolveSameOriginURL("/swagger.v1.json")
	if err != nil {
		return "", err
	}
	safe := log.SanitizeURL(u)
	resp, err := anonymousGet(ctx, u)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GET %s: status %s", safe, resp.Status)
	}
	if resp.Request != nil && resp.Request.URL != nil {
		final := *resp.Request.URL
		final.User = nil
		if !withinInstanceURL(&final) {
			return "", fmt.Errorf("GET %s: redirected outside the Forgejo instance, to %s", safe, final.Redacted())
		}
	}
	dec := json.NewDecoder(io.LimitReader(resp.Body, maxSwaggerBody))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return "", fmt.Errorf("GET %s: not a JSON object", safe)
	}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return "", fmt.Errorf("GET %s: %w", safe, err)
		}
		key, _ := tok.(string)
		if key != "info" {
			var skip json.RawMessage
			if err := dec.Decode(&skip); err != nil {
				return "", fmt.Errorf("GET %s: %w", safe, err)
			}
			continue
		}
		var info struct {
			Version string `json:"version"`
		}
		if err := dec.Decode(&info); err != nil {
			return "", fmt.Errorf("GET %s: info: %w", safe, err)
		}
		if info.Version == "" {
			return "", fmt.Errorf("GET %s: info carries no version", safe)
		}
		return info.Version, nil
	}
	return "", fmt.Errorf("GET %s: no info object", safe)
}

// ValidateVersion refuses a Forgejo version the SDK cannot parse, or one
// without a numeric major component. The SDK parses the recorded version every
// time it builds a client, so a value that fails here would make every call
// fail later instead of startup failing now.
func ValidateVersion(v string) error {
	if _, err := goversion.NewVersion(v); err != nil {
		return fmt.Errorf("the Forgejo version %q is not a version the Forgejo SDK can parse: %w", v, err)
	}
	_, err := MajorVersion(v)
	return err
}

// SameVersion reports whether two Forgejo versions are equal, ignoring build
// metadata, so "16.0.7" and "16.0.7+gitea-1.22.0" are the same. Unparsable
// values compare as strings.
func SameVersion(a, b string) bool {
	va, errA := goversion.NewVersion(a)
	vb, errB := goversion.NewVersion(b)
	if errA != nil || errB != nil {
		return strings.TrimSpace(a) == strings.TrimSpace(b)
	}
	return va.Equal(vb)
}

// MajorVersion returns the major component of a Forgejo version such as
// "16.0.3" or "16.0.0-dev-741-6f391573+gitea-1.22.0".
//
// Only the major number counts. Semantic versioning would sort a development
// build of 16 below 16.0.0, but such a build already has every 16 feature this
// server relies on.
func MajorVersion(v string) (int, error) {
	s := strings.TrimPrefix(strings.TrimSpace(v), "v")
	major, _, _ := strings.Cut(s, ".")
	n, err := strconv.Atoi(major)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("the Forgejo version %q has no numeric major component", v)
	}
	return n, nil
}

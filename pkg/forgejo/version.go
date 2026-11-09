// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"

	sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// maxVersionBody bounds the /version response read at startup.
const maxVersionBody = 64 << 10

// serverVersion holds the Forgejo version read at startup, or nothing.
var serverVersion atomic.Value

// SetServerVersion records the Forgejo version read at startup. Every SDK
// client built afterwards is told it, and so skips the version request the SDK
// otherwise sends each time a client is constructed.
func SetServerVersion(v string) { serverVersion.Store(v) }

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

// ErrSignInRequired reports that Forgejo answered the anonymous version probe
// with 401 or 403 and published no readable swagger document either: the
// instance is there, but it requires sign-in to say which version it is.
var ErrSignInRequired = errors.New("sign-in required to read the Forgejo version")

// maxSwaggerBody bounds how much of /swagger.v1.json the fallback reads. The
// document is about 850 KB; info sits near its start.
const maxSwaggerBody = 4 << 20

// ProbeServerVersion asks the configured Forgejo instance for its version
// without sending any credential. It requests /api/v1/version, which needs no
// credential on a default instance. An instance with REQUIRE_SIGNIN_VIEW
// answers that with 401 or 403 but still publishes /swagger.v1.json, whose
// info.version carries the same value, so on 401 or 403 the probe reads that
// instead. When neither yields a version after a 401 or 403, the error wraps
// ErrSignInRequired. Any other failure -- no response at all, or another
// status -- is returned as is, without trying swagger, so a wrong URL never
// looks like a closed instance.
func ProbeServerVersion(ctx context.Context) (string, error) {
	u, err := resolveURL("/version")
	if err != nil {
		return "", err
	}
	resp, err := anonymousGet(ctx, u)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized, http.StatusForbidden:
		version, swaggerErr := probeSwaggerVersion(ctx)
		if swaggerErr != nil {
			return "", fmt.Errorf("%w: GET %s: status %s; %w", ErrSignInRequired, u, resp.Status, swaggerErr)
		}
		return version, nil
	default:
		return "", fmt.Errorf("GET %s: status %s", u, resp.Status)
	}
	var body struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxVersionBody)).Decode(&body); err != nil {
		return "", fmt.Errorf("GET %s: %w", u, err)
	}
	if body.Version == "" {
		return "", fmt.Errorf("GET %s: response carries no version", u)
	}
	return body.Version, nil
}

// anonymousGet sends a GET with no credential through the shared raw client,
// so the TLS settings, the redirect policy and the rejection handling apply.
func anonymousGet(ctx context.Context, u string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("build request for %s: %w", u, err)
	}
	req.Header.Set("User-Agent", userAgent())
	req.Header.Set("Accept", "application/json")
	resp, err := rawHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", u, err)
	}
	return resp, nil
}

// probeSwaggerVersion reads info.version from the instance's
// /swagger.v1.json. It walks the top-level object as a token stream and stops
// once info is decoded, reading at most maxSwaggerBody bytes.
func probeSwaggerVersion(ctx context.Context) (string, error) {
	u, err := resolveSameOriginURL("/swagger.v1.json")
	if err != nil {
		return "", err
	}
	resp, err := anonymousGet(ctx, u)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GET %s: status %s", u, resp.Status)
	}
	dec := json.NewDecoder(io.LimitReader(resp.Body, maxSwaggerBody))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return "", fmt.Errorf("GET %s: not a JSON object", u)
	}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return "", fmt.Errorf("GET %s: %w", u, err)
		}
		key, _ := tok.(string)
		if key != "info" {
			var skip json.RawMessage
			if err := dec.Decode(&skip); err != nil {
				return "", fmt.Errorf("GET %s: %w", u, err)
			}
			continue
		}
		var info struct {
			Version string `json:"version"`
		}
		if err := dec.Decode(&info); err != nil {
			return "", fmt.Errorf("GET %s: info: %w", u, err)
		}
		if info.Version == "" {
			return "", fmt.Errorf("GET %s: info carries no version", u)
		}
		return info.Version, nil
	}
	return "", fmt.Errorf("GET %s: no info object", u)
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

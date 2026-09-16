// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo

import (
	"context"
	"encoding/json"
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

// ProbeServerVersion asks the configured Forgejo instance for its version
// without sending any credential. /api/v1/version needs none, and in
// resource-server mode there is no credential to send before a caller arrives.
func ProbeServerVersion(ctx context.Context) (string, error) {
	u, err := resolveURL("/version")
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", fmt.Errorf("build version request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent())
	req.Header.Set("Accept", "application/json")

	resp, err := rawHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("GET %s: %w", u, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
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

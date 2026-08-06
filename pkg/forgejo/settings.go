// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo

import (
	"context"
	"sync"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v2/pkg/flag"
)

// settingsCacheEntry holds a cached, resolved pagination ceiling for one
// instance. ok is false when the ceiling could not be determined (endpoint
// unreachable, 403, old server, ...); callers must treat that as "unknown",
// never as a ceiling of zero.
type settingsCacheEntry struct {
	maxResponseItems int
	ok               bool
}

var (
	settingsCacheMu sync.Mutex
	// settingsCache is keyed on the instance base URL (flag.URL), not on any
	// particular *forgejo.Client value. Client(ctx) hands out a fresh
	// ephemeral client whenever a token is present in the context, so keying
	// on the client would defeat caching entirely.
	settingsCache = map[string]settingsCacheEntry{}
)

// MaxResponseItems returns the instance's configured max_response_items
// pagination ceiling (GET /api/v1/settings/api), fetching and caching it on
// first use. ok is false when the ceiling is unknown — e.g. the endpoint
// errored, returned a non-2xx status (older servers, or instances that
// restrict the settings endpoint), or is otherwise unreachable. Callers must
// treat ok == false as "unknown", not as a ceiling of zero.
func MaxResponseItems(ctx context.Context) (max int, ok bool) {
	instance := flag.URL

	settingsCacheMu.Lock()
	if entry, found := settingsCache[instance]; found {
		settingsCacheMu.Unlock()
		return entry.maxResponseItems, entry.ok
	}
	settingsCacheMu.Unlock()

	entry := fetchMaxResponseItems(ctx)

	settingsCacheMu.Lock()
	settingsCache[instance] = entry
	settingsCacheMu.Unlock()

	return entry.maxResponseItems, entry.ok
}

func fetchMaxResponseItems(ctx context.Context) settingsCacheEntry {
	client, err := Client(ctx)
	if err != nil {
		return settingsCacheEntry{ok: false}
	}

	settings, resp, err := client.GetGlobalAPISettings()
	if err != nil {
		return settingsCacheEntry{ok: false}
	}
	if resp != nil && (resp.StatusCode < 200 || resp.StatusCode >= 300) {
		return settingsCacheEntry{ok: false}
	}
	if settings == nil {
		return settingsCacheEntry{ok: false}
	}

	return settingsCacheEntry{maxResponseItems: settings.MaxResponseItems, ok: true}
}

// resetSettingsCacheForTesting clears the cached pagination ceiling for all
// instances. Exported via SetClientForTesting in testing.go so tests do not
// leak a cached ceiling from one instance/test into the next.
func resetSettingsCacheForTesting() {
	settingsCacheMu.Lock()
	defer settingsCacheMu.Unlock()
	settingsCache = map[string]settingsCacheEntry{}
}

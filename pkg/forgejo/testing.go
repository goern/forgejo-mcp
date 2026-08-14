package forgejo

import (
	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// SetClientForTesting overrides the singleton client for testing purposes.
// It also resets the cached instance pagination ceiling (MaxResponseItems),
// so a ceiling cached against one test's httptest server never leaks into
// the next test.
func SetClientForTesting(c *forgejo_sdk.Client) {
	clientMu.Lock()
	client = c
	clientMu.Unlock()
	resetSettingsCacheForTesting()
}

// ResetClientForTesting clears the singleton so the next Client call rebuilds
// it from the current flag values. Tests that assert on client-construction
// failure need this: without it they only pass when an earlier test happened
// to populate the singleton, which makes them order-dependent under
// -shuffle=on.
func ResetClientForTesting() {
	SetClientForTesting(nil)
}

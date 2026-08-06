package forgejo

import (
	"sync"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// SetClientForTesting overrides the singleton client for testing purposes.
// It also resets the cached instance pagination ceiling (MaxResponseItems),
// so a ceiling cached against one test's httptest server never leaks into
// the next test.
func SetClientForTesting(c *forgejo_sdk.Client) {
	clientOnce = sync.Once{}
	client = c
	clientOnce.Do(func() {}) // mark as initialized
	resetSettingsCacheForTesting()
}

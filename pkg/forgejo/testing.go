package forgejo

import (
	"sync"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// SetClientForTesting overrides the singleton client for testing purposes.
func SetClientForTesting(c *forgejo_sdk.Client) {
	clientOnce = sync.Once{}
	client = c
	clientOnce.Do(func() {}) // mark as initialized
}

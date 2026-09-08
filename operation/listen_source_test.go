// SPDX-License-Identifier: GPL-3.0-or-later

package operation

import (
	"os"
	"testing"
)

// mustReadSource reads a source file in this package, for the two tests that
// assert on the shape of the code rather than on its behaviour. Both guard
// properties no behavioural test can see: that no transport binds its own
// listener, and that the conformance table covers every transport.
func mustReadSource(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(b)
}

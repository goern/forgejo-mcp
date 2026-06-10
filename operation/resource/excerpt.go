// SPDX-License-Identifier: GPL-3.0-or-later

package resource

import "unicode/utf8"

// Excerpt truncates s to at most max bytes without splitting a UTF-8 rune,
// appending an ellipsis when truncation happened.
func Excerpt(s string, max int) string {
	if len(s) <= max {
		return s
	}
	cut := max
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + "…"
}

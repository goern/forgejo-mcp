// SPDX-License-Identifier: GPL-3.0-or-later

package resource

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestExcerptShortStringUnchanged(t *testing.T) {
	if got := Excerpt("hello", 200); got != "hello" {
		t.Fatalf("got %q", got)
	}
}

func TestExcerptTruncatesOnRuneBoundary(t *testing.T) {
	// "é" is 2 bytes; place a multibyte rune straddling the cut point.
	s := strings.Repeat("a", 199) + "éxxxx"
	got := Excerpt(s, 200)
	if !utf8.ValidString(got) {
		t.Fatalf("excerpt is not valid UTF-8: %q", got)
	}
	if want := strings.Repeat("a", 199) + "…"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestExcerptExactLimitNoEllipsis(t *testing.T) {
	s := strings.Repeat("a", 200)
	if got := Excerpt(s, 200); got != s {
		t.Fatalf("got %q", got)
	}
}

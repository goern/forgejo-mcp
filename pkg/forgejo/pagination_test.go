// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo

import (
	"net/http"
	"testing"
)

func TestTotalCountPresent(t *testing.T) {
	h := http.Header{}
	h.Set(TotalCountHeader, "333")
	n, ok := TotalCount(h)
	if !ok || n != 333 {
		t.Fatalf("got n=%d ok=%v, want 333/true", n, ok)
	}
}

func TestTotalCountZeroIsStillPresent(t *testing.T) {
	h := http.Header{}
	h.Set(TotalCountHeader, "0")
	n, ok := TotalCount(h)
	if !ok || n != 0 {
		t.Fatalf("got n=%d ok=%v, want 0/true", n, ok)
	}
}

func TestTotalCountAbsent(t *testing.T) {
	for name, h := range map[string]http.Header{
		"nil header":       nil,
		"empty header":     {},
		"unrelated header": {"Content-Type": []string{"application/json"}},
	} {
		t.Run(name, func(t *testing.T) {
			n, ok := TotalCount(h)
			if ok || n != 0 {
				t.Fatalf("got n=%d ok=%v, want 0/false", n, ok)
			}
		})
	}
}

func TestTotalCountGarbage(t *testing.T) {
	for _, raw := range []string{"not-a-number", "-1", "1.5", "1e3", ""} {
		t.Run(raw, func(t *testing.T) {
			h := http.Header{}
			h.Set(TotalCountHeader, raw)
			n, ok := TotalCount(h)
			if ok || n != 0 {
				t.Fatalf("raw=%q: got n=%d ok=%v, want 0/false", raw, n, ok)
			}
		})
	}
}

func TestTotalCountPtrPresentAndAbsent(t *testing.T) {
	h := http.Header{}
	h.Set(TotalCountHeader, "42")
	if p := TotalCountPtr(h); p == nil || *p != 42 {
		t.Fatalf("expected pointer to 42, got %v", p)
	}

	h2 := http.Header{}
	if p := TotalCountPtr(h2); p != nil {
		t.Fatalf("expected nil pointer for absent header, got %v", *p)
	}

	h3 := http.Header{}
	h3.Set(TotalCountHeader, "garbage")
	if p := TotalCountPtr(h3); p != nil {
		t.Fatalf("expected nil pointer for garbage header, got %v", *p)
	}
}

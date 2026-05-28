package resource

import (
	"strings"
	"testing"
)

func TestBounded_UnderCap(t *testing.T) {
	items := []string{"a", "b", "c"}
	r := Bounded(items, 10, "list_tool")
	if r.Truncated {
		t.Fatal("expected not truncated")
	}
	if len(r.Items) != 3 {
		t.Errorf("expected 3 items, got %d", len(r.Items))
	}
	if r.Sentinel() != "" {
		t.Errorf("expected empty sentinel, got %q", r.Sentinel())
	}
}

func TestBounded_AtCap(t *testing.T) {
	items := make([]string, 30)
	for i := range items {
		items[i] = "x"
	}
	r := Bounded(items, 30, "list_tool")
	if r.Truncated {
		t.Fatal("expected not truncated at exact cap")
	}
	if r.Sentinel() != "" {
		t.Errorf("expected empty sentinel at cap, got %q", r.Sentinel())
	}
}

func TestBounded_OverCap_SentinelContents(t *testing.T) {
	items := make([]string, 35)
	for i := range items {
		items[i] = "x"
	}
	r := Bounded(items, 30, "list_issues")
	if !r.Truncated {
		t.Fatal("expected truncated")
	}
	if len(r.Items) != 30 {
		t.Errorf("expected 30 items, got %d", len(r.Items))
	}
	s := r.Sentinel()
	if s == "" {
		t.Fatal("expected non-empty sentinel")
	}
	if !strings.Contains(s, "list_issues") {
		t.Errorf("sentinel must name the list tool, got %q", s)
	}
	if !strings.Contains(s, "30 of 35") {
		t.Errorf("sentinel must include cap/total, got %q", s)
	}
}

package resource

import (
	"errors"
	"testing"
)

func TestParseBranchProtections(t *testing.T) {
	p, err := ParseBranchProtections("forgejo://repo/goern/forgejo-mcp/branch_protections")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Owner != "goern" || p.Repo != "forgejo-mcp" {
		t.Errorf("got %+v", p)
	}
}

func TestParseBranchProtections_Invalid(t *testing.T) {
	for _, uri := range []string{
		"forgejo://repo/goern/forgejo-mcp",                      // missing segment
		"forgejo://repo/goern/forgejo-mcp/branch_protection",    // singular, no rule
		"forgejo://repo/goern/forgejo-mcp/branch_protections/x", // extra segment
		"https://repo/goern/forgejo-mcp/branch_protections",     // wrong scheme
	} {
		if _, err := ParseBranchProtections(uri); !errors.Is(err, ErrInvalidParams) {
			t.Errorf("ParseBranchProtections(%q): expected ErrInvalidParams, got %v", uri, err)
		}
	}
}

func TestParseBranchProtection(t *testing.T) {
	p, err := ParseBranchProtection("forgejo://repo/goern/forgejo-mcp/branch_protection/main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Owner != "goern" || p.Repo != "forgejo-mcp" || p.Rule != "main" {
		t.Errorf("got %+v", p)
	}
}

func TestParseBranchProtection_GlobRuleWithSlash(t *testing.T) {
	p, err := ParseBranchProtection("forgejo://repo/goern/forgejo-mcp/branch_protection/release/v1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Rule != "release/v1" {
		t.Errorf("expected rule reassembled to 'release/v1', got %q", p.Rule)
	}
}

func TestParseBranchProtection_Invalid(t *testing.T) {
	for _, uri := range []string{
		"forgejo://repo/goern/forgejo-mcp/branch_protection",  // no rule
		"forgejo://repo/goern/forgejo-mcp/branch_protections", // plural collection
		"forgejo://owner/goern/branch_protection/main",        // wrong host
	} {
		if _, err := ParseBranchProtection(uri); !errors.Is(err, ErrInvalidParams) {
			t.Errorf("ParseBranchProtection(%q): expected ErrInvalidParams, got %v", uri, err)
		}
	}
}

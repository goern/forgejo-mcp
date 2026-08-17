package resource

import (
	"errors"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
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

// TestParseBranchProtection_GlobRuleWithSlash exercises the parser in isolation.
// It does NOT show that a raw '/' is usable from a client: a real read is matched
// against the URI template first, and that match rejects the raw slash long before
// this function is called. TestBranchProtectionRuleEncoding below covers the path
// an actual read takes; read the two together.
func TestParseBranchProtection_GlobRuleWithSlash(t *testing.T) {
	p, err := ParseBranchProtection("forgejo://repo/goern/forgejo-mcp/branch_protection/release/v1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Rule != "release/v1" {
		t.Errorf("expected rule reassembled to 'release/v1', got %q", p.Rule)
	}
}

// TestBranchProtectionRuleEncoding pins the encoding a caller must actually use.
// Branch protection rules are branch patterns, so slashes are the common case,
// and the two failure modes are silent in opposite directions: a raw '/' never
// matches the template (the read is rejected as not-found), while a double-encoded
// %252F matches and resolves to the literal text "%2F" instead of a slash.
func TestBranchProtectionRuleEncoding(t *testing.T) {
	re := mcp.NewResourceTemplate("forgejo://repo/{owner}/{repo}/branch_protection/{rule}", "x").URITemplate.Regexp()

	for _, tc := range []struct {
		uri        string
		wantMatch  bool
		wantRule   string
		wantParsed bool
	}{
		{"forgejo://repo/o/r/branch_protection/main", true, "main", true},
		{"forgejo://repo/o/r/branch_protection/release%2Fv1", true, "release/v1", true},
		{"forgejo://repo/o/r/branch_protection/needs%20space", true, "needs space", true},
		// Reaches the parser, but silently means the wrong rule.
		{"forgejo://repo/o/r/branch_protection/release%252Fv1", true, "release%2Fv1", true},
		// Never reaches the parser at all.
		{"forgejo://repo/o/r/branch_protection/release/v1", false, "", false},
		{"forgejo://repo/o/r/branch_protection/release/*", false, "", false},
		{"forgejo://repo/o/r/branch_protection/needs+space", false, "", false},
	} {
		if got := re.MatchString(tc.uri); got != tc.wantMatch {
			t.Errorf("template match for %q: got %v, want %v", tc.uri, got, tc.wantMatch)
		}
		if !tc.wantParsed {
			continue
		}
		p, err := ParseBranchProtection(tc.uri)
		if err != nil {
			t.Errorf("ParseBranchProtection(%q): unexpected error: %v", tc.uri, err)
			continue
		}
		if p.Rule != tc.wantRule {
			t.Errorf("ParseBranchProtection(%q): rule = %q, want %q", tc.uri, p.Rule, tc.wantRule)
		}
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

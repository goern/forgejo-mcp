package resource

import (
	"errors"
	"testing"
)

func TestParseOwner(t *testing.T) {
	p, err := ParseOwner("forgejo://owner/goern")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Owner != "goern" {
		t.Errorf("expected owner=goern, got %q", p.Owner)
	}
}

func TestParseOwner_WrongHost(t *testing.T) {
	_, err := ParseOwner("forgejo://repo/goern")
	if err == nil {
		t.Fatal("expected error for wrong host")
	}
}

func TestParseRepo(t *testing.T) {
	p, err := ParseRepo("forgejo://repo/goern/forgejo-mcp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Owner != "goern" || p.Repo != "forgejo-mcp" {
		t.Errorf("unexpected: %+v", p)
	}
}

func TestParseRepo_TooShort(t *testing.T) {
	_, err := ParseRepo("forgejo://repo/goern")
	if err == nil {
		t.Fatal("expected error for missing repo segment")
	}
}

func TestParseCommit_HappyPath(t *testing.T) {
	sha := "a3f1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9"
	uri := "forgejo://repo/goern/forgejo-mcp/commit/" + sha
	p, err := ParseCommit(uri)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.SHA != sha {
		t.Errorf("expected sha=%q, got %q", sha, p.SHA)
	}
}

func TestParseCommit_ShortSHA(t *testing.T) {
	_, err := ParseCommit("forgejo://repo/goern/forgejo-mcp/commit/abc123")
	if err == nil {
		t.Fatal("expected error for short sha")
	}
	if !errors.Is(err, ErrInvalidParams) {
		t.Errorf("expected ErrInvalidParams, got %v", err)
	}
}

func TestParseIssue_HappyPath(t *testing.T) {
	p, err := ParseIssue("forgejo://repo/goern/forgejo-mcp/issue/42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Index != 42 {
		t.Errorf("expected index=42, got %d", p.Index)
	}
}

func TestParseIssue_NonNumericIndex(t *testing.T) {
	_, err := ParseIssue("forgejo://repo/goern/forgejo-mcp/issue/abc")
	if err == nil {
		t.Fatal("expected error for non-numeric index")
	}
	if !errors.Is(err, ErrInvalidParams) {
		t.Errorf("expected ErrInvalidParams, got %v", err)
	}
}

func TestParsePR_HappyPath(t *testing.T) {
	p, err := ParsePR("forgejo://repo/goern/forgejo-mcp/pr/7")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Index != 7 {
		t.Errorf("expected index=7, got %d", p.Index)
	}
}

func TestParseComment_HappyPath(t *testing.T) {
	p, err := ParseComment("forgejo://repo/goern/forgejo-mcp/issue/42/comment/99")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Kind != "issue" || p.Index != 42 || p.ID != 99 {
		t.Errorf("unexpected: %+v", p)
	}
}

func TestParseComment_PRKind(t *testing.T) {
	p, err := ParseComment("forgejo://repo/goern/forgejo-mcp/pr/7/comment/5")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Kind != "pr" {
		t.Errorf("expected kind=pr, got %q", p.Kind)
	}
}

func TestParseComment_UnknownKind(t *testing.T) {
	_, err := ParseComment("forgejo://repo/goern/forgejo-mcp/wiki/1/comment/5")
	if err == nil {
		t.Fatal("expected error for unknown kind")
	}
	if !errors.Is(err, ErrInvalidParams) {
		t.Errorf("expected ErrInvalidParams, got %v", err)
	}
}

func TestParseStatus_HappyPath(t *testing.T) {
	sha := "a3f1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9"
	p, err := ParseStatus("forgejo://repo/goern/forgejo-mcp/commit/" + sha + "/status")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.SHA != sha {
		t.Errorf("expected sha=%q, got %q", sha, p.SHA)
	}
}

func TestParseStatus_ShortSHA(t *testing.T) {
	_, err := ParseStatus("forgejo://repo/goern/forgejo-mcp/commit/abc/status")
	if err == nil {
		t.Fatal("expected error for short sha")
	}
}

// TestParseForgejoURI_EmptySegments verifies that URIs with empty or
// whitespace-only path segments are rejected by all entity parsers, not
// silently aliased to canonical forms.
func TestParseForgejoURI_EmptySegments(t *testing.T) {
	cases := []struct {
		name string
		uri  string
	}{
		{
			name: "double slash mid-path",
			uri:  "forgejo://repo/goern//forgejo-mcp",
		},
		{
			name: "trailing slash",
			uri:  "forgejo://repo/goern/forgejo-mcp/",
		},
		{
			name: "percent-encoded whitespace segment",
			uri:  "forgejo://repo/%20/forgejo-mcp",
		},
		{
			name: "empty owner (single slash after host)",
			uri:  "forgejo://owner/",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Use ParseRepo as a representative caller; the rejection happens
			// inside parseForgejoURI which all parsers invoke first.
			_, err := ParseRepo(tc.uri)
			if err == nil {
				t.Errorf("expected error for URI %q, got nil", tc.uri)
			}
		})
	}
}

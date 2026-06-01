package operation

import "testing"

func TestExtractToken(t *testing.T) {
	tests := []struct {
		name string
		auth string
		want string
	}{
		{"empty header", "", ""},
		{"token scheme", "token abc123", "abc123"},
		{"bearer scheme", "Bearer abc123", "abc123"},
		{"lowercase bearer", "bearer abc123", "abc123"},
		{"uppercase bearer", "BEARER abc123", "abc123"},
		{"uppercase token", "TOKEN abc123", "abc123"},
		{"mixed-case token", "ToKeN abc123", "abc123"},
		{"unrecognized scheme", "Basic abc123", ""},
		// Spec: bare tokens (no scheme prefix) MUST be rejected and treated
		// as if no Authorization header were present.
		{"bare token rejected", "abc123", ""},
		{"bare token with leading space rejected", " abc123", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractToken(tt.auth); got != tt.want {
				t.Errorf("extractToken(%q) = %q, want %q", tt.auth, got, tt.want)
			}
		})
	}
}

// TestExtractToken_CaseInsensitiveEquivalence asserts every case variant of a
// recognized scheme resolves to the identical token value (spec scenario
// "Scheme matching is case-insensitive").
func TestExtractToken_CaseInsensitiveEquivalence(t *testing.T) {
	variants := []string{"bearer abc123", "Bearer abc123", "BEARER abc123", "token abc123", "TOKEN abc123"}
	for _, v := range variants {
		if got := extractToken(v); got != "abc123" {
			t.Errorf("extractToken(%q) = %q, want %q", v, got, "abc123")
		}
	}
}

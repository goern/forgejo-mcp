// SPDX-License-Identifier: GPL-3.0-or-later

package cmd

import (
	stdflag "flag"
	"reflect"
	"testing"
)

// withFlagSet installs a parsed flag set for the precedence helpers to inspect,
// exactly as initFlags does, and restores whatever was there before.
func withFlagSet(t *testing.T, args []string) {
	t.Helper()
	prev := flagSet
	fs := stdflag.NewFlagSet("forgejo-mcp", stdflag.ContinueOnError)
	fs.String("host", "127.0.0.1", "")
	fs.String("allowed-hosts", "", "")
	fs.String("allowed-origins", "", "")
	fs.Bool("allow-operator-token-fallback", false, "")
	if err := fs.Parse(args); err != nil {
		t.Fatalf("parse %v: %v", args, err)
	}
	flagSet = fs
	t.Cleanup(func() { flagSet = prev })
}

func TestExplicitFlagBeatsTheEnvironment(t *testing.T) {
	// The bug this test exists for: the old code decided "was the flag set?" by
	// comparing it to its default value, which cannot tell "not set" from
	// "explicitly set to the default". An inherited environment variable could
	// therefore move the bind address off loopback while the command line
	// plainly said 127.0.0.1.
	withFlagSet(t, []string{"-host", "127.0.0.1"})
	t.Setenv("FORGEJO_MCP_HOST", "0.0.0.0")

	if got := resolveString("127.0.0.1", "FORGEJO_MCP_HOST", "host"); got != "127.0.0.1" {
		t.Fatalf("the environment overrode an explicit flag: got %q", got)
	}
}

func TestEnvironmentIsUsedWhenTheFlagIsAbsent(t *testing.T) {
	// The control: without this, a helper that always returned the flag would
	// pass the test above while breaking every environment-configured
	// deployment.
	withFlagSet(t, nil)
	t.Setenv("FORGEJO_MCP_HOST", "0.0.0.0")

	if got := resolveString("127.0.0.1", "FORGEJO_MCP_HOST", "host"); got != "0.0.0.0" {
		t.Fatalf("the environment was ignored when no flag was passed: got %q", got)
	}
}

func TestDefaultSurvivesWhenNeitherIsGiven(t *testing.T) {
	withFlagSet(t, nil)
	t.Setenv("FORGEJO_MCP_HOST", "")

	if got := resolveString("127.0.0.1", "FORGEJO_MCP_HOST", "host"); got != "127.0.0.1" {
		t.Fatalf("the default was lost: got %q", got)
	}
}

func TestExplicitNonDefaultFlagAlsoWins(t *testing.T) {
	withFlagSet(t, []string{"-host", "192.0.2.1"})
	t.Setenv("FORGEJO_MCP_HOST", "0.0.0.0")

	if got := resolveString("192.0.2.1", "FORGEJO_MCP_HOST", "host"); got != "192.0.2.1" {
		t.Fatalf("an explicit non-default flag was overridden: got %q", got)
	}
}

func TestFlagWasPassedDistinguishesDefaultFromUnset(t *testing.T) {
	withFlagSet(t, []string{"-host", "127.0.0.1"})
	if !flagWasPassed("host") {
		t.Error("a flag set explicitly to its default value read as unset")
	}
	if flagWasPassed("allowed-hosts") {
		t.Error("an unset flag read as passed")
	}
}

func TestSplitList(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{",", nil},
		{"  ,  ", nil},
		{"a.example.org", []string{"a.example.org"}},
		{" a.example.org , b.example.org ", []string{"a.example.org", "b.example.org"}},
		{"a.example.org,,b.example.org", []string{"a.example.org", "b.example.org"}},
	}
	for _, c := range cases {
		if got := splitList(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("splitList(%q) = %#v, want %#v", c.in, got, c.want)
		}
	}
}

func TestIsTruthy(t *testing.T) {
	for _, v := range []string{"1", "true", "TRUE", "yes", "on", " on "} {
		if !isTruthy(v) {
			t.Errorf("isTruthy(%q) = false", v)
		}
	}
	for _, v := range []string{"", "0", "false", "no", "off", "maybe"} {
		if isTruthy(v) {
			t.Errorf("isTruthy(%q) = true", v)
		}
	}
}

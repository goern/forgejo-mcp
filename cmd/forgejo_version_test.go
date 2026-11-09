// SPDX-License-Identifier: GPL-3.0-or-later

package cmd

import (
	stdflag "flag"
	"strings"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation"
)

// withForgejoVersionFlag parses args into a flag set holding -forgejo-version,
// as initFlags does, and restores the previous state afterwards. A nil args
// leaves flagSet nil, which is how --cli mode runs initConfig.
func withForgejoVersionFlag(t *testing.T, args []string) {
	t.Helper()
	prevSet, prevValue := flagSet, forgejoVersion
	t.Cleanup(func() { flagSet, forgejoVersion = prevSet, prevValue })
	forgejoVersion = ""
	if args == nil {
		flagSet = nil
		return
	}
	fs := stdflag.NewFlagSet("forgejo-mcp", stdflag.ContinueOnError)
	fs.StringVar(&forgejoVersion, "forgejo-version", "", "")
	if err := fs.Parse(args); err != nil {
		t.Fatalf("parse %v: %v", args, err)
	}
	flagSet = fs
}

// Spec forgejo-version-detection, scenario "Flag wins over environment".
func TestForgejoVersionFlagWinsOverEnvironment(t *testing.T) {
	t.Setenv("FORGEJO_VERSION", "15.0.11")
	withForgejoVersionFlag(t, []string{"-forgejo-version", "16.0.7"})

	got, err := resolveForgejoVersion()
	if err != nil || got != "16.0.7" {
		t.Fatalf("resolveForgejoVersion = %q, %v; want 16.0.7", got, err)
	}
}

func TestForgejoVersionEnvironmentAlone(t *testing.T) {
	t.Setenv("FORGEJO_VERSION", "16.0.7+gitea-1.22.0")
	withForgejoVersionFlag(t, []string{})

	got, err := resolveForgejoVersion()
	if err != nil || got != "16.0.7+gitea-1.22.0" {
		t.Fatalf("resolveForgejoVersion = %q, %v; want 16.0.7+gitea-1.22.0", got, err)
	}
}

// --cli runs initConfig without parsing a flag set.
func TestForgejoVersionCLIModeReadsEnvironment(t *testing.T) {
	t.Setenv("FORGEJO_VERSION", "16")
	withForgejoVersionFlag(t, nil)

	got, err := resolveForgejoVersion()
	if err != nil || got != "16" {
		t.Fatalf("resolveForgejoVersion = %q, %v; want 16", got, err)
	}
}

// Spec forgejo-version-detection, scenario "Unparsable value".
func TestForgejoVersionRefusesAnUnparsableValue(t *testing.T) {
	t.Setenv("FORGEJO_VERSION", "")
	for _, v := range []string{"banana", "v", ".16", "-1.0"} {
		withForgejoVersionFlag(t, []string{"-forgejo-version", v})

		_, err := resolveForgejoVersion()
		if err == nil {
			t.Fatalf("resolveForgejoVersion accepted %q", v)
		}
		if !strings.Contains(err.Error(), "-forgejo-version") {
			t.Fatalf("error %q does not name -forgejo-version", err)
		}
	}
}

// Spec forgejo-version-detection, scenario "Setting absent".
func TestForgejoVersionAbsent(t *testing.T) {
	t.Setenv("FORGEJO_VERSION", "")
	withForgejoVersionFlag(t, []string{})

	got, err := resolveForgejoVersion()
	if err != nil || got != "" {
		t.Fatalf("resolveForgejoVersion = %q, %v; want empty", got, err)
	}
}

// The setting is valid in every mode: it is not a resource-server-only
// setting, so passthrough mode does not refuse it.
func TestPassthroughAcceptsForgejoVersion(t *testing.T) {
	withAuthFlags(t, nil)
	t.Setenv("FORGEJO_VERSION", "16.0.7")
	for _, s := range resourceServerSettings() {
		if s.envVar == "FORGEJO_VERSION" || s.flagName == "forgejo-version" {
			t.Fatal("forgejo-version is listed as a resource-server-only setting")
		}
	}
	resolveAuthSettings()
	if err := operation.ValidateAuthConfig("http", false); err != nil {
		t.Fatalf("passthrough with FORGEJO_VERSION: %v", err)
	}
}

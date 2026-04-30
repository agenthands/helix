package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestUpdateCommandFlagParsing(t *testing.T) {
	cmd := newUpdateCommand()
	if cmd.Use != "update" {
		t.Errorf("Use = %q, want update", cmd.Use)
	}
	if !cmd.SilenceUsage {
		t.Errorf("SilenceUsage = false, want true")
	}
	if !cmd.SilenceErrors {
		t.Errorf("SilenceErrors = false, want true")
	}
	if cmd.Flags().Lookup("prerelease") == nil {
		t.Errorf("--prerelease flag missing")
	}
}

func TestUpgradeCommandFlagParsing(t *testing.T) {
	cmd := newUpgradeCommand()
	if cmd.Use != "upgrade" {
		t.Errorf("Use = %q, want upgrade", cmd.Use)
	}
	for _, f := range []string{"prerelease", "version", "check", "dry-run"} {
		if cmd.Flags().Lookup(f) == nil {
			t.Errorf("--%s flag missing", f)
		}
	}
	if !cmd.SilenceUsage || !cmd.SilenceErrors {
		t.Errorf("Silence{Usage,Errors} not both true")
	}
}

func TestUpgradeDaemonShortCircuit(t *testing.T) {
	t.Setenv("HELIX_RUNNING_AS_DAEMON", "1")

	cmd := newUpgradeCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(out.String(), "restart the daemon manually") {
		t.Errorf("output missing daemon hint: %q", out.String())
	}
}

func TestUpgradeRegistration(t *testing.T) {
	root := NewRootCommand()
	have := map[string]bool{}
	for _, c := range root.Commands() {
		have[c.Name()] = true
	}
	if !have["update"] {
		t.Errorf("update subcommand not registered")
	}
	if !have["upgrade"] {
		t.Errorf("upgrade subcommand not registered")
	}
}

func TestUpgradeFlagPrecedenceVersionWinsOverPrerelease(t *testing.T) {
	// Cobra-level: confirm both flags can be set together. The actual
	// precedence (Version wins) is implemented in upgrade.selectRelease;
	// this test asserts the cobra surface accepts the combination so
	// the package-level test in internal/upgrade can run.
	cmd := newUpgradeCommand()
	cmd.SetArgs([]string{"--prerelease", "--version", "v1.10.0", "--dry-run"})
	// Use ParseFlags directly so we don't need a stub network. We only
	// need to confirm parsing doesn't error.
	if err := cmd.ParseFlags([]string{"--prerelease", "--version", "v1.10.0", "--dry-run"}); err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}
	prerelease, _ := cmd.Flags().GetBool("prerelease")
	version, _ := cmd.Flags().GetString("version")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	if !prerelease {
		t.Errorf("prerelease = false, want true")
	}
	if version != "v1.10.0" {
		t.Errorf("version = %q, want v1.10.0", version)
	}
	if !dryRun {
		t.Errorf("dry-run = false, want true")
	}
}

func TestUpdateRegistration(t *testing.T) {
	root := NewRootCommand()
	for _, c := range root.Commands() {
		if c.Name() == "update" {
			if c.Flags().Lookup("prerelease") == nil {
				t.Errorf("registered update command missing --prerelease")
			}
			return
		}
	}
	t.Errorf("update subcommand not registered on root")
}

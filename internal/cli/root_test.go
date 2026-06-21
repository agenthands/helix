package cli

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// withRoutingSeams swaps the package-level daemon entry seam for a fake that
// only records whether it was called, then restores the original. This keeps the
// routing assertions hermetic — no real daemon spawn, no network bind.
//
// Phase 94 RETIRE-01: the stdio forwarder head was deleted, so there is no longer
// a forwarder seam to swap; the CLI's only routes are the daemon (--serve) and
// the no-arg help path.
func withRoutingSeams(t *testing.T) (daemonCalled *bool) {
	t.Helper()
	origDaemon := runDaemonFn
	t.Cleanup(func() {
		runDaemonFn = origDaemon
	})
	dmn := false
	runDaemonFn = func(cmd *cobra.Command) error {
		dmn = true
		return nil
	}
	return &dmn
}

// execRoot runs the root command with the given args against a captured
// stdout/stderr buffer and returns the error and combined output.
func execRoot(t *testing.T, args ...string) (error, string) {
	t.Helper()
	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return err, out.String()
}

// Test 1 (no-arg help): bare `helix` prints grouped help, exits 0 (nil error),
// and does NOT enter the forwarder (no stdio MCP session) or the daemon.
func TestRunRoot_NoArgPrintsGroupedHelpExitsZero(t *testing.T) {
	dmn := withRoutingSeams(t)

	err, out := execRoot(t)

	require.NoError(t, err, "no-arg helix must exit 0")
	assert.False(t, *dmn, "no-arg helix must NOT enter the daemon")
	// Grouped help output: cobra renders group titles as section headers, and
	// lists the registered subcommands.
	assert.Contains(t, out, "Usage:", "help output should contain a Usage section")
	assert.Contains(t, out, "setup", "help should list the setup command")
	assert.Contains(t, out, "status", "help should list the status command")
}

// Test 2 (explicit serve preserved): `--serve` still routes to the daemon.
func TestRunRoot_ServeRoutesToDaemon(t *testing.T) {
	dmn := withRoutingSeams(t)

	err, _ := execRoot(t, "--serve")

	require.NoError(t, err)
	assert.True(t, *dmn, "--serve must route to the daemon")
}

// Test 3 (version preserved): `--version` prints the version and exits 0,
// without entering the daemon.
func TestRunRoot_VersionPrintsAndExitsZero(t *testing.T) {
	dmn := withRoutingSeams(t)

	err, out := execRoot(t, "--version")

	require.NoError(t, err)
	assert.Contains(t, out, "helix version")
	assert.False(t, *dmn, "--version must not enter the daemon")
}

// Test 4 (legacy stdio rejected): Phase 94 RETIRE-01 deleted the stdio forwarder
// head, so `--mode=stdio` is no longer an accepted transport — it falls through
// to the unknown-mode error and must NOT route to the daemon.
func TestRunRoot_StdioModeRejected(t *testing.T) {
	dmn := withRoutingSeams(t)

	err, _ := execRoot(t, "--mode=stdio")

	require.Error(t, err, "stdio mode is deleted and must surface an error")
	assert.Contains(t, err.Error(), "unknown mode")
	assert.False(t, *dmn, "--mode=stdio must not enter the daemon")
}

// TestRunRoot_UnknownModeErrors: an unrecognized --mode still surfaces an error
// (the unknown-mode branch is preserved).
func TestRunRoot_UnknownModeErrors(t *testing.T) {
	withRoutingSeams(t)

	err, _ := execRoot(t, "--mode=bogus")

	require.Error(t, err, "unknown mode must surface an error")
	assert.Contains(t, err.Error(), "unknown mode")
}

// TestNewRootCommand_HasCommandGroups asserts the root command defines cobra
// command groups (AddGroup) and that every visible subcommand is assigned to a
// group, so the help screen is organized by capability. Phase 91 adds the
// generated verb groups on top of this scaffold.
func TestNewRootCommand_HasCommandGroups(t *testing.T) {
	cmd := NewRootCommand()

	groups := cmd.Groups()
	require.NotEmpty(t, groups, "root command must define command groups via AddGroup")

	groupIDs := make(map[string]bool, len(groups))
	for _, g := range groups {
		assert.NotEmpty(t, g.ID, "group ID must be non-empty")
		assert.NotEmpty(t, g.Title, "group title must be non-empty")
		groupIDs[g.ID] = true
	}

	// Every user-facing subcommand we register must be assigned to a declared
	// group (cobra renders ungrouped commands under "Additional Commands").
	wantGrouped := map[string]bool{
		"setup": true, "status": true, "activate": true,
		"deactivate": true, "nudge": true, "update": true, "upgrade": true,
	}
	for _, sub := range cmd.Commands() {
		if !wantGrouped[sub.Name()] {
			continue
		}
		assert.NotEmpty(t, sub.GroupID, "%s must be assigned to a command group", sub.Name())
		assert.True(t, groupIDs[sub.GroupID],
			"%s assigned to undeclared group %q", sub.Name(), sub.GroupID)
	}
}

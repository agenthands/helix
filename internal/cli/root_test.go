package cli

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// withRoutingSeams swaps the package-level forwarder/daemon entry seams for
// fakes that only record whether they were called, then restores the originals.
// This keeps the routing assertions hermetic — no real daemon spawn, no stdio
// MCP session, no network bind.
func withRoutingSeams(t *testing.T) (forwarderCalled, daemonCalled *bool) {
	t.Helper()
	origFwd := runForwarderFn
	origDaemon := runDaemonFn
	t.Cleanup(func() {
		runForwarderFn = origFwd
		runDaemonFn = origDaemon
	})
	fwd := false
	dmn := false
	runForwarderFn = func(cmd *cobra.Command) error {
		fwd = true
		return nil
	}
	runDaemonFn = func(cmd *cobra.Command) error {
		dmn = true
		return nil
	}
	return &fwd, &dmn
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
	fwd, dmn := withRoutingSeams(t)

	err, out := execRoot(t)

	require.NoError(t, err, "no-arg helix must exit 0")
	assert.False(t, *fwd, "no-arg helix must NOT enter the forwarder (no stdio MCP session)")
	assert.False(t, *dmn, "no-arg helix must NOT enter the daemon")
	// Grouped help output: cobra renders group titles as section headers, and
	// lists the registered subcommands.
	assert.Contains(t, out, "Usage:", "help output should contain a Usage section")
	assert.Contains(t, out, "setup", "help should list the setup command")
	assert.Contains(t, out, "status", "help should list the status command")
}

// Test 2 (explicit serve preserved): `--serve` still routes to the daemon.
func TestRunRoot_ServeRoutesToDaemon(t *testing.T) {
	fwd, dmn := withRoutingSeams(t)

	err, _ := execRoot(t, "--serve")

	require.NoError(t, err)
	assert.True(t, *dmn, "--serve must route to the daemon")
	assert.False(t, *fwd, "--serve must not enter the forwarder")
}

// Test 3 (explicit http preserved): `--mode=http` still routes to the daemon.
func TestRunRoot_HTTPModeRoutesToDaemon(t *testing.T) {
	fwd, dmn := withRoutingSeams(t)

	err, _ := execRoot(t, "--mode=http")

	require.NoError(t, err)
	assert.True(t, *dmn, "--mode=http must route to the daemon")
	assert.False(t, *fwd, "--mode=http must not enter the forwarder")
}

// Test 4 (explicit stdio preserved): `--mode=stdio` still routes to the
// forwarder. Phase 94 owns the full forwarder-head deletion; this plan only
// changes the no-arg/auto path.
func TestRunRoot_StdioModeRoutesToForwarder(t *testing.T) {
	fwd, dmn := withRoutingSeams(t)

	err, _ := execRoot(t, "--mode=stdio")

	require.NoError(t, err)
	assert.True(t, *fwd, "--mode=stdio must still route to the forwarder")
	assert.False(t, *dmn, "--mode=stdio must not enter the daemon")
}

// Test 5 (version preserved): `--version` prints the version and exits 0,
// without entering forwarder or daemon.
func TestRunRoot_VersionPrintsAndExitsZero(t *testing.T) {
	fwd, dmn := withRoutingSeams(t)

	err, out := execRoot(t, "--version")

	require.NoError(t, err)
	assert.Contains(t, out, "helix version")
	assert.False(t, *fwd, "--version must not enter the forwarder")
	assert.False(t, *dmn, "--version must not enter the daemon")
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

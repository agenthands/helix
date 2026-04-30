package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/agenthands/helix/internal/upgrade"
)

// newUpgradeCommand builds the install verb `helix upgrade`: download +
// minisign-verify the latest release archive, atomically swap the
// running binary in place, and re-launch with the same args minus the
// `upgrade` verb. Daemon-aware (D-08 short-circuit), permission-aware
// (D-09 sudo hint), downgrade-refusing (D-10).
//
// Flags shipped in v1.9 per D-12:
//   --prerelease       include rc/beta/alpha tags
//   --version vX.Y.Z   pin to a specific tag (wins over --prerelease)
//   --check            alias for `helix update` (read-only)
//   --dry-run          download + verify but skip swap+relaunch
func newUpgradeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Download, verify, and install a newer Helix release",
		Long: `Download the latest GitHub Release archive for the current
platform, verify its minisign signature against the embedded public key,
atomically swap the running binary, and re-launch with the same args.

Refuses to upgrade if:
  - Running inside a helix daemon (restart the daemon manually).
  - Install path is not writable (re-run with sudo).
  - Latest release is older than or equal to the running version
    (no --force-downgrade flag exists; users wanting an older version
    install manually from GitHub Releases).

Per D-12 flag precedence: --version always wins over --prerelease.`,
		RunE: runUpgrade,
		// Per 52-PATTERNS.md (internal/cli/setup.go:42-45) — every cobra
		// subcommand sets BOTH SilenceUsage and SilenceErrors. The typed
		// error taxonomy carries display semantics; cobra's default usage
		// dump is noise on every error path.
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.Flags().Bool("prerelease", false, "Include pre-release versions (rc/beta/alpha)")
	cmd.Flags().String("version", "", "Pin to specific version (overrides --prerelease)")
	cmd.Flags().Bool("check", false, "Check for updates without installing (alias for `helix update`)")
	cmd.Flags().Bool("dry-run", false, "Download + verify but do not swap or restart")
	return cmd
}

// runUpgrade implements the install flow. Order matters per the
// system-architecture diagram (RESEARCH.md):
//   1. RunningInDaemon short-circuit (D-08) — print manual hint, exit clean.
//   2. --check delegates to runUpdate (D-12).
//   3. Otherwise, hand off to upgrade.Upgrade with the user's flags.
func runUpgrade(cmd *cobra.Command, args []string) error {
	prerelease, _ := cmd.Flags().GetBool("prerelease")
	version, _ := cmd.Flags().GetString("version")
	check, _ := cmd.Flags().GetBool("check")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	// D-08: refuse if running inside daemon. The hint is printed via the
	// upgrade package's RunningInDaemon path inside upgrade.Upgrade, but
	// we also short-circuit here so --check (which routes to Update) is
	// blocked the same way.
	if upgrade.RunningInDaemon() {
		fmt.Fprintln(cmd.OutOrStdout(),
			"helix is running as a daemon child process; restart the daemon manually after upgrading from a non-daemon shell")
		return nil
	}

	upgrade.SetCurrentVersion(CurrentVersion())

	// --check (D-12): functionally identical to `helix update`. Delegate
	// directly so the output formatting is the same.
	if check {
		return upgrade.Update(cmd.Context(), upgrade.Options{
			Prerelease: prerelease,
			Current:    CurrentVersion(),
			Stdout:     cmd.OutOrStdout(),
		})
	}

	return upgrade.Upgrade(cmd.Context(), upgrade.Options{
		Prerelease: prerelease,
		Version:    version,
		DryRun:     dryRun,
		Current:    CurrentVersion(),
		Stdout:     cmd.OutOrStdout(),
	})
}

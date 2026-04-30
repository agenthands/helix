package cli

import (
	"github.com/spf13/cobra"

	"github.com/agenthands/helix/internal/upgrade"
)

// newUpdateCommand builds the read-only `helix update` subcommand. It
// queries the GitHub Releases API for the current release info, prints
// `current/latest/status` plus the release-notes body, and exits.
// Mutates nothing on disk and never makes network calls beyond the
// single API check (D-06: distinct from `upgrade` install verb).
func newUpdateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Check for a newer Helix release without installing",
		Long: `Read-only check against the GitHub Releases API.
Prints current and latest version plus release notes. No download, no swap.

Use 'helix upgrade' to actually install. The two verbs match the
'apt update' / 'apt upgrade' semantic split per CONTEXT.md D-06.`,
		RunE: runUpdate,
		// Per 52-PATTERNS.md analog (internal/cli/setup.go:42-45) — every
		// cobra subcommand sets BOTH SilenceUsage and SilenceErrors so the
		// typed-error taxonomy controls the error display path. Failing
		// to set both pollutes the user's terminal on every error.
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.Flags().Bool("prerelease", false, "Include pre-release versions (rc/beta/alpha)")
	return cmd
}

// runUpdate executes the read-only API check. Threads the binary's
// version (`cli.CurrentVersion()`) into upgrade.Update so the
// downgrade-vs-upgrade decision is correct on the user's machine.
func runUpdate(cmd *cobra.Command, _ []string) error {
	prerelease, _ := cmd.Flags().GetBool("prerelease")
	// Thread the running binary's version into the User-Agent header for
	// observability (so GitHub-side abuse-tracking can correlate requests
	// to a specific helix version).
	upgrade.SetCurrentVersion(CurrentVersion())
	return upgrade.Update(cmd.Context(), upgrade.Options{
		Prerelease: prerelease,
		Current:    CurrentVersion(),
		Stdout:     cmd.OutOrStdout(),
	})
}

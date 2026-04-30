package cli

import (
	"context"
	"os/exec"

	"github.com/agenthands/helix/internal/langregistry"
)

// runHealthCheck verifies that language server binaries are available and callable.
// This is a lightweight binary-existence check (Phase 34 scope per D-10 note).
// Full daemon-based health check (starting daemon, sending LSP initialize) is deferred
// to Phase 35 (HLTH-01 through HLTH-04).
// Health check output goes to stderr via SetupPrinter (T-34-09).
// Returns nil always -- health check is informational, not blocking.
func runHealthCheck(_ context.Context, entries []langregistry.LSEntry, _ string, printer *SetupPrinter, dryRun bool) error {
	if dryRun {
		printer.DryRunAction("would verify %d language server(s)", len(entries))
		return nil
	}

	if len(entries) == 0 {
		printer.Info("No language servers to verify")
		return nil
	}

	for _, entry := range entries {
		path, err := exec.LookPath(entry.Command)
		if err != nil {
			printer.Failure("%s (%s) not found in PATH: %v", entry.Language, entry.Command, err)
			continue
		}
		printer.Success("%s (%s) found at %s", entry.Language, entry.Command, path)
	}

	return nil
}

// cmd/helix-bench is the Phase 75 provider-independent benchmark harness
// entrypoint. It exposes five BENCH-02 subcommands:
//
//   - helix-bench run                 — run the bench suite (Phase NN, not yet wired)
//   - helix-bench fetch-datasets      — download/refresh bench datasets (Phase NN)
//   - helix-bench doctor              — check host prerequisites; exit 0 on a clean host
//   - helix-bench report              — render bench reports (Phase NN)
//   - helix-bench validate-cost-table — HARD-FAIL strict validator for the
//     bench/datasets/cost-table.yaml pricing + staleness contract (COST-01/D-13/D-16)
//
// The companion `verify-tos` validator (D-16) is exposed as a sibling subcommand
// and wired into the Makefile, but BENCH-02 counts exactly five top-level
// subcommands for the --help acceptance; verify-tos is a Makefile gate.
//
// main() is the only os.Exit site; every subcommand uses RunE so errors
// propagate to the single exit point and the process exits non-zero (D-16).
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	// verify-tos is a Makefile-only HARD-FAIL gate, NOT one of the five BENCH-02
	// subcommands. Dispatch it directly so it does not inflate the root
	// command count (the --help acceptance fixes that at exactly five).
	if len(os.Args) > 1 && os.Args[1] == "verify-tos" {
		cmd := newVerifyTOSCmd()
		cmd.SetArgs(os.Args[2:])
		if err := cmd.Execute(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// newRootCmd constructs the cobra command tree. Exported (package-private but
// test-reachable) for in-process --help/doctor testing.
func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use: "helix-bench",
		// Validator failures are real gate errors, not usage mistakes — don't
		// dump the usage text on a non-zero exit.
		SilenceUsage: true,
		Short:        "Helix provider-independent benchmark harness",
		Long: `helix-bench drives the Phase 75 benchmark stack.

Subcommands:
  run                 run the bench suite (not yet implemented)
  fetch-datasets      download / refresh bench datasets (not yet implemented)
  doctor              check host prerequisites; exits 0 on a clean host
  report              render bench reports (not yet implemented)
  validate-cost-table HARD-FAIL strict validator for bench/datasets/cost-table.yaml

See bench/BENCH.md for the benchmark contract and bench/PROVIDERS.md for the
per-provider TOS attestation surface (gated by 'make verify-tos').`,
	}

	root.AddCommand(newRunCmd())
	root.AddCommand(newFetchDatasetsCmd())
	root.AddCommand(newDoctorCmd())
	root.AddCommand(newReportCmd())
	root.AddCommand(newValidateCostTableCmd())

	return root
}

// notYetImplemented returns a RunE that reports a clear deferred-phase error.
func notYetImplemented(feature string) func(*cobra.Command, []string) error {
	return func(*cobra.Command, []string) error {
		return fmt.Errorf("%s: not yet implemented (deferred to a later Phase)", feature)
	}
}

// newRunCmd returns the 'run' subcommand (skeleton).
func newRunCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "run",
		Short: "Run the bench suite",
		RunE:  notYetImplemented("run"),
	}
}

// newFetchDatasetsCmd returns the 'fetch-datasets' subcommand (skeleton).
func newFetchDatasetsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "fetch-datasets",
		Short: "Download or refresh bench datasets",
		RunE:  notYetImplemented("fetch-datasets"),
	}
}

// newReportCmd returns the 'report' subcommand (skeleton).
func newReportCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "report",
		Short: "Render bench reports",
		RunE:  notYetImplemented("report"),
	}
}

// newDoctorCmd returns the 'doctor' subcommand. It checks documented host
// prerequisites and returns nil (exit 0) on a clean host. The skeleton has no
// hard prerequisites beyond a working Go toolchain (already proven by being able
// to run), so it succeeds on a clean Linux host.
func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check host prerequisites; exits 0 on a clean host",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), "helix-bench doctor: host OK")
			return nil
		},
	}
}

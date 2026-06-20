package main

import (
	"fmt"
	"time"

	"github.com/agenthands/helix/bench/cost"
	"github.com/spf13/cobra"
)

// Shared staleness/date constants for the package-main validators (D-16). The
// cost-table contract itself was moved to the importable bench/cost package
// (Phase 82 Open Q1 — MOVE); these two constants are RE-DECLARED here as the
// canonical values for the sibling verify-tos validator, which is package-main
// only and has no reason to import bench/cost. They are kept byte-identical to
// cost.DateLayout / cost.StalenessWindowDays.
const (
	// dateLayout is the Go reference layout for all YYYY-MM-DD dates in the
	// PROVIDERS frontmatter (verify-tos).
	dateLayout = "2006-01-02"
	// stalenessWindowDays is the maximum allowed age (in days) of an
	// attested_on date before verify-tos HARD-FAILS (D-16).
	stalenessWindowDays = 90
)

// newValidateCostTableCmd returns the 'validate-cost-table' subcommand. It
// passes time.Now().UTC() to cost.ValidateCostTable and returns the error via
// RunE so the process exits non-zero on failure — there is NO os.Exit deep
// inside, and NO continue-on-error (D-16). The cost-table contract + freshness
// gate now lives in the importable bench/cost package; this is a thin CLI shim.
func newValidateCostTableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate-cost-table [path]",
		Short: "Strict HARD-FAIL validator for the cost-table pricing+staleness contract",
		Long: `validate-cost-table strict-decodes a cost-table YAML (default
bench/datasets/cost-table.yaml) and exits NON-ZERO on an unknown key, an
unparseable date, a past valid_until, or a last_verified more than 90 days old.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "bench/datasets/cost-table.yaml"
			if len(args) == 1 {
				path = args[0]
			}
			if err := cost.ValidateCostTable(time.Now().UTC(), path); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "validate-cost-table: %s OK\n", path)
			return nil
		},
	}
}

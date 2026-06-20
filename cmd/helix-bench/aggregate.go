package main

import (
	"fmt"
	"time"

	aggregator "github.com/agenthands/helix/bench/aggregator"
	"github.com/spf13/cobra"
)

// aggregate.go is the D-02 operator entry point: a thin cobra subcommand that
// wraps the PURE bench/aggregator.Aggregate orchestrator. It constructs an
// aggregator.Config from flags, injects today = time.Now().UTC() for the cost
// freshness gate (never time.Now() deep inside the pure code, D-08/D-13), calls
// Aggregate(runDir, cfg), and propagates the error via RunE so the process
// exits NON-ZERO and writes NO reports on a deficiency (fail-closed,
// single-exit semantics — main() is the only os.Exit site).
//
// ExpectedN comes from --runs (the caller's expected run count / manifest
// value), NEVER the on-disk file count (Pitfall 3 / D-05): a disk count hides a
// partial matrix, so a missing run must surface as a deficiency, not be
// silently accepted.

// defaultAggregateSeed is the fixed bootstrap seed (D-08) so a bare
// `helix-bench aggregate <run_dir>` produces byte-identical reports across
// re-runs. An operator may override it with --seed for an alternate replicate.
const defaultAggregateSeed uint64 = 42

// aggregateCostTablePath is the pinned cost table the freshness-gated cost
// rollup joins against (COST-03). It mirrors validate-cost-table's default so
// the two CLI surfaces agree on the canonical pricing source.
const aggregateCostTablePath = "bench/datasets/cost-table.yaml"

// newAggregateCmd returns the 'aggregate' subcommand. It is the primary,
// re-runnable entry point that turns a durable multi-run tree
// (<run_dir>/<task>/<mode>/<run_index>/result.v2.json) into the first
// leaderboard.md + cost_quality.md. RunE returns the Aggregate error verbatim
// so a deficient run exits non-zero with no reports written.
func newAggregateCmd() *cobra.Command {
	var (
		runs      int
		seed      uint64
		iters     int
		ciLevel   float64
		costTable string
	)

	cmd := &cobra.Command{
		Use:   "aggregate <run_dir>",
		Short: "Aggregate a multi-run tree into leaderboard.md + cost_quality.md",
		Long: `aggregate reduces a durable multi-run benchmark tree into the first
leaderboard.md + cost_quality.md (STATS-01/COST-03).

It globs <run_dir>/<task>/<mode>/<run_index>/result.v2.json, enforces the
fail-closed N-gate (--runs, NEVER the disk count): if any (task,mode) cell has
fewer than --runs valid rows the command exits NON-ZERO and writes NO reports
(D-05). On a sufficient run it performs the two-level BCa reduction under a
seeded RNG (byte-deterministic, D-08) and writes both reports into <run_dir>.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runDir := args[0]
			cfg := aggregator.Config{
				ExpectedN:     runs,
				Seed:          seed,
				Iterations:    iters,
				CILevel:       ciLevel,
				CostTablePath: costTable,
				// Injected freshness clock (never time.Now() deep inside the
				// pure aggregator). The cost rollup gates pricing rows against
				// this; a stale/unknown row degrades to a soft em-dash per cell.
				// A cost-table LOAD failure, by contrast, fails CLOSED (WR-03).
				Today: time.Now().UTC(),
			}
			if _, err := aggregator.Aggregate(runDir, cfg); err != nil {
				// Fail-closed: propagate the deficiency to the single exit point
				// (non-zero) — Aggregate has already written nothing.
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "aggregate: wrote %s/leaderboard.md\n", runDir)
			fmt.Fprintf(cmd.OutOrStdout(), "aggregate: wrote %s/cost_quality.md\n", runDir)
			return nil
		},
	}

	// --runs is the EXPECTED N for the fail-closed gate (D-05), not the disk
	// count. It defaults to 3 (the standard STATS-01 minimum).
	cmd.Flags().IntVar(&runs, "runs", 3, "expected runs per (task,mode); fail-closed if any cell has fewer (NEVER the disk count)")
	cmd.Flags().Uint64Var(&seed, "seed", defaultAggregateSeed, "bootstrap RNG seed (fixed for byte-deterministic reports, D-08)")
	cmd.Flags().IntVar(&iters, "iterations", 10000, "BCa bootstrap replicate count (floored to 10000)")
	cmd.Flags().Float64Var(&ciLevel, "ci-level", 0.95, "confidence level for the BCa intervals")
	// --cost-table is the pinned pricing source for the cost rollup. A LOAD
	// failure (missing/unparseable/empty) fails CLOSED (non-zero exit, no reports)
	// per WR-03; a per-row pricing gap (unknown model_id) stays a soft em-dash.
	cmd.Flags().StringVar(&costTable, "cost-table", aggregateCostTablePath, "cost-table YAML path (LOAD failure fails closed, WR-03)")

	return cmd
}

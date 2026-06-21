package main

import (
	"fmt"
	"path/filepath"
	"time"

	aggregator "github.com/agenthands/helix/bench/aggregator"
	"github.com/spf13/cobra"
)

// report.go is the Phase 89 (REPORT-05) operator entry point: a thin cobra
// subcommand that regenerates ALL 4 reports (leaderboard.md + cost_quality.md +
// per_language.md + ablations.md) for an existing durable run tree resolved by
// --run-id. It MIRRORS newAggregateCmd's discipline EXACTLY — same Config flag
// defaults, the same injected `today = time.Now().UTC()` cost-freshness clock at
// the CLI boundary, and verbatim error propagation so a deficient/under-populated
// run exits NON-ZERO and writes NO reports (fail-closed, single-exit semantics).
//
// The ONLY structural difference from `aggregate <run_dir>` is the input shape:
// `report` takes --run-id <id> + --out <root> and resolves runDir =
// filepath.Join(out, id), where `aggregate` takes the runDir positionally. Both
// then funnel through aggregator.Aggregate -> the shared renderAll, so report and
// aggregate produce BYTE-IDENTICAL reports over the same tree (REPORT-05).
//
// run-id is VALIDATED as a single [A-Za-z0-9_-]+ path segment (isValidRunID,
// cloned from bench/evaluators/swebench/harness.go — it is package-private there)
// BEFORE any filepath.Join, so a "../etc"/absolute/leading-'-'/empty value can
// never escape the report tree (T-89-03-01 path-traversal mitigation, V5).

// reportOutDefault is the durable report root --run-id resolves against. It mirrors
// the `run` subcommand's --out default (bench/reports) so `report --run-id <id>`
// regenerates the reports for the tree `run --run-id <id>` wrote.
const reportOutDefault = "bench/reports"

// isValidRunID reports whether s is a non-empty [A-Za-z0-9_-]+ that does NOT start
// with '-'. It is CLONED verbatim from bench/evaluators/swebench/harness.go (that
// copy is package-private to swebench) rather than imported — the leading-'-'
// refusal closes the flag-smuggling vector and the explicit, total charset rejects
// '/' / '.' / whitespace so a run-id can never be a multi-segment or traversal path.
func isValidRunID(s string) bool {
	if s == "" || s[0] == '-' {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z':
		case c >= 'A' && c <= 'Z':
		case c >= '0' && c <= '9':
		case c == '_' || c == '-':
		default:
			return false
		}
	}
	return true
}

// newReportCmd returns the 'report' subcommand. It regenerates all 4 reports for an
// existing durable run tree resolved by --run-id, calling the SAME aggregator render
// path `aggregate` uses (REPORT-05 byte-identity). RunE returns the Aggregate error
// verbatim so a deficient run exits non-zero with no reports written.
func newReportCmd() *cobra.Command {
	var (
		runID     string
		out       string
		runs      int
		seed      uint64
		iters     int
		ciLevel   float64
		costTable string
	)

	cmd := &cobra.Command{
		Use:   "report --run-id <id>",
		Short: "Regenerate leaderboard/per_language/ablations/cost_quality reports for a run tree",
		Long: `report regenerates ALL 4 markdown reports (leaderboard.md, per_language.md,
ablations.md, cost_quality.md) for an existing durable multi-run tree resolved by
--run-id under --out (default bench/reports), i.e. <out>/<run-id>/.

It funnels through the SAME aggregator render path as 'aggregate', so report and
aggregate produce byte-identical reports over the same tree (REPORT-05). --run-id
is validated as a single [A-Za-z0-9_-]+ path segment before any path join, so a
'..'/absolute/leading-'-' value can never escape the report tree. A deficient run
(any cell with fewer than --runs valid rows) or a cost-table LOAD failure exits
NON-ZERO and writes NO reports (fail-closed, mirroring aggregate).`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// V5 input validation BEFORE filepath.Join (T-89-03-01): reject a run-id
			// that is empty, leading-'-', or carries any char outside [A-Za-z0-9_-],
			// so it is always a single, traversal-free path segment.
			if !isValidRunID(runID) {
				return fmt.Errorf("report: invalid --run-id %q: must be a non-empty [A-Za-z0-9_-]+ segment (no '..'/'/'/leading '-')", runID)
			}
			runDir := filepath.Join(out, runID)

			cfg := aggregator.Config{
				ExpectedN:     runs,
				Seed:          seed,
				Iterations:    iters,
				CILevel:       ciLevel,
				CostTablePath: costTable,
				// Injected freshness clock at the CLI boundary (never time.Now() deep
				// inside the pure aggregator) — mirrors newAggregateCmd exactly.
				Today: time.Now().UTC(),
			}
			if _, err := aggregator.Aggregate(runDir, cfg); err != nil {
				// Fail-closed: propagate the deficiency to the single exit point
				// (non-zero) — Aggregate has already written nothing.
				return err
			}
			for _, name := range []string{"leaderboard.md", "per_language.md", "ablations.md", "cost_quality.md"} {
				fmt.Fprintf(cmd.OutOrStdout(), "report: wrote %s\n", filepath.Join(runDir, name))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&runID, "run-id", "", "run identifier (single [A-Za-z0-9_-]+ segment); resolves <out>/<run-id>/")
	cmd.Flags().StringVar(&out, "out", reportOutDefault, "durable report root that --run-id resolves against")
	// The reduction flags mirror newAggregateCmd EXACTLY so report and aggregate
	// agree on N-gate, seed, iterations, CI level, and cost-table source.
	cmd.Flags().IntVar(&runs, "runs", 3, "expected runs per (task,mode); fail-closed if any cell has fewer (NEVER the disk count)")
	cmd.Flags().Uint64Var(&seed, "seed", defaultAggregateSeed, "bootstrap RNG seed (fixed for byte-deterministic reports, D-08)")
	cmd.Flags().IntVar(&iters, "iterations", 10000, "BCa bootstrap replicate count (floored to 10000)")
	cmd.Flags().Float64Var(&ciLevel, "ci-level", 0.95, "confidence level for the BCa intervals")
	cmd.Flags().StringVar(&costTable, "cost-table", aggregateCostTablePath, "cost-table YAML path (LOAD failure fails closed, WR-03)")

	return cmd
}

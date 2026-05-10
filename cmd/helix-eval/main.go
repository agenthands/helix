// cmd/helix-eval is the Phase 67 evaluation harness entrypoint.
// It exposes two subcommands:
//
//   - helix-eval run     — dispatch tasks across modes, collect results
//   - helix-eval validate-rules — validate expected_tools.yaml rule files
//
// The 'run' subcommand:
//  1. Enforces the EVAL-06 ZDR corpus gate via runner.AssertCorpusAllowed.
//  2. Runs all (corpus task × mode) pairs via runner.RunMatrix.
//  3. Emits all 6 EVAL-04 report files under <out>/<run-id>/:
//       eval_report.json, eval_report.md, cost_summary.json,
//       tool_behavior.json, safety_compliance.json, run_metadata.json.
//  4. Exits 0 if all tasks succeeded; exits 1 if any task failed.
//     Judge failure does NOT affect exit code (EVAL-07).
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/agenthands/helix/internal/eval/judge"
	"github.com/agenthands/helix/internal/eval/report"
	"github.com/agenthands/helix/internal/eval/runner"
	"github.com/agenthands/helix/internal/eval/score"
	"github.com/spf13/cobra"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// newRootCmd constructs the cobra command tree. Exported for testing.
func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "helix-eval",
		Short: "Helix evaluation harness",
		Long: `helix-eval runs the Phase 67 evaluation harness.

Use 'helix-eval run' for the full eval matrix or 'helix-eval run --quick' for
the in-process scripted-agent harness validation (<30s wall-time).

See eval/EVAL.md for TOS attestation, retention policy, and the distinction
between eval-quick (harness validation) and eval (real agent behavior).`,
	}

	root.AddCommand(newRunCmd())
	root.AddCommand(newValidateRulesCmd())

	return root
}

// newRunCmd returns the 'run' subcommand.
func newRunCmd() *cobra.Command {
	var (
		corpus     string
		modes      []string
		runID      string
		quick      bool
		judgeModel string
		noJudge    bool
		out        string
		helixBin   string
	)

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run the evaluation matrix",
		Long: `Run the evaluation harness over the corpus for the specified modes.

Without --quick, runs per-(task, mode) pairs through the phasegraph pipeline.
With --quick, stubs the agent (harness-wiring validation only, <30s wall-time).

Note: --quick does NOT measure real Claude Code agent behavior. Use 'make eval'
for behavior measurements (see eval/EVAL.md).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCommand(cmd, corpus, modes, runID, out, helixBin, quick, judgeModel, noJudge)
		},
	}

	cmd.Flags().StringVar(&corpus, "corpus", "eval/corpus", "path to corpus directory")
	cmd.Flags().StringArrayVar(&modes, "mode", []string{"baseline", "native", "semantic", "semantic_guarded"}, "eval mode(s); repeatable")
	cmd.Flags().StringVar(&runID, "run-id", "", "run identifier (default: ISO-8601 timestamp)")
	cmd.Flags().BoolVar(&quick, "quick", false, "use in-process scripted agent (harness validation only, <30s)")
	cmd.Flags().StringVar(&judgeModel, "judge-model", "claude-sonnet-4-6", "LLM judge model (informational only)")
	cmd.Flags().BoolVar(&noJudge, "no-judge", true, "skip the informational LLM judge pass (default: skip)")
	cmd.Flags().StringVar(&out, "out", "eval/reports", "output directory for reports")
	cmd.Flags().StringVar(&helixBin, "helix-bin", "helix", "path to the helix binary for daemon subprocess")

	return cmd
}

// runCommand implements the 'run' subcommand body.
func runCommand(cmd *cobra.Command, corpus string, modes []string, runID, out, helixBin string, quick bool, judgeModel string, noJudge bool) error {
	ctx := cmd.Context()

	// Default run-id from timestamp.
	if runID == "" {
		runID = time.Now().UTC().Format("20060102T150405Z")
	}

	// Step 0: capture run metadata before anything else.
	meta := report.CaptureRunMetadata(modes, corpus)
	meta.RunID = runID
	meta.StartedAt = time.Now().UTC()

	// Create run output directory.
	runOutDir := filepath.Join(out, runID)
	if err := os.MkdirAll(runOutDir, 0700); err != nil {
		return fmt.Errorf("helix-eval run: mkdir run output dir: %w", err)
	}

	// Step 1: enforce EVAL-06 ZDR gate.
	if err := runner.AssertCorpusAllowed(corpus); err != nil {
		return fmt.Errorf("helix-eval run: ZDR gate: %w", err)
	}

	// Resolve judge model. Default judgeModel flag is "claude-sonnet-4-6" (full model ID)
	// but also accept short aliases "sonnet" / "opus" via ResolveModel.
	resolvedModel := judgeModel
	if m, err := judge.ResolveModel(judgeModel); err == nil {
		resolvedModel = m
	}
	// If the full model ID was passed directly (e.g. "claude-sonnet-4-6"),
	// keep it as-is — ResolveModel only knows short aliases.
	_ = resolvedModel

	// Step 2: run the matrix.
	if quick {
		// In-process scripted-agent path (EVAL-03, Plan 67-06a).
		// Pitfall-6 banner is emitted inside RunQuick.
		qOpts := runner.QuickOpts{
			FixturesDir:  corpus,
			Modes:        modes,
			OutDir:       out,
			RunID:        runID,
			AllowSuccess: true, // --quick flag opens the success gate
		}
		qSummary, err := runner.RunQuick(ctx, qOpts)
		if err != nil {
			return fmt.Errorf("helix-eval run --quick: %w", err)
		}
		// Step 5: print summary.
		total := qSummary.TotalResults
		succeeded := 0
		for _, r := range qSummary.Results {
			if r.Success {
				succeeded++
			}
		}
		fmt.Fprintf(cmd.OutOrStdout(), "helix-eval quick complete: %d/%d tasks succeeded\n", succeeded, total)
		fmt.Fprintf(cmd.OutOrStdout(), "Reports written to: %s\n", filepath.Join(out, runID))
		if succeeded < total {
			os.Exit(1)
		}
		return nil
	}

	r := runner.NewRunner(runner.Config{
		CorpusDir:   corpus,
		OutDir:      out,
		HelixBin:    helixBin,
		RunID:       runID,
		MaxParallel: 1,
	})

	results, _ := r.RunMatrix(ctx, modes)
	// RunMatrix errors are non-fatal (individual task errors captured in results).

	// Judge pass (informational only — NEVER affects exit code per EVAL-07).
	// The judge runs post-aggregate on the full result set. It is explicitly
	// excluded from the eval-quick path (judge is local-only, per project memory).
	judgeReportPath := filepath.Join(runOutDir, "tool_behavior_judge.json")
	{
		apiKey := os.Getenv("ANTHROPIC_API_KEY")
		jClient := judge.NewClient(judge.Options{APIKey: apiKey})
		// Build judge inputs from results (trace events not available at this aggregation
		// layer; judge sees task IDs and modes for prompt construction from trace files).
		var judgeInputs []judge.Input
		for _, res := range results {
			judgeInputs = append(judgeInputs, judge.Input{
				TaskID:          res.TaskID,
				Mode:            res.Mode,
				TaskKind:        "unknown", // enriched from corpus in future phases
				TaskDescription: res.TaskID,
			})
		}
		// EVAL-07: Run returns Output only (no error) — judge failures cannot propagate.
		judgeOut := judge.RunWithOptions(ctx, jClient, judgeInputs, resolvedModel, judge.RunOptions{
			NoJudge: noJudge,
		})
		_ = judge.WriteJudgeReport(judgeReportPath, judgeOut)
		// WriteJudgeReport error is intentionally ignored — judge output is informational.
	}

	// Step 3: build score map (empty for now — per-task scores written by RunTask).
	scores := make(map[string]score.Score)

	meta.EndedAt = time.Now().UTC()

	// Step 4: write all 6 EVAL-04 report files.
	var writeErrors []string

	if err := report.WriteRunMetadata(filepath.Join(runOutDir, "run_metadata.json"), meta); err != nil {
		writeErrors = append(writeErrors, err.Error())
	}
	if err := report.WriteCostSummary(filepath.Join(runOutDir, "cost_summary.json"), results); err != nil {
		writeErrors = append(writeErrors, err.Error())
	}
	if err := report.WriteSafetyCompliance(filepath.Join(runOutDir, "safety_compliance.json"), results); err != nil {
		writeErrors = append(writeErrors, err.Error())
	}
	if err := report.WriteToolBehavior(filepath.Join(runOutDir, "tool_behavior.json"), scores); err != nil {
		writeErrors = append(writeErrors, err.Error())
	}
	if err := report.WriteEvalReportWithJudge(
		filepath.Join(runOutDir, "eval_report.json"),
		filepath.Join(runOutDir, "eval_report.md"),
		judgeReportPath,
		results, scores, meta,
	); err != nil {
		writeErrors = append(writeErrors, err.Error())
	}

	if len(writeErrors) > 0 {
		return fmt.Errorf("helix-eval run: report write errors:\n  %s", strings.Join(writeErrors, "\n  "))
	}

	// Step 5: print summary.
	total := len(results)
	succeeded := 0
	for _, r := range results {
		if r.Success {
			succeeded++
		}
	}
	fmt.Fprintf(cmd.OutOrStdout(), "helix-eval run complete: %d/%d tasks succeeded\n", succeeded, total)
	fmt.Fprintf(cmd.OutOrStdout(), "Reports written to: %s\n", runOutDir)

	// Step 6: exit code. Judge failure does NOT affect exit code (EVAL-07).
	if succeeded < total {
		os.Exit(1)
	}

	return nil
}

// newValidateRulesCmd returns the 'validate-rules' subcommand.
func newValidateRulesCmd() *cobra.Command {
	var corpus string

	cmd := &cobra.Command{
		Use:   "validate-rules",
		Short: "Validate expected_tools.yaml rule files in corpus",
		Long: `validate-rules parses every expected_tools.yaml in the corpus directory
and reports schema errors. Use this before committing new corpus tasks to ensure
the heuristic rule DSL is well-formed.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			errs := score.ValidateCorpus(corpus)
			if len(errs) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "All expected_tools.yaml files are valid.")
				return nil
			}
			var msgs []string
			for _, e := range errs {
				msgs = append(msgs, e.Error())
			}
			return fmt.Errorf("validate-rules found %d error(s):\n  %s", len(errs), strings.Join(msgs, "\n  "))
		},
	}

	cmd.Flags().StringVar(&corpus, "corpus", "eval/corpus", "path to corpus directory")

	return cmd
}

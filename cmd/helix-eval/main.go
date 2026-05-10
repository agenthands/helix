// cmd/helix-eval is the Phase 67 evaluation harness entrypoint.
// It exposes two subcommands:
//
//   - helix-eval run     — dispatch tasks across modes, collect results
//   - helix-eval validate-rules — validate expected_tools.yaml rule files
//
// Both subcommands return "not yet implemented" until Wave 1+ lands real bodies.
// The binary compiles and --help works from Wave 0 onward.
package main

import (
	"fmt"
	"os"

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
	)

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run the evaluation matrix",
		Long: `Run the evaluation harness over the corpus for the specified modes.

Without --quick, spawns out-of-process daemon + claude CLI subprocesses per
(task, mode) pair. With --quick, uses an in-process scripted agent for fast
harness-wiring validation.

Note: --quick does NOT measure real Claude Code agent behavior. Use 'make eval'
for behavior measurements (see eval/EVAL.md).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = corpus
			_ = modes
			_ = runID
			_ = quick
			_ = judgeModel
			_ = noJudge
			_ = out
			return fmt.Errorf("not yet implemented (Phase 67 Wave 1+)")
		},
	}

	cmd.Flags().StringVar(&corpus, "corpus", "eval/corpus", "path to corpus directory")
	cmd.Flags().StringArrayVar(&modes, "mode", []string{"baseline", "native", "semantic", "semantic_guarded"}, "eval mode(s); repeatable")
	cmd.Flags().StringVar(&runID, "run-id", "", "run identifier (default: ISO-8601 timestamp + git SHA)")
	cmd.Flags().BoolVar(&quick, "quick", false, "use in-process scripted agent (harness validation only, <30s)")
	cmd.Flags().StringVar(&judgeModel, "judge-model", "claude-sonnet-4-6", "LLM judge model (informational only)")
	cmd.Flags().BoolVar(&noJudge, "no-judge", false, "skip the informational LLM judge pass")
	cmd.Flags().StringVar(&out, "out", "eval/reports", "output directory for reports")

	return cmd
}

// newValidateRulesCmd returns the 'validate-rules' subcommand.
func newValidateRulesCmd() *cobra.Command {
	var corpus string

	cmd := &cobra.Command{
		Use:   "validate-rules",
		Short: "Validate expected_tools.yaml rule files in corpus",
		Long: `validate-rules parses every expected_tools.yaml in the corpus directory
and reports schema errors. Use this before committing new corpus tasks to ensure
the heuristic rule DSL is well-formed.

Real implementation lands in Wave 2+ when the rule DSL ships.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = corpus
			return fmt.Errorf("not yet implemented (Phase 67 Wave 2+)")
		},
	}

	cmd.Flags().StringVar(&corpus, "corpus", "eval/corpus", "path to corpus directory")

	return cmd
}

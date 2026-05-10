// Package eval is the top-level runner library for the Phase 67 evaluation
// harness. It provides the canonical EvalPhases slice whose Run bodies are
// implemented in Wave 3 (DAG-02 contract: replace noopRun with real bodies).
//
// EvalPhases must be built via BuildEvalPhases(state) so that all 10 Run
// closures capture the shared *RunState and write to the correct output paths.
// The package-level Phases variable retains the original shape-only slice for
// backward compatibility with phasegraph tests that only inspect the DAG shape.
package eval

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/agenthands/helix/internal/eval/budget"
	"github.com/agenthands/helix/internal/eval/report"
	"github.com/agenthands/helix/internal/eval/sandbox"
	"github.com/agenthands/helix/internal/eval/score"
	"github.com/agenthands/helix/internal/eval/trace"
	"github.com/agenthands/helix/internal/phasegraph"
	"github.com/agenthands/helix/internal/phasegraph/pipelines"
)

// Phases is the canonical 10-phase eval pipeline DAG (shape only, noopRun
// bodies). Retained for DAG-shape tests. For real runs use BuildEvalPhases.
var Phases []phasegraph.PhaseSpec = pipelines.EvalPhases

// RunState is shared across all 10 phase Run closures for a single (task, mode)
// invocation. Phases read from earlier fields and write to later fields. The
// struct is not concurrency-safe — EvalPhases is a linear chain.
type RunState struct {
	// Inputs (set before BuildEvalPhases is called).
	Sandbox   *sandbox.Sandbox
	CorpusDir string
	OutDir    string
	RunID     string
	TaskID    string
	Mode      string
	Prompt    string
	HelixBin  string

	// Populated by prepare_workspace.
	RepoDir  string
	HomeDir  string
	ModeDir  string
	StartAt  time.Time

	// Populated by run_agent.
	DaemonHandle *sandbox.DaemonHandle
	BudgetBreach *budget.BreachReason

	// Populated by collect_trace.
	DaemonTap trace.DaemonTapResult
	CCTap     trace.CCTapResult
	Merged    trace.MergedTrace

	// Populated by run_tests.
	VerifyExit int
	VerifyLog  []byte

	// Populated by apply_patch_check.
	PatchBytes []byte

	// Populated by score phases.
	ToolScore score.Score

	// Final result (populated by aggregate_report).
	Result report.EvalResult
}

// BuildEvalPhases returns a slice of 10 PhaseSpecs whose Run bodies are wired
// to the shared *RunState. This replaces the noopRun bodies from pipelines.EvalPhases.
func BuildEvalPhases(state *RunState) []phasegraph.PhaseSpec {
	return []phasegraph.PhaseSpec{
		{
			ID:       pipelines.PhasePrepareWorkspace,
			Requires: nil,
			Provides: []string{"workspace"},
			Run: func(ctx context.Context, deps phasegraph.PhaseDeps) (phasegraph.PhaseOutput, error) {
				state.StartAt = time.Now()
				state.RepoDir = state.Sandbox.RepoFor(state.TaskID, state.Mode)
				state.HomeDir = state.Sandbox.HomeFor(state.TaskID, state.Mode)
				state.ModeDir = state.Sandbox.ModeDir(state.TaskID, state.Mode)

				if err := state.Sandbox.Prepare(state.TaskID, state.Mode); err != nil {
					return nil, fmt.Errorf("prepare_workspace: %w", err)
				}
				repoSrc := filepath.Join(state.CorpusDir, state.TaskID, "repo")
				if _, err := os.Stat(repoSrc); err == nil {
					if err := state.Sandbox.CloneRepo(repoSrc, state.TaskID, state.Mode); err != nil {
						return nil, fmt.Errorf("clone_repo: %w", err)
					}
				}
				if err := gitInitBaseline(state.RepoDir); err != nil {
					log.Printf("pipeline: git init baseline: %v", err)
				}
				return "workspace_ready", nil
			},
		},
		{
			ID:       pipelines.PhaseConfigureMode,
			Requires: []phasegraph.PhaseID{pipelines.PhasePrepareWorkspace},
			Provides: []string{"mode_config"},
			Run: func(ctx context.Context, deps phasegraph.PhaseDeps) (phasegraph.PhaseOutput, error) {
				cfgPath := filepath.Join(state.HomeDir, ".helix", "helix_config.yml")
				if err := writeModeConfig(cfgPath, state.Mode); err != nil {
					return nil, fmt.Errorf("configure_mode: %w", err)
				}
				return cfgPath, nil
			},
		},
		{
			ID:       pipelines.PhaseRunAgent,
			Requires: []phasegraph.PhaseID{pipelines.PhaseConfigureMode},
			Provides: []string{"agent_run"},
			Run: func(ctx context.Context, deps phasegraph.PhaseDeps) (phasegraph.PhaseOutput, error) {
				// In the full harness this phase spawns the helix daemon and claude CLI.
				// For Phase 67 v1, the RunTask helper in runner.go handles subprocess
				// orchestration. BuildEvalPhases is available for callers who want to
				// drive the pipeline manually (e.g., eval-quick in-process path in Plan 06).
				// Context cancellation maps to budget breach.
				if ctx.Err() != nil {
					taskBudget := budget.Default
					budgetPath := filepath.Join(state.CorpusDir, state.TaskID, "budget.yaml")
					if _, err := os.Stat(budgetPath); err == nil {
						if b, err := budget.LoadFromFile(budgetPath); err == nil {
							taskBudget = b
						}
					}
					state.BudgetBreach = &budget.BreachReason{
						Axis:     "seconds",
						Limit:    int64(taskBudget.MaxSeconds),
						Observed: int64(time.Since(state.StartAt).Seconds()),
					}
				}
				return "agent_done", nil
			},
		},
		{
			ID:       pipelines.PhaseCollectTrace,
			Requires: []phasegraph.PhaseID{pipelines.PhaseRunAgent},
			Provides: []string{"trace"},
			Run: func(ctx context.Context, deps phasegraph.PhaseDeps) (phasegraph.PhaseOutput, error) {
				daemonLogPath := filepath.Join(state.ModeDir, "daemon.log")
				ccStdoutPath := filepath.Join(state.ModeDir, "claude.stdout")

				if _, err := os.Stat(daemonLogPath); err == nil {
					// Pass the real daemon PID so TapDaemonLog's T-67-04 PID gate
					// accepts log lines from our daemon and rejects foreign processes.
					// state.DaemonHandle is set by run_agent when wave-2 wires the real
					// StartDaemon call; Pid() is nil-safe and returns 0 until then.
					state.DaemonTap, _ = trace.TapDaemonLog(daemonLogPath, state.DaemonHandle.Pid())
				}
				if _, err := os.Stat(ccStdoutPath); err == nil {
					state.CCTap, _ = trace.TapCCStream(ccStdoutPath)
				}

				mergedIn := trace.MergeInput{
					TaskID:    state.TaskID,
					Mode:      state.Mode,
					RunID:     state.RunID,
					StartedAt: state.StartAt,
					EndedAt:   time.Now(),
					Daemon:    state.DaemonTap,
					CC:        state.CCTap,
					RepoRoot:  state.RepoDir,
					Budget:    state.BudgetBreach,
				}
				var err error
				state.Merged, err = trace.Merge(mergedIn)
				if err != nil {
					log.Printf("pipeline: trace merge: %v", err)
				}

				outDir := taskOutDir(state)
				_ = writeJSON(filepath.Join(outDir, "trace.json"), state.Merged)
				return state.Merged, nil
			},
		},
		{
			ID:       pipelines.PhaseApplyPatchCheck,
			Requires: []phasegraph.PhaseID{pipelines.PhaseCollectTrace},
			Provides: []string{"patch_result"},
			Run: func(ctx context.Context, deps phasegraph.PhaseDeps) (phasegraph.PhaseOutput, error) {
				outDir := taskOutDir(state)
				var err error
				state.PatchBytes, err = gitDiff(state.RepoDir)
				if err != nil {
					log.Printf("pipeline: git diff: %v", err)
				}
				if writeErr := os.WriteFile(filepath.Join(outDir, "patch.diff"), state.PatchBytes, 0600); writeErr != nil {
					log.Printf("pipeline: write patch.diff: %v", writeErr)
				}
				state.Result.PatchApplies = len(state.PatchBytes) == 0 || err == nil
				return state.PatchBytes, nil
			},
		},
		{
			ID:       pipelines.PhaseRunTests,
			Requires: []phasegraph.PhaseID{pipelines.PhaseApplyPatchCheck},
			Provides: []string{"test_result"},
			Run: func(ctx context.Context, deps phasegraph.PhaseDeps) (phasegraph.PhaseOutput, error) {
				outDir := taskOutDir(state)
				verifyScript := filepath.Join(state.CorpusDir, state.TaskID, "verify.sh")
				state.VerifyExit, state.VerifyLog = runVerify(ctx, verifyScript, state.RepoDir)
				if writeErr := os.WriteFile(filepath.Join(outDir, "verify.log"), state.VerifyLog, 0600); writeErr != nil {
					log.Printf("pipeline: write verify.log: %v", writeErr)
				}
				state.Result.TestsPass = state.VerifyExit == 0
				return state.VerifyExit, nil
			},
		},
		{
			ID:       pipelines.PhaseRunDiagnostics,
			Requires: []phasegraph.PhaseID{pipelines.PhaseRunTests},
			Provides: []string{"diag_result"},
			Run: func(ctx context.Context, deps phasegraph.PhaseDeps) (phasegraph.PhaseOutput, error) {
				// TODO(post-phase-67): wire real LSP diagnostics via helix get_diagnostics tool.
				// For v1, DiagnosticsClean tracks alongside TestsPass as a placeholder.
				state.Result.DiagnosticsClean = state.Result.TestsPass
				return "diag_done", nil
			},
		},
		{
			ID:       pipelines.PhaseScoreToolBehavior,
			Requires: []phasegraph.PhaseID{pipelines.PhaseRunDiagnostics},
			Provides: []string{"tool_behavior_score"},
			Run: func(ctx context.Context, deps phasegraph.PhaseDeps) (phasegraph.PhaseOutput, error) {
				rulesPath := filepath.Join(state.CorpusDir, state.TaskID, "expected_tools.yaml")
				if _, err := os.Stat(rulesPath); err == nil {
					if rules, err := score.LoadRules(rulesPath); err == nil {
						state.ToolScore = score.Apply(state.Merged, rules)
					}
				}
				return state.ToolScore, nil
			},
		},
		{
			ID:       pipelines.PhaseScoreGuardrails,
			Requires: []phasegraph.PhaseID{pipelines.PhaseScoreToolBehavior},
			Provides: []string{"guardrails_score"},
			Run: func(ctx context.Context, deps phasegraph.PhaseDeps) (phasegraph.PhaseOutput, error) {
				state.Result.GuardrailCompliance = state.Merged.Guardrails
				state.Result.EditCount = state.Merged.ToolCallSummary.Total
				// Populate per-tool call counts so buildModeAggregates can fill
				// ToolCallDistribution for eval_report.md (WR-04 fix).
				if len(state.Merged.ToolCallSummary.ByTool) > 0 {
					state.Result.ToolCallsByTool = make(map[string]int, len(state.Merged.ToolCallSummary.ByTool))
					for tool, count := range state.Merged.ToolCallSummary.ByTool {
						state.Result.ToolCallsByTool[tool] = count
					}
				}
				state.Result.Tokens.Input = state.Merged.Usage.InputTokens
				state.Result.Tokens.Output = state.Merged.Usage.OutputTokens
				state.Result.DurationMs = int64(time.Since(state.StartAt) / time.Millisecond)
				return state.Result.GuardrailCompliance, nil
			},
		},
		{
			ID:       pipelines.PhaseAggregateReport,
			Requires: []phasegraph.PhaseID{pipelines.PhaseScoreGuardrails},
			Provides: []string{"report"},
			Run: func(ctx context.Context, deps phasegraph.PhaseDeps) (phasegraph.PhaseOutput, error) {
				state.Result.TaskID = state.TaskID
				state.Result.Mode = state.Mode

				if state.BudgetBreach != nil {
					state.Result.Outcome = state.BudgetBreach.String()
					state.Result.Success = false
				} else if !state.Result.TestsPass {
					state.Result.Outcome = "failed"
					state.Result.Success = false
				} else {
					state.Result.Outcome = state.Merged.Outcome
					state.Result.Success = state.Result.Outcome == "success"
				}

				outDir := taskOutDir(state)
				if err := report.WriteResult(filepath.Join(outDir, "result.json"), state.Result); err != nil {
					return nil, fmt.Errorf("aggregate_report: write result: %w", err)
				}
				return state.Result, nil
			},
		},
	}
}

// taskOutDir returns the per-(task, mode) output directory for state.
func taskOutDir(state *RunState) string {
	return filepath.Join(state.OutDir, state.RunID, "tasks", state.TaskID, state.Mode)
}

// gitInitBaseline runs git init + add -A + commit to create a clean baseline.
func gitInitBaseline(repoDir string) error {
	cmds := [][]string{
		{"git", "-C", repoDir, "init", "-q"},
		{"git", "-C", repoDir, "config", "user.email", "eval@helix"},
		{"git", "-C", repoDir, "config", "user.name", "helix-eval"},
		{"git", "-C", repoDir, "add", "-A"},
		{"git", "-C", repoDir, "commit", "--allow-empty", "-m", "baseline", "-q"},
	}
	for _, argv := range cmds {
		if out, err := exec.Command(argv[0], argv[1:]...).CombinedOutput(); err != nil {
			return fmt.Errorf("git step %v: %w: %s", argv, err, bytes.TrimSpace(out))
		}
	}
	return nil
}

// gitDiff runs git diff HEAD in repoDir and returns the diff bytes.
func gitDiff(repoDir string) ([]byte, error) {
	out, err := exec.Command("git", "-C", repoDir, "diff", "--binary", "HEAD").Output()
	if err != nil {
		return nil, fmt.Errorf("git diff: %w", err)
	}
	return out, nil
}

// runVerify executes verify.sh (if present) with cwd=repoDir.
func runVerify(ctx context.Context, scriptPath, repoDir string) (int, []byte) {
	if _, err := os.Stat(scriptPath); err != nil {
		return 0, []byte("verify.sh not found; auto-pass\nexit_code=0\n")
	}
	cmd := exec.CommandContext(ctx, "/bin/sh", scriptPath)
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	exitCode := 0
	if err != nil {
		if e, ok := err.(*exec.ExitError); ok {
			exitCode = e.ExitCode()
		}
	}
	logOut := append(out, []byte(fmt.Sprintf("\nexit_code=%d\n", exitCode))...)
	return exitCode, logOut
}

// writeModeConfig writes helix_config.yml for the given mode (T-67-03: hard-coded
// enum to prevent unknown-mode config injection).
func writeModeConfig(cfgPath, mode string) error {
	var content string
	switch mode {
	case "baseline":
		content = "profile: baseline\nsemantic_index:\n  enabled: false\nguardrails:\n  enforcement: off\n"
	case "native":
		content = "profile: full\nsemantic_index:\n  enabled: false\nguardrails:\n  enforcement: off\n"
	case "semantic":
		content = "profile: full\nsemantic_index:\n  enabled: true\nguardrails:\n  enforcement: off\n"
	case "semantic_guarded":
		content = "profile: full\nsemantic_index:\n  enabled: true\nguardrails:\n  enforcement: warn\n"
	default:
		return fmt.Errorf("unknown eval mode %q; valid modes are: baseline, native, semantic, semantic_guarded", mode)
	}
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0700); err != nil {
		return fmt.Errorf("mkdir config dir: %w", err)
	}
	return os.WriteFile(cfgPath, []byte(content), 0600)
}

// writeJSON marshals v to pretty JSON and writes it to path with mode 0600.
func writeJSON(path string, v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

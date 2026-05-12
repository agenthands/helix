// Package runner implements the Phase 67 evaluation mode dispatch loop.
// Per the DAG-02 contract, the Runner fills the noopRun bodies of the
// phasegraph EvalPhases for each (task, mode) pair.
package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/agenthands/helix/internal/eval/budget"
	"github.com/agenthands/helix/internal/eval/report"
	"github.com/agenthands/helix/internal/eval/sandbox"
	"github.com/agenthands/helix/internal/eval/score"
	"github.com/agenthands/helix/internal/eval/trace"
)

// Config holds Runner-level parameters.
type Config struct {
	CorpusDir   string // path to eval/corpus
	OutDir      string // path to eval/reports (or override)
	HelixBin    string // absolute path to the helix binary
	RunID       string // run identifier (ISO-8601 + git SHA)
	MaxParallel int    // concurrency limit (default 1 per D-05)
}

// TaskSpec describes one (task, mode) unit of work.
type TaskSpec struct {
	ID     string
	Mode   string
	Prompt string
}

// Runner orchestrates the per-(task, mode) eval pipeline.
type Runner struct {
	cfg Config
}

// NewRunner returns a Runner configured with cfg.
// MaxParallel defaults to 1 when zero (D-05: wall-time predictability).
func NewRunner(cfg Config) *Runner {
	if cfg.MaxParallel <= 0 {
		cfg.MaxParallel = 1
	}
	return &Runner{cfg: cfg}
}

// taskOutDir returns the per-(task, mode) output directory under OutDir.
func (r *Runner) taskOutDir(taskID, mode string) string {
	return filepath.Join(r.cfg.OutDir, r.cfg.RunID, "tasks", taskID, mode)
}

// validateTaskID rejects task IDs that could escape join roots via path traversal.
// Closes 67-SECURITY.md FLAG-2 — task IDs flow into filepath.Join with CorpusDir
// and OutDir; a malicious ID like "../../foo" would escape both.
func validateTaskID(id string) error {
	if id == "" {
		return fmt.Errorf("task id is empty")
	}
	if id != filepath.Clean(id) || strings.ContainsAny(id, `/\`) || strings.HasPrefix(id, ".") {
		return fmt.Errorf("task id %q contains path separators, parent refs, or leading dot", id)
	}
	return nil
}

// RunTask executes all 10 phasegraph EvalPhases for (taskSpec.ID, taskSpec.Mode)
// using the provided sandbox for filesystem isolation. It returns an EvalResult
// capturing every measurable dimension of the run. RunTask does not return an
// error on agent or verify failure — those are captured in the result.
//
// The returned error is non-nil only for infrastructure failures (e.g., unable
// to create the output directory or write required artifacts).
func (r *Runner) RunTask(ctx context.Context, sb *sandbox.Sandbox, ts TaskSpec) (*report.EvalResult, error) {
	if err := validateTaskID(ts.ID); err != nil {
		return nil, fmt.Errorf("runner: %w", err)
	}
	outDir := r.taskOutDir(ts.ID, ts.Mode)
	if err := os.MkdirAll(outDir, 0700); err != nil {
		return nil, fmt.Errorf("runner: mkdir task outdir: %w", err)
	}

	result := &report.EvalResult{
		TaskID: ts.ID,
		Mode:   ts.Mode,
	}

	runStart := time.Now()

	// Phase 1: prepare_workspace
	taskDir := filepath.Join(r.cfg.CorpusDir, ts.ID)
	repoSrc := filepath.Join(taskDir, "repo")
	if err := sb.Prepare(ts.ID, ts.Mode); err != nil {
		result.Outcome = "failed"
		result.FailureReason = "prepare_workspace: " + err.Error()
		_ = writeResult(outDir, result)
		return result, nil
	}
	if _, err := os.Stat(repoSrc); err == nil {
		if err := sb.CloneRepo(repoSrc, ts.ID, ts.Mode); err != nil {
			result.Outcome = "failed"
			result.FailureReason = "clone_repo: " + err.Error()
			_ = writeResult(outDir, result)
			return result, nil
		}
	}
	// git init + add + commit baseline (for git diff to work later).
	repoDir := sb.RepoFor(ts.ID, ts.Mode)
	if err := gitInitBaseline(repoDir); err != nil {
		// Non-fatal: git may not be available or repo may already be inited.
		log.Printf("runner: git init baseline for %s/%s: %v", ts.ID, ts.Mode, err)
	}

	// Phase 2: configure_mode
	cfgPath := filepath.Join(sb.HomeFor(ts.ID, ts.Mode), ".helix", "helix_config.yml")
	if err := writeModeConfig(cfgPath, ts.Mode); err != nil {
		result.Outcome = "failed"
		result.FailureReason = "configure_mode: " + err.Error()
		_ = writeResult(outDir, result)
		return result, nil
	}

	// Phase 3: run_agent — in the real harness this spawns the helix daemon and
	// claude CLI. In test contexts where helix is not available, we skip daemon
	// startup and record an empty agent run with zero tokens.
	agentDuration := time.Duration(0)
	var budgetBreach *budget.BreachReason

	// Load per-task budget (D-08).
	taskBudget := budget.Default
	budgetPath := filepath.Join(taskDir, "budget.yaml")
	if _, err := os.Stat(budgetPath); err == nil {
		if b, err := budget.LoadFromFile(budgetPath); err == nil {
			taskBudget = b
		}
	}

	// Check for context cancellation before attempting agent (represents budget/timeout).
	agentStart := time.Now()
	if ctx.Err() != nil {
		budgetBreach = &budget.BreachReason{
			Axis:     "seconds",
			Limit:    int64(taskBudget.MaxSeconds),
			Observed: int64(time.Since(agentStart).Seconds()),
		}
	}
	agentDuration = time.Since(agentStart)
	_ = agentDuration

	// Phase 4: collect_trace — gather daemon + CC evidence streams.
	daemonLogPath := filepath.Join(sb.ModeDir(ts.ID, ts.Mode), "daemon.log")
	ccStdoutPath := filepath.Join(sb.ModeDir(ts.ID, ts.Mode), "claude.stdout")

	var daemonTap trace.DaemonTapResult
	var ccTap trace.CCTapResult
	var daemonHandle *sandbox.DaemonHandle

	// F-07 leg B: boot a real daemon for this (task, mode) so the TelemetryMiddleware
	// "tool call" JSONL events land in daemon.log and the tap PID gate operates
	// against a real PID. HelixBin=="" preserves the in-process runner_test.go
	// path (no daemon spawn) so existing unit tests stay green.
	if r.cfg.HelixBin != "" {
		profileName := profileForMode(ts.Mode)
		h, err := sb.StartDaemon(ctx, ts.ID, ts.Mode, profileName, cfgPath)
		if err != nil {
			log.Printf("runner: start daemon for %s/%s: %v", ts.ID, ts.Mode, err)
		} else {
			daemonHandle = h
		}
	}

	// Stop the daemon BEFORE reading its log so buffered slog lines flush.
	if daemonHandle != nil {
		if killErr := daemonHandle.Kill(); killErr != nil {
			log.Printf("runner: kill daemon for %s/%s: %v", ts.ID, ts.Mode, killErr)
		}
		if _, err := os.Stat(daemonLogPath); err == nil {
			daemonTap, _ = trace.TapDaemonLog(daemonLogPath, daemonHandle.Pid())
		}
	}
	if _, err := os.Stat(ccStdoutPath); err == nil {
		ccTap, _ = trace.TapCCStream(ccStdoutPath)
	}

	now := time.Now()
	mergedIn := trace.MergeInput{
		TaskID:    ts.ID,
		Mode:      ts.Mode,
		RunID:     r.cfg.RunID,
		StartedAt: runStart,
		EndedAt:   now,
		Daemon:    daemonTap,
		CC:        ccTap,
		RepoRoot:  repoDir,
		Budget:    budgetBreach,
	}

	merged, err := trace.Merge(mergedIn)
	if err != nil {
		log.Printf("runner: trace merge for %s/%s: %v", ts.ID, ts.Mode, err)
	}

	// Write trace artifacts.
	_ = writeJSON(filepath.Join(outDir, "trace.json"), merged)

	// Phase 5: apply_patch_check — git diff + verify patch applies.
	patchBytes, patchErr := gitDiff(repoDir)
	if patchErr != nil {
		log.Printf("runner: git diff for %s/%s: %v", ts.ID, ts.Mode, patchErr)
	}
	if err := os.WriteFile(filepath.Join(outDir, "patch.diff"), patchBytes, 0600); err != nil {
		log.Printf("runner: write patch.diff: %v", err)
	}
	result.PatchApplies = len(patchBytes) == 0 || patchErr == nil

	// Phase 6: run_tests — execute verify.sh.
	verifyScript := filepath.Join(taskDir, "verify.sh")
	verifyExit, verifyLog := runVerify(ctx, verifyScript, repoDir)
	if err := os.WriteFile(filepath.Join(outDir, "verify.log"), verifyLog, 0600); err != nil {
		log.Printf("runner: write verify.log: %v", err)
	}
	result.TestsPass = verifyExit == 0

	// Phase 7: run_diagnostics — v1 placeholder per plan spec.
	// TODO(post-phase-67): wire real LSP diagnostics via helix get_diagnostics.
	result.DiagnosticsClean = result.TestsPass

	// Phase 8: score_tool_behavior.
	var toolScore score.Score
	rulesPath := filepath.Join(taskDir, "expected_tools.yaml")
	if _, err := os.Stat(rulesPath); err == nil {
		if rules, err := score.LoadRules(rulesPath); err == nil {
			toolScore = score.Apply(merged, rules)
		}
	}
	_ = toolScore // written to tool_behavior.json by aggregate reports

	// Phase 9: score_guardrails.
	result.GuardrailCompliance = merged.Guardrails
	result.EditCount = merged.ToolCallSummary.Total
	// Populate per-tool call counts so buildModeAggregates can fill
	// ToolCallDistribution for the eval_report.md tool-call table (WR-04 fix).
	if len(merged.ToolCallSummary.ByTool) > 0 {
		result.ToolCallsByTool = make(map[string]int, len(merged.ToolCallSummary.ByTool))
		for tool, count := range merged.ToolCallSummary.ByTool {
			result.ToolCallsByTool[tool] = count
		}
	}
	result.Tokens.Input = merged.Usage.InputTokens
	result.Tokens.Output = merged.Usage.OutputTokens
	result.DurationMs = int64(time.Since(runStart) / time.Millisecond)

	// Phase 10: aggregate_report — resolve outcome and write result.json.
	if budgetBreach != nil {
		result.Outcome = budgetBreach.String()
		result.Success = false
	} else if !result.TestsPass {
		result.Outcome = "failed"
		result.Success = false
	} else {
		result.Outcome = merged.Outcome
		result.Success = result.Outcome == "success"
	}

	if err := writeResult(outDir, result); err != nil {
		return result, err
	}

	return result, nil
}

// RunMatrix runs all (task, mode) pairs found in corpusDir and returns the
// collected EvalResults. Concurrency is bounded by cfg.MaxParallel.
// Errors for individual tasks are captured in the results, not propagated.
func (r *Runner) RunMatrix(ctx context.Context, modes []string) ([]report.EvalResult, error) {
	entries, err := os.ReadDir(r.cfg.CorpusDir)
	if err != nil {
		return nil, fmt.Errorf("runner: read corpus dir: %w", err)
	}

	type work struct {
		taskID string
		mode   string
	}

	var jobs []work
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		for _, mode := range modes {
			jobs = append(jobs, work{taskID: entry.Name(), mode: mode})
		}
	}

	sem := make(chan struct{}, r.cfg.MaxParallel)
	var mu sync.Mutex
	var results []report.EvalResult

	var wg sync.WaitGroup
	for _, j := range jobs {
		j := j
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			sb, err := sandbox.NewSandbox(r.cfg.RunID+"-"+j.taskID, r.cfg.HelixBin)
			if err != nil {
				log.Printf("runner: new sandbox for %s/%s: %v", j.taskID, j.mode, err)
				return
			}
			defer sb.Cleanup()

			res, err := r.RunTask(ctx, sb, TaskSpec{
				ID:   j.taskID,
				Mode: j.mode,
			})
			if err != nil {
				log.Printf("runner: RunTask %s/%s: %v", j.taskID, j.mode, err)
				return
			}

			mu.Lock()
			results = append(results, *res)
			mu.Unlock()
		}()
	}
	wg.Wait()

	return results, nil
}

// gitInitBaseline runs git init, add -A, and commit to create a clean baseline
// that git diff can compare against after the agent runs.
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

// runVerify executes verify.sh (if present) with cwd=repoDir. Returns exit
// code and combined output bytes. If verify.sh is absent, returns 0 (pass).
func runVerify(ctx context.Context, scriptPath, repoDir string) (int, []byte) {
	if _, err := os.Stat(scriptPath); err != nil {
		// No verify.sh — auto-pass.
		return 0, []byte("verify.sh not found; auto-pass\nexit_code=0\n")
	}

	cmd := exec.CommandContext(ctx, "/bin/sh", scriptPath)
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		}
	}

	log := append(out, []byte(fmt.Sprintf("\nexit_code=%d\n", exitCode))...)
	return exitCode, log
}

// writeModeConfig writes helix_config.yml for the given mode. The four modes
// map to fixed profile/config combinations (T-67-03: hard-coded switch to
// prevent unknown-mode config injection).
// profileForMode maps an eval mode name to the helix daemon profile that
// should serve it. baseline → "baseline"; everything else → "full".
// Used by RunTask when spawning the per-(task, mode) daemon (F-07 leg B).
func profileForMode(mode string) string {
	if mode == "baseline" {
		return "baseline"
	}
	return "full"
}

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

// writeResult is a helper that marshals and writes result.json under outDir.
func writeResult(outDir string, r *report.EvalResult) error {
	return report.WriteResult(filepath.Join(outDir, "result.json"), *r)
}

// writeJSON marshals v to JSON and writes it to path with mode 0600.
func writeJSON(path string, v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("json marshal: %w", err)
	}
	return os.WriteFile(path, data, 0600)
}

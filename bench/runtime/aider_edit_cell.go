// Package runtime — aider_edit_cell.go owns the Phase 100 (EDITBENCH-01 live half +
// BASELINE-01) polyglot-edit drive leg. runAiderEditCell is a mode-name branch off
// RunCell (detected by aiderEditMode, mirroring baselineRagMode at cell.go:434) that
// spawns the warm daemon, loads the vendored exercise, drives the Plan 01
// deterministic EDIT-verb AgentFn + live native TestFn through aiderpolyglot.RunExercise
// VERBATIM, stamps edit_format_applied, and writes a schema-valid result.v2.
//
// Divergence from the daemon spine (cell.go): the aider-edit branch OWNS its own
// fixture path resolution — the standard cellSeedDir/scripted_agent.yaml seed-dir
// assumptions do NOT apply. The fixture exercise dir arrives as cfg.SeedDir (the
// caller resolves bench/datasets/aider-polyglot/fixtures/<lang>/exercises/practice/<task>);
// CloneRepo copies the WHOLE exercise tree (stub, test, cases, go.mod, .meta/) into the
// per-cell repo working copy, and the AgentFn reads the pristine reference from the
// fixture dir (ex.SrcDir) while applying edits into the cloned working copy (workDir).
//
// WR-01: RunExercise is called VERBATIM — restorePristineTests runs after the edit,
// before each grade, only when ex.SrcDir != "" (loader.go:262-268). The deterministic
// agent edits only files.solution stubs, never files.test (aider_edit_agent.go).
package runtime

import (
	"context"
	"fmt"
	"os"
	"sort"
	"time"

	aiderpolyglot "github.com/agenthands/helix/bench/datasets/aider-polyglot"
	"github.com/agenthands/helix/bench/evaluators"
	"github.com/agenthands/helix/bench/evaluators/coordinator"
	"github.com/agenthands/helix/bench/languages"
	"github.com/agenthands/helix/bench/runners"
	benchsandbox "github.com/agenthands/helix/bench/runtime/sandbox"
	"github.com/agenthands/helix/bench/runtime/subprocess"
	"github.com/agenthands/helix/internal/eval/runner"
	"github.com/agenthands/helix/internal/eval/trace"
)

// aiderEditBenchmark is the benchmark-suite name carried on every aider-edit row.
// It is fixed: the arm always runs the aider-polyglot fixtures.
const aiderEditBenchmark = "aider-polyglot"

// aiderEditResultInput is the deterministic-metrics-only input to assembleAiderEditResult.
// It carries ONLY reproducible fields (no latency, no timestamp, no absolute path):
// the graded pass/fail, the apply-format flag, and the cell's task/language axes.
type aiderEditResultInput struct {
	Task     string
	Language string
	// Passed is the RunExercise grade (the native test outcome).
	Passed bool
	// Applied is edit_format_applied — whether the deterministic EDIT verb applied
	// the reference format to the stub (true) or could not (false). Recorded as a
	// *bool open key so a literal false survives marshalling (Pitfall 2).
	Applied bool
}

// assembleAiderEditResult builds the schema-valid result.v2 bytes for an aider-edit
// cell from DETERMINISTIC metrics only. It is a pure function (no IO, no clock, no
// RNG): the same input always produces byte-identical output, which is what makes the
// committed baseline byte-reproducible (BASELINE-01 / Pitfall 3).
//
// It synthesizes the grade through the SAME coordinator.Grade contract the daemon
// spine uses (so task_success / edit metrics are derived identically), then routes the
// row through BuildResult with EditFormatApplied stamped. The trace_ref is left empty
// (the committed baseline does not commit a per-run trace path — an absolute scratch
// path would be machine-specific and non-reproducible). UsagePresent is false: the
// deterministic agent drives no model, so token metrics are explicit null (never a
// fabricated 0).
//
// Both the live cell (runAiderEditCell) and the hermetic sibling test drive this helper,
// so the SOLE authoritative proof exercises the exact assembly the baseline commits.
func assembleAiderEditResult(in aiderEditResultInput) ([]byte, error) {
	outcome := "failed"
	verifyExit := 1
	if in.Passed {
		outcome = "success"
		verifyExit = 0
	}

	postOutcome := languages.TestOutcome{Passed: in.Passed, ExitCode: verifyExit}
	// The aider-edit arm has no separate pre-patch snapshot (the stub is the pre-patch
	// state); a zero-value pre-patch outcome keeps the grader contract identical.
	//
	// WR-01 (byte-reproducible baseline must be git-independent): the patch_validator
	// graders are SKIPPED entirely via SkipPatchValidator — NO git process is spawned
	// during baseline assembly, so the committed bytes can never embed an
	// environment-specific git error string (e.g. "git not found on PATH") into
	// metric_errors and diverge across machines/CI. The patch_validator metrics are
	// repo-derived (non-reproducible) anyway, so they are explicitly nulled below with
	// an honest "excluded from the deterministic baseline" annotation. RepoDir is left
	// empty because it is never consulted on the skip path.
	metrics, metricErrs := coordinator.Grade(context.Background(), coordinator.GradeInput{
		TestOutcome:        postOutcome,
		PrePatchOutcome:    languages.TestOutcome{},
		RepoDir:            "",
		Merged:             trace.MergedTrace{},
		UsagePresent:       false,
		Agent:              "scripted",
		SkipPatchValidator: true,
	})

	// Pitfall 3 (non-reproducible committed baseline): the patch_validator metrics
	// (files_modified / edit_locality / edit_distance_patch) are repo-derived — they
	// vary with the machine's git working tree, so they are NOT byte-reproducible and
	// MUST NOT enter the committed baseline. They are NOT computed at all here
	// (SkipPatchValidator above suppresses the git graders), so they are already nil;
	// the explicit nil assignments below are belt-and-suspenders. Record the exclusion
	// reason in the metric_errors annotations so the row stays self-describing. Only
	// the truly deterministic quality metrics (task_success, verified_correctness,
	// edit_format_applied, outcome) survive into the committed bytes.
	metrics.FilesModified = nil
	metrics.EditLocality = nil
	metrics.EditDistancePatch = nil
	const baselineNullReason = "excluded from the deterministic committed baseline (repo-derived, non-reproducible)"
	metricErrs = append(metricErrs,
		evaluators.MetricError{Metric: "files_modified", Grader: "patch_validator", Reason: baselineNullReason},
		evaluators.MetricError{Metric: "edit_locality", Grader: "patch_validator", Reason: baselineNullReason},
		evaluators.MetricError{Metric: "edit_distance_patch", Grader: "patch_validator", Reason: baselineNullReason},
	)
	// Sort the annotations by (metric, grader) so the emitted order can never drift on
	// a grader-iteration change (Pitfall 3 — deterministic sort-before-emit).
	sort.Slice(metricErrs, func(i, j int) bool {
		if metricErrs[i].Metric != metricErrs[j].Metric {
			return metricErrs[i].Metric < metricErrs[j].Metric
		}
		return metricErrs[i].Grader < metricErrs[j].Grader
	})

	applied := in.Applied
	resultBytes, err := BuildResult(ResultInput{
		TaskID:            in.Task,
		Mode:              aiderEditMode,
		Benchmark:         aiderEditBenchmark,
		RunIndex:          0,
		Outcome:           outcome,
		TraceRef:          "",
		Language:          in.Language,
		Fairness:          runners.DefaultContract,
		AblationStatus:    "",
		EditFormatApplied: &applied,
		Metrics:           metrics,
		MetricErrors:      metricErrs,
	})
	if err != nil {
		return nil, fmt.Errorf("bench/runtime: build aider-edit result.v2: %w", err)
	}
	return resultBytes, nil
}

// runAiderEditCell is the aider_edit drive leg invoked from RunCell after the
// unconditional fairness gate. It mirrors the daemon spine (sandbox / clone / spawn /
// drive / grade / build+validate / write) with the divergence documented in the file
// header: the fixture tree is resolved from cfg.SeedDir (the caller owns that
// resolution), the agent drives helix EDIT verbs through aiderpolyglot.RunExercise
// VERBATIM, and the deterministic metrics route through assembleAiderEditResult.
//
// res arrives pre-populated by RunCell with the validated path layout
// (ResultPath/MergedTracePath) and Task/Mode. A non-nil return is an infrastructure
// failure (scratch preserved); a graded fail is a normal CellResult.
func runAiderEditCell(ctx context.Context, cfg CellConfig, res CellResult) (CellResult, error) {
	sb, err := benchsandbox.New(cfg.RunID, cfg.HelixBin, cfg.OutDir)
	if err != nil {
		return res, fmt.Errorf("bench/runtime: create sandbox: %w", err)
	}
	res.ScratchDir = sb.Root

	cleanedUp := false
	cleanup := func() {
		if cleanedUp {
			return
		}
		_ = sb.Cleanup()
		cleanedUp = true
	}
	preserve := func(e error) (CellResult, error) {
		res.ScratchPreserved = true
		fmt.Fprintf(os.Stderr, "bench/runtime: cell %s/%s failed; preserving scratch at %s: %v\n",
			cfg.Task, cfg.Mode, sb.Root, e)
		return res, e
	}

	if err := sb.Prepare(cfg.Task, cfg.Mode); err != nil {
		return preserve(fmt.Errorf("bench/runtime: prepare cell: %w", err))
	}

	// DIVERGENCE: the fixture exercise dir is cfg.SeedDir (the caller resolved
	// bench/datasets/aider-polyglot/fixtures/<lang>/exercises/practice/<task>). Clone
	// the WHOLE tree into the per-cell repo working copy (CloneRepo is a recursive walk).
	if cfg.SeedDir == "" {
		return preserve(fmt.Errorf("bench/runtime: aider_edit cell requires a fixture SeedDir"))
	}
	if err := sb.CloneRepo(cfg.SeedDir, cfg.Task, cfg.Mode); err != nil {
		return preserve(fmt.Errorf("bench/runtime: clone exercise repo: %w", err))
	}
	workDir := sb.RepoFor(cfg.Task, cfg.Mode)

	// Load the exercise from the FIXTURE dir so ex.SrcDir is the pristine source the
	// AgentFn reads the reference body from AND the WR-01 restore reads pristine tests
	// from (loader.go:262-268). Edits are applied into workDir (the cloned copy).
	ex, err := aiderpolyglot.LoadExercise(cfg.SeedDir, cfg.Language)
	if err != nil {
		return preserve(fmt.Errorf("bench/runtime: load exercise: %w", err))
	}

	// Spawn the warm daemon exactly like the daemon spine (Unix socket, HTTP off,
	// store-off default).
	cfgPath, err := writeCellConfig(sb, cfg.Task, cfg.Mode, "bench-full", false)
	if err != nil {
		return preserve(fmt.Errorf("bench/runtime: write cell config: %w", err))
	}
	h, err := subprocess.StartDaemon(ctx, sb, cfg.Task, cfg.Mode, "bench-full", cfgPath)
	if err != nil {
		return preserve(fmt.Errorf("bench/runtime: start daemon: %w", err))
	}
	daemonPID := h.Pid()
	if daemonPID <= 0 {
		_ = h.Kill()
		return preserve(fmt.Errorf("bench/runtime: daemon returned non-positive pid %d", daemonPID))
	}

	start := time.Now()

	// Build the deterministic EDIT-verb agent (dials the warm daemon over the socket)
	// + the live native TestFn, then drive RunExercise VERBATIM (WR-01 intact inside).
	applied := false
	agentFn := newDeterministicEditAgent(sb.SocketFor(cfg.Task, cfg.Mode), &applied)
	testFn := newNativeTestFn()
	attempt := aiderpolyglot.RunExercise(ctx, ex, workDir, testFn, agentFn)

	// Map the attempt grade to a verify exit code (mirrors runRAGCell:165-169).
	verifyExit := 1
	if attempt.Passed {
		verifyExit = 0
	}
	res.VerifyExitCode = verifyExit

	// Reap the daemon (graceful Stop + Kill fallback, mirroring cell.go:603-617).
	graceful, stopErr := h.Stop(daemonGracefulStopTimeout)
	if stopErr != nil {
		fmt.Fprintf(os.Stderr, "bench/runtime: cell %s/%s graceful stop error (falling back to Kill): %v\n",
			cfg.Task, cfg.Mode, stopErr)
	}
	if !graceful || stopErr != nil {
		fmt.Fprintf(os.Stderr, "bench/runtime: cell %s/%s daemon did not exit gracefully within %s; killing\n",
			cfg.Task, cfg.Mode, daemonGracefulStopTimeout)
	}
	if err := h.Kill(); err != nil {
		return preserve(fmt.Errorf("bench/runtime: kill daemon: %w", err))
	}

	// Synthesize a faithful CC leg from a single replace_in_file StepResult (no model →
	// zero Usage, never a fabricated token count). trace.Merge tolerates the CC-only leg
	// (the daemon also logged the call, but we do not tap it for the deterministic arm —
	// the row's validity does not depend on the tap, mirroring runRAGCell's CC-only merge).
	steps := []runner.StepResult{{Tool: "replace_in_file", AtTime: start}}
	cc := SynthCCTap(steps)
	merged, err := trace.Merge(trace.MergeInput{
		TaskID:         cfg.Task,
		Mode:           cfg.Mode,
		RunID:          cfg.RunID,
		StartedAt:      start,
		EndedAt:        time.Now(),
		CC:             cc,
		VerifyExitCode: verifyExit,
		RepoRoot:       workDir,
	})
	if err != nil {
		return preserve(fmt.Errorf("bench/runtime: merge trace: %w", err))
	}
	res.Merged = merged
	res.ToolCallTotal = merged.ToolCallSummary.Total
	res.CCLegPresent = ccLegPresent(merged)

	// Build + validate result.v2 through the SAME deterministic assembly the committed
	// baseline uses (so the live row and the committed baseline agree). The merged trace
	// is written durably for debugging, but the result row carries only deterministic
	// metrics (no latency / absolute path).
	resultBytes, err := assembleAiderEditResult(aiderEditResultInput{
		Task:     cfg.Task,
		Language: cfg.Language,
		Passed:   attempt.Passed,
		Applied:  applied,
	})
	if err != nil {
		return preserve(err)
	}
	if err := Validate(resultBytes); err != nil {
		return preserve(fmt.Errorf("bench/runtime: result.v2 invalid: %w", err))
	}
	res.ResultValid = true

	if err := writeDurable(res.ResultPath, resultBytes); err != nil {
		return preserve(fmt.Errorf("bench/runtime: write result.v2: %w", err))
	}
	traceBytes, err := marshalTrace(merged)
	if err != nil {
		return preserve(fmt.Errorf("bench/runtime: marshal merged trace: %w", err))
	}
	if err := writeDurable(res.MergedTracePath, traceBytes); err != nil {
		return preserve(fmt.Errorf("bench/runtime: write merged trace: %w", err))
	}

	cleanup()
	return res, nil
}

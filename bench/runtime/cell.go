// Package runtime — cell.go owns the end-to-end single-cell orchestrator
// (BENCH-04, the heart of Phase 77). RunCell composes the Plan 01 primitives
// (bench sandbox, per-cell daemon subprocess, mode->profile resolver, seed task)
// and the Plan 02 pure transforms (SynthCCTap, BuildResult/Validate) with the
// reused Phase 67 internal/eval/trace machinery (TapDaemonLog, Merge) into one
// runnable cell:
//
//	resolve mode -> profile        (D-05)
//	sandbox prepare + clone repo   (D-07, ephemeral OS-temp scratch)
//	spawn per-cell daemon          (D-06, Unix socket, HTTP off)
//	capture daemonPID BEFORE Kill  (criterion #4 PID gate)
//	drive scripted edit            (D-06 forwarder transport, drive.go)
//	kill daemon, PID-gated tap      (D-08, terminate daemon THEN tap)
//	run verify.sh                   (D-04 outcome source)
//	synth CC leg + 2-leg Merge      (D-02)
//	build + validate result.v2      (D-04)
//	write durable artifacts         (D-08, <out>/<task>/<mode>/)
//	cleanup scratch on success only (D-08 preserve-on-failure)
//
// This is the spine of internal/eval/runner/daemon_tap_integration_test.go
// generalized to one cell plus the synth CC leg and the result builder. It
// carries the three Nyquist smoke assertions (2-leg trace, zero PID cross-talk,
// schema-valid result) into the returned CellResult so callers/tests assert them.
package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/agenthands/helix/bench/runners"
	benchsandbox "github.com/agenthands/helix/bench/runtime/sandbox"
	"github.com/agenthands/helix/bench/runtime/subprocess"
	"github.com/agenthands/helix/internal/eval/runner"
	"github.com/agenthands/helix/internal/eval/trace"
)

// claudeMaxToolCalls bounds the wired-not-gating claude run via --max-turns. The
// seed task is a single edit; a small bound keeps a local claude run from looping.
const claudeMaxToolCalls = 20

// CellConfig is the full set of inputs to run one <run_id>/<task>/<mode> cell.
type CellConfig struct {
	// RunID is the run identifier (eval shape, e.g. "20060102T150405Z"). It is
	// metadata only here; the durable out dir is OutDir (already run-scoped).
	RunID string
	// Benchmark is the benchmark suite name (e.g. "toolbench-go").
	Benchmark string
	// Task is the task id (e.g. "sum-doubler"). Validated against path traversal.
	Task string
	// Mode is the bench mode name (e.g. "your_agent_full"). Resolved to a
	// profile via the MODE.md resolver; validated against path traversal.
	Mode string
	// RunIndex is the 0-based repetition index for this (task, mode) cell.
	RunIndex int

	// HelixBin is the path to the helix binary used to spawn the daemon and the
	// stdio forwarder. Must be resolvable (absolute path or on PATH).
	HelixBin string
	// SeedDir is the source task directory cloned into the cell's repo working
	// copy (e.g. bench/datasets/toolbench-go/sum-doubler). It must contain the
	// scripted_agent.yaml the driver replays and the verify.sh outcome script.
	SeedDir string
	// OutDir is the durable artifact root for this run (e.g.
	// bench/reports/<run_id>). Durable artifacts land under <OutDir>/<task>/<mode>/.
	OutDir string

	// RunnersRoot, when non-empty, overrides the mode->profile resolver root
	// (for tests). Empty resolves from the bench/runners package directory.
	RunnersRoot string

	// Agent selects the drive leg. "" or "scripted" (the CI gate) replays the
	// task's scripted_agent.yaml through the forwarder; "claude" drives the real
	// claude CLI agent (D-01, wired-not-gating — locally runnable, never the CI
	// default). The rest of the spine (sandbox/daemon/tap/merge/result) is
	// identical for both; only the drive step differs.
	Agent string
	// Prompt is the task instruction handed to the claude agent (Agent=="claude").
	// Ignored by the scripted path. Sourced from the seed task's task.json.
	Prompt string
}

// CellResult is the outcome of one cell plus the three Nyquist signals the
// smoke must assert (RESEARCH §"Nyquist"):
//
//  1. ToolCallTotal>=1 AND CCLegPresent — the 2-leg merged trace is real.
//  2. RejectedForeignPid==0           — no PID cross-talk into this cell.
//  3. ResultValid (and ResultErr==nil) — result.v2 is schema-valid.
type CellResult struct {
	Task string
	Mode string

	// Merged is the 2-leg merged trace produced by trace.Merge.
	Merged trace.MergedTrace
	// VerifyExitCode is verify.sh's exit code (0 = pass) — the outcome source.
	VerifyExitCode int

	// Nyquist signal 1: tool-call continuity + CC-leg presence.
	ToolCallTotal int
	CCLegPresent  bool
	// Nyquist signal 2: PID cross-talk gate.
	RejectedForeignPid int
	// Nyquist signal 3: result.v2 schema validity.
	ResultValid bool

	// Durable artifact paths under <OutDir>/<task>/<mode>/.
	ResultPath      string
	MergedTracePath string

	// ScratchDir is the ephemeral OS-temp sandbox root. On a successful cell it
	// has been removed; on failure it is PRESERVED (D-08) and this path points
	// at the retained dir for debugging.
	ScratchDir string
	// ScratchPreserved is true when the cell failed and scratch was kept.
	ScratchPreserved bool
}

// validateCellKey rejects task/benchmark/mode names that could escape the cell
// out dir via path traversal (V5 / T-77-08). Mirrors runner.validateTaskID; must
// run BEFORE any filepath.Join with these segments.
func validateCellKey(name, kind string) error {
	if name == "" {
		return fmt.Errorf("%s is empty", kind)
	}
	if name != filepath.Clean(name) || strings.ContainsAny(name, `/\`) || strings.HasPrefix(name, ".") {
		return fmt.Errorf("%s %q contains path separators, parent refs, or a leading dot", kind, name)
	}
	return nil
}

// RunCell executes one <run_id>/<task>/<mode> cell end-to-end and returns its
// CellResult (with the three Nyquist signals populated). The returned error is
// non-nil only for infrastructure failures (resolver/sandbox/daemon spawn/merge/
// build/write); a verify-failed or tool-errored run is a normal CellResult with
// the outcome reflected in Merged.Outcome and VerifyExitCode.
//
// D-08 preserve-on-failure: the ephemeral scratch sandbox is removed only when
// the cell completes without an infrastructure error. On any such error the
// scratch dir is PRESERVED and surfaced via the returned cfg-derived path (logged
// here and recorded in CellResult.ScratchDir when a CellResult is available).
func RunCell(ctx context.Context, cfg CellConfig) (CellResult, error) {
	res := CellResult{Task: cfg.Task, Mode: cfg.Mode}

	// V5/T-77-08: validate every path segment BEFORE any join.
	if err := validateCellKey(cfg.Task, "task"); err != nil {
		return res, fmt.Errorf("bench/runtime: %w", err)
	}
	if err := validateCellKey(cfg.Mode, "mode"); err != nil {
		return res, fmt.Errorf("bench/runtime: %w", err)
	}
	if err := validateCellKey(cfg.Benchmark, "benchmark"); err != nil {
		return res, fmt.Errorf("bench/runtime: %w", err)
	}
	if cfg.HelixBin == "" {
		return res, errors.New("bench/runtime: empty helix binary path")
	}

	// IN-05: the durable artifact path below is <OutDir>/<task>/<mode>/ with NO
	// run-index segment. cfg.RunIndex is metadata-only this phase (it flows into
	// result.v2's run_index field), so two cells that share an OutDir AND differ
	// only by RunIndex would overwrite the same two files. Single-rep per OutDir is
	// the only layout Phase 77 supports: repetitions (pass@k / multi-run) MUST be
	// given distinct OutDirs by the caller. Threading RunIndex into the path
	// (<task>/<mode>/<run_index>/) is deferred to Phase 79 when repetitions land.
	res.ResultPath = filepath.Join(cfg.OutDir, cfg.Task, cfg.Mode, "result.v2.json")
	res.MergedTracePath = filepath.Join(cfg.OutDir, cfg.Task, cfg.Mode, "trace.json")

	// (1) Resolve mode -> profile (D-05).
	var (
		profileName string
		err         error
	)
	if cfg.RunnersRoot != "" {
		profileName, err = runners.ResolveProfileFromRoot(cfg.RunnersRoot, cfg.Mode)
	} else {
		profileName, err = runners.ResolveProfile(cfg.Mode)
	}
	if err != nil {
		return res, fmt.Errorf("bench/runtime: resolve profile for mode %q: %w", cfg.Mode, err)
	}

	// (2) Bench sandbox (D-07): ephemeral OS-temp scratch for HOME/repo/socket;
	// durable artifacts go under cfg.OutDir.
	sb, err := benchsandbox.New(cfg.RunID, cfg.HelixBin, cfg.OutDir)
	if err != nil {
		return res, fmt.Errorf("bench/runtime: create sandbox: %w", err)
	}
	res.ScratchDir = sb.Root

	// preserve-on-failure gate: any infra error past this point leaves scratch
	// in place. cleanupOnSuccess is flipped to call sb.Cleanup() only on the
	// fully-successful path.
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
		// D-08: do NOT Cleanup; surface the preserved path for debugging.
		fmt.Fprintf(os.Stderr, "bench/runtime: cell %s/%s failed; preserving scratch at %s: %v\n",
			cfg.Task, cfg.Mode, sb.Root, e)
		return res, e
	}

	if err := sb.Prepare(cfg.Task, cfg.Mode); err != nil {
		return preserve(fmt.Errorf("bench/runtime: prepare cell: %w", err))
	}
	if err := sb.CloneRepo(cfg.SeedDir, cfg.Task, cfg.Mode); err != nil {
		return preserve(fmt.Errorf("bench/runtime: clone seed repo: %w", err))
	}

	// (3) Spawn the per-cell daemon (D-06) and capture its PID IMMEDIATELY for
	// the PID-gated tap (before any Kill — criterion #4).
	//
	// Per-cell config (T-77 isolation): the semantic_index.store.path default is
	// cwd-relative (".helix/semantic.duckdb"), so without an override every
	// parallel daemon would open the SAME DuckDB file under the process cwd and
	// deadlock on the file lock (criterion #1 --parallel must not collide). We
	// pin an ABSOLUTE per-cell store path under the cell's isolated HOME so each
	// daemon's semantic store is private — preserving the bench-full control arm
	// (semantic enabled) while keeping cells hermetic.
	cfgPath, err := writeCellConfig(sb, cfg.Task, cfg.Mode, profileName)
	if err != nil {
		return preserve(fmt.Errorf("bench/runtime: write cell config: %w", err))
	}

	// Capture the span start BEFORE StartDaemon (which blocks up to 10s polling
	// for the socket). Anchoring DurationMs after the expensive daemon boot would
	// understate the wall-clock span and let daemon-side activation/warm-up
	// tool_calls (emitted during the socket-wait window) sort before StartedAt
	// (WR-06). start therefore measures the full harness span including daemon
	// boot; it is passed as MergeInput.StartedAt.
	start := time.Now()

	h, err := subprocess.StartDaemon(ctx, sb, cfg.Task, cfg.Mode, profileName, cfgPath)
	if err != nil {
		return preserve(fmt.Errorf("bench/runtime: start daemon: %w", err))
	}
	daemonPID := h.Pid()
	if daemonPID <= 0 {
		_ = h.Kill()
		return preserve(fmt.Errorf("bench/runtime: daemon returned non-positive pid %d", daemonPID))
	}

	// (4) Drive the agent over the per-cell socket. The default scripted path (the
	// CI gate) replays scripted_agent.yaml through the forwarder; the claude path
	// (D-01, wired-not-gating) spawns the real claude CLI against the same socket
	// via the per-mode MCP config. Both leave the daemon's tool_calls in
	// daemon.log for the PID-gated tap; only the CC (agent-tap) leg differs. A
	// drive failure is non-fatal — we still kill+tap+merge so a partial run is
	// observable.
	repoDir := sb.RepoFor(cfg.Task, cfg.Mode)
	var steps []runner.StepResult
	switch cfg.Agent {
	case "", "scripted":
		script, lerr := runner.LoadScript(filepath.Join(repoDir, "scripted_agent.yaml"))
		if lerr != nil {
			_ = h.Kill()
			return preserve(fmt.Errorf("bench/runtime: load scripted_agent.yaml: %w", lerr))
		}
		var driveErr error
		steps, driveErr = driveScript(ctx, cfg.HelixBin, sb.SocketFor(cfg.Task, cfg.Mode), repoDir, script)
		if driveErr != nil {
			fmt.Fprintf(os.Stderr, "bench/runtime: cell %s/%s drive error (continuing to tap): %v\n",
				cfg.Task, cfg.Mode, driveErr)
		}
	case "claude":
		// D-01 wired-not-gating: drive the real claude CLI. Its autonomous
		// tool-calls land in daemon.log (tapped below); the scripted StepResults
		// are empty, so the CC leg is synthesized from the daemon side only. A
		// missing claude binary surfaces as subprocess.ErrClaudeNotFound.
		if _, cerr := subprocess.StartClaude(ctx, sb, subprocess.ClaudeConfig{
			HelixBin:     cfg.HelixBin,
			TaskID:       cfg.Task,
			Mode:         cfg.Mode,
			Prompt:       cfg.Prompt,
			MaxToolCalls: claudeMaxToolCalls,
		}); cerr != nil {
			_ = h.Kill()
			return preserve(fmt.Errorf("bench/runtime: drive claude: %w", cerr))
		}
	default:
		_ = h.Kill()
		return preserve(fmt.Errorf("bench/runtime: unknown agent %q (want scripted|claude)", cfg.Agent))
	}

	// (5) Terminate the daemon, THEN PID-gated tap (D-08). The tap reads
	// daemon.log AFTER the daemon exits; its correctness depends on the daemon
	// writing tool_call log lines synchronously/unbuffered at the app layer (NOT
	// on the kill "flushing" anything — SIGKILL terminates immediately and cannot
	// flush userspace buffers). If daemon log buffering is ever introduced, this
	// tap breaks and the kill-then-tap ordering must be revisited.
	if err := h.Kill(); err != nil {
		return preserve(fmt.Errorf("bench/runtime: kill daemon: %w", err))
	}
	daemonLog := filepath.Join(sb.ModeDir(cfg.Task, cfg.Mode), "daemon.log")
	tap, err := trace.TapDaemonLog(daemonLog, daemonPID)
	if err != nil {
		return preserve(fmt.Errorf("bench/runtime: tap daemon log: %w", err))
	}

	// (6) Run verify.sh in the repo working copy; its exit code is the outcome.
	// A ctx cancellation/timeout during verify is an INFRASTRUCTURE error (the
	// harness was cancelled), NOT a task failure — route it through preserve so it
	// is not conflated with a genuine verify failure in the result.v2 (WR-01).
	verifyExit, verifyErr := runVerify(ctx, filepath.Join(repoDir, "verify.sh"), repoDir)
	if verifyErr != nil {
		return preserve(fmt.Errorf("bench/runtime: verify: %w", verifyErr))
	}
	res.VerifyExitCode = verifyExit

	// (7) Synthesize the CC (agent-tap) leg from the scripted StepResults (D-02).
	cc := SynthCCTap(steps)

	// (8) 2-leg merge (Daemon + synth CC) + outcome resolution (D-02/D-04).
	merged, err := trace.Merge(trace.MergeInput{
		TaskID:         cfg.Task,
		Mode:           cfg.Mode,
		RunID:          cfg.RunID,
		StartedAt:      start,
		EndedAt:        time.Now(),
		Daemon:         tap,
		CC:             cc,
		VerifyExitCode: verifyExit,
		RepoRoot:       repoDir,
	})
	if err != nil {
		return preserve(fmt.Errorf("bench/runtime: merge trace: %w", err))
	}
	res.Merged = merged

	// Nyquist signals 1 + 2.
	res.ToolCallTotal = merged.ToolCallSummary.Total
	res.CCLegPresent = ccLegPresent(merged)
	res.RejectedForeignPid = tap.RejectedForeignPid

	// (9) Build + validate result.v2 (D-04). outcome from merged.Outcome; tokens
	// 0 for the scripted gate (Pitfall 6); fairness from DefaultContract.
	resultBytes, err := BuildResult(ResultInput{
		TaskID:    cfg.Task,
		Mode:      cfg.Mode,
		Benchmark: cfg.Benchmark,
		RunIndex:  cfg.RunIndex,
		Outcome:   merged.Outcome,
		TraceRef:  res.MergedTracePath,
		Fairness:  runners.DefaultContract,
	})
	if err != nil {
		return preserve(fmt.Errorf("bench/runtime: build result.v2: %w", err))
	}
	if err := Validate(resultBytes); err != nil {
		return preserve(fmt.Errorf("bench/runtime: result.v2 invalid: %w", err))
	}
	res.ResultValid = true // Nyquist signal 3

	// (10) Write durable artifacts under <out>/<task>/<mode>/ (D-08).
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

	// (11) Success: delete ephemeral scratch (D-08 — preserved only on failure).
	cleanup()
	return res, nil
}

// runVerify executes verify.sh with cwd=repoDir and returns its exit code
// (0 = pass) plus an infrastructure error. A missing verify.sh auto-passes
// (0, nil). Mirrors internal/eval/runner.runVerify; the seed task's verify.sh
// runs `go test`.
//
// WR-01: a ctx cancellation/timeout is distinguished from a real non-zero verify
// exit. When ctx is done the context kills the process and cmd.Run returns a
// non-ExitError (or a SIGKILL-coded ExitError); mapping that to a fabricated exit
// code would record a harness cancellation as a task FAILURE. Instead the context
// error is returned as an infra error so the caller routes it through the
// preserve-on-failure path rather than into trace.Merge.
func runVerify(ctx context.Context, scriptPath, repoDir string) (int, error) {
	if _, err := os.Stat(scriptPath); err != nil {
		return 0, nil // no verify.sh — auto-pass
	}
	cmd := exec.CommandContext(ctx, "/bin/sh", scriptPath)
	cmd.Dir = repoDir
	err := cmd.Run()
	if ctx.Err() != nil {
		// Cancellation/timeout: infra error, not a task outcome.
		return 0, fmt.Errorf("verify cancelled/timed out: %w", ctx.Err())
	}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode(), nil
		}
		return 1, nil // non-ExitError failure (e.g. /bin/sh not found) — treat as fail
	}
	return 0, nil
}

// ccLegPresent reports whether the merged trace carries at least one CC-side
// event (Source == "cc") — the agent-tap leg presence check for Nyquist signal 1.
// An exit-code-only smoke would pass even if the CC leg silently dropped; this
// makes its presence an explicit assertion.
func ccLegPresent(merged trace.MergedTrace) bool {
	for _, ev := range merged.Events {
		if ev.Source == "cc" {
			return true
		}
	}
	return false
}

// writeCellConfig writes a per-cell helix_config.yml into the cell HOME and
// returns its path (for StartDaemon's --config=). It pins the resolved profile
// and disables the semantic index for the cell daemon.
//
// Why disable semantic_index here (T-77 isolation, criterion #1):
// internal/semantic/store rejects absolute store paths (T-57-02-01) and the
// store is opened EAGERLY at daemon startup (daemon.go:318) relative to the
// daemon's CWD — before any workspace is activated. With the cwd-relative
// default (".helix/semantic.duckdb"), every parallel cell daemon opens the SAME
// DuckDB file under the shared process cwd and deadlocks on its file lock,
// breaking "no collisions on --parallel" (criterion #1). eval's StartDaemon does
// not expose cmd.Dir, and D-07 forbids forking it, so the hermetic per-cell fix
// is to set semantic_index.enabled=false (daemon.go:308 makes the store nil and
// the daemon proceeds). This affects only the daemon's semantic INFRA, not the
// bench-full TOOL surface (still applied via --profile=bench-full); the Phase 77
// seed task drives a text-level replace_in_file edit that needs no semantic
// store. Phase 78+ (corpus needing semantic tools) can revisit by spawning the
// daemon with a per-cell cwd.
func writeCellConfig(sb *benchsandbox.Sandbox, task, mode, profileName string) (string, error) {
	home := sb.HomeFor(task, mode)
	cfgDir := filepath.Join(home, ".helix")
	if err := os.MkdirAll(cfgDir, 0700); err != nil {
		return "", fmt.Errorf("mkdir cell config dir: %w", err)
	}
	cfgPath := filepath.Join(cfgDir, "helix_config.yml")
	cfg := fmt.Sprintf("profile: %s\nsemantic_index:\n  enabled: false\n", profileName)
	if err := os.WriteFile(cfgPath, []byte(cfg), 0600); err != nil {
		return "", fmt.Errorf("write cell config: %w", err)
	}
	return cfgPath, nil
}

// marshalTrace serializes the merged trace as indented JSON for the durable
// trace.json artifact (trace_ref target in result.v2).
func marshalTrace(merged trace.MergedTrace) ([]byte, error) {
	b, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal merged trace: %w", err)
	}
	return b, nil
}

// writeDurable writes b to path, creating parent dirs (0700) as needed.
func writeDurable(path string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("mkdir %q: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, b, 0600); err != nil {
		return fmt.Errorf("write %q: %w", path, err)
	}
	return nil
}

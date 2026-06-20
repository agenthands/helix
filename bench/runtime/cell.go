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
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/agenthands/helix/bench/evaluators/coordinator"
	"github.com/agenthands/helix/bench/languages"
	_ "github.com/agenthands/helix/bench/languages/go" // Caddy-style init() registers the Go runner for (internal-toolbench, go) (D-10)
	"github.com/agenthands/helix/bench/runners"
	benchsandbox "github.com/agenthands/helix/bench/runtime/sandbox"
	"github.com/agenthands/helix/bench/runtime/subprocess"
	"github.com/agenthands/helix/internal/eval/runner"
	evalsandbox "github.com/agenthands/helix/internal/eval/sandbox"
	"github.com/agenthands/helix/internal/eval/trace"
)

// claudeMaxToolCalls bounds the wired-not-gating claude run via --max-turns. The
// seed task is a single edit; a small bound keeps a local claude run from looping.
const claudeMaxToolCalls = 20

// noSemanticMode is the bench-mode name of the semantic-store-disabled ablation
// arm. The kernel gate (Plan 04, effSemanticDisabled) forces NoopLookup for this
// arm, so its semantic-store read counter MUST be 0; the cell asserts that
// (assertNoSemanticReads) and fails hard on any violation. The Phase 80 "pending"
// ablation-status deferral marker is GONE — the kernel disable_semantic_subsystem
// guarantee (ABLATE-06) lands this phase, so the no_semantic arm no longer emits a
// partial-row marker; its result.v2 ablation status field is now empty like every
// honest mode (omitempty drops the key).
const noSemanticMode = "your_agent_no_semantic"

// daemonReadsTotalMsg is the slog msg the daemon emits on a single shutdown log
// line carrying the semantic-store read counter value (Phase 81 ABLATE-06, Task
// 0 path A). The bench daemon runs HTTP-disabled over a Unix socket (D-06), so
// the Prometheus /metrics scrape is unreachable; scrapeSemanticReadsTotal parses
// this line from daemon.log instead. MUST match the msg string in
// internal/daemon/shutdown.go.
const daemonReadsTotalMsg = "semantic store reads total"

// daemonGracefulStopTimeout bounds the graceful SIGTERM teardown RunCell attempts
// before falling back to the hard Kill. It is sized off the daemon's WORST-CASE
// shutdown wall-time, not just the kernel drain (WR-01): d.shutdown() first runs
// the kernel Shutdown(ctx) bounded by Daemon.ShutdownTimeout (default 10s;
// internal/daemon/shutdown.go:13-16), THEN flushes the trace exporter under a
// SEPARATE 5s context (internal/daemon/shutdown.go:43), and only afterwards emits
// the "semantic store reads total" line (shutdown.go:60-62). Worst case is
// therefore ~10s (kernel) + ~5s (trace flush) ≈ 15s, plus the post-flush emit and
// listener close. The 20s budget = 10s + 5s + 5s slack, so a daemon that uses its
// entire kernel-drain window AND a slow trace flush still emits the proof line
// gracefully BEFORE the Kill fallback reaps it (Phase 81 ABLATE-06, WR-01/WR-02).
const daemonGracefulStopTimeout = 20 * time.Second

// scrapeSemanticReadsTotal reads the semantic-store read counter value the daemon
// emits at shutdown (Task 0 path A) from a daemon JSONL log file. It scans for the
// last line whose msg == daemonReadsTotalMsg and reports BOTH its count field AND
// whether such a line was present at all.
//
// present is the load-bearing fail-CLOSED signal (Phase 81-06, WR-02): under the
// graceful teardown (RunCell calls DaemonHandle.Stop before Kill) the daemon ALWAYS
// reaches d.shutdown() and emits this line whenever d.obs != nil — so on a real
// no_semantic run the line is expected to be present. An ABSENT line therefore no
// longer means "zero reads"; it means the daemon never reached graceful shutdown
// and the zero-reads PROOF DID NOT RUN. The caller (assertNoSemanticReads) turns an
// absent line into a HARD failure on the no_semantic arm — the missing line can no
// longer masquerade as a clean count=0 (the root fail-open anti-pattern flagged at
// 81-VERIFICATION.md:127 is removed). Malformed/truncated lines are skipped
// defensively (mirrors trace.TapDaemonLog).
func scrapeSemanticReadsTotal(daemonLogPath string) (count int, present bool, err error) {
	f, ferr := os.Open(daemonLogPath)
	if ferr != nil {
		return 0, false, fmt.Errorf("scrape semantic reads total: open %q: %w", daemonLogPath, ferr)
	}
	defer f.Close()

	// WR-03: decode count as *int so a msg-matching line whose `count` is absent,
	// JSON-null, or non-integer is treated as MALFORMED (present stays false) and
	// the fail-closed gate stays honest — a garbled count must NOT masquerade as a
	// clean count=0. NOTE: this parser is duplicated by scanReadsTotalLine in
	// bench/runtime/no_semantic_emission_integration_test.go (IN-03); the two MUST
	// stay in lockstep — any change here (struct, *int handling, last-line-wins)
	// must be mirrored there.
	type readsTotalLine struct {
		Msg   string `json:"msg"`
		Count *int   `json:"count"`
	}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 256*1024), 256*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var raw readsTotalLine
		if jerr := json.Unmarshal(line, &raw); jerr != nil {
			continue // truncated/malformed line — skip defensively
		}
		if raw.Msg == daemonReadsTotalMsg {
			if raw.Count == nil {
				continue // msg matched but count missing/non-integer — not a valid proof line
			}
			count = *raw.Count // last occurrence wins (the final shutdown line)
			present = true     // the proof line was emitted (graceful shutdown ran)
		}
	}
	if serr := scanner.Err(); serr != nil {
		return 0, false, fmt.Errorf("scrape semantic reads total: scan %q: %w", daemonLogPath, serr)
	}
	return count, present, nil
}

// assertNoSemanticReads is the fail-CLOSED runtime verification (D-05, criterion
// #2): on the no_semantic ablation arm the semantic-store read counter MUST be 0
// (the kernel gate of Plan 04 forces NoopLookup, so a real read should be
// impossible). It hard-fails the arm on EITHER of two conditions:
//
//   - present == false: the reads-total proof line was ABSENT from daemon.log.
//     Under the graceful teardown (RunCell's DaemonHandle.Stop before Kill) the
//     daemon always reaches d.shutdown() and emits this line, so its absence means
//     the daemon never shut down gracefully and the zero-reads guarantee was NEVER
//     PROVEN. This is the WR-02 fail-CLOSED inversion of the old silent count=0: an
//     unproven run can no longer masquerade as a proven one.
//   - reads != 0: a read SURVIVED the gate — the central integrity threat the
//     phase exists to close (T-81-05-01).
//
// Both return an error RunCell turns into a HARD cell failure (not a warn). For
// every other mode the assertion is a no-op (scope guard, Test 3): a non-zero read
// count is expected off the no_semantic arm, and an absent line there is benign.
func assertNoSemanticReads(mode string, reads int, present bool) error {
	if mode != noSemanticMode {
		return nil
	}
	if !present {
		return fmt.Errorf(
			"no_semantic ablation UNPROVEN: the helix_semantic_store_reads_total proof line was ABSENT " +
				"from daemon.log; the daemon did not reach graceful shutdown (d.shutdown()) so the zero-reads " +
				"guarantee was never emitted — the no_semantic arm result is INVALID and the cell fails hard " +
				"(fail-CLOSED, ABLATE-06, D-05, WR-02)")
	}
	if reads != 0 {
		return fmt.Errorf(
			"no_semantic ablation violated: helix_semantic_store_reads_total == %d (want 0); "+
				"a semantic-store read survived the kernel disable_semantic_subsystem gate (Plan 04) — "+
				"the no_semantic arm result is INVALID and the cell fails hard (ABLATE-06, D-05)",
			reads)
	}
	return nil
}

// CellConfig is the full set of inputs to run one <run_id>/<task>/<mode> cell.
type CellConfig struct {
	// RunID is the run identifier (eval shape, e.g. "20060102T150405Z"). It is
	// metadata only here; the durable out dir is OutDir (already run-scoped).
	RunID string
	// Benchmark is the benchmark suite name (e.g. "internal-toolbench").
	Benchmark string
	// Language is the D-07 language axis (e.g. "go"). It selects the
	// LanguageRunner via languages.RunnerFor(Benchmark, Language) (D-10) and is
	// validated against path traversal (T-78-03).
	Language string
	// Task is the task id (e.g. "IT-go-patch-apply-1"). Validated against path
	// traversal.
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
	// copy (e.g. bench/datasets/internal-toolbench/go/IT-go-patch-apply-1). It
	// must contain the scripted_agent.yaml the driver replays and the verify.sh
	// outcome script.
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

	// StoreOptIn opts this cell's per-cell semantic store ON (D-01/D-02). When
	// false (the default), writeCellConfig writes semantic_index.enabled=false so
	// the daemon never opens a DuckDB store (the parallel-safe default). When
	// true, the config enables the index AND StartDaemon is given
	// WithWorkingDir(repoDir) so the cwd-relative store path
	// (".helix/semantic.duckdb") resolves per-cell — keeping store-on cells
	// hermetic (D-03 substrate; the parallel isolation assertion lands in Plan 04).
	StoreOptIn bool
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

	// SemanticStoreReads is the helix_semantic_store_reads_total counter value the
	// daemon emitted at shutdown (Task 0 path A), scraped from daemon.log. On the
	// no_semantic arm it MUST be 0 (the Plan 04 kernel gate forces NoopLookup);
	// off that arm it is informational. (D-05, criterion #2.)
	SemanticStoreReads int
	// SemanticReadViolation is the HARD-fail signal for the no_semantic arm: true
	// when SemanticStoreReads > 0 on the no_semantic mode, i.e. a semantic-store
	// read survived the kernel gate. RunCell returns a non-nil error in this case
	// (the cell is a failure, not a success) and preserves scratch for debugging
	// (T-81-05-01). Always false off the no_semantic arm.
	SemanticReadViolation bool

	// Deferred is the D-02 fail-close signal: the cell short-circuited BEFORE any
	// sandbox/daemon and produced NO result.v2.json (currently only baseline_rag,
	// the registered fail-closed stub deferred to Phase 83). A deferred cell returns
	// a nil error (it is a registered stub, not an infra failure) and is neither a
	// success (ResultValid stays false) nor an infra error at the matrix layer.
	Deferred bool
	// DeferredReason is the human-readable reason the cell was deferred (names the
	// phase the real arm lands in). Empty when Deferred is false.
	DeferredReason string
}

// validateCellKey rejects task/benchmark/mode names that could escape the cell
// out dir via path traversal (V5 / T-77-08). Mirrors runner.validateTaskID; must
// run BEFORE any filepath.Join with these segments. IN-05: the predicate body is
// shared with the matrix layer via validatePathSegment so the two cannot drift;
// this wrapper preserves the cell-layer error prose.
func validateCellKey(name, kind string) error {
	return validatePathSegment(name, kind)
}

// cellDurablePaths computes the durable result.v2.json + trace.json paths for a
// cell, threading the <run_index> segment into the layout
// (<OutDir>/<task>/<mode>/<run_index>/...) per Pitfall 3 (per (task, mode,
// run_index)). The task/mode segments are assumed already V5-validated by the
// caller (RunCell validates them up front); the run_index segment is formatted
// via strconv.Itoa and routed through validateRunIndexSegment (V5 / T-79-04-01)
// BEFORE the join so a malformed numeric segment can never become a path.
func cellDurablePaths(outDir, task, mode string, runIndex int) (resultPath, tracePath string, err error) {
	seg := strconv.Itoa(runIndex)
	if err := validateRunIndexSegment(seg); err != nil {
		return "", "", err
	}
	resultPath = filepath.Join(outDir, task, mode, seg, "result.v2.json")
	tracePath = filepath.Join(outDir, task, mode, seg, "trace.json")
	return resultPath, tracePath, nil
}

// validateRunIndexSegment guards the run_index path segment (T-79-04-01). A
// run_index is a non-negative integer; a negative value formats with a leading
// "-" (e.g. "-1") which is neither a clean nor a positive numeric segment, so it
// is rejected before it can become a path. The formatted segment is then routed
// through the shared validatePathSegment predicate (rejects "..", separators, a
// leading dot — reused from validate.go so the run_index segment cannot drift
// from the task/mode/benchmark guards).
func validateRunIndexSegment(seg string) error {
	if seg == "" || seg[0] == '-' {
		return fmt.Errorf("run_index segment %q is not a non-negative integer", seg)
	}
	return validatePathSegment(seg, "run_index")
}

// prePatchSnapshot captures a pre-patch test outcome for regression_checker's
// pre-patch passing set (D-05 / Pitfall 6). It runs the structured
// LanguageRunner's Setup then RunTests against repoDir and returns the outcome.
// A nil runner (no structured runner registered — the verify.sh fallback path)
// yields a nil outcome and no error: there is no pre-patch test snapshot to take.
// MUST be called BEFORE the agent's edit (driveScript) so the captured set
// reflects the repo as it stood before the patch.
func prePatchSnapshot(ctx context.Context, r languages.LanguageRunner, repoDir string) (*languages.TestOutcome, error) {
	if r == nil {
		return nil, nil
	}
	if err := r.Setup(ctx, repoDir); err != nil {
		return nil, fmt.Errorf("pre-patch runner setup: %w", err)
	}
	out, err := r.RunTests(ctx, repoDir)
	if err != nil {
		return nil, fmt.Errorf("pre-patch run tests: %w", err)
	}
	return &out, nil
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
	if err := validateCellKey(cfg.Language, "language"); err != nil {
		return res, fmt.Errorf("bench/runtime: %w", err)
	}
	if cfg.HelixBin == "" {
		return res, errors.New("bench/runtime: empty helix binary path")
	}

	var err error

	// Pitfall 3: the durable artifact path is now keyed per (task, mode,
	// run_index): <OutDir>/<task>/<mode>/<run_index>/{result.v2.json,trace.json}.
	// cfg.RunIndex is threaded into the path (no longer metadata-only), so two
	// cells sharing an OutDir that differ only by RunIndex land in distinct dirs
	// and no longer overwrite one another (Phase 79 repetitions / pass@k). The
	// run_index segment is V5-guarded inside cellDurablePaths (T-79-04-01).
	res.ResultPath, res.MergedTracePath, err = cellDurablePaths(cfg.OutDir, cfg.Task, cfg.Mode, cfg.RunIndex)
	if err != nil {
		return res, fmt.Errorf("bench/runtime: %w", err)
	}

	// (1) Resolve mode -> profile (D-05).
	var profileName string
	if cfg.RunnersRoot != "" {
		profileName, err = runners.ResolveProfileFromRoot(cfg.RunnersRoot, cfg.Mode)
	} else {
		profileName, err = runners.ResolveProfile(cfg.Mode)
	}
	if err != nil {
		return res, fmt.Errorf("bench/runtime: resolve profile for mode %q: %w", cfg.Mode, err)
	}

	// (1b) D-04 startup fairness gate: the single compile-time contract every
	// runner shares is validated UNCONDITIONALLY right after profile resolution and
	// BEFORE any sandbox/daemon. A non-nil return means an override deviates from the
	// shared budget without a WaiverReason — an unfair benchmark — so we refuse to
	// run it (fatal, no daemon spawned). Validate() is pure + CI-cheap (a loop over a
	// compile-time map); the committed DefaultContract has no overrides so this gate
	// never fatals in CI under the current contract. (Open Q1 scope A: gate here
	// unconditionally; the always-on CI contract test is Plan 04's separate guarantee.)
	if verr := runners.DefaultContract.Validate(); verr != nil {
		return res, fmt.Errorf("bench/runtime: fairness contract invalid: %w", verr)
	}

	// (1c) D-02 baseline_rag fail-close: baseline_rag is a registered stub whose
	// REAL RAG arm (chromem-go + cmd/helix-bench-rag) is deferred to Phase 83
	// (ABLATE-04). It must NOT spawn a daemon or emit a result row this phase, so we
	// short-circuit BY MODE NAME (not a frontmatter marker — keeps the two-key
	// resolver change-free) right after the fairness gate and BEFORE benchsandbox.New.
	// The path-segment validation above (and ResultPath layout) already ran; we set
	// the Deferred signal and return a nil error (a registered stub, not an infra
	// failure) so the matrix counts it as neither a success nor an infra error.
	if cfg.Mode == "baseline_rag" {
		res.Deferred = true
		res.DeferredReason = "baseline_rag: real RAG arm deferred to Phase 83 (ABLATE-04)"
		return res, nil
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
	// Per-cell config (T-77 isolation, D-01/D-02/D-03): the semantic_index store
	// is OFF by default (writeCellConfig emits enabled=cfg.StoreOptIn). The store
	// path default is cwd-relative (".helix/semantic.duckdb"); a store-on cell
	// without a per-cell cwd would open the SAME DuckDB file under the shared
	// process cwd and deadlock on the file lock (criterion #1 --parallel must not
	// collide). So a StoreOptIn cell ALSO passes WithWorkingDir(repoDir) into
	// StartDaemon below, resolving the cwd-relative store under the cell's own repo
	// — hermetic per cell. Store-off cells (the default) open no store at all.
	cfgPath, err := writeCellConfig(sb, cfg.Task, cfg.Mode, profileName, cfg.StoreOptIn)
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

	// D-03: a StoreOptIn cell runs its daemon with cwd=repoDir so the cwd-relative
	// store path (".helix/semantic.duckdb") resolves per-cell. Store-off cells pass
	// no option (cmd.Dir stays the harness cwd; no store is opened either way).
	var daemonOpts []evalsandbox.DaemonOption
	if cfg.StoreOptIn {
		daemonOpts = append(daemonOpts, evalsandbox.WithWorkingDir(sb.RepoFor(cfg.Task, cfg.Mode)))
	}

	h, err := subprocess.StartDaemon(ctx, sb, cfg.Task, cfg.Mode, profileName, cfgPath, daemonOpts...)
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

	// D-05 / Pitfall 6: capture a PRE-PATCH test snapshot BEFORE driving the
	// agent's edit, so regression_checker has the pre-patch passing set to diff
	// the post-patch outcome against. When no structured runner is registered
	// (verify.sh fallback) there is no pre-patch snapshot (nil) and regression_rate
	// is left null by the coordinator. A snapshot error is an INFRASTRUCTURE
	// failure (ctx cancellation/runner setup) routed through preserve like the
	// post-patch RunTests below (WR-01).
	langRunner := languages.RunnerFor(cfg.Benchmark, cfg.Language)
	prePatch, ppErr := prePatchSnapshot(ctx, langRunner, repoDir)
	if ppErr != nil {
		_ = h.Kill()
		return preserve(fmt.Errorf("bench/runtime: pre-patch snapshot: %w", ppErr))
	}

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
		// tool-calls land in daemon.log and ARE observable via the PID-gated
		// daemon tap below. WR-03: `steps` stays nil for the claude path, and
		// SynthCCTap reads ONLY `steps` (it does not consult the daemon leg), so
		// the synthesized CC leg currently carries NO claude tool activity — it is
		// a content-free SessionInit+Result pair. With the WR-02 fix, ccLegPresent
		// therefore reports CCLegPresent == false for a claude cell (the cc leg has
		// no tool event), which is accurate: the claude agent's tool activity is
		// visible only on the daemon leg this phase. Threading the returned
		// *agent.Result into the CC synth (so the claude tool-uses populate the cc
		// leg) is deferred to whoever finishes the claude path in a later phase;
		// the result is intentionally discarded until then. A missing claude binary
		// surfaces as subprocess.ErrClaudeNotFound.
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

	// (5) Terminate the daemon, THEN PID-gated tap (D-08). Teardown is now a
	// graceful Stop (SIGTERM + bounded wait) FOLLOWED BY a Kill fallback:
	//
	//   - h.Stop sends SIGTERM to the daemon group, which the daemon's
	//     signal.NotifyContext catches → g.Wait() unblocks → d.shutdown() runs and
	//     FLUSHES the "semantic store reads total" line (shutdown.go:60-62). This is
	//     the WR-02 fix: SIGKILL is untrappable and could never let shutdown() emit
	//     that line, so on every prior real run the reads-total proof was missing and
	//     the no_semantic gate passed vacuously. Graceful Stop makes the line real.
	//   - h.Kill is the hard fallback that reaps a non-graceful / straggler daemon
	//     (and any descendant LS / `go test`) via the group SIGKILL + buffered drain.
	//     It runs unconditionally after Stop so a daemon that did NOT exit gracefully
	//     is still reaped (and Kill on an already-exited process is a clean no-op).
	//
	// The tap still reads daemon.log AFTER the daemon has fully exited; tool_call
	// lines are written synchronously/unbuffered at the app layer, and the
	// reads-total line is now flushed by the graceful shutdown above (not by any
	// kill "flush" — Kill remains a hard, non-flushing terminator).
	graceful, stopErr := h.Stop(daemonGracefulStopTimeout)
	if stopErr != nil {
		fmt.Fprintf(os.Stderr, "bench/runtime: cell %s/%s graceful stop error (falling back to Kill): %v\n",
			cfg.Task, cfg.Mode, stopErr)
	}
	if !graceful || stopErr != nil {
		// Non-graceful (or Stop errored): the daemon did not drain within the
		// window — reap it hard. (On the graceful path Kill is still called below as
		// a no-op reaper to guarantee the process is fully reaped before the tap.)
		fmt.Fprintf(os.Stderr, "bench/runtime: cell %s/%s daemon did not exit gracefully within %s; killing\n",
			cfg.Task, cfg.Mode, daemonGracefulStopTimeout)
	}
	if err := h.Kill(); err != nil {
		return preserve(fmt.Errorf("bench/runtime: kill daemon: %w", err))
	}
	daemonLog := filepath.Join(sb.ModeDir(cfg.Task, cfg.Mode), "daemon.log")
	tap, err := trace.TapDaemonLog(daemonLog, daemonPID)
	if err != nil {
		return preserve(fmt.Errorf("bench/runtime: tap daemon log: %w", err))
	}

	// (5b) Independent runtime verification (D-05, criterion #2): on the
	// no_semantic arm assert helix_semantic_store_reads_total == 0. The daemon
	// emitted its read-counter value on a single shutdown log line (Task 0 path A —
	// HTTP is disabled over the bench Unix socket so the Prometheus scrape is
	// unreachable); scrapeSemanticReadsTotal reads it from daemon.log. The Plan 04
	// kernel gate forces NoopLookup for this arm, so a non-zero count means a read
	// SURVIVED the gate. This is the Plan 04 build-but-block gate's independent
	// proof: the store EXISTS and could be queried, and we verify it was NOT. On a
	// violation the cell FAILS HARD (mirrors the fail-closed shape of the fairness
	// gate above) — a corrupted no_semantic row must never be published
	// (T-81-05-01). Off the no_semantic arm assertNoSemanticReads is a no-op.
	//
	// WR-04 — SCOPE OF THE GUARANTEE: helix_semantic_store_reads_total counts ONLY
	// reads that funnel through the store's counting wrappers in
	// internal/semantic/store/effective_graph.go (s.queryContext / s.queryRowContext).
	// It is NOT a universal witness for "no DB read happened": tx-scoped read paths
	// that call the raw *sql.Tx / *sql.DB methods directly (e.g. snapshot.go,
	// overlay.go tx-scoped reads) are not instrumented and would NOT increment this
	// counter. The zero-reads proof therefore certifies that no read reached the
	// counted effective-graph path on the no_semantic arm — which is the path the
	// Plan 04 build-but-block gate un-wires — not that the database was provably
	// untouched by every conceivable code path. Do not over-trust the counter as a
	// universal read-detector when reasoning about new background read paths.
	semanticReads, readsPresent, srErr := scrapeSemanticReadsTotal(daemonLog)
	if srErr != nil {
		return preserve(fmt.Errorf("bench/runtime: scrape semantic reads total: %w", srErr))
	}
	res.SemanticStoreReads = semanticReads
	if vErr := assertNoSemanticReads(cfg.Mode, semanticReads, readsPresent); vErr != nil {
		res.SemanticReadViolation = true
		fmt.Fprintf(os.Stderr, "bench/runtime: cell %s/%s FAILED no_semantic zero-reads gate: %v\n",
			cfg.Task, cfg.Mode, vErr)
		return preserve(fmt.Errorf("bench/runtime: %w", vErr))
	}

	// (6) Resolve the outcome (D-10 dispatch). When a structured LanguageRunner is
	// registered for (Benchmark, Language), its RunTests provides the outcome
	// (Passed -> exit 0, else non-zero); a non-nil RunTests error is an
	// INFRASTRUCTURE failure (e.g. ctx cancellation/timeout) routed through
	// preserve exactly like a verify infra error (WR-01). When no runner is
	// registered (RunnerFor==nil) the cell falls back to verify.sh, whose exit code
	// is the outcome; a ctx cancellation there is likewise an infra error.
	var verifyExit int
	var postPatch *languages.TestOutcome
	if r := langRunner; r != nil {
		// WR-06: honor the full LanguageRunner lifecycle — Setup (pre-test
		// preparation: dependency fetch, build-cache warm-up) MUST run before
		// RunTests. For the hermetic Go fixture Setup is a no-op (beyond honoring
		// ctx cancellation), but a future runner whose RunTests depends on Setup
		// having run (e.g. fetching modules into an offline cache) would silently
		// fail if the only production caller skipped it. A Setup error is an
		// INFRASTRUCTURE failure routed through preserve like any other. (The
		// pre-patch snapshot above already ran Setup once; re-running is harmless
		// for the hermetic Go runner and keeps the post-patch lifecycle explicit.)
		if serr := r.Setup(ctx, repoDir); serr != nil {
			return preserve(fmt.Errorf("bench/runtime: runner setup: %w", serr))
		}
		outcome, runErr := r.RunTests(ctx, repoDir)
		if runErr != nil {
			return preserve(fmt.Errorf("bench/runtime: run tests: %w", runErr))
		}
		oc := outcome
		postPatch = &oc
		if outcome.Passed {
			verifyExit = 0
		} else {
			verifyExit = 1
		}
	} else {
		var verifyErr error
		verifyExit, verifyErr = runVerify(ctx, filepath.Join(repoDir, "verify.sh"), repoDir)
		if verifyErr != nil {
			return preserve(fmt.Errorf("bench/runtime: verify: %w", verifyErr))
		}
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

	// (8b) Grade the cell (Phase 79 / D-07). The coordinator fans the cell's
	// artifacts out to all 5 graders and returns the full nullable Metrics record
	// + the per-grader failure annotations; it never errors and never drops the
	// row. The post-patch outcome is the structured runner's outcome when one ran,
	// else a synthesized outcome from the verify.sh exit code so task_success is
	// still derivable. usagePresent is the D-01 out-of-band signal: a claude run
	// whose merged trace carries a provider usage block (the scripted Go-ToolBench
	// corpus is usage-absent, so its token metrics are explicit null). Analyze
	// consumes res.Merged directly — NEVER a re-merge (METRIC-06).
	var postOutcome languages.TestOutcome
	if postPatch != nil {
		postOutcome = *postPatch
	} else {
		postOutcome = languages.TestOutcome{Passed: verifyExit == 0, ExitCode: verifyExit}
	}
	// usagePresent is a true presence signal (D-01/D-03), NOT a value threshold:
	// merged.UsagePresent is set by the CC tap when a `result` event carried a
	// provider usage block. A real claude run reporting a genuine all-zero usage
	// block must therefore still be classified usage-present, so token metrics are
	// emitted as pointers-to-0 rather than silently nulled. The scripted corpus
	// leaves UsagePresent false (cctap.go), keeping its token metrics explicit null.
	usagePresent := cfg.Agent == "claude" && merged.UsagePresent
	var prePatchVal languages.TestOutcome
	if prePatch != nil {
		prePatchVal = *prePatch
	}
	metrics, metricErrs := coordinator.Grade(ctx, coordinator.GradeInput{
		TestOutcome:     postOutcome,
		PrePatchOutcome: prePatchVal,
		RepoDir:         repoDir,
		Merged:          merged,
		UsagePresent:    usagePresent,
		Agent:           cfg.Agent,
	})

	// (9) Build + validate result.v2 (D-04). outcome from merged.Outcome; fairness
	// from DefaultContract; the full nullable metrics record + annotations from the
	// coordinator (METRIC-01).
	resultBytes, err := BuildResult(ResultInput{
		TaskID:    cfg.Task,
		Mode:      cfg.Mode,
		Benchmark: cfg.Benchmark,
		RunIndex:  cfg.RunIndex,
		Outcome:   merged.Outcome,
		TraceRef:  res.MergedTracePath,
		Fairness:  runners.DefaultContract,
		// AblationStatus is now empty for EVERY mode: the Phase 80 "pending"
		// deferral marker is removed because the kernel disable_semantic_subsystem
		// guarantee (ABLATE-06) lands this phase. The no_semantic arm's clean
		// measurement is verified by the zero-reads assertion above, not flagged
		// partial. (Empty => omitempty drops the key.)
		AblationStatus: "",
		Metrics:        metrics,
		MetricErrors:   metricErrs,
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

// ccLegPresent reports whether the merged trace carries genuine CC-side TOOL
// activity — the agent-tap leg presence check for Nyquist signal 1.
//
// WR-02: this must assert a cc-side *tool* event, NOT merely any cc event.
// SynthCCTap ALWAYS emits a SessionInit and a Result event (both Source:"cc"),
// even when steps is empty (SynthCCTap(nil) still produces 2 cc events). A
// "any Source==cc" check is therefore structurally vacuous — it can never be
// false, so it would alias an empty/scripted-less run with one that carried real
// tool activity (exactly the exit-code-only aliasing the harness defends against).
// Gating on a cc-side ToolResult (or an AssistantMsg carrying a non-empty
// ToolUses) makes the signal meaningful: an empty-steps run (e.g. the claude
// branch with no scripted steps) correctly reports CCLegPresent == false.
func ccLegPresent(merged trace.MergedTrace) bool {
	for _, ev := range merged.Events {
		if ev.Source != "cc" {
			continue
		}
		if ev.Kind == trace.KindToolResult || len(ev.ToolUses) > 0 {
			return true
		}
	}
	return false
}

// writeCellConfig writes a per-cell helix_config.yml into the cell HOME and
// returns its path (for StartDaemon's --config=). It pins the resolved profile
// and toggles the semantic index from storeOptIn (D-01/D-02).
//
// semantic_index.enabled is parameterized (T-77 isolation, criterion #1):
// internal/semantic/store opens the store EAGERLY at daemon startup relative to
// the daemon's CWD, and the store-path default is cwd-relative
// (".helix/semantic.duckdb"). With the index ENABLED, every cell daemon sharing
// the harness cwd would open the SAME DuckDB file and deadlock on its file lock,
// breaking "no collisions on --parallel". So:
//
//   - storeOptIn=false (the default): emit enabled=false — the store is never
//     opened (daemon.go makes it nil and proceeds). This affects only the
//     daemon's semantic INFRA, not the bench-full TOOL surface (still applied via
//     --profile). The Phase 77 patch_apply seed needs no semantic store.
//   - storeOptIn=true (the D-01/D-02 store-on class, e.g. incremental_update):
//     emit enabled=true. RunCell ALSO passes WithWorkingDir(repoDir) to
//     StartDaemon so the cwd-relative store resolves under the cell's own repo
//     (.helix/semantic.duckdb inside repoDir) — private per cell, no shared-cwd
//     lock contention. (The full --parallel store-on isolation assertion lands in
//     Plan 04.)
func writeCellConfig(sb *benchsandbox.Sandbox, task, mode, profileName string, storeOptIn bool) (string, error) {
	home := sb.HomeFor(task, mode)
	cfgDir := filepath.Join(home, ".helix")
	if err := os.MkdirAll(cfgDir, 0700); err != nil {
		return "", fmt.Errorf("mkdir cell config dir: %w", err)
	}
	cfgPath := filepath.Join(cfgDir, "helix_config.yml")
	cfg := fmt.Sprintf("profile: %s\nsemantic_index:\n  enabled: %v\n", profileName, storeOptIn)
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

// writeDurable atomically writes b to path, creating parent dirs (0700) as
// needed. The durable path is keyed by (task, mode, run_index) via
// cellDurablePaths, so distinct cells never target the same path and the
// concurrent-writers-to-one-path scenario cannot arise under the current
// single-rep ExpandMatrix (RunIndex always 0). The atomic temp-file+rename is
// kept as defense for a future repetition axis (pass@k) or a re-run that
// overwrites an existing row: a bare os.WriteFile (O_CREATE|O_TRUNC) could leave
// a torn/partial file that a reader observes — a real filesystem data race the
// Go race detector cannot see (it only instruments memory). Staging to a temp
// file in the same directory and os.Rename into place makes the swap atomic
// within a filesystem, so a concurrent reader sees either the old file or the
// fully-written new one, never a half-written intermediate.
func writeDurable(path string, b []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("mkdir %q: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".tmp-"+filepath.Base(path)+"-*")
	if err != nil {
		return fmt.Errorf("create temp for %q: %w", path, err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("write temp for %q: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("close temp for %q: %w", path, err)
	}
	// IN-01: os.CreateTemp already creates the file 0600, so an explicit Chmod
	// here is a no-op on every platform — rely on the CreateTemp default.
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("rename temp into %q: %w", path, err)
	}
	return nil
}

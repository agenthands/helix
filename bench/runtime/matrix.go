// Package runtime — matrix.go owns the (benchmark x mode x task) expansion and
// the --parallel-bounded dispatcher that drives each expanded cell through the
// Plan 03 single-cell orchestrator (RunCell). It is the multi-cell layer the
// `helix-bench run` subcommand sits directly on top of (Plan 04 / criterion #1):
//
//	ExpandMatrix(benchmarks, languages, modes, tasks) -> []Cell   (cartesian product, V5-validated)
//	RunMatrix(ctx, cells, parallel, cfg)              -> Summary   (sem-bounded RunCell dispatch)
//
// No TCP ports are introduced here: each cell spawns a per-cell Unix-domain-socket
// daemon (D-06, inherited from RunCell), so "no port collisions on --parallel=N"
// holds by construction regardless of the parallel bound. The only shared resource
// the bound protects is host capacity (T-77-12 DoS mitigation): concurrency is
// capped by a sem channel of size `parallel` (mirrors internal/eval/runner.go:304).
package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Cell identifies one (benchmark, language, mode, task) point in the expanded
// matrix. The shared run_id is NOT carried per cell — it is a run-level value
// threaded through RunMatrixConfig so every cell in a run shares one out dir root
// (D-08). Language is the D-07 axis: it joins the seed dir as
// <root>/<benchmark>/<language>/<task> and selects the LanguageRunner (D-10).
type Cell struct {
	Benchmark string
	Language  string
	Mode      string
	Task      string
	// RunIndex is the 0-based repetition index for this (task, mode) cell. It is
	// threaded into the durable path (<task>/<mode>/<run_index>/) so repetitions of
	// the same (task, mode) land in distinct dirs (Pitfall 3). ExpandMatrix emits
	// single-rep cells (RunIndex==0); a future repetition axis sets it per cell.
	RunIndex int
}

// RunMatrixConfig carries the run-level inputs shared by every cell in a single
// `helix-bench run` invocation. Per-cell fields (Benchmark/Mode/Task) come from
// the Cell; everything here is constant across the run.
type RunMatrixConfig struct {
	// RunID is the shared run identifier (eval shape, e.g. "20060102T150405Z").
	RunID string
	// HelixBin is the path to the helix binary used to spawn each cell's daemon
	// and stdio forwarder (resolvable: absolute path or on PATH).
	HelixBin string
	// OutDir is the durable artifact root for this run (e.g. bench/reports/<run_id>).
	// Each cell writes under <OutDir>/<task>/<mode>/ (D-08).
	OutDir string
	// DatasetsRoot is the root under which seed task dirs live; a cell's seed dir
	// is <DatasetsRoot>/<benchmark>/<language>/<task> (e.g. bench/datasets) (D-07).
	DatasetsRoot string
	// Agent selects the agent driver. "scripted" (default, the CI gate) drives the
	// scripted_agent.yaml through the forwarder; "claude" wires the real claude CLI
	// branch (D-01, wired-not-gating — locally runnable, never the CI default).
	Agent string
	// RunnersRoot, when non-empty, overrides the mode->profile resolver root (tests).
	RunnersRoot string
}

// CellOutcome is the per-cell result the dispatcher aggregates into a Summary.
// Success is true iff the cell ran end-to-end AND its verify step passed (exit 0).
type CellOutcome struct {
	Cell    Cell
	Result  CellResult
	Success bool
	// Deferred is the D-02 distinct third outcome: a fail-closed stub (baseline_rag)
	// that produced NO result row. A deferred cell has Success==false (ResultValid is
	// false) AND Err==nil (it is a registered stub, not an infra failure), so without
	// this flag it would be indistinguishable from a plain verify-fail. Deferred==true
	// lets the smoke assert "registered + deferred" — neither succeeded nor errored.
	Deferred bool
	// Err is the infrastructure error from RunCell (nil for a normal pass/fail run).
	// A verify-failed cell has Err==nil and Success==false.
	Err error
}

// Summary aggregates the per-cell outcomes of a RunMatrix dispatch. Succeeded>=1
// drives the process exit (exit 0 iff >=1 cell succeeded — helix-eval semantics).
type Summary struct {
	Total     int
	Succeeded int
	Outcomes  []CellOutcome
}

// validateMatrixID rejects benchmark/mode/task ids that could escape a join root
// via path traversal (V5 / T-77-10). Mirrors internal/eval/runner.validateTaskID
// and bench/runtime.validateCellKey; MUST run before any id becomes a path
// segment (the seed-dir join and the per-cell out dir). RunCell re-validates the
// task/mode/benchmark defensively, but rejecting here fails the whole expansion
// fast rather than per cell. IN-05: the predicate body is shared with the cell
// layer via validatePathSegment so the two cannot drift; this wrapper preserves
// the matrix-layer error prose.
func validateMatrixID(id, kind string) error {
	return validatePathSegment(id, kind)
}

// ExpandMatrix produces the cartesian product (benchmark x language x mode x task)
// of cells, validating every id against path traversal BEFORE it becomes a path
// segment (V5/T-77-10; the <language> axis is covered by T-78-03). It returns an
// error if any list is empty or any id is unsafe.
//
// Cell ordering is deterministic: benchmarks (outer) x languages x modes x tasks
// (inner), so a fixed input always yields the same cell sequence (stable artifact
// layout, stable tests).
func ExpandMatrix(benchmarks, languages, modes, tasks []string) ([]Cell, error) {
	if len(benchmarks) == 0 {
		return nil, fmt.Errorf("bench/runtime: no benchmarks given")
	}
	if len(languages) == 0 {
		return nil, fmt.Errorf("bench/runtime: no languages given")
	}
	if len(modes) == 0 {
		return nil, fmt.Errorf("bench/runtime: no modes given")
	}
	if len(tasks) == 0 {
		return nil, fmt.Errorf("bench/runtime: no tasks given")
	}

	for _, b := range benchmarks {
		if err := validateMatrixID(b, "benchmark"); err != nil {
			return nil, fmt.Errorf("bench/runtime: %w", err)
		}
	}
	for _, l := range languages {
		if err := validateMatrixID(l, "language"); err != nil {
			return nil, fmt.Errorf("bench/runtime: %w", err)
		}
	}
	for _, m := range modes {
		if err := validateMatrixID(m, "mode"); err != nil {
			return nil, fmt.Errorf("bench/runtime: %w", err)
		}
	}
	for _, t := range tasks {
		if err := validateMatrixID(t, "task"); err != nil {
			return nil, fmt.Errorf("bench/runtime: %w", err)
		}
	}

	cells := make([]Cell, 0, len(benchmarks)*len(languages)*len(modes)*len(tasks))
	for _, b := range benchmarks {
		for _, l := range languages {
			for _, m := range modes {
				for _, t := range tasks {
					cells = append(cells, Cell{Benchmark: b, Language: l, Mode: m, Task: t})
				}
			}
		}
	}
	return cells, nil
}

// RunMatrix dispatches every cell through RunCell under a semaphore bounded by
// `parallel` (T-77-12: never exceed `parallel` concurrent cells; mirrors
// internal/eval/runner.go:304). A parallel value < 1 is treated as 1.
//
// Per-cell infrastructure errors are captured in the CellOutcome (not propagated)
// so one bad cell never aborts the run — the aggregate Summary.Succeeded drives
// the single process exit. The returned error is non-nil only for a usage error
// (e.g. nil context) — cell-level failures live in the Summary.
func RunMatrix(ctx context.Context, cells []Cell, parallel int, cfg RunMatrixConfig) (Summary, error) {
	if ctx == nil {
		return Summary{}, fmt.Errorf("bench/runtime: nil context")
	}
	return dispatch(ctx, cells, parallel, func(ctx context.Context, c Cell) CellOutcome {
		return runOneCell(ctx, c, cfg)
	})
}

// dispatch runs `run` for every cell under a semaphore bounded by `parallel`
// (T-77-12: never exceed `parallel` concurrent invocations; mirrors
// internal/eval/runner.go:304). A parallel value < 1 is treated as 1. It is the
// concurrency-bounded core of RunMatrix, factored out so the bound is unit-tested
// without spawning real daemons (the test injects a counting `run`).
func dispatch(ctx context.Context, cells []Cell, parallel int, run func(context.Context, Cell) CellOutcome) (Summary, error) {
	if parallel < 1 {
		parallel = 1
	}

	sem := make(chan struct{}, parallel)
	var (
		mu       sync.Mutex
		outcomes = make([]CellOutcome, len(cells))
		wg       sync.WaitGroup
	)

	for i, c := range cells {
		i, c := i, c
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			// On cancellation, already-queued goroutines must not proceed into
			// sandbox creation / daemon spawn (wasted setup/teardown after the
			// operator cancelled). Record a cancelled outcome instead.
			if ctx.Err() != nil {
				oc := CellOutcome{Cell: c, Err: ctx.Err()}
				mu.Lock()
				outcomes[i] = oc
				mu.Unlock()
				return
			}

			oc := run(ctx, c)

			mu.Lock()
			outcomes[i] = oc
			mu.Unlock()
		}()
	}
	wg.Wait()

	sum := Summary{Total: len(cells), Outcomes: outcomes}
	for _, oc := range outcomes {
		if oc.Success {
			sum.Succeeded++
		}
	}
	return sum, nil
}

// cellSeedDir returns the on-disk seed dir for a cell:
// <DatasetsRoot>/<benchmark>/<language>/<task> (D-07 <lang> seed join). The
// segments are V5-validated by ExpandMatrix before a Cell ever reaches here.
func cellSeedDir(datasetsRoot string, c Cell) string {
	return filepath.Join(datasetsRoot, c.Benchmark, c.Language, c.Task)
}

// runOneCell maps a Cell + run-level config to a CellConfig and dispatches it
// through RunCell. A cell "succeeds" iff RunCell returned no infra error AND the
// verify step passed (exit 0). The seed dir is
// <DatasetsRoot>/<benchmark>/<language>/<task> (D-07).
func runOneCell(ctx context.Context, c Cell, cfg RunMatrixConfig) CellOutcome {
	seedDir := cellSeedDir(cfg.DatasetsRoot, c)

	// IN-03: read+decode <seedDir>/task.json ONCE and derive both the prompt and
	// the store opt-in from that single decode, instead of parsing the same file
	// twice (once per helper). A missing/unreadable task.json yields the zero meta
	// (empty prompt, store OFF) — a benign metadata read, never fatal.
	meta := readTaskMeta(seedDir)

	// The claude branch (D-01) needs the task prompt; the scripted gate does not.
	var prompt string
	if cfg.Agent == "claude" {
		prompt = meta.Prompt
	}

	// StoreOptIn (D-01/D-02): the per-cell semantic store is OFF by default and is
	// opted in by the seed task's capability (incremental_update needs the store)
	// or an explicit task.json semantic_index override (the D-02 escape hatch).
	storeOptIn := meta.storeOptIn()

	res, err := RunCell(ctx, CellConfig{
		RunID:       cfg.RunID,
		Benchmark:   c.Benchmark,
		Language:    c.Language,
		Task:        c.Task,
		Mode:        c.Mode,
		RunIndex:    c.RunIndex, // thread the cell's actual run index (no longer hard-coded 0)
		HelixBin:    cfg.HelixBin,
		SeedDir:     seedDir,
		OutDir:      cfg.OutDir,
		RunnersRoot: cfg.RunnersRoot,
		Agent:       cfg.Agent,
		Prompt:      prompt,
		StoreOptIn:  storeOptIn,
	})

	oc := CellOutcome{Cell: c, Result: res, Err: err}
	// Success requires a clean infra run AND a passing verify (D-04 outcome). A
	// deferred stub (baseline_rag) has ResultValid==false → not Success; Err==nil →
	// not an infra error; Deferred==true is the distinct third outcome (D-02).
	oc.Success = err == nil && res.VerifyExitCode == 0 && res.ResultValid
	oc.Deferred = res.Deferred
	return oc
}

// taskMeta is the decoded shape of a seed <seedDir>/task.json that the matrix
// layer consumes. IN-03: it unifies the prompt (claude branch) and store opt-in
// (always) fields into ONE struct so task.json is read+unmarshaled once per cell.
type taskMeta struct {
	Capability    string `json:"capability"`
	SemanticIndex *bool  `json:"semantic_index"`
	Prompt        string `json:"prompt"`
}

// readTaskMeta reads+decodes <seedDir>/task.json once. A missing/unreadable or
// malformed task.json yields the zero taskMeta (empty prompt, store OFF) — a
// benign metadata read, never fatal (mirrors the prior per-helper behavior).
func readTaskMeta(seedDir string) taskMeta {
	var meta taskMeta
	b, err := os.ReadFile(filepath.Join(seedDir, "task.json"))
	if err != nil {
		return meta
	}
	if err := json.Unmarshal(b, &meta); err != nil {
		return taskMeta{}
	}
	return meta
}

// storeOptIn decides whether the cell opts the per-cell semantic store ON
// (RESEARCH Open-Q1: derive-from-capability). The store is opted in when an
// explicit "semantic_index" override is present (the D-02 escape hatch wins) or,
// absent that, when capability=="incremental_update" (the store-on class).
func (m taskMeta) storeOptIn() bool {
	if m.SemanticIndex != nil {
		return *m.SemanticIndex // explicit override wins (D-02 escape hatch)
	}
	return m.Capability == "incremental_update"
}

// deriveStoreOptIn is a thin wrapper preserving the standalone predicate used by
// store_isolation_test.go: read task.json once and apply the store opt-in rule.
func deriveStoreOptIn(seedDir string) bool {
	return readTaskMeta(seedDir).storeOptIn()
}

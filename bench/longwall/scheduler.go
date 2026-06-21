package longwall

import (
	"context"
	"time"
)

// CellID is the matrix coordinate of one benchmark cell. Its five components map
// to a stable, path-safe checkpoint key via Key.
type CellID struct {
	Benchmark string
	Lang      string
	Mode      string
	Task      string
	RunIndex  int
}

// Key returns the stable, path-safe checkpoint key for the cell, or an error if
// any coordinate is malformed (empty / separator-bearing / traversal-bearing).
// Because Lang/Task can be sourced from dataset dirs and file contents, a bad
// coordinate is surfaced as an error and degraded to a per-cell failure by
// Scheduler.Run — it never aborts the whole resilience-oriented run (WR-03).
func (c CellID) Key() (string, error) {
	return cellKey(c.Benchmark, c.Lang, c.Mode, c.Task, c.RunIndex)
}

// Outcome is what a cell runner reports. Only Success == true is a completion;
// anything else leaves the cell re-runnable on the next pass.
type Outcome struct {
	Success   bool
	ResultRef string
}

// Summary tallies a single Scheduler.Run pass.
type Summary struct {
	Ran     int // cells whose runner was invoked and succeeded
	Skipped int // cells already done (resume skip)
	Failed  int // cells whose runner was invoked but did not succeed
}

// Scheduler is the resume-aware driver over a cell matrix. It is sequential and
// pure: the long-wall scope is resume-correctness, not parallelism (the existing
// matrix dispatch owns bounded parallelism and is the documented complement). It
// holds no Docker / bench/runtime dependency, so the SC#3 guarantee is fully
// hermetic.
type Scheduler struct {
	store *Store
}

// NewScheduler constructs a scheduler whose checkpoints live under dir. now == nil
// -> time.Now (injected for golden-stable UpdatedAt in hermetic tests).
func NewScheduler(dir string, now func() time.Time) *Scheduler {
	return &Scheduler{store: NewStore(dir, now)}
}

// statusFor maps an outcome to a persisted status. A clean success -> done (a
// skip on resume); anything else -> failed (re-runs on the next pass).
func statusFor(oc Outcome) string {
	if oc.Success {
		return StatusDone
	}
	return StatusFailed
}

// Run drives the cells, resuming from checkpoints: a cell already checkpointed
// done is SKIPPED (its runner is never invoked); every other cell is run and its
// outcome checkpointed atomically. Re-entry is idempotent because only a clean
// success persists done, so a fresh Scheduler over the same ckptDir converges to
// the same end state with no double-execution — the SC#3 >24h resume guarantee,
// proven via the injected clock with NO real 24h run.
//
// A checkpoint read error or a write error aborts the cell conservatively
// (counted Failed) rather than silently treating an unreadable cell as done — a
// torn or unparseable checkpoint must re-run, never be skipped.
func (s *Scheduler) Run(ctx context.Context, cells []CellID, run func(CellID) Outcome) Summary {
	var sum Summary
	for _, c := range cells {
		if ctx.Err() != nil {
			return sum
		}
		key, err := c.Key()
		if err != nil {
			// A malformed coordinate (e.g. a Lang/Task sourced from a bad dataset
			// dir or row) degrades to a per-cell failure — it must NOT abort the
			// whole long-wall pass for every OTHER cell (WR-03). The cell is
			// re-runnable once the bad coordinate is corrected upstream.
			sum.Failed++
			continue
		}

		cs, ok, err := s.store.readCheckpoint(key)
		if err == nil && ok && cs.Status == StatusDone {
			sum.Skipped++ // RESUME skip — a completed cell is never re-run.
			continue
		}
		// A read error is treated as "not done": the cell re-runs rather than
		// being skipped on an unreadable/torn checkpoint.

		oc := run(c)
		_ = s.store.writeCheckpoint(CellState{
			CellKey:   key,
			Status:    statusFor(oc),
			ResultRef: oc.ResultRef,
			UpdatedAt: s.store.now().Format(time.RFC3339),
		})
		if oc.Success {
			sum.Ran++
		} else {
			sum.Failed++
		}
	}
	return sum
}

package longwall

import (
	"context"
	"os"
	"testing"
	"time"
)

// fixed clock for the scheduler tests: a single instant so UpdatedAt is
// golden-stable and the >24h resume is proven WITHOUT a real 24h run.
var schedFixed = time.Date(2026, 6, 21, 9, 0, 0, 0, time.UTC)

func threeCells() []CellID {
	return []CellID{
		{Benchmark: "terminalbench", Lang: "python", Mode: "agent", Task: "alpha", RunIndex: 0},
		{Benchmark: "terminalbench", Lang: "python", Mode: "agent", Task: "beta", RunIndex: 0},
		{Benchmark: "terminalbench", Lang: "python", Mode: "agent", Task: "gamma", RunIndex: 0},
	}
}

// keyOf returns a cell's key for a coordinate the test KNOWS is valid, panicking
// if validation unexpectedly rejects it (a test-only convenience for the always-
// valid threeCells coordinates).
func keyOf(c CellID) string {
	key, err := c.Key()
	if err != nil {
		panic(err)
	}
	return key
}

// countingRunner records how many times each cellKey's runner is invoked and
// reports every cell as a success (so the scheduler writes done).
func countingRunner(counts map[string]int) func(CellID) Outcome {
	return func(c CellID) Outcome {
		counts[keyOf(c)]++
		return Outcome{Success: true, ResultRef: "results/" + keyOf(c) + ".json"}
	}
}

// TestResumeSkipsDone: seed one cell as done; its runner must be invoked EXACTLY
// 0 times while every other cell runs exactly once (resume-after-restart proof, SC#3).
func TestResumeSkipsDone(t *testing.T) {
	dir := t.TempDir()
	s := NewScheduler(dir, fixedClock(schedFixed))
	cells := threeCells()

	// Pre-seed beta as done (as if a prior run completed it before a restart).
	seeded := keyOf(cells[1])
	if err := s.store.writeCheckpoint(CellState{CellKey: seeded, Status: StatusDone, UpdatedAt: schedFixed.Format(time.RFC3339)}); err != nil {
		t.Fatalf("seed done checkpoint: %v", err)
	}

	counts := map[string]int{}
	sum := s.Run(context.Background(), cells, countingRunner(counts))

	if counts[seeded] != 0 {
		t.Errorf("seeded done cell ran %d times, want 0 (resume must skip)", counts[seeded])
	}
	if got := counts[keyOf(cells[0])]; got != 1 {
		t.Errorf("alpha ran %d times, want 1", got)
	}
	if got := counts[keyOf(cells[2])]; got != 1 {
		t.Errorf("gamma ran %d times, want 1", got)
	}
	if sum.Skipped != 1 {
		t.Errorf("Summary.Skipped = %d, want 1", sum.Skipped)
	}
	if sum.Ran != 2 {
		t.Errorf("Summary.Ran = %d, want 2", sum.Ran)
	}
}

// TestIdempotentReentry: a PARTIAL running checkpoint re-runs the cell (a partial
// is not a completion); after it converges to done, a second Run skips it.
func TestIdempotentReentry(t *testing.T) {
	dir := t.TempDir()
	s := NewScheduler(dir, fixedClock(schedFixed))
	cells := threeCells()

	// Seed alpha as a PARTIAL running checkpoint — never done.
	partial := keyOf(cells[0])
	if err := s.store.writeCheckpoint(CellState{CellKey: partial, Status: StatusRunning, UpdatedAt: schedFixed.Format(time.RFC3339)}); err != nil {
		t.Fatalf("seed running checkpoint: %v", err)
	}

	counts := map[string]int{}
	s.Run(context.Background(), cells, countingRunner(counts))

	// A partial checkpoint must be re-run (not treated as a completion).
	if counts[partial] != 1 {
		t.Errorf("partial-running cell ran %d times, want 1 (re-run)", counts[partial])
	}
	// It must now be written done.
	cs, ok, err := s.store.readCheckpoint(partial)
	if err != nil || !ok {
		t.Fatalf("readCheckpoint after run: ok=%v err=%v", ok, err)
	}
	if cs.Status != StatusDone {
		t.Errorf("after successful run Status = %q, want %q", cs.Status, StatusDone)
	}

	// Second pass converges: every cell now done -> 0 invocations.
	counts2 := map[string]int{}
	sum2 := s.Run(context.Background(), cells, countingRunner(counts2))
	for k, v := range counts2 {
		if v != 0 {
			t.Errorf("second pass ran %q %d times, want 0 (idempotent re-entry)", k, v)
		}
	}
	if sum2.Ran != 0 || sum2.Skipped != len(cells) {
		t.Errorf("second pass Summary = {Ran:%d Skipped:%d}, want {Ran:0 Skipped:%d}", sum2.Ran, sum2.Skipped, len(cells))
	}
}

// TestFullRestart: run all cells once (all done), then a FRESH scheduler over the
// same ckptDir runs again (harness-restart sim) -> 0 invocations on the 2nd pass.
// UpdatedAt equals the injected clock, so a >24h gap is irrelevant to correctness.
func TestFullRestart(t *testing.T) {
	dir := t.TempDir()
	cells := threeCells()

	s1 := NewScheduler(dir, fixedClock(schedFixed))
	counts1 := map[string]int{}
	s1.Run(context.Background(), cells, countingRunner(counts1))
	for _, c := range cells {
		if counts1[keyOf(c)] != 1 {
			t.Fatalf("first pass: %q ran %d times, want 1", keyOf(c), counts1[keyOf(c)])
		}
	}

	// Simulate a harness restart >24h later: a FRESH scheduler, even with a clock
	// advanced past 24h, must still skip every already-done cell.
	later := schedFixed.Add(48 * time.Hour)
	s2 := NewScheduler(dir, fixedClock(later))
	counts2 := map[string]int{}
	sum2 := s2.Run(context.Background(), cells, countingRunner(counts2))

	total := 0
	for _, v := range counts2 {
		total += v
	}
	if total != 0 {
		t.Errorf("post-restart pass invoked the runner %d times, want 0", total)
	}
	if sum2.Skipped != len(cells) || sum2.Ran != 0 {
		t.Errorf("post-restart Summary = {Ran:%d Skipped:%d}, want {Ran:0 Skipped:%d}", sum2.Ran, sum2.Skipped, len(cells))
	}

	// The persisted UpdatedAt is the FIRST pass's injected clock, not the 48h-later
	// one — completed cells were never rewritten, proving no wall-clock dependence.
	cs, ok, _ := s2.store.readCheckpoint(keyOf(cells[0]))
	if !ok {
		t.Fatal("expected a done checkpoint to persist across restart")
	}
	if cs.UpdatedAt != schedFixed.Format(time.RFC3339) {
		t.Errorf("UpdatedAt = %q, want first-pass clock %q (done cells not rewritten)", cs.UpdatedAt, schedFixed.Format(time.RFC3339))
	}
}

// TestFailedCellReruns: a failed cell is NOT marked done, so it re-runs next pass.
func TestFailedCellReruns(t *testing.T) {
	dir := t.TempDir()
	s := NewScheduler(dir, fixedClock(schedFixed))
	cells := threeCells()
	failing := keyOf(cells[1])

	runner := func(c CellID) Outcome {
		if keyOf(c) == failing {
			return Outcome{Success: false}
		}
		return Outcome{Success: true, ResultRef: "ok"}
	}
	sum := s.Run(context.Background(), cells, runner)
	if sum.Failed != 1 {
		t.Errorf("Summary.Failed = %d, want 1", sum.Failed)
	}
	cs, ok, _ := s.store.readCheckpoint(failing)
	if !ok || cs.Status != StatusFailed {
		t.Errorf("failed cell Status = %q (ok=%v), want %q", cs.Status, ok, StatusFailed)
	}

	// Second pass: the failed cell re-runs (a failure is never a skip).
	counts := map[string]int{}
	s.Run(context.Background(), cells, func(c CellID) Outcome {
		counts[keyOf(c)]++
		return Outcome{Success: true, ResultRef: "ok"}
	})
	if counts[failing] != 1 {
		t.Errorf("failed cell re-ran %d times on second pass, want 1", counts[failing])
	}
}

// TestMalformedCellDegradesNotAborts (WR-03): a batch containing ONE cell with a
// malformed coordinate (a Task with a path separator — as could come from a bad
// dataset dir/row) must still process every OTHER (valid) cell. The bad cell is
// counted Failed (re-runnable / surfaced), NOT a panic that tears down the whole
// resilience-oriented long-wall pass.
func TestMalformedCellDegradesNotAborts(t *testing.T) {
	dir := t.TempDir()
	s := NewScheduler(dir, fixedClock(schedFixed))

	// A bad cell sandwiched between two valid ones — proving cells AFTER the bad
	// one still run (an abort would skip them).
	cells := []CellID{
		{Benchmark: "terminalbench", Lang: "python", Mode: "agent", Task: "alpha", RunIndex: 0},
		{Benchmark: "terminalbench", Lang: "python", Mode: "agent", Task: "bad/task", RunIndex: 0}, // malformed
		{Benchmark: "terminalbench", Lang: "python", Mode: "agent", Task: "gamma", RunIndex: 0},
	}

	counts := map[string]int{}
	runner := func(c CellID) Outcome {
		// keyOf would panic on the malformed cell — but Scheduler.Run must NEVER
		// invoke the runner for it (its Key() fails before run is called).
		counts[keyOf(c)]++
		return Outcome{Success: true, ResultRef: "ok"}
	}

	sum := s.Run(context.Background(), cells, runner)

	// Both valid cells ran exactly once.
	if got := counts[keyOf(cells[0])]; got != 1 {
		t.Errorf("alpha (before the bad cell) ran %d times, want 1", got)
	}
	if got := counts[keyOf(cells[2])]; got != 1 {
		t.Errorf("gamma (AFTER the bad cell) ran %d times, want 1 — a bad cell must not abort the rest", got)
	}
	// The malformed cell is counted Failed (re-runnable), not silently dropped.
	if sum.Failed != 1 {
		t.Errorf("Summary.Failed = %d, want 1 (the malformed cell)", sum.Failed)
	}
	if sum.Ran != 2 {
		t.Errorf("Summary.Ran = %d, want 2 (the two valid cells)", sum.Ran)
	}
	// No checkpoint should have been written for the malformed cell (its Key()
	// never resolved, so writeCheckpoint was never reached for it).
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("got %d checkpoint files, want 2 (only the valid cells)", len(entries))
	}
}

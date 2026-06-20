package runtime

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// TestExpandMatrixCartesianCount asserts the expansion yields exactly
// |benchmarks| x |languages| x |modes| x |tasks| cells, with no duplicates and a
// deterministic (benchmark-outer, language, mode, task-inner) ordering.
func TestExpandMatrixCartesianCount(t *testing.T) {
	benchmarks := []string{"internal-toolbench", "other-bench"}
	languages := []string{"go", "rust"}
	modes := []string{"your_agent_full", "your_agent_no_lsp"}
	tasks := []string{"IT-go-patch-apply-1", "fizzbuzz", "rename-sym"}

	cells, err := ExpandMatrix(benchmarks, languages, modes, tasks, 1)
	if err != nil {
		t.Fatalf("ExpandMatrix: unexpected error: %v", err)
	}

	want := len(benchmarks) * len(languages) * len(modes) * len(tasks)
	if len(cells) != want {
		t.Fatalf("cell count = %d; want %d", len(cells), want)
	}

	// Every (benchmark, language, mode, task) tuple is present exactly once.
	seen := make(map[Cell]int, len(cells))
	for _, c := range cells {
		seen[c]++
	}
	if len(seen) != want {
		t.Fatalf("distinct cells = %d; want %d (duplicates in expansion)", len(seen), want)
	}
	for _, b := range benchmarks {
		for _, l := range languages {
			for _, m := range modes {
				for _, tk := range tasks {
					k := Cell{Benchmark: b, Language: l, Mode: m, Task: tk}
					if seen[k] != 1 {
						t.Errorf("cell %+v appeared %d times; want 1", k, seen[k])
					}
				}
			}
		}
	}

	// Deterministic ordering: first cell is (benchmarks[0], languages[0],
	// modes[0], tasks[0]).
	first := Cell{Benchmark: benchmarks[0], Language: languages[0], Mode: modes[0], Task: tasks[0]}
	if cells[0] != first {
		t.Errorf("cells[0] = %+v; want %+v (ordering not deterministic)", cells[0], first)
	}
}

// TestExpandMatrixSingleCell asserts the canonical seed expansion yields exactly
// one cell with Language=="go" and the other axes set (D-07 axis substrate).
func TestExpandMatrixSingleCell(t *testing.T) {
	cells, err := ExpandMatrix(
		[]string{"internal-toolbench"},
		[]string{"go"},
		[]string{"your_agent_full"},
		[]string{"IT-go-patch-apply-1"},
		1,
	)
	if err != nil {
		t.Fatalf("ExpandMatrix: unexpected error: %v", err)
	}
	if len(cells) != 1 {
		t.Fatalf("cell count = %d; want 1", len(cells))
	}
	want := Cell{
		Benchmark: "internal-toolbench",
		Language:  "go",
		Mode:      "your_agent_full",
		Task:      "IT-go-patch-apply-1",
	}
	if cells[0] != want {
		t.Fatalf("cells[0] = %+v; want %+v", cells[0], want)
	}
}

// TestExpandMatrixRuns asserts the D-04 runs axis (STATS-01 producer half):
// ExpandMatrix(..., N) emits exactly N cells per (benchmark, language, mode,
// task) with distinct RunIndex 0..N-1, and a runs value < 1 clamps to 1 (never
// an empty matrix). It is the Wave-0 scaffold for the multi-run producer.
func TestExpandMatrixRuns(t *testing.T) {
	benchmarks := []string{"internal-toolbench"}
	languages := []string{"go"}
	modes := []string{"your_agent_full"}
	tasks := []string{"t1", "t2"}
	const runs = 3

	cells, err := ExpandMatrix(benchmarks, languages, modes, tasks, runs)
	if err != nil {
		t.Fatalf("ExpandMatrix: unexpected error: %v", err)
	}

	// Total = |b| * |l| * |m| * |t| * runs = 1*1*1*2*3 = 6.
	want := len(benchmarks) * len(languages) * len(modes) * len(tasks) * runs
	if len(cells) != want {
		t.Fatalf("cell count = %d; want %d", len(cells), want)
	}

	// For each task there are exactly `runs` cells with RunIndex {0,1,2} (a set,
	// not order-dependent).
	byTask := make(map[string]map[int]int)
	for _, c := range cells {
		if byTask[c.Task] == nil {
			byTask[c.Task] = make(map[int]int)
		}
		byTask[c.Task][c.RunIndex]++
	}
	for _, tk := range tasks {
		idx := byTask[tk]
		if len(idx) != runs {
			t.Errorf("task %q: %d distinct RunIndex values; want %d (%v)", tk, len(idx), runs, idx)
		}
		for r := 0; r < runs; r++ {
			if idx[r] != 1 {
				t.Errorf("task %q: RunIndex %d appeared %d times; want exactly 1", tk, r, idx[r])
			}
		}
	}

	// runs < 1 clamps to 1: exactly one cell per (b,l,m,t), RunIndex 0.
	clamped, err := ExpandMatrix(benchmarks, languages, modes, tasks, 0)
	if err != nil {
		t.Fatalf("ExpandMatrix(runs=0): unexpected error: %v", err)
	}
	wantClamped := len(benchmarks) * len(languages) * len(modes) * len(tasks)
	if len(clamped) != wantClamped {
		t.Fatalf("runs=0 cell count = %d; want %d (clamp to 1)", len(clamped), wantClamped)
	}
	for _, c := range clamped {
		if c.RunIndex != 0 {
			t.Errorf("runs=0: cell %+v has RunIndex %d; want 0", c, c.RunIndex)
		}
	}
}

// TestExpandMatrixRejectsPathTraversal asserts that a "../"-containing or
// absolute id in ANY axis (benchmark/language/mode/task) fails the expansion
// (V5/T-77-10 / T-78-03 for the new <lang> segment).
func TestExpandMatrixRejectsPathTraversal(t *testing.T) {
	cases := []struct {
		name       string
		benchmarks []string
		languages  []string
		modes      []string
		tasks      []string
	}{
		{"task parent-ref", []string{"internal-toolbench"}, []string{"go"}, []string{"your_agent_full"}, []string{"../etc/passwd"}},
		{"task separator", []string{"internal-toolbench"}, []string{"go"}, []string{"your_agent_full"}, []string{"a/b"}},
		{"task leading dot", []string{"internal-toolbench"}, []string{"go"}, []string{"your_agent_full"}, []string{".hidden"}},
		{"mode parent-ref", []string{"internal-toolbench"}, []string{"go"}, []string{"../x"}, []string{"IT-go-patch-apply-1"}},
		{"benchmark parent-ref", []string{"../x"}, []string{"go"}, []string{"your_agent_full"}, []string{"IT-go-patch-apply-1"}},
		{"absolute task", []string{"internal-toolbench"}, []string{"go"}, []string{"your_agent_full"}, []string{"/abs"}},
		{"language parent-ref", []string{"internal-toolbench"}, []string{"../x"}, []string{"your_agent_full"}, []string{"IT-go-patch-apply-1"}},
		{"language separator", []string{"internal-toolbench"}, []string{"a/b"}, []string{"your_agent_full"}, []string{"IT-go-patch-apply-1"}},
		{"language leading dot", []string{"internal-toolbench"}, []string{".go"}, []string{"your_agent_full"}, []string{"IT-go-patch-apply-1"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ExpandMatrix(tc.benchmarks, tc.languages, tc.modes, tc.tasks, 1); err == nil {
				t.Fatalf("ExpandMatrix(%v, %v, %v, %v) = nil error; want a path-traversal rejection",
					tc.benchmarks, tc.languages, tc.modes, tc.tasks)
			}
		})
	}
}

// TestExpandMatrixRejectsEmpty asserts an empty axis is an error (no silent
// empty matrix that would make `run` a no-op and exit 0).
func TestExpandMatrixRejectsEmpty(t *testing.T) {
	if _, err := ExpandMatrix(nil, []string{"go"}, []string{"m"}, []string{"t"}, 1); err == nil {
		t.Error("empty benchmarks: want error, got nil")
	}
	if _, err := ExpandMatrix([]string{"b"}, nil, []string{"m"}, []string{"t"}, 1); err == nil {
		t.Error("empty languages: want error, got nil")
	}
	if _, err := ExpandMatrix([]string{"b"}, []string{"go"}, nil, []string{"t"}, 1); err == nil {
		t.Error("empty modes: want error, got nil")
	}
	if _, err := ExpandMatrix([]string{"b"}, []string{"go"}, []string{"m"}, nil, 1); err == nil {
		t.Error("empty tasks: want error, got nil")
	}
}

// TestDispatchRespectsParallelBound asserts the dispatcher NEVER runs more than
// `parallel` cells concurrently (T-77-12). The injected runner increments a live
// counter on entry, records the peak, and decrements on exit; the peak must never
// exceed the bound. A barrier holds each goroutine until enough have entered so the
// test would actually observe a violation if the bound were not enforced.
func TestDispatchRespectsParallelBound(t *testing.T) {
	const (
		nCells   = 12
		parallel = 3
	)

	cells := make([]Cell, nCells)
	for i := range cells {
		cells[i] = Cell{Benchmark: "b", Mode: "m", Task: "t"}
	}

	var (
		live       int64
		peak       int64
		enteredCnt int64
		release    = make(chan struct{})
	)

	runner := func(ctx context.Context, c Cell) CellOutcome {
		cur := atomic.AddInt64(&live, 1)
		for {
			p := atomic.LoadInt64(&peak)
			if cur <= p || atomic.CompareAndSwapInt64(&peak, p, cur) {
				break
			}
		}
		// Hold the first `parallel` goroutines simultaneously to prove the window is
		// open; the watcher releases the barrier once that many are in-flight, after
		// which every goroutine (including these) proceeds. A bounded dispatcher can
		// never get `parallel` goroutines concurrently past the increment if the
		// semaphore is smaller — so reaching the barrier IS the bound assertion.
		atomic.AddInt64(&enteredCnt, 1)
		<-release
		atomic.AddInt64(&live, -1)
		return CellOutcome{Cell: c, Success: true}
	}

	// Release the barrier once `parallel` goroutines are concurrently in-flight.
	go func() {
		for atomic.LoadInt64(&enteredCnt) < parallel {
			time.Sleep(time.Millisecond)
		}
		close(release)
	}()

	sum, err := dispatch(context.Background(), cells, parallel, runner)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if got := atomic.LoadInt64(&peak); got > parallel {
		t.Fatalf("peak concurrency = %d; exceeded parallel bound %d", got, parallel)
	}
	if sum.Total != nCells || sum.Succeeded != nCells {
		t.Fatalf("summary = {Total:%d Succeeded:%d}; want {%d %d}", sum.Total, sum.Succeeded, nCells, nCells)
	}
}

// TestDispatchAggregatesSuccess asserts Summary.Succeeded counts only cells whose
// outcome reports Success (exit-0 + valid result), exercising the >=1-success exit
// driver helix-bench run relies on.
func TestDispatchAggregatesSuccess(t *testing.T) {
	cells := []Cell{
		{Benchmark: "b", Mode: "m", Task: "pass1"},
		{Benchmark: "b", Mode: "m", Task: "fail1"},
		{Benchmark: "b", Mode: "m", Task: "pass2"},
	}
	runner := func(ctx context.Context, c Cell) CellOutcome {
		return CellOutcome{Cell: c, Success: c.Task != "fail1"}
	}
	sum, err := dispatch(context.Background(), cells, 2, runner)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if sum.Total != 3 || sum.Succeeded != 2 {
		t.Fatalf("summary = {Total:%d Succeeded:%d}; want {3 2}", sum.Total, sum.Succeeded)
	}
}

// TestRunMatrixNilContext asserts the usage-error guard.
func TestRunMatrixNilContext(t *testing.T) {
	//nolint:staticcheck // intentionally passing a nil context to exercise the guard
	if _, err := RunMatrix(nil, nil, 1, RunMatrixConfig{}); err == nil {
		t.Error("RunMatrix(nil ctx): want error, got nil")
	}
}

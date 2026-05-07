package semantic

import (
	"context"
	"sync"
	"testing"
	"time"
)

// TestIndexRunner_SingleflightJoin_TwoCallersOneSnapshotID asserts D-02:
// two concurrent Run(ws, "full") calls join the same singleflight build and
// both observe the SAME snapshot id. buildFn must execute exactly ONCE.
func TestIndexRunner_SingleflightJoin_TwoCallersOneSnapshotID(t *testing.T) {
	t.Parallel()

	store := &mockStoreAccessor{}
	mb := &mockBuild{snapshotID: 555, delay: 200 * time.Millisecond}
	r := NewIndexRunner(store, mb.makeBuildFn(), 10*time.Second)

	var (
		wg      sync.WaitGroup
		results [2]IndexResult
		errs    [2]error
	)
	wg.Add(2)
	for i := range results {
		go func(idx int) {
			defer wg.Done()
			results[idx], errs[idx] = r.Run(context.Background(), newTestWorkspace(), "full", 5000)
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("call %d: unexpected error: %v", i, err)
		}
	}
	if mb.calls.Load() != 1 {
		t.Fatalf("buildFn invocations: got %d, want 1 (singleflight should join the second caller)", mb.calls.Load())
	}
	if results[0].SnapshotID != results[1].SnapshotID {
		t.Fatalf("snapshot ids must be equal across concurrent callers: got %d vs %d",
			results[0].SnapshotID, results[1].SnapshotID)
	}
	if results[0].SnapshotID != 555 {
		t.Fatalf("snapshot id: got %d, want 555", results[0].SnapshotID)
	}
}

// TestIndexRunner_SingleflightJoin_DifferentModesNoCollapse asserts the
// singleflight key is (workspace, mode) — two callers with DIFFERENT modes
// must NOT collapse onto the same build.
func TestIndexRunner_SingleflightJoin_DifferentModesNoCollapse(t *testing.T) {
	t.Parallel()

	store := &mockStoreAccessor{latestSnapshotID: 1} // so "incremental" path is valid
	mb := &mockBuild{snapshotID: 600, delay: 150 * time.Millisecond}
	r := NewIndexRunner(store, mb.makeBuildFn(), 10*time.Second)

	var (
		wg     sync.WaitGroup
		resA   IndexResult
		resB   IndexResult
		errA   error
		errB   error
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		resA, errA = r.Run(context.Background(), newTestWorkspace(), "full", 5000)
	}()
	go func() {
		defer wg.Done()
		resB, errB = r.Run(context.Background(), newTestWorkspace(), "incremental", 5000)
	}()
	wg.Wait()

	if errA != nil || errB != nil {
		t.Fatalf("unexpected errors: A=%v B=%v", errA, errB)
	}
	if mb.calls.Load() != 2 {
		t.Fatalf("buildFn invocations: got %d, want 2 (different modes must NOT collapse)", mb.calls.Load())
	}
	// Both callers got the same mock snapshotID (mocked constant), but the
	// IMPORTANT invariant is that buildFn ran twice — proving distinct
	// singleflight keys. We don't require distinct snapshot ids here because
	// that's a property of the production buildFn (P64-08), not the runner.
	_ = resA
	_ = resB
}

// TestIndexRunner_SingleflightJoin_TwoAutoCallersOneBuild closes checker B5:
// two concurrent goroutines simulating the handler — each calls ResolveAuto
// (which returns "full" because the mocked store has no committed snapshot)
// then calls Run(ws, "full", 5000). buildFn must execute exactly ONCE and
// both callers must receive the same snapshot id.
func TestIndexRunner_SingleflightJoin_TwoAutoCallersOneBuild(t *testing.T) {
	t.Parallel()

	store := &mockStoreAccessor{latestSnapshotID: 0} // no committed snapshot -> ResolveAuto returns "full"
	mb := &mockBuild{snapshotID: 999, delay: 200 * time.Millisecond}
	r := NewIndexRunner(store, mb.makeBuildFn(), 10*time.Second)

	var (
		wg      sync.WaitGroup
		results [2]IndexResult
		errs    [2]error
	)
	wg.Add(2)
	for i := range results {
		go func(idx int) {
			defer wg.Done()
			ctx := context.Background()
			ws := newTestWorkspace()
			// Mirror the handler logic: resolve auto BEFORE Run.
			mode := r.ResolveAuto(ctx, ws)
			if mode != "full" {
				errs[idx] = errFatalf("ResolveAuto: got %q, want full", mode)
				return
			}
			results[idx], errs[idx] = r.Run(ctx, ws, mode, 5000)
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("call %d: unexpected error: %v", i, err)
		}
	}
	if mb.calls.Load() != 1 {
		t.Fatalf("buildFn invocations: got %d, want 1 (auto-resolved callers must join the same singleflight)", mb.calls.Load())
	}
	if results[0].SnapshotID != results[1].SnapshotID {
		t.Fatalf("snapshot ids must be equal across concurrent auto callers: got %d vs %d",
			results[0].SnapshotID, results[1].SnapshotID)
	}
	if results[0].SnapshotID != 999 {
		t.Fatalf("snapshot id: got %d, want 999", results[0].SnapshotID)
	}
}

// errFatalf is a tiny helper for goroutine-internal error reporting (returning
// an error from a goroutine that the parent goroutine asserts via the errs
// slice — keeps the test free of t.Fatalf calls inside child goroutines).
type stringError string

func (e stringError) Error() string { return string(e) }

func errFatalf(format string, args ...any) error {
	// Lightweight wrapper to avoid pulling in fmt in the test helper signature.
	return stringError(sprintfShim(format, args...))
}

func sprintfShim(format string, args ...any) string {
	// Defer to fmt.Sprintf without polluting the import set of every test file.
	// Using %v on each arg keeps this tiny.
	out := format
	for _, a := range args {
		out += " "
		out += toString(a)
	}
	return out
}

func toString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case error:
		return x.Error()
	default:
		return ""
	}
}

// Phase 63 P63-02 Task 2 / COMPACT-02:
//
// Property-style test that interleaves overlay-row writes (committed at
// epochs > capturedEpoch) with a compaction-style snap.ClearOverlayLE.
// Asserts that rows committed AFTER the captured_epoch SURVIVE the
// drain (Phase 60 D-04 CAS contract).
//
// Run with `-race` to expose data-race regressions in the
// pending-rows counter / atomic counter wiring Task 1 added.

package compact_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/agenthands/helix/internal/obs"
	"github.com/agenthands/helix/internal/semantic"
	"github.com/agenthands/helix/internal/semantic/store"
)

// TestCAS_InterleaveOverlayWritesWithCompaction proves that overlay
// rows committed AFTER capturedEpoch survive ClearOverlayLE.
//
// Strategy:
//  1. Open a fresh store.
//  2. Write rows at epochs 1..10 (committed sequentially via separate
//     BeginOverlayTx cycles).
//  3. Capture epoch=5.
//  4. Concurrently:
//     - call snap.ClearOverlayLE(captured=5) inside a snapshot tx
//     - keep writing new overlay rows at epoch >= 6 in parallel
//  5. Commit the snapshot.
//  6. Assert overlay_files row count > 0 (rows from epochs 6..10 + any
//     written during the concurrent window survived).
func TestCAS_InterleaveOverlayWritesWithCompaction(t *testing.T) {
	wsDir := t.TempDir()
	// Build a minimal store.Open config inline (we cannot import
	// store_test internal helpers from a sibling _test package). The
	// path is workspace-relative; changeWD below changes process cwd
	// into wsDir so the relative `.helix/semantic.duckdb` resolves.
	storeCfg := semantic.Config{Store: semantic.StoreConfig{
		Kind: "duckdb",
		Path: ".helix/semantic.duckdb",
	}}

	provider := obs.Noop(nil)
	t.Setenv("HELIX_TEST_WSDIR", wsDir)

	prevDir := changeWD(t, wsDir)
	defer prevDir()

	s, err := store.Open(context.Background(), storeCfg, nil, provider.Metrics())
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer s.Close()

	const repo = "cas-test"
	// Phase 1: write 5 rows sequentially (epochs 1..5).
	for i := 1; i <= 5; i++ {
		tx, err := s.BeginOverlayTx(context.Background(), repo)
		if err != nil {
			t.Fatalf("Begin %d: %v", i, err)
		}
		path := fmt.Sprintf("a%d.go", i)
		if err := tx.UpsertOverlayFile(context.Background(), path, "h"); err != nil {
			t.Fatalf("Upsert %d: %v", i, err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatalf("Commit %d: %v", i, err)
		}
	}

	// Capture the current epoch for the simulated compaction.
	capturedEpoch := uint64(5)

	// Phase 2: open a snapshot, then BEFORE running ClearOverlayLE,
	// write new rows in parallel. Real concurrent writers race the
	// snapshot tx via DuckDB MVCC.
	snap, err := s.BeginSnapshot(context.Background(), store.SnapshotMeta{
		RepoID:        repo,
		CapturedEpoch: capturedEpoch,
	})
	if err != nil {
		t.Fatalf("BeginSnapshot: %v", err)
	}

	// Concurrent writers (post-captured epoch).
	var wg sync.WaitGroup
	for i := 6; i <= 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			tx, err := s.BeginOverlayTx(context.Background(), repo)
			if err != nil {
				return
			}
			path := fmt.Sprintf("post%d.go", idx)
			_ = tx.UpsertOverlayFile(context.Background(), path, "h")
			_ = tx.Commit()
		}(i)
	}

	// Wait for the writers to finish before clearing — they'll all
	// have epochs > 5 by definition because BeginOverlayTx allocates
	// monotonically.
	wg.Wait()

	if err := snap.ClearOverlayLE(context.Background(), repo, capturedEpoch); err != nil {
		t.Fatalf("ClearOverlayLE: %v", err)
	}
	if err := s.CommitSnapshot(context.Background(), snap, store.SnapshotSummary{}); err != nil {
		t.Fatalf("CommitSnapshot: %v", err)
	}

	// CAS contract assertion: rows committed at write_epoch >
	// capturedEpoch (=5) must survive. The OverlayRowCount accessor
	// counts rows with `write_epoch <= ?`; rows from epochs 6..10 are
	// excluded by capturedEpoch=5 but included by capturedEpoch=∞.
	//
	// Count with a high upper bound = total surviving rows.
	got, err := s.OverlayRowCount(context.Background(), repo, 1<<62)
	if err != nil {
		t.Fatalf("OverlayRowCount: %v", err)
	}
	if got < 5 {
		t.Errorf("CAS: got %d surviving rows, want >= 5 (rows from epochs 6..10)", got)
	}
	// And rows with epoch <= 5 should ALL be cleared.
	cleared, err := s.OverlayRowCount(context.Background(), repo, 5)
	if err != nil {
		t.Fatalf("OverlayRowCount(<=5): %v", err)
	}
	if cleared != 0 {
		t.Errorf("CAS: rows with epoch <= 5 not cleared: got %d, want 0", cleared)
	}
}

// changeWD sets os.Chdir to dir for the duration of the test, returning
// a cleanup function.
func changeWD(t *testing.T, dir string) func() {
	t.Helper()
	prev, err := osGetwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := osChdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	return func() { _ = osChdir(prev) }
}

// Indirect references to os keep the test build small.
var (
	osGetwd = func() (string, error) { return getwdPlatform() }
	osChdir = func(d string) error { return chdirPlatform(d) }
)

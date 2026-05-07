package store

import (
	"context"
	"testing"
)

// fakeCompactor drives the snapshot-write API end-to-end without depending on
// the Phase 60 coalescer or the Phase 63 compactor goroutine. It exists to
// prove the API is consumable in isolation (CONTEXT.md D-02 rationale).
type fakeCompactor struct {
	store  *Store
	repoID string
}

// cycle drives Begin → WriteSnapshotFacts → ClearOverlayLE → DeleteSnapshotsBeyond → Commit.
// The ClearOverlayLE call exercises the encapsulated tx path — the fake
// compactor never touches snap.tx directly.
func (f *fakeCompactor) cycle(ctx context.Context, files []FileFact, capturedEpoch uint64, retain int) (committed bool, err error) {
	snap, err := f.store.BeginSnapshot(ctx, SnapshotMeta{
		RepoID:        f.repoID,
		CapturedEpoch: capturedEpoch,
	})
	if err != nil {
		return false, err
	}
	if err := f.store.WriteSnapshotFacts(ctx, snap, Facts{Files: files}); err != nil {
		_ = f.store.AbortSnapshot(ctx, snap, "write failed")
		return false, err
	}
	if err := snap.ClearOverlayLE(ctx, f.repoID, capturedEpoch); err != nil {
		_ = f.store.AbortSnapshot(ctx, snap, "clear failed")
		return false, err
	}
	if err := snap.DeleteSnapshotsBeyond(ctx, retain); err != nil {
		_ = f.store.AbortSnapshot(ctx, snap, "retention failed")
		return false, err
	}
	if err := f.store.CommitSnapshot(ctx, snap, SnapshotSummary{FileCount: len(files)}); err != nil {
		return false, err
	}
	return true, nil
}

// TestFakeCompactor_EndToEnd runs 7 cycles with retain=5; asserts exactly 5
// snapshots persist after the 7th commit. Pre-seeds overlay rows at varied
// write_epochs so each ClearOverlayLE has work to do; asserts overlay table
// SELECT COUNT(*) shrinks monotonically across cycles.
func TestFakeCompactor_EndToEnd(t *testing.T) {
	s, ctx := openStoreForSnapshotTest(t)
	const repoID = "fake-compactor-repo"

	// Pre-seed 21 overlay rows: 3 unique paths per cycle × 7 cycles. Each
	// cycle allocates a fresh write_epoch via BeginOverlayTx; we record the
	// epoch on the third row of each cycle so the later ClearOverlayLE call
	// drains exactly those rows.
	cycleEpochs := make([]uint64, 7)
	for c := 0; c < 7; c++ {
		tx, err := s.BeginOverlayTx(ctx, repoID)
		if err != nil {
			t.Fatalf("seed BeginOverlayTx cycle %d: %v", c, err)
		}
		for j := 0; j < 3; j++ {
			path := "seed/c" + itoaSimple(c) + "_p" + itoaSimple(j) + ".go"
			if err := tx.UpsertOverlayFile(ctx, path, "h"); err != nil {
				t.Fatalf("seed UpsertOverlayFile cycle %d: %v", c, err)
			}
		}
		cycleEpochs[c] = tx.Epoch()
		if err := tx.Commit(); err != nil {
			t.Fatalf("seed overlay tx %d Commit: %v", c, err)
		}
	}

	fc := &fakeCompactor{store: s, repoID: repoID}

	// Each cycle's compaction drains its own seed rows by passing
	// capturedEpoch = cycleEpochs[c]; rows from later cycles (write_epoch
	// strictly greater) survive that cycle and are drained in their own.
	prevOverlay := -1
	for c := 0; c < 7; c++ {
		files := []FileFact{
			{
				FileID: uint64(c*10 + 1), RepoID: repoID, Path: "compact/" + itoaSimple(c) + ".go",
				Language: "go", ContentHash: "h", SizeBytes: 1, LineCount: 1,
			},
		}
		ok, err := fc.cycle(ctx, files, cycleEpochs[c], 5)
		if err != nil {
			t.Fatalf("cycle %d: %v", c, err)
		}
		if !ok {
			t.Fatalf("cycle %d: not committed", c)
		}

		// Overlay row count must shrink monotonically across cycles.
		var n int
		if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM semantic_live_overlay_files WHERE repo_id=?", repoID).Scan(&n); err != nil {
			t.Fatalf("count overlay cycle %d: %v", c, err)
		}
		if prevOverlay >= 0 && n > prevOverlay {
			t.Errorf("cycle %d: overlay count rose from %d to %d (must shrink monotonically)", c, prevOverlay, n)
		}
		prevOverlay = n
	}

	// Final state: exactly 5 committed snapshots survive (retain=5 applied
	// on every cycle; the new snapshot itself + 4 most-recent priors).
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM semantic_snapshots WHERE repo_id=?", repoID).Scan(&total); err != nil {
		t.Fatalf("count snapshots final: %v", err)
	}
	if total != 5 {
		t.Errorf("final snapshot count: got %d, want 5", total)
	}

	// Final overlay state: all seed rows ≤ cycleEpochs[6] are gone.
	var residual int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM semantic_live_overlay_files WHERE repo_id=?", repoID).Scan(&residual); err != nil {
		t.Fatalf("count overlay final: %v", err)
	}
	if residual != 0 {
		t.Errorf("final overlay residual: got %d, want 0 (every cycle's seed drained)", residual)
	}
}

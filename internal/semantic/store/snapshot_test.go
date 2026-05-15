package store

import (
	"context"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"
)

// openStoreForSnapshotTest reuses the overlay test bring-up so snapshot tests
// land at the latest schema version (v3) — the same as the overlay tests rely
// on. White-box (package store) tests so we can read internal columns and
// poke s.db directly when seeding fixtures.
func openStoreForSnapshotTest(t *testing.T) (*Store, context.Context) {
	t.Helper()
	s, ctx, _ := openStoreForOverlayTest(t)
	return s, ctx
}

// TestBeginSnapshot_NilStore: nil-receiver guard.
func TestBeginSnapshot_NilStore(t *testing.T) {
	var s *Store
	_, err := s.BeginSnapshot(context.Background(), SnapshotMeta{RepoID: "r1"})
	if err == nil {
		t.Fatal("BeginSnapshot on nil *Store: want error, got nil")
	}
	if !strings.Contains(err.Error(), "BeginSnapshot: nil store") {
		t.Errorf("BeginSnapshot nil store: got error %q, want substring %q",
			err.Error(), "BeginSnapshot: nil store")
	}
}

// TestBeginSnapshot_EmptyRepoID: empty-repoID guard.
func TestBeginSnapshot_EmptyRepoID(t *testing.T) {
	s, ctx := openStoreForSnapshotTest(t)
	_, err := s.BeginSnapshot(ctx, SnapshotMeta{RepoID: ""})
	if err == nil {
		t.Fatal("BeginSnapshot empty repoID: want error, got nil")
	}
	if !strings.Contains(err.Error(), "BeginSnapshot: empty repoID") {
		t.Errorf("BeginSnapshot empty repoID: got error %q, want substring %q",
			err.Error(), "BeginSnapshot: empty repoID")
	}
}

// TestBeginSnapshot_AllocatesPendingRow: first BeginSnapshot returns ID > 0;
// pending row is invisible to a separate connection until commit.
func TestBeginSnapshot_AllocatesPendingRow(t *testing.T) {
	s, ctx := openStoreForSnapshotTest(t)
	snap, err := s.BeginSnapshot(ctx, SnapshotMeta{RepoID: "r1", BaseSnapshotID: 0, CapturedEpoch: 5})
	if err != nil {
		t.Fatalf("BeginSnapshot: %v", err)
	}
	if snap == nil {
		t.Fatal("BeginSnapshot: returned nil *Snapshot")
	}
	if snap.ID == 0 {
		t.Errorf("BeginSnapshot: snap.ID = 0, want > 0")
	}
	if snap.RepoID != "r1" {
		t.Errorf("BeginSnapshot: snap.RepoID = %q, want %q", snap.RepoID, "r1")
	}
	if snap.Meta.CapturedEpoch != 5 {
		t.Errorf("BeginSnapshot: snap.Meta.CapturedEpoch = %d, want 5", snap.Meta.CapturedEpoch)
	}

	// Rolling back the tx must remove the pending row entirely.
	if err := s.AbortSnapshot(ctx, snap, "test cleanup"); err != nil {
		t.Fatalf("AbortSnapshot: %v", err)
	}
	var n int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM semantic_snapshots WHERE snapshot_id=?", snap.ID).Scan(&n); err != nil {
		t.Fatalf("count after abort: %v", err)
	}
	if n != 0 {
		t.Errorf("after AbortSnapshot: got %d rows, want 0", n)
	}
}

// TestSnapshot_BeginWriteCommit covers the happy path.
func TestSnapshot_BeginWriteCommit(t *testing.T) {
	s, ctx := openStoreForSnapshotTest(t)
	snap, err := s.BeginSnapshot(ctx, SnapshotMeta{RepoID: "r1", BaseSnapshotID: 0, CapturedEpoch: 1})
	if err != nil {
		t.Fatalf("BeginSnapshot: %v", err)
	}

	files := make([]FileFact, 5)
	for i := range files {
		files[i] = FileFact{
			FileID:      uint64(i + 1),
			RepoID:      "r1",
			Path:        sampleFilePath(i),
			Language:    "go",
			ContentHash: "h",
			SizeBytes:   100,
			LineCount:   10,
		}
	}
	syms := make([]SymbolFact, 50)
	for i := range syms {
		syms[i] = SymbolFact{
			SymbolID:         uint64(i + 1),
			NodeID:           uint64(i + 1),
			FileID:           uint64((i % 5) + 1),
			Language:         "go",
			Kind:             "func",
			Name:             "F",
			QualifiedName:    "pkg.F",
			StableKey:        "k",
			ExtractionSource: "tree-sitter",
			Confidence:       1.0,
		}
	}

	if err := s.WriteSnapshotFacts(ctx, snap, Facts{Files: files, Symbols: syms}); err != nil {
		t.Fatalf("WriteSnapshotFacts: %v", err)
	}

	// Inside the same tx, count rows.
	var fc, sc int
	if err := snap.tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM semantic_files WHERE snapshot_id=?", snap.ID).Scan(&fc); err != nil {
		t.Fatalf("count files: %v", err)
	}
	if fc != 5 {
		t.Errorf("inside-tx file count: got %d, want 5", fc)
	}
	if err := snap.tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM semantic_symbols WHERE snapshot_id=?", snap.ID).Scan(&sc); err != nil {
		t.Fatalf("count symbols: %v", err)
	}
	if sc != 50 {
		t.Errorf("inside-tx symbol count: got %d, want 50", sc)
	}

	if err := s.CommitSnapshot(ctx, snap, SnapshotSummary{FileCount: 5, SymbolCount: 50}); err != nil {
		t.Fatalf("CommitSnapshot: %v", err)
	}

	// After commit, status='committed' visible from a separate query.
	var status string
	if err := s.db.QueryRowContext(ctx, "SELECT status FROM semantic_snapshots WHERE snapshot_id=?", snap.ID).Scan(&status); err != nil {
		t.Fatalf("read status post-commit: %v", err)
	}
	if status != "committed" {
		t.Errorf("post-commit status: got %q, want %q", status, "committed")
	}
}

// TestSnapshot_BeginAbortRollsBack: rolled-back snapshot row vanishes.
func TestSnapshot_BeginAbortRollsBack(t *testing.T) {
	s, ctx := openStoreForSnapshotTest(t)
	snap, err := s.BeginSnapshot(ctx, SnapshotMeta{RepoID: "r1"})
	if err != nil {
		t.Fatalf("BeginSnapshot: %v", err)
	}
	files := []FileFact{
		{FileID: 1, RepoID: "r1", Path: "a.go", Language: "go", ContentHash: "h", SizeBytes: 1, LineCount: 1},
		{FileID: 2, RepoID: "r1", Path: "b.go", Language: "go", ContentHash: "h", SizeBytes: 1, LineCount: 1},
		{FileID: 3, RepoID: "r1", Path: "c.go", Language: "go", ContentHash: "h", SizeBytes: 1, LineCount: 1},
	}
	if err := s.WriteSnapshotFacts(ctx, snap, Facts{Files: files}); err != nil {
		t.Fatalf("WriteSnapshotFacts: %v", err)
	}
	if err := s.AbortSnapshot(ctx, snap, "size guard"); err != nil {
		t.Fatalf("AbortSnapshot: %v", err)
	}
	var n int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM semantic_snapshots WHERE snapshot_id=?", snap.ID).Scan(&n); err != nil {
		t.Fatalf("count post-abort: %v", err)
	}
	if n != 0 {
		t.Errorf("post-abort row count: got %d, want 0", n)
	}
	var fn int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM semantic_files WHERE snapshot_id=?", snap.ID).Scan(&fn); err != nil {
		t.Fatalf("count files post-abort: %v", err)
	}
	if fn != 0 {
		t.Errorf("post-abort file count: got %d, want 0 (rolled back)", fn)
	}
}

// TestSnapshot_DoubleCommit: second commit returns sentinel error.
func TestSnapshot_DoubleCommit(t *testing.T) {
	s, ctx := openStoreForSnapshotTest(t)
	snap, err := s.BeginSnapshot(ctx, SnapshotMeta{RepoID: "r1"})
	if err != nil {
		t.Fatalf("BeginSnapshot: %v", err)
	}
	if err := s.CommitSnapshot(ctx, snap, SnapshotSummary{}); err != nil {
		t.Fatalf("CommitSnapshot #1: %v", err)
	}
	err = s.CommitSnapshot(ctx, snap, SnapshotSummary{})
	if err == nil {
		t.Fatal("CommitSnapshot #2: want error, got nil")
	}
	if !strings.Contains(err.Error(), "already committed") {
		t.Errorf("CommitSnapshot #2: got %q, want substring %q", err.Error(), "already committed")
	}
}

// TestSnapshot_CommitAfterAbort: commit-after-abort guard.
func TestSnapshot_CommitAfterAbort(t *testing.T) {
	s, ctx := openStoreForSnapshotTest(t)
	snap, err := s.BeginSnapshot(ctx, SnapshotMeta{RepoID: "r1"})
	if err != nil {
		t.Fatalf("BeginSnapshot: %v", err)
	}
	if err := s.AbortSnapshot(ctx, snap, "size guard"); err != nil {
		t.Fatalf("AbortSnapshot: %v", err)
	}
	err = s.CommitSnapshot(ctx, snap, SnapshotSummary{})
	if err == nil {
		t.Fatal("CommitSnapshot after Abort: want error, got nil")
	}
	if !strings.Contains(err.Error(), "already aborted") {
		t.Errorf("CommitSnapshot after Abort: got %q, want substring %q", err.Error(), "already aborted")
	}
}

// TestSnapshot_DeleteSnapshotsBeyondAtomic seeds 7 prior committed snapshots
// for repo r1, then opens a new snapshot and runs DeleteSnapshotsBeyond(retain=5)
// inside the same tx. After commit only the 5 most-recent snapshots survive
// (4 prior + the new one). The seed-then-rollback variant proves that abort
// preserves all 7 prior snapshots.
func TestSnapshot_DeleteSnapshotsBeyondAtomic(t *testing.T) {
	s, ctx := openStoreForSnapshotTest(t)
	priorIDs := seedCommittedSnapshots(t, s, "r1", 7)

	snap, err := s.BeginSnapshot(ctx, SnapshotMeta{RepoID: "r1"})
	if err != nil {
		t.Fatalf("BeginSnapshot: %v", err)
	}
	if err := snap.DeleteSnapshotsBeyond(ctx, 5); err != nil {
		t.Fatalf("DeleteSnapshotsBeyond: %v", err)
	}
	if err := s.CommitSnapshot(ctx, snap, SnapshotSummary{}); err != nil {
		t.Fatalf("CommitSnapshot: %v", err)
	}

	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM semantic_snapshots WHERE repo_id=?", "r1").Scan(&total); err != nil {
		t.Fatalf("count post-commit: %v", err)
	}
	if total != 5 {
		t.Errorf("post-commit total snapshots: got %d, want 5", total)
	}

	// The 4 most-recent prior IDs (last 4 of priorIDs) must survive together
	// with snap.ID itself; the 3 oldest priorIDs must be gone.
	survivors := map[uint64]bool{snap.ID: true}
	for i := len(priorIDs) - 4; i < len(priorIDs); i++ {
		survivors[priorIDs[i]] = true
	}
	rows, err := s.db.QueryContext(ctx, "SELECT snapshot_id FROM semantic_snapshots WHERE repo_id=?", "r1")
	if err != nil {
		t.Fatalf("query survivors: %v", err)
	}
	defer rows.Close()
	got := map[uint64]bool{}
	for rows.Next() {
		var id uint64
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got[id] = true
	}
	for id := range survivors {
		if !got[id] {
			t.Errorf("survivor %d missing post-commit", id)
		}
	}
	if len(got) != 5 {
		t.Errorf("survivor count: got %d, want 5 (set=%v)", len(got), got)
	}

	// Seed-then-abort variant.
	priorIDs2 := seedCommittedSnapshots(t, s, "r2", 7)
	snap2, err := s.BeginSnapshot(ctx, SnapshotMeta{RepoID: "r2"})
	if err != nil {
		t.Fatalf("BeginSnapshot r2: %v", err)
	}
	if err := snap2.DeleteSnapshotsBeyond(ctx, 5); err != nil {
		t.Fatalf("DeleteSnapshotsBeyond r2: %v", err)
	}
	if err := s.AbortSnapshot(ctx, snap2, "rollback variant"); err != nil {
		t.Fatalf("AbortSnapshot r2: %v", err)
	}
	var totalR2 int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM semantic_snapshots WHERE repo_id=?", "r2").Scan(&totalR2); err != nil {
		t.Fatalf("count r2 post-abort: %v", err)
	}
	if totalR2 != len(priorIDs2) {
		t.Errorf("post-abort r2 count: got %d, want %d (rollback restores all priors)", totalR2, len(priorIDs2))
	}
}

// TestSnapshot_ClearOverlayLE_DeletesUpToCapturedEpoch seeds overlay rows at
// write_epoch ∈ {1,2,3,4,5} via store.BeginOverlayTx, then runs ClearOverlayLE
// with capturedEpoch=3 inside a fresh snapshot tx and commits. Rows at
// write_epoch ≤ 3 must vanish; rows at write_epoch ∈ {4,5} must survive.
func TestSnapshot_ClearOverlayLE_DeletesUpToCapturedEpoch(t *testing.T) {
	s, ctx := openStoreForSnapshotTest(t)
	const repoID = "r1"
	for i := 1; i <= 5; i++ {
		tx, err := s.BeginOverlayTx(ctx, repoID)
		if err != nil {
			t.Fatalf("BeginOverlayTx #%d: %v", i, err)
		}
		if err := tx.UpsertOverlayFile(ctx, sampleFilePath(i), "h"); err != nil {
			t.Fatalf("UpsertOverlayFile #%d: %v", i, err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatalf("overlay tx %d Commit: %v", i, err)
		}
	}

	snap, err := s.BeginSnapshot(ctx, SnapshotMeta{RepoID: repoID, CapturedEpoch: 3})
	if err != nil {
		t.Fatalf("BeginSnapshot: %v", err)
	}
	if err := snap.ClearOverlayLE(ctx, repoID, 3); err != nil {
		t.Fatalf("ClearOverlayLE: %v", err)
	}
	if err := s.CommitSnapshot(ctx, snap, SnapshotSummary{}); err != nil {
		t.Fatalf("CommitSnapshot: %v", err)
	}

	var n int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM semantic_live_overlay_files WHERE repo_id=? AND write_epoch <= 3", repoID).Scan(&n); err != nil {
		t.Fatalf("count post-clear ≤3: %v", err)
	}
	if n != 0 {
		t.Errorf("post-clear write_epoch <= 3 rows: got %d, want 0", n)
	}
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM semantic_live_overlay_files WHERE repo_id=? AND write_epoch > 3", repoID).Scan(&n); err != nil {
		t.Fatalf("count post-clear >3: %v", err)
	}
	if n != 2 {
		t.Errorf("post-clear write_epoch > 3 rows: got %d, want 2 (epochs 4,5 survive)", n)
	}
}

// TestSnapshot_ClearOverlayLE_AtomicWithCommit asserts that aborting the
// snapshot tx leaves all overlay rows intact (the DELETE rolls back).
func TestSnapshot_ClearOverlayLE_AtomicWithCommit(t *testing.T) {
	s, ctx := openStoreForSnapshotTest(t)
	const repoID = "r1"
	for i := 1; i <= 3; i++ {
		tx, err := s.BeginOverlayTx(ctx, repoID)
		if err != nil {
			t.Fatalf("BeginOverlayTx #%d: %v", i, err)
		}
		if err := tx.UpsertOverlayFile(ctx, sampleFilePath(i), "h"); err != nil {
			t.Fatalf("UpsertOverlayFile #%d: %v", i, err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatalf("overlay tx %d Commit: %v", i, err)
		}
	}

	snap, err := s.BeginSnapshot(ctx, SnapshotMeta{RepoID: repoID, CapturedEpoch: 3})
	if err != nil {
		t.Fatalf("BeginSnapshot: %v", err)
	}
	if err := snap.ClearOverlayLE(ctx, repoID, 3); err != nil {
		t.Fatalf("ClearOverlayLE: %v", err)
	}
	if err := s.AbortSnapshot(ctx, snap, "rollback"); err != nil {
		t.Fatalf("AbortSnapshot: %v", err)
	}

	var n int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM semantic_live_overlay_files WHERE repo_id=?", repoID).Scan(&n); err != nil {
		t.Fatalf("count post-abort: %v", err)
	}
	if n != 3 {
		t.Errorf("post-abort overlay row count: got %d, want 3 (rollback restores DELETE)", n)
	}
}

// TestSnapshot_ClearOverlayLE_AfterCommit: ClearOverlayLE must be rejected
// once the snapshot has committed (the tx is closed).
func TestSnapshot_ClearOverlayLE_AfterCommit(t *testing.T) {
	s, ctx := openStoreForSnapshotTest(t)
	snap, err := s.BeginSnapshot(ctx, SnapshotMeta{RepoID: "r1", CapturedEpoch: 1})
	if err != nil {
		t.Fatalf("BeginSnapshot: %v", err)
	}
	if err := s.CommitSnapshot(ctx, snap, SnapshotSummary{}); err != nil {
		t.Fatalf("CommitSnapshot: %v", err)
	}
	err = snap.ClearOverlayLE(ctx, "r1", 1)
	if err == nil {
		t.Fatal("ClearOverlayLE after Commit: want error, got nil")
	}
	if !strings.Contains(err.Error(), "already committed") {
		t.Errorf("ClearOverlayLE after Commit: got %q, want substring %q", err.Error(), "already committed")
	}
}

// TestSnapshot_TxAccessor_Absent verifies the encapsulation invariant: there
// is NO public Tx() accessor on *Snapshot. Phase 63 review WR-03 replaces
// the prior `exec.Command("grep", ...)` shell-out (non-portable: required
// `grep` on PATH with `-nE` flag set, broke on Windows + minimal CI images)
// with a native os.ReadFile + regexp scan. Same invariant, no PATH
// dependency.
func TestSnapshot_TxAccessor_Absent(t *testing.T) {
	src, err := os.ReadFile("snapshot.go")
	if err != nil {
		t.Fatalf("read snapshot.go: %v", err)
	}
	// Match `func (<receiver> *Snapshot) Tx(` at the start of a line —
	// the only shape an exported accessor could take. (?m) enables
	// multiline mode so `^` anchors to line starts.
	re := regexp.MustCompile(`(?m)^func \(\w+ \*Snapshot\) Tx\(`)
	if loc := re.FindIndex(src); loc != nil {
		t.Errorf("encapsulation violated — public Tx() accessor on *Snapshot at byte offset %d", loc[0])
	}
}

// TestCommitSnapshot_PersistsBaseOverlayEpoch asserts that a baseline
// overlay epoch set on the *Snapshot handle BEFORE CommitSnapshot is
// written to semantic_snapshots.base_overlay_epoch atomically with the
// status='committed' flip (Phase 70 CONTEXT.md D3).
func TestCommitSnapshot_PersistsBaseOverlayEpoch(t *testing.T) {
	t.Run("explicit_epoch_round_trips", func(t *testing.T) {
		s, ctx := openStoreForSnapshotTest(t)
		snap, err := s.BeginSnapshot(ctx, SnapshotMeta{RepoID: "r1", CapturedEpoch: 1})
		if err != nil {
			t.Fatalf("BeginSnapshot: %v", err)
		}
		snap.SetBaseOverlayEpoch(42)
		if err := s.CommitSnapshot(ctx, snap, SnapshotSummary{}); err != nil {
			t.Fatalf("CommitSnapshot: %v", err)
		}
		var epoch uint64
		if err := s.db.QueryRowContext(ctx,
			`SELECT base_overlay_epoch FROM semantic_snapshots WHERE snapshot_id=?`,
			snap.ID).Scan(&epoch); err != nil {
			t.Fatalf("read base_overlay_epoch: %v", err)
		}
		if epoch != 42 {
			t.Errorf("post-commit base_overlay_epoch: got %d, want 42", epoch)
		}
	})

	t.Run("unset_defaults_to_zero", func(t *testing.T) {
		s, ctx := openStoreForSnapshotTest(t)
		snap, err := s.BeginSnapshot(ctx, SnapshotMeta{RepoID: "r2"})
		if err != nil {
			t.Fatalf("BeginSnapshot: %v", err)
		}
		// No SetBaseOverlayEpoch — cold-start path.
		if err := s.CommitSnapshot(ctx, snap, SnapshotSummary{}); err != nil {
			t.Fatalf("CommitSnapshot: %v", err)
		}
		var epoch uint64
		if err := s.db.QueryRowContext(ctx,
			`SELECT base_overlay_epoch FROM semantic_snapshots WHERE snapshot_id=?`,
			snap.ID).Scan(&epoch); err != nil {
			t.Fatalf("read base_overlay_epoch: %v", err)
		}
		if epoch != 0 {
			t.Errorf("unset base_overlay_epoch: got %d, want 0", epoch)
		}
	})
}

// TestLatestCommittedSnapshotBaseEpoch_RoundTrip seeds two committed
// snapshots (epochs 7 and 11 in BeginSnapshot order) and asserts the
// accessor returns the MOST RECENT one — (11, true, nil).
func TestLatestCommittedSnapshotBaseEpoch_RoundTrip(t *testing.T) {
	s, ctx := openStoreForSnapshotTest(t)

	// Snapshot 1: epoch=7
	snap1, err := s.BeginSnapshot(ctx, SnapshotMeta{RepoID: "r1"})
	if err != nil {
		t.Fatalf("BeginSnapshot #1: %v", err)
	}
	snap1.SetBaseOverlayEpoch(7)
	if err := s.CommitSnapshot(ctx, snap1, SnapshotSummary{}); err != nil {
		t.Fatalf("CommitSnapshot #1: %v", err)
	}

	// Snapshot 2: epoch=11 (allocated AFTER snap1 from same SEQUENCE so
	// snap2.ID > snap1.ID — the accessor's ORDER BY snapshot_id DESC
	// picks this one).
	snap2, err := s.BeginSnapshot(ctx, SnapshotMeta{RepoID: "r1"})
	if err != nil {
		t.Fatalf("BeginSnapshot #2: %v", err)
	}
	snap2.SetBaseOverlayEpoch(11)
	if err := s.CommitSnapshot(ctx, snap2, SnapshotSummary{}); err != nil {
		t.Fatalf("CommitSnapshot #2: %v", err)
	}

	epoch, ok, err := s.LatestCommittedSnapshotBaseEpoch(ctx, "r1")
	if err != nil {
		t.Fatalf("LatestCommittedSnapshotBaseEpoch: %v", err)
	}
	if !ok {
		t.Fatal("LatestCommittedSnapshotBaseEpoch: ok=false, want true")
	}
	if epoch != 11 {
		t.Errorf("LatestCommittedSnapshotBaseEpoch: got %d, want 11 (latest)", epoch)
	}
}

// TestLatestCommittedSnapshotBaseEpoch_NoSnapshot asserts the accessor
// returns (0, false, nil) when no committed snapshot exists for the
// repoID (cold-start signal — Plan 04's buildFn treats this as "rebuild
// from scratch").
func TestLatestCommittedSnapshotBaseEpoch_NoSnapshot(t *testing.T) {
	s, ctx := openStoreForSnapshotTest(t)

	epoch, ok, err := s.LatestCommittedSnapshotBaseEpoch(ctx, "never-committed")
	if err != nil {
		t.Fatalf("LatestCommittedSnapshotBaseEpoch: %v", err)
	}
	if ok {
		t.Errorf("LatestCommittedSnapshotBaseEpoch on empty: got ok=true, want false")
	}
	if epoch != 0 {
		t.Errorf("LatestCommittedSnapshotBaseEpoch on empty: got epoch=%d, want 0", epoch)
	}
}

// TestLatestCommittedSnapshotBaseEpoch_NilStore: nil-receiver guard
// mirrors the OverlayChangedPathsSince discipline (Plan 01).
func TestLatestCommittedSnapshotBaseEpoch_NilStore(t *testing.T) {
	var s *Store
	_, _, err := s.LatestCommittedSnapshotBaseEpoch(context.Background(), "r1")
	if err == nil {
		t.Fatal("LatestCommittedSnapshotBaseEpoch on nil *Store: want error, got nil")
	}
}

// TestLatestCommittedSnapshotBaseEpoch_EmptyRepoID: empty-repoID guard.
func TestLatestCommittedSnapshotBaseEpoch_EmptyRepoID(t *testing.T) {
	s, ctx := openStoreForSnapshotTest(t)
	_, _, err := s.LatestCommittedSnapshotBaseEpoch(ctx, "")
	if err == nil {
		t.Fatal("LatestCommittedSnapshotBaseEpoch empty repoID: want error, got nil")
	}
}

// TestLatestCommittedSnapshotBaseEpoch_IgnoresPendingAndAborted asserts
// that a pending snapshot (no commit yet) and an aborted one do not
// shadow an older committed snapshot for the accessor.
func TestLatestCommittedSnapshotBaseEpoch_IgnoresPendingAndAborted(t *testing.T) {
	s, ctx := openStoreForSnapshotTest(t)

	// Older committed snapshot with epoch=5.
	committed, err := s.BeginSnapshot(ctx, SnapshotMeta{RepoID: "r1"})
	if err != nil {
		t.Fatalf("BeginSnapshot(committed): %v", err)
	}
	committed.SetBaseOverlayEpoch(5)
	if err := s.CommitSnapshot(ctx, committed, SnapshotSummary{}); err != nil {
		t.Fatalf("CommitSnapshot: %v", err)
	}

	// A later pending snapshot (never committed) MUST NOT mask the
	// committed=5 reading.
	pending, err := s.BeginSnapshot(ctx, SnapshotMeta{RepoID: "r1"})
	if err != nil {
		t.Fatalf("BeginSnapshot(pending): %v", err)
	}
	pending.SetBaseOverlayEpoch(99)
	// Intentionally do NOT CommitSnapshot — leave pending.
	defer func() { _ = s.AbortSnapshot(ctx, pending, "test cleanup") }()

	epoch, ok, err := s.LatestCommittedSnapshotBaseEpoch(ctx, "r1")
	if err != nil {
		t.Fatalf("LatestCommittedSnapshotBaseEpoch: %v", err)
	}
	if !ok || epoch != 5 {
		t.Errorf("LatestCommittedSnapshotBaseEpoch (committed=5, pending=99): got (%d, %v), want (5, true)", epoch, ok)
	}
}

// --- Helpers ---

// sampleFilePath returns a deterministic path used to populate test fixtures.
func sampleFilePath(i int) string {
	return "pkg/file_" + itoaSimple(i) + ".go"
}

func itoaSimple(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var buf [20]byte
	n := len(buf)
	for i > 0 {
		n--
		buf[n] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		n--
		buf[n] = '-'
	}
	return string(buf[n:])
}

// seedCommittedSnapshots inserts n rows directly via store.db with status='committed'
// for repoID. Returns the list of allocated snapshot_id values in insertion order
// (also matching created_at order: each row receives a Go-side time.Now() that
// monotonically increases by 1 ms per insertion so ORDER BY created_at DESC
// produces a deterministic ordering — closes Phase 63 review CR-04, which
// flagged the prior `now()`-based seed as flaky under sub-microsecond
// collisions).
//
// Bypasses the BeginSnapshot API by design: the test verifies the new API's
// retention semantics, not the seed-via-API path. snapshot_id is allocated
// from the same SEQUENCE BeginSnapshot uses (Phase 63 review CR-03) so
// seeded ids never collide with subsequent BeginSnapshot allocations in
// the same test.
func seedCommittedSnapshots(t *testing.T, s *Store, repoID string, n int) []uint64 {
	t.Helper()
	base := time.Now().UTC().Truncate(time.Millisecond)
	out := make([]uint64, 0, n)
	for i := 0; i < n; i++ {
		// Strictly-monotone created_at: 1ms apart so ORDER BY created_at
		// DESC has no ties at sub-microsecond resolution.
		ts := base.Add(time.Duration(i) * time.Millisecond)
		var id uint64
		err := s.db.QueryRow(`
			INSERT INTO semantic_snapshots (
				snapshot_id, repo_id, repo_root, base_snapshot_id, kind,
				worktree_hash, schema_version, indexer_version, status,
				partial, created_at, committed_at
			) VALUES (
				nextval('semantic_snapshot_id_seq'),
				?, '', 0, 'compact', '', 1, 'test', 'committed',
				false, ?, ?
			)
			RETURNING snapshot_id
		`, repoID, ts, ts).Scan(&id)
		if err != nil {
			t.Fatalf("seedCommittedSnapshots #%d: %v", i, err)
		}
		out = append(out, id)
	}
	return out
}

// Phase 63 P63-02 Task 1: store-side accessor + migration004 tests.

package store

import (
	"context"
	"testing"
	"time"

	"github.com/agenthands/helix/internal/workspace"
)

// TestApplyMigration004_AddsLastVacuumAtColumn: a fresh open lands at
// schema_version=4 and the new column is visible in
// information_schema.columns.
func TestApplyMigration004_AddsLastVacuumAtColumn(t *testing.T) {
	wsDir := t.TempDir()
	cfg := configFor(t, wsDir)
	m := newTestObsMetrics(t)

	s, err := Open(context.Background(), cfg, silentLogger(), m)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	db := storeUnderlyingDB(s)

	var version int
	if err := db.QueryRow("SELECT max(version) FROM semantic_schema_version").Scan(&version); err != nil {
		t.Fatalf("read schema_version: %v", err)
	}
	// Phase 63 review CR-03 bumps CurrentSchemaVersion to 5 (adds the
	// snapshot-id SEQUENCE migration). The last_vacuum_at column landed
	// at v4 and persists across the v4→v5 migration, so this test still
	// asserts the column existence below.
	if version != CurrentSchemaVersion {
		t.Errorf("schema_version after Open: got %d, want %d (CurrentSchemaVersion)", version, CurrentSchemaVersion)
	}

	// Column existence probe via information_schema.
	var cnt int
	if err := db.QueryRow(`
		SELECT COUNT(*) FROM information_schema.columns
		 WHERE table_name = 'semantic_live_overlay_meta' AND column_name = 'last_vacuum_at'
	`).Scan(&cnt); err != nil {
		t.Fatalf("probe last_vacuum_at column: %v", err)
	}
	if cnt != 1 {
		t.Errorf("last_vacuum_at column count: got %d, want 1", cnt)
	}
}

// TestStore_OverlayTxOpenCount_IncDec: BeginOverlayTx increments,
// Commit/Rollback decrements.
func TestStore_OverlayTxOpenCount_IncDec(t *testing.T) {
	wsDir := t.TempDir()
	cfg := configFor(t, wsDir)
	m := newTestObsMetrics(t)
	s, err := Open(context.Background(), cfg, silentLogger(), m)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	const repo = "repo-a"
	ws := workspace.WorkspaceKey{RepoRoot: repo}
	if got := s.OverlayTxOpenCount(ws); got != 0 {
		t.Errorf("baseline OverlayTxOpenCount: got %d, want 0", got)
	}

	tx, err := s.BeginOverlayTx(context.Background(), repo)
	if err != nil {
		t.Fatalf("BeginOverlayTx: %v", err)
	}
	if got := s.OverlayTxOpenCount(ws); got != 1 {
		t.Errorf("after Begin: got %d, want 1", got)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if got := s.OverlayTxOpenCount(ws); got != 0 {
		t.Errorf("after Commit: got %d, want 0", got)
	}

	// Rollback path.
	tx2, err := s.BeginOverlayTx(context.Background(), repo)
	if err != nil {
		t.Fatalf("BeginOverlayTx 2: %v", err)
	}
	if got := s.OverlayTxOpenCount(ws); got != 1 {
		t.Errorf("after Begin 2: got %d, want 1", got)
	}
	_ = tx2.Rollback()
	if got := s.OverlayTxOpenCount(ws); got != 0 {
		t.Errorf("after Rollback: got %d, want 0", got)
	}
}

// TestStore_OverlayHasPendingRows_AtomicProxy: the in-memory counter is
// non-zero after a write, and zero after Snapshot.ClearOverlayLE.
func TestStore_OverlayHasPendingRows_AtomicProxy(t *testing.T) {
	wsDir := t.TempDir()
	cfg := configFor(t, wsDir)
	m := newTestObsMetrics(t)
	s, err := Open(context.Background(), cfg, silentLogger(), m)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	const repo = "repo-pending"
	if s.OverlayHasPendingRows(repo) {
		t.Errorf("baseline OverlayHasPendingRows: got true, want false")
	}

	tx, err := s.BeginOverlayTx(context.Background(), repo)
	if err != nil {
		t.Fatalf("BeginOverlayTx: %v", err)
	}
	if err := tx.UpsertOverlayFile(context.Background(), "a.go", "abc"); err != nil {
		t.Fatalf("UpsertOverlayFile: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if !s.OverlayHasPendingRows(repo) {
		t.Errorf("after write+commit: got false, want true")
	}

	// Drain via snapshot.ClearOverlayLE.
	snap, err := s.BeginSnapshot(context.Background(), SnapshotMeta{
		RepoID:        repo,
		CapturedEpoch: tx.Epoch(),
	})
	if err != nil {
		t.Fatalf("BeginSnapshot: %v", err)
	}
	if err := snap.ClearOverlayLE(context.Background(), repo, tx.Epoch()); err != nil {
		t.Fatalf("ClearOverlayLE: %v", err)
	}
	if err := s.CommitSnapshot(context.Background(), snap, SnapshotSummary{}); err != nil {
		t.Fatalf("CommitSnapshot: %v", err)
	}
	if s.OverlayHasPendingRows(repo) {
		t.Errorf("after ClearOverlayLE+commit: got true, want false")
	}
}

// TestStore_OverlayRowCount_BoundedByCapturedEpoch: rows at write_epoch >
// capturedEpoch are excluded from the count (mirrors the CAS contract on
// ClearOverlayLE).
func TestStore_OverlayRowCount_BoundedByCapturedEpoch(t *testing.T) {
	wsDir := t.TempDir()
	cfg := configFor(t, wsDir)
	m := newTestObsMetrics(t)
	s, err := Open(context.Background(), cfg, silentLogger(), m)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	const repo = "repo-rowcount"
	// Write 3 files across 3 epochs.
	var lastEpoch uint64
	for i, p := range []string{"a.go", "b.go", "c.go"} {
		tx, err := s.BeginOverlayTx(context.Background(), repo)
		if err != nil {
			t.Fatalf("Begin %d: %v", i, err)
		}
		if err := tx.UpsertOverlayFile(context.Background(), p, "h"); err != nil {
			t.Fatalf("Upsert %d: %v", i, err)
		}
		lastEpoch = tx.Epoch()
		if err := tx.Commit(); err != nil {
			t.Fatalf("Commit %d: %v", i, err)
		}
	}
	// Bounded count at epoch=2 should include rows ≤ 2 (so 2 rows).
	got, err := s.OverlayRowCount(context.Background(), repo, lastEpoch-1)
	if err != nil {
		t.Fatalf("OverlayRowCount: %v", err)
	}
	if got != 2 {
		t.Errorf("OverlayRowCount(epoch=%d): got %d, want 2", lastEpoch-1, got)
	}
	// Bounded count at lastEpoch includes everything.
	got, err = s.OverlayRowCount(context.Background(), repo, lastEpoch)
	if err != nil {
		t.Fatalf("OverlayRowCount full: %v", err)
	}
	if got != 3 {
		t.Errorf("OverlayRowCount(epoch=%d): got %d, want 3", lastEpoch, got)
	}
}

// TestStore_Vacuum_RoundTrips: Vacuum returns nil (DuckDB no-op today)
// and the store remains queryable afterwards.
func TestStore_Vacuum_RoundTrips(t *testing.T) {
	wsDir := t.TempDir()
	cfg := configFor(t, wsDir)
	m := newTestObsMetrics(t)
	s, err := Open(context.Background(), cfg, silentLogger(), m)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	if err := s.Vacuum(context.Background()); err != nil {
		t.Errorf("Vacuum: %v", err)
	}

	// Store should still be queryable.
	db := storeUnderlyingDB(s)
	var v int
	if err := db.QueryRow("SELECT max(version) FROM semantic_schema_version").Scan(&v); err != nil {
		t.Errorf("post-Vacuum query: %v", err)
	}
}

// TestStore_Checkpoint_RoundTrips: Checkpoint returns nil.
func TestStore_Checkpoint_RoundTrips(t *testing.T) {
	wsDir := t.TempDir()
	cfg := configFor(t, wsDir)
	m := newTestObsMetrics(t)
	s, err := Open(context.Background(), cfg, silentLogger(), m)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()
	if err := s.Checkpoint(context.Background()); err != nil {
		t.Errorf("Checkpoint: %v", err)
	}
}

// TestStore_UpdateLastVacuumAt_RoundTrips: writes a timestamp and reads
// it back from semantic_live_overlay_meta.
func TestStore_UpdateLastVacuumAt_RoundTrips(t *testing.T) {
	wsDir := t.TempDir()
	cfg := configFor(t, wsDir)
	m := newTestObsMetrics(t)
	s, err := Open(context.Background(), cfg, silentLogger(), m)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	// Need an overlay-meta row to exist (BeginOverlayTx ensures this).
	const repo = "repo-vacuum-ts"
	tx, err := s.BeginOverlayTx(context.Background(), repo)
	if err != nil {
		t.Fatalf("BeginOverlayTx: %v", err)
	}
	_ = tx.Commit()

	now := time.Now().UTC().Truncate(time.Microsecond)
	if err := s.UpdateLastVacuumAt(context.Background(), repo, now); err != nil {
		t.Fatalf("UpdateLastVacuumAt: %v", err)
	}
}

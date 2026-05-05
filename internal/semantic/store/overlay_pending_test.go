package store

import (
	"testing"
)

// seedSemanticFileRow inserts a minimal semantic_files row so the
// MarkFileSemanticPending UPDATE has a target row. The row mirrors the
// Phase 59 fact-emitter contract (NOT NULL columns get sensible test values).
func seedSemanticFileRow(t *testing.T, s *Store, repoID, path string) {
	t.Helper()
	_, err := s.db.Exec(`
		INSERT INTO semantic_files (
			snapshot_id, file_id, repo_id, path, language,
			content_hash, size_bytes, line_count, indexed_at
		) VALUES (1, 1, ?, ?, 'go', 'hash', 0, 0, now())
	`, repoID, path)
	if err != nil {
		t.Fatalf("seed semantic_files row: %v", err)
	}
}

// TestOverlayTx_MarkFileSemanticPending_StampsClosedEnumReason (P1):
// MarkFileSemanticPending(ctx, path, "preempted") sets extraction_partial=true
// and partial_reason="preempted" on the row.
//
// Note: the on-disk schema column name is `extraction_partial`, not `partial`
// (Phase 59 migration 002 added `extraction_partial BOOLEAN` and
// `partial_reason TEXT` to semantic_files; only semantic_symbols /
// semantic_references got the bare `partial` column). The 61-01 plan text
// reads "partial=TRUE" but the column it maps to on semantic_files is
// extraction_partial — see overlay.go for the SQL.
func TestOverlayTx_MarkFileSemanticPending_StampsClosedEnumReason(t *testing.T) {
	s, ctx, _ := openStoreForOverlayTest(t)
	seedSemanticFileRow(t, s, "ws1", "src/a.go")

	tx, err := s.BeginOverlayTx(ctx, "ws1")
	if err != nil {
		t.Fatalf("BeginOverlayTx: %v", err)
	}
	if err := tx.MarkFileSemanticPending(ctx, "src/a.go", "preempted"); err != nil {
		t.Fatalf("MarkFileSemanticPending: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	var (
		extractionPartial bool
		reason            string
	)
	if err := s.db.QueryRow(`SELECT extraction_partial, partial_reason
		FROM semantic_files
		WHERE repo_id = 'ws1' AND path = 'src/a.go'`).
		Scan(&extractionPartial, &reason); err != nil {
		t.Fatalf("read semantic_files row: %v", err)
	}
	if !extractionPartial {
		t.Errorf("extraction_partial: got false, want true")
	}
	if reason != "preempted" {
		t.Errorf("partial_reason: got %q, want %q", reason, "preempted")
	}
}

// TestOverlayTx_MarkFileSemanticPending_AcceptsAllClosedEnumReasons (P2):
// All four valid reasons accepted (preempted, bulk_update_pending,
// lsp_unavailable, "budget exhausted").
func TestOverlayTx_MarkFileSemanticPending_AcceptsAllClosedEnumReasons(t *testing.T) {
	s, ctx, _ := openStoreForOverlayTest(t)

	cases := []string{"preempted", "bulk_update_pending", "lsp_unavailable", "budget exhausted"}
	for i, reason := range cases {
		path := "src/p" + string(rune('a'+i)) + ".go"
		seedSemanticFileRow(t, s, "ws1", path)

		tx, err := s.BeginOverlayTx(ctx, "ws1")
		if err != nil {
			t.Fatalf("BeginOverlayTx for %q: %v", reason, err)
		}
		if err := tx.MarkFileSemanticPending(ctx, path, reason); err != nil {
			t.Errorf("MarkFileSemanticPending(%q): want nil, got %v", reason, err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatalf("Commit: %v", err)
		}

		var got string
		if err := s.db.QueryRow(`SELECT partial_reason FROM semantic_files
			WHERE repo_id = 'ws1' AND path = ?`, path).Scan(&got); err != nil {
			t.Fatalf("read row for %q: %v", reason, err)
		}
		if got != reason {
			t.Errorf("partial_reason persisted: got %q, want %q", got, reason)
		}
	}
}

// TestOverlayTx_MarkFileSemanticPending_RejectsUnknownReason (P3):
// An invalid reason returns an error AND does not write.
func TestOverlayTx_MarkFileSemanticPending_RejectsUnknownReason(t *testing.T) {
	s, ctx, _ := openStoreForOverlayTest(t)
	seedSemanticFileRow(t, s, "ws1", "src/x.go")

	tx, err := s.BeginOverlayTx(ctx, "ws1")
	if err != nil {
		t.Fatalf("BeginOverlayTx: %v", err)
	}
	if err := tx.MarkFileSemanticPending(ctx, "src/x.go", "garbage"); err == nil {
		t.Fatal("MarkFileSemanticPending(garbage): want error, got nil")
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	// The row's extraction_partial must remain false (no write happened).
	var (
		extractionPartial bool
		reason            *string // nullable
	)
	if err := s.db.QueryRow(`SELECT extraction_partial, partial_reason
		FROM semantic_files
		WHERE repo_id = 'ws1' AND path = 'src/x.go'`).
		Scan(&extractionPartial, &reason); err != nil {
		t.Fatalf("read row: %v", err)
	}
	if extractionPartial {
		t.Errorf("extraction_partial: got true, want false (rejected reason must NOT write)")
	}
	if reason != nil {
		t.Errorf("partial_reason: got %v, want nil", *reason)
	}
}

// TestOverlayTx_MarkFileSemanticPending_NoRowIsNoOp (P4 — adapted from plan):
// When the file row does not exist on semantic_files, MarkFileSemanticPending
// returns nil (UPDATE 0 rows is not an error). This adapts the plan's P4
// "creates row if not exists" — semantic_files requires snapshot_id+file_id
// (Phase 59 fact-emitter contract); the per-file-pending API is purely an
// UPDATE of partial_reason / extraction_partial against rows already emitted
// by the extractor. The Phase 60 overlay tables (semantic_live_overlay_files)
// don't carry partial_reason columns. Per CONTEXT D-05's "persisting via the
// existing partial_reason columns on semantic_files" the UPDATE-only form
// matches the schema invariant; rows for paths the extractor has not yet
// processed simply have no marker.
func TestOverlayTx_MarkFileSemanticPending_NoRowIsNoOp(t *testing.T) {
	s, ctx, _ := openStoreForOverlayTest(t)

	tx, err := s.BeginOverlayTx(ctx, "ws1")
	if err != nil {
		t.Fatalf("BeginOverlayTx: %v", err)
	}
	if err := tx.MarkFileSemanticPending(ctx, "src/never-extracted.go", "preempted"); err != nil {
		t.Errorf("MarkFileSemanticPending on absent row: want nil, got %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
}

// TestOverlayTx_MarkFileSemanticPending_EpochAdvancesPerCommit (P5):
// Two BeginOverlayTx → MarkFileSemanticPending → Commit cycles; assert
// second.Epoch() == first.Epoch() + 1 (epoch advances by exactly 1 per
// Commit, mirroring the D-04 contract documented in overlay.go preamble).
func TestOverlayTx_MarkFileSemanticPending_EpochAdvancesPerCommit(t *testing.T) {
	s, ctx, _ := openStoreForOverlayTest(t)
	seedSemanticFileRow(t, s, "ws1", "src/e.go")

	tx1, err := s.BeginOverlayTx(ctx, "ws1")
	if err != nil {
		t.Fatalf("BeginOverlayTx #1: %v", err)
	}
	firstEpoch := tx1.Epoch()
	if err := tx1.MarkFileSemanticPending(ctx, "src/e.go", "preempted"); err != nil {
		t.Fatalf("MarkFileSemanticPending #1: %v", err)
	}
	if err := tx1.Commit(); err != nil {
		t.Fatalf("Commit #1: %v", err)
	}

	tx2, err := s.BeginOverlayTx(ctx, "ws1")
	if err != nil {
		t.Fatalf("BeginOverlayTx #2: %v", err)
	}
	secondEpoch := tx2.Epoch()
	if err := tx2.MarkFileSemanticPending(ctx, "src/e.go", "lsp_unavailable"); err != nil {
		t.Fatalf("MarkFileSemanticPending #2: %v", err)
	}
	if err := tx2.Commit(); err != nil {
		t.Fatalf("Commit #2: %v", err)
	}

	if secondEpoch != firstEpoch+1 {
		t.Errorf("epoch advance: first=%d second=%d (expected second = first+1; D-04 monotone-by-commit)",
			firstEpoch, secondEpoch)
	}
}

package store

import (
	"context"
	"testing"
	"time"
)

// TestBeginOverlayTx_AllocatesEpoch covers Behavior 4 from 60-02-PLAN.md:
// the first BeginOverlayTx for a workspace returns epoch=1; commits it;
// the second BeginOverlayTx returns epoch=2.
func TestBeginOverlayTx_AllocatesEpoch(t *testing.T) {
	s, ctx, _ := openStoreForOverlayTest(t)

	tx1, err := s.BeginOverlayTx(ctx, "ws1")
	if err != nil {
		t.Fatalf("BeginOverlayTx #1: %v", err)
	}
	if tx1.Epoch() != 1 {
		t.Errorf("tx1 epoch: got %d, want 1", tx1.Epoch())
	}
	if tx1.RepoID() != "ws1" {
		t.Errorf("tx1 repoID: got %q, want %q", tx1.RepoID(), "ws1")
	}
	if err := tx1.Commit(); err != nil {
		t.Fatalf("tx1.Commit: %v", err)
	}

	// Read back through s.db: meta row must reflect current_epoch=1.
	var ce uint64
	if err := s.db.QueryRow(`SELECT current_epoch FROM semantic_live_overlay_meta
		WHERE repo_id = 'ws1'`).Scan(&ce); err != nil {
		t.Fatalf("read current_epoch: %v", err)
	}
	if ce != 1 {
		t.Errorf("after tx1.Commit: current_epoch=%d, want 1", ce)
	}

	tx2, err := s.BeginOverlayTx(ctx, "ws1")
	if err != nil {
		t.Fatalf("BeginOverlayTx #2: %v", err)
	}
	if tx2.Epoch() != 2 {
		t.Errorf("tx2 epoch: got %d, want 2", tx2.Epoch())
	}
	if err := tx2.Commit(); err != nil {
		t.Fatalf("tx2.Commit: %v", err)
	}
}

// TestBeginOverlayTx_RollbackPreservesEpoch covers Behavior 5 (D-04
// invariant: rollback does NOT rewind current_epoch). Open tx → epoch 1 →
// rollback → open tx → epoch 2 (NOT 1).
func TestBeginOverlayTx_RollbackPreservesEpoch(t *testing.T) {
	s, ctx, _ := openStoreForOverlayTest(t)

	tx1, err := s.BeginOverlayTx(ctx, "ws1")
	if err != nil {
		t.Fatalf("BeginOverlayTx #1: %v", err)
	}
	if tx1.Epoch() != 1 {
		t.Errorf("tx1 epoch: got %d, want 1", tx1.Epoch())
	}
	if err := tx1.Rollback(); err != nil {
		t.Fatalf("tx1.Rollback: %v", err)
	}

	// After rollback, current_epoch is STILL 1 (the bump committed
	// independently of the user-visible tx).
	var ce uint64
	if err := s.db.QueryRow(`SELECT current_epoch FROM semantic_live_overlay_meta
		WHERE repo_id = 'ws1'`).Scan(&ce); err != nil {
		t.Fatalf("read current_epoch after rollback: %v", err)
	}
	if ce != 1 {
		t.Errorf("after tx1.Rollback: current_epoch=%d, want 1 (D-04: rollback does NOT rewind)", ce)
	}

	tx2, err := s.BeginOverlayTx(ctx, "ws1")
	if err != nil {
		t.Fatalf("BeginOverlayTx #2: %v", err)
	}
	if tx2.Epoch() != 2 {
		t.Errorf("tx2 epoch after tx1 rollback: got %d, want 2 (rolled-back epoch is unused, NOT recycled)", tx2.Epoch())
	}
	if err := tx2.Commit(); err != nil {
		t.Fatalf("tx2.Commit: %v", err)
	}
}

// TestBeginOverlayTx_CrossWorkspaceParallelism covers Behavior 6 (per-
// workspace mutex isolation). Two goroutines, each opens a tx for a
// distinct workspace and holds it for ~hold time before commit. Total wall
// time should be approximately ONE hold period (parallel), not TWO
// (serial). Each workspace gets epoch=1.
func TestBeginOverlayTx_CrossWorkspaceParallelism(t *testing.T) {
	s, ctx, _ := openStoreForOverlayTest(t)

	hold := 100 * time.Millisecond

	type result struct {
		ws    string
		epoch uint64
		err   error
	}
	out := make(chan result, 2)

	start := time.Now()
	go func() {
		tx, err := s.BeginOverlayTx(ctx, "ws-a")
		if err != nil {
			out <- result{"ws-a", 0, err}
			return
		}
		time.Sleep(hold)
		if cerr := tx.Commit(); cerr != nil {
			out <- result{"ws-a", tx.Epoch(), cerr}
			return
		}
		out <- result{"ws-a", tx.Epoch(), nil}
	}()
	go func() {
		tx, err := s.BeginOverlayTx(ctx, "ws-b")
		if err != nil {
			out <- result{"ws-b", 0, err}
			return
		}
		time.Sleep(hold)
		if cerr := tx.Commit(); cerr != nil {
			out <- result{"ws-b", tx.Epoch(), cerr}
			return
		}
		out <- result{"ws-b", tx.Epoch(), nil}
	}()

	results := make(map[string]uint64, 2)
	for i := 0; i < 2; i++ {
		r := <-out
		if r.err != nil {
			t.Fatalf("goroutine %s: %v", r.ws, r.err)
		}
		results[r.ws] = r.epoch
	}
	elapsed := time.Since(start)

	// Each workspace independently allocated epoch=1.
	if results["ws-a"] != 1 {
		t.Errorf("ws-a epoch: got %d, want 1 (independent counter)", results["ws-a"])
	}
	if results["ws-b"] != 1 {
		t.Errorf("ws-b epoch: got %d, want 1 (independent counter)", results["ws-b"])
	}

	// Wall time must be closer to one hold period than two. We allow a
	// generous slack (1.7x) for goroutine scheduling jitter on loaded CI;
	// the bug we're catching (cross-workspace lock contention) would push
	// elapsed time to ≥ 2x hold, which is well outside this margin.
	maxAllowed := time.Duration(float64(hold) * 1.7)
	if elapsed > maxAllowed {
		t.Errorf("cross-workspace BeginOverlayTx serialized: elapsed=%s > maxAllowed=%s (hold=%s); per-workspace mutex is leaking across repoIDs", elapsed, maxAllowed, hold)
	}
}

// TestBeginOverlayTx_SameWorkspaceSerializes is the inverse: two goroutines
// against the SAME workspace MUST serialize (so each sees a unique epoch).
// Wall time should be at least 2x hold; epochs must be 1 and 2 in some
// order.
func TestBeginOverlayTx_SameWorkspaceSerializes(t *testing.T) {
	s, ctx, _ := openStoreForOverlayTest(t)

	hold := 60 * time.Millisecond

	type result struct {
		epoch uint64
		err   error
	}
	out := make(chan result, 2)

	start := time.Now()
	for i := 0; i < 2; i++ {
		go func() {
			tx, err := s.BeginOverlayTx(ctx, "ws-shared")
			if err != nil {
				out <- result{0, err}
				return
			}
			time.Sleep(hold)
			if cerr := tx.Commit(); cerr != nil {
				out <- result{tx.Epoch(), cerr}
				return
			}
			out <- result{tx.Epoch(), nil}
		}()
	}

	got := map[uint64]bool{}
	for i := 0; i < 2; i++ {
		r := <-out
		if r.err != nil {
			t.Fatalf("goroutine: %v", r.err)
		}
		if got[r.epoch] {
			t.Fatalf("duplicate epoch %d (same-workspace mutex broken)", r.epoch)
		}
		got[r.epoch] = true
	}
	elapsed := time.Since(start)

	if !got[1] || !got[2] {
		t.Errorf("expected epochs {1, 2}; got %v", got)
	}
	// Same-workspace tx must serialize → elapsed ≥ ~2x hold.
	minAllowed := time.Duration(float64(hold) * 1.5)
	if elapsed < minAllowed {
		t.Errorf("same-workspace BeginOverlayTx ran in parallel: elapsed=%s < minAllowed=%s (hold=%s); the per-workspace mutex is not serializing", elapsed, minAllowed, hold)
	}
}

// TestOverlayTx_UpsertOverlayFile_StampsEpoch covers the round-trip from
// UpsertOverlayFile → committed row carrying the right write_epoch.
func TestOverlayTx_UpsertOverlayFile_StampsEpoch(t *testing.T) {
	s, ctx, _ := openStoreForOverlayTest(t)

	tx, err := s.BeginOverlayTx(ctx, "ws1")
	if err != nil {
		t.Fatalf("BeginOverlayTx: %v", err)
	}
	if err := tx.UpsertOverlayFile(ctx, "src/foo.go", "hash-1"); err != nil {
		t.Fatalf("UpsertOverlayFile: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	var (
		hash   string
		status string
		epoch  uint64
	)
	if err := s.db.QueryRow(`SELECT content_hash, status, write_epoch
		FROM semantic_live_overlay_files
		WHERE repo_id = 'ws1' AND path = 'src/foo.go'`).Scan(&hash, &status, &epoch); err != nil {
		t.Fatalf("read overlay file row: %v", err)
	}
	if hash != "hash-1" {
		t.Errorf("content_hash: got %q, want %q", hash, "hash-1")
	}
	if status != "live" {
		t.Errorf("status: got %q, want %q", status, "live")
	}
	if epoch != 1 {
		t.Errorf("write_epoch: got %d, want 1", epoch)
	}
}

// TestOverlayTx_MarkFileDeleted asserts the tombstone path: UpsertOverlayFile
// then MarkFileDeleted on the same path → row's status='deleted' and
// write_epoch matches the second tx.
func TestOverlayTx_MarkFileDeleted(t *testing.T) {
	s, ctx, _ := openStoreForOverlayTest(t)

	// First tx: live row, epoch=1.
	tx1, err := s.BeginOverlayTx(ctx, "ws1")
	if err != nil {
		t.Fatalf("BeginOverlayTx #1: %v", err)
	}
	if err := tx1.UpsertOverlayFile(ctx, "src/del.go", "hash-1"); err != nil {
		t.Fatalf("UpsertOverlayFile: %v", err)
	}
	if err := tx1.Commit(); err != nil {
		t.Fatalf("Commit #1: %v", err)
	}

	// Second tx: tombstone the same path, epoch=2.
	tx2, err := s.BeginOverlayTx(ctx, "ws1")
	if err != nil {
		t.Fatalf("BeginOverlayTx #2: %v", err)
	}
	if err := tx2.MarkFileDeleted(ctx, "src/del.go"); err != nil {
		t.Fatalf("MarkFileDeleted: %v", err)
	}
	if err := tx2.Commit(); err != nil {
		t.Fatalf("Commit #2: %v", err)
	}

	var (
		status string
		epoch  uint64
	)
	if err := s.db.QueryRow(`SELECT status, write_epoch
		FROM semantic_live_overlay_files
		WHERE repo_id = 'ws1' AND path = 'src/del.go'`).Scan(&status, &epoch); err != nil {
		t.Fatalf("read overlay file row: %v", err)
	}
	if status != "deleted" {
		t.Errorf("status: got %q, want %q", status, "deleted")
	}
	if epoch != 2 {
		t.Errorf("write_epoch on tombstone: got %d, want 2", epoch)
	}
}

// TestOverlayTx_MarkSymbolsDeleted_EmptyList is a smoke test: empty fileIDs
// is a no-op (no error, no SQL issued).
func TestOverlayTx_MarkSymbolsDeleted_EmptyList(t *testing.T) {
	s, ctx, _ := openStoreForOverlayTest(t)

	tx, err := s.BeginOverlayTx(ctx, "ws1")
	if err != nil {
		t.Fatalf("BeginOverlayTx: %v", err)
	}
	if err := tx.MarkSymbolsDeleted(ctx, nil); err != nil {
		t.Errorf("MarkSymbolsDeleted(nil): want nil err, got %v", err)
	}
	if err := tx.MarkSymbolsDeleted(ctx, []uint64{}); err != nil {
		t.Errorf("MarkSymbolsDeleted([]): want nil err, got %v", err)
	}
	if err := tx.MarkReferencesDeleted(ctx, nil); err != nil {
		t.Errorf("MarkReferencesDeleted(nil): want nil err, got %v", err)
	}
	if err := tx.MarkEdgesDeleted(ctx, nil); err != nil {
		t.Errorf("MarkEdgesDeleted(nil): want nil err, got %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
}

// TestFlushOverlay covers Behavior 9: FlushOverlay returns nil and does not
// touch epoch / rows.
func TestFlushOverlay(t *testing.T) {
	s, ctx, _ := openStoreForOverlayTest(t)

	// Seed a row + epoch.
	tx, err := s.BeginOverlayTx(ctx, "ws1")
	if err != nil {
		t.Fatalf("BeginOverlayTx: %v", err)
	}
	if err := tx.UpsertOverlayFile(ctx, "src/x.go", "h"); err != nil {
		t.Fatalf("UpsertOverlayFile: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	if err := s.FlushOverlay(ctx); err != nil {
		t.Errorf("FlushOverlay: want nil, got %v", err)
	}

	// Epoch unchanged after Flush.
	var ce uint64
	if err := s.db.QueryRow(`SELECT current_epoch FROM semantic_live_overlay_meta
		WHERE repo_id = 'ws1'`).Scan(&ce); err != nil {
		t.Fatalf("read current_epoch: %v", err)
	}
	if ce != 1 {
		t.Errorf("FlushOverlay touched current_epoch: got %d, want 1", ce)
	}

	// Row still there after Flush.
	var n int
	if err := s.db.QueryRow(`SELECT count(*) FROM semantic_live_overlay_files
		WHERE repo_id = 'ws1' AND path = 'src/x.go'`).Scan(&n); err != nil {
		t.Fatalf("count overlay rows: %v", err)
	}
	if n != 1 {
		t.Errorf("FlushOverlay deleted rows: count=%d, want 1", n)
	}
}

// TestBeginOverlayTx_RejectsEmptyRepoID asserts the input-validation guard.
func TestBeginOverlayTx_RejectsEmptyRepoID(t *testing.T) {
	s, ctx, _ := openStoreForOverlayTest(t)

	_, err := s.BeginOverlayTx(ctx, "")
	if err == nil {
		t.Fatal("BeginOverlayTx(\"\"): want error, got nil")
	}
}

// --- Phase 62 P02 store extension tests (Task 2) ---

// TestUpsertGraphScores_RoundTrip writes 3 score rows, commits, and asserts
// they land via direct SQL count + status check.
func TestUpsertGraphScores_RoundTrip(t *testing.T) {
	s, ctx, _ := openStoreForOverlayTest(t)
	tx, err := s.BeginOverlayTx(ctx, "ws1")
	if err != nil {
		t.Fatalf("BeginOverlayTx: %v", err)
	}
	rows := []ScoreRow{
		{NodeID: 1, Score: 0.5, GraphVersion: 1, Status: "exact"},
		{NodeID: 2, Score: 0.3, GraphVersion: 1, Status: "stale"},
		{NodeID: 3, Score: 0.2, GraphVersion: 1, Status: "approximate"},
	}
	if err := tx.UpsertGraphScores(ctx, "call_graph", rows); err != nil {
		t.Fatalf("UpsertGraphScores: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM semantic_graph_scores
		WHERE repo_id='ws1' AND score_name='call_graph'`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 3 {
		t.Errorf("score row count: got %d, want 3", n)
	}
	var st string
	if err := s.db.QueryRow(`SELECT status FROM semantic_graph_scores
		WHERE repo_id='ws1' AND node_id=2 AND score_name='call_graph'`).Scan(&st); err != nil {
		t.Fatalf("read status: %v", err)
	}
	if st != "stale" {
		t.Errorf("status[node=2]: got %q, want stale", st)
	}
}

// TestUpsertGraphScores_EmptyNoOp asserts the empty-list shortcut.
func TestUpsertGraphScores_EmptyNoOp(t *testing.T) {
	s, ctx, _ := openStoreForOverlayTest(t)
	tx, err := s.BeginOverlayTx(ctx, "ws1")
	if err != nil {
		t.Fatalf("BeginOverlayTx: %v", err)
	}
	if err := tx.UpsertGraphScores(ctx, "call_graph", nil); err != nil {
		t.Errorf("nil rows: %v", err)
	}
	if err := tx.UpsertGraphScores(ctx, "call_graph", []ScoreRow{}); err != nil {
		t.Errorf("empty rows: %v", err)
	}
	_ = tx.Commit()
}

// TestUpsertGraphScores_DoesNotBumpGraphVersion enforces D-06 — score-only
// commits must NOT advance graph_version.
func TestUpsertGraphScores_DoesNotBumpGraphVersion(t *testing.T) {
	s, ctx, _ := openStoreForOverlayTest(t)

	// First open ensures meta row exists; capture pre-state.
	tx0, err := s.BeginOverlayTx(ctx, "ws1")
	if err != nil {
		t.Fatalf("BeginOverlayTx #1: %v", err)
	}
	if err := tx0.Commit(); err != nil {
		t.Fatalf("Commit #1: %v", err)
	}
	gvBefore, err := s.CurrentGraphVersion(ctx, "ws1")
	if err != nil {
		t.Fatalf("CurrentGraphVersion before: %v", err)
	}

	tx, err := s.BeginOverlayTx(ctx, "ws1")
	if err != nil {
		t.Fatalf("BeginOverlayTx: %v", err)
	}
	if err := tx.UpsertGraphScores(ctx, "call_graph",
		[]ScoreRow{{NodeID: 1, Score: 0.5, GraphVersion: 0, Status: "exact"}}); err != nil {
		t.Fatalf("UpsertGraphScores: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	gvAfter, err := s.CurrentGraphVersion(ctx, "ws1")
	if err != nil {
		t.Fatalf("CurrentGraphVersion after: %v", err)
	}
	if gvAfter != gvBefore {
		t.Errorf("graph_version moved: before=%d after=%d (D-06: only ApplyRepair bumps)", gvBefore, gvAfter)
	}
}

// TestUpsertGraphScores_RejectsInvalidStatus asserts the closed-enum guard.
func TestUpsertGraphScores_RejectsInvalidStatus(t *testing.T) {
	s, ctx, _ := openStoreForOverlayTest(t)
	tx, err := s.BeginOverlayTx(ctx, "ws1")
	if err != nil {
		t.Fatalf("BeginOverlayTx: %v", err)
	}
	defer tx.Rollback()
	cases := []string{"missing", "exploded", ""}
	for _, status := range cases {
		err := tx.UpsertGraphScores(ctx, "call_graph",
			[]ScoreRow{{NodeID: 1, Score: 0.5, GraphVersion: 1, Status: status}})
		if err == nil {
			t.Errorf("status=%q: expected invalid status error", status)
		}
	}
}

// TestBumpGraphVersion_ReturnsNewValue exercises the single-bump helper.
func TestBumpGraphVersion_ReturnsNewValue(t *testing.T) {
	s, ctx, _ := openStoreForOverlayTest(t)
	for i := 1; i <= 3; i++ {
		tx, err := s.BeginOverlayTx(ctx, "ws1")
		if err != nil {
			t.Fatalf("BeginOverlayTx #%d: %v", i, err)
		}
		gv, err := tx.BumpGraphVersion(ctx)
		if err != nil {
			t.Fatalf("BumpGraphVersion #%d: %v", i, err)
		}
		if gv != uint64(i) {
			t.Errorf("BumpGraphVersion #%d: got %d, want %d", i, gv, i)
		}
		if err := tx.Commit(); err != nil {
			t.Fatalf("Commit #%d: %v", i, err)
		}
	}
	gv, err := s.CurrentGraphVersion(ctx, "ws1")
	if err != nil {
		t.Fatalf("CurrentGraphVersion: %v", err)
	}
	if gv != 3 {
		t.Errorf("final graph_version: got %d, want 3", gv)
	}
}

// TestCurrentGraphVersion_ZeroForUnknownRepo asserts the pre-init read path.
func TestCurrentGraphVersion_ZeroForUnknownRepo(t *testing.T) {
	s, ctx, _ := openStoreForOverlayTest(t)
	gv, err := s.CurrentGraphVersion(ctx, "ws-never-opened")
	if err != nil {
		t.Fatalf("CurrentGraphVersion: %v", err)
	}
	if gv != 0 {
		t.Errorf("unknown repo: got gv=%d, want 0", gv)
	}
}

// TestUpsertEdgesWithMerge_LSPSkipsCommentInsert seeds a validated LSP edge
// then writes a comment edge for the same triple — the comment row MUST be
// silently dropped.
func TestUpsertEdgesWithMerge_LSPSkipsCommentInsert(t *testing.T) {
	s, ctx, _ := openStoreForOverlayTest(t)
	tx, err := s.BeginOverlayTx(ctx, "ws1")
	if err != nil {
		t.Fatalf("BeginOverlayTx: %v", err)
	}
	// Seed: validated lsp.references at conf=1.0.
	if err := tx.UpsertEdgesWithMerge(ctx, []EdgeRow{{
		SrcNodeID: 1, DstNodeID: 2, EdgeKind: "RESOLVES_TO",
		Source: "lsp.references", Confidence: 1.0, Weight: 1.0,
		ValidationState: "validated",
	}}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// Now write comment row for the same triple.
	if err := tx.UpsertEdgesWithMerge(ctx, []EdgeRow{{
		SrcNodeID: 1, DstNodeID: 2, EdgeKind: "RESOLVES_TO",
		Source: "comment.tsdoc", Confidence: 0.60, Weight: 1.0,
		ValidationState: "unresolved",
	}}); err != nil {
		t.Fatalf("comment: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	var src, n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM semantic_live_overlay_edges
		WHERE repo_id='ws1' AND src_node_id=1 AND dst_node_id=2 AND edge_kind='RESOLVES_TO'`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Fatalf("row count for triple: got %d, want 1", n)
	}
	var source string
	if err := s.db.QueryRow(`SELECT source FROM semantic_live_overlay_edges
		WHERE repo_id='ws1' AND src_node_id=1 AND dst_node_id=2 AND edge_kind='RESOLVES_TO'`).Scan(&source); err != nil {
		t.Fatalf("read source: %v", err)
	}
	if source != "lsp.references" {
		t.Errorf("source: got %q, want lsp.references (LSP wins)", source)
	}
	_ = src
}

// TestUpsertEdgesWithMerge_LSPDeletesCommentBeforeInsert seeds a comment
// edge then writes a validated LSP edge for the same triple — only the LSP
// row remains.
func TestUpsertEdgesWithMerge_LSPDeletesCommentBeforeInsert(t *testing.T) {
	s, ctx, _ := openStoreForOverlayTest(t)
	tx, err := s.BeginOverlayTx(ctx, "ws1")
	if err != nil {
		t.Fatalf("BeginOverlayTx: %v", err)
	}
	if err := tx.UpsertEdgesWithMerge(ctx, []EdgeRow{{
		SrcNodeID: 1, DstNodeID: 2, EdgeKind: "RESOLVES_TO",
		Source: "comment.tsdoc", Confidence: 0.60, Weight: 1.0,
		ValidationState: "unresolved",
	}}); err != nil {
		t.Fatalf("seed comment: %v", err)
	}
	if err := tx.UpsertEdgesWithMerge(ctx, []EdgeRow{{
		SrcNodeID: 1, DstNodeID: 2, EdgeKind: "RESOLVES_TO",
		Source: "lsp.references", Confidence: 1.0, Weight: 1.0,
		ValidationState: "validated",
	}}); err != nil {
		t.Fatalf("lsp insert: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	var n int
	var source string
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM semantic_live_overlay_edges
		WHERE repo_id='ws1' AND src_node_id=1 AND dst_node_id=2 AND edge_kind='RESOLVES_TO'`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Errorf("row count: got %d, want 1", n)
	}
	if err := s.db.QueryRow(`SELECT source FROM semantic_live_overlay_edges
		WHERE repo_id='ws1' AND src_node_id=1 AND dst_node_id=2 AND edge_kind='RESOLVES_TO'`).Scan(&source); err != nil {
		t.Fatalf("read source: %v", err)
	}
	if source != "lsp.references" {
		t.Errorf("source: got %q, want lsp.references", source)
	}
}

// TestUpsertEdgesWithMerge_LSPRefutesCommentAtDifferentDst is the D-14
// refutation invariant: an LSP edge at (src=1, dst=B) must DELETE a comment
// edge at (src=1, dst=A) for the same edge_kind — comment edges that are
// refuted must NOT survive at lower confidence.
func TestUpsertEdgesWithMerge_LSPRefutesCommentAtDifferentDst(t *testing.T) {
	s, ctx, _ := openStoreForOverlayTest(t)
	tx, err := s.BeginOverlayTx(ctx, "ws1")
	if err != nil {
		t.Fatalf("BeginOverlayTx: %v", err)
	}
	// Comment edge points at dst=10.
	if err := tx.UpsertEdgesWithMerge(ctx, []EdgeRow{{
		SrcNodeID: 1, DstNodeID: 10, EdgeKind: "RESOLVES_TO",
		Source: "comment.tsdoc", Confidence: 0.60, Weight: 1.0,
		ValidationState: "unresolved",
	}}); err != nil {
		t.Fatalf("seed comment dst=10: %v", err)
	}
	// LSP edge for same (src,kind) but DIFFERENT dst=20.
	if err := tx.UpsertEdgesWithMerge(ctx, []EdgeRow{{
		SrcNodeID: 1, DstNodeID: 20, EdgeKind: "RESOLVES_TO",
		Source: "lsp.references", Confidence: 1.0, Weight: 1.0,
		ValidationState: "validated",
	}}); err != nil {
		t.Fatalf("lsp insert dst=20: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	rows, err := s.db.Query(`SELECT dst_node_id, source FROM semantic_live_overlay_edges
		WHERE repo_id='ws1' AND src_node_id=1 AND edge_kind='RESOLVES_TO'
		ORDER BY dst_node_id`)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()
	type row struct {
		dst    uint64
		source string
	}
	var got []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.dst, &r.source); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got = append(got, r)
	}
	if len(got) != 1 {
		t.Fatalf("row count: got %d (%v), want 1 (D-14 refutation)", len(got), got)
	}
	if got[0].dst != 20 || got[0].source != "lsp.references" {
		t.Errorf("surviving row: got (dst=%d, src=%q), want (dst=20, src=lsp.references)", got[0].dst, got[0].source)
	}
}

// --- Phase 62 P04 cluster persistence tests ---

// TestUpsertClusters_RoundTrip writes 3 cluster rows, commits, and asserts
// the row count + algorithm value via direct SQL.
func TestUpsertClusters_RoundTrip(t *testing.T) {
	s, ctx, _ := openStoreForOverlayTest(t)
	tx, err := s.BeginOverlayTx(ctx, "ws-cluster-rt")
	if err != nil {
		t.Fatalf("BeginOverlayTx: %v", err)
	}
	clusters := []ClusterSummary{
		{ID: 1, MemberCount: 3},
		{ID: 10, MemberCount: 2},
		{ID: 20, MemberCount: 1},
	}
	if err := tx.UpsertClusters(ctx, "weak_components", 5, clusters); err != nil {
		t.Fatalf("UpsertClusters: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM semantic_clusters
		WHERE repo_id='ws-cluster-rt' AND graph_version=5`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 3 {
		t.Errorf("cluster row count: got %d, want 3", n)
	}
	var algo string
	if err := s.db.QueryRow(`SELECT algorithm FROM semantic_clusters
		WHERE repo_id='ws-cluster-rt' AND graph_version=5 AND cluster_id=10`).Scan(&algo); err != nil {
		t.Fatalf("read algorithm: %v", err)
	}
	if algo != "weak_components" {
		t.Errorf("algorithm[cluster=10]: got %q, want weak_components", algo)
	}
}

// TestUpsertClusters_EmptyNoOp asserts the empty-list shortcut and that the
// nil-tx / empty-projection guards reject malformed calls.
func TestUpsertClusters_EmptyNoOp(t *testing.T) {
	s, ctx, _ := openStoreForOverlayTest(t)
	tx, err := s.BeginOverlayTx(ctx, "ws-cluster-empty")
	if err != nil {
		t.Fatalf("BeginOverlayTx: %v", err)
	}
	if err := tx.UpsertClusters(ctx, "weak_components", 1, nil); err != nil {
		t.Errorf("nil rows: %v", err)
	}
	if err := tx.UpsertClusters(ctx, "weak_components", 1, []ClusterSummary{}); err != nil {
		t.Errorf("empty rows: %v", err)
	}
	if err := tx.UpsertClusters(ctx, "", 1, []ClusterSummary{{ID: 1, MemberCount: 1}}); err == nil {
		t.Errorf("empty projection: expected error")
	}
	_ = tx.Commit()
}

// TestUpsertClusterMembers_RoundTrip writes member rows for 3 clusters of
// sizes [2, 3, 1] and asserts 6 total rows land.
func TestUpsertClusterMembers_RoundTrip(t *testing.T) {
	s, ctx, _ := openStoreForOverlayTest(t)
	tx, err := s.BeginOverlayTx(ctx, "ws-cluster-members")
	if err != nil {
		t.Fatalf("BeginOverlayTx: %v", err)
	}
	clusters := []ClusterSummary{
		{ID: 1, MemberCount: 2},
		{ID: 10, MemberCount: 3},
		{ID: 20, MemberCount: 1},
	}
	if err := tx.UpsertClusters(ctx, "weak_components", 5, clusters); err != nil {
		t.Fatalf("UpsertClusters: %v", err)
	}
	members := []ClusterMemberRow{
		{ClusterID: 1, NodeID: 1}, {ClusterID: 1, NodeID: 2},
		{ClusterID: 10, NodeID: 10}, {ClusterID: 10, NodeID: 11}, {ClusterID: 10, NodeID: 12},
		{ClusterID: 20, NodeID: 20},
	}
	if err := tx.UpsertClusterMembers(ctx, "weak_components", 5, members); err != nil {
		t.Fatalf("UpsertClusterMembers: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM semantic_cluster_members
		WHERE repo_id='ws-cluster-members' AND graph_version=5`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 6 {
		t.Errorf("member row count: got %d, want 6", n)
	}

	// Empty list short-circuits without error.
	tx2, err := s.BeginOverlayTx(ctx, "ws-cluster-members")
	if err != nil {
		t.Fatalf("BeginOverlayTx #2: %v", err)
	}
	if err := tx2.UpsertClusterMembers(ctx, "weak_components", 5, nil); err != nil {
		t.Errorf("nil rows: %v", err)
	}
	_ = tx2.Commit()
}

// TestDeleteClustersForGraphVersion seeds clusters + members at gv=5,
// deletes them, and asserts both tables are empty for that gv.
func TestDeleteClustersForGraphVersion(t *testing.T) {
	s, ctx, _ := openStoreForOverlayTest(t)

	// Seed.
	tx, err := s.BeginOverlayTx(ctx, "ws-cluster-delete")
	if err != nil {
		t.Fatalf("BeginOverlayTx: %v", err)
	}
	clusters := []ClusterSummary{{ID: 1, MemberCount: 2}, {ID: 10, MemberCount: 1}}
	members := []ClusterMemberRow{
		{ClusterID: 1, NodeID: 1}, {ClusterID: 1, NodeID: 2},
		{ClusterID: 10, NodeID: 10},
	}
	if err := tx.UpsertClusters(ctx, "weak_components", 5, clusters); err != nil {
		t.Fatalf("UpsertClusters: %v", err)
	}
	if err := tx.UpsertClusterMembers(ctx, "weak_components", 5, members); err != nil {
		t.Fatalf("UpsertClusterMembers: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit seed: %v", err)
	}

	// Delete.
	tx2, err := s.BeginOverlayTx(ctx, "ws-cluster-delete")
	if err != nil {
		t.Fatalf("BeginOverlayTx #2: %v", err)
	}
	if err := tx2.DeleteClustersForGraphVersion(ctx, "weak_components", 5); err != nil {
		t.Fatalf("DeleteClustersForGraphVersion: %v", err)
	}
	if err := tx2.Commit(); err != nil {
		t.Fatalf("Commit delete: %v", err)
	}

	var nClusters, nMembers int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM semantic_clusters
		WHERE repo_id='ws-cluster-delete' AND graph_version=5`).Scan(&nClusters); err != nil {
		t.Fatalf("count clusters: %v", err)
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM semantic_cluster_members
		WHERE repo_id='ws-cluster-delete' AND graph_version=5`).Scan(&nMembers); err != nil {
		t.Fatalf("count members: %v", err)
	}
	if nClusters != 0 || nMembers != 0 {
		t.Errorf("after delete: clusters=%d members=%d, want 0/0", nClusters, nMembers)
	}
}

// TestUpsertClusters_PrimaryKeyComposite asserts that two clusters at the
// same graph_version with distinct cluster_ids both persist, and that a
// repeat write of the same (repo, gv, cluster_id) updates rather than
// duplicates.
func TestUpsertClusters_PrimaryKeyComposite(t *testing.T) {
	s, ctx, _ := openStoreForOverlayTest(t)

	// First write: two distinct cluster_ids → two rows.
	tx, err := s.BeginOverlayTx(ctx, "ws-cluster-pk")
	if err != nil {
		t.Fatalf("BeginOverlayTx: %v", err)
	}
	clusters := []ClusterSummary{
		{ID: 1, MemberCount: 5},
		{ID: 2, MemberCount: 7},
	}
	if err := tx.UpsertClusters(ctx, "weak_components", 9, clusters); err != nil {
		t.Fatalf("UpsertClusters: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit #1: %v", err)
	}

	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM semantic_clusters
		WHERE repo_id='ws-cluster-pk' AND graph_version=9`).Scan(&n); err != nil {
		t.Fatalf("count #1: %v", err)
	}
	if n != 2 {
		t.Errorf("after first write: got %d rows, want 2", n)
	}

	// Second write: same (repo, gv, cluster_id=1) with new score → should
	// UPDATE not INSERT, leaving total row count at 2.
	tx2, err := s.BeginOverlayTx(ctx, "ws-cluster-pk")
	if err != nil {
		t.Fatalf("BeginOverlayTx #2: %v", err)
	}
	if err := tx2.UpsertClusters(ctx, "weak_components", 9,
		[]ClusterSummary{{ID: 1, MemberCount: 99}}); err != nil {
		t.Fatalf("UpsertClusters #2: %v", err)
	}
	if err := tx2.Commit(); err != nil {
		t.Fatalf("Commit #2: %v", err)
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM semantic_clusters
		WHERE repo_id='ws-cluster-pk' AND graph_version=9`).Scan(&n); err != nil {
		t.Fatalf("count #2: %v", err)
	}
	if n != 2 {
		t.Errorf("after re-upsert: got %d rows, want 2 (update, not duplicate)", n)
	}

	// Score column reflects the updated MemberCount.
	var score float64
	if err := s.db.QueryRow(`SELECT score FROM semantic_clusters
		WHERE repo_id='ws-cluster-pk' AND graph_version=9 AND cluster_id=1`).Scan(&score); err != nil {
		t.Fatalf("read score: %v", err)
	}
	if score != 99.0 {
		t.Errorf("score[cluster=1] after update: got %g, want 99", score)
	}
}

// --- Helpers ---

// openStoreForOverlayTest opens a fresh store (lands at v3 via the migration
// registry) and returns it together with a background context and a cancel
// hook. The store is registered for cleanup so each test gets an isolated DB.
func openStoreForOverlayTest(t *testing.T) (*Store, context.Context, context.CancelFunc) {
	t.Helper()
	wsDir := t.TempDir()
	cfg := configFor(t, wsDir)
	m := newTestObsMetrics(t)
	s, err := Open(context.Background(), cfg, silentLogger(), m)
	if err != nil {
		t.Fatalf("Open(fresh): %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	return s, ctx, cancel
}

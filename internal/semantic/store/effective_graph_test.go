// Phase 64 P64-02 effective-graph queries: RED test cases.
//
// 14 tests across the FIVE store methods land here:
//
//	QueryEffectiveAdjacency       (Tests 1-4)
//	CountStaleScoreRows           (Tests 5-6)
//	MarkAllScoreRowsStale         (Tests 7-8)
//	LatestCommittedSnapshot       (Tests 9-10)
//	IterateCommittedSymbols       (Tests 11-14)
//
// Tests are white-box (package store) so the helpers can read internal
// schema columns and seed via direct INSERTs into the underlying *sql.DB.
//
// Schema reality consumed by these tests (per migrations.go):
//
//   - semantic_edges: keyed by (snapshot_id, edge_id). NO repo_id; the test
//     filters via the snapshot_id of the latest committed snapshot for
//     repoID.
//   - semantic_live_overlay_edges: keyed by (repo_id, edge_id). Carries
//     status TEXT (closed enum 'live' | 'deleted' — overlay's MarkEdges
//     Deleted flips 'live' → 'deleted' as the tombstone gesture).
//   - semantic_graph_scores: keyed by (repo_id, graph_version, node_id,
//     score_name). The interface's `projection` arg maps to score_name.
//   - semantic_symbols: keyed by (snapshot_id, symbol_id). No path or
//     docstring columns — path is JOINed from semantic_files; docstring is
//     emitted as "" until a future migration materializes it.

package store

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agenthands/helix/internal/semantic/graph"
)

// seedCommittedSnapshot inserts a minimal committed snapshot row directly
// (bypassing BeginSnapshot) so tests can stamp arbitrary snapshot_ids and
// drive the LatestCommittedSnapshot edge-case suite without coupling to
// the BeginSnapshot SEQUENCE allocator.
func seedCommittedSnapshot(t *testing.T, ctx context.Context, s *Store, repoID string, snapID uint64, status string) {
	t.Helper()
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO semantic_snapshots (
			snapshot_id, repo_id, repo_root, base_snapshot_id, kind,
			worktree_hash, schema_version, indexer_version, status,
			partial, created_at, committed_at
		) VALUES (?, ?, '', 0, 'compact', '', 5, 'test', ?, false, now(), now())
	`, snapID, repoID, status); err != nil {
		t.Fatalf("seedCommittedSnapshot(%q, snap=%d, status=%q): %v", repoID, snapID, status, err)
	}
}

// seedSnapshotEdge inserts directly into semantic_edges keyed by snapshot_id.
func seedSnapshotEdge(t *testing.T, ctx context.Context, s *Store, snapID, edgeID, src, dst uint64, edgeKind string, weight float64) {
	t.Helper()
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO semantic_edges (
			snapshot_id, edge_id, src_node_id, dst_node_id, src_kind,
			dst_kind, edge_kind, weight, confidence, validation_state,
			source, created_at
		) VALUES (?, ?, ?, ?, 'symbol', 'symbol', ?, ?, 1.0, 'validated', 'lsp.references', now())
	`, snapID, edgeID, src, dst, edgeKind, weight); err != nil {
		t.Fatalf("seedSnapshotEdge(snap=%d, edge=%d, %d→%d %s): %v", snapID, edgeID, src, dst, edgeKind, err)
	}
}

// seedOverlayEdge inserts directly into semantic_live_overlay_edges with
// the requested status ('live' or 'deleted').
func seedOverlayEdge(t *testing.T, ctx context.Context, s *Store, repoID string, edgeID, src, dst uint64, edgeKind, status string, weight float64) {
	t.Helper()
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO semantic_live_overlay_edges (
			repo_id, edge_id, src_node_id, dst_node_id, edge_kind,
			status, validation_state, confidence, weight, source,
			fact_json, updated_at, write_epoch
		) VALUES (?, ?, ?, ?, ?, ?, 'validated', 1.0, ?, 'lsp.references', NULL, now(), 1)
	`, repoID, edgeID, src, dst, edgeKind, status, weight); err != nil {
		t.Fatalf("seedOverlayEdge(%q, edge=%d, %d→%d %s status=%q): %v", repoID, edgeID, src, dst, edgeKind, status, err)
	}
}

// seedScoreRow inserts a row into semantic_graph_scores.
func seedScoreRow(t *testing.T, ctx context.Context, s *Store, repoID string, gv, nodeID uint64, scoreName, status string) {
	t.Helper()
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO semantic_graph_scores (
			repo_id, snapshot_id, graph_version, node_id, score_name,
			score, rank, status, computed_at, algorithm_version
		) VALUES (?, 0, ?, ?, ?, 0.5, NULL, ?, now(), 'test')
	`, repoID, gv, nodeID, scoreName, status); err != nil {
		t.Fatalf("seedScoreRow(%q, gv=%d, node=%d, name=%q, status=%q): %v",
			repoID, gv, nodeID, scoreName, status, err)
	}
}

// seedCommittedSymbol inserts a snapshot symbol row plus its file row so
// the JOIN in IterateCommittedSymbols resolves the path.
func seedCommittedSymbol(t *testing.T, ctx context.Context, s *Store, snapID, symbolID, fileID uint64, name, path string, lineStart int) {
	t.Helper()
	// Insert the file row first (idempotent across multiple symbols sharing
	// the same file_id).
	if _, err := s.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO semantic_files (
			snapshot_id, file_id, repo_id, path, language, content_hash,
			size_bytes, line_count, generated, ignored, indexed_at
		) VALUES (?, ?, 'r-it', ?, 'go', 'h', 100, 10, false, false, now())
	`, snapID, fileID, path); err != nil {
		t.Fatalf("seedCommittedSymbol: insert file (snap=%d, file=%d, path=%q): %v",
			snapID, fileID, path, err)
	}
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO semantic_symbols (
			snapshot_id, symbol_id, node_id, file_id, language, kind, name,
			qualified_name, stable_key, start_byte, end_byte, start_line,
			start_col, end_line, end_col, extraction_source, confidence
		) VALUES (?, ?, ?, ?, 'go', 'func', ?, ?, ?, 0, 100, ?, 0, ?, 0, 'tree-sitter', 1.0)
	`, snapID, symbolID, symbolID, fileID, name, name, name, lineStart, lineStart+5); err != nil {
		t.Fatalf("seedCommittedSymbol(snap=%d, sym=%d, name=%q): %v", snapID, symbolID, name, err)
	}
}

// --- Tests 1-4: QueryEffectiveAdjacency ---

func TestQueryEffectiveAdjacency_SnapshotOnly(t *testing.T) {
	s, ctx, _ := openStoreForOverlayTest(t)
	repoID := "r-adj-snap"
	const snap uint64 = 1001
	seedCommittedSnapshot(t, ctx, s, repoID, snap, "committed")
	// Snapshot edges A→B(0.5), B→C(0.7) under edge_kind="call_graph"
	seedSnapshotEdge(t, ctx, s, snap, 1, 1 /*A*/, 2 /*B*/, "call_graph", 0.5)
	seedSnapshotEdge(t, ctx, s, snap, 2, 2 /*B*/, 3 /*C*/, "call_graph", 0.7)

	out, in, err := s.QueryEffectiveAdjacency(ctx, repoID, "call_graph")
	if err != nil {
		t.Fatalf("QueryEffectiveAdjacency: %v", err)
	}
	if got := out[1][2]; got != 0.5 {
		t.Errorf("out[A][B] = %v, want 0.5", got)
	}
	if got := out[2][3]; got != 0.7 {
		t.Errorf("out[B][C] = %v, want 0.7", got)
	}
	if got := in[2][1]; got != 0.5 {
		t.Errorf("in[B][A] = %v, want 0.5", got)
	}
	if got := in[3][2]; got != 0.7 {
		t.Errorf("in[C][B] = %v, want 0.7", got)
	}
}

func TestQueryEffectiveAdjacency_OverlayAddsEdge(t *testing.T) {
	s, ctx, _ := openStoreForOverlayTest(t)
	repoID := "r-adj-add"
	const snap uint64 = 1002
	seedCommittedSnapshot(t, ctx, s, repoID, snap, "committed")
	seedSnapshotEdge(t, ctx, s, snap, 1, 1, 2, "call_graph", 0.5)
	seedOverlayEdge(t, ctx, s, repoID, 99, 2, 3, "call_graph", "live", 0.3)

	out, _, err := s.QueryEffectiveAdjacency(ctx, repoID, "call_graph")
	if err != nil {
		t.Fatalf("QueryEffectiveAdjacency: %v", err)
	}
	if got := out[1][2]; got != 0.5 {
		t.Errorf("out[A][B] (snapshot) = %v, want 0.5", got)
	}
	if got := out[2][3]; got != 0.3 {
		t.Errorf("out[B][C] (overlay) = %v, want 0.3", got)
	}
}

func TestQueryEffectiveAdjacency_OverlayTombstoneRemovesEdge(t *testing.T) {
	s, ctx, _ := openStoreForOverlayTest(t)
	repoID := "r-adj-tomb"
	const snap uint64 = 1003
	seedCommittedSnapshot(t, ctx, s, repoID, snap, "committed")
	seedSnapshotEdge(t, ctx, s, snap, 1, 1, 2, "call_graph", 0.5)
	// Overlay row B→C with status='deleted' (the tombstone gesture).
	seedOverlayEdge(t, ctx, s, repoID, 99, 2, 3, "call_graph", "deleted", 0.3)

	out, _, err := s.QueryEffectiveAdjacency(ctx, repoID, "call_graph")
	if err != nil {
		t.Fatalf("QueryEffectiveAdjacency: %v", err)
	}
	if got := out[1][2]; got != 0.5 {
		t.Errorf("out[A][B] = %v, want 0.5", got)
	}
	if _, present := out[2][3]; present {
		t.Errorf("out[B][C] present despite tombstone (status='deleted')")
	}
}

func TestQueryEffectiveAdjacency_ProjectionFilter(t *testing.T) {
	s, ctx, _ := openStoreForOverlayTest(t)
	repoID := "r-adj-proj"
	const snap uint64 = 1004
	seedCommittedSnapshot(t, ctx, s, repoID, snap, "committed")
	seedSnapshotEdge(t, ctx, s, snap, 1, 1, 2, "call_graph", 0.5)
	seedSnapshotEdge(t, ctx, s, snap, 2, 1, 2, "reference_graph", 0.7)

	out, _, err := s.QueryEffectiveAdjacency(ctx, repoID, "call_graph")
	if err != nil {
		t.Fatalf("QueryEffectiveAdjacency: %v", err)
	}
	if got := out[1][2]; got != 0.5 {
		t.Errorf("out[A][B] (call_graph) = %v, want 0.5", got)
	}
	// Projection filter must drop the reference_graph edge — under the
	// (1→2) key the only weight returned is the call_graph one.
	if len(out[1]) != 1 {
		t.Errorf("out[A] = %v, want exactly one outgoing edge after projection filter", out[1])
	}
}

// --- Tests 5-6: CountStaleScoreRows ---

func TestCountStaleScoreRows_StaleAndTotal(t *testing.T) {
	s, ctx, _ := openStoreForOverlayTest(t)
	repoID := "r-cnt"
	for i := 0; i < 5; i++ {
		seedScoreRow(t, ctx, s, repoID, 1, uint64(i+1), "call_graph", "exact")
	}
	for i := 0; i < 3; i++ {
		seedScoreRow(t, ctx, s, repoID, 1, uint64(i+100), "call_graph", "stale")
	}

	stale, total, err := s.CountStaleScoreRows(ctx, repoID, "call_graph")
	if err != nil {
		t.Fatalf("CountStaleScoreRows: %v", err)
	}
	if stale != 3 {
		t.Errorf("stale = %d, want 3", stale)
	}
	if total != 8 {
		t.Errorf("total = %d, want 8", total)
	}
}

func TestCountStaleScoreRows_LockFree_RaceSafe(t *testing.T) {
	if testing.Short() {
		t.Skip("race-safety stress; skipped under -short")
	}
	s, ctx, _ := openStoreForOverlayTest(t)
	repoID := "r-race"
	for i := 0; i < 10; i++ {
		seedScoreRow(t, ctx, s, repoID, 1, uint64(i+1), "call_graph", "exact")
	}

	// Background overlay tx writer to create lock contention.
	var stop atomic.Bool
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for !stop.Load() {
			tx, err := s.BeginOverlayTx(ctx, repoID)
			if err != nil {
				return
			}
			_ = tx.Commit()
		}
	}()

	// 100 concurrent reader probes — must NOT deadlock against the
	// overlay-tx writer since CountStaleScoreRows reads with no overlay
	// lock per the SchedulerStore contract.
	var readers sync.WaitGroup
	for i := 0; i < 100; i++ {
		readers.Add(1)
		go func() {
			defer readers.Done()
			done := make(chan error, 1)
			go func() {
				_, _, e := s.CountStaleScoreRows(ctx, repoID, "call_graph")
				done <- e
			}()
			select {
			case e := <-done:
				if e != nil {
					t.Errorf("CountStaleScoreRows under contention: %v", e)
				}
			case <-time.After(5 * time.Second):
				t.Errorf("CountStaleScoreRows deadlocked (>5s under contention)")
			}
		}()
	}
	readers.Wait()
	stop.Store(true)
	wg.Wait()
}

// --- Tests 7-8: MarkAllScoreRowsStale ---

func TestMarkAllScoreRowsStale_FlipsAll(t *testing.T) {
	s, ctx, _ := openStoreForOverlayTest(t)
	repoID := "r-mark"
	for i := 0; i < 10; i++ {
		seedScoreRow(t, ctx, s, repoID, 1, uint64(i+1), "call_graph", "exact")
	}

	if err := s.MarkAllScoreRowsStale(ctx, repoID, "call_graph"); err != nil {
		t.Fatalf("MarkAllScoreRowsStale: %v", err)
	}
	stale, total, err := s.CountStaleScoreRows(ctx, repoID, "call_graph")
	if err != nil {
		t.Fatalf("CountStaleScoreRows post-mark: %v", err)
	}
	if stale != 10 || total != 10 {
		t.Errorf("post-mark counts: stale=%d total=%d, want 10/10", stale, total)
	}

	// Idempotent re-call: still 10/10, no error.
	if err := s.MarkAllScoreRowsStale(ctx, repoID, "call_graph"); err != nil {
		t.Fatalf("MarkAllScoreRowsStale (idempotent): %v", err)
	}
	stale2, total2, err := s.CountStaleScoreRows(ctx, repoID, "call_graph")
	if err != nil {
		t.Fatalf("CountStaleScoreRows post-mark-idempotent: %v", err)
	}
	if stale2 != 10 || total2 != 10 {
		t.Errorf("post-mark-idempotent counts: stale=%d total=%d, want 10/10", stale2, total2)
	}
}

func TestMarkAllScoreRowsStale_PerProjection(t *testing.T) {
	s, ctx, _ := openStoreForOverlayTest(t)
	repoID := "r-mark-proj"
	for i := 0; i < 5; i++ {
		seedScoreRow(t, ctx, s, repoID, 1, uint64(i+1), "call_graph", "exact")
		seedScoreRow(t, ctx, s, repoID, 1, uint64(i+1), "reference_graph", "exact")
	}

	if err := s.MarkAllScoreRowsStale(ctx, repoID, "call_graph"); err != nil {
		t.Fatalf("MarkAllScoreRowsStale: %v", err)
	}
	staleA, totalA, _ := s.CountStaleScoreRows(ctx, repoID, "call_graph")
	if staleA != 5 || totalA != 5 {
		t.Errorf("call_graph counts: stale=%d total=%d, want 5/5", staleA, totalA)
	}
	staleB, totalB, _ := s.CountStaleScoreRows(ctx, repoID, "reference_graph")
	if staleB != 0 || totalB != 5 {
		t.Errorf("reference_graph (untouched) counts: stale=%d total=%d, want 0/5", staleB, totalB)
	}
}

// --- Tests 9-10: LatestCommittedSnapshot ---

func TestLatestCommittedSnapshot_NoneReturnsZero(t *testing.T) {
	s, ctx, _ := openStoreForOverlayTest(t)
	id, err := s.LatestCommittedSnapshot(ctx, "r-empty")
	if err != nil {
		t.Fatalf("LatestCommittedSnapshot empty: %v", err)
	}
	if id != 0 {
		t.Errorf("LatestCommittedSnapshot empty: got %d, want 0", id)
	}
}

func TestLatestCommittedSnapshot_ReturnsMaxCommitted(t *testing.T) {
	s, ctx, _ := openStoreForOverlayTest(t)
	repoID := "r-latest"
	seedCommittedSnapshot(t, ctx, s, repoID, 1, "committed")
	seedCommittedSnapshot(t, ctx, s, repoID, 2, "committed")
	seedCommittedSnapshot(t, ctx, s, repoID, 3, "committed")
	// In-progress build that must be ignored (status != 'committed').
	seedCommittedSnapshot(t, ctx, s, repoID, 4, "pending")

	id, err := s.LatestCommittedSnapshot(ctx, repoID)
	if err != nil {
		t.Fatalf("LatestCommittedSnapshot: %v", err)
	}
	if id != 3 {
		t.Errorf("LatestCommittedSnapshot: got %d, want 3", id)
	}
}

// --- Tests 11-14: IterateCommittedSymbols ---

func TestIterateCommittedSymbols_StableSortBySymbolID(t *testing.T) {
	s, ctx, _ := openStoreForOverlayTest(t)
	const snap uint64 = 5001
	seedCommittedSnapshot(t, ctx, s, "r-it", snap, "committed")
	// Insert IDs in non-sorted order: 30, 10, 50, 20, 40 with names c,a,e,b,d.
	seedCommittedSymbol(t, ctx, s, snap, 30, 1, "c", "c.go", 1)
	seedCommittedSymbol(t, ctx, s, snap, 10, 1, "a", "c.go", 1)
	seedCommittedSymbol(t, ctx, s, snap, 50, 1, "e", "c.go", 1)
	seedCommittedSymbol(t, ctx, s, snap, 20, 1, "b", "c.go", 1)
	seedCommittedSymbol(t, ctx, s, snap, 40, 1, "d", "c.go", 1)

	var got []string
	if err := s.IterateCommittedSymbols(ctx, snap, func(r SymbolRow) bool {
		got = append(got, r.Name)
		return true
	}); err != nil {
		t.Fatalf("IterateCommittedSymbols: %v", err)
	}
	want := []string{"a", "b", "c", "d", "e"}
	if len(got) != len(want) {
		t.Fatalf("name count: got %d (%v), want %d (%v)", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("name[%d] = %q, want %q (full got=%v)", i, got[i], want[i], got)
		}
	}
}

func TestIterateCommittedSymbols_AbortsOnFalse(t *testing.T) {
	s, ctx, _ := openStoreForOverlayTest(t)
	const snap uint64 = 5002
	seedCommittedSnapshot(t, ctx, s, "r-it", snap, "committed")
	for i := uint64(1); i <= 10; i++ {
		seedCommittedSymbol(t, ctx, s, snap, i, 1, "s", "f.go", 1)
	}

	var calls int
	err := s.IterateCommittedSymbols(ctx, snap, func(r SymbolRow) bool {
		calls++
		return calls < 3 // returns false on the 3rd call.
	})
	if err != nil {
		t.Fatalf("IterateCommittedSymbols: %v", err)
	}
	if calls != 3 {
		t.Errorf("calls = %d, want 3 (abort-on-false honored)", calls)
	}
}

func TestIterateCommittedSymbols_SkipsNonMatchingSnapshot(t *testing.T) {
	s, ctx, _ := openStoreForOverlayTest(t)
	const snap1 uint64 = 5003
	const snap2 uint64 = 5004
	seedCommittedSnapshot(t, ctx, s, "r-it", snap1, "committed")
	seedCommittedSymbol(t, ctx, s, snap1, 1, 1, "x", "x.go", 1)

	var calls int
	err := s.IterateCommittedSymbols(ctx, snap2, func(r SymbolRow) bool {
		calls++
		return true
	})
	if err != nil {
		t.Fatalf("IterateCommittedSymbols (other snap): %v", err)
	}
	if calls != 0 {
		t.Errorf("calls = %d, want 0 (snap2 has no symbols)", calls)
	}
}

func TestIterateCommittedSymbols_PopulatesAllFields(t *testing.T) {
	s, ctx, _ := openStoreForOverlayTest(t)
	const snap uint64 = 5005
	seedCommittedSnapshot(t, ctx, s, "r-it", snap, "committed")
	seedCommittedSymbol(t, ctx, s, snap, 42, 7, "MyFunc", "pkg/foo.go", 123)

	var rows []SymbolRow
	if err := s.IterateCommittedSymbols(ctx, snap, func(r SymbolRow) bool {
		rows = append(rows, r)
		return true
	}); err != nil {
		t.Fatalf("IterateCommittedSymbols: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	r := rows[0]
	if r.SymbolID != "42" {
		t.Errorf("SymbolID = %q, want %q", r.SymbolID, "42")
	}
	if r.Name != "MyFunc" {
		t.Errorf("Name = %q, want %q", r.Name, "MyFunc")
	}
	if r.Path != "pkg/foo.go" {
		t.Errorf("Path = %q, want %q", r.Path, "pkg/foo.go")
	}
	if r.FileID != "7" {
		t.Errorf("FileID = %q, want %q", r.FileID, "7")
	}
	if r.LineStart != 123 {
		t.Errorf("LineStart = %d, want 123", r.LineStart)
	}
	// Docstring is intentionally empty until a future migration materializes
	// it; assert the contract explicitly.
	if r.Docstring != "" {
		t.Errorf("Docstring = %q, want empty (Schema 5 has no docstring column)", r.Docstring)
	}
}

// Compile-time guard: ensure SymbolRow remains exported through the package
// surface so P64-07 retrieval/corpus.go can take a stable dependency.
var _ SymbolRow = SymbolRow{}

// Compile-time guard: ensure graph.NodeID is the alias type the adjacency
// maps round-trip through. If this drifts, the adapters in
// internal/daemon/rank_wiring.go would silently lose type identity.
var _ graph.NodeID = uint64(0)

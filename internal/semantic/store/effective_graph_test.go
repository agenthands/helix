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
	"fmt"
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

// --- Phase 65 65-10 Task 1: QueryRankedFiles ---

// seedSnapshotFileAndSymbol inserts a (semantic_files, semantic_symbols) pair
// keyed at snapshotID. Score rows JOIN to these to resolve per-file paths in
// QueryRankedFiles.
func seedSnapshotFileAndSymbol(
	t *testing.T,
	ctx context.Context,
	s *Store,
	snapID, fileID, symbolID uint64,
	repoID, path, name string,
) {
	t.Helper()
	if _, err := s.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO semantic_files (
			snapshot_id, file_id, repo_id, path, language, content_hash,
			size_bytes, line_count, generated, ignored, indexed_at
		) VALUES (?, ?, ?, ?, 'go', 'h', 100, 10, false, false, now())
	`, snapID, fileID, repoID, path); err != nil {
		t.Fatalf("seedSnapshotFileAndSymbol: file (snap=%d, file=%d): %v", snapID, fileID, err)
	}
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO semantic_symbols (
			snapshot_id, symbol_id, node_id, file_id, language, kind, name,
			qualified_name, stable_key, start_byte, end_byte, start_line,
			start_col, end_line, end_col, extraction_source, confidence
		) VALUES (?, ?, ?, ?, 'go', 'func', ?, ?, ?, 0, 100, 1, 0, 5, 0, 'tree-sitter', 1.0)
	`, snapID, symbolID, symbolID, fileID, name, name, name); err != nil {
		t.Fatalf("seedSnapshotFileAndSymbol: symbol (snap=%d, sym=%d): %v", snapID, symbolID, err)
	}
}

// seedScoreRowWithValue inserts a semantic_graph_scores row with an explicit
// score value (the existing seedScoreRow helper hardcodes 0.5).
func seedScoreRowWithValue(
	t *testing.T,
	ctx context.Context,
	s *Store,
	repoID string,
	gv, nodeID uint64,
	scoreName, status string,
	score float64,
) {
	t.Helper()
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO semantic_graph_scores (
			repo_id, snapshot_id, graph_version, node_id, score_name,
			score, rank, status, computed_at, algorithm_version
		) VALUES (?, 0, ?, ?, ?, ?, NULL, ?, now(), 'test')
	`, repoID, gv, nodeID, scoreName, score, status); err != nil {
		t.Fatalf("seedScoreRowWithValue(%q, gv=%d, node=%d, name=%q, status=%q, score=%g): %v",
			repoID, gv, nodeID, scoreName, status, score, err)
	}
}

// Test 1: empty store → (nil, nil)
func TestStore_QueryRankedFiles_EmptyStore(t *testing.T) {
	s, ctx, _ := openStoreForOverlayTest(t)
	got, err := s.QueryRankedFiles(ctx, "r-empty", "call_graph", 0)
	if err != nil {
		t.Fatalf("QueryRankedFiles: %v", err)
	}
	if got != nil {
		t.Errorf("got %v, want nil for empty store", got)
	}
}

// Test 2: 5 score rows over 3 distinct files → per-file MAX score, sorted DESC.
func TestStore_QueryRankedFiles_PopulatedSnapshot(t *testing.T) {
	s, ctx, _ := openStoreForOverlayTest(t)
	repoID := "r-pop"
	const snap uint64 = 7001
	seedCommittedSnapshot(t, ctx, s, repoID, snap, "committed")

	// Three files; symbol IDs allocated dense.
	seedSnapshotFileAndSymbol(t, ctx, s, snap, 1, 101, repoID, "src/a.go", "A1")
	seedSnapshotFileAndSymbol(t, ctx, s, snap, 1, 102, repoID, "src/a.go", "A2")
	seedSnapshotFileAndSymbol(t, ctx, s, snap, 2, 201, repoID, "src/b.go", "B1")
	seedSnapshotFileAndSymbol(t, ctx, s, snap, 3, 301, repoID, "src/c.go", "C1")
	seedSnapshotFileAndSymbol(t, ctx, s, snap, 3, 302, repoID, "src/c.go", "C2")

	// Score rows: a.go has 0.3 + 0.7 (max=0.7); b.go has 0.5; c.go has 0.9 + 0.1 (max=0.9).
	const gv uint64 = 1
	seedScoreRowWithValue(t, ctx, s, repoID, gv, 101, "call_graph", "exact", 0.3)
	seedScoreRowWithValue(t, ctx, s, repoID, gv, 102, "call_graph", "exact", 0.7)
	seedScoreRowWithValue(t, ctx, s, repoID, gv, 201, "call_graph", "exact", 0.5)
	seedScoreRowWithValue(t, ctx, s, repoID, gv, 301, "call_graph", "exact", 0.9)
	seedScoreRowWithValue(t, ctx, s, repoID, gv, 302, "call_graph", "exact", 0.1)

	got, err := s.QueryRankedFiles(ctx, repoID, "call_graph", 0)
	if err != nil {
		t.Fatalf("QueryRankedFiles: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d rows, want 3 distinct files (got=%v)", len(got), got)
	}
	// Sorted DESC by per-file MAX score: c.go(0.9), a.go(0.7), b.go(0.5).
	wantPaths := []string{"src/c.go", "src/a.go", "src/b.go"}
	wantScores := []float64{0.9, 0.7, 0.5}
	for i, want := range wantPaths {
		if got[i].Path != want {
			t.Errorf("got[%d].Path = %q, want %q", i, got[i].Path, want)
		}
		if got[i].Score != wantScores[i] {
			t.Errorf("got[%d].Score = %g, want %g", i, got[i].Score, wantScores[i])
		}
	}
}

// Test 3: identical scores → tiebreak by path ASC.
func TestStore_QueryRankedFiles_StableKeyTiebreak(t *testing.T) {
	s, ctx, _ := openStoreForOverlayTest(t)
	repoID := "r-tie"
	const snap uint64 = 7002
	seedCommittedSnapshot(t, ctx, s, repoID, snap, "committed")

	seedSnapshotFileAndSymbol(t, ctx, s, snap, 1, 101, repoID, "src/zeta.go", "Z")
	seedSnapshotFileAndSymbol(t, ctx, s, snap, 2, 201, repoID, "src/alpha.go", "A")
	seedSnapshotFileAndSymbol(t, ctx, s, snap, 3, 301, repoID, "src/beta.go", "B")

	const gv uint64 = 1
	seedScoreRowWithValue(t, ctx, s, repoID, gv, 101, "call_graph", "exact", 0.5)
	seedScoreRowWithValue(t, ctx, s, repoID, gv, 201, "call_graph", "exact", 0.5)
	seedScoreRowWithValue(t, ctx, s, repoID, gv, 301, "call_graph", "exact", 0.5)

	got, err := s.QueryRankedFiles(ctx, repoID, "call_graph", 0)
	if err != nil {
		t.Fatalf("QueryRankedFiles: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d rows, want 3", len(got))
	}
	wantPaths := []string{"src/alpha.go", "src/beta.go", "src/zeta.go"}
	for i, want := range wantPaths {
		if got[i].Path != want {
			t.Errorf("got[%d].Path = %q, want %q (path-ASC tiebreak)", i, got[i].Path, want)
		}
	}
}

// Test 4: every returned row carries a non-zero graph_version equal to the
// score row's graph_version.
func TestStore_QueryRankedFiles_GraphVersionStamped(t *testing.T) {
	s, ctx, _ := openStoreForOverlayTest(t)
	repoID := "r-gv"
	const snap uint64 = 7003
	seedCommittedSnapshot(t, ctx, s, repoID, snap, "committed")

	seedSnapshotFileAndSymbol(t, ctx, s, snap, 1, 101, repoID, "src/a.go", "A")
	const gv uint64 = 42
	seedScoreRowWithValue(t, ctx, s, repoID, gv, 101, "call_graph", "exact", 0.5)

	got, err := s.QueryRankedFiles(ctx, repoID, "call_graph", 0)
	if err != nil {
		t.Fatalf("QueryRankedFiles: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d rows, want 1", len(got))
	}
	if got[0].GraphVersion != gv {
		t.Errorf("got[0].GraphVersion = %d, want %d (must be the score-row gv, NOT 0 or snapshot_id)",
			got[0].GraphVersion, gv)
	}
}

// Test 5: limit caps the number of returned rows.
func TestStore_QueryRankedFiles_RespectsLimit(t *testing.T) {
	s, ctx, _ := openStoreForOverlayTest(t)
	repoID := "r-lim"
	const snap uint64 = 7004
	seedCommittedSnapshot(t, ctx, s, repoID, snap, "committed")

	// 10 files, descending scores 0.95, 0.85, 0.75, ...
	const gv uint64 = 1
	for i := 0; i < 10; i++ {
		fileID := uint64(i + 1)
		symID := 100 + uint64(i)
		path := "src/f" + string(rune('0'+i)) + ".go"
		seedSnapshotFileAndSymbol(t, ctx, s, snap, fileID, symID, repoID, path, "S")
		seedScoreRowWithValue(t, ctx, s, repoID, gv, symID, "call_graph", "exact", 0.95-0.10*float64(i))
	}

	got, err := s.QueryRankedFiles(ctx, repoID, "call_graph", 3)
	if err != nil {
		t.Fatalf("QueryRankedFiles: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d rows, want 3 (limit=3 over 10 files)", len(got))
	}
	// Top 3 by score-DESC.
	if got[0].Score < got[1].Score || got[1].Score < got[2].Score {
		t.Errorf("results not score-DESC: got %v", got)
	}
}

// --- Phase 65 65-11 Task 1: QuerySymbolByLocation + Node/StableKey resolvers ---

// seedSnapshotSymbolWithRange inserts a (semantic_files, semantic_symbols) pair
// with explicit (start_line, start_col, end_line, end_col, stable_key) plus
// derived start_byte / end_byte (start_byte = startLine*1000+startCol, the
// inner-scope discriminator QuerySymbolByLocation orders by). Stamps a
// dedicated INSERT OR IGNORE on semantic_files so multiple symbols can share a
// path within the same snapshot.
func seedSnapshotSymbolWithRange(
	t *testing.T,
	ctx context.Context,
	s *Store,
	snapID, fileID, symbolID uint64,
	repoID, path, name, stableKey string,
	startLine, startCol, endLine, endCol int,
) {
	t.Helper()
	if _, err := s.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO semantic_files (
			snapshot_id, file_id, repo_id, path, language, content_hash,
			size_bytes, line_count, generated, ignored, indexed_at
		) VALUES (?, ?, ?, ?, 'go', 'h', 100, 100, false, false, now())
	`, snapID, fileID, repoID, path); err != nil {
		t.Fatalf("seedSnapshotSymbolWithRange: file (snap=%d, file=%d): %v", snapID, fileID, err)
	}
	startByte := startLine*1000 + startCol
	endByte := endLine*1000 + endCol
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO semantic_symbols (
			snapshot_id, symbol_id, node_id, file_id, language, kind, name,
			qualified_name, stable_key, start_byte, end_byte, start_line,
			start_col, end_line, end_col, extraction_source, confidence
		) VALUES (?, ?, ?, ?, 'go', 'func', ?, ?, ?, ?, ?, ?, ?, ?, ?, 'tree-sitter', 1.0)
	`, snapID, symbolID, symbolID, fileID, name, name, stableKey,
		startByte, endByte, startLine, startCol, endLine, endCol); err != nil {
		t.Fatalf("seedSnapshotSymbolWithRange: symbol (snap=%d, sym=%d): %v", snapID, symbolID, err)
	}
}

// Test 1: hit on the exact start-of-range coordinate.
func TestStore_QuerySymbolByLocation_HitOnExactStart(t *testing.T) {
	s, ctx, _ := openStoreForOverlayTest(t)
	repoID := "r-loc-1"
	const snap uint64 = 8001
	seedCommittedSnapshot(t, ctx, s, repoID, snap, "committed")
	seedSnapshotSymbolWithRange(t, ctx, s, snap, 1, 100, repoID,
		"src/a.go", "Alpha", "src/a.go::Alpha",
		10, 5, 20, 1)

	got, ok, err := s.QuerySymbolByLocation(ctx, repoID, "src/a.go", 10, 5)
	if err != nil {
		t.Fatalf("QuerySymbolByLocation: %v", err)
	}
	if !ok {
		t.Fatalf("got ok=false, want true on exact-start hit")
	}
	if got != "src/a.go::Alpha" {
		t.Errorf("got stable_key=%q, want %q", got, "src/a.go::Alpha")
	}
}

// Test 2: hit on a coordinate strictly inside the range.
func TestStore_QuerySymbolByLocation_HitInsideRange(t *testing.T) {
	s, ctx, _ := openStoreForOverlayTest(t)
	repoID := "r-loc-2"
	const snap uint64 = 8002
	seedCommittedSnapshot(t, ctx, s, repoID, snap, "committed")
	seedSnapshotSymbolWithRange(t, ctx, s, snap, 1, 100, repoID,
		"src/a.go", "Alpha", "alpha-stable",
		10, 1, 20, 80)

	got, ok, err := s.QuerySymbolByLocation(ctx, repoID, "src/a.go", 15, 30)
	if err != nil {
		t.Fatalf("QuerySymbolByLocation: %v", err)
	}
	if !ok {
		t.Fatalf("got ok=false, want true on inside-range hit")
	}
	if got != "alpha-stable" {
		t.Errorf("got stable_key=%q, want %q", got, "alpha-stable")
	}
}

// Test 3: miss when the coordinate is strictly outside every symbol range.
func TestStore_QuerySymbolByLocation_MissOutsideRange(t *testing.T) {
	s, ctx, _ := openStoreForOverlayTest(t)
	repoID := "r-loc-3"
	const snap uint64 = 8003
	seedCommittedSnapshot(t, ctx, s, repoID, snap, "committed")
	seedSnapshotSymbolWithRange(t, ctx, s, snap, 1, 100, repoID,
		"src/a.go", "Alpha", "alpha-stable",
		10, 1, 20, 80)

	got, ok, err := s.QuerySymbolByLocation(ctx, repoID, "src/a.go", 5, 1)
	if err != nil {
		t.Fatalf("QuerySymbolByLocation (before range): %v", err)
	}
	if ok || got != "" {
		t.Errorf("got=(%q, %v), want ('', false) before range", got, ok)
	}
	got, ok, err = s.QuerySymbolByLocation(ctx, repoID, "src/a.go", 30, 1)
	if err != nil {
		t.Fatalf("QuerySymbolByLocation (after range): %v", err)
	}
	if ok || got != "" {
		t.Errorf("got=(%q, %v), want ('', false) after range", got, ok)
	}
}

// Test 4: a symbol at the right (line, col) but in a different file is a miss.
func TestStore_QuerySymbolByLocation_PathMismatch(t *testing.T) {
	s, ctx, _ := openStoreForOverlayTest(t)
	repoID := "r-loc-4"
	const snap uint64 = 8004
	seedCommittedSnapshot(t, ctx, s, repoID, snap, "committed")
	seedSnapshotSymbolWithRange(t, ctx, s, snap, 1, 100, repoID,
		"src/a.go", "Alpha", "alpha-stable",
		10, 1, 20, 80)

	got, ok, err := s.QuerySymbolByLocation(ctx, repoID, "src/other.go", 15, 30)
	if err != nil {
		t.Fatalf("QuerySymbolByLocation: %v", err)
	}
	if ok || got != "" {
		t.Errorf("got=(%q, %v), want ('', false) for different path", got, ok)
	}
}

// Test 5: when multiple symbols overlap the (line, col), the smallest-span
// (innermost) one wins.
func TestStore_QuerySymbolByLocation_PrefersInnerScope(t *testing.T) {
	s, ctx, _ := openStoreForOverlayTest(t)
	repoID := "r-loc-5"
	const snap uint64 = 8005
	seedCommittedSnapshot(t, ctx, s, repoID, snap, "committed")
	// Outer symbol spans lines 5-30.
	seedSnapshotSymbolWithRange(t, ctx, s, snap, 1, 100, repoID,
		"src/a.go", "Outer", "outer-stable",
		5, 1, 30, 1)
	// Inner symbol spans lines 10-15.
	seedSnapshotSymbolWithRange(t, ctx, s, snap, 1, 101, repoID,
		"src/a.go", "Inner", "inner-stable",
		10, 1, 15, 1)

	got, ok, err := s.QuerySymbolByLocation(ctx, repoID, "src/a.go", 12, 5)
	if err != nil {
		t.Fatalf("QuerySymbolByLocation: %v", err)
	}
	if !ok {
		t.Fatalf("got ok=false, want true (both outer + inner overlap)")
	}
	if got != "inner-stable" {
		t.Errorf("got stable_key=%q, want %q (inner-scope MUST win on overlap)",
			got, "inner-stable")
	}
}

// Test 6: when no committed snapshot exists for repoID, the reader returns
// a clean miss without error (consistent with QueryRankedFiles).
func TestStore_QuerySymbolByLocation_NoCommittedSnapshot(t *testing.T) {
	s, ctx, _ := openStoreForOverlayTest(t)
	got, ok, err := s.QuerySymbolByLocation(ctx, "r-never-opened", "src/a.go", 10, 1)
	if err != nil {
		t.Fatalf("QuerySymbolByLocation: %v", err)
	}
	if ok || got != "" {
		t.Errorf("got=(%q, %v), want ('', false) when no committed snapshot", got, ok)
	}
}

// Test 7: QueryNodeIDByStableKey hit + miss.
func TestStore_QueryNodeIDByStableKey_HitAndMiss(t *testing.T) {
	s, ctx, _ := openStoreForOverlayTest(t)
	repoID := "r-node-7"
	const snap uint64 = 8007
	seedCommittedSnapshot(t, ctx, s, repoID, snap, "committed")
	seedSnapshotSymbolWithRange(t, ctx, s, snap, 1, 7777, repoID,
		"src/a.go", "Sigma", "sigma-key",
		10, 1, 20, 1)

	got, ok, err := s.QueryNodeIDByStableKey(ctx, repoID, "sigma-key")
	if err != nil {
		t.Fatalf("QueryNodeIDByStableKey hit: %v", err)
	}
	if !ok || got != 7777 {
		t.Errorf("got=(%d, %v), want (7777, true)", got, ok)
	}
	got, ok, err = s.QueryNodeIDByStableKey(ctx, repoID, "no-such-key")
	if err != nil {
		t.Fatalf("QueryNodeIDByStableKey miss: %v", err)
	}
	if ok || got != 0 {
		t.Errorf("got=(%d, %v), want (0, false) for unknown key", got, ok)
	}
	// No committed snapshot path: clean miss without error.
	got, ok, err = s.QueryNodeIDByStableKey(ctx, "r-never-opened", "sigma-key")
	if err != nil {
		t.Fatalf("QueryNodeIDByStableKey no-snapshot: %v", err)
	}
	if ok || got != 0 {
		t.Errorf("got=(%d, %v), want (0, false) when no committed snapshot", got, ok)
	}
}

// Test 8: QueryStableKeyByNodeID hit + miss (inverse of Test 7).
func TestStore_QueryStableKeyByNodeID_HitAndMiss(t *testing.T) {
	s, ctx, _ := openStoreForOverlayTest(t)
	repoID := "r-node-8"
	const snap uint64 = 8008
	seedCommittedSnapshot(t, ctx, s, repoID, snap, "committed")
	seedSnapshotSymbolWithRange(t, ctx, s, snap, 1, 8888, repoID,
		"src/a.go", "Tau", "tau-key",
		10, 1, 20, 1)

	got, ok, err := s.QueryStableKeyByNodeID(ctx, repoID, 8888)
	if err != nil {
		t.Fatalf("QueryStableKeyByNodeID hit: %v", err)
	}
	if !ok || got != "tau-key" {
		t.Errorf("got=(%q, %v), want (\"tau-key\", true)", got, ok)
	}
	got, ok, err = s.QueryStableKeyByNodeID(ctx, repoID, 99999)
	if err != nil {
		t.Fatalf("QueryStableKeyByNodeID miss: %v", err)
	}
	if ok || got != "" {
		t.Errorf("got=(%q, %v), want (\"\", false) for unknown node id", got, ok)
	}
}

// --- Phase 65 65-12 Task 1: QuerySymbolLocationByStableKey ---

// Test 9a: hit — stable_key resolves to (path, start_line, start_col).
func TestStore_QuerySymbolLocationByStableKey_Hit(t *testing.T) {
	s, ctx, _ := openStoreForOverlayTest(t)
	repoID := "r-loc-9a"
	const snap uint64 = 9001
	seedCommittedSnapshot(t, ctx, s, repoID, snap, "committed")
	seedSnapshotSymbolWithRange(t, ctx, s, snap, 1, 9001, repoID,
		"src/loc.go", "Locator", "loc-stable-key",
		42, 7, 60, 1)

	path, line, col, ok, err := s.QuerySymbolLocationByStableKey(ctx, repoID, "loc-stable-key")
	if err != nil {
		t.Fatalf("QuerySymbolLocationByStableKey: %v", err)
	}
	if !ok {
		t.Fatalf("got ok=false, want true on stable-key hit")
	}
	if path != "src/loc.go" {
		t.Errorf("path: got %q, want %q", path, "src/loc.go")
	}
	if line != 42 {
		t.Errorf("line: got %d, want 42", line)
	}
	if col != 7 {
		t.Errorf("col: got %d, want 7", col)
	}
}

// Test 9b: miss — unknown stable_key returns ("", 0, 0, false, nil).
func TestStore_QuerySymbolLocationByStableKey_Miss(t *testing.T) {
	s, ctx, _ := openStoreForOverlayTest(t)
	repoID := "r-loc-9b"
	const snap uint64 = 9002
	seedCommittedSnapshot(t, ctx, s, repoID, snap, "committed")
	seedSnapshotSymbolWithRange(t, ctx, s, snap, 1, 9002, repoID,
		"src/loc.go", "Locator", "loc-stable-key",
		10, 1, 20, 1)

	path, line, col, ok, err := s.QuerySymbolLocationByStableKey(ctx, repoID, "no-such-key")
	if err != nil {
		t.Fatalf("QuerySymbolLocationByStableKey: %v", err)
	}
	if ok || path != "" || line != 0 || col != 0 {
		t.Errorf("got=(%q, %d, %d, %v), want (\"\", 0, 0, false) for unknown key",
			path, line, col, ok)
	}
}

// Test 9c: no committed snapshot — clean miss, no error.
func TestStore_QuerySymbolLocationByStableKey_NoCommittedSnapshot(t *testing.T) {
	s, ctx, _ := openStoreForOverlayTest(t)
	path, line, col, ok, err := s.QuerySymbolLocationByStableKey(ctx, "r-never-opened", "any-key")
	if err != nil {
		t.Fatalf("QuerySymbolLocationByStableKey: %v", err)
	}
	if ok || path != "" || line != 0 || col != 0 {
		t.Errorf("got=(%q, %d, %d, %v), want (\"\", 0, 0, false) when no committed snapshot",
			path, line, col, ok)
	}
}

// --- Phase 69 69-01: ClusterStatusForGraphVersion ---
//
// STATUS-01 read seam — returns counts + computed_at + IsCurrent/ActualGraphVersion
// discriminators for the requested (repoID, graphVersion) over semantic_clusters
// + semantic_cluster_members. Lock-free per D-09 (no Begin/Commit/Abort/Write on
// the read path); MemberCount is COUNT(*) over semantic_cluster_members, NOT the
// overloaded semantic_clusters.score column.
//
// Stale-fallback semantics (Approach A from 69-CONTEXT D1): when no rows exist at
// the requested gv but rows exist at a strictly-lower gv for the same repo, the
// accessor returns IsCurrent=false, ActualGraphVersion=<highest prior gv>, and
// counts/timestamp for THAT prior gv. The adapter (Plan 69-05) uses the
// discriminators to emit `"current"` / `"stale"` / `"unknown"`.

// clusterSeed is one (cluster_id, memberCount) tuple consumed by
// seedClusterRows. memberCount writes into ClusterSummary.MemberCount (which
// the production UpsertClusters then writes into semantic_clusters.score —
// this is the overloaded column the accessor MUST NOT trust). For the actual
// row count over semantic_cluster_members, the helper generates `actualMembers`
// distinct node_ids; if actualMembers == 0 it defaults to memberCount.
type clusterSeed struct {
	id            uint64
	memberCount   int // → semantic_clusters.score (DO NOT TRUST in accessor)
	actualMembers int // → number of semantic_cluster_members rows; 0 means use memberCount
}

// seedClusterRows opens an OverlayTx, writes the cluster summaries +
// per-member rows for (repoID, gv) via the production UpsertClusters /
// UpsertClusterMembers, and commits. Exported (lowercase, package-local) so
// Plan 69-06's integration test can reuse it.
func seedClusterRows(t *testing.T, ctx context.Context, s *Store, repoID string, gv uint64, seeds []clusterSeed) {
	t.Helper()
	tx, err := s.BeginOverlayTx(ctx, repoID)
	if err != nil {
		t.Fatalf("seedClusterRows: BeginOverlayTx(%q): %v", repoID, err)
	}
	summaries := make([]ClusterSummary, 0, len(seeds))
	var members []ClusterMemberRow
	var nodeCounter uint64 = 1_000_000 * gv // gv-disjoint node-id space across calls.
	for _, sd := range seeds {
		summaries = append(summaries, ClusterSummary{ID: sd.id, MemberCount: sd.memberCount})
		n := sd.actualMembers
		if n == 0 {
			n = sd.memberCount
		}
		for i := 0; i < n; i++ {
			nodeCounter++
			members = append(members, ClusterMemberRow{ClusterID: sd.id, NodeID: nodeCounter})
		}
	}
	const projection = "call_graph"
	if err := tx.UpsertClusters(ctx, projection, gv, summaries); err != nil {
		_ = tx.Rollback()
		t.Fatalf("seedClusterRows: UpsertClusters(%q, gv=%d): %v", repoID, gv, err)
	}
	if err := tx.UpsertClusterMembers(ctx, projection, gv, members); err != nil {
		_ = tx.Rollback()
		t.Fatalf("seedClusterRows: UpsertClusterMembers(%q, gv=%d): %v", repoID, gv, err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("seedClusterRows: Commit(%q, gv=%d): %v", repoID, gv, err)
	}
}

// Test 1 / 4 / 5 / 7: basic-current, truly-empty, future-only, and
// member-count-derivation cases for ClusterStatusForGraphVersion.
func TestClusterStatusForGraphVersion_Basic(t *testing.T) {
	type wantRow struct {
		graphVersion       uint64
		actualGraphVersion uint64
		isCurrent          bool
		memberCount        int
		clusterCount       int
		computedAtPositive bool // require ComputedAt > 0 when populated
	}
	tests := []struct {
		name   string
		seed   func(t *testing.T, ctx context.Context, s *Store, repoID string)
		query  uint64
		expect wantRow
	}{
		{
			name: "Test1_BasicPopulatedCurrent_gv5_two_clusters_3plus2_members",
			seed: func(t *testing.T, ctx context.Context, s *Store, repoID string) {
				seedClusterRows(t, ctx, s, repoID, 5, []clusterSeed{
					{id: 1, memberCount: 3},
					{id: 2, memberCount: 2},
				})
			},
			query: 5,
			expect: wantRow{
				graphVersion:       5,
				actualGraphVersion: 5,
				isCurrent:          true,
				memberCount:        5,
				clusterCount:       2,
				computedAtPositive: true,
			},
		},
		{
			name:  "Test4_TrulyEmpty_no_rows_at_any_gv_returns_zero_value",
			seed:  func(t *testing.T, ctx context.Context, s *Store, repoID string) {},
			query: 5,
			expect: wantRow{
				graphVersion:       5,
				actualGraphVersion: 0,
				isCurrent:          false,
				memberCount:        0,
				clusterCount:       0,
				computedAtPositive: false,
			},
		},
		{
			name: "Test5_WrongGv_only_future_gv10_rows_query_gv5_no_fallback_to_future",
			seed: func(t *testing.T, ctx context.Context, s *Store, repoID string) {
				seedClusterRows(t, ctx, s, repoID, 10, []clusterSeed{
					{id: 1, memberCount: 2},
				})
			},
			query: 5,
			expect: wantRow{
				graphVersion:       5,
				actualGraphVersion: 0,
				isCurrent:          false,
				memberCount:        0,
				clusterCount:       0,
				computedAtPositive: false,
			},
		},
		{
			name: "Test7_MemberCount_from_COUNT_not_score_column",
			// ClusterSummary.MemberCount=99 writes 99.0 into the
			// semantic_clusters.score column (overlay.go:835 quirk) but
			// only 3 actual semantic_cluster_members rows exist. The
			// accessor MUST return 3.
			seed: func(t *testing.T, ctx context.Context, s *Store, repoID string) {
				seedClusterRows(t, ctx, s, repoID, 5, []clusterSeed{
					{id: 1, memberCount: 99, actualMembers: 3},
				})
			},
			query: 5,
			expect: wantRow{
				graphVersion:       5,
				actualGraphVersion: 5,
				isCurrent:          true,
				memberCount:        3, // NOT 99 — proves COUNT(*) wins over score
				clusterCount:       1,
				computedAtPositive: true,
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, ctx, _ := openStoreForOverlayTest(t)
			repoID := "r-69-01-" + tc.name
			tc.seed(t, ctx, s, repoID)

			got, err := s.ClusterStatusForGraphVersion(ctx, repoID, tc.query)
			if err != nil {
				t.Fatalf("ClusterStatusForGraphVersion: %v", err)
			}
			if got.GraphVersion != tc.expect.graphVersion {
				t.Errorf("GraphVersion = %d, want %d", got.GraphVersion, tc.expect.graphVersion)
			}
			if got.ActualGraphVersion != tc.expect.actualGraphVersion {
				t.Errorf("ActualGraphVersion = %d, want %d", got.ActualGraphVersion, tc.expect.actualGraphVersion)
			}
			if got.IsCurrent != tc.expect.isCurrent {
				t.Errorf("IsCurrent = %v, want %v", got.IsCurrent, tc.expect.isCurrent)
			}
			if got.MemberCount != tc.expect.memberCount {
				t.Errorf("MemberCount = %d, want %d", got.MemberCount, tc.expect.memberCount)
			}
			if got.ClusterCount != tc.expect.clusterCount {
				t.Errorf("ClusterCount = %d, want %d", got.ClusterCount, tc.expect.clusterCount)
			}
			if tc.expect.computedAtPositive && got.ComputedAt <= 0 {
				t.Errorf("ComputedAt = %d, want > 0 (populated case)", got.ComputedAt)
			}
			if !tc.expect.computedAtPositive && got.ComputedAt != 0 {
				t.Errorf("ComputedAt = %d, want 0 (empty/zero-value case)", got.ComputedAt)
			}
		})
	}
}

// Test 2 / 3: stale-fallback picks HIGHEST prior gv with rows for the repo.
func TestClusterStatusForGraphVersion_StaleFallback(t *testing.T) {
	type wantRow struct {
		actualGraphVersion uint64
		isCurrent          bool
		memberCount        int
		clusterCount       int
	}
	tests := []struct {
		name   string
		seed   func(t *testing.T, ctx context.Context, s *Store, repoID string)
		query  uint64
		expect wantRow
	}{
		{
			name: "Test2_StaleFallback_only_gv4_query_gv5",
			seed: func(t *testing.T, ctx context.Context, s *Store, repoID string) {
				seedClusterRows(t, ctx, s, repoID, 4, []clusterSeed{
					{id: 1, memberCount: 3},
					{id: 2, memberCount: 2},
				})
			},
			query: 5,
			expect: wantRow{
				actualGraphVersion: 4,
				isCurrent:          false,
				memberCount:        5,
				clusterCount:       2,
			},
		},
		{
			name: "Test3_StaleFallback_picks_highest_prior_gv2_and_gv4_query_gv5",
			seed: func(t *testing.T, ctx context.Context, s *Store, repoID string) {
				seedClusterRows(t, ctx, s, repoID, 2, []clusterSeed{
					{id: 10, memberCount: 7},
				})
				seedClusterRows(t, ctx, s, repoID, 4, []clusterSeed{
					{id: 1, memberCount: 3},
					{id: 2, memberCount: 2},
				})
			},
			query: 5,
			expect: wantRow{
				actualGraphVersion: 4, // NOT 2 — must pick highest prior
				isCurrent:          false,
				memberCount:        5, // counts for gv=4 only
				clusterCount:       2,
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, ctx, _ := openStoreForOverlayTest(t)
			repoID := "r-69-01-stale-" + tc.name
			tc.seed(t, ctx, s, repoID)

			got, err := s.ClusterStatusForGraphVersion(ctx, repoID, tc.query)
			if err != nil {
				t.Fatalf("ClusterStatusForGraphVersion: %v", err)
			}
			if got.GraphVersion != tc.query {
				t.Errorf("GraphVersion = %d, want %d (caller's requested gv echoed back)", got.GraphVersion, tc.query)
			}
			if got.ActualGraphVersion != tc.expect.actualGraphVersion {
				t.Errorf("ActualGraphVersion = %d, want %d", got.ActualGraphVersion, tc.expect.actualGraphVersion)
			}
			if got.IsCurrent != tc.expect.isCurrent {
				t.Errorf("IsCurrent = %v, want %v", got.IsCurrent, tc.expect.isCurrent)
			}
			if got.MemberCount != tc.expect.memberCount {
				t.Errorf("MemberCount = %d, want %d", got.MemberCount, tc.expect.memberCount)
			}
			if got.ClusterCount != tc.expect.clusterCount {
				t.Errorf("ClusterCount = %d, want %d", got.ClusterCount, tc.expect.clusterCount)
			}
			if got.ComputedAt <= 0 {
				t.Errorf("ComputedAt = %d, want > 0 (rows from prior gv carry a timestamp)", got.ComputedAt)
			}
		})
	}
}

// Test 6: 100 concurrent reader probes alongside a background BeginOverlayTx
// writer must not deadlock and must not return error. Mirrors
// TestCountStaleScoreRows_LockFree_RaceSafe.
func TestClusterStatusForGraphVersion_LockFree_RaceSafe(t *testing.T) {
	if testing.Short() {
		t.Skip("race-safety stress; skipped under -short")
	}
	s, ctx, _ := openStoreForOverlayTest(t)
	repoID := "r-69-01-race"
	seedClusterRows(t, ctx, s, repoID, 1, []clusterSeed{
		{id: 1, memberCount: 3},
		{id: 2, memberCount: 2},
	})

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

	var readers sync.WaitGroup
	for i := 0; i < 100; i++ {
		readers.Add(1)
		go func() {
			defer readers.Done()
			done := make(chan error, 1)
			go func() {
				_, e := s.ClusterStatusForGraphVersion(ctx, repoID, 1)
				done <- e
			}()
			select {
			case e := <-done:
				if e != nil {
					t.Errorf("ClusterStatusForGraphVersion under contention: %v", e)
				}
			case <-time.After(5 * time.Second):
				t.Errorf("ClusterStatusForGraphVersion deadlocked (>5s under contention)")
			}
		}()
	}
	readers.Wait()
	stop.Store(true)
	wg.Wait()
}

// --- Phase 71-01 Task 1: QuerySymbolByName + LatestExtractorRunID ---

// TestQuerySymbolByName_Exact: a single (path, name) match returns 1 row.
func TestQuerySymbolByName_Exact(t *testing.T) {
	s, ctx, _ := openStoreForOverlayTest(t)
	repoID := "r-qsbn-exact"
	const snap uint64 = 9101
	seedCommittedSnapshot(t, ctx, s, repoID, snap, "committed")
	seedSnapshotSymbolWithRange(t, ctx, s, snap, 1, 100, repoID,
		"src/a.go", "Alpha", "src/a.go::Alpha",
		10, 0, 20, 0)

	got, err := s.QuerySymbolByName(ctx, repoID, "src/a.go", "Alpha")
	if err != nil {
		t.Fatalf("QuerySymbolByName: %v", err)
	}
	if len(got) != 1 || got[0] != "src/a.go::Alpha" {
		t.Errorf("got = %v, want [src/a.go::Alpha]", got)
	}
}

// TestQuerySymbolByName_Ambiguous: multiple matches return all rows
// (capped at 6) in deterministic stable_key ASC order.
func TestQuerySymbolByName_Ambiguous(t *testing.T) {
	s, ctx, _ := openStoreForOverlayTest(t)
	repoID := "r-qsbn-amb"
	const snap uint64 = 9102
	seedCommittedSnapshot(t, ctx, s, repoID, snap, "committed")
	// Seed three overloads sharing (path, name) but distinct stable_keys.
	seedSnapshotSymbolWithRange(t, ctx, s, snap, 10, 1001, repoID,
		"src/a.go", "Foo", "src/a.go::Foo#a", 1, 0, 5, 0)
	seedSnapshotSymbolWithRange(t, ctx, s, snap, 10, 1002, repoID,
		"src/a.go", "Foo", "src/a.go::Foo#b", 10, 0, 15, 0)
	seedSnapshotSymbolWithRange(t, ctx, s, snap, 10, 1003, repoID,
		"src/a.go", "Foo", "src/a.go::Foo#c", 20, 0, 25, 0)

	got, err := s.QuerySymbolByName(ctx, repoID, "src/a.go", "Foo")
	if err != nil {
		t.Fatalf("QuerySymbolByName: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d rows, want 3", len(got))
	}
	want := []string{"src/a.go::Foo#a", "src/a.go::Foo#b", "src/a.go::Foo#c"}
	for i, sk := range want {
		if got[i] != sk {
			t.Errorf("got[%d]=%q, want %q (order)", i, got[i], sk)
		}
	}
}

// TestQuerySymbolByName_CapsAtSix: 8 overloads return only 6 rows.
// Allows the seed resolver to detect ">5 candidates" before truncating to 5.
func TestQuerySymbolByName_CapsAtSix(t *testing.T) {
	s, ctx, _ := openStoreForOverlayTest(t)
	repoID := "r-qsbn-cap"
	const snap uint64 = 9103
	seedCommittedSnapshot(t, ctx, s, repoID, snap, "committed")
	for i := 0; i < 8; i++ {
		seedSnapshotSymbolWithRange(t, ctx, s, snap, uint64(20+i), uint64(2000+i), repoID,
			"src/a.go", "Many", fmt.Sprintf("src/a.go::Many#%02d", i),
			i*10+1, 0, i*10+5, 0)
	}
	got, err := s.QuerySymbolByName(ctx, repoID, "src/a.go", "Many")
	if err != nil {
		t.Fatalf("QuerySymbolByName: %v", err)
	}
	if len(got) != 6 {
		t.Errorf("got %d rows, want 6 (LIMIT cap)", len(got))
	}
}

// TestQuerySymbolByName_NotFound: unknown name returns nil slice, nil error.
func TestQuerySymbolByName_NotFound(t *testing.T) {
	s, ctx, _ := openStoreForOverlayTest(t)
	repoID := "r-qsbn-miss"
	const snap uint64 = 9104
	seedCommittedSnapshot(t, ctx, s, repoID, snap, "committed")
	seedSnapshotSymbolWithRange(t, ctx, s, snap, 1, 100, repoID,
		"src/a.go", "Alpha", "src/a.go::Alpha", 1, 0, 5, 0)

	got, err := s.QuerySymbolByName(ctx, repoID, "src/a.go", "DoesNotExist")
	if err != nil {
		t.Fatalf("QuerySymbolByName: %v", err)
	}
	if got != nil {
		t.Errorf("got = %v, want nil", got)
	}
}

// TestQuerySymbolByName_NoCommittedSnapshot: empty repo returns nil/nil.
func TestQuerySymbolByName_NoCommittedSnapshot(t *testing.T) {
	s, ctx, _ := openStoreForOverlayTest(t)
	got, err := s.QuerySymbolByName(ctx, "r-empty", "src/a.go", "Anything")
	if err != nil {
		t.Fatalf("QuerySymbolByName: %v", err)
	}
	if got != nil {
		t.Errorf("got = %v, want nil for empty repo", got)
	}
}

// TestQuerySymbolByName_NilStore: nil receiver returns error.
func TestQuerySymbolByName_NilStore(t *testing.T) {
	var s *Store
	_, err := s.QuerySymbolByName(context.Background(), "r", "p", "n")
	if err == nil {
		t.Fatalf("got nil error, want nil-store error")
	}
}

// TestLatestExtractorRunID_PopulatedSnapshot: returns non-empty id derived
// from the latest committed snapshot id.
func TestLatestExtractorRunID_PopulatedSnapshot(t *testing.T) {
	s, ctx, _ := openStoreForOverlayTest(t)
	repoID := "r-extrun-pop"
	const snap uint64 = 9201
	seedCommittedSnapshot(t, ctx, s, repoID, snap, "committed")
	got, err := s.LatestExtractorRunID(ctx, repoID)
	if err != nil {
		t.Fatalf("LatestExtractorRunID: %v", err)
	}
	want := "snap-9201"
	if got != want {
		t.Errorf("got=%q, want %q", got, want)
	}
}

// TestLatestExtractorRunID_EmptyRepo: empty repo returns "" with nil error.
func TestLatestExtractorRunID_EmptyRepo(t *testing.T) {
	s, ctx, _ := openStoreForOverlayTest(t)
	got, err := s.LatestExtractorRunID(ctx, "r-empty")
	if err != nil {
		t.Fatalf("LatestExtractorRunID: %v", err)
	}
	if got != "" {
		t.Errorf("got=%q, want empty for empty repo", got)
	}
}

// TestLatestExtractorRunID_NilStore: nil receiver returns error.
func TestLatestExtractorRunID_NilStore(t *testing.T) {
	var s *Store
	_, err := s.LatestExtractorRunID(context.Background(), "r")
	if err == nil {
		t.Fatalf("got nil error, want nil-store error")
	}
}

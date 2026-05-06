package lspenrich_test

import (
	"context"
	"testing"

	"github.com/agenthands/helix/internal/semantic/lspenrich"
	"github.com/agenthands/helix/internal/semantic/store"
)

// realMergeStoreAdapter is the test-time CascadeStore that wraps a real
// *store.OverlayTx and translates lspenrich.Edge → store.EdgeRow on the
// UpsertEdgesWithMerge call path. Phase 62 P02 Task 4.5 (B3) — this is the
// production-shaped boundary the daemon adapter mirrors.
type realMergeStoreAdapter struct {
	store *store.Store
}

func (a *realMergeStoreAdapter) BeginCascadeTx(ctx context.Context, repoID string) (lspenrich.CascadeTx, error) {
	tx, err := a.store.BeginOverlayTx(ctx, repoID)
	if err != nil {
		return nil, err
	}
	return &realMergeTxAdapter{tx: tx}, nil
}

type realMergeTxAdapter struct {
	tx *store.OverlayTx
}

func (a *realMergeTxAdapter) UpsertSymbols(ctx context.Context, path string, syms []lspenrich.Symbol) error {
	return nil
}

func (a *realMergeTxAdapter) UpsertReferences(ctx context.Context, path string, refs []lspenrich.Reference) error {
	return nil
}

func (a *realMergeTxAdapter) UpsertEdges(ctx context.Context, edges []lspenrich.Edge) error {
	return a.UpsertEdgesWithMerge(ctx, edges)
}

func (a *realMergeTxAdapter) UpsertEdgesWithMerge(ctx context.Context, edges []lspenrich.Edge) error {
	if len(edges) == 0 {
		return nil
	}
	rows := make([]store.EdgeRow, 0, len(edges))
	for _, e := range edges {
		rows = append(rows, store.EdgeRow{
			SrcNodeID:       e.SrcNodeID,
			DstNodeID:       e.DstNodeID,
			EdgeKind:        e.Kind,
			Source:          e.Source,
			Confidence:      e.Confidence,
			Weight:          e.Weight,
			ValidationState: e.ValidationState,
		})
	}
	return a.tx.UpsertEdgesWithMerge(ctx, rows)
}

func (a *realMergeTxAdapter) UpsertDiagnostics(ctx context.Context, path string, diags []lspenrich.Diagnostic) error {
	return nil
}

func (a *realMergeTxAdapter) WriteInvalidations(ctx context.Context) error { return nil }
func (a *realMergeTxAdapter) MarkFileSemanticPending(ctx context.Context, path, reason string) error {
	return a.tx.MarkFileSemanticPending(ctx, path, reason)
}
func (a *realMergeTxAdapter) Commit() error   { return a.tx.Commit() }
func (a *realMergeTxAdapter) Rollback() error { return a.tx.Rollback() }
func (a *realMergeTxAdapter) Epoch() uint64   { return a.tx.Epoch() }

// hoverOnlyMergeLSP is a fakeLSP that emits a single LSP-validated hover
// edge with concrete src/dst node IDs. Other cascade methods return zero.
type hoverOnlyMergeLSP struct {
	src, dst uint64
	kind     string
}

func (h *hoverOnlyMergeLSP) DocumentSymbol(ctx context.Context, path string) ([]lspenrich.Symbol, error) {
	return []lspenrich.Symbol{{Name: "S1", Path: path}}, nil
}
func (h *hoverOnlyMergeLSP) DrainDiagnostics(path string) []lspenrich.Diagnostic { return nil }
func (h *hoverOnlyMergeLSP) Hover(ctx context.Context, sym lspenrich.Symbol) (*lspenrich.Edge, error) {
	return &lspenrich.Edge{
		SrcNodeID:       h.src,
		DstNodeID:       h.dst,
		Kind:            h.kind,
		Source:          "lsp.hover",
		Confidence:      1.0,
		Weight:          1.0,
		ValidationState: "validated",
	}, nil
}
func (h *hoverOnlyMergeLSP) CallHierarchy(ctx context.Context, sym lspenrich.Symbol, depth int) ([]lspenrich.Edge, error) {
	return nil, nil
}
func (h *hoverOnlyMergeLSP) TypeHierarchy(ctx context.Context, sym lspenrich.Symbol, depth int) ([]lspenrich.Edge, error) {
	return nil, nil
}
func (h *hoverOnlyMergeLSP) Implementation(ctx context.Context, sym lspenrich.Symbol) ([]lspenrich.Edge, error) {
	return nil, nil
}
func (h *hoverOnlyMergeLSP) Definition(ctx context.Context, ref lspenrich.Reference) (*lspenrich.Edge, error) {
	return nil, nil
}
func (h *hoverOnlyMergeLSP) ReferencesForSymbol(sym lspenrich.Symbol) []lspenrich.Reference {
	return nil
}

// TestCascadeLSP_UpgradesCommentEdgeInPlace closes Phase 62 AC10 — when a
// pre-seeded comment edge points at dst=A and the cascade later emits an
// LSP-validated edge for the same (src, edge_kind) at a DIFFERENT dst=B,
// the comment row must be deleted (D-14 refutation invariant).
//
// Test path:
//
//  1. Open a real *store.Store backed by a temp DuckDB.
//  2. Seed (src=1, dst=10, TYPE_OF, comment.tsdoc, 0.60) directly via
//     tx.UpsertEdgesWithMerge.
//  3. Run a cascade that emits (src=1, dst=20, TYPE_OF, lsp.hover, 1.0)
//     for the same (src, kind) but a different dst — the production
//     UpsertEdgesWithMerge SQL deletes the comment row before insert.
//  4. Assert: only the LSP row at dst=20 remains.
func TestCascadeLSP_UpgradesCommentEdgeInPlace(t *testing.T) {
	s := openRealStoreForCascade(t)
	adapter := &realMergeStoreAdapter{store: s}

	ctx := context.Background()

	// Step 1: seed comment edge.
	seedTx, err := s.BeginOverlayTx(ctx, "ws-merge")
	if err != nil {
		t.Fatalf("seed BeginOverlayTx: %v", err)
	}
	if err := seedTx.UpsertEdgesWithMerge(ctx, []store.EdgeRow{{
		SrcNodeID:       1,
		DstNodeID:       10,
		EdgeKind:        "TYPE_OF",
		Source:          "comment.tsdoc",
		Confidence:      0.60,
		Weight:          1.0,
		ValidationState: "unresolved",
	}}); err != nil {
		t.Fatalf("seed comment: %v", err)
	}
	if err := seedTx.Commit(); err != nil {
		t.Fatalf("seed Commit: %v", err)
	}

	// Step 2: confirm seed landed.
	var seeded int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM semantic_live_overlay_edges
		WHERE repo_id='ws-merge' AND src_node_id=1 AND edge_kind='TYPE_OF'`).Scan(&seeded); err != nil {
		t.Fatalf("count after seed: %v", err)
	}
	if seeded != 1 {
		t.Fatalf("expected 1 comment row after seed, got %d", seeded)
	}

	// Step 3: run cascade emitting LSP edge at dst=20.
	c := &lspenrich.Cascade{
		Store:        adapter,
		LSP:          &hoverOnlyMergeLSP{src: 1, dst: 20, kind: "TYPE_OF"},
		Capabilities: lspenrich.NewCapabilityCache(),
	}
	job := lspenrich.JobForTest("ws-merge", "/r1.go")
	budget := mustBudget(t, cascadeNow())
	out := c.Run(ctx, job, "go", &budget, func() bool { return false })
	if out != lspenrich.OutcomeApplied {
		t.Fatalf("cascade outcome=%q, want OutcomeApplied", out)
	}

	// Step 4: assert only the LSP row at dst=20 remains.
	rows, err := s.DB().Query(`SELECT dst_node_id, source, confidence FROM semantic_live_overlay_edges
		WHERE repo_id='ws-merge' AND src_node_id=1 AND edge_kind='TYPE_OF' AND status='live'
		ORDER BY dst_node_id`)
	if err != nil {
		t.Fatalf("query post-cascade: %v", err)
	}
	defer rows.Close()
	type row struct {
		dst    uint64
		source string
		conf   float64
	}
	var got []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.dst, &r.source, &r.conf); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got = append(got, r)
	}
	if len(got) != 1 {
		t.Fatalf("post-cascade row count: got %d (%v), want 1 (D-14 refutation closes AC10)", len(got), got)
	}
	if got[0].dst != 20 {
		t.Errorf("surviving dst: got %d, want 20", got[0].dst)
	}
	if got[0].source != "lsp.hover" {
		t.Errorf("surviving source: got %q, want lsp.hover", got[0].source)
	}
	if got[0].conf < 1.0 {
		t.Errorf("surviving confidence: got %v, want >= 1.0", got[0].conf)
	}
}

package lspenrich_test

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/agenthands/helix/internal/obs"
	"github.com/agenthands/helix/internal/semantic"
	"github.com/agenthands/helix/internal/semantic/lspenrich"
	"github.com/agenthands/helix/internal/semantic/store"
)

// W2: real-store epoch test. Two consecutive Cascade.Run calls against the
// production *store.Store backed by an in-memory-style temp DuckDB; assert
// second.Epoch() == first.Epoch() + 1 (the Phase 60 D-04 monotone-forever
// epoch contract holds when the cascade commits via the real store).
func TestCascade_Epoch_RealStore_AdvancesByOnePerCommit(t *testing.T) {
	store := openRealStoreForCascade(t)
	adapter := &realStoreCascadeAdapter{store: store}

	lsp := newFakeLSP()
	lsp.referencesPerSymbol = 0

	c := &lspenrich.Cascade{
		Store:        adapter,
		LSP:          lsp,
		Capabilities: lspenrich.NewCapabilityCache(),
	}

	ctx := context.Background()

	job := lspenrich.JobForTest("ws-real-epoch", "/r1.go")
	budget1 := mustBudget(t, cascadeNow)
	out1 := c.Run(ctx, job, "go", &budget1, func() bool { return false })
	if out1 != lspenrich.OutcomeApplied {
		t.Fatalf("Run #1: outcome=%q, want OutcomeApplied", out1)
	}

	budget2 := mustBudget(t, cascadeNow)
	out2 := c.Run(ctx, job, "go", &budget2, func() bool { return false })
	if out2 != lspenrich.OutcomeApplied {
		t.Fatalf("Run #2: outcome=%q, want OutcomeApplied", out2)
	}

	if len(adapter.epochs) != 2 {
		t.Fatalf("expected 2 committed epochs, got %d", len(adapter.epochs))
	}
	if adapter.epochs[1] != adapter.epochs[0]+1 {
		t.Errorf("real-store epoch: tx2.Epoch()=%d, tx1.Epoch()=%d (want exactly +1)",
			adapter.epochs[1], adapter.epochs[0])
	}
}

// realStoreCascadeAdapter is the test-time CascadeStore implementation that
// opens a real *store.OverlayTx, wraps it in a small adapter that satisfies
// lspenrich.CascadeTx, and records the per-tx epoch for the W2 assertion.
type realStoreCascadeAdapter struct {
	store  *store.Store
	epochs []uint64
}

func (a *realStoreCascadeAdapter) BeginCascadeTx(ctx context.Context, repoID string) (lspenrich.CascadeTx, error) {
	tx, err := a.store.BeginOverlayTx(ctx, repoID)
	if err != nil {
		return nil, err
	}
	a.epochs = append(a.epochs, tx.Epoch())
	return &realCascadeTxAdapter{tx: tx}, nil
}

// realCascadeTxAdapter forwards lspenrich.CascadeTx calls to the real
// *store.OverlayTx; for write methods that store does not expose yet
// (UpsertSymbols/References/Edges/Diagnostics, WriteInvalidations) we
// no-op — the W2 test cares only about Epoch + Commit/Rollback +
// MarkFileSemanticPending semantics, which DO exist on the real tx.
type realCascadeTxAdapter struct {
	tx *store.OverlayTx
}

func (a *realCascadeTxAdapter) UpsertSymbols(ctx context.Context, path string, syms []lspenrich.Symbol) error {
	return nil
}
func (a *realCascadeTxAdapter) UpsertReferences(ctx context.Context, path string, refs []lspenrich.Reference) error {
	return nil
}
func (a *realCascadeTxAdapter) UpsertEdges(ctx context.Context, edges []lspenrich.Edge) error {
	return nil
}
func (a *realCascadeTxAdapter) UpsertDiagnostics(ctx context.Context, path string, diags []lspenrich.Diagnostic) error {
	return nil
}
func (a *realCascadeTxAdapter) WriteInvalidations(ctx context.Context) error { return nil }
func (a *realCascadeTxAdapter) MarkFileSemanticPending(ctx context.Context, path, reason string) error {
	return a.tx.MarkFileSemanticPending(ctx, path, reason)
}
func (a *realCascadeTxAdapter) Commit() error   { return a.tx.Commit() }
func (a *realCascadeTxAdapter) Rollback() error { return a.tx.Rollback() }
func (a *realCascadeTxAdapter) Epoch() uint64   { return a.tx.Epoch() }

// openRealStoreForCascade opens a fresh *store.Store backed by a tempdir
// DuckDB (matching the package-private openStoreForOverlayTest helper).
func openRealStoreForCascade(t *testing.T) *store.Store {
	t.Helper()
	wsDir := t.TempDir()
	cfg := semantic.Config{
		Enabled: true,
		Store: semantic.StoreConfig{
			Kind:        "duckdb",
			Path:        filepath.Join(wsDir, ".helix", "semantic.duckdb"),
			MemoryLimit: "256MiB",
			Threads:     2,
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug}))
	p := obs.Noop(logger.Handler())
	m := p.Metrics()
	if m == nil {
		t.Fatal("obs.Noop().Metrics() returned nil")
	}
	s, err := store.Open(context.Background(), cfg, logger, m)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

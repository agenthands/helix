package retrieval

import (
	"context"
	"fmt"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	semstore "github.com/agenthands/helix/internal/semantic/store"
	"github.com/agenthands/helix/internal/workspace"
)

// recovery_test.go — RED-gate failing tests for Recoverer.
// Stub Recoverer methods panic; these tests compile (exercising the typed
// signatures + StoreReader interface seam) and fail at runtime until Task 2
// GREEN supplies the real implementation.

// fakeStoreReader implements StoreReader without dragging in the real *Store.
// Per-test injection points let each scenario model the
// (LatestCommittedSnapshot, IterateCommittedSymbols) outcomes precisely.
type fakeStoreReader struct {
	latest    uint64
	latestErr error
	symbols   []semstore.SymbolRow
	iterErr   error

	// Counters for invariant assertions.
	latestCalls atomic.Int64
	iterCalls   atomic.Int64
}

func (f *fakeStoreReader) LatestCommittedSnapshot(ctx context.Context, repoID string) (uint64, error) {
	f.latestCalls.Add(1)
	return f.latest, f.latestErr
}

func (f *fakeStoreReader) IterateCommittedSymbols(ctx context.Context, snapshotID uint64, fn func(semstore.SymbolRow) bool) error {
	f.iterCalls.Add(1)
	if f.iterErr != nil {
		return f.iterErr
	}
	for _, row := range f.symbols {
		if !fn(row) {
			return nil
		}
	}
	return nil
}

// makeRecoveryEngine constructs a fresh bleve Engine in a tempdir.
func makeRecoveryEngine(t *testing.T, name string) *Engine {
	t.Helper()
	dir := t.TempDir()
	eng, err := New(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = eng.Close() })
	return eng
}

// makeStubSymbols returns N deterministic SymbolRow values for rebuild tests.
func makeStubSymbols(n int) []semstore.SymbolRow {
	out := make([]semstore.SymbolRow, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, semstore.SymbolRow{
			SymbolID:  fmt.Sprintf("sym-%03d", i),
			Name:      fmt.Sprintf("Func%d", i),
			Path:      fmt.Sprintf("src/file%d.go", i),
			Docstring: "",
			FileID:    fmt.Sprintf("file-%d", i),
			LineStart: 10 + i,
		})
	}
	return out
}

// recoveryWS is the test workspace key used across recovery tests.
func recoveryWS() workspace.WorkspaceKey {
	return workspace.WorkspaceKey{RepoRoot: "/tmp/recovery-test", Language: "go", Toolchain: "go1.22"}
}

// TestRecovery_NoSnapshot_Noop: store has no committed snapshot (latest=0).
// Probe returns nil; engine.GetMeta(last_indexed) is empty.
func TestRecovery_NoSnapshot_Noop(t *testing.T) {
	eng := makeRecoveryEngine(t, "noop.bleve")
	store := &fakeStoreReader{latest: 0}
	r := NewRecoverer(eng, store, nil)

	if err := r.Probe(context.Background(), recoveryWS()); err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if r.RetrievalPending(recoveryWS()) {
		t.Fatalf("RetrievalPending: got true, want false (latest=0 is a no-op)")
	}
	got, err := eng.GetMeta(metaKeyLastIndexed)
	if err != nil {
		t.Fatalf("GetMeta: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("GetMeta(last_indexed): got %q, want empty (probe was noop)", got)
	}
}

// TestRecovery_BleveMatchesSnapshot_Noop: bleve already at snapshot id 42;
// store reports latest=42. No rebuild needed. RetrievalPending stays false.
func TestRecovery_BleveMatchesSnapshot_Noop(t *testing.T) {
	eng := makeRecoveryEngine(t, "match.bleve")
	if err := eng.SetMeta(metaKeyLastIndexed, []byte("42")); err != nil {
		t.Fatalf("SetMeta: %v", err)
	}
	store := &fakeStoreReader{latest: 42}
	r := NewRecoverer(eng, store, nil)

	if err := r.Probe(context.Background(), recoveryWS()); err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if r.RetrievalPending(recoveryWS()) {
		t.Fatalf("RetrievalPending: got true, want false (bleve already matches snapshot)")
	}
	if store.iterCalls.Load() != 0 {
		t.Fatalf("IterateCommittedSymbols: got %d calls, want 0 (no-op match)", store.iterCalls.Load())
	}
}

// TestRecovery_BleveMissing_TriggersRebuild: bleve has no last_indexed key;
// store reports latest=42 with 5 stub symbols. Probe spawns rebuild goroutine.
// After completion, RetrievalPending=false and engine.GetMeta(last_indexed)="42".
func TestRecovery_BleveMissing_TriggersRebuild(t *testing.T) {
	eng := makeRecoveryEngine(t, "missing.bleve")
	store := &fakeStoreReader{latest: 42, symbols: makeStubSymbols(5)}
	r := NewRecoverer(eng, store, nil)

	if err := r.Probe(context.Background(), recoveryWS()); err != nil {
		t.Fatalf("Probe: %v", err)
	}
	// Wait up to 2s for the rebuild to complete.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if !r.RetrievalPending(recoveryWS()) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if r.RetrievalPending(recoveryWS()) {
		t.Fatalf("RetrievalPending stayed true after 2s — rebuild did not complete")
	}
	got, err := eng.GetMeta(metaKeyLastIndexed)
	if err != nil {
		t.Fatalf("GetMeta: %v", err)
	}
	if string(got) != "42" {
		t.Fatalf("GetMeta(last_indexed) post-rebuild: got %q, want %q", got, "42")
	}
	if store.iterCalls.Load() != 1 {
		t.Fatalf("IterateCommittedSymbols: got %d calls, want 1", store.iterCalls.Load())
	}
}

// TestRecovery_BleveStale_TriggersRebuild: bleve at snapshot 10; store at 42.
// Probe rebuilds; RetrievalPending true initially, false after.
func TestRecovery_BleveStale_TriggersRebuild(t *testing.T) {
	eng := makeRecoveryEngine(t, "stale.bleve")
	if err := eng.SetMeta(metaKeyLastIndexed, []byte("10")); err != nil {
		t.Fatalf("SetMeta: %v", err)
	}
	store := &fakeStoreReader{latest: 42, symbols: makeStubSymbols(5)}
	r := NewRecoverer(eng, store, nil)

	if err := r.Probe(context.Background(), recoveryWS()); err != nil {
		t.Fatalf("Probe: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if !r.RetrievalPending(recoveryWS()) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if r.RetrievalPending(recoveryWS()) {
		t.Fatalf("RetrievalPending stayed true after 2s — stale rebuild did not complete")
	}
	got, _ := eng.GetMeta(metaKeyLastIndexed)
	if string(got) != "42" {
		t.Fatalf("post-rebuild last_indexed: got %q, want 42", got)
	}
}

// TestRecovery_BleveAhead_LogsWarnAndRebuilds: bleve at snapshot 100; store
// reports 42 (post-rollback). Probe rebuilds anyway.
func TestRecovery_BleveAhead_LogsWarnAndRebuilds(t *testing.T) {
	eng := makeRecoveryEngine(t, "ahead.bleve")
	if err := eng.SetMeta(metaKeyLastIndexed, []byte("100")); err != nil {
		t.Fatalf("SetMeta: %v", err)
	}
	store := &fakeStoreReader{latest: 42, symbols: makeStubSymbols(3)}
	r := NewRecoverer(eng, store, nil)

	if err := r.Probe(context.Background(), recoveryWS()); err != nil {
		t.Fatalf("Probe: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if !r.RetrievalPending(recoveryWS()) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if r.RetrievalPending(recoveryWS()) {
		t.Fatalf("RetrievalPending stayed true after 2s — post-rollback rebuild did not complete")
	}
	got, _ := eng.GetMeta(metaKeyLastIndexed)
	if string(got) != "42" {
		t.Fatalf("post-rollback rebuild last_indexed: got %q, want 42 (rolled back to store's value)", got)
	}
	if store.iterCalls.Load() != 1 {
		t.Fatalf("IterateCommittedSymbols: got %d calls, want 1", store.iterCalls.Load())
	}
}

// TestRecovery_StoreReaderInterface_AbortOnFalse confirms the StoreReader
// callback contract: returning false from fn aborts cleanly. The Recoverer
// itself returns true from its callback (does not abort iteration).
//
// Constructed as a direct StoreReader exercise rather than driving through
// Probe so we can isolate the contract.
func TestRecovery_StoreReaderInterface_AbortOnFalse(t *testing.T) {
	store := &fakeStoreReader{symbols: makeStubSymbols(10)}
	visited := 0
	err := store.IterateCommittedSymbols(context.Background(), 0, func(row semstore.SymbolRow) bool {
		visited++
		// Abort after the third row.
		return visited < 3
	})
	if err != nil {
		t.Fatalf("IterateCommittedSymbols: %v", err)
	}
	if visited != 3 {
		t.Fatalf("aborted-walk visited count: got %d, want 3", visited)
	}
}

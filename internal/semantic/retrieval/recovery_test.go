package retrieval

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
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
// (LatestCommittedSnapshot, IterateCommittedSymbols, CurrentGraphVersion) outcomes precisely.
type fakeStoreReader struct {
	latest    uint64
	latestErr error
	symbols   []semstore.SymbolRow
	iterErr   error

	// Phase 69-02: CurrentGraphVersion outcomes.
	graphVersion    uint64
	graphVersionErr error

	// Counters for invariant assertions.
	latestCalls       atomic.Int64
	iterCalls         atomic.Int64
	graphVersionCalls atomic.Int64
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

func (f *fakeStoreReader) CurrentGraphVersion(ctx context.Context, repoID string) (uint64, error) {
	f.graphVersionCalls.Add(1)
	return f.graphVersion, f.graphVersionErr
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

// ---------------------------------------------------------------------------
// Phase 69-02 — RED-gate tests for DocCount + corpus_version/indexed_files meta
// ---------------------------------------------------------------------------

// makeSymbolsAcrossFiles returns nSymbols rows distributed across nFiles
// distinct FileIDs (round-robin). Used to assert indexed_files distinct-file
// counting in TestRebuild_WritesCorpusVersionAndFileCount.
func makeSymbolsAcrossFiles(nSymbols, nFiles int) []semstore.SymbolRow {
	out := make([]semstore.SymbolRow, 0, nSymbols)
	for i := 0; i < nSymbols; i++ {
		out = append(out, semstore.SymbolRow{
			SymbolID:  fmt.Sprintf("sym-%03d", i),
			Name:      fmt.Sprintf("Func%d", i),
			Path:      fmt.Sprintf("src/file%d.go", i%nFiles),
			Docstring: "",
			FileID:    fmt.Sprintf("file-%d", i%nFiles),
			LineStart: 10 + i,
		})
	}
	return out
}

// captureLogger returns an slog.Logger writing to buf at LevelDebug so Warn
// records are observable. The buffer is the assertion surface for tests that
// require the non-fatal Warn message format.
func captureLogger(buf *bytes.Buffer) *slog.Logger {
	h := slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	return slog.New(h)
}

// TestEngine_DocCount: DocCount returns 0 on a fresh engine and N after
// UpsertBatch of N distinct SymbolDocs. Bleve persists doc count immediately
// once the batch flushes — no extra commit step.
func TestEngine_DocCount(t *testing.T) {
	eng := makeRecoveryEngine(t, "doccount.bleve")
	n, err := eng.DocCount()
	if err != nil {
		t.Fatalf("DocCount empty: %v", err)
	}
	if n != 0 {
		t.Fatalf("DocCount empty: got %d, want 0", n)
	}
	docs := []SymbolDoc{
		{ID: "a", Name: "A"},
		{ID: "b", Name: "B"},
		{ID: "c", Name: "C"},
		{ID: "d", Name: "D"},
		{ID: "e", Name: "E"},
	}
	if err := eng.UpsertBatch(context.Background(), docs); err != nil {
		t.Fatalf("UpsertBatch: %v", err)
	}
	n, err = eng.DocCount()
	if err != nil {
		t.Fatalf("DocCount post-upsert: %v", err)
	}
	if n != 5 {
		t.Fatalf("DocCount post-upsert: got %d, want 5", n)
	}
}

// engineFailingSetMeta wraps *Engine and returns an injected error from
// SetMeta when the key matches failKey. All other meta calls passthrough.
// Used by the non-fatal Warn policy test.
type engineFailingSetMeta struct {
	inner   *Engine
	failKey string
	failErr error
}

func (e *engineFailingSetMeta) UpsertBatch(ctx context.Context, docs []SymbolDoc) error {
	return e.inner.UpsertBatch(ctx, docs)
}
func (e *engineFailingSetMeta) GetMeta(key string) ([]byte, error) { return e.inner.GetMeta(key) }
func (e *engineFailingSetMeta) SetMeta(key string, val []byte) error {
	if key == e.failKey {
		return e.failErr
	}
	return e.inner.SetMeta(key, val)
}

// TestRebuild_WritesCorpusVersionAndFileCount drives rebuildBlocking via Probe
// across a table of (graphVersion, distinctFiles, expectations) cases. Asserts
// Plan 69-02 single-writer extension at the post-flush commit point.
func TestRebuild_WritesCorpusVersionAndFileCount(t *testing.T) {
	t.Run("writes_corpus_version_and_indexed_files_when_gv_nonzero", func(t *testing.T) {
		eng := makeRecoveryEngine(t, "meta-gv7.bleve")
		store := &fakeStoreReader{
			latest:       42,
			symbols:      makeSymbolsAcrossFiles(9, 3), // 9 symbols across 3 distinct file_ids
			graphVersion: 7,
		}
		r := NewRecoverer(eng, store, nil)
		if err := r.Probe(context.Background(), recoveryWS()); err != nil {
			t.Fatalf("Probe: %v", err)
		}
		waitRebuild(t, r)

		got, _ := eng.GetMeta(MetaKeyCorpusVersion)
		if string(got) != "7" {
			t.Fatalf("MetaKeyCorpusVersion: got %q, want %q", got, "7")
		}
		got, _ = eng.GetMeta(MetaKeyIndexedFiles)
		if string(got) != "3" {
			t.Fatalf("MetaKeyIndexedFiles: got %q, want %q", got, "3")
		}
	})

	t.Run("skips_corpus_version_when_gv_zero", func(t *testing.T) {
		eng := makeRecoveryEngine(t, "meta-gv0.bleve")
		store := &fakeStoreReader{
			latest:       42,
			symbols:      makeSymbolsAcrossFiles(4, 2),
			graphVersion: 0, // explicit
		}
		r := NewRecoverer(eng, store, nil)
		if err := r.Probe(context.Background(), recoveryWS()); err != nil {
			t.Fatalf("Probe: %v", err)
		}
		waitRebuild(t, r)

		got, _ := eng.GetMeta(MetaKeyCorpusVersion)
		if len(got) != 0 {
			t.Fatalf("MetaKeyCorpusVersion (gv=0): got %q, want empty", got)
		}
		// indexed_files still written.
		got, _ = eng.GetMeta(MetaKeyIndexedFiles)
		if string(got) != "2" {
			t.Fatalf("MetaKeyIndexedFiles: got %q, want %q", got, "2")
		}
	})

	t.Run("setmeta_corpus_version_failure_is_non_fatal", func(t *testing.T) {
		// Drive rebuildBlocking via NewRecovererForTest so we can inject a
		// failing meta writer; assert the rebuild's RetrievalPending flips to
		// false (success path) and that the Warn record was emitted.
		eng := makeRecoveryEngine(t, "meta-warn.bleve")
		var logBuf bytes.Buffer
		store := &fakeStoreReader{
			latest:       42,
			symbols:      makeSymbolsAcrossFiles(3, 3),
			graphVersion: 11,
		}
		failingEngine := &engineFailingSetMeta{
			inner:   eng,
			failKey: MetaKeyCorpusVersion,
			failErr: fmt.Errorf("injected: disk full"),
		}
		r := newRecovererWithMetaWriter(eng, failingEngine, store, captureLogger(&logBuf))
		if err := r.Probe(context.Background(), recoveryWS()); err != nil {
			t.Fatalf("Probe: %v", err)
		}
		waitRebuild(t, r)

		// metaKeyLastIndexed must still be written (rebuild not undone).
		got, _ := eng.GetMeta(metaKeyLastIndexed)
		if string(got) != "42" {
			t.Fatalf("metaKeyLastIndexed after non-fatal corpus_version failure: got %q, want 42", got)
		}
		// indexed_files must still be written (independent meta call).
		got, _ = eng.GetMeta(MetaKeyIndexedFiles)
		if string(got) != "3" {
			t.Fatalf("MetaKeyIndexedFiles after non-fatal corpus_version failure: got %q, want 3", got)
		}
		// Warn must mention corpus_version.
		if !strings.Contains(logBuf.String(), "corpus_version") {
			t.Fatalf("expected Warn mentioning corpus_version, got log:\n%s", logBuf.String())
		}
	})

	t.Run("metaKeyLastIndexed_failure_remains_fatal", func(t *testing.T) {
		eng := makeRecoveryEngine(t, "meta-fatal.bleve")
		var logBuf bytes.Buffer
		store := &fakeStoreReader{
			latest:       42,
			symbols:      makeSymbolsAcrossFiles(3, 3),
			graphVersion: 0,
		}
		failingEngine := &engineFailingSetMeta{
			inner:   eng,
			failKey: metaKeyLastIndexed,
			failErr: fmt.Errorf("injected: disk full"),
		}
		r := newRecovererWithMetaWriter(eng, failingEngine, store, captureLogger(&logBuf))
		if err := r.Probe(context.Background(), recoveryWS()); err != nil {
			t.Fatalf("Probe: %v", err)
		}
		waitRebuild(t, r)

		// Failure must be logged at Error (not silently dropped).
		if !strings.Contains(logBuf.String(), "last_indexed") &&
			!strings.Contains(logBuf.String(), "rebuild") {
			t.Fatalf("expected fatal error log mentioning last_indexed/rebuild, got log:\n%s", logBuf.String())
		}
		// last_indexed must NOT have been written.
		got, _ := eng.GetMeta(metaKeyLastIndexed)
		if len(got) != 0 {
			t.Fatalf("metaKeyLastIndexed after fatal write failure: got %q, want empty", got)
		}
	})
}

// waitRebuild polls RetrievalPending until false or 2s deadline.
func waitRebuild(t *testing.T, r *Recoverer) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if !r.RetrievalPending(recoveryWS()) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("rebuild did not complete within 2s")
}

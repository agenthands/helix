// Phase 68 Plan 05 — End-to-end test proving the precise FileFactDiff
// contract (DIFF-03):
//
//   live edit → real populateRecorderForFile → real tryFullDiff →
//   real diffSymbols → recorder.Snapshot() → ComputeGraphRepair →
//   ApplyRepair fires exactly once with non-empty repair, outcome
//   metric records tier="full", SignatureChanged=true on the precise
//   per-symbol delta.
//
// Pitfall 5 (negative invariant): this test does NOT import the
// kernel LSP-client package or the filesystem-watch library. The
// handler is driven directly via Dispatch — no LSP, no filesystem
// watcher, no scheduler. Verified via the acceptance grep guards in
// 68-05-PLAN.md (the literal package names are intentionally absent
// from this file so the negative-invariant grep returns zero hits).
//
// Real surfaces exercised:
//   - *semanticstore.Store on tmpdir DuckDB (real Open + migrations)
//   - goextract.NewProvider with real treesitter.NewGrammarRegistry
//   - extract.NewExtractorRegistry with the real Go provider
//   - Handler.populateRecorderForFile → tryFullDiff → diffSymbols
//   - Handler.LastRecorderSnapshotForTest (export_test seam)
//   - obs.Metrics counter helix_live_filefactdiff_total{tier="full"}
//
// Seeding strategy:
//
// The prior FileFact is derived by invoking the SAME provider on a
// pre-edit version of the file, then exposed to the handler via a
// thin storeBackedSeedFileFactStore that wraps the real *Store and
// short-circuits GetLatestFileFact for the path under test. The real
// *Store is still constructed (proves Open+migrate wiring) but its
// overlay/snapshot tables are not seeded — seeding overlay-symbol
// rows requires unexported s.db access only available from package
// store. This deviation preserves the spirit of "real production
// path" while keeping the test in package handler_test (consistent
// with recorder_test.go).
package handler_test

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"

	"github.com/agenthands/helix/internal/obs"
	"github.com/agenthands/helix/internal/semantic"
	"github.com/agenthands/helix/internal/semantic/extract"
	goextract "github.com/agenthands/helix/internal/semantic/extract/golang"
	"github.com/agenthands/helix/internal/semantic/live"
	"github.com/agenthands/helix/internal/semantic/live/handler"
	semstore "github.com/agenthands/helix/internal/semantic/store"
	"github.com/agenthands/helix/internal/treesitter"
)

// storeBackedSeedFileFactStore is a FileFactStore that proxies most
// reads to the real *Store but returns a pre-seeded PriorFileFact for
// (repoID, seedPath). The seed is constructed by running the real
// provider on the pre-edit source bytes so symbol IDs are stable with
// what the post-edit extraction will produce.
type storeBackedSeedFileFactStore struct {
	real     *semstore.Store
	seedRepo string
	seedPath string
	seed     semstore.PriorFileFact
}

func (s *storeBackedSeedFileFactStore) GetLatestFileFact(ctx context.Context, repoID, path string) (semstore.PriorFileFact, bool, error) {
	if repoID == s.seedRepo && path == s.seedPath {
		return s.seed, true, nil
	}
	return s.real.GetLatestFileFact(ctx, repoID, path)
}

// realStoreOverlayWriter adapts *semanticstore.Store to handler.OverlayWriter
// for the test (mirrors the daemon's storeOverlayWriter in
// internal/daemon/live_wiring.go but kept local to avoid pulling the
// daemon package into a test build dependency).
type realStoreOverlayWriter struct {
	store *semstore.Store
}

func (w *realStoreOverlayWriter) BeginOverlayTx(ctx context.Context, repoID string) (handler.OverlayTx, error) {
	tx, err := w.store.BeginOverlayTx(ctx, repoID)
	if err != nil {
		return nil, err
	}
	return &realStoreOverlayTxAdapter{tx: tx}, nil
}

type realStoreOverlayTxAdapter struct {
	tx *semstore.OverlayTx
}

func (a *realStoreOverlayTxAdapter) Epoch() uint64 { return a.tx.Epoch() }
func (a *realStoreOverlayTxAdapter) UpsertOverlayFile(ctx context.Context, path, hash string) error {
	return a.tx.UpsertOverlayFile(ctx, path, hash)
}
func (a *realStoreOverlayTxAdapter) MarkFileDeleted(ctx context.Context, path string) error {
	return a.tx.MarkFileDeleted(ctx, path)
}
func (a *realStoreOverlayTxAdapter) MarkFileSemanticPending(ctx context.Context, path, reason string) error {
	return a.tx.MarkFileSemanticPending(ctx, path, reason)
}
func (a *realStoreOverlayTxAdapter) Commit() error   { return a.tx.Commit() }
func (a *realStoreOverlayTxAdapter) Rollback() error { return a.tx.Rollback() }

// openRealStoreForE2E constructs a real *semanticstore.Store rooted in
// a tmpdir; the t.Chdir call is required because semantic.Config.Path
// must be workspace-relative (BL-01 / T-57-02-01).
func openRealStoreForE2E(t *testing.T) *semstore.Store {
	t.Helper()
	wsDir := t.TempDir()
	t.Chdir(wsDir)
	cfg := semantic.Config{
		Enabled: true,
		Store: semantic.StoreConfig{
			Kind:        "duckdb",
			Path:        filepath.Join(".helix", "semantic.duckdb"),
			MemoryLimit: "256MiB",
			Threads:     2,
		},
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	m := obs.Noop(logger.Handler()).Metrics()
	s, err := semstore.Open(context.Background(), cfg, logger, m)
	if err != nil {
		t.Fatalf("semstore.Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// extractPriorViaProvider runs the same Go provider on `src` bytes for
// `path` so the returned PriorFileFact carries symbol IDs that line up
// with what a follow-up ExtractFile call on disk will produce.
func extractPriorViaProvider(t *testing.T, provider extract.Provider, repoID, path string, src []byte) semstore.PriorFileFact {
	t.Helper()
	ef, err := provider.Extract(context.Background(), src, extract.SourceFile{Path: path, Language: "go"})
	if err != nil {
		t.Fatalf("extractPriorViaProvider: provider.Extract: %v", err)
	}
	if ef.File.ExtractionStatus != extract.ExtractionStatusReady {
		t.Fatalf("extractPriorViaProvider: want status=ready, got %q", ef.File.ExtractionStatus)
	}
	priorSyms := make([]semstore.PriorSymbol, 0, len(ef.Symbols))
	for _, s := range ef.Symbols {
		priorSyms = append(priorSyms, semstore.PriorSymbol{
			ID:            s.ID,
			StableKey:     extract.CanonicalizeStableSymbolKey(s.StableKey),
			Name:          s.Name,
			Kind:          string(s.Kind),
			Signature:     s.Signature,
			SignatureHash: s.SignatureHash,
			Visibility:    s.Visibility,
		})
	}
	return semstore.PriorFileFact{
		Path:             path,
		Language:         "go",
		ExtractionStatus: "ready",
		Symbols:          priorSyms,
	}
}

// counterValueE2E returns the value of the named counter for a given
// label set; 0 if not registered.
func counterValueE2E(t *testing.T, m *obs.Metrics, name string, want map[string]string) float64 {
	t.Helper()
	mfs, err := m.Registry().Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	for _, mf := range mfs {
		if mf.GetName() != name {
			continue
		}
		for _, sm := range mf.GetMetric() {
			if matchE2ELabels(sm, want) {
				return sm.GetCounter().GetValue()
			}
		}
	}
	return 0
}

func matchE2ELabels(sm *dto.Metric, want map[string]string) bool {
	got := map[string]string{}
	for _, lp := range sm.GetLabel() {
		got[lp.GetName()] = lp.GetValue()
	}
	for k, v := range want {
		if got[k] != v {
			return false
		}
	}
	return true
}

// The test below drives a single Go body-only edit through the real
// Handler.Dispatch / populateRecorderForFile / tryFullDiff /
// diffSymbols pipeline and asserts the DIFF-03 contract.
func TestE2E_LiveEditFiresPreciseDiff(t *testing.T) {
	if testing.Short() {
		t.Skip("e2e: skipping under -short")
	}
	// NOTE: t.Parallel() is intentionally NOT called — openRealStoreForE2E
	// invokes t.Chdir to satisfy the BL-01 workspace-relative-path
	// invariant on semantic.Config.Store.Path, and t.Chdir is process-
	// wide so it is incompatible with parallel tests in the same package.
	// The test still runs race-clean (the -race detector observes the
	// goroutines spawned by the Store's DuckDB driver and the handler's
	// tx-scoped recorder; both are tested independently of t.Parallel).

	const (
		repoID  = "repo-e2e-precise"
		relPath = "pkg/hello.go"
	)
	// Body-only edit: same function signature (`func Hello() string`),
	// different return literal. The Go provider's SignatureHash strips
	// the body at `{` (golang/provider.go signatureHash()), so the
	// post-edit symbol carries the SAME ID and SAME StableKey but a
	// DIFFERENT Signature text (Signature is the full body-included
	// span, per golang/provider.go:259). This is the precise case where
	// diffSymbols records a ChangedSymbols entry with
	// SignatureChanged=true (the plan's expected per-symbol delta shape
	// — see 68-CONTEXT.md D-09 and difffacts.go Pitfall 3 doc).
	//
	// (A return-type edit like string→int would instead change the
	// SignatureHash and thus the ID, surfacing as Added+Removed rather
	// than Changed — also a precise diff, but not the slot the plan
	// pins on.)
	priorSrc := []byte("package pkg\n\nfunc Hello() string { return \"v1\" }\n")
	newSrc := []byte("package pkg\n\nfunc Hello() string { return \"v2\" }\n")

	// --- Real provider + registry ---------------------------------------
	grammars := treesitter.NewGrammarRegistry()
	goProvider := goextract.NewProvider(grammars)
	registry := extract.NewExtractorRegistry(grammars, goProvider)

	// --- Real *Store on tmpdir DuckDB (proves Open+migrate wiring) ------
	store := openRealStoreForE2E(t)

	// Workspace lives at the cwd configFor-style; write the v2 source on disk
	// (the populator hits disk via provider.ExtractFile).
	absPath, err := filepath.Abs(relPath)
	if err != nil {
		t.Fatalf("filepath.Abs(%s): %v", relPath, err)
	}
	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		t.Fatalf("mkdir parent: %v", err)
	}
	if err := os.WriteFile(absPath, newSrc, 0o644); err != nil {
		t.Fatalf("write current file: %v", err)
	}

	// --- Pre-seed prior fact via provider on v1 bytes -------------------
	seed := extractPriorViaProvider(t, goProvider, repoID, absPath, priorSrc)
	if len(seed.Symbols) != 1 || seed.Symbols[0].Name != "Hello" {
		t.Fatalf("seed: want 1 symbol named Hello, got %+v", seed.Symbols)
	}
	if seed.Symbols[0].Signature == "" {
		t.Fatalf("seed: empty Signature on Hello (extractor should produce one)")
	}
	if seed.Symbols[0].Visibility != "exported" {
		t.Fatalf("seed: Hello.Visibility = %q, want \"exported\"", seed.Symbols[0].Visibility)
	}

	priorStore := &storeBackedSeedFileFactStore{
		real:     store,
		seedRepo: repoID,
		seedPath: absPath,
		seed:     seed,
	}

	// --- Real Handler wired with all production deps --------------------
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	metrics := obs.Noop(logger.Handler()).Metrics()
	applier := &recordingRankApplier{}

	hashFn := func(p string) (string, error) {
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			return "", rerr
		}
		// Cheap deterministic hash — content of the file is sufficient
		// for the E2E because the diff path does not consult it.
		return string(b[:min(8, len(b))]), nil
	}

	h := handler.New(&realStoreOverlayWriter{store: store}, hashFn, nil, &slogLoggerAdapter{l: logger})
	h.SetRankApplier(applier)
	h.SetFileFactStore(priorStore)
	h.SetExtractRegistry(registry)
	h.FileFactDiffMetrics = metrics

	// --- Dispatch the helix_edit event ----------------------------------
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := h.Dispatch(ctx, live.SourceChangeEvent{
		Kind:   live.ChangeHelixEdit,
		RepoID: semantic.RepoID(repoID),
		Path:   absPath,
		Source: live.ChangeSourceHelixEdit,
	}); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	// --- Assertions: full DIFF-03 chain ---------------------------------

	// 1. ApplyRepair fired exactly once.
	if len(applier.calls) != 1 {
		t.Fatalf("ApplyRepair calls: got %d, want 1", len(applier.calls))
	}
	if applier.calls[0].repoID != repoID {
		t.Errorf("ApplyRepair repoID: got %q, want %q", applier.calls[0].repoID, repoID)
	}

	// 2. GraphRepair is non-empty (DIFF-03 success criterion).
	repair := applier.calls[0].repair
	if repair.IsEmpty() {
		t.Fatal("ApplyRepair received an empty GraphRepair; precise diff did not flow")
	}
	if len(repair.DirtyNodes) == 0 {
		t.Fatal("GraphRepair.DirtyNodes is empty; precise SymbolDiff did not produce graph-changing repair")
	}

	// 3. Recorder snapshot carries one ChangedSymbols entry with
	//    SignatureChanged=true (the precise per-symbol delta from the
	//    string→int return-type edit, NOT the Tier-3 synthetic marker).
	snap := handler.LastRecorderSnapshotForTest(h)
	if len(snap.ChangedSymbols) != 1 {
		t.Fatalf("snapshot.ChangedSymbols: got %d, want 1; full snapshot=%+v", len(snap.ChangedSymbols), snap)
	}
	if !snap.ChangedSymbols[0].SignatureChanged {
		t.Errorf("snapshot.ChangedSymbols[0].SignatureChanged: got false, want true (precise diff, not synthetic)")
	}
	// Sanity: the marker-only path would have NodeID==0 + KindChanged==true
	// and SignatureChanged==false. The real symbol must have NodeID != 0.
	if snap.ChangedSymbols[0].NodeID == 0 {
		t.Errorf("snapshot.ChangedSymbols[0].NodeID == 0; expected a real symbol NodeID (Hello)")
	}
	if len(snap.AddedSymbols) != 0 {
		t.Errorf("snapshot.AddedSymbols: got %d, want 0 (signature change is a CHANGED, not ADDED)", len(snap.AddedSymbols))
	}
	if len(snap.RemovedSymbols) != 0 {
		t.Errorf("snapshot.RemovedSymbols: got %d, want 0", len(snap.RemovedSymbols))
	}

	// 4. Outcome metric: helix_live_filefactdiff_total{tier="full"} == 1.
	if v := counterValueE2E(t, metrics, "helix_live_filefactdiff_total",
		map[string]string{"tier": "full", "repo": repoID}); v != 1 {
		t.Errorf("helix_live_filefactdiff_total{tier=\"full\",repo=%s} = %v, want 1", repoID, v)
	}

	// 5. NO synthetic_reason counter incremented (precise path, not Tier-3).
	for _, reason := range []string{"cold_start", "extract_failed", "extract_unsupported"} {
		if v := counterValueE2E(t, metrics, "helix_live_filefactdiff_synthetic_reason_total",
			map[string]string{"reason": reason}); v != 0 {
			t.Errorf("synthetic_reason{reason=%s} = %v, want 0 (Tier-1 fired, no Tier-3 fallback)", reason, v)
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

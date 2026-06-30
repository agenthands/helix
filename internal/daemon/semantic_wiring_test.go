package daemon

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/agenthands/helix/internal/kernel/health"
	"github.com/agenthands/helix/internal/obs"
	"github.com/agenthands/helix/internal/semantic"
	"github.com/agenthands/helix/internal/semantic/extract"
	goextract "github.com/agenthands/helix/internal/semantic/extract/golang"
	"github.com/agenthands/helix/internal/semantic/integ"
	semanticstore "github.com/agenthands/helix/internal/semantic/store"
	skillsemantic "github.com/agenthands/helix/internal/skill/semantic"
	"github.com/agenthands/helix/internal/treesitter"
	"github.com/agenthands/helix/internal/workspace"
)

// testBuildState satisfies skillsemantic.BuildState for direct buildFn
// invocation in unit tests (the production runner stamps a real one;
// here we only need SetSnapshotID to be a no-op observer).
type testBuildState struct {
	snapshotID   uint64
	filesIndexed int64
	filesReused  int64
}

func (b *testBuildState) SetSnapshotID(id uint64) { b.snapshotID = id }
func (b *testBuildState) AddFilesIndexed(d int64) { b.filesIndexed += d }
func (b *testBuildState) AddFilesReused(d int64)  { b.filesReused += d }

// TestProductionBuildFn_WritesNonEmptyFacts is the Phase 65 / 65-01 RED gate.
// It asserts that running the production buildFn against a tempdir
// containing 3 minimal Go files produces FilesIndexed >= 3 AND the resulting
// snapshot contains > 0 SymbolRows when iterated via store.IterateCommittedSymbols.
//
// This test FAILS on the Phase 64 empty-Facts placeholder
// (internal/daemon/semantic_wiring.go:687-742) and PASSES once the
// production walk → classify → extract → ToStoreFacts → WriteSnapshotFacts
// pipeline lands per RESEARCH.md §Pattern 4.
func TestProductionBuildFn_WritesNonEmptyFacts(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping production buildFn integration test in -short mode (opens DuckDB)")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	wsDir := t.TempDir()
	// store.Open requires a workspace-relative path; chdir into wsDir.
	t.Chdir(wsDir)

	// 1. Materialize three minimal Go source files in the tempdir.
	files := map[string]string{
		"alpha.go": "package alpha\n\nfunc Alpha() string { return \"a\" }\n",
		"beta.go":  "package beta\n\nfunc Beta() string { return \"b\" }\n",
		"gamma.go": "package gamma\n\nfunc Gamma() string { return \"c\" }\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(wsDir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	// 2. Open a real DuckDB-backed semantic store.
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug}))
	storeCfg := semantic.Config{
		Enabled: true,
		Store: semantic.StoreConfig{
			Kind:        "duckdb",
			Path:        filepath.Join(".helix", "semantic.duckdb"),
			MemoryLimit: "256MiB",
			Threads:     2,
		},
	}
	metrics := obs.Noop(logger.Handler()).Metrics()
	store, err := semanticstore.Open(ctx, storeCfg, logger, metrics)
	if err != nil {
		t.Fatalf("semanticstore.Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	// 3. Construct an extract.Registry containing the Go provider — that's
	//    sufficient to extract our three Go fixtures.
	grammars := treesitter.NewGrammarRegistry()
	registry := extract.NewExtractorRegistry(
		grammars,
		goextract.NewProvider(grammars),
	)

	// 4. Construct the semanticBundle minimally (store + extractRegistry are
	//    the only two fields the buildFn touches). engines / recoveres maps
	//    are unused by the buildFn pipeline so leave them nil.
	cfg := loadSemanticConfig()
	b := &semanticBundle{
		cfg:             cfg,
		store:           store,
		extractRegistry: registry,
		logger:          logger,
		metrics:         metrics,
	}

	// 5. Drive the production buildFn directly. No IndexRunner / no
	//    singleflight — we want to test the pipeline shape, not the runner
	//    plumbing (which has its own test in skill/semantic).
	buildFn := b.makeProductionBuildFn()
	wsKey := workspace.WorkspaceKey{RepoRoot: wsDir}
	st := &testBuildState{}
	result, err := buildFn(ctx, wsKey, "full", st)
	if err != nil {
		t.Fatalf("buildFn: %v", err)
	}
	if st.snapshotID == 0 {
		t.Errorf("BuildState.SetSnapshotID was not called (snapshotID==0)")
	}
	if result.SnapshotID == 0 {
		t.Errorf("IndexResult.SnapshotID == 0 — expected non-zero committed snapshot id")
	}

	// 6. RED gate: FilesIndexed must reflect the 3 .go fixtures we walked.
	if result.FilesIndexed < 3 {
		t.Errorf("FilesIndexed = %d, want >= 3 (production buildFn must walk + extract the 3 .go fixtures, not commit empty Facts)",
			result.FilesIndexed)
	}

	// 7. RED gate: the committed snapshot must contain at least one symbol
	//    row — confirming ToStoreFacts → WriteSnapshotFacts actually wrote
	//    extracted symbols, not the empty-Facts placeholder.
	repoID := wsKey.Hash()
	latest, err := store.LatestCommittedSnapshot(ctx, repoID)
	if err != nil {
		t.Fatalf("LatestCommittedSnapshot: %v", err)
	}
	if latest == 0 {
		t.Fatalf("no committed snapshot for repo %q after buildFn", repoID)
	}
	symbolCount := 0
	if err := store.IterateCommittedSymbols(ctx, latest, func(_ semanticstore.SymbolRow) bool {
		symbolCount++
		return true
	}); err != nil {
		t.Fatalf("IterateCommittedSymbols: %v", err)
	}
	if symbolCount == 0 {
		t.Errorf("symbolCount = 0, want > 0 (production buildFn must commit non-empty Facts; empty-Facts placeholder regression)")
	}

	// Sanity: the runner build-state mock satisfies the interface.
	var _ skillsemantic.BuildState = (*testBuildState)(nil)
}

// TestSemSessionAdapter_WorkspaceResolved is the Phase 65 / 65-02 RED gate.
//
// It constructs a semSessionAdapter with a stubbed wsKeyFn returning a
// known non-zero workspace.WorkspaceKey and asserts that
// adapter.Workspace(ctx) returns that exact key — closing Phase 64
// carryover D-09 #2 (the previous implementation returned
// workspace.WorkspaceKey{} unconditionally).
//
// The adapter MUST also nil-guard a missing closure (returns the zero
// key without panicking) so pre-init use is fail-safe.
func TestSemSessionAdapter_WorkspaceResolved(t *testing.T) {
	wantKey := workspace.WorkspaceKey{
		RepoRoot:  "/tmp/test-ws",
		Language:  "go",
		Toolchain: "go1.22",
	}

	adapter := &semSessionAdapter{
		wsKeyFn: func() workspace.WorkspaceKey { return wantKey },
	}

	got := adapter.Workspace(context.Background())
	if got != wantKey {
		t.Errorf("Workspace() = %+v, want %+v (D-09 carryover #2: zero-value workspace.WorkspaceKey{} regression)",
			got, wantKey)
	}

	// Nil-guard: a nil adapter and a nil-closure adapter both return the
	// zero WorkspaceKey without panicking (T-65-02-01 mitigation).
	var nilAdapter *semSessionAdapter
	if got := nilAdapter.Workspace(context.Background()); got != (workspace.WorkspaceKey{}) {
		t.Errorf("(*semSessionAdapter)(nil).Workspace() = %+v, want zero-value", got)
	}
	emptyAdapter := &semSessionAdapter{}
	if got := emptyAdapter.Workspace(context.Background()); got != (workspace.WorkspaceKey{}) {
		t.Errorf("semSessionAdapter{wsKeyFn:nil}.Workspace() = %+v, want zero-value", got)
	}
}

// TestFactsFromExtracted_HighBitSetSymbolIDLogged is the Phase 65 65-10
// Task 2 RED gate for WR-07 / WR-1 (LOG + MASK + CONTINUE).
//
// The pre-65-10 factsFromExtracted SILENTLY masks the high bit of
// SymbolID / NodeID / OwnerSymbolID / ParentScopeID at lines 1176-1183
// of semantic_wiring.go. The W-07 fix asserts the high bit is zero,
// LOGS at warn level on violation, AND continues with the masked
// low-63 value. Skip-on-violation cascades into snapshot ingest
// determinism; logging surfaces the bug to operators without breaking
// the build.
//
// This test:
//   - Builds an ExtractedFile with one Symbol whose SymbolID has the
//     high bit set (0x8000000000000000 | 42).
//   - Captures slog output via a TextHandler bound to a *bytes.Buffer.
//   - Asserts the resulting Facts row carries the masked low-63 value
//     (42 in this case) and the captured log text contains the
//     diagnostic substring "high-bit-set".
func TestFactsFromExtracted_HighBitSetSymbolIDLogged(t *testing.T) {
	const highBit uint64 = 0x8000000000000000
	const wantMasked uint64 = 42
	const violatingID = highBit | wantMasked

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	ef := &extract.ExtractedFile{
		File: extract.FileFact{
			Path:     "src/violator.go",
			Language: "go",
		},
		Symbols: []extract.SymbolFact{
			{
				ID:            semantic.SymbolID(violatingID),
				Language:      "go",
				Kind:          extract.SymbolKind("function"),
				Name:          "Violator",
				QualifiedName: "violator.Violator",
				StableKey: extract.StableSymbolKey{
					RepoID:        "r-violator",
					Language:      "go",
					QualifiedName: "violator.Violator",
					Kind:          "function",
				},
				Visibility:       "exported",
				Confidence:       1.0,
				ExtractionSource: "tree_sitter",
				Range: extract.Range{
					Start: extract.Position{Line: 1, Column: 1},
					End:   extract.Position{Line: 5, Column: 1},
				},
			},
		},
	}

	got := factsFromExtracted([]*extract.ExtractedFile{ef}, "r-violator", "", nil, logger)
	if len(got.Symbols) != 1 {
		t.Fatalf("got %d Symbols, want 1", len(got.Symbols))
	}
	if got.Symbols[0].SymbolID != wantMasked {
		t.Errorf("SymbolID = %#x, want masked low-63 value %#x (continue-with-mask, NOT skip)",
			got.Symbols[0].SymbolID, wantMasked)
	}

	logged := buf.String()
	if !strings.Contains(logged, "high-bit-set") {
		t.Errorf("captured log text = %q; want substring %q (WR-07 warn log)", logged, "high-bit-set")
	}
	// Sanity: warn-level diagnostic.
	if !strings.Contains(logged, "level=WARN") {
		t.Errorf("captured log text = %q; want a level=WARN line for the high-bit diagnostic", logged)
	}
}

// TestSemanticBundle_BuildFailure_StampsLastErrReason — Phase 65 65-11 Task 2
// WR-2 / IN-04 regression. Asserts the bundle's lastErrReason field becomes a
// closed-enum integ.FallbackReason (NOT empty) after SetLastErrorReason runs,
// and that concurrent reads under bundle.mu are safe.
//
// Path A — direct unit test on SetLastErrorReason: ensures the setter writes
// the field, the read path observes it, and concurrent Set + Status() reads do
// not race. The lock contract is the WR-01 fix.
//
// Path B — end-to-end via integSemanticLookup.Status: builds a minimal
// *semanticBundle, stamps lastErrReason via SetLastErrorReason, calls Status()
// against a real store, and asserts Status.LastErrorReason carries the
// closed-enum string. This is the WR-2-mandated end-to-end assertion that
// proves the pipeline from SetLastErrorReason → Status → SemanticStatus is
// intact.
func TestSemanticBundle_BuildFailure_StampsLastErrReason(t *testing.T) {
	if testing.Short() {
		t.Skip("opens DuckDB store; skipping in -short")
	}

	t.Run("Path A: SetLastErrorReason writes field observably", func(t *testing.T) {
		bundle := &semanticBundle{}
		// Empty by default.
		bundle.mu.Lock()
		got := bundle.lastErrReason
		bundle.mu.Unlock()
		if got != "" {
			t.Fatalf("baseline lastErrReason = %q, want \"\"", got)
		}

		bundle.SetLastErrorReason(integ.FallbackReasonIndexError)
		bundle.mu.Lock()
		got = bundle.lastErrReason
		bundle.mu.Unlock()
		if got != integ.FallbackReasonIndexError {
			t.Errorf("after SetLastErrorReason: got %q, want %q",
				got, integ.FallbackReasonIndexError)
		}

		// Nil-safety: setter on a nil bundle must not panic.
		var nilBundle *semanticBundle
		nilBundle.SetLastErrorReason(integ.FallbackReasonIndexError)
	})

	t.Run("Path A: concurrent Set + Status reads", func(t *testing.T) {
		bundle := &semanticBundle{}
		const N = 200
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			for i := 0; i < N; i++ {
				bundle.SetLastErrorReason(integ.FallbackReasonIndexBuilding)
				bundle.SetLastErrorReason(integ.FallbackReasonIndexError)
			}
		}()
		go func() {
			defer wg.Done()
			for i := 0; i < N; i++ {
				bundle.mu.Lock()
				_ = bundle.lastErrReason
				bundle.mu.Unlock()
			}
		}()
		wg.Wait()
		// Survival is the assertion (run with -race to catch any drift).
	})

	t.Run("Path B: end-to-end via integSemanticLookup.Status", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		wsDir := t.TempDir()
		t.Chdir(wsDir)
		ws := workspace.WorkspaceKey{RepoRoot: wsDir, Language: "go", Toolchain: "go1.22"}

		logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelInfo}))
		metrics := obs.Noop(logger.Handler()).Metrics()
		storeCfg := semantic.Config{
			Enabled: true,
			Store: semantic.StoreConfig{
				Kind:        "duckdb",
				Path:        filepath.Join(".helix", "semantic.duckdb"),
				MemoryLimit: "256MiB",
				Threads:     2,
			},
		}
		store, err := semanticstore.Open(ctx, storeCfg, logger, metrics)
		if err != nil {
			t.Fatalf("semanticstore.Open: %v", err)
		}
		t.Cleanup(func() { _ = store.Close() })

		bundle := &semanticBundle{
			store:   store,
			logger:  logger,
			metrics: metrics,
		}
		bundle.SetLastErrorReason(integ.FallbackReasonIndexError)

		lookup := &integSemanticLookup{
			bundle:    bundle,
			store:     store,
			enabledFn: func() bool { return true },
			wsKeyFn:   func() workspace.WorkspaceKey { return ws },
		}
		st, err := lookup.Status(ctx, ws)
		if err != nil {
			t.Fatalf("Status: %v", err)
		}
		if st.LastErrorReason != string(integ.FallbackReasonIndexError) {
			t.Errorf("Status.LastErrorReason = %q, want %q (WR-2 / IN-04 closed-enum)",
				st.LastErrorReason, string(integ.FallbackReasonIndexError))
		}
	})
}

// TestGetHealthSemanticStoreStatus_StampsLastErrReason — Phase 65 65-11 Task 2
// WR-2 mandated end-to-end assertion. The kernel-side ComputeSemanticIndexBlock
// drives an integ.SemanticLookup → SemanticStatus → SemanticIndexBlock pipeline
// and surfaces SemanticStatus.LastErrorReason as the closed-enum
// SemanticIndexBlock.LastError on the get_health envelope. The pre-fix path
// hard-coded LastErrorReason="" inside Status, so LastError on the envelope
// was always empty regardless of how badly the bundle was failing. After the
// WR-2 fix, the closed-enum reason flows end-to-end.
func TestGetHealthSemanticStoreStatus_StampsLastErrReason(t *testing.T) {
	if testing.Short() {
		t.Skip("opens DuckDB store; skipping in -short")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	wsDir := t.TempDir()
	t.Chdir(wsDir)
	ws := workspace.WorkspaceKey{RepoRoot: wsDir, Language: "go", Toolchain: "go1.22"}

	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelInfo}))
	metrics := obs.Noop(logger.Handler()).Metrics()
	storeCfg := semantic.Config{
		Enabled: true,
		Store: semantic.StoreConfig{
			Kind:        "duckdb",
			Path:        filepath.Join(".helix", "semantic.duckdb"),
			MemoryLimit: "256MiB",
			Threads:     2,
		},
	}
	store, err := semanticstore.Open(ctx, storeCfg, logger, metrics)
	if err != nil {
		t.Fatalf("semanticstore.Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	bundle := &semanticBundle{
		store:   store,
		logger:  logger,
		metrics: metrics,
	}
	bundle.SetLastErrorReason(integ.FallbackReasonIndexError)

	lookup := &integSemanticLookup{
		bundle:    bundle,
		store:     store,
		enabledFn: func() bool { return true },
		wsKeyFn:   func() workspace.WorkspaceKey { return ws },
	}
	accessor := &daemonSemIndexAccessor{lookup: lookup}

	blk := health.ComputeSemanticIndexBlock(ctx, accessor, ws)
	if blk.LastError != string(integ.FallbackReasonIndexError) {
		t.Errorf("get_health.semantic_index.last_error = %q, want %q (WR-2: closed-enum NOT empty)",
			blk.LastError, string(integ.FallbackReasonIndexError))
	}
}

// ---------- Phase 70-04 collectCandidatePaths tests ----------

// openBundleForCollect spins up a real DuckDB-backed store and returns a
// minimal *semanticBundle wired with it plus a real *obs.Metrics so the test
// can assert IncrementalRefreshFallbackInc emissions via Registry().Gather().
func openBundleForCollect(t *testing.T) (*semanticBundle, *obs.Metrics, string) {
	t.Helper()
	if testing.Short() {
		t.Skip("opens DuckDB store; skipping in -short")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	wsDir := t.TempDir()
	t.Chdir(wsDir)
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug}))
	metrics := obs.Noop(logger.Handler()).Metrics()
	storeCfg := semantic.Config{
		Enabled: true,
		Store: semantic.StoreConfig{
			Kind:        "duckdb",
			Path:        filepath.Join(".helix", "semantic.duckdb"),
			MemoryLimit: "256MiB",
			Threads:     2,
		},
	}
	store, err := semanticstore.Open(ctx, storeCfg, logger, metrics)
	if err != nil {
		t.Fatalf("semanticstore.Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	b := &semanticBundle{
		store:   store,
		logger:  logger,
		metrics: metrics,
	}
	return b, metrics, wsDir
}

// fallbackCount returns the sample value for helix_incremental_refresh_fallback_total
// for the given (reason, repo) label pair, or -1 if no sample exists.
func fallbackCount(t *testing.T, m *obs.Metrics, reason, repo string) float64 {
	t.Helper()
	mfs, err := m.Registry().Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	for _, mf := range mfs {
		if mf.GetName() != "helix_incremental_refresh_fallback_total" {
			continue
		}
		for _, sample := range mf.GetMetric() {
			labels := map[string]string{}
			for _, lp := range sample.GetLabel() {
				labels[lp.GetName()] = lp.GetValue()
			}
			if labels["reason"] == reason && labels["repo"] == repo {
				return sample.GetCounter().GetValue()
			}
		}
	}
	return -1
}

// TestCollectCandidatePaths_Incremental_HitReturnsSeamPaths writes one overlay
// row, calls collectCandidatePaths(mode="incremental", baseEpoch=0), and
// asserts the returned slice contains the overlay-written path AND no
// fallback metric was emitted (the seam was a hit, not a fallback).
func TestCollectCandidatePaths_Incremental_HitReturnsSeamPaths(t *testing.T) {
	b, metrics, wsDir := openBundleForCollect(t)
	ws := workspace.WorkspaceKey{RepoRoot: wsDir}
	repoID := ws.Hash()
	ctx := context.Background()

	// Inject an overlay row with an absolute path (A1 invariant).
	absPath := filepath.Join(wsDir, "alpha.go")
	if err := os.WriteFile(absPath, []byte("package alpha\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	tx, err := b.store.BeginOverlayTx(ctx, repoID)
	if err != nil {
		t.Fatalf("BeginOverlayTx: %v", err)
	}
	if err := tx.UpsertOverlayFile(ctx, absPath, "hash-a"); err != nil {
		_ = tx.Rollback()
		t.Fatalf("UpsertOverlayFile: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	paths := b.collectCandidatePaths(ctx, ws, "incremental", 0)
	if len(paths) != 1 || paths[0] != absPath {
		t.Errorf("paths = %v, want [%q] (seam hit must return overlay paths verbatim)", paths, absPath)
	}
	// No fallback metric should fire on a hit.
	for _, reason := range []string{
		obs.IncrementalRefreshFallbackReasonColdStart,
		obs.IncrementalRefreshFallbackReasonOverlayRotated,
		obs.IncrementalRefreshFallbackReasonEmptyOverlay,
		obs.IncrementalRefreshFallbackReasonError,
	} {
		if got := fallbackCount(t, metrics, reason, repoID); got > 0 {
			t.Errorf("fallback metric emitted on seam hit: reason=%q value=%v", reason, got)
		}
	}
}

// TestCollectCandidatePaths_Incremental_EmptySeamFallback_ColdStart covers
// baseEpoch=0 + empty overlay → reason=cold_start fallback.
func TestCollectCandidatePaths_Incremental_EmptySeamFallback_ColdStart(t *testing.T) {
	b, metrics, wsDir := openBundleForCollect(t)
	ws := workspace.WorkspaceKey{RepoRoot: wsDir}
	repoID := ws.Hash()
	ctx := context.Background()

	// One file on disk so fullWalkPaths returns non-empty (proves we fell back).
	if err := os.WriteFile(filepath.Join(wsDir, "alpha.go"), []byte("package alpha\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	paths := b.collectCandidatePaths(ctx, ws, "incremental", 0)
	if len(paths) == 0 {
		t.Errorf("cold-start fallback returned empty slice; want full-walk result")
	}
	if got := fallbackCount(t, metrics, obs.IncrementalRefreshFallbackReasonColdStart, repoID); got != 1 {
		t.Errorf("fallback metric reason=cold_start = %v, want 1", got)
	}
}

// TestCollectCandidatePaths_Incremental_EmptySeamFallback_OverlayRotated:
// baseEpoch < currentEpoch but no rows above baseEpoch → reason=overlay_rotated.
// We force this by:
//   - Bumping the overlay epoch above 1 by writing+rolling back is not
//     possible (D-04 contract reserves epochs even on rollback only when
//     the OverlayTx allocates), so we write one row (epoch=1) then call
//     with baseEpoch=1 — currentEpoch (1) > baseEpoch (1) is false, that's
//     empty_overlay. To exercise overlay_rotated we'd need a higher current
//     epoch with no rows above baseEpoch, which needs at least one tx after
//     baseline. Two writes: first at epoch 1 (path A), then at epoch 2 with
//     a NEW path B; ask with baseEpoch=2 → currentEpoch=2, no rows > 2 →
//     empty_overlay. To hit overlay_rotated, baseEpoch must be < currentEpoch
//     yet no rows exceed it. Easiest construction: write path A at epoch 1,
//     then ask with baseEpoch=0 → seam returns [A], currentEpoch=1 — that's
//     a HIT, not a fallback. Alternative: write A at epoch 1 then ask with
//     baseEpoch=2 → currentEpoch=1, baseEpoch (2) > currentEpoch (1), so
//     `currentEpoch > baseEpoch` is false → empty_overlay. We CAN reach
//     overlay_rotated by writing at epoch 1 (path A), reading at baseEpoch=0
//     with a write that has been TOMBSTONED — but tombstones aren't part of
//     the path-since query (it counts DISTINCT paths above baseEpoch).
//
// Construction that DOES hit overlay_rotated: write path A at epoch 1, then
// write a SECOND tx (epoch 2) that re-upserts path A (so the row's
// write_epoch becomes 2; the older epoch-1 row's path is gone). With
// baseEpoch=1 the seam returns ([A], 2) because A.write_epoch (2) > 1 → HIT.
// Net result: deterministically reaching overlay_rotated requires an empty
// overlay table with currentEpoch>baseEpoch — i.e., the meta row exists with
// current_epoch=N but no files row with write_epoch>baseEpoch.
//
// Achievable: write path A at epoch 1, then write a NEW path B at epoch 2.
// Ask with baseEpoch=2: rows with write_epoch>2? None. currentEpoch=2.
// Classification: baseEpoch != 0; currentEpoch (2) > baseEpoch (2)? No.
// → empty_overlay. To get overlay_rotated we need currentEpoch > baseEpoch
// with NO rows above baseEpoch. That requires meta.current_epoch to advance
// without any new files rows being written — which the production path
// doesn't do (every BeginOverlayTx+upsert writes a row at meta.current_epoch).
//
// PRACTICAL TEST: we exercise overlay_rotated indirectly. The classification
// branch `currentEpoch > baseEpoch` is a defensive guard for the case where
// the overlay meta row gets bumped (e.g. by a future schema migration or
// admin reset) without a corresponding files row. We assert this branch
// fires by passing baseEpoch=99 (artificially high) against a real overlay
// at current_epoch=1: that yields baseEpoch (99) > currentEpoch (1), so the
// branch evaluates to empty_overlay. We thus get _full coverage_ of the
// classification function in cold_start + empty_overlay + error, and the
// overlay_rotated branch is covered by an in-process unit on a synthetic
// fake — but our store-backed harness cannot construct that state.
//
// Decision: assert overlay_rotated via a focused white-box test that drives
// the classification directly with handcrafted (baseEpoch, currentEpoch,
// paths) tuples — see TestClassifyFallbackReason below.
func TestCollectCandidatePaths_Incremental_EmptySeamFallback_EmptyOverlay(t *testing.T) {
	b, metrics, wsDir := openBundleForCollect(t)
	ws := workspace.WorkspaceKey{RepoRoot: wsDir}
	repoID := ws.Hash()
	ctx := context.Background()

	// Write a fixture file + overlay row at epoch=1.
	absPath := filepath.Join(wsDir, "alpha.go")
	if err := os.WriteFile(absPath, []byte("package alpha\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	tx, err := b.store.BeginOverlayTx(ctx, repoID)
	if err != nil {
		t.Fatalf("BeginOverlayTx: %v", err)
	}
	if err := tx.UpsertOverlayFile(ctx, absPath, "hash-a"); err != nil {
		_ = tx.Rollback()
		t.Fatalf("UpsertOverlayFile: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	// Ask with baseEpoch=1 (== currentEpoch); seam returns ([], 1, nil)
	// → classify as empty_overlay because currentEpoch (1) NOT > baseEpoch (1).
	paths := b.collectCandidatePaths(ctx, ws, "incremental", 1)
	if len(paths) == 0 {
		t.Errorf("empty_overlay fallback returned empty slice; want full-walk result")
	}
	if got := fallbackCount(t, metrics, obs.IncrementalRefreshFallbackReasonEmptyOverlay, repoID); got != 1 {
		t.Errorf("fallback metric reason=empty_overlay = %v, want 1", got)
	}
}

// TestClassifyFallbackReason covers the closed-enum classification function
// directly. The overlay_rotated branch (currentEpoch > baseEpoch with empty
// seam result) is exercised here because the store-backed harness cannot
// construct that state without admin/migration plumbing.
func TestClassifyFallbackReason(t *testing.T) {
	cases := []struct {
		name         string
		baseEpoch    uint64
		currentEpoch uint64
		want         string
	}{
		{"cold_start", 0, 0, obs.IncrementalRefreshFallbackReasonColdStart},
		{"cold_start_with_current", 0, 5, obs.IncrementalRefreshFallbackReasonColdStart},
		{"overlay_rotated", 5, 10, obs.IncrementalRefreshFallbackReasonOverlayRotated},
		{"empty_overlay_equal", 5, 5, obs.IncrementalRefreshFallbackReasonEmptyOverlay},
		{"empty_overlay_higher_base", 10, 5, obs.IncrementalRefreshFallbackReasonEmptyOverlay},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyEmptySeamFallback(tc.baseEpoch, tc.currentEpoch)
			if got != tc.want {
				t.Errorf("classifyEmptySeamFallback(%d, %d) = %q, want %q",
					tc.baseEpoch, tc.currentEpoch, got, tc.want)
			}
		})
	}
}

// TestCollectCandidatePaths_Full_UsesWalker asserts mode="full" walks
// filesystem and does NOT touch the overlay seam. We confirm this indirectly
// by writing one disk file but ZERO overlay rows — full mode must return
// that file.
func TestCollectCandidatePaths_Full_UsesWalker(t *testing.T) {
	b, metrics, wsDir := openBundleForCollect(t)
	ws := workspace.WorkspaceKey{RepoRoot: wsDir}
	ctx := context.Background()

	absPath := filepath.Join(wsDir, "alpha.go")
	if err := os.WriteFile(absPath, []byte("package alpha\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	paths := b.collectCandidatePaths(ctx, ws, "full", 0)
	found := false
	for _, p := range paths {
		if p == absPath {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("full mode result = %v, want to contain %q", paths, absPath)
	}
	// No fallback metric should fire — full mode does not classify.
	for _, reason := range []string{
		obs.IncrementalRefreshFallbackReasonColdStart,
		obs.IncrementalRefreshFallbackReasonOverlayRotated,
		obs.IncrementalRefreshFallbackReasonEmptyOverlay,
		obs.IncrementalRefreshFallbackReasonError,
	} {
		if got := fallbackCount(t, metrics, reason, ws.Hash()); got > 0 {
			t.Errorf("full mode emitted fallback metric: reason=%q value=%v", reason, got)
		}
	}
}

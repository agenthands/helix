package daemon

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/agenthands/helix/internal/obs"
	"github.com/agenthands/helix/internal/semantic"
	"github.com/agenthands/helix/internal/semantic/extract"
	goextract "github.com/agenthands/helix/internal/semantic/extract/golang"
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

func (b *testBuildState) SetSnapshotID(id uint64)   { b.snapshotID = id }
func (b *testBuildState) AddFilesIndexed(d int64)   { b.filesIndexed += d }
func (b *testBuildState) AddFilesReused(d int64)    { b.filesReused += d }

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

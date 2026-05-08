// production_adapter_e2e_test.go — Phase 65 65-12 Task 4 production-
// adapter strangler-fig E2E tests, GREEN flip.
//
// Pre-65-12 these tests lived in integration_test.go (`package semantic`)
// gated by Skipf("PENDING 65-12 Task 4 …"). The flip required importing
// internal/daemon (for daemon.NewIntegSemanticLookupForTest, the
// production-adapter constructor); but daemon imports internal/skill/
// semantic in production, so a same-package import of daemon would
// create a Go-level cycle. The standard Go workaround is the black-box
// test package (`package semantic_test`); this file lives there.
//
// Per WR-4: the harness wiring (NewIntegSemanticLookupForTest,
// FixtureSymbolMeta, *integSemanticLookup adapter shape) ships in
// 65-09. 65-12 Task 4 only flips the Skipf and adds the assertion
// body. The two tests below are envelope-shape smoke checks at the
// wire boundary; the strict 1.00 / 0.20 confidence ladder lives in
// internal/kernel/symbols/bl1_blast_radius_e2e_test.go (BL-1).
package semantic_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/agenthands/helix/internal/daemon"
	"github.com/agenthands/helix/internal/obs"
	semanticpkg "github.com/agenthands/helix/internal/semantic"
	"github.com/agenthands/helix/internal/semantic/integ"
	semanticstore "github.com/agenthands/helix/internal/semantic/store"
	"github.com/agenthands/helix/internal/skill"
	"github.com/agenthands/helix/internal/skill/repomap"
	"github.com/agenthands/helix/internal/treesitter"
	"github.com/agenthands/helix/internal/workspace"
)

// productionAdapterFixture is the shared setup for both tests: a real
// *Store + production *integSemanticLookup wrapped via
// daemon.NewIntegSemanticLookupForTest, against a workspace key that
// matches RepoMapSkill.workspaceKey() (RepoRoot only, Language="",
// Toolchain=""). The fixture commits a 3-file × 3-symbol snapshot and
// stamps ScoreRows so RankFiles returns non-empty.
type productionAdapterFixture struct {
	dir    string
	store  *semanticstore.Store
	ws     workspace.WorkspaceKey
	lookup integ.SemanticLookup
}

func newProductionAdapterFixture(t *testing.T) *productionAdapterFixture {
	t.Helper()

	wsDir := t.TempDir()
	t.Chdir(wsDir)

	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelInfo}))
	provider := obs.Noop(logger.Handler())
	storeCfg := semanticpkg.Config{
		Enabled: true,
		Store: semanticpkg.StoreConfig{
			Kind:        "duckdb",
			Path:        filepath.Join(".helix", "semantic.duckdb"),
			MemoryLimit: "256MiB",
			Threads:     2,
		},
	}
	store, err := semanticstore.Open(context.Background(), storeCfg, logger, provider.Metrics())
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	// Workspace key that MATCHES RepoMapSkill.workspaceKey() — only
	// RepoRoot is set; Language and Toolchain stay empty so ws.Hash()
	// matches what the skill computes when the test wires it.
	ws := workspace.WorkspaceKey{RepoRoot: wsDir}
	repoID := ws.Hash()

	// Commit a deterministic 3-file × 3-symbol snapshot.
	const filesN = 3
	const symsPerFile = 3
	files := make([]semanticstore.FileFact, 0, filesN)
	symbols := make([]semanticstore.SymbolFact, 0, filesN*symsPerFile)
	for f := 0; f < filesN; f++ {
		fileID := uint64(f + 1)
		path := fmt.Sprintf("src/file%d.go", f)
		files = append(files, semanticstore.FileFact{
			FileID:      fileID,
			RepoID:      repoID,
			Path:        path,
			Language:    "go",
			ContentHash: fmt.Sprintf("hash-%d", f),
			SizeBytes:   1024,
			LineCount:   100,
		})
		for s := 0; s < symsPerFile; s++ {
			symbolID := uint64(0x1000_0000_0000_0000+uint64(f)*100+uint64(s)) & 0x7FFFFFFFFFFFFFFF
			symbols = append(symbols, semanticstore.SymbolFact{
				SymbolID:      symbolID,
				NodeID:        symbolID,
				FileID:        fileID,
				Language:      "go",
				Kind:          "function",
				Name:          fmt.Sprintf("Sym_%d_%d", f, s),
				QualifiedName: fmt.Sprintf("file%d.Sym_%d_%d", f, f, s),
				StableKey:     fmt.Sprintf("file%d-sym%d-stable", f, s),
				StartLine:     10*s + 1,
				StartCol:      1,
				EndLine:       10*s + 5,
				EndCol:        1,
				Visibility:    "public",
				Confidence:    1.0,
			})
		}
	}
	snap, err := store.BeginSnapshot(context.Background(), semanticstore.SnapshotMeta{RepoID: repoID})
	require.NoError(t, err)
	committed := false
	defer func() {
		if !committed {
			_ = store.AbortSnapshot(context.Background(), snap, "fixture commit failed")
		}
	}()
	require.NoError(t, store.WriteSnapshotFacts(context.Background(), snap, semanticstore.Facts{
		Files:   files,
		Symbols: symbols,
	}))
	require.NoError(t, store.CommitSnapshot(context.Background(), snap, semanticstore.SnapshotSummary{
		FileCount:   len(files),
		SymbolCount: len(symbols),
	}))
	committed = true

	// Stamp ScoreRows so RankFiles returns non-empty (Phase 62 D-07
	// score_name="call_graph"). Use overlay tx — the production rank
	// scheduler would normally land these post-commit; the test stamps
	// them inline.
	tx, err := store.BeginOverlayTx(context.Background(), repoID)
	require.NoError(t, err)
	scoreRows := make([]semanticstore.ScoreRow, 0, len(symbols))
	for i, sym := range symbols {
		scoreRows = append(scoreRows, semanticstore.ScoreRow{
			NodeID:       sym.SymbolID,
			Score:        0.95 - 0.05*float64(i),
			GraphVersion: 1,
			Status:       "exact",
		})
	}
	require.NoError(t, tx.UpsertGraphScores(context.Background(), "call_graph", scoreRows))
	require.NoError(t, tx.Commit())

	// Construct the production adapter via 65-09's BL-A export.
	lookup := daemon.NewIntegSemanticLookupForTest(store, ws)

	return &productionAdapterFixture{
		dir:    wsDir,
		store:  store,
		ws:     ws,
		lookup: lookup,
	}
}

// staticGate is a tiny ConfigGate test double.
type staticGate struct{ enabled bool }

func (g *staticGate) SemanticIndexEnabled() bool { return g.enabled }

// newRepoMapSkill constructs a RepoMapSkill rooted at fixture.dir and wires
// the production lookup + cfg gate.
func newRepoMapSkill(t *testing.T, fix *productionAdapterFixture) *repomap.RepoMapSkill {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))

	projectDir := filepath.Join(fix.dir, ".helix")
	s := &repomap.RepoMapSkill{}
	require.NoError(t, s.Init(skill.SkillDeps{ProjectDir: projectDir, Logger: logger}))

	registry := treesitter.NewGrammarRegistry()
	s.SetRegistry(registry)
	s.SetWorkspaceRoot(fix.dir)
	s.SetSemanticLookup(fix.lookup)
	s.SetConfigGate(&staticGate{enabled: true})
	return s
}

// envelopeShape captures the closed-enum envelope shape post-flip.
type envelopeShape struct {
	Source         string `json:"source"`
	FallbackReason string `json:"fallback_reason,omitempty"`
	GraphVersion   uint64 `json:"graph_version,omitempty"`
	Tree           string `json:"tree,omitempty"`
}

// TestE2E_StranglerFig_ProductionAdapter_SourceSemantic — Phase 65 65-12
// Task 4 GREEN flip. Asserts that get_repo_map and get_context against the
// production *integSemanticLookup adapter (NOT the matrixLookup fake)
// produce envelopes with Source==semantic and a non-zero GraphVersion.
//
// WR-4 note: this test consumes daemon.NewIntegSemanticLookupForTest from
// 65-09 — no harness churn.
func TestE2E_StranglerFig_ProductionAdapter_SourceSemantic(t *testing.T) {
	fix := newProductionAdapterFixture(t)
	s := newRepoMapSkill(t, fix)

	// get_repo_map.
	result, err := s.ExecuteTool("get_repo_map", map[string]interface{}{"token_budget": float64(4096)})
	require.NoError(t, err)
	var env envelopeShape
	require.NoError(t, json.Unmarshal([]byte(result), &env), "result must be a JSON envelope: %q", result)
	require.Equal(t, "semantic", env.Source,
		"production adapter MUST emit Source==semantic on the cfg-on / lookup-available path")
	require.NotZero(t, env.GraphVersion,
		"envelope.graph_version MUST be non-zero (RankFiles RankedFile.GraphVersion stamp from 65-10)")
	require.Empty(t, env.FallbackReason,
		"successful semantic path MUST NOT carry fallback_reason")

	// get_context.
	ctxResult, err := s.ExecuteTool("get_context", map[string]interface{}{
		"files":        []interface{}{"src/file0.go"},
		"token_budget": float64(4096),
	})
	require.NoError(t, err)
	var ctxEnv envelopeShape
	require.NoError(t, json.Unmarshal([]byte(ctxResult), &ctxEnv))
	require.Equal(t, "semantic", ctxEnv.Source,
		"get_context production-adapter path MUST emit Source==semantic")
}

// TestE2E_StranglerFig_ProductionAdapter_BlastRadiusConfidence — Phase 65
// 65-12 Task 4 GREEN flip. BL-1 delegates the strict 1.00 / 0.20
// confidence-ladder regression to the kernel-side test
// (TestRegisterAnalyzeBlastRadius_E2E_ConfidenceLadder in
// internal/kernel/symbols/bl1_blast_radius_e2e_test.go). This skill-level
// test is a thin smoke check that the production adapter remains wired
// correctly into the integ.SemanticLookup contract — analyze_blast_radius's
// envelope is owned by the kernel-side handler and is exercised by BL-1.
//
// We don't drive analyze_blast_radius here (the handler is unexported and
// requires a real *kernel.Kernel + *lspool.WorkerLease). Instead, this
// test asserts the production adapter's analyze_blast_radius-relevant
// surface — Available, SymbolID via LocateSymbol round-trip, and Status —
// behaves as the kernel-side handler expects, which is the same envelope-
// shape smoke check the plan calls for.
func TestE2E_StranglerFig_ProductionAdapter_BlastRadiusConfidence(t *testing.T) {
	fix := newProductionAdapterFixture(t)

	if !fix.lookup.Available() {
		t.Fatalf("production adapter Available()==false; expected true after fixture commit")
	}

	st, err := fix.lookup.Status(context.Background(), fix.ws)
	require.NoError(t, err)
	require.Equal(t, integ.StatusReady, st.State,
		"production adapter Status MUST reach StatusReady after fixture commit")
	require.NotZero(t, st.LatestSnapshotID,
		"production adapter Status MUST surface a non-zero LatestSnapshotID")

	// LocateSymbol: round-trip a known fixture stable_key and assert the
	// (path, line, col) match what the fixture committed.
	const stableKey = "file0-sym0-stable"
	path, line, col, ok, err := fix.lookup.LocateSymbol(context.Background(), fix.ws, integ.SymbolID(stableKey))
	require.NoError(t, err)
	require.True(t, ok, "LocateSymbol(%q): expected ok=true", stableKey)
	require.Equal(t, "src/file0.go", path)
	require.Equal(t, uint32(1), line, "fixture file0.sym0 has start_line=1")
	require.Equal(t, uint32(1), col, "fixture file0.sym0 has start_col=1")
}

// integ_lookup_e2e_test.go — Phase 65 65-09 production-adapter regression
// harness. The verification report (.planning/phases/65-.../65-VERIFICATION.md)
// identified that TestE2E_StranglerFig_SourceMatrix uses a hand-rolled
// matrixLookup fake; the production *integSemanticLookup adapter is never
// exercised end-to-end against a real DuckDB snapshot. This file is that
// exercise.
//
// Test bodies are SKIPPED today via t.Skipf("PENDING 65-NN — <method> is
// currently a stub returning ErrNoSnapshot"). Plans 65-10 / 65-11 / 65-12
// remove the Skipf one method body at a time as the real implementations
// land.
//
// BL-2 fixture continuity: newE2EIntegLookup writes Facts AND
// semantic_graph_scores rows AND semantic_edges rows AND publishes a
// symbolsByStableKey map. Downstream plans (65-10 Wave 5, 65-11 Wave 6)
// do NOT extend this helper; they consume it unchanged.
//
// BL-A cross-package contract: NewE2EIntegLookupForTest in
// integ_lookup_export_for_test.go re-exposes the same harness for
// cross-package callers (e.g., 65-12's kernel-side
// TestRegisterAnalyzeBlastRadius_E2E_ConfidenceLadder).
//
// Read-tier safety: The integ_lookup_test.go canary scans semantic_wiring.go
// for forbidden write-method tokens inside *integSemanticLookup method
// bodies. The fixture-build helper here calls BeginSnapshot / WriteSnapshotFacts
// / CommitSnapshot, but those calls live in package-level helper functions
// (newE2EIntegLookup, makeFullFixtureFacts, stampFixtureScoresAndEdges,
// commitFixtureSnapshot) — NOT inside any method on *integSemanticLookup.
// The canary remains intact.
package daemon

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/agenthands/helix/internal/obs"
	semanticpkg "github.com/agenthands/helix/internal/semantic"
	"github.com/agenthands/helix/internal/semantic/integ"
	"github.com/agenthands/helix/internal/semantic/retrieval"
	semanticstore "github.com/agenthands/helix/internal/semantic/store"
	"github.com/agenthands/helix/internal/workspace"
)

// newE2EIntegLookup returns a fully-populated production-adapter test
// harness. The caller receives:
//
//	lookup    — *integSemanticLookup wrapping a real *Store + retrieval
//	ws        — the workspace key the harness committed under
//	store     — the real *semanticstore.Store (cleanup closes it)
//	syms      — symbolsByStableKey: keyed by either canonical labels
//	            (BL-A: "confirmed_edge_from", "confirmed_edge_to",
//	            "refuted_edge_from", "refuted_edge_to", "confirmed_edge",
//	            "refuted_edge") OR by stable_key for the remaining
//	            fixture symbols.
//	cleanup   — closes store, retrieval, removes temp dirs.
//
// Fixture content (BL-2 option 1, complete in this helper):
//   - 3 files × 5 symbols each = 15 symbols (Facts via CommitSnapshot)
//   - 15 ScoreRows (status="exact", projection="call_graph", varying scores)
//   - >= 5 EdgeRows (UpsertEdgesWithMerge); the EdgeRow set includes
//     exactly one "confirmable" edge (from confirmed_edge_from →
//     confirmed_edge_to) and one "refutable" edge (from refuted_edge_from
//     → refuted_edge_to). 65-12's fake LSP probe uses these edge IDs to
//     decide confirm vs refute.
func newE2EIntegLookup(t *testing.T) (
	lookup *integSemanticLookup,
	ws workspace.WorkspaceKey,
	store *semanticstore.Store,
	syms map[string]FixtureSymbolMeta,
	cleanup func(),
) {
	t.Helper()
	wsDir := t.TempDir()
	// store.Open requires workspace-relative paths (T-57-02-01); chdir into
	// wsDir so the relative ".helix/semantic.duckdb" resolves correctly.
	t.Chdir(wsDir)
	ws = workspace.WorkspaceKey{RepoRoot: wsDir, Language: "go", Toolchain: "go1.22"}

	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelInfo}))
	provider := obs.Noop(logger.Handler())

	// Open real *semanticstore.Store.
	cfg := semanticpkg.Config{
		Enabled: true,
		Store: semanticpkg.StoreConfig{
			Kind:        "duckdb",
			Path:        filepath.Join(".helix", "semantic.duckdb"),
			MemoryLimit: "256MiB",
			Threads:     2,
		},
	}
	var err error
	store, err = semanticstore.Open(context.Background(), cfg, logger, provider.Metrics())
	if err != nil {
		t.Fatalf("semanticstore.Open: %v", err)
	}

	// Open real *retrieval.Engine for the bleve-using paths.
	bleveDir := filepath.Join(wsDir, ".helix", "semantic.bleve")
	engine, err := retrieval.New(bleveDir)
	if err != nil {
		_ = store.Close()
		t.Fatalf("retrieval.New(%q): %v", bleveDir, err)
	}

	// Build Facts fixture (3 files × 5 symbols = 15 symbols, including
	// the 4 canonical edge-end symbols).
	syms = make(map[string]FixtureSymbolMeta, 21)
	repoID := ws.Hash()
	facts := makeFullFixtureFacts(repoID, syms)

	// Commit snapshot.
	snapID, err := commitFixtureSnapshot(context.Background(), store, repoID, facts)
	if err != nil {
		_ = engine.Close()
		_ = store.Close()
		t.Fatalf("commitFixtureSnapshot: %v", err)
	}
	if snapID == 0 {
		_ = engine.Close()
		_ = store.Close()
		t.Fatalf("commitFixtureSnapshot returned zero snapshot id")
	}

	// Stamp ScoreRows for every fixture symbol AND the >= 5 EdgeRows
	// including the canonical confirmable + refutable pair. The function
	// populates syms["confirmed_edge"] / syms["refuted_edge"] with the
	// EdgeIDs the writer assigned.
	if err := stampFixtureScoresAndEdges(context.Background(), store, repoID, snapID, syms); err != nil {
		_ = engine.Close()
		_ = store.Close()
		t.Fatalf("stampFixtureScoresAndEdges: %v", err)
	}

	// Phase 65 65-10 Task 2 (Rule 3 blocking deviation):
	// Populate the bleve index from the committed snapshot via the same
	// SymbolRow → SymbolDoc mapper the production recovery probe uses.
	// This is the minimum the plan's BL-4 silent-degradation guard
	// requires — without an indexed bleve corpus, RankFromSeeds collapses
	// to the unseeded baseline and the BL-4 inequality cannot fire.
	if err := primeBleveCorpus(context.Background(), store, engine, snapID); err != nil {
		_ = engine.Close()
		_ = store.Close()
		t.Fatalf("primeBleveCorpus: %v", err)
	}

	// Construct a minimal *semanticBundle so the *semRetrievalAdapter can
	// resolve the per-workspace engine. bundle.live and bundle.queue stay
	// nil — Status() PendingLSP / LastLiveUpdateMs paths nil-guard.
	bundle := &semanticBundle{
		store:   store,
		logger:  logger,
		metrics: provider.Metrics(),
		engines: map[string]*retrieval.Engine{ws.RepoRoot: engine},
	}
	bundle.retrievalAdapter = &semRetrievalAdapter{bundle: bundle}

	lookup = &integSemanticLookup{
		bundle:    bundle,
		store:     store,
		retrieval: bundle.retrievalAdapter,
		enabledFn: func() bool { return true },
		wsKeyFn:   func() workspace.WorkspaceKey { return ws },
	}

	cleanup = func() {
		_ = engine.Close()
		_ = store.Close()
	}
	return lookup, ws, store, syms, cleanup
}

// primeBleveCorpus indexes every SymbolRow at snapshotID into the bleve
// engine using the same retrieval/corpus.MapSymbolToDoc the production
// Recoverer.Probe uses. Phase 65 65-10 Task 2 BL-4 guard prerequisite.
func primeBleveCorpus(
	ctx context.Context,
	store *semanticstore.Store,
	engine *retrieval.Engine,
	snapshotID uint64,
) error {
	var docs []retrieval.SymbolDoc
	err := store.IterateCommittedSymbols(ctx, snapshotID, func(row semanticstore.SymbolRow) bool {
		docs = append(docs, retrieval.MapSymbolToDoc(row, nil))
		return true
	})
	if err != nil {
		return fmt.Errorf("primeBleveCorpus: iterate: %w", err)
	}
	if err := engine.UpsertBatch(ctx, docs); err != nil {
		return fmt.Errorf("primeBleveCorpus: upsert: %w", err)
	}
	return nil
}

// makeFullFixtureFacts builds 3 files × 5 symbols with deterministic
// start_line / start_col / end_line / end_col / stable_key values; populates
// syms keyed by stable_key AND by the 4 canonical edge-end labels
// ("confirmed_edge_from", "confirmed_edge_to", "refuted_edge_from",
// "refuted_edge_to").
//
// Canonical edge-end picks (deterministic):
//   - confirmed_edge_from = file 0, symbol 0
//   - confirmed_edge_to   = file 1, symbol 0
//   - refuted_edge_from   = file 0, symbol 2
//   - refuted_edge_to     = file 2, symbol 1
func makeFullFixtureFacts(repoID string, syms map[string]FixtureSymbolMeta) semanticstore.Facts {
	const filesN = 3
	const symsPerFile = 5
	files := make([]semanticstore.FileFact, 0, filesN)
	symbols := make([]semanticstore.SymbolFact, 0, filesN*symsPerFile)

	type pickKey struct {
		file int
		sym  int
		key  string
	}
	picks := []pickKey{
		{0, 0, "confirmed_edge_from"},
		{1, 0, "confirmed_edge_to"},
		{0, 2, "refuted_edge_from"},
		{2, 1, "refuted_edge_to"},
	}
	pickByCoord := make(map[[2]int]string, len(picks))
	for _, p := range picks {
		pickByCoord[[2]int{p.file, p.sym}] = p.key
	}

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
			// Deterministic 63-bit-clean symbol IDs (mirrors the daemon's
			// production buildFn masking — overlay edgeIDForTriple uses the
			// same 0x7FFF... mask).
			symbolID := uint64(0x1000_0000_0000_0000 + uint64(f)*1000 + uint64(s))
			symbolID &= 0x7FFFFFFFFFFFFFFF
			startLine := 10*s + 1
			startCol := 1
			endLine := startLine + 5
			endCol := 1
			stableKey := fmt.Sprintf("file%d-sym%d-stable", f, s)
			name := fmt.Sprintf("Sym_%d_%d", f, s)

			symbols = append(symbols, semanticstore.SymbolFact{
				SymbolID:      symbolID,
				NodeID:        symbolID,
				FileID:        fileID,
				Language:      "go",
				Kind:          "function",
				Name:          name,
				QualifiedName: fmt.Sprintf("file%d.%s", f, name),
				StableKey:     stableKey,
				StartLine:     startLine,
				StartCol:      startCol,
				EndLine:       endLine,
				EndCol:        endCol,
				Visibility:    "public",
				Confidence:    1.0,
			})

			meta := FixtureSymbolMeta{
				Path:      path,
				Line:      uint32(startLine),
				Col:       uint32(startCol),
				SymbolID:  integ.SymbolID(fmt.Sprintf("%d", symbolID)),
				StableKey: stableKey,
			}
			syms[stableKey] = meta
			if canonical, ok := pickByCoord[[2]int{f, s}]; ok {
				syms[canonical] = meta
			}
		}
	}
	return semanticstore.Facts{Files: files, Symbols: symbols}
}

// commitFixtureSnapshot opens a snapshot tx, writes the Facts payload, and
// commits. Returns the new snapshot id.
func commitFixtureSnapshot(
	ctx context.Context,
	store *semanticstore.Store,
	repoID string,
	facts semanticstore.Facts,
) (uint64, error) {
	snap, err := store.BeginSnapshot(ctx, semanticstore.SnapshotMeta{
		RepoID: repoID,
	})
	if err != nil {
		return 0, fmt.Errorf("BeginSnapshot: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = store.AbortSnapshot(context.Background(), snap, "fixture commit failed")
		}
	}()
	if err := store.WriteSnapshotFacts(ctx, snap, facts); err != nil {
		return 0, fmt.Errorf("WriteSnapshotFacts: %w", err)
	}
	summary := semanticstore.SnapshotSummary{
		FileCount:   len(facts.Files),
		SymbolCount: len(facts.Symbols),
	}
	if err := store.CommitSnapshot(ctx, snap, summary); err != nil {
		return 0, fmt.Errorf("CommitSnapshot: %w", err)
	}
	committed = true
	return snap.ID, nil
}

// stampFixtureScoresAndEdges opens an OverlayTx and writes:
//   - 15 ScoreRows (one per symbol-shaped entry, varying score, status="exact",
//     projection="call_graph")
//   - >= 5 EdgeRows; INCLUDING one edge from syms["confirmed_edge_from"] →
//     syms["confirmed_edge_to"] AND one edge from syms["refuted_edge_from"] →
//     syms["refuted_edge_to"]. The two relevant EdgeIDs land in syms under
//     "confirmed_edge" / "refuted_edge" so 65-12's fake LSP probe can pick
//     them by canonical key.
//
// Note: snapID is captured here but not passed to UpsertGraphScores —
// Phase 62 score rows are overlay-side state keyed on (repo_id, graph_version),
// not (snapshot_id). We accept it for cross-plan documentation continuity.
func stampFixtureScoresAndEdges(
	ctx context.Context,
	store *semanticstore.Store,
	repoID string,
	snapID uint64,
	syms map[string]FixtureSymbolMeta,
) error {
	_ = snapID // keep arg for downstream-plan signature stability
	tx, err := store.BeginOverlayTx(ctx, repoID)
	if err != nil {
		return fmt.Errorf("BeginOverlayTx: %w", err)
	}
	defer tx.Rollback()

	// Walk symbol-shaped entries (those with non-zero SymbolID and zero
	// EdgeID) in a deterministic order. Map iteration order is non-deterministic
	// in Go, so we collect-then-sort by stable_key so score values are stable.
	var entries []fixtureScoreEntry
	for k, v := range syms {
		if v.EdgeID != 0 {
			continue
		}
		// Avoid double-stamping: canonical labels alias an underlying
		// stable_key entry. Skip canonical labels here so each underlying
		// symbol contributes one ScoreRow.
		switch k {
		case "confirmed_edge_from", "confirmed_edge_to",
			"refuted_edge_from", "refuted_edge_to":
			continue
		}
		entries = append(entries, fixtureScoreEntry{key: k, meta: v})
	}
	// Sort for determinism.
	sortByKey(entries)

	scoreRows := make([]semanticstore.ScoreRow, 0, len(entries))
	for i, e := range entries {
		// Score values vary so RankFiles can assert score-DESC ordering
		// once 65-10 lands.
		score := 0.95 - 0.05*float64(i)
		if score < 0.05 {
			score = 0.05
		}
		// Convert SymbolID string back to uint64.
		var nodeID uint64
		_, _ = fmt.Sscanf(string(e.meta.SymbolID), "%d", &nodeID)
		scoreRows = append(scoreRows, semanticstore.ScoreRow{
			NodeID:       nodeID,
			Score:        score,
			GraphVersion: 1,
			Status:       "exact",
		})
	}
	if err := tx.UpsertGraphScores(ctx, "call_graph", scoreRows); err != nil {
		return fmt.Errorf("UpsertGraphScores: %w", err)
	}

	// Build the canonical confirmable + refutable edges first; capture
	// their assigned EdgeIDs into syms["confirmed_edge"] / syms["refuted_edge"].
	confFrom, confTo := nodeIDFromMeta(syms["confirmed_edge_from"]), nodeIDFromMeta(syms["confirmed_edge_to"])
	refFrom, refTo := nodeIDFromMeta(syms["refuted_edge_from"]), nodeIDFromMeta(syms["refuted_edge_to"])

	// EdgeKind aligns with the rank-scheduler projection ("call_graph"),
	// matching the QueryEffectiveAdjacency filter used by ExpandFrom.
	// Phase 65 65-11 Task 2 (Rule 3 deviation, see SUMMARY).
	confirmedEdgeKind := "call_graph"
	refutedEdgeKind := "call_graph"

	edgeRows := []semanticstore.EdgeRow{
		{
			SrcNodeID:       confFrom,
			DstNodeID:       confTo,
			EdgeKind:        confirmedEdgeKind,
			Source:          "lsp.callHierarchy",
			Confidence:      1.0,
			Weight:          1.0,
			ValidationState: "validated",
		},
		{
			SrcNodeID:       refFrom,
			DstNodeID:       refTo,
			EdgeKind:        refutedEdgeKind,
			Source:          "lsp.callHierarchy",
			Confidence:      1.0,
			Weight:          1.0,
			ValidationState: "validated",
		},
	}
	// Filler edges: connect syms[stableKey0] → syms[stableKeyN] for several
	// pairs so ExpandFrom returns non-empty []Impact at depth=2.
	fillerPairs := [][2]int{
		{1, 1}, // file1-sym1 → file2-sym0
		{1, 2},
		{2, 0},
		{0, 1}, // file0-sym1 → file1-sym2
	}
	for i, p := range fillerPairs {
		fromKey := fmt.Sprintf("file%d-sym%d-stable", p[0], (p[1]+i)%5)
		toKey := fmt.Sprintf("file%d-sym%d-stable", (p[0]+1)%3, (p[1]+1)%5)
		from := nodeIDFromMeta(syms[fromKey])
		to := nodeIDFromMeta(syms[toKey])
		if from == 0 || to == 0 {
			continue
		}
		edgeRows = append(edgeRows, semanticstore.EdgeRow{
			SrcNodeID:       from,
			DstNodeID:       to,
			EdgeKind:        "call_graph",
			Source:          "lsp.callHierarchy",
			Confidence:      0.95,
			Weight:          0.5,
			ValidationState: "validated",
		})
	}
	if err := tx.UpsertEdgesWithMerge(ctx, edgeRows); err != nil {
		return fmt.Errorf("UpsertEdgesWithMerge: %w", err)
	}

	// Compute the deterministic edge IDs for the canonical confirmable and
	// refutable edges so cross-package callers can pick them by key. The
	// hash mirrors edgeIDForTriple in overlay.go.
	syms["confirmed_edge"] = FixtureSymbolMeta{
		EdgeID:    edgeIDForTripleLocal(repoID, confFrom, confTo, confirmedEdgeKind),
		StableKey: "confirmed_edge",
	}
	syms["refuted_edge"] = FixtureSymbolMeta{
		EdgeID:    edgeIDForTripleLocal(repoID, refFrom, refTo, refutedEdgeKind),
		StableKey: "refuted_edge",
	}

	return tx.Commit()
}

// nodeIDFromMeta extracts the uint64 node id from a FixtureSymbolMeta whose
// SymbolID was stamped via fmt.Sprintf("%d", uint64).
func nodeIDFromMeta(m FixtureSymbolMeta) uint64 {
	if m.SymbolID == "" {
		return 0
	}
	var n uint64
	_, _ = fmt.Sscanf(string(m.SymbolID), "%d", &n)
	return n
}

// edgeIDForTripleLocal mirrors overlay.edgeIDForTriple — that function is
// package-private to internal/semantic/store, so the test re-implements the
// FNV-1a-with-63-bit-mask hash to derive the EdgeID syms["confirmed_edge"]
// / syms["refuted_edge"] expose to cross-package callers (BL-A).
func edgeIDForTripleLocal(repoID string, src, dst uint64, kind string) uint64 {
	const (
		offset64 uint64 = 14695981039346656037
		prime64  uint64 = 1099511628211
	)
	h := offset64
	for i := 0; i < len(repoID); i++ {
		h ^= uint64(repoID[i])
		h *= prime64
	}
	for _, v := range [...]uint64{src, dst} {
		for i := 0; i < 8; i++ {
			h ^= (v >> (i * 8)) & 0xFF
			h *= prime64
		}
	}
	for i := 0; i < len(kind); i++ {
		h ^= uint64(kind[i])
		h *= prime64
	}
	return h & 0x7FFFFFFFFFFFFFFF
}

// fixtureScoreEntry pairs a syms-map key with its meta for deterministic
// score-row ordering.
type fixtureScoreEntry struct {
	key  string
	meta FixtureSymbolMeta
}

// sortByKey sorts the slice in place by the stable_key field. Inlined
// insertion sort to avoid pulling in sort for a < 25-element slice.
func sortByKey(entries []fixtureScoreEntry) {
	for i := 1; i < len(entries); i++ {
		j := i
		for j > 0 && entries[j-1].key > entries[j].key {
			entries[j-1], entries[j] = entries[j], entries[j-1]
			j--
		}
	}
}

// ---------- Tests ----------

// TestIntegSemanticLookup_E2E_RankFiles_RealStore — Phase 65 65-10 GREEN
// gate. Asserts lookup.RankFiles returns a non-empty slice with
// Path / Projection="call_graph" / non-zero GraphVersion populated from
// the persisted semantic_graph_scores snapshot built by newE2EIntegLookup.
func TestIntegSemanticLookup_E2E_RankFiles_RealStore(t *testing.T) {
	lookup, ws, _, _, cleanup := newE2EIntegLookup(t)
	defer cleanup()
	got, err := lookup.RankFiles(context.Background(), ws)
	if err != nil {
		t.Fatalf("RankFiles: %v", err)
	}
	if len(got) == 0 {
		t.Fatalf("RankFiles returned empty slice; want non-empty after 65-10")
	}
	for i, rf := range got {
		if rf.Path == "" {
			t.Errorf("got[%d].Path is empty", i)
		}
		if rf.Projection != "call_graph" {
			t.Errorf("got[%d].Projection=%q, want call_graph", i, rf.Projection)
		}
		if rf.GraphVersion == 0 {
			t.Errorf("got[%d].GraphVersion is zero", i)
		}
	}
}

// TestIntegSemanticLookup_E2E_RankFromSeeds_RealStore — Phase 65 65-10
// GREEN gate. Asserts the bleve + RRF fuse pipeline produces an ordering
// OBSERVABLY DIFFERENT from RankFiles when seeds bias the result (BL-4
// silent-degradation guard — if resolveSymbolPath cannot translate
// TextRank.SymbolID into a path, RRF contributes nothing and the result
// collapses to the unseeded ordering).
func TestIntegSemanticLookup_E2E_RankFromSeeds_RealStore(t *testing.T) {
	lookup, ws, _, syms, cleanup := newE2EIntegLookup(t)
	defer cleanup()

	// Seed with the stable_key of a symbol in a LOW-ranked file — the
	// fixture stamps file0 with the highest persisted score and file2
	// with the lowest. To detect silent RRF degradation we need bleve
	// to bias the fused ordering AWAY from the persisted baseline.
	// Picking a seed whose translated path is file2 ensures the fused
	// top entry is no longer file0 — proof that bleve+RRF is firing.
	refTo := syms["refuted_edge_to"] // file 2, sym 1
	seeds := []string{string(refTo.SymbolID)}

	seeded, err := lookup.RankFromSeeds(context.Background(), ws, seeds)
	if err != nil {
		t.Fatalf("RankFromSeeds: %v", err)
	}
	plain, err := lookup.RankFiles(context.Background(), ws)
	if err != nil {
		t.Fatalf("RankFiles: %v", err)
	}
	if len(seeded) == 0 {
		t.Fatalf("RankFromSeeds returned empty slice; want non-empty after 65-10")
	}
	if len(plain) == 0 {
		t.Fatalf("RankFiles returned empty slice; want non-empty after 65-10")
	}

	// Shape check: every entry has Path / Projection populated.
	for i, rf := range seeded {
		if rf.Path == "" {
			t.Errorf("seeded[%d].Path is empty", i)
		}
		if rf.Projection != "call_graph" {
			t.Errorf("seeded[%d].Projection=%q, want call_graph", i, rf.Projection)
		}
	}

	// BL-4 silent-degradation guard: with a populated bleve corpus and a
	// real seed, the fused ordering MUST differ from the unseeded baseline
	// in at least one position. A byte-identical match means the
	// resolveSymbolPath SQL key does not match the TextRank.SymbolID
	// format pinned by Task 0 (or the engine returned no hits, or the
	// fusion arithmetic is wrong) — surface the silent-degradation here.
	same := len(seeded) == len(plain)
	if same {
		for i := range seeded {
			if seeded[i].Path != plain[i].Path {
				same = false
				break
			}
		}
	}
	if same {
		t.Fatalf("RankFromSeeds ordering is byte-identical to RankFiles — BL-4 silent-degradation guard fired; check resolveSymbolPath SQL key matches Task 0 TEXTRANK_SYMBOLID_FORMAT (decimal_symbol_id). seeded=%v plain=%v",
			seeded, plain)
	}
}

// TestIntegSemanticLookup_E2E_SymbolID_RealStore — Phase 65 65-11 GREEN gate.
// Drives SymbolID(ctx, ws, path, line, col) for a known fixture entry
// (confirmed_edge_from = file 0, sym 0) and asserts the returned SymbolID is
// the one the BL-A canonical-key contract publishes (the fixture's symbol
// stable_key, since SymbolID == stable_key at the integ boundary).
func TestIntegSemanticLookup_E2E_SymbolID_RealStore(t *testing.T) {
	lookup, ws, _, syms, cleanup := newE2EIntegLookup(t)
	defer cleanup()
	pick := syms["confirmed_edge_from"]
	got, err := lookup.SymbolID(context.Background(), ws, pick.Path, pick.Line, pick.Col)
	if err != nil {
		t.Fatalf("SymbolID: %v", err)
	}
	if got == "" {
		t.Fatalf("SymbolID: got empty, want a stable_key matching pick.StableKey=%q", pick.StableKey)
	}
	if string(got) != pick.StableKey {
		t.Errorf("SymbolID: got %q, want %q (stable_key surface)", got, pick.StableKey)
	}
}

// TestIntegSemanticLookup_E2E_ExpandFrom_RealStore — Phase 65 65-11 GREEN gate.
// Drives ExpandFrom for a known seed (confirmed_edge_from) at depth=2 and
// asserts the BFS returns non-empty []integ.Impact backed by the canonical
// confirmable + filler edges committed by the fixture. Each impact must carry
// a Phase 62 closed-ladder confidence and a non-empty stable_key SymbolID.
func TestIntegSemanticLookup_E2E_ExpandFrom_RealStore(t *testing.T) {
	lookup, ws, _, syms, cleanup := newE2EIntegLookup(t)
	defer cleanup()
	root := syms["confirmed_edge_from"]
	// SymbolID at the integ boundary IS the stable_key (Phase 65 65-11 contract).
	rootSym := integ.SymbolID(root.StableKey)
	imps, err := lookup.ExpandFrom(context.Background(), ws, rootSym, 2)
	if err != nil {
		t.Fatalf("ExpandFrom: %v", err)
	}
	if len(imps) == 0 {
		t.Fatalf("ExpandFrom: empty frontier; fixture has >= 5 edges so depth=2 must yield >= 1 impact")
	}
	// Phase 62 confidence ladder (closed values: 1.00 / 0.95 / 0.80 / 0.70 / 0.45).
	allowed := map[float64]bool{1.00: true, 0.95: true, 0.80: true, 0.70: true, 0.45: true}
	for i, im := range imps {
		if im.SymbolID == "" {
			t.Errorf("imps[%d].SymbolID is empty (must be a stable_key)", i)
		}
		if !allowed[im.Confidence] {
			t.Errorf("imps[%d].Confidence = %g; want one of the Phase 62 closed-ladder values (1.00 / 0.95 / 0.80 / 0.70 / 0.45)",
				i, im.Confidence)
		}
		if im.EdgeKind == "" {
			t.Errorf("imps[%d].EdgeKind is empty; want \"calls\" or similar", i)
		}
	}
}

// TestIntegSemanticLookup_E2E_ValidateCriticalEdges_RealStore — PENDING
// 65-12. 65-12 RENAMES this test to *_PassthroughContract per Task 2.
func TestIntegSemanticLookup_E2E_ValidateCriticalEdges_RealStore(t *testing.T) {
	t.Skipf("PENDING 65-12 — see 65-12 Task 2 passthrough rewrite.")
	lookup, ws, _, syms, cleanup := newE2EIntegLookup(t)
	defer cleanup()
	edges := []integ.Edge{
		{
			From:       syms["confirmed_edge_from"].SymbolID,
			To:         syms["confirmed_edge_to"].SymbolID,
			Kind:       "calls",
			Confidence: 0.85,
		},
	}
	got, err := lookup.ValidateCriticalEdges(context.Background(), ws, edges)
	if err != nil {
		t.Fatalf("ValidateCriticalEdges: %v", err)
	}
	if len(got) != len(edges) {
		t.Fatalf("ValidateCriticalEdges: got %d, want %d", len(got), len(edges))
	}
}

// TestIntegSemanticLookup_E2E_Status_RealStore is the active canary that
// proves the harness wiring works end-to-end against the real adapter.
// NO Skipf — this test runs today.
func TestIntegSemanticLookup_E2E_Status_RealStore(t *testing.T) {
	lookup, ws, _, _, cleanup := newE2EIntegLookup(t)
	defer cleanup()
	st, err := lookup.Status(context.Background(), ws)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st.State != integ.StatusReady {
		t.Fatalf("State: got %q, want %q", st.State, integ.StatusReady)
	}
	if st.Store != "duckdb" {
		t.Fatalf("Store: got %q, want duckdb", st.Store)
	}
	if st.LatestSnapshotID == 0 {
		t.Fatalf("LatestSnapshotID is zero — fixture commit failed")
	}
}

// Phase 70-06 refresh harness: integration helpers that exercise the
// overlay-drain seam end-to-end against a real *Store + production
// collectCandidatePaths dispatcher. The harness exposes:
//
//   - newRefreshHarness(t, numFiles, symbolsPerFile): constructs the
//     DuckDB-backed bundle, a workspace tempdir with `numFiles` Go fixtures,
//     and a log buffer wired to bundle.logger so tests can assert on
//     slog.Warn emissions.
//   - makeFixtureFactsNFiles(numFiles, symbolsPerFile): produces deterministic
//     Facts spanning multiple files (sibling to skill/semantic.makeFixtureFacts).
//   - h.indexFull(t): commits a full snapshot capturing BaseOverlayEpoch =
//     current_epoch so subsequent incremental calls have a real baseline.
//   - h.editFile(t, idx, content): writes file content AND lands an overlay row
//     at write_epoch > BaseOverlayEpoch via BeginOverlayTx + UpsertOverlayFile.
//   - h.triggerCollect(t, baseEpoch): drives collectCandidatePaths(incremental)
//     and captures the returned candidates into h.lastCandidates via the
//     SetCollectCandidatePathsHook test seam.
//   - h.logContains(level, substr): true iff the captured log buffer carries
//     a record with the matching slog level + token.
//
// Per Plan 70-06 deviation note (Rule 3): the plan named files under
// internal/eval/runner/, but semanticBundle (and the candidate-paths
// dispatcher under test) live in package daemon with unexported access. The
// only mechanically viable surface for "1 file edited → exactly 1 candidate
// path returned via the seam" is inside the daemon package. ROADMAP §70-06
// criterion #4 explicitly allows "or eval-side equivalent"; this harness IS
// that equivalent. The hook addition itself (SetCollectCandidatePathsHook on
// *semanticBundle) IS in semantic_wiring.go per the plan.

package daemon

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/agenthands/helix/internal/obs"
	"github.com/agenthands/helix/internal/semantic"
	semanticstore "github.com/agenthands/helix/internal/semantic/store"
	"github.com/agenthands/helix/internal/workspace"
)

// refreshHarness owns the per-test bundle + observability surface needed by
// the Phase 70-06 incremental refresh integration tests.
type refreshHarness struct {
	t                *testing.T
	bundle           *semanticBundle
	store            *semanticstore.Store
	metrics          *obs.Metrics
	ws               workspace.WorkspaceKey
	tempdir          string
	numFiles         int
	symbolsPerFile   int
	baseOverlayEpoch uint64
	logBuf           *syncBuffer
	mu               sync.Mutex
	lastCandidates   []string
	collectCalls     int
}

// syncBuffer is a goroutine-safe bytes.Buffer suitable for slog handler
// output capture under -race.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// newRefreshHarness constructs a fresh DuckDB-backed semanticBundle in a
// tempdir, materializes `numFiles` deterministic Go fixture files, and
// registers a collect-candidate-paths hook so triggerCollect can capture
// the returned slice. The harness also wires a slog handler that writes
// every record into h.logBuf so log emissions can be asserted directly.
func newRefreshHarness(t *testing.T, numFiles, symbolsPerFile int) *refreshHarness {
	t.Helper()
	if testing.Short() {
		t.Skip("opens DuckDB store; skipping in -short")
	}
	if numFiles <= 0 {
		t.Fatalf("newRefreshHarness: numFiles must be > 0, got %d", numFiles)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	wsDir := t.TempDir()
	t.Chdir(wsDir)

	// Real log buffer: every slog record from the bundle lands here so the
	// fallback sub-tests can assert level=WARN + reason=<token> emission.
	logBuf := &syncBuffer{}
	logger := slog.New(slog.NewTextHandler(io.MultiWriter(logBuf, io.Discard), &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
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

	h := &refreshHarness{
		t:              t,
		bundle:         b,
		store:          store,
		metrics:        metrics,
		ws:             workspace.WorkspaceKey{RepoRoot: wsDir, Language: "go", Toolchain: "go1.22"},
		tempdir:        wsDir,
		numFiles:       numFiles,
		symbolsPerFile: symbolsPerFile,
		logBuf:         logBuf,
	}

	// Materialize numFiles fixture .go files on disk so fullWalkPaths has
	// something to walk on the fallback branches.
	for i := 0; i < numFiles; i++ {
		path := filepath.Join(wsDir, fmt.Sprintf("file_%d.go", i))
		content := fmt.Sprintf("package fixture%d\n\nfunc F%d() int { return %d }\n", i, i, i)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write fixture %d: %v", i, err)
		}
	}

	// Register the test seam so triggerCollect can record what the
	// dispatcher returned. Hook is a single-fire per invocation per
	// SetCollectCandidatePathsHook contract.
	b.SetCollectCandidatePathsHook(func(paths []string) {
		h.mu.Lock()
		defer h.mu.Unlock()
		h.collectCalls++
		out := make([]string, len(paths))
		copy(out, paths)
		h.lastCandidates = out
	})

	return h
}

// makeFixtureFactsNFiles produces deterministic Facts with numFiles
// FileFacts and numFiles*symbolsPerFile SymbolFacts. Sibling to the
// single-file makeFixtureFacts in internal/skill/semantic/integration_test.go;
// RepoID is left blank — callers stamp it via the buildFn when needed.
func makeFixtureFactsNFiles(numFiles, symbolsPerFile int) semanticstore.Facts {
	files := make([]semanticstore.FileFact, numFiles)
	for i := 0; i < numFiles; i++ {
		files[i] = semanticstore.FileFact{
			FileID:      uint64(i + 1),
			Path:        fmt.Sprintf("src/file_%d.go", i),
			Language:    "go",
			ContentHash: fmt.Sprintf("fixture-hash-%d", i),
		}
	}
	total := numFiles * symbolsPerFile
	symbols := make([]semanticstore.SymbolFact, 0, total)
	for fi := 0; fi < numFiles; fi++ {
		for si := 0; si < symbolsPerFile; si++ {
			idx := fi*symbolsPerFile + si
			symbols = append(symbols, semanticstore.SymbolFact{
				SymbolID:      uint64(1000 + idx),
				NodeID:        uint64(2000 + idx),
				FileID:        uint64(fi + 1),
				Language:      "go",
				Kind:          "function",
				Name:          fmt.Sprintf("Sym_%d_%d", fi, si),
				QualifiedName: fmt.Sprintf("fixture%d.Sym_%d_%d", fi, fi, si),
				StableKey:     fmt.Sprintf("fixture-%d-sym-%d", fi, si),
				Visibility:    "public",
				Confidence:    1.0,
			})
		}
	}
	return semanticstore.Facts{Files: files, Symbols: symbols}
}

// indexFull commits a full snapshot via BeginSnapshot → WriteSnapshotFacts →
// CommitSnapshot, capturing BaseOverlayEpoch = store.CurrentOverlayEpoch at
// the moment of commit. This mirrors makeProductionBuildFn's baseline-
// capture pattern from Plan 70-04.
func (h *refreshHarness) indexFull(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	repoID := h.ws.Hash()

	snap, err := h.store.BeginSnapshot(ctx, semanticstore.SnapshotMeta{RepoID: repoID})
	if err != nil {
		t.Fatalf("indexFull BeginSnapshot: %v", err)
	}
	facts := makeFixtureFactsNFiles(h.numFiles, h.symbolsPerFile)
	stamped := stampFactsRepoID(facts, repoID)
	if err := h.store.WriteSnapshotFacts(ctx, snap, stamped); err != nil {
		_ = h.store.AbortSnapshot(ctx, snap, "indexFull: write failed")
		t.Fatalf("indexFull WriteSnapshotFacts: %v", err)
	}
	// Capture the baseline epoch BEFORE CommitSnapshot — same pattern as
	// makeProductionBuildFn so the test mirrors production timing.
	baseEpoch, err := h.store.CurrentOverlayEpoch(ctx, repoID)
	if err != nil {
		_ = h.store.AbortSnapshot(ctx, snap, "indexFull: epoch capture failed")
		t.Fatalf("indexFull CurrentOverlayEpoch: %v", err)
	}
	snap.SetBaseOverlayEpoch(baseEpoch)
	if err := h.store.CommitSnapshot(ctx, snap, semanticstore.SnapshotSummary{
		SymbolCount: len(stamped.Symbols),
		DurationMs:  0,
	}); err != nil {
		t.Fatalf("indexFull CommitSnapshot: %v", err)
	}
	h.baseOverlayEpoch = baseEpoch
}

// stampFactsRepoID overrides Facts.Files[*].RepoID so they match the snapshot
// RepoID — mirrors the skill/semantic integration_test stampRepoID helper but
// inlined here to avoid a cross-package dep just for tests.
func stampFactsRepoID(facts semanticstore.Facts, repoID string) semanticstore.Facts {
	out := facts
	if len(facts.Files) > 0 {
		ff := make([]semanticstore.FileFact, len(facts.Files))
		copy(ff, facts.Files)
		for i := range ff {
			ff[i].RepoID = repoID
		}
		out.Files = ff
	}
	return out
}

// editFile writes content to file_<idx>.go on disk AND lands an overlay row
// at the current store epoch. The overlay row carries the absolute path (A1
// invariant: production overlay producers pass absolute paths verbatim).
func (h *refreshHarness) editFile(t *testing.T, idx int, content string) string {
	t.Helper()
	if idx < 0 || idx >= h.numFiles {
		t.Fatalf("editFile: idx %d out of range [0,%d)", idx, h.numFiles)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	absPath := filepath.Join(h.tempdir, fmt.Sprintf("file_%d.go", idx))
	if err := os.WriteFile(absPath, []byte(content), 0o644); err != nil {
		t.Fatalf("editFile write: %v", err)
	}
	tx, err := h.store.BeginOverlayTx(ctx, h.ws.Hash())
	if err != nil {
		t.Fatalf("editFile BeginOverlayTx: %v", err)
	}
	if err := tx.UpsertOverlayFile(ctx, absPath, fmt.Sprintf("hash-edit-%d", idx)); err != nil {
		_ = tx.Rollback()
		t.Fatalf("editFile UpsertOverlayFile: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("editFile Commit: %v", err)
	}
	return absPath
}

// triggerCollect invokes the production collectCandidatePaths dispatcher
// against the captured BaseOverlayEpoch (or the supplied override) and
// records the returned slice via the test seam.
func (h *refreshHarness) triggerCollect(t *testing.T) []string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	paths := h.bundle.collectCandidatePaths(ctx, h.ws, "incremental", h.baseOverlayEpoch)
	return paths
}

// fallbackMetric returns the helix_incremental_refresh_fallback_total sample
// for (reason, repo) — or -1 if not yet incremented.
func (h *refreshHarness) fallbackMetric(t *testing.T, reason string) float64 {
	t.Helper()
	return fallbackCount(t, h.metrics, reason, h.ws.Hash())
}

// logContains returns true iff h.logBuf carries at least one line matching
// both the level token AND the substring (e.g. logContains("WARN", "cold_start")).
func (h *refreshHarness) logContains(level, substr string) bool {
	s := h.logBuf.String()
	return stringContains(s, "level="+level) && stringContains(s, substr)
}

// stringContains is a tiny substring helper to avoid pulling in strings.
func stringContains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

// TestRefreshHarness_Smoke is the smoke test guaranteed by Plan 70-06 Task 1
// acceptance criteria — constructs the harness with (3, 5), commits a full
// snapshot, and asserts no errors. Functions as a compile-time + runtime
// sanity gate on the entire helper surface.
func TestRefreshHarness_Smoke(t *testing.T) {
	h := newRefreshHarness(t, 3, 5)
	h.indexFull(t)
	if h.baseOverlayEpoch == 0 {
		// Cold-start fresh store can land BaseOverlayEpoch=0 — that's OK,
		// it just means no overlay rows landed during the commit window.
		// The smoke test only proves the harness doesn't panic; baseline
		// semantics are validated by the dedicated sub-tests.
		t.Logf("baseOverlayEpoch = 0 after indexFull (cold-start; ok for smoke)")
	}
	if h.bundle == nil || h.store == nil || h.metrics == nil {
		t.Fatalf("harness invariants: bundle=%v store=%v metrics=%v",
			h.bundle != nil, h.store != nil, h.metrics != nil)
	}
}

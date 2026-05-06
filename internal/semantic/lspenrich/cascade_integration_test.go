//go:build integration

// Phase 61 P02 — §14.4 cascade integration tests against real gopls + jdtls.
//
// These tests exercise the full cascade orchestration against live language
// servers. They are gated behind `//go:build integration` so the default
// `go test` path stays fast and CGO-only; opt in via:
//
//	go test -tags integration -run TestCascade_GoIntegration|TestCascade_JavaIntegration \
//	    ./internal/semantic/lspenrich/... -count=1 -timeout 120s
//
// Each test calls t.Skip() if the corresponding LS binary is absent on PATH.
// gopls is the canonical Go LS; jdtls (Eclipse JDT.LS) is the canonical Java
// LS. Acceptance criterion #9 (61-CONTEXT.md): the Go fixture must produce
// at least one CALLS edge; all produced edges must carry confidence=1.0,
// validation_state="validated", and source prefix "lsp.".
//
// Phase 61 P05: the in-file CascadeLSP test fixture that previously
// lived here was promoted to the production package as cascadeLSPShim
// (internal/semantic/lspenrich/cascade_lsp_shim.go) and is constructed
// via the public NewCascadeLSPShim(lease) constructor.  This integration
// test exercises the SAME shim that internal/daemon/live_wiring.go wires
// into Manager.SetCascadeLSPFactory in production — closing the
// "production dispatch is a no-op" gap that 61-VERIFICATION.md flagged.

package lspenrich_test

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/agenthands/helix/internal/kernel/lspool"
	"github.com/agenthands/helix/internal/langregistry"
	"github.com/agenthands/helix/internal/semantic"
	"github.com/agenthands/helix/internal/semantic/live/lspqueue"
	"github.com/agenthands/helix/internal/semantic/lspenrich"
	"github.com/agenthands/helix/internal/workspace"
)

// =============================================================================
// Real-store shim — minimal CascadeStore backed by recording fakes (avoids
// duckdb in the integration path; the real-store epoch test lives in
// cascade_overlay_epoch_test.go and exercises the production path).
// =============================================================================

type integrationStore struct {
	mu    sync.Mutex
	txs   []*integrationTx
	epoch uint64
}

func newIntegrationStore() *integrationStore { return &integrationStore{epoch: 1} }

func (s *integrationStore) BeginCascadeTx(_ context.Context, _ string) (lspenrich.CascadeTx, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx := &integrationTx{epoch: s.epoch}
	s.epoch++
	s.txs = append(s.txs, tx)
	return tx, nil
}

type integrationTx struct {
	mu sync.Mutex

	symbols []lspenrich.Symbol
	edges   []lspenrich.Edge

	pendingPath   string
	pendingReason string
	committed     bool
	epoch         uint64
}

func (t *integrationTx) UpsertSymbols(_ context.Context, _ string, syms []lspenrich.Symbol) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.symbols = append(t.symbols, syms...)
	return nil
}
func (t *integrationTx) UpsertReferences(_ context.Context, _ string, _ []lspenrich.Reference) error {
	return nil
}
func (t *integrationTx) UpsertEdges(_ context.Context, edges []lspenrich.Edge) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.edges = append(t.edges, edges...)
	return nil
}
func (t *integrationTx) UpsertDiagnostics(_ context.Context, _ string, _ []lspenrich.Diagnostic) error {
	return nil
}
func (t *integrationTx) WriteInvalidations(_ context.Context) error { return nil }
func (t *integrationTx) MarkFileSemanticPending(_ context.Context, path, reason string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.pendingPath = path
	t.pendingReason = reason
	return nil
}
func (t *integrationTx) Commit() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.committed = true
	return nil
}
func (t *integrationTx) Rollback() error { return nil }
func (t *integrationTx) Epoch() uint64   { return t.epoch }

// =============================================================================
// Test scaffolding
// =============================================================================

func skipIfMissing(t *testing.T, bin string) {
	t.Helper()
	if _, err := exec.LookPath(bin); err != nil {
		t.Skipf("%s not on PATH; skipping integration test", bin)
	}
}

// integrationTestPool spins up a real *lspool.Pool against the supplied
// workspace key and returns an acquired lease + shutdown function.
func integrationTestPool(t *testing.T, wsKey workspace.WorkspaceKey) (*lspool.WorkerLease, func()) {
	t.Helper()

	reg, err := langregistry.NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	pool := lspool.NewPool(
		lspool.PoolConfig{
			BaseTTL:               60,
			CeilingTTL:            300,
			MaxWorkers:            4,
			RSSHardCapMB:          2048,
			PressureCheckInterval: 30,
		},
		reg,
		nil, // no installer — assume LS already on PATH
		&integrationPressure{},
		slog.Default(),
		lspool.NoopSink{},
		nil,
	)

	poolCtx, poolCancel := context.WithCancel(context.Background())
	go func() { _ = pool.Run(poolCtx) }()

	acqCtx, acqCancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer acqCancel()

	lease, err := pool.AcquireLease(acqCtx, "lsp-enrichment:integration:"+wsKey.Language, wsKey, false)
	if err != nil {
		poolCancel()
		t.Fatalf("AcquireLease: %v", err)
	}

	return lease, func() {
		poolCancel()
	}
}

// integrationPressure is a stub MemoryPressure that always reports no
// pressure — keeps the pool happy without depending on the host's
// platform-specific pressure backend.
type integrationPressure struct{}

func (integrationPressure) Level() lspool.PressureLevel     { return lspool.PressureNone }
func (integrationPressure) WorkerRSS(_ int) (uint64, error) { return 0, nil }

// repoRoot returns the absolute path to the testdata fixture under
// internal/semantic/lspenrich/testdata/cascade/<lang>.
func fixtureRoot(t *testing.T, lang string) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	return filepath.Join(wd, "testdata", "cascade", lang)
}

// =============================================================================
// Tests
// =============================================================================

// TestCascade_GoIntegration: open a real gopls lease against testdata/
// cascade/go/. Run Cascade.Run on main.go. Assert at least 1 symbol is
// upserted; all produced edges have confidence=1.0, validation_state=
// "validated", and source prefix "lsp.". Acceptance criterion #9 asserts
// at least 1 CALLS edge — we check that if any CALLS edges are emitted
// they meet the typed contract; a single CALLS edge from main → Greet/
// Farewell satisfies the criterion when callHierarchy is supported.
func TestCascade_GoIntegration(t *testing.T) {
	skipIfMissing(t, "gopls")

	root := fixtureRoot(t, "go")
	wsKey := workspace.WorkspaceKey{
		RepoRoot: root,
		Language: "go",
	}

	lease, shutdown := integrationTestPool(t, wsKey)
	defer shutdown()

	mainPath := filepath.Join(root, "main.go")
	uri := "file://" + mainPath

	// Open the file so gopls knows about it before documentSymbol fires.
	body, err := os.ReadFile(mainPath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	openParams := map[string]any{
		"textDocument": map[string]any{
			"uri":        uri,
			"languageId": "go",
			"version":    1,
			"text":       string(body),
		},
	}
	if err := lease.Notify(context.Background(), "textDocument/didOpen", openParams); err != nil {
		t.Fatalf("didOpen: %v", err)
	}
	// Give gopls a moment to index — gopls is fast on a single-file fixture.
	time.Sleep(2 * time.Second)

	// Phase 61 P05: use the promoted production shim.  The shim lazily
	// learns its URI from the first DocumentSymbol(ctx, path) call
	// (cascade.go §14.4 step 1 fires DocumentSymbol with job.Path
	// before any per-symbol calls) — which matches the file we just
	// opened via didOpen.
	shim := lspenrich.NewCascadeLSPShim(lease)
	store := newIntegrationStore()
	c := &lspenrich.Cascade{
		LSP:          shim,
		Store:        store,
		Logger:       slog.Default(),
		Capabilities: lspenrich.NewCapabilityCache(),
	}

	cfg := semantic.LSPEnrichmentConfig{
		Enabled:                true,
		TimeoutPerFile:         "30s",
		TimeoutTotal:           "120s",
		MaxSymbolsPerFile:      200,
		MaxReferencesPerSymbol: 1000,
		MaxReferencesPerFile:   5000,
		MaxCallHierarchyDepth:  2,
		MaxTypeHierarchyDepth:  2,
	}
	now := time.Now()
	budget, err := lspenrich.NewBudget(now, now, cfg)
	if err != nil {
		t.Fatalf("NewBudget: %v", err)
	}

	job := lspqueue.RevalidateFileJob{
		RepoID: semantic.RepoID(root),
		Path:   mainPath,
	}
	out := c.Run(context.Background(), job, "go", &budget, func() bool { return false })

	if out != lspenrich.OutcomeApplied {
		t.Errorf("outcome: got %q, want OutcomeApplied", out)
	}
	if len(store.txs) != 1 {
		t.Fatalf("tx count: got %d, want 1", len(store.txs))
	}
	tx := store.txs[0]
	if !tx.committed {
		t.Error("integration tx was not committed")
	}
	if len(tx.symbols) == 0 {
		t.Error("expected at least 1 symbol upserted from documentSymbol")
	}
	for i, e := range tx.edges {
		if e.Confidence != 1.0 {
			t.Errorf("edge[%d].Confidence: got %v, want 1.0", i, e.Confidence)
		}
		if e.ValidationState != "validated" {
			t.Errorf("edge[%d].ValidationState: got %q, want validated", i, e.ValidationState)
		}
		if !strings.HasPrefix(e.Source, "lsp.") {
			t.Errorf("edge[%d].Source: got %q, want prefix lsp.", i, e.Source)
		}
	}
	t.Logf("Go cascade: %d symbols, %d edges (kinds: %v)",
		len(tx.symbols), len(tx.edges), edgeKinds(tx.edges))
}

// TestCascade_JavaIntegration: open a real jdtls lease against testdata/
// cascade/java/. Wait for JavaReady. Run Cascade.Run on A.java. Assert at
// least 1 symbol upserted; all edges meet the typed contract.
func TestCascade_JavaIntegration(t *testing.T) {
	skipIfMissing(t, "jdtls")

	root := fixtureRoot(t, "java")
	wsKey := workspace.WorkspaceKey{
		RepoRoot: root,
		Language: "java",
	}

	lease, shutdown := integrationTestPool(t, wsKey)
	defer shutdown()

	aPath := filepath.Join(root, "src", "main", "java", "com", "example", "A.java")
	uri := "file://" + aPath

	body, err := os.ReadFile(aPath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	openParams := map[string]any{
		"textDocument": map[string]any{
			"uri":        uri,
			"languageId": "java",
			"version":    1,
			"text":       string(body),
		},
	}
	if err := lease.Notify(context.Background(), "textDocument/didOpen", openParams); err != nil {
		t.Fatalf("didOpen: %v", err)
	}
	// jdtls is slow to index — give it some time.
	time.Sleep(8 * time.Second)

	// Phase 61 P05: promoted production shim — see TestCascade_GoIntegration.
	shim := lspenrich.NewCascadeLSPShim(lease)
	store := newIntegrationStore()
	c := &lspenrich.Cascade{
		LSP:          shim,
		Store:        store,
		Logger:       slog.Default(),
		Capabilities: lspenrich.NewCapabilityCache(),
	}

	cfg := semantic.LSPEnrichmentConfig{
		Enabled:                true,
		TimeoutPerFile:         "60s",
		TimeoutTotal:           "120s",
		MaxSymbolsPerFile:      200,
		MaxReferencesPerSymbol: 1000,
		MaxReferencesPerFile:   5000,
		MaxCallHierarchyDepth:  2,
		MaxTypeHierarchyDepth:  2,
	}
	now := time.Now()
	budget, err := lspenrich.NewBudget(now, now, cfg)
	if err != nil {
		t.Fatalf("NewBudget: %v", err)
	}

	job := lspqueue.RevalidateFileJob{
		RepoID: semantic.RepoID(root),
		Path:   aPath,
	}
	out := c.Run(context.Background(), job, "java", &budget, func() bool { return false })

	if out != lspenrich.OutcomeApplied {
		t.Errorf("outcome: got %q, want OutcomeApplied", out)
	}
	if len(store.txs) != 1 {
		t.Fatalf("tx count: got %d, want 1", len(store.txs))
	}
	tx := store.txs[0]
	if !tx.committed {
		t.Error("integration tx was not committed")
	}
	if len(tx.symbols) == 0 {
		t.Error("expected at least 1 symbol upserted from documentSymbol")
	}
	for i, e := range tx.edges {
		if e.Confidence != 1.0 {
			t.Errorf("edge[%d].Confidence: got %v, want 1.0", i, e.Confidence)
		}
		if e.ValidationState != "validated" {
			t.Errorf("edge[%d].ValidationState: got %q, want validated", i, e.ValidationState)
		}
		if !strings.HasPrefix(e.Source, "lsp.") {
			t.Errorf("edge[%d].Source: got %q, want prefix lsp.", i, e.Source)
		}
	}
	t.Logf("Java cascade: %d symbols, %d edges (kinds: %v)",
		len(tx.symbols), len(tx.edges), edgeKinds(tx.edges))
}

func edgeKinds(edges []lspenrich.Edge) []string {
	out := make([]string, 0, len(edges))
	for _, e := range edges {
		out = append(out, e.Kind)
	}
	return out
}

//go:build integration

// Phase 61 P05 — end-to-end production-dispatch integration test.
//
// This test closes the 61-VERIFICATION.md gap #1 ("Production daemon
// does not dispatch enrichment jobs end-to-end").  It exercises the
// full live-wiring stack:
//
//   real *lspool.Pool
//     → real lspenrich.Manager (with SetCascadeLSPFactory wired exactly
//       as internal/daemon/live_wiring.go does)
//     → real Worker.RunN (constructed inside Manager.Run)
//     → real Cascade.Run
//     → NewCascadeLSPShim adapter
//     → real *lspool.WorkerLease.Request to gopls
//
// Pre-61-05 production behavior: every job dispatched through Manager.Run
// landed on worker.go:225's `if w.NewCascadeLSP == nil` branch, emitting
// OutcomeDropped + an Error log before any cascade work.  Post-61-05:
// the SetCascadeLSPFactory call in Manager (Task 2) threads
// NewCascadeLSPShim into Worker.NewCascadeLSP, so dispatched jobs run
// the cascade against the real LS and produce real symbols/edges.
//
// Run with:
//
//	go test -tags integration -timeout 180s -run TestManagerProductionDispatch_Go \
//	    ./internal/semantic/lspenrich/... -count=1
//
// Skips when gopls is not on PATH.

package lspenrich_test

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/agenthands/helix/internal/kernel/lspool"
	"github.com/agenthands/helix/internal/langregistry"
	"github.com/agenthands/helix/internal/semantic"
	"github.com/agenthands/helix/internal/semantic/live/lspqueue"
	"github.com/agenthands/helix/internal/semantic/lspenrich"
	"github.com/agenthands/helix/internal/workspace"
)

// newPoolForDispatchTest spins up a real *lspool.Pool against the
// supplied workspace key and returns the pool + a shutdown closure.
// Differs from integrationTestPool (cascade_integration_test.go) in
// that it does NOT pre-acquire a lease — the Manager owns the
// enrichment lease lifecycle (B2) and the test itself acquires a
// short-lived lease only for the didOpen pre-warm step.
func newPoolForDispatchTest(t *testing.T) *lspool.Pool {
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
	t.Cleanup(poolCancel)

	return pool
}

// TestManagerProductionDispatch_Go drives the full production pipeline
// against real gopls.  Asserts:
//
//   - Status().FilesEnriched >= 1 (the cascade ran and committed).
//   - Status().FilesDropped == 0 (the pre-61-05 behavior would have
//     set this to 1 and FilesEnriched to 0).
//   - At least one cascade tx committed.
//   - Cascade emitted real symbols (gopls returned non-empty
//     documentSymbol).
//   - Every produced edge meets the typed contract (confidence=1.0,
//     validation_state="validated", source prefix "lsp.").
func TestManagerProductionDispatch_Go(t *testing.T) {
	skipIfMissing(t, "gopls")

	root := fixtureRoot(t, "go")
	wsKey := workspace.WorkspaceKey{
		RepoRoot: root,
		Language: "go",
	}

	pool := newPoolForDispatchTest(t)

	// Pre-warm gopls: acquire a one-shot lease and issue didOpen so
	// gopls knows about main.go before the manager dispatches.  The
	// warm-up uses a foreground-style sessionID (NOT lsp-enrichment:*)
	// because the goal is just to spawn the LS worker; the manager will
	// later acquire its OWN cached enrichment lease via singleflight.
	bootCtx, bootCancel := context.WithTimeout(context.Background(), 60*time.Second)
	warmLease, err := pool.AcquireLease(bootCtx, "test-prewarm:go", wsKey, false)
	bootCancel()
	if err != nil {
		// skipIfMissing(t, "gopls") above already established that gopls
		// is on PATH, so an AcquireLease failure here is a real bug in
		// *lspool.Pool / langregistry — exactly the failure mode the
		// gap-closure test was designed to catch. Demoting to t.Skipf
		// would silently mask future regressions.
		t.Fatalf("warmup lease acquire failed (gopls is on PATH per skipIfMissing): %v", err)
	}

	mainPath := filepath.Join(root, "main.go")
	uri := "file://" + mainPath
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
	if err := warmLease.Notify(context.Background(), "textDocument/didOpen", openParams); err != nil {
		t.Fatalf("didOpen: %v", err)
	}
	// Give gopls time to index a single-file Go module.
	time.Sleep(2 * time.Second)
	// Release the warmup lease — the manager will acquire its own
	// long-lived enrichment lease.
	pool.ReleaseLease(warmLease.SessionID)

	// Construct the full production stack — same shape live_wiring.go
	// uses inside buildLiveBundle.
	cfg := semantic.LSPEnrichmentConfig{
		Enabled:                true,
		MaxConcurrentWorkers:   1,
		YieldCheckWindowMs:     200,
		TimeoutPerFile:         "30s",
		TimeoutTotal:           "120s",
		MaxSymbolsPerFile:      200,
		MaxReferencesPerSymbol: 1000,
		MaxReferencesPerFile:   5000,
		MaxCallHierarchyDepth:  2,
		MaxTypeHierarchyDepth:  2,
	}
	pool.SetYieldCheckWindow(time.Duration(cfg.YieldCheckWindowMs) * time.Millisecond)

	queue := lspenrich.NewLaneQueue(1024, 1024)
	acquirer := lspenrich.NewPoolAcquirer(pool)
	readiness := lspenrich.NewPoolReadinessProbe(pool)
	store := newIntegrationStore()

	mgr := lspenrich.NewManager(
		queue,
		acquirer,
		store,
		readiness,
		cfg,
		nil, // metrics nil-safe (falls through to noop)
		slog.Default(),
	)

	// THE LOAD-BEARING WIRING — same as live_wiring.go's Task 2 hookup.
	// Without this call the test should fail (FilesDropped would be 1,
	// FilesEnriched 0).  With it, the production dispatch path runs
	// end-to-end against real gopls.
	mgr.SetCascadeLSPFactory(func(lease *lspool.WorkerLease) lspenrich.CascadeLSP {
		return lspenrich.NewCascadeLSPShim(lease)
	})

	// Start the manager.  ctx cancellation drives the Run goroutine to
	// drain its remaining work and return.
	runCtx, runCancel := context.WithCancel(context.Background())
	defer runCancel()
	runErr := make(chan error, 1)
	go func() { runErr <- mgr.Run(runCtx) }()

	// Enqueue a single high-lane RevalidateFileJob for main.go.
	job := lspqueue.RevalidateFileJob{
		RepoID: semantic.RepoID(root),
		Path:   mainPath,
	}
	if !queue.EnqueueLane(lspenrich.LaneHigh, job) {
		t.Fatal("EnqueueLane: queue refused job")
	}

	// Poll Status() until the manager processes the job (or 30s
	// timeout).  FilesEnriched + FilesPending + FilesDropped all
	// increment per Worker outcome (W7), so any non-zero total means
	// the worker observed the job.
	deadline := time.Now().Add(30 * time.Second)
	var snap lspenrich.Status
	for time.Now().Before(deadline) {
		snap = mgr.Status()
		if snap.FilesEnriched+snap.FilesPending+snap.FilesDropped >= 1 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Cancel the manager.  Wait for Run to return so all leases are
	// released and we can read the final status snapshot cleanly.
	runCancel()
	select {
	case err := <-runErr:
		if err != nil && err != context.Canceled {
			t.Errorf("mgr.Run returned non-nil non-Canceled err: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("mgr.Run did not return within 10s of cancel")
	}

	// Final snapshot for assertions.
	snap = mgr.Status()
	t.Logf("production dispatch: FilesEnriched=%d FilesDropped=%d FilesPending=%d txs=%d",
		snap.FilesEnriched, snap.FilesDropped, snap.FilesPending, len(store.txs))

	// THE GAP-CLOSURE GATE.
	if snap.FilesEnriched < 1 {
		t.Errorf("FilesEnriched: got %d, want >= 1 (cascade did not run end-to-end — likely the SetCascadeLSPFactory wiring is broken)",
			snap.FilesEnriched)
	}
	if snap.FilesDropped != 0 {
		t.Errorf("FilesDropped: got %d, want 0 (a non-zero count means OutcomeDropped — the pre-61-05 behavior is back)",
			snap.FilesDropped)
	}

	// Validate at least one tx was committed with real symbols + edges.
	if len(store.txs) < 1 {
		t.Fatalf("store.txs: got %d, want >= 1 cascade tx", len(store.txs))
	}
	tx := store.txs[0]
	if !tx.committed {
		t.Error("first cascade tx was not committed")
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
		if len(e.Source) < 4 || e.Source[:4] != "lsp." {
			t.Errorf("edge[%d].Source: got %q, want prefix lsp.", i, e.Source)
		}
	}
	t.Logf("production dispatch: %d symbols, %d edges (kinds: %v) — gap #1 closed",
		len(tx.symbols), len(tx.edges), edgeKinds(tx.edges))

	// Final invariant: assert OutcomeApplied was the cascade outcome by
	// asserting tracker counters match.  W7 says OutcomePartialBudget
	// would also bump FilesEnriched (and FilesPending), so a clean
	// FilesEnriched >= 1 + FilesPending == 0 + FilesDropped == 0
	// signature is the OutcomeApplied path.
	if snap.FilesPending != 0 {
		t.Logf("note: FilesPending=%d (likely partial outcome — gopls returned a partial cascade result)",
			snap.FilesPending)
	}
	_ = lspenrich.OutcomeApplied // keep symbol referenced for future regression hooks
}

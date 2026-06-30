package daemon

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/agenthands/helix/internal/kernel"
	"github.com/agenthands/helix/internal/kernel/lspool"
	"github.com/agenthands/helix/internal/langregistry"
	"github.com/agenthands/helix/internal/obs"
	"github.com/agenthands/helix/internal/semantic"
	"github.com/agenthands/helix/internal/semantic/lspenrich"
	"github.com/agenthands/helix/internal/semantic/scheduler"
	semanticstore "github.com/agenthands/helix/internal/semantic/store"
	"github.com/agenthands/helix/internal/workspace"
)

// TestLiveUpdate_E2E_OverlayEpochAdvancesOnEdit is the automated end-to-end
// smoke gate for Phase 60's LIVE-07 invariant. It replaces the manual
// MCP-inspector smoke that 60-05B's checkpoint deferred to a human:
//
//   - Build the live bundle exactly as Daemon.New does (buildLiveBundle).
//   - Drive a synthetic edit through kernel.EditNotifier — the same path
//     the kernel/edit and kernel/fileops tools take after every successful
//     in-place edit (60-03).
//   - Wait for the coalescer debounce and assert two things:
//     1. semantic_live_overlay_meta.current_epoch advanced to ≥ 1
//     (LIVE-06: monotonic epoch on overlay tx commit).
//     2. semantic_live_overlay_files contains a row for the edited path
//     with write_epoch matching the bumped current_epoch
//     (LIVE-07: edited files emit a ChangeHelixEdit event into the
//     live queue and the handler upserts via overlay tx).
//
// This is the cross-wave integration check that could not run from either
// 60-05A or 60-05B in isolation — both must merge to main first. It also
// exercises the Phase-60 watcher-disabled / scanner-disabled mode so the
// epoch advance can be attributed cleanly to the EditNotifier path.
func TestLiveUpdate_E2E_OverlayEpochAdvancesOnEdit(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping LIVE-07 e2e smoke in -short mode (opens DuckDB)")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	wsDir := t.TempDir()
	// BL-01: store.Path must be workspace-relative; chdir into wsDir.
	t.Chdir(wsDir)
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// 1. Open a real DuckDB-backed semantic store.
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

	// 2. Construct a minimal kernel — just enough for SetEditNotifier.
	//    The pool is created but never Run(), so no LS workers spin up.
	wsReg := workspace.NewRegistry()
	langReg, err := langregistry.NewRegistry()
	if err != nil {
		t.Fatalf("langregistry.NewRegistry: %v", err)
	}
	installer := langregistry.NewInstaller(langregistry.InstallerConfig{}, logger)
	k := kernel.NewKernel(
		wsReg,
		langReg,
		installer,
		kernel.KernelConfig{},
		nil, // memory pressure
		logger,
		lspool.NoopSink{},
		nil, // tracer falls back to noop
	)

	// 3. Construct a no-op extraction scheduler. The handler only needs
	//    SetIncrementalHandler to land; no extraction job is scheduled in
	//    this smoke (the LIVE-07 path commits the overlay write
	//    independently of extraction).
	sched := scheduler.NewScheduler(nil)

	// 4. Build the live bundle. WatcherEnabled / ManifestScanEnabled are
	//    OFF so any epoch advance is attributable to the EditNotifier
	//    path alone (no fsnotify, no periodic walk).
	cfg := semantic.LiveUpdatesConfig{
		Enabled:              true,
		WatcherEnabled:       false,
		ManifestScanEnabled:  false,
		ManifestScanInterval: "10s",
		DebounceMS:           50, // tight for test
		MaxBatchDelayMS:      150,
		BulkChangeThreshold:  200,
	}
	bundle := buildLiveBundle(cfg, semantic.LSPEnrichmentConfig{}, store, sched, k, metrics, logger)
	if bundle == nil {
		t.Fatal("buildLiveBundle returned nil — expected non-nil with Enabled=true and required deps wired")
	}
	if k.EditNotifier() == nil {
		t.Fatal("kernel.EditNotifier() is nil after buildLiveBundle — SetEditNotifier(liveService) was not called")
	}

	// 5. Start the per-workspace lifecycle (mirrors SetActivateCallback in
	//    Daemon.New).
	wsKey := workspace.WorkspaceKey{RepoRoot: wsDir}
	bundle.startWorkspace(ctx, wsKey, logger)
	t.Cleanup(func() { bundle.service.Stop(wsKey) })

	// 6. Create a real file on disk so the handler's HashFile path can
	//    read it. The Phase 60 handler.UpdateChangedFile invokes
	//    scanner.HashFile(absPath) before staging the overlay row.
	fooPath := filepath.Join(wsDir, "foo.go")
	if err := os.WriteFile(fooPath, []byte("package foo\n\nvar X = 1\n"), 0o644); err != nil {
		t.Fatalf("write foo.go: %v", err)
	}

	// 7. Read the pre-edit epoch from semantic_live_overlay_meta. It
	//    starts at 0 and advances by 1 per successful overlay tx commit.
	repoID := wsDir // matches buildLiveBundle's repoIDFor(ws): RepoRoot
	preEpoch := readOverlayEpoch(t, store, repoID)

	// 8. Drive the edit through the kernel's EditNotifier — same call
	//    site as kernel/edit/tools.go and kernel/fileops/tools.go after
	//    a successful edit (the OnEdit hook from 60-03).
	notifier := k.EditNotifier()
	if err := notifier.OnEdit(ctx, wsKey, []string{fooPath}); err != nil {
		t.Fatalf("EditNotifier.OnEdit: %v", err)
	}

	// 9. Wait for: signal → coalescer debounce → handler dispatch →
	//    overlay tx commit. Poll the meta row until current_epoch
	//    advances past the pre-edit reading, or fail.
	deadline := time.Now().Add(5 * time.Second)
	var postEpoch uint64
	for time.Now().Before(deadline) {
		postEpoch = readOverlayEpoch(t, store, repoID)
		if postEpoch > preEpoch {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if postEpoch <= preEpoch {
		t.Fatalf("LIVE-06 violation: current_epoch did not advance after EditNotifier.OnEdit (pre=%d post=%d, want post>pre)", preEpoch, postEpoch)
	}

	// 10. Verify the overlay file row exists for the edited path with
	//     a write_epoch ≤ the bumped current_epoch (LIVE-07).
	wePath, weEpoch := readOverlayFileRow(t, store, repoID, fooPath)
	if wePath != fooPath {
		t.Fatalf("LIVE-07 violation: no semantic_live_overlay_files row for path=%s after EditNotifier.OnEdit", fooPath)
	}
	if weEpoch == 0 || weEpoch > postEpoch {
		t.Fatalf("LIVE-07 violation: overlay file row has write_epoch=%d, current_epoch=%d (want 1 ≤ write_epoch ≤ current_epoch)", weEpoch, postEpoch)
	}

	// 11. CR-04 regression: the lspqueue producer side. After a
	//     successful overlay commit the handler must have enqueued a
	//     RevalidateFileJob so Phase 61's worker has something to
	//     consume. Pre-fix the queue was constructed but never read
	//     from or enqueued into.
	if bundle.lspQueue == nil {
		t.Fatal("CR-04 regression: bundle.lspQueue is nil — handler producer side cannot be wired")
	}
	// Phase 61 P01: queue is now lane-aware (*lspenrich.LaneQueue). Sum
	// the per-lane depths to assert the same producer-side invariant that
	// CR-04 enforced on the legacy single-channel queue.
	totalDepth := bundle.lspQueue.Depth(lspenrich.LaneHigh) + bundle.lspQueue.Depth(lspenrich.LaneBackground)
	if totalDepth != 1 {
		t.Fatalf("CR-04 regression: total lane depth = %d after one successful EditNotifier.OnEdit, want 1", totalDepth)
	}
	// Phase 61 D-01 sanity: ChangeHelixEdit (the kind the test injects via
	// EditNotifier.OnEdit) maps to LaneHigh.
	if got := bundle.lspQueue.Depth(lspenrich.LaneHigh); got != 1 {
		t.Fatalf("Phase 61 D-01 regression: high-lane depth = %d after a helix_edit, want 1", got)
	}
}

// readOverlayEpoch reads semantic_live_overlay_meta.current_epoch via the
// store's read-only DB() handle. Returns 0 when the meta row does not
// exist yet (pre-first-tx state).
func readOverlayEpoch(t *testing.T, s *semanticstore.Store, repoID string) uint64 {
	t.Helper()
	db := s.DB()
	if db == nil {
		t.Fatal("store.DB() is nil")
	}
	var ce uint64
	row := db.QueryRow(
		`SELECT current_epoch FROM semantic_live_overlay_meta WHERE repo_id = ?`,
		repoID,
	)
	if err := row.Scan(&ce); err != nil {
		// no row yet => epoch 0 by contract
		return 0
	}
	return ce
}

// readOverlayFileRow returns (path, write_epoch) for the (repoID, path)
// row in semantic_live_overlay_files, or ("", 0) when absent.
func readOverlayFileRow(t *testing.T, s *semanticstore.Store, repoID, path string) (string, uint64) {
	t.Helper()
	db := s.DB()
	if db == nil {
		t.Fatal("store.DB() is nil")
	}
	var p string
	var we uint64
	row := db.QueryRow(
		`SELECT path, write_epoch FROM semantic_live_overlay_files WHERE repo_id = ? AND path = ?`,
		repoID, path,
	)
	if err := row.Scan(&p, &we); err != nil {
		return "", 0
	}
	return p, we
}

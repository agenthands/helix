// Package daemon: live-update wiring helpers for Phase 60-05B.
//
// This file is split out from daemon.go so the (verbose) construction of
// the live-update pipeline does not bloat the main bootstrap function.
// Everything here runs inside Daemon.New AFTER the semantic scheduler is
// open (Phase 59) and BEFORE the SetActivateCallback closure is wired
// (the callback captures the resulting managers).
package daemon

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/agenthands/helix/internal/kernel"
	"github.com/agenthands/helix/internal/kernel/lspool"
	"github.com/agenthands/helix/internal/obs"
	"github.com/agenthands/helix/internal/phasegraph"
	"github.com/agenthands/helix/internal/phasegraph/pipelines"
	"github.com/agenthands/helix/internal/semantic"
	"github.com/agenthands/helix/internal/semantic/live"
	"github.com/agenthands/helix/internal/semantic/live/coalescer"
	"github.com/agenthands/helix/internal/semantic/live/handler"
	"github.com/agenthands/helix/internal/semantic/live/scanner"
	"github.com/agenthands/helix/internal/semantic/lspenrich"
	liveservice "github.com/agenthands/helix/internal/semantic/live/service"
	"github.com/agenthands/helix/internal/semantic/live/watcher"
	"github.com/agenthands/helix/internal/semantic/scheduler"
	semanticstore "github.com/agenthands/helix/internal/semantic/store"
	"github.com/agenthands/helix/internal/workspace"
)

// liveWatcherManager is the interface the daemon needs from the
// 60-05A watcher.Manager. Declared inside the daemon package so this
// plan (60-05B) compiles against a placeholder before P05A merges; once
// P05A lands, *watcher.Manager satisfies this interface unchanged.
//
// Method set mirrors scanner.Manager so the daemon wiring is symmetric.
type liveWatcherManager interface {
	Start(ctx context.Context, ws workspace.WorkspaceKey) error
	Stop(ws workspace.WorkspaceKey)
}

// liveBundle holds the per-daemon live-update state. SetActivateCallback
// captures *liveBundle by pointer so the per-workspace lifecycle calls
// can land in the closure.
type liveBundle struct {
	service    *liveservice.Service
	scannerMgr *scanner.Manager
	watcherMgr liveWatcherManager // nil when watcher is disabled OR P05A not yet merged
	// lspQueue holds the Phase 61 lane-aware queue. The handler enqueues
	// via the LSPLaneEnqueuer interface; Phase 61's worker (P02) reads via
	// LaneQueue.Drain. Phase 60 used the single-channel *lspqueue.Queue;
	// Phase 61 P01 swaps in the 2-lane variant.
	lspQueue *lspenrich.LaneQueue

	// enrichMgr is the Phase 61 P03 enrichment manager. nil when
	// LSPEnrichment is disabled or MaxConcurrentWorkers <= 0. Owns the
	// per-(wsKey, lang) lease cache (B2 fix-path-A) and the worker
	// goroutines draining the lane queue.
	//
	// The daemon's Run goroutine pulls Run(ctx) into the top-level
	// errgroup; OnWorkspaceDeactivate is invoked from the gRPC
	// DeactivateWorkspace handler so cached leases for the deactivated
	// workspace are released promptly.
	enrichMgr *lspenrich.Manager
}

// Run starts the long-lived enrichment worker goroutines. Returns nil
// immediately when the bundle has no enrichment manager (LSPEnrichment
// disabled). The daemon's top-level errgroup owns the resulting goroutine.
func (b *liveBundle) Run(ctx context.Context) error {
	if b == nil || b.enrichMgr == nil {
		// Block until ctx done so the errgroup goroutine doesn't return
		// immediately and tear down its sibling goroutines.
		<-ctx.Done()
		return ctx.Err()
	}
	err := b.enrichMgr.Run(ctx)
	if err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}

// OnWorkspaceDeactivate releases all enrichment leases cached for ws (B2
// fix-path-A — CONTEXT lines 492-494). No-op when the bundle has no
// enrichment manager. Wired into the gRPC DeactivateWorkspace handler in
// daemon.go.
func (b *liveBundle) OnWorkspaceDeactivate(ws workspace.WorkspaceKey) {
	if b == nil || b.enrichMgr == nil {
		return
	}
	b.enrichMgr.OnWorkspaceDeactivate(ws)
}

// Stop releases all cached leases across all workspaces. Called from
// daemon shutdown as the safety-net fallback when individual workspace
// deactivation events were not delivered. No-op when the bundle has no
// enrichment manager.
func (b *liveBundle) Stop() {
	if b == nil || b.enrichMgr == nil {
		return
	}
	b.enrichMgr.Stop()
}

// startWorkspace fires the per-workspace lifecycle hooks. Called from
// SetActivateCallback after kernel activation succeeds.
func (b *liveBundle) startWorkspace(ctx context.Context, ws workspace.WorkspaceKey, logger *slog.Logger) {
	if b == nil || b.service == nil {
		return
	}
	b.service.Start(ctx, ws)
	if b.watcherMgr != nil {
		if err := b.watcherMgr.Start(ctx, ws); err != nil {
			logger.Warn("live: watcher start", "ws", ws, "err", err)
		}
	}
	if b.scannerMgr != nil {
		if err := b.scannerMgr.Start(ctx, ws); err != nil {
			logger.Warn("live: scanner start", "ws", ws, "err", err)
		}
	}
}

// storeFileHashLookup adapts *semanticstore.Store to the classifier's
// live.FileHashLookup interface. The store's QueryEffectiveFiles is the
// snapshot ⊕ overlay merged read; for Phase 60 the schema's
// EffectiveContentHash is not yet available as a typed read, so this
// adapter ALWAYS returns ("", false, nil) — the classifier then treats
// every path as "unknown to store" and emits ChangeFileCreated. This is
// safe: the handler's UpdateChangedFile path always upserts, and the
// classifier's prev-vs-new distinction is currently a label-only
// concern (the metric helper records the kind).
//
// The TODO marker is retained so a Phase 62+ revision (which will
// implement a typed EffectiveContentHash query) can drop this adapter.
type storeFileHashLookup struct {
	store *semanticstore.Store
}

func (s *storeFileHashLookup) EffectiveContentHash(_ context.Context, _ semantic.RepoID, _ string) (string, bool, error) {
	// TODO(60-D-05): wire to a real semantic_files.content_hash + overlay
	// merge query when Phase 62 lands the effective-fact diff body.
	return "", false, nil
}

// scannerStoreLookup adapts *semanticstore.Store to scanner.FileHashLookup.
// Returns an empty map for Phase 60: the snapshot side is empty until
// Phase 59's snapshot writer surfaces rows, and the scanner only needs
// the snapshot side to detect deletions. An empty map means "no known
// files yet, every walked path is a 'new' change" — handled correctly
// by the upstream classifier+handler chain.
type scannerStoreLookup struct {
	store *semanticstore.Store
}

func (s *scannerStoreLookup) KnownFiles(_ context.Context, _ semantic.RepoID) (map[string]string, error) {
	// TODO(60-D-05): wire to `SELECT path, content_hash FROM semantic_files
	// WHERE repo_id = ?` once Phase 59's snapshot writer populates rows.
	return map[string]string{}, nil
}

// noopLogAdapter is the slog logger handed to coalescer.New / handler.New
// when the caller hasn't supplied a richer one.
type noopLogAdapter struct{ inner *slog.Logger }

func (n noopLogAdapter) Warn(msg string, args ...any) { n.inner.Warn(msg, args...) }
func (n noopLogAdapter) Info(msg string, args ...any) { n.inner.Info(msg, args...) }

// buildLiveBundle wires the live-update pipeline. Returns a non-nil
// *liveBundle when SemanticIndex.Enabled && LiveUpdates.Enabled. nil
// otherwise — the callsite in daemon.go MUST handle nil.
//
// On success the kernel's EditNotifier is installed and the scheduler's
// IncrementalHandler is wired. The watcher manager field stays nil (P05A
// will populate it via a daemon-package setter when the watcher package
// lands and the bootstrap is updated).
//
// Phase 61 P03 wiring: enrichCfg drives optional construction of the LSP
// enrichment manager. When enrichCfg.Enabled && MaxConcurrentWorkers > 0
// the bundle's enrichMgr is non-nil; the daemon's top-level errgroup pulls
// bundle.Run(ctx) and the gRPC DeactivateWorkspace handler invokes
// bundle.OnWorkspaceDeactivate(wsKey).
func buildLiveBundle(
	cfg semantic.LiveUpdatesConfig,
	enrichCfg semantic.LSPEnrichmentConfig,
	store *semanticstore.Store,
	sched *scheduler.Scheduler,
	k *kernel.Kernel,
	metrics *obs.Metrics,
	logger *slog.Logger,
) *liveBundle {
	if !cfg.Enabled {
		return nil
	}
	if store == nil || sched == nil || k == nil {
		// Required dependencies missing — degraded mode. Log once and
		// return nil so the SetActivateCallback skip-path activates.
		logger.Warn("live: skipping bundle — missing store, scheduler, or kernel")
		return nil
	}

	// 1. Build the handler (consumer of coalescer dispatches).
	overlayAdapter := &storeOverlayWriter{store: store}
	h := handler.New(
		overlayAdapter,
		scanner.HashFile,
		sched,
		noopLogAdapter{inner: logger},
	)
	// Producer side of the Phase 61 lane-aware queue (Phase 60 P04
	// promise + CR-04 fix; Phase 61 P01 lane-aware): after every
	// successful overlay commit the handler enqueues a RevalidateFileJob
	// into the lane chosen by selectLane (helix_edit → high; everything
	// else → background). The queue itself is constructed below and
	// assigned to bundle.lspQueue; we set the handler field here so the
	// handler keeps the same lifecycle.
	lspQ := lspenrich.NewLaneQueue(1024, 1024)
	h.LSPQueue = lspQ
	// Wire scheduler's IncrementalHandler back-edge (60-04).
	sched.SetIncrementalHandler(h)

	// 2. Build the classifier closure.
	hashLookup := &storeFileHashLookup{store: store}
	classifierFn := func(ctx context.Context, repoID semantic.RepoID, path string, src live.ChangeSource) (live.SourceChangeKind, bool, error) {
		return live.ClassifyPathChange(ctx, repoID, path, hashLookup, scanner.HashFile, src)
	}

	// 3. repoIDFor: workspace.RepoRoot is the canonical repo identity in
	// Phase 60 (matches the scheduler's WorkspaceID-derivation).
	repoIDFor := func(ws workspace.WorkspaceKey) semantic.RepoID {
		return semantic.RepoID(ws.RepoRoot)
	}

	// 4. Construct the live service (60-04 spine). Metrics is plumbed
	// into the coalescer Config so the {dropped, applied, error}
	// outcomes land in helix_semantic_live_updates_total{kind, outcome}
	// per invariant #4 (CR-01 fix). Nil-safe in the coalescer; in
	// production *obs.Metrics is always non-nil.
	liveService := liveservice.New(
		coalescer.Config{
			Debounce:            time.Duration(cfg.DebounceMS) * time.Millisecond,
			MaxBatchDelay:       time.Duration(cfg.MaxBatchDelayMS) * time.Millisecond,
			BulkChangeThreshold: cfg.BulkChangeThreshold,
			QueueSize:           1024,
			Metrics:             metrics,
		},
		h,
		classifierFn,
		repoIDFor,
		noopLogAdapter{inner: logger},
	)

	// 5. Wire kernel.SetEditNotifier (LIVE-07 invariant: kernel does NOT
	// import semantic; the interface lives in internal/kernel/notifier.go).
	k.SetEditNotifier(liveService)

	// 6. Construct managers gated on the per-feature config keys. The
	// lspqueue was created above and assigned to the handler so the
	// producer side is wired (CR-04 fix); here we hold onto it via the
	// bundle so Phase 61's worker (and admin/status callers) can reach
	// it through the bundle accessor.
	bundle := &liveBundle{service: liveService, lspQueue: lspQ}

	// 6a. Phase 61 P03: construct LSP enrichment manager when enabled.
	//
	// **B4 (P01-T2 setter)**: populate Pool's yieldCheckWindow from cfg
	// unconditionally — even when the manager is disabled, the foreground-
	// busy machinery is harmless.
	if k.Pool() != nil && enrichCfg.YieldCheckWindowMs > 0 {
		k.Pool().SetYieldCheckWindow(time.Duration(enrichCfg.YieldCheckWindowMs) * time.Millisecond)
	}
	if enrichCfg.Enabled && enrichCfg.MaxConcurrentWorkers > 0 {
		acquirer := lspenrich.NewPoolAcquirer(k.Pool())
		readiness := lspenrich.NewPoolReadinessProbe(k.Pool())
		cascadeStore := &storeCascadeStoreAdapter{store: store}
		enrichMetrics := lspenrich.NewProdMetricsSink(metrics)
		enrichMgr := lspenrich.NewManager(
			lspQ,
			acquirer,
			cascadeStore,
			readiness,
			enrichCfg,
			enrichMetrics,
			logger,
		)
		// Phase 61-05 — wire production CascadeLSPFactory so dispatched
		// jobs run the cascade against real LSPs (gopls/jdtls/...) via
		// *lspool.WorkerLease.Request.  Without this every dispatched
		// job would land on worker.go's `NewCascadeLSP factory is nil`
		// branch (OutcomeDropped + Error log) — the pre-61-05 gap that
		// 61-VERIFICATION.md flagged.
		enrichMgr.SetCascadeLSPFactory(func(lease *lspool.WorkerLease) lspenrich.CascadeLSP {
			return lspenrich.NewCascadeLSPShim(lease)
		})
		bundle.enrichMgr = enrichMgr
		logger.Info("lsp-enrichment manager constructed",
			"max_concurrent_workers", enrichCfg.MaxConcurrentWorkers,
			"yield_check_window_ms", enrichCfg.YieldCheckWindowMs,
			"timeout_per_file", enrichCfg.TimeoutPerFile,
			"cascade_lsp_factory", "production",
		)
	} else {
		logger.Info("lsp-enrichment disabled by config",
			"enabled", enrichCfg.Enabled,
			"max_concurrent_workers", enrichCfg.MaxConcurrentWorkers,
		)
	}

	if cfg.ManifestScanEnabled {
		interval, err := time.ParseDuration(cfg.ManifestScanInterval)
		if err != nil || interval <= 0 {
			interval = 10 * time.Second
		}
		bundle.scannerMgr = scanner.NewManager(
			liveService,
			&scannerStoreLookup{store: store},
			repoIDFor,
			scanner.Config{Interval: interval},
			logger,
		)
	}

	if cfg.WatcherEnabled {
		bundle.watcherMgr = watcher.NewManager(
			liveService,
			watcher.Config{
				DebounceMs: time.Duration(cfg.DebounceMS) * time.Millisecond,
			},
			logger,
		)
	}

	// 7. Phase 60 D-06 wiring validator (CR-03 fix). The phasegraph
	// constructor returns a 9-phase DAG whose Run closures fail when
	// any required component is nil — invoking the runner here
	// converts a misconfiguration into a bootstrap-time error rather
	// than a silent no-op at runtime. Failure logs but does not fail
	// the daemon: in production the only paths to nil are programmer
	// bugs that the kernel/scheduler tests would already have caught,
	// and aborting the daemon for a stale wiring would block other
	// subsystems unnecessarily. The log line is the operator-visible
	// signal.
	pg, perr := phasegraph.ValidatePhaseGraph(pipelines.BuildLiveUpdatePhases(pipelines.LiveUpdateComponents{
		EditNotifier:         liveService,
		OverlayStore:         store,
		IncrementalScheduler: h,
		LSPRevalidationQueue: bundle.lspQueue,
	}))
	if perr != nil {
		logger.Warn("live: phase-graph validate failed", "err", perr)
	} else if _, rerr := phasegraph.RunPhaseGraph(context.Background(), pg); rerr != nil {
		logger.Warn("live: phase-graph wiring validator failed", "err", rerr)
	} else {
		logger.Info("live: phase-graph wiring validator passed", "phases", 9)
	}

	return bundle
}

// storeOverlayWriter adapts *semanticstore.Store to handler.OverlayWriter.
// The handler's OverlayTx interface is a subset of store.OverlayTx — the
// adapter wraps the concrete tx behind handler.OverlayTx.
type storeOverlayWriter struct {
	store *semanticstore.Store
}

func (s *storeOverlayWriter) BeginOverlayTx(ctx context.Context, repoID string) (handler.OverlayTx, error) {
	tx, err := s.store.BeginOverlayTx(ctx, repoID)
	if err != nil {
		return nil, err
	}
	return &storeOverlayTxAdapter{tx: tx}, nil
}

// storeOverlayTxAdapter wraps *semanticstore.OverlayTx with the
// handler.OverlayTx subset. Method bodies are pure forwards.
type storeOverlayTxAdapter struct {
	tx *semanticstore.OverlayTx
}

func (a *storeOverlayTxAdapter) Epoch() uint64 { return a.tx.Epoch() }
func (a *storeOverlayTxAdapter) UpsertOverlayFile(ctx context.Context, path, hash string) error {
	return a.tx.UpsertOverlayFile(ctx, path, hash)
}
func (a *storeOverlayTxAdapter) MarkFileDeleted(ctx context.Context, path string) error {
	return a.tx.MarkFileDeleted(ctx, path)
}

// MarkFileSemanticPending forwards to the underlying *store.OverlayTx.
// Phase 61 P01 D-05: handler.markBulkPending stamps every affected file
// partial_reason="bulk_update_pending" via this seam.
func (a *storeOverlayTxAdapter) MarkFileSemanticPending(ctx context.Context, path, reason string) error {
	return a.tx.MarkFileSemanticPending(ctx, path, reason)
}
func (a *storeOverlayTxAdapter) Commit() error   { return a.tx.Commit() }
func (a *storeOverlayTxAdapter) Rollback() error { return a.tx.Rollback() }

// storeCascadeStoreAdapter adapts *semanticstore.Store to lspenrich.CascadeStore.
// The lspenrich.CascadeStore seam is the per-file overlay-tx surface the
// enrichment cascade uses; the adapter opens *store.OverlayTx and wraps it
// in storeCascadeTxAdapter (below) which implements the broader cascade
// surface (Upsert*, MarkFileSemanticPending, Commit, Rollback, Epoch).
//
// Phase 61 P03: live_wiring constructs the adapter when LSP enrichment is
// enabled and passes it to the lspenrich.Manager. The adapter forwards
// MarkFileSemanticPending + Commit + Rollback + Epoch to the underlying
// *store.OverlayTx; the Upsert* methods that do not yet exist on the real
// store (Phase 60 left them as no-op stubs — Phase 62 wires the real
// upsert path) accept the call and return nil.
type storeCascadeStoreAdapter struct {
	store *semanticstore.Store
}

func (a *storeCascadeStoreAdapter) BeginCascadeTx(ctx context.Context, repoID string) (lspenrich.CascadeTx, error) {
	tx, err := a.store.BeginOverlayTx(ctx, repoID)
	if err != nil {
		return nil, err
	}
	return &storeCascadeTxAdapter{tx: tx}, nil
}

// storeCascadeTxAdapter wraps *semanticstore.OverlayTx with the broader
// lspenrich.CascadeTx surface. Phase 61 P03: the cascade exercises the
// MarkFileSemanticPending + Commit + Rollback + Epoch methods against
// the real tx; the upsert methods are no-ops (Phase 62 wires the real
// upsert path). The adapter also implements WriteInvalidations as a
// no-op (Phase 60 D-04 already documents this as a typed stub).
type storeCascadeTxAdapter struct {
	tx *semanticstore.OverlayTx
}

func (a *storeCascadeTxAdapter) UpsertSymbols(ctx context.Context, path string, syms []lspenrich.Symbol) error {
	// Phase 62 wires the real upsert path; Phase 61 commits the partial
	// outcome via MarkFileSemanticPending only.
	return nil
}

func (a *storeCascadeTxAdapter) UpsertReferences(ctx context.Context, path string, refs []lspenrich.Reference) error {
	return nil
}

func (a *storeCascadeTxAdapter) UpsertEdges(ctx context.Context, edges []lspenrich.Edge) error {
	// B3 (62-02 Task 4.5): forward to the merge entrypoint so out-of-tree
	// callers still using the legacy alias get the D-14 boundary too.
	return a.UpsertEdgesWithMerge(ctx, edges)
}

// UpsertEdgesWithMerge translates lspenrich.Edge → semanticstore.EdgeRow
// and routes through OverlayTx.UpsertEdgesWithMerge so cascade-emitted
// LSP rows enter the (src, dst, edge_kind) merge boundary uniformly
// (Phase 62 P02 D-14, B3).
func (a *storeCascadeTxAdapter) UpsertEdgesWithMerge(ctx context.Context, edges []lspenrich.Edge) error {
	if len(edges) == 0 {
		return nil
	}
	rows := make([]semanticstore.EdgeRow, 0, len(edges))
	for _, e := range edges {
		rows = append(rows, semanticstore.EdgeRow{
			SrcNodeID:       e.SrcNodeID,
			DstNodeID:       e.DstNodeID,
			EdgeKind:        e.Kind,
			Source:          e.Source,
			Confidence:      e.Confidence,
			Weight:          e.Weight,
			ValidationState: e.ValidationState,
		})
	}
	return a.tx.UpsertEdgesWithMerge(ctx, rows)
}

func (a *storeCascadeTxAdapter) UpsertDiagnostics(ctx context.Context, path string, diags []lspenrich.Diagnostic) error {
	return nil
}

func (a *storeCascadeTxAdapter) WriteInvalidations(ctx context.Context) error {
	// Phase 60 D-04 stub; Phase 62 wires the consumer.
	return nil
}

func (a *storeCascadeTxAdapter) MarkFileSemanticPending(ctx context.Context, path, reason string) error {
	return a.tx.MarkFileSemanticPending(ctx, path, reason)
}

func (a *storeCascadeTxAdapter) Commit() error   { return a.tx.Commit() }
func (a *storeCascadeTxAdapter) Rollback() error { return a.tx.Rollback() }
func (a *storeCascadeTxAdapter) Epoch() uint64   { return a.tx.Epoch() }

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
	"log/slog"
	"time"

	"github.com/agenthands/helix/internal/kernel"
	"github.com/agenthands/helix/internal/obs"
	"github.com/agenthands/helix/internal/semantic"
	"github.com/agenthands/helix/internal/semantic/live"
	"github.com/agenthands/helix/internal/semantic/live/coalescer"
	"github.com/agenthands/helix/internal/semantic/live/handler"
	"github.com/agenthands/helix/internal/semantic/live/lspqueue"
	"github.com/agenthands/helix/internal/semantic/live/scanner"
	liveservice "github.com/agenthands/helix/internal/semantic/live/service"
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
	lspQueue   *lspqueue.Queue
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

// noopMetricsLogger is the slog logger handed to coalescer.New / handler.New
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
func buildLiveBundle(
	cfg semantic.LiveUpdatesConfig,
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

	// 4. Construct the live service (60-04 spine).
	liveService := liveservice.New(
		coalescer.Config{
			Debounce:            time.Duration(cfg.DebounceMS) * time.Millisecond,
			MaxBatchDelay:       time.Duration(cfg.MaxBatchDelayMS) * time.Millisecond,
			BulkChangeThreshold: cfg.BulkChangeThreshold,
			QueueSize:           1024,
		},
		h,
		classifierFn,
		repoIDFor,
		noopLogAdapter{inner: logger},
	)

	// 5. Wire kernel.SetEditNotifier (LIVE-07 invariant: kernel does NOT
	// import semantic; the interface lives in internal/kernel/notifier.go).
	k.SetEditNotifier(liveService)

	// 6. Construct managers gated on the per-feature config keys.
	bundle := &liveBundle{service: liveService, lspQueue: lspqueue.New(1024)}

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

	// Watcher manager wiring is deferred to setLiveWatcherManager() —
	// P05A's *watcher.Manager satisfies liveWatcherManager and the
	// daemon's bootstrap will install it once the watcher package lands.
	_ = metrics // metrics is reserved for future per-bundle wiring (counters live on *obs.Metrics)
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
func (a *storeOverlayTxAdapter) Commit() error   { return a.tx.Commit() }
func (a *storeOverlayTxAdapter) Rollback() error { return a.tx.Rollback() }

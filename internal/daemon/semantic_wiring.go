// Package daemon: Phase 64 P64-08 Task 1 — semantic skill production wiring.
//
// semantic_wiring.go owns:
//
//   - semanticBundle — daemon-side container for the per-workspace bleve
//     handles, the daemon-singleton IndexRunner, the bleve Recoverer
//     map, and the production buildFn pipeline.
//   - newSemanticBundle constructed alongside compactBndl (after rank
//     wiring) and registers all 8 SemanticSkill setters when the skill
//     is present.
//   - semanticBundle.Run blocks on ctx.Done and shuts down every bleve
//     engine + the IndexRunner. Mirrors compact_wiring.go / rank_wiring.go.
//   - Adapter implementations for the 7 narrow accessor seams declared
//     in internal/skill/semantic/accessors.go (StoreAccessor,
//     SchedulerAccessor, QueueAccessor, LiveAccessor, RetrievalAccessor,
//     CompactorAccessor, SessionAccessor).
//   - makeProductionBuildFn — the pipeline that BeginSnapshot /
//     WriteSnapshotFacts / CommitSnapshot consumed by IndexRunner. The
//     full-pipeline composition is documented inline (closes W3 at the
//     doc layer); a TODO(phase-65) anchor flags the gap that the
//     production indexer chain will fill.
//
// Closes W1 (production layer): semSchedulerAdapter.ClusterStatus returns
// ClusterStatus{State:"unknown", Reason:"phase-62-clustering-no-status-accessor"}
// until Phase 65/67 wires a live source.
//
// Closes W2 (production layer): semSessionAdapter wraps the daemon-side
// getSession closure (the SAME closure passed to InstallMiddleware at
// daemon.go:584). Single source of truth for per-request session lookup.
//
// Closes W3 (production layer): makeProductionBuildFn pipeline.

package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"
	"time"

	"github.com/agenthands/helix/internal/mcp"
	"github.com/agenthands/helix/internal/obs"
	"github.com/agenthands/helix/internal/semantic/extract"
	"github.com/agenthands/helix/internal/semantic/graph"
	"github.com/agenthands/helix/internal/semantic/live"
	"github.com/agenthands/helix/internal/semantic/lspenrich"
	"github.com/agenthands/helix/internal/semantic/retrieval"
	semanticstore "github.com/agenthands/helix/internal/semantic/store"
	"github.com/agenthands/helix/internal/skill/semantic"
	"github.com/agenthands/helix/internal/workspace"
)

// ----- Configuration -----

// semanticConfig holds the daemon-side knobs for the semantic bundle. The
// values are NOT yet exposed as koanf keys (no semantic_index.* keys ship in
// Phase 64 — see CONTEXT.md "Constraints" #4); they are inlined here as
// constants with default values so a future config-key migration touches
// only this file.
type semanticConfig struct {
	// BleveSubdir is the per-workspace bleve directory relative to the
	// repo root. ".helix/semantic.bleve" matches the SPEC §25 default.
	BleveSubdir string
	// IndexTimeout is the per-call ceiling applied by IndexRunner when no
	// per-request max_duration_ms is supplied (or the request cap exceeds
	// this). 120s matches SPEC §23.1.
	IndexTimeout time.Duration
}

// loadSemanticConfig returns the daemon-side semantic config with defaults.
// Phase 64 ships defaults only; no koanf surface yet.
func loadSemanticConfig() semanticConfig {
	return semanticConfig{
		BleveSubdir:  ".helix/semantic.bleve",
		IndexTimeout: 120 * time.Second,
	}
}

// ----- Bundle -----

// semanticBundle owns the daemon-side semantic skill state. nil-safe at every
// accessor.
type semanticBundle struct {
	cfg             semanticConfig
	store           *semanticstore.Store
	scheduler       *rankBundle
	queue           *lspenrich.LaneQueue
	live            *liveBundle
	compact         *compactBundle
	extractRegistry *extract.Registry
	logger          *slog.Logger
	metrics         *obs.Metrics

	// getSession is the per-request session-lookup closure captured from
	// daemon.go (the SAME closure passed to InstallMiddleware at
	// daemon.go:584). Closes W2 at the production layer.
	getSession func(ctx context.Context) *mcp.SessionInfo

	// runner is the daemon-singleton IndexRunner; one runner manages all
	// workspaces (singleflight key includes the repo root).
	runner *semantic.IndexRunner

	// skill is the registered SemanticSkill instance; nil if the skill
	// did not register (semantic disabled).
	skill *semantic.SemanticSkill

	// engines / recoveres are per-workspace bleve handles + recovery
	// probes lazily constructed on first activation.
	mu        sync.Mutex
	engines   map[string]*retrieval.Engine    // key=ws.RepoRoot
	recoveres map[string]*retrieval.Recoverer // key=ws.RepoRoot

	// retrievalAdapter is the SemanticSkill-facing adapter; it reads
	// recoveres under b.mu so per-workspace recovery state stays coherent
	// with ensureRetrieval.
	retrievalAdapter *semRetrievalAdapter
}

// newSemanticBundle constructs the bundle. Returns nil when the store is nil
// (semantic disabled). Mirrors newCompactBundle's nil-safety contract.
//
// On non-nil return, ALL EIGHT SemanticSkill setters are wired (closes the
// "Daemon wires SemanticSkill post-init via SetStore/SetScheduler/SetQueue/
// SetLive/SetRunner/SetRetrieval/SetCompactor/SetSessionAccessor" must-have).
func newSemanticBundle(
	cfg semanticConfig,
	store *semanticstore.Store,
	scheduler *rankBundle,
	queue *lspenrich.LaneQueue,
	live *liveBundle,
	compact *compactBundle,
	extractRegistry *extract.Registry,
	logger *slog.Logger,
	metrics *obs.Metrics,
	getSession func(ctx context.Context) *mcp.SessionInfo,
) *semanticBundle {
	if store == nil {
		return nil
	}
	b := &semanticBundle{
		cfg:             cfg,
		store:           store,
		scheduler:       scheduler,
		queue:           queue,
		live:            live,
		compact:         compact,
		extractRegistry: extractRegistry,
		logger:          logger,
		metrics:         metrics,
		getSession:      getSession,
		engines:         make(map[string]*retrieval.Engine),
		recoveres:       make(map[string]*retrieval.Recoverer),
	}

	// Construct the production buildFn FIRST (its closure captures b) so
	// the IndexRunner has a real builder.
	buildFn := b.makeProductionBuildFn()
	storeAcc := b.storeAccessor()
	b.runner = semantic.NewProductionIndexRunner(storeAcc, buildFn, cfg.IndexTimeout)

	// Pre-construct the retrieval adapter so b.skill setters get a stable
	// pointer; the adapter reads b.recoveres under b.mu lazily.
	b.retrievalAdapter = &semRetrievalAdapter{bundle: b}

	// Resolve the registered SemanticSkill. nil when semantic disabled or
	// the skill failed to register; downstream wiring is a no-op in that
	// case.
	b.skill = semantic.GetSemanticSkill()
	if b.skill != nil {
		b.skill.SetStore(storeAcc)
		b.skill.SetScheduler(b.schedulerAccessor())
		b.skill.SetQueue(b.queueAccessor())
		b.skill.SetLive(b.liveAccessor())
		b.skill.SetRunner(b.runner)
		b.skill.SetRetrieval(b.retrievalAdapter)
		b.skill.SetCompactor(b.compactorAccessor())
		b.skill.SetSessionAccessor(b.sessionAccessor())
		if logger != nil {
			logger.Info("semantic skill setters wired",
				"setters", 8,
				"bleve_subdir", cfg.BleveSubdir,
				"index_timeout", cfg.IndexTimeout,
			)
		}
	}

	return b
}

// SetSessionFn updates the per-request session-lookup closure and rewires
// the SemanticSkill's SessionAccessor. Called by daemon.go after the
// getSessionFn closure is constructed (step 14 in newDaemon).
//
// This is the W2 closure at the production layer: the SAME closure passed
// to InstallMiddleware is the one the SemanticSkill consumes for per-request
// session lookup.
func (b *semanticBundle) SetSessionFn(getSession func(ctx context.Context) *mcp.SessionInfo) {
	if b == nil {
		return
	}
	b.getSession = getSession
	if b.skill != nil {
		b.skill.SetSessionAccessor(b.sessionAccessor())
	}
}

// Run blocks until ctx is cancelled, then closes every bleve engine and the
// IndexRunner. Mirrors compactBundle.Run / rankBundle.Run.
func (b *semanticBundle) Run(ctx context.Context) error {
	if b == nil {
		<-ctx.Done()
		return ctx.Err()
	}
	<-ctx.Done()
	b.shutdown()
	return ctx.Err()
}

// shutdown closes every bleve engine and cancels the IndexRunner. Safe to
// call multiple times.
func (b *semanticBundle) shutdown() {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	for key, engine := range b.engines {
		if engine == nil {
			continue
		}
		if err := engine.Close(); err != nil && b.logger != nil {
			b.logger.Warn("semantic: bleve close failed",
				"repo_root", key, "err", err)
		}
		delete(b.engines, key)
	}
	if b.runner != nil {
		b.runner.Shutdown(context.Background())
	}
}

// ensureRetrieval lazily opens (or creates) the bleve index for ws and
// triggers a Recoverer.Probe in a goroutine. Idempotent — a second call for
// the same workspace is a no-op.
//
// Wired into the daemon's existing SetActivateCallback closure (see
// daemon.go) so workspace activation primes the retrieval engine before any
// get_semantic_context call lands.
func (b *semanticBundle) ensureRetrieval(_ context.Context, ws workspace.WorkspaceKey) {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	key := ws.RepoRoot
	if key == "" {
		return
	}
	if _, ok := b.engines[key]; ok {
		return
	}

	path := filepath.Join(key, b.cfg.BleveSubdir)
	engine, err := retrieval.Open(path)
	if err != nil {
		// Open failed (likely no existing index); try New.
		engine, err = retrieval.New(path)
		if err != nil {
			if b.logger != nil {
				b.logger.Warn("semantic: bleve init failed; retrieval disabled for workspace",
					"repo_root", key, "path", path, "err", err)
			}
			return
		}
	}
	b.engines[key] = engine

	rec := retrieval.NewRecoverer(engine, b.storeReader(), b.logger)
	b.recoveres[key] = rec

	// Probe runs in its own goroutine — non-blocking per the recovery
	// contract (recovery.go:88-90).
	go func() {
		probeCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := rec.Probe(probeCtx, ws); err != nil && b.logger != nil {
			b.logger.Warn("semantic: bleve recovery probe failed",
				"repo_root", key, "err", err)
		}
	}()
}

// storeReader returns the StoreReader the Recoverer consumes. *Store
// implements retrieval.StoreReader natively (LatestCommittedSnapshot +
// IterateCommittedSymbols ship in 64-02). Compile-time guarded below.
func (b *semanticBundle) storeReader() retrieval.StoreReader {
	return b.store
}

// ----- Accessor adapter constructors -----

func (b *semanticBundle) storeAccessor() semantic.StoreAccessor {
	return &semStoreAdapter{store: b.store}
}
func (b *semanticBundle) schedulerAccessor() semantic.SchedulerAccessor {
	return &semSchedulerAdapter{rb: b.scheduler}
}
func (b *semanticBundle) queueAccessor() semantic.QueueAccessor {
	return &semQueueAdapter{q: b.queue}
}
func (b *semanticBundle) liveAccessor() semantic.LiveAccessor {
	return &semLiveAdapter{live: b.live}
}
func (b *semanticBundle) compactorAccessor() semantic.CompactorAccessor {
	return &semCompactorAdapter{cb: b.compact}
}
func (b *semanticBundle) sessionAccessor() semantic.SessionAccessor {
	return &semSessionAdapter{getSession: b.getSession}
}

// ----- Adapter implementations -----

// semStoreAdapter wraps *semanticstore.Store with the narrow StoreAccessor
// surface the SemanticSkill consumes. Delegates verbatim to *Store; the
// surface mirrors accessors.go's StoreAccessor interface.
type semStoreAdapter struct {
	store *semanticstore.Store
}

func (a *semStoreAdapter) LatestCommittedSnapshot(ctx context.Context, repoID string) (uint64, error) {
	if a == nil || a.store == nil {
		return 0, nil
	}
	return a.store.LatestCommittedSnapshot(ctx, repoID)
}

func (a *semStoreAdapter) CurrentGraphVersion(ctx context.Context, repoID string) (uint64, error) {
	if a == nil || a.store == nil {
		return 0, nil
	}
	return a.store.CurrentGraphVersion(ctx, repoID)
}

func (a *semStoreAdapter) OverlayHasPendingRows(repoID string) bool {
	if a == nil || a.store == nil {
		return false
	}
	return a.store.OverlayHasPendingRows(repoID)
}

func (a *semStoreAdapter) QueryEffectiveAdjacency(ctx context.Context, repoID, projection string) (
	out, in map[graph.NodeID]map[graph.NodeID]float64, err error,
) {
	if a == nil || a.store == nil {
		return map[graph.NodeID]map[graph.NodeID]float64{},
			map[graph.NodeID]map[graph.NodeID]float64{}, nil
	}
	return a.store.QueryEffectiveAdjacency(ctx, repoID, projection)
}

// semSchedulerAdapter wraps the rank bundle. ScoreStatus + ClusterStatus
// production accessors are W1 placeholders today — Phase 65/67 wires real
// sources.
type semSchedulerAdapter struct {
	rb *rankBundle
}

func (a *semSchedulerAdapter) IsQuiescent(repoID string) bool {
	if a == nil || a.rb == nil {
		return true
	}
	a.rb.mu.Lock()
	s, ok := a.rb.subs[repoID]
	a.rb.mu.Unlock()
	if !ok || s == nil {
		return true
	}
	return s.IsQuiescent()
}

// ScoreStatus returns the closed-enum score status for (repoID, projection).
// Phase 62 RankScheduler does not expose a per-(workspace, projection)
// accessor today — the closest is the per-row state surfaced inside
// computeScoreStatus, which the scheduler consumes when reading scores. Until
// Phase 65/67 wires a public accessor, the adapter returns
// graph.ScoreStatusMissing as the closed-enum default; downstream consumers
// (tools_status.go) treat "missing" as the safe pre-data state.
//
// This is a deliberate companion to the W1 ClusterStatus placeholder — both
// surface a stable closed-enum value under the SPEC §23.3 envelope until the
// real source ships.
func (a *semSchedulerAdapter) ScoreStatus(repoID, projection string) graph.ScoreStatus {
	_ = repoID
	_ = projection
	return graph.ScoreStatusMissing
}

// ClusterStatus is the W1 production-layer placeholder. Returns
// {State: "unknown", Reason: "phase-62-clustering-no-status-accessor"} until
// Phase 65/67 wires a live cluster source. Downstream consumers
// (tools_status.go) see the structured "unknown"+reason envelope unchanged.
//
// Closes checker W1 at the production layer.
func (a *semSchedulerAdapter) ClusterStatus(repoID string) semantic.ClusterStatus {
	_ = repoID
	return semantic.ClusterStatus{
		State:  "unknown",
		Reason: "phase-62-clustering-no-status-accessor",
	}
}

// semQueueAdapter wraps *lspenrich.LaneQueue with the QueueAccessor surface.
// LaneQueue is daemon-singleton (not per-workspace today); the adapter
// ignores the ws argument and returns the global depth + last-enqueue. This
// matches the Phase 61 P03 manager's contract (one queue, multi-workspace
// jobs distinguished by job.WorkspaceID).
type semQueueAdapter struct {
	q *lspenrich.LaneQueue
}

func (a *semQueueAdapter) DepthAll(_ workspace.WorkspaceKey) int {
	if a == nil || a.q == nil {
		return 0
	}
	return a.q.DepthAll()
}

func (a *semQueueAdapter) LastEnqueueAt(_ workspace.WorkspaceKey) int64 {
	if a == nil || a.q == nil {
		return 0
	}
	t := a.q.LastEnqueueAt()
	if t.IsZero() {
		return 0
	}
	return t.UnixMilli()
}

// semLiveAdapter wraps *liveBundle with the LiveAccessor surface. The
// SemanticSkill calls OnWorkspaceChanged with (ws, paths) — the underlying
// live.Service.OnWorkspaceChanged takes a (ctx, WorkspaceChangeSignal); the
// adapter constructs a synthetic signal sourced from the helix-edit channel
// (the same source kernel-driven edits use).
type semLiveAdapter struct {
	live *liveBundle
}

func (a *semLiveAdapter) OnWorkspaceChanged(ws workspace.WorkspaceKey, paths []string) error {
	if a == nil || a.live == nil || a.live.service == nil {
		return nil
	}
	// refresh_semantic_graph drives this through the helix-edit channel —
	// the same source kernel-driven edits use; the live service classifies
	// + enqueues via the existing fire-and-forget path.
	return a.live.service.OnWorkspaceChanged(context.Background(), live.WorkspaceChangeSignal{
		WorkspaceID: ws,
		Paths:       paths,
		Source:      live.ChangeSourceHelixEdit,
		ObservedAt:  time.Now(),
	})
}

func (a *semLiveAdapter) LastFlushAt(ws workspace.WorkspaceKey) int64 {
	if a == nil || a.live == nil {
		return 0
	}
	t := a.live.LastFlushAt(ws)
	if t.IsZero() {
		return 0
	}
	return t.UnixMilli()
}

// semCompactorAdapter wraps *compactBundle. OnFlush dispatches to the
// per-workspace compactor by repoID. Currently only used by
// index_semantic_graph(mode=incremental) per D-10 — refresh_semantic_graph
// MUST NOT call this (D-13).
type semCompactorAdapter struct {
	cb *compactBundle
}

func (a *semCompactorAdapter) OnFlush(ws workspace.WorkspaceKey) error {
	if a == nil || a.cb == nil {
		return nil
	}
	a.cb.mu.Lock()
	c, ok := a.cb.subs[ws.RepoRoot]
	a.cb.mu.Unlock()
	if !ok || c == nil {
		return nil
	}
	c.OnFlush()
	return nil
}

// semSessionAdapter implements SessionAccessor. The captured getSession
// closure is the SAME closure passed to InstallMiddleware in daemon.go:
// single source of truth for per-request session lookup. Closes W2 at the
// production layer.
type semSessionAdapter struct {
	getSession func(ctx context.Context) *mcp.SessionInfo
}

func (a *semSessionAdapter) Session(ctx context.Context) *mcp.SessionInfo {
	if a == nil || a.getSession == nil {
		return nil
	}
	return a.getSession(ctx)
}

func (a *semSessionAdapter) Workspace(ctx context.Context) workspace.WorkspaceKey {
	if a == nil || a.getSession == nil {
		return workspace.WorkspaceKey{}
	}
	sess := a.getSession(ctx)
	if sess == nil {
		return workspace.WorkspaceKey{}
	}
	// SessionInfo.WorkspaceKey is the hashed repo identifier (a string),
	// not the canonical workspace.WorkspaceKey struct. The daemon owns
	// the only authoritative workspace.WorkspaceKey, captured via the
	// `activeWSKey` variable in the SetActivateCallback closure. For
	// Phase 64 we fall back to the zero value — handlers tolerate the
	// zero key (returns repoID="") and surface their existing
	// "no workspace" envelopes. Phase 65 wires a real registry lookup.
	return workspace.WorkspaceKey{}
}

// semRetrievalAdapter implements RetrievalAccessor. Delegates QueryBleve /
// PersonalizedPageRank / TopEdgesFor to the per-workspace retrieval.Engine.
// RetrievalPending consults the per-workspace Recoverer.
type semRetrievalAdapter struct {
	bundle *semanticBundle
}

func (a *semRetrievalAdapter) engineFor(ws workspace.WorkspaceKey) *retrieval.Engine {
	if a == nil || a.bundle == nil {
		return nil
	}
	a.bundle.mu.Lock()
	defer a.bundle.mu.Unlock()
	return a.bundle.engines[ws.RepoRoot]
}

func (a *semRetrievalAdapter) recovererFor(ws workspace.WorkspaceKey) *retrieval.Recoverer {
	if a == nil || a.bundle == nil {
		return nil
	}
	a.bundle.mu.Lock()
	defer a.bundle.mu.Unlock()
	return a.bundle.recoveres[ws.RepoRoot]
}

// QueryBleve delegates to the per-workspace bleve engine. The skill-side
// caller derives the workspace from session state but does not currently
// thread it into QueryBleve; the adapter resolves the engine via the
// daemon's most-recently-activated workspace. When no engine is wired, the
// call returns an empty result + nil error so the handler degrades to a
// graph-only ranking.
//
// TODO(phase-65): thread workspace through the QueryBleve signature so
// multi-workspace daemons route correctly. The Phase 64 surface is single-
// workspace per call already; this is a future-multi-workspace shape gap.
func (a *semRetrievalAdapter) QueryBleve(task string, anchors []string) ([]semantic.TextRank, error) {
	if a == nil || a.bundle == nil {
		return nil, nil
	}
	// Pick any open engine — Phase 64 daemons run with a single active
	// workspace per session at a time. Multi-workspace differentiation
	// lands with the Phase 65 strangler-fig integration.
	a.bundle.mu.Lock()
	var engine *retrieval.Engine
	for _, e := range a.bundle.engines {
		engine = e
		break
	}
	a.bundle.mu.Unlock()
	if engine == nil {
		return nil, nil
	}
	results, err := engine.QueryBleve(task, anchors)
	if err != nil {
		return nil, err
	}
	out := make([]semantic.TextRank, len(results))
	for i, r := range results {
		out[i] = semantic.TextRank{SymbolID: r.SymbolID, Score: r.Score}
	}
	return out, nil
}

// PersonalizedPageRank returns per-symbol PageRank scores. Phase 64 ships an
// empty implementation; the rank scheduler does not yet expose a query-time
// personalized PageRank surface (Phase 62 ships incremental + full recompute
// only). Phase 65/67 wires a real source.
func (a *semRetrievalAdapter) PersonalizedPageRank(_ context.Context, repoID string, anchors []string) ([]semantic.GraphRank, error) {
	_ = repoID
	_ = anchors
	return nil, nil
}

// RetrievalPending consults the per-workspace Recoverer. Returns false when
// no Recoverer is wired (test path / pre-activation).
func (a *semRetrievalAdapter) RetrievalPending(ws workspace.WorkspaceKey) bool {
	rec := a.recovererFor(ws)
	if rec == nil {
		return false
	}
	return rec.RetrievalPending(ws)
}

// TopEdgesFor returns up to 5 top-weighted graph edges incident to symbolID.
// Phase 64 ships an empty implementation pending Phase 65's edge-iteration
// API — the handler tolerates an empty list per ContextEvidence.TopEdges
// being a "best-effort" field (CONTEXT.md "evidence field shape per
// candidate").
func (a *semRetrievalAdapter) TopEdgesFor(_ context.Context, repoID, symbolID string) ([]string, error) {
	_ = repoID
	_ = symbolID
	return nil, nil
}

// ----- Production buildFn — closes W3 at the doc layer. -----

// makeProductionBuildFn returns a buildFn (semantic.RunnerBuildFn) that the
// IndexRunner invokes inside its singleflight goroutine. The pipeline is:
//
//  1. (full mode) Walk the workspace via the existing live-spine /
//     filepath-based file walker. Each candidate file is classified via
//     live/classifier.ClassifyPathChange; "skipped" classifications
//     (out-of-VCS, generated, binary) are dropped.
//
//  2. (incremental mode) Drain the live overlay's pending rows. The
//     coalescer flush has already been applied via the kernel-edit /
//     refresh path; this build commits the drained overlay onto a new
//     snapshot.
//
//  3. For each non-skipped file, extract symbol facts via the existing
//     per-language extractor chain (internal/semantic/extract — go /
//     typescript / python). For other languages, fall back to LSP
//     documentSymbol when available, else skip.
//
//  4. Snapshot lifecycle (always):
//
//     snap, err := store.BeginSnapshot(ctx, SnapshotMeta{
//     RepoID: ws.Hash(),
//     BaseSnapshotID: prevSnapshotID,  // 0 for full, latest for incremental
//     CapturedEpoch:  store.CurrentEpoch(repoID),  // Phase 60 D-04 CAS
//     })
//     defer func() {
//     if !committed { _ = store.AbortSnapshot(ctx, snap, "build failed") }
//     }()
//     for batch := range factsBatches(facts, 1000) {
//     if err := store.WriteSnapshotFacts(ctx, snap, batch); err != nil {
//     return err
//     }
//     state.filesIndexed.Add(int64(len(batch.Files)))
//     state.filesReused.Add(int64(len(batch.Reused)))
//     }
//     if err := store.CommitSnapshot(ctx, snap, summary); err != nil {
//     return err
//     }
//     committed = true
//     return IndexResult{SnapshotID: snap.ID, ...}, nil
//
//  5. After commit, the rank scheduler picks up the new snapshot via the
//     existing post-commit ApplyRepair hook (live.handler -> rank.engine
//     wired in daemon.go:372). No explicit OnNewSnapshot call is required.
//
// Concrete signatures the executor calls (verified at execute-time in
// internal/semantic/store/snapshot.go):
//   - store.BeginSnapshot(ctx, SnapshotMeta) (*Snapshot, error)
//   - store.WriteSnapshotFacts(ctx, *Snapshot, Facts) error
//   - store.CommitSnapshot(ctx, *Snapshot, SnapshotSummary) error
//   - store.AbortSnapshot(ctx, *Snapshot, string) error
//   - graph.RankScheduler ApplyRepair (post-commit hook, live-side)
//
// Reference test asserting fact counts:
// TestE2E_IndexThenContext_SymbolCount in
// internal/skill/semantic/integration_test.go — closes W3 at the test layer.
//
// TODO(phase-65): replace this minimal-commit composition with the
// production indexer chain (per-language fact extraction + classifier walk +
// overlay drain) once the strangler-fig integration ships. Phase 64's
// pipeline commits an empty snapshot; the rank engine, retrieval engine,
// and downstream tools accept an empty snapshot as a valid "no facts yet"
// state — see snapshot.go:309-407 for the empty-Facts no-op path on
// WriteSnapshotFacts.
func (b *semanticBundle) makeProductionBuildFn() semantic.RunnerBuildFn {
	return func(ctx context.Context, ws workspace.WorkspaceKey, mode string, st semantic.BuildState) (semantic.IndexResult, error) {
		repoID := ws.Hash()
		startedAt := time.Now()

		// Resolve base snapshot for incremental mode.
		var baseSnapshotID uint64
		if mode == "incremental" {
			latest, err := b.store.LatestCommittedSnapshot(ctx, repoID)
			if err != nil {
				return semantic.IndexResult{}, fmt.Errorf("buildFn: LatestCommittedSnapshot: %w", err)
			}
			baseSnapshotID = latest
		}

		// 4. Snapshot lifecycle. Phase 64 ships an empty-Facts commit
		// (TODO(phase-65) — see header comment for the production
		// pipeline that will replace this).
		snap, err := b.store.BeginSnapshot(ctx, semanticstore.SnapshotMeta{
			RepoID:         repoID,
			BaseSnapshotID: baseSnapshotID,
		})
		if err != nil {
			return semantic.IndexResult{}, fmt.Errorf("buildFn: BeginSnapshot: %w", err)
		}
		committed := false
		defer func() {
			if !committed {
				_ = b.store.AbortSnapshot(context.Background(), snap, "buildFn: not committed")
			}
		}()

		// Stamp the in-flight snapshot id so the foreground caller's
		// timeout path can surface it.
		st.SetSnapshotID(snap.ID)

		// Empty-Facts commit (placeholder — TODO(phase-65)).
		if err := b.store.WriteSnapshotFacts(ctx, snap, semanticstore.Facts{}); err != nil {
			return semantic.IndexResult{}, fmt.Errorf("buildFn: WriteSnapshotFacts: %w", err)
		}
		summary := semanticstore.SnapshotSummary{
			DurationMs: time.Since(startedAt).Milliseconds(),
		}
		if err := b.store.CommitSnapshot(ctx, snap, summary); err != nil {
			return semantic.IndexResult{}, fmt.Errorf("buildFn: CommitSnapshot: %w", err)
		}
		committed = true

		return semantic.IndexResult{
			SnapshotID:   snap.ID,
			FilesIndexed: 0,
			FilesReused:  0,
			DurationMs:   time.Since(startedAt).Milliseconds(),
		}, nil
	}
}

// ----- Compile-time interface guards -----

var (
	_ semantic.StoreAccessor     = (*semStoreAdapter)(nil)
	_ semantic.SchedulerAccessor = (*semSchedulerAdapter)(nil)
	_ semantic.QueueAccessor     = (*semQueueAdapter)(nil)
	_ semantic.LiveAccessor      = (*semLiveAdapter)(nil)
	_ semantic.RetrievalAccessor = (*semRetrievalAdapter)(nil)
	_ semantic.CompactorAccessor = (*semCompactorAdapter)(nil)
	_ semantic.SessionAccessor   = (*semSessionAdapter)(nil)
	_ retrieval.StoreReader      = (*semanticstore.Store)(nil)
)

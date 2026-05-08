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
//   - makeProductionBuildFn — the production pipeline consumed by
//     IndexRunner: walk → live.ClassifyPathChange → per-language
//     provider.Extract → factsFromExtracted (composes extract.ToStoreFacts
//     plus per-file FileID / NodeID assignment) → store.WriteSnapshotFacts
//     → CommitSnapshot. Phase 65 (65-01) replaced the Phase 64 empty-Facts
//     placeholder with this pipeline (D-09 carryover #1).
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
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/agenthands/helix/internal/mcp"
	"github.com/agenthands/helix/internal/obs"
	semanticpkg "github.com/agenthands/helix/internal/semantic"
	"github.com/agenthands/helix/internal/semantic/extract"
	"github.com/agenthands/helix/internal/semantic/graph"
	"github.com/agenthands/helix/internal/semantic/integ"
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

	// wsKeyFn is the daemon's active-workspace closure (the same one
	// already passed to symbols.RegisterTools / edit.RegisterTools /
	// fileops.RegisterTools at internal/daemon/daemon.go:524). Closes
	// Phase 64 carryover D-09 #2: semSessionAdapter.Workspace returns
	// a real workspace.WorkspaceKey instead of the zero value, so
	// retrieval queries via the session adapter are workspace-scoped
	// (RESEARCH §Pattern 3 a). Multi-workspace expansion will swap
	// this closure for a real registry without changing the field shape.
	wsKeyFn func() workspace.WorkspaceKey

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
	wsKeyFn func() workspace.WorkspaceKey,
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
		wsKeyFn:         wsKeyFn,
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
	return &semSessionAdapter{getSession: b.getSession, wsKeyFn: b.wsKeyFn}
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
	wsKeyFn    func() workspace.WorkspaceKey
}

func (a *semSessionAdapter) Session(ctx context.Context) *mcp.SessionInfo {
	if a == nil || a.getSession == nil {
		return nil
	}
	return a.getSession(ctx)
}

// Workspace returns the daemon's active workspace key via the wsKeyFn
// closure. nil-safe: a nil adapter or a nil closure returns the zero
// WorkspaceKey (T-65-02-01 mitigation). Closes Phase 64 carryover
// D-09 #2 — the unconditional zero return is gone. The closure source
// of truth is internal/daemon/daemon.go:524 (the same activeWSKey
// closure already passed to symbols/edit/fileops RegisterTools).
//
// Multi-workspace expansion will swap this closure for a real registry
// without changing the field shape; the closure-based pass-through is
// the simplest single-workspace implementation today.
func (a *semSessionAdapter) Workspace(_ context.Context) workspace.WorkspaceKey {
	if a == nil || a.wsKeyFn == nil {
		return workspace.WorkspaceKey{}
	}
	return a.wsKeyFn()
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

// ----- integ.SemanticLookup production adapter (Phase 65 65-03) -----

// integSemanticLookup adapts the daemon's semanticBundle to the
// integ.SemanticLookup interface (Phase 65 D-03). Read-only by contract:
// the M-readtier mitigation forbids any reference to the snapshot-write
// surface (BeginSnapshot / CommitSnapshot / AbortSnapshot /
// WriteSnapshotFacts / OnFlush / BumpGraphVersion) inside any method body
// here. The grep canary at internal/daemon/integ_lookup_test.go
// (TestIntegSemanticLookup_ReadTierCanary) blocks new write-method tokens
// from sneaking in.
//
// Joins the existing semStoreAdapter / semRetrievalAdapter family —
// composes them rather than re-deriving any state. Method bodies that need
// the underlying *semanticstore.Store / *semRetrievalAdapter / *rankBundle
// reach through the bundle pointer (l.bundle.store, l.bundle.scheduler,
// etc.) so per-workspace state stays coherent with the rest of the
// semantic surface.
//
// Phase 65 65-03 introduces the type with the correct shape and read-only
// method bodies; full ranking / expansion / validation logic ships in
// 65-05 (RankFiles + RankFromSeeds), 65-06 (ExpandFrom +
// ValidateCriticalEdges + SymbolID), and 65-07 (Status field-source
// completion). Until those waves land the methods return Phase 65 D-06
// "no committed snapshot" / "not yet implemented" sentinels — the
// surface is stable for the consumer adapters wired in 65-04.
type integSemanticLookup struct {
	bundle    *semanticBundle
	store     *semanticstore.Store
	retrieval *semRetrievalAdapter
	rank      *rankBundle
	enabledFn func() bool
	wsKeyFn   func() workspace.WorkspaceKey
}

// Available reports whether the lookup is configured AND the underlying
// store handle is live. False on nil receiver, nil enabledFn (treated as
// "not wired"), enabledFn()==false, or nil store. Per D-06, this never
// triggers background indexing.
func (l *integSemanticLookup) Available() bool {
	if l == nil {
		return false
	}
	if l.enabledFn == nil || !l.enabledFn() {
		return false
	}
	return l.store != nil
}

// SymbolID translates an LSP file:line:col location into a stable Phase 59
// EXTRACT-02 SymbolID (Open Question #3 resolution). 65-03 stub returns
// ErrNoSnapshot until 65-06 wires the canonical reader; the interface
// shape and error contract are stable.
func (l *integSemanticLookup) SymbolID(_ context.Context, _ workspace.WorkspaceKey, _ string, _, _ uint32) (integ.SymbolID, error) {
	if !l.Available() {
		return integ.SymbolID(""), integ.ErrIndexErrored
	}
	return integ.SymbolID(""), integ.ErrNoSnapshot
}

// RankFiles returns the workspace-wide ranked file list for the default
// "call_graph" projection. 65-03 stub returns ErrNoSnapshot; 65-05 wires
// the persisted-score reader (RESEARCH Open Question #2 resolution).
func (l *integSemanticLookup) RankFiles(_ context.Context, _ workspace.WorkspaceKey) ([]integ.RankedFile, error) {
	if !l.Available() {
		return nil, integ.ErrIndexErrored
	}
	return nil, integ.ErrNoSnapshot
}

// RankFromSeeds returns a ranked file list biased toward seeds. 65-03 stub
// returns ErrNoSnapshot; 65-05 wires the bleve + RRF fuse pipeline.
func (l *integSemanticLookup) RankFromSeeds(_ context.Context, _ workspace.WorkspaceKey, _ []string) ([]integ.RankedFile, error) {
	if !l.Available() {
		return nil, integ.ErrIndexErrored
	}
	return nil, integ.ErrNoSnapshot
}

// ExpandFrom returns the depth-bounded blast-radius frontier rooted at
// sym. 65-03 stub returns ErrNoSnapshot; 65-06 wires BFS over
// QueryEffectiveAdjacency with the Phase 62 confidence ladder.
func (l *integSemanticLookup) ExpandFrom(_ context.Context, _ workspace.WorkspaceKey, _ integ.SymbolID, _ int) ([]integ.Impact, error) {
	if !l.Available() {
		return nil, integ.ErrIndexErrored
	}
	return nil, integ.ErrNoSnapshot
}

// ValidateCriticalEdges runs Pass 2 LSP validation. 65-03 stub returns
// the input edges with LSPConfirmed=false (M-readtier-safe pass-through —
// no LSP traffic, no confidence change). 65-06 ships the real validator.
func (l *integSemanticLookup) ValidateCriticalEdges(_ context.Context, _ workspace.WorkspaceKey, edges []integ.Edge) ([]integ.ValidatedEdge, error) {
	if !l.Available() {
		return nil, integ.ErrIndexErrored
	}
	out := make([]integ.ValidatedEdge, 0, len(edges))
	for _, e := range edges {
		out = append(out, integ.ValidatedEdge{Edge: e, LSPConfirmed: false})
	}
	return out, nil
}

// Status returns a closed-shape SemanticStatus. 65-03 wires every field
// that already has a read-only accessor — store snapshot id and graph
// version, overlay-pending, queue depth, last-flush. PendingLSP routes
// through the existing semQueueAdapter; LastLiveUpdateMs through
// semLiveAdapter. LastErrorReason is empty in the steady state and is
// populated by 65-07 once the error-stamp accessor lands. The closed-enum
// State is StatusReady when the lookup is available (consumers map to
// StatusDisabled on Available()==false themselves via the source-selection
// step — Pitfall §3).
func (l *integSemanticLookup) Status(ctx context.Context, ws workspace.WorkspaceKey) (integ.SemanticStatus, error) {
	if !l.Available() {
		return integ.SemanticStatus{State: integ.StatusDisabled, Store: "duckdb"}, nil
	}
	repoID := ws.Hash()
	snap, err := l.store.LatestCommittedSnapshot(ctx, repoID)
	if err != nil {
		return integ.SemanticStatus{State: integ.StatusError, Store: "duckdb"}, integ.ErrIndexErrored
	}
	gv, err := l.store.CurrentGraphVersion(ctx, repoID)
	if err != nil {
		return integ.SemanticStatus{State: integ.StatusError, Store: "duckdb"}, integ.ErrIndexErrored
	}
	overlay := l.store.OverlayHasPendingRows(repoID)

	var pendingLSP int
	var lastLiveMs int64
	if l.bundle != nil {
		if l.bundle.queue != nil {
			pendingLSP = l.bundle.queue.DepthAll()
		}
		if l.bundle.live != nil {
			if t := l.bundle.live.LastFlushAt(ws); !t.IsZero() {
				lastLiveMs = t.UnixMilli()
			}
		}
	}

	state := integ.StatusReady
	if snap == 0 {
		state = integ.StatusBuilding
	}
	return integ.SemanticStatus{
		State:            state,
		Store:            "duckdb",
		LatestSnapshotID: snap,
		GraphVersion:     gv,
		OverlayActive:    overlay,
		PendingLSP:       pendingLSP,
		LastLiveUpdateMs: lastLiveMs,
		LastErrorReason:  "",
	}, nil
}

// integLookupAccessor returns a SemanticLookup adapter wired to the bundle.
// Returns nil on nil receiver. Phase 65 65-04 / 65-05 / 65-06 / 65-07 will
// SetSemanticLookup on consumer skills (RepoMapSkill, kernel/symbols
// blast-radius bridge, kernel/health) using this accessor.
func (b *semanticBundle) integLookupAccessor() integ.SemanticLookup {
	if b == nil {
		return integ.NoopLookup{}
	}
	return &integSemanticLookup{
		bundle:    b,
		store:     b.store,
		retrieval: b.retrievalAdapter,
		rank:      b.scheduler,
		enabledFn: func() bool { return b.store != nil },
		wsKeyFn:   b.wsKeyFn,
	}
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
// Phase 65 (65-01) replaces the empty-Facts placeholder with the real
// production pipeline (D-09 carryover #1): walk → ClassifyPathChange →
// per-language Extract → ToStoreFacts → WriteSnapshotFacts. Per-file
// extraction failures are non-fatal (RESEARCH.md §Pattern 4): the build
// commits whatever extracted successfully; total failure still commits an
// empty snapshot, matching Phase 64 behavior.
func (b *semanticBundle) makeProductionBuildFn() semantic.RunnerBuildFn {
	return func(ctx context.Context, ws workspace.WorkspaceKey, mode string, st semantic.BuildState) (semantic.IndexResult, error) {
		repoID := ws.Hash()
		startedAt := time.Now()

		// 1. Resolve base snapshot for incremental mode.
		var baseSnapshotID uint64
		if mode == "incremental" {
			latest, err := b.store.LatestCommittedSnapshot(ctx, repoID)
			if err != nil {
				return semantic.IndexResult{}, fmt.Errorf("buildFn: LatestCommittedSnapshot: %w", err)
			}
			baseSnapshotID = latest
		}

		// 2. Walk (full mode) or drain overlay (incremental mode) to build
		//    the candidate path set, then run the per-path classify + extract
		//    loop. Per-file errors are non-fatal (research §Pattern 4
		//    acceptable error behavior).
		paths := b.collectCandidatePaths(ws, mode)
		extracted := b.classifyAndExtract(ctx, repoID, paths)

		// 4. Convert per-language facts to the locked store wire format.
		//    ToStoreFacts is the Phase 65 D-08 unblock adapter; it leaves
		//    FileID / NodeID / RefID zero-valued by contract. The buildFn
		//    composes ToStoreFacts in a per-file loop here so per-file
		//    symbols / references can be re-stamped with the file's
		//    assigned FileID — preserving the (snapshot_id, file_id) and
		//    (snapshot_id, symbol_id / ref_id) primary-key invariants.
		facts := factsFromExtracted(extracted, repoID)

		// 5. Snapshot lifecycle.
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

		if err := b.store.WriteSnapshotFacts(ctx, snap, facts); err != nil {
			return semantic.IndexResult{}, fmt.Errorf("buildFn: WriteSnapshotFacts: %w", err)
		}
		st.AddFilesIndexed(int64(len(extracted)))

		summary := semanticstore.SnapshotSummary{
			FileCount:      len(facts.Files),
			SymbolCount:    len(facts.Symbols),
			ReferenceCount: len(facts.References),
			DurationMs:     time.Since(startedAt).Milliseconds(),
		}
		if err := b.store.CommitSnapshot(ctx, snap, summary); err != nil {
			return semantic.IndexResult{}, fmt.Errorf("buildFn: CommitSnapshot: %w", err)
		}
		committed = true

		return semantic.IndexResult{
			SnapshotID:   snap.ID,
			FilesIndexed: int64(len(extracted)),
			FilesReused:  0,
			DurationMs:   time.Since(startedAt).Milliseconds(),
		}, nil
	}
}

// collectCandidatePaths builds the candidate path set the production buildFn
// classifies + extracts:
//
//   - mode=full: walk ws.RepoRoot via filepath.WalkDir, skipping the
//     `.helix/` and `.git/` subtrees plus all dot-directories. Returns
//     absolute paths to regular files only (no directories, no symlinks).
//   - mode=incremental: today the live overlay does not expose an
//     enumerable per-path drain surface (Phase 60 bumped pending counters
//     but kept the per-path set internal to the coalescer). Until that
//     surface lands, fall back to the same full-walk path so an
//     incremental request still produces a non-empty Facts commit when
//     the workspace has source files. The overlay-drain optimization is
//     a follow-up.
func (b *semanticBundle) collectCandidatePaths(ws workspace.WorkspaceKey, _ string) []string {
	if ws.RepoRoot == "" {
		return nil
	}
	var paths []string
	_ = filepath.WalkDir(ws.RepoRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			// Per-entry walk error: skip the entry, keep walking siblings.
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		// Skip well-known generated / VCS / helix-local subtrees.
		name := d.Name()
		if d.IsDir() {
			if path != ws.RepoRoot && (name == ".git" || name == ".helix" || strings.HasPrefix(name, ".") && name != ".") {
				return fs.SkipDir
			}
			return nil
		}
		// Reject symlinks at enumeration time (T-60-04-03 mitigation; the
		// classifier double-checks downstream).
		if d.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		// Reject anything that's not a regular file.
		if !d.Type().IsRegular() {
			return nil
		}
		paths = append(paths, path)
		return nil
	})
	return paths
}

// langFromExt maps a file path to a canonical extract.Provider language
// identifier. Returns "" for unrecognized extensions; the buildFn loop
// treats that as "skip silently". The mapping mirrors the per-language
// providers registered by the daemon at bootstrap (Phase 59 P05): Go,
// TypeScript / TSX, JavaScript / JSX, Python.
func langFromExt(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go":
		return "go"
	case ".ts", ".tsx":
		return "typescript"
	case ".js", ".jsx":
		return "javascript"
	case ".py":
		return "python"
	default:
		return ""
	}
}

// classifyAndExtract runs the per-path classify + extract loop. For each
// candidate path it asks live.ClassifyPathChange whether the path is a
// real source file (skipping deletes / unknown / lookup-error paths), maps
// the extension to a language, resolves the per-language provider, reads
// the source bytes, and calls provider.Extract.
//
// Per-file errors are non-fatal (research §Pattern 4 acceptable error
// behavior): read failures, classifier errors, and per-extractor failures
// log at debug and the loop continues. Total failure (zero successful
// extractions) returns an empty slice; the caller still commits an empty
// snapshot, matching Phase 64 behavior.
//
// The FileHashLookup is the same storeFileHashLookup adapter the live
// bundle uses (live_wiring.go:156-164) so the classifier sees a
// consistent view of the snapshot+overlay store.
func (b *semanticBundle) classifyAndExtract(ctx context.Context, repoID string, paths []string) []*extract.ExtractedFile {
	if len(paths) == 0 || b.extractRegistry == nil {
		return nil
	}
	hashLookup := &storeFileHashLookup{store: b.store}
	var extracted []*extract.ExtractedFile
	for _, path := range paths {
		kind, ok, err := live.ClassifyPathChange(
			ctx,
			semanticpkg.RepoID(repoID),
			path,
			hashLookup,
			nil, // hasher: classifier falls back to default
			live.ChangeSourceFsnotify,
		)
		if err != nil || !ok {
			continue
		}
		if kind == live.ChangeFileDeleted {
			// Deletion: nothing to extract; the snapshot's effective graph
			// drops the file via the BaseSnapshotID lineage.
			continue
		}
		lang := langFromExt(path)
		if lang == "" {
			continue
		}
		provider, ok := b.extractRegistry.Provider(lang)
		if !ok {
			// Language not first-class today; skip silently. Phase 59
			// emits partial=true for these via the scheduler path; the
			// production buildFn keeps the loop simple.
			continue
		}
		source, err := os.ReadFile(path)
		if err != nil {
			if b.logger != nil {
				b.logger.Debug("buildFn: read source failed; skipping path",
					"path", path, "err", err)
			}
			continue
		}
		ef, err := provider.Extract(ctx, source, extract.SourceFile{
			Path:     path,
			Language: lang,
		})
		if err != nil {
			if b.logger != nil {
				b.logger.Debug("buildFn: provider.Extract failed; skipping path",
					"path", path, "lang", lang, "err", err)
			}
			continue
		}
		if ef == nil {
			continue
		}
		extracted = append(extracted, ef)
	}
	return extracted
}

// factsFromExtracted composes the wire-format Facts payload from a slice of
// per-language ExtractedFile records. Each ExtractedFile is converted via
// extract.ToStoreFacts (the locked Phase 65 D-08 adapter) and then re-
// stamped with:
//
//   - a per-file FileID allocated as a 1-based index (so the
//     (snapshot_id, file_id) PK is dense and collision-free within the
//     snapshot), plus the snapshot's RepoID on every FileFact.
//   - the same FileID propagated onto every Symbol and Reference belonging
//     to that file (so the symbols/references rows reference a real
//     semantic_files row at the schema level — this is the linkage that
//     ToStoreFacts intentionally drops because the wire shape is flat).
//   - per-symbol NodeID = SymbolID (Schema 5 has no separate node-id
//     allocator at this layer; the kernel-side node-id model collapses to
//     symbol-id for the symbol-anchored rows the buildFn emits today).
//   - per-reference RefID and NodeID derived from a 1-based incrementing
//     index, scoped to the snapshot. References extracted by the per-
//     language providers carry zero IDs (extract.ToStoreFacts leaves them
//     at INSERT time per its contract); the buildFn assigns them here.
//
// Returns a Facts value safe to pass directly to *Store.WriteSnapshotFacts.
func factsFromExtracted(extracted []*extract.ExtractedFile, repoID string) semanticstore.Facts {
	if len(extracted) == 0 {
		return semanticstore.Facts{}
	}
	out := semanticstore.Facts{
		Files:      make([]semanticstore.FileFact, 0, len(extracted)),
		Symbols:    make([]semanticstore.SymbolFact, 0, 4*len(extracted)),
		References: make([]semanticstore.ReferenceFact, 0, 4*len(extracted)),
	}
	var refSeq uint64
	for i, ef := range extracted {
		if ef == nil {
			continue
		}
		fileID := uint64(i + 1)
		// Reuse ToStoreFacts for the field-by-field translation, then re-
		// stamp the IDs / RepoID. Wrapping a single ExtractedFile keeps the
		// per-file linkage between symbols/references and their owning file.
		single := extract.ToStoreFacts([]*extract.ExtractedFile{ef})
		for j := range single.Files {
			single.Files[j].FileID = fileID
			single.Files[j].RepoID = repoID
		}
		for j := range single.Symbols {
			single.Symbols[j].FileID = fileID
			// Mask SymbolID / NodeID to 63 bits — the duckdb-go driver
			// rejects uint64 values with the high bit set. Mirrors the
			// overlay edge-id helper at internal/semantic/store/overlay.go:937.
			single.Symbols[j].SymbolID &= 0x7FFFFFFFFFFFFFFF
			if single.Symbols[j].NodeID == 0 {
				single.Symbols[j].NodeID = single.Symbols[j].SymbolID
			} else {
				single.Symbols[j].NodeID &= 0x7FFFFFFFFFFFFFFF
			}
			single.Symbols[j].OwnerSymbolID &= 0x7FFFFFFFFFFFFFFF
			single.Symbols[j].ParentScopeID &= 0x7FFFFFFFFFFFFFFF
		}
		for j := range single.References {
			single.References[j].FileID = fileID
			refSeq++
			single.References[j].RefID = refSeq
			if single.References[j].NodeID == 0 {
				single.References[j].NodeID = refSeq
			}
		}
		out.Files = append(out.Files, single.Files...)
		out.Symbols = append(out.Symbols, single.Symbols...)
		out.References = append(out.References, single.References...)
	}
	return out
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
	_ integ.SemanticLookup       = (*integSemanticLookup)(nil)
)

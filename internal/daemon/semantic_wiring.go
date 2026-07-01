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
// Closes W1 (production layer): semSchedulerAdapter.ClusterStatus
// delegates to NewSchedulerAccessorForStore (Plan 69-05) which derives
// the closed-enum state from *Store.ClusterStatusForGraphVersion +
// CurrentGraphVersion; semRetrievalAdapter.RetrievalStatus delegates to
// NewRetrievalAccessorForStore which reads bleve corpus-state meta.
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
	"sort"
	"strings"
	"sync"
	"time"

	serr "github.com/agenthands/helix/internal/errors"
	"github.com/agenthands/helix/internal/mcp"
	"github.com/agenthands/helix/internal/obs"
	semanticpkg "github.com/agenthands/helix/internal/semantic"
	"github.com/agenthands/helix/internal/semantic/classifier"
	"github.com/agenthands/helix/internal/semantic/cochange"
	"github.com/agenthands/helix/internal/semantic/crossrepo"
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
	// TypeResolMaxFixpoint / TypeResolMinConfidence / TypeResolEmitUnresolved
	// are the v2.12 Phase 136 type-resolution knobs, mirroring
	// TypeResolutionConfig.{MaxFixpointIterations, MinConfidenceForEdge,
	// EmitUnresolvedEdges}. Inlined here (no koanf surface yet) so the batch
	// type-edge driver in factsFromExtracted has its parameters.
	TypeResolMaxFixpoint    int
	TypeResolMinConfidence  float64
	TypeResolEmitUnresolved bool
}

// loadSemanticConfig returns the daemon-side semantic config with defaults.
// Phase 64 ships defaults only; no koanf surface yet.
func loadSemanticConfig() semanticConfig {
	return semanticConfig{
		BleveSubdir:             ".helix/semantic.bleve",
		IndexTimeout:            120 * time.Second,
		TypeResolMaxFixpoint:    8,
		TypeResolMinConfidence:  0.45,
		TypeResolEmitUnresolved: false,
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
	// repoReg tracks indexed repos for cross-repo (CROSS_IMPORTS) resolution.
	// Multi-repo awareness without the activeWSKey refactor (in-memory; the
	// buildFn registers each repo as it indexes).
	repoReg *multiRepoRegistry

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

	// lastErrReason carries the most recent build/live/overlay-flush
	// closed-enum FallbackReason; set via SetLastErrorReason at every
	// daemon-side error path that owns a bundle pointer (Phase 65 65-11
	// Task 2 — IN-04 / WR-2). Read by integSemanticLookup.Status under
	// bundle.mu so kernel/health surfaces the closed-enum reason on
	// get_health.semantic_index.last_error.
	lastErrReason integ.FallbackReason

	// collectCandidatePathsHook is a test-only seam (Phase 70-06) fired
	// once per collectCandidatePaths return with the final candidate slice.
	// nil in production. Read under mu so concurrent reset-during-collect
	// races stay benign. Setting via SetCollectCandidatePathsHook.
	collectCandidatePathsHook func(paths []string)
}

// SetCollectCandidatePathsHook installs a test-only observer fired once per
// collectCandidatePaths return with the final candidate slice. Nil-safe;
// production callers never set it. Phase 70-06: integration tests use this
// seam to assert "1 file edited → exactly 1 candidate path returned" end-
// to-end through the dispatcher.
func (b *semanticBundle) SetCollectCandidatePathsHook(fn func(paths []string)) {
	if b == nil {
		return
	}
	b.mu.Lock()
	b.collectCandidatePathsHook = fn
	b.mu.Unlock()
}

// fireCollectCandidatePathsHook fires the registered test hook (if any) with
// the slice the dispatcher is about to return. Reads the hook under b.mu so
// SetCollectCandidatePathsHook races stay benign.
func (b *semanticBundle) fireCollectCandidatePathsHook(paths []string) {
	if b == nil {
		return
	}
	b.mu.Lock()
	fn := b.collectCandidatePathsHook
	b.mu.Unlock()
	if fn != nil {
		fn(paths)
	}
}

// SetLastErrorReason stamps the bundle's lastErrReason field with a closed-enum
// FallbackReason. Called by every daemon-side build/live/overlay-flush error
// path so integSemanticLookup.Status surfaces a stable closed-enum reason
// (NOT raw error text) on the get_health envelope. Pass "" to clear after a
// successful commit. Nil-safe.
//
// Phase 65 65-11 Task 2 — IN-04 / WR-2.
func (b *semanticBundle) SetLastErrorReason(r integ.FallbackReason) {
	if b == nil {
		return
	}
	b.mu.Lock()
	b.lastErrReason = r
	b.mu.Unlock()
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
	effSemanticDisabled bool,
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
		repoReg:         newMultiRepoRegistry(),
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
	// Phase 81 ABLATE-06 / Pitfall 3 (A5): under the gate, SKIP the entire
	// Set*Accessor block so the direct-DuckDB-read SemanticSkill tools
	// (find_related_symbols, explain_symbol_deep, validate_graph_edge,
	// cluster/impact) and the SetImpactLookup ExpandFrom back-channel get nil
	// accessors. These tools bypass integ.ChooseSource, so the cfgGate gate is
	// insufficient — leaving the accessors nil is the structural block (the
	// handlers already nil-guard). This closes the THIRD integLookupAccessor
	// hand-out (the other three are gated in daemon.go: symbols, repomap,
	// guardrail). The bundle/store itself stays built (D-04 build-but-block).
	b.skill = semantic.GetSemanticSkill()
	if b.skill != nil && !effSemanticDisabled {
		b.skill.SetStore(storeAcc)
		b.skill.SetScheduler(b.schedulerAccessor())
		b.skill.SetQueue(b.queueAccessor())
		b.skill.SetLive(b.liveAccessor())
		b.skill.SetRunner(b.runner)
		b.skill.SetRetrieval(b.retrievalAdapter)
		b.skill.SetCompactor(b.compactorAccessor())
		b.skill.SetSessionAccessor(b.sessionAccessor())
		b.skill.SetSymbolByName(b.symbolByNameAccessor())
		b.skill.SetExtractorRun(b.extractorRunAccessor())
		b.skill.SetClusterMap(b.clusterMapAccessor())
		b.skill.SetClusterMember(b.clusterMemberAccessor())
		b.skill.SetClusterPageRank(b.clusterPageRankAccessor())
		b.skill.SetImpactLookup(b.integLookupAccessor())
		b.skill.SetSymbolEdges(b.symbolEdgesAccessor())
		b.skill.SetDataFlowReachability(b.dataFlowReachabilityAccessor())
		b.skill.SetClusterMembership(b.clusterMembershipAccessor())
		// TypeChain (SetTypeChain) and EdgeEvidence (SetEdgeEvidence) deferred to Phase 75
		// — schema columns absent in Schema v6 (no tree_sitter_kind / tier / evidence_kind).
		if logger != nil {
			logger.Info("semantic skill setters wired",
				"setters", 15,
				"bleve_subdir", cfg.BleveSubdir,
				"index_timeout", cfg.IndexTimeout,
			)
		}
	} else if b.skill != nil {
		// Phase 81 ABLATE-06 / Pitfall 5: under the gate, EXPLICITLY clear every
		// accessor to nil rather than merely skipping the setters. SemanticSkill
		// is a process-global singleton (semantic.GetSemanticSkill()); a prior
		// non-gated daemon construction in the same process would otherwise leave
		// a STALE real accessor wired. The idempotent null-object reset (mirroring
		// the repomap SetSemanticLookup(nil) reset in daemon.go) is the structural
		// block that guarantees the direct-read tools have no read path.
		b.skill.SetStore(nil)
		b.skill.SetScheduler(nil)
		b.skill.SetQueue(nil)
		b.skill.SetLive(nil)
		b.skill.SetRunner(nil)
		b.skill.SetRetrieval(nil)
		b.skill.SetCompactor(nil)
		b.skill.SetSessionAccessor(nil)
		b.skill.SetSymbolByName(nil)
		b.skill.SetExtractorRun(nil)
		b.skill.SetClusterMap(nil)
		b.skill.SetClusterMember(nil)
		b.skill.SetClusterPageRank(nil)
		b.skill.SetImpactLookup(nil)
		b.skill.SetSymbolEdges(nil)
		b.skill.SetDataFlowReachability(nil)
		b.skill.SetClusterMembership(nil)
		if logger != nil {
			logger.Info("semantic skill accessors gated off (effSemanticDisabled)",
				"setters", 0,
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
	return &semSchedulerAdapter{rb: b.scheduler, store: b.store}
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

// CurrentOverlayEpoch delegates to *Store.CurrentOverlayEpoch (Phase 63
// pattern; Phase 70-04 seam). Returns (0, nil) on nil-adapter / nil-store
// to mirror the cold-start signal the upstream accessor uses.
func (a *semStoreAdapter) CurrentOverlayEpoch(ctx context.Context, repoID string) (uint64, error) {
	if a == nil || a.store == nil {
		return 0, nil
	}
	return a.store.CurrentOverlayEpoch(ctx, repoID)
}

// OverlayChangedPathsSince delegates to *Store.OverlayChangedPathsSince
// (Plan 70-01). Returns (nil, 0, nil) on nil-adapter / nil-store — the
// caller treats that as cold-start + empty drain (full-walk fallback).
func (a *semStoreAdapter) OverlayChangedPathsSince(ctx context.Context, repoID string, baseEpoch uint64) (paths []string, currentEpoch uint64, err error) {
	if a == nil || a.store == nil {
		return nil, 0, nil
	}
	return a.store.OverlayChangedPathsSince(ctx, repoID, baseEpoch)
}

// LatestCommittedSnapshotBaseEpoch delegates to
// *Store.LatestCommittedSnapshotBaseEpoch (Plan 70-02). Returns
// (0, false, nil) on nil-adapter / nil-store — same cold-start signal the
// upstream accessor returns when no committed snapshot exists.
func (a *semStoreAdapter) LatestCommittedSnapshotBaseEpoch(ctx context.Context, repoID string) (epoch uint64, ok bool, err error) {
	if a == nil || a.store == nil {
		return 0, false, nil
	}
	return a.store.LatestCommittedSnapshotBaseEpoch(ctx, repoID)
}

// semSchedulerAdapter wraps the rank bundle plus the semantic store.
// IsQuiescent reads from the rank bundle directly; ClusterStatus
// delegates to NewSchedulerAccessorForStore (Plan 69-05) so the daemon
// and the Plan 69-06 E2E test share a single derivation code path.
type semSchedulerAdapter struct {
	rb    *rankBundle
	store *semanticstore.Store
}

func (a *semSchedulerAdapter) IsQuiescent(repoID string) bool {
	if a == nil || a.rb == nil {
		return true
	}
	// WR-06: lock held through IsQuiescent. The bundle mutex is released
	// only AFTER the per-sub IsQuiescent() read returns. Rationale (REVIEW
	// option A): IsQuiescent is a cheap atomic-bool read that does not
	// block, so the slight extension of the bundle mutex's hold time is
	// safe; the alternative (release-then-call) raced with subs[repoID]
	// being mutated concurrently and could deref a freed sub.
	a.rb.mu.Lock()
	defer a.rb.mu.Unlock()
	s, ok := a.rb.subs[repoID]
	if !ok || s == nil {
		return true
	}
	return s.IsQuiescent()
}

// ScoreStatus returns the closed-enum score status for (repoID, projection).
// ScoreStatus pending Phase 65/67 per-projection accessor (RESEARCH Q9);
// returns graph.ScoreStatusMissing as the closed-enum default — downstream
// consumers (tools_status.go) treat "missing" as the safe pre-data state.
func (a *semSchedulerAdapter) ScoreStatus(repoID, projection string) graph.ScoreStatus {
	_ = repoID
	_ = projection
	return graph.ScoreStatusMissing
}

// ClusterStatus delegates to the factory-produced accessor so the daemon
// and the Plan 69-06 E2E test share a single derivation code path. See
// semantic_accessor_factories.go for the full state-derivation logic.
func (a *semSchedulerAdapter) ClusterStatus(repoID string) semantic.ClusterStatus {
	return NewSchedulerAccessorForStore(a.store).ClusterStatus(repoID)
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

// FlushNow synchronously drains the coalescer's pending batch for ws,
// delegating to live.Service.FlushNow (Plan 70-03). Returns nil for
// nil-adapter / nil-live-service / unregistered workspace (the upstream
// service returns nil for unregistered workspaces; we just guard the
// pointer chain). Phase 70-04 seam.
func (a *semLiveAdapter) FlushNow(ctx context.Context, ws workspace.WorkspaceKey) error {
	if a == nil || a.live == nil || a.live.service == nil {
		return nil
	}
	return a.live.service.FlushNow(ctx, ws)
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

// RetrievalStatus delegates to the factory-produced accessor (Plan
// 69-05) so the daemon and the Plan 69-06 E2E test share a single
// derivation code path. The factory reads bleve corpus-state meta using
// the exported retrieval.MetaKey* constants and applies the closed-enum
// priority order (bleve-unavailable > corpus_version-uninitialized >
// corpus_version-lag > compactor-never-ran). See
// semantic_accessor_factories.go.
func (a *semRetrievalAdapter) RetrievalStatus(ws workspace.WorkspaceKey) semantic.RetrievalStatus {
	if a == nil || a.bundle == nil {
		return NewRetrievalAccessorForStore(nil, nil).RetrievalStatus(ws)
	}
	return NewRetrievalAccessorForStore(a.bundle.store, a.engineFor(ws)).RetrievalStatus(ws)
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
// EXTRACT-02 SymbolID. Phase 65 65-11 Task 2 — real implementation
// backed by *Store.QuerySymbolByLocation.
//
// Returns:
//   - (stableKey, nil) on hit at the latest committed snapshot.
//   - ("", integ.ErrNoSnapshot) when no snapshot covers (path, line, col)
//     (either no snapshot has been committed yet OR the cursor lies outside
//     every symbol's range — caller treats both as "fall through to LSP").
//   - ("", integ.ErrIndexErrored) when the lookup is unavailable.
//   - ("", wrapped store error) on a real SQL error.
//
// SymbolID at the integ boundary IS the stable_key (Phase 59 EXTRACT-02
// canonical identity); kernel callers pass it back through ExpandFrom and
// the orchestrator without ever round-tripping it through the store-side
// uint64 graph.NodeID surface.
func (l *integSemanticLookup) SymbolID(ctx context.Context, ws workspace.WorkspaceKey, path string, line, col uint32) (integ.SymbolID, error) {
	if !l.Available() {
		return integ.SymbolID(""), integ.ErrIndexErrored
	}
	repoID := ws.Hash()
	stableKey, ok, err := l.store.QuerySymbolByLocation(ctx, repoID, path, line, col)
	if err != nil {
		return integ.SymbolID(""), fmt.Errorf("integSemanticLookup.SymbolID: %w", err)
	}
	if !ok {
		return integ.SymbolID(""), integ.ErrNoSnapshot
	}
	return integ.SymbolID(stableKey), nil
}

// defaultRankProjection is the projection key consumed by RankFiles and
// RankFromSeeds. "call_graph" is Phase 62's primary projection — the
// kernel-side blast-radius / repomap consumers all read against it.
const defaultRankProjection = "call_graph"

// rrfFusionConstant is the standard reciprocal-rank-fusion (RRF) k
// parameter (Cormack & Clarke 2009). k=60 is the canonical value used
// across the IR literature; smaller values bias toward top-ranked
// items, larger toward broader coverage.
const rrfFusionConstant = 60.0

// RankFiles returns the workspace-wide ranked file list for the default
// "call_graph" projection. Phase 65 65-10 Task 2 — real implementation
// (was a 65-03 stub returning ErrNoSnapshot).
//
// Reads *Store.QueryRankedFiles for (repoID, "call_graph") at the latest
// committed snapshot. Returns ErrNoSnapshot when no scores exist (either
// the store is empty or the rank scheduler has not yet emitted scores
// for the latest commit).
func (l *integSemanticLookup) RankFiles(ctx context.Context, ws workspace.WorkspaceKey) ([]integ.RankedFile, error) {
	if !l.Available() {
		return nil, integ.ErrIndexErrored
	}
	repoID := ws.Hash()
	rows, err := l.store.QueryRankedFiles(ctx, repoID, defaultRankProjection, 0)
	if err != nil {
		return nil, fmt.Errorf("integSemanticLookup.RankFiles: %w", err)
	}
	if len(rows) == 0 {
		return nil, integ.ErrNoSnapshot
	}
	out := make([]integ.RankedFile, 0, len(rows))
	for _, r := range rows {
		out = append(out, integ.RankedFile{
			Path:         r.Path,
			Score:        r.Score,
			Projection:   defaultRankProjection,
			GraphVersion: r.GraphVersion,
		})
	}
	return out, nil
}

// RankFromSeeds returns a ranked file list biased toward seeds. Phase 65
// 65-10 Task 2 — real implementation (was a 65-03 stub returning
// ErrNoSnapshot).
//
// Algorithm: reciprocal rank fusion (RRF, k=60) over two ranked sources:
//
//  1. The persisted graph-score baseline from QueryRankedFiles for the
//     default "call_graph" projection.
//  2. The bleve-backed text-rank list from the per-workspace retrieval
//     engine, with `seeds` joined as the query string and passed verbatim
//     as anchors (anchored docs that ALSO match the query score higher).
//
// Each source contributes 1/(k + rank) per file; collisions ADD. Result
// sort: score DESC, path ASC.
//
// Empty seeds short-circuit to RankFiles. When BOTH sources return empty
// — either the persisted scores are absent AND bleve has no hits — the
// function returns ErrNoSnapshot.
func (l *integSemanticLookup) RankFromSeeds(ctx context.Context, ws workspace.WorkspaceKey, seeds []string) ([]integ.RankedFile, error) {
	if !l.Available() {
		return nil, integ.ErrIndexErrored
	}
	if len(seeds) == 0 {
		return l.RankFiles(ctx, ws)
	}
	repoID := ws.Hash()

	rows, err := l.store.QueryRankedFiles(ctx, repoID, defaultRankProjection, 0)
	if err != nil {
		return nil, fmt.Errorf("integSemanticLookup.RankFromSeeds: persisted: %w", err)
	}

	var textRanks []retrieval.TextRank
	if l.retrieval != nil {
		engine := l.retrieval.engineFor(ws)
		if engine != nil {
			tr, qErr := engine.QueryBleve(strings.Join(seeds, " "), seeds)
			if qErr != nil {
				return nil, integ.ErrBleveRebuilding
			}
			textRanks = tr
		}
	}

	if len(rows) == 0 && len(textRanks) == 0 {
		return nil, integ.ErrNoSnapshot
	}

	fused := rrfFuseFiles(ctx, l.store, repoID, rows, textRanks)

	out := make([]integ.RankedFile, 0, len(fused))
	for _, f := range fused {
		out = append(out, integ.RankedFile{
			Path:         f.Path,
			Score:        f.Score,
			Projection:   defaultRankProjection,
			GraphVersion: f.GraphVersion,
		})
	}
	return out, nil
}

// fusedRow is one entry in the RRF-fused output. GraphVersion is
// inherited from the persisted baseline when the path appears there;
// paths sourced exclusively from bleve text-rank carry GraphVersion=0
// (they are not anchored to a graph commit).
type fusedRow struct {
	Path         string
	Score        float64
	GraphVersion uint64
}

// rrfFuseFiles fuses (baseline persisted scores, bleve text-rank hits)
// via reciprocal rank fusion (k=60). Each input contributes 1/(k+rank)
// per appearance; collisions on the same path SUM. Bleve hits are
// translated to file paths via resolveSymbolPath (Phase 65 65-10 Task 0
// pin: TextRank.SymbolID is decimal symbol_id format → SQL JOIN on
// semantic_symbols.symbol_id at the latest committed snapshot).
//
// Sort: score DESC, then path ASC (matches the underlying QueryRankedFiles
// stable-key tiebreak).
func rrfFuseFiles(
	ctx context.Context,
	store *semanticstore.Store,
	repoID string,
	baseline []semanticstore.RankedFileRow,
	textRanks []retrieval.TextRank,
) []fusedRow {
	scoreByPath := make(map[string]*fusedRow, len(baseline)+len(textRanks))
	for i, r := range baseline {
		rank := i + 1
		scoreByPath[r.Path] = &fusedRow{
			Path:         r.Path,
			Score:        1.0 / (rrfFusionConstant + float64(rank)),
			GraphVersion: r.GraphVersion,
		}
	}
	// Resolve the latest committed snapshot for repoID once so the bleve
	// path-translation lookup can JOIN on (snapshot_id, symbol_id).
	// Failure (or empty snapshot) is non-fatal — fall back to baseline-only.
	var latestSnap uint64
	if len(textRanks) > 0 && store != nil {
		if snap, err := store.LatestCommittedSnapshot(ctx, repoID); err == nil {
			latestSnap = snap
		}
	}
	if latestSnap == 0 {
		// No snapshot to resolve against — emit baseline-only.
		out := make([]fusedRow, 0, len(scoreByPath))
		for _, fr := range scoreByPath {
			out = append(out, *fr)
		}
		sortFusedRows(out)
		return out
	}
	for i, tr := range textRanks {
		rank := i + 1
		path, ok := resolveSymbolPath(ctx, store, latestSnap, tr.SymbolID)
		if !ok {
			continue
		}
		contribution := 1.0 / (rrfFusionConstant + float64(rank))
		if existing, present := scoreByPath[path]; present {
			existing.Score += contribution
		} else {
			scoreByPath[path] = &fusedRow{Path: path, Score: contribution}
		}
	}
	out := make([]fusedRow, 0, len(scoreByPath))
	for _, fr := range scoreByPath {
		out = append(out, *fr)
	}
	sortFusedRows(out)
	return out
}

// sortFusedRows applies the (score DESC, path ASC) ordering used by every
// rrfFuseFiles return path.
func sortFusedRows(rows []fusedRow) {
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Score != rows[j].Score {
			return rows[i].Score > rows[j].Score
		}
		return rows[i].Path < rows[j].Path
	})
}

// resolveSymbolPath looks up the file path of a TextRank.SymbolID at the
// given snapshot. Phase 65 65-10 Task 0 BL-4 pin: TextRank.SymbolID is
// decimal symbol_id format (strconv.FormatUint(uint64, 10), set by
// retrieval/corpus.go via store.SymbolRow.SymbolID), so the lookup goes
// through *Store.QuerySymbolPath which parses the decimal and JOINs
// semantic_symbols on (snapshot_id, symbol_id).
//
// Returns ("", false) on miss or any non-nil error from the store.
func resolveSymbolPath(ctx context.Context, store *semanticstore.Store, snapshotID uint64, symbolID string) (string, bool) {
	if store == nil || snapshotID == 0 {
		return "", false
	}
	path, ok, err := store.QuerySymbolPath(ctx, snapshotID, symbolID)
	if err != nil || !ok {
		return "", false
	}
	return path, true
}

// ExpandFrom returns the depth-bounded blast-radius frontier rooted at sym.
// Phase 65 65-11 Task 2 — real implementation: BFS over the (repoID,
// "call_graph") effective adjacency to depth=2, mapping each edge's weight
// onto the Phase 62 closed-ladder confidence. Each visited node yields one
// integ.Impact whose Evidence carries the (from, to, kind, confidence) edge
// and whose SymbolID is the target's stable_key.
//
// Returns:
//   - ([]Impact, nil) — including ([]Impact{}, nil) when the seed has no
//     outbound edges (depth=0 frontier is empty by definition; the caller
//     treats a non-nil empty slice as "no expansion" rather than an error).
//   - (nil, integ.ErrIndexErrored) when Available()==false.
//   - (nil, integ.ErrNoSnapshot) when the seed sym is not present in the
//     latest committed snapshot.
//
// The seed itself is NOT emitted as an Impact (the orchestrator treats the
// seed cursor specially); only neighbors at distance >= 1 from the seed are.
//
// depth <= 0 is normalized to the default (2). depth > 2 still works but is
// a SemanticLookup contract violation upstream — the orchestrator caps at
// blastRadiusExpansionDepth.
func (l *integSemanticLookup) ExpandFrom(ctx context.Context, ws workspace.WorkspaceKey, sym integ.SymbolID, depth int) ([]integ.Impact, error) {
	if !l.Available() {
		return nil, integ.ErrIndexErrored
	}
	if depth <= 0 {
		depth = 2
	}
	repoID := ws.Hash()

	// Resolve the seed stable_key → graph.NodeID. A miss here is the
	// "seed not in latest committed snapshot" path; the orchestrator
	// classifies it as ErrNoSnapshot and falls through to LSP.
	startNode, ok, err := l.store.QueryNodeIDByStableKey(ctx, repoID, string(sym))
	if err != nil {
		return nil, fmt.Errorf("integSemanticLookup.ExpandFrom: resolve start: %w", err)
	}
	if !ok {
		return nil, integ.ErrNoSnapshot
	}

	// Pull the effective adjacency once for the whole BFS (snapshot edges ⊕
	// live overlay edges, status='live' tombstoned overlay rows excluded).
	out, _, err := l.store.QueryEffectiveAdjacency(ctx, repoID, defaultRankProjection)
	if err != nil {
		return nil, fmt.Errorf("integSemanticLookup.ExpandFrom: %w", err)
	}

	return bfsExpand(ctx, l.store, repoID, graph.NodeID(startNode), out, depth), nil
}

// bfsExpand performs a frontier-based BFS over the outbound adjacency map
// rooted at start, to the given depth. Each visited target yields one
// integ.Impact carrying the edge that brought us there as Evidence, the
// target's stable_key as SymbolID, and a Phase 62 closed-ladder confidence
// derived from the edge weight.
//
// Visit-once semantics: a node first reached at depth d is NOT re-emitted
// when re-encountered at a later depth. This keeps the result set bounded
// even on graphs with high in-degree.
func bfsExpand(
	ctx context.Context,
	store *semanticstore.Store,
	repoID string,
	start graph.NodeID,
	out map[graph.NodeID]map[graph.NodeID]float64,
	depth int,
) []integ.Impact {
	visited := map[graph.NodeID]struct{}{start: {}}
	frontier := []graph.NodeID{start}
	impacts := make([]integ.Impact, 0, 8)

	startKey, _, _ := store.QueryStableKeyByNodeID(ctx, repoID, uint64(start))

	for d := 1; d <= depth; d++ {
		next := make([]graph.NodeID, 0, len(frontier))
		for _, src := range frontier {
			neighbors, ok := out[src]
			if !ok {
				continue
			}
			// Stable per-frontier-node ordering: edges arrive from a
			// map iteration so we sort target NodeIDs ASC before
			// emitting impacts. Determinism matters for cross-test
			// assertions.
			targets := make([]graph.NodeID, 0, len(neighbors))
			for tgt := range neighbors {
				targets = append(targets, tgt)
			}
			sort.Slice(targets, func(i, j int) bool { return targets[i] < targets[j] })

			srcKey := startKey
			if src != start {
				if k, ok, _ := store.QueryStableKeyByNodeID(ctx, repoID, uint64(src)); ok {
					srcKey = k
				}
			}

			for _, tgt := range targets {
				if _, seen := visited[tgt]; seen {
					continue
				}
				visited[tgt] = struct{}{}
				next = append(next, tgt)

				weight := neighbors[tgt]
				tgtKey, _, _ := store.QueryStableKeyByNodeID(ctx, repoID, uint64(tgt))
				confidence := confidenceFromWeight(weight)
				edge := integ.Edge{
					From:       integ.SymbolID(srcKey),
					To:         integ.SymbolID(tgtKey),
					Kind:       "calls",
					Confidence: confidence,
				}
				impacts = append(impacts, integ.Impact{
					SymbolID:   integ.SymbolID(tgtKey),
					EdgeKind:   "calls",
					Confidence: confidence,
					Evidence: integ.Evidence{
						Edges: []integ.Edge{edge},
						Ranks: []float64{weight},
					},
				})
			}
		}
		if len(next) == 0 {
			break
		}
		frontier = next
	}
	return impacts
}

// confidenceFromWeight maps a graph edge weight onto the Phase 62 closed-ladder
// confidence values:
//
//	weight >= 1.00 → 0.95 (snapshot+overlay merged; LSP-validated probe in 65-12 flips to 1.00)
//	weight >= 0.80 → 0.80 (tree-sitter + local resolution)
//	weight >= 0.60 → 0.70 (tree-sitter only)
//	weight <  0.60 → 0.45 (heuristic / weak signal)
//
// The 1.00 value is reserved for the Pass-2 LSP-confirmed path (set by
// applyValidationVerdicts in the kernel orchestrator); ExpandFrom never
// emits 1.00 itself because Pass 1 alone cannot LSP-confirm an edge.
func confidenceFromWeight(weight float64) float64 {
	switch {
	case weight >= 1.00:
		return 0.95
	case weight >= 0.80:
		return 0.80
	case weight >= 0.60:
		return 0.70
	default:
		return 0.45
	}
}

// ValidateCriticalEdges runs Pass 2 LSP validation. Phase 65 65-12 Task 2
// architectural decision: this method is a permanent passthrough — it
// returns the input edges with LSPConfirmed=false, without issuing any
// LSP traffic. The Pass-2 LSP probe responsibility is moved to the
// kernel-side analyze_blast_radius orchestrator (lspProbeForEdges in
// internal/kernel/symbols/blast_radius_strangler.go), which uses the
// lspool.WorkerLease the orchestrator already holds. The daemon-side
// adapter cannot perform LSP work without either borrowing a kernel-side
// LSP client (back-call breach) or duplicating LSP machinery on the
// daemon side; both are M-readtier breaches.
//
// Test fakes (matrixLookup, fakeLookup) MAY still drive verdicts
// directly — that path is preserved by the lspProbeFn==nil fallback in
// analyzeBlastRadiusViaLookup. In production, lspProbeFn is always
// non-nil, so this method is invoked but its result is discarded.
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

// LocateSymbol resolves an integ.SymbolID (= the Phase 59 EXTRACT-02
// stable_key, per 65-11 Task 2 contract) into the symbol's declaration
// location at the latest committed snapshot. Returns
// ("", 0, 0, false, ErrIndexErrored) on Available()==false; otherwise
// delegates to *Store.QuerySymbolLocationByStableKey.
//
// Phase 65 65-12 Task 1: produced for the kernel-side analyze_blast_radius
// Pass-2 LSP probe. The probe needs (path, line, col) per edge endpoint
// to issue FindReferences via the orchestrator-held lease.
func (l *integSemanticLookup) LocateSymbol(ctx context.Context, ws workspace.WorkspaceKey, sym integ.SymbolID) (string, uint32, uint32, bool, error) {
	if !l.Available() {
		return "", 0, 0, false, integ.ErrIndexErrored
	}
	repoID := ws.Hash()
	path, line, col, ok, err := l.store.QuerySymbolLocationByStableKey(ctx, repoID, string(sym))
	if err != nil {
		return "", 0, 0, false, fmt.Errorf("integSemanticLookup.LocateSymbol: %w", err)
	}
	return path, line, col, ok, nil
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

	// WR-01 (Phase 65 65-11 Task 2): snapshot bundle.queue / bundle.live /
	// bundle.lastErrReason under bundle.mu. The pre-WR-01 code released the
	// lock between the queue and live reads, which raced with concurrent
	// SetLastErrorReason / engine eviction. Hold the lock for the whole
	// snapshot, then call DepthAll / LastFlushAt OUTSIDE the lock — those
	// methods take their own internal locks and could deadlock if invoked
	// while we hold bundle.mu.
	var pendingLSP int
	var lastLiveMs int64
	var lastErr integ.FallbackReason
	if l.bundle != nil {
		l.bundle.mu.Lock()
		q := l.bundle.queue
		lv := l.bundle.live
		lastErr = l.bundle.lastErrReason
		l.bundle.mu.Unlock()
		if q != nil {
			pendingLSP = q.DepthAll()
		}
		if lv != nil {
			if t := lv.LastFlushAt(ws); !t.IsZero() {
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
		LastErrorReason:  string(lastErr),
	}, nil
}

// Visibility returns the access-level classification for sym at the latest
// committed snapshot. Phase 66 Plan 02 — OI-02 resolution.
//
// The current implementation returns (VisUnknown, ErrUnsupported) as a seam
// placeholder; the full store-backed implementation (reading the Visibility
// field emitted by per-language extractors) is wired in Wave 3. Rule predicates
// in Wave 2 call this method and fall back to D-19 conservative-warn on
// ErrUnsupported, so the guardrail layer is safe to deploy now.
//
// Production callers: internal/guardrails/rules/g003_public_api_edit.go
func (l *integSemanticLookup) Visibility(_ context.Context, _ workspace.WorkspaceKey, _ integ.SymbolID) (integ.Visibility, error) {
	// Phase 66 Wave-3 TODO: query l.store for symbol.Visibility string, then
	// return integ.ParseVisibility(rawVis). Store method: QuerySymbolVisibility.
	return integ.VisUnknown, serr.ErrUnsupported
}

// IsEntrypointReachable returns true when sym is reachable from one of the
// workspace's externally-callable entry points. Phase 66 Plan 02 — OI-03
// resolution.
//
// The current implementation returns (false, ErrUnsupported) as a seam
// placeholder; the full store-backed implementation querying the Phase 62
// call-graph entry-point set is wired in Wave 3. Rule predicates in Wave 2
// fall back to D-19 conservative-warn on ErrUnsupported.
//
// Production callers: internal/guardrails/rules/g003_public_api_edit.go
func (l *integSemanticLookup) IsEntrypointReachable(_ context.Context, _ workspace.WorkspaceKey, _ integ.SymbolID) (bool, error) {
	// Phase 66 Wave-3 TODO: query l.store for call-graph reachability from
	// annotated entry points. Store method: QueryEntrypointReachability.
	return false, serr.ErrUnsupported
}

// daemonCfgGate is the production ConfigGate consumed by Phase 65 strangler-
// fig consumers (RepoMapSkill in 65-05; kernel/symbols + kernel/health in
// 65-06 / 65-07). Wraps the daemon's koanf-resolved
// cfg.SemanticIndex.Enabled flag so the skill side does not import internal/
// config directly.
//
// Construction site: internal/daemon/daemon.go step 12e — the wiring takes
// place even when sBndl is nil (semantic disabled), so the priority ladder
// emits SourceTreeSitter for the steady-state v1.9 path (D-04 / Pitfall §3)
// instead of SourceFallback + index_disabled.
type daemonCfgGate struct {
	enabled bool
}

// SemanticIndexEnabled reports whether the koanf-resolved cfg.SemanticIndex
// .Enabled flag is on.
func (g *daemonCfgGate) SemanticIndexEnabled() bool {
	if g == nil {
		return false
	}
	return g.enabled
}

// Compile-time guard: daemonCfgGate must satisfy integ.ConfigGate.
var _ integ.ConfigGate = (*daemonCfgGate)(nil)

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
		var baseEpoch uint64
		if mode == "incremental" {
			latest, err := b.store.LatestCommittedSnapshot(ctx, repoID)
			if err != nil {
				return semantic.IndexResult{}, fmt.Errorf("buildFn: LatestCommittedSnapshot: %w", err)
			}
			baseSnapshotID = latest
			// Phase 70-04: pull the baseline overlay epoch the latest
			// committed snapshot persisted (Plan 02). (0, false) signals
			// cold-start; collectCandidatePaths classifies that as
			// fallback_reason=cold_start and falls back to full-walk.
			epoch, _, epochErr := b.store.LatestCommittedSnapshotBaseEpoch(ctx, repoID)
			if epochErr != nil {
				if b.logger != nil {
					b.logger.Warn("buildFn: LatestCommittedSnapshotBaseEpoch failed; treating as cold-start",
						"repo", repoID, "err", epochErr)
				}
				baseEpoch = 0
			} else {
				baseEpoch = epoch
			}
		}

		// 2. Walk (full mode) or drain overlay (incremental mode) to build
		//    the candidate path set, then run the per-path classify + extract
		//    loop. Per-file errors are non-fatal (research §Pattern 4
		//    acceptable error behavior).
		paths := b.collectCandidatePaths(ctx, ws, mode, baseEpoch)
		extracted := b.classifyAndExtract(ctx, repoID, paths)

		// 4. Convert per-language facts to the locked store wire format.
		//    ToStoreFacts is the Phase 65 D-08 unblock adapter; it leaves
		//    FileID / NodeID / RefID zero-valued by contract. The buildFn
		//    composes ToStoreFacts in a per-file loop here so per-file
		//    symbols / references can be re-stamped with the file's
		//    assigned FileID — preserving the (snapshot_id, file_id) and
		//    (snapshot_id, symbol_id / ref_id) primary-key invariants.
		repoModule := detectRepoModulePath(ws.RepoRoot)
		// Register this repo in the in-memory multi-repo registry so sibling
		// repos' imports can resolve to it; query the known set (minus self) to
		// let this repo's CROSS_IMPORTS resolve to siblings when present.
		var knownRepos map[string]string
		if b.repoReg != nil {
			b.repoReg.Register(repoID, repoModule, ws.RepoRoot)
			knownRepos = b.repoReg.KnownRepos(repoID)
		}
		facts := factsFromExtracted(extracted, repoID, repoModule, knownRepos, b.logger,
			withTypeResolution(typeResolveParams{
				maxFixpoint:    b.cfg.TypeResolMaxFixpoint,
				minConfidence:  b.cfg.TypeResolMinConfidence,
				emitUnresolved: b.cfg.TypeResolEmitUnresolved,
			}))

		// 4b. FILE_CHANGES_WITH: mine git history for file co-change coupling.
		//     Best-effort — a non-git workspace or git failure is non-fatal;
		//     co-change is enrichment layered on top of the symbol graph.
		if co, cerr := cochange.Mine(ctx, ws.RepoRoot); cerr == nil {
			for _, c := range co {
				na := cochange.FileNodeID(repoID, c.PathA)
				nb := cochange.FileNodeID(repoID, c.PathB)
				weight := float64(c.Count)
				if weight > 1 {
					weight = 1
				}
				conf := 0.3 + 0.1*float64(c.Count)
				if conf > 0.9 {
					conf = 0.9
				}
				facts.Edges = append(facts.Edges,
					semanticstore.EdgeFact{
						SrcNodeID: na, DstNodeID: nb, EdgeKind: "FILE_CHANGES_WITH",
						SrcKind: "file", DstKind: "file", Source: "git.cochange",
						Confidence: conf, Weight: weight,
					},
					semanticstore.EdgeFact{
						SrcNodeID: nb, DstNodeID: na, EdgeKind: "FILE_CHANGES_WITH",
						SrcKind: "file", DstKind: "file", Source: "git.cochange",
						Confidence: conf, Weight: weight,
					},
				)
			}
		} else if b.logger != nil {
			b.logger.Debug("cochange mine skipped", "repo_root", ws.RepoRoot, "err", cerr)
		}

		// Assign a dense 1-based EdgeID across the assembled batch. The
		// semantic_edges PK is (snapshot_id, edge_id) and WriteSnapshotFacts
		// inserts EdgeID verbatim with no allocator, so edges left at the
		// zero default collide on the second row. This is the single
		// allocation point for the whole snapshot — it covers BOTH the
		// factsFromExtracted edges (heritage / imports / classifier /
		// CROSS_* / DEFINES / MEMBER_OF / TESTS / HANDLES / SIMILAR_TO /
		// DATA_FLOWS) AND the FILE_CHANGES_WITH edges appended just above.
		for i := range facts.Edges {
			facts.Edges[i].EdgeID = uint64(i + 1)
		}

		// 5. Snapshot lifecycle.
		snap, err := b.store.BeginSnapshot(ctx, semanticstore.SnapshotMeta{
			RepoID:         repoID,
			BaseSnapshotID: baseSnapshotID,
		})
		if err != nil {
			// WR-2 / IN-04 (Phase 65 65-11 Task 2): stamp the closed-enum
			// reason so get_health.semantic_index.last_error surfaces
			// "index_error" (NOT raw error text or empty) until the next
			// successful build clears the stamp.
			b.SetLastErrorReason(integ.FallbackReasonIndexError)
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
			// WR-2 / IN-04: stamp index_error on WriteSnapshotFacts failure.
			b.SetLastErrorReason(integ.FallbackReasonIndexError)
			return semantic.IndexResult{}, fmt.Errorf("buildFn: WriteSnapshotFacts: %w", err)
		}
		st.AddFilesIndexed(int64(len(extracted)))

		summary := semanticstore.SnapshotSummary{
			FileCount:      len(facts.Files),
			SymbolCount:    len(facts.Symbols),
			ReferenceCount: len(facts.References),
			DurationMs:     time.Since(startedAt).Milliseconds(),
		}

		// Phase 70-04: capture the current overlay epoch IMMEDIATELY before
		// CommitSnapshot and stamp it on the snapshot (Pitfall 3 mitigation).
		// The next incremental refresh reads this via
		// LatestCommittedSnapshotBaseEpoch and feeds it to
		// OverlayChangedPathsSince so the drain set is "everything written
		// after this commit". Capture failure → baseEpoch=0 signals
		// cold-start to the next refresh (full-walk fallback).
		baseOverlayEpoch, epochErr := b.store.CurrentOverlayEpoch(ctx, repoID)
		if epochErr != nil {
			if b.logger != nil {
				b.logger.Warn("buildFn: capture base_overlay_epoch failed; persisting 0",
					"event", "capture_base_overlay_epoch_failed",
					"repo", repoID, "err", epochErr)
			}
			baseOverlayEpoch = 0
		}
		snap.SetBaseOverlayEpoch(baseOverlayEpoch)

		if err := b.store.CommitSnapshot(ctx, snap, summary); err != nil {
			// WR-2 / IN-04: stamp index_error on CommitSnapshot failure.
			b.SetLastErrorReason(integ.FallbackReasonIndexError)
			return semantic.IndexResult{}, fmt.Errorf("buildFn: CommitSnapshot: %w", err)
		}
		committed = true

		// WR-2: clear the lastErrReason stamp on successful commit so a
		// previously-stamped transient error stops surfacing on get_health.
		b.SetLastErrorReason("")

		return semantic.IndexResult{
			SnapshotID:   snap.ID,
			FilesIndexed: int64(len(extracted)),
			FilesReused:  0,
			DurationMs:   time.Since(startedAt).Milliseconds(),
		}, nil
	}
}

// collectCandidatePaths builds the candidate path set the production buildFn
// classifies + extracts.
//   - mode=full: walk ws.RepoRoot via filepath.WalkDir (.helix, .git, dot-dirs excluded).
//   - mode=incremental: query OverlayChangedPathsSince(baseEpoch); on empty
//     result, fall back to full-walk and emit the bounded-label fallback metric.
func (b *semanticBundle) collectCandidatePaths(ctx context.Context, ws workspace.WorkspaceKey, mode string, baseEpoch uint64) []string {
	if ws.RepoRoot == "" {
		b.fireCollectCandidatePathsHook(nil)
		return nil
	}
	if mode != "incremental" {
		out := b.fullWalkPaths(ws)
		b.fireCollectCandidatePathsHook(out)
		return out
	}
	repoID := ws.Hash()
	paths, currentEpoch, err := b.store.OverlayChangedPathsSince(ctx, repoID, baseEpoch)
	if err != nil {
		if b.logger != nil {
			b.logger.Warn("collectCandidatePaths: overlay seam error; falling back to full-walk",
				"repo", repoID, "err", err)
		}
		if b.metrics != nil {
			b.metrics.IncrementalRefreshFallbackInc(obs.IncrementalRefreshFallbackReasonError, repoID)
		}
		out := b.fullWalkPaths(ws)
		b.fireCollectCandidatePathsHook(out)
		return out
	}
	if len(paths) > 0 {
		// A1: paths are absolute per RESEARCH.md §Open Questions RESOLVED —
		// overlay producer (handler.go:417) writes the verbatim caller-supplied
		// absolute path; no filepath.Join translation needed. If a future
		// producer is added that writes relative paths, this assumption MUST
		// be revisited.
		b.fireCollectCandidatePathsHook(paths)
		return paths
	}
	reason := classifyEmptySeamFallback(baseEpoch, currentEpoch)
	if b.logger != nil {
		b.logger.Warn("collectCandidatePaths: incremental fell back to full-walk",
			"repo", repoID, "reason", reason,
			"base_epoch", baseEpoch, "current_epoch", currentEpoch)
	}
	if b.metrics != nil {
		b.metrics.IncrementalRefreshFallbackInc(reason, repoID)
	}
	out := b.fullWalkPaths(ws)
	b.fireCollectCandidatePathsHook(out)
	return out
}

// classifyEmptySeamFallback maps (baseEpoch, currentEpoch) to a closed-enum
// fallback reason for the empty-OverlayChangedPathsSince case.
//
//	baseEpoch == 0                     → cold_start    (caller never observed an epoch)
//	currentEpoch > baseEpoch           → overlay_rotated (overlay advanced with no per-path
//	                                                       rows above baseEpoch)
//	otherwise                          → empty_overlay (overlay quiet since baseline)
func classifyEmptySeamFallback(baseEpoch, currentEpoch uint64) string {
	switch {
	case baseEpoch == 0:
		return obs.IncrementalRefreshFallbackReasonColdStart
	case currentEpoch > baseEpoch:
		return obs.IncrementalRefreshFallbackReasonOverlayRotated
	default:
		return obs.IncrementalRefreshFallbackReasonEmptyOverlay
	}
}

// fullWalkPaths walks ws.RepoRoot and returns absolute paths to all regular
// files outside dot-directories (.helix, .git, etc.). Extracted from the
// pre-Phase-70 collectCandidatePaths body verbatim — mode="full" behavior
// is byte-identical.
func (b *semanticBundle) fullWalkPaths(ws workspace.WorkspaceKey) []string {
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
		// WR-04: simplified — git/helix/any other dot-directory is
		// uniformly excluded by the strings.HasPrefix(name, ".") check.
		// The pre-WR-04 form named git+helix as separate equality arms
		// against the same prefix predicate; the named arms were dead
		// code (every match would also satisfy the prefix check). The
		// `name != "."` guard is also dead — d.Name() never returns "."
		// for a non-root entry produced by filepath.WalkDir.
		name := d.Name()
		if d.IsDir() {
			if path != ws.RepoRoot && strings.HasPrefix(name, ".") {
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
// providers registered by the daemon at bootstrap (Phase 59 P05 +
// v2.12 Phase 135): Go, TypeScript / TSX, JavaScript / JSX, Python, and
// the C-family — C (.c/.h), C++ (.cpp/.cc/.cxx/.hpp/.hh/.hxx), C# (.cs),
// and Java (.java).
//
// Rust / Kotlin / PHP / Ruby are intentionally NOT mapped here: their
// type resolvers are v2.13 stubs, so enabling their extraction now would
// add untested surface. They stay "" until v2.13 wires them.
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
	case ".c", ".h":
		return "c"
	case ".cpp", ".cc", ".cxx", ".hpp", ".hh", ".hxx":
		return "cpp"
	case ".cs":
		return "c_sharp"
	case ".java":
		return "java"
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

// factsOption configures optional passes in factsFromExtracted. The ~20
// existing callers pass none (backward-compatible); only the production
// buildFn opts into the type-edge driver via withTypeResolution. Decision B1
// (least churn): the driver runs INSIDE factsFromExtracted where nameToNode /
// nameCount live, so no caller's signature changes and nameToNode need not be
// returned.
type factsOption func(*factsConfig)

// factsConfig accumulates the applied factsOptions.
type factsConfig struct {
	typeResolve    bool
	typeResolveCfg typeResolveParams
}

// withTypeResolution enables the batch RESOLVES_TO type-edge driver with the
// given knobs. Only the production buildFn passes it.
func withTypeResolution(p typeResolveParams) factsOption {
	return func(c *factsConfig) {
		c.typeResolve = true
		c.typeResolveCfg = p
	}
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
//
// WR-07 / WR-1 (Phase 65 65-10 Task 2): the high-bit-set guard is now
// surfaced. On every input symbol, if SymbolID / OwnerSymbolID /
// ParentScopeID has the high bit set (which the duckdb-go driver
// rejects), `logger` (when non-nil) emits a warn-level diagnostic with
// the violating IDs AND the function continues with the masked low-63
// value (LOG + MASK + CONTINUE). Skip-on-violation cascades into the
// snapshot pipeline and breaks ingest determinism; logging-only does
// not. A genuine collision-fix (assigning fresh IDs on conflict) is
// tracked as a deferred follow-up — see plan's `<deferred>` block.
func factsFromExtracted(extracted []*extract.ExtractedFile, repoID, repoModulePath string, knownRepos map[string]string, logger *slog.Logger, opts ...factsOption) semanticstore.Facts {
	if len(extracted) == 0 {
		return semanticstore.Facts{}
	}
	var fc factsConfig
	for _, o := range opts {
		o(&fc)
	}
	const highBitMask uint64 = 0x8000000000000000
	const lowBitsMask uint64 = 0x7FFFFFFFFFFFFFFF
	out := semanticstore.Facts{
		Files:      make([]semanticstore.FileFact, 0, len(extracted)),
		Symbols:    make([]semanticstore.SymbolFact, 0, 4*len(extracted)),
		References: make([]semanticstore.ReferenceFact, 0, 4*len(extracted)),
	}
	var refSeq uint64
	var routeHandlers []routeHandlerLink
	seenExtModule := make(map[uint64]bool) // dedup external-module nodes across files
	var pendingDst []pendingDstResolve     // shallow edges awaiting name-index resolution
	var fpNodes []fingerprintedNode        // per-symbol fingerprints for SIMILAR_TO / DATA_FLOWS
	var nodeToParams = map[uint64][]uint64{} // func/method NodeID -> ordered param NodeIDs (emit-order adjacency, D1b)
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

			// WR-07 / WR-1: assert SymbolID high-bit is zero. On
			// violation, LOG at warn level AND continue with the masked
			// low-63 value. Skip-on-violation cascades into the snapshot
			// pipeline and breaks ingest determinism; logging surfaces
			// the bug to operators without breaking the build. A
			// genuine collision-fix (assigning fresh IDs on conflict)
			// is tracked as a deferred follow-up — see plan's
			// <deferred> block.
			if single.Symbols[j].SymbolID&highBitMask != 0 ||
				single.Symbols[j].OwnerSymbolID&highBitMask != 0 ||
				single.Symbols[j].ParentScopeID&highBitMask != 0 {
				if logger != nil {
					logger.Warn(
						"factsFromExtracted: high-bit-set SymbolID — masking + continuing (WR-07; collision-fix deferred)",
						"symbol_id", single.Symbols[j].SymbolID,
						"owner_symbol_id", single.Symbols[j].OwnerSymbolID,
						"parent_scope_id", single.Symbols[j].ParentScopeID,
					)
				}
			}

			// Mask SymbolID / NodeID to 63 bits — the duckdb-go driver
			// rejects uint64 values with the high bit set. Mirrors the
			// overlay edge-id helper at internal/semantic/store/overlay.go:937.
			single.Symbols[j].SymbolID &= lowBitsMask
			if single.Symbols[j].NodeID == 0 {
				single.Symbols[j].NodeID = single.Symbols[j].SymbolID
			} else {
				single.Symbols[j].NodeID &= lowBitsMask
			}
			single.Symbols[j].OwnerSymbolID &= lowBitsMask
			single.Symbols[j].ParentScopeID &= lowBitsMask
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

		// Collect per-symbol fingerprints for the batch-level SIMILAR_TO /
		// DATA_FLOWS passes. single.Symbols[k] (masked NodeID) and
		// ef.Symbols[k] (provider-computed MinHash / ASTProfile) are 1:1 in
		// index — ToStoreFacts iterates ef.Symbols in order. Only symbols the
		// provider fingerprinted (function/method bodies ≥ minhash.MinNodes)
		// carry a non-nil signature/profile.
		for k := range single.Symbols {
			if k >= len(ef.Symbols) {
				break
			}
			es := ef.Symbols[k]
			if es.MinHash == nil && es.Profile == nil && es.ContextVec == nil && es.FlowSummary == nil {
				continue
			}
			fpNodes = append(fpNodes, fingerprintedNode{
				nodeID:   single.Symbols[k].NodeID,
				kind:     single.Symbols[k].Kind,
				language: single.Symbols[k].Language,
				sig:      es.MinHash,
				profile:  es.Profile,
				vec:      es.ContextVec,
				flow:     es.FlowSummary,
			})
		}
		// nodeToParams: a function/method's params are the maximal consecutive
		// KindParameter run following it in single.Symbols (emit-order adjacency —
		// NOT the flat DEFINES container heuristic; v2.9 D1b red-team fold).
		for k := range single.Symbols {
			sk := single.Symbols[k].Kind
			if sk != string(extract.KindFunction) && sk != string(extract.KindMethod) {
				continue
			}
			fnID := single.Symbols[k].NodeID
			if fnID == 0 {
				continue
			}
			var params []uint64
			for j := k + 1; j < len(single.Symbols); j++ {
				if single.Symbols[j].Kind != string(extract.KindParameter) {
					break
				}
				if single.Symbols[j].NodeID != 0 {
					params = append(params, single.Symbols[j].NodeID)
				}
			}
			if len(params) > 0 {
				nodeToParams[fnID] = params
			}
		}

		// Heritage → IMPLEMENTS/EXTENDS edges. Source is the file's first
		// symbol (providers don't populate HeritageFact.SubjectID); the
		// target NAME (h.Target, possibly qualified) is resolved against the
		// batch name index after the loop (resolvePendingDst). In-repo
		// targets get a real DstNodeID + raised confidence; out-of-repo
		// targets (stdlib/external types) stay DstNodeID=0 at base confidence.
		for _, h := range ef.Heritage {
			srcID := uint64(0)
			if len(single.Symbols) > 0 {
				srcID = single.Symbols[0].NodeID
			}
			kind := "IMPLEMENTS"
			if h.Relation == "extends" {
				kind = "EXTENDS"
			}
			out.Edges = append(out.Edges, semanticstore.EdgeFact{
				SrcNodeID:  srcID,
				DstNodeID:  0, // resolved post-loop via batch name index (in-repo only)
				EdgeKind:   kind,
				Source:     "tree_sitter",
				Confidence: 0.20,
				Weight:     0.5,
			})
			pendingDst = append(pendingDst, pendingDstResolve{
				edgeIdx:    len(out.Edges) - 1,
				candidates: []string{h.Target, lastNameSegment(h.Target)},
				srcNodeID:  srcID,
				confidence: 0.70, // tree-sitter + local name resolution
				weight:     0.6,
			})
		}

		extByLocalName := make(map[string]uint64) // import local-name → external-module node (for CROSS_CALLS)
		for _, imp := range ef.Imports {
			srcID := uint64(0)
			if len(single.Symbols) > 0 {
				srcID = single.Symbols[0].NodeID
			}
			out.Edges = append(out.Edges, semanticstore.EdgeFact{
				SrcNodeID:  srcID,
				DstNodeID:  0, // resolved post-loop: named imports → in-repo symbol; bare module imports stay 0
				EdgeKind:   "IMPORTS",
				Source:     "tree_sitter",
				Confidence: 0.20,
				Weight:     0.5,
			})
			// Named imports ({ Foo } from "./mod" / from mod import Foo) carry
			// the imported symbol name; resolve to its in-repo definition when
			// present. Bare module imports (import "fmt") have no named symbol
			// and stay DstNodeID=0 (module-level, no single in-repo target).
			if importCands := importTargetCandidates(imp); len(importCands) > 0 {
				pendingDst = append(pendingDst, pendingDstResolve{
					edgeIdx:    len(out.Edges) - 1,
					candidates: importCands,
					srcNodeID:  srcID,
					confidence: 0.60,
					weight:     0.5,
				})
			}

			// CROSS_IMPORTS: imports whose module path is external to this repo
			// promote to a named external-module node + a CROSS_IMPORTS edge.
			// When the module resolves to a known sibling repo (in-memory
			// multi-repo registry), the node's LSPIdentity carries the sibling
			// repoID and node+edge are stamped at higher confidence (federation).
			if crossrepo.ClassifyImport(repoModulePath, ef.File.Language, imp.Source) != crossrepo.External {
				continue
			}
			modPrefix := crossrepo.ModulePathPrefix(ef.File.Language, imp.Source)
			if modPrefix == "" {
				continue
			}
			resolvedRepo, resolved := crossrepo.Resolve(ef.File.Language, imp.Source, knownRepos)
			extID := uint64(extract.StableSymbolID(extract.BuildProviderKey(extract.SymbolMeta{
				RepoID: repoID, Language: ef.File.Language, PackagePath: modPrefix,
				QualifiedName: modPrefix, Kind: string(extract.KindExternalModule),
				SignatureHash: modPrefix, RelPath: modPrefix, Visibility: "exported",
			}))) & lowBitsMask
			if !seenExtModule[extID] {
				seenExtModule[extID] = true
				node := semanticstore.SymbolFact{
					SymbolID:         extID,
					NodeID:           extID,
					FileID:           fileID,
					Language:         ef.File.Language,
					Kind:             string(extract.KindExternalModule),
					Name:             modPrefix,
					QualifiedName:    modPrefix,
					PackagePath:      modPrefix,
					Visibility:       "exported",
					ExtractionSource: "crossrepo",
					Confidence:       0.50,
				}
				if resolved {
					node.LSPIdentity = resolvedRepo
					node.Confidence = 0.70
				}
				out.Symbols = append(out.Symbols, node)
			}
			edgeConf := 0.50
			edgeSource := "crossrepo.classifier"
			if resolved {
				edgeConf = 0.70
				edgeSource = "crossrepo.resolved"
			}
			out.Edges = append(out.Edges, semanticstore.EdgeFact{
				SrcNodeID:  srcID,
				DstNodeID:  extID,
				EdgeKind:   "CROSS_IMPORTS",
				Source:     edgeSource,
				Confidence: edgeConf,
				Weight:     0.4,
			})
			if ln := importLocalName(ef.File.Language, imp.Source); ln != "" {
				if _, ok := extByLocalName[ln]; !ok {
					extByLocalName[ln] = extID
				}
			}
		}

		// CROSS_CALLS: a member-call (pkg.Method / Bar.method) whose receiver
		// matches an external module's local binding → cross-repo call edge.
		// Requires providers to populate ReceiverText on member-call references.
		for j := range single.References {
			ref := &single.References[j]
			if ref.RefKind != "call" || ref.ReceiverText == "" {
				continue
			}
			extID, ok := extByLocalName[ref.ReceiverText]
			if !ok {
				continue
			}
			srcID := ref.NodeID
			if srcID == 0 && len(single.Symbols) > 0 {
				srcID = single.Symbols[0].NodeID
			}
			out.Edges = append(out.Edges, semanticstore.EdgeFact{
				SrcNodeID:  srcID,
				DstNodeID:  extID,
				EdgeKind:   "CROSS_CALLS",
				Source:     "crossrepo.classifier",
				Confidence: 0.45,
				Weight:     0.4,
			})
		}

		// Classifier-based edges: HTTP_CALLS, ASYNC_CALLS, EMITS, LISTENS_ON.
		// Walk reference.calls and classify by name using per-language pattern tables.
		lang := ef.File.Language
		for j := range single.References {
			ref := &single.References[j]
			if ref.RefKind != "call" {
				continue
			}
			kind := string(classifier.ClassifyCall(lang, ref.Name))
			if kind == "" {
				continue
			}
			// Use the reference's NodeID as source
			srcID := ref.NodeID
			if srcID == 0 && len(single.Symbols) > 0 {
				srcID = single.Symbols[0].NodeID
			}
			out.Edges = append(out.Edges, semanticstore.EdgeFact{
				SrcNodeID:  srcID,
				DstNodeID:  0, // resolved post-loop: callee name → in-repo symbol when present
				EdgeKind:   kind,
				Source:     "classifier",
				Confidence: 0.45,
				Weight:     0.3,
			})
			// Resolve the callee NAME to its in-repo definition when present
			// (e.g. an in-repo emitter/handler function). External-library
			// callees (the common case for HTTP/async) stay DstNodeID=0.
			pendingDst = append(pendingDst, pendingDstResolve{
				edgeIdx:    len(out.Edges) - 1,
				candidates: []string{ref.Name},
				srcNodeID:  srcID,
				confidence: 0.55, // name-classified + resolved in-repo callee
				weight:     0.3,
			})
		}

		// DEFINES edges: container symbol → contained symbols.
		// First symbol in file is typically the outer class/module.
		if len(single.Symbols) >= 2 {
			containerID := single.Symbols[0].NodeID
			for j := 1; j < len(single.Symbols); j++ {
				out.Edges = append(out.Edges, semanticstore.EdgeFact{
					SrcNodeID:  containerID,
					DstNodeID:  single.Symbols[j].NodeID,
					EdgeKind:   "DEFINES",
					Source:     "tree_sitter",
					Confidence: 0.70,
					Weight:     0.9,
				})
			}
		}
		// Route synthesis: promote detected HTTP route registrations into
		// Route symbols. HANDLES edges (handler → route) are emitted after
		// the batch name index is built, so the handler can be resolved by
		// name across files.
		for _, r := range ef.Routes {
			if r.Path == "" {
				continue
			}
			name := r.Method + " " + r.Path
			key := extract.BuildProviderKey(extract.SymbolMeta{
				RepoID:        repoID,
				Language:      r.Language,
				PackagePath:   r.File,
				QualifiedName: name,
				Kind:          string(extract.KindRoute),
				SignatureHash: name,
				RelPath:       r.File,
				Visibility:    "exported",
			})
			sid := uint64(extract.StableSymbolID(key)) & lowBitsMask
			out.Symbols = append(out.Symbols, semanticstore.SymbolFact{
				SymbolID:         sid,
				NodeID:           sid,
				FileID:           fileID,
				Language:         r.Language,
				Kind:             string(extract.KindRoute),
				Name:             name,
				QualifiedName:    name,
				Visibility:       "exported",
				ExtractionSource: "tree_sitter",
				Confidence:       0.60,
			})
			if r.Handler != "" {
				routeHandlers = append(routeHandlers, routeHandlerLink{
					handlerName: r.Handler,
					routeNodeID: sid,
				})
			}
		}
		// Resource synthesis: promote detected ORM / data-entity definitions
		// (GORM, SQLAlchemy, TypeORM, JPA, EF, Diesel, …) into Resource symbols.
		for _, rc := range ef.Resources {
			if rc.Name == "" {
				continue
			}
			key := extract.BuildProviderKey(extract.SymbolMeta{
				RepoID: repoID, Language: rc.Language, PackagePath: rc.File,
				QualifiedName: rc.Name, Kind: string(extract.KindResource),
				SignatureHash: rc.Name, RelPath: rc.File, Visibility: "exported",
			})
			sid := uint64(extract.StableSymbolID(key)) & lowBitsMask
			out.Symbols = append(out.Symbols, semanticstore.SymbolFact{
				SymbolID:         sid,
				NodeID:           sid,
				FileID:           fileID,
				Language:         rc.Language,
				Kind:             string(extract.KindResource),
				Name:             rc.Name,
				QualifiedName:    rc.Name,
				Visibility:       "exported",
				ExtractionSource: "tree_sitter",
				Confidence:       0.60,
			})
		}
	}

	// Snapshot-level SymbolID dedup (WR — v2.12 Phase 135). The store's
	// semantic_symbols PRIMARY KEY is (snapshot_id, symbol_id); two rows with
	// the same SymbolID abort the whole WriteSnapshotFacts INSERT. Collapsing
	// same-SymbolID rows is always semantically safe: SymbolID is derived from
	// the canonical StableKey, so two rows sharing it ARE one node by
	// definition — and edges already reference NodeID (== SymbolID), which
	// survives on the kept row, so no endpoint is lost.
	//
	// Root cause (C, deferred follow-up): the C extractor emits a
	// definition.struct symbol for a struct DEFINITION *and* for every struct
	// TYPE-USE (`struct Foo* p`), and signatureHash truncates at `{`, so both
	// hash to `struct Foo` → identical StableKey → identical SymbolID. Latent
	// since the store PK always demanded per-snapshot SymbolID uniqueness; only
	// reached now that Phase 135 B1 routes C into production. The deeper fix
	// (C should not emit type-uses as definition symbols) would churn the
	// with_fields golden and is out of scope here.
	//
	// Tiebreak is DETERMINISTIC and prefers the RICHER symbol (the definition):
	// document/cross-file order does not guarantee the definition is seen
	// first (C forward-refs), and keeping an impoverished type-use row would
	// strip the struct's body/fields from explain-symbol-deep. Richness order:
	// (1) a signature containing `{` (a body) beats one without; (2) else the
	// longer signature; (3) else first-seen (stable). This layer, NOT the
	// provider: per-provider StableKey dedup would change existing goldens.
	if len(out.Symbols) > 1 {
		idxBySymbolID := make(map[uint64]int, len(out.Symbols))
		kept := out.Symbols[:0]
		for _, s := range out.Symbols {
			if s.SymbolID == 0 {
				kept = append(kept, s)
				continue
			}
			if prevIdx, seen := idxBySymbolID[s.SymbolID]; seen {
				if richerSymbol(s, kept[prevIdx]) {
					kept[prevIdx] = s
				}
				continue
			}
			idxBySymbolID[s.SymbolID] = len(kept)
			kept = append(kept, s)
		}
		out.Symbols = kept
	}
	// ── Batch-level cross-cutting edges: MEMBER_OF + TESTS ──
	//
	// MEMBER_OF is the inverse of DEFINES (member → container), enabling
	// "what does X belong to?" outgoing traversal. Derived from the per-file
	// DEFINES edges emitted above so the two directions never diverge.
	memberEdges := make([]semanticstore.EdgeFact, 0, len(out.Edges))
	for _, e := range out.Edges {
		if e.EdgeKind == "DEFINES" && e.SrcNodeID != 0 && e.DstNodeID != 0 {
			memberEdges = append(memberEdges, semanticstore.EdgeFact{
				SrcNodeID:  e.DstNodeID,
				DstNodeID:  e.SrcNodeID,
				EdgeKind:   "MEMBER_OF",
				SrcKind:    e.DstKind,
				DstKind:    e.SrcKind,
				Source:     "tree_sitter",
				Confidence: 0.70,
				Weight:     0.9,
			})
		}
	}
	out.Edges = append(out.Edges, memberEdges...)

	// TESTS links a test symbol to the production symbol it most likely
	// exercises. Name-heuristic, batch-level: build a name→NodeID index over
	// non-test symbols, then for each test symbol strip the test prefix and
	// look up candidates. Low confidence (0.30) — name match, not reference.
	fileIDToPath := make(map[uint64]string, len(out.Files))
	for i := range out.Files {
		fileIDToPath[out.Files[i].FileID] = out.Files[i].Path
	}
	nameToNode := make(map[string]uint64, len(out.Symbols))
	nameCount := make(map[string]int, len(out.Symbols)) // anti-mis-bind: callee name -> candidate count (D5)
	for i := range out.Symbols {
		s := out.Symbols[i]
		if s.NodeID == 0 || s.Name == "" || s.Kind == string(extract.KindRoute) || s.Kind == string(extract.KindResource) {
			continue
		}
		if isTestSymbol(s.Language, s.Name, fileIDToPath[s.FileID]) {
			continue
		}
		nameCount[s.Name]++
		if _, exists := nameToNode[s.Name]; !exists {
			nameToNode[s.Name] = s.NodeID
		}
	}

	// Resolve the shallow IMPLEMENTS/EXTENDS/IMPORTS/classifier edges whose
	// DstNodeID was deferred to the batch name index. In-repo targets get a
	// real endpoint + raised confidence; out-of-repo targets keep DstNodeID=0.
	resolvePendingDst(out.Edges, pendingDst, nameToNode)
	testEdges := make([]semanticstore.EdgeFact, 0, 16)
	for i := range out.Symbols {
		s := out.Symbols[i]
		if s.NodeID == 0 {
			continue
		}
		path := fileIDToPath[s.FileID]
		if !isTestSymbol(s.Language, s.Name, path) {
			continue
		}
		for _, cand := range testTargetCandidates(s.Language, s.Name) {
			tgt, ok := nameToNode[cand]
			if !ok || tgt == s.NodeID {
				continue
			}
			testEdges = append(testEdges, semanticstore.EdgeFact{
				SrcNodeID:  s.NodeID,
				DstNodeID:  tgt,
				EdgeKind:   "TESTS",
				Source:     "tree_sitter",
				Confidence: 0.30,
				Weight:     0.5,
			})
			break // first matching candidate wins
		}
	}
	out.Edges = append(out.Edges, testEdges...)

	// HANDLES edges: link each route's handler symbol to the Route symbol.
	// Handler resolved by name against the batch index built above.
	handleEdges := make([]semanticstore.EdgeFact, 0, len(routeHandlers))
	for _, rl := range routeHandlers {
		handlerNode, ok := nameToNode[rl.handlerName]
		if !ok || handlerNode == 0 || handlerNode == rl.routeNodeID {
			continue
		}
		handleEdges = append(handleEdges, semanticstore.EdgeFact{
			SrcNodeID:  handlerNode,
			DstNodeID:  rl.routeNodeID,
			EdgeKind:   "HANDLES",
			Source:     "tree_sitter",
			Confidence: 0.45,
			Weight:     0.4,
		})
	}
	out.Edges = append(out.Edges, handleEdges...)

	// SIMILAR_TO (MinHash near-clone) + STRUCTURAL_TWIN (ASTProfile structural-
	// profile similarity) + SEMANTICALLY_RELATED (Random-Indexing vocabulary
	// similarity) — batch-level passes over the per-symbol fingerprints
	// collected during the file loop. Each is bounded (LSH candidate set /
	// quantized profile buckets / language buckets) and per-node fan-out capped
	// at minhash.MaxEdgesPerNode. SIMILAR_TO/STRUCTURAL_TWIN read SHAPE; RELATED
	// reads VOCABULARY — orthogonal signals over the same nodes. DATA_FLOWS
	// (def_use) reads param->param interprocedural DATA DEPENDENCE (v2.9).
	out.Edges = append(out.Edges, similarToEdges(fpNodes)...)
	out.Edges = append(out.Edges, structuralTwinEdges(fpNodes)...)
	out.Edges = append(out.Edges, semanticallyRelatedEdges(fpNodes)...)
	out.Edges = append(out.Edges, dataFlowEdges(fpNodes, nodeToParams, nameToNode, nameCount)...)

	// RESOLVES_TO type edges (v2.12 Phase 136). Runs LAST — after out.Symbols
	// is final (deduped) and nameToNode/nameCount are built — so the typeIndex
	// points at definition nodes and the batch reader is self-consistent
	// pre-commit. Appends to out.Edges BEFORE the buildFn's dense EdgeID stamp.
	// Gated to the production buildFn (which opts in); the many test callers
	// pass no opts and skip it.
	if fc.typeResolve {
		resolveTypeEdges(&out, repoID, extracted, nameToNode, nameCount, fileIDToPath, fc.typeResolveCfg)
	}

	return out
}

// richerSymbol reports whether candidate c is a "richer" symbol than the
// currently-kept row k for the same SymbolID, and thus should replace it in
// the snapshot-level dedup. Deterministic total order over the two rows:
//
//  1. a signature containing `{` (carries a body / field list — i.e. the
//     definition) beats one that does not;
//  2. else the longer signature wins (more structural detail);
//  3. else keep the incumbent (false) — stable, first-seen order preserved.
//
// No wall-clock, no map iteration, no rand: same inputs → same verdict.
func richerSymbol(c, k semanticstore.SymbolFact) bool {
	cHasBody := strings.Contains(c.Signature, "{")
	kHasBody := strings.Contains(k.Signature, "{")
	if cHasBody != kHasBody {
		return cHasBody
	}
	return len(c.Signature) > len(k.Signature)
}

// ----- Phase 74 P1 store-backed accessor adapters (D-01) -----
//
// Five thin adapters wrapping *semanticstore.Store methods to satisfy the
// five D-01 accessor interfaces declared in internal/skill/semantic/accessors.go.
// Pattern mirrors the existing semStoreAdapter (nil-guard + delegate).
// All imports (context, semanticstore, semantic, integ) are already present.

// semP1SymbolByNameAdapter wraps *semanticstore.Store to satisfy SymbolByNameAccessor.
// QuerySymbolByName converts []string (stable keys) → []integ.SymbolID.
type semP1SymbolByNameAdapter struct {
	store *semanticstore.Store
}

func (b *semanticBundle) symbolByNameAccessor() semantic.SymbolByNameAccessor {
	return &semP1SymbolByNameAdapter{store: b.store}
}

func (a *semP1SymbolByNameAdapter) QuerySymbolByName(ctx context.Context, repoID, path, name string) ([]integ.SymbolID, error) {
	if a == nil || a.store == nil {
		return nil, nil
	}
	raw, err := a.store.QuerySymbolByName(ctx, repoID, path, name)
	if err != nil {
		return nil, err
	}
	out := make([]integ.SymbolID, len(raw))
	for i, s := range raw {
		out[i] = integ.SymbolID(s)
	}
	return out, nil
}

// semP1ExtractorRunAdapter wraps *semanticstore.Store to satisfy ExtractorRunAccessor.
// LatestExtractorRunID delegates directly with no type conversion.
type semP1ExtractorRunAdapter struct {
	store *semanticstore.Store
}

func (b *semanticBundle) extractorRunAccessor() semantic.ExtractorRunAccessor {
	return &semP1ExtractorRunAdapter{store: b.store}
}

func (a *semP1ExtractorRunAdapter) LatestExtractorRunID(ctx context.Context, repoID string) (string, error) {
	if a == nil || a.store == nil {
		return "", nil
	}
	return a.store.LatestExtractorRunID(ctx, repoID)
}

// semP1ClusterMapAdapter wraps *semanticstore.Store to satisfy ClusterMapAccessor.
// QueryClusterSummaries converts []semanticstore.ClusterSummaryResult → []semantic.ClusterSummaryRow.
type semP1ClusterMapAdapter struct {
	store *semanticstore.Store
}

func (b *semanticBundle) clusterMapAccessor() semantic.ClusterMapAccessor {
	return &semP1ClusterMapAdapter{store: b.store}
}

func (a *semP1ClusterMapAdapter) QueryClusterSummaries(ctx context.Context, repoID, projection string, graphVersion uint64, topN int) ([]semantic.ClusterSummaryRow, error) {
	if a == nil || a.store == nil {
		return nil, nil
	}
	raw, err := a.store.QueryClusterSummaries(ctx, repoID, projection, graphVersion, topN)
	if err != nil {
		return nil, err
	}
	out := make([]semantic.ClusterSummaryRow, len(raw))
	for i, r := range raw {
		out[i] = semantic.ClusterSummaryRow{ClusterIntID: r.ClusterIntID, MemberCount: r.MemberCount}
	}
	return out, nil
}

// semP1ClusterMemberAdapter wraps *semanticstore.Store to satisfy ClusterMemberAccessor.
// QueryClusterMembers converts []semanticstore.ClusterMemberResult → []semantic.ClusterMemberRow.
type semP1ClusterMemberAdapter struct {
	store *semanticstore.Store
}

func (b *semanticBundle) clusterMemberAccessor() semantic.ClusterMemberAccessor {
	return &semP1ClusterMemberAdapter{store: b.store}
}

func (a *semP1ClusterMemberAdapter) QueryClusterMembers(ctx context.Context, repoID, projection string, graphVersion, clusterIntID uint64, limit int) ([]semantic.ClusterMemberRow, error) {
	if a == nil || a.store == nil {
		return nil, nil
	}
	raw, err := a.store.QueryClusterMembers(ctx, repoID, projection, graphVersion, clusterIntID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]semantic.ClusterMemberRow, len(raw))
	for i, r := range raw {
		out[i] = semantic.ClusterMemberRow{NodeID: r.NodeID, SymbolID: r.SymbolID}
	}
	return out, nil
}

// semP1ClusterPageRankAdapter wraps *semanticstore.Store to satisfy ClusterPageRankAccessor.
// QueryNodePageRanks returns map[uint64]float64 on both sides — no type conversion needed.
type semP1ClusterPageRankAdapter struct {
	store *semanticstore.Store
}

func (b *semanticBundle) clusterPageRankAccessor() semantic.ClusterPageRankAccessor {
	return &semP1ClusterPageRankAdapter{store: b.store}
}

func (a *semP1ClusterPageRankAdapter) QueryNodePageRanks(ctx context.Context, repoID, projection string, graphVersion uint64, nodeIDs []uint64) (map[uint64]float64, error) {
	if a == nil || a.store == nil {
		return nil, nil
	}
	return a.store.QueryNodePageRanks(ctx, repoID, projection, graphVersion, nodeIDs)
}

// ----- Phase 74 P1 two-hop adapter structs (D-01a FOLD) -----
//
// semP1SymbolEdgesAdapter and semP1ClusterMembershipAdapter satisfy the
// two D-01a FOLD accessor interfaces that require inline SQL via the
// *semanticstore.Store helper methods added to effective_graph.go.
// Both use QueryNodeIDByStableKey as the first hop for the stable_key →
// node_id translation.

// semP1SymbolEdgesAdapter satisfies SymbolEdgesAccessor with three
// direction methods backed by QuerySymbolEdgesIncoming / Outgoing.
// Threat T-74-03-01: all SQL parameters are positional (no interpolation).
// Pitfall 1: JOIN uses semantic_symbols.symbol_id (= semantic_edges src/dst node id).
// Pitfall 2: CallersOf adds edge_kind='CALLS' filter; IncomingEdgesOf does not.
type semP1SymbolEdgesAdapter struct {
	store *semanticstore.Store
}

func (b *semanticBundle) symbolEdgesAccessor() semantic.SymbolEdgesAccessor {
	return &semP1SymbolEdgesAdapter{store: b.store}
}

// resolveSymbolEdgesNodeID is a shared helper for the three direction methods:
// resolves the latest committed snapshot and translates the stable_key sym
// into the uint64 node_id used as src/dst in semantic_edges.
func (a *semP1SymbolEdgesAdapter) resolveSymbolEdgesNodeID(ctx context.Context, repoID string, sym integ.SymbolID) (snapshotID uint64, nodeID uint64, ok bool, err error) {
	snapshotID, err = a.store.LatestCommittedSnapshot(ctx, repoID)
	if err != nil {
		return 0, 0, false, err
	}
	if snapshotID == 0 {
		return 0, 0, false, nil
	}
	nodeID, ok, err = a.store.QueryNodeIDByStableKey(ctx, repoID, string(sym))
	if err != nil {
		return 0, 0, false, err
	}
	return snapshotID, nodeID, ok, nil
}

// assembleEdgeRows converts a []symbolEdgeRaw slice into []semantic.SymbolEdgeRow
// by resolving each src_node_id and dst_node_id back to their stable_keys.
func (a *semP1SymbolEdgesAdapter) assembleEdgeRows(ctx context.Context, repoID string, raw []semanticstore.SymbolEdgeRaw) ([]semantic.SymbolEdgeRow, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	out := make([]semantic.SymbolEdgeRow, 0, len(raw))
	for _, r := range raw {
		fromKey, fromOK, err := a.store.QueryStableKeyByNodeID(ctx, repoID, r.SrcNodeID)
		if err != nil {
			return nil, err
		}
		if !fromOK {
			continue
		}
		toKey, toOK, err := a.store.QueryStableKeyByNodeID(ctx, repoID, r.DstNodeID)
		if err != nil {
			return nil, err
		}
		if !toOK {
			continue
		}
		out = append(out, semantic.SymbolEdgeRow{
			From:         integ.SymbolID(fromKey),
			To:           integ.SymbolID(toKey),
			InternalKind: r.EdgeKind,
		})
	}
	return out, nil
}

// CallersOf returns CALLS-filtered incoming edges (dst_node_id=sym, edge_kind='CALLS').
func (a *semP1SymbolEdgesAdapter) CallersOf(ctx context.Context, repoID string, sym integ.SymbolID) ([]semantic.SymbolEdgeRow, error) {
	if a == nil || a.store == nil {
		return nil, nil
	}
	snapshotID, nodeID, ok, err := a.resolveSymbolEdgesNodeID(ctx, repoID, sym)
	if err != nil || !ok {
		return nil, err
	}
	raw, err := a.store.QuerySymbolEdgesIncoming(ctx, snapshotID, nodeID, true /* callsOnly */)
	if err != nil {
		return nil, err
	}
	return a.assembleEdgeRows(ctx, repoID, raw)
}

// IncomingEdgesOf returns all incoming edges (dst_node_id=sym, no edge_kind filter).
func (a *semP1SymbolEdgesAdapter) IncomingEdgesOf(ctx context.Context, repoID string, sym integ.SymbolID) ([]semantic.SymbolEdgeRow, error) {
	if a == nil || a.store == nil {
		return nil, nil
	}
	snapshotID, nodeID, ok, err := a.resolveSymbolEdgesNodeID(ctx, repoID, sym)
	if err != nil || !ok {
		return nil, err
	}
	raw, err := a.store.QuerySymbolEdgesIncoming(ctx, snapshotID, nodeID, false /* all kinds */)
	if err != nil {
		return nil, err
	}
	return a.assembleEdgeRows(ctx, repoID, raw)
}

// OutgoingEdgesOf returns all outgoing edges (src_node_id=sym, no edge_kind filter).
func (a *semP1SymbolEdgesAdapter) OutgoingEdgesOf(ctx context.Context, repoID string, sym integ.SymbolID) ([]semantic.SymbolEdgeRow, error) {
	if a == nil || a.store == nil {
		return nil, nil
	}
	snapshotID, nodeID, ok, err := a.resolveSymbolEdgesNodeID(ctx, repoID, sym)
	if err != nil || !ok {
		return nil, err
	}
	raw, err := a.store.QuerySymbolEdgesOutgoing(ctx, snapshotID, nodeID)
	if err != nil {
		return nil, err
	}
	return a.assembleEdgeRows(ctx, repoID, raw)
}

// semP1DataFlowReachabilityAdapter satisfies DataFlowReachabilityAccessor
// (v2.10) for trace_data_flow. ReachableFrom resolves the seed PARAMETER symbol
// to its node, bulk-loads ALL DATA_FLOWS edges in the snapshot in ONE query
// (QueryAllDataFlowEdges — the red-team M1 fold, avoiding N OutgoingEdgesOf
// roundtrips + the per-edge QueryStableKeyByNodeID storm), runs an in-memory
// BFS from the seed (hop-capped, visited-set, deterministic), and resolves
// reachable nodeIDs back to SymbolIDs. Read-only (D-09 invariant).
//
// Seed semantics (v2.10 L3): the seed MUST be a PARAMETER. A function seed
// returns empty because DATA_FLOWS edges are param-anchored (src_node_id = the
// param node, not the function node) and there is no query-time function->params
// path at HEAD (CONTAINS function->param is not tree-sitter-emitted).
type semP1DataFlowReachabilityAdapter struct {
	store *semanticstore.Store
}

func (b *semanticBundle) dataFlowReachabilityAccessor() semantic.DataFlowReachabilityAccessor {
	return &semP1DataFlowReachabilityAdapter{store: b.store}
}

func (a *semP1DataFlowReachabilityAdapter) ReachableFrom(ctx context.Context, repoID string, seed integ.SymbolID, maxHops int) ([]semantic.ReachableNode, error) {
	if a == nil || a.store == nil {
		return nil, nil
	}
	snapshotID, err := a.store.LatestCommittedSnapshot(ctx, repoID)
	if err != nil {
		return nil, err
	}
	if snapshotID == 0 {
		return nil, nil
	}
	seedNode, ok, err := a.store.QueryNodeIDByStableKey(ctx, repoID, string(seed))
	if err != nil {
		return nil, err
	}
	if !ok || seedNode == 0 {
		return nil, nil // seed symbol not in the latest committed snapshot
	}
	if maxHops < 0 {
		maxHops = 0
	}
	// Bulk-load all DATA_FLOWS edges once; BFS in-memory over the adjacency.
	all, err := a.store.QueryAllDataFlowEdges(ctx, snapshotID)
	if err != nil {
		return nil, err
	}
	adj := make(map[uint64][]uint64, len(all))
	for _, e := range all {
		adj[e.SrcNodeID] = append(adj[e.SrcNodeID], e.DstNodeID)
	}
	// Deterministic BFS: process neighbors in ascending nodeID order.
	visited := map[uint64]int{seedNode: 0}
	frontier := []uint64{seedNode}
	for hop := 1; hop <= maxHops; hop++ {
		var next []uint64
		for _, n := range frontier {
			nb := append([]uint64(nil), adj[n]...)
			sort.Slice(nb, func(i, j int) bool { return nb[i] < nb[j] })
			for _, d := range nb {
				if _, seen := visited[d]; seen {
					continue
				}
				visited[d] = hop
				next = append(next, d)
			}
		}
		sort.Slice(next, func(i, j int) bool { return next[i] < next[j] })
		frontier = next
		if len(frontier) == 0 {
			break
		}
	}
	// Resolve reachable nodeIDs -> SymbolIDs; sort by (Hops, SymbolID).
	out := make([]semantic.ReachableNode, 0, len(visited))
	for nodeID, hops := range visited {
		key, keyOK, err := a.store.QueryStableKeyByNodeID(ctx, repoID, nodeID)
		if err != nil {
			return nil, err
		}
		if !keyOK {
			continue
		}
		out = append(out, semantic.ReachableNode{SymbolID: integ.SymbolID(key), Hops: hops})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Hops != out[j].Hops {
			return out[i].Hops < out[j].Hops
		}
		return out[i].SymbolID < out[j].SymbolID
	})
	return out, nil
}

// semP1ClusterMembershipAdapter satisfies ClusterMembershipAccessor with
// a two-hop lookup: CurrentGraphVersion + QueryNodeIDByStableKey, then
// QueryClusterIDOfNode.
// Pitfall 3: graph_version is resolved internally via CurrentGraphVersion —
// the interface does not carry it as a parameter.
// Threat T-74-03-02: all SQL parameters are positional.
type semP1ClusterMembershipAdapter struct {
	store *semanticstore.Store
}

func (b *semanticBundle) clusterMembershipAccessor() semantic.ClusterMembershipAccessor {
	return &semP1ClusterMembershipAdapter{store: b.store}
}

// ClusterIDOf returns the cluster_id and member count for the cluster
// containing symbolID at the latest committed graph_version for repoID.
// Returns (0, 0, nil) when not found — handler emits fallback_reason="cluster_boost_unavailable".
func (a *semP1ClusterMembershipAdapter) ClusterIDOf(ctx context.Context, repoID string, symbolID integ.SymbolID) (uint64, int, error) {
	if a == nil || a.store == nil {
		return 0, 0, nil
	}
	// Step 1: resolve graph_version internally (Pitfall 3).
	gv, err := a.store.CurrentGraphVersion(ctx, repoID)
	if err != nil {
		return 0, 0, err
	}
	if gv == 0 {
		return 0, 0, nil
	}
	// Step 2: stable_key → node_id.
	nodeID, ok, err := a.store.QueryNodeIDByStableKey(ctx, repoID, string(symbolID))
	if err != nil {
		return 0, 0, err
	}
	if !ok {
		return 0, 0, nil
	}
	// Step 3: node_id → cluster_id + member_count.
	clusterID, memberCount, found, err := a.store.QueryClusterIDOfNode(ctx, repoID, gv, nodeID)
	if err != nil {
		return 0, 0, err
	}
	if !found {
		return 0, 0, nil
	}
	return clusterID, memberCount, nil
}

// ----- Compile-time interface guards -----

var (
	_ semantic.StoreAccessor             = (*semStoreAdapter)(nil)
	_ semantic.SchedulerAccessor         = (*semSchedulerAdapter)(nil)
	_ semantic.QueueAccessor             = (*semQueueAdapter)(nil)
	_ semantic.LiveAccessor              = (*semLiveAdapter)(nil)
	_ semantic.RetrievalAccessor         = (*semRetrievalAdapter)(nil)
	_ semantic.CompactorAccessor         = (*semCompactorAdapter)(nil)
	_ semantic.SessionAccessor           = (*semSessionAdapter)(nil)
	_ retrieval.StoreReader              = (*semanticstore.Store)(nil)
	_ integ.SemanticLookup               = (*integSemanticLookup)(nil)
	_ semantic.SymbolByNameAccessor      = (*semP1SymbolByNameAdapter)(nil)
	_ semantic.ExtractorRunAccessor      = (*semP1ExtractorRunAdapter)(nil)
	_ semantic.ClusterMapAccessor        = (*semP1ClusterMapAdapter)(nil)
	_ semantic.ClusterMemberAccessor     = (*semP1ClusterMemberAdapter)(nil)
	_ semantic.ClusterPageRankAccessor   = (*semP1ClusterPageRankAdapter)(nil)
	_ semantic.SymbolEdgesAccessor       = (*semP1SymbolEdgesAdapter)(nil)
	_ semantic.ClusterMembershipAccessor = (*semP1ClusterMembershipAdapter)(nil)
)

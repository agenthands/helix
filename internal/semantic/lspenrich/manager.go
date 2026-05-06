package lspenrich

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	serr "github.com/agenthands/helix/internal/errors"
	"github.com/agenthands/helix/internal/kernel/lspool"
	"github.com/agenthands/helix/internal/semantic"
	"github.com/agenthands/helix/internal/workspace"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/singleflight"
)

// LeaseReleaser is the optional interface a LeaseAcquirer may satisfy when it
// owns the underlying lease lifecycle.  Production: *lspool.Pool implements
// `ReleaseLease(sessionID string)`.  When the wrapping LeaseAcquirer is
// *PoolAcquirer (or any adapter that exposes `ReleaseLease(string)`), the
// Manager invokes it on Stop / OnWorkspaceDeactivate to honor the B2 lease
// lifecycle invariant (CONTEXT lines 492-494).
//
// When the LeaseAcquirer does NOT satisfy this interface (unit-test fakes
// that don't track release), the Manager simply drops the cached handle —
// the test asserts the cache invariant via the AcquireFor call counter.
type LeaseReleaser interface {
	ReleaseLease(sessionID string)
}

// wsLangKey is the cache key for the per-(wsKey, lang) lease map.
type wsLangKey struct {
	ws   workspace.WorkspaceKey
	lang string
}

// Manager owns the enrichment workers AND the per-(wsKey, lang) lease cache
// (B2 fix-path-A).  Single instance per daemon (D-02: cap is global across
// languages and workspaces).
//
// **B2 invariant**: AcquireFor returns a long-lived clean lease cached per
// (wsKey, lang).  ErrCircuitOpen and other AcquireLease errors are NOT
// cached — the slot is left empty so the next job retries.  Stop +
// OnWorkspaceDeactivate release cached leases via the LeaseReleaser
// optional interface (production: *lspool.Pool.ReleaseLease).
//
// The Manager also satisfies the P02 Worker.LeaseProvider interface so the
// worker can call back into Manager.AcquireFor for production wiring.
type Manager struct {
	queue     *LaneQueue
	acquirer  LeaseAcquirer
	store     CascadeStore // narrow seam consumed by Worker
	readiness ReadinessProbe
	cfg       semantic.LSPEnrichmentConfig
	metrics   MetricsSink
	logger    *slog.Logger
	tracker   *statusTracker

	// newCascadeLSP is the factory used by Manager.Run to construct
	// Worker.NewCascadeLSP.  nil-safe: when nil, Worker.processOne emits
	// OutcomeDropped (the pre-61-05 behavior).  Production wiring in
	// internal/daemon/live_wiring.go MUST set this via
	// SetCascadeLSPFactory before Run is invoked.
	newCascadeLSP CascadeLSPFactory

	// cancel is set inside Run so Stop can cancel the worker goroutines.
	cancelMu sync.Mutex
	cancel   context.CancelFunc

	// **B2 — lease cache**: per (wsKey, lang) long-lived clean lease.
	// Released on OnWorkspaceDeactivate (workspace) or Stop (all).
	// ErrCircuitOpen on AcquireLease is NOT cached — slot left empty so
	// the next job retries.
	leasesMu sync.Mutex
	leases   map[wsLangKey]*lspool.WorkerLease

	// acquireFlight serializes concurrent AcquireFor calls on the SAME
	// (wsKey, lang) key — at most ONE underlying AcquireLease invocation
	// runs in-flight per key.  All concurrent callers share the result.
	acquireFlight singleflight.Group
}

// NewManager constructs a Manager with the supplied collaborators.  Both
// metrics and logger are nil-safe (fall back to noopMetrics / slog.Default
// at first use).  store + readiness MAY be nil for tests that exercise the
// lease-cache surface only — the Run path requires non-nil dependencies for
// the Worker pipeline.
//
// Phase 61-05 production-wiring seam: NewManager does NOT take a
// CascadeLSPFactory directly — call SetCascadeLSPFactory after
// construction (typically from internal/daemon/live_wiring.go).  When
// no factory is set, Worker.processOne lands every job on
// OutcomeDropped + an Error log (the pre-61-05 behaviour, kept as the
// safe default for tests that exercise only the lease-cache surface).
func NewManager(
	queue *LaneQueue,
	acquirer LeaseAcquirer,
	store CascadeStore,
	readiness ReadinessProbe,
	cfg semantic.LSPEnrichmentConfig,
	metrics MetricsSink,
	logger *slog.Logger,
) *Manager {
	if metrics == nil {
		metrics = noopMetrics{}
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Manager{
		queue:     queue,
		acquirer:  acquirer,
		store:     store,
		readiness: readiness,
		cfg:       cfg,
		metrics:   metrics,
		logger:    logger,
		tracker:   newStatusTracker(),
		leases:    make(map[wsLangKey]*lspool.WorkerLease),
	}
}

// SetCascadeLSPFactory injects the production CascadeLSP factory.  MUST
// be called BEFORE Run is invoked; calling after Run has started is a
// no-op (the Worker has already been constructed with whatever factory
// value was set at the moment Run() reached the Worker struct literal).
//
// nil is permitted (preserves the pre-61-05 dropped-job behaviour for
// tests that exercise only the readiness/dropped paths via direct
// Worker construction without going through Manager.Run).
//
// Production wiring (internal/daemon/live_wiring.go) supplies a closure
// that calls NewCascadeLSPShim(lease) to bind the worker's per-job
// *lspool.WorkerLease to a CascadeLSP shim that dispatches LSP method
// calls to lease.Request.
func (m *Manager) SetCascadeLSPFactory(f CascadeLSPFactory) {
	m.newCascadeLSP = f
}

// AcquireFor returns the cached *WorkerLease for (wsKey, lang) or lazily
// acquires one (sessionID = "lsp-enrichment:<repo>:<lang>"; dirty=false; clean
// lease for share-until-dirty reuse with foreground sessions).  ErrCircuitOpen
// (and any other AcquireLease error) surfaces the error WITHOUT caching the
// failed handle — the next call retries.
//
// Concurrency: a sync.Mutex around the map + acquire is sufficient here
// because lease acquisition is rare relative to job throughput (one Acquire
// per (workspace, language) per process lifetime under normal operation).
// For 100 concurrent AcquireFor calls on a cold key, the lock funnels to a
// single AcquireLease call — the post-acquire re-check handles the race.
//
// Manager satisfies the P02 LeaseProvider interface (worker.go).
func (m *Manager) AcquireFor(ctx context.Context, ws workspace.WorkspaceKey, lang string) (*lspool.WorkerLease, error) {
	key := wsLangKey{ws: ws, lang: lang}

	// Fast path: cache hit, no flight required.
	m.leasesMu.Lock()
	if cached, ok := m.leases[key]; ok && cached != nil {
		m.leasesMu.Unlock()
		return cached, nil
	}
	m.leasesMu.Unlock()

	// Slow path: at most ONE AcquireLease for this key runs across all
	// concurrent callers (singleflight).  Other callers block on Do and
	// receive the shared result.  ErrCircuitOpen and other errors are
	// surfaced as-is and NOT cached so the next call retries.
	flightKey := sessionIDFor(ws, lang)
	v, err, _ := m.acquireFlight.Do(flightKey, func() (interface{}, error) {
		// Re-check the cache under the shared singleflight identity in
		// case the cache was populated between our miss and entering Do
		// (very narrow window; defensive).
		m.leasesMu.Lock()
		if cached, ok := m.leases[key]; ok && cached != nil {
			m.leasesMu.Unlock()
			return cached, nil
		}
		m.leasesMu.Unlock()

		sessionID := flightKey
		lease, lerr := m.acquirer.AcquireLease(ctx, sessionID, ws, false /* clean */)
		if lerr != nil {
			// B2 — ErrCircuitOpen and other errors are NOT cached.
			_ = errors.Is(lerr, serr.ErrCircuitOpen) // documented contract
			return nil, lerr
		}
		// Insert into cache while still holding singleflight identity.
		m.leasesMu.Lock()
		m.leases[key] = lease
		m.leasesMu.Unlock()
		return lease, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(*lspool.WorkerLease), nil
}

// OnWorkspaceDeactivate releases all cached leases for the given workspace
// key.  Wired in P03-T5 from the daemon's existing workspace-deactivate path
// (B2 — CONTEXT lines 492-494: "long-lived per (workspace, language) lease
// released on workspace.OnDeactivate callback").
func (m *Manager) OnWorkspaceDeactivate(ws workspace.WorkspaceKey) {
	m.leasesMu.Lock()
	defer m.leasesMu.Unlock()
	for k, lease := range m.leases {
		if k.ws == ws {
			m.releaseLease(lease)
			delete(m.leases, k)
		}
	}
}

// Run starts cfg.MaxConcurrentWorkers worker goroutines (clamped to >= 1)
// draining the shared LaneQueue.  Returns ctx.Err() when ctx is cancelled or
// the worker errgroup returns a non-nil error.  On Run exit (success or
// failure) all cached leases are released.
func (m *Manager) Run(ctx context.Context) error {
	if m.queue == nil {
		return errors.New("lspenrich.Manager.Run: nil queue")
	}
	ctx, cancel := context.WithCancel(ctx)
	m.cancelMu.Lock()
	m.cancel = cancel
	m.cancelMu.Unlock()

	wrappedMetrics := &trackedMetrics{inner: m.metrics, tracker: m.tracker}
	if m.newCascadeLSP == nil {
		// 61-05: surface the deferred-wiring failure mode loudly at
		// startup instead of silently at first-job time (when every
		// dispatched job lands on OutcomeDropped + an Error log).
		m.logger.Warn("lsp-enrichment manager: no CascadeLSPFactory set; all dispatched jobs will land on OutcomeDropped")
	}
	w := &Worker{
		Queue:         m.queue,
		Leases:        m,          // B2 — Manager IS the LeaseProvider.
		Acquirer:      m.acquirer, // For ForegroundBusy only.
		Store:         m.store,
		Readiness:     m.readiness,
		BudgetCfg:     m.cfg,
		Metrics:       wrappedMetrics,
		Logger:        m.logger,
		NewCascadeLSP: m.newCascadeLSP, // 61-05 production-dispatch wiring
	}
	n := m.cfg.MaxConcurrentWorkers
	if n <= 0 {
		n = 1
	}
	m.logger.Info("lsp-enrichment manager starting",
		"max_concurrent_workers", n,
		"yield_check_window_ms", m.cfg.YieldCheckWindowMs,
		"timeout_per_file", m.cfg.TimeoutPerFile,
		"cascade_lsp_factory", cascadeFactoryStateLabel(m.newCascadeLSP),
	)
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error { return w.RunN(gctx, n) })
	err := g.Wait()
	// **B2** — on Run exit, release all cached leases.
	m.releaseAll()
	return err
}

// Stop cancels any in-flight Run goroutines and releases all cached leases.
// Idempotent: calling Stop twice is safe (the second call is a no-op for the
// cancel side; releaseAll on an empty cache is a no-op).
func (m *Manager) Stop() {
	m.cancelMu.Lock()
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	m.cancelMu.Unlock()
	m.releaseAll()
}

// Status returns a defensive snapshot of the manager's state.  Phase 65 wires
// this into get_health.
func (m *Manager) Status() Status {
	depths := map[Lane]int{
		LaneHigh:       0,
		LaneBackground: 0,
	}
	if m.queue != nil {
		depths[LaneHigh] = m.queue.Depth(LaneHigh)
		depths[LaneBackground] = m.queue.Depth(LaneBackground)
	}
	return m.tracker.snapshot(depths)
}

// Queue returns the underlying lane queue.  Test-only / Phase 64 admin tool
// surface (refresh_semantic_graph will replace direct queue access with a
// public Enqueue API).
func (m *Manager) Queue() *LaneQueue { return m.queue }

// RecordOutcomeForTesting bumps the per-outcome counters from outside the
// worker pipeline.  Used by status_test.go to exercise the W7 partial_budget
// invariant without spinning up a Worker.
func (m *Manager) RecordOutcomeForTesting(o Outcome) { m.tracker.recordOutcome(o) }

// RecordErrorForTesting writes to the LastErrorPerLanguage map from outside
// the worker pipeline.  Used by status_test.go to verify W11 string-error
// semantics without spinning up a Worker.
func (m *Manager) RecordErrorForTesting(lang, msg string) { m.tracker.recordError(lang, msg) }

// releaseAll releases every cached lease (any workspace, any language).
// Called on Stop and on Run exit.  Holds the cache lock for the duration.
func (m *Manager) releaseAll() {
	m.leasesMu.Lock()
	defer m.leasesMu.Unlock()
	for k, lease := range m.leases {
		m.releaseLease(lease)
		delete(m.leases, k)
	}
}

// releaseLease asks the LeaseAcquirer (if it satisfies the LeaseReleaser
// optional interface) to release the underlying pool lease.  When the
// acquirer does not implement ReleaseLease (test fakes), the cached handle
// is simply dropped — that is sufficient to satisfy the cache-invariant
// tests; production wiring routes through *lspool.Pool which DOES implement
// ReleaseLease.
//
// MUST be called with m.leasesMu held by the caller (releaseAll +
// OnWorkspaceDeactivate paths) OR with no concurrent caller to leases (the
// AcquireFor singleflight-loser branch — the lease is local, not yet
// inserted into the cache).
func (m *Manager) releaseLease(lease *lspool.WorkerLease) {
	if lease == nil {
		return
	}
	if r, ok := m.acquirer.(LeaseReleaser); ok {
		r.ReleaseLease(lease.SessionID)
	}
}

// cascadeFactoryStateLabel returns "production" when f is non-nil and
// "<unset>" otherwise.  Used in the startup INFO log so operators can
// see at a glance whether the production dispatch path is wired or the
// no-op (pre-61-05) default is in effect.
func cascadeFactoryStateLabel(f CascadeLSPFactory) string {
	if f == nil {
		return "<unset>"
	}
	return "production"
}

// sessionIDFor returns the canonical lsp-enrichment session ID for (ws,
// lang).  Format: "lsp-enrichment:<repoRoot>:<lang>".  CONTEXT line 269:
// "Enrichment session IDs MUST start with the lsp-enrichment: prefix; the
// pool relies on this prefix to distinguish enrichment from foreground
// traffic."
func sessionIDFor(ws workspace.WorkspaceKey, lang string) string {
	return fmt.Sprintf("lsp-enrichment:%s:%s", ws.RepoRoot, lang)
}

// trackedMetrics decorates a MetricsSink with statusTracker side-effects.
// On every LSPEnrichmentTotal call it parses the outcome string back into the
// closed-enum Outcome and bumps the matching counter; on every
// LSPEnrichmentErrors call it records the error class per language.  The
// inner sink continues to receive every call verbatim (no fan-out / no drop).
type trackedMetrics struct {
	inner   MetricsSink
	tracker *statusTracker
}

func (t *trackedMetrics) LSPEnrichmentTotal(language, outcome string) {
	t.tracker.recordOutcome(Outcome(outcome))
	t.inner.LSPEnrichmentTotal(language, outcome)
}

func (t *trackedMetrics) LSPEnrichmentDuration(language string, secs float64) {
	t.inner.LSPEnrichmentDuration(language, secs)
}

func (t *trackedMetrics) LSPEnrichmentErrors(language, outcome string) {
	t.tracker.recordError(language, outcome)
	t.inner.LSPEnrichmentErrors(language, outcome)
}

func (t *trackedMetrics) LSPEnrichmentLaneDepth(lane string, depth int) {
	t.inner.LSPEnrichmentLaneDepth(lane, depth)
}

func (t *trackedMetrics) LSPEnrichmentBulkSuppressed(n int) {
	t.inner.LSPEnrichmentBulkSuppressed(n)
}

// Compile-time assertion: *Manager satisfies LeaseProvider (the seam in
// worker.go).  Production wiring constructs Worker.Leases = manager.
var _ LeaseProvider = (*Manager)(nil)

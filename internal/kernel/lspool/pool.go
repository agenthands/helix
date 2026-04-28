package lspool

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"go.opentelemetry.io/otel/trace"
	tracenoop "go.opentelemetry.io/otel/trace/noop"

	"github.com/postfix/serena/internal/langregistry"
	"github.com/postfix/serena/internal/workspace"
)

// PoolConfig holds configuration for the LS worker pool.
type PoolConfig struct {
	BaseTTL               int // seconds, default 300
	CeilingTTL            int // seconds, default 3600
	MaxWorkers            int // default 10
	RSSHardCapMB          int // default 2048
	PressureCheckInterval int // seconds, default 10
	RestartBudget         int // default 3, consecutive crashes before circuit stays open (D-05)
}

// DefaultPoolConfig returns the default pool configuration.
func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		BaseTTL:               300,
		CeilingTTL:            3600,
		MaxWorkers:            10,
		RSSHardCapMB:          2048,
		PressureCheckInterval: 10,
	}
}

// ErrMaxWorkersReached is returned when the pool has reached its maximum worker count.
var ErrMaxWorkersReached = errors.New("maximum number of workers reached")

// Pool manages a pool of LS workers with TTL, pressure eviction, and share-until-dirty policy.
type Pool struct {
	mu             sync.RWMutex
	workers        map[string]*Worker         // keyed by worker ID
	leases         map[string]*WorkerLease    // keyed by session ID
	circuits       map[string]*CircuitBreaker // keyed by language
	registry       *langregistry.Registry
	installer      *langregistry.Installer
	pressure       MemoryPressure
	config         PoolConfig
	logger         *slog.Logger
	metrics        MetricsSink
	sessionMetrics SessionTimeoutSink
	tracer         trace.Tracer // Phase 55-01: injected for ls.request child spans; nil-safe noop fallback
	nextID         int
	done           chan struct{}
	runCtx         context.Context // lifecycle context from Run(); workers use this instead of request ctx
}

// SetSessionTimeoutSink wires an optional sink that receives the
// timeout phase of workspace lifecycle. Defaults to a no-op when unset,
// so the pool's existing constructors stay backward-compatible. Daemon
// wiring lives in plan 53-03.
func (p *Pool) SetSessionTimeoutSink(s SessionTimeoutSink) {
	if s == nil {
		s = NoopSessionTimeoutSink{}
	}
	p.mu.Lock()
	p.sessionMetrics = s
	p.mu.Unlock()
}

// NewPool creates a new LS worker pool.
// The registry provides language server resolution for worker creation.
// The installer uses three-tier resolution (PATH/download/error) to find LS binaries.
// metrics is the MetricsSink receiving worker lifecycle and circuit state
// events; pass NoopSink{} (or nil, which is converted) to disable.
// tracer is the trace.Tracer threaded into each Worker so Worker.Request can
// emit an `ls.request` child span (Phase 55-01 / OBS-04 #1). When nil, a noop
// tracer is substituted so existing tests that build a Pool without tracing
// continue to compile and run with zero allocation on the hot path.
func NewPool(cfg PoolConfig, registry *langregistry.Registry, installer *langregistry.Installer, pressure MemoryPressure, logger *slog.Logger, metrics MetricsSink, tracer trace.Tracer) *Pool {
	if metrics == nil {
		metrics = NoopSink{}
	}
	if tracer == nil {
		// nil-safe fallback: existing tests in this package construct pools
		// without tracing wired. Returning a noop Tracer here preserves the
		// D-17 zero-allocation property for the noop path.
		tracer = tracenoop.NewTracerProvider().Tracer("lspool")
	}
	return &Pool{
		workers:        make(map[string]*Worker),
		leases:         make(map[string]*WorkerLease),
		circuits:       make(map[string]*CircuitBreaker),
		registry:       registry,
		installer:      installer,
		pressure:       pressure,
		config:         cfg,
		logger:         logger.With("component", "lspool"),
		metrics:        metrics,
		sessionMetrics: NoopSessionTimeoutSink{},
		tracer:         tracer,
		done:           make(chan struct{}),
	}
}

// Run starts background goroutines for TTL checks and pressure eviction.
// Blocks until ctx is cancelled.
func (p *Pool) Run(ctx context.Context) error {
	p.runCtx = ctx // Store lifecycle context for worker spawning.
	ttlTicker := time.NewTicker(30 * time.Second)
	defer ttlTicker.Stop()

	pressureInterval := time.Duration(p.config.PressureCheckInterval) * time.Second
	if pressureInterval <= 0 {
		pressureInterval = 10 * time.Second
	}
	pressureTicker := time.NewTicker(pressureInterval)
	defer pressureTicker.Stop()

	defer close(p.done)

	for {
		select {
		case <-ctx.Done():
			p.stopAll(context.Background())
			return ctx.Err()
		case <-ttlTicker.C:
			p.checkTTLs()
		case <-pressureTicker.C:
			p.checkPressure()
		}
	}
}

// AcquireLease creates a lease binding a session to an LS worker.
// Per D-02: clean sessions share warm workers; dirty sessions get dedicated workers.
func (p *Pool) AcquireLease(ctx context.Context, sessionID string, wsKey workspace.WorkspaceKey, dirty bool) (*WorkerLease, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// If not dirty, look for existing warm worker for this language+workDir.
	if !dirty {
		if w := p.workerForKeyLocked(wsKey); w != nil {
			lease := NewWorkerLease(sessionID, w, false)
			p.leases[sessionID] = lease
			p.logger.Info("shared lease acquired", "session", sessionID, "worker", w.ID())
			// Phase 53 D-02: cache HIT, scope=clean (share-until-dirty match).
			p.metrics.LSPoolCacheInc(wsKey.Language, ResultHit, ScopeClean)
			return lease, nil
		}
	}

	// Need a new worker. Check circuit breaker first.
	cb := p.circuitForLanguage(wsKey.Language)
	if !cb.CanAttempt() {
		// Phase 53 D-02: cache MISS, scope=crashed (circuit blocked reuse).
		p.metrics.LSPoolCacheInc(wsKey.Language, ResultMiss, ScopeCrashed)
		return nil, cb.CircuitOpenErr()
	}

	// Check max workers limit.
	if len(p.workers) >= p.config.MaxWorkers {
		// Phase 53 WR-03: capacity exhaustion is orthogonal to cache
		// effectiveness. Emitting a miss here would distort the hit-rate
		// signal — operators reading serena_lspool_cache_total expect that
		// metric to reflect cache reuse, not pool sizing. Skip emission and
		// surface the capacity error instead, matching the "PREFER skipping
		// over mislabeling" pattern used in DeactivateWorkspace.
		return nil, ErrMaxWorkersReached
	}

	// Phase 53 IN-04: capture circuit failure count BEFORE spawn so the
	// restart-after-failures emission is independent of cb.RecordSuccess()
	// ordering. spawnWorkerLocked no longer emits LSPoolRestart itself.
	priorFailures := cb.Failures()

	// Spawn new worker.
	worker, err := p.spawnWorkerLocked(ctx, wsKey)
	if err != nil {
		cb.RecordFailure()
		// Phase 53 D-02: spawn failure is a circuit-relevant signal -> crashed.
		p.metrics.LSPoolCacheInc(wsKey.Language, ResultMiss, ScopeCrashed)
		return nil, fmt.Errorf("spawning worker: %w", err)
	}
	cb.RecordSuccess()
	// Phase 53 D-15 / IN-04: emit a restart event when the circuit had
	// recorded failures before this successful spawn.
	if priorFailures > 0 {
		p.metrics.LSPoolRestart(wsKey.Language)
	}

	lease := NewWorkerLease(sessionID, worker, dirty)
	p.leases[sessionID] = lease
	p.logger.Info("new lease acquired", "session", sessionID, "worker", worker.ID(), "dirty", dirty)
	// Phase 53 D-02: cache MISS, scope=clean for fresh share-eligible spawn,
	// scope=dirty for dedicated-worker spawn.
	scope := ScopeClean
	if dirty {
		scope = ScopeDirty
	}
	p.metrics.LSPoolCacheInc(wsKey.Language, ResultMiss, scope)
	return lease, nil
}

// ReleaseLease releases a session's lease. If the worker has no remaining leases,
// its idle TTL countdown begins.
func (p *Pool) ReleaseLease(sessionID string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	lease, ok := p.leases[sessionID]
	if !ok {
		return
	}
	delete(p.leases, sessionID)
	p.logger.Info("lease released", "session", sessionID, "worker", lease.Worker.ID())
}

// PromoteToDirty promotes a clean shared lease to a dirty dedicated worker.
// Per D-08: spawns new worker, transfers session, releases old shared lease.
func (p *Pool) PromoteToDirty(ctx context.Context, sessionID string) (*WorkerLease, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	oldLease, ok := p.leases[sessionID]
	if !ok {
		return nil, fmt.Errorf("no lease found for session %s", sessionID)
	}

	if oldLease.Dirty {
		return oldLease, nil // Already dirty.
	}

	wsKey := workspace.WorkspaceKey{
		RepoRoot:  oldLease.Worker.WorkDir(),
		Language:  oldLease.Worker.Language(),
		Toolchain: "", // Will be populated from original key if needed.
	}

	// Check circuit breaker.
	cb := p.circuitForLanguage(wsKey.Language)
	if !cb.CanAttempt() {
		return nil, cb.CircuitOpenErr()
	}

	// Check max workers.
	if len(p.workers) >= p.config.MaxWorkers {
		return nil, ErrMaxWorkersReached
	}

	// Phase 53 IN-04: capture circuit failure count BEFORE spawn so the
	// restart-after-failures emission is independent of cb.RecordSuccess()
	// ordering. spawnWorkerLocked no longer emits LSPoolRestart itself.
	priorFailures := cb.Failures()

	// Spawn new dedicated worker.
	newWorker, err := p.spawnWorkerLocked(ctx, wsKey)
	if err != nil {
		cb.RecordFailure()
		return nil, fmt.Errorf("spawning dirty worker: %w", err)
	}
	cb.RecordSuccess()
	// Phase 53 D-15 / IN-04: emit a restart event when the circuit had
	// recorded failures before this successful spawn.
	if priorFailures > 0 {
		p.metrics.LSPoolRestart(wsKey.Language)
	}

	// Remove old lease.
	delete(p.leases, sessionID)

	// Create new dirty lease.
	newLease := NewWorkerLease(sessionID, newWorker, true)
	p.leases[sessionID] = newLease

	p.logger.Info("promoted to dirty", "session", sessionID,
		"old_worker", oldLease.Worker.ID(), "new_worker", newWorker.ID())
	return newLease, nil
}

// WorkerCount returns the number of active workers.
func (p *Pool) WorkerCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.workers)
}

// WorkerForTests returns the first worker matching (language, workDir).
// TEST-ONLY — used by integration tests to reach into adapter readiness state
// (e.g. Phase 56 TestRustAnalyzer_NotificationDispatchEndToEnd, waitJavaReady).
// Do NOT use this in production code paths; use AcquireLease instead.
func (p *Pool) WorkerForTests(language, workDir string) *Worker {
	p.mu.RLock()
	defer p.mu.RUnlock()
	for _, w := range p.workers {
		if w.Language() == language && w.WorkDir() == workDir {
			return w
		}
	}
	return nil
}

// LeaseCount returns the number of active leases.
func (p *Pool) LeaseCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.leases)
}

// workerForKeyLocked finds a warm Ready worker matching the workspace key.
// Must be called with p.mu held.
func (p *Pool) workerForKeyLocked(wsKey workspace.WorkspaceKey) *Worker {
	for _, w := range p.workers {
		if w.Language() == wsKey.Language && w.WorkDir() == wsKey.RepoRoot && w.State() == WorkerReady {
			return w
		}
	}
	return nil
}

// spawnWorkerLocked creates and starts a new worker. Must be called with p.mu held.
func (p *Pool) spawnWorkerLocked(ctx context.Context, wsKey workspace.WorkspaceKey) (*Worker, error) {
	p.nextID++
	id := fmt.Sprintf("w-%s-%d", wsKey.Language, p.nextID)

	// Resolve LS binary from the language registry.
	entry, ok := p.registry.Get(wsKey.Language)
	if !ok {
		return nil, fmt.Errorf("no language server configured for %s", wsKey.Language)
	}

	// Use three-tier installer resolution (PATH/download/error) if available,
	// otherwise fall back to entry.Command directly.
	command, args := entry.Command, entry.Args
	if p.installer != nil {
		resolved, resolvedArgs, err := p.installer.Resolve(ctx, entry)
		if err != nil {
			return nil, fmt.Errorf("resolving language server for %s: %w", wsKey.Language, err)
		}
		command, args = resolved, resolvedArgs
	}

	quirks := GetQuirkAdapter(entry)
	worker := NewWorker(id, wsKey.Language, wsKey.RepoRoot, command, args, p.logger, p.tracer)
	worker.SetQuirks(quirks)

	// Start the worker with the pool's lifecycle context (not the request context)
	// so the LS process outlives individual tool calls.
	startCtx := ctx
	if p.runCtx != nil {
		startCtx = p.runCtx
	}
	p.mu.Unlock()
	err := worker.Start(startCtx)
	p.mu.Lock()

	if err != nil {
		return nil, err
	}

	p.workers[id] = worker
	// METRIC-03: worker gauge +1 on spawn.
	p.metrics.LSPoolWorkersSet(worker.Language(), +1)
	// Phase 53 IN-04: the restart-after-failures emission moved to the
	// callers (AcquireLease / PromoteToDirty), which capture priorFailures
	// BEFORE spawnWorkerLocked runs and emit only after a successful spawn.
	// Keeping the emission here previously coupled correctness to the
	// caller invoking cb.RecordSuccess() AFTER spawn returned — fragile if
	// a future refactor inlines RecordSuccess inside this helper. The
	// caller-driven pattern decouples the metric from temporal ordering.
	return worker, nil
}

// circuitForLanguage returns (or creates) the circuit breaker for a language.
// Must be called with p.mu held.
func (p *Pool) circuitForLanguage(language string) *CircuitBreaker {
	cb, ok := p.circuits[language]
	if !ok {
		cb = NewCircuitBreaker(language, 5*time.Minute, p.config.RestartBudget, p.metrics)
		p.circuits[language] = cb
	}
	return cb
}

// checkTTLs iterates workers and retires idle ones whose TTL has expired.
func (p *Pool) checkTTLs() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for id, w := range p.workers {
		if w.State() != WorkerReady {
			continue
		}

		// Check if worker has active leases.
		hasLease := false
		for _, lease := range p.leases {
			if lease.Worker.ID() == id {
				hasLease = true
				break
			}
		}
		if hasLease {
			continue
		}

		// Compute adaptive TTL.
		ttl := w.Metrics().TTL(p.config.BaseTTL, p.config.CeilingTTL)
		if ttl == 0 {
			continue // TTL=0 means no idle timeout (D-07).
		}

		idle := w.Metrics().IdleDuration()
		if idle >= time.Duration(ttl)*time.Second {
			p.logger.Info("retiring idle worker", "worker", id, "idle", idle, "ttl", ttl)
			lang := w.Language()
			go w.Stop(context.Background())
			delete(p.workers, id)
			// METRIC-03: idle-TTL retirement path.
			p.metrics.LSPoolWorkersSet(lang, -1)
			p.metrics.LSPoolEviction(lang, EvictIdle)
			// Phase 53 D-04: workspace lifecycle "timeout" phase. Emitted via
			// the parallel SessionTimeoutSink to avoid the lspool↔kernel cycle.
			// Daemon adapter forwards this to *obs.Metrics.SessionLifecycleInc(lang, "timeout").
			p.sessionMetrics.SessionTimeout(lang)
		}
	}
}

// checkPressure evicts workers under memory pressure per D-05 eviction order.
func (p *Pool) checkPressure() {
	if p.pressure == nil {
		return
	}

	level := p.pressure.Level()
	if level < PressureHigh {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.logger.Warn("memory pressure detected", "level", level.String())

	// Eviction order per D-05:
	// (1) RSS > hard cap
	// (2) unhealthy/stuck
	// (3) LRU with score==0
	// (4) lowest score/highest RSS ratio
	// (5) oldest idle

	hardCapBytes := uint64(p.config.RSSHardCapMB) * 1024 * 1024

	// Pass 1: RSS > hard cap.
	for id, w := range p.workers {
		if w.State() != WorkerReady {
			continue
		}
		rss, err := p.pressure.WorkerRSS(w.Pid())
		if err != nil {
			continue
		}
		if rss > hardCapBytes {
			p.logger.Warn("evicting worker: RSS exceeds hard cap", "worker", id, "rss_mb", rss/1024/1024)
			p.evictWorkerLocked(id, w, EvictPressure)
			return // Evict one at a time.
		}
	}

	// Pass 2: Unhealthy/stuck workers (non-Ready, non-Starting states that aren't shutting down).
	for id, w := range p.workers {
		state := w.State()
		if state != WorkerReady && state != WorkerStarting && state != WorkerInitializing && state != WorkerShuttingDown && state != WorkerStopped {
			p.logger.Warn("evicting unhealthy worker", "worker", id, "state", state.String())
			// Unhealthy state reached outside the normal transition graph is
			// how crashes surface in the current pool: the LS process is
			// gone but the worker record remains. Book it as a crash.
			p.evictWorkerLocked(id, w, EvictCrash)
			return
		}
	}

	// Pass 3: LRU with score==0 (no recent reuse).
	for id, w := range p.workers {
		if w.State() != WorkerReady {
			continue
		}
		m := w.Metrics()
		m.mu.Lock()
		score := m.ReuseScore
		m.mu.Unlock()
		if score < 0.01 {
			p.logger.Warn("evicting zero-score worker", "worker", id)
			p.evictWorkerLocked(id, w, EvictPressure)
			return
		}
	}

	// Pass 4-5: oldest idle worker as fallback.
	var oldestID string
	var oldestIdle time.Duration
	for id, w := range p.workers {
		if w.State() != WorkerReady {
			continue
		}
		idle := w.Metrics().IdleDuration()
		if idle > oldestIdle {
			oldestIdle = idle
			oldestID = id
		}
	}
	if oldestID != "" {
		p.logger.Warn("evicting oldest idle worker", "worker", oldestID, "idle", oldestIdle)
		p.evictWorkerLocked(oldestID, p.workers[oldestID], EvictPressure)
	}
}

// evictWorkerLocked stops and removes a worker. Must be called with p.mu held.
// reason is one of EvictIdle / EvictPressure / EvictCrash / EvictShutdown and
// is emitted on the evictions counter (D-13).
func (p *Pool) evictWorkerLocked(id string, w *Worker, reason string) {
	lang := w.Language()
	// Remove any leases for this worker.
	for sid, lease := range p.leases {
		if lease.Worker.ID() == id {
			delete(p.leases, sid)
		}
	}
	delete(p.workers, id)
	go w.Stop(context.Background())
	// METRIC-03: worker gauge -1 + reasoned eviction counter.
	p.metrics.LSPoolWorkersSet(lang, -1)
	p.metrics.LSPoolEviction(lang, reason)
}

// stopAll stops all workers.
func (p *Pool) stopAll(ctx context.Context) {
	p.mu.Lock()
	type snap struct {
		w    *Worker
		lang string
	}
	workers := make([]snap, 0, len(p.workers))
	for _, w := range p.workers {
		workers = append(workers, snap{w: w, lang: w.Language()})
	}
	p.workers = make(map[string]*Worker)
	p.leases = make(map[string]*WorkerLease)
	p.mu.Unlock()

	for _, s := range workers {
		_ = s.w.Stop(ctx)
		// METRIC-03: shutdown path — gauge -1 + reasoned eviction counter.
		p.metrics.LSPoolWorkersSet(s.lang, -1)
		p.metrics.LSPoolEviction(s.lang, EvictShutdown)
	}
}

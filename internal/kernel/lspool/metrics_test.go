package lspool

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/postfix/serena/internal/workspace"
)

// recordingSink is a thread-safe MetricsSink that captures every call for
// assertion. Slices are append-only under a mutex so tests can make
// order-dependent assertions without racing.
type recordingSink struct {
	mu sync.Mutex

	workers        []workerEvent
	evictions      []evictionEvent
	circuitStates  []circuitEvent
	restarts       []string
	cacheDecisions []cacheEvent
}

type cacheEvent struct {
	lang   string
	result string
	scope  string
}

type workerEvent struct {
	lang  string
	delta float64
}

type evictionEvent struct {
	lang   string
	reason string
}

type circuitEvent struct {
	lang  string
	state float64
}

func (r *recordingSink) LSPoolWorkersSet(language string, delta float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.workers = append(r.workers, workerEvent{lang: language, delta: delta})
}

func (r *recordingSink) LSPoolEviction(language, reason string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.evictions = append(r.evictions, evictionEvent{lang: language, reason: reason})
}

func (r *recordingSink) LSPoolCircuitStateSet(language string, state float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.circuitStates = append(r.circuitStates, circuitEvent{lang: language, state: state})
}

func (r *recordingSink) LSPoolRestart(language string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.restarts = append(r.restarts, language)
}

func (r *recordingSink) LSPoolCacheInc(language, result, scope string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cacheDecisions = append(r.cacheDecisions, cacheEvent{lang: language, result: result, scope: scope})
}

// snapshot returns a consistent copy of all recorded events.
func (r *recordingSink) snapshot() (w []workerEvent, e []evictionEvent, c []circuitEvent, rs []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	w = append(w, r.workers...)
	e = append(e, r.evictions...)
	c = append(c, r.circuitStates...)
	rs = append(rs, r.restarts...)
	return
}

// --- Interface / no-op tests ---------------------------------------------

func TestMetricsSink_NoopSinkSatisfiesInterface(t *testing.T) {
	var _ MetricsSink = NoopSink{}
}

func TestNoopSink_safe(t *testing.T) {
	// All four methods must be callable without panic on the zero value.
	var sink MetricsSink = NoopSink{}
	sink.LSPoolWorkersSet("go", +1)
	sink.LSPoolWorkersSet("go", -1)
	sink.LSPoolEviction("go", EvictIdle)
	sink.LSPoolEviction("go", EvictPressure)
	sink.LSPoolEviction("go", EvictCrash)
	sink.LSPoolEviction("go", EvictShutdown)
	sink.LSPoolCircuitStateSet("go", CircuitClosed)
	sink.LSPoolCircuitStateSet("go", CircuitHalfOpen)
	sink.LSPoolCircuitStateSet("go", CircuitOpen)
	sink.LSPoolRestart("go")
	sink.LSPoolCacheInc("go", ResultHit, ScopeClean)
	sink.LSPoolCacheInc("go", ResultMiss, ScopeDirty)
	sink.LSPoolCacheInc("go", ResultMiss, ScopeCrashed)
}

func TestMetricsSink_EvictionReasonConstants(t *testing.T) {
	assert.Equal(t, "idle", EvictIdle)
	assert.Equal(t, "pressure", EvictPressure)
	assert.Equal(t, "crash", EvictCrash)
	assert.Equal(t, "shutdown", EvictShutdown)
}

func TestMetricsSink_CircuitStateConstants(t *testing.T) {
	assert.Equal(t, float64(0), CircuitClosed)
	assert.Equal(t, float64(1), CircuitHalfOpen)
	assert.Equal(t, float64(2), CircuitOpen)
}

// --- Pool lifecycle tests (unit level; no real worker process) -----------

// newTestPoolWithSink builds a Pool wired to a recording sink. We use the
// same testRegistry/testLogger helpers already present in pool_test.go.
func newTestPoolWithSink(t *testing.T, sink MetricsSink) *Pool {
	t.Helper()
	return NewPool(testPoolConfig(), testRegistry(), nil, &mockPressure{level: PressureNone}, testLogger(), sink)
}

// fakeWorker returns a Worker stub with just enough fields populated for the
// lifecycle hooks to exercise. We bypass Start() because those tests would
// need a real LS binary.
func fakeWorker(id, lang string) *Worker {
	w := NewWorker(id, lang, "/tmp/test-"+lang, "true", nil, testLogger())
	w.state.Store(int32(WorkerReady))
	return w
}

func TestPool_evictWorkerLocked_EmitsGaugeAndReason(t *testing.T) {
	cases := []struct {
		name   string
		reason string
	}{
		{"idle", EvictIdle},
		{"pressure", EvictPressure},
		{"crash", EvictCrash},
		{"shutdown", EvictShutdown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sink := &recordingSink{}
			p := newTestPoolWithSink(t, sink)

			w := fakeWorker("w-go-1", "go")
			p.mu.Lock()
			p.workers[w.ID()] = w
			p.evictWorkerLocked(w.ID(), w, tc.reason)
			p.mu.Unlock()

			wEvents, eEvents, _, _ := sink.snapshot()
			if assert.Len(t, wEvents, 1, "expected exactly one gauge event") {
				assert.Equal(t, "go", wEvents[0].lang)
				assert.Equal(t, float64(-1), wEvents[0].delta)
			}
			if assert.Len(t, eEvents, 1, "expected exactly one eviction event") {
				assert.Equal(t, "go", eEvents[0].lang)
				assert.Equal(t, tc.reason, eEvents[0].reason)
			}
			assert.NotContains(t, p.workers, w.ID(), "worker should be removed")
		})
	}
}

func TestPool_stopAll_EmitsShutdownEvictions(t *testing.T) {
	sink := &recordingSink{}
	p := newTestPoolWithSink(t, sink)

	// Inject two fake workers of different languages.
	wGo := fakeWorker("w-go-1", "go")
	wPy := fakeWorker("w-py-1", "python")
	// Force both into the stopped state so w.Stop() returns quickly without
	// touching an LS process.
	wGo.state.Store(int32(WorkerStopped))
	wPy.state.Store(int32(WorkerStopped))
	p.workers[wGo.ID()] = wGo
	p.workers[wPy.ID()] = wPy

	p.stopAll(nil)

	wEvents, eEvents, _, _ := sink.snapshot()
	assert.Len(t, wEvents, 2)
	assert.Len(t, eEvents, 2)
	langs := map[string]bool{}
	for _, ev := range eEvents {
		assert.Equal(t, EvictShutdown, ev.reason)
		langs[ev.lang] = true
	}
	assert.True(t, langs["go"])
	assert.True(t, langs["python"])
}

// --- Circuit breaker state reporter --------------------------------------

func TestCircuit_stateReport(t *testing.T) {
	sink := &recordingSink{}
	cb := NewCircuitBreaker("go", 10*time.Millisecond, 3, sink)

	// Construction emits initial closed state.
	_, _, states, _ := sink.snapshot()
	if assert.Len(t, states, 1) {
		assert.Equal(t, "go", states[0].lang)
		assert.Equal(t, CircuitClosed, states[0].state)
	}

	// Failure -> open.
	cb.RecordFailure()
	_, _, states, _ = sink.snapshot()
	assert.Equal(t, CircuitOpen, states[len(states)-1].state)

	// Wait out backoff, probe -> half-open.
	time.Sleep(15 * time.Millisecond)
	assert.True(t, cb.CanAttempt())
	_, _, states, _ = sink.snapshot()
	assert.Equal(t, CircuitHalfOpen, states[len(states)-1].state)

	// Success -> closed.
	cb.RecordSuccess()
	_, _, states, _ = sink.snapshot()
	assert.Equal(t, CircuitClosed, states[len(states)-1].state)
}

func TestCircuit_nilSinkReplacedWithNoop(t *testing.T) {
	// Should not panic despite nil sink argument.
	cb := NewCircuitBreaker("go", time.Second, 3, nil)
	cb.RecordFailure()
	cb.RecordSuccess()
	assert.True(t, cb.CanAttempt())
}

// --- Cache decision emission ---------------------------------------------

// TestPool_CacheMetricsEmission exercises four distinct branches in
// AcquireLease (hit/clean shared-lease, miss/crashed circuit-blocked,
// miss/clean MaxWorkers reached, miss/crashed spawn-failure) and asserts
// each emits exactly one LSPoolCacheInc with the right (result, scope).
func TestPool_CacheMetricsEmission(t *testing.T) {
	t.Run("hit/clean shared lease", func(t *testing.T) {
		sink := &recordingSink{}
		p := newTestPoolWithSink(t, sink)

		// Pre-inject a Ready worker matching the workspace key.
		w := fakeWorker("w-go-1", "go")
		w.workDir = "/tmp/wsA"
		p.mu.Lock()
		p.workers[w.ID()] = w
		p.mu.Unlock()

		key := workspace.WorkspaceKey{RepoRoot: "/tmp/wsA", Language: "go"}
		_, err := p.AcquireLease(context.Background(), "s1", key, false)
		assert.NoError(t, err)

		_, _, _, _, cache := snapshotAll(sink)
		if assert.Len(t, cache, 1, "expected one cache emission") {
			assert.Equal(t, "go", cache[0].lang)
			assert.Equal(t, ResultHit, cache[0].result)
			assert.Equal(t, ScopeClean, cache[0].scope)
		}
	})

	t.Run("miss/crashed circuit blocked", func(t *testing.T) {
		sink := &recordingSink{}
		p := newTestPoolWithSink(t, sink)

		// Trip the circuit so CanAttempt returns false.
		cb := NewCircuitBreaker("go", 1*time.Hour, 1, sink)
		cb.RecordFailure() // budget=1, 1 failure trips
		p.mu.Lock()
		p.circuits["go"] = cb
		p.mu.Unlock()

		key := workspace.WorkspaceKey{RepoRoot: "/tmp/wsB", Language: "go"}
		_, err := p.AcquireLease(context.Background(), "s2", key, false)
		assert.Error(t, err)

		_, _, _, _, cache := snapshotAll(sink)
		if assert.Len(t, cache, 1) {
			assert.Equal(t, ResultMiss, cache[0].result)
			assert.Equal(t, ScopeCrashed, cache[0].scope)
		}
	})

	t.Run("miss/clean max workers reached", func(t *testing.T) {
		sink := &recordingSink{}
		// MaxWorkers=0 means any AcquireLease (after no warm worker found)
		// goes straight to ErrMaxWorkersReached.
		cfg := testPoolConfig()
		cfg.MaxWorkers = 0
		p := NewPool(cfg, testRegistry(), nil, &mockPressure{level: PressureNone}, testLogger(), sink)

		key := workspace.WorkspaceKey{RepoRoot: "/tmp/wsC", Language: "go"}
		_, err := p.AcquireLease(context.Background(), "s3", key, false)
		assert.ErrorIs(t, err, ErrMaxWorkersReached)

		_, _, _, _, cache := snapshotAll(sink)
		if assert.Len(t, cache, 1) {
			assert.Equal(t, ResultMiss, cache[0].result)
			assert.Equal(t, ScopeClean, cache[0].scope)
		}
	})

	t.Run("miss/dirty max workers reached", func(t *testing.T) {
		sink := &recordingSink{}
		cfg := testPoolConfig()
		cfg.MaxWorkers = 0
		p := NewPool(cfg, testRegistry(), nil, &mockPressure{level: PressureNone}, testLogger(), sink)

		key := workspace.WorkspaceKey{RepoRoot: "/tmp/wsD", Language: "go"}
		_, err := p.AcquireLease(context.Background(), "s4", key, true)
		assert.ErrorIs(t, err, ErrMaxWorkersReached)

		_, _, _, _, cache := snapshotAll(sink)
		if assert.Len(t, cache, 1) {
			assert.Equal(t, ResultMiss, cache[0].result)
			assert.Equal(t, ScopeDirty, cache[0].scope)
		}
	})

	t.Run("miss/crashed spawn failure", func(t *testing.T) {
		sink := &recordingSink{}
		p := newTestPoolWithSink(t, sink)

		// Use an unregistered language so spawnWorkerLocked errors with
		// "no language server configured for X" before ever calling Start.
		key := workspace.WorkspaceKey{RepoRoot: "/tmp/wsE", Language: "no_such_lang_xyz"}
		_, err := p.AcquireLease(context.Background(), "s5", key, false)
		assert.Error(t, err)

		_, _, _, _, cache := snapshotAll(sink)
		// Filter to cache emissions (circuit may also emit state events).
		if assert.GreaterOrEqual(t, len(cache), 1) {
			last := cache[len(cache)-1]
			assert.Equal(t, ResultMiss, last.result)
			assert.Equal(t, ScopeCrashed, last.scope)
		}
	})
}

// snapshotAll extends snapshot() with cache events.
func snapshotAll(r *recordingSink) ([]workerEvent, []evictionEvent, []circuitEvent, []string, []cacheEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	w := append([]workerEvent(nil), r.workers...)
	e := append([]evictionEvent(nil), r.evictions...)
	c := append([]circuitEvent(nil), r.circuitStates...)
	rs := append([]string(nil), r.restarts...)
	cd := append([]cacheEvent(nil), r.cacheDecisions...)
	return w, e, c, rs, cd
}

// TestPool_CheckTTLs_EmitsSessionTimeout asserts that idle-TTL eviction
// also emits SessionTimeout(lang) via the parallel SessionTimeoutSink
// (Phase 53 D-04 timeout phase).
func TestPool_CheckTTLs_EmitsSessionTimeout(t *testing.T) {
	sink := &recordingSink{}
	p := newTestPoolWithSink(t, sink)
	tsink := &recordingTimeoutSink{}
	p.SetSessionTimeoutSink(tsink)

	// Inject a Ready worker with old LastUsedAt to trigger TTL eviction.
	w := fakeWorker("w-go-1", "go")
	wm := w.Metrics()
	wm.StartedAt = time.Now().Add(-2 * time.Hour)
	wm.LastUsedAt = time.Now().Add(-2 * time.Hour)
	p.mu.Lock()
	p.workers[w.ID()] = w
	p.mu.Unlock()

	p.checkTTLs()

	tsink.mu.Lock()
	defer tsink.mu.Unlock()
	if assert.Len(t, tsink.langs, 1) {
		assert.Equal(t, "go", tsink.langs[0])
	}
}

type recordingTimeoutSink struct {
	mu    sync.Mutex
	langs []string
}

func (r *recordingTimeoutSink) SessionTimeout(language string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.langs = append(r.langs, language)
}

// --- Pool construction safety --------------------------------------------

func TestPool_nilMetricsDefaultsToNoop(t *testing.T) {
	// Passing a nil sink must not panic; the Pool should default to NoopSink
	// internally so later hook calls are safe.
	p := NewPool(testPoolConfig(), testRegistry(), nil, &mockPressure{level: PressureNone}, testLogger(), nil)
	assert.NotNil(t, p.metrics)

	// Exercise an eviction path with a fake worker to prove no panic.
	w := fakeWorker("w-go-1", "go")
	p.mu.Lock()
	p.workers[w.ID()] = w
	p.evictWorkerLocked(w.ID(), w, EvictIdle)
	p.mu.Unlock()
}

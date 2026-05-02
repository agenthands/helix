package lspool

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingSink is a thread-safe MetricsSink that captures every call for
// assertion. Slices are append-only under a mutex so tests can make
// order-dependent assertions without racing.
type recordingSink struct {
	mu sync.Mutex

	workers       []workerEvent
	evictions     []evictionEvent
	circuitStates []circuitEvent
	restarts      []string
	lookups       []lookupEvent
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

type lookupEvent struct {
	lang   string
	result string
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

func (r *recordingSink) LSPoolLookup(language, result string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lookups = append(r.lookups, lookupEvent{lang: language, result: result})
}

// snapshot returns a consistent copy of all recorded events. The lookups slice
// is the LAST return value to keep ordering stable when adding new event
// families (Phase 53 D-14).
func (r *recordingSink) snapshot() (w []workerEvent, e []evictionEvent, c []circuitEvent, rs []string, lk []lookupEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	w = append(w, r.workers...)
	e = append(e, r.evictions...)
	c = append(c, r.circuitStates...)
	rs = append(rs, r.restarts...)
	lk = append(lk, r.lookups...)
	return
}

// --- Interface / no-op tests ---------------------------------------------

func TestMetricsSink_NoopSinkSatisfiesInterface(t *testing.T) {
	var _ MetricsSink = NoopSink{}
}

func TestNoopSink_safe(t *testing.T) {
	// All methods must be callable without panic on the zero value.
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
	sink.LSPoolLookup("go", LookupHit)
	sink.LSPoolLookup("go", LookupMiss)
}

func TestMetricsSink_EvictionReasonConstants(t *testing.T) {
	assert.Equal(t, "idle", EvictIdle)
	assert.Equal(t, "pressure", EvictPressure)
	assert.Equal(t, "crash", EvictCrash)
	assert.Equal(t, "shutdown", EvictShutdown)
}

// TestMetricsSink_LookupResultConstants pins the closed-enum result label
// values for helix_lspool_lookups_total. Phase 53 D-04.
func TestMetricsSink_LookupResultConstants(t *testing.T) {
	assert.Equal(t, "hit", LookupHit)
	assert.Equal(t, "miss", LookupMiss)
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
	return NewPool(testPoolConfig(), testRegistry(), nil, &mockPressure{level: PressureNone}, testLogger(), sink, nil)
}

// fakeWorker returns a Worker stub with just enough fields populated for the
// lifecycle hooks to exercise. We bypass Start() because those tests would
// need a real LS binary.
func fakeWorker(id, lang string) *Worker {
	w := NewWorker(id, lang, "/tmp/test-"+lang, "true", nil, testLogger(), nil)
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

			wEvents, eEvents, _, _, _ := sink.snapshot()
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

	wEvents, eEvents, _, _, _ := sink.snapshot()
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
	_, _, states, _, _ := sink.snapshot()
	if assert.Len(t, states, 1) {
		assert.Equal(t, "go", states[0].lang)
		assert.Equal(t, CircuitClosed, states[0].state)
	}

	// Failure -> open.
	cb.RecordFailure()
	_, _, states, _, _ = sink.snapshot()
	assert.Equal(t, CircuitOpen, states[len(states)-1].state)

	// Wait out backoff, probe -> half-open.
	time.Sleep(15 * time.Millisecond)
	assert.True(t, cb.CanAttempt())
	_, _, states, _, _ = sink.snapshot()
	assert.Equal(t, CircuitHalfOpen, states[len(states)-1].state)

	// Success -> closed.
	cb.RecordSuccess()
	_, _, states, _, _ = sink.snapshot()
	assert.Equal(t, CircuitClosed, states[len(states)-1].state)
}

func TestCircuit_nilSinkReplacedWithNoop(t *testing.T) {
	// Should not panic despite nil sink argument.
	cb := NewCircuitBreaker("go", time.Second, 3, nil)
	cb.RecordFailure()
	cb.RecordSuccess()
	assert.True(t, cb.CanAttempt())
}

// --- Pool construction safety --------------------------------------------

func TestPool_nilMetricsDefaultsToNoop(t *testing.T) {
	// Passing a nil sink must not panic; the Pool should default to NoopSink
	// internally so later hook calls are safe.
	p := NewPool(testPoolConfig(), testRegistry(), nil, &mockPressure{level: PressureNone}, testLogger(), nil, nil)
	assert.NotNil(t, p.metrics)

	// Exercise an eviction path with a fake worker to prove no panic.
	w := fakeWorker("w-go-1", "go")
	p.mu.Lock()
	p.workers[w.ID()] = w
	p.evictWorkerLocked(w.ID(), w, EvictIdle)
	p.mu.Unlock()
}

// --- AcquireLease lookup emission (Phase 53 D-02) ------------------------

// TestPool_AcquireLease_LookupEmission pins the canonical hit/miss boundary
// per D-02:
//   - workerForKeyLocked share-path returns non-nil  → LookupHit
//   - any spawn-path execution (success, refusal, dirty bypass) → LookupMiss
//
// Dirty acquires bypass the share branch by design and therefore always emit
// miss, even when a warm worker exists for the key.
func TestPool_AcquireLease_LookupEmission(t *testing.T) {
	t.Run("share_path_emits_hit", func(t *testing.T) {
		sink := &recordingSink{}
		p := newTestPoolWithSink(t, sink)

		// Pre-warm a Ready worker matching the workspace key so
		// workerForKeyLocked finds it. The workspace key's RepoRoot must
		// equal Worker.WorkDir() (see workerForKeyLocked predicate); the
		// shared `fakeWorker` helper hard-codes "/tmp/test-<lang>", so we
		// build the worker directly with the test key's RepoRoot here.
		key := testKey()
		w := NewWorker("w-go-1", key.Language, key.RepoRoot, "true", nil, testLogger(), nil)
		w.state.Store(int32(WorkerReady))
		p.mu.Lock()
		p.workers[w.ID()] = w
		p.mu.Unlock()

		lease, err := p.AcquireLease(context.Background(), "sess-1", key, false /* dirty */)
		require.NoError(t, err)
		require.NotNil(t, lease)
		assert.False(t, lease.Dirty)

		_, _, _, _, lookups := sink.snapshot()
		if assert.Len(t, lookups, 1, "expected exactly one lookup event on the share path") {
			assert.Equal(t, key.Language, lookups[0].lang)
			assert.Equal(t, LookupHit, lookups[0].result)
		}
	})

	t.Run("spawn_path_emits_miss", func(t *testing.T) {
		sink := &recordingSink{}
		// MaxWorkers=0 forces ErrMaxWorkersReached AFTER the miss emit (the
		// miss is recorded BEFORE the circuit/max-workers gate per D-02), so
		// we observe the emission without spawning a real LS process.
		cfg := testPoolConfig()
		cfg.MaxWorkers = 0
		p := NewPool(cfg, testRegistry(), nil, &mockPressure{level: PressureNone}, testLogger(), sink, nil)

		key := testKey()
		lease, err := p.AcquireLease(context.Background(), "sess-2", key, false /* dirty */)
		require.Error(t, err, "expected refusal at MaxWorkers=0")
		assert.Nil(t, lease)

		_, _, _, _, lookups := sink.snapshot()
		if assert.Len(t, lookups, 1, "expected exactly one lookup event on the spawn path (miss)") {
			assert.Equal(t, key.Language, lookups[0].lang)
			assert.Equal(t, LookupMiss, lookups[0].result)
		}
	})

	t.Run("dirty_path_emits_miss", func(t *testing.T) {
		sink := &recordingSink{}
		cfg := testPoolConfig()
		cfg.MaxWorkers = 0
		p := NewPool(cfg, testRegistry(), nil, &mockPressure{level: PressureNone}, testLogger(), sink, nil)

		// Pre-warm a Ready worker that WOULD satisfy the share path if not for
		// dirty=true. Dirty bypasses workerForKeyLocked by design (D-02).
		// Build the worker directly so its WorkDir matches the test key's
		// RepoRoot — same reason as the share-path sub-test above.
		key := testKey()
		w := NewWorker("w-go-1", key.Language, key.RepoRoot, "true", nil, testLogger(), nil)
		w.state.Store(int32(WorkerReady))
		p.mu.Lock()
		p.workers[w.ID()] = w
		p.mu.Unlock()

		lease, err := p.AcquireLease(context.Background(), "sess-3", key, true /* dirty */)
		require.Error(t, err, "expected refusal at MaxWorkers=0 on dirty acquire")
		assert.Nil(t, lease)

		_, _, _, _, lookups := sink.snapshot()
		if assert.Len(t, lookups, 1, "dirty acquire should emit one miss (cache bypassed)") {
			assert.Equal(t, key.Language, lookups[0].lang)
			assert.Equal(t, LookupMiss, lookups[0].result)
		}
	})
}

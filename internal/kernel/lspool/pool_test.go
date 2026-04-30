package lspool

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/agenthands/helix/internal/langregistry"
	"github.com/agenthands/helix/internal/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockPressure implements MemoryPressure for testing.
type mockPressure struct {
	level PressureLevel
	rss   map[int]uint64
}

func (m *mockPressure) Level() PressureLevel { return m.level }
func (m *mockPressure) WorkerRSS(pid int) (uint64, error) {
	if v, ok := m.rss[pid]; ok {
		return v, nil
	}
	return 0, nil
}

// testKey returns a workspace key for testing.
func testKey() workspace.WorkspaceKey {
	return workspace.WorkspaceKey{
		RepoRoot: "/tmp/test-project",
		Language: "go",
	}
}

// testPoolConfig returns a pool config for testing with short timeouts.
func testPoolConfig() PoolConfig {
	return PoolConfig{
		BaseTTL:               1,
		CeilingTTL:            10,
		MaxWorkers:            5,
		RSSHardCapMB:          2048,
		PressureCheckInterval: 1,
	}
}

// testRegistry returns a language registry for pool tests.
func testRegistry() *langregistry.Registry {
	reg, _ := langregistry.NewRegistry()
	return reg
}

func TestCircuitBreaker_RecordFailure_JitteredBackoff(t *testing.T) {
	cb := NewCircuitBreaker("go", 1*time.Minute, 10, NoopSink{})

	// Initially can attempt.
	assert.True(t, cb.CanAttempt())

	// Record first failure: backoff in [1s, 3s] (decorrelated jitter).
	cb.RecordFailure()
	assert.Equal(t, 1, cb.Failures())
	b := cb.BackoffDuration()
	assert.GreaterOrEqual(t, b, 1*time.Second)
	assert.LessOrEqual(t, b, 3*time.Second)

	// Should not be able to attempt immediately.
	assert.False(t, cb.CanAttempt())

	// Record second failure: backoff in [1s, prevSleep*3] capped at maxBackoff.
	cb.RecordFailure()
	assert.Equal(t, 2, cb.Failures())
	b2 := cb.BackoffDuration()
	assert.GreaterOrEqual(t, b2, 1*time.Second)
	assert.LessOrEqual(t, b2, 1*time.Minute)

	// Record third failure.
	cb.RecordFailure()
	assert.Equal(t, 3, cb.Failures())
	b3 := cb.BackoffDuration()
	assert.GreaterOrEqual(t, b3, 1*time.Second)
	assert.LessOrEqual(t, b3, 1*time.Minute)
}

func TestCircuitBreaker_RecordSuccess_ResetsBackoff(t *testing.T) {
	cb := NewCircuitBreaker("go", 1*time.Minute, 3, NoopSink{})

	cb.RecordFailure()
	cb.RecordFailure()
	assert.Equal(t, 2, cb.Failures())

	cb.RecordSuccess()
	assert.Equal(t, 0, cb.Failures())
	assert.Equal(t, time.Duration(0), cb.BackoffDuration())
	assert.True(t, cb.CanAttempt())
}

func TestCircuitBreaker_RetriesAfterBackoff(t *testing.T) {
	// With a budget higher than failures, circuit retries after backoff.
	cb := NewCircuitBreaker("go", 10*time.Millisecond, 100, NoopSink{})

	// Record a couple failures (within budget).
	cb.RecordFailure()
	cb.RecordFailure()

	// Backoff should be capped at maxBackoff.
	assert.LessOrEqual(t, cb.BackoffDuration(), 10*time.Millisecond)

	// Wait for backoff to expire.
	time.Sleep(50 * time.Millisecond)

	// Should be able to attempt again (single probe).
	assert.True(t, cb.CanAttempt())
}

func TestWorkerMetrics_TTL_Adaptive(t *testing.T) {
	m := &WorkerMetrics{
		StartedAt:  time.Now(),
		LastUsedAt: time.Now(),
	}

	// With zero score, TTL should be base.
	ttl := m.TTL(300, 3600)
	assert.Equal(t, 300, ttl)

	// After reuse, score increases, TTL increases.
	m.OnReuse()
	m.OnReuse()
	m.OnReuse()
	ttl = m.TTL(300, 3600)
	assert.Greater(t, ttl, 300)

	// TTL should not exceed ceiling.
	for i := 0; i < 100; i++ {
		m.OnReuse()
	}
	ttl = m.TTL(300, 3600)
	assert.LessOrEqual(t, ttl, 3600)
}

func TestWorkerMetrics_TTL_ZeroMeansNoTimeout(t *testing.T) {
	// Per D-07: TTL=0 means no idle timeout.
	m := &WorkerMetrics{
		StartedAt:  time.Now(),
		LastUsedAt: time.Now(),
	}
	ttl := m.TTL(0, 3600)
	assert.Equal(t, 0, ttl)
}

func TestLease_IsMutation(t *testing.T) {
	assert.True(t, IsMutation("textDocument/didChange"))
	assert.True(t, IsMutation("textDocument/didOpen"))
	assert.True(t, IsMutation("textDocument/didClose"))
	assert.True(t, IsMutation("textDocument/didSave"))
	assert.True(t, IsMutation("textDocument/rename"))

	assert.False(t, IsMutation("textDocument/definition"))
	assert.False(t, IsMutation("textDocument/references"))
	assert.False(t, IsMutation("textDocument/hover"))
	assert.False(t, IsMutation("textDocument/documentSymbol"))
}

func TestPressureLevel_String(t *testing.T) {
	assert.Equal(t, "none", PressureNone.String())
	assert.Equal(t, "low", PressureLow.String())
	assert.Equal(t, "medium", PressureMedium.String())
	assert.Equal(t, "high", PressureHigh.String())
	assert.Equal(t, "critical", PressureCritical.String())
}

func TestWorkerState_String(t *testing.T) {
	assert.Equal(t, "starting", WorkerStarting.String())
	assert.Equal(t, "initializing", WorkerInitializing.String())
	assert.Equal(t, "ready", WorkerReady.String())
	assert.Equal(t, "shutting_down", WorkerShuttingDown.String())
	assert.Equal(t, "stopped", WorkerStopped.String())
}

func TestQuirks_RegistryLanguages(t *testing.T) {
	// Verify that common languages are available via the registry.
	reg := testRegistry()
	languages := []string{"go", "python", "typescript", "rust"}
	for _, lang := range languages {
		entry, ok := reg.Get(lang)
		assert.True(t, ok, "missing registry entry for %s", lang)
		assert.NotEmpty(t, entry.Command, "empty command for %s", lang)

		// Verify GetQuirkAdapter returns a valid adapter.
		adapter := GetQuirkAdapter(entry)
		assert.NotNil(t, adapter, "nil adapter for %s", lang)
	}
}

func TestPoolConfig_Defaults(t *testing.T) {
	cfg := DefaultPoolConfig()
	assert.Equal(t, 300, cfg.BaseTTL)
	assert.Equal(t, 3600, cfg.CeilingTTL)
	assert.Equal(t, 10, cfg.MaxWorkers)
	assert.Equal(t, 2048, cfg.RSSHardCapMB)
	assert.Equal(t, 10, cfg.PressureCheckInterval)
}

func TestPool_NewPool(t *testing.T) {
	pressure := &mockPressure{level: PressureNone}
	cfg := testPoolConfig()
	pool := NewPool(cfg, testRegistry(), nil, pressure, testLogger(), NoopSink{})
	require.NotNil(t, pool)
	assert.Equal(t, 0, pool.WorkerCount())
	assert.Equal(t, 0, pool.LeaseCount())
}

func TestLease_ConcurrencyControl(t *testing.T) {
	// Test that mutation serialization works via RWMutex.
	// We can't test with real workers, but we can verify the IsMutation logic.
	mutations := []string{
		"textDocument/didChange",
		"textDocument/didOpen",
		"textDocument/didClose",
		"textDocument/didSave",
		"textDocument/rename",
	}
	reads := []string{
		"textDocument/definition",
		"textDocument/references",
		"textDocument/hover",
		"textDocument/documentSymbol",
		"workspace/symbol",
	}

	for _, m := range mutations {
		assert.True(t, IsMutation(m), "expected %s to be a mutation", m)
	}
	for _, r := range reads {
		assert.False(t, IsMutation(r), "expected %s to be a read", r)
	}
}

func TestCircuitBreaker_Concurrent(t *testing.T) {
	cb := NewCircuitBreaker("go", 1*time.Minute, 3, NoopSink{})
	var wg sync.WaitGroup

	// Concurrent failures and success checks.
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cb.RecordFailure()
			_ = cb.CanAttempt()
			_ = cb.BackoffDuration()
		}()
	}
	wg.Wait()

	cb.RecordSuccess()
	assert.True(t, cb.CanAttempt())
}

func TestWorkerMetrics_ConcurrentReuse(t *testing.T) {
	m := &WorkerMetrics{
		StartedAt:  time.Now(),
		LastUsedAt: time.Now(),
	}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.OnReuse()
		}()
	}
	wg.Wait()

	m.mu.Lock()
	assert.Equal(t, int64(100), m.UseCount)
	m.mu.Unlock()
}

func TestPool_RunAndShutdown(t *testing.T) {
	pressure := &mockPressure{level: PressureNone}
	cfg := testPoolConfig()
	pool := NewPool(cfg, testRegistry(), nil, pressure, testLogger(), NoopSink{})

	ctx, cancel := context.WithCancel(context.Background())

	// Run in background.
	done := make(chan error, 1)
	go func() {
		done <- pool.Run(ctx)
	}()

	// Cancel immediately.
	cancel()

	err := <-done
	assert.ErrorIs(t, err, context.Canceled)
}

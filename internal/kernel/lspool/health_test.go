package lspool

import (
	"testing"
	"time"

	gen "github.com/postfix/serena/protocol/gen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthSnapshot_EmptyPool(t *testing.T) {
	pressure := &mockPressure{level: PressureNone}
	pool := NewPool(testPoolConfig(), testRegistry(), nil, pressure, testLogger(), NoopSink{})

	report := pool.HealthSnapshot()
	assert.Empty(t, report.Workspaces)
	assert.Equal(t, "", report.Summary)
	assert.Equal(t, 0, report.TotalWorkers())
}

func TestHealthSnapshot_HealthyWorker(t *testing.T) {
	pressure := &mockPressure{level: PressureNone}
	pool := NewPool(testPoolConfig(), testRegistry(), nil, pressure, testLogger(), NoopSink{})

	// Manually add a worker in Ready state with a closed circuit.
	w := NewWorker("w-go-1", "go", "/tmp/test-project", "gopls", nil, testLogger())
	w.state.Store(int32(WorkerReady))
	pool.mu.Lock()
	pool.workers["w-go-1"] = w
	pool.circuits["go"] = NewCircuitBreaker("go", 5*time.Minute, 3, NoopSink{})
	pool.mu.Unlock()

	report := pool.HealthSnapshot()
	require.Len(t, report.Workspaces, 1)
	require.Len(t, report.Workspaces[0].Workers, 1)

	wh := report.Workspaces[0].Workers[0]
	assert.Equal(t, "w-go-1", wh.ID)
	assert.Equal(t, "go", wh.Language)
	assert.Equal(t, "/tmp/test-project", wh.WorkDir)
	assert.Equal(t, "gopls", wh.Command)
	assert.Equal(t, "healthy", wh.State)
	assert.False(t, wh.Indexing)
}

func TestHealthSnapshot_DegradedWorker(t *testing.T) {
	pressure := &mockPressure{level: PressureNone}
	pool := NewPool(testPoolConfig(), testRegistry(), nil, pressure, testLogger(), NoopSink{})

	w := NewWorker("w-go-1", "go", "/tmp/test-project", "gopls", nil, testLogger())
	w.state.Store(int32(WorkerReady))

	cb := NewCircuitBreaker("go", 5*time.Minute, 10, NoopSink{})
	// Record a failure and wait for backoff to enable half-open probe.
	cb.RecordFailure()
	// Force half-open state directly for deterministic test.
	cb.mu.Lock()
	cb.state = CircuitHalfOpen
	cb.mu.Unlock()

	pool.mu.Lock()
	pool.workers["w-go-1"] = w
	pool.circuits["go"] = cb
	pool.mu.Unlock()

	report := pool.HealthSnapshot()
	require.Len(t, report.Workspaces, 1)
	require.Len(t, report.Workspaces[0].Workers, 1)
	assert.Equal(t, "degraded", report.Workspaces[0].Workers[0].State)
}

func TestHealthSnapshot_FailedCircuit(t *testing.T) {
	pressure := &mockPressure{level: PressureNone}
	pool := NewPool(testPoolConfig(), testRegistry(), nil, pressure, testLogger(), NoopSink{})

	w := NewWorker("w-go-1", "go", "/tmp/test-project", "gopls", nil, testLogger())
	w.state.Store(int32(WorkerReady))

	cb := NewCircuitBreaker("go", 5*time.Minute, 3, NoopSink{})
	// Force circuit to open state.
	cb.mu.Lock()
	cb.state = CircuitOpen
	cb.failures = 3
	cb.mu.Unlock()

	pool.mu.Lock()
	pool.workers["w-go-1"] = w
	pool.circuits["go"] = cb
	pool.mu.Unlock()

	report := pool.HealthSnapshot()
	require.Len(t, report.Workspaces, 1)
	require.Len(t, report.Workspaces[0].Workers, 1)
	assert.Equal(t, "failed", report.Workspaces[0].Workers[0].State)

	// Circuit should also be reported.
	require.Len(t, report.Workspaces[0].Circuits, 1)
	assert.Equal(t, "open", report.Workspaces[0].Circuits[0].State)
	assert.Equal(t, 3, report.Workspaces[0].Circuits[0].Failures)
}

func TestHealthSnapshot_IndexingWorker(t *testing.T) {
	pressure := &mockPressure{level: PressureNone}
	pool := NewPool(testPoolConfig(), testRegistry(), nil, pressure, testLogger(), NoopSink{})

	w := NewWorker("w-go-1", "go", "/tmp/test-project", "gopls", nil, testLogger())
	w.state.Store(int32(WorkerInitializing))

	pool.mu.Lock()
	pool.workers["w-go-1"] = w
	pool.mu.Unlock()

	report := pool.HealthSnapshot()
	require.Len(t, report.Workspaces, 1)
	require.Len(t, report.Workspaces[0].Workers, 1)

	wh := report.Workspaces[0].Workers[0]
	assert.Equal(t, "healthy (indexing)", wh.State)
	assert.True(t, wh.Indexing)
}

func TestHealthSnapshot_StoppedWorker(t *testing.T) {
	pressure := &mockPressure{level: PressureNone}
	pool := NewPool(testPoolConfig(), testRegistry(), nil, pressure, testLogger(), NoopSink{})

	w := NewWorker("w-go-1", "go", "/tmp/test-project", "gopls", nil, testLogger())
	w.state.Store(int32(WorkerStopped))

	pool.mu.Lock()
	pool.workers["w-go-1"] = w
	pool.mu.Unlock()

	report := pool.HealthSnapshot()
	require.Len(t, report.Workspaces, 1)
	require.Len(t, report.Workspaces[0].Workers, 1)
	assert.Equal(t, "failed", report.Workspaces[0].Workers[0].State)
}

func TestCapabilitiesFromServer(t *testing.T) {
	trueVal := true
	hoverOpt := gen.Or_Boolean_HoverOptions{Value: &trueVal}
	defOpt := gen.Or_Boolean_DefinitionOptions{Value: &trueVal}
	refOpt := gen.Or_Boolean_ReferenceOptions{Value: &trueVal}

	caps := gen.ServerCapabilities{
		HoverProvider:      &hoverOpt,
		DefinitionProvider: &defOpt,
		ReferencesProvider: &refOpt,
	}

	result := capabilitiesFromServer(caps)
	assert.Contains(t, result, "hover")
	assert.Contains(t, result, "definition")
	assert.Contains(t, result, "references")
	assert.Len(t, result, 3)
}

func TestCapabilitiesFromServer_Empty(t *testing.T) {
	caps := gen.ServerCapabilities{}
	result := capabilitiesFromServer(caps)
	assert.Empty(t, result)
}

func TestCircuitStateString(t *testing.T) {
	assert.Equal(t, "closed", circuitStateString(CircuitClosed))
	assert.Equal(t, "half_open", circuitStateString(CircuitHalfOpen))
	assert.Equal(t, "open", circuitStateString(CircuitOpen))
	assert.Equal(t, "unknown", circuitStateString(99))
}

func TestWorker_Command(t *testing.T) {
	w := NewWorker("w-1", "go", "/tmp", "gopls", []string{"-remote=auto"}, testLogger())
	assert.Equal(t, "gopls", w.Command())
}

func TestCircuitBreaker_State(t *testing.T) {
	cb := NewCircuitBreaker("go", 5*time.Minute, 3, NoopSink{})
	assert.Equal(t, CircuitClosed, cb.State())

	cb.RecordFailure()
	assert.Equal(t, CircuitOpen, cb.State())

	cb.RecordSuccess()
	assert.Equal(t, CircuitClosed, cb.State())
}

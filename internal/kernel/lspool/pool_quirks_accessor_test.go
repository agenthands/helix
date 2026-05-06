package lspool

import (
	"testing"

	"github.com/agenthands/helix/internal/workspace"
)

// ACC1: JdtlsAdapter accessor returns nil when no Java worker is registered
// for the workspace.  Pool starts empty; the call must not panic and must
// return nil.
func TestPool_JdtlsAdapter_NilWhenNoJavaWorker(t *testing.T) {
	pool := NewPool(testPoolConfig(), testRegistry(), nil, &mockPressure{}, testLogger(), NoopSink{}, nil)

	wsKey := workspace.WorkspaceKey{RepoRoot: "/tmp/no-java", Language: "java"}
	got := pool.JdtlsAdapter(wsKey)
	if got != nil {
		t.Errorf("JdtlsAdapter on empty pool returned %v, want nil", got)
	}
}

// ACC2: RustAnalyzerAdapter accessor returns nil when no Rust worker is
// registered for the workspace.
func TestPool_RustAnalyzerAdapter_NilWhenNoRustWorker(t *testing.T) {
	pool := NewPool(testPoolConfig(), testRegistry(), nil, &mockPressure{}, testLogger(), NoopSink{}, nil)

	wsKey := workspace.WorkspaceKey{RepoRoot: "/tmp/no-rust", Language: "rust"}
	got := pool.RustAnalyzerAdapter(wsKey)
	if got != nil {
		t.Errorf("RustAnalyzerAdapter on empty pool returned %v, want nil", got)
	}
}

// ACC3: When a worker exists but its quirks adapter is the wrong type, the
// accessor returns nil rather than mis-cast.  Direct construction of the
// in-memory pool worker map exercises the type-assertion guard.
func TestPool_JdtlsAdapter_NilWhenQuirksWrongType(t *testing.T) {
	pool := NewPool(testPoolConfig(), testRegistry(), nil, &mockPressure{}, testLogger(), NoopSink{}, nil)

	// Inject a worker that has no quirks adapter (or a non-Jdtls one).
	wsKey := workspace.WorkspaceKey{RepoRoot: "/tmp/wrong-quirks", Language: "java"}
	w := &Worker{
		id:       "w-fake-1",
		language: "java",
		workDir:  wsKey.RepoRoot,
	}
	w.state.Store(int32(WorkerReady))
	pool.mu.Lock()
	pool.workers["w-fake-1"] = w
	pool.mu.Unlock()

	got := pool.JdtlsAdapter(wsKey)
	if got != nil {
		t.Errorf("JdtlsAdapter with non-Jdtls quirks returned %v, want nil", got)
	}
}

// ACC4: When the Java worker exists and its quirks IS *JdtlsAdapter, the
// accessor returns that adapter.
func TestPool_JdtlsAdapter_ReturnsAdapterWhenPresent(t *testing.T) {
	pool := NewPool(testPoolConfig(), testRegistry(), nil, &mockPressure{}, testLogger(), NoopSink{}, nil)

	wsKey := workspace.WorkspaceKey{RepoRoot: "/tmp/java-ws", Language: "java"}
	wantAdapter := &JdtlsAdapter{}
	w := &Worker{
		id:       "w-java-1",
		language: "java",
		workDir:  wsKey.RepoRoot,
		quirks:   wantAdapter,
	}
	w.state.Store(int32(WorkerReady))
	pool.mu.Lock()
	pool.workers["w-java-1"] = w
	pool.mu.Unlock()

	got := pool.JdtlsAdapter(wsKey)
	if got != wantAdapter {
		t.Errorf("JdtlsAdapter returned %v, want %v", got, wantAdapter)
	}
}

// ACC5: Same as ACC4 but for RustAnalyzerAdapter.
func TestPool_RustAnalyzerAdapter_ReturnsAdapterWhenPresent(t *testing.T) {
	pool := NewPool(testPoolConfig(), testRegistry(), nil, &mockPressure{}, testLogger(), NoopSink{}, nil)

	wsKey := workspace.WorkspaceKey{RepoRoot: "/tmp/rust-ws", Language: "rust"}
	wantAdapter := &RustAnalyzerAdapter{}
	w := &Worker{
		id:       "w-rust-1",
		language: "rust",
		workDir:  wsKey.RepoRoot,
		quirks:   wantAdapter,
	}
	w.state.Store(int32(WorkerReady))
	pool.mu.Lock()
	pool.workers["w-rust-1"] = w
	pool.mu.Unlock()

	got := pool.RustAnalyzerAdapter(wsKey)
	if got != wantAdapter {
		t.Errorf("RustAnalyzerAdapter returned %v, want %v", got, wantAdapter)
	}
}

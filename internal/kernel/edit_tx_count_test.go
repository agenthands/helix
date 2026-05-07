// Phase 63 P63-02 Task 1: ActiveEditTxCount + BeginEditTx tests.

package kernel_test

import (
	"sync"
	"testing"

	"github.com/agenthands/helix/internal/kernel"
	"github.com/agenthands/helix/internal/workspace"
)

// TestKernel_ActiveEditTxCount_IncDec: BeginEditTx increments, release
// decrements. Idempotent under double-release.
func TestKernel_ActiveEditTxCount_IncDec(t *testing.T) {
	// Use a unique repo root per test so the global registry doesn't
	// leak state across the suite.
	const repo = "/tmp/edit-tx-count-inc-dec-repo"
	k := &kernel.Kernel{}
	ws := workspace.WorkspaceKey{RepoRoot: repo}

	if got := k.ActiveEditTxCount(ws); got != 0 {
		t.Errorf("baseline count: got %d, want 0", got)
	}

	release := k.BeginEditTx(ws)
	if got := k.ActiveEditTxCount(ws); got != 1 {
		t.Errorf("count after Begin: got %d, want 1", got)
	}

	// Concurrent stamp.
	release2 := k.BeginEditTx(ws)
	if got := k.ActiveEditTxCount(ws); got != 2 {
		t.Errorf("count after second Begin: got %d, want 2", got)
	}

	release()
	if got := k.ActiveEditTxCount(ws); got != 1 {
		t.Errorf("count after first release: got %d, want 1", got)
	}

	// Double-release on the same closure must be idempotent.
	release()
	if got := k.ActiveEditTxCount(ws); got != 1 {
		t.Errorf("count after double-release: got %d, want 1 (idempotent)", got)
	}

	release2()
	if got := k.ActiveEditTxCount(ws); got != 0 {
		t.Errorf("count after final release: got %d, want 0", got)
	}
}

// TestKernel_ActiveEditTxCount_NilSafe: a nil kernel returns 0 and
// BeginEditTx returns a no-op release closure.
func TestKernel_ActiveEditTxCount_NilSafe(t *testing.T) {
	var k *kernel.Kernel
	ws := workspace.WorkspaceKey{RepoRoot: "/tmp/x"}
	if got := k.ActiveEditTxCount(ws); got != 0 {
		t.Errorf("nil kernel count: got %d, want 0", got)
	}
	release := k.BeginEditTx(ws)
	release() // must not panic
}

// TestKernel_ActiveEditTxCount_Concurrent: concurrent Begin / release
// pairs preserve the invariant count == 0 at quiesce.
func TestKernel_ActiveEditTxCount_Concurrent(t *testing.T) {
	const repo = "/tmp/edit-tx-count-concurrent-repo"
	k := &kernel.Kernel{}
	ws := workspace.WorkspaceKey{RepoRoot: repo}

	const N = 100
	var wg sync.WaitGroup
	wg.Add(N)
	for i := 0; i < N; i++ {
		go func() {
			defer wg.Done()
			release := k.BeginEditTx(ws)
			defer release()
			// no-op body
		}()
	}
	wg.Wait()
	if got := k.ActiveEditTxCount(ws); got != 0 {
		t.Errorf("count after N goroutines: got %d, want 0", got)
	}
}

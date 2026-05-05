package lspool

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/agenthands/helix/internal/workspace"
)

// stampForegroundLeaseForTesting is a test helper mirroring the side-effect
// AcquireLease performs on a non-enrichment session: it stamps
// lastForegroundLease[wsKey] = time.Now() iff sessionID lacks the
// "lsp-enrichment:" prefix. Phase 61 D-04.
//
// Living on the same package as Pool gives the tests a deterministic seam
// without spinning up a real LS worker.
func stampForegroundLeaseForTesting(p *Pool, sessionID string, wsKey workspace.WorkspaceKey) {
	if strings.HasPrefix(sessionID, "lsp-enrichment:") {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.lastForegroundLease == nil {
		p.lastForegroundLease = make(map[workspace.WorkspaceKey]time.Time)
	}
	p.lastForegroundLease[wsKey] = time.Now()
}

// F1: New pool — ForegroundBusy returns false (no lease ever stamped).
func TestPool_ForegroundBusy_EmptyReturnsFalse(t *testing.T) {
	pool := NewPool(testPoolConfig(), testRegistry(), nil, &mockPressure{}, testLogger(), NoopSink{}, nil)
	if pool.ForegroundBusy(testKey()) {
		t.Fatal("ForegroundBusy on empty pool returned true; want false")
	}
}

// F2: Foreground lease stamp → ForegroundBusy true within the window.
func TestPool_ForegroundBusy_RegistersWithinWindow(t *testing.T) {
	pool := NewPool(testPoolConfig(), testRegistry(), nil, &mockPressure{}, testLogger(), NoopSink{}, nil)
	pool.SetYieldCheckWindow(200 * time.Millisecond)
	stampForegroundLeaseForTesting(pool, "foreground-tool-call", testKey())
	if !pool.ForegroundBusy(testKey()) {
		t.Fatal("ForegroundBusy: got false immediately after stamp; want true")
	}
}

// F3: Enrichment session ID does NOT register as foreground.
func TestPool_ForegroundBusy_EnrichmentSessionIgnored(t *testing.T) {
	pool := NewPool(testPoolConfig(), testRegistry(), nil, &mockPressure{}, testLogger(), NoopSink{}, nil)
	pool.SetYieldCheckWindow(200 * time.Millisecond)
	stampForegroundLeaseForTesting(pool, "lsp-enrichment:r1:go", testKey())
	if pool.ForegroundBusy(testKey()) {
		t.Fatal("ForegroundBusy after enrichment-prefixed stamp: got true; want false")
	}
}

// F4: After window elapses, ForegroundBusy returns false.
func TestPool_ForegroundBusy_ExpiresAfterWindow(t *testing.T) {
	pool := NewPool(testPoolConfig(), testRegistry(), nil, &mockPressure{}, testLogger(), NoopSink{}, nil)
	pool.SetYieldCheckWindow(50 * time.Millisecond)
	stampForegroundLeaseForTesting(pool, "foreground-tool-call", testKey())
	time.Sleep(120 * time.Millisecond) // window + safety margin
	if pool.ForegroundBusy(testKey()) {
		t.Fatal("ForegroundBusy after window+margin: got true; want false")
	}
}

// F5: Per-wsKey isolation — stamping A does not make B busy.
func TestPool_ForegroundBusy_PerWsKey(t *testing.T) {
	pool := NewPool(testPoolConfig(), testRegistry(), nil, &mockPressure{}, testLogger(), NoopSink{}, nil)
	pool.SetYieldCheckWindow(200 * time.Millisecond)
	wsA := workspace.WorkspaceKey{RepoRoot: "/a", Language: "go"}
	wsB := workspace.WorkspaceKey{RepoRoot: "/b", Language: "go"}
	stampForegroundLeaseForTesting(pool, "foreground-tool-call", wsA)
	if !pool.ForegroundBusy(wsA) {
		t.Fatal("wsA after stamp: got false; want true")
	}
	if pool.ForegroundBusy(wsB) {
		t.Fatal("wsB without stamp: got true; want false (per-wsKey isolation)")
	}
}

// SY1: Default zero-value pool — ForegroundBusy uses 200ms window.
//
// We assert by stamping then querying inside a sub-window (busy=true),
// and after a super-window sleep (busy=false). Real clock; tolerances
// chosen to avoid CI flakiness.
func TestPool_SetYieldCheckWindow_DefaultIs200ms(t *testing.T) {
	pool := NewPool(testPoolConfig(), testRegistry(), nil, &mockPressure{}, testLogger(), NoopSink{}, nil)
	// Do NOT call SetYieldCheckWindow — exercise the default.
	stampForegroundLeaseForTesting(pool, "foreground-tool-call", testKey())
	time.Sleep(50 * time.Millisecond)
	if !pool.ForegroundBusy(testKey()) {
		t.Fatal("default window: busy after 50ms returned false; expected true (default 200ms)")
	}
	time.Sleep(250 * time.Millisecond) // total ~300ms > 200ms default
	if pool.ForegroundBusy(testKey()) {
		t.Fatal("default window: busy after 300ms returned true; expected false (default 200ms expired)")
	}
}

// SY2: SetYieldCheckWindow(500ms) extends the busy window.
func TestPool_SetYieldCheckWindow_OverrideExpands(t *testing.T) {
	pool := NewPool(testPoolConfig(), testRegistry(), nil, &mockPressure{}, testLogger(), NoopSink{}, nil)
	pool.SetYieldCheckWindow(500 * time.Millisecond)
	stampForegroundLeaseForTesting(pool, "foreground-tool-call", testKey())
	time.Sleep(250 * time.Millisecond)
	if !pool.ForegroundBusy(testKey()) {
		t.Fatal("500ms override: busy after 250ms returned false; expected true (window not yet expired)")
	}
}

// SY3: Concurrent SetYieldCheckWindow + ForegroundBusy is race-clean
// (verified by `go test -race`).
func TestPool_SetYieldCheckWindow_ConcurrentSafe(t *testing.T) {
	pool := NewPool(testPoolConfig(), testRegistry(), nil, &mockPressure{}, testLogger(), NoopSink{}, nil)
	pool.SetYieldCheckWindow(100 * time.Millisecond)
	stampForegroundLeaseForTesting(pool, "foreground-tool-call", testKey())

	const goroutines = 8
	const iterations = 500
	var wg sync.WaitGroup
	stop := make(chan struct{})

	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(gid int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				select {
				case <-stop:
					return
				default:
				}
				if gid%2 == 0 {
					pool.SetYieldCheckWindow(time.Duration(50+i%200) * time.Millisecond)
				} else {
					_ = pool.ForegroundBusy(testKey())
				}
			}
		}(g)
	}

	// Let the loops run a brief moment then signal stop.
	time.Sleep(20 * time.Millisecond)
	close(stop)
	wg.Wait()
}

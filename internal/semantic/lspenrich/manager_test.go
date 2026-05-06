package lspenrich_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	serr "github.com/agenthands/helix/internal/errors"
	"github.com/agenthands/helix/internal/kernel/lspool"
	"github.com/agenthands/helix/internal/semantic"
	"github.com/agenthands/helix/internal/semantic/lspenrich"
	"github.com/agenthands/helix/internal/workspace"
)

// noopAcquirer satisfies lspenrich.LeaseAcquirer with all-zero behavior.
// Used by status tests that never invoke Manager.Run.
type noopAcquirer struct{}

func (noopAcquirer) AcquireLease(_ context.Context, _ string, _ workspace.WorkspaceKey, _ bool) (*lspool.WorkerLease, error) {
	return nil, errors.New("noopAcquirer.AcquireLease should not be called")
}
func (noopAcquirer) ForegroundBusy(_ workspace.WorkspaceKey) bool { return false }

// recordingAcquirer counts AcquireLease calls per (sessionID, wsKey, lang).
// Returns a fresh placeholder *lspool.WorkerLease (with the supplied
// sessionID) on success, or the configured error.  Tracks ReleaseLease
// invocations via a release counter on each placeholder lease via a
// side-table, since the production Manager calls Pool.ReleaseLease(sessionID).
type recordingAcquirer struct {
	mu sync.Mutex

	// total AcquireLease calls.
	calls int

	// Per-key acquire counts.
	perKey map[string]int

	// Captured sessionIDs ordered.
	sessions []string

	// Captured wsKeys ordered (parallel to sessions).
	wsKeys []workspace.WorkspaceKey

	// Configured per-key error: returned once when set, then cleared.  Used by
	// the ErrCircuitOpen test to fail the first call and succeed afterwards.
	errOnce map[string]error

	// Release tracking — maps sessionID → release count.  Manager.releaseAll
	// + OnWorkspaceDeactivate invoke Manager.releaseSession (a side-table on
	// Manager); the recordingAcquirer hands out leases tagged with sessionID
	// so the Manager can ask back which leases it has handed out.  The test
	// inspects releaseCount via this acquirer.
	releaseCount map[string]int
}

func newRecordingAcquirer() *recordingAcquirer {
	return &recordingAcquirer{
		perKey:       map[string]int{},
		errOnce:      map[string]error{},
		releaseCount: map[string]int{},
	}
}

func (r *recordingAcquirer) AcquireLease(_ context.Context, sessionID string, wsKey workspace.WorkspaceKey, _ bool) (*lspool.WorkerLease, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	r.perKey[sessionID]++
	r.sessions = append(r.sessions, sessionID)
	r.wsKeys = append(r.wsKeys, wsKey)
	if err, ok := r.errOnce[sessionID]; ok && err != nil {
		// Clear so the next call succeeds.
		delete(r.errOnce, sessionID)
		return nil, err
	}
	// Return a placeholder lease whose SessionID we record so the Manager
	// can reference it.
	return &lspool.WorkerLease{SessionID: sessionID}, nil
}

func (r *recordingAcquirer) ForegroundBusy(_ workspace.WorkspaceKey) bool { return false }

// ReleaseLease — production Manager calls this method.  The Manager invokes
// it via the LeaseReleaser optional interface (we wire that on the seam).
func (r *recordingAcquirer) ReleaseLease(sessionID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.releaseCount[sessionID]++
}

// =============================================================================
// M-Cache1: First AcquireFor on a (wsKey, lang) calls AcquireLease once.
// Second AcquireFor on the SAME (wsKey, lang) returns the cached lease — no
// second AcquireLease call.
// =============================================================================
func TestManager_AcquireFor_CachesPerKey_M_Cache1(t *testing.T) {
	q := lspenrich.NewLaneQueue(8, 8)
	r := newRecordingAcquirer()
	m := lspenrich.NewManager(q, r, nil, nil, semantic.LSPEnrichmentConfig{}, nil, nil)

	wsA := workspace.WorkspaceKey{RepoRoot: "/repo/a"}

	ctx := context.Background()
	l1, err := m.AcquireFor(ctx, wsA, "go")
	if err != nil {
		t.Fatalf("AcquireFor #1: err=%v", err)
	}
	l2, err := m.AcquireFor(ctx, wsA, "go")
	if err != nil {
		t.Fatalf("AcquireFor #2: err=%v", err)
	}
	if l1 != l2 {
		t.Errorf("expected same cached lease pointer; got l1=%p l2=%p", l1, l2)
	}
	if r.calls != 1 {
		t.Errorf("recordingAcquirer.calls=%d, want 1 (cache miss only on first)", r.calls)
	}
}

// =============================================================================
// M-Cache2: Per-(wsKey, lang) keying.  Three distinct keys → three distinct
// AcquireLease calls; subsequent same-key lookups hit cache.
// =============================================================================
func TestManager_AcquireFor_PerKeyKeying_M_Cache2(t *testing.T) {
	q := lspenrich.NewLaneQueue(8, 8)
	r := newRecordingAcquirer()
	m := lspenrich.NewManager(q, r, nil, nil, semantic.LSPEnrichmentConfig{}, nil, nil)

	ctx := context.Background()
	wsA := workspace.WorkspaceKey{RepoRoot: "/repo/a"}
	wsB := workspace.WorkspaceKey{RepoRoot: "/repo/b"}

	if _, err := m.AcquireFor(ctx, wsA, "go"); err != nil {
		t.Fatalf("AcquireFor wsA go: %v", err)
	}
	if _, err := m.AcquireFor(ctx, wsA, "java"); err != nil {
		t.Fatalf("AcquireFor wsA java: %v", err)
	}
	if _, err := m.AcquireFor(ctx, wsB, "go"); err != nil {
		t.Fatalf("AcquireFor wsB go: %v", err)
	}
	if r.calls != 3 {
		t.Errorf("after 3 distinct (wsKey, lang) pairs: calls=%d, want 3", r.calls)
	}

	// Same-key lookups should NOT add new calls.
	if _, err := m.AcquireFor(ctx, wsA, "go"); err != nil {
		t.Fatalf("AcquireFor wsA go (again): %v", err)
	}
	if _, err := m.AcquireFor(ctx, wsA, "java"); err != nil {
		t.Fatalf("AcquireFor wsA java (again): %v", err)
	}
	if r.calls != 3 {
		t.Errorf("after same-key lookups: calls=%d, want still 3", r.calls)
	}
}

// =============================================================================
// M-Cache3: ErrCircuitOpen on AcquireLease is NOT cached.  The next call
// retries (fresh AcquireLease invocation).
// =============================================================================
func TestManager_AcquireFor_ErrCircuitOpen_NotCached_M_Cache3(t *testing.T) {
	q := lspenrich.NewLaneQueue(8, 8)
	r := newRecordingAcquirer()

	// Pre-set: first call for sessionID lsp-enrichment:/repo/a:go fails.
	r.errOnce["lsp-enrichment:/repo/a:go"] = serr.ErrCircuitOpen

	m := lspenrich.NewManager(q, r, nil, nil, semantic.LSPEnrichmentConfig{}, nil, nil)
	wsA := workspace.WorkspaceKey{RepoRoot: "/repo/a"}
	ctx := context.Background()

	// First call: errors out.
	if _, err := m.AcquireFor(ctx, wsA, "go"); err == nil {
		t.Fatal("first AcquireFor: want ErrCircuitOpen, got nil")
	} else if !errors.Is(err, serr.ErrCircuitOpen) {
		t.Fatalf("first AcquireFor: want ErrCircuitOpen, got %v", err)
	}

	// Second call: errOnce now cleared; should retry and succeed.
	if l, err := m.AcquireFor(ctx, wsA, "go"); err != nil {
		t.Fatalf("second AcquireFor: %v", err)
	} else if l == nil {
		t.Fatal("second AcquireFor: got nil lease without error")
	}

	if r.calls != 2 {
		t.Errorf("recordingAcquirer.calls=%d, want 2 (failed handle not cached)", r.calls)
	}
}

// =============================================================================
// M-Deactivate1: OnWorkspaceDeactivate releases the wsA leases and leaves wsB
// intact.
// =============================================================================
func TestManager_OnWorkspaceDeactivate_ReleasesPerWS_M_Deactivate1(t *testing.T) {
	q := lspenrich.NewLaneQueue(8, 8)
	r := newRecordingAcquirer()
	m := lspenrich.NewManager(q, r, nil, nil, semantic.LSPEnrichmentConfig{}, nil, nil)

	ctx := context.Background()
	wsA := workspace.WorkspaceKey{RepoRoot: "/repo/a"}
	wsB := workspace.WorkspaceKey{RepoRoot: "/repo/b"}

	if _, err := m.AcquireFor(ctx, wsA, "go"); err != nil {
		t.Fatalf("acquire wsA/go: %v", err)
	}
	if _, err := m.AcquireFor(ctx, wsA, "java"); err != nil {
		t.Fatalf("acquire wsA/java: %v", err)
	}
	if _, err := m.AcquireFor(ctx, wsB, "go"); err != nil {
		t.Fatalf("acquire wsB/go: %v", err)
	}

	m.OnWorkspaceDeactivate(wsA)

	r.mu.Lock()
	releasedA_go := r.releaseCount["lsp-enrichment:/repo/a:go"]
	releasedA_java := r.releaseCount["lsp-enrichment:/repo/a:java"]
	releasedB_go := r.releaseCount["lsp-enrichment:/repo/b:go"]
	r.mu.Unlock()

	if releasedA_go != 1 || releasedA_java != 1 {
		t.Errorf("expected wsA/go and wsA/java to each release once; got %d / %d",
			releasedA_go, releasedA_java)
	}
	if releasedB_go != 0 {
		t.Errorf("expected wsB/go untouched; got release count %d", releasedB_go)
	}

	// The cache must no longer hold wsA entries — re-AcquireFor causes a
	// fresh AcquireLease call.
	if _, err := m.AcquireFor(ctx, wsA, "go"); err != nil {
		t.Fatalf("post-deactivate re-acquire: %v", err)
	}
	if r.calls != 4 {
		t.Errorf("post-deactivate re-acquire: calls=%d, want 4 (wsA/go re-fetched)", r.calls)
	}
}

// =============================================================================
// M-Stop1: Stop releases ALL cached leases across all workspaces.
// =============================================================================
func TestManager_Stop_ReleasesAll_M_Stop1(t *testing.T) {
	q := lspenrich.NewLaneQueue(8, 8)
	r := newRecordingAcquirer()
	m := lspenrich.NewManager(q, r, nil, nil, semantic.LSPEnrichmentConfig{}, nil, nil)

	ctx := context.Background()
	wsA := workspace.WorkspaceKey{RepoRoot: "/repo/a"}
	wsB := workspace.WorkspaceKey{RepoRoot: "/repo/b"}

	for _, args := range []struct {
		ws   workspace.WorkspaceKey
		lang string
	}{{wsA, "go"}, {wsA, "java"}, {wsB, "go"}} {
		if _, err := m.AcquireFor(ctx, args.ws, args.lang); err != nil {
			t.Fatalf("acquire %v/%s: %v", args.ws.RepoRoot, args.lang, err)
		}
	}

	m.Stop()

	r.mu.Lock()
	totalReleases := 0
	for _, n := range r.releaseCount {
		totalReleases += n
	}
	r.mu.Unlock()

	if totalReleases != 3 {
		t.Errorf("total releases after Stop = %d, want 3", totalReleases)
	}
}

// =============================================================================
// M-Concurrent1: 100 concurrent AcquireFor calls for the same (wsKey, lang)
// result in exactly ONE underlying AcquireLease call.  Race-clean.
// =============================================================================
func TestManager_AcquireFor_ConcurrentSingleflight_M_Concurrent1(t *testing.T) {
	q := lspenrich.NewLaneQueue(8, 8)
	r := newRecordingAcquirer()
	m := lspenrich.NewManager(q, r, nil, nil, semantic.LSPEnrichmentConfig{}, nil, nil)

	wsA := workspace.WorkspaceKey{RepoRoot: "/repo/a"}
	const N = 100
	var wg sync.WaitGroup
	var firstLease atomic.Pointer[lspool.WorkerLease]
	mismatches := atomic.Int64{}

	ctx := context.Background()
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			l, err := m.AcquireFor(ctx, wsA, "go")
			if err != nil {
				t.Errorf("AcquireFor: %v", err)
				return
			}
			// All N goroutines must observe the same cached lease pointer
			// (singleflight via the post-acquire re-check).
			if !firstLease.CompareAndSwap(nil, l) {
				if firstLease.Load() != l {
					mismatches.Add(1)
				}
			}
		}()
	}
	wg.Wait()

	if r.calls != 1 {
		t.Errorf("AcquireLease call count = %d, want 1 (singleflight)", r.calls)
	}
	if mismatches.Load() != 0 {
		t.Errorf("observed %d mismatched lease pointers; expected all goroutines to see the same cached lease", mismatches.Load())
	}
}

// =============================================================================
// M1 (Manager.Run): starts N workers per cfg.MaxConcurrentWorkers; ctx-cancel
// returns ctx.Err().
// =============================================================================
func TestManager_Run_HonorsCtxCancel_M1(t *testing.T) {
	q := lspenrich.NewLaneQueue(8, 8)
	r := newRecordingAcquirer()
	cfg := semantic.LSPEnrichmentConfig{
		MaxConcurrentWorkers: 2,
		TimeoutPerFile:       "1s",
		TimeoutTotal:         "10s",
	}
	m := lspenrich.NewManager(q, r, nil, nil, cfg, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())

	// Run in a goroutine; cancel after a tiny grace period.
	done := make(chan error, 1)
	go func() { done <- m.Run(ctx) }()

	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Run returned %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after ctx-cancel within 2s")
	}
}

// =============================================================================
// M2: Manager.Stop is idempotent.
// =============================================================================
func TestManager_Stop_Idempotent_M2(t *testing.T) {
	q := lspenrich.NewLaneQueue(8, 8)
	m := lspenrich.NewManager(q, &noopAcquirer{}, nil, nil, semantic.LSPEnrichmentConfig{}, nil, nil)
	m.Stop()
	m.Stop() // must not panic / deadlock
}

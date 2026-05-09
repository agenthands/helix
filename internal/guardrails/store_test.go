package guardrails

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agenthands/helix/internal/workspace"
)

// fakeMetrics is a test double that captures metric calls.
type fakeMetrics struct {
	issued  []string
	expired []string
	lookups []string
	mu      sync.Mutex
}

func (f *fakeMetrics) ReceiptIssuedInc(class string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.issued = append(f.issued, class)
}

func (f *fakeMetrics) ReceiptExpiredInc(reason string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.expired = append(f.expired, reason)
}

func (f *fakeMetrics) ReceiptLookupInc(outcome string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lookups = append(f.lookups, outcome)
}

func (f *fakeMetrics) issuedCount(class string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, c := range f.issued {
		if c == class {
			n++
		}
	}
	return n
}

func (f *fakeMetrics) expiredCount(reason string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, r := range f.expired {
		if r == reason {
			n++
		}
	}
	return n
}

func (f *fakeMetrics) lookupCount(outcome string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, o := range f.lookups {
		if o == outcome {
			n++
		}
	}
	return n
}

var testWS = workspace.WorkspaceKey{RepoRoot: "/repo", Language: "go", Toolchain: "1.25"}
var otherWS = workspace.WorkspaceKey{RepoRoot: "/other", Language: "go", Toolchain: "1.25"}

func issueReceipt(t *testing.T, s *Store, ws workspace.WorkspaceKey, gv uint64) ReceiptID {
	t.Helper()
	id, err := s.Issue(ws, ClassReferencesChecked, ReferencesCheckedScope{SymbolID: "sym1"}, IssueFields{
		GraphVersion: gv,
	})
	if err != nil {
		t.Fatalf("Issue failed: %v", err)
	}
	return id
}

func TestStore_TTLExpiry(t *testing.T) {
	t.Parallel()

	now := time.Now()
	clock := &atomic.Value{}
	clock.Store(now)

	fm := &fakeMetrics{}
	tick := make(chan time.Time)
	s := NewStoreWithTicker(StoreOptions{
		Metrics: fm,
		Now:     func() time.Time { return clock.Load().(time.Time) },
	}, tick)
	defer s.Close()

	// Issue with default 5-minute TTL.
	id := issueReceipt(t, s, testWS, 1)

	// Still valid.
	r, ok := s.Get(testWS, id)
	if !ok || r == nil {
		t.Fatal("expected hit before TTL expiry")
	}

	// Advance clock past TTL.
	clock.Store(now.Add(6 * time.Minute))

	// Trigger janitor.
	tick <- clock.Load().(time.Time)
	// Give janitor goroutine time to sweep.
	time.Sleep(50 * time.Millisecond)

	// Now should be gone.
	r2, ok2 := s.Get(testWS, id)
	if ok2 || r2 != nil {
		t.Fatal("expected miss after TTL + janitor sweep")
	}

	if fm.expiredCount("ttl") < 1 {
		t.Errorf("expected at least 1 ttl expiry, got %d", fm.expiredCount("ttl"))
	}
}

func TestStore_OpportunisticExpiry(t *testing.T) {
	t.Parallel()

	past := time.Now().Add(-10 * time.Minute)
	fm := &fakeMetrics{}
	// Don't start a janitor — use a never-firing channel.
	tick := make(chan time.Time)
	s := NewStoreWithTicker(StoreOptions{
		Metrics: fm,
		Now:     func() time.Time { return past }, // issue in the past
	}, tick)
	defer s.Close()

	// Issue at a time so ExpiresAt = past + 5m = still in the past.
	id, err := s.Issue(testWS, ClassReferencesChecked, ReferencesCheckedScope{}, IssueFields{})
	if err != nil {
		t.Fatalf("Issue failed: %v", err)
	}

	// Switch clock to "now" — ExpiresAt is in the past.
	realNow := time.Now()
	tick2 := make(chan time.Time)
	s2 := NewStoreWithTicker(StoreOptions{
		Metrics: fm,
		Now:     func() time.Time { return realNow },
	}, tick2)
	defer s2.Close()

	// Manually insert the receipt with expired ExpiresAt into s2.
	// We use issue fields with zero TTL = default, but override via a second issue.
	// Actually: just use the original store with a different clock for Get.
	// Simpler approach: use the original store; set a very short TTL.
	tick3 := make(chan time.Time)
	s3 := NewStoreWithTicker(StoreOptions{
		Metrics: fm,
		Now:     func() time.Time { return past },
	}, tick3)
	defer s3.Close()

	// Issue with 1-nanosecond TTL.
	id3, err := s3.Issue(testWS, ClassReferencesChecked, ReferencesCheckedScope{}, IssueFields{
		TTL: 1 * time.Nanosecond,
	})
	if err != nil {
		t.Fatalf("Issue s3 failed: %v", err)
	}
	_ = id

	// Now switch s3's clock to the future for Get.
	s3.now = func() time.Time { return time.Now() }

	// Get should opportunistically expire without janitor running.
	r, ok := s3.Get(testWS, id3)
	if ok || r != nil {
		t.Fatal("expected opportunistic expiry without janitor")
	}
	if fm.lookupCount("expired") < 1 {
		t.Errorf("expected at least 1 expired lookup, got %d", fm.lookupCount("expired"))
	}
}

func TestStore_WorkspaceMismatch(t *testing.T) {
	t.Parallel()

	fm := &fakeMetrics{}
	tick := make(chan time.Time)
	s := NewStoreWithTicker(StoreOptions{Metrics: fm}, tick)
	defer s.Close()

	id := issueReceipt(t, s, testWS, 1)

	// Looking up with a different workspace should return miss (not workspace_mismatch
	// in this case since the receipt isn't in the other workspace's entry).
	r, ok := s.Get(otherWS, id)
	if ok || r != nil {
		t.Fatal("expected miss when looking up from wrong workspace")
	}
	if fm.lookupCount("miss") < 1 {
		t.Errorf("expected miss lookup, got lookups=%v", fm.lookups)
	}
}

func TestStore_GraphVersionAdvance(t *testing.T) {
	t.Parallel()

	fm := &fakeMetrics{}
	tick := make(chan time.Time)
	s := NewStoreWithTicker(StoreOptions{Metrics: fm}, tick)
	defer s.Close()

	// Issue at graph_version 42.
	id := issueReceipt(t, s, testWS, 42)

	// Confirm it's there.
	r, ok := s.Get(testWS, id)
	if !ok || r == nil {
		t.Fatal("expected hit before invalidation")
	}

	// Advance graph version to 43 — receipt at GV=42 should be deleted.
	s.InvalidateOnGraphVersionAdvance(testWS, 43)

	r2, ok2 := s.Get(testWS, id)
	if ok2 || r2 != nil {
		t.Fatal("expected miss after graph_version invalidation")
	}
	if fm.expiredCount("graph_drift") < 1 {
		t.Errorf("expected at least 1 graph_drift expiry, got %d", fm.expiredCount("graph_drift"))
	}
}

func TestStore_LRU(t *testing.T) {
	t.Parallel()

	fm := &fakeMetrics{}
	tick := make(chan time.Time)
	// Cap = 4.
	s := NewStoreWithTicker(StoreOptions{LRUCap: 4, Metrics: fm}, tick)
	defer s.Close()

	ids := make([]ReceiptID, 4)
	for i := range ids {
		ids[i] = issueReceipt(t, s, testWS, uint64(i+1))
	}

	// All 4 should be present.
	for _, id := range ids {
		_, ok := s.Get(testWS, id)
		if !ok {
			t.Errorf("expected receipt %q to be present before LRU eviction", id)
		}
	}

	// Issuing a 5th should evict the oldest (ids[0]).
	_ = issueReceipt(t, s, testWS, 5)

	if fm.expiredCount("lru_evicted") < 1 {
		t.Errorf("expected at least 1 lru_evicted, got %d", fm.expiredCount("lru_evicted"))
	}
}

func TestStore_Janitor(t *testing.T) {
	t.Parallel()

	past := time.Now().Add(-10 * time.Minute)
	clockVal := past
	var clockMu sync.Mutex
	getClock := func() time.Time {
		clockMu.Lock()
		defer clockMu.Unlock()
		return clockVal
	}

	fm := &fakeMetrics{}
	tick := make(chan time.Time, 1)
	s := NewStoreWithTicker(StoreOptions{
		Metrics: fm,
		Now:     getClock,
	}, tick)
	defer s.Close()

	// Issue with 1-nanosecond TTL so it's immediately expired by the janitor.
	_, err := s.Issue(testWS, ClassReferencesChecked, ReferencesCheckedScope{}, IssueFields{
		TTL: 1 * time.Nanosecond,
	})
	if err != nil {
		t.Fatalf("Issue failed: %v", err)
	}

	// Advance clock so ExpiresAt is in the past.
	clockMu.Lock()
	clockVal = time.Now()
	clockMu.Unlock()

	// Send a tick to trigger janitor sweep.
	tick <- time.Now()

	// Wait for janitor to process.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if fm.expiredCount("ttl") >= 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if fm.expiredCount("ttl") < 1 {
		t.Errorf("expected janitor to sweep expired receipt; expired=%v", fm.expired)
	}
}

func TestStore_Concurrent(t *testing.T) {
	t.Parallel()

	tick := make(chan time.Time)
	s := NewStoreWithTicker(StoreOptions{}, tick)
	defer s.Close()

	const N = 100
	const M = 100

	var wg sync.WaitGroup
	wg.Add(N)
	for i := 0; i < N; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < M; j++ {
				id, err := s.Issue(testWS, ClassReferencesChecked, ReferencesCheckedScope{}, IssueFields{})
				if err != nil {
					return
				}
				s.Get(testWS, id)
			}
		}()
	}
	wg.Wait()
}

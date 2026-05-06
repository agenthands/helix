// Phase 62 P02 RED gate — failing tests for Engine.ApplyRepair.
//
// The recording fake `recordingRepairStore` satisfies the narrow
// `RepairStore` interface declared in apply_repair.go (Task 3). Because
// production has not yet landed, this file does not compile until both
// Task 2 (store extensions) and Task 3 (graph package implementation) are
// in place. That is the intentional RED gate.
package graph

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// recordingRepairStore is the unit-test fake. It tracks BumpGraphVersion
// invocations and lock-acquire / lock-release sequencing.
type recordingRepairStore struct {
	mu          sync.Mutex
	bumps       int
	currentGV   uint64
	lockEvents  []string
	beginCalls  int
	commitCalls int
}

func (s *recordingRepairStore) lockFor(repoID string) (acquire func(), release func()) {
	return func() {
			s.mu.Lock()
			s.lockEvents = append(s.lockEvents, "lock:"+repoID)
		}, func() {
			s.lockEvents = append(s.lockEvents, "unlock:"+repoID)
			s.mu.Unlock()
		}
}

type recordingRepairTx struct {
	store    *recordingRepairStore
	repoID   string
	rolled   bool
	commited bool
}

func (s *recordingRepairStore) LockWorkspace(repoID string) (release func()) {
	acq, rel := s.lockFor(repoID)
	acq()
	return rel
}

func (s *recordingRepairStore) BeginRepairTx(ctx context.Context, repoID string) (RepairTx, error) {
	s.beginCalls++
	return &recordingRepairTx{store: s, repoID: repoID}, nil
}

func (s *recordingRepairStore) CurrentGraphVersion(ctx context.Context, repoID string) (uint64, error) {
	return s.currentGV, nil
}

func (t *recordingRepairTx) BumpGraphVersion(ctx context.Context) (uint64, error) {
	t.store.bumps++
	t.store.currentGV++
	return t.store.currentGV, nil
}

func (t *recordingRepairTx) MarkSymbolsDeleted(ctx context.Context, fileIDs []uint64) error {
	return nil
}

func (t *recordingRepairTx) MarkEdgesDeleted(ctx context.Context, nodeIDs []uint64) error {
	return nil
}

func (t *recordingRepairTx) UpsertEdgesWithMerge(ctx context.Context, edges []EdgeUpsert) error {
	return nil
}

func (t *recordingRepairTx) Commit() error {
	t.commited = true
	t.store.commitCalls++
	return nil
}

func (t *recordingRepairTx) Rollback() error {
	t.rolled = true
	return nil
}

func TestApplyRepair_VersionMonotonic(t *testing.T) {
	s := &recordingRepairStore{}
	e := &Engine{store: s}
	repair := GraphRepair{DirtyNodes: []NodeID{1, 2}}
	for i := 1; i <= 5; i++ {
		gv, bumped, err := e.ApplyRepair(context.Background(), "ws", repair)
		if err != nil {
			t.Fatalf("call #%d: %v", i, err)
		}
		if !bumped {
			t.Fatalf("call #%d: bumped=false, want true", i)
		}
		if gv != uint64(i) {
			t.Errorf("call #%d: gv=%d, want %d", i, gv, i)
		}
	}
	if s.bumps != 5 {
		t.Errorf("bumps=%d, want 5", s.bumps)
	}
}

func TestApplyRepair_BodyOnlyNoBump(t *testing.T) {
	s := &recordingRepairStore{currentGV: 7}
	e := &Engine{store: s}
	// Empty repair (body-only diff produces this).
	gv, bumped, err := e.ApplyRepair(context.Background(), "ws", GraphRepair{})
	if err != nil {
		t.Fatalf("ApplyRepair(empty): %v", err)
	}
	if bumped {
		t.Errorf("bumped=true, want false (body-only short-circuit)")
	}
	if gv != 7 {
		t.Errorf("gv=%d, want 7 (no advance)", gv)
	}
	if s.bumps != 0 {
		t.Errorf("BumpGraphVersion called %d times on empty repair, want 0", s.bumps)
	}
	if s.beginCalls != 0 {
		t.Errorf("BeginRepairTx called %d times on empty repair, want 0", s.beginCalls)
	}
}

func TestApplyRepair_SingleBumpSiteOnly(t *testing.T) {
	s := &recordingRepairStore{}
	e := &Engine{store: s}
	const N = 10
	for i := 0; i < N; i++ {
		repair := GraphRepair{DirtyNodes: []NodeID{NodeID(i + 1)}}
		if _, _, err := e.ApplyRepair(context.Background(), "ws", repair); err != nil {
			t.Fatalf("call #%d: %v", i, err)
		}
	}
	if s.bumps != N {
		t.Errorf("BumpGraphVersion called %d times, want exactly %d (D-06 single-bump invariant)", s.bumps, N)
	}
}

func TestApplyRepair_HoldsMutex(t *testing.T) {
	s := &recordingRepairStore{}
	e := &Engine{store: s}
	repair := GraphRepair{DirtyNodes: []NodeID{1}}
	if _, _, err := e.ApplyRepair(context.Background(), "ws", repair); err != nil {
		t.Fatalf("ApplyRepair: %v", err)
	}
	// Lock acquired, then released; both events recorded.
	if len(s.lockEvents) != 2 {
		t.Fatalf("lockEvents=%v, want exactly [lock,unlock]", s.lockEvents)
	}
	if s.lockEvents[0] != "lock:ws" || s.lockEvents[1] != "unlock:ws" {
		t.Errorf("lockEvents=%v, want [lock:ws, unlock:ws]", s.lockEvents)
	}
}

// productionMutexStore implements RepairStore against a real per-workspace
// mutex map (mirroring store.overlayLockFor) so the W1 serialization test
// can exercise the lock without a DuckDB dependency.
type productionMutexStore struct {
	mu       sync.Mutex
	locks    map[string]*sync.Mutex
	beginCh  chan time.Time
	commitCh chan time.Time
	gvMu     sync.Mutex
	gv       uint64
}

func newProductionMutexStore() *productionMutexStore {
	return &productionMutexStore{
		locks:    map[string]*sync.Mutex{},
		beginCh:  make(chan time.Time, 8),
		commitCh: make(chan time.Time, 8),
	}
}

func (s *productionMutexStore) LockWorkspace(repoID string) func() {
	s.mu.Lock()
	mu, ok := s.locks[repoID]
	if !ok {
		mu = &sync.Mutex{}
		s.locks[repoID] = mu
	}
	s.mu.Unlock()
	mu.Lock()
	return mu.Unlock
}

type productionMutexTx struct {
	store *productionMutexStore
}

func (s *productionMutexStore) BeginRepairTx(ctx context.Context, repoID string) (RepairTx, error) {
	s.beginCh <- time.Now()
	// Hold for a beat to expose serialization windows.
	time.Sleep(20 * time.Millisecond)
	return &productionMutexTx{store: s}, nil
}

func (s *productionMutexStore) CurrentGraphVersion(ctx context.Context, repoID string) (uint64, error) {
	s.gvMu.Lock()
	defer s.gvMu.Unlock()
	return s.gv, nil
}

func (t *productionMutexTx) BumpGraphVersion(ctx context.Context) (uint64, error) {
	t.store.gvMu.Lock()
	defer t.store.gvMu.Unlock()
	t.store.gv++
	return t.store.gv, nil
}

func (t *productionMutexTx) MarkSymbolsDeleted(ctx context.Context, ids []uint64) error { return nil }
func (t *productionMutexTx) MarkEdgesDeleted(ctx context.Context, ids []uint64) error   { return nil }
func (t *productionMutexTx) UpsertEdgesWithMerge(ctx context.Context, e []EdgeUpsert) error {
	return nil
}
func (t *productionMutexTx) Commit() error {
	t.store.commitCh <- time.Now()
	return nil
}
func (t *productionMutexTx) Rollback() error { return nil }

func TestApplyRepair_ProductionMutexSerializes(t *testing.T) {
	s := newProductionMutexStore()
	e := &Engine{store: s}

	repair := GraphRepair{DirtyNodes: []NodeID{1}}
	var done atomic.Int32
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _, _ = e.ApplyRepair(context.Background(), "ws-shared", repair)
		done.Add(1)
	}()
	go func() {
		defer wg.Done()
		_, _, _ = e.ApplyRepair(context.Background(), "ws-shared", repair)
		done.Add(1)
	}()
	wg.Wait()

	close(s.beginCh)
	close(s.commitCh)

	begins := drainTimes(s.beginCh)
	commits := drainTimes(s.commitCh)
	if len(begins) != 2 || len(commits) != 2 {
		t.Fatalf("expected 2 begins + 2 commits, got begins=%d commits=%d", len(begins), len(commits))
	}
	// Serialization invariant: the second goroutine's begin happens AFTER
	// the first goroutine's commit (modulo a tiny scheduling jitter).
	earliestCommit := commits[0]
	if commits[1].Before(earliestCommit) {
		earliestCommit = commits[1]
	}
	latestBegin := begins[0]
	if begins[1].After(latestBegin) {
		latestBegin = begins[1]
	}
	if latestBegin.Before(earliestCommit) {
		t.Errorf("ApplyRepair did not serialize: latest begin=%v < earliest commit=%v",
			latestBegin, earliestCommit)
	}
}

func drainTimes(ch chan time.Time) []time.Time {
	out := []time.Time{}
	for t := range ch {
		out = append(out, t)
	}
	return out
}

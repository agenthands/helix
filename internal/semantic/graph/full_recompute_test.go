// Phase 62 P03 RED gate — failing tests for RunFullRecompute.
package graph

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
)

// fakeFullRecomputeStore is a recording fake that satisfies SchedulerStore
// for the full-recompute path. It tracks score writes + projection deletes
// and lets the test inject a "graph_version advanced mid-run" preemption.
//
// Two mutexes:
//   - lockMu is the per-workspace lock the SUT acquires via LockWorkspace
//   - dataMu protects the data fields the recording accesses
//
// Conflating them deadlocks: RunFullRecompute holds the workspace lock
// across DeleteScoresForProjection / UpsertGraphScores, both of which
// touch the data fields.
type fakeFullRecomputeStore struct {
	lockMu    sync.Mutex
	dataMu    sync.Mutex
	repoID    string
	startGV   uint64
	endGV     uint64
	nodes     []NodeID
	adjOut    map[NodeID]map[NodeID]float64
	adjIn     map[NodeID]map[NodeID]float64
	rowsByGV  map[uint64][]ScoreRowSnapshot
	deletedGV []uint64
}

// ScoreRowSnapshot captures a row that the scheduler asked the tx to upsert.
type ScoreRowSnapshot struct {
	Projection string
	NodeID     NodeID
	Score      float64
	Status     string
	GV         uint64
}

func newFakeFullRecomputeStore() *fakeFullRecomputeStore {
	return &fakeFullRecomputeStore{
		adjOut:   map[NodeID]map[NodeID]float64{},
		adjIn:    map[NodeID]map[NodeID]float64{},
		rowsByGV: map[uint64][]ScoreRowSnapshot{},
	}
}

func (s *fakeFullRecomputeStore) LockWorkspace(string) func() {
	s.lockMu.Lock()
	return s.lockMu.Unlock
}
func (s *fakeFullRecomputeStore) BeginRepairTx(ctx context.Context, repoID string) (RepairTx, error) {
	return &fakeFullRecomputeTx{store: s, repoID: repoID}, nil
}
func (s *fakeFullRecomputeStore) CurrentGraphVersion(context.Context, string) (uint64, error) {
	return s.endGV, nil
}
func (s *fakeFullRecomputeStore) QueryEffectiveGraph(_ context.Context, _, _ string) ([]NodeID, map[NodeID]map[NodeID]float64, error) {
	out := map[NodeID]map[NodeID]float64{}
	for k, v := range s.adjOut {
		inner := map[NodeID]float64{}
		for k2, v2 := range v {
			inner[k2] = v2
		}
		out[k] = inner
	}
	nodes := append([]NodeID(nil), s.nodes...)
	return nodes, out, nil
}
func (s *fakeFullRecomputeStore) QueryEffectiveAdjacency(_ context.Context, _, _ string) (map[NodeID]map[NodeID]float64, map[NodeID]map[NodeID]float64, error) {
	return s.adjOut, s.adjIn, nil
}
func (s *fakeFullRecomputeStore) CountStaleScoreRows(context.Context, string, string) (int, int, error) {
	return 0, len(s.nodes), nil
}
func (s *fakeFullRecomputeStore) MarkAllScoreRowsStale(context.Context, string, string) error {
	return nil
}

type fakeFullRecomputeTx struct {
	store  *fakeFullRecomputeStore
	repoID string
	// preemptOnUpsert advances endGV between BeginRepairTx and the first
	// UpsertGraphScores call to simulate a competing ApplyRepair landing.
	preemptOnUpsert bool
}

func (t *fakeFullRecomputeTx) BumpGraphVersion(context.Context) (uint64, error) {
	t.store.endGV++
	return t.store.endGV, nil
}
func (t *fakeFullRecomputeTx) MarkSymbolsDeleted(context.Context, []uint64) error  { return nil }
func (t *fakeFullRecomputeTx) MarkEdgesDeleted(context.Context, []uint64) error    { return nil }
func (t *fakeFullRecomputeTx) UpsertEdgesWithMerge(context.Context, []EdgeUpsert) error {
	return nil
}
func (t *fakeFullRecomputeTx) UpsertGraphScores(_ context.Context, projection string, rows []ScoreRow) error {
	t.store.dataMu.Lock()
	defer t.store.dataMu.Unlock()
	if t.preemptOnUpsert {
		t.store.endGV++
		t.preemptOnUpsert = false
	}
	// Mirror the SQL ON CONFLICT contract — same (projection, NodeID, GV)
	// row gets overwritten in place, not duplicated. The store-side
	// UpsertGraphScores carries this semantics; we replicate it so the
	// fake matches production's last-write-wins behavior under preemption
	// rewrites.
	for _, r := range rows {
		bucket := t.store.rowsByGV[r.GraphVersion]
		replaced := false
		for i, existing := range bucket {
			if existing.Projection == projection && existing.NodeID == r.NodeID {
				bucket[i] = ScoreRowSnapshot{
					Projection: projection, NodeID: r.NodeID, Score: r.Score,
					Status: r.Status, GV: r.GraphVersion,
				}
				replaced = true
				break
			}
		}
		if !replaced {
			bucket = append(bucket, ScoreRowSnapshot{
				Projection: projection, NodeID: r.NodeID, Score: r.Score,
				Status: r.Status, GV: r.GraphVersion,
			})
		}
		t.store.rowsByGV[r.GraphVersion] = bucket
	}
	return nil
}
func (t *fakeFullRecomputeTx) DeleteScoresForProjection(_ context.Context, projection string) error {
	t.store.dataMu.Lock()
	defer t.store.dataMu.Unlock()
	for gv, rows := range t.store.rowsByGV {
		kept := rows[:0]
		for _, r := range rows {
			if r.Projection != projection {
				kept = append(kept, r)
			}
		}
		if len(kept) == 0 {
			t.store.deletedGV = append(t.store.deletedGV, gv)
			delete(t.store.rowsByGV, gv)
		} else {
			t.store.rowsByGV[gv] = kept
		}
	}
	return nil
}
func (t *fakeFullRecomputeTx) Commit() error   { return nil }
func (t *fakeFullRecomputeTx) Rollback() error { return nil }

func TestRunFullRecompute_BasicWritesNewRows(t *testing.T) {
	store := newFakeFullRecomputeStore()
	store.startGV = 5
	store.endGV = 5
	for i := 1; i <= 50; i++ {
		store.nodes = append(store.nodes, NodeID(i))
	}
	preempted, err := RunFullRecompute(context.Background(), "ws", "call_graph", store, FullRecomputeOptions{
		Damping: 0.85, Epsilon: 1e-6, MaxIter: 100,
	})
	if err != nil {
		t.Fatalf("RunFullRecompute: %v", err)
	}
	if preempted {
		t.Errorf("preempted=true on quiet run, want false")
	}
	rows := store.rowsByGV[5]
	if len(rows) != 50 {
		t.Errorf("rows@gv=5: got %d, want 50", len(rows))
	}
	for _, r := range rows {
		if r.Status != string(ScoreStatusExact) {
			t.Errorf("row node=%d status=%q, want exact", r.NodeID, r.Status)
		}
	}
}

func TestRunFullRecompute_PreemptedMarksApproximate(t *testing.T) {
	store := newFakeFullRecomputeStore()
	store.startGV = 5
	store.endGV = 5
	for i := 1; i <= 20; i++ {
		store.nodes = append(store.nodes, NodeID(i))
	}
	// Wrap BeginRepairTx so the returned tx has preemptOnUpsert=true.
	store2 := &preemptingStore{inner: store}
	preempted, err := RunFullRecompute(context.Background(), "ws", "call_graph", store2, FullRecomputeOptions{
		Damping: 0.85, Epsilon: 1e-6, MaxIter: 100,
	})
	if err != nil {
		t.Fatalf("RunFullRecompute: %v", err)
	}
	if !preempted {
		t.Errorf("preempted=false, want true (endGV advanced mid-run)")
	}
	var seen int
	for _, rows := range store.rowsByGV {
		for _, r := range rows {
			seen++
			if r.Status != string(ScoreStatusApproximate) {
				t.Errorf("node=%d status=%q, want approximate (preempted run)", r.NodeID, r.Status)
			}
		}
	}
	if seen != 20 {
		t.Errorf("rows seen=%d, want 20", seen)
	}
}

type preemptingStore struct {
	inner *fakeFullRecomputeStore
	once  atomic.Bool
}

func (s *preemptingStore) LockWorkspace(repoID string) func() { return s.inner.LockWorkspace(repoID) }
func (s *preemptingStore) BeginRepairTx(ctx context.Context, repoID string) (RepairTx, error) {
	tx, err := s.inner.BeginRepairTx(ctx, repoID)
	if err != nil {
		return nil, err
	}
	if s.once.CompareAndSwap(false, true) {
		// Cast back, set the preempt flag.
		ftx := tx.(*fakeFullRecomputeTx)
		ftx.preemptOnUpsert = true
	}
	return tx, nil
}
func (s *preemptingStore) CurrentGraphVersion(ctx context.Context, repoID string) (uint64, error) {
	return s.inner.CurrentGraphVersion(ctx, repoID)
}
func (s *preemptingStore) QueryEffectiveGraph(ctx context.Context, r, p string) ([]NodeID, map[NodeID]map[NodeID]float64, error) {
	return s.inner.QueryEffectiveGraph(ctx, r, p)
}
func (s *preemptingStore) QueryEffectiveAdjacency(ctx context.Context, r, p string) (map[NodeID]map[NodeID]float64, map[NodeID]map[NodeID]float64, error) {
	return s.inner.QueryEffectiveAdjacency(ctx, r, p)
}
func (s *preemptingStore) CountStaleScoreRows(ctx context.Context, r, p string) (int, int, error) {
	return s.inner.CountStaleScoreRows(ctx, r, p)
}
func (s *preemptingStore) MarkAllScoreRowsStale(ctx context.Context, r, p string) error {
	return s.inner.MarkAllScoreRowsStale(ctx, r, p)
}

func TestRunFullRecompute_DeletesPriorRowsInSameTx(t *testing.T) {
	store := newFakeFullRecomputeStore()
	store.startGV = 5
	store.endGV = 5
	for i := 1; i <= 10; i++ {
		store.nodes = append(store.nodes, NodeID(i))
	}
	// Pre-populate rows from an older gv=4 generation that should be deleted.
	store.rowsByGV[4] = []ScoreRowSnapshot{
		{Projection: "call_graph", NodeID: 1, Score: 0.1, Status: "exact", GV: 4},
		{Projection: "call_graph", NodeID: 2, Score: 0.1, Status: "exact", GV: 4},
	}
	if _, err := RunFullRecompute(context.Background(), "ws", "call_graph", store, FullRecomputeOptions{
		Damping: 0.85, Epsilon: 1e-6, MaxIter: 100,
	}); err != nil {
		t.Fatalf("RunFullRecompute: %v", err)
	}
	if _, ok := store.rowsByGV[4]; ok {
		t.Errorf("rows@gv=4 still present after RunFullRecompute, want deleted")
	}
	if len(store.rowsByGV[5]) != 10 {
		t.Errorf("rows@gv=5: got %d, want 10", len(store.rowsByGV[5]))
	}
}

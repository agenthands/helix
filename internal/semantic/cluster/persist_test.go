// Phase 62 P04 — RunClusterDetection orchestration tests.
//
// These tests use a hand-rolled fake ClusterStore + ClusterTx to verify
// the call sequencing, rollback-on-error, and graph_version threading.
// The store package's own UpsertClusters/UpsertClusterMembers/
// DeleteClustersForGraphVersion behavior is covered by overlay_test.go.

package cluster

import (
	"context"
	"errors"
	"testing"

	"github.com/agenthands/helix/internal/semantic/graph"
	"github.com/agenthands/helix/internal/semantic/store"
)

// fakeStore satisfies the ClusterStore interface with caller-supplied
// graph + a configurable graph_version. The fake records every call to
// the returned tx so tests can assert sequencing.
type fakeStore struct {
	gv    uint64
	nodes []graph.NodeID
	edges map[graph.NodeID]map[graph.NodeID]float64

	// Per-call hooks (nil = success).
	graphVersionErr error
	queryErr        error
	beginErr        error
}

func (f *fakeStore) CurrentGraphVersion(_ context.Context, _ string) (uint64, error) {
	return f.gv, f.graphVersionErr
}

func (f *fakeStore) QueryEffectiveGraph(_ context.Context, _, _ string) (
	[]graph.NodeID, map[graph.NodeID]map[graph.NodeID]float64, error,
) {
	return f.nodes, f.edges, f.queryErr
}

func (f *fakeStore) BeginOverlayTx(_ context.Context, _ string) (ClusterTx, error) {
	if f.beginErr != nil {
		return nil, f.beginErr
	}
	return &fakeTx{}, nil
}

// fakeTx records every call so tests can assert ordering and rollback.
type fakeTx struct {
	calls []string

	deleteCalls   []deleteCall
	upsertClus    []upsertClustersCall
	upsertMembers []upsertMembersCall

	// Per-call error injection.
	deleteErr        error
	upsertClusErr    error
	upsertMembersErr error
	commitErr        error

	committed  bool
	rolledBack bool
}

type deleteCall struct {
	projection string
	gv         uint64
}

type upsertClustersCall struct {
	projection string
	gv         uint64
	rows       []store.ClusterSummary
}

type upsertMembersCall struct {
	projection string
	gv         uint64
	rows       []store.ClusterMemberRow
}

func (t *fakeTx) DeleteClustersForGraphVersion(_ context.Context, projection string, gv uint64) error {
	t.calls = append(t.calls, "Delete")
	t.deleteCalls = append(t.deleteCalls, deleteCall{projection, gv})
	return t.deleteErr
}

func (t *fakeTx) UpsertClusters(_ context.Context, projection string, gv uint64, rows []store.ClusterSummary) error {
	t.calls = append(t.calls, "UpsertClusters")
	t.upsertClus = append(t.upsertClus, upsertClustersCall{projection, gv, rows})
	return t.upsertClusErr
}

func (t *fakeTx) UpsertClusterMembers(_ context.Context, projection string, gv uint64, rows []store.ClusterMemberRow) error {
	t.calls = append(t.calls, "UpsertClusterMembers")
	t.upsertMembers = append(t.upsertMembers, upsertMembersCall{projection, gv, rows})
	return t.upsertMembersErr
}

func (t *fakeTx) Commit() error {
	t.calls = append(t.calls, "Commit")
	if t.commitErr != nil {
		return t.commitErr
	}
	t.committed = true
	return nil
}

func (t *fakeTx) Rollback() error {
	t.calls = append(t.calls, "Rollback")
	t.rolledBack = true
	return nil
}

// TestRunClusterDetection_RoundTrip drives the orchestrator over a 9-node
// 3-component graph and asserts (a) WeakComponents partitioned correctly,
// (b) Delete ran BEFORE UpsertClusters, (c) all three writes ran, (d)
// Commit landed and Rollback did NOT.
func TestRunClusterDetection_RoundTrip(t *testing.T) {
	nodes, edges := threeComponents()
	fs := &fakeStore{gv: 7, nodes: nodes, edges: edges}

	count, gv, err := RunClusterDetection(context.Background(), "wsX", "weak_components", fs)
	if err != nil {
		t.Fatalf("RunClusterDetection: %v", err)
	}
	if count != 3 {
		t.Errorf("count: got %d, want 3", count)
	}
	if gv != 7 {
		t.Errorf("gv: got %d, want 7", gv)
	}

	// Capture the tx the fake handed out by re-invoking BeginOverlayTx —
	// since the orchestrator already closed it, we instead capture by
	// having BeginOverlayTx return a tx the test holds. Restructure:
	// we cannot easily reach into the closed tx here, so re-run with a
	// tx-capturing fake.
	captured := &fakeTx{}
	captureFS := &capturingFakeStore{fakeStore: fs, tx: captured}
	if _, _, err := RunClusterDetection(context.Background(), "wsX", "weak_components", captureFS); err != nil {
		t.Fatalf("RunClusterDetection (capture): %v", err)
	}
	if !captured.committed {
		t.Errorf("captured tx not committed")
	}
	if captured.rolledBack {
		t.Errorf("captured tx unexpectedly rolled back")
	}
	want := []string{"Delete", "UpsertClusters", "UpsertClusterMembers", "Commit"}
	if len(captured.calls) != len(want) {
		t.Fatalf("call count: got %d (%v), want %d (%v)",
			len(captured.calls), captured.calls, len(want), want)
	}
	for i, c := range want {
		if captured.calls[i] != c {
			t.Errorf("call[%d]: got %q, want %q", i, captured.calls[i], c)
		}
	}
	// Sanity: the cluster + member writes use the same gv.
	if got := captured.upsertClus[0].gv; got != 7 {
		t.Errorf("UpsertClusters gv: got %d, want 7", got)
	}
	if got := captured.upsertMembers[0].gv; got != 7 {
		t.Errorf("UpsertClusterMembers gv: got %d, want 7", got)
	}
	// Three components → three summaries; sum of MemberCount == total members.
	if len(captured.upsertClus[0].rows) != 3 {
		t.Errorf("UpsertClusters row count: got %d, want 3", len(captured.upsertClus[0].rows))
	}
	if len(captured.upsertMembers[0].rows) != 9 {
		t.Errorf("UpsertClusterMembers row count: got %d, want 9", len(captured.upsertMembers[0].rows))
	}
}

// capturingFakeStore exposes the inner fakeTx so tests can inspect calls
// after RunClusterDetection completes.
type capturingFakeStore struct {
	*fakeStore
	tx *fakeTx
}

func (c *capturingFakeStore) BeginOverlayTx(_ context.Context, _ string) (ClusterTx, error) {
	if c.beginErr != nil {
		return nil, c.beginErr
	}
	return c.tx, nil
}

// TestRunClusterDetection_RollsBackOnError injects an UpsertClusters
// failure and asserts (a) error propagated, (b) Rollback called,
// (c) Commit NOT called.
func TestRunClusterDetection_RollsBackOnError(t *testing.T) {
	nodes, edges := threeComponents()
	captured := &fakeTx{
		upsertClusErr: errors.New("boom"),
	}
	fs := &capturingFakeStore{
		fakeStore: &fakeStore{gv: 7, nodes: nodes, edges: edges},
		tx:        captured,
	}
	_, _, err := RunClusterDetection(context.Background(), "wsX", "weak_components", fs)
	if err == nil {
		t.Fatalf("expected error from UpsertClusters")
	}
	if captured.committed {
		t.Errorf("tx unexpectedly committed")
	}
	if !captured.rolledBack {
		t.Errorf("tx not rolled back")
	}
	// Commit must NOT appear in the call trail.
	for _, c := range captured.calls {
		if c == "Commit" {
			t.Errorf("Commit call observed in trail %v despite error", captured.calls)
		}
	}
	// Rollback must appear AFTER the failed UpsertClusters.
	want := []string{"Delete", "UpsertClusters", "Rollback"}
	if len(captured.calls) != len(want) {
		t.Fatalf("call trail: got %v, want %v", captured.calls, want)
	}
	for i, c := range want {
		if captured.calls[i] != c {
			t.Errorf("call[%d]: got %q, want %q", i, captured.calls[i], c)
		}
	}
}

// TestRunClusterDetection_UsesCurrentGraphVersion seeds the fake at gv=42
// and asserts that the persistence calls are stamped with gv=42 (not 0).
func TestRunClusterDetection_UsesCurrentGraphVersion(t *testing.T) {
	nodes, edges := threeComponents()
	captured := &fakeTx{}
	fs := &capturingFakeStore{
		fakeStore: &fakeStore{gv: 42, nodes: nodes, edges: edges},
		tx:        captured,
	}
	count, gv, err := RunClusterDetection(context.Background(), "wsX", "weak_components", fs)
	if err != nil {
		t.Fatalf("RunClusterDetection: %v", err)
	}
	if gv != 42 {
		t.Errorf("returned gv: got %d, want 42", gv)
	}
	if count != 3 {
		t.Errorf("count: got %d, want 3", count)
	}
	if captured.deleteCalls[0].gv != 42 {
		t.Errorf("Delete gv: got %d, want 42", captured.deleteCalls[0].gv)
	}
	if captured.upsertClus[0].gv != 42 {
		t.Errorf("UpsertClusters gv: got %d, want 42", captured.upsertClus[0].gv)
	}
	if captured.upsertMembers[0].gv != 42 {
		t.Errorf("UpsertClusterMembers gv: got %d, want 42", captured.upsertMembers[0].gv)
	}
}

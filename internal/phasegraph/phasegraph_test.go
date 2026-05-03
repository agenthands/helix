package phasegraph_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/agenthands/helix/internal/phasegraph"
)

// helper: noopRun is a benign Run body for fixtures that never inspects deps.
func noopRun(_ context.Context, _ phasegraph.PhaseDeps) (phasegraph.PhaseOutput, error) {
	return nil, nil
}

// TestValidate_HappyPath: 3-phase linear chain (A→B→C) returns
// Order={A,B,C} and ShutdownOrder={C,B,A}.
func TestValidate_HappyPath(t *testing.T) {
	phases := []phasegraph.PhaseSpec{
		{ID: "A", Run: noopRun},
		{ID: "B", Requires: []phasegraph.PhaseID{"A"}, Run: noopRun},
		{ID: "C", Requires: []phasegraph.PhaseID{"B"}, Run: noopRun},
	}
	g, err := phasegraph.ValidatePhaseGraph(phases)
	if err != nil {
		t.Fatalf("ValidatePhaseGraph: unexpected error: %v", err)
	}
	if len(g.Order) != 3 {
		t.Fatalf("Order len = %d, want 3", len(g.Order))
	}
	wantOrder := []phasegraph.PhaseID{"A", "B", "C"}
	for i, p := range g.Order {
		if p.ID != wantOrder[i] {
			t.Errorf("Order[%d].ID = %q, want %q", i, p.ID, wantOrder[i])
		}
	}
	wantShutdown := []phasegraph.PhaseID{"C", "B", "A"}
	if len(g.ShutdownOrder) != 3 {
		t.Fatalf("ShutdownOrder len = %d, want 3", len(g.ShutdownOrder))
	}
	for i, p := range g.ShutdownOrder {
		if p.ID != wantShutdown[i] {
			t.Errorf("ShutdownOrder[%d].ID = %q, want %q", i, p.ID, wantShutdown[i])
		}
	}
}

// TestValidate_RejectsDuplicateID: two phases sharing ID="x" returns
// PhaseGraphError{Kind:"duplicate_phase", Phase:"x"}.
func TestValidate_RejectsDuplicateID(t *testing.T) {
	phases := []phasegraph.PhaseSpec{
		{ID: "x", Run: noopRun},
		{ID: "x", Run: noopRun},
	}
	_, err := phasegraph.ValidatePhaseGraph(phases)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var pgErr phasegraph.PhaseGraphError
	if !errors.As(err, &pgErr) {
		t.Fatalf("expected PhaseGraphError, got %T: %v", err, err)
	}
	if pgErr.Kind != "duplicate_phase" {
		t.Errorf("Kind = %q, want duplicate_phase", pgErr.Kind)
	}
	if pgErr.Phase != "x" {
		t.Errorf("Phase = %q, want x", pgErr.Phase)
	}
}

// TestValidate_RejectsMissingDep: phase B requires PhaseID("a") but A is absent.
func TestValidate_RejectsMissingDep(t *testing.T) {
	phases := []phasegraph.PhaseSpec{
		{ID: "B", Requires: []phasegraph.PhaseID{"a"}, Run: noopRun},
	}
	_, err := phasegraph.ValidatePhaseGraph(phases)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var pgErr phasegraph.PhaseGraphError
	if !errors.As(err, &pgErr) {
		t.Fatalf("expected PhaseGraphError, got %T: %v", err, err)
	}
	if pgErr.Kind != "missing_dependency" {
		t.Errorf("Kind = %q, want missing_dependency", pgErr.Kind)
	}
	found := false
	for _, m := range pgErr.Missing {
		if m == "a" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Missing = %v, want to contain 'a'", pgErr.Missing)
	}
}

// TestValidate_RejectsSingleNodeCycle: phase A requires PhaseID("a") (self-loop).
func TestValidate_RejectsSingleNodeCycle(t *testing.T) {
	phases := []phasegraph.PhaseSpec{
		{ID: "a", Requires: []phasegraph.PhaseID{"a"}, Run: noopRun},
	}
	_, err := phasegraph.ValidatePhaseGraph(phases)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var pgErr phasegraph.PhaseGraphError
	if !errors.As(err, &pgErr) {
		t.Fatalf("expected PhaseGraphError, got %T: %v", err, err)
	}
	if pgErr.Kind != "cycle" {
		t.Errorf("Kind = %q, want cycle", pgErr.Kind)
	}
	found := false
	for _, n := range pgErr.Cycle {
		if n == "a" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Cycle = %v, want to contain 'a'", pgErr.Cycle)
	}
}

// TestValidate_RejectsMultiNodeCycle: A→B→C→A.
func TestValidate_RejectsMultiNodeCycle(t *testing.T) {
	phases := []phasegraph.PhaseSpec{
		{ID: "A", Requires: []phasegraph.PhaseID{"C"}, Run: noopRun},
		{ID: "B", Requires: []phasegraph.PhaseID{"A"}, Run: noopRun},
		{ID: "C", Requires: []phasegraph.PhaseID{"B"}, Run: noopRun},
	}
	_, err := phasegraph.ValidatePhaseGraph(phases)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var pgErr phasegraph.PhaseGraphError
	if !errors.As(err, &pgErr) {
		t.Fatalf("expected PhaseGraphError, got %T: %v", err, err)
	}
	if pgErr.Kind != "cycle" {
		t.Errorf("Kind = %q, want cycle", pgErr.Kind)
	}
	want := map[phasegraph.PhaseID]bool{"A": false, "B": false, "C": false}
	for _, n := range pgErr.Cycle {
		if _, ok := want[n]; ok {
			want[n] = true
		}
	}
	for id, seen := range want {
		if !seen {
			t.Errorf("Cycle %v missing %q", pgErr.Cycle, id)
		}
	}
}

// TestRunPhaseGraph_HappyPath: 3-phase chain runs in order; outputs map keyed
// by PhaseID; shutdown called in reverse on completion.
func TestRunPhaseGraph_HappyPath(t *testing.T) {
	var runOrder []phasegraph.PhaseID
	mkRun := func(id phasegraph.PhaseID) phasegraph.PhaseRunFunc {
		return func(_ context.Context, _ phasegraph.PhaseDeps) (phasegraph.PhaseOutput, error) {
			runOrder = append(runOrder, id)
			return string("out:" + id), nil
		}
	}
	phases := []phasegraph.PhaseSpec{
		{ID: "A", Run: mkRun("A")},
		{ID: "B", Requires: []phasegraph.PhaseID{"A"}, Run: mkRun("B")},
		{ID: "C", Requires: []phasegraph.PhaseID{"B"}, Run: mkRun("C")},
	}
	g, err := phasegraph.ValidatePhaseGraph(phases)
	if err != nil {
		t.Fatalf("ValidatePhaseGraph: %v", err)
	}
	res, err := phasegraph.RunPhaseGraph(context.Background(), g)
	if err != nil {
		t.Fatalf("RunPhaseGraph: %v", err)
	}
	if res.Failed != "" {
		t.Errorf("Failed = %q, want empty", res.Failed)
	}
	wantRun := []phasegraph.PhaseID{"A", "B", "C"}
	if len(runOrder) != 3 {
		t.Fatalf("runOrder len = %d, want 3", len(runOrder))
	}
	for i, id := range runOrder {
		if id != wantRun[i] {
			t.Errorf("runOrder[%d] = %q, want %q", i, id, wantRun[i])
		}
	}
	for _, id := range wantRun {
		if _, ok := res.Outputs[id]; !ok {
			t.Errorf("Outputs missing %q", id)
		}
	}
	if _, ok := res.PhaseDurations["A"]; !ok {
		t.Errorf("PhaseDurations missing A")
	}
}

// TestRunPhaseGraph_PhaseError_TriggersShutdown: middle phase returns error →
// already-completed phases get Shutdown invoked; no later phases run.
func TestRunPhaseGraph_PhaseError_TriggersShutdown(t *testing.T) {
	var runOrder []phasegraph.PhaseID
	var shutdownOrder []phasegraph.PhaseID
	mkRun := func(id phasegraph.PhaseID, fail bool) phasegraph.PhaseRunFunc {
		return func(_ context.Context, _ phasegraph.PhaseDeps) (phasegraph.PhaseOutput, error) {
			runOrder = append(runOrder, id)
			if fail {
				return nil, errors.New("boom")
			}
			return string("out:" + id), nil
		}
	}
	mkShutdown := func(id phasegraph.PhaseID) phasegraph.PhaseShutdownFunc {
		return func(_ context.Context, _ phasegraph.PhaseOutput) error {
			shutdownOrder = append(shutdownOrder, id)
			return nil
		}
	}
	phases := []phasegraph.PhaseSpec{
		{ID: "A", Run: mkRun("A", false), Shutdown: mkShutdown("A")},
		{ID: "B", Requires: []phasegraph.PhaseID{"A"}, Run: mkRun("B", true), Shutdown: mkShutdown("B")},
		{ID: "C", Requires: []phasegraph.PhaseID{"B"}, Run: mkRun("C", false), Shutdown: mkShutdown("C")},
	}
	g, err := phasegraph.ValidatePhaseGraph(phases)
	if err != nil {
		t.Fatalf("ValidatePhaseGraph: %v", err)
	}
	res, err := phasegraph.RunPhaseGraph(context.Background(), g)
	if err == nil {
		t.Fatal("expected error from RunPhaseGraph, got nil")
	}
	if res == nil {
		t.Fatal("expected non-nil result")
	}
	if res.Failed != "B" {
		t.Errorf("Failed = %q, want B", res.Failed)
	}
	// runOrder must NOT contain C.
	for _, id := range runOrder {
		if id == "C" {
			t.Errorf("runOrder contained C after B failed: %v", runOrder)
		}
	}
	// Shutdown must have been invoked for A only (B's Run errored before completing,
	// so its output is not in the outputs map; C never ran).
	foundA := false
	for _, id := range shutdownOrder {
		if id == "A" {
			foundA = true
		}
		if id == "C" {
			t.Errorf("Shutdown invoked for C, which never ran: %v", shutdownOrder)
		}
	}
	if !foundA {
		t.Errorf("Shutdown not invoked for completed phase A: %v", shutdownOrder)
	}
}

// TestWriteDOT_EmitsDigraph: WriteDOT writes a digraph header and one edge per
// Requires entry.
func TestWriteDOT_EmitsDigraph(t *testing.T) {
	phases := []phasegraph.PhaseSpec{
		{ID: "A", Run: noopRun},
		{ID: "B", Requires: []phasegraph.PhaseID{"A"}, Run: noopRun},
	}
	var sb strings.Builder
	if err := phasegraph.WriteDOT(&sb, phases); err != nil {
		t.Fatalf("WriteDOT: %v", err)
	}
	out := sb.String()
	if !strings.Contains(out, "digraph") {
		t.Errorf("output missing 'digraph' header: %q", out)
	}
	if !strings.Contains(out, "\"B\"") || !strings.Contains(out, "\"A\"") {
		t.Errorf("output missing phase node references: %q", out)
	}
	// One edge for B → A.
	if !strings.Contains(out, "->") {
		t.Errorf("output missing edge: %q", out)
	}
}

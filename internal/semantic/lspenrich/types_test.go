package lspenrich_test

import (
	"context"
	"testing"

	"github.com/agenthands/helix/internal/semantic/lspenrich"
	"github.com/agenthands/helix/internal/semantic/store"
)

// T4-1: Outcome is a typed string — assigning a constant compiles.
func TestTypes_OutcomeIsTypedString(t *testing.T) {
	var o lspenrich.Outcome = lspenrich.OutcomeApplied
	if string(o) != "applied" {
		t.Errorf("OutcomeApplied: got %q, want applied", string(o))
	}
}

// T4-2: All 5 closed-enum constants are distinct strings with the spec values.
func TestTypes_OutcomeConstantsDistinctAndExpected(t *testing.T) {
	want := map[lspenrich.Outcome]string{
		lspenrich.OutcomeApplied:           "applied",
		lspenrich.OutcomePartialBudget:     "partial_budget",
		lspenrich.OutcomePartialPreempted:  "partial_preempted",
		lspenrich.OutcomePartialLSPUnavail: "partial_lsp_unavailable",
		lspenrich.OutcomeDropped:           "dropped",
	}
	seen := make(map[string]lspenrich.Outcome)
	for k, v := range want {
		if string(k) != v {
			t.Errorf("Outcome %v: string %q, want %q", k, string(k), v)
		}
		if existing, dup := seen[v]; dup && existing != k {
			t.Errorf("duplicate value %q for %v and %v", v, existing, k)
		}
		seen[v] = k
	}
	if len(seen) != 5 {
		t.Errorf("distinct outcome strings: got %d, want 5", len(seen))
	}
}

// T4-3: MetricsSink is satisfied by a hand-written 5-method struct.
type metricsImpl struct {
	totalCalls       int
	durationCalls    int
	errorCalls       int
	laneDepthCalls   int
	bulkSuppCalls    int
}

func (m *metricsImpl) LSPEnrichmentTotal(string, string)        { m.totalCalls++ }
func (m *metricsImpl) LSPEnrichmentDuration(string, float64)    { m.durationCalls++ }
func (m *metricsImpl) LSPEnrichmentErrors(string, string)       { m.errorCalls++ }
func (m *metricsImpl) LSPEnrichmentLaneDepth(string, int)       { m.laneDepthCalls++ }
func (m *metricsImpl) LSPEnrichmentBulkSuppressed(int)          { m.bulkSuppCalls++ }

func TestTypes_MetricsSinkInterfaceShape(t *testing.T) {
	var sink lspenrich.MetricsSink = &metricsImpl{}
	sink.LSPEnrichmentTotal("go", "applied")
	sink.LSPEnrichmentDuration("go", 1.5)
	sink.LSPEnrichmentErrors("go", "timeout")
	sink.LSPEnrichmentLaneDepth("high", 5)
	sink.LSPEnrichmentBulkSuppressed(250)

	impl := sink.(*metricsImpl)
	if impl.totalCalls != 1 || impl.durationCalls != 1 || impl.errorCalls != 1 ||
		impl.laneDepthCalls != 1 || impl.bulkSuppCalls != 1 {
		t.Errorf("call counts: total=%d duration=%d errors=%d laneDepth=%d bulkSupp=%d (each want 1)",
			impl.totalCalls, impl.durationCalls, impl.errorCalls,
			impl.laneDepthCalls, impl.bulkSuppCalls)
	}
}

// T4-4: OverlayStore has exactly one method (BeginOverlayTx). A test stub
// implementing that one method satisfies the interface.
type overlayStoreStub struct {
	beginCalls int
}

func (s *overlayStoreStub) BeginOverlayTx(_ context.Context, _ string) (*store.OverlayTx, error) {
	s.beginCalls++
	return nil, nil // tests below only verify the interface shape
}

func TestTypes_OverlayStoreInterfaceShape(t *testing.T) {
	var os lspenrich.OverlayStore = &overlayStoreStub{}
	tx, err := os.BeginOverlayTx(context.Background(), "ws1")
	if err != nil {
		t.Fatalf("BeginOverlayTx: %v", err)
	}
	if tx != nil {
		t.Errorf("stub returned non-nil tx; want nil for shape-only test")
	}
	stub := os.(*overlayStoreStub)
	if stub.beginCalls != 1 {
		t.Errorf("BeginOverlayTx call count: got %d, want 1", stub.beginCalls)
	}
}

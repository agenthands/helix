package pipelines_test

import (
	"testing"

	"github.com/agenthands/helix/internal/phasegraph"
	"github.com/agenthands/helix/internal/phasegraph/pipelines"
)

// TestSemanticIndexPipelineValidates: pipelines.SemanticIndexPhases passes ValidatePhaseGraph.
func TestSemanticIndexPipelineValidates(t *testing.T) {
	g, err := phasegraph.ValidatePhaseGraph(pipelines.SemanticIndexPhases)
	if err != nil {
		t.Fatalf("SemanticIndexPhases failed validation: %v", err)
	}
	if len(g.Order) != len(pipelines.SemanticIndexPhases) {
		t.Errorf("Order len = %d, want %d", len(g.Order), len(pipelines.SemanticIndexPhases))
	}
}

// TestLiveUpdatePipelineValidates: pipelines.LiveUpdatePhases passes.
func TestLiveUpdatePipelineValidates(t *testing.T) {
	g, err := phasegraph.ValidatePhaseGraph(pipelines.LiveUpdatePhases)
	if err != nil {
		t.Fatalf("LiveUpdatePhases failed validation: %v", err)
	}
	if len(g.Order) != len(pipelines.LiveUpdatePhases) {
		t.Errorf("Order len = %d, want %d", len(g.Order), len(pipelines.LiveUpdatePhases))
	}
}

// TestEvalPipelineValidates: pipelines.EvalPhases passes.
func TestEvalPipelineValidates(t *testing.T) {
	g, err := phasegraph.ValidatePhaseGraph(pipelines.EvalPhases)
	if err != nil {
		t.Fatalf("EvalPhases failed validation: %v", err)
	}
	if len(g.Order) != len(pipelines.EvalPhases) {
		t.Errorf("Order len = %d, want %d", len(g.Order), len(pipelines.EvalPhases))
	}
}

// TestPipelineCount_MatchesSpec: SemanticIndexPhases has 12 entries (SPEC §39.5),
// LiveUpdatePhases has 9 (SPEC §39.6), EvalPhases has 10 (SPEC §39.7).
func TestPipelineCount_MatchesSpec(t *testing.T) {
	if got, want := len(pipelines.SemanticIndexPhases), 12; got != want {
		t.Errorf("SemanticIndexPhases len = %d, want %d (SPEC §39.5)", got, want)
	}
	if got, want := len(pipelines.LiveUpdatePhases), 9; got != want {
		t.Errorf("LiveUpdatePhases len = %d, want %d (SPEC §39.6)", got, want)
	}
	if got, want := len(pipelines.EvalPhases), 10; got != want {
		t.Errorf("EvalPhases len = %d, want %d (SPEC §39.7)", got, want)
	}
}

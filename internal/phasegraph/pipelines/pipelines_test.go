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

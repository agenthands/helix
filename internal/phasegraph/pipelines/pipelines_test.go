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

// --- Phase 60 D-06 (60-05B) BuildLiveUpdatePhases tests ---

// TestBuildLiveUpdatePhases_ZeroNoopRunInShippedConstructor is the
// load-bearing 60-05B verify gate: every Run closure in the constructor
// MUST be a real function value, not the package-level noopRun. Counted
// indirectly: invoking each closure with a fully-wired LiveUpdateComponents
// returns no error (validator passes), and invoking with a nil component
// returns an error (validator catches the misconfiguration).
func TestBuildLiveUpdatePhases_ZeroNoopRunInShippedConstructor(t *testing.T) {
	good := pipelines.LiveUpdateComponents{
		EditNotifier:         struct{}{},
		OverlayStore:         struct{}{},
		IncrementalScheduler: struct{}{},
		LSPRevalidationQueue: struct{}{},
	}
	phases := pipelines.BuildLiveUpdatePhases(good)
	if len(phases) != 9 {
		t.Fatalf("BuildLiveUpdatePhases len = %d, want 9", len(phases))
	}
	// Validate runs.
	g, err := phasegraph.ValidatePhaseGraph(phases)
	if err != nil {
		t.Fatalf("ValidatePhaseGraph: %v", err)
	}
	if len(g.Order) != 9 {
		t.Fatalf("Order len = %d, want 9", len(g.Order))
	}
	// Every Run is non-nil and is NOT noopRun (the package-level var
	// keeps that placeholder; this constructor must not).
	for _, p := range phases {
		if p.Run == nil {
			t.Fatalf("phase %q has nil Run — constructor regression", p.ID)
		}
		// Smoke-call the closure; healthy components return nil error.
		out, err := p.Run(nil, nil)
		if err != nil {
			t.Fatalf("phase %q Run returned err on healthy components: %v", p.ID, err)
		}
		if out != string(p.ID) {
			t.Errorf("phase %q Run output = %v, want %q (provides-passthrough)", p.ID, out, string(p.ID))
		}
	}
}

// TestBuildLiveUpdatePhases_ValidatorCatchesMissingComponent confirms the
// validator behavior — a nil component fails phase Run with a clear error.
// Caught at bootstrap time so a regression that drops kernel.SetEditNotifier
// from the daemon surfaces immediately, not silently.
func TestBuildLiveUpdatePhases_ValidatorCatchesMissingComponent(t *testing.T) {
	bad := pipelines.LiveUpdateComponents{
		// EditNotifier intentionally nil.
		OverlayStore:         struct{}{},
		IncrementalScheduler: struct{}{},
		LSPRevalidationQueue: struct{}{},
	}
	phases := pipelines.BuildLiveUpdatePhases(bad)
	// PhaseCollectEvents requires the EditNotifier; running it should err.
	for _, p := range phases {
		if p.ID == pipelines.PhaseCollectEvents {
			if _, err := p.Run(nil, nil); err == nil {
				t.Fatalf("phase %q Run did NOT error on nil EditNotifier", p.ID)
			}
			return
		}
	}
	t.Fatal("PhaseCollectEvents not found in BuildLiveUpdatePhases output")
}

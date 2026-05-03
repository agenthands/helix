package phasegraph

import (
	"context"
	"time"
)

// RunPhaseGraph executes a validated [PhaseGraph] in topological order. On any
// Run or Validate error it invokes Shutdown for every phase whose output was
// recorded so far (in reverse-topological order) and returns the original
// error wrapped in the result.
//
// The contract is verbatim from SPEC §39.8.
//
// Per-phase deadlines are NOT enforced here — callers wrap ctx with
// [context.WithTimeout] before invoking RunPhaseGraph or wrap individual
// PhaseRunFunc closures with their own budget. See package doc threat note.
func RunPhaseGraph(ctx context.Context, graph *PhaseGraph) (*PhaseGraphResult, error) {
	result := &PhaseGraphResult{
		PhaseDurations: map[PhaseID]time.Duration{},
		Outputs:        map[PhaseID]PhaseOutput{},
	}

	for _, phase := range graph.Order {
		deps := buildPhaseDeps(phase, result.Outputs)
		start := time.Now()
		out, err := phase.Run(ctx, deps)
		duration := time.Since(start)

		result.Record(phase.ID, duration, err)
		if err != nil {
			ShutdownCompleted(ctx, graph, result.Outputs)
			return result, err
		}

		if phase.Validate != nil {
			if vErr := phase.Validate(out); vErr != nil {
				result.Failed = phase.ID
				ShutdownCompleted(ctx, graph, result.Outputs)
				return result, vErr
			}
		}

		result.Outputs[phase.ID] = out
	}

	return result, nil
}

// buildPhaseDeps returns a fresh PhaseDeps map containing only the outputs
// listed in phase.Requires. Phases see exactly what they declared — nothing
// more — so a phase that forgot to declare a dep cannot accidentally read it.
func buildPhaseDeps(phase PhaseSpec, outputs map[PhaseID]PhaseOutput) PhaseDeps {
	deps := make(PhaseDeps, len(phase.Requires))
	for _, dep := range phase.Requires {
		if out, ok := outputs[dep]; ok {
			deps[dep] = out
		}
	}
	return deps
}

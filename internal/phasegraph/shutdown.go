package phasegraph

import (
	"context"
	"errors"
)

// ShutdownCompleted invokes Shutdown on every phase whose output was recorded
// in `outputs`, walking [PhaseGraph.ShutdownOrder] (reverse topological order)
// and skipping phases that never completed. Errors from individual Shutdown
// calls are aggregated via [errors.Join] and returned together. Shutdown
// itself never panics — a missing Shutdown func is a no-op.
//
// Public so callers (e.g. test helpers, P02 daemon shutdown) can reuse the
// same teardown sequence outside of [RunPhaseGraph].
func ShutdownCompleted(ctx context.Context, graph *PhaseGraph, outputs map[PhaseID]PhaseOutput) error {
	if graph == nil {
		return nil
	}
	var errs []error
	for _, phase := range graph.ShutdownOrder {
		out, ok := outputs[phase.ID]
		if !ok {
			continue
		}
		if phase.Shutdown == nil {
			continue
		}
		if err := phase.Shutdown(ctx, out); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return errors.Join(errs...)
}

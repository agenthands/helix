// Package phasegraph is a stdlib-only DAG orchestrator for ordered, validated
// pipelines (daemon bootstrap, semantic indexing, live updates, evaluation).
//
// The package validates a slice of [PhaseSpec] against three correctness rules
// (no duplicate IDs, no missing dependencies, no cycles), produces a
// topologically-sorted [PhaseGraph], and runs phases in dependency order with
// reverse-topological shutdown.
//
// SPEC §39 owns the contract; this package is the v1.10 reference implementation.
//
// # Threat model
//
//   - Phase Run functions are caller-supplied closures. Per-phase timeouts are
//     the caller's responsibility (wrap with [context.WithTimeout]). The library
//     does not enforce per-phase budgets — it cannot impose policy on
//     downstream consumers.
//   - DOT output (see [WriteDOT]) contains only declared phase IDs and
//     dependency edges. Both are static, developer-chosen constants. No
//     file paths, env vars, or runtime data leak.
//   - There is no public RunPhases([]PhaseSpec) overload: [RunPhaseGraph] only
//     accepts a *PhaseGraph obtainable from a successful [ValidatePhaseGraph]
//     call. This makes validate-before-run an API-shape invariant rather than
//     a documentation note.
package phasegraph

import (
	"context"
	"time"
)

// PhaseID is the stable string identifier for a phase. The empty PhaseID is
// reserved as the "no-phase" sentinel (e.g., [PhaseGraphResult.Failed] is empty
// on success).
type PhaseID string

// PhaseOutput is the value a phase emits and that downstream phases consume.
// It is intentionally an empty interface: phases type-assert at the read site
// rather than threading a generic parameter through the entire library
// (CONTEXT.md §"Claude's Discretion").
type PhaseOutput interface{}

// PhaseDeps is the dependency map passed to [PhaseRunFunc]. Keys are the
// PhaseIDs declared in [PhaseSpec.Requires]; values are the [PhaseOutput] of
// the phase with that ID.
type PhaseDeps map[PhaseID]any

// PhaseRunFunc is the executable body of a phase. ctx carries cancellation;
// deps holds outputs from already-completed dependency phases.
type PhaseRunFunc func(ctx context.Context, deps PhaseDeps) (PhaseOutput, error)

// PhaseValidateFunc lets a phase optionally validate its own output before
// downstream phases see it. A non-nil error aborts the run and triggers
// shutdown of completed phases.
type PhaseValidateFunc func(output PhaseOutput) error

// PhaseShutdownFunc is invoked in reverse-topological order on every phase
// whose output was recorded in the outputs map. It runs both on a clean
// teardown and after a Run/Validate failure mid-pipeline.
type PhaseShutdownFunc func(ctx context.Context, output PhaseOutput) error

// PhaseSpec is the static declaration of one phase. The contract is verbatim
// from SPEC §39.2 — fields are intentionally exported so callers compose
// pipelines as ordinary slice literals.
type PhaseSpec struct {
	ID       PhaseID
	Requires []PhaseID
	Provides []string
	Run      PhaseRunFunc
	Validate PhaseValidateFunc
	Shutdown PhaseShutdownFunc
}

// PhaseGraph is the validated, topologically-sorted result of a successful
// [ValidatePhaseGraph] call. ShutdownOrder is always Reverse(Order).
type PhaseGraph struct {
	Order         []PhaseSpec
	ShutdownOrder []PhaseSpec
}

// PhaseGraphResult is what [RunPhaseGraph] returns. Failed is empty on success
// and set to the offending PhaseID on failure. PhaseDurations records every
// phase whose Run was invoked, including the failing one.
type PhaseGraphResult struct {
	PhaseDurations map[PhaseID]time.Duration
	Outputs        map[PhaseID]PhaseOutput
	Failed         PhaseID
}

// Record appends one phase's wall-clock duration to PhaseDurations and, if err
// is non-nil, marks the result as failed at that phase. It allocates the
// PhaseDurations map lazily so a zero-value PhaseGraphResult is usable.
func (r *PhaseGraphResult) Record(id PhaseID, d time.Duration, err error) {
	if r.PhaseDurations == nil {
		r.PhaseDurations = map[PhaseID]time.Duration{}
	}
	r.PhaseDurations[id] = d
	if err != nil {
		r.Failed = id
	}
}

package phasegraph

import (
	"fmt"
	"strings"
)

// PhaseGraphError is the typed error returned by [ValidatePhaseGraph] on every
// failure mode. Callers should use [errors.As] to discriminate — never assert
// on Error() string contents.
//
// Field population by Kind:
//
//	"duplicate_phase"    → Phase
//	"missing_dependency" → Missing
//	"cycle"              → Cycle (offending node sequence; len ≥ 1)
type PhaseGraphError struct {
	Kind    string
	Phase   PhaseID
	Missing []PhaseID
	Cycle   []PhaseID
}

// Error formats the typed fields into a human-readable message. Format is
// stable but not part of the public API — tests assert on the typed fields.
func (e PhaseGraphError) Error() string {
	switch e.Kind {
	case "duplicate_phase":
		return fmt.Sprintf("phasegraph: duplicate phase id %q", e.Phase)
	case "missing_dependency":
		return fmt.Sprintf("phasegraph: missing dependencies: %s", joinIDs(e.Missing))
	case "cycle":
		return fmt.Sprintf("phasegraph: cycle detected: %s", joinIDs(e.Cycle))
	default:
		return fmt.Sprintf("phasegraph: %s", e.Kind)
	}
}

// joinIDs renders a slice of PhaseIDs as a comma-separated, quoted list.
func joinIDs(ids []PhaseID) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = fmt.Sprintf("%q", string(id))
	}
	return strings.Join(parts, ", ")
}

// ValidatePhaseGraph is the SPEC §39.3 entrypoint. It runs three correctness
// checks (no duplicate IDs, no missing deps, no cycles) and returns a
// topologically-sorted [PhaseGraph] on success.
//
// On failure the returned error is always a [PhaseGraphError] — use
// [errors.As] to extract typed fields.
func ValidatePhaseGraph(phases []PhaseSpec) (*PhaseGraph, error) {
	g := buildPhaseGraph(phases)
	if dup := g.findDuplicateIDs(); dup != nil {
		return nil, PhaseGraphError{Kind: "duplicate_phase", Phase: *dup}
	}
	if missing := g.findMissingDependencies(); len(missing) > 0 {
		return nil, PhaseGraphError{Kind: "missing_dependency", Missing: missing}
	}
	if cycle := g.findCycle(); len(cycle) > 0 {
		return nil, PhaseGraphError{Kind: "cycle", Cycle: cycle}
	}
	order := g.kahnSort()
	return &PhaseGraph{Order: order, ShutdownOrder: reverse(order)}, nil
}

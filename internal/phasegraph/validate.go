package phasegraph

import (
	"fmt"
	"io"
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
//
// For a variant that emits a Graphviz `.dot` file on validation failure, see
// [ValidatePhaseGraphWithOptions].
func ValidatePhaseGraph(phases []PhaseSpec) (*PhaseGraph, error) {
	return ValidatePhaseGraphWithOptions(phases, ValidateOptions{})
}

// ValidateOptions is the optional configuration accepted by
// [ValidatePhaseGraphWithOptions]. Each field is independently optional —
// the zero value is equivalent to calling [ValidatePhaseGraph].
type ValidateOptions struct {
	// DOTSink, when non-nil, receives a Graphviz `digraph` rendering of
	// `phases` whenever validation fails. SPEC §39.9 calls out the
	// `.helix/debug/phasegraph-*.dot` path convention; the writer choice is
	// the caller's. On success nothing is written.
	DOTSink io.Writer
}

// ValidatePhaseGraphWithOptions is the option-bearing variant of
// [ValidatePhaseGraph]. The original signature is preserved so existing
// callers do not need to adopt the options struct.
//
// When opts.DOTSink is non-nil and validation fails, a Graphviz digraph is
// written to the sink before the error is returned. WriteDOT errors are
// intentionally suppressed: they are debug-only output, and surfacing them
// would mask the original PhaseGraphError that the caller cares about.
func ValidatePhaseGraphWithOptions(phases []PhaseSpec, opts ValidateOptions) (*PhaseGraph, error) {
	g := buildPhaseGraph(phases)
	if dup := g.findDuplicateIDs(); dup != nil {
		err := PhaseGraphError{Kind: "duplicate_phase", Phase: *dup}
		writeDOTBestEffort(opts.DOTSink, phases)
		return nil, err
	}
	if missing := g.findMissingDependencies(); len(missing) > 0 {
		err := PhaseGraphError{Kind: "missing_dependency", Missing: missing}
		writeDOTBestEffort(opts.DOTSink, phases)
		return nil, err
	}
	if cycle := g.findCycle(); len(cycle) > 0 {
		err := PhaseGraphError{Kind: "cycle", Cycle: cycle}
		writeDOTBestEffort(opts.DOTSink, phases)
		return nil, err
	}
	order := g.kahnSort()
	return &PhaseGraph{Order: order, ShutdownOrder: reverse(order)}, nil
}

// writeDOTBestEffort emits a digraph to sink (if non-nil) and discards any
// write error. This is debug output: surfacing a sink error here would
// shadow the validation error the caller actually wants.
func writeDOTBestEffort(sink io.Writer, phases []PhaseSpec) {
	if sink == nil {
		return
	}
	_ = WriteDOT(sink, phases)
}

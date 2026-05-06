package lspenrich

import (
	"sync"
	"sync/atomic"
)

// Status is a read-only snapshot of the enrichment Manager's state.
// Phase 61 D-09: ships the accessor; Phase 65 wires it into the get_health
// MCP tool.  All fields are values (not pointers) — Manager.Status() returns
// a defensive copy so consumers cannot mutate manager state.
//
// W11 — LastErrorPerLanguage is map[string]string for v1: the value is
// err.Error() of the most recent error class (e.g. "circuit_open",
// "timeout").  Phase 65 may evolve to a structured error type without
// breaking this surface — consumers MUST treat the string as opaque per
// (lang, recency) and not parse it.
type Status struct {
	// LaneDepths is the per-lane queue length at snapshot time.  Always
	// populated for both LaneHigh and LaneBackground (zero if unused).
	LaneDepths map[Lane]int

	// InFlight reserved for future use (per-job in-flight counter for
	// Phase 65 get_health visualizations).  Phase 61 v1 leaves this at
	// zero — the worker does not yet maintain a counter.
	InFlight int

	// FilesEnriched counts cascade outcomes that were applied (or
	// applied-with-caveat per W7 partial_budget).  Note: partial_budget
	// bumps BOTH FilesEnriched and FilesPending, so consumers can compute
	// fully_enriched := FilesEnriched - FilesPending.
	FilesEnriched uint64

	// FilesDropped counts cascade outcomes where no facts were committed
	// (OutcomeDropped).
	FilesDropped uint64

	// FilesPreempted counts cascade outcomes that yielded between steps
	// (OutcomePartialPreempted).
	FilesPreempted uint64

	// FilesPending is the count of partial outcomes that left the file
	// pending re-enrichment: partial_budget (W7), partial_preempted, and
	// partial_lsp_unavailable.
	FilesPending uint64

	// LastErrorPerLanguage records the most recent error classification
	// string per language.  Empty when the language has had no errors.
	LastErrorPerLanguage map[string]string
}

// statusTracker is the package-internal mutable state behind Status().  All
// counter access is via atomics; the per-language error map uses a
// sync.RWMutex so Status() reads can run concurrently.
type statusTracker struct {
	filesEnriched  atomic.Uint64
	filesDropped   atomic.Uint64
	filesPreempted atomic.Uint64
	filesPending   atomic.Uint64
	inFlight       atomic.Int64

	mu               sync.RWMutex
	lastErrorPerLang map[string]string
}

func newStatusTracker() *statusTracker {
	return &statusTracker{lastErrorPerLang: make(map[string]string)}
}

// recordOutcome bumps the counter(s) corresponding to the cascade outcome.
// Per W7, OutcomePartialBudget is "applied with caveat" — it bumps BOTH
// filesEnriched AND filesPending so get_health consumers can compute
// fully_enriched = enriched - pending.
func (s *statusTracker) recordOutcome(outcome Outcome) {
	switch outcome {
	case OutcomeApplied:
		s.filesEnriched.Add(1)
	case OutcomeDropped:
		s.filesDropped.Add(1)
	case OutcomePartialPreempted:
		s.filesPreempted.Add(1)
		s.filesPending.Add(1)
	case OutcomePartialBudget:
		// W7 — partial_budget is "applied with caveat": the file IS
		// queryable (partial facts committed) BUT also pending re-
		// enrichment.  Surface both counters so get_health consumers
		// can compute fully-enriched = enriched - pending.
		s.filesEnriched.Add(1)
		s.filesPending.Add(1)
	case OutcomePartialLSPUnavail:
		s.filesPending.Add(1)
	}
}

// recordError stores the most recent error string per language.  W11 — the
// stored value is opaque to callers (the .Error() string of whatever the
// worker classified) and may evolve to a structured type in Phase 65 without
// breaking the Status surface.
func (s *statusTracker) recordError(lang, msg string) {
	if lang == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastErrorPerLang[lang] = msg
}

// snapshot returns a defensive copy of the tracker's state.  laneDepths is
// supplied by the caller (the Manager — only it knows the queue).
func (s *statusTracker) snapshot(laneDepths map[Lane]int) Status {
	s.mu.RLock()
	lpe := make(map[string]string, len(s.lastErrorPerLang))
	for k, v := range s.lastErrorPerLang {
		lpe[k] = v
	}
	s.mu.RUnlock()
	return Status{
		LaneDepths:           laneDepths,
		InFlight:             int(s.inFlight.Load()),
		FilesEnriched:        s.filesEnriched.Load(),
		FilesDropped:         s.filesDropped.Load(),
		FilesPreempted:       s.filesPreempted.Load(),
		FilesPending:         s.filesPending.Load(),
		LastErrorPerLanguage: lpe,
	}
}

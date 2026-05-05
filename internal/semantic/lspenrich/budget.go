package lspenrich

import (
	"fmt"
	"time"

	"github.com/agenthands/helix/internal/semantic"
)

// Budget is the per-file enrichment budget — Phase 61 D-07 / acceptance #8 /
// SPEC-DRAFT.md §14.2. A Budget value carries:
//
//   - perFileDeadline   — derived from cfg.TimeoutPerFile (added to `now`)
//   - totalDeadline     — derived from cfg.TimeoutTotal (added to `workerStart`)
//   - symbolsLeft       — counts down on each ConsumeSymbol call (cap=cfg.MaxSymbolsPerFile)
//   - refsPerSymbol     — verbatim cfg.MaxReferencesPerSymbol; ConsumeReferences
//                         rejects n>refsPerSymbol on every call (per-symbol gate)
//   - refsLeftFile      — counts down by n on each ConsumeReferences call
//                         (cap=cfg.MaxReferencesPerFile, per-file gate)
//   - callDepth         — verbatim cfg.MaxCallHierarchyDepth (read by cascade)
//   - typeDepth         — verbatim cfg.MaxTypeHierarchyDepth (read by cascade)
//
// Both deadlines are checked by HasRemainingTime; whichever expires first
// causes the cascade to abort with OutcomePartialBudget. ConsumeSymbol /
// ConsumeReferences guard against negative-counter underflow (the
// post-exhaustion call simply returns false).
//
// Construction: NewBudget(now, workerStart, cfg). `now` is the per-file
// reference time (typically the moment the cascade picks the file up);
// `workerStart` is the moment the worker goroutine began (the total-budget
// anchor — the budget burns down at the rate of cfg.TimeoutTotal across the
// whole worker lifetime, NOT per-file).
type Budget struct {
	perFileDeadline time.Time
	totalDeadline   time.Time
	symbolsLeft     int
	refsPerSymbol   int
	refsLeftFile    int
	callDepth       int
	typeDepth       int
}

// NewBudget parses TimeoutPerFile + TimeoutTotal from cfg and snapshots the
// per-file caps. Both ParseDuration calls run before any field is assigned;
// either failure returns a zero-valued Budget plus a wrapped error. Callers
// who want graceful degradation (Worker.processOne) record OutcomeDropped
// when err != nil — the file is dropped without a partial_reason stamp.
func NewBudget(now, workerStart time.Time, cfg semantic.LSPEnrichmentConfig) (Budget, error) {
	perFile, err := time.ParseDuration(cfg.TimeoutPerFile)
	if err != nil {
		return Budget{}, fmt.Errorf("NewBudget: parse TimeoutPerFile %q: %w", cfg.TimeoutPerFile, err)
	}
	total, err := time.ParseDuration(cfg.TimeoutTotal)
	if err != nil {
		return Budget{}, fmt.Errorf("NewBudget: parse TimeoutTotal %q: %w", cfg.TimeoutTotal, err)
	}
	return Budget{
		perFileDeadline: now.Add(perFile),
		totalDeadline:   workerStart.Add(total),
		symbolsLeft:     cfg.MaxSymbolsPerFile,
		refsPerSymbol:   cfg.MaxReferencesPerSymbol,
		refsLeftFile:    cfg.MaxReferencesPerFile,
		callDepth:       cfg.MaxCallHierarchyDepth,
		typeDepth:       cfg.MaxTypeHierarchyDepth,
	}, nil
}

// HasRemainingTime returns true iff `now` is strictly before BOTH the per-
// file deadline AND the total deadline. A zero-valued Budget (returned on
// NewBudget error) has a zero-time perFileDeadline; HasRemainingTime then
// returns false because now > zero-time for any non-zero `now`.
func (b *Budget) HasRemainingTime(now time.Time) bool {
	if b == nil {
		return false
	}
	if b.perFileDeadline.IsZero() || b.totalDeadline.IsZero() {
		return false
	}
	return now.Before(b.perFileDeadline) && now.Before(b.totalDeadline)
}

// ConsumeSymbol decrements the per-file symbol counter and reports whether
// the budget allowed the consumption (true on success; false if the cap
// is already exhausted). Post-exhaustion calls remain false — the counter
// does not go negative.
func (b *Budget) ConsumeSymbol() bool {
	if b == nil {
		return false
	}
	if b.symbolsLeft <= 0 {
		return false
	}
	b.symbolsLeft--
	return true
}

// ConsumeReferences validates n against BOTH the per-symbol cap (rejects
// any single n that exceeds MaxReferencesPerSymbol) AND the per-file
// running counter (rejects when remaining < n). On success decrements the
// per-file counter by n. Negative n is treated as a no-op success.
func (b *Budget) ConsumeReferences(n int) bool {
	if b == nil {
		return false
	}
	if n <= 0 {
		return true
	}
	if n > b.refsPerSymbol {
		return false
	}
	if n > b.refsLeftFile {
		return false
	}
	b.refsLeftFile -= n
	return true
}

// CallDepth returns MaxCallHierarchyDepth verbatim (cascade reads this when
// preparing callHierarchy/incomingCalls + outgoingCalls walks).
func (b *Budget) CallDepth() int {
	if b == nil {
		return 0
	}
	return b.callDepth
}

// TypeDepth returns MaxTypeHierarchyDepth verbatim (cascade reads this when
// preparing typeHierarchy/supertypes + subtypes walks).
func (b *Budget) TypeDepth() int {
	if b == nil {
		return 0
	}
	return b.typeDepth
}

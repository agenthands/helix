package lspenrich_test

import (
	"testing"
	"time"

	"github.com/agenthands/helix/internal/semantic"
	"github.com/agenthands/helix/internal/semantic/lspenrich"
)

// budgetCfg returns an LSPEnrichmentConfig with conservative test defaults.
// Individual tests override specific fields they exercise.
func budgetCfg() semantic.LSPEnrichmentConfig {
	return semantic.LSPEnrichmentConfig{
		Enabled:                true,
		TimeoutPerFile:         "5s",
		TimeoutTotal:           "120s",
		MaxSymbolsPerFile:      200,
		MaxReferencesPerSymbol: 1000,
		MaxReferencesPerFile:   5000,
		MaxCallHierarchyDepth:  2,
		MaxTypeHierarchyDepth:  2,
	}
}

// fixedNow is the deterministic reference time used by every Budget test;
// HasRemainingTime / ConsumeSymbol / ConsumeReferences are all derived from
// (now, totalNow, cfg) so tests do NOT need a real wall clock.
var fixedNow = time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC)

// TestBudget_B1_ParseTimeoutPerFile: NewBudget with TimeoutPerFile="5s" sets
// the per-file deadline to now+5s; an unparseable duration returns an error.
func TestBudget_B1_ParseTimeoutPerFile(t *testing.T) {
	cfg := budgetCfg()
	cfg.TimeoutPerFile = "5s"
	b, err := lspenrich.NewBudget(fixedNow, fixedNow, cfg)
	if err != nil {
		t.Fatalf("NewBudget(valid 5s): unexpected error: %v", err)
	}
	if !b.HasRemainingTime(fixedNow.Add(4 * time.Second)) {
		t.Errorf("HasRemainingTime(now+4s) = false; want true (5s budget)")
	}
	if b.HasRemainingTime(fixedNow.Add(6 * time.Second)) {
		t.Errorf("HasRemainingTime(now+6s) = true; want false (deadline expired)")
	}

	// Bad duration → error and zero-valued budget.
	cfg.TimeoutPerFile = "not-a-duration"
	bad, err := lspenrich.NewBudget(fixedNow, fixedNow, cfg)
	if err == nil {
		t.Fatal("NewBudget(garbage TimeoutPerFile): want error, got nil")
	}
	// HasRemainingTime on a zero-valued budget always returns false (no deadline set).
	if bad.HasRemainingTime(fixedNow) {
		t.Errorf("HasRemainingTime on bad-budget: got true, want false (zero-valued)")
	}
}

// TestBudget_B2_HasRemainingTime: per-file deadline boundary.
func TestBudget_B2_HasRemainingTime(t *testing.T) {
	cfg := budgetCfg()
	cfg.TimeoutPerFile = "5s"
	b, err := lspenrich.NewBudget(fixedNow, fixedNow, cfg)
	if err != nil {
		t.Fatalf("NewBudget: %v", err)
	}
	if !b.HasRemainingTime(fixedNow.Add(1 * time.Second)) {
		t.Errorf("HasRemainingTime(now+1s) = false; want true")
	}
	if b.HasRemainingTime(fixedNow.Add(6 * time.Second)) {
		t.Errorf("HasRemainingTime(now+6s) = true; want false")
	}
}

// TestBudget_B3_ConsumeSymbol: ConsumeSymbol returns true MaxSymbolsPerFile
// times then false on the (N+1)th call.
func TestBudget_B3_ConsumeSymbol(t *testing.T) {
	cfg := budgetCfg()
	cfg.MaxSymbolsPerFile = 200
	b, err := lspenrich.NewBudget(fixedNow, fixedNow, cfg)
	if err != nil {
		t.Fatalf("NewBudget: %v", err)
	}
	for i := 0; i < 200; i++ {
		if !b.ConsumeSymbol() {
			t.Fatalf("ConsumeSymbol #%d: false; want true (cap=200)", i+1)
		}
	}
	if b.ConsumeSymbol() {
		t.Error("ConsumeSymbol #201: true; want false (cap exhausted)")
	}
	// Repeated post-exhaustion calls must remain false (no negative-counter underflow).
	if b.ConsumeSymbol() {
		t.Error("ConsumeSymbol #202 (post-exhaustion): true; want false")
	}
}

// TestBudget_B4_ConsumeReferences: per-file cap enforced.
func TestBudget_B4_ConsumeReferences(t *testing.T) {
	cfg := budgetCfg()
	cfg.MaxReferencesPerFile = 5000
	cfg.MaxReferencesPerSymbol = 1000
	b, err := lspenrich.NewBudget(fixedNow, fixedNow, cfg)
	if err != nil {
		t.Fatalf("NewBudget: %v", err)
	}
	if !b.ConsumeReferences(50) {
		t.Error("ConsumeReferences(50): false; want true (well under per-file cap)")
	}
	if !b.ConsumeReferences(100) {
		t.Error("ConsumeReferences(100): false; want true (still under cap)")
	}
	// Over per-file cap → false, no negative-counter.
	if b.ConsumeReferences(99999) {
		t.Error("ConsumeReferences(99999): true; want false (exceeds per-file cap)")
	}
	// Per-symbol cap also enforced. After fresh budget, n>per-symbol returns false.
	b2, _ := lspenrich.NewBudget(fixedNow, fixedNow, cfg)
	if b2.ConsumeReferences(2000) {
		t.Error("ConsumeReferences(2000): true; want false (exceeds per-symbol cap of 1000)")
	}
}

// TestBudget_B5_TotalDeadlineDistinctFromPerFile: when total expired but
// per-file still has remaining time, HasRemainingTime returns false.
func TestBudget_B5_TotalDeadlineTakesPrecedence(t *testing.T) {
	cfg := budgetCfg()
	cfg.TimeoutPerFile = "5s"
	cfg.TimeoutTotal = "120s"

	// workerStart 119s before fixedNow → total deadline at fixedNow+1s.
	workerStart := fixedNow.Add(-119 * time.Second)
	b, err := lspenrich.NewBudget(fixedNow, workerStart, cfg)
	if err != nil {
		t.Fatalf("NewBudget: %v", err)
	}
	// At fixedNow+2s: per-file (5s) still has 3s remaining; total (1s) expired.
	if b.HasRemainingTime(fixedNow.Add(2 * time.Second)) {
		t.Error("HasRemainingTime: total deadline expired must take precedence over per-file remaining")
	}
	// At fixedNow+0.5s: total still has 0.5s; per-file 4.5s — both ok.
	if !b.HasRemainingTime(fixedNow.Add(500 * time.Millisecond)) {
		t.Error("HasRemainingTime(now+0.5s): false; want true (both deadlines have remaining)")
	}
}

// TestBudget_B6_HierarchyDepthAccessors: GetCallDepth + GetTypeDepth read
// the config values verbatim.
func TestBudget_B6_HierarchyDepthAccessors(t *testing.T) {
	cfg := budgetCfg()
	cfg.MaxCallHierarchyDepth = 3
	cfg.MaxTypeHierarchyDepth = 4
	b, err := lspenrich.NewBudget(fixedNow, fixedNow, cfg)
	if err != nil {
		t.Fatalf("NewBudget: %v", err)
	}
	if b.CallDepth() != 3 {
		t.Errorf("CallDepth: got %d, want 3", b.CallDepth())
	}
	if b.TypeDepth() != 4 {
		t.Errorf("TypeDepth: got %d, want 4", b.TypeDepth())
	}
}

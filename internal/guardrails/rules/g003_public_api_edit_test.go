package rules_test

import (
	"context"
	"testing"
	"time"

	"github.com/agenthands/helix/internal/guardrails"
	"github.com/agenthands/helix/internal/guardrails/rules"
	"github.com/agenthands/helix/internal/semantic"
	"github.com/agenthands/helix/internal/semantic/integ"
	"github.com/agenthands/helix/internal/workspace"
)

func makeSCWithLookup(t *testing.T, enforcement string, lookup integ.SemanticLookup) rules.SessionContext {
	t.Helper()
	return rules.SessionContext{
		Workspace:       workspace.WorkspaceKey{RepoRoot: "/repo"},
		Profile:         "claude-code",
		Lookup:          lookup,
		Store:           makeStore(t),
		Config:          semantic.GuardrailsConfig{Enforcement: enforcement},
		OutlineProvider: &fakeOutlineProvider{},
		GraphVersion:    1,
		Now:             time.Now(),
	}
}

func TestG003_Private_NoRefs_NotEntrypoint_Allows(t *testing.T) {
	lookup := &fakeVisLookup{vis: integ.VisPrivate, entryPointResult: false}
	sc := makeSCWithLookup(t, "enforce", lookup)
	args := rules.RuleArgs{
		Tool:            "replace_symbol_body",
		SymbolID:        "internalFn",
		SymbolName:      "internalFn",
		RefCount:        0,
		SignatureChange: false,
	}
	d := rules.EvaluateG003(context.Background(), args, sc)
	if d.Action != rules.Allow {
		t.Errorf("private, no refs, not entrypoint: expected Allow, got %v", d.Action)
	}
}

func TestG003_Exported_NoReceipt_Blocks(t *testing.T) {
	lookup := &fakeVisLookup{vis: integ.VisExported, entryPointResult: false}
	sc := makeSCWithLookup(t, "enforce", lookup)
	args := rules.RuleArgs{
		Tool:       "replace_symbol_body",
		SymbolID:   "HandleAuth",
		SymbolName: "HandleAuth",
	}
	d := rules.EvaluateG003(context.Background(), args, sc)
	if d.Action != rules.Block {
		t.Errorf("exported symbol, no receipt: expected Block, got %v", d.Action)
	}
	if d.Rule != "G-003" {
		t.Errorf("expected Rule=G-003, got %q", d.Rule)
	}
	// Must require impact_checked
	hasImpact := false
	for _, rr := range d.RequiredReceipts {
		if rr.Class == guardrails.ClassImpactChecked {
			hasImpact = true
		}
	}
	if !hasImpact {
		t.Errorf("expected impact_checked in RequiredReceipts, got %v", d.RequiredReceipts)
	}
	// Must suggest analyze_blast_radius
	found := false
	for _, s := range d.SuggestedTools {
		if s == "analyze_blast_radius" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected analyze_blast_radius in SuggestedTools, got %v", d.SuggestedTools)
	}
}

// TestG003_RejectsReferencesCheckedAlone is the key D-15 invariant: references_checked
// alone is NOT accepted for G-003 public-API edits; impact_checked is required.
func TestG003_RejectsReferencesCheckedAlone(t *testing.T) {
	lookup := &fakeVisLookup{vis: integ.VisExported, entryPointResult: false}
	store := makeStore(t)
	ws := workspace.WorkspaceKey{RepoRoot: "/repo"}
	now := time.Now()

	// Issue references_checked receipt (NOT impact_checked).
	id, err := store.Issue(ws, guardrails.ClassReferencesChecked, guardrails.ReferencesCheckedScope{
		SymbolID: "HandleAuth",
		FilePath: "src/api/handler.go",
		RefCount: 5,
	}, guardrails.IssueFields{
		GraphVersion: 1,
		Freshness:    "exact",
		TTL:          5 * time.Minute,
	})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	sc := rules.SessionContext{
		Workspace:       ws,
		Lookup:          lookup,
		Store:           store,
		Config:          semantic.GuardrailsConfig{Enforcement: "enforce"},
		OutlineProvider: &fakeOutlineProvider{},
		GraphVersion:    1,
		Now:             now,
	}
	args := rules.RuleArgs{
		Tool:     "replace_symbol_body",
		SymbolID: "HandleAuth",
		Receipts: []guardrails.ReceiptID{id},
	}
	d := rules.EvaluateG003(context.Background(), args, sc)
	// D-15: references_checked alone is REJECTED — must still require impact_checked.
	if d.Action == rules.Allow {
		t.Error("G-003 must NOT allow with only references_checked; impact_checked is required (D-15)")
	}
	// The required receipt must be impact_checked.
	hasImpact := false
	for _, rr := range d.RequiredReceipts {
		if rr.Class == guardrails.ClassImpactChecked {
			hasImpact = true
		}
	}
	if !hasImpact {
		t.Errorf("rejected case: required receipts must list impact_checked, got %v", d.RequiredReceipts)
	}
}

func TestG003_Exported_WithImpactChecked_Allows(t *testing.T) {
	lookup := &fakeVisLookup{vis: integ.VisExported, entryPointResult: false}
	store := makeStore(t)
	ws := workspace.WorkspaceKey{RepoRoot: "/repo"}
	now := time.Now()

	// Issue impact_checked receipt.
	id, err := store.Issue(ws, guardrails.ClassImpactChecked, guardrails.ImpactCheckedScope{
		SymbolID:  "HandleAuth",
		RefCount:  5,
		PublicAPI: true,
	}, guardrails.IssueFields{
		GraphVersion: 1,
		Freshness:    "exact",
		TTL:          5 * time.Minute,
	})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	sc := rules.SessionContext{
		Workspace:       ws,
		Lookup:          lookup,
		Store:           store,
		Config:          semantic.GuardrailsConfig{Enforcement: "enforce"},
		OutlineProvider: &fakeOutlineProvider{},
		GraphVersion:    1,
		Now:             now,
	}
	args := rules.RuleArgs{
		Tool:     "replace_symbol_body",
		SymbolID: "HandleAuth",
		Receipts: []guardrails.ReceiptID{id},
	}
	d := rules.EvaluateG003(context.Background(), args, sc)
	if d.Action != rules.Allow {
		t.Errorf("exported symbol with impact_checked: expected Allow, got %v (msg: %s)", d.Action, d.Message)
	}
}

func TestG003_EntrypointReachable_RejectsReferencesCheckedAlone(t *testing.T) {
	lookup := &fakeVisLookup{vis: integ.VisPrivate, entryPointResult: true}
	store := makeStore(t)
	ws := workspace.WorkspaceKey{RepoRoot: "/repo"}
	now := time.Now()

	// Issue references_checked (NOT impact_checked).
	id, err := store.Issue(ws, guardrails.ClassReferencesChecked, guardrails.ReferencesCheckedScope{
		SymbolID: "internalHandler",
		FilePath: "src/internal/api.go",
		RefCount: 2,
	}, guardrails.IssueFields{
		GraphVersion: 1,
		Freshness:    "exact",
		TTL:          5 * time.Minute,
	})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	sc := rules.SessionContext{
		Workspace:       ws,
		Lookup:          lookup,
		Store:           store,
		Config:          semantic.GuardrailsConfig{Enforcement: "enforce"},
		OutlineProvider: &fakeOutlineProvider{},
		GraphVersion:    1,
		Now:             now,
	}
	args := rules.RuleArgs{
		Tool:     "replace_symbol_body",
		SymbolID: "internalHandler",
		Receipts: []guardrails.ReceiptID{id},
	}
	d := rules.EvaluateG003(context.Background(), args, sc)
	if d.Action == rules.Allow {
		t.Error("entrypoint-reachable symbol: references_checked alone must NOT allow")
	}
}

func TestG003_SignatureChange_WithRefs_Blocks(t *testing.T) {
	lookup := &fakeVisLookup{vis: integ.VisPrivate, entryPointResult: false}
	sc := makeSCWithLookup(t, "enforce", lookup)
	args := rules.RuleArgs{
		Tool:            "replace_symbol_body",
		SymbolID:        "helperFn",
		SymbolName:      "helperFn",
		SignatureChange: true,
		RefCount:        3,
	}
	d := rules.EvaluateG003(context.Background(), args, sc)
	if d.Action != rules.Block {
		t.Errorf("signature change with refs: expected Block, got %v", d.Action)
	}
}

func TestG003_Degraded_UppercaseName_Warns(t *testing.T) {
	// Degraded: Available()=false; SymbolName starts with uppercase.
	degraded := &fakeDegradedLookup{}
	store := makeStore(t)
	ws := workspace.WorkspaceKey{RepoRoot: "/repo"}
	sc := rules.SessionContext{
		Workspace:       ws,
		Lookup:          degraded,
		Store:           store,
		Config:          semantic.GuardrailsConfig{Enforcement: "enforce"},
		OutlineProvider: &fakeOutlineProvider{},
		GraphVersion:    1,
		Now:             time.Now(),
	}
	args := rules.RuleArgs{
		Tool:       "replace_symbol_body",
		SymbolName: "HandleAuth", // uppercase → Go heuristic: exported
		SymbolID:   "HandleAuth",
	}
	d := rules.EvaluateG003(context.Background(), args, sc)
	// Degraded conservative: Warn (not silent Allow).
	if d.Action == rules.Allow {
		t.Errorf("degraded mode with uppercase symbol: expected Warn or Block, got Allow")
	}
}

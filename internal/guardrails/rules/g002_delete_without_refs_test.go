package rules_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/agenthands/helix/internal/guardrails"
	"github.com/agenthands/helix/internal/guardrails/rules"
	"github.com/agenthands/helix/internal/semantic"
	"github.com/agenthands/helix/internal/semantic/integ"
	"github.com/agenthands/helix/internal/workspace"
)

// fakeDegradedLookup returns ErrUnsupported for Visibility/IsEntrypointReachable.
type fakeDegradedLookup struct {
	fakeNoopLookup
}

func (f *fakeDegradedLookup) Available() bool { return false }
func (f *fakeDegradedLookup) Visibility(_ context.Context, _ workspace.WorkspaceKey, _ integ.SymbolID) (integ.Visibility, error) {
	return integ.VisUnknown, errors.ErrUnsupported
}

func TestG002_DeleteFile_TrackedExported_Blocks(t *testing.T) {
	outline := &fakeOutlineProvider{}
	sc := makeSC(t, "enforce", outline)
	args := rules.RuleArgs{
		Tool:                "delete_file",
		Path:                "src/auth/main.go",
		IsTrackedSourceFile: true,
		HasExportedSymbols:  true,
	}
	d := rules.EvaluateG002(context.Background(), args, sc)
	if d.Action != rules.Block {
		t.Errorf("delete_file tracked+exported: expected Block, got %v", d.Action)
	}
	if d.Rule != "G-002" {
		t.Errorf("expected Rule=G-002, got %q", d.Rule)
	}
}

func TestG002_SafeDeleteSymbol_NoReceipts_Blocks(t *testing.T) {
	outline := &fakeOutlineProvider{}
	sc := makeSC(t, "enforce", outline)
	args := rules.RuleArgs{
		Tool: "safe_delete_symbol",
		Path: "src/api/service.go",
	}
	d := rules.EvaluateG002(context.Background(), args, sc)
	if d.Action != rules.Block {
		t.Errorf("safe_delete_symbol with no receipts: expected Block, got %v", d.Action)
	}
}

func TestG002_ReplaceSymbolBody_NoRefs_Private_Allows(t *testing.T) {
	outline := &fakeOutlineProvider{}
	store := makeStore(t)
	ws := workspace.WorkspaceKey{RepoRoot: "/repo"}
	sc := rules.SessionContext{
		Workspace:       ws,
		Lookup:          &fakeNoopLookup{available: true},
		Store:           store,
		Config:          semantic.GuardrailsConfig{Enforcement: "enforce"},
		OutlineProvider: outline,
		GraphVersion:    1,
		Now:             time.Now(),
	}
	// Visibility returns Private
	fakeLookup := &fakeVisLookup{vis: integ.VisPrivate}
	sc.Lookup = fakeLookup

	args := rules.RuleArgs{
		Tool:     "replace_symbol_body",
		Path:     "src/internal/util.go",
		SymbolID: "internalHelper",
		RefCount: 0,
	}
	d := rules.EvaluateG002(context.Background(), args, sc)
	if d.Action != rules.Allow {
		t.Errorf("private symbol with 0 refs: expected Allow, got %v", d.Action)
	}
}

func TestG002_ReplaceSymbolBody_HasRefs_Blocks(t *testing.T) {
	outline := &fakeOutlineProvider{}
	fakeLookup := &fakeVisLookup{vis: integ.VisPrivate}
	store := makeStore(t)
	ws := workspace.WorkspaceKey{RepoRoot: "/repo"}
	sc := rules.SessionContext{
		Workspace:       ws,
		Lookup:          fakeLookup,
		Store:           store,
		Config:          semantic.GuardrailsConfig{Enforcement: "enforce"},
		OutlineProvider: outline,
		GraphVersion:    1,
		Now:             time.Now(),
	}
	args := rules.RuleArgs{
		Tool:     "replace_symbol_body",
		Path:     "src/internal/util.go",
		SymbolID: "internalHelper",
		RefCount: 5, // has refs
	}
	d := rules.EvaluateG002(context.Background(), args, sc)
	if d.Action != rules.Block {
		t.Errorf("symbol with refs and no receipt: expected Block, got %v", d.Action)
	}
}

func TestG002_ReplaceSymbolBody_Exported_Blocks(t *testing.T) {
	outline := &fakeOutlineProvider{}
	fakeLookup := &fakeVisLookup{vis: integ.VisExported}
	store := makeStore(t)
	ws := workspace.WorkspaceKey{RepoRoot: "/repo"}
	sc := rules.SessionContext{
		Workspace:       ws,
		Lookup:          fakeLookup,
		Store:           store,
		Config:          semantic.GuardrailsConfig{Enforcement: "enforce"},
		OutlineProvider: outline,
		GraphVersion:    1,
		Now:             time.Now(),
	}
	args := rules.RuleArgs{
		Tool:     "replace_symbol_body",
		Path:     "src/api/handler.go",
		SymbolID: "HandleAuth",
		RefCount: 0,
	}
	d := rules.EvaluateG002(context.Background(), args, sc)
	if d.Action != rules.Block {
		t.Errorf("exported symbol: expected Block, got %v", d.Action)
	}
}

func TestG002_ReplaceSymbolBody_Exported_WithReferencesChecked_Allows(t *testing.T) {
	outline := &fakeOutlineProvider{}
	fakeLookup := &fakeVisLookup{vis: integ.VisExported}
	store := makeStore(t)
	ws := workspace.WorkspaceKey{RepoRoot: "/repo"}
	now := time.Now()

	// Issue references_checked receipt for the symbol.
	id, err := store.Issue(ws, guardrails.ClassReferencesChecked, guardrails.ReferencesCheckedScope{
		SymbolID: "HandleAuth",
		FilePath: "src/api/handler.go",
		RefCount: 0,
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
		Lookup:          fakeLookup,
		Store:           store,
		Config:          semantic.GuardrailsConfig{Enforcement: "enforce"},
		OutlineProvider: outline,
		GraphVersion:    1,
		Now:             now,
	}
	args := rules.RuleArgs{
		Tool:     "replace_symbol_body",
		Path:     "src/api/handler.go",
		SymbolID: "HandleAuth",
		RefCount: 0,
		Receipts: []guardrails.ReceiptID{id},
	}
	d := rules.EvaluateG002(context.Background(), args, sc)
	if d.Action != rules.Allow {
		t.Errorf("exported symbol with references_checked: expected Allow, got %v (msg: %s)", d.Action, d.Message)
	}
}

func TestG002_DegradedMode_TrackedFile_Warns(t *testing.T) {
	outline := &fakeOutlineProvider{}
	degraded := &fakeDegradedLookup{}
	store := makeStore(t)
	ws := workspace.WorkspaceKey{RepoRoot: "/repo"}
	sc := rules.SessionContext{
		Workspace:       ws,
		Lookup:          degraded,
		Store:           store,
		Config:          semantic.GuardrailsConfig{Enforcement: "enforce"},
		OutlineProvider: outline,
		GraphVersion:    1,
		Now:             time.Now(),
	}
	args := rules.RuleArgs{
		Tool:                "delete_file",
		Path:                "src/auth/main.go",
		IsTrackedSourceFile: true,
	}
	d := rules.EvaluateG002(context.Background(), args, sc)
	// Degraded mode: conservative warn (cannot count refs)
	if d.Action != rules.Warn {
		t.Errorf("degraded mode delete_file: expected Warn (conservative), got %v", d.Action)
	}
}

// fakeVisLookup returns a configurable Visibility.
type fakeVisLookup struct {
	fakeNoopLookup
	vis              integ.Visibility
	entryPointResult bool
}

func (f *fakeVisLookup) Available() bool { return true }
func (f *fakeVisLookup) Visibility(_ context.Context, _ workspace.WorkspaceKey, _ integ.SymbolID) (integ.Visibility, error) {
	return f.vis, nil
}
func (f *fakeVisLookup) IsEntrypointReachable(_ context.Context, _ workspace.WorkspaceKey, _ integ.SymbolID) (bool, error) {
	return f.entryPointResult, nil
}

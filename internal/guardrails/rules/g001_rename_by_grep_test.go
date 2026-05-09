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

// fakeOutlineProvider returns a static list of symbols for any path.
type fakeOutlineProvider struct {
	symbols []rules.OutlineSymbol
}

func (f *fakeOutlineProvider) SymbolsInFile(_ context.Context, _ workspace.WorkspaceKey, _ string) ([]rules.OutlineSymbol, error) {
	return f.symbols, nil
}

// fakeNoopLookup is a minimal SemanticLookup fake for unit tests.
type fakeNoopLookup struct {
	available bool
}

func (f *fakeNoopLookup) Available() bool { return f.available }
func (f *fakeNoopLookup) SymbolID(_ context.Context, _ workspace.WorkspaceKey, _ string, _, _ uint32) (integ.SymbolID, error) {
	return "", nil
}
func (f *fakeNoopLookup) RankFiles(_ context.Context, _ workspace.WorkspaceKey) ([]integ.RankedFile, error) {
	return nil, nil
}
func (f *fakeNoopLookup) RankFromSeeds(_ context.Context, _ workspace.WorkspaceKey, _ []string) ([]integ.RankedFile, error) {
	return nil, nil
}
func (f *fakeNoopLookup) ExpandFrom(_ context.Context, _ workspace.WorkspaceKey, _ integ.SymbolID, _ int) ([]integ.Impact, error) {
	return nil, nil
}
func (f *fakeNoopLookup) ValidateCriticalEdges(_ context.Context, _ workspace.WorkspaceKey, edges []integ.Edge) ([]integ.ValidatedEdge, error) {
	return nil, nil
}
func (f *fakeNoopLookup) LocateSymbol(_ context.Context, _ workspace.WorkspaceKey, _ integ.SymbolID) (string, uint32, uint32, bool, error) {
	return "", 0, 0, false, nil
}
func (f *fakeNoopLookup) Status(_ context.Context, _ workspace.WorkspaceKey) (integ.SemanticStatus, error) {
	return integ.SemanticStatus{}, nil
}
func (f *fakeNoopLookup) Visibility(_ context.Context, _ workspace.WorkspaceKey, _ integ.SymbolID) (integ.Visibility, error) {
	return integ.VisUnknown, nil
}
func (f *fakeNoopLookup) IsEntrypointReachable(_ context.Context, _ workspace.WorkspaceKey, _ integ.SymbolID) (bool, error) {
	return false, nil
}

// makeStore creates a fresh store for tests.
func makeStore(t *testing.T) *guardrails.Store {
	t.Helper()
	s := guardrails.NewStore(guardrails.StoreOptions{})
	t.Cleanup(s.Close)
	return s
}

// makeSC builds a SessionContext for tests.
func makeSC(t *testing.T, enforcement string, outline *fakeOutlineProvider) rules.SessionContext {
	t.Helper()
	return rules.SessionContext{
		Workspace:       workspace.WorkspaceKey{RepoRoot: "/repo"},
		Profile:         "claude-code",
		Lookup:          &fakeNoopLookup{available: true},
		Store:           makeStore(t),
		Config:          semantic.GuardrailsConfig{Enforcement: enforcement},
		OutlineProvider: outline,
		GraphVersion:    1,
		Now:             time.Now(),
	}
}

func TestG001_RenameSymbolExempt(t *testing.T) {
	outline := &fakeOutlineProvider{
		symbols: []rules.OutlineSymbol{
			{Name: "Foo", Kind: "function"},
		},
	}
	sc := makeSC(t, "enforce", outline)
	args := rules.RuleArgs{
		Tool: "rename_symbol",
		Path: "pkg/api/handler.go",
		Find: "Foo",
	}
	d := rules.EvaluateG001(context.Background(), args, sc)
	if d.Action != rules.Allow {
		t.Errorf("rename_symbol should be exempt from G-001, got %v", d.Action)
	}
}

func TestG001_ShortFindAllowed(t *testing.T) {
	outline := &fakeOutlineProvider{}
	sc := makeSC(t, "enforce", outline)
	args := rules.RuleArgs{
		Tool: "fuzzy_edit",
		Path: "pkg/api/handler.go",
		Find: "x", // length < 3
	}
	d := rules.EvaluateG001(context.Background(), args, sc)
	if d.Action != rules.Allow {
		t.Errorf("short find (len<3) should Allow, got %v", d.Action)
	}
}

func TestG001_StartsWithDigitAllowed(t *testing.T) {
	outline := &fakeOutlineProvider{}
	sc := makeSC(t, "enforce", outline)
	args := rules.RuleArgs{
		Tool: "fuzzy_edit",
		Path: "pkg/api/handler.go",
		Find: "123abc",
	}
	d := rules.EvaluateG001(context.Background(), args, sc)
	if d.Action != rules.Allow {
		t.Errorf("find starting with digit should Allow (non-identifier), got %v", d.Action)
	}
}

func TestG001_SymbolMatch_Enforce_Blocks(t *testing.T) {
	outline := &fakeOutlineProvider{
		symbols: []rules.OutlineSymbol{
			{Name: "AuthMiddleware", Kind: "function"},
		},
	}
	sc := makeSC(t, "enforce", outline)
	args := rules.RuleArgs{
		Tool:     "fuzzy_edit",
		Path:     "pkg/api/handler.go",
		Find:     "AuthMiddleware",
		Receipts: nil,
	}
	d := rules.EvaluateG001(context.Background(), args, sc)
	if d.Action != rules.Block {
		t.Errorf("enforce mode: symbol match with no receipts should Block, got %v", d.Action)
	}
	if d.Rule != "G-001" {
		t.Errorf("expected Rule=G-001, got %q", d.Rule)
	}
	// Must require references_checked OR impact_checked.
	found := false
	for _, rr := range d.RequiredReceipts {
		if rr.Class == guardrails.ClassReferencesChecked || rr.Class == guardrails.ClassImpactChecked {
			found = true
		}
	}
	if !found {
		t.Errorf("expected RequiredReceipts to include references_checked or impact_checked, got %v", d.RequiredReceipts)
	}
}

func TestG001_SymbolMatch_Warn_Warns(t *testing.T) {
	outline := &fakeOutlineProvider{
		symbols: []rules.OutlineSymbol{
			{Name: "AuthMiddleware", Kind: "function"},
		},
	}
	sc := makeSC(t, "warn", outline)
	args := rules.RuleArgs{
		Tool: "fuzzy_edit",
		Path: "pkg/api/handler.go",
		Find: "AuthMiddleware",
	}
	d := rules.EvaluateG001(context.Background(), args, sc)
	if d.Action != rules.Warn {
		t.Errorf("warn mode: symbol match with no receipts should Warn, got %v", d.Action)
	}
}

func TestG001_SymbolMatch_WithReferencesCheckedReceipt_Allows(t *testing.T) {
	outline := &fakeOutlineProvider{
		symbols: []rules.OutlineSymbol{
			{Name: "AuthMiddleware", Kind: "function"},
		},
	}
	store := makeStore(t)
	ws := workspace.WorkspaceKey{RepoRoot: "/repo"}
	now := time.Now()
	// Issue a references_checked receipt for this symbol.
	id, err := store.Issue(ws, guardrails.ClassReferencesChecked, guardrails.ReferencesCheckedScope{
		SymbolID: "AuthMiddleware",
		FilePath: "pkg/api/handler.go",
		RefCount: 3,
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
		Lookup:          &fakeNoopLookup{available: true},
		Store:           store,
		Config:          semantic.GuardrailsConfig{Enforcement: "enforce"},
		OutlineProvider: outline,
		GraphVersion:    1,
		Now:             now,
	}
	args := rules.RuleArgs{
		Tool:     "fuzzy_edit",
		Path:     "pkg/api/handler.go",
		Find:     "AuthMiddleware",
		SymbolID: "AuthMiddleware",
		Receipts: []guardrails.ReceiptID{id},
	}
	d := rules.EvaluateG001(context.Background(), args, sc)
	if d.Action != rules.Allow {
		t.Errorf("with valid references_checked receipt: expected Allow, got %v (msg: %s)", d.Action, d.Message)
	}
}

func TestG001_ReplaceInFile_SymbolMatch_Blocks(t *testing.T) {
	outline := &fakeOutlineProvider{
		symbols: []rules.OutlineSymbol{
			{Name: "getValue", Kind: "method"},
		},
	}
	sc := makeSC(t, "enforce", outline)
	args := rules.RuleArgs{
		Tool: "replace_in_file",
		Path: "pkg/api/service.go",
		Find: "getValue",
	}
	d := rules.EvaluateG001(context.Background(), args, sc)
	if d.Action != rules.Block {
		t.Errorf("replace_in_file with symbol match: expected Block, got %v", d.Action)
	}
}

func TestG001_ReplaceInFile_NotInOutline_Allows(t *testing.T) {
	outline := &fakeOutlineProvider{
		symbols: []rules.OutlineSymbol{
			{Name: "otherFunc", Kind: "function"},
		},
	}
	sc := makeSC(t, "enforce", outline)
	args := rules.RuleArgs{
		Tool: "replace_in_file",
		Path: "pkg/api/service.go",
		Find: "getValue", // not in outline
	}
	d := rules.EvaluateG001(context.Background(), args, sc)
	if d.Action != rules.Allow {
		t.Errorf("replace_in_file with non-symbol find: expected Allow (safe), got %v", d.Action)
	}
}

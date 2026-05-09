package rules_test

import (
	"context"
	"testing"
	"time"

	"github.com/agenthands/helix/internal/guardrails"
	"github.com/agenthands/helix/internal/guardrails/catalogs"
	"github.com/agenthands/helix/internal/guardrails/rules"
	"github.com/agenthands/helix/internal/semantic"
	"github.com/agenthands/helix/internal/workspace"
)

// makeCatalogSC creates a SessionContext with a single "go" catalog loaded.
func makeCatalogSC(t *testing.T, enforcement string, cat catalogs.Catalog) rules.SessionContext {
	t.Helper()
	return rules.SessionContext{
		Workspace:       workspace.WorkspaceKey{RepoRoot: "/repo"},
		Profile:         "claude-code",
		Lookup:          &fakeNoopLookup{available: true},
		Store:           makeStore(t),
		Config:          semantic.GuardrailsConfig{Enforcement: enforcement},
		OutlineProvider: &fakeOutlineProvider{},
		GraphVersion:    1,
		Now:             time.Now(),
		Catalogs:        map[string]catalogs.Catalog{"go": cat},
	}
}

var goCatalog = catalogs.Catalog{
	ImportPatterns:     []string{"crypto/*", "golang.org/x/crypto/*"},
	PathGlobs:          []string{"**/auth*.go", "**/*_auth.go"},
	IdentifierPatterns: []string{".*Auth.*", ".*Password.*", ".*Secret.*"},
}

// TestG005_NoSignals: no matching signals → Allow.
func TestG005_NoSignals(t *testing.T) {
	sc := makeCatalogSC(t, "enforce", goCatalog)
	args := rules.RuleArgs{
		Tool:       "fuzzy_edit",
		Path:       "pkg/models/user.go",
		SymbolName: "GetUser",
	}
	d := rules.EvaluateG005(context.Background(), args, sc)
	if d.Action != rules.Allow {
		t.Errorf("no signals: expected Allow, got %v", d.Action)
	}
	if d.Rule != "" {
		t.Errorf("no signals: expected empty Rule, got %q", d.Rule)
	}
}

// TestG005_PathGlobOnly: path matches glob → sensitive.
func TestG005_PathGlobOnly(t *testing.T) {
	sc := makeCatalogSC(t, "enforce", goCatalog)
	args := rules.RuleArgs{
		Tool:       "fuzzy_edit",
		Path:       "pkg/auth_handler.go", // matches **/auth*.go
		SymbolName: "GetUser",
	}
	d := rules.EvaluateG005(context.Background(), args, sc)
	if d.Action == rules.Allow && d.Rule == "" {
		t.Error("path glob match: expected non-Allow decision (security-sensitive)")
	}
	if d.Rule != "G-005" {
		t.Errorf("expected Rule=G-005, got %q", d.Rule)
	}
}

// TestG005_IdentifierOnly: identifier matches pattern → sensitive.
func TestG005_IdentifierOnly(t *testing.T) {
	sc := makeCatalogSC(t, "enforce", goCatalog)
	args := rules.RuleArgs{
		Tool:       "replace_symbol_body",
		Path:       "pkg/models/user.go",
		SymbolName: "AuthMiddleware", // matches .*Auth.*
	}
	d := rules.EvaluateG005(context.Background(), args, sc)
	if d.Action == rules.Allow && d.Rule == "" {
		t.Error("identifier match: expected non-Allow decision")
	}
	if d.Rule != "G-005" {
		t.Errorf("expected Rule=G-005, got %q", d.Rule)
	}
}

// TestG005_ImportOnly: import matches pattern → sensitive.
func TestG005_ImportOnly(t *testing.T) {
	sc := makeCatalogSC(t, "enforce", goCatalog)
	args := rules.RuleArgs{
		Tool:        "fuzzy_edit",
		Path:        "pkg/models/user.go",
		SymbolName:  "GetUser",
		FileImports: []string{"crypto/aes"}, // matches crypto/*
	}
	d := rules.EvaluateG005(context.Background(), args, sc)
	if d.Action == rules.Allow && d.Rule == "" {
		t.Error("import match: expected non-Allow decision")
	}
	if d.Rule != "G-005" {
		t.Errorf("expected Rule=G-005, got %q", d.Rule)
	}
}

// TestG005_AllThreeSignals: all three signals triggered.
func TestG005_AllThreeSignals(t *testing.T) {
	args := rules.RuleArgs{
		Tool:        "fuzzy_edit",
		Path:        "pkg/auth_handler.go",
		SymbolName:  "AuthMiddleware",
		FileImports: []string{"crypto/rand"},
	}
	matched, signals := rules.IsSecuritySensitive(args, goCatalog)
	if !matched {
		t.Error("expected matched=true for all-three-signal case")
	}
	signalSet := map[string]bool{}
	for _, s := range signals {
		signalSet[s] = true
	}
	if !signalSet["path_glob"] {
		t.Error("expected path_glob signal")
	}
	if !signalSet["identifier"] {
		t.Error("expected identifier signal")
	}
	if !signalSet["import_pattern"] {
		t.Error("expected import_pattern signal")
	}
}

// TestG005_Sensitive_NoReceipt_Blocks: sensitive + no receipt → Block (enforce).
func TestG005_Sensitive_NoReceipt_Blocks(t *testing.T) {
	sc := makeCatalogSC(t, "enforce", goCatalog)
	args := rules.RuleArgs{
		Tool:       "fuzzy_edit",
		Path:       "pkg/auth_handler.go",
		SymbolName: "AuthMiddleware",
	}
	d := rules.EvaluateG005(context.Background(), args, sc)
	if d.Action != rules.Block {
		t.Errorf("sensitive + enforce + no receipt: expected Block, got %v", d.Action)
	}
	// Must require context_gathered and structural_overview.
	classes := map[guardrails.ReceiptClass]bool{}
	for _, rr := range d.RequiredReceipts {
		classes[rr.Class] = true
	}
	if !classes[guardrails.ClassContextGathered] {
		t.Error("expected context_gathered in RequiredReceipts")
	}
	if !classes[guardrails.ClassStructuralOverview] {
		t.Error("expected structural_overview in RequiredReceipts")
	}
	// Must suggest get_context and get_repo_map.
	toolSet := map[string]bool{}
	for _, s := range d.SuggestedTools {
		toolSet[s] = true
	}
	if !toolSet["get_context"] {
		t.Error("expected get_context in SuggestedTools")
	}
	if !toolSet["get_repo_map"] {
		t.Error("expected get_repo_map in SuggestedTools")
	}
}

// TestG005_Sensitive_WithContextGathered_AllowsWithVerifyEdit: sensitive + context_gathered → Allow + verify_edit obligation.
func TestG005_Sensitive_WithContextGathered_AllowsWithVerifyEdit(t *testing.T) {
	store := makeStore(t)
	ws := workspace.WorkspaceKey{RepoRoot: "/repo"}
	now := time.Now()

	// Issue context_gathered receipt for the file.
	id, err := store.Issue(ws, guardrails.ClassContextGathered, guardrails.ContextGatheredScope{
		FileSet:  []string{"pkg/auth_handler.go"},
		TaskHash: "abc123",
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
		OutlineProvider: &fakeOutlineProvider{},
		GraphVersion:    1,
		Now:             now,
		Catalogs:        map[string]catalogs.Catalog{"go": goCatalog},
	}
	args := rules.RuleArgs{
		Tool:       "fuzzy_edit",
		Path:       "pkg/auth_handler.go",
		SymbolName: "AuthMiddleware",
		TouchedFiles: []string{"pkg/auth_handler.go"},
		Receipts:   []guardrails.ReceiptID{id},
	}
	d := rules.EvaluateG005(context.Background(), args, sc)
	if d.Action != rules.Allow {
		t.Errorf("sensitive with context_gathered: expected Allow, got %v (msg: %s)", d.Action, d.Message)
	}
	// Must include verify_edit in SuggestedTools.
	found := false
	for _, s := range d.SuggestedTools {
		if s == "verify_edit" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected verify_edit in SuggestedTools, got %v", d.SuggestedTools)
	}
}

// TestG005_DegradedGrepFallback: Lookup.Available()=false, FileImports=nil, FileImportsFromGrep has crypto → sensitive.
func TestG005_DegradedGrepFallback(t *testing.T) {
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
		Catalogs:        map[string]catalogs.Catalog{"go": goCatalog},
	}
	args := rules.RuleArgs{
		Tool:                "fuzzy_edit",
		Path:                "pkg/models/user.go",
		SymbolName:          "GetUser",
		FileImports:         nil, // not available (degraded)
		FileImportsFromGrep: []string{"crypto/aes"}, // grep fallback
	}
	matched, signals := rules.IsSecuritySensitive(args, goCatalog)
	if !matched {
		t.Error("degraded grep fallback: expected matched=true when FileImportsFromGrep has crypto/aes")
	}
	found := false
	for _, s := range signals {
		if s == "import_pattern" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected import_pattern signal via grep fallback, got %v", signals)
	}
	d := rules.EvaluateG005(context.Background(), args, sc)
	if d.Action == rules.Allow && d.Rule == "" {
		t.Error("degraded grep: sensitive + enforce + no receipt: expected non-Allow")
	}
	if d.Rule != "G-005" {
		t.Errorf("expected Rule=G-005, got %q", d.Rule)
	}
}

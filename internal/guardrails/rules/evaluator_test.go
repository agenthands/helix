package rules_test

import (
	"context"
	"testing"

	"github.com/agenthands/helix/internal/guardrails/catalogs"
	"github.com/agenthands/helix/internal/guardrails/rules"
)

// TestEvaluator_FuzzyEditAllRules: fuzzy_edit triggers G-001 (symbol match), G-004 (large), G-005 (sensitive).
// We verify via the returned decisions slice that multiple rules fire.
func TestEvaluator_FuzzyEditAllRules(t *testing.T) {
	evaluator := rules.DefaultEvaluator{}
	outline := &fakeOutlineProvider{
		symbols: []rules.OutlineSymbol{
			{Name: "AuthMiddleware", Kind: "function"},
		},
	}
	sc := makeSC(t, "enforce", outline)
	// Set up catalog to trigger G-005 on identifier.
	from, err := loadGoTestCatalog()
	if err == nil {
		sc.Catalogs = map[string]catalogs.Catalog{"go": from}
	}

	args := rules.RuleArgs{
		Tool:         "fuzzy_edit",
		Path:         "pkg/auth_handler.go", // G-005 path glob hit
		Find:         "AuthMiddleware",       // G-001 symbol match
		SymbolName:   "AuthMiddleware",       // G-005 identifier hit
		ChangedLines: 51,                     // G-004 threshold exceeded
		TouchedFiles: []string{"pkg/auth_handler.go"},
	}
	decisions := evaluator.Evaluate(context.Background(), args, sc)
	// Should have at least G-001 + G-004 triggered (G-005 depends on catalog).
	if len(decisions) == 0 {
		t.Error("fuzzy_edit with multiple triggers: expected at least one triggered rule")
	}
	ruleSet := map[string]bool{}
	for _, d := range decisions {
		ruleSet[d.Rule] = true
	}
	if !ruleSet["G-001"] {
		t.Errorf("expected G-001 in triggered rules, got %v", ruleSet)
	}
	if !ruleSet["G-004"] {
		t.Errorf("expected G-004 in triggered rules, got %v", ruleSet)
	}
}

// TestEvaluator_RenameSymbolSkipsG001G004: rename_symbol should not trigger G-001 or G-004.
func TestEvaluator_RenameSymbolSkipsG001G004(t *testing.T) {
	evaluator := rules.DefaultEvaluator{}
	outline := &fakeOutlineProvider{
		symbols: []rules.OutlineSymbol{
			{Name: "AuthMiddleware", Kind: "function"},
		},
	}
	sc := makeSC(t, "enforce", outline)
	args := rules.RuleArgs{
		Tool:         "rename_symbol",
		Path:         "pkg/api/handler.go",
		Find:         "AuthMiddleware",
		SymbolName:   "AuthMiddleware",
		ChangedLines: 100, // large, but G-004 skip for rename_symbol
		TouchedFiles: []string{"pkg/api/handler.go"},
	}
	decisions := evaluator.Evaluate(context.Background(), args, sc)
	for _, d := range decisions {
		if d.Rule == "G-001" {
			t.Error("rename_symbol should NOT trigger G-001 (exempt)")
		}
		if d.Rule == "G-004" {
			t.Error("rename_symbol should NOT trigger G-004 (single-symbol op)")
		}
	}
}

// TestEvaluator_DeleteFileSkipsG003G004: delete_file should only trigger G-002 + G-005.
func TestEvaluator_DeleteFileSkipsG003G004(t *testing.T) {
	evaluator := rules.DefaultEvaluator{}
	sc := makeSC(t, "enforce", &fakeOutlineProvider{})
	args := rules.RuleArgs{
		Tool:                "delete_file",
		Path:                "pkg/api/handler.go",
		IsTrackedSourceFile: true,
		ChangedLines:        100, // irrelevant for delete_file
		TouchedFiles:        []string{"pkg/api/handler.go"},
	}
	decisions := evaluator.Evaluate(context.Background(), args, sc)
	for _, d := range decisions {
		if d.Rule == "G-003" {
			t.Error("delete_file should NOT trigger G-003")
		}
		if d.Rule == "G-004" {
			t.Error("delete_file should NOT trigger G-004")
		}
	}
	// Must include G-002 (tracked source file).
	hasG002 := false
	for _, d := range decisions {
		if d.Rule == "G-002" {
			hasG002 = true
		}
	}
	if !hasG002 {
		t.Error("delete_file on tracked source: expected G-002 in decisions")
	}
}

// TestEvaluator_UnknownToolReturnsEmpty: unknown tool should return empty decisions.
func TestEvaluator_UnknownToolReturnsEmpty(t *testing.T) {
	evaluator := rules.DefaultEvaluator{}
	sc := makeSC(t, "enforce", &fakeOutlineProvider{})
	args := rules.RuleArgs{
		Tool: "get_diagnostics",
		Path: "pkg/api/handler.go",
	}
	decisions := evaluator.Evaluate(context.Background(), args, sc)
	if len(decisions) != 0 {
		t.Errorf("unknown tool: expected empty decisions, got %v", decisions)
	}
}

// loadGoTestCatalog is a helper that returns a Catalog for use in evaluator tests.
func loadGoTestCatalog() (catalogs.Catalog, error) {
	return catalogs.Catalog{
		ImportPatterns:     []string{"crypto/*"},
		PathGlobs:          []string{"**/auth*.go"},
		IdentifierPatterns: []string{".*Auth.*"},
	}, nil
}

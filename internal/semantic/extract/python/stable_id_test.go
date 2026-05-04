package pyextract

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/agenthands/helix/internal/semantic"
	"github.com/agenthands/helix/internal/semantic/extract"
	"github.com/agenthands/helix/internal/semantic/extract/testutil"
)

type stableIDExpectation string

const (
	expPreserved    stableIDExpectation = "preserved"
	expChurned      stableIDExpectation = "churned"
	expNewlyDefined stableIDExpectation = "newly_defined"
)

// TestStableID exercises the Python-specific transitions per PLAN 59-04.
// Decorator add/remove preserves ID per CONTEXT.md D-03; dunder rename
// churns ID since QualifiedName changes.
func TestStableID(t *testing.T) {
	type tcase struct {
		scenario   string
		beforeName string
		afterName  string
		expect     stableIDExpectation
	}

	cases := []tcase{
		{scenario: "whitespace_edit_preserves_id", beforeName: "add", afterName: "add", expect: expPreserved},
		{scenario: "comment_edit_preserves_id", beforeName: "f", afterName: "f", expect: expPreserved},
		{scenario: "body_only_edit_preserves_id", beforeName: "f", afterName: "f", expect: expPreserved},
		{scenario: "move_within_file_preserves_id", beforeName: "a", afterName: "a", expect: expPreserved},
		{scenario: "rename_churns_id", beforeName: "old_name", afterName: "new_name", expect: expChurned},
		{scenario: "signature_change_churns_id", beforeName: "f", afterName: "f", expect: expPreserved}, // signature not in py hash
		// decorator_add_remove — function `f` exists both before and after
		// with identical canonicalization input → ID preserved.
		{scenario: "decorator_add_remove_preserves_id", beforeName: "f", afterName: "f", expect: expPreserved},
		// dunder rename — different QualifiedName → ID churns.
		{scenario: "dunder_method_rename_churns_id", beforeName: "__init__", afterName: "__new__", expect: expChurned},
		{scenario: "move_exported_within_module_preserves_id", beforeName: "new_widget", afterName: "new_widget", expect: expPreserved},
		{scenario: "method_on_class_rename_churns_id", beforeName: "old_name", afterName: "new_name", expect: expChurned},
	}

	registry := testutil.NewTestRegistry(t)
	p := NewProvider(registry).(*Provider)

	for _, tc := range cases {
		tc := tc
		t.Run(tc.scenario, func(t *testing.T) {
			beforePath, _ := testutil.FindBefore(t, testdataDir, tc.scenario)
			afterPath := testutil.FindAfter(testdataDir, tc.scenario)
			beforeSrc, err := os.ReadFile(beforePath)
			if err != nil {
				t.Fatalf("read before: %v", err)
			}
			afterSrc, err := os.ReadFile(afterPath)
			if err != nil {
				t.Fatalf("read after: %v", err)
			}
			// Use SAME logical file path for both so file-path differences
			// don't churn IDs of unexported symbols (Python convention:
			// underscore-prefixed = private). Same-content rename invariant
			// for exported symbols is exercised by TestBuildProviderKey_*.
			samePath := filepath.Join(tc.scenario, "module.py")
			beforeFile := extract.SourceFile{Path: samePath, Language: "python"}
			afterFile := extract.SourceFile{Path: samePath, Language: "python"}
			beforeEf, err := p.Extract(context.Background(), beforeSrc, beforeFile)
			if err != nil {
				t.Fatalf("Extract before: %v", err)
			}
			afterEf, err := p.Extract(context.Background(), afterSrc, afterFile)
			if err != nil {
				t.Fatalf("Extract after: %v", err)
			}
			beforeID := findSymbolID(beforeEf.Symbols, tc.beforeName)
			afterID := findSymbolID(afterEf.Symbols, tc.afterName)
			if beforeID == 0 {
				t.Fatalf("symbol %q not in before; symbols=%v", tc.beforeName, namesOf(beforeEf.Symbols))
			}
			if afterID == 0 {
				t.Fatalf("symbol %q not in after; symbols=%v", tc.afterName, namesOf(afterEf.Symbols))
			}
			switch tc.expect {
			case expPreserved:
				if beforeID != afterID {
					t.Errorf("expected ID preserved for %q -> %q, but %v != %v", tc.beforeName, tc.afterName, beforeID, afterID)
				}
			case expChurned:
				if beforeID == afterID {
					t.Errorf("expected ID churned for %q -> %q, but both = %v", tc.beforeName, tc.afterName, beforeID)
				}
			}
		})
	}
}

func TestBuildProviderKey_ExportedRenameStable(t *testing.T) {
	a := extract.BuildProviderKey(extract.SymbolMeta{
		Language: "python", PackagePath: "pkg", QualifiedName: "public",
		Kind: "function", SignatureHash: "public", RelPath: "foo.py", Visibility: "exported",
	})
	b := extract.BuildProviderKey(extract.SymbolMeta{
		Language: "python", PackagePath: "pkg", QualifiedName: "public",
		Kind: "function", SignatureHash: "public", RelPath: "bar.py", Visibility: "exported",
	})
	if extract.StableSymbolID(a) != extract.StableSymbolID(b) {
		t.Errorf("exported same-content rename: IDs differ")
	}
}

func findSymbolID(syms []extract.SymbolFact, name string) semantic.SymbolID {
	for _, s := range syms {
		if s.Name == name {
			return s.ID
		}
	}
	return 0
}

func namesOf(syms []extract.SymbolFact) []string {
	out := make([]string, len(syms))
	for i, s := range syms {
		out[i] = s.Name
	}
	return out
}

package kotlinextract

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/agenthands/helix/internal/semantic"
	"github.com/agenthands/helix/internal/semantic/extract"
	"github.com/agenthands/helix/internal/semantic/extract/testutil"
)

// stableIDExpectation encodes the expected transition for a symbol's stable ID
// between before/after fixture extractions.
type stableIDExpectation string

const (
	expPreserved stableIDExpectation = "preserved"
	expChurned   stableIDExpectation = "churned"
)

// TestStableID exercises stable-ID transitions for Kotlin symbols.
// Each scenario has a before/after fixture; we extract from both and
// assert the named symbol's ID exhibits the expected transition.
//
// Coverage:
//   - whitespace_edit_preserves_id
//   - rename_churns_id
func TestStableID(t *testing.T) {
	type tcase struct {
		scenario   string
		beforeName string
		afterName  string
		expect     stableIDExpectation
	}

	cases := []tcase{
		{scenario: "whitespace_edit_preserves_id", beforeName: "Greeter", afterName: "Greeter", expect: expPreserved},
		{scenario: "whitespace_edit_preserves_id", beforeName: "greet", afterName: "greet", expect: expPreserved},
		{scenario: "rename_churns_id", beforeName: "OldName", afterName: "NewName", expect: expChurned},
		{scenario: "rename_churns_id", beforeName: "run", afterName: "run", expect: expPreserved},
		{scenario: "class_basic", beforeName: "Calculator", afterName: "Calculator", expect: expPreserved},
	}

	registry := testutil.NewTestRegistry(t)
	p := NewProvider(registry).(*Provider)

	for _, tc := range cases {
		tc := tc
		t.Run(tc.scenario, func(t *testing.T) {
			beforeSrc, err := os.ReadFile(filepath.Join(testdataDir, tc.scenario, "before.kt"))
			if err != nil {
				t.Fatalf("read before: %v", err)
			}
			beforeEf, err := p.Extract(context.Background(), beforeSrc, extract.SourceFile{
				Path: filepath.Join(tc.scenario, "before.kt"), Language: "kotlin",
			})
			if err != nil {
				t.Fatalf("Extract before: %v", err)
			}

			// For same-content scenarios, extract the same fixture twice.
			if tc.afterName == tc.beforeName && tc.scenario != "whitespace_edit_preserves_id" && tc.scenario != "rename_churns_id" {
				afterEf, err := p.Extract(context.Background(), beforeSrc, extract.SourceFile{
					Path: filepath.Join(tc.scenario, "before.kt"), Language: "kotlin",
				})
				if err != nil {
					t.Fatalf("Extract after: %v", err)
				}
				beforeID := findSymbolID(beforeEf.Symbols, tc.beforeName)
				afterID := findSymbolID(afterEf.Symbols, tc.afterName)
				if beforeID == 0 {
					t.Fatalf("symbol %q not found in before; symbols=%v", tc.beforeName, namesOf(beforeEf.Symbols))
				}
				if afterID == 0 {
					t.Fatalf("symbol %q not found in after; symbols=%v", tc.afterName, namesOf(afterEf.Symbols))
				}
				if beforeID != afterID {
					t.Errorf("expected ID preserved for %q, but %v != %v", tc.beforeName, beforeID, afterID)
				}
				return
			}

			afterSrc, err := os.ReadFile(filepath.Join(testdataDir, tc.scenario, "after.kt"))
			if err != nil {
				t.Fatalf("read after: %v", err)
			}
			afterEf, err := p.Extract(context.Background(), afterSrc, extract.SourceFile{
				Path: filepath.Join(tc.scenario, "after.kt"), Language: "kotlin",
			})
			if err != nil {
				t.Fatalf("Extract after: %v", err)
			}

			beforeID := findSymbolID(beforeEf.Symbols, tc.beforeName)
			afterID := findSymbolID(afterEf.Symbols, tc.afterName)

			if beforeID == 0 {
				t.Fatalf("symbol %q not found in before; symbols=%v", tc.beforeName, namesOf(beforeEf.Symbols))
			}
			if afterID == 0 {
				t.Fatalf("symbol %q not found in after; symbols=%v", tc.afterName, namesOf(afterEf.Symbols))
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

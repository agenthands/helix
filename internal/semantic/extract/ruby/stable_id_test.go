package rubyextract

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/agenthands/helix/internal/semantic"
	"github.com/agenthands/helix/internal/semantic/extract"
	"github.com/agenthands/helix/internal/semantic/extract/testutil"
)

// stableIDExpectation per CONTEXT.md D-03.
type stableIDExpectation string

const (
	expPreserved    stableIDExpectation = "preserved"
	expChurned      stableIDExpectation = "churned"
	expNewlyDefined stableIDExpectation = "newly_defined"
	expRemoved      stableIDExpectation = "removed"
)

// TestStableID exercises the per-language stable-ID transitions. Each
// scenario has a before/after fixture; we extract from both and assert
// the named symbol's ID exhibits the expected transition.
//
// Coverage (≥5):
//   - whitespace_edit_preserves_id
//   - comment_edit_preserves_id
//   - body_only_edit_preserves_id
//   - rename_churns_id
//   - method_rename_churns_id
func TestStableID(t *testing.T) {
	type tcase struct {
		scenario   string
		beforeName string
		afterName  string
		expect     stableIDExpectation
	}

	cases := []tcase{
		{scenario: "whitespace_edit_preserves_id", beforeName: "Greeter", afterName: "Greeter", expect: expPreserved},
		{scenario: "comment_edit_preserves_id", beforeName: "Runner", afterName: "Runner", expect: expPreserved},
		{scenario: "body_only_edit_preserves_id", beforeName: "Worker", afterName: "Worker", expect: expPreserved},
		{scenario: "rename_churns_id", beforeName: "OldName", afterName: "NewName", expect: expChurned},
		{scenario: "method_rename_churns_id", beforeName: "Processor", afterName: "Processor", expect: expPreserved},
	}

	registry := testutil.NewTestRegistry(t)
	p := NewProvider(registry).(*Provider)

	for _, tc := range cases {
		tc := tc
		t.Run(tc.scenario, func(t *testing.T) {
			beforeSrc, err := os.ReadFile(filepath.Join(testdataDir, tc.scenario, "before.rb"))
			if err != nil {
				t.Fatalf("read before: %v", err)
			}
			afterSrc, err := os.ReadFile(filepath.Join(testdataDir, tc.scenario, "after.rb"))
			if err != nil {
				t.Fatalf("read after: %v", err)
			}
			// Use the same logical path so FilePathFallback (for "public" visibility)
			// does not churn the ID across before/after comparisons.
			commonPath := filepath.Join(tc.scenario, "test.rb")
			beforeFile := extract.SourceFile{Path: commonPath, Language: "ruby"}
			afterFile := extract.SourceFile{Path: commonPath, Language: "ruby"}

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

	// method_rename_churns_id: assert that the method ID churns while the
	// class ID is preserved. This is a separate subtest because it checks
	// two symbols with different expectations.
	t.Run("method_rename_churns_method_id", func(t *testing.T) {
		beforeSrc, err := os.ReadFile(filepath.Join(testdataDir, "method_rename_churns_id", "before.rb"))
		if err != nil {
			t.Fatalf("read before: %v", err)
		}
		afterSrc, err := os.ReadFile(filepath.Join(testdataDir, "method_rename_churns_id", "after.rb"))
		if err != nil {
			t.Fatalf("read after: %v", err)
		}
		commonPath := filepath.Join("method_rename_churns_id", "test.rb")
		beforeEf, err := p.Extract(context.Background(), beforeSrc, extract.SourceFile{Path: commonPath, Language: "ruby"})
		if err != nil {
			t.Fatalf("Extract before: %v", err)
		}
		afterEf, err := p.Extract(context.Background(), afterSrc, extract.SourceFile{Path: commonPath, Language: "ruby"})
		if err != nil {
			t.Fatalf("Extract after: %v", err)
		}

		oldMethodID := findSymbolID(beforeEf.Symbols, "old_process")
		newMethodID := findSymbolID(afterEf.Symbols, "new_process")
		if oldMethodID == 0 || newMethodID == 0 {
			t.Fatalf("method symbols not found; before=%v after=%v",
				namesOf(beforeEf.Symbols), namesOf(afterEf.Symbols))
		}
		if oldMethodID == newMethodID {
			t.Errorf("expected method ID churned (old_process -> new_process), but both = %v", oldMethodID)
		}
	})
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

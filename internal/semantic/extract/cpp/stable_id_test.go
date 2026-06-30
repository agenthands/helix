package cppextract

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
	expRemoved      stableIDExpectation = "removed"
)

// TestStableID exercises stable-ID transitions per PLAN 59-04 for C++.
// Each scenario has a before/after fixture; we extract from both with a
// common file path (simulating edits to the same C++ file) and assert
// the named symbol's ID exhibits the expected transition.
//
// Coverage:
//   - whitespace_edit_preserves_id
//   - comment_edit_preserves_id
//   - body_only_edit_preserves_id
//   - rename_churns_id
//   - signature_change_churns_id
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
		{scenario: "rename_churns_id", beforeName: "oldName", afterName: "newName", expect: expChurned},
		{scenario: "signature_change_churns_id", beforeName: "f", afterName: "f", expect: expChurned},
	}

	registry := testutil.NewTestRegistry(t)
	p := NewProvider(registry).(*Provider)

	for _, tc := range cases {
		tc := tc
		t.Run(tc.scenario, func(t *testing.T) {
			beforeSrc, err := os.ReadFile(filepath.Join(testdataDir, tc.scenario, "before.cpp"))
			if err != nil {
				t.Fatalf("read before: %v", err)
			}
			afterSrc, err := os.ReadFile(filepath.Join(testdataDir, tc.scenario, "after.cpp"))
			if err != nil {
				t.Fatalf("read after: %v", err)
			}
			// Use a common file path for both extractions — the stable-ID
			// contract tests behaviour under edits to the same file. C++
			// uses Visibility: "global" which embeds relPath into the
			// key; using distinct before/after paths would churn IDs
			// even for whitespace-only edits.
			commonPath := filepath.Join(tc.scenario, "file.cpp")
			beforeEf, err := p.Extract(context.Background(), beforeSrc, extract.SourceFile{
				Path: commonPath, Language: "cpp",
			})
			if err != nil {
				t.Fatalf("Extract before: %v", err)
			}
			afterEf, err := p.Extract(context.Background(), afterSrc, extract.SourceFile{
				Path: commonPath, Language: "cpp",
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

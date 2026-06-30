package csharpextract

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/agenthands/helix/internal/semantic"
	"github.com/agenthands/helix/internal/semantic/extract"
	"github.com/agenthands/helix/internal/semantic/extract/testutil"
)

// stableIDExpectation mirrors the Go extractor's per CONTEXT.md D-03.
type stableIDExpectation string

const (
	expPreserved    stableIDExpectation = "preserved"
	expChurned      stableIDExpectation = "churned"
	expNewlyDefined stableIDExpectation = "newly_defined"
	expRemoved      stableIDExpectation = "removed"
)

// TestStableID exercises the per-language stable-ID transitions.
// Each scenario has a before/after fixture; we extract from
// both and assert the named symbol's ID exhibits the expected transition.
//
// Coverage (≥5):
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
		{scenario: "whitespace_edit_preserves_id", beforeName: "Calc", afterName: "Calc", expect: expPreserved},
		{scenario: "comment_edit_preserves_id", beforeName: "Processor", afterName: "Processor", expect: expPreserved},
		{scenario: "body_only_edit_preserves_id", beforeName: "Counter", afterName: "Counter", expect: expPreserved},
		{scenario: "rename_churns_id", beforeName: "OldName", afterName: "NewName", expect: expChurned},
		{scenario: "signature_change_churns_id", beforeName: "Process", afterName: "Process", expect: expChurned},
	}

	registry := testutil.NewTestRegistry(t)
	p := NewProvider(registry).(*Provider)

	for _, tc := range cases {
		tc := tc
		t.Run(tc.scenario, func(t *testing.T) {
			beforeSrc, err := os.ReadFile(filepath.Join(testdataDir, tc.scenario, "before.cs"))
			if err != nil {
				t.Fatalf("read before: %v", err)
			}
			afterSrc, err := os.ReadFile(filepath.Join(testdataDir, tc.scenario, "after.cs"))
			if err != nil {
				t.Fatalf("read after: %v", err)
			}

			beforeFile := extract.SourceFile{Path: filepath.Join(tc.scenario, "before.cs"), Language: "c_sharp"}
			afterFile := extract.SourceFile{Path: filepath.Join(tc.scenario, "after.cs"), Language: "c_sharp"}

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
}

// TestBuildProviderKey_ExportedRenameStable directly exercises
// extract.BuildProviderKey to prove the EXTRACT-02 invariant for exported
// symbols even when the provider's Extract method is bypassed.
func TestBuildProviderKey_ExportedRenameStable(t *testing.T) {
	registry := testutil.NewTestRegistry(t)
	_ = NewProvider(registry).(*Provider)

	// C# class rename: the class itself is exported.
	a := extract.BuildProviderKey(extract.SymbolMeta{
		Language: "c_sharp", QualifiedName: "OldName",
		Kind: "class", Visibility: "exported",
	})
	b := extract.BuildProviderKey(extract.SymbolMeta{
		Language: "c_sharp", QualifiedName: "NewName",
		Kind: "class", Visibility: "exported",
	})
	if extract.StableSymbolID(a) == extract.StableSymbolID(b) {
		t.Errorf("expected different IDs for OldName vs NewName")
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
	names := make([]string, len(syms))
	for i, s := range syms {
		names[i] = s.Name
	}
	return names
}

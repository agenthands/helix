//go:build cgo

package goextract

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

// TestStableID exercises the per-language stable-ID transitions enumerated
// in PLAN 59-04. Each scenario has a before/after fixture; we extract from
// both and assert the named symbol's ID exhibits the expected transition.
//
// Coverage (≥10):
//   - whitespace_edit_preserves_id
//   - comment_edit_preserves_id
//   - body_only_edit_preserves_id
//   - move_within_file_preserves_id
//   - rename_churns_id
//   - signature_change_churns_id
//   - receiver_pointer_to_value_churns_id (Go-specific)
//   - same_content_file_rename_preserves_id (uses two distinct file paths)
//   - move_exported_within_package_preserves_id
//   - method_on_receiver_rename_churns_id
//   - generic_type_param_rename_preserves_id
func TestStableID(t *testing.T) {
	type tcase struct {
		scenario  string
		beforeName string
		afterName  string
		expect    stableIDExpectation
		// alternate file paths for before/after — used by
		// same_content_file_rename_preserves_id to prove BuildProviderKey's
		// EXTRACT-02 invariant.
		beforePath string
		afterPath  string
	}

	cases := []tcase{
		{scenario: "whitespace_edit_preserves_id", beforeName: "Add", afterName: "Add", expect: expPreserved},
		{scenario: "comment_edit_preserves_id", beforeName: "F", afterName: "F", expect: expPreserved},
		{scenario: "body_only_edit_preserves_id", beforeName: "F", afterName: "F", expect: expPreserved},
		{scenario: "move_within_file_preserves_id", beforeName: "A", afterName: "A", expect: expPreserved},
		{scenario: "rename_churns_id", beforeName: "OldName", afterName: "NewName", expect: expChurned},
		{scenario: "signature_change_churns_id", beforeName: "F", afterName: "F", expect: expChurned},
		{scenario: "receiver_pointer_to_value_churns_id", beforeName: "M", afterName: "M", expect: expChurned},
		{
			scenario:   "same_content_file_rename_preserves_id",
			beforeName: "Public", afterName: "Public", expect: expPreserved,
			beforePath: "pkg/foo.go", afterPath: "pkg/bar.go", // same package, different file
		},
		{scenario: "move_exported_within_package_preserves_id", beforeName: "New", afterName: "New", expect: expPreserved},
		{scenario: "method_on_receiver_rename_churns_id", beforeName: "Old", afterName: "New", expect: expChurned},
		{scenario: "generic_type_param_rename_preserves_id", beforeName: "F", afterName: "F", expect: expPreserved},
	}

	registry := testutil.NewTestRegistry(t)
	p := NewProvider(registry).(*Provider)

	for _, tc := range cases {
		tc := tc
		t.Run(tc.scenario, func(t *testing.T) {
			beforeSrc, err := os.ReadFile(filepath.Join(testdataDir, tc.scenario, "before.go"))
			if err != nil {
				t.Fatalf("read before: %v", err)
			}
			afterSrc, err := os.ReadFile(filepath.Join(testdataDir, tc.scenario, "after.go"))
			if err != nil {
				t.Fatalf("read after: %v", err)
			}
			beforeFile := extract.SourceFile{Path: filepath.Join(tc.scenario, "before.go"), Language: "go"}
			afterFile := extract.SourceFile{Path: filepath.Join(tc.scenario, "after.go"), Language: "go"}
			if tc.beforePath != "" {
				beforeFile.Path = tc.beforePath
			}
			if tc.afterPath != "" {
				afterFile.Path = tc.afterPath
			}
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
// symbols even when the provider's Extract method is bypassed. Mirrors
// the W5 mandate in PLAN 59-04.
func TestBuildProviderKey_ExportedRenameStable(t *testing.T) {
	a := extract.BuildProviderKey(extract.SymbolMeta{
		Language: "go", PackagePath: "pkg", QualifiedName: "Public",
		Kind: "function", SignatureHash: "()int", RelPath: "foo.go", Visibility: "exported",
	})
	b := extract.BuildProviderKey(extract.SymbolMeta{
		Language: "go", PackagePath: "pkg", QualifiedName: "Public",
		Kind: "function", SignatureHash: "()int", RelPath: "bar.go", Visibility: "exported",
	})
	if extract.StableSymbolID(a) != extract.StableSymbolID(b) {
		t.Errorf("exported same-content rename: IDs differ\n  a=%+v -> %v\n  b=%+v -> %v",
			a, extract.StableSymbolID(a), b, extract.StableSymbolID(b))
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

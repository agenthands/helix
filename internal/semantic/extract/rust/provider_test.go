package rustextract

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/agenthands/helix/internal/semantic/extract"
	"github.com/agenthands/helix/internal/semantic/extract/testutil"
)

const testdataDir = "testdata"

// TestProvider_Golden table-drives golden comparison over every scenario
// directory under testdata/. For each scenario:
//  1. Read before.rs.
//  2. Run provider Extract.
//  3. Compare normalized output against expected.json.
//
// If `-update` is passed, expected.json is regenerated from the actual emit.
func TestProvider_Golden(t *testing.T) {
	registry := testutil.NewTestRegistry(t)
	p := NewProvider(registry).(*Provider)
	scenarios := testutil.ListScenarios(t, testdataDir)
	if len(scenarios) < 10 {
		t.Fatalf("expected at least 10 scenarios under %s, got %d", testdataDir, len(scenarios))
	}
	for _, sc := range scenarios {
		sc := sc
		t.Run(sc, func(t *testing.T) {
			beforePath, _ := testutil.FindBefore(t, testdataDir, sc)
			src, err := os.ReadFile(beforePath)
			if err != nil {
				t.Fatalf("read %s: %v", beforePath, err)
			}
			ef, err := p.Extract(context.Background(), src, extract.SourceFile{
				Path:     filepath.Join(sc, filepath.Base(beforePath)),
				Language: "rust",
			})
			if err != nil {
				t.Fatalf("Extract: %v", err)
			}
			testutil.GoldenCompare(t, testdataDir, sc, ef)

			if afterPath := testutil.FindAfter(testdataDir, sc); afterPath != "" {
				asrc, err := os.ReadFile(afterPath)
				if err != nil {
					t.Fatalf("read %s: %v", afterPath, err)
				}
				af, err := p.Extract(context.Background(), asrc, extract.SourceFile{
					Path:     filepath.Join(sc, filepath.Base(afterPath)),
					Language: "rust",
				})
				if err != nil {
					t.Fatalf("Extract(after): %v", err)
				}
				testutil.GoldenCompareAfter(t, testdataDir, sc, af)
			}
		})
	}
}

// TestProvider_Determinism ensures two consecutive extractions on the same
// source produce byte-identical normalized output. Locks the SignatureHash
// + sort-order discipline.
func TestProvider_Determinism(t *testing.T) {
	registry := testutil.NewTestRegistry(t)
	p := NewProvider(registry).(*Provider)
	src, err := os.ReadFile(filepath.Join(testdataDir, "function_basic", "before.rs"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	a, err := p.Extract(context.Background(), src, extract.SourceFile{Path: "function_basic/before.rs", Language: "rust"})
	if err != nil {
		t.Fatalf("Extract A: %v", err)
	}
	b, err := p.Extract(context.Background(), src, extract.SourceFile{Path: "function_basic/before.rs", Language: "rust"})
	if err != nil {
		t.Fatalf("Extract B: %v", err)
	}
	an, err := testutil.NormalizeForGolden(a)
	if err != nil {
		t.Fatalf("normalize a: %v", err)
	}
	bn, err := testutil.NormalizeForGolden(b)
	if err != nil {
		t.Fatalf("normalize b: %v", err)
	}
	if string(an) != string(bn) {
		t.Errorf("non-deterministic emit:\n--- a ---\n%s\n--- b ---\n%s", an, bn)
	}
}

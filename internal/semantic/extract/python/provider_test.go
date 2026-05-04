//go:build cgo

package pyextract

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/agenthands/helix/internal/semantic/extract"
	"github.com/agenthands/helix/internal/semantic/extract/testutil"
)

const testdataDir = "testdata"

func TestProvider_Golden(t *testing.T) {
	registry := testutil.NewTestRegistry(t)
	p := NewProvider(registry).(*Provider)
	scenarios := testutil.ListScenarios(t, testdataDir)
	if len(scenarios) < 30 {
		t.Fatalf("expected at least 30 scenarios under %s, got %d", testdataDir, len(scenarios))
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
				Language: "python",
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
					Language: "python",
				})
				if err != nil {
					t.Fatalf("Extract(after): %v", err)
				}
				testutil.GoldenCompareAfter(t, testdataDir, sc, af)
			}
		})
	}
}

func TestProvider_Determinism(t *testing.T) {
	registry := testutil.NewTestRegistry(t)
	p := NewProvider(registry).(*Provider)
	src, err := os.ReadFile(filepath.Join(testdataDir, "function_basic", "before.py"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	a, err := p.Extract(context.Background(), src, extract.SourceFile{Path: "function_basic/before.py", Language: "python"})
	if err != nil {
		t.Fatalf("Extract A: %v", err)
	}
	b, err := p.Extract(context.Background(), src, extract.SourceFile{Path: "function_basic/before.py", Language: "python"})
	if err != nil {
		t.Fatalf("Extract B: %v", err)
	}
	an, _ := testutil.NormalizeForGolden(a)
	bn, _ := testutil.NormalizeForGolden(b)
	if string(an) != string(bn) {
		t.Errorf("non-deterministic emit:\n--- a ---\n%s\n--- b ---\n%s", an, bn)
	}
}

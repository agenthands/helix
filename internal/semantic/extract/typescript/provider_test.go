package tsextract

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/agenthands/helix/internal/semantic/extract"
	"github.com/agenthands/helix/internal/semantic/extract/testutil"
)

const testdataDir = "testdata"

// TestProvider_Golden — table-driven golden over every scenario directory.
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
				Language: "typescript",
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
					Language: "typescript",
				})
				if err != nil {
					t.Fatalf("Extract(after): %v", err)
				}
				testutil.GoldenCompareAfter(t, testdataDir, sc, af)
			}
		})
	}
}

// TestProvider_Determinism — twice on same input → byte-identical emit.
func TestProvider_Determinism(t *testing.T) {
	registry := testutil.NewTestRegistry(t)
	p := NewProvider(registry).(*Provider)
	src, err := os.ReadFile(filepath.Join(testdataDir, "function_basic", "before.ts"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	a, err := p.Extract(context.Background(), src, extract.SourceFile{Path: "function_basic/before.ts", Language: "typescript"})
	if err != nil {
		t.Fatalf("Extract A: %v", err)
	}
	b, err := p.Extract(context.Background(), src, extract.SourceFile{Path: "function_basic/before.ts", Language: "typescript"})
	if err != nil {
		t.Fatalf("Extract B: %v", err)
	}
	an, _ := testutil.NormalizeForGolden(a)
	bn, _ := testutil.NormalizeForGolden(b)
	if string(an) != string(bn) {
		t.Errorf("non-deterministic emit:\n--- a ---\n%s\n--- b ---\n%s", an, bn)
	}
}

// TestProvider_ReceiverCapture — a member call lib.foo() stamps the
// receiver ("lib") on the call reference (backs CROSS_CALLS edges that
// match a call receiver to an external import); a bare foo() leaves it
// empty.
func TestProvider_ReceiverCapture(t *testing.T) {
	registry := testutil.NewTestRegistry(t)
	p := NewProvider(registry).(*Provider)
	src := []byte("function run(): void {\n  lib.foo();\n  foo();\n}\n")
	ef, err := p.Extract(context.Background(), src, extract.SourceFile{Path: "recv.ts", Language: "typescript"})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	var member, bare *extract.ReferenceFact
	for i := range ef.References {
		r := &ef.References[i]
		if r.Kind != extract.ReferenceKind("call") || r.Name != "foo" {
			continue
		}
		switch r.ReceiverText {
		case "lib":
			member = r
		case "":
			bare = r
		}
	}
	if member == nil {
		t.Errorf("member call lib.foo(): want reference.call with ReceiverText %q, found none", "lib")
	}
	if bare == nil {
		t.Errorf("bare call foo(): want reference.call with empty ReceiverText, found none")
	}
}

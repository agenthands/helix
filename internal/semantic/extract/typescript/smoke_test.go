package tsextract

import (
	"context"
	"testing"

	"github.com/agenthands/helix/internal/semantic/extract"
	"github.com/agenthands/helix/internal/semantic/extract/testutil"
)

// TestSmoke_ProviderConstructs — query compiles against both grammars.
func TestSmoke_ProviderConstructs(t *testing.T) {
	registry := testutil.NewTestRegistry(t)
	p := NewProvider(registry)
	if p.Language() != "typescript" {
		t.Fatalf("Language() = %q, want typescript", p.Language())
	}
	exts := p.Extensions()
	want := map[string]bool{".ts": true, ".tsx": true, ".js": true, ".jsx": true, ".mjs": true, ".cjs": true}
	got := map[string]bool{}
	for _, e := range exts {
		got[e] = true
	}
	for w := range want {
		if !got[w] {
			t.Errorf("missing extension %q", w)
		}
	}
}

func TestSmoke_ExtractsBasicFunction(t *testing.T) {
	registry := testutil.NewTestRegistry(t)
	p := NewProvider(registry).(*Provider)
	src := []byte(`function hello(): string { return "hi"; }
`)
	ef, err := p.Extract(context.Background(), src, extract.SourceFile{Path: "main.ts", Language: "typescript"})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	found := false
	for _, s := range ef.Symbols {
		if s.Name == "hello" && s.Kind == extract.KindFunction {
			found = true
		}
	}
	if !found {
		t.Errorf("symbol hello not found in: %+v", ef.Symbols)
	}
}

func TestSmoke_TSXGrammarDispatch(t *testing.T) {
	registry := testutil.NewTestRegistry(t)
	p := NewProvider(registry).(*Provider)
	src := []byte(`function App(): JSX.Element { return <div>hi</div>; }
`)
	ef, err := p.Extract(context.Background(), src, extract.SourceFile{Path: "App.tsx", Language: "typescript"})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	found := false
	for _, s := range ef.Symbols {
		if s.Name == "App" {
			found = true
		}
	}
	if !found {
		t.Errorf("App not found in tsx parse: %+v", ef.Symbols)
	}
}

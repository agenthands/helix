package pyextract

import (
	"context"
	"testing"

	"github.com/agenthands/helix/internal/semantic/extract"
	"github.com/agenthands/helix/internal/semantic/extract/testutil"
)

func TestSmoke_ProviderConstructs(t *testing.T) {
	registry := testutil.NewTestRegistry(t)
	p := NewProvider(registry)
	if p.Language() != "python" {
		t.Fatalf("Language() = %q, want python", p.Language())
	}
	if got := p.Extensions(); len(got) != 1 || got[0] != ".py" {
		t.Fatalf("Extensions() = %v, want [.py]", got)
	}
}

func TestSmoke_ExtractsBasicFunction(t *testing.T) {
	registry := testutil.NewTestRegistry(t)
	p := NewProvider(registry).(*Provider)
	src := []byte("def hello():\n    return 'hi'\n")
	ef, err := p.Extract(context.Background(), src, extract.SourceFile{Path: "main.py", Language: "python"})
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
		t.Errorf("symbol hello not found: %+v", ef.Symbols)
	}
}

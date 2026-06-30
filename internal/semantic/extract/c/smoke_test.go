package cextract

import (
	"context"
	"testing"

	"github.com/agenthands/helix/internal/semantic/extract"
	"github.com/agenthands/helix/internal/semantic/extract/testutil"
)

func TestSmoke_ProviderConstructs(t *testing.T) {
	registry := testutil.NewTestRegistry(t)
	p := NewProvider(registry)
	if p.Language() != "c" {
		t.Fatalf("Language() = %q, want %q", p.Language(), "c")
	}
	if got := p.Extensions(); len(got) != 2 || got[0] != ".c" || got[1] != ".h" {
		t.Fatalf("Extensions() = %v, want [.c .h]", got)
	}
	if !p.SupportsLSPEnrichment() {
		t.Fatalf("SupportsLSPEnrichment() = false, want true")
	}
}

func TestSmoke_ExtractsBasicFunction(t *testing.T) {
	registry := testutil.NewTestRegistry(t)
	p := NewProvider(registry).(*Provider)
	src := []byte(`int hello(void) { return 42; }`)
	ef, err := p.Extract(context.Background(), src, extract.SourceFile{Path: "main.c", Language: "c"})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if ef == nil {
		t.Fatalf("ef == nil")
	}
	if ef.File.ExtractionStatus != extract.ExtractionStatusReady {
		t.Errorf("status = %q, want %q", ef.File.ExtractionStatus, extract.ExtractionStatusReady)
	}
	foundFn := false
	for _, s := range ef.Symbols {
		if s.Name == "hello" && s.Kind == extract.KindFunction {
			foundFn = true
		}
	}
	if !foundFn {
		t.Errorf("did not find function hello in: %+v", ef.Symbols)
	}
}

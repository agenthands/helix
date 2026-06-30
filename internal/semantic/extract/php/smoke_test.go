package phpextract

import (
	"context"
	"testing"

	"github.com/agenthands/helix/internal/semantic/extract"
	"github.com/agenthands/helix/internal/semantic/extract/testutil"
)

func TestSmoke_ProviderConstructs(t *testing.T) {
	registry := testutil.NewTestRegistry(t)
	p := NewProvider(registry)
	if p.Language() != "php" {
		t.Fatalf("Language() = %q, want %q", p.Language(), "php")
	}
	if got := p.Extensions(); len(got) != 1 || got[0] != ".php" {
		t.Fatalf("Extensions() = %v, want [.php]", got)
	}
	if !p.SupportsLSPEnrichment() {
		t.Fatalf("SupportsLSPEnrichment() = false, want true")
	}
}

func TestSmoke_ExtractsBasicClass(t *testing.T) {
	registry := testutil.NewTestRegistry(t)
	p := NewProvider(registry).(*Provider)
	src := []byte(`<?php
class Hello {
    public function greet(): string { return "hi"; }
}`)
	ef, err := p.Extract(context.Background(), src, extract.SourceFile{Path: "Hello.php", Language: "php"})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if ef == nil {
		t.Fatalf("ef == nil")
	}
	if ef.File.ExtractionStatus != extract.ExtractionStatusReady {
		t.Errorf("status = %q, want %q", ef.File.ExtractionStatus, extract.ExtractionStatusReady)
	}
	foundClass := false
	for _, s := range ef.Symbols {
		if s.Name == "Hello" && s.Kind == extract.KindClass {
			foundClass = true
		}
	}
	if !foundClass {
		t.Errorf("did not find class Hello in: %+v", ef.Symbols)
	}
}

//go:build cgo

package goextract

import (
	"context"
	"testing"

	"github.com/agenthands/helix/internal/semantic/extract"
	"github.com/agenthands/helix/internal/semantic/extract/testutil"
)

// TestSmoke_ProviderConstructs verifies the embedded queries compile
// against the injected grammar. Acts as an early-warning canary for
// query-syntax regressions.
func TestSmoke_ProviderConstructs(t *testing.T) {
	registry := testutil.NewTestRegistry(t)
	p := NewProvider(registry)
	if p.Language() != "go" {
		t.Fatalf("Language() = %q, want %q", p.Language(), "go")
	}
	if got := p.Extensions(); len(got) != 1 || got[0] != ".go" {
		t.Fatalf("Extensions() = %v, want [.go]", got)
	}
	if !p.SupportsLSPEnrichment() {
		t.Fatalf("SupportsLSPEnrichment() = false, want true")
	}
}

// TestSmoke_ExtractsBasicFunction verifies the Extract method returns
// a non-nil ExtractedFile and finds at least one symbol for a trivial
// Go source.
func TestSmoke_ExtractsBasicFunction(t *testing.T) {
	registry := testutil.NewTestRegistry(t)
	p := NewProvider(registry).(*Provider)
	src := []byte(`package main

func Hello() string { return "hi" }
`)
	ef, err := p.Extract(context.Background(), src, extract.SourceFile{Path: "main.go", Language: "go"})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if ef == nil {
		t.Fatalf("ef == nil")
	}
	if ef.File.ExtractionStatus != extract.ExtractionStatusReady {
		t.Errorf("status = %q, want %q", ef.File.ExtractionStatus, extract.ExtractionStatusReady)
	}
	foundHello := false
	for _, s := range ef.Symbols {
		if s.Name == "Hello" && s.Kind == extract.KindFunction {
			foundHello = true
			if s.Confidence != extract.ConfidenceTSOnly {
				t.Errorf("confidence = %v, want %v", s.Confidence, extract.ConfidenceTSOnly)
			}
			if s.Visibility != "exported" {
				t.Errorf("visibility = %q, want exported", s.Visibility)
			}
		}
	}
	if !foundHello {
		t.Errorf("did not find symbol Hello in: %+v", ef.Symbols)
	}
}

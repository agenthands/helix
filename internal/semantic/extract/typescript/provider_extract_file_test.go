package tsextract

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/agenthands/helix/internal/semantic/extract"
	"github.com/agenthands/helix/internal/semantic/extract/testutil"
)

// TestExtractFile_HappyPath_TS exercises the Phase 68 D-03 ExtractFile
// shim for the TypeScript provider: file is read from disk and delegated
// to Extract.
func TestExtractFile_HappyPath_TS(t *testing.T) {
	t.Parallel()
	registry := testutil.NewTestRegistry(t)
	p := NewProvider(registry).(*Provider)

	dir := t.TempDir()
	path := filepath.Join(dir, "hello.ts")
	src := []byte("export function hello(): string { return \"v1\"; }\n")
	if err := os.WriteFile(path, src, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	ef, err := p.ExtractFile(context.Background(), "repo-1", path)
	if err != nil {
		t.Fatalf("ExtractFile: %v", err)
	}
	if ef == nil {
		t.Fatal("ExtractFile returned nil ExtractedFile")
	}
	if ef.File.ExtractionStatus != extract.ExtractionStatusReady {
		t.Errorf("ExtractionStatus = %q, want %q", ef.File.ExtractionStatus, extract.ExtractionStatusReady)
	}
	if len(ef.Symbols) == 0 {
		t.Fatal("expected at least one symbol")
	}
	foundHello := false
	for _, s := range ef.Symbols {
		if s.Name == "hello" {
			foundHello = true
			break
		}
	}
	if !foundHello {
		t.Errorf("expected symbol hello, got %+v", ef.Symbols)
	}
}

// TestExtractFile_FileNotFound_TS asserts os.ReadFile failure surfaces as
// a partial-status ExtractedFile, not a Go error.
func TestExtractFile_FileNotFound_TS(t *testing.T) {
	t.Parallel()
	registry := testutil.NewTestRegistry(t)
	p := NewProvider(registry).(*Provider)

	bogus := filepath.Join(t.TempDir(), "does-not-exist.ts")

	ef, err := p.ExtractFile(context.Background(), "repo-1", bogus)
	if err != nil {
		t.Fatalf("ExtractFile returned err (expected partial, not error): %v", err)
	}
	if ef == nil {
		t.Fatal("ExtractFile returned nil ExtractedFile on missing file (expected partial)")
	}
	if ef.File.ExtractionStatus != extract.ExtractionStatusPartial {
		t.Errorf("ExtractionStatus = %q, want %q", ef.File.ExtractionStatus, extract.ExtractionStatusPartial)
	}
	if ef.File.PartialReason != extract.PartialReasonPermissionDenied {
		t.Errorf("PartialReason = %q, want %q", ef.File.PartialReason, extract.PartialReasonPermissionDenied)
	}
	if ef.File.ErrorMessage == "" {
		t.Error("expected non-empty ErrorMessage on missing file")
	}
}

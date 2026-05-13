package goextract

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/agenthands/helix/internal/semantic/extract"
	"github.com/agenthands/helix/internal/semantic/extract/testutil"
)

// TestExtractFile_HappyPath_Go exercises the Phase 68 D-03 ExtractFile
// shim: the provider reads `path` from disk and delegates to Extract,
// returning a Ready ExtractedFile when the file parses cleanly.
func TestExtractFile_HappyPath_Go(t *testing.T) {
	t.Parallel()
	registry := testutil.NewTestRegistry(t)
	p := NewProvider(registry).(*Provider)

	dir := t.TempDir()
	path := filepath.Join(dir, "hello.go")
	src := []byte("package x\n\nfunc Hello() string { return \"v1\" }\n")
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
		if s.Name == "Hello" {
			foundHello = true
			break
		}
	}
	if !foundHello {
		t.Errorf("expected symbol Hello, got %+v", ef.Symbols)
	}
}

// TestExtractFile_FileNotFound_Go asserts that an os.ReadFile failure
// returns a partial-status ExtractedFile rather than a Go error (Phase 68
// partialFile convention).
func TestExtractFile_FileNotFound_Go(t *testing.T) {
	t.Parallel()
	registry := testutil.NewTestRegistry(t)
	p := NewProvider(registry).(*Provider)

	bogus := filepath.Join(t.TempDir(), "does-not-exist.go")

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

// TestExtractFile_RepoIDIgnoredByGo asserts the repoID argument is not
// consumed by the Go provider — behavior is identical with "" and a
// nonempty repo identifier.
func TestExtractFile_RepoIDIgnoredByGo(t *testing.T) {
	t.Parallel()
	registry := testutil.NewTestRegistry(t)
	p := NewProvider(registry).(*Provider)

	dir := t.TempDir()
	path := filepath.Join(dir, "hello.go")
	src := []byte("package x\n\nfunc Hello() string { return \"v1\" }\n")
	if err := os.WriteFile(path, src, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	ef1, err := p.ExtractFile(context.Background(), "", path)
	if err != nil {
		t.Fatalf("ExtractFile(\"\", ...): %v", err)
	}
	ef2, err := p.ExtractFile(context.Background(), "anything", path)
	if err != nil {
		t.Fatalf("ExtractFile(\"anything\", ...): %v", err)
	}
	if ef1.File.ExtractionStatus != ef2.File.ExtractionStatus {
		t.Errorf("ExtractionStatus differs: %q vs %q",
			ef1.File.ExtractionStatus, ef2.File.ExtractionStatus)
	}
	if len(ef1.Symbols) != len(ef2.Symbols) {
		t.Errorf("symbol count differs: %d vs %d", len(ef1.Symbols), len(ef2.Symbols))
	}
}

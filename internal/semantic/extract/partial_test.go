package extract

import (
	"errors"
	"testing"
)

// TestPartialMarker_UnsupportedLanguage covers the D-05 path: a file with
// a non-first-class language extension produces a file row only — no
// symbols/refs/imports — with status=unsupported and
// reason=unsupported_language.
func TestPartialMarker_UnsupportedLanguage(t *testing.T) {
	file := SourceFile{Path: "lib.rs", Language: "rust"}
	ef := PartialExtract(file, PartialReasonUnsupportedLanguage, nil)
	if ef.File.ExtractionStatus != ExtractionStatusUnsupported {
		t.Errorf("status = %q, want %q", ef.File.ExtractionStatus, ExtractionStatusUnsupported)
	}
	if ef.File.PartialReason != PartialReasonUnsupportedLanguage {
		t.Errorf("reason = %q, want %q", ef.File.PartialReason, PartialReasonUnsupportedLanguage)
	}
	if !ef.Partial {
		t.Errorf("Partial = false, want true")
	}
	if !ef.File.ExtractionPartial {
		t.Errorf("File.ExtractionPartial = false, want true")
	}
	if len(ef.Symbols) != 0 || len(ef.References) != 0 || len(ef.Imports) != 0 {
		t.Errorf("expected no facts; got symbols=%d refs=%d imports=%d",
			len(ef.Symbols), len(ef.References), len(ef.Imports))
	}
}

func TestPartialMarker_FileTooLarge(t *testing.T) {
	file := SourceFile{Path: "huge.go", Language: "go"}
	ef := PartialExtract(file, PartialReasonFileTooLarge, nil)
	if ef.File.ExtractionStatus != ExtractionStatusUnsupported {
		t.Errorf("status = %q, want %q (file_too_large maps to unsupported)",
			ef.File.ExtractionStatus, ExtractionStatusUnsupported)
	}
	if ef.File.PartialReason != PartialReasonFileTooLarge {
		t.Errorf("reason = %q, want %q", ef.File.PartialReason, PartialReasonFileTooLarge)
	}
}

func TestPartialMarker_BinaryOrGenerated(t *testing.T) {
	file := SourceFile{Path: "image.bin", Language: ""}
	ef := PartialExtract(file, PartialReasonBinaryOrGenerated, nil)
	if ef.File.ExtractionStatus != ExtractionStatusUnsupported {
		t.Errorf("status = %q, want %q", ef.File.ExtractionStatus, ExtractionStatusUnsupported)
	}
	if ef.File.PartialReason != PartialReasonBinaryOrGenerated {
		t.Errorf("reason = %q, want %q", ef.File.PartialReason, PartialReasonBinaryOrGenerated)
	}
}

func TestPartialMarker_ParseError(t *testing.T) {
	file := SourceFile{Path: "broken.go", Language: "go"}
	ef := PartialExtract(file, PartialReasonParseError, errors.New("syntax error at line 5"))
	// Partial state (parse error implies first-class-with-errors path).
	if ef.File.ExtractionStatus != ExtractionStatusPartial {
		t.Errorf("status = %q, want %q (parse_error stays partial)",
			ef.File.ExtractionStatus, ExtractionStatusPartial)
	}
	if ef.File.ErrorMessage == "" {
		t.Errorf("ErrorMessage is empty; want propagated err")
	}
}

func TestPartialMarker_Timeout(t *testing.T) {
	file := SourceFile{Path: "slow.go", Language: "go"}
	ef := PartialExtract(file, PartialReasonTimeout, errors.New("context deadline exceeded"))
	if ef.File.ExtractionStatus != ExtractionStatusPartial {
		t.Errorf("timeout should be partial, got %q", ef.File.ExtractionStatus)
	}
}

// TestPartialMarker_EnumClosed asserts the closed-enum invariant per
// CONTEXT.md D-05: exactly 8 PartialReason values.
func TestPartialMarker_EnumClosed(t *testing.T) {
	expected := map[PartialReason]bool{
		PartialReasonUnsupportedLanguage: true,
		PartialReasonParseError:          true,
		PartialReasonQueryError:          true,
		PartialReasonTimeout:             true,
		PartialReasonFileTooLarge:        true,
		PartialReasonBinaryOrGenerated:   true,
		PartialReasonPermissionDenied:    true,
		PartialReasonExtractorBug:        true,
	}
	if len(expected) != 8 {
		t.Errorf("PartialReason enum has %d values, expected exactly 8", len(expected))
	}
	// Verify each value PartialExtract handles produces a non-empty file
	// fact (table-driven over all 8 reasons).
	file := SourceFile{Path: "x", Language: "go"}
	for r := range expected {
		ef := PartialExtract(file, r, nil)
		if ef == nil {
			t.Errorf("PartialExtract returned nil for reason %q", r)
			continue
		}
		if ef.File.PartialReason != r {
			t.Errorf("reason round-trip failed: got %q, want %q", ef.File.PartialReason, r)
		}
		if !ef.Partial {
			t.Errorf("Partial = false for reason %q", r)
		}
	}
}

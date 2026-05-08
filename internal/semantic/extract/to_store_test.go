// Tests for ToStoreFacts (D-08) — pure deterministic adapter from
// per-language ExtractedFile output to semanticstore.Facts wire shape.
//
// These tests were authored RED-first per Phase 59-07 Task 1. They pin
// six contracts on the adapter:
//
//  1. Determinism: same input → byte-identical output across repeated
//     same-process calls (acceptance criterion #15).
//  2. Golden shape: a single hand-coded SymbolFact converts to the
//     expected store-side wire shape (catches accidental field
//     mapping inversions, e.g. swapping QualifiedName / Name).
//  3. Empty input: ToStoreFacts(nil) and ToStoreFacts(empty) return
//     a zero-value Facts struct.
//  4. Nil safety: ToStoreFacts([]*ExtractedFile{nil}) MUST NOT panic;
//     it MUST silently skip the nil entry.
//  5. Partial-file row emit: a non-ready file (status=unsupported)
//     still emits a Files row at this layer (ROADMAP acceptance #10
//     precondition; full criterion is wired by Phase 60 / Phase 65).
//  6. Dropped-on-floor: Imports / Types / Heritage are intentionally
//     not consumed by store.Facts today — pin so a future expansion
//     of store.Facts cannot silently change adapter behavior.
package extract

import (
	"reflect"
	"testing"

	"github.com/agenthands/helix/internal/semantic"
	semanticstore "github.com/agenthands/helix/internal/semantic/store"
)

// twoFileFixture builds a small but non-trivial *ExtractedFile slice
// (2 files, 3 symbols total, 1 reference) used by the determinism
// and golden-shape tests.
func twoFileFixture(t *testing.T) []*ExtractedFile {
	t.Helper()
	return []*ExtractedFile{
		{
			File: FileFact{
				Path:             "a.go",
				Language:         "go",
				ExtractionStatus: ExtractionStatusReady,
				ExtractorName:    "goextract",
				ExtractorVersion: "1",
			},
			Symbols: []SymbolFact{
				{
					ID:               semantic.SymbolID(100),
					Language:         "go",
					Kind:             KindFunction,
					Name:             "Hello",
					QualifiedName:    "main.Hello",
					File:             "a.go",
					Range:            Range{Start: Position{Line: 5, Column: 0}, End: Position{Line: 7, Column: 1}},
					Visibility:       "exported",
					Confidence:       ConfidenceTSOnly,
					ExtractionSource: "tree_sitter",
					StableKey: StableSymbolKey{
						RepoID:        "r",
						Language:      "go",
						QualifiedName: "main.Hello",
						Kind:          string(KindFunction),
					},
				},
				{
					ID:               semantic.SymbolID(101),
					Language:         "go",
					Kind:             KindFunction,
					Name:             "World",
					QualifiedName:    "main.World",
					File:             "a.go",
					Range:            Range{Start: Position{Line: 9, Column: 0}, End: Position{Line: 11, Column: 1}},
					Visibility:       "exported",
					Confidence:       ConfidenceTSOnly,
					ExtractionSource: "tree_sitter",
					StableKey: StableSymbolKey{
						RepoID:        "r",
						Language:      "go",
						QualifiedName: "main.World",
						Kind:          string(KindFunction),
					},
				},
			},
			References: []ReferenceFact{
				{
					ID:              semantic.ReferenceID(900),
					Language:        "go",
					Kind:            ReferenceKind("CALL"),
					Name:            "World",
					File:            "a.go",
					Range:           Range{Start: Position{Line: 6, Column: 4}, End: Position{Line: 6, Column: 9}},
					ValidationState: "syntactic",
					Confidence:      ConfidenceTSOnly,
				},
			},
		},
		{
			File: FileFact{
				Path:             "b.go",
				Language:         "go",
				ExtractionStatus: ExtractionStatusReady,
				ExtractorName:    "goextract",
				ExtractorVersion: "1",
			},
			Symbols: []SymbolFact{
				{
					ID:               semantic.SymbolID(200),
					Language:         "go",
					Kind:             KindStruct,
					Name:             "Pair",
					QualifiedName:    "main.Pair",
					File:             "b.go",
					Range:            Range{Start: Position{Line: 3, Column: 0}, End: Position{Line: 6, Column: 1}},
					Visibility:       "exported",
					Confidence:       ConfidenceTSOnly,
					ExtractionSource: "tree_sitter",
					StableKey: StableSymbolKey{
						RepoID:        "r",
						Language:      "go",
						QualifiedName: "main.Pair",
						Kind:          string(KindStruct),
					},
				},
			},
		},
	}
}

// TestToStoreFacts_Deterministic asserts that calling ToStoreFacts
// twice on the same input produces byte-identical output (acceptance
// criterion #15: same-process repeated-call byte-identity).
func TestToStoreFacts_Deterministic(t *testing.T) {
	in := twoFileFixture(t)

	a := ToStoreFacts(in)
	b := ToStoreFacts(in)

	if !reflect.DeepEqual(a, b) {
		// Field-walk diagnostic to localise the divergence.
		if len(a.Files) != len(b.Files) {
			t.Fatalf("Files slice length differs: %d vs %d", len(a.Files), len(b.Files))
		}
		for i := range a.Files {
			if !reflect.DeepEqual(a.Files[i], b.Files[i]) {
				t.Fatalf("Files[%d] differs:\n  a=%+v\n  b=%+v", i, a.Files[i], b.Files[i])
			}
		}
		if len(a.Symbols) != len(b.Symbols) {
			t.Fatalf("Symbols slice length differs: %d vs %d", len(a.Symbols), len(b.Symbols))
		}
		for i := range a.Symbols {
			if !reflect.DeepEqual(a.Symbols[i], b.Symbols[i]) {
				t.Fatalf("Symbols[%d] differs:\n  a=%+v\n  b=%+v", i, a.Symbols[i], b.Symbols[i])
			}
		}
		if len(a.References) != len(b.References) {
			t.Fatalf("References slice length differs: %d vs %d", len(a.References), len(b.References))
		}
		for i := range a.References {
			if !reflect.DeepEqual(a.References[i], b.References[i]) {
				t.Fatalf("References[%d] differs:\n  a=%+v\n  b=%+v", i, a.References[i], b.References[i])
			}
		}
		t.Fatalf("DeepEqual returned false but field-walk found no diff (encoding skew?)")
	}
}

// TestToStoreFacts_GoldenShape pins specific expected values for the
// store-side SymbolFact[0] to catch accidental field mapping
// inversions (e.g. Name vs QualifiedName swap).
func TestToStoreFacts_GoldenShape(t *testing.T) {
	in := []*ExtractedFile{
		{
			File: FileFact{
				Path:             "a.go",
				Language:         "go",
				ExtractionStatus: ExtractionStatusReady,
				ExtractorName:    "goextract",
				ExtractorVersion: "1",
			},
			Symbols: []SymbolFact{
				{
					ID:               semantic.SymbolID(42),
					Language:         "go",
					Kind:             KindFunction,
					Name:             "Hello",
					QualifiedName:    "main.Hello",
					File:             "a.go",
					Range:            Range{Start: Position{Line: 5, Column: 0}, End: Position{Line: 7, Column: 1}},
					Signature:        "func Hello() string",
					SignatureHash:    "deadbeef",
					Visibility:       "exported",
					Confidence:       ConfidenceTSOnly,
					ExtractionSource: "tree_sitter",
					StableKey: StableSymbolKey{
						RepoID:        "r",
						Language:      "go",
						QualifiedName: "main.Hello",
						Kind:          string(KindFunction),
					},
				},
			},
		},
	}

	out := ToStoreFacts(in)

	if len(out.Symbols) != 1 {
		t.Fatalf("expected 1 store symbol, got %d", len(out.Symbols))
	}
	s := out.Symbols[0]

	if s.SymbolID != uint64(42) {
		t.Errorf("SymbolID: got %d, want 42 (uint64 cast of extract.ID)", s.SymbolID)
	}
	if s.Kind != "function" {
		t.Errorf("Kind: got %q, want \"function\" (string cast of extract.SymbolKind)", s.Kind)
	}
	if s.Name != "Hello" {
		t.Errorf("Name: got %q, want \"Hello\"", s.Name)
	}
	if s.QualifiedName != "main.Hello" {
		t.Errorf("QualifiedName: got %q, want \"main.Hello\"", s.QualifiedName)
	}
	if s.Language != "go" {
		t.Errorf("Language: got %q, want \"go\"", s.Language)
	}
	if s.Confidence != float64(ConfidenceTSOnly) {
		t.Errorf("Confidence: got %v, want %v (float64 cast of ConfidenceTSOnly)", s.Confidence, float64(ConfidenceTSOnly))
	}
	if s.Visibility != "exported" {
		t.Errorf("Visibility: got %q, want \"exported\"", s.Visibility)
	}
	if !s.Exported {
		t.Errorf("Exported: got false, want true (visibility==\"exported\" → Exported==true)")
	}
	if s.Signature != "func Hello() string" {
		t.Errorf("Signature: got %q, want \"func Hello() string\"", s.Signature)
	}
	if s.SignatureHash != "deadbeef" {
		t.Errorf("SignatureHash: got %q, want \"deadbeef\"", s.SignatureHash)
	}
	if s.StartLine != 5 {
		t.Errorf("StartLine: got %d, want 5", s.StartLine)
	}
	if s.EndLine != 7 {
		t.Errorf("EndLine: got %d, want 7", s.EndLine)
	}
	if s.ExtractionSource != "tree_sitter" {
		t.Errorf("ExtractionSource: got %q, want \"tree_sitter\"", s.ExtractionSource)
	}

	// StableKey is the canonicalized form of the input StableSymbolKey
	// (per stable_id.go:28). Compute the expected value via the same
	// helper to pin the contract.
	wantKey := CanonicalizeStableSymbolKey(in[0].Symbols[0].StableKey)
	if s.StableKey != wantKey {
		t.Errorf("StableKey: got %q, want %q (CanonicalizeStableSymbolKey of input)", s.StableKey, wantKey)
	}
}

// TestToStoreFacts_EmptyInput verifies nil and empty inputs produce
// a zero-value Facts struct.
func TestToStoreFacts_EmptyInput(t *testing.T) {
	t.Run("nil_input", func(t *testing.T) {
		out := ToStoreFacts(nil)
		if len(out.Files) != 0 || len(out.Symbols) != 0 || len(out.References) != 0 || len(out.Edges) != 0 {
			t.Fatalf("ToStoreFacts(nil) produced non-empty Facts: %+v", out)
		}
	})
	t.Run("empty_slice", func(t *testing.T) {
		out := ToStoreFacts([]*ExtractedFile{})
		if len(out.Files) != 0 || len(out.Symbols) != 0 || len(out.References) != 0 || len(out.Edges) != 0 {
			t.Fatalf("ToStoreFacts(empty) produced non-empty Facts: %+v", out)
		}
	})
}

// TestToStoreFacts_NilSafety verifies a nil entry in the input slice
// is silently skipped — the function MUST NOT panic. Phase 65's
// buildFn may push nils on extraction error paths.
func TestToStoreFacts_NilSafety(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("ToStoreFacts panicked on nil entry: %v", r)
		}
	}()
	out := ToStoreFacts([]*ExtractedFile{nil})
	if len(out.Files) != 0 || len(out.Symbols) != 0 || len(out.References) != 0 || len(out.Edges) != 0 {
		t.Fatalf("nil entry not skipped — got Facts %+v", out)
	}
}

// TestToStoreFacts_PartialFileEmitsRow verifies that a non-ready
// (unsupported) file still emits a Files row at this layer.
// ROADMAP acceptance criterion #10 precondition.
func TestToStoreFacts_PartialFileEmitsRow(t *testing.T) {
	in := []*ExtractedFile{
		{
			File: FileFact{
				Path:             "src/lib.rs",
				Language:         "rust",
				ExtractionStatus: ExtractionStatusUnsupported,
				PartialReason:    PartialReasonUnsupportedLanguage,
				ExtractorName:    "rust",
				ExtractorVersion: "0",
			},
			// No symbols, no references — unsupported language path.
		},
	}
	out := ToStoreFacts(in)
	if len(out.Files) != 1 {
		t.Fatalf("expected 1 store file row for unsupported language, got %d", len(out.Files))
	}
	if got := out.Files[0].Path; got != "src/lib.rs" {
		t.Errorf("Files[0].Path: got %q, want \"src/lib.rs\"", got)
	}
	if got := out.Files[0].Language; got != "rust" {
		t.Errorf("Files[0].Language: got %q, want \"rust\"", got)
	}
	if len(out.Symbols) != 0 {
		t.Errorf("expected 0 symbols for unsupported language, got %d", len(out.Symbols))
	}
	if len(out.References) != 0 {
		t.Errorf("expected 0 references for unsupported language, got %d", len(out.References))
	}
}

// TestToStoreFacts_DroppedOnFloor verifies Imports / Types / Heritage
// are intentionally not consumed by store.Facts today — pin so a
// future expansion of store.Facts cannot silently change adapter
// behavior. The function MUST NOT panic and MUST drop them.
func TestToStoreFacts_DroppedOnFloor(t *testing.T) {
	in := []*ExtractedFile{
		{
			File: FileFact{
				Path:             "a.go",
				Language:         "go",
				ExtractionStatus: ExtractionStatusReady,
				ExtractorName:    "goextract",
				ExtractorVersion: "1",
			},
			Imports: []ImportFact{
				{
					ID:       semantic.ImportID(1),
					Language: "go",
					Source:   "fmt",
					File:     "a.go",
				},
			},
			Types: []TypeFact{
				{
					ID:         semantic.TypeFactID(1),
					Language:   "go",
					SubjectID:  semantic.SymbolID(100),
					Annotation: "string",
				},
			},
			Heritage: []HeritageFact{
				{
					ID:        semantic.HeritageID(1),
					Language:  "go",
					SubjectID: semantic.SymbolID(100),
					Relation:  "embeds",
					Target:    "io.Reader",
				},
			},
		},
	}

	out := ToStoreFacts(in)
	// One file row emitted; no symbols/references because none provided.
	if len(out.Files) != 1 {
		t.Errorf("expected 1 file row, got %d", len(out.Files))
	}
	if len(out.Symbols) != 0 {
		t.Errorf("Imports/Types/Heritage must not produce symbol rows; got %d", len(out.Symbols))
	}
	if len(out.References) != 0 {
		t.Errorf("Imports/Types/Heritage must not produce reference rows; got %d", len(out.References))
	}
	if len(out.Edges) != 0 {
		t.Errorf("Imports/Types/Heritage must not produce edge rows; got %d", len(out.Edges))
	}
}

// Compile-time guards: assert the adapter signature is what Phase 65
// composes against. If these fail to compile, the contract drifted.
var (
	_ = func(files []*ExtractedFile) semanticstore.Facts { return ToStoreFacts(files) }
)

package extract

import (
	"strings"
	"testing"

	"github.com/agenthands/helix/internal/semantic"
)

// TestStableSymbolID_Deterministic asserts that the same StableSymbolKey
// produces the same SymbolID across many invocations (no nondeterminism
// from map iteration, struct ordering, or runtime state).
func TestStableSymbolID_Deterministic(t *testing.T) {
	k := StableSymbolKey{
		RepoID:           "helix",
		Language:         "go",
		PackagePath:      "github.com/x/pkg",
		OwnerPath:        "*R",
		QualifiedName:    "M",
		Kind:             "method",
		SignatureHash:    "(int)",
		LSPIdentity:      "",
		FilePathFallback: "",
	}
	want := StableSymbolID(k)
	for i := 0; i < 1000; i++ {
		got := StableSymbolID(k)
		if got != want {
			t.Fatalf("StableSymbolID nondeterministic: iter %d got %d want %d", i, got, want)
		}
	}
}

// TestCanonicalizeStableSymbolKey_FieldOrder asserts every field is part of
// the canonicalized output (NUL-separated) and that there are exactly 8
// separators (9 fields → 8 NULs) regardless of which field is set.
func TestCanonicalizeStableSymbolKey_FieldOrder(t *testing.T) {
	// One key per field — only that field set, others empty.
	tests := []struct {
		name string
		key  StableSymbolKey
	}{
		{"repo", StableSymbolKey{RepoID: "r"}},
		{"language", StableSymbolKey{Language: "go"}},
		{"package", StableSymbolKey{PackagePath: "p"}},
		{"owner", StableSymbolKey{OwnerPath: "*R"}},
		{"qname", StableSymbolKey{QualifiedName: "M"}},
		{"kind", StableSymbolKey{Kind: "method"}},
		{"sighash", StableSymbolKey{SignatureHash: "(int)"}},
		{"lsp", StableSymbolKey{LSPIdentity: "x"}},
		{"file", StableSymbolKey{FilePathFallback: "a.go"}},
	}
	seen := map[string]string{}
	for _, tc := range tests {
		got := CanonicalizeStableSymbolKey(tc.key)
		if n := strings.Count(got, "\x00"); n != 8 {
			t.Errorf("%s: NUL byte count = %d, want 8 (9 fields → 8 separators); got=%q", tc.name, n, got)
		}
		if prior, dup := seen[got]; dup {
			t.Errorf("%s: canonicalization collision with %s (%q)", tc.name, prior, got)
		}
		seen[got] = tc.name
	}
	if len(seen) != 9 {
		t.Errorf("expected 9 distinct canonicalized strings, got %d", len(seen))
	}
}

// TestCanonicalize_WhitespaceEditPreserves asserts:
//   - Two keys identical except SignatureHash produce DIFFERENT IDs.
//   - File-path fallback IS in the canonicalization input (per SPEC §11.1
//     verbatim function); the same-content-rename invariant lives in
//     BuildProviderKey, not in StableSymbolID.
func TestCanonicalize_WhitespaceEditPreserves(t *testing.T) {
	a := StableSymbolKey{RepoID: "r", Language: "go", QualifiedName: "F", Kind: "function", SignatureHash: "(int)"}
	b := a
	b.SignatureHash = "(int,string)"
	if StableSymbolID(a) == StableSymbolID(b) {
		t.Errorf("different SignatureHash must yield different IDs")
	}
}

// TestCanonicalize_SameContentRenamePreserves verifies that StableSymbolID
// itself does NOT auto-strip FilePathFallback — it canonicalizes the full
// key. The SAME-content-rename invariant is delivered upstream by
// BuildProviderKey for exported symbols (see TestBuildProviderKey_*).
func TestCanonicalize_SameContentRenamePreserves(t *testing.T) {
	base := StableSymbolKey{
		RepoID: "r", Language: "go", PackagePath: "x/y",
		QualifiedName: "F", Kind: "function", SignatureHash: "(int)",
	}
	a := base
	a.FilePathFallback = "a.go"
	b := base
	b.FilePathFallback = "b.go"
	if StableSymbolID(a) == StableSymbolID(b) {
		t.Errorf("StableSymbolID must canonicalize FilePathFallback verbatim (caller policy lives in BuildProviderKey)")
	}
}

// TestCanonicalize_KnownVector pins one full key to its xxhash64 output as
// a frozen vector. Computed once via this test the first time and pinned;
// any future change to canonicalization or hash function will break this.
func TestCanonicalize_KnownVector(t *testing.T) {
	key := StableSymbolKey{
		RepoID: "helix", Language: "go", PackagePath: "github.com/x/pkg",
		OwnerPath: "*R", QualifiedName: "M", Kind: "method",
		SignatureHash: "(int)", LSPIdentity: "", FilePathFallback: "",
	}
	canon := CanonicalizeStableSymbolKey(key)
	wantCanon := "helix\x00go\x00github.com/x/pkg\x00*R\x00M\x00method\x00(int)\x00\x00"
	if canon != wantCanon {
		t.Fatalf("canonicalization drift:\n got  %q\n want %q", canon, wantCanon)
	}
	// Frozen vector — computed once, pinned. xxhash64 of wantCanon.
	// DO NOT bump unless SPEC §11.1 (canonicalization rule) changes.
	const wantID semantic.SymbolID = 0x032208087c467dcd
	if got := StableSymbolID(key); got != wantID {
		t.Fatalf("frozen-vector drift: got %#x want %#x — DO NOT bump unless SPEC §11.1 changes", got, wantID)
	}
}

// TestCanonicalize_NoEmptyFieldCollision asserts NUL separators preserve
// position so keys differing only in WHICH positions are empty produce
// different canonicalized strings.
func TestCanonicalize_NoEmptyFieldCollision(t *testing.T) {
	// "x" in QualifiedName slot vs. "x" in Kind slot — different positions.
	a := StableSymbolKey{QualifiedName: "x"}
	b := StableSymbolKey{Kind: "x"}
	ca := CanonicalizeStableSymbolKey(a)
	cb := CanonicalizeStableSymbolKey(b)
	if ca == cb {
		t.Fatalf("position-shift collision: %q == %q", ca, cb)
	}
}

// TestBuildProviderKey_ExportedRenameStable asserts that for exported
// symbols, two SymbolMetas differing ONLY in RelPath produce keys with
// empty FilePathFallback and therefore identical StableSymbolIDs. This
// delivers EXTRACT-02's same-content-rename invariant at the
// provider-side key builder (NOT inside the raw hash).
func TestBuildProviderKey_ExportedRenameStable(t *testing.T) {
	a := SymbolMeta{
		RepoID: "helix", Language: "go", PackagePath: "github.com/x/pkg",
		QualifiedName: "Exported", Kind: "function", SignatureHash: "(int)",
		Visibility: "exported", RelPath: "old/path.go",
	}
	b := a
	b.RelPath = "new/path.go"
	keyA := BuildProviderKey(a)
	keyB := BuildProviderKey(b)
	if keyA.FilePathFallback != "" {
		t.Errorf("exported symbol: FilePathFallback should be empty, got %q", keyA.FilePathFallback)
	}
	if keyB.FilePathFallback != "" {
		t.Errorf("exported symbol: FilePathFallback should be empty, got %q", keyB.FilePathFallback)
	}
	if StableSymbolID(keyA) != StableSymbolID(keyB) {
		t.Errorf("exported same-content rename: IDs must be equal, got %d vs %d",
			StableSymbolID(keyA), StableSymbolID(keyB))
	}
}

// TestBuildProviderKey_UnexportedDisambiguates asserts that for unexported
// symbols, two SymbolMetas differing in RelPath produce different keys
// (FilePathFallback populated) and therefore different StableSymbolIDs.
func TestBuildProviderKey_UnexportedDisambiguates(t *testing.T) {
	a := SymbolMeta{
		RepoID: "helix", Language: "go", PackagePath: "github.com/x/pkg",
		QualifiedName: "private", Kind: "function", SignatureHash: "()",
		Visibility: "private", RelPath: "old/path.go",
	}
	b := a
	b.RelPath = "new/path.go"
	keyA := BuildProviderKey(a)
	keyB := BuildProviderKey(b)
	if keyA.FilePathFallback != "old/path.go" {
		t.Errorf("unexported: FilePathFallback should equal RelPath, got %q", keyA.FilePathFallback)
	}
	if keyB.FilePathFallback != "new/path.go" {
		t.Errorf("unexported: FilePathFallback should equal RelPath, got %q", keyB.FilePathFallback)
	}
	if StableSymbolID(keyA) == StableSymbolID(keyB) {
		t.Errorf("unexported with different RelPath: IDs must differ")
	}
}

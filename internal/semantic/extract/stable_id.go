package extract

import (
	"strings"

	"github.com/cespare/xxhash/v2"

	"github.com/agenthands/helix/internal/semantic"
)

// StableSymbolKey carries the 9 fields fed into stable-symbol-ID
// canonicalization per SPEC §11.1. Field order is normative — never
// reorder; new fields append only.
type StableSymbolKey struct {
	RepoID           string
	Language         string
	PackagePath      string
	OwnerPath        string
	QualifiedName    string
	Kind             string
	SignatureHash    string
	LSPIdentity      string
	FilePathFallback string
}

// CanonicalizeStableSymbolKey joins fields with NUL bytes in the SPEC §11.1
// order. Field order is normative — never reorder; new fields append only.
func CanonicalizeStableSymbolKey(k StableSymbolKey) string {
	var b strings.Builder
	b.Grow(256)
	fields := [...]string{
		k.RepoID, k.Language, k.PackagePath, k.OwnerPath,
		k.QualifiedName, k.Kind, k.SignatureHash,
		k.LSPIdentity, k.FilePathFallback,
	}
	for i, f := range fields {
		if i > 0 {
			b.WriteByte(0x00)
		}
		b.WriteString(f)
	}
	return b.String()
}

// StableSymbolID returns xxhash64 of the canonicalized key. SPEC §11.1.
func StableSymbolID(k StableSymbolKey) semantic.SymbolID {
	return semantic.SymbolID(xxhash.Sum64String(CanonicalizeStableSymbolKey(k)))
}

// SymbolMeta is the input shape consumed by BuildProviderKey. Per-language
// providers (P04) assemble a SymbolMeta from tree-sitter capture results
// and call BuildProviderKey to obtain a StableSymbolKey for hashing.
type SymbolMeta struct {
	RepoID, Language, PackagePath, OwnerPath, QualifiedName string
	Kind, SignatureHash, RelPath, Visibility                string
}

// BuildProviderKey constructs a StableSymbolKey from extracted symbol
// metadata. EXTRACT-02's same-content-rename invariant lives HERE (not
// in StableSymbolID itself):
//
//   - For exported symbols (Visibility == "exported") whose QualifiedName
//     is stable across file moves within the same package, FilePathFallback
//     is left empty so renaming the source file does NOT churn the ID.
//   - For unexported symbols (where QualifiedName collisions across files
//     in the same package are possible), FilePathFallback is set to the
//     relative file path so the ID disambiguates.
//
// Callers (P04 per-language providers) MUST use BuildProviderKey rather
// than constructing StableSymbolKey directly.
//
// LSPIdentity is intentionally left empty in Phase 59. Phase 61 may
// populate it during LSP enrichment, at which point the canonicalized
// input changes and the ID will too — by design (see SPEC §11.1 rule 1).
func BuildProviderKey(meta SymbolMeta) StableSymbolKey {
	k := StableSymbolKey{
		RepoID:        meta.RepoID,
		Language:      meta.Language,
		PackagePath:   meta.PackagePath,
		OwnerPath:     meta.OwnerPath,
		QualifiedName: meta.QualifiedName,
		Kind:          meta.Kind,
		SignatureHash: meta.SignatureHash,
		LSPIdentity:   "", // Phase 59 leaves blank; Phase 61 may fill.
	}
	if meta.Visibility == "exported" {
		// Same-content-rename / exported-symbol-move within package preserves ID.
		k.FilePathFallback = ""
	} else {
		// Unexported needs file-level disambiguation.
		k.FilePathFallback = meta.RelPath
	}
	return k
}

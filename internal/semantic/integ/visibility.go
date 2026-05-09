// visibility.go — Visibility closed enum for the semantic integ layer (Phase 66 Plan 02).
//
// Visibility classifies the access level of a symbol as reported by per-language
// extractors (internal/semantic/extract/*/provider.go). The raw extractor string
// is translated to this typed enum via ParseVisibility so downstream rule predicates
// (G-002, G-003) can compare against well-known constants rather than raw strings.
//
// Language coverage:
//   - Go:         uppercase → VisExported; lowercase → VisPackage
//   - TypeScript: export keyword → VisExported; private keyword → VisPrivate
//   - JavaScript: export keyword → VisExported; (no formal access modifiers)
//   - Python:     leading underscore → VisPrivate; otherwise VisExported
//   - Java:       public → VisPublic; protected → VisProtected; private → VisPrivate; default → VisPackage
//   - C#:         public → VisPublic; protected → VisProtected; private → VisPrivate
//   - Rust:       pub → VisExported; pub(crate) → VisPackage; (private) → VisPrivate
//   - Others:     extractor does not emit visibility → VisUnknown (D-19 conservative-warn fallback)
//
// T-66-10 mitigation: ParseVisibility("") → VisUnknown; VisUnknown.IsPublicLike() == false,
// so an absent or unparseable Visibility never grants a public-like classification
// that could bypass G-003.
package integ

// Visibility is a closed enum of symbol access levels derived from per-language
// extractor strings. Use ParseVisibility to construct from raw extractor output.
type Visibility string

const (
	// VisPublic classifies symbols with language-native "public" access
	// (Java/C# public, Rust pub at module root reachable externally).
	VisPublic Visibility = "public"

	// VisExported classifies symbols exported by naming convention or keyword:
	// Go uppercase identifiers, TypeScript/JavaScript "export" keyword,
	// Python symbols without a leading underscore.
	VisExported Visibility = "exported"

	// VisProtected classifies symbols with protected access in class-based
	// languages (Java protected, C# protected).
	VisProtected Visibility = "protected"

	// VisPackage classifies package-private symbols: Java default (no modifier),
	// Go lowercase identifiers, Rust pub(crate).
	VisPackage Visibility = "package"

	// VisPrivate classifies explicitly private symbols: Java/C# private,
	// TypeScript "private" keyword, Python leading-underscore convention,
	// Rust items without pub.
	VisPrivate Visibility = "private"

	// VisUnknown is the fallback for empty strings, unrecognised values, or
	// languages whose extractors do not emit visibility metadata. IsPublicLike()
	// returns false for VisUnknown (T-66-10 conservative-safe default).
	VisUnknown Visibility = "unknown"
)

// IsPublicLike reports whether v is broadly accessible and therefore subject
// to G-003 (public-API-edit) receipt requirements. Returns true for VisPublic,
// VisExported, and VisProtected; false for VisPackage, VisPrivate, and VisUnknown.
//
// VisUnknown deliberately returns false (T-66-10): an unknown visibility must
// never silently grant public-like status. The caller's degraded-mode fallback
// (D-19 conservative-warn) handles the VisUnknown case separately.
func (v Visibility) IsPublicLike() bool {
	return v == VisPublic || v == VisExported || v == VisProtected
}

// ParseVisibility translates a raw extractor visibility string into the
// typed Visibility enum. Unrecognised or empty strings map to VisUnknown.
//
// Canonical extractor strings and their sources:
//
//	"exported"  → VisExported  (Go uppercase, TypeScript/JavaScript export, Python no-underscore)
//	"public"    → VisPublic    (Java public, C# public)
//	"protected" → VisProtected (Java protected, C# protected)
//	"package"   → VisPackage   (Java default, Go lowercase, Rust pub(crate))
//	"private"   → VisPrivate   (Java/C#/TypeScript private, Python leading-underscore)
//	""          → VisUnknown   (extractor did not emit visibility)
//	other       → VisUnknown   (future-proof: unknown extractor value)
func ParseVisibility(raw string) Visibility {
	switch raw {
	case "exported":
		return VisExported
	case "public":
		return VisPublic
	case "protected":
		return VisProtected
	case "package":
		return VisPackage
	case "private":
		return VisPrivate
	default:
		return VisUnknown
	}
}

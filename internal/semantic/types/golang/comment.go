package golang

import "regexp"

// godocReturnsRE matches the canonical GoDoc "<Name> returns [a|an|the] *Type"
// shape. The captured token is the LAST identifier-like word; the optional
// "*" pointer marker is stripped.
//
// Examples (caught):
//   - "// Foo returns a *UserRepository for the given user."
//   - "// Bar returns the Foo it found."
//   - "// Baz returns an Error if anything fails."
//
// Per D-12 the regex is bounded (no quantifier nesting) — no
// catastrophic-backtracking risk. Inputs are bounded to Phase 59-extracted
// doc-comment fields (already truncated at extraction).
// Alternatives ordered longest-first so e.g. "an" wins over "a" before the
// regex engine commits the `(?:...)?` group. The trailing `\s+` after the
// optional article guarantees we do not consume the article's own letters
// as part of the type-name capture (catches "returns an Error" → "Error").
var godocReturnsRE = regexp.MustCompile(`(?i)\breturns\b(?:\s+(?:the|an|a))?\s+\*?(\w+)`)

// ParseGoDocType extracts a probable type name from a GoDoc comment.
// Returns "" if no pattern matches.
//
// The cap on returned confidence (0.60) is enforced upstream — the caller
// (resolver.go tier 5) marks the response with EvidenceComment which
// CapCommentConfidence pins at ConfidenceComment.
func ParseGoDocType(doc string) string {
	if doc == "" {
		return ""
	}
	m := godocReturnsRE.FindStringSubmatch(doc)
	if len(m) >= 2 {
		return m[1]
	}
	return ""
}

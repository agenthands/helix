package typescript

import "regexp"

// TSDoc / JSDoc share the same surface for the patterns we care about:
//
//   - @type {<TypeName>}
//   - @param {<TypeName>} <name> [- description]
//   - @returns {<TypeName>}
//   - @return  {<TypeName>}  (legacy JSDoc)
//
// Per D-12 the regexes are simple (no quantifier nesting); inputs are
// bounded by Phase 59-extracted doc-comment fields.
var (
	tsdocTypeRE    = regexp.MustCompile(`@type\s*\{([A-Za-z_][A-Za-z0-9_]*)\}`)
	tsdocParamRE   = regexp.MustCompile(`@param\s*\{([A-Za-z_][A-Za-z0-9_]*)\}`)
	tsdocReturnsRE = regexp.MustCompile(`@returns?\s*\{([A-Za-z_][A-Za-z0-9_]*)\}`)
)

// ParseTSDocType extracts a probable type name from a TSDoc / JSDoc comment.
// Returns "" if no recognised pattern matches. Order of precedence:
// @type > @returns > @param.
func ParseTSDocType(doc string) string {
	if doc == "" {
		return ""
	}
	if m := tsdocTypeRE.FindStringSubmatch(doc); len(m) >= 2 {
		return m[1]
	}
	if m := tsdocReturnsRE.FindStringSubmatch(doc); len(m) >= 2 {
		return m[1]
	}
	if m := tsdocParamRE.FindStringSubmatch(doc); len(m) >= 2 {
		return m[1]
	}
	return ""
}

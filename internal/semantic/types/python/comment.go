package python

import "regexp"

// Python ships two type-bearing comment / annotation surfaces in v1:
//
//   - "# type: <TypeName>" — legacy Python type comments (PEP 484).
//   - "x: <TypeName>" / "def f(x: T) -> R:" — PEP-484 annotations on
//     declarations and function signatures.
//
// Both are regex-parsed; per D-12 there are no third-party deps.
var (
	pyTypeCommentRE = regexp.MustCompile(`#\s*type\s*:\s*([A-Za-z_][A-Za-z0-9_]*)`)
	pyReturnArrowRE = regexp.MustCompile(`->\s*([A-Za-z_][A-Za-z0-9_]*)`)
	pyVarAnnoRE     = regexp.MustCompile(`(?:^|\b)(?:[a-zA-Z_]\w*)\s*:\s*([A-Za-z_][A-Za-z0-9_]*)`)
)

// ParsePyTypeComment extracts the PEP-484 "# type: <T>" surface. Returns
// "" when no recognised pattern matches.
func ParsePyTypeComment(doc string) string {
	if doc == "" {
		return ""
	}
	if m := pyTypeCommentRE.FindStringSubmatch(doc); len(m) >= 2 {
		return m[1]
	}
	return ""
}

// ParsePyAnnotation extracts the type from a Python signature using the
// PEP-484 annotation surface. Function signatures with a return arrow
// `-> R` win over the parameter / variable annotation. Returns "" when no
// pattern matches.
func ParsePyAnnotation(sig string) string {
	if sig == "" {
		return ""
	}
	if m := pyReturnArrowRE.FindStringSubmatch(sig); len(m) >= 2 {
		return m[1]
	}
	if m := pyVarAnnoRE.FindStringSubmatch(sig); len(m) >= 2 {
		return m[1]
	}
	return ""
}

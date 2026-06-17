package runtime

import (
	"fmt"
	"path/filepath"
	"strings"
)

// validatePathSegment is the single source of truth for the security-relevant
// path-traversal predicate shared by the matrix layer (validateMatrixID) and the
// cell layer (validateCellKey). IN-05: these two validators were byte-for-byte
// identical bodies that only differed in their error-prefix wrapping; keeping two
// copies of a security-relevant check risks them drifting apart (a future
// hardening applied to one and not the other silently weakens the cell-layer
// check). Both call sites now delegate here and keep their distinct wrapping.
//
// It rejects names that filepath.Clean rewrites ("..", "a//b", a trailing slash),
// that contain any path separator (a single embedded "a/b" is caught by the
// explicit ContainsAny clause — Clean leaves it unchanged, so the Clean check
// alone would NOT reject it), or that begin with a dot — i.e. anything that could
// escape a join root once it becomes a path segment (V5 / T-77-08 / T-77-10). MUST
// run BEFORE any filepath.Join with the segment.
func validatePathSegment(name, kind string) error {
	if name == "" {
		return fmt.Errorf("%s is empty", kind)
	}
	if name != filepath.Clean(name) || strings.ContainsAny(name, `/\`) || strings.HasPrefix(name, ".") {
		return fmt.Errorf("%s %q contains path separators, parent refs, or a leading dot", kind, name)
	}
	return nil
}

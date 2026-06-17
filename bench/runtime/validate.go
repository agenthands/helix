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
// It rejects a name that is empty, that filepath.Clean would rewrite (parent refs
// like "..", redundant separators), that contains a path separator, or that
// begins with a dot — i.e. anything that could escape a join root once it becomes
// a path segment (V5 / T-77-08 / T-77-10). MUST run BEFORE any filepath.Join with
// the segment.
func validatePathSegment(name, kind string) error {
	if name == "" {
		return fmt.Errorf("%s is empty", kind)
	}
	if name != filepath.Clean(name) || strings.ContainsAny(name, `/\`) || strings.HasPrefix(name, ".") {
		return fmt.Errorf("%s %q contains path separators, parent refs, or a leading dot", kind, name)
	}
	return nil
}

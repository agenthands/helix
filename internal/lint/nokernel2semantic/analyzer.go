// Package nokernel2semantic provides a go/analysis Analyzer that fails the
// build if any package whose import path begins with internal/kernel/ imports
// a package whose import path begins with internal/semantic/.
//
// Enforces Phase 60 LIVE-07 invariant #1 (60-CONTEXT.md "Acceptance criteria"
// item #1): "internal/kernel/ does not import internal/semantic/...". The
// rule is one-directional — semantic→kernel is allowed (and is in fact the
// architecture; semantic depends on kernel for typed IDs, fileops, etc.).
//
// Trace:
//
//	Acceptance #1 (LIVE-07 invariant #1)  →  this analyzer  →  cmd/vet-nokernel2semantic
//	                                       (`make vet` runs the singlechecker)
//
// The forbiddenImportPrefix string is intentionally a prefix (no trailing
// slash) so it matches any sub-package of internal/semantic/ without per-
// package allowlisting.
package nokernel2semantic

import (
	"strings"

	"golang.org/x/tools/go/analysis"
)

const checkedPkgPrefix = "github.com/agenthands/helix/internal/kernel"
const forbiddenImportPrefix = "github.com/agenthands/helix/internal/semantic"

// Analyzer enforces LIVE-07 invariant #1: internal/kernel/* must not import
// internal/semantic/*.
var Analyzer = &analysis.Analyzer{
	Name: "nokernel2semantic",
	Doc:  "fails if internal/kernel/* imports internal/semantic/*",
	Run: func(pass *analysis.Pass) (interface{}, error) {
		if !strings.HasPrefix(pass.Pkg.Path(), checkedPkgPrefix) {
			return nil, nil
		}
		for _, file := range pass.Files {
			for _, imp := range file.Imports {
				path := strings.Trim(imp.Path.Value, `"`)
				if strings.HasPrefix(path, forbiddenImportPrefix) {
					pass.Reportf(imp.Pos(),
						"internal/kernel/* must not import internal/semantic/* (got import %q in %s)",
						path, pass.Pkg.Path())
				}
			}
		}
		return nil, nil
	},
}

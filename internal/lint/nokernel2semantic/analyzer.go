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
// CI note: .github/workflows/go-test.yml currently runs plain `go vet ./...`
// (without `-vettool=`), so the singlechecker only fires on local
// `make vet` / `make test` invocations today. A follow-up wiring step (out
// of scope per 60-01-PLAN.md "do NOT add a new workflow file in this plan")
// can swap the CI step from `go vet ./...` to `make vet` to make the
// LIVE-07 #1 gate apply on every PR.
//
// The forbiddenImportPrefix string is intentionally a prefix (no trailing
// slash) so it matches any sub-package of internal/semantic/ without per-
// package allowlisting.
//
// # Allowlist
//
// Phase 65 wave 0 (RESEARCH.md Common Pitfalls §1; M-vet) opens a single,
// named, types-only seam: kernel packages may import the package whose
// path is exactly allowedSemanticIntegPath, OR any sub-package under it
// (slash-boundary). The slash-boundary check is mandatory (Acceptance A6):
// a bare strings.HasPrefix would silently let `internal/semantic/integ_evil`
// through, since "integ" is a prefix of "integ_evil". We require an exact
// match OR a "/" character immediately after the allowed prefix to count
// as allowlisted.
//
// Adding more allowlisted paths requires a deliberate code edit here PLUS
// a new analysistest fixture under testdata/. There is no config-driven
// expansion path on purpose — the architectural boundary is defined in
// source.
package nokernel2semantic

import (
	"strings"

	"golang.org/x/tools/go/analysis"
)

const (
	checkedPkgPrefix         = "github.com/agenthands/helix/internal/kernel"
	forbiddenImportPrefix    = "github.com/agenthands/helix/internal/semantic"
	allowedSemanticIntegPath = "github.com/agenthands/helix/internal/semantic/integ"
)

// isForbidden reports whether `path` is a kernel-forbidden semantic import.
// True iff path is under internal/semantic/* AND is NOT the allowlisted
// internal/semantic/integ exact path or any of its sub-packages (slash-
// boundary check, per Acceptance A6).
func isForbidden(path string) bool {
	if !strings.HasPrefix(path, forbiddenImportPrefix) {
		return false
	}
	if path == allowedSemanticIntegPath {
		return false
	}
	if strings.HasPrefix(path, allowedSemanticIntegPath+"/") {
		return false
	}
	return true
}

// Analyzer enforces LIVE-07 invariant #1: internal/kernel/* must not import
// internal/semantic/*, with the exception of the allowlisted
// internal/semantic/integ types-only seam (Phase 65 wave 0).
var Analyzer = &analysis.Analyzer{
	Name: "nokernel2semantic",
	Doc:  "fails if internal/kernel/* imports internal/semantic/* (allowlist: internal/semantic/integ and sub-packages)",
	Run: func(pass *analysis.Pass) (interface{}, error) {
		if !strings.HasPrefix(pass.Pkg.Path(), checkedPkgPrefix) {
			return nil, nil
		}
		for _, file := range pass.Files {
			for _, imp := range file.Imports {
				path := strings.Trim(imp.Path.Value, `"`)
				if isForbidden(path) {
					pass.Reportf(imp.Pos(),
						"internal/kernel/* must not import internal/semantic/* (got import %q in %s)",
						path, pass.Pkg.Path())
				}
			}
		}
		return nil, nil
	},
}

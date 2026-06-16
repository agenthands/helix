// Package ablationleakage provides a go/analysis Analyzer that fails the build
// if an ablation-gated tool package imports a disabled-subsystem package it
// must not reach.
//
// Enforces ABLATE-08 (76-CONTEXT.md D-07): the bench-runner namespace
// `github.com/agenthands/helix/bench/runners` MUST NOT import either of the
// two disabled-subsystem packages `internal/kernel/lspool` or
// `internal/semantic/store`. This is a genuinely-architectural edge — a bench
// runner orchestrates a daemon subprocess; it never links the kernel pool or
// the semantic store directly — so the analyzer is the static, compile-time
// complement to the kernel `Unsupported` runtime guard (Plan 76-01).
//
// The forbidden edge is deliberately NOT `internal/kernel/* → internal/fuzzy`:
// the structured-edit tools legitimately import `internal/fuzzy` in the
// enabled build, so that edge is flag-conditional, not architectural (RESEARCH
// Pitfall 5 / D-08 honest scope).
//
// Match form is exact-package OR slash-suffix subpath (the noduckdb /
// nokernel2semantic slash-boundary discipline): a sibling package whose path
// merely has a forbidden prefix as a non-slash substring (e.g.
// `internal/semantic/storehouse`) is NOT flagged. A bare strings.HasPrefix
// would silently over-flag such lookalikes.
//
// The real `bench/runners/*` packages land in Phase 80; correctness this phase
// is proven entirely by the testdata fixtures (the go tool ignores testdata/),
// so the green→red flip is demonstrable now without a real consumer.
package ablationleakage

import (
	"strings"

	"golang.org/x/tools/go/analysis"
)

// checkedPkgPrefix is the ablation-gated tool package namespace. Only packages
// whose import path is rooted here are analyzed.
const checkedPkgPrefix = "github.com/agenthands/helix/bench/runners"

// forbiddenImportPrefixes are the disabled-subsystem packages a bench runner
// must never import directly. Matched exact-OR-prefix+"/" (slash boundary).
var forbiddenImportPrefixes = []string{
	"github.com/agenthands/helix/internal/kernel/lspool",
	"github.com/agenthands/helix/internal/semantic/store",
}

// Analyzer enforces ABLATE-08: bench-runner packages must not import the
// disabled LSP-pool or semantic-store subsystems.
var Analyzer = &analysis.Analyzer{
	Name: "ablationleakage",
	Doc:  "fails if an ablation-gated bench-runner package imports a disabled-subsystem package (internal/kernel/lspool or internal/semantic/store)",
	Run: func(pass *analysis.Pass) (interface{}, error) {
		if !strings.HasPrefix(pass.Pkg.Path(), checkedPkgPrefix) {
			return nil, nil
		}
		for _, file := range pass.Files {
			for _, imp := range file.Imports {
				path := strings.Trim(imp.Path.Value, `"`)
				for _, forbidden := range forbiddenImportPrefixes {
					if path == forbidden || strings.HasPrefix(path, forbidden+"/") {
						pass.Reportf(imp.Pos(),
							"ablation-gated %s must not import %s (got %s)",
							checkedPkgPrefix, forbidden, path)
					}
				}
			}
		}
		return nil, nil
	},
}

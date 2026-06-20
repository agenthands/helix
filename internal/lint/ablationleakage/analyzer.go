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
	"go/ast"
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

// semanticReadMethods is the exact set of data-bearing SemanticLookup read
// method NAMES the call-site gate check flags (D-06). These are the back-channel
// semantic reads ABLATE-06 forbids on the no_semantic arm: a direct call to one
// of these — outside the allowlist and not routed through integ.ChooseSource —
// is a gate bypass. The set is deliberately scoped to the data-returning reads
// (RankFiles / ExpandFrom / ValidateCriticalEdges); lifecycle/status methods
// (Available, Status) are NOT reads and are intentionally excluded.
//
// Plan 04 MUST keep its production wiring inside gateAllowedPkgPrefixes (below)
// so these reads stay legitimate on the real tree.
var semanticReadMethods = map[string]bool{
	"ExpandFrom":            true,
	"RankFiles":             true,
	"ValidateCriticalEdges": true,
}

// chooseSourceFn is the routing gate name. A file that syntactically contains a
// call to this function is treated by the narrow-AST check as gating its
// semantic reads (the green path). This is the documented narrow-AST
// approximation of "routes through ChooseSource" (RESEARCH Pitfall 6 / A3); the
// SSA receiver-typed call-graph proof is the deferred higher-precision upgrade.
const chooseSourceFn = "ChooseSource"

// gateAllowedPkgPrefixes are the package namespaces permitted to call a
// SemanticLookup read method directly without a ChooseSource guard: the integ
// adapter that *implements* the lookup, the semantic store/skill packages that
// own the read path, and the daemon composition root that constructs and gates
// the wiring. A package whose path is rooted here is exempt from the call-site
// check. Matched exact-OR-prefix+"/" (the same slash-boundary discipline as the
// import check above — a lookalike like `.../integration` is NOT exempted).
var gateAllowedPkgPrefixes = []string{
	"github.com/agenthands/helix/internal/semantic",
	"github.com/agenthands/helix/internal/skill/semantic",
	"github.com/agenthands/helix/internal/daemon",
	"github.com/agenthands/helix/internal/kernel/symbols",
	"github.com/agenthands/helix/internal/kernel/health",
}

// pkgInAllowlist reports whether pkgPath is rooted at one of the gate-allowed
// prefixes, using exact-OR-slash-boundary matching so lookalike siblings are not
// silently exempted.
func pkgInAllowlist(pkgPath string) bool {
	for _, prefix := range gateAllowedPkgPrefixes {
		if pkgPath == prefix || strings.HasPrefix(pkgPath, prefix+"/") {
			return true
		}
	}
	return false
}

// fileRoutesThroughChooseSource reports whether the file contains any call whose
// selector/identifier is ChooseSource — the narrow-AST evidence that semantic
// reads in this file are gated.
func fileRoutesThroughChooseSource(file *ast.File) bool {
	gated := false
	ast.Inspect(file, func(n ast.Node) bool {
		if gated {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if callee, ok := call.Fun.(*ast.SelectorExpr); ok && callee.Sel.Name == chooseSourceFn {
			gated = true
			return false
		}
		if callee, ok := call.Fun.(*ast.Ident); ok && callee.Name == chooseSourceFn {
			gated = true
			return false
		}
		return true
	})
	return gated
}

// Analyzer enforces ABLATE-08 (import boundary) and ABLATE-06 (call-site gate):
//
//   - Import boundary: bench-runner packages must not import the disabled
//     LSP-pool or semantic-store subsystems.
//   - Call-site gate (D-06): a direct SemanticLookup read method call
//     (ExpandFrom / RankFiles / ValidateCriticalEdges) outside the gate
//     allowlist and not routed through integ.ChooseSource is a gate bypass and
//     is flagged. This is the static, compile-time complement to Plan 05's
//     runtime read-counter assertion.
var Analyzer = &analysis.Analyzer{
	Name: "ablationleakage",
	Doc:  "fails if an ablation-gated bench-runner package imports a disabled-subsystem package, or if a semantic read bypasses integ.ChooseSource (D-06)",
	Run: func(pass *analysis.Pass) (interface{}, error) {
		// Check 1 (ABLATE-08): import-boundary on the bench-runner namespace.
		// Preserved verbatim from Phase 76 — the slash-boundary discipline
		// (exact-OR-prefix+"/") must not regress.
		if strings.HasPrefix(pass.Pkg.Path(), checkedPkgPrefix) {
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
		}

		// Check 2 (ABLATE-06 / D-06): semantic-read call-site gate. Packages
		// rooted in the gate allowlist own the read path and are exempt.
		if pkgInAllowlist(pass.Pkg.Path()) {
			return nil, nil
		}
		for _, file := range pass.Files {
			// A file that routes through ChooseSource gates its reads (the
			// green path) — exempt every read in it.
			if fileRoutesThroughChooseSource(file) {
				continue
			}
			ast.Inspect(file, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				if semanticReadMethods[sel.Sel.Name] {
					pass.Reportf(sel.Sel.Pos(),
						"semantic read %s must route through integ.ChooseSource (ABLATE-06 gate bypass)",
						sel.Sel.Name)
				}
				return true
			})
		}
		return nil, nil
	},
}

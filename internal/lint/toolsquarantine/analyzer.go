// Package toolsquarantine provides a go/analysis Analyzer that fails the build
// if any runtime/cmd package imports the dev-time tools/ tree.
//
// Enforces TUNE-01 (Phase 106): no package OUTSIDE
// github.com/agenthands/helix/tools may import
// github.com/agenthands/helix/tools/... . The tools/ tree holds the dev-time
// DSPy offline-tuning harness (Plan 02), which must never become a runtime
// dependency of the shipped binary, go.mod, or `go test ./...`. The tools/ tree
// itself ships NO .go/go.mod, so it is already invisible to the Go build; this
// analyzer is the static, compile-time belt that keeps it that way if a future
// `tools/*.go` is ever added.
//
// This is the INVERSION of internal/lint/ablationleakage's import-boundary leg:
// ablationleakage opts a narrow namespace IN (only bench/runners packages are
// checked); this analyzer opts the tools/ namespace OUT (it may self-import) and
// checks every OTHER package for a forbidden tools/ import.
//
// Match form is exact-package OR slash-suffix subpath (the same noduckdb /
// ablationleakage slash-boundary discipline): a sibling package whose path
// merely has "tools" as a non-slash substring (e.g.
// internal/toolsupport) is NOT flagged, and an import of such a lookalike is
// NOT matched. A bare strings.HasPrefix would silently over-flag.
//
// The analyzer is import-boundary-ONLY: it does NOT scan for pip/exec.Command
// shell-outs. internal/langregistry/installer.go legitimately shells pip/pipx to
// install language servers; a blanket shell-out ban would falsely flag it
// (106-RESEARCH Pitfall 2). The deliberate violation lives only under testdata/
// (which the go tool ignores), so `make vet` on the real tree stays green.
package toolsquarantine

import (
	"strings"

	"golang.org/x/tools/go/analysis"
)

// toolsPrefix is the dev-time tools/ namespace. No package whose own path is
// outside this prefix may import a package rooted here.
const toolsPrefix = "github.com/agenthands/helix/tools"

// Analyzer fails if a runtime/cmd package imports the dev-time tools/ tree.
var Analyzer = &analysis.Analyzer{
	Name: "toolsquarantine",
	Doc:  "fails if a runtime/cmd package imports the dev-time tools/ tree",
	Run: func(pass *analysis.Pass) (interface{}, error) {
		// Self-import exemption: a package whose OWN path is rooted under the
		// tools/ prefix may import other tools/ packages. Matched
		// exact-OR-slash-boundary so a lookalike like internal/toolsupport is
		// NOT exempted (it is analyzed; it simply imports nothing under tools/).
		pkgPath := pass.Pkg.Path()
		if pkgPath == toolsPrefix || strings.HasPrefix(pkgPath, toolsPrefix+"/") {
			return nil, nil
		}
		for _, file := range pass.Files {
			for _, imp := range file.Imports {
				path := strings.Trim(imp.Path.Value, `"`)
				if path == toolsPrefix || strings.HasPrefix(path, toolsPrefix+"/") {
					pass.Reportf(imp.Pos(),
						"runtime package %s must not import dev-time %s (got %s)",
						pkgPath, toolsPrefix, path)
				}
			}
		}
		return nil, nil
	},
}

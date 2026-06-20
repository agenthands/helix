// Package benchragleakage provides a go/analysis Analyzer that fails the build
// if the standalone baseline_rag MCP server package cmd/helix-bench-rag imports
// a daemon-side subsystem it must not link.
//
// Enforces ABLATE-04 criterion #1c (Phase 83): the standalone control-arm
// binary `github.com/agenthands/helix/cmd/helix-bench-rag` MUST NOT import
// `internal/kernel` or `internal/semantic`. This binary is a PROVABLY-isolated
// control arm — it constructs its MCP server via the SDK directly and shares no
// code with the Helix daemon — so the analyzer is the static, compile-time
// complement to the dynamic transitive import-set test (leakage_test.go's
// go/packages NeedDeps check).
//
// internal/mcp is not listed explicitly: it transitively links both
// internal/kernel and internal/semantic, so any path that reaches it trips the
// gate via those prefixes anyway.
//
// Match form is exact-package OR slash-suffix subpath (the same slash-boundary
// discipline as internal/lint/ablationleakage:170): a sibling package whose path
// merely has a forbidden prefix as a non-slash substring (e.g.
// `internal/kernelextra`) is NOT flagged. A bare strings.HasPrefix would
// silently over-flag such lookalikes.
//
// This is a PURE import-prefix gate: it carries none of the ablationleakage
// call-site (ChooseSource) machinery.
package benchragleakage

import (
	"strings"

	"golang.org/x/tools/go/analysis"
)

// checkedPkgPrefix is the standalone baseline_rag server namespace. Only packages
// whose import path is rooted here are analyzed.
const checkedPkgPrefix = "github.com/agenthands/helix/cmd/helix-bench-rag"

// forbiddenImportPrefixes are the daemon-side subsystems the standalone server
// must never import. Matched exact-OR-prefix+"/" (slash boundary).
var forbiddenImportPrefixes = []string{
	"github.com/agenthands/helix/internal/kernel",
	"github.com/agenthands/helix/internal/semantic",
}

// Analyzer enforces ABLATE-04 #1c: cmd/helix-bench-rag must not import
// internal/kernel or internal/semantic (transitively the same as not importing
// internal/mcp). Static, compile-time complement to the dynamic transitive
// import-set test.
var Analyzer = &analysis.Analyzer{
	Name: "benchragleakage",
	Doc:  "fails if cmd/helix-bench-rag imports internal/kernel or internal/semantic (ABLATE-04 #1c)",
	Run: func(pass *analysis.Pass) (interface{}, error) {
		// Only analyze packages rooted at the standalone server namespace.
		// Exact-OR-slash-boundary so a hypothetical sibling like
		// cmd/helix-bench-rag-extra is not swept in.
		path := pass.Pkg.Path()
		if path != checkedPkgPrefix && !strings.HasPrefix(path, checkedPkgPrefix+"/") {
			return nil, nil
		}
		for _, file := range pass.Files {
			for _, imp := range file.Imports {
				impPath := strings.Trim(imp.Path.Value, `"`)
				for _, forbidden := range forbiddenImportPrefixes {
					if impPath == forbidden || strings.HasPrefix(impPath, forbidden+"/") {
						pass.Reportf(imp.Pos(),
							"isolated %s must not import %s (got %s)",
							checkedPkgPrefix, forbidden, impPath)
					}
				}
			}
		}
		return nil, nil
	},
}

// Package noduckdb provides a go/analysis Analyzer that fails the build if
// the duckdb-go module is imported from any package outside the allowlisted
// internal/semantic/store/ subtree.
//
// Match form is exact-package OR slash-suffix subpath. Sibling repos like
// duckdb-go-bindings or duckdb-go-sibling are NOT flagged. A future major
// bump (v3) is matched via the `/v3` subpath; the constant does not need to
// change. The exact module path is locked by CONTEXT.md D-12.
package noduckdb

import (
	"strings"

	"golang.org/x/tools/go/analysis"
)

const allowedPkgPrefix = "github.com/agenthands/helix/internal/semantic/store"
const forbiddenImport = "github.com/duckdb/duckdb-go" // D-12 lock; prefix form (omits /v2)

// Analyzer enforces STORE-06: duckdb-go may only be imported from
// internal/semantic/store/*.
var Analyzer = &analysis.Analyzer{
	Name: "noduckdb",
	Doc:  "fails if duckdb-go is imported outside internal/semantic/store/",
	Run: func(pass *analysis.Pass) (interface{}, error) {
		if strings.HasPrefix(pass.Pkg.Path(), allowedPkgPrefix) {
			return nil, nil
		}
		for _, file := range pass.Files {
			for _, imp := range file.Imports {
				path := strings.Trim(imp.Path.Value, `"`)
				if path == forbiddenImport || strings.HasPrefix(path, forbiddenImport+"/") {
					pass.Reportf(imp.Pos(),
						"duckdb-go may only be imported from %s (got %s)",
						allowedPkgPrefix, pass.Pkg.Path())
				}
			}
		}
		return nil, nil
	},
}

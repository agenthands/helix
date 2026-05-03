// Package noduckdb provides a go/analysis Analyzer that fails the build if
// the duckdb-go module is imported from any package outside the allowlisted
// internal/semantic/store/ subtree.
//
// The forbiddenImport string uses the prefix form (omits the "/v2" major-
// version suffix) so future major-version bumps still match without changing
// this constant. The exact module path is locked by CONTEXT.md D-12.
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
				if strings.HasPrefix(path, forbiddenImport) {
					pass.Reportf(imp.Pos(),
						"duckdb-go may only be imported from %s (got %s)",
						allowedPkgPrefix, pass.Pkg.Path())
				}
			}
		}
		return nil, nil
	},
}

// Package compactusesstore provides a go/analysis Analyzer enforcing
// the Phase 63 P63-02 boundary: internal/semantic/compact/ MUST NOT
// import duckdb-go directly. All DB access routes through
// internal/semantic/store. Belt-and-braces over the broader vet-noduckdb.
//
// CONTEXT.md hard invariant — the compactor consumes typed primitives
// from the store package (BeginSnapshot, ClearOverlayLE, Vacuum,
// Checkpoint) so the SQL boundary stays auditable in one place.
package compactusesstore

import (
	"strings"

	"golang.org/x/tools/go/analysis"
)

const (
	compactPkgPrefix   = "github.com/agenthands/helix/internal/semantic/compact"
	forbiddenImport    = "github.com/duckdb/duckdb-go"
	allowedStorePrefix = "github.com/agenthands/helix/internal/semantic/store"
)

// Analyzer enforces the compact→store boundary.
var Analyzer = &analysis.Analyzer{
	Name: "compactusesstore",
	Doc:  "fails if internal/semantic/compact imports duckdb-go directly; route through internal/semantic/store",
	Run: func(pass *analysis.Pass) (interface{}, error) {
		if !strings.HasPrefix(pass.Pkg.Path(), compactPkgPrefix) {
			return nil, nil
		}
		for _, file := range pass.Files {
			for _, imp := range file.Imports {
				path := strings.Trim(imp.Path.Value, `"`)
				if path == forbiddenImport || strings.HasPrefix(path, forbiddenImport+"/") {
					pass.Reportf(imp.Pos(),
						"internal/semantic/compact may not import %s directly; route through %s",
						forbiddenImport, allowedStorePrefix)
				}
			}
		}
		return nil, nil
	},
}

// Stub package used only by analysistest fixtures. Mirrors the production
// `internal/semantic/integ/` types-only seam introduced in Phase 65 wave 0:
// kernel packages may import THIS exact path (or any sub-package under it)
// without tripping the nokernel2semantic analyzer.
package integ

// Sentinel is an exported symbol so the fixture importer has something to
// reference; the analyzer cares only about import paths, but giving the
// package an exported name keeps the test fixture honest.
var Sentinel struct{}

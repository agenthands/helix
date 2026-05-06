// Package siblingpkg imports two distinct sibling-namespace forms:
//
//   - github.com/duckdb/duckdb-go-sibling          (bare repo)
//   - github.com/duckdb/duckdb-go-sibling/v2       (slash subpath)
//
// The analyzer MUST NOT flag either import — they are sibling repos,
// not the canonical duckdb-go module locked by D-12. Locking BOTH forms
// in a single fixture (WR-NEW-03) prevents a regression where the slash
// boundary is mistakenly removed: a guard like `path == forbiddenImport`
// alone would leak through duckdb-go/v2 (a real reject case covered by
// badpkg) and a guard like `HasPrefix(path, forbiddenImport)` alone
// would over-flag duckdb-go-sibling (a real allow case covered here).
//
// No //want directive: analysistest fails the test if the analyzer
// reports anything for this file, which is exactly the assertion this
// fixture is designed to make. See WR-02 closure in 57-05-PLAN.md and
// the WR-NEW-03 follow-up tightening.
package siblingpkg

import (
	_ "github.com/duckdb/duckdb-go-sibling"
	_ "github.com/duckdb/duckdb-go-sibling/v2"
)

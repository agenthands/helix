// Package v2 is an analysistest fixture proving the noduckdb analyzer
// does not over-match the duckdb-go-sibling/v2 subpath (WR-NEW-03). It
// pairs with .../duckdb-go-sibling/sibling.go to lock the slash-boundary
// allowance from BOTH directions: bare and /vN.
package v2

func Stub() {}

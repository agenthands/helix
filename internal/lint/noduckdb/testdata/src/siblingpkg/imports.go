// Package siblingpkg imports github.com/duckdb/duckdb-go-sibling.
// The analyzer MUST NOT flag this import — it is a sibling repo, not
// the canonical duckdb-go module locked by D-12.
//
// No //want directive — analysistest fails if the analyzer reports
// anything for this file.
package siblingpkg

import _ "github.com/duckdb/duckdb-go-sibling"

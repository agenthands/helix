// This package's import path under the testdata GOPATH starts with the checked
// prefix `github.com/agenthands/helix/bench/runners`, and it imports
// `internal/semantic/storehouse` — a bare-prefix lookalike of the forbidden
// `internal/semantic/store`. Without a slash-boundary check the analyzer would
// silently over-flag this; the slash-OR-exact discipline demands it stay
// silent. Mirrors the noduckdb duckdb-go-sibling regression guard.
//
// No //want directive: analysistest fails the test if the analyzer reports
// anything for this file, which is exactly the assertion this fixture makes.
package siblingrunner

import _ "github.com/agenthands/helix/internal/semantic/storehouse"

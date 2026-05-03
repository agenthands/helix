// This package's import path under the testdata GOPATH starts with the
// allowlisted prefix `github.com/agenthands/helix/internal/semantic/store`,
// so the analyzer must NOT report.
package goodpkg

import _ "github.com/duckdb/duckdb-go/v2" // (no want directive — must be silent)

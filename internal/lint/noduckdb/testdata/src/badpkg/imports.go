package badpkg

import _ "github.com/duckdb/duckdb-go/v2" // want `duckdb-go may only be imported from .*`

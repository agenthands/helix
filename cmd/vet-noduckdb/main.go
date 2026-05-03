// Command vet-noduckdb is a singlechecker binary wrapping the noduckdb
// Analyzer. It is wired into `make vet` via `go vet -vettool=...` to enforce
// STORE-06 on every test run.
package main

import (
	"github.com/agenthands/helix/internal/lint/noduckdb"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() { singlechecker.Main(noduckdb.Analyzer) }

// Command vet-bench-rag-leakage is a singlechecker binary wrapping the
// benchragleakage Analyzer. It is wired into `make vet` via `go vet -vettool=...`
// to enforce ABLATE-04 #1c (the standalone baseline_rag server →
// internal/kernel|internal/semantic import boundary) on every test run. Static,
// compile-time complement to cmd/helix-bench-rag/leakage_test.go.
package main

import (
	"github.com/agenthands/helix/internal/lint/benchragleakage"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() { singlechecker.Main(benchragleakage.Analyzer) }

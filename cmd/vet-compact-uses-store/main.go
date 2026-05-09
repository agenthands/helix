// Command vet-compact-uses-store is a singlechecker binary wrapping the
// compactusesstore Analyzer. Phase 63 P63-02 Task 3 belt-and-braces over
// vet-noduckdb to enforce the compact→store boundary.
package main

import (
	"github.com/agenthands/helix/internal/lint/compactusesstore"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() { singlechecker.Main(compactusesstore.Analyzer) }

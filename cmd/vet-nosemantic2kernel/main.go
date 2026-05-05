// Command vet-nosemantic2kernel is a singlechecker binary wrapping the
// nosemantic2kernel Analyzer. Wired into `make vet` via
// `go vet -vettool=...` to enforce Phase 61 ENRICH-01 acceptance criterion #1
// (61-CONTEXT.md acceptance #1) on every test run.
package main

import (
	"github.com/agenthands/helix/internal/lint/nosemantic2kernel"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() { singlechecker.Main(nosemantic2kernel.Analyzer) }

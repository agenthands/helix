// Command vet-nokernel2semantic is a singlechecker binary wrapping the
// nokernel2semantic Analyzer. Wired into `make vet` via
// `go vet -vettool=...` to enforce Phase 60 LIVE-07 invariant #1
// (60-CONTEXT.md acceptance criterion #1) on every test run.
package main

import (
	"github.com/agenthands/helix/internal/lint/nokernel2semantic"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() { singlechecker.Main(nokernel2semantic.Analyzer) }

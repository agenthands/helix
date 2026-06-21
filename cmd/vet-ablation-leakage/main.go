// Command vet-ablation-leakage is a singlechecker binary wrapping the
// ablationleakage Analyzer. It is wired into `make vet` via
// `go vet -vettool=...` to enforce ABLATE-08 (the bench-runner →
// disabled-subsystem import boundary) on every test run.
package main

import (
	"github.com/agenthands/helix/internal/lint/ablationleakage"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() { singlechecker.Main(ablationleakage.Analyzer) }

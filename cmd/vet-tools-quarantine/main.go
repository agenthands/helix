// Command vet-tools-quarantine is a singlechecker binary wrapping the
// toolsquarantine Analyzer. It is wired into `make vet` via
// `go vet -vettool=...` to enforce the runtime → dev-time tools/ import
// boundary on every test run: no package outside
// github.com/agenthands/helix/tools may import the dev-time tools/ tree, so the
// DSPy offline-tuning harness never leaks into the shipped binary or go.mod.
package main

import (
	"github.com/agenthands/helix/internal/lint/toolsquarantine"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() { singlechecker.Main(toolsquarantine.Analyzer) }

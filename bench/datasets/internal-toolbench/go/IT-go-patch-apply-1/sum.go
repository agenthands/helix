// Package sumdoubler is the Phase 77 seed bench task (internal-toolbench/go/IT-go-patch-apply-1).
//
// Double is DELIBERATELY WRONG: it returns x instead of x*2, so sum_test.go
// FAILS before any edit. The scripted agent's single replace_in_file step flips
// "return x" to "return x * 2", after which `go test` passes (D-03). This is the
// SEED task only — the full ToolBench Go corpus is Phase 78.
package sumdoubler

// Double should return twice its argument.
func Double(x int) int {
	return x
}

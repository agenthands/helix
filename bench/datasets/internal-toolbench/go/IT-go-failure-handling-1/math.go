// Package mathfix is the internal-toolbench/go failure_handling fixture
// (IT-go-failure-handling-1).
//
// Negate is DELIBERATELY WRONG: it returns n instead of -n. The scripted agent
// first attempts an edit against a non-existent path (typo.go) with
// expect_error:true — exercising the error path (D-05 capability) — then
// RECOVERS by editing the correct file math.go. The test passes only after the
// recovery edit lands.
package mathfix

// Negate should return -n.
func Negate(n int) int {
	return n
}

// Package callgraph is the internal-toolbench/go call_graph fixture
// (IT-go-call-graph-1) — the D-05 exemplar.
//
// Compute panics on a zero divisor. Three callers (RunA, RunB, RunC) invoke it.
// get_call_hierarchy(Compute, direction=incoming) enumerates all three call
// sites; the scripted agent must add a zero guard at EVERY site or the
// corresponding subtest panics. The test requires the FULL incoming-call
// hierarchy to be acted on (D-05): miss one caller and the fixture fails.
package callgraph

// Compute returns 100 / divisor and panics when divisor == 0.
func Compute(divisor int) int {
	return 100 / divisor
}

// RunA is an incoming caller of Compute. It must guard against a zero divisor.
func RunA(divisor int) int {
	return Compute(divisor)
}

// RunB is an incoming caller of Compute. It must guard against a zero divisor.
func RunB(divisor int) int {
	return Compute(divisor)
}

// RunC is an incoming caller of Compute. It must guard against a zero divisor.
func RunC(divisor int) int {
	return Compute(divisor)
}

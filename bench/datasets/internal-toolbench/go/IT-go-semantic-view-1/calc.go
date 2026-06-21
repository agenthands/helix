// Package calc is the internal-toolbench/go semantic_view fixture
// (IT-go-semantic-view-1).
//
// Calculator exposes four methods that are DELIBERATELY stubbed with
// panic("not implemented"). get_symbol_overview on this file enumerates all four
// (Add, Sub, Mul, Div); the scripted agent must implement EVERY method the
// overview surfaces or calc_test.go fails (a missed method panics). D-05: the
// test requires the full symbol-overview output to be acted on.
package calc

// Calculator performs integer arithmetic.
type Calculator struct{}

// Add returns a + b.
func (Calculator) Add(a, b int) int {
	panic("not implemented")
}

// Sub returns a - b.
func (Calculator) Sub(a, b int) int {
	panic("not implemented")
}

// Mul returns a * b.
func (Calculator) Mul(a, b int) int {
	panic("not implemented")
}

// Div returns a / b (integer division).
func (Calculator) Div(a, b int) int {
	panic("not implemented")
}

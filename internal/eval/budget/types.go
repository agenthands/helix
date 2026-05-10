// Package budget declares the cross-cutting BreachReason type used by
// the watchdog (Plan 02) and the trace merger (Plan 03). Behavior
// (watchdog enforcement, YAML loading) lands in Plan 02's budget.go.
package budget

// BreachReason describes which D-08 axis tripped a budget breach.
// It is a Wave-0 cross-plan type contract: Plan 02 (watchdog) and Plan 03
// (trace merger) both consume this type; it must land in Wave 0 so those
// plans can run as parallel-disjoint Wave-1 work without one importing from
// the other's behavior code.
type BreachReason struct {
	Axis     string `json:"axis"`     // "input_tokens" | "output_tokens" | "seconds" | "tool_calls"
	Limit    int64  `json:"limit"`    // configured cap that was breached
	Observed int64  `json:"observed"` // measured value at breach
}

// String returns the D-08 outcome wording: "failed-with-cause: budget_<axis>".
func (b BreachReason) String() string { return "failed-with-cause: budget_" + b.Axis }

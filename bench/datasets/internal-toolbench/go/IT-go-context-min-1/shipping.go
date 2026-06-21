// Package shipping is the internal-toolbench/go context_minimization fixture
// (IT-go-context-min-1).
//
// The package has many small helpers (noise), but ONLY ShippingCost is wrong:
// it returns 0 instead of weight * ratePerKg. get_context surfaces the relevant
// symbol out of the noise; the scripted agent fixes just that one function. The
// test pins ShippingCost AND spot-checks an untouched helper, so the fix must be
// targeted (D-05: act on the minimized context, not the whole file).
package shipping

const ratePerKg = 3

// ShippingCost is the ONE relevant target: it must return weight * ratePerKg.
func ShippingCost(weight int) int {
	return 0
}

// Below are noise helpers that are already correct and must remain unchanged.

// Surcharge adds a flat handling fee.
func Surcharge(base int) int {
	return base + 5
}

// Discountable reports whether an order qualifies for a discount.
func Discountable(total int) bool {
	return total > 100
}

// RoundUp rounds a value up to the nearest ten.
func RoundUp(v int) int {
	if v%10 == 0 {
		return v
	}
	return v + (10 - v%10)
}

// Label formats a shipping label string.
func Label(zone string) string {
	return "zone-" + zone
}

// IsHeavy reports whether a weight exceeds the heavy threshold.
func IsHeavy(weight int) bool {
	return weight > 20
}

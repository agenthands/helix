package runtime

// Shared pointer-builder helpers for the runtime package tests (IN-04). These
// consolidate the previously duplicated iPtr/bPtr/fPtr (result_test.go) and
// intPtr/floatPtr (deltas_test.go) helpers into one set so the nullable metric
// pointers used across result/deltas tests have a single canonical builder.

func iPtr(i int) *int         { return &i }
func bPtr(b bool) *bool       { return &b }
func fPtr(f float64) *float64 { return &f }

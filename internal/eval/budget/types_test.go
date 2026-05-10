package budget

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBreachReasonShape verifies the BreachReason struct has the correct fields,
// zero-value is valid, and round-trips via JSON.
func TestBreachReasonShape(t *testing.T) {
	// Zero-value must be valid (no required non-nil fields).
	var zero BreachReason
	assert.Equal(t, "", zero.Axis)
	assert.Equal(t, int64(0), zero.Limit)
	assert.Equal(t, int64(0), zero.Observed)

	// Round-trip via JSON.
	br := BreachReason{Axis: "seconds", Limit: 300, Observed: 312}
	data, err := json.Marshal(br)
	require.NoError(t, err)

	var got BreachReason
	require.NoError(t, json.Unmarshal(data, &got))
	assert.Equal(t, br.Axis, got.Axis)
	assert.Equal(t, br.Limit, got.Limit)
	assert.Equal(t, br.Observed, got.Observed)
}

// TestBreachReasonString verifies the D-08 outcome wording verbatim.
func TestBreachReasonString(t *testing.T) {
	br := BreachReason{Axis: "seconds", Limit: 300, Observed: 312}
	assert.Equal(t, "failed-with-cause: budget_seconds", br.String(),
		"D-08 outcome wording must be 'failed-with-cause: budget_<axis>'")

	// Also verify other axes.
	cases := []struct {
		axis string
		want string
	}{
		{"input_tokens", "failed-with-cause: budget_input_tokens"},
		{"output_tokens", "failed-with-cause: budget_output_tokens"},
		{"tool_calls", "failed-with-cause: budget_tool_calls"},
	}
	for _, tc := range cases {
		t.Run(tc.axis, func(t *testing.T) {
			b := BreachReason{Axis: tc.axis}
			assert.Equal(t, tc.want, b.String())
		})
	}
}

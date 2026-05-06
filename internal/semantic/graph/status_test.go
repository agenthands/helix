// Phase 62 P02 RED gate — failing tests for computeScoreStatus closed enum.
//
// Pins the (currentGV, rowGV, hasRow, approximate) → ScoreStatus matrix:
//
//   - hasRow=false → ScoreStatusMissing (regardless of other inputs)
//   - approximate=true → ScoreStatusApproximate (overrides equality)
//   - rowGV < currentGV → ScoreStatusStale
//   - rowGV == currentGV → ScoreStatusExact
package graph

import "testing"

func TestComputeScoreStatus_ClosedEnum(t *testing.T) {
	cases := []struct {
		name        string
		currentGV   uint64
		rowGV       uint64
		hasRow      bool
		approximate bool
		want        ScoreStatus
	}{
		{"missing-no-row", 5, 0, false, false, ScoreStatusMissing},
		{"missing-no-row-with-approximate", 5, 0, false, true, ScoreStatusMissing},
		{"exact-equal-versions", 5, 5, true, false, ScoreStatusExact},
		{"stale-older-version", 5, 3, true, false, ScoreStatusStale},
		{"approximate-overrides-equality", 5, 5, true, true, ScoreStatusApproximate},
		{"approximate-on-stale-row", 5, 3, true, true, ScoreStatusApproximate},
		{"exact-zero-versions", 0, 0, true, false, ScoreStatusExact},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := computeScoreStatus(tc.currentGV, tc.rowGV, tc.hasRow, tc.approximate)
			if got != tc.want {
				t.Errorf("computeScoreStatus(%d,%d,%v,%v)=%q, want %q",
					tc.currentGV, tc.rowGV, tc.hasRow, tc.approximate, got, tc.want)
			}
			// Closed-enum invariant: result is one of the 4 declared constants.
			switch got {
			case ScoreStatusExact, ScoreStatusApproximate, ScoreStatusStale, ScoreStatusMissing:
			default:
				t.Errorf("returned non-enum status %q", got)
			}
		})
	}
}

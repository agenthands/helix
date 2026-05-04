package extract

import "testing"

// TestConfidenceLadder asserts the 5 named constants equal exactly the
// SPEC §11.2 numeric values. Values are float32 to match
// SymbolFact.Confidence.
func TestConfidenceLadder(t *testing.T) {
	tests := []struct {
		name string
		got  float32
		want float32
	}{
		{"ConfidenceLSPOnly", ConfidenceLSPOnly, 1.00},
		{"ConfidenceLSPMerged", ConfidenceLSPMerged, 0.95},
		{"ConfidenceTSPlusLocal", ConfidenceTSPlusLocal, 0.80},
		{"ConfidenceTSOnly", ConfidenceTSOnly, 0.70},
		{"ConfidenceHeuristic", ConfidenceHeuristic, 0.45},
	}
	for _, tc := range tests {
		if tc.got != tc.want {
			t.Errorf("%s = %v, want %v", tc.name, tc.got, tc.want)
		}
	}
}

// TestConfidenceLadder_NoFloatDrift asserts ConfidenceTSOnly == 0.70
// using direct == (no epsilon). Phase 59 emits exactly this value; any
// float drift here would silently break the bucket assignment elsewhere.
func TestConfidenceLadder_NoFloatDrift(t *testing.T) {
	if ConfidenceTSOnly != 0.70 {
		t.Fatalf("ConfidenceTSOnly drift: got %v, want 0.70 (no epsilon)", ConfidenceTSOnly)
	}
}

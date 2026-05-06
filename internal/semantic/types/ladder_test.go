package types

import (
	"testing"
)

// TestLadderConstants_AllSeven asserts the SPEC §38.2 7-tier confidence
// ladder constants are exactly 1.00 / 0.90 / 0.80 / 0.70 / 0.60 / 0.45 / 0.20.
func TestLadderConstants_AllSeven(t *testing.T) {
	cases := []struct {
		name string
		got  float64
		want float64
	}{
		{"ConfidenceLSP", ConfidenceLSP, 1.00},
		{"ConfidenceAnnotation", ConfidenceAnnotation, 0.90},
		{"ConfidenceConstructor", ConfidenceConstructor, 0.80},
		{"ConfidenceAssignment", ConfidenceAssignment, 0.70},
		{"ConfidenceComment", ConfidenceComment, 0.60},
		{"ConfidenceHeuristic", ConfidenceHeuristic, 0.45},
		{"ConfidenceUnknown", ConfidenceUnknown, 0.20},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Fatalf("%s = %v, want %v", tc.name, tc.got, tc.want)
			}
		})
	}
}

// TestCapCommentConfidence_CommentCapped: an EvidenceComment row at 0.85 is
// capped to 0.60 — TYPES-03 invariant.
func TestCapCommentConfidence_CommentCapped(t *testing.T) {
	got := CapCommentConfidence(0.85, EvidenceComment)
	if got != ConfidenceComment {
		t.Fatalf("CapCommentConfidence(0.85, EvidenceComment) = %v, want %v", got, ConfidenceComment)
	}
}

// TestCapCommentConfidence_NonCommentNotCapped: an EvidenceAnnotation row at
// 0.90 is preserved.
func TestCapCommentConfidence_NonCommentNotCapped(t *testing.T) {
	got := CapCommentConfidence(0.90, EvidenceAnnotation)
	if got != 0.90 {
		t.Fatalf("CapCommentConfidence(0.90, EvidenceAnnotation) = %v, want 0.90", got)
	}
}

// TestCapCommentConfidence_BelowCapNotChanged: comment row already below the
// cap survives unchanged.
func TestCapCommentConfidence_BelowCapNotChanged(t *testing.T) {
	got := CapCommentConfidence(0.50, EvidenceComment)
	if got != 0.50 {
		t.Fatalf("CapCommentConfidence(0.50, EvidenceComment) = %v, want 0.50", got)
	}
}

// TestConfidenceForEvidence_Roundtrip pins the evidence→confidence mapping.
func TestConfidenceForEvidence_Roundtrip(t *testing.T) {
	cases := []struct {
		kind EvidenceKind
		want float64
	}{
		{EvidenceLSP, ConfidenceLSP},
		{EvidenceAnnotation, ConfidenceAnnotation},
		{EvidenceConstructor, ConfidenceConstructor},
		{EvidenceAssignment, ConfidenceAssignment},
		{EvidenceComment, ConfidenceComment},
		{EvidenceHeuristic, ConfidenceHeuristic},
		{EvidenceUnknown, ConfidenceUnknown},
	}
	for _, tc := range cases {
		got := ConfidenceForEvidence(tc.kind)
		if got != tc.want {
			t.Fatalf("ConfidenceForEvidence(%q) = %v, want %v", tc.kind, got, tc.want)
		}
	}
}

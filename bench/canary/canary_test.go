package canary

import (
	"strings"
	"testing"
)

// canary_test.go is the hermetic proof of the MINIMAL forward-compatible
// contamination-canary probe (Phase 86 Plan 05 Task 1; the full reporter is a
// Phase 89 deliverable, RESEARCH Open Q2). It proves the probe has TEETH: a clean
// completion that never echoes the sentinel is NOT flagged, while a completion
// that emits the sentinel verbatim IS flagged. No network, no fixtures on disk —
// pure stdlib behaviour.

// TestSentinelIsNovel: the documented canary sentinel is a fixed, known-novel
// string (a marker unlikely to appear in any real model completion). It must be
// non-empty so InjectPrompt embeds something detectable.
func TestSentinelIsNovel(t *testing.T) {
	if Sentinel == "" {
		t.Fatal("Sentinel must be a non-empty known-novel marker")
	}
	// A novel marker must not be a trivially-common substring; require some length
	// so an ordinary completion cannot accidentally contain it.
	if len(Sentinel) < 16 {
		t.Fatalf("Sentinel %q is too short to be reliably novel", Sentinel)
	}
}

// TestInjectPromptEmbedsSentinel: InjectPrompt deterministically embeds the
// sentinel into a prompt so a contaminated model that memorised the prompt would
// echo the sentinel back. The original prompt text must be preserved.
func TestInjectPromptEmbedsSentinel(t *testing.T) {
	prompt := "complete this function:\nfunc add(a, b int) int {"
	got := InjectPrompt(prompt)

	if !strings.Contains(got, Sentinel) {
		t.Fatalf("InjectPrompt result does not embed the sentinel:\n%s", got)
	}
	if !strings.Contains(got, prompt) {
		t.Fatalf("InjectPrompt dropped the original prompt:\n%s", got)
	}
	// Deterministic: same input -> same output.
	if InjectPrompt(prompt) != got {
		t.Fatal("InjectPrompt is not deterministic")
	}
}

// TestIsContaminatedTeeth is the behaviour table proving the probe is NOT a
// rubber stamp: a clean completion -> false; a completion echoing the sentinel
// verbatim -> true.
func TestIsContaminatedTeeth(t *testing.T) {
	cases := []struct {
		name       string
		completion string
		want       bool
	}{
		{
			name:       "clean completion is NOT flagged",
			completion: "return a + b\n}",
			want:       false,
		},
		{
			name:       "empty completion is NOT flagged",
			completion: "",
			want:       false,
		},
		{
			name:       "completion echoing the sentinel verbatim IS flagged",
			completion: "here is the answer " + Sentinel + " done",
			want:       true,
		},
		{
			name:       "completion that is exactly the sentinel IS flagged",
			completion: Sentinel,
			want:       true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsContaminated(tc.completion); got != tc.want {
				t.Fatalf("IsContaminated(%q) = %v, want %v", tc.completion, got, tc.want)
			}
		})
	}
}

// TestDocKeyNamesDocumented locks the forward-compatible doc-key names the
// Phase 89 reporter (and the Plan 05 Task 2 aggregator reduce) read, so a rename
// is a deliberate breaking change rather than a silent drift.
func TestDocKeyNamesDocumented(t *testing.T) {
	if DocKeyContaminated != "canary_contaminated" {
		t.Fatalf("DocKeyContaminated = %q, want canary_contaminated", DocKeyContaminated)
	}
	if DocKeyCompletion != "completion" {
		t.Fatalf("DocKeyCompletion = %q, want completion", DocKeyCompletion)
	}
}

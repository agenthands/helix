package runtime

import (
	"strings"
	"testing"

	"github.com/agenthands/helix/bench/canary"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// prompt_inject_test.go is the hermetic proof of the INFRA-05 production canary
// injector (RESEARCH Open Q5): a deterministic, pure selector that embeds
// canary.Sentinel into a SELECT subset of task prompts so a contaminated model that
// memorised the prompt echoes the sentinel back into its completion (where rowCanary
// already reads it at score time). No network; no clock; no RNG.

// TestInjectCanarySelectedContainsSentinel: a SELECTED task's prompt carries the
// canary.Sentinel (via canary.InjectPrompt) and preserves the original prompt verbatim
// as a prefix (InjectPrompt appends).
func TestInjectCanarySelectedContainsSentinel(t *testing.T) {
	// Find a task key the deterministic selector selects (the predicate is stable, so
	// at least one of a handful of keys lands in an every-Kth bucket).
	var selectedKey string
	for _, k := range []string{"t0", "t1", "t2", "t3", "t4", "t5", "t6", "t7", "t8", "t9"} {
		if canarySelected(k) {
			selectedKey = k
			break
		}
	}
	require.NotEmpty(t, selectedKey, "at least one of 10 keys must be selected by the every-Kth predicate")

	orig := "complete this function"
	got := InjectCanaryIfSelected(selectedKey, orig)
	assert.Contains(t, got, canary.Sentinel, "a selected task's prompt carries the canary sentinel")
	assert.True(t, strings.HasPrefix(got, orig), "the original prompt is preserved verbatim as a prefix")
	assert.Equal(t, canary.InjectPrompt(orig), got,
		"selection routes through canary.InjectPrompt (single source of truth)")
}

// TestInjectCanaryNonSelectedIsVerbatim: a NON-selected task's prompt is byte-identical
// to the input (no sentinel, no mutation).
func TestInjectCanaryNonSelectedIsVerbatim(t *testing.T) {
	var unselectedKey string
	for _, k := range []string{"t0", "t1", "t2", "t3", "t4", "t5", "t6", "t7", "t8", "t9"} {
		if !canarySelected(k) {
			unselectedKey = k
			break
		}
	}
	require.NotEmpty(t, unselectedKey, "at least one of 10 keys must be NON-selected")

	orig := "complete this function"
	got := InjectCanaryIfSelected(unselectedKey, orig)
	assert.Equal(t, orig, got, "a non-selected task's prompt is byte-identical to the input")
	assert.NotContains(t, got, canary.Sentinel, "a non-selected prompt never carries the sentinel")
}

// TestInjectCanaryIdempotentDeterministic: the selector is a PURE function of the task
// key — repeated calls with the same key yield byte-identical output, and selection is
// stable across calls (no clock, no RNG, no map-iteration nondeterminism).
func TestInjectCanaryIdempotentDeterministic(t *testing.T) {
	for _, k := range []string{"alpha", "beta", "gamma", "delta", "task-dirty", "task-clean"} {
		orig := "prompt for " + k
		first := InjectCanaryIfSelected(k, orig)
		for i := 0; i < 5; i++ {
			assert.Equal(t, first, InjectCanaryIfSelected(k, orig),
				"InjectCanaryIfSelected(%q) must be deterministic across calls", k)
			assert.Equal(t, canarySelected(k), canarySelected(k),
				"canarySelected(%q) must be a stable predicate", k)
		}
	}
}

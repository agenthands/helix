//go:build llmjudge

package judge

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestAdoptionNegativeExemplarVerdict is the hermetic-on-a-Score proof for
// ROADMAP SC#2: the judge rubric ships a 6th `adoption` dimension whose
// grep-finds-a-definition response scores 0.0, and that zero flows through
// ComputeVerdict to a non-pass verdict. A lone adoption=0.0 yields a
// `soft_fail` (one-zero rule, rubric.go ComputeVerdict), and the second
// `doubleZero` case below escalates to a hard `fail`. Together they prove the
// rubric can demonstrably reach a non-pass / FAILING verdict rather than being a
// trivially-always-pass gate. It needs no API key — it constructs Scores
// directly (a grep response would be judged adoption=0.0) and asserts the
// verdicts (T-101-06).
func TestAdoptionNegativeExemplarVerdict(t *testing.T) {
	// A grep-finds-a-definition response: every other dimension is fine, but the
	// model reached for `grep` as the first command for a code-symbol question, so
	// the adoption dimension is the single zero.
	grepResponse := &Score{
		ToolChoice:           1.0,
		DescriptionUse:       1.0,
		OutputInterpretation: 1.0,
		UncertaintyHandling:  1.0,
		PolyglotReasoning:    1.0,
		Adoption:             0.0,
	}
	require.NoError(t, ComputeVerdict(grepResponse))
	require.Equal(t, "soft_fail", grepResponse.Verdict,
		"a single adoption=0.0 zero must yield soft_fail (one zero), proving the rubric can fail on a grep response")
	require.Contains(t, grepResponse.Failures, "adoption",
		"the adoption dimension must be listed among the failing dimensions")

	// Two zeros (adoption + tool_choice) must escalate to a hard fail.
	doubleZero := &Score{
		ToolChoice:           0.0,
		DescriptionUse:       1.0,
		OutputInterpretation: 1.0,
		UncertaintyHandling:  1.0,
		PolyglotReasoning:    1.0,
		Adoption:             0.0,
	}
	require.NoError(t, ComputeVerdict(doubleZero))
	require.Equal(t, "fail", doubleZero.Verdict,
		"two zeros (adoption + tool_choice) must yield a hard fail")
	require.Contains(t, doubleZero.Failures, "adoption")
	require.Contains(t, doubleZero.Failures, "tool_choice")

	// ValidateScoreValues must reject an off-scale adoption value.
	require.Error(t, ValidateScoreValues(&Score{Adoption: 0.7}),
		"adoption=0.7 is not in {0.0,0.5,1.0} and must be rejected")

	// RubricPrompt must carry the 6th dimension anchor, the negative exemplar, the
	// JSON key, and the updated dimension count.
	prompt := RubricPrompt()
	require.Contains(t, prompt, "### adoption", "RubricPrompt must define the adoption anchor")
	require.Contains(t, prompt, `"adoption"`, "RubricPrompt JSON template must include the adoption key")
	require.True(t, strings.Contains(strings.ToLower(prompt), "grep"),
		"RubricPrompt must describe the grep-finds-a-definition negative exemplar")
	require.Contains(t, prompt, "6 dimensions",
		"RubricPrompt prose must count 6 dimensions, not 5")
}

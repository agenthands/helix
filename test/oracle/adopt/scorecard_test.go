package adopt

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/agenthands/helix/internal/cli"
)

// fixtureTranscript mirrors the load-bearing subset of test/oracle/llm.Transcript
// (transcript.go:18). Only Response is read by the pure scorer; the other fields are
// committed for shape parity / human legibility and intentionally ignored here.
type fixtureTranscript struct {
	Response string `json:"response"`
}

// loadFixtureBucket reads every testdata/transcripts/{prefix}-*.json fixture, unmarshals
// its Response field, and returns the resulting []Bucket. It asserts at least MinTasks
// entries loaded so a future fixture deletion cannot silently shrink the bucket below the
// empty-bucket floor (re-introducing the 0/0 vacuity Scorecard rejects).
func loadFixtureBucket(t *testing.T, prefix string) []Bucket {
	t.Helper()
	pattern := filepath.Join("testdata", "transcripts", prefix+"-*.json")
	paths, err := filepath.Glob(pattern)
	require.NoError(t, err, "glob %s", pattern)
	require.GreaterOrEqualf(t, len(paths), MinTasks,
		"fixture bucket %q must hold >= MinTasks (%d) transcripts, found %d (a deletion shrank it below the floor)",
		prefix, MinTasks, len(paths))

	buckets := make([]Bucket, 0, len(paths))
	for _, p := range paths {
		data, err := os.ReadFile(p)
		require.NoErrorf(t, err, "reading fixture %s", p)
		var tr fixtureTranscript
		require.NoErrorf(t, json.Unmarshal(data, &tr), "unmarshaling fixture %s", p)
		require.NotEmptyf(t, tr.Response, "fixture %s has an empty response field", p)
		buckets = append(buckets, Bucket{Response: tr.Response})
	}
	return buckets
}

// TestFirstCommandNotSubstring nails Pitfall 2 (T-101-03): a response whose PROSE mentions
// "helix" but whose first EMITTED command is grep classifies as a FALLBACK, never a choice.
// A strings.Contains classifier would wrongly count the prose "helix" as adoption.
func TestFirstCommandNotSubstring(t *testing.T) {
	resp := "The helix tool would help here, but let me just grep.\ngrep -rn 'func NewClient' ."
	require.Equal(t, "The helix tool would help here, but let me just grep.", FirstCommand(resp),
		"FirstCommand must return the first line (prose), proving classification is keyed on the emitted command")

	chose, fellBack := ClassifyChoice(resp)
	require.False(t, chose, "a prose mention of helix must NOT classify as a choice (substring inflation)")
	require.False(t, fellBack, "the prose first line is not a fallback command either")

	// And when the model actually emits grep first, it is a FALLBACK despite any helix prose.
	chose2, fellBack2 := ClassifyChoice("grep -rn 'helix go-to-definition' .")
	require.False(t, chose2, "a grep command that merely contains the word helix is not a choice")
	require.True(t, fellBack2, "grep as the first command is a fallback")
}

// TestSabotagedSkillRevertAndFail is the hermetic heart of the phase (T-101-01): the intact
// fixtures (model chose helix verbs) must beat the sabotaged fixtures (matrix stripped ->
// model grepped) on choice_rate by at least MaterialDrop. Identical scores FAIL with a
// "scorecard measures nothing" message — the gate is presumed broken if it can only go green.
func TestSabotagedSkillRevertAndFail(t *testing.T) {
	intact := loadFixtureBucket(t, "intact")
	sabotaged := loadFixtureBucket(t, "sabotaged")

	scIntact, err := Scorecard(intact)
	require.NoError(t, err)
	scSab, err := Scorecard(sabotaged)
	require.NoError(t, err)

	drop := scIntact.ChoiceRate - scSab.ChoiceRate
	require.GreaterOrEqualf(t, drop, MaterialDrop,
		"choice_rate did not drop by a material margin when the decision matrix was stripped (%.2f -> %.2f, drop %.2f < MaterialDrop %.2f): scorecard measures nothing",
		scIntact.ChoiceRate, scSab.ChoiceRate, drop, MaterialDrop)

	// Complementarity guard on BOTH buckets: every fixture is classified as exactly one of
	// choice/fallback, so the two rates sum to 1.0.
	require.InDelta(t, 1.0, scIntact.ChoiceRate+scIntact.FallbackRate, 1e-9,
		"intact bucket: choice_rate + fallback_rate must be complementary")
	require.InDelta(t, 1.0, scSab.ChoiceRate+scSab.FallbackRate, 1e-9,
		"sabotaged bucket: choice_rate + fallback_rate must be complementary")
}

// TestComplementary asserts choice_rate + fallback_rate == 1.0 on the intact fixture bucket
// (every transcript is classified as exactly one of choice/fallback).
func TestComplementary(t *testing.T) {
	sc, err := Scorecard(loadFixtureBucket(t, "intact"))
	require.NoError(t, err)
	require.InDelta(t, 1.0, sc.ChoiceRate+sc.FallbackRate, 1e-9,
		"choice_rate (%.2f) + fallback_rate (%.2f) must equal 1.0", sc.ChoiceRate, sc.FallbackRate)
}

// TestEmptyBucketRejected asserts an empty or one-element bucket is an ERROR, never a 1.0
// pass (Pitfall 4 / Phase 87 CR-01, T-101-04).
func TestEmptyBucketRejected(t *testing.T) {
	for name, in := range map[string][]Bucket{
		"nil":         nil,
		"empty-slice": {},
		"one-element": {{Response: "helix go-to-definition --symbol X"}},
	} {
		t.Run(name, func(t *testing.T) {
			res, err := Scorecard(in)
			require.Error(t, err, "a sub-floor bucket must return an error, not a result")
			require.Equal(t, ScorecardResult{}, res, "a rejected bucket must yield a zero result, never choice_rate=1.0")
			require.NotEqual(t, 1.0, res.ChoiceRate, "0/0 must never read as choice_rate=1.0")
		})
	}
}

// TestSabotageNonNoop asserts StripDecisionMatrix actually shortens the REAL embedded skill
// body (T-101-02): a future SKILL.md restructure that renames the "## Decision matrix"
// heading would silently make the strip a no-op and re-introduce vacuity — this turns RED.
func TestSabotageNonNoop(t *testing.T) {
	body := cli.EmbeddedSkillBody()
	stripped := StripDecisionMatrix(body)
	require.Lessf(t, len(stripped), len(body),
		"StripDecisionMatrix must remove a section from the real embedded skill (in=%d, out=%d); a silent no-op means the '## Decision matrix' anchor moved",
		len(body), len(stripped))
}

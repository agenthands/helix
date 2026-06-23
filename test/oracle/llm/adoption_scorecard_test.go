//go:build llm

package llm

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/agenthands/helix/internal/cli"
	"github.com/agenthands/helix/test/oracle/adopt"
)

// TestAdoptionScorecardLive is the opt-in, INFORMATIONAL live leg of the Phase
// 101 adoption scorecard (ADOPT-02, ROADMAP SC#1 live measurement). For each
// helix-appropriate code task it interrogates a real subject model TWICE — once
// with the INTACT embedded SKILL.md (decision matrix present) and once with the
// matrix STRIPPED (adopt.StripDecisionMatrix) — captures the transcripts, and
// feeds both buckets to the SAME pure adopt.Scorecard (single source of truth;
// no re-implemented classifier).
//
// Honest gating: //go:build llm + SkipWithoutAPIKey(t) as the FIRST line. Without
// an API key the test SKIPs hermetically (never a vacuous pass, never an error).
// This leg is INFORMATIONAL and NEVER blocks merge: it asserts only that the
// scorecards build (the bucket is above adopt.MinTasks) and LOGS choice/fallback
// rates and the matrix-stripped drop. The authoritative revert-and-fail proof is
// the hermetic Plan 01 leg in test/oracle/adopt — live models are
// nondeterministic, so this leg makes no hard MaterialDrop assertion.
//
// Transcripts are written ONLY to the GITIGNORED test/oracle/llm/testdata/
// transcripts/ run-artifact dir (WriteTranscript), never to the committed
// adopt/testdata/ hermetic fixtures (101-RESEARCH Open Question 1).
func TestAdoptionScorecardLive(t *testing.T) {
	SkipWithoutAPIKey(t) // FIRST line — hermetic skip without a key (opt-in/informational).

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	client := NewClient()
	model := SubjectModel()

	// Single source of truth: load the embedded SKILL.md body and derive the
	// matrix-stripped (sabotaged) variant via the pure adopt helper — no inlined
	// copy, no re-embedded markdown.
	body := cli.EmbeddedSkillBody()
	require.NotEmpty(t, body, "embedded SKILL.md body is empty")
	sabotaged := adopt.StripDecisionMatrix(body)
	require.Less(t, len(sabotaged), len(body),
		"StripDecisionMatrix must materially shrink the body (non-noop)")

	intactSystem := SkillSystemPrompt(body)
	sabotagedSystem := SkillSystemPrompt(sabotaged)

	tasks := SkillTaskDescriptions()
	require.GreaterOrEqual(t, len(tasks), adopt.MinTasks,
		"task bucket must be at least adopt.MinTasks so Scorecard does not hit the empty-bucket floor")

	var (
		intactBucket    []adopt.Bucket
		sabotagedBucket []adopt.Bucket
		callIdx         int
	)

	for i, task := range tasks {
		task := task
		userPrompt := SkillTaskUserPrompt(task)

		// Intact-skill condition.
		if callIdx > 0 {
			InterCallDelay()
		}
		callIdx++
		intactResp, intactStop, err := AskSingleTurn(ctx, client, model, intactSystem, userPrompt)
		require.NoError(t, err, "intact AskSingleTurn failed for task %d", i)
		intactBucket = append(intactBucket, adopt.Bucket{Response: intactResp})
		WriteTranscript(t, &Transcript{
			ScenarioID: "adoption-intact-" + itoa(i),
			Category:   "selection",
			Model:      model,
			System:     intactSystem,
			UserPrompt: userPrompt,
			Response:   intactResp,
			StopReason: intactStop,
		})

		// Matrix-stripped (sabotaged) condition, same task.
		InterCallDelay()
		callIdx++
		sabResp, sabStop, err := AskSingleTurn(ctx, client, model, sabotagedSystem, userPrompt)
		require.NoError(t, err, "sabotaged AskSingleTurn failed for task %d", i)
		sabotagedBucket = append(sabotagedBucket, adopt.Bucket{Response: sabResp})
		WriteTranscript(t, &Transcript{
			ScenarioID: "adoption-sabotaged-" + itoa(i),
			Category:   "selection",
			Model:      model,
			System:     sabotagedSystem,
			UserPrompt: userPrompt,
			Response:   sabResp,
			StopReason: sabStop,
		})

		// Per-response inspection logged via the SAME single-source classifier —
		// never a re-implemented FirstCommand/Classify.
		intactChose, intactFell := adopt.ClassifyChoice(intactResp)
		sabChose, sabFell := adopt.ClassifyChoice(sabResp)
		t.Logf("task %d: intact first=%q chose=%v fellBack=%v | sabotaged first=%q chose=%v fellBack=%v",
			i, adopt.FirstCommand(intactResp), intactChose, intactFell,
			adopt.FirstCommand(sabResp), sabChose, sabFell)
	}

	// Feed BOTH captured buckets to the SAME pure scorer (single source of truth).
	scIntact, err := adopt.Scorecard(intactBucket)
	require.NoError(t, err, "intact Scorecard must build (bucket above the floor)")
	scSab, err := adopt.Scorecard(sabotagedBucket)
	require.NoError(t, err, "sabotaged Scorecard must build (bucket above the floor)")

	// INFORMATIONAL: log the metrics and the matrix-stripped drop. No hard
	// MaterialDrop assertion here — live models are nondeterministic; the
	// authoritative revert-and-fail is the hermetic Plan 01 test.
	t.Logf("LIVE ADOPTION SCORECARD: intact choice_rate=%.2f fallback_rate=%.2f (%d/%d) | sabotaged choice_rate=%.2f fallback_rate=%.2f (%d/%d) | drop=%.2f",
		scIntact.ChoiceRate, scIntact.FallbackRate, scIntact.Choices, scIntact.Total,
		scSab.ChoiceRate, scSab.FallbackRate, scSab.Choices, scSab.Total,
		scIntact.ChoiceRate-scSab.ChoiceRate)
}

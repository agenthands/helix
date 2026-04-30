//go:build llmjudge

package judge

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	llm "github.com/agenthands/helix/test/oracle/llm"
)

// TestJudge reads transcripts produced by behavioral tests (Plan 01/02) and
// scores them via the LLM judge rubric. Results are informational only —
// never blocks merge (D-04). Sequential with inter-call delays (D-19).
func TestJudge(t *testing.T) {
	llm.SkipWithoutAPIKey(t)

	model, selfJudged := llm.JudgeModel()
	client := llm.NewClient()

	// Read all transcript files.
	transcriptDir := llm.TranscriptDir()
	entries, err := os.ReadDir(transcriptDir)
	if err != nil {
		t.Skipf("no transcripts found at %s — run llm tests first: %v", transcriptDir, err)
		return
	}

	var transcripts []*llm.Transcript
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		tr, err := llm.ReadTranscript(filepath.Join(transcriptDir, entry.Name()))
		if err != nil {
			t.Logf("skipping unreadable transcript %s: %v", entry.Name(), err)
			continue
		}
		transcripts = append(transcripts, tr)
	}

	if len(transcripts) == 0 {
		t.Skip("no transcripts found — run llm tests first")
		return
	}

	t.Logf("Found %d transcripts to judge (model=%s, self_judged=%v)", len(transcripts), model, selfJudged)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
	defer cancel()

	// Collect scores outside t.Run to avoid future race risk if parallelism is added.
	var scores []*Score
	for i, tr := range transcripts {
		if i > 0 {
			llm.InterCallDelay()
		}

		score, err := ScoreTranscript(ctx, client, model, selfJudged, tr)
		if err != nil {
			t.Logf("ERROR scoring %s: %v", tr.ScenarioID, err)
			continue
		}

		WriteScore(t, score)
		t.Logf("Score for %s: verdict=%s total=%.1f", tr.ScenarioID, score.Verdict, score.Total)
		scores = append(scores, score)
	}

	// Compute and write aggregate report.
	if len(scores) > 0 {
		report := Aggregate(scores)
		WriteAggregate(t, report)
		t.Logf("Aggregate: %d pass, %d soft_fail, %d fail out of %d scored",
			report.PassCount, report.SoftFailCount, report.FailCount, report.TotalTranscripts)
	} else {
		t.Log("No scores produced — cannot compute aggregate")
	}
}

// TestJudgeInline is a placeholder for inline judge mode (D-16).
// The canonical path is offline: run -tags=llm first to generate transcripts,
// then -tags=llmjudge to judge them.
func TestJudgeInline(t *testing.T) {
	if os.Getenv("HELIX_INLINE_JUDGE") != "1" {
		t.Skip("HELIX_INLINE_JUDGE not set to 1 — skipping inline mode")
	}
	llm.SkipWithoutAPIKey(t)

	t.Skip("Inline judge mode not yet implemented — use -tags=llm first, then -tags=llmjudge for canonical offline flow.")
}

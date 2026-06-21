//go:build llm || llmjudge

package llm

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/agenthands/helix/test/harness"
)

// Transcript captures a normalized LLM interaction for judge replay (D-13).
// Non-deterministic fields (request IDs, timestamps, token counts, latency)
// are stripped during capture.
type Transcript struct {
	ScenarioID string `json:"scenario_id"`
	Category   string `json:"category"` // "selection", "disambiguation", "interpretation"
	Model      string `json:"model"`
	System     string `json:"system"`
	UserPrompt string `json:"user_prompt"`
	Response   string `json:"response"`
	StopReason string `json:"stop_reason"`
	SelfJudged bool   `json:"self_judged,omitempty"`
}

// WriteTranscript marshals a transcript to indented JSON and writes it to
// testdata/transcripts/{scenario_id}.json (D-14).
//
// Transcripts are run ARTIFACTS, not golden fixtures: their Response field is
// raw, non-deterministic live-LLM output. They are written to a persistent path
// on purpose so the two-stage offline judge flow can read them back across two
// separate `go test` invocations (`-tags=llm` to generate, then `-tags=llmjudge`
// to score — see test/oracle/judge/judge_test.go and TranscriptDir). To keep the
// working tree clean on every keyed run, this directory is .gitignored and the
// previously-committed copies were removed from tracking (WR-93-04); the files
// are local run artifacts, never golden references checked into git.
func WriteTranscript(t *testing.T, tr *Transcript) {
	t.Helper()

	dir := TranscriptDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("creating transcript dir: %v", err)
	}

	data, err := json.MarshalIndent(tr, "", "  ")
	if err != nil {
		t.Fatalf("marshaling transcript: %v", err)
	}

	path := filepath.Join(dir, fmt.Sprintf("%s.json", tr.ScenarioID))
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("writing transcript %s: %v", path, err)
	}
	t.Logf("transcript written: %s", path)
}

// ReadTranscript reads and unmarshals a transcript from a file path.
func ReadTranscript(path string) (*Transcript, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading transcript %s: %w", path, err)
	}
	var tr Transcript
	if err := json.Unmarshal(data, &tr); err != nil {
		return nil, fmt.Errorf("unmarshaling transcript %s: %w", path, err)
	}
	return &tr, nil
}

// TranscriptDir returns the absolute path to the transcript output directory.
func TranscriptDir() string {
	return filepath.Join(harness.ProjectRoot(), "test", "oracle", "llm", "testdata", "transcripts")
}

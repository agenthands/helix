//go:build llm || llmjudge

package llm

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/postfix/serena/test/harness"
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

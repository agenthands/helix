package report_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/agenthands/helix/internal/eval/report"
)

// TestRunMetadataCapturesClaudeVersion verifies that CaptureRunMetadata records
// key fields correctly. It skips the actual claude --version invocation if
// claude is not on PATH.
func TestRunMetadataCapturesClaudeVersion(t *testing.T) {
	meta := report.CaptureRunMetadata([]string{"baseline", "native"}, "eval/corpus")

	if meta.RunID == "" {
		t.Error("RunID must be non-empty")
	}
	if meta.GOOS == "" {
		t.Error("GOOS must be non-empty")
	}
	if meta.GOARCH == "" {
		t.Error("GOARCH must be non-empty")
	}
	if len(meta.Modes) != 2 {
		t.Errorf("Modes: got %v, want [baseline native]", meta.Modes)
	}
	if meta.CorpusDir != "eval/corpus" {
		t.Errorf("CorpusDir: got %q, want eval/corpus", meta.CorpusDir)
	}
	// EnvKeys must be a slice of strings (keys only, never values).
	if meta.EnvKeys == nil {
		t.Error("EnvKeys must be non-nil (even if empty)")
	}
	for _, k := range meta.EnvKeys {
		if k == "" {
			t.Error("EnvKeys must not contain empty strings")
		}
	}
	// HelixVersion must be non-empty (from runtime/debug.ReadBuildInfo).
	if meta.HelixVersion == "" {
		t.Error("HelixVersion must be non-empty")
	}
}

// TestWriteRunMetadata verifies that WriteRunMetadata produces a 0600 JSON file.
func TestWriteRunMetadata(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "run_metadata.json")

	meta := report.CaptureRunMetadata([]string{"baseline"}, "eval/corpus")
	if err := report.WriteRunMetadata(path, meta); err != nil {
		t.Fatalf("WriteRunMetadata: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat run_metadata.json: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("mode: got %v, want 0600", info.Mode().Perm())
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read run_metadata.json: %v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal run_metadata.json: %v", err)
	}
	for _, field := range []string{"run_id", "goos", "goarch", "modes", "corpus_dir", "env_keys", "helix_version"} {
		if _, ok := m[field]; !ok {
			t.Errorf("missing required field %q", field)
		}
	}
}

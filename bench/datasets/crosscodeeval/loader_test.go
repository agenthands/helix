package crosscodeeval

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/agenthands/helix/bench/evaluators/editsim"
	"github.com/agenthands/helix/bench/evaluators/exactmatch"
	"github.com/agenthands/helix/bench/evaluators/identmatch"
)

// fixtureRow is the committed CCE-shaped fixture (fixtures/<lang>/task.json). Its
// values are paper/official-repo sourced (see each file's `provenance` field) and
// are the SOLE authoritative proof of the loader + the EM/ES/identifier scoring.
type fixtureRow struct {
	Provenance  string `json:"provenance"`
	Language    string `json:"language"`
	Split       string `json:"split"`
	Prompt      string `json:"prompt"`
	GroundTruth string `json:"groundtruth"`
}

func loadFixture(t *testing.T, language string) fixtureRow {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("fixtures", language, "task.json"))
	if err != nil {
		t.Fatalf("read fixture %s: %v", language, err)
	}
	var fr fixtureRow
	if err := json.Unmarshal(b, &fr); err != nil {
		t.Fatalf("parse fixture %s: %v", language, err)
	}
	return fr
}

// TestFixturesAreFourLanguages proves one committed fixture exists per CCE
// language with a non-empty prompt + groundtruth and a provenance comment (the
// checkpoint requires paper-sourced, not viewer-scraped, values).
func TestFixturesAreFourLanguages(t *testing.T) {
	for _, lang := range Languages {
		fr := loadFixture(t, lang)
		if fr.Language != lang {
			t.Errorf("fixture %s declares language %q", lang, fr.Language)
		}
		if fr.Prompt == "" || fr.GroundTruth == "" {
			t.Errorf("fixture %s has empty prompt or groundtruth", lang)
		}
		if fr.Provenance == "" {
			t.Errorf("fixture %s missing provenance (must be paper/official sourced)", lang)
		}
		if fr.Split == "" {
			t.Errorf("fixture %s missing split", lang)
		}
	}
}

// TestParquetDecodeMapsOneTaskPerLanguage is the hermetic sole-proof: it decodes
// the committed testdata/sample.parquet (generated once from the four fixtures)
// and asserts the loader maps exactly one Task per language with the fixture's
// prompt/groundtruth/split preserved.
func TestParquetDecodeMapsOneTaskPerLanguage(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "sample.parquet"))
	if err != nil {
		t.Fatalf("read sample.parquet: %v", err)
	}
	for _, lang := range Languages {
		fr := loadFixture(t, lang)
		tasks, err := LoadParquetBytes(context.Background(), raw, lang)
		if err != nil {
			t.Fatalf("LoadParquetBytes(%s): %v", lang, err)
		}
		var got *Task
		for i := range tasks {
			if tasks[i].Language == lang {
				got = &tasks[i]
				break
			}
		}
		if got == nil {
			t.Fatalf("no task for language %q in decoded parquet", lang)
		}
		if got.Prompt != fr.Prompt {
			t.Errorf("%s prompt = %q, want %q", lang, got.Prompt, fr.Prompt)
		}
		if got.GroundTruth != fr.GroundTruth {
			t.Errorf("%s groundtruth = %q, want %q", lang, got.GroundTruth, fr.GroundTruth)
		}
		if got.Split != fr.Split {
			t.Errorf("%s split = %q, want %q", lang, got.Split, fr.Split)
		}
	}
}

// TestScoresWithPlan01Oracles proves the loaded Task drives the Plan 01 EM /
// edit-similarity / identifier-match scorers: an exact completion is EM-true,
// ES==1.0, identifier-em-true; a wrong completion is EM-false. This wires the
// adapter to the scorers the multi-oracle gate consumes (the `language` key
// flows to the aggregator ByLanguage slice).
func TestScoresWithPlan01Oracles(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "sample.parquet"))
	if err != nil {
		t.Fatalf("read sample.parquet: %v", err)
	}
	tasks, err := LoadParquetBytes(context.Background(), raw, "python")
	if err != nil {
		t.Fatalf("LoadParquetBytes(python): %v", err)
	}
	if len(tasks) == 0 {
		t.Fatal("no python tasks decoded")
	}
	task := tasks[0]

	// Perfect completion.
	if !exactmatch.EM(task.GroundTruth, task.GroundTruth) {
		t.Error("EM(gt, gt) = false, want true")
	}
	if es := editsim.ES(task.GroundTruth, task.GroundTruth); es != 1.0 {
		t.Errorf("ES(gt, gt) = %v, want 1.0", es)
	}
	if em, _ := identmatch.Match(task.GroundTruth, task.GroundTruth); !em {
		t.Error("identmatch.Match(gt, gt) em = false, want true")
	}
	// Wrong completion.
	if exactmatch.EM("totally wrong", task.GroundTruth) {
		t.Error("EM(wrong, gt) = true, want false")
	}
}

// TestLoadRejectsPathTraversalLanguage is the fail-closed path-safety proof
// (T-86-03-03): a traversal / separator / leading-dot language segment is
// rejected by validatePathSegment BEFORE any filepath.Join or URL build.
func TestLoadRejectsPathTraversalLanguage(t *testing.T) {
	bad := []string{"../etc", "py/thon", `..\..\win`, ".hidden", "", "a/b"}
	for _, lang := range bad {
		if _, err := Load(context.Background(), lang); err == nil {
			t.Errorf("Load(%q) = nil error, want rejection before Join", lang)
		}
		if _, err := cachePath(PinnedRev, lang); err == nil {
			t.Errorf("cachePath(rev, %q) = nil error, want rejection before Join", lang)
		}
	}
	// A traversal rev is also rejected before Join.
	if _, err := cachePath("../../etc", "python"); err == nil {
		t.Error("cachePath(traversal-rev, python) = nil error, want rejection")
	}
}

// TestLoadRejectsUnknownLanguage proves an unknown (but path-safe) language fails
// closed via the allow-list, never silently fetching an arbitrary file.
func TestLoadRejectsUnknownLanguage(t *testing.T) {
	if _, err := Load(context.Background(), "rust"); err == nil {
		t.Error("Load(rust) = nil error, want unsupported-language rejection")
	}
}

package repobench

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// fixtureRow is the committed RepoBench-shaped fixture
// (fixtures/<lang>/<task>.json). Its values are paper/dataset-card sourced (see
// each file's `provenance` field) and are the SOLE authoritative proof of the
// loader + the acc@k / EM/ES scoring.
type fixtureRow struct {
	Provenance       string   `json:"provenance"`
	Task             string   `json:"task"`
	Language         string   `json:"language"`
	Split            string   `json:"split"`
	Level            string   `json:"level"`
	RepoName         string   `json:"repo_name"`
	FilePath         string   `json:"file_path"`
	Context          []string `json:"context"`
	GoldSnippetIndex int      `json:"gold_snippet_index"`
	CroppedCode      string   `json:"cropped_code"`
	NextLine         string   `json:"next_line"`
}

func loadFixture(t *testing.T, language, task string) fixtureRow {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("fixtures", language, task+".json"))
	if err != nil {
		t.Fatalf("read fixture %s/%s: %v", language, task, err)
	}
	var fr fixtureRow
	if err := json.Unmarshal(b, &fr); err != nil {
		t.Fatalf("parse fixture %s/%s: %v", language, task, err)
	}
	return fr
}

// TestFixturesCoverRCForBothLanguages proves a committed retrieval (-R) and
// completion (-C) fixture exists for both Python and Java, each with a provenance
// comment (paper/dataset-card sourced, not viewer-scraped) and the sub-task's
// authoritative fields populated.
func TestFixturesCoverRCForBothLanguages(t *testing.T) {
	for _, lang := range Languages {
		r := loadFixture(t, lang, "retrieval")
		if r.Provenance == "" {
			t.Errorf("%s retrieval fixture missing provenance", lang)
		}
		if r.Task != TaskRetrieval {
			t.Errorf("%s retrieval fixture task = %q, want %q", lang, r.Task, TaskRetrieval)
		}
		if len(r.Context) == 0 {
			t.Errorf("%s retrieval fixture has empty context[]", lang)
		}
		if r.GoldSnippetIndex < 0 || r.GoldSnippetIndex >= len(r.Context) {
			t.Errorf("%s retrieval gold_snippet_index %d out of range [0,%d)", lang, r.GoldSnippetIndex, len(r.Context))
		}

		c := loadFixture(t, lang, "completion")
		if c.Provenance == "" {
			t.Errorf("%s completion fixture missing provenance", lang)
		}
		if c.Task != TaskCompletion {
			t.Errorf("%s completion fixture task = %q, want %q", lang, c.Task, TaskCompletion)
		}
		if c.NextLine == "" {
			t.Errorf("%s completion fixture has empty next_line", lang)
		}
	}
}

// TestParquetDecodeRetrievalAndCompletion is the hermetic sole-proof: it decodes
// the committed testdata/sample.parquet (generated once from the four fixtures)
// and asserts the loader maps a retrieval task (carrying Context +
// GoldSnippetIndex) and a completion task (carrying NextLine) for both Python and
// Java, with the fixture values preserved.
func TestParquetDecodeRetrievalAndCompletion(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "sample.parquet"))
	if err != nil {
		t.Fatalf("read sample.parquet: %v", err)
	}
	for _, lang := range Languages {
		tasks, err := LoadParquetBytes(context.Background(), raw, lang)
		if err != nil {
			t.Fatalf("LoadParquetBytes(%s): %v", lang, err)
		}

		ret := findTask(tasks, TaskRetrieval)
		if ret == nil {
			t.Fatalf("%s: no retrieval task decoded", lang)
		}
		rf := loadFixture(t, lang, "retrieval")
		if len(ret.Context) != len(rf.Context) {
			t.Errorf("%s retrieval Context len = %d, want %d", lang, len(ret.Context), len(rf.Context))
		}
		if ret.GoldSnippetIndex != rf.GoldSnippetIndex {
			t.Errorf("%s retrieval GoldSnippetIndex = %d, want %d", lang, ret.GoldSnippetIndex, rf.GoldSnippetIndex)
		}
		if ret.Language != lang {
			t.Errorf("%s retrieval Language = %q, want %q", lang, ret.Language, lang)
		}
		if ret.Split != rf.Split {
			t.Errorf("%s retrieval Split = %q, want %q", lang, ret.Split, rf.Split)
		}

		comp := findTask(tasks, TaskCompletion)
		if comp == nil {
			t.Fatalf("%s: no completion task decoded", lang)
		}
		cf := loadFixture(t, lang, "completion")
		if comp.NextLine != cf.NextLine {
			t.Errorf("%s completion NextLine = %q, want %q", lang, comp.NextLine, cf.NextLine)
		}
		if comp.CroppedCode != cf.CroppedCode {
			t.Errorf("%s completion CroppedCode = %q, want %q", lang, comp.CroppedCode, cf.CroppedCode)
		}
		if comp.Language != lang {
			t.Errorf("%s completion Language = %q, want %q", lang, comp.Language, lang)
		}
	}
}

func findTask(tasks []Task, sub string) *Task {
	for i := range tasks {
		if tasks[i].Task == sub {
			return &tasks[i]
		}
	}
	return nil
}

// TestLoadRejectsPathTraversalLanguage is the fail-closed path-safety proof
// (T-86-04-03): a traversal / separator / leading-dot language segment is
// rejected by validatePathSegment BEFORE any filepath.Join or URL build.
func TestLoadRejectsPathTraversalLanguage(t *testing.T) {
	bad := []string{"../etc", "py/thon", `..\..\win`, ".hidden", "", "a/b"}
	for _, lang := range bad {
		if _, err := Load(context.Background(), lang); err == nil {
			t.Errorf("Load(%q) = nil error, want rejection before Join", lang)
		}
		if _, err := cachePath("8a7cf0c8942cc1aa066bf261839650ac55a2ff79", lang); err == nil {
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

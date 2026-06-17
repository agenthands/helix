package languages_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/agenthands/helix/bench/languages"
	golang "github.com/agenthands/helix/bench/languages/go"
)

// corpusRoot is the on-disk datasets root relative to this test file:
// bench/languages/coverage_test.go -> ../datasets (which contains
// internal-toolbench/go/*/task.json).
const corpusRoot = "../datasets"

// TestCoverageGoIsTenOfTen is the headline C2 assertion: the live Go corpus
// (the 10 fixtures from Plans 02-04) covers all 10 declared capabilities.
func TestCoverageGoIsTenOfTen(t *testing.T) {
	declared := golang.GoRunner{}.Capabilities()

	rep, err := languages.Coverage(corpusRoot, "internal-toolbench", "go", declared)
	if err != nil {
		t.Fatalf("Coverage() error: %v", err)
	}
	if rep.Declared != 10 {
		t.Errorf("declared = %d, want 10", rep.Declared)
	}
	if rep.Covered != 10 {
		t.Errorf("covered = %d, want 10 (missing: %v)", rep.Covered, rep.Missing)
	}
	if len(rep.Missing) != 0 {
		t.Errorf("missing = %v, want empty (Go must be 10/10)", rep.Missing)
	}
}

// TestCoverageDetectsGap proves the aggregator is NOT a rubber stamp (D-11 /
// T-78-12): a synthetic corpus missing one capability must report it in Missing.
func TestCoverageDetectsGap(t *testing.T) {
	declared := golang.GoRunner{}.Capabilities()

	// Build a synthetic corpus that covers every capability EXCEPT
	// CapFailureHandling.
	root := t.TempDir()
	langDir := filepath.Join(root, "internal-toolbench", "go")
	if err := os.MkdirAll(langDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for i, c := range declared {
		if c == languages.CapFailureHandling {
			continue // deliberately omit one capability
		}
		dir := filepath.Join(langDir, "synthetic-"+string(c))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		body, _ := json.Marshal(map[string]string{
			"id":         "IT-go-synthetic-" + string(rune('a'+i)),
			"capability": string(c),
		})
		if err := os.WriteFile(filepath.Join(dir, "task.json"), body, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	rep, err := languages.Coverage(root, "internal-toolbench", "go", declared)
	if err != nil {
		t.Fatalf("Coverage() error: %v", err)
	}
	if rep.Declared != 10 {
		t.Errorf("declared = %d, want 10", rep.Declared)
	}
	if rep.Covered != 9 {
		t.Errorf("covered = %d, want 9 (one omitted)", rep.Covered)
	}
	if len(rep.Missing) != 1 || rep.Missing[0] != languages.CapFailureHandling {
		t.Errorf("missing = %v, want [%s]", rep.Missing, languages.CapFailureHandling)
	}
}

// TestCoverageDeduplicatesDeclared is the WR-04 regression guard: a `declared`
// slice that REPEATS a covered capability must never report Covered > Declared.
// Before the fix, Covered was incremented per raw-slice element while Declared
// counted the deduplicated set — so a duplicate inflated Covered past Declared,
// an incoherent report that would also corrupt the 10/10 gate.
func TestCoverageDeduplicatesDeclared(t *testing.T) {
	// A single capability, declared twice.
	declared := []languages.Capability{
		languages.CapFailureHandling,
		languages.CapFailureHandling,
	}

	// Synthetic corpus covering that one capability.
	root := t.TempDir()
	langDir := filepath.Join(root, "internal-toolbench", "go")
	dir := filepath.Join(langDir, "synthetic-dup")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]string{
		"id":         "IT-go-synthetic-dup",
		"capability": string(languages.CapFailureHandling),
	})
	if err := os.WriteFile(filepath.Join(dir, "task.json"), body, 0o644); err != nil {
		t.Fatal(err)
	}

	rep, err := languages.Coverage(root, "internal-toolbench", "go", declared)
	if err != nil {
		t.Fatalf("Coverage() error: %v", err)
	}
	if rep.Declared != 1 {
		t.Errorf("declared = %d, want 1 (deduplicated)", rep.Declared)
	}
	if rep.Covered != 1 {
		t.Errorf("covered = %d, want 1 (deduplicated)", rep.Covered)
	}
	if rep.Covered > rep.Declared {
		t.Errorf("covered (%d) > declared (%d): over-count regression", rep.Covered, rep.Declared)
	}
	if len(rep.Missing) != 0 {
		t.Errorf("missing = %v, want empty", rep.Missing)
	}
}

// TestNamespaceIsITGoZeroT67 is criterion C4: every Go fixture task id matches
// ^IT-go- and NONE matches ^T-67- (zero collision between the IT-go-* corpus and
// the Phase 67 T-67-* planning namespace).
func TestNamespaceIsITGoZeroT67(t *testing.T) {
	reITGo := regexp.MustCompile(`^IT-go-`)
	reT67 := regexp.MustCompile(`^T-67-`)

	ids, err := taskIDs(filepath.Join(corpusRoot, "internal-toolbench", "go"))
	if err != nil {
		t.Fatalf("reading task ids: %v", err)
	}
	if len(ids) == 0 {
		t.Fatal("found 0 task.json ids under internal-toolbench/go")
	}
	for _, id := range ids {
		if !reITGo.MatchString(id) {
			t.Errorf("task id %q does not match ^IT-go-", id)
		}
		if reT67.MatchString(id) {
			t.Errorf("task id %q matches ^T-67- (namespace collision)", id)
		}
	}
}

// TestDocsExist feeds the C1/C4 existence/structure check: both human-facing docs
// are on disk.
func TestDocsExist(t *testing.T) {
	for _, p := range []string{
		filepath.Join(corpusRoot, "internal-toolbench", "CAPABILITIES.md"),
		filepath.Join(corpusRoot, "internal-toolbench", "PHASE67_CROSSWALK.md"),
	} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("expected doc %s to exist: %v", p, err)
		}
	}
}

// taskIDs reads the "id" field of every <langDir>/*/task.json (test helper that
// mirrors the aggregator's capability read but for the id, to assert namespace).
func taskIDs(langDir string) ([]string, error) {
	entries, err := os.ReadDir(langDir)
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(langDir, e.Name(), "task.json"))
		if err != nil {
			continue // dirs without a task.json are not corpus cells
		}
		var meta struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(raw, &meta); err != nil {
			return nil, err
		}
		ids = append(ids, meta.ID)
	}
	return ids, nil
}

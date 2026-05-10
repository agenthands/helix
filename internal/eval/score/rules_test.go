package score_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/agenthands/helix/internal/eval/score"
)

// TestLoadRulesAllConstructs parses a fixture yaml that exercises every DSL
// construct and asserts that every field round-trips correctly.
func TestLoadRulesAllConstructs(t *testing.T) {
	const yaml = `
task_kind: rename
expect_sequence:
  - id: rename-after-references
    score: 1
    pattern:
      - tool: find_references
        args_match:
          symbol: AuthMiddleware
      - tool: rename_symbol
        args_match:
          old_name: AuthMiddleware
forbid_sequence:
  - id: rename-by-grep
    score: -1
    pattern:
      - tool: search_for_pattern
      - tool: replace_in_file
expect_set:
  - id: verified-after-edit
    score: 1
    tools:
      - rename_symbol
      - verify_edit
forbid_set:
  - id: delete-without-references
    score: -1
    when:
      tool_used: delete_file
    require_prior:
      any_of:
        - find_references
        - analyze_blast_radius
receipts:
  - id: safe-delete-with-receipts
    score: 1
    when:
      tool_used: safe_delete_symbol
    require:
      receipts_non_empty: true
`
	dir := t.TempDir()
	path := filepath.Join(dir, "expected_tools.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	r, err := score.LoadRules(path)
	if err != nil {
		t.Fatalf("LoadRules: %v", err)
	}

	if r.TaskKind != "rename" {
		t.Errorf("TaskKind = %q; want %q", r.TaskKind, "rename")
	}

	// ExpectSequence
	if len(r.ExpectSequence) != 1 {
		t.Fatalf("ExpectSequence len = %d; want 1", len(r.ExpectSequence))
	}
	es := r.ExpectSequence[0]
	if es.ID != "rename-after-references" {
		t.Errorf("ExpectSequence[0].ID = %q", es.ID)
	}
	if es.Score != 1 {
		t.Errorf("ExpectSequence[0].Score = %d; want 1", es.Score)
	}
	if len(es.Pattern) != 2 {
		t.Fatalf("pattern len = %d; want 2", len(es.Pattern))
	}
	if es.Pattern[0].Tool != "find_references" {
		t.Errorf("pattern[0].Tool = %q", es.Pattern[0].Tool)
	}
	if es.Pattern[0].ArgsMatch["symbol"] != "AuthMiddleware" {
		t.Errorf("pattern[0].ArgsMatch[symbol] = %q", es.Pattern[0].ArgsMatch["symbol"])
	}

	// ForbidSequence
	if len(r.ForbidSequence) != 1 {
		t.Fatalf("ForbidSequence len = %d; want 1", len(r.ForbidSequence))
	}
	fs := r.ForbidSequence[0]
	if fs.ID != "rename-by-grep" {
		t.Errorf("ForbidSequence[0].ID = %q", fs.ID)
	}
	if fs.Score != -1 {
		t.Errorf("ForbidSequence[0].Score = %d; want -1", fs.Score)
	}

	// ExpectSet
	if len(r.ExpectSet) != 1 {
		t.Fatalf("ExpectSet len = %d; want 1", len(r.ExpectSet))
	}
	eset := r.ExpectSet[0]
	if eset.ID != "verified-after-edit" {
		t.Errorf("ExpectSet[0].ID = %q", eset.ID)
	}
	if len(eset.Tools) != 2 {
		t.Errorf("ExpectSet[0].Tools len = %d; want 2", len(eset.Tools))
	}

	// ForbidSet
	if len(r.ForbidSet) != 1 {
		t.Fatalf("ForbidSet len = %d; want 1", len(r.ForbidSet))
	}
	fset := r.ForbidSet[0]
	if fset.ID != "delete-without-references" {
		t.Errorf("ForbidSet[0].ID = %q", fset.ID)
	}
	if fset.When.ToolUsed != "delete_file" {
		t.Errorf("ForbidSet[0].When.ToolUsed = %q", fset.When.ToolUsed)
	}
	if len(fset.RequirePrior.AnyOf) != 2 {
		t.Errorf("ForbidSet[0].RequirePrior.AnyOf len = %d; want 2", len(fset.RequirePrior.AnyOf))
	}

	// Receipts
	if len(r.Receipts) != 1 {
		t.Fatalf("Receipts len = %d; want 1", len(r.Receipts))
	}
	rec := r.Receipts[0]
	if rec.ID != "safe-delete-with-receipts" {
		t.Errorf("Receipts[0].ID = %q", rec.ID)
	}
	if rec.When.ToolUsed != "safe_delete_symbol" {
		t.Errorf("Receipts[0].When.ToolUsed = %q", rec.When.ToolUsed)
	}
	if !rec.Require.ReceiptsNonEmpty {
		t.Error("Receipts[0].Require.ReceiptsNonEmpty = false; want true")
	}
}

// TestLoadRulesUnknownKeyFails asserts that a yaml with a typo'd key
// returns a non-nil error (KnownFields strict parsing).
func TestLoadRulesUnknownKeyFails(t *testing.T) {
	const yaml = `
task_kind: rename
expectedSequence:
  - id: bad-typo
    score: 1
    pattern:
      - tool: find_references
`
	dir := t.TempDir()
	path := filepath.Join(dir, "expected_tools.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := score.LoadRules(path)
	if err == nil {
		t.Fatal("LoadRules: expected error for unknown key, got nil")
	}
}

// TestLoadRulesMissingRequired asserts that a rule missing id returns an error.
func TestLoadRulesMissingRequired(t *testing.T) {
	const yaml = `
task_kind: rename
expect_sequence:
  - score: 1
    pattern:
      - tool: find_references
`
	dir := t.TempDir()
	path := filepath.Join(dir, "expected_tools.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := score.LoadRules(path)
	if err == nil {
		t.Fatal("LoadRules: expected error for missing id, got nil")
	}
}

// TestLoadRulesArgsMatchRegex asserts that a _regex suffix key is compiled
// into ArgsMatchRegex and that a bad regex returns an error.
func TestLoadRulesArgsMatchRegex(t *testing.T) {
	t.Run("valid_regex", func(t *testing.T) {
		const yaml = `
task_kind: rename
expect_sequence:
  - id: rename-exported
    score: 1
    pattern:
      - tool: rename_symbol
        args_match:
          symbol_regex: "^[A-Z][a-zA-Z0-9_]+$"
`
		dir := t.TempDir()
		path := filepath.Join(dir, "expected_tools.yaml")
		if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
			t.Fatal(err)
		}

		r, err := score.LoadRules(path)
		if err != nil {
			t.Fatalf("LoadRules: %v", err)
		}
		step := r.ExpectSequence[0].Pattern[0]
		if step.ArgsMatchRegex["symbol_regex"] == nil {
			t.Error("ArgsMatchRegex[symbol_regex] is nil; want compiled *regexp.Regexp")
		}
	})

	t.Run("invalid_regex", func(t *testing.T) {
		const yaml = `
task_kind: rename
expect_sequence:
  - id: bad-regex
    score: 1
    pattern:
      - tool: rename_symbol
        args_match:
          symbol_regex: "[invalid("
`
		dir := t.TempDir()
		path := filepath.Join(dir, "expected_tools.yaml")
		if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
			t.Fatal(err)
		}

		_, err := score.LoadRules(path)
		if err == nil {
			t.Fatal("LoadRules: expected error for invalid regex, got nil")
		}
	})
}

// TestLoadRulesNegativeScoreNormalized asserts that forbid_sequence with
// score: -1 round-trips identically.
func TestLoadRulesNegativeScoreNormalized(t *testing.T) {
	const yaml = `
task_kind: rename
forbid_sequence:
  - id: rename-by-grep
    score: -1
    pattern:
      - tool: search_for_pattern
      - tool: replace_in_file
`
	dir := t.TempDir()
	path := filepath.Join(dir, "expected_tools.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	r, err := score.LoadRules(path)
	if err != nil {
		t.Fatalf("LoadRules: %v", err)
	}
	if len(r.ForbidSequence) != 1 {
		t.Fatalf("ForbidSequence len = %d; want 1", len(r.ForbidSequence))
	}
	if r.ForbidSequence[0].Score != -1 {
		t.Errorf("Score = %d; want -1", r.ForbidSequence[0].Score)
	}
}

// TestValidateRulesCommand exercises the validate-rules subcommand end-to-end.
// It creates a small fixture corpus, invokes validate-rules, and verifies that
// a well-formed corpus returns 0 and a corpus with a typo'd key returns non-zero.
func TestValidateRulesCommand(t *testing.T) {
	// Build a fixture corpus with one good task.
	corpus := t.TempDir()
	goodTask := filepath.Join(corpus, "good-task-001")
	if err := os.MkdirAll(goodTask, 0o755); err != nil {
		t.Fatal(err)
	}
	goodYAML := `
task_kind: rename
expect_sequence:
  - id: rename-after-references
    score: 1
    pattern:
      - tool: find_references
      - tool: rename_symbol
`
	if err := os.WriteFile(filepath.Join(goodTask, "expected_tools.yaml"), []byte(goodYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	// Good corpus: expect 0 errors.
	errs := score.ValidateCorpus(corpus)
	if len(errs) != 0 {
		t.Errorf("ValidateCorpus(good): expected 0 errors, got %d: %v", len(errs), errs)
	}

	// Add a bad task.
	badTask := filepath.Join(corpus, "bad-task-001")
	if err := os.MkdirAll(badTask, 0o755); err != nil {
		t.Fatal(err)
	}
	badYAML := `
task_kind: rename
expectedSequence:
  - id: typo-key
    score: 1
    pattern:
      - tool: find_references
`
	if err := os.WriteFile(filepath.Join(badTask, "expected_tools.yaml"), []byte(badYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	// Bad corpus: expect >=1 error.
	errs = score.ValidateCorpus(corpus)
	if len(errs) == 0 {
		t.Error("ValidateCorpus(bad): expected >=1 error, got 0")
	}
}

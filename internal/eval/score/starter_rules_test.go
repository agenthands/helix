package score_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/agenthands/helix/internal/eval/score"
	"github.com/agenthands/helix/internal/eval/trace"
)

// corpusPath returns the absolute path to eval/corpus/<task-id>/expected_tools.yaml.
// It uses the source file location to resolve the repo root at test time so the
// test works regardless of the working directory from which go test is invoked.
func corpusPath(taskID string) string {
	_, thisFile, _, _ := runtime.Caller(0)
	// thisFile is internal/eval/score/starter_rules_test.go
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
	return filepath.Join(repoRoot, "eval", "corpus", taskID, "expected_tools.yaml")
}

// TestStarterRulesRename loads go-rename-public-001/expected_tools.yaml and
// asserts that a synthetic "ideal" rename trace earns a positive total and
// that a "bad" trace (grep + replace_in_file) earns a negative total.
func TestStarterRulesRename(t *testing.T) {
	rules, err := score.LoadRules(corpusPath("go-rename-public-001"))
	if err != nil {
		t.Fatalf("LoadRules go-rename-public-001: %v", err)
	}

	// Ideal trace: find_references(AuthMiddleware) → rename_symbol(old_name=AuthMiddleware).
	idealTrace := trace.MergedTrace{
		SchemaVersion: "1",
		TaskID:        "go-rename-public-001",
		Mode:          "semantic",
		Events: []trace.Event{
			{Source: "daemon", Kind: trace.KindToolCall, Tool: "find_references", Outcome: "success", ArgsSummary: `{"symbol":"AuthMiddleware"}`},
			{Source: "daemon", Kind: trace.KindToolCall, Tool: "rename_symbol", Outcome: "success", ArgsSummary: `{"old_name":"AuthMiddleware","new_name":"AuthGuard"}`},
		},
	}
	idealScore := score.Apply(idealTrace, rules)
	if idealScore.Total <= 0 {
		t.Errorf("ideal rename trace: Total = %d; want > 0", idealScore.Total)
	}

	// Bad trace: search_for_pattern → replace_in_file(find_regex=AuthMiddleware).
	badTrace := trace.MergedTrace{
		SchemaVersion: "1",
		TaskID:        "go-rename-public-001",
		Mode:          "semantic",
		Events: []trace.Event{
			{Source: "daemon", Kind: trace.KindToolCall, Tool: "search_for_pattern", Outcome: "success"},
			{Source: "daemon", Kind: trace.KindToolCall, Tool: "replace_in_file", Outcome: "success", ArgsSummary: `{"find_regex":"AuthMiddleware"}`},
		},
	}
	badScore := score.Apply(badTrace, rules)
	if badScore.Total >= 0 {
		t.Errorf("bad rename trace: Total = %d; want < 0", badScore.Total)
	}
}

// TestStarterRulesDelete loads go-delete-symbol-001/expected_tools.yaml and
// asserts that a trace with safe_delete_symbol + receipts earns +1, while a
// trace with delete_file (no prior references) earns -1.
func TestStarterRulesDelete(t *testing.T) {
	rules, err := score.LoadRules(corpusPath("go-delete-symbol-001"))
	if err != nil {
		t.Fatalf("LoadRules go-delete-symbol-001: %v", err)
	}

	// Ideal trace: find_references → safe_delete_symbol with non-empty receipts.
	idealTrace := trace.MergedTrace{
		SchemaVersion: "1",
		TaskID:        "go-delete-symbol-001",
		Mode:          "semantic",
		Events: []trace.Event{
			{Source: "daemon", Kind: trace.KindToolCall, Tool: "find_references", Outcome: "success", ArgsSummary: `{"symbol":"LegacyParser"}`},
			{
				Source:      "daemon",
				Kind:        trace.KindToolCall,
				Tool:        "safe_delete_symbol",
				Outcome:     "success",
				ArgsSummary: `symbol=LegacyParser receipts:["rcpt-001","rcpt-002"]`,
			},
		},
	}
	idealScore := score.Apply(idealTrace, rules)
	if idealScore.Total <= 0 {
		t.Errorf("ideal delete trace: Total = %d; want > 0", idealScore.Total)
	}

	// Bad trace: delete_file without prior find_references.
	badTrace := trace.MergedTrace{
		SchemaVersion: "1",
		TaskID:        "go-delete-symbol-001",
		Mode:          "semantic",
		Events: []trace.Event{
			{Source: "daemon", Kind: trace.KindToolCall, Tool: "delete_file", Outcome: "success"},
		},
	}
	badScore := score.Apply(badTrace, rules)
	if badScore.Total >= 0 {
		t.Errorf("bad delete trace: Total = %d; want < 0", badScore.Total)
	}
}

// TestStarterRulesPublicAPI loads go-public-api-001/expected_tools.yaml and
// asserts that analyze_blast_radius before replace_symbol_body earns +1, while
// replace_symbol_body without blast-radius check earns -1.
func TestStarterRulesPublicAPI(t *testing.T) {
	rules, err := score.LoadRules(corpusPath("go-public-api-001"))
	if err != nil {
		t.Fatalf("LoadRules go-public-api-001: %v", err)
	}

	// Ideal trace: analyze_blast_radius(ProcessRequest) → replace_symbol_body(ProcessRequest).
	idealTrace := trace.MergedTrace{
		SchemaVersion: "1",
		TaskID:        "go-public-api-001",
		Mode:          "semantic",
		Events: []trace.Event{
			{Source: "daemon", Kind: trace.KindToolCall, Tool: "analyze_blast_radius", Outcome: "success", ArgsSummary: `{"symbol":"ProcessRequest"}`},
			{Source: "daemon", Kind: trace.KindToolCall, Tool: "replace_symbol_body", Outcome: "success", ArgsSummary: `{"symbol":"ProcessRequest"}`},
		},
	}
	idealScore := score.Apply(idealTrace, rules)
	if idealScore.Total <= 0 {
		t.Errorf("ideal public-api trace: Total = %d; want > 0", idealScore.Total)
	}

	// Bad trace: replace_symbol_body without prior analyze_blast_radius.
	badTrace := trace.MergedTrace{
		SchemaVersion: "1",
		TaskID:        "go-public-api-001",
		Mode:          "semantic",
		Events: []trace.Event{
			{Source: "daemon", Kind: trace.KindToolCall, Tool: "replace_symbol_body", Outcome: "success", ArgsSummary: `{"symbol":"ProcessRequest"}`},
		},
	}
	badScore := score.Apply(badTrace, rules)
	if badScore.Total >= 0 {
		t.Errorf("bad public-api trace: Total = %d; want < 0", badScore.Total)
	}
}

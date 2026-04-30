package edit

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/agenthands/helix/internal/fuzzy"
	gen "github.com/agenthands/helix/protocol/gen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReplaceBodyFuzzy_ExactMatchWithinBody(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.go")
	content := "package main\n\nfunc hello() {\n\tfmt.Println(\"hello\")\n\tfmt.Println(\"world\")\n}\n"
	require.NoError(t, os.WriteFile(filePath, []byte(content), 0o644))

	plan := &EditPlan{
		URI:        "file://" + filePath,
		SymbolName: "hello",
		EditType:   EditTypeReplaceBody,
		NewContent: "\tfmt.Println(\"replaced\")",
		Range: gen.Range{
			Start: gen.Position{Line: 2, Character: 0},
			End:   gen.Position{Line: 5, Character: 1},
		},
	}

	// searchBody matches a sub-region within the body exactly.
	info, err := ReplaceBodyWithPlan(context.Background(), nil, nil, plan, "go", "\tfmt.Println(\"hello\")")
	require.NoError(t, err)
	require.NotNil(t, info)
	assert.Equal(t, fuzzy.StrategyExact, info.Strategy)
	assert.Equal(t, 1.0, info.Score)

	got, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Contains(t, string(got), "replaced")
	assert.Contains(t, string(got), "world") // Other line should be preserved
}

func TestReplaceBodyFuzzy_WhitespaceNormalized(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.go")
	// File has a tab-indented line.
	content := "package main\n\nfunc hello() {\n\tfmt.Println(\"hello\")\n}\n"
	require.NoError(t, os.WriteFile(filePath, []byte(content), 0o644))

	plan := &EditPlan{
		URI:        "file://" + filePath,
		SymbolName: "hello",
		EditType:   EditTypeReplaceBody,
		NewContent: "\tfmt.Println(\"replaced\")",
		Range: gen.Range{
			Start: gen.Position{Line: 2, Character: 0},
			End:   gen.Position{Line: 4, Character: 1},
		},
	}

	// searchBody has trailing spaces -- whitespace-normalized should match.
	info, err := ReplaceBodyWithPlan(context.Background(), nil, nil, plan, "go", "\tfmt.Println(\"hello\")  ")
	require.NoError(t, err)
	require.NotNil(t, info)
	assert.Equal(t, fuzzy.StrategyWhitespace, info.Strategy)
	assert.Equal(t, 0.95, info.Score)
}

func TestReplaceBodyFuzzy_NoSearchBody_FullReplace(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.go")
	content := "package main\n\nfunc hello() {\n\tfmt.Println(\"hello\")\n}\n"
	require.NoError(t, os.WriteFile(filePath, []byte(content), 0o644))

	plan := &EditPlan{
		URI:        "file://" + filePath,
		SymbolName: "hello",
		EditType:   EditTypeReplaceBody,
		NewContent: "NEW BODY",
		Range: gen.Range{
			Start: gen.Position{Line: 2, Character: 0},
			End:   gen.Position{Line: 4, Character: 1},
		},
	}

	// Empty searchBody -- full body replace (D-02: behavior unchanged).
	info, err := ReplaceBodyWithPlan(context.Background(), nil, nil, plan, "go", "")
	require.NoError(t, err)
	assert.Nil(t, info) // No fuzzy info for full body replace (D-08).

	got, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Contains(t, string(got), "NEW BODY")
}

func TestReplaceBodyFuzzy_NoMatchInBody(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.go")
	content := "package main\n\nfunc hello() {\n\tfmt.Println(\"hello\")\n}\n"
	require.NoError(t, os.WriteFile(filePath, []byte(content), 0o644))

	plan := &EditPlan{
		URI:        "file://" + filePath,
		SymbolName: "hello",
		EditType:   EditTypeReplaceBody,
		NewContent: "replacement",
		Range: gen.Range{
			Start: gen.Position{Line: 2, Character: 0},
			End:   gen.Position{Line: 4, Character: 1},
		},
	}

	// searchBody doesn't match anything in the body.
	_, err := ReplaceBodyWithPlan(context.Background(), nil, nil, plan, "go", "completely_nonexistent_text")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no fuzzy match found")
}

func TestReplaceBodyFuzzy_OffsetTranslation(t *testing.T) {
	// Verify that fuzzy match byte offsets are correctly translated from
	// body-relative to absolute file positions (Pitfall 2).
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.go")
	// "package main\n\n" is 14 bytes prefix before the function body.
	content := "package main\n\nfunc greet() {\n\tfmt.Println(\"alpha\")\n\tfmt.Println(\"beta\")\n\tfmt.Println(\"gamma\")\n}\n"
	require.NoError(t, os.WriteFile(filePath, []byte(content), 0o644))

	plan := &EditPlan{
		URI:        "file://" + filePath,
		SymbolName: "greet",
		EditType:   EditTypeReplaceBody,
		NewContent: "\tfmt.Println(\"REPLACED\")",
		Range: gen.Range{
			Start: gen.Position{Line: 2, Character: 0},
			End:   gen.Position{Line: 6, Character: 1},
		},
	}

	// Target just the "beta" line, leaving alpha and gamma untouched.
	info, err := ReplaceBodyWithPlan(context.Background(), nil, nil, plan, "go", "\tfmt.Println(\"beta\")")
	require.NoError(t, err)
	require.NotNil(t, info)
	assert.Equal(t, fuzzy.StrategyExact, info.Strategy)

	got, err := os.ReadFile(filePath)
	require.NoError(t, err)
	result := string(got)
	assert.Contains(t, result, "alpha", "alpha line should be preserved")
	assert.Contains(t, result, "REPLACED", "beta line should be replaced")
	assert.Contains(t, result, "gamma", "gamma line should be preserved")
	assert.NotContains(t, result, "beta", "original beta line should be gone")
}

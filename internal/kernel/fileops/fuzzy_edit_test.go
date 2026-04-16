package fileops

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/postfix/serena/internal/fuzzy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFuzzyEdit_ExactMatch(t *testing.T) {
	root := t.TempDir()
	content := "func hello() {\n\tfmt.Println(\"hello\")\n}\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, "test.go"), []byte(content), 0o644))

	result, err := FuzzyEdit(root, "test.go", "func hello() {\n\tfmt.Println(\"hello\")\n}", "func hello() {\n\tfmt.Println(\"world\")\n}", false)
	require.NoError(t, err)
	assert.Equal(t, fuzzy.StrategyExact, result.Strategy)
	assert.Equal(t, 1.0, result.Score)

	got, err := os.ReadFile(filepath.Join(root, "test.go"))
	require.NoError(t, err)
	assert.Contains(t, string(got), "world")
}

func TestFuzzyEdit_WhitespaceNormalized(t *testing.T) {
	root := t.TempDir()
	// Source has trailing spaces on lines
	content := "func hello() {  \n\tfmt.Println(\"hello\")  \n}\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, "test.go"), []byte(content), 0o644))

	// Search without trailing spaces -- whitespace-normalized matches
	result, err := FuzzyEdit(root, "test.go", "func hello() {\n\tfmt.Println(\"hello\")\n}", "func hello() {\n\tfmt.Println(\"world\")\n}", false)
	require.NoError(t, err)
	assert.Equal(t, fuzzy.StrategyWhitespace, result.Strategy)
	assert.Equal(t, 0.95, result.Score)

	got, err := os.ReadFile(filepath.Join(root, "test.go"))
	require.NoError(t, err)
	assert.Contains(t, string(got), "world")
}

func TestFuzzyEdit_NoMatch(t *testing.T) {
	root := t.TempDir()
	content := "func hello() {\n\tfmt.Println(\"hello\")\n}\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, "test.go"), []byte(content), 0o644))

	_, err := FuzzyEdit(root, "test.go", "func nonexistent() {}", "replacement", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no fuzzy match found")
}

func TestFuzzyEdit_InvalidPath(t *testing.T) {
	root := t.TempDir()
	_, err := FuzzyEdit(root, "nonexistent.go", "search", "replace", false)
	require.Error(t, err)
}

func TestFuzzyEdit_Ellipsis(t *testing.T) {
	root := t.TempDir()
	content := "line1\nline2\nline3\nline4\nline5\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, "test.txt"), []byte(content), 0o644))

	// With ellipsis enabled, "..." segments should match
	result, err := FuzzyEdit(root, "test.txt", "line1\n...\nline5", "LINE1\n...\nLINE5", true)
	require.NoError(t, err)
	assert.NotNil(t, result)

	got, _ := os.ReadFile(filepath.Join(root, "test.txt"))
	assert.Contains(t, string(got), "LINE1")
	assert.Contains(t, string(got), "LINE5")
}

func TestFuzzyEdit_EllipsisDisabled(t *testing.T) {
	root := t.TempDir()
	content := "line1\nline2\nline3\nline4\nline5\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, "test.txt"), []byte(content), 0o644))

	// With ellipsis disabled, "..." is treated as literal text -- should fail
	_, err := FuzzyEdit(root, "test.txt", "line1\n...\nline5", "LINE1\n...\nLINE5", false)
	require.Error(t, err)
}

// --- replace_in_file fuzzy fallback tests (FUZZ-06) ---

func TestReplaceInFileFuzzyFallback_LiteralNoMatch_FuzzySucceeds(t *testing.T) {
	root := t.TempDir()
	// Source has trailing spaces -- literal match of clean pattern will fail
	content := "func hello() {  \n\tfmt.Println(\"hello\")  \n}\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, "test.go"), []byte(content), 0o644))

	// Literal match will fail (no trailing spaces in pattern), but fuzzy should succeed
	// This tests the building blocks: ReplaceInFile returns 0, then fuzzy.Match succeeds.
	count, err := ReplaceInFile(root, "test.go", "func hello() {\n\tfmt.Println(\"hello\")\n}", "replacement", false)
	require.NoError(t, err)
	assert.Equal(t, 0, count) // Literal match returns 0

	// Verify fuzzy.Match would succeed on the same content
	fileContent, err := ReadFile(root, "test.go")
	require.NoError(t, err)
	fResult, fErr := fuzzy.Match(fileContent, "func hello() {\n\tfmt.Println(\"hello\")\n}", fuzzy.Options{
		Replacement:   "replacement",
		AllowEllipsis: false,
	})
	require.NoError(t, fErr)
	assert.Equal(t, fuzzy.StrategyWhitespace, fResult.Strategy)
}

func TestReplaceInFile_RegexNoFuzzyFallback(t *testing.T) {
	root := t.TempDir()
	content := "func hello() {}\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, "test.go"), []byte(content), 0o644))

	// Regex that doesn't match -- should NOT trigger fuzzy fallback
	count, err := ReplaceInFile(root, "test.go", "nonexistent_pattern_xyz", "replacement", true)
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	// File should be unchanged
	got, _ := os.ReadFile(filepath.Join(root, "test.go"))
	assert.Equal(t, "func hello() {}\n", string(got))
}

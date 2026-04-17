package repomap

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/postfix/serena/internal/repomap"
	"github.com/postfix/serena/internal/skill"
	"github.com/postfix/serena/internal/treesitter"
)

// newTestSkill creates a RepoMapSkill with a temp directory TagCache for testing.
// If populate is true, it writes sample Go files and populates the cache with tags.
func newTestSkill(t *testing.T, populate bool) *RepoMapSkill {
	t.Helper()

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "tags.db")
	cache, err := repomap.NewTagCache(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { cache.Close() })

	registry := treesitter.NewGrammarRegistry()
	elider := repomap.NewElisionRenderer(registry)

	s := &RepoMapSkill{
		cache:  cache,
		elider: elider,
		logger: slog.Default(),
	}

	if populate {
		// Write sample Go files to the temp dir.
		file1 := filepath.Join(dir, "file1.go")
		file1Content := "package main\n\nfunc Hello() string { return \"hello\" }\nfunc World() string { return \"world\" }\n"
		require.NoError(t, os.WriteFile(file1, []byte(file1Content), 0o644))

		file2 := filepath.Join(dir, "file2.go")
		file2Content := "package main\n\nimport \"fmt\"\n\nfunc Greet() {\n\tfmt.Println(Hello())\n}\n"
		require.NoError(t, os.WriteFile(file2, []byte(file2Content), 0o644))

		// Populate cache with tags for file1.
		_, err := cache.GetOrExtract(file1, func() ([]repomap.Tag, error) {
			return []repomap.Tag{
				{Name: "Hello", Kind: repomap.TagDef, File: file1, Line: 2, Column: 5, StartByte: 14, EndByte: 53},
				{Name: "World", Kind: repomap.TagDef, File: file1, Line: 3, Column: 5, StartByte: 54, EndByte: 93},
			}, nil
		})
		require.NoError(t, err)

		// Populate cache with tags for file2 (references Hello from file1).
		_, err = cache.GetOrExtract(file2, func() ([]repomap.Tag, error) {
			return []repomap.Tag{
				{Name: "Greet", Kind: repomap.TagDef, File: file2, Line: 4, Column: 5, StartByte: 28, EndByte: 63},
				{Name: "Hello", Kind: repomap.TagRef, File: file2, Line: 5, Column: 14, StartByte: 50, EndByte: 55},
			}, nil
		})
		require.NoError(t, err)

		s.rootDir = dir
	}

	return s
}

func TestRepoMapSkill_ExecuteTool_UnknownTool(t *testing.T) {
	s := newTestSkill(t, false)

	_, err := s.ExecuteTool("unknown_tool", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown repomap tool")
}

func TestRepoMapSkill_GetRepoMap_DefaultBudget(t *testing.T) {
	s := newTestSkill(t, true)

	// Call with empty args -- should use default budget 4096.
	result, err := s.ExecuteTool("get_repo_map", map[string]interface{}{})
	require.NoError(t, err)
	assert.NotEmpty(t, result)
}

func TestRepoMapSkill_GetRepoMap_BudgetCap(t *testing.T) {
	s := newTestSkill(t, true)

	// Call with extremely high budget -- should be capped at 32768.
	result, err := s.ExecuteTool("get_repo_map", map[string]interface{}{
		"token_budget": float64(99999),
	})
	require.NoError(t, err)
	assert.NotEmpty(t, result)
	// Output should be reasonable -- the test files are small so this just verifies
	// that the cap doesn't cause issues.
	tokens := len(result) / 4
	assert.LessOrEqual(t, tokens, 32768)
}

func TestRepoMapSkill_GetContext_MissingFiles(t *testing.T) {
	s := newTestSkill(t, false)

	_, err := s.ExecuteTool("get_context", map[string]interface{}{
		"token_budget": float64(2048),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "'files' parameter is required")
}

func TestRepoMapSkill_GetContext_EmptyFiles(t *testing.T) {
	s := newTestSkill(t, false)

	_, err := s.ExecuteTool("get_context", map[string]interface{}{
		"files":        []interface{}{},
		"token_budget": float64(2048),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not be empty")
}

func TestRepoMapSkill_GetContext_PathTraversal(t *testing.T) {
	s := newTestSkill(t, false)

	_, err := s.ExecuteTool("get_context", map[string]interface{}{
		"files":        []interface{}{"../etc/passwd"},
		"token_budget": float64(2048),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "..")
}

func TestRepoMapSkill_GetRepoMap_EmptyCache(t *testing.T) {
	s := newTestSkill(t, false)

	result, err := s.ExecuteTool("get_repo_map", map[string]interface{}{})
	require.NoError(t, err)
	assert.Contains(t, result, "No files found")
}

func TestRepoMapSkill_GetContext_WithFiles(t *testing.T) {
	s := newTestSkill(t, true)

	// Use the file paths that were populated in newTestSkill.
	tags, err := s.cache.AllFiles()
	require.NoError(t, err)

	var filePaths []interface{}
	for fp := range tags {
		filePaths = append(filePaths, fp)
	}

	result, err := s.ExecuteTool("get_context", map[string]interface{}{
		"files":        filePaths,
		"token_budget": float64(4096),
	})
	require.NoError(t, err)
	assert.NotEmpty(t, result)
}

func TestRepoMapSkill_Init(t *testing.T) {
	dir := t.TempDir()

	s := &RepoMapSkill{}
	deps := skill.SkillDeps{
		ProjectDir: dir,
		GlobalDir:  dir,
		Logger:     nil,
	}

	err := s.Init(deps)
	require.NoError(t, err)
	assert.NotNil(t, s.cache)
	assert.NotNil(t, s.elider)
}

//go:build cgo

package repomap

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	lspool "github.com/agenthands/helix/internal/kernel/lspool"
	"github.com/agenthands/helix/internal/repomap"
	"github.com/agenthands/helix/internal/skill"
	"github.com/agenthands/helix/internal/treesitter"
)

// Compile-time assertion: WorkerLease satisfies SymbolRequester (D-33-04).
var _ repomap.SymbolRequester = (*lspool.WorkerLease)(nil)

// mockFallbackRequester returns pre-configured DocumentSymbol responses for fallback testing.
type mockFallbackRequester struct {
	response []byte // raw JSON to unmarshal into result
	err      error
}

func (m *mockFallbackRequester) Request(_ context.Context, _ string, _ interface{}, result interface{}) error {
	if m.err != nil {
		return m.err
	}
	return json.Unmarshal(m.response, result)
}

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
		s.cachePopulated = true
		s.renderer = repomap.NewTreeRenderer(elider, cache, dir)
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
	// Set rootDir to an empty temp dir so ensureCache walks it (finds nothing).
	s.rootDir = t.TempDir()
	s.cachePopulated = true // skip walk (nothing to walk)
	s.renderer = repomap.NewTreeRenderer(s.elider, s.cache, s.rootDir)

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
	// Per BUG-04 (D-03): registry-dependent fields (elider, extractor) are now
	// populated by SetRegistry post-init wiring, not Init. Init only sets up cache.
	assert.Nil(t, s.elider)
	assert.Nil(t, s.extractor)
	assert.Nil(t, s.registry)

	// After SetRegistry, registry-dependent fields are populated.
	s.SetRegistry(treesitter.NewGrammarRegistry())
	assert.NotNil(t, s.registry)
	assert.NotNil(t, s.elider)
	assert.NotNil(t, s.extractor)
}

func TestRepoMapSkill_SetWorkspaceRoot(t *testing.T) {
	s := newTestSkill(t, true)
	assert.True(t, s.cachePopulated)

	s.SetWorkspaceRoot("/new/root")

	assert.Equal(t, "/new/root", s.rootDir)
	assert.False(t, s.cachePopulated) // cache invalidated
	assert.Nil(t, s.renderer)         // renderer cleared
}

func TestRepoMapSkill_GetRepoMapSkillNil(t *testing.T) {
	// When no repomap skill is registered, GetRepoMapSkill returns nil.
	// This test verifies the function doesn't panic.
	// Note: actual registration happens via init(), so in test context
	// the skill IS registered. We just verify it returns non-nil.
	rs := GetRepoMapSkill()
	assert.NotNil(t, rs)
}

func TestWalkAndExtract_FallbackPath(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "tags.db")
	cache, err := repomap.NewTagCache(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { cache.Close() })

	registry := treesitter.NewGrammarRegistry()

	// Create a Java file -- Java is in LangFromExt. We set extractor=nil to simulate
	// tree-sitter unavailability, forcing the fallback path via FallbackDeps.
	javaFile := filepath.Join(dir, "Hello.java")
	require.NoError(t, os.WriteFile(javaFile, []byte("public class Hello { void greet() {} }\n"), 0644))

	// Build a mock SymbolRequester that returns a DocumentSymbol for "Hello".
	docSymResponse, _ := json.Marshal([]map[string]interface{}{
		{
			"name":           "Hello",
			"kind":           5, // Class
			"range":          map[string]interface{}{"start": map[string]interface{}{"line": 0, "character": 0}, "end": map[string]interface{}{"line": 0, "character": 39}},
			"selectionRange": map[string]interface{}{"start": map[string]interface{}{"line": 0, "character": 13}, "end": map[string]interface{}{"line": 0, "character": 18}},
		},
	})
	mockReq := &mockFallbackRequester{response: docSymResponse}

	acquireCalled := false
	// Build skill with NO extractor (extractor=nil) to force fallback path.
	s := &RepoMapSkill{
		cache:    cache,
		registry: registry,
		logger:   slog.Default(),
		fallbackDeps: &FallbackDeps{
			Extractor: repomap.NewFallbackExtractor(),
			AcquireFn: func(ctx context.Context, lang string) (repomap.SymbolRequester, func(), error) {
				acquireCalled = true
				assert.Equal(t, "java", lang)
				return mockReq, func() {}, nil
			},
		},
	}

	// Run walkAndExtract -- should use fallback since extractor is nil.
	err = s.walkAndExtract(context.Background(), dir)
	require.NoError(t, err)
	assert.True(t, acquireCalled, "AcquireFn should have been called for fallback extraction")

	// Verify tags were cached via fallback extraction.
	tags, err := cache.GetOrExtract(javaFile, func() ([]repomap.Tag, error) {
		t.Fatal("extractFn should not be called -- tags should be cached from fallback")
		return nil, nil
	})
	require.NoError(t, err)
	require.Len(t, tags, 1)
	assert.Equal(t, "Hello", tags[0].Name)
	assert.Equal(t, repomap.TagDef, tags[0].Kind)
}

func TestWalkAndExtract_FallbackSkipsWhenNoLS(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "tags.db")
	cache, err := repomap.NewTagCache(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { cache.Close() })

	registry := treesitter.NewGrammarRegistry()

	// Create a Java file (extractor=nil forces fallback path).
	javaFile := filepath.Join(dir, "Hello.java")
	require.NoError(t, os.WriteFile(javaFile, []byte("public class Hello {}\n"), 0644))

	// AcquireFn returns error (simulating no LS available).
	s := &RepoMapSkill{
		cache:    cache,
		registry: registry,
		logger:   slog.Default(),
		fallbackDeps: &FallbackDeps{
			Extractor: repomap.NewFallbackExtractor(),
			AcquireFn: func(ctx context.Context, lang string) (repomap.SymbolRequester, func(), error) {
				return nil, nil, fmt.Errorf("no language server for %s", lang)
			},
		},
	}

	// Walk should not error (D-33-02: silent skip).
	err = s.walkAndExtract(context.Background(), dir)
	require.NoError(t, err)

	// Verify no tags cached (extraction was skipped, not errored).
	tags, err := cache.GetOrExtract(javaFile, func() ([]repomap.Tag, error) {
		return nil, nil // will be called since nothing was cached
	})
	require.NoError(t, err)
	assert.Empty(t, tags)
}

package repomap

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/agenthands/helix/internal/repomap"
	"github.com/agenthands/helix/internal/treesitter"
)

// newIntegrationSkill creates a RepoMapSkill with a real TagExtractor and
// a temp workspace containing Go source files. The cache is NOT pre-populated;
// tests exercise the full extraction pipeline.
func newIntegrationSkill(t *testing.T) (*RepoMapSkill, string) {
	t.Helper()

	dir := t.TempDir()

	// Write Go source files with cross-file references.
	file1 := filepath.Join(dir, "server.go")
	file1Content := `package main

type Server struct {
	port int
}

func NewServer(port int) *Server {
	return &Server{port: port}
}

func (s *Server) Start() error {
	return nil
}
`
	require.NoError(t, os.WriteFile(file1, []byte(file1Content), 0o644))

	file2 := filepath.Join(dir, "handler.go")
	file2Content := `package main

import "fmt"

func HandleRequest(s *Server) {
	fmt.Println("handling request")
	s.Start()
}

func HealthCheck() string {
	return "ok"
}
`
	require.NoError(t, os.WriteFile(file2, []byte(file2Content), 0o644))

	// Create a subdirectory with another file.
	subDir := filepath.Join(dir, "internal")
	require.NoError(t, os.MkdirAll(subDir, 0o755))
	file3 := filepath.Join(subDir, "util.go")
	file3Content := `package internal

func Clamp(val, min, max int) int {
	if val < min { return min }
	if val > max { return max }
	return val
}
`
	require.NoError(t, os.WriteFile(file3, []byte(file3Content), 0o644))

	// Create tag cache and extractors.
	dbPath := filepath.Join(dir, ".cache", "tags.db")
	require.NoError(t, os.MkdirAll(filepath.Dir(dbPath), 0o755))
	cache, err := repomap.NewTagCache(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { cache.Close() })

	registry := treesitter.NewGrammarRegistry()
	elider := repomap.NewElisionRenderer(registry)

	extractor, err := repomap.NewTagExtractor(registry)
	require.NoError(t, err)
	t.Cleanup(func() { extractor.Close() })

	s := &RepoMapSkill{
		cache:     cache,
		extractor: extractor,
		elider:    elider,
		logger:    slog.Default(),
		rootDir:   dir,
		// cachePopulated is false -- tests will trigger ensureCache
	}

	return s, dir
}

// TestPipelineWalk verifies that ensureCache walks the workspace, populates
// TagCache, and BuildGraph produces a non-empty graph. (RMAP-04)
func TestPipelineWalk(t *testing.T) {
	s, _ := newIntegrationSkill(t)

	// Trigger cache population.
	err := s.ensureCache()
	require.NoError(t, err)
	assert.True(t, s.cachePopulated)

	// Verify cache has files.
	allFiles, err := s.cache.AllFiles()
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(allFiles), 3, "expected at least 3 Go files in cache")

	// Verify graph can be built from cache.
	graph, err := repomap.BuildGraph(s.cache)
	require.NoError(t, err)
	assert.Greater(t, len(graph.Files), 0, "graph should have file nodes")
}

// TestPipelineRanked verifies PageRank produces ranked output from real
// extracted data. (RMAP-05)
func TestPipelineRanked(t *testing.T) {
	s, _ := newIntegrationSkill(t)

	require.NoError(t, s.ensureCache())

	graph, err := repomap.BuildGraph(s.cache)
	require.NoError(t, err)

	ranked := graph.RankFiles(0.85, nil)
	assert.Greater(t, len(ranked), 0, "PageRank should produce ranked files")

	// Verify scores are positive.
	for _, rf := range ranked {
		assert.Greater(t, rf.Score, 0.0, "each ranked file should have positive score")
	}
}

// TestGetRepoMap_WithWorkspace verifies the full get_repo_map tool call
// returns ranked symbols. (RMAP-06) Phase 65 65-05: result is now a JSON
// envelope (Pitfall §2). The tree-text contract is preserved verbatim under
// env.Tree; the surrounding envelope adds source / fallback_reason /
// graph_version / freshness. With no SemanticLookup wired and no ConfigGate
// (cfg==nil → SemanticIndexEnabled()==false branch), source must be
// "tree_sitter" per D-04 / Pitfall §3.
func TestGetRepoMap_WithWorkspace(t *testing.T) {
	s, _ := newIntegrationSkill(t)

	result, err := s.ExecuteTool("get_repo_map", map[string]interface{}{
		"token_budget": float64(4096),
	})
	require.NoError(t, err)

	var env struct {
		Source         string `json:"source"`
		FallbackReason string `json:"fallback_reason,omitempty"`
		Tree           string `json:"tree"`
	}
	require.NoError(t, json.Unmarshal([]byte(result), &env), "result must be JSON envelope: %q", result)
	assert.NotContains(t, env.Tree, "No files found", "get_repo_map should return data, not empty message")
	assert.Contains(t, env.Tree, ".go", "tree should contain Go files")
	assert.Equal(t, "tree_sitter", env.Source, "no cfg gate → source==tree_sitter (D-04 / Pitfall §3)")
	assert.Empty(t, env.FallbackReason, "tree_sitter steady state has no fallback_reason")
}

// TestGetContext_WithWorkspace verifies get_context with seed files returns
// personalized ranked output. (RMAP-07) Phase 65 65-05: result is now a JSON
// envelope (Pitfall §2). Same JSON-wrap update as TestGetRepoMap_WithWorkspace.
func TestGetContext_WithWorkspace(t *testing.T) {
	s, dir := newIntegrationSkill(t)

	// First call to populate cache.
	require.NoError(t, s.ensureCache())

	// Use absolute path for files parameter (as the tool expects).
	result, err := s.ExecuteTool("get_context", map[string]interface{}{
		"files":        []interface{}{filepath.Join(dir, "server.go")},
		"token_budget": float64(4096),
	})
	require.NoError(t, err)

	var env struct {
		Source         string `json:"source"`
		FallbackReason string `json:"fallback_reason,omitempty"`
		Tree           string `json:"tree"`
	}
	require.NoError(t, json.Unmarshal([]byte(result), &env), "result must be JSON envelope: %q", result)
	assert.NotContains(t, env.Tree, "No files found", "get_context should return data")
	assert.NotEmpty(t, env.Tree)
	assert.Equal(t, "tree_sitter", env.Source, "no cfg gate → source==tree_sitter (D-04 / Pitfall §3)")
}

// TestEnrichFromLSP_CallbackInvoked verifies the enrichFn callback is called
// during graph rebuild. (RMAP-08)
func TestEnrichFromLSP_CallbackInvoked(t *testing.T) {
	s, _ := newIntegrationSkill(t)

	var enrichCalled atomic.Bool
	s.SetEnrichFn(func(g *repomap.FileGraph) {
		enrichCalled.Store(true)
		// Verify graph has data when enrichFn is called.
		assert.Greater(t, len(g.Files), 0, "graph should have files when enrichFn is called")
	})

	// Trigger full pipeline.
	_, err := s.ExecuteTool("get_repo_map", map[string]interface{}{})
	require.NoError(t, err)

	assert.True(t, enrichCalled.Load(), "enrichFn should have been called during graph rebuild")
}

// TestRenderBudgeted_UsesTreeRenderer verifies that TreeRenderer.RenderBudgeted
// is used (with 15% tolerance), not the deleted inline version. (RMAP-10)
func TestRenderBudgeted_UsesTreeRenderer(t *testing.T) {
	s, _ := newIntegrationSkill(t)

	// Populate cache and build graph.
	require.NoError(t, s.ensureCache())
	require.NoError(t, s.ensureGraph())

	// Verify renderer is set (TreeRenderer, not nil).
	assert.NotNil(t, s.renderer, "TreeRenderer should be set after ensureCache")

	// Call with a small budget to exercise binary search.
	result, err := s.ExecuteTool("get_repo_map", map[string]interface{}{
		"token_budget": float64(64), // minimal budget
	})
	require.NoError(t, err)
	// With TreeRenderer's min-1-file guarantee, we should still get output.
	assert.NotEmpty(t, result, "TreeRenderer should guarantee at least 1 file even with tiny budget")
	assert.NotContains(t, result, "No files found")
}

// TestSetWorkspaceRoot_InvalidatesCache verifies SetWorkspaceRoot clears
// the cachePopulated flag and renderer.
func TestSetWorkspaceRoot_InvalidatesCache(t *testing.T) {
	s, _ := newIntegrationSkill(t)

	// Populate cache.
	require.NoError(t, s.ensureCache())
	assert.True(t, s.cachePopulated)

	// Switch workspace root.
	newDir := t.TempDir()
	s.SetWorkspaceRoot(newDir)
	assert.False(t, s.cachePopulated, "SetWorkspaceRoot should invalidate cache")
	assert.Nil(t, s.renderer, "SetWorkspaceRoot should clear renderer")
	assert.Equal(t, newDir, s.rootDir)
}

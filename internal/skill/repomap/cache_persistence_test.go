package repomap

import (
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

// TestCachePersistence_DaemonRestart simulates a full daemon restart cycle:
//  1. Create a RepoMapSkill (daemon session 1), extract tags, call get_repo_map
//  2. Close the cache (daemon shutdown)
//  3. Create a NEW RepoMapSkill pointing at the same tags.db (daemon session 2)
//  4. Call get_repo_map again — must produce identical output from cached tags
//  5. Verify extractFn was NOT called for unchanged files (cache hit)
//
// This is the automated version of the Phase 27 "human_needed" verification item.
func TestCachePersistence_DaemonRestart(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, ".cache", "tags.db")
	require.NoError(t, os.MkdirAll(filepath.Dir(dbPath), 0o755))

	// Write source files.
	writeFixtureFiles(t, dir)

	// --- Session 1: first daemon lifecycle ---
	var extractCount1 atomic.Int64
	result1 := runSession(t, dir, dbPath, &extractCount1)

	assert.Greater(t, extractCount1.Load(), int64(0),
		"session 1 should extract tags (cold cache)")
	assert.NotEmpty(t, result1)
	assert.NotContains(t, result1, "No files found")

	// --- Session 2: simulated daemon restart ---
	var extractCount2 atomic.Int64
	result2 := runSession(t, dir, dbPath, &extractCount2)

	assert.Equal(t, int64(0), extractCount2.Load(),
		"session 2 should NOT re-extract tags (cache hit from SQLite)")
	assert.Equal(t, result1, result2,
		"get_repo_map output must be identical across restart")
}

// TestCachePersistence_ModifiedFileReextracts verifies that modifying a file
// between restarts triggers re-extraction for that file only.
func TestCachePersistence_ModifiedFileReextracts(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, ".cache", "tags.db")
	require.NoError(t, os.MkdirAll(filepath.Dir(dbPath), 0o755))

	writeFixtureFiles(t, dir)

	// Session 1: populate cache.
	var extractCount1 atomic.Int64
	runSession(t, dir, dbPath, &extractCount1)
	initialExtracts := extractCount1.Load()
	assert.Greater(t, initialExtracts, int64(0))

	// Modify one file between sessions.
	modifiedFile := filepath.Join(dir, "server.go")
	require.NoError(t, os.WriteFile(modifiedFile, []byte(`package main

type Server struct {
	port    int
	healthy bool
}

func NewServer(port int) *Server {
	return &Server{port: port, healthy: true}
}

func (s *Server) Start() error {
	return nil
}

func (s *Server) IsHealthy() bool {
	return s.healthy
}
`), 0o644))

	// Session 2: only modified file should re-extract.
	var extractCount2 atomic.Int64
	runSession(t, dir, dbPath, &extractCount2)
	assert.Equal(t, int64(1), extractCount2.Load(),
		"only the modified file should be re-extracted")
}

// runSession creates a RepoMapSkill with a real TagExtractor, populates the
// cache from the given directory using the given tags.db, calls get_repo_map,
// and returns the output. The extractCounter is incremented each time the
// extract function is actually invoked (cache miss).
func runSession(t *testing.T, workspaceDir, dbPath string, extractCounter *atomic.Int64) string {
	t.Helper()

	cache, err := repomap.NewTagCache(dbPath)
	require.NoError(t, err)
	defer cache.Close()

	registry := treesitter.NewGrammarRegistry()
	elider := repomap.NewElisionRenderer(registry)

	extractor, err := repomap.NewTagExtractor(registry)
	require.NoError(t, err)
	defer extractor.Close()

	// Wrap the extractor to count actual extraction calls.
	s := &RepoMapSkill{
		cache:     cache,
		extractor: extractor,
		elider:    elider,
		logger:    slog.Default(),
		rootDir:   workspaceDir,
		registry:  registry,
	}

	// Override walkAndExtract to count cache misses.
	// We do this by calling ensureCache which uses the real walkAndExtract,
	// but we instrument GetOrExtract calls via a counting wrapper.
	//
	// Since we can't easily intercept GetOrExtract, we use a different approach:
	// run the walk manually and count extractions.
	err = walkAndCount(s, workspaceDir, extractCounter)
	require.NoError(t, err)
	s.cachePopulated = true
	s.renderer = repomap.NewTreeRenderer(s.elider, s.cache, workspaceDir)

	// Build graph and call get_repo_map.
	result, err := s.ExecuteTool("get_repo_map", map[string]interface{}{
		"token_budget": float64(4096),
	})
	require.NoError(t, err)
	return result
}

// walkAndCount walks the workspace and populates the cache, counting actual
// extraction invocations (cache misses).
func walkAndCount(s *RepoMapSkill, root string, counter *atomic.Int64) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		lang := repomap.LangFromExt(path)
		if lang == "" {
			return nil
		}
		_, extractErr := s.cache.GetOrExtract(path, func() ([]repomap.Tag, error) {
			counter.Add(1)
			source, readErr := os.ReadFile(path)
			if readErr != nil {
				return nil, readErr
			}
			return s.extractor.Extract(source, path, lang)
		})
		if extractErr != nil {
			s.logger.Debug("extraction failed", "path", path, "error", extractErr)
		}
		return nil
	})
}

func writeFixtureFiles(t *testing.T, dir string) {
	t.Helper()

	require.NoError(t, os.WriteFile(filepath.Join(dir, "server.go"), []byte(`package main

type Server struct {
	port int
}

func NewServer(port int) *Server {
	return &Server{port: port}
}

func (s *Server) Start() error {
	return nil
}
`), 0o644))

	require.NoError(t, os.WriteFile(filepath.Join(dir, "handler.go"), []byte(`package main

import "fmt"

func HandleRequest(s *Server) {
	fmt.Println("handling request")
	s.Start()
}

func HealthCheck() string {
	return "ok"
}
`), 0o644))

	subDir := filepath.Join(dir, "internal")
	require.NoError(t, os.MkdirAll(subDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "util.go"), []byte(`package internal

func Clamp(val, min, max int) int {
	if val < min { return min }
	if val > max { return max }
	return val
}
`), 0o644))
}

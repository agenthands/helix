//go:build cgo

package repomap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/postfix/serena/internal/treesitter"
)

// newTestRenderer creates a TreeRenderer backed by a temp directory with sample Go files,
// a real TagCache populated with tags, and a real ElisionRenderer.
func newTestRenderer(t *testing.T) (*TreeRenderer, string) {
	t.Helper()
	rootDir := t.TempDir()

	// Create directory structure.
	require.NoError(t, os.MkdirAll(filepath.Join(rootDir, "pkg", "util"), 0o755))

	// Write sample Go files with known content.
	writeGoFile(t, rootDir, "main.go", `package main

func main() {
	fmt.Println("hello")
}
`)
	writeGoFile(t, filepath.Join(rootDir, "pkg"), "server.go", `package pkg

func StartServer(addr string) error {
	return nil
}

func StopServer() {
	// cleanup
}
`)
	writeGoFile(t, filepath.Join(rootDir, "pkg", "util"), "helper.go", `package util

func FormatName(first, last string) string {
	return first + " " + last
}
`)

	// Create cache and populate with tags for each file.
	cache := newTestCache(t)

	populateCache(t, cache, filepath.Join(rootDir, "main.go"), []Tag{
		{Name: "main", Kind: TagDef, File: filepath.Join(rootDir, "main.go"), Line: 2, Column: 5, StartByte: 14, EndByte: 48},
	})
	populateCache(t, cache, filepath.Join(rootDir, "pkg", "server.go"), []Tag{
		{Name: "StartServer", Kind: TagDef, File: filepath.Join(rootDir, "pkg", "server.go"), Line: 2, Column: 5, StartByte: 13, EndByte: 56},
		{Name: "StopServer", Kind: TagDef, File: filepath.Join(rootDir, "pkg", "server.go"), Line: 6, Column: 5, StartByte: 58, EndByte: 88},
	})
	populateCache(t, cache, filepath.Join(rootDir, "pkg", "util", "helper.go"), []Tag{
		{Name: "FormatName", Kind: TagDef, File: filepath.Join(rootDir, "pkg", "util", "helper.go"), Line: 2, Column: 5, StartByte: 15, EndByte: 68},
	})

	registry := treesitter.NewGrammarRegistry()
	elider := NewElisionRenderer(registry)
	renderer := NewTreeRenderer(elider, cache, rootDir)

	return renderer, rootDir
}

func writeGoFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func populateCache(t *testing.T, cache *TagCache, filePath string, tags []Tag) {
	t.Helper()
	_, err := cache.GetOrExtract(filePath, func() ([]Tag, error) {
		return tags, nil
	})
	require.NoError(t, err)
}

func TestEstimateTokens(t *testing.T) {
	assert.Equal(t, 1, EstimateTokens("1234"))
	assert.Equal(t, 0, EstimateTokens(""))
	assert.Equal(t, 0, EstimateTokens("ab"))
	assert.Equal(t, 2, EstimateTokens("12345678"))
	assert.Equal(t, 25, EstimateTokens(strings.Repeat("x", 100)))
}

func TestRenderBudgeted_AllFilesFit(t *testing.T) {
	renderer, rootDir := newTestRenderer(t)

	ranked := []RankedFile{
		{Path: filepath.Join(rootDir, "main.go"), Score: 0.5},
		{Path: filepath.Join(rootDir, "pkg", "server.go"), Score: 0.3},
		{Path: filepath.Join(rootDir, "pkg", "util", "helper.go"), Score: 0.2},
	}

	// Large budget should include all 3 files.
	output := renderer.RenderBudgeted(ranked, 10000)
	assert.Contains(t, output, "main.go")
	assert.Contains(t, output, "server.go")
	assert.Contains(t, output, "helper.go")
}

func TestRenderBudgeted_MinOneFile(t *testing.T) {
	renderer, rootDir := newTestRenderer(t)

	ranked := []RankedFile{
		{Path: filepath.Join(rootDir, "main.go"), Score: 0.5},
		{Path: filepath.Join(rootDir, "pkg", "server.go"), Score: 0.3},
		{Path: filepath.Join(rootDir, "pkg", "util", "helper.go"), Score: 0.2},
	}

	// Very small budget should still include at least 1 file.
	output := renderer.RenderBudgeted(ranked, 100)
	assert.NotEmpty(t, output)
	assert.Contains(t, output, "main.go")
}

func TestRenderBudgeted_BudgetFitting(t *testing.T) {
	renderer, rootDir := newTestRenderer(t)

	ranked := []RankedFile{
		{Path: filepath.Join(rootDir, "main.go"), Score: 0.5},
		{Path: filepath.Join(rootDir, "pkg", "server.go"), Score: 0.3},
		{Path: filepath.Join(rootDir, "pkg", "util", "helper.go"), Score: 0.2},
	}

	// Use a moderate budget and check that the output fits within 15% tolerance.
	budget := 500
	output := renderer.RenderBudgeted(ranked, budget)
	tokens := EstimateTokens(output)

	// Output should be within budget OR within 15% tolerance.
	if tokens > budget {
		pctErr := float64(tokens-budget) / float64(budget)
		assert.Less(t, pctErr, 0.15, "output tokens %d exceeded budget %d by more than 15%%", tokens, budget)
	}
}

func TestRenderBudgeted_EmptyRankedList(t *testing.T) {
	renderer, _ := newTestRenderer(t)

	output := renderer.RenderBudgeted(nil, 10000)
	assert.Empty(t, output)

	output = renderer.RenderBudgeted([]RankedFile{}, 10000)
	assert.Empty(t, output)
}

func TestRenderTree_TreeStructure(t *testing.T) {
	renderer, rootDir := newTestRenderer(t)

	ranked := []RankedFile{
		{Path: filepath.Join(rootDir, "pkg", "server.go"), Score: 0.5},
		{Path: filepath.Join(rootDir, "pkg", "util", "helper.go"), Score: 0.3},
	}

	output := renderer.renderTree(ranked)

	// Should contain tree-structured directory nesting.
	assert.Contains(t, output, "pkg")
	assert.Contains(t, output, "server.go")
	assert.Contains(t, output, "util")
	assert.Contains(t, output, "helper.go")
}

func TestRenderBudgeted_RankOrder(t *testing.T) {
	renderer, rootDir := newTestRenderer(t)

	ranked := []RankedFile{
		{Path: filepath.Join(rootDir, "main.go"), Score: 0.9},
		{Path: filepath.Join(rootDir, "pkg", "server.go"), Score: 0.5},
		{Path: filepath.Join(rootDir, "pkg", "util", "helper.go"), Score: 0.1},
	}

	// Large budget includes all files.
	output := renderer.RenderBudgeted(ranked, 10000)

	// All 3 files should be present since budget is large.
	assert.Contains(t, output, "main.go")
	assert.Contains(t, output, "server.go")
	assert.Contains(t, output, "helper.go")

	// With a tight budget, only the highest-ranked files should be included.
	// Binary search includes ranked[:N] -- so main.go (rank 0.9) is always first.
	tightOutput := renderer.RenderBudgeted(ranked, 1)
	assert.Contains(t, tightOutput, "main.go", "highest-ranked file should be included with tight budget")
}

func TestLangFromExt(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"foo.go", "go"},
		{"bar.py", "python"},
		{"baz.ts", "typescript"},
		{"qux.tsx", "tsx"},
		{"app.js", "javascript"},
		{"Main.java", "java"},
		{"lib.rs", "rust"},
		{"app.rb", "ruby"},
		{"index.php", "php"},
		{"hello.c", "c"},
		{"hello.cpp", "cpp"},
		{"script.r", "r"},
		{"Script.R", "r"},
		{"app.swift", "swift"},
		{"unknown.xyz", ""},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			assert.Equal(t, tt.want, LangFromExt(tt.path))
		})
	}
}

package repomap

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/postfix/serena/internal/treesitter"
)

// TestRepomap_PolyglotRanking reproduces BUG-01: on a synthetic polyglot
// workspace (Go pkg + deeply-nested Lua testdata fixture), RankFiles must
// return a Go file at top-1, not a Lua fixture file. Pre-fix this test
// FAILS (negative control, see RESEARCH.md line 208). Post-fix (F1-B in
// graph.go from Plan 02) it PASSES.
func TestRepomap_PolyglotRanking(t *testing.T) {
	cache, paths := newTestCacheWithTags(t, polyglotFixture())
	_ = paths

	g, err := BuildGraph(cache)
	require.NoError(t, err)

	ranked := g.RankFiles(0.85, nil)
	require.NotEmpty(t, ranked, "ranked output must not be empty")

	top := ranked[0].Path
	// Top-1 MUST be a Go file under pkg/.
	assert.Equal(t, ".go", filepath.Ext(top), "top-ranked file must be Go, got %s", top)
	assert.True(t,
		strings.Contains(top, string(filepath.Separator)+"pkg"+string(filepath.Separator)),
		"top-ranked file must live under pkg/, got %s", top)

	// Additional guard: at least one .go file must appear in top-3.
	topExts := []string{filepath.Ext(ranked[0].Path)}
	if len(ranked) > 1 {
		topExts = append(topExts, filepath.Ext(ranked[1].Path))
	}
	if len(ranked) > 2 {
		topExts = append(topExts, filepath.Ext(ranked[2].Path))
	}
	assert.Contains(t, topExts, ".go", "top-3 must contain at least one .go file")
}

// TestRepomap_PolyglotRender asserts RenderBudgeted output contains a Go
// symbol and is not 100% Lua content. Guards the render amplifier in
// render.go (RESEARCH §Candidate 4).
func TestRepomap_PolyglotRender(t *testing.T) {
	cache, paths := newTestCacheWithTags(t, polyglotFixture())
	g, err := BuildGraph(cache)
	require.NoError(t, err)

	ranked := g.RankFiles(0.85, nil)
	require.NotEmpty(t, ranked)

	// Derive rootDir by stripping the known relative suffix from any path.
	rootDir := rootDirFromPath(t, paths["pkg/server.go"], filepath.Join("pkg", "server.go"))

	registry := treesitter.NewGrammarRegistry()
	elider := NewElisionRenderer(registry)
	renderer := NewTreeRenderer(elider, cache, rootDir)

	out := renderer.RenderBudgeted(ranked, 2048)
	require.NotEmpty(t, out)

	// Must contain at least one Go symbol from the pkg/ fixture. Post-fix, the
	// rendered tree should include Go filenames from pkg/ (server.go, store.go,
	// handler.go, logger.go) which carry the lowercase symbol substrings.
	containsGoSym := strings.Contains(out, "server") ||
		strings.Contains(out, "store") ||
		strings.Contains(out, "handler") ||
		strings.Contains(out, "logger")
	assert.True(t, containsGoSym,
		"rendered output must contain a Go file/symbol from pkg/, got:\n%s", out)

	// Must reference at least one .go file (pre-fix the renderer is 100% Lua).
	assert.Contains(t, out, ".go",
		"rendered output must reference at least one .go file, got:\n%s", out)
}

// rootDirFromPath strips a known relative suffix from an absolute path to
// recover the root directory used by newTestCacheWithTags.
func rootDirFromPath(t *testing.T, absPath, relSuffix string) string {
	t.Helper()
	require.True(t,
		strings.HasSuffix(absPath, string(filepath.Separator)+relSuffix),
		"abs path %q does not end with %q", absPath, relSuffix)
	return absPath[:len(absPath)-len(relSuffix)-1]
}

// polyglotFixture returns the map[file]->[]Tag that reliably reproduces
// the BUG-01 rank-dominance symptom. See RESEARCH.md §Synthetic Fixture
// Design (lines 175-208) for the rationale.
//
// Key reproduction mechanic: Go defs are QUALIFIED (via buildQualifiedName,
// e.g. "Store.Add") but Go refs are stored BARE (buildQualifiedName runs only
// on defs — see extractor.go:235-261). Thus when Go code does `s.add(...)`,
// the ref lands as bare "add" and collides with Lua's `add` def, pumping
// rank across the language boundary into the closed Lua sink subgraph.
func polyglotFixture() map[string][]Tag {
	return map[string][]Tag{
		// --- Go package pkg/ with qualified defs (realistic after
		// buildQualifiedName) and bare refs (realistic for Go call sites).
		// Go defs being qualified means they do NOT collide with Go refs,
		// so Go refs leak out to any other file (Lua) that defines the bare name.
		"pkg/server.go": {
			{Name: "pkg.Server", Kind: TagDef, Line: 10, Column: 6},
			{Name: "Server.Start", Kind: TagDef, Line: 20, Column: 18},
			{Name: "Server.Stop", Kind: TagDef, Line: 30, Column: 18},
			// Bare refs (common method names) — collide with Lua defs.
			{Name: "log", Kind: TagRef, Line: 22, Column: 5},
			{Name: "add", Kind: TagRef, Line: 23, Column: 5},
			{Name: "new", Kind: TagRef, Line: 24, Column: 5},
			{Name: "trim", Kind: TagRef, Line: 25, Column: 5},
			{Name: "split", Kind: TagRef, Line: 26, Column: 5},
		},
		"pkg/store.go": {
			{Name: "pkg.Store", Kind: TagDef, Line: 10, Column: 6},
			{Name: "Store.Add", Kind: TagDef, Line: 20, Column: 18},
			{Name: "Store.Get", Kind: TagDef, Line: 30, Column: 18},
			{Name: "Store.Delete", Kind: TagDef, Line: 40, Column: 18},
			{Name: "log", Kind: TagRef, Line: 22, Column: 5},
			{Name: "add", Kind: TagRef, Line: 23, Column: 5},
			{Name: "new", Kind: TagRef, Line: 24, Column: 5},
		},
		"pkg/logger.go": {
			{Name: "pkg.Logger", Kind: TagDef, Line: 10, Column: 6},
			{Name: "Logger.Log", Kind: TagDef, Line: 20, Column: 18},
			{Name: "Logger.Info", Kind: TagDef, Line: 30, Column: 18},
			{Name: "Logger.Debug", Kind: TagDef, Line: 40, Column: 18},
			{Name: "Logger.Warn", Kind: TagDef, Line: 50, Column: 18},
			{Name: "Logger.Error", Kind: TagDef, Line: 60, Column: 18},
			// bare refs again — Go code calls helpers with bare identifiers.
			{Name: "trim", Kind: TagRef, Line: 22, Column: 5},
			{Name: "split", Kind: TagRef, Line: 23, Column: 5},
		},
		"pkg/handler.go": {
			{Name: "pkg.Handler", Kind: TagDef, Line: 10, Column: 6},
			{Name: "Handler.Handle", Kind: TagDef, Line: 20, Column: 18},
			// bare refs dominate — method calls in Go
			{Name: "add", Kind: TagRef, Line: 21, Column: 5},
			{Name: "log", Kind: TagRef, Line: 22, Column: 5},
			{Name: "new", Kind: TagRef, Line: 23, Column: 5},
			{Name: "subtract", Kind: TagRef, Line: 24, Column: 5},
			{Name: "multiply", Kind: TagRef, Line: 25, Column: 5},
		},
		"pkg/util.go": {
			{Name: "pkg.Trim", Kind: TagDef, Line: 10, Column: 6},
			{Name: "pkg.Split", Kind: TagDef, Line: 20, Column: 6},
			{Name: "log", Kind: TagRef, Line: 12, Column: 5},
			{Name: "add", Kind: TagRef, Line: 13, Column: 5},
			{Name: "new", Kind: TagRef, Line: 14, Column: 5},
		},

		// --- Deeply-nested Lua testdata fixture with bare-name collisions ---
		"testdata/fixtures/lua/deep/nested/fixture/main.lua": {
			{Name: "main", Kind: TagDef, Line: 1, Column: 1},
			// refs into calculator.lua / utils.lua — bare names collide with Go defs
			{Name: "add", Kind: TagRef, Line: 10, Column: 1},
			{Name: "subtract", Kind: TagRef, Line: 11, Column: 1},
			{Name: "multiply", Kind: TagRef, Line: 12, Column: 1},
			{Name: "log", Kind: TagRef, Line: 13, Column: 1},
			{Name: "trim", Kind: TagRef, Line: 14, Column: 1},
			{Name: "Logger", Kind: TagRef, Line: 15, Column: 1},
			{Name: "new", Kind: TagRef, Line: 16, Column: 1},
		},
		"testdata/fixtures/lua/deep/nested/fixture/calculator.lua": {
			// bare names from `function calculator.add(...)` etc.
			{Name: "add", Kind: TagDef, Line: 1, Column: 1},
			{Name: "subtract", Kind: TagDef, Line: 10, Column: 1},
			{Name: "multiply", Kind: TagDef, Line: 20, Column: 1},
			{Name: "divide", Kind: TagDef, Line: 30, Column: 1},
			{Name: "power", Kind: TagDef, Line: 40, Column: 1},
		},
		"testdata/fixtures/lua/deep/nested/fixture/utils.lua": {
			{Name: "log", Kind: TagDef, Line: 1, Column: 1},
			{Name: "trim", Kind: TagDef, Line: 10, Column: 1},
			{Name: "split", Kind: TagDef, Line: 20, Column: 1},
			{Name: "Logger", Kind: TagDef, Line: 30, Column: 1},
			{Name: "new", Kind: TagDef, Line: 40, Column: 1},
			{Name: "Info", Kind: TagDef, Line: 50, Column: 1},
			{Name: "Debug", Kind: TagDef, Line: 60, Column: 1},
			{Name: "Warn", Kind: TagDef, Line: 70, Column: 1},
			{Name: "Error", Kind: TagDef, Line: 80, Column: 1},
		},
	}
}

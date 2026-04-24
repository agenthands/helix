package repomap

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestCacheWithTags creates a TagCache populated with the given file->tags mapping.
// Each file gets a real temp file on disk (required for mtime checks in GetOrExtract).
func newTestCacheWithTags(t *testing.T, files map[string][]Tag) (*TagCache, map[string]string) {
	t.Helper()
	cache := newTestCache(t)
	dir := t.TempDir()

	// Map from logical name to real temp file path.
	paths := make(map[string]string, len(files))

	for name, tags := range files {
		// Create a real temp file so os.Stat works in GetOrExtract.
		fp := filepath.Join(dir, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(fp), 0o755))
		require.NoError(t, os.WriteFile(fp, []byte("// "+name), 0o644))
		paths[name] = fp

		// Rewrite tags with the real file path.
		realTags := make([]Tag, len(tags))
		for i, tag := range tags {
			realTags[i] = tag
			realTags[i].File = fp
		}

		// Populate cache via GetOrExtract.
		tagsCopy := realTags
		_, err := cache.GetOrExtract(fp, func() ([]Tag, error) {
			return tagsCopy, nil
		})
		require.NoError(t, err)
	}

	return cache, paths
}

func TestBuildGraph_CrossFileEdges(t *testing.T) {
	cache, paths := newTestCacheWithTags(t, map[string][]Tag{
		"fileA.go": {
			{Name: "Foo", Kind: TagRef, Line: 5, Column: 1},
		},
		"fileB.go": {
			{Name: "Foo", Kind: TagDef, Line: 1, Column: 5},
		},
	})

	g, err := BuildGraph(cache)
	require.NoError(t, err)

	// Edge from fileA (ref) -> fileB (def). F1-B: weight = sqrt(1) * 1/sqrt(1+1) = sqrt(1)/sqrt(2) ≈ 0.7071.
	require.Contains(t, g.Edges, paths["fileA.go"])
	assert.InDelta(t, math.Sqrt(1)*1.0/math.Sqrt(2), g.Edges[paths["fileA.go"]][paths["fileB.go"]], 0.001)
}

func TestBuildGraph_WeightByRefCount(t *testing.T) {
	cache, paths := newTestCacheWithTags(t, map[string][]Tag{
		"fileA.go": {
			{Name: "Bar", Kind: TagRef, Line: 1, Column: 1},
			{Name: "Bar", Kind: TagRef, Line: 5, Column: 1},
			{Name: "Bar", Kind: TagRef, Line: 10, Column: 1},
			{Name: "Bar", Kind: TagRef, Line: 15, Column: 1},
		},
		"fileC.go": {
			{Name: "Bar", Kind: TagDef, Line: 1, Column: 5},
		},
	})

	g, err := BuildGraph(cache)
	require.NoError(t, err)

	// 4 refs, defDegree("Bar")=1 -> weight = sqrt(4) * 1/sqrt(2) ≈ 1.4142.
	require.Contains(t, g.Edges, paths["fileA.go"])
	assert.InDelta(t, 2.0*1.0/math.Sqrt(2), g.Edges[paths["fileA.go"]][paths["fileC.go"]], 0.001)
}

func TestBuildGraph_IsolatedDefinition(t *testing.T) {
	cache, paths := newTestCacheWithTags(t, map[string][]Tag{
		"fileD.go": {
			{Name: "Baz", Kind: TagDef, Line: 1, Column: 5},
		},
	})

	g, err := BuildGraph(cache)
	require.NoError(t, err)

	// Self-loop: fileD -> fileD with weight 0.1.
	require.Contains(t, g.Edges, paths["fileD.go"])
	assert.InDelta(t, 0.1, g.Edges[paths["fileD.go"]][paths["fileD.go"]], 0.001)
}

func TestBuildGraph_SkipsSameFileRefs(t *testing.T) {
	cache, paths := newTestCacheWithTags(t, map[string][]Tag{
		"fileE.go": {
			{Name: "Qux", Kind: TagDef, Line: 1, Column: 5},
			{Name: "Qux", Kind: TagRef, Line: 10, Column: 1},
		},
	})

	g, err := BuildGraph(cache)
	require.NoError(t, err)

	// There should be no cross-file edge from E to E via ref matching.
	// Only the self-loop (if any) from isolated def logic should NOT apply
	// since there IS a ref for "Qux" (just same-file).
	// The graph may or may not have E depending on implementation.
	// Key assertion: no edge from E->E created by the ref-matching logic.
	if targets, ok := g.Edges[paths["fileE.go"]]; ok {
		// If there's a self-edge, it should NOT have weight sqrt(1)=1.0
		// (which would indicate the same-file ref was counted).
		if w, hasEdge := targets[paths["fileE.go"]]; hasEdge {
			assert.Less(t, w, 0.5, "same-file ref should not create a weighted cross-file edge")
		}
	}
}

func TestBuildGraph_EmptyCache(t *testing.T) {
	cache := newTestCache(t)

	g, err := BuildGraph(cache)
	require.NoError(t, err)
	assert.Equal(t, 0, g.NodeCount())
	assert.Equal(t, 0, g.EdgeCount())
}

func TestEnrichFromLSP_AddsCrossFileEdge(t *testing.T) {
	g := NewFileGraph()
	g.Files["source.go"] = true

	refs := []Location{
		{URI: "file:///other.go", Line: 10},
	}
	g.EnrichFromLSP("source.go", refs)

	// Edge from /other.go -> source.go with weight 1.0.
	require.Contains(t, g.Edges, "/other.go")
	assert.InDelta(t, 1.0, g.Edges["/other.go"]["source.go"], 0.001)
}

func TestEnrichFromLSP_SkipsSameFile(t *testing.T) {
	g := NewFileGraph()
	g.Files["source.go"] = true

	refs := []Location{
		{URI: "file://source.go", Line: 5},
	}
	g.EnrichFromLSP("source.go", refs)

	// No edge should be created for same-file reference.
	assert.Equal(t, 0, g.EdgeCount(), "same-file LSP ref should not create an edge")
}

func TestTagCache_AllFiles(t *testing.T) {
	cache, paths := newTestCacheWithTags(t, map[string][]Tag{
		"a.go": {
			{Name: "A", Kind: TagDef, Line: 1, Column: 5},
		},
		"b.go": {
			{Name: "B", Kind: TagRef, Line: 2, Column: 1},
		},
	})

	allFiles, err := cache.AllFiles()
	require.NoError(t, err)
	assert.Len(t, allFiles, 2)

	// Both files should be present with their tags.
	require.Contains(t, allFiles, paths["a.go"])
	require.Contains(t, allFiles, paths["b.go"])
	assert.Equal(t, "A", allFiles[paths["a.go"]][0].Name)
	assert.Equal(t, "B", allFiles[paths["b.go"]][0].Name)
}

func TestTagCache_Version(t *testing.T) {
	cache := newTestCache(t)
	dir := t.TempDir()

	v0 := cache.Version()

	// Version increments after GetOrExtract stores tags.
	fp := writeTempFile(t, dir, "a.go", "package a\n")
	_, err := cache.GetOrExtract(fp, func() ([]Tag, error) {
		return []Tag{{Name: "A", Kind: TagDef, File: fp, Line: 1}}, nil
	})
	require.NoError(t, err)
	v1 := cache.Version()
	assert.Greater(t, v1, v0, "version should increment after store")

	// Version increments after InvalidateFile.
	require.NoError(t, cache.InvalidateFile(fp))
	v2 := cache.Version()
	assert.Greater(t, v2, v1, "version should increment after invalidate")

	// Version increments after Clear.
	require.NoError(t, cache.Clear())
	v3 := cache.Version()
	assert.Greater(t, v3, v2, "version should increment after clear")
}

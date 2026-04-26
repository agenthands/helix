package repomap

import (
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestCache(t *testing.T) *TagCache {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "tags.db")
	cache, err := NewTagCache(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { cache.Close() })
	return cache
}

func writeTempFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

func TestTagCache_NewAndClose(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "tags.db")

	cache, err := NewTagCache(dbPath)
	require.NoError(t, err)

	// Verify db file exists on disk.
	_, err = os.Stat(dbPath)
	require.NoError(t, err, "tags.db should exist on disk after creation")

	require.NoError(t, cache.Close())
}

func TestTagCache_GetOrExtract_CacheMiss(t *testing.T) {
	cache := newTestCache(t)
	dir := t.TempDir()
	filePath := writeTempFile(t, dir, "hello.go", "package main\n\nfunc Hello() {}\n")

	var calls atomic.Int32
	extractFn := func() ([]Tag, error) {
		calls.Add(1)
		return []Tag{
			{Name: "Hello", Kind: TagDef, File: filePath, Line: 2, Column: 5, StartByte: 20, EndByte: 50},
			{Name: "fmt.Println", Kind: TagRef, File: filePath, Line: 3, Column: 1, StartByte: 55, EndByte: 70},
			{Name: "os.Exit", Kind: TagRef, File: filePath, Line: 4, Column: 1, StartByte: 75, EndByte: 85},
		}, nil
	}

	tags, err := cache.GetOrExtract(filePath, extractFn)
	require.NoError(t, err)
	assert.Len(t, tags, 3)
	assert.Equal(t, "Hello", tags[0].Name)
	assert.Equal(t, TagDef, tags[0].Kind)
	assert.Equal(t, "fmt.Println", tags[1].Name)
	assert.Equal(t, TagRef, tags[1].Kind)
	assert.Equal(t, "os.Exit", tags[2].Name)
	assert.Equal(t, TagRef, tags[2].Kind)
	assert.Equal(t, int32(1), calls.Load(), "extractFn should be called exactly once")
}

func TestTagCache_GetOrExtract_CacheHit(t *testing.T) {
	cache := newTestCache(t)
	dir := t.TempDir()
	filePath := writeTempFile(t, dir, "hello.go", "package main\n\nfunc Hello() {}\n")

	var calls atomic.Int32
	extractFn := func() ([]Tag, error) {
		calls.Add(1)
		return []Tag{
			{Name: "Hello", Kind: TagDef, File: filePath, Line: 2, Column: 5, StartByte: 20, EndByte: 50},
			{Name: "fmt.Println", Kind: TagRef, File: filePath, Line: 3, Column: 1, StartByte: 55, EndByte: 70},
			{Name: "os.Exit", Kind: TagRef, File: filePath, Line: 4, Column: 1, StartByte: 75, EndByte: 85},
		}, nil
	}

	// First call: cache miss.
	tags1, err := cache.GetOrExtract(filePath, extractFn)
	require.NoError(t, err)
	assert.Len(t, tags1, 3)

	// Second call: cache hit, extractFn should NOT be called again.
	tags2, err := cache.GetOrExtract(filePath, extractFn)
	require.NoError(t, err)
	assert.Len(t, tags2, 3)
	assert.Equal(t, tags1[0].Name, tags2[0].Name)
	assert.Equal(t, tags1[1].Name, tags2[1].Name)
	assert.Equal(t, tags1[2].Name, tags2[2].Name)
	assert.Equal(t, int32(1), calls.Load(), "extractFn should be called only once (cache hit)")
}

func TestTagCache_GetOrExtract_MtimeInvalidation(t *testing.T) {
	cache := newTestCache(t)
	dir := t.TempDir()
	filePath := writeTempFile(t, dir, "hello.go", "package main\n\nfunc funcA() {}\n")

	var calls atomic.Int32

	// First extraction: returns tags-v1.
	tags1, err := cache.GetOrExtract(filePath, func() ([]Tag, error) {
		calls.Add(1)
		return []Tag{
			{Name: "funcA", Kind: TagDef, File: filePath, Line: 2, Column: 5, StartByte: 20, EndByte: 50},
		}, nil
	})
	require.NoError(t, err)
	assert.Len(t, tags1, 1)
	assert.Equal(t, "funcA", tags1[0].Name)

	// Touch file to change mtime.
	require.NoError(t, os.WriteFile(filePath, []byte("package main\n\nfunc funcB() {}\n"), 0o644))

	// Second extraction: mtime changed, should call extractFn again.
	tags2, err := cache.GetOrExtract(filePath, func() ([]Tag, error) {
		calls.Add(1)
		return []Tag{
			{Name: "funcB", Kind: TagDef, File: filePath, Line: 2, Column: 5, StartByte: 20, EndByte: 50},
		}, nil
	})
	require.NoError(t, err)
	assert.Len(t, tags2, 1)
	assert.Equal(t, "funcB", tags2[0].Name, "should return fresh extraction after mtime change")
	assert.Equal(t, int32(2), calls.Load(), "extractFn should be called twice (mtime invalidation)")
}

func TestTagCache_InvalidateFile(t *testing.T) {
	cache := newTestCache(t)
	dir := t.TempDir()
	filePath := writeTempFile(t, dir, "hello.go", "package main\n\nfunc Hello() {}\n")

	var calls atomic.Int32
	extractFn := func() ([]Tag, error) {
		calls.Add(1)
		return []Tag{
			{Name: "Hello", Kind: TagDef, File: filePath, Line: 2, Column: 5, StartByte: 20, EndByte: 50},
		}, nil
	}

	// Populate cache.
	_, err := cache.GetOrExtract(filePath, extractFn)
	require.NoError(t, err)
	assert.Equal(t, int32(1), calls.Load())

	// Invalidate.
	require.NoError(t, cache.InvalidateFile(filePath))

	// Next GetOrExtract should call extractFn again.
	_, err = cache.GetOrExtract(filePath, extractFn)
	require.NoError(t, err)
	assert.Equal(t, int32(2), calls.Load(), "extractFn should be called again after invalidation")
}

func TestTagCache_Clear(t *testing.T) {
	cache := newTestCache(t)
	dir := t.TempDir()

	file1 := writeTempFile(t, dir, "a.go", "package a\n\nfunc A() {}\n")
	file2 := writeTempFile(t, dir, "b.go", "package b\n\nfunc B() {}\n")

	var calls1, calls2 atomic.Int32

	// Populate both files.
	_, err := cache.GetOrExtract(file1, func() ([]Tag, error) {
		calls1.Add(1)
		return []Tag{{Name: "A", Kind: TagDef, File: file1, Line: 2, Column: 5, StartByte: 10, EndByte: 20}}, nil
	})
	require.NoError(t, err)

	_, err = cache.GetOrExtract(file2, func() ([]Tag, error) {
		calls2.Add(1)
		return []Tag{{Name: "B", Kind: TagDef, File: file2, Line: 2, Column: 5, StartByte: 10, EndByte: 20}}, nil
	})
	require.NoError(t, err)

	assert.Equal(t, int32(1), calls1.Load())
	assert.Equal(t, int32(1), calls2.Load())

	// Clear all.
	require.NoError(t, cache.Clear())

	// Both files should require re-extraction.
	_, err = cache.GetOrExtract(file1, func() ([]Tag, error) {
		calls1.Add(1)
		return []Tag{{Name: "A", Kind: TagDef, File: file1, Line: 2, Column: 5, StartByte: 10, EndByte: 20}}, nil
	})
	require.NoError(t, err)

	_, err = cache.GetOrExtract(file2, func() ([]Tag, error) {
		calls2.Add(1)
		return []Tag{{Name: "B", Kind: TagDef, File: file2, Line: 2, Column: 5, StartByte: 10, EndByte: 20}}, nil
	})
	require.NoError(t, err)

	assert.Equal(t, int32(2), calls1.Load(), "file1 should be re-extracted after Clear")
	assert.Equal(t, int32(2), calls2.Load(), "file2 should be re-extracted after Clear")
}

// fakeRepoMapSink is a recording MetricsSink for emission assertions.
type fakeRepoMapSink struct {
	hits     int
	misses   int
	observed []float64
	langs    []string
}

func (f *fakeRepoMapSink) RepoMapCacheInc(lang, result string) {
	f.langs = append(f.langs, lang)
	if result == ResultHit {
		f.hits++
		return
	}
	f.misses++
}

func (f *fakeRepoMapSink) RepoMapExtractObserve(_ string, s float64) {
	f.observed = append(f.observed, s)
}

func TestTagCache_MetricsEmission(t *testing.T) {
	cache := newTestCache(t)
	sink := &fakeRepoMapSink{}
	cache.SetMetrics(sink)

	dir := t.TempDir()
	filePath := writeTempFile(t, dir, "hello.go", "package main\n\nfunc Hello() {}\n")

	extractFn := func() ([]Tag, error) {
		return []Tag{
			{Name: "Hello", Kind: TagDef, File: filePath, Line: 2, Column: 5, StartByte: 20, EndByte: 50},
		}, nil
	}

	// Cold path: miss + extract observation.
	_, err := cache.GetOrExtract(filePath, extractFn)
	require.NoError(t, err)

	// Warm path: hit, no extract observation.
	_, err = cache.GetOrExtract(filePath, extractFn)
	require.NoError(t, err)

	assert.Equal(t, 1, sink.hits, "expected exactly 1 hit on the warm path")
	assert.Equal(t, 1, sink.misses, "expected exactly 1 miss on the cold path")
	assert.Len(t, sink.observed, 1, "expected exactly 1 extract-duration observation")
	// All emissions should carry the resolved language label ("go").
	for _, l := range sink.langs {
		assert.Equal(t, "go", l)
	}
}

// TestTagCache_MetricsEmission_MissOnExtractError verifies that extractFn
// failures still emit miss + extract-duration so miss-rate dashboards
// include failed extractions.
func TestTagCache_MetricsEmission_MissOnExtractError(t *testing.T) {
	cache := newTestCache(t)
	sink := &fakeRepoMapSink{}
	cache.SetMetrics(sink)

	dir := t.TempDir()
	filePath := writeTempFile(t, dir, "broken.go", "package main\n")

	extractFn := func() ([]Tag, error) {
		return nil, fmt.Errorf("simulated extractor failure")
	}

	_, err := cache.GetOrExtract(filePath, extractFn)
	assert.Error(t, err)
	assert.Equal(t, 0, sink.hits)
	assert.Equal(t, 1, sink.misses)
	assert.Len(t, sink.observed, 1)
}

// TestTagCache_MetricsEmission_MissOnStatError verifies that os.Stat
// failures (e.g. file deleted between walk and extract, permission
// denied) still emit a miss + extract-duration observation. Phase 53
// WR-04: stat failures are silently invisible without this — operators
// watching serena_repomap_cache_total need to see this class of failure.
func TestTagCache_MetricsEmission_MissOnStatError(t *testing.T) {
	cache := newTestCache(t)
	sink := &fakeRepoMapSink{}
	cache.SetMetrics(sink)

	dir := t.TempDir()
	// File path that does NOT exist on disk — os.Stat will fail.
	missingPath := filepath.Join(dir, "nonexistent.go")

	extractFn := func() ([]Tag, error) {
		t.Fatal("extractFn must NOT be invoked when stat fails")
		return nil, nil
	}

	_, err := cache.GetOrExtract(missingPath, extractFn)
	assert.Error(t, err, "GetOrExtract must surface stat error")
	assert.Equal(t, 0, sink.hits)
	assert.Equal(t, 1, sink.misses, "stat failure must emit cache miss")
	assert.Len(t, sink.observed, 1, "stat failure must record extract observation")
	for _, l := range sink.langs {
		assert.Equal(t, "go", l, "language label must resolve from extension before stat")
	}
}

func TestTagCache_Persistence(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "tags.db")
	filePath := writeTempFile(t, dir, "hello.go", "package main\n\nfunc Hello() {}\n")

	var calls atomic.Int32
	extractFn := func() ([]Tag, error) {
		calls.Add(1)
		return []Tag{
			{Name: "Hello", Kind: TagDef, File: filePath, Line: 2, Column: 5, StartByte: 20, EndByte: 50},
			{Name: "Run", Kind: TagRef, File: filePath, Line: 3, Column: 1, StartByte: 55, EndByte: 65},
		}, nil
	}

	// Create first cache, store tags, close it.
	cache1, err := NewTagCache(dbPath)
	require.NoError(t, err)

	tags1, err := cache1.GetOrExtract(filePath, extractFn)
	require.NoError(t, err)
	assert.Len(t, tags1, 2)
	require.NoError(t, cache1.Close())

	// Create a NEW cache at the same dbPath (simulates daemon restart).
	cache2, err := NewTagCache(dbPath)
	require.NoError(t, err)
	defer cache2.Close()

	// extractFn should NOT be called (tags survived restart).
	tags2, err := cache2.GetOrExtract(filePath, extractFn)
	require.NoError(t, err)
	assert.Len(t, tags2, 2)
	assert.Equal(t, "Hello", tags2[0].Name)
	assert.Equal(t, TagDef, tags2[0].Kind)
	assert.Equal(t, "Run", tags2[1].Name)
	assert.Equal(t, TagRef, tags2[1].Kind)
	assert.Equal(t, int32(1), calls.Load(), "extractFn should be called only once (tags persisted across restart)")
}

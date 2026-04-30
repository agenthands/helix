package repomap

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMetricsSink_NoopSinkSatisfiesInterface(t *testing.T) {
	var _ MetricsSink = NoopSink{}
}

func TestNoopSink_safe(t *testing.T) {
	// All methods must be callable without panic on the zero value, even
	// for unknown label values (the sink layer never validates — that
	// discipline lives in the obs.Metrics helper drop-unknown guards).
	var sink MetricsSink = NoopSink{}
	sink.RepoMapLookup("go", LookupHit)
	sink.RepoMapLookup("go", LookupMiss)
	sink.RepoMapLookup("go", "garbage")
	sink.RepoMapExtractObserve("go", ExtractorTreesitter, 0.001)
	sink.RepoMapExtractObserve("go", ExtractorLSP, 0.05)
	sink.RepoMapExtractObserve("go", ExtractorFallback, 0.1)
	sink.RepoMapExtractObserve("go", "garbage", 0.2)
}

// TestMetricsSink_LookupResultConstants pins the closed-enum result label
// values for helix_repomap_lookups_total. Phase 53 D-04.
func TestMetricsSink_LookupResultConstants(t *testing.T) {
	assert.Equal(t, "hit", LookupHit)
	assert.Equal(t, "miss", LookupMiss)
}

// TestMetricsSink_ExtractorConstants pins the closed-enum extractor label
// values for helix_repomap_extract_duration_seconds. Phase 53 D-07.
func TestMetricsSink_ExtractorConstants(t *testing.T) {
	assert.Equal(t, "treesitter", ExtractorTreesitter)
	assert.Equal(t, "lsp", ExtractorLSP)
	assert.Equal(t, "fallback", ExtractorFallback)
}

// recordingSink is a thread-safe MetricsSink test double used by Task 3.2
// (cache hit/miss emission) and Task 3.3 (per-extractor latency observation).
// Mirrors the lspool recordingSink shape exactly: mutex-guarded append-only
// event slices, snapshot() returning copies.
type lookupEvent struct {
	lang   string
	result string
}

type extractEvent struct {
	lang      string
	extractor string
	seconds   float64
}

type recordingSink struct {
	mu       sync.Mutex
	lookups  []lookupEvent
	extracts []extractEvent
}

func (r *recordingSink) RepoMapLookup(language, result string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lookups = append(r.lookups, lookupEvent{lang: language, result: result})
}

func (r *recordingSink) RepoMapExtractObserve(language, extractor string, seconds float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.extracts = append(r.extracts, extractEvent{lang: language, extractor: extractor, seconds: seconds})
}

// snapshot returns consistent copies of the recorded events.
func (r *recordingSink) snapshot() ([]lookupEvent, []extractEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	lk := make([]lookupEvent, len(r.lookups))
	copy(lk, r.lookups)
	ex := make([]extractEvent, len(r.extracts))
	copy(ex, r.extracts)
	return lk, ex
}

// TestTagCache_GetOrExtract_LookupEmission pins the hit/miss emission contract
// per Phase 53 D-03:
//   - mtime-match branch (cache.go:101-109)            → LookupHit, no extract observation.
//   - extractFn-invocation branch (cache.go:111-122)   → LookupMiss, extract observation
//     happens upstream in skill.go (Q-2 Option 2), NOT here.
func TestTagCache_GetOrExtract_LookupEmission(t *testing.T) {
	t.Run("hit_branch_emits_hit", func(t *testing.T) {
		sink := &recordingSink{}
		cache := newTestCache(t)
		cache.SetMetricsSink(sink)

		dir := t.TempDir()
		filePath := writeTempFile(t, dir, "hello.go", "package main\n\nfunc Hello() {}\n")

		// Prime the cache via a real GetOrExtract call. The first call
		// emits LookupMiss (priming); we then reset the sink and assert
		// the second call (mtime-match) emits exactly one LookupHit.
		_, err := cache.GetOrExtract(filePath, func() ([]Tag, error) {
			return []Tag{
				{Name: "Hello", Kind: TagDef, File: filePath, Line: 2, Column: 5, StartByte: 20, EndByte: 50},
			}, nil
		})
		assert.NoError(t, err)

		// Reset the recorder before the second (hit) call.
		sink.mu.Lock()
		sink.lookups = nil
		sink.extracts = nil
		sink.mu.Unlock()

		_, err = cache.GetOrExtract(filePath, func() ([]Tag, error) {
			t.Fatal("extractFn must not run on cache hit")
			return nil, nil
		})
		assert.NoError(t, err)

		lk, ex := sink.snapshot()
		if assert.Len(t, lk, 1, "expected exactly one lookup event on hit") {
			assert.Equal(t, "go", lk[0].lang)
			assert.Equal(t, LookupHit, lk[0].result)
		}
		assert.Empty(t, ex, "cache.go must not emit extract observations (Q-2 Option 2)")
	})

	t.Run("miss_branch_emits_miss", func(t *testing.T) {
		sink := &recordingSink{}
		cache := newTestCache(t)
		cache.SetMetricsSink(sink)

		dir := t.TempDir()
		filePath := writeTempFile(t, dir, "hello.go", "package main\n\nfunc Hello() {}\n")

		extractCalled := false
		_, err := cache.GetOrExtract(filePath, func() ([]Tag, error) {
			extractCalled = true
			return []Tag{
				{Name: "Hello", Kind: TagDef, File: filePath, Line: 2, Column: 5, StartByte: 20, EndByte: 50},
			}, nil
		})
		assert.NoError(t, err)
		assert.True(t, extractCalled, "extractFn must run on cache miss")

		lk, ex := sink.snapshot()
		if assert.Len(t, lk, 1, "expected exactly one lookup event on miss") {
			assert.Equal(t, "go", lk[0].lang)
			assert.Equal(t, LookupMiss, lk[0].result)
		}
		assert.Empty(t, ex, "cache.go must not emit extract observations (Q-2 Option 2)")
	})

	t.Run("nil_sink_default_is_safe", func(t *testing.T) {
		// A TagCache from NewTagCache() that is never wired with a sink
		// must default to NoopSink{} so emissions are safe no-ops.
		cache := newTestCache(t)
		// Explicit nil to also exercise the SetMetricsSink nil-normalization.
		cache.SetMetricsSink(nil)

		dir := t.TempDir()
		filePath := writeTempFile(t, dir, "hello.go", "package main\n\nfunc Hello() {}\n")

		_, err := cache.GetOrExtract(filePath, func() ([]Tag, error) {
			return []Tag{}, nil
		})
		assert.NoError(t, err)
	})
}

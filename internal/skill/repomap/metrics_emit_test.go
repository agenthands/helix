//go:build cgo

package repomap

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/agenthands/helix/internal/repomap"
	"github.com/agenthands/helix/internal/treesitter"
)

// dispatcherRecordingSink is a thread-safe repomap.MetricsSink test double
// scoped to this file (the internal/repomap package's recordingSink is
// package-private and not importable from here). Captures every call for
// per-extractor branch assertions.
type dispatcherRecordingSink struct {
	mu       sync.Mutex
	lookups  []string // result values
	extracts []extractRecord
}

type extractRecord struct {
	lang      string
	extractor string
}

func (r *dispatcherRecordingSink) RepoMapLookup(_, result string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lookups = append(r.lookups, result)
}

func (r *dispatcherRecordingSink) RepoMapExtractObserve(language, extractor string, _ float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.extracts = append(r.extracts, extractRecord{lang: language, extractor: extractor})
}

func (r *dispatcherRecordingSink) snapshot() ([]string, []extractRecord) {
	r.mu.Lock()
	defer r.mu.Unlock()
	lk := make([]string, len(r.lookups))
	copy(lk, r.lookups)
	ex := make([]extractRecord, len(r.extracts))
	copy(ex, r.extracts)
	return lk, ex
}

// TestDispatcher_TreesitterBranch_ObservesTreesitter exercises the primary
// dispatcher branch (s.extractor != nil) and asserts a single
// RepoMapExtractObserve(lang, "treesitter", _) emission per file.
func TestDispatcher_TreesitterBranch_ObservesTreesitter(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "tags.db")
	cache, err := repomap.NewTagCache(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { cache.Close() })

	registry := treesitter.NewGrammarRegistry()
	extractor, err := repomap.NewTagExtractor(registry)
	require.NoError(t, err)

	// Write a Go file (registry supports Go via go-tree-sitter).
	goFile := filepath.Join(dir, "hello.go")
	require.NoError(t, os.WriteFile(goFile, []byte("package main\n\nfunc Hello() {}\n"), 0o644))

	sink := &dispatcherRecordingSink{}
	cache.SetMetricsSink(sink) // wire cache for hit/miss assertion symmetry

	s := &RepoMapSkill{
		cache:     cache,
		extractor: extractor,
		registry:  registry,
		logger:    slog.Default(),
		metrics:   sink,
	}

	require.NoError(t, s.walkAndExtract(context.Background(), dir))

	_, extracts := sink.snapshot()
	require.Len(t, extracts, 1, "expected exactly one extract observation for the single .go file")
	assert.Equal(t, "go", extracts[0].lang)
	assert.Equal(t, repomap.ExtractorTreesitter, extracts[0].extractor)
}

// TestDispatcher_LSPBranch_ObservesLSP exercises the LSP fallback branch
// (extractor == nil but FallbackDeps wired) and asserts the extractor label
// is "lsp".
func TestDispatcher_LSPBranch_ObservesLSP(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "tags.db")
	cache, err := repomap.NewTagCache(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { cache.Close() })

	registry := treesitter.NewGrammarRegistry()

	javaFile := filepath.Join(dir, "Hello.java")
	require.NoError(t, os.WriteFile(javaFile, []byte("public class Hello { void greet() {} }\n"), 0o644))

	docSymResponse, _ := json.Marshal([]map[string]interface{}{
		{
			"name":           "Hello",
			"kind":           5,
			"range":          map[string]interface{}{"start": map[string]interface{}{"line": 0, "character": 0}, "end": map[string]interface{}{"line": 0, "character": 39}},
			"selectionRange": map[string]interface{}{"start": map[string]interface{}{"line": 0, "character": 13}, "end": map[string]interface{}{"line": 0, "character": 18}},
		},
	})
	mockReq := &mockFallbackRequester{response: docSymResponse}

	sink := &dispatcherRecordingSink{}
	s := &RepoMapSkill{
		cache:    cache,
		registry: registry,
		logger:   slog.Default(),
		metrics:  sink,
		fallbackDeps: &FallbackDeps{
			Extractor: repomap.NewFallbackExtractor(),
			AcquireFn: func(_ context.Context, lang string) (repomap.SymbolRequester, func(), error) {
				return mockReq, func() {}, nil
			},
		},
	}

	require.NoError(t, s.walkAndExtract(context.Background(), dir))

	_, extracts := sink.snapshot()
	require.Len(t, extracts, 1, "expected exactly one extract observation for the single .java file")
	assert.Equal(t, "java", extracts[0].lang)
	assert.Equal(t, repomap.ExtractorLSP, extracts[0].extractor)
}

// TestDispatcher_LSPBranch_ObservesLSPEvenOnAcquireError pins the design
// rule that failed extractions still record their latency (D-07).
func TestDispatcher_LSPBranch_ObservesLSPEvenOnAcquireError(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "tags.db")
	cache, err := repomap.NewTagCache(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { cache.Close() })

	registry := treesitter.NewGrammarRegistry()

	javaFile := filepath.Join(dir, "Hello.java")
	require.NoError(t, os.WriteFile(javaFile, []byte("public class Hello {}\n"), 0o644))

	sink := &dispatcherRecordingSink{}
	s := &RepoMapSkill{
		cache:    cache,
		registry: registry,
		logger:   slog.Default(),
		metrics:  sink,
		fallbackDeps: &FallbackDeps{
			Extractor: repomap.NewFallbackExtractor(),
			AcquireFn: func(_ context.Context, lang string) (repomap.SymbolRequester, func(), error) {
				return nil, nil, fmt.Errorf("no LS for %s", lang)
			},
		},
	}

	require.NoError(t, s.walkAndExtract(context.Background(), dir))

	_, extracts := sink.snapshot()
	require.Len(t, extracts, 1, "AcquireFn errors must still record latency (D-07)")
	assert.Equal(t, "java", extracts[0].lang)
	assert.Equal(t, repomap.ExtractorLSP, extracts[0].extractor)
}

// TestDispatcher_FallbackBranch_ObservesFallback exercises the third
// "no extractor and no LSP" branch and asserts the extractor label is
// "fallback".
func TestDispatcher_FallbackBranch_ObservesFallback(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "tags.db")
	cache, err := repomap.NewTagCache(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { cache.Close() })

	registry := treesitter.NewGrammarRegistry()

	// Java file, but NO extractor and NO fallbackDeps wired. Hits the
	// third "no extractor available for this language" branch.
	javaFile := filepath.Join(dir, "Hello.java")
	require.NoError(t, os.WriteFile(javaFile, []byte("public class Hello {}\n"), 0o644))

	sink := &dispatcherRecordingSink{}
	s := &RepoMapSkill{
		cache:    cache,
		registry: registry,
		logger:   slog.Default(),
		metrics:  sink,
		// extractor: nil, fallbackDeps: nil  → fallback branch
	}

	require.NoError(t, s.walkAndExtract(context.Background(), dir))

	_, extracts := sink.snapshot()
	require.Len(t, extracts, 1, "expected exactly one extract observation on the fallback branch")
	assert.Equal(t, "java", extracts[0].lang)
	assert.Equal(t, repomap.ExtractorFallback, extracts[0].extractor)
}

// TestDispatcher_NilSinkSafe pins the contract that a never-wired skill
// (s.metrics == nil) does not panic on emission. The metricsSink() helper
// normalizes nil to NoopSink{}.
func TestDispatcher_NilSinkSafe(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "tags.db")
	cache, err := repomap.NewTagCache(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { cache.Close() })

	registry := treesitter.NewGrammarRegistry()
	extractor, err := repomap.NewTagExtractor(registry)
	require.NoError(t, err)

	goFile := filepath.Join(dir, "hello.go")
	require.NoError(t, os.WriteFile(goFile, []byte("package main\n"), 0o644))

	// No metrics field set; bypass Init.
	s := &RepoMapSkill{
		cache:     cache,
		extractor: extractor,
		registry:  registry,
		logger:    slog.Default(),
	}

	// Must not panic.
	require.NoError(t, s.walkAndExtract(context.Background(), dir))
}

// TestDispatcher_RenderNoOpExtractFn_DoesNotEmitObserve pins Q-2 Option 2:
// render.go's no-op extractFn caller MUST NOT trigger a RepoMapExtractObserve
// emission, because no real extractor ran. The cache's lookup miss WILL fire
// (cache.go owns lookup emission), but the histogram must stay clean.
func TestDispatcher_RenderNoOpExtractFn_DoesNotEmitObserve(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "tags.db")
	cache, err := repomap.NewTagCache(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { cache.Close() })

	sink := &dispatcherRecordingSink{}
	cache.SetMetricsSink(sink)

	goFile := filepath.Join(dir, "hello.go")
	require.NoError(t, os.WriteFile(goFile, []byte("package main\n"), 0o644))

	// Drive cache.GetOrExtract with the same no-op extractFn shape that
	// internal/repomap/render.go:127-129 uses.
	_, err = cache.GetOrExtract(goFile, func() ([]repomap.Tag, error) {
		return nil, nil
	})
	require.NoError(t, err)

	lookups, extracts := sink.snapshot()
	assert.Len(t, lookups, 1, "expected exactly one lookup emission (miss)")
	assert.Equal(t, repomap.LookupMiss, lookups[0])
	assert.Empty(t, extracts, "render.go no-op extractFn must NOT trigger RepoMapExtractObserve")
}

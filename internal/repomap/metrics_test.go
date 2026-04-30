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

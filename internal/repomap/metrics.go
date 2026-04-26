package repomap

// MetricsSink is the minimal surface internal/repomap needs from the
// observability layer (D-08 invariant: repomap never imports internal/obs).
// Implemented by *obs.Metrics; compile-time assertion lives in
// internal/daemon/wiring_test.go (Plan 53-03).
type MetricsSink interface {
	// RepoMapCacheInc records a cache-decision for the given language.
	// result must be one of ResultHit / ResultMiss.
	RepoMapCacheInc(language, result string)
	// RepoMapExtractObserve records the duration in seconds of a cold-path
	// tag extraction for the given language.
	RepoMapExtractObserve(language string, seconds float64)
}

// Cache decision label values per Phase 53 D-02.
const (
	ResultHit  = "hit"
	ResultMiss = "miss"
)

// NoopSink is the zero-allocation default implementation; used when metrics
// are disabled or in tests that do not assert emission.
type NoopSink struct{}

// RepoMapCacheInc implements MetricsSink.
func (NoopSink) RepoMapCacheInc(string, string) {}

// RepoMapExtractObserve implements MetricsSink.
func (NoopSink) RepoMapExtractObserve(string, float64) {}

// Compile-time assertion that NoopSink satisfies MetricsSink.
var _ MetricsSink = NoopSink{}

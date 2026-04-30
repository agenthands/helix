package repomap

// MetricsSink is the minimal surface repomap needs from the observability
// layer. Implemented ad-hoc by *obs.Metrics (compile-time-checked at wire-up
// in internal/daemon/wiring_test.go); repomap itself never imports
// internal/obs (Phase 53 D-15).
//
// Method signatures are frozen to match the *obs.Metrics helpers declared
// in internal/obs/metrics.go (Phase 53 D-15). If those helper signatures
// drift, the wiring_test.go compile-time assertion fails — fix is to ALIGN
// this interface, NOT to mutate internal/obs/metrics.go.
type MetricsSink interface {
	// RepoMapLookup increments helix_repomap_lookups_total for a
	// (language, result) pair. result MUST be one of the LookupXxx
	// constants below — closed enum per Phase 53 D-04.
	RepoMapLookup(language, result string)

	// RepoMapExtractObserve records the elapsed seconds of a single
	// extractor invocation (cache-miss path only). extractor MUST be one
	// of the ExtractorXxx constants below — closed enum per Phase 53 D-07.
	RepoMapExtractObserve(language, extractor string, seconds float64)
}

// Cache lookup result constants — closed enum per Phase 53 D-04.
const (
	LookupHit  = "hit"
	LookupMiss = "miss"
)

// Extractor type constants — closed enum per Phase 53 D-07.
const (
	ExtractorTreesitter = "treesitter"
	ExtractorLSP        = "lsp"
	ExtractorFallback   = "fallback"
)

// NoopSink is used by tests and bootstrap paths where metrics are not wired.
// All methods are lock-free no-ops; a value-receiver keeps them trivially
// inlinable.
type NoopSink struct{}

// RepoMapLookup implements MetricsSink.
func (NoopSink) RepoMapLookup(string, string) {}

// RepoMapExtractObserve implements MetricsSink.
func (NoopSink) RepoMapExtractObserve(string, string, float64) {}

// Compile-time assertion that NoopSink satisfies MetricsSink.
var _ MetricsSink = NoopSink{}

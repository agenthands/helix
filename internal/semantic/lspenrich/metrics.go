package lspenrich

import "github.com/agenthands/helix/internal/obs"

// ProdMetricsSink delegates the lspenrich.MetricsSink interface (declared
// in types.go per B3 — single source of truth) to *obs.Metrics. The sink
// is pure forwarding: every method passes its arguments straight through to
// the matching closed-enum drop-on-unknown helper on *obs.Metrics. The
// helpers themselves enforce the bounded-label discipline (T-61-03-01).
//
// B3 resolution: ProdMetricsSink IMPLEMENTS the MetricsSink interface from
// types.go; it does NOT redeclare the interface. The compile-time
// assertion at the bottom of this file makes that contract structural.
type ProdMetricsSink struct{ M *obs.Metrics }

// NewProdMetricsSink returns a MetricsSink that forwards to *obs.Metrics.
// nil-safe: a nil m yields a noop sink so unit tests / degraded-bootstrap
// paths can construct the worker without an *obs.Metrics.
func NewProdMetricsSink(m *obs.Metrics) MetricsSink {
	if m == nil {
		return noopMetrics{}
	}
	return ProdMetricsSink{M: m}
}

func (p ProdMetricsSink) LSPEnrichmentTotal(language, outcome string) {
	p.M.LSPEnrichmentTotal(language, outcome)
}

func (p ProdMetricsSink) LSPEnrichmentDuration(language string, secs float64) {
	p.M.LSPEnrichmentDuration(language, secs)
}

func (p ProdMetricsSink) LSPEnrichmentErrors(language, outcome string) {
	p.M.LSPEnrichmentErrors(language, outcome)
}

func (p ProdMetricsSink) LSPEnrichmentLaneDepth(lane string, depth int) {
	p.M.LSPEnrichmentLaneDepth(lane, depth)
}

func (p ProdMetricsSink) LSPEnrichmentBulkSuppressed(n int) {
	p.M.LSPEnrichmentBulkSuppressed(n)
}

// Compile-time assertion: ProdMetricsSink satisfies MetricsSink (B3).
var _ MetricsSink = ProdMetricsSink{}

// noopMetrics is the package-internal nil-safe MetricsSink. Used by
// NewProdMetricsSink when m == nil and by NewManager when no sink is
// supplied. Mirrors the coalescer.noopMetrics pattern.
type noopMetrics struct{}

func (noopMetrics) LSPEnrichmentTotal(string, string)     {}
func (noopMetrics) LSPEnrichmentDuration(string, float64) {}
func (noopMetrics) LSPEnrichmentErrors(string, string)    {}
func (noopMetrics) LSPEnrichmentLaneDepth(string, int)    {}
func (noopMetrics) LSPEnrichmentBulkSuppressed(int)       {}

// Compile-time assertion: noopMetrics satisfies MetricsSink.
var _ MetricsSink = noopMetrics{}

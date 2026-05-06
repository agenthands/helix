package graph

// metrics.go declares the bounded-label metrics surface Engine + future
// Scheduler call. The MetricsSink interface lives in apply_repair.go (it
// is the natural home, alongside the Engine struct that owns the field);
// this file holds the no-op implementation tests can inject when they do
// not want to bind to *obs.Metrics.

// NoopMetrics implements MetricsSink with no-op methods. Useful in tests
// that exercise Engine without caring about metric emissions.
type NoopMetrics struct{}

// Compile-time assertion: NoopMetrics satisfies MetricsSink.
var _ MetricsSink = NoopMetrics{}

func (NoopMetrics) SemanticGraphVersionSet(string, uint64)             {}
func (NoopMetrics) SemanticGraphRepairInc(string)                       {}
func (NoopMetrics) SemanticGraphPagerankObserve(string, string, float64) {}
func (NoopMetrics) SemanticGraphScoreStatusInc(string, string)          {}

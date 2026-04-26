package lspool

// MetricsSink is the minimal surface lspool needs from the observability
// layer. Implemented by *obs.Metrics (checked at wire-up time in
// internal/daemon); lspool itself never imports internal/obs (D-08).
//
// Method signatures are frozen to match the *obs.Metrics helpers declared in
// internal/obs/metrics.go. A compile-time assertion in internal/daemon/
// wiring_test.go pins the two together.
type MetricsSink interface {
	// LSPoolWorkersSet adjusts the active worker gauge for a language by
	// delta. Positive on spawn, negative on destroy/evict.
	LSPoolWorkersSet(language string, delta float64)

	// LSPoolEviction increments the eviction counter for a language and
	// reason. reason must be one of the EvictXxx constants below (D-13).
	LSPoolEviction(language, reason string)

	// LSPoolCircuitStateSet publishes the circuit breaker state for a
	// language: CircuitClosed / CircuitHalfOpen / CircuitOpen (D-14).
	LSPoolCircuitStateSet(language string, state float64)

	// LSPoolRestart increments the restart counter for a language (D-15).
	LSPoolRestart(language string)

	// LSPoolCacheInc increments the cache-decision counter (Phase 53 D-13).
	// result must be one of ResultHit / ResultMiss; scope must be one of
	// ScopeClean / ScopeDirty / ScopeCrashed. Unknown values are dropped at
	// the helper-side (closed-enum guard in *obs.Metrics).
	LSPoolCacheInc(language, result, scope string)
}

// Eviction reason constants — closed enum per D-13. The label space for
// serena_lspool_evictions_total{reason} MUST stay bounded to this set so the
// CI cardinality lint can prove the bound.
const (
	EvictIdle     = "idle"
	EvictPressure = "pressure"
	EvictCrash    = "crash"
	EvictShutdown = "shutdown"
)

// Circuit state codes per D-14. Kept as float64 so sink implementations can
// call Gauge.Set directly without a conversion step.
const (
	CircuitClosed   float64 = 0
	CircuitHalfOpen float64 = 1
	CircuitOpen     float64 = 2
)

// Cache decision label values per Phase 53 D-02. result ∈ {hit, miss},
// scope ∈ {clean, dirty, crashed} — the closed enum guarded at the helper.
const (
	ResultHit  = "hit"
	ResultMiss = "miss"

	ScopeClean   = "clean"
	ScopeDirty   = "dirty"
	ScopeCrashed = "crashed"
)

// NoopSink is used by tests and bootstrap paths where metrics are not wired.
// All methods are lock-free no-ops; a value-receiver keeps them trivially
// inlinable.
type NoopSink struct{}

// LSPoolWorkersSet implements MetricsSink.
func (NoopSink) LSPoolWorkersSet(string, float64) {}

// LSPoolEviction implements MetricsSink.
func (NoopSink) LSPoolEviction(string, string) {}

// LSPoolCircuitStateSet implements MetricsSink.
func (NoopSink) LSPoolCircuitStateSet(string, float64) {}

// LSPoolRestart implements MetricsSink.
func (NoopSink) LSPoolRestart(string) {}

// LSPoolCacheInc implements MetricsSink.
func (NoopSink) LSPoolCacheInc(string, string, string) {}

// Compile-time assertion that NoopSink satisfies MetricsSink.
var _ MetricsSink = NoopSink{}

package kernel

// SessionMetricsSink is the minimal surface internal/kernel needs to emit
// workspace lifecycle counters (D-04/D-05). The daemon-layer call sites
// (deactivate, shutdown) consume this same interface from internal/daemon
// (Plan 53-03). Compile-time assertion lives in internal/daemon/wiring_test.go.
type SessionMetricsSink interface {
	// SessionLifecycleInc records a workspace lifecycle event for the given
	// language. phase must be one of Phase* constants.
	SessionLifecycleInc(language, phase string)
}

// Phase label values per Phase 53 D-04. Closed enum: activate | deactivate |
// timeout | shutdown.
const (
	PhaseActivate   = "activate"
	PhaseDeactivate = "deactivate"
	PhaseTimeout    = "timeout"
	PhaseShutdown   = "shutdown"
)

// NoopSessionSink is the zero-allocation default; named to avoid clobbering
// any existing NoopSink in package kernel.
type NoopSessionSink struct{}

// SessionLifecycleInc implements SessionMetricsSink.
func (NoopSessionSink) SessionLifecycleInc(string, string) {}

// Compile-time assertion that NoopSessionSink satisfies SessionMetricsSink.
var _ SessionMetricsSink = NoopSessionSink{}

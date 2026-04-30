package daemon

import (
	"net/http"
	"sync"

	"github.com/agenthands/helix/internal/obs"
)

// httpSessionMiddleware wraps the SDK's StreamableHTTPHandler to emit
// helix_session_lifecycle_total{transport="http"} on first-seen
// Mcp-Session-Id (started), DELETE /mcp (ended, best-effort), and 5xx
// responses (error). See Phase 53 D-09 + Q-1 Option 2 for the design
// rationale.
//
// CAVEAT (documented in USAGE.md by Plan 06): phase="ended" for HTTP is
// best-effort. The MCP SDK v1.5.0 (go.mod:14) does not expose a
// per-session lifecycle hook; the SDK owns session timeout/cleanup
// internally. This wrapper observes only the externally-visible signals
// — DELETE /mcp from the client (which the MCP spec defines for clean
// session termination) and 5xx responses from the upstream. Sessions
// that disappear due to server-side timeout will NOT register an
// "ended" event.
//
// Thread safety: the seen map uses sync.Map.LoadOrStore so concurrent
// first-seen requests for the same Mcp-Session-Id emit started exactly
// once (test: TestHTTPSessionMiddleware/concurrent_first_seen_idempotent).
//
// Memory bound: an attacker-supplied stream of distinct Mcp-Session-Id
// values can grow the seen map without bound (T-53-13). The MCP SDK's
// own session lifecycle bounds the live-session population in practice;
// future hardening (max session count + LRU eviction) is documented
// as a v1.10 follow-up if scrape data shows growth.
func httpSessionMiddleware(next http.Handler, metrics *obs.Metrics) http.Handler {
	var seen sync.Map // sessionID -> struct{}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sessionID := r.Header.Get("Mcp-Session-Id")
		if sessionID != "" {
			if _, loaded := seen.LoadOrStore(sessionID, struct{}{}); !loaded {
				metrics.SessionLifecycleInc("started", "http")
			}
		}
		if r.Method == http.MethodDelete && sessionID != "" {
			// Best-effort `ended` semantic — see CAVEAT above. The
			// emission fires BEFORE next.ServeHTTP so a panic in the
			// inner handler still leaves the metric consistent with
			// the client's intent (the client has already signalled
			// session termination by reaching the DELETE endpoint).
			metrics.SessionLifecycleInc("ended", "http")
			seen.Delete(sessionID)
		}
		rw := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(rw, r)
		if rw.effectiveStatus() >= 500 {
			metrics.SessionLifecycleInc("error", "http")
		}
	})
}

// statusRecorder captures the HTTP status code written by the inner
// handler so the wrapper can detect 5xx responses for the (error, http)
// emission.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

// WriteHeader records the status code before forwarding to the wrapped
// ResponseWriter so the inner handler's response is unaffected.
func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// effectiveStatus returns the recorded status, defaulting to 200 if the
// inner handler never called WriteHeader explicitly. This mirrors
// net/http's behavior on Write() without a prior WriteHeader: bytes are
// flushed with an implicit 200 status. Reading r.status when it's the
// zero value (0) would cause the >= 500 check to spuriously misclassify
// — the explicit accessor makes intent clear.
func (r *statusRecorder) effectiveStatus() int {
	if r.status == 0 {
		return http.StatusOK
	}
	return r.status
}

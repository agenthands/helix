package guardrails

import (
	"log/slog"
	"os"
	"sync/atomic"
)

// jsonlSink is the package-level slog.Logger used to emit "receipt issued"
// JSONL lines consumed by internal/eval/trace/tap.go (F-08 producer side).
//
// The eval harness's trace tap reads daemon.log and looks for
// msg=="receipt issued" with fields {receipt_class, workspace, snapshot_id,
// graph_version, trace_id, pid}. Wired by SetJSONLLogger from the daemon
// bootstrap; nil-safe (no emission when unset, e.g. unit tests).
//
// atomic.Pointer is used instead of a plain *slog.Logger so concurrent
// callers of emitReceiptIssued never race a SetJSONLLogger call from test
// setup or hot-reload paths.
var jsonlSink atomic.Pointer[slog.Logger]

// SetJSONLLogger wires the JSONL emitter. Pass nil to disable emission.
// Process-global state (mirrors the other slog-sink wiring patterns across
// internal/mcp/middleware.go); tests that swap the sink must not run in
// parallel and should restore via t.Cleanup.
func SetJSONLLogger(l *slog.Logger) {
	if l == nil {
		jsonlSink.Store(nil)
		return
	}
	jsonlSink.Store(l)
}

// emitReceiptIssued logs a single tap-compatible "receipt issued" JSONL line.
// No-ops when jsonlSink is unset. Called from Store.Issue's success path
// after the receipt has been inserted and ReceiptIssuedInc has fired.
func emitReceiptIssued(class string, wsKey string, snapshotID, graphVersion uint64, traceID string) {
	l := jsonlSink.Load()
	if l == nil {
		return
	}
	l.Info("receipt issued",
		"receipt_class", class,
		"workspace", wsKey,
		"snapshot_id", snapshotID,
		"graph_version", graphVersion,
		"trace_id", traceID,
		"pid", os.Getpid(),
	)
}

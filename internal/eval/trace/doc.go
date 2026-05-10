// Package trace reads the two evidence streams produced by a Phase 67 eval run
// (daemon TelemetryMiddleware JSONL and Claude Code --output-format=stream-json
// output), merges them into a single chronologically-ordered trace.json, and
// writes the per-mode per-task artifacts (trace_daemon.jsonl, trace_cc.json,
// trace.json). Wall-clock alignment is the merge key per 67-RESEARCH.md
// §"Trace Merge Schema". Real implementation lands in Wave 1+.
package trace

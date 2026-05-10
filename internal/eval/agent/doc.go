// Package agent wraps the Claude Code CLI subprocess (and the scripted-agent
// shim for eval-quick) used by the Phase 67 evaluation harness. It builds the
// correct argv (--bare --strict-mcp-config --output-format=stream-json, etc.),
// starts the process, captures stdout/stderr, and feeds the output to the trace
// collector. Real implementation lands in Wave 1+.
package agent

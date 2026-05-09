// Package guardrails registers the no-tool skill that triggers the
// guardrails package's init() side effects (receipt-sink wiring) via a
// blank import in internal/daemon/imports.go.
//
// The actual middleware enforcement path lives in internal/mcp/guardrail_middleware.go
// and is wired into the MCP server by internal/daemon/daemon.go step 14b.5.
// This skill exists solely to satisfy the Caddy-style init() registration
// convention; it exposes no MCP tools.
package guardrails

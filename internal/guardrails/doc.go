// Package guardrails ships server-side capability receipts (Phase 66, GUARD-01..GUARD-07).
// Middleware install order MUST remain LIFO with LazyInit installed last (executes first);
// see internal/mcp/lazy_init.go:106-112.
package guardrails

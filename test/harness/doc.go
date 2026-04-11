//go:build integration || llm || llmjudge

// Package harness provides shared test infrastructure for oracle test packages.
// It wraps daemon creation, MCP client sessions, fixture preparation, tool
// invocation helpers, and golden file management.
//
// Oracle packages under test/oracle/ import this package. Do NOT import
// anything from test/integration/ (D-01, D-10).
package harness

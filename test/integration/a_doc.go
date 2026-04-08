//go:build integration

// Package integration_test provides end-to-end integration tests for Serena's MCP tools.
// Tests exercise the full protocol path: MCP client -> daemon -> kernel -> tool -> response.
//
// Run with: go test -tags integration ./test/integration/...
package integration_test

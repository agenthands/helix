// Package integ contains integration tests for the MCP layer.
// Tests in this package use the //go:build integration tag and require
// a real receipt store + Prometheus metrics instance.
//
// Run with: go test -tags=integration ./internal/mcp/integ/
package integ

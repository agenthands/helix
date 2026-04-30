// Package integration_test provides end-to-end integration tests for Helix's MCP tools.
// Tests exercise the full protocol path: MCP client -> daemon -> kernel -> tool -> response.
//
// The Java subset (java_test.go) runs under default `go test ./...` by sharing a warm
// jdtls workspace across runs via test/integration/jdtlscache (Phase 48, BUG-03).
// Other language tests remain gated on `-tags integration`.
package integration_test

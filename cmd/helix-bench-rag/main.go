// Command helix-bench-rag is a STANDALONE, provably-isolated MCP server for the
// bench `baseline_rag` control arm (Phase 83, ABLATE-04 criterion #1). It exposes
// EXACTLY four tools — rag_search, rag_read_chunk, grep, read_file — over the
// leaf bench/ragindex embedding index, and shares NO code with the Helix daemon.
//
// CRITICAL ISOLATION INVARIANT: this binary MUST NOT import internal/kernel,
// internal/semantic, OR internal/mcp (the last transitively links the former
// two). The MCP server is constructed via the SDK directly
// (github.com/modelcontextprotocol/go-sdk/mcp: NewServer + AddTool +
// StdioTransport). The boundary is enforced both dynamically (leakage_test.go's
// transitive go/packages NeedDeps test) and statically (internal/lint/
// benchragleakage wired into `make vet`).
//
// main() is the only os.Exit site; the root uses RunE so errors propagate to the
// single exit point (mirrors cmd/helix-bench/main.go).
package main

import (
	"fmt"
	"os"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

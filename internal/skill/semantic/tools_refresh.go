package semantic

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// RefreshSemanticGraphArgs is the typed-args input schema for
// refresh_semantic_graph (stub — Task 1 RED gate; full impl lands in Task 2).
type RefreshSemanticGraphArgs struct {
	Paths      []string `json:"paths,omitempty"           jsonschema:"strict-subset path filter"`
	WaitForLSP bool     `json:"wait_for_lsp,omitempty"`
	MaxWaitMs  int      `json:"max_wait_ms,omitempty"     jsonschema:"default 3000"`
}

// handleRefreshSemanticGraph is the testable handler body.
//
// Stub for Task 1 RED gate — panics so the failing tests in
// tools_refresh_test.go can be observed under `go test`. Real implementation
// lands in Task 2.
func (s *SemanticSkill) handleRefreshSemanticGraph(ctx context.Context, args RefreshSemanticGraphArgs) *mcpsdk.CallToolResult {
	panic("not implemented (RED gate stub — Task 2 will implement)")
}

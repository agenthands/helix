// export_status_test.go — Phase 69-06: test-only export of the
// unexported handleGetSemanticGraphStatus handler so the external
// `package semantic_test` E2E (status_e2e_external_test.go) can invoke
// it without forcing a new public method onto SemanticSkill.
//
// Pattern: standard Go `export_test.go` — file lives in `package
// semantic` and is compiled only for tests, so the exported name is
// visible to the black-box `semantic_test` package without leaking into
// the production API.

package semantic

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// HandleGetSemanticGraphStatusForTest invokes the unexported status
// handler. Plan 69-06 E2E test calls this against a SemanticSkill wired
// with the production daemon.NewSchedulerAccessorForStore +
// daemon.NewRetrievalAccessorForStore factory adapters.
func HandleGetSemanticGraphStatusForTest(s *SemanticSkill, ctx context.Context) *mcpsdk.CallToolResult {
	return s.handleGetSemanticGraphStatus(ctx, GetSemanticGraphStatusArgs{})
}

// HandleIndexSemanticGraphForTest invokes the unexported index handler.
// Plan 69-06 E2E test uses this to drive a real BeginSnapshot →
// WriteSnapshotFacts → CommitSnapshot through the harness buildFn before
// asserting status.
func HandleIndexSemanticGraphForTest(s *SemanticSkill, ctx context.Context, mode string) *mcpsdk.CallToolResult {
	return s.handleIndexSemanticGraph(ctx, IndexSemanticGraphArgs{Mode: mode})
}

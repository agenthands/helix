package guardrails

import (
	"context"
	"sync/atomic"
)

// receiptIssueSink is the package-level atomic sink for receipt issuance.
// Mirrors the editOutcomeSink pattern from internal/mcp/middleware.go.
// nil = no-op (test mode); set by daemon bootstrap (Plan 04) via SetReceiptIssueSink.
var receiptIssueSink atomic.Pointer[func(ctx context.Context, class ReceiptClass, scope ReceiptScope, tool string) (ReceiptID, error)]

// SetReceiptIssueSink installs the production receipt issuance closure.
// Called once from daemon bootstrap (Plan 04) after the Store is initialized.
// Subsequent calls overwrite the previous sink.
func SetReceiptIssueSink(fn func(ctx context.Context, class ReceiptClass, scope ReceiptScope, tool string) (ReceiptID, error)) {
	receiptIssueSink.Store(&fn)
}

// SetReceiptIssueSinkForTest installs a test sink and returns a cleanup function
// that restores the nil sink. Use with t.Cleanup:
//
//	t.Cleanup(guardrails.SetReceiptIssueSinkForTest(myFakeSink))
func SetReceiptIssueSinkForTest(fn func(ctx context.Context, class ReceiptClass, scope ReceiptScope, tool string) (ReceiptID, error)) func() {
	SetReceiptIssueSink(fn)
	return func() {
		receiptIssueSink.Store(nil)
	}
}

// IssueReceiptOnSuccess calls the installed sink to issue a receipt.
// Returns ("", nil) if no sink is installed (test mode / read-tool tests that
// don't wire the full receipt store).
// Tool handlers call this on their success path:
//
//	id, _ := guardrails.IssueReceiptOnSuccess(ctx, guardrails.ClassReferencesChecked, scope, "find_references")
func IssueReceiptOnSuccess(ctx context.Context, class ReceiptClass, scope ReceiptScope, tool string) (ReceiptID, error) {
	p := receiptIssueSink.Load()
	if p == nil || *p == nil {
		return "", nil
	}
	return (*p)(ctx, class, scope, tool)
}

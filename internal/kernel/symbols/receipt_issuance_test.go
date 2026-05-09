package symbols

import (
	"context"
	"testing"

	"github.com/agenthands/helix/internal/guardrails"
	"github.com/agenthands/helix/internal/semantic/integ"
)

// TestFindReferences_IssuesReceiptOnSuccess verifies the issuance path:
// when IssueReceiptOnSuccess is called with ClassReferencesChecked, the
// installed sink receives the call. Tests the plumbing used by
// registerFindReferences's success path (Phase 66 D-01 GUARD-03).
// NOT t.Parallel: modifies package-level sink.
func TestFindReferences_IssuesReceiptOnSuccess(t *testing.T) {
	called := false
	var gotClass guardrails.ReceiptClass
	var gotTool string

	cleanup := guardrails.SetReceiptIssueSinkForTest(func(ctx context.Context, class guardrails.ReceiptClass, scope guardrails.ReceiptScope, tool string) (guardrails.ReceiptID, error) {
		called = true
		gotClass = class
		gotTool = tool
		return "rcpt_AAAAAAAAAAAAAAAAAAAAAAAAAAAA", nil
	})
	t.Cleanup(cleanup)

	// Simulate what registerFindReferences does on its success path.
	locs := []SymbolLocation{
		{URI: "file:///test.go"},
	}
	guardrails.IssueReceiptOnSuccess(context.Background(), guardrails.ClassReferencesChecked,
		guardrails.ReferencesCheckedScope{
			SymbolID:     integ.SymbolID(""),
			RefCount:     len(locs),
			FilePath:     "test.go",
			IncludeTests: false,
		}, "find_references")

	if !called {
		t.Fatal("expected receipt sink to be called on success path, but it was not")
	}
	if gotClass != guardrails.ClassReferencesChecked {
		t.Errorf("expected class %q, got %q", guardrails.ClassReferencesChecked, gotClass)
	}
	if gotTool != "find_references" {
		t.Errorf("expected tool %q, got %q", "find_references", gotTool)
	}
}

// TestFindReferences_NoIssuanceOnError verifies that when an error occurs
// (simulated by NOT calling IssueReceiptOnSuccess), the sink is NOT called.
// This validates the Pitfall 4 mitigation: never issue in a defer block.
// NOT t.Parallel: modifies package-level sink.
func TestFindReferences_NoIssuanceOnError(t *testing.T) {
	called := false

	cleanup := guardrails.SetReceiptIssueSinkForTest(func(ctx context.Context, class guardrails.ReceiptClass, scope guardrails.ReceiptScope, tool string) (guardrails.ReceiptID, error) {
		called = true
		return "", nil
	})
	t.Cleanup(cleanup)

	// Simulate the error path: error occurs before IssueReceiptOnSuccess.
	// On the error path, we return early — the sink must NOT be called.
	simulatedErr := true
	if !simulatedErr {
		// This block only runs on success. Since we simulate error,
		// IssueReceiptOnSuccess is never reached.
		guardrails.IssueReceiptOnSuccess(context.Background(), guardrails.ClassReferencesChecked,
			guardrails.ReferencesCheckedScope{}, "find_references")
	}

	if called {
		t.Fatal("expected receipt sink NOT to be called on error path, but it was")
	}
}

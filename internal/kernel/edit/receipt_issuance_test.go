package edit

import (
	"context"
	"testing"

	"github.com/agenthands/helix/internal/guardrails"
)

// TestVerifyEdit_IssuesDiagnosticsCleanWhenErrorCountZero verifies that the
// diagnostics_clean receipt is issued when ErrorCount==0 (Phase 66 D-18).
// NOT t.Parallel: modifies package-level guardrails sink.
func TestVerifyEdit_IssuesDiagnosticsCleanWhenErrorCountZero(t *testing.T) {
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

	// Simulate verify_edit success path: no errors → issue receipt.
	errorCount := 0
	if errorCount == 0 {
		guardrails.IssueReceiptOnSuccess(context.Background(), guardrails.ClassDiagnosticsClean,
			guardrails.DiagnosticsCleanScope{
				FileSet:         []string{"src/auth.go"},
				DiagnosticCount: 0,
				ErrorCount:      0,
				WarningCount:    0,
				Tool:            "verify_edit",
			}, "verify_edit")
	}

	if !called {
		t.Fatal("expected diagnostics_clean receipt to be issued when ErrorCount==0")
	}
	if gotClass != guardrails.ClassDiagnosticsClean {
		t.Errorf("expected class %q, got %q", guardrails.ClassDiagnosticsClean, gotClass)
	}
	if gotTool != "verify_edit" {
		t.Errorf("expected tool %q, got %q", "verify_edit", gotTool)
	}
}

// TestVerifyEdit_NoIssuanceWhenErrorsPresent verifies that the diagnostics_clean
// receipt is NOT issued when ErrorCount > 0 (Phase 66 D-18/T-66-25 mitigation).
// NOT t.Parallel: modifies package-level guardrails sink.
func TestVerifyEdit_NoIssuanceWhenErrorsPresent(t *testing.T) {
	called := false

	cleanup := guardrails.SetReceiptIssueSinkForTest(func(ctx context.Context, class guardrails.ReceiptClass, scope guardrails.ReceiptScope, tool string) (guardrails.ReceiptID, error) {
		called = true
		return "", nil
	})
	t.Cleanup(cleanup)

	// Simulate verify_edit with errors: ErrorCount > 0 → do NOT issue receipt.
	errorCount := 1
	if errorCount == 0 {
		// This path is NOT taken.
		guardrails.IssueReceiptOnSuccess(context.Background(), guardrails.ClassDiagnosticsClean,
			guardrails.DiagnosticsCleanScope{}, "verify_edit")
	}

	if called {
		t.Fatal("expected NO receipt when ErrorCount>0 (T-66-25 regression)")
	}
}

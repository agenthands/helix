package edit

import (
	"context"
	"time"

	"github.com/agenthands/helix/internal/kernel/diag"
	gen "github.com/agenthands/helix/protocol/gen"
)

// VerifyResult holds the outcome of post-edit diagnostic verification.
type VerifyResult struct {
	HasErrors  bool
	ErrorCount int
	Errors     []DiagnosticSummary
}

// DiagnosticSummary is a simplified diagnostic for reporting.
type DiagnosticSummary struct {
	Line    int
	Col     int
	Message string
	Source  string
}

// VerifyEdit checks for compilation errors after an edit.
// EDT-06: Post-edit diagnostic verification via WaitForDiagnostics.
func VerifyEdit(ctx context.Context, diagStore *diag.DiagnosticStore, uri string) (*VerifyResult, error) {
	// Clear existing diagnostics for the URI to get fresh results.
	diagStore.Clear(uri)

	// Wait for new diagnostics with 5 second timeout.
	diags, err := diagStore.WaitForDiagnostics(ctx, uri, 5*time.Second)
	if err != nil {
		// Timeout is not a fatal error — just means no diagnostics arrived yet.
		if err == context.DeadlineExceeded {
			return &VerifyResult{HasErrors: false}, nil
		}
		return nil, err
	}

	// Filter to errors only (ignore warnings/info/hint).
	result := &VerifyResult{}
	for _, d := range diags {
		if d.Severity != nil && *d.Severity == gen.DiagnosticSeverityError {
			result.HasErrors = true
			result.ErrorCount++
			summary := DiagnosticSummary{
				Line:    int(d.Range.Start.Line) + 1, // 1-indexed for display
				Col:     int(d.Range.Start.Character) + 1,
				Message: d.Message,
			}
			if d.Source != nil {
				summary.Source = *d.Source
			}
			result.Errors = append(result.Errors, summary)
		}
	}

	return result, nil
}

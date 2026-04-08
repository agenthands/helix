package edit

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/postfix/serena/internal/kernel/lspool"
)

// InsertBefore inserts content immediately before a symbol's range start.
// Per D-13: LSP-first, DocumentSymbol range boundaries are sufficient.
func InsertBefore(ctx context.Context, lease *lspool.WorkerLease, uri string, symbolName string, content string) error {
	plan, err := PlanEdit(ctx, lease, uri, symbolName, EditTypeInsertBefore, content)
	if err != nil {
		return fmt.Errorf("plan edit: %w", err)
	}
	return InsertBeforeWithPlan(ctx, lease, plan)
}

// InsertBeforeWithPlan executes an insert-before using a pre-computed plan.
func InsertBeforeWithPlan(ctx context.Context, lease *lspool.WorkerLease, plan *EditPlan) error {
	filePath := uriToPath(plan.URI)

	source, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read file %s: %w", filePath, err)
	}

	// Compute insertion point at symbol's range start.
	insertByte, _ := rangeToByteOffsets(source, plan.Range)

	// Ensure content ends with newline for clean separation.
	insertContent := plan.NewContent
	if !strings.HasSuffix(insertContent, "\n") {
		insertContent += "\n"
	}

	var result []byte
	result = append(result, source[:insertByte]...)
	result = append(result, []byte(insertContent)...)
	result = append(result, source[insertByte:]...)

	if err := os.WriteFile(filePath, result, 0644); err != nil {
		return fmt.Errorf("write file %s: %w", filePath, err)
	}

	if err := notifyDidChange(ctx, lease, plan.URI, string(result)); err != nil {
		return fmt.Errorf("didChange notification: %w", err)
	}

	return nil
}

// InsertAfter inserts content immediately after a symbol's range end.
// Per D-13: LSP-first, DocumentSymbol range boundaries are sufficient.
func InsertAfter(ctx context.Context, lease *lspool.WorkerLease, uri string, symbolName string, content string) error {
	plan, err := PlanEdit(ctx, lease, uri, symbolName, EditTypeInsertAfter, content)
	if err != nil {
		return fmt.Errorf("plan edit: %w", err)
	}
	return InsertAfterWithPlan(ctx, lease, plan)
}

// InsertAfterWithPlan executes an insert-after using a pre-computed plan.
func InsertAfterWithPlan(ctx context.Context, lease *lspool.WorkerLease, plan *EditPlan) error {
	filePath := uriToPath(plan.URI)

	source, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read file %s: %w", filePath, err)
	}

	// Compute insertion point at symbol's range end.
	_, insertByte := rangeToByteOffsets(source, plan.Range)

	// Add newline separator between symbol and inserted content.
	insertContent := "\n" + plan.NewContent
	if !strings.HasSuffix(insertContent, "\n") {
		insertContent += "\n"
	}

	var result []byte
	result = append(result, source[:insertByte]...)
	result = append(result, []byte(insertContent)...)
	result = append(result, source[insertByte:]...)

	if err := os.WriteFile(filePath, result, 0644); err != nil {
		return fmt.Errorf("write file %s: %w", filePath, err)
	}

	if err := notifyDidChange(ctx, lease, plan.URI, string(result)); err != nil {
		return fmt.Errorf("didChange notification: %w", err)
	}

	return nil
}

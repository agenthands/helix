package mcp_test

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/agenthands/helix/internal/config"
	"github.com/agenthands/helix/internal/degrade"
	"github.com/agenthands/helix/internal/mcp"
	"github.com/agenthands/helix/internal/obs"
)

// slowHandler returns a MethodHandler that sleeps for the given duration.
func slowHandler(d time.Duration) mcpsdk.MethodHandler {
	return func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
		select {
		case <-time.After(d):
			return &mcpsdk.CallToolResult{}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func noopSession(ctx context.Context) *mcp.SessionInfo {
	return &mcp.SessionInfo{Profile: "p", Mode: "m", Language: "l"}
}

// budgetFnFrom creates a BudgetFunc from a DegradationConfig.
func budgetFnFrom(cfg config.DegradationConfig) mcp.BudgetFunc {
	return func(toolName string) time.Duration {
		return degrade.BudgetFor(toolName, cfg)
	}
}

func TestDeadlinePropagation_SlowHandler(t *testing.T) {
	// A handler sleeping 2s with ClassRead budget (5s default) completes normally.
	provider := obs.Noop(slog.NewTextHandler(io.Discard, nil))
	bf := budgetFnFrom(config.DegradationConfig{}) // zero value -> defaults (5s for read)
	mw := mcp.TelemetryMiddleware(provider, noopSession, bf, discardLogger())

	h := mw(slowHandler(2 * time.Second))
	start := time.Now()
	result, err := h(context.Background(), "tools/call", newCallToolReq("read_file"))
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if elapsed < 2*time.Second {
		t.Errorf("handler returned too quickly: %v", elapsed)
	}
}

func TestDeadlinePropagation_Timeout(t *testing.T) {
	// A handler sleeping 10s with ClassRead budget (5s default) returns DeadlineExceeded.
	provider := obs.Noop(slog.NewTextHandler(io.Discard, nil))
	bf := budgetFnFrom(config.DegradationConfig{}) // zero value -> defaults (5s for read)
	mw := mcp.TelemetryMiddleware(provider, noopSession, bf, discardLogger())

	h := mw(slowHandler(10 * time.Second))
	start := time.Now()
	_, err := h(context.Background(), "tools/call", newCallToolReq("read_file"))
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected deadline exceeded error")
	}
	if err != context.DeadlineExceeded {
		t.Errorf("expected DeadlineExceeded, got: %v", err)
	}
	// Should timeout around 5s, not 10s.
	if elapsed > 7*time.Second {
		t.Errorf("took too long (%v), expected ~5s timeout", elapsed)
	}
}

func TestDeadlinePropagation_ConfigOverride(t *testing.T) {
	// DegradationConfig{TimeoutRead: 1} causes a 2s handler to timeout.
	provider := obs.Noop(slog.NewTextHandler(io.Discard, nil))
	bf := budgetFnFrom(config.DegradationConfig{TimeoutRead: 1}) // 1 second override
	mw := mcp.TelemetryMiddleware(provider, noopSession, bf, discardLogger())

	h := mw(slowHandler(2 * time.Second))
	start := time.Now()
	_, err := h(context.Background(), "tools/call", newCallToolReq("read_file"))
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected deadline exceeded error")
	}
	if err != context.DeadlineExceeded {
		t.Errorf("expected DeadlineExceeded, got: %v", err)
	}
	if elapsed > 2*time.Second {
		t.Errorf("took too long (%v), expected ~1s timeout", elapsed)
	}
}

func TestDeadlinePropagation_NonToolCall(t *testing.T) {
	// method "tools/list" does NOT get a deadline (passes through unchanged).
	provider := obs.Noop(slog.NewTextHandler(io.Discard, nil))
	bf := budgetFnFrom(config.DegradationConfig{TimeoutRead: 1})
	mw := mcp.TelemetryMiddleware(provider, noopSession, bf, discardLogger())

	// This handler checks that the context has no deadline.
	inner := func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
		if _, ok := ctx.Deadline(); ok {
			t.Error("non-tool-call method should NOT have a deadline")
		}
		return &mcpsdk.ListToolsResult{}, nil
	}

	h := mw(inner)
	_, err := h(context.Background(), "tools/list", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeadlinePropagation_ZeroBudget(t *testing.T) {
	// An unmapped tool still gets ClassRead default budget (never zero).
	provider := obs.Noop(slog.NewTextHandler(io.Discard, nil))
	bf := budgetFnFrom(config.DegradationConfig{}) // zero values -> defaults
	mw := mcp.TelemetryMiddleware(provider, noopSession, bf, discardLogger())

	// This handler verifies the context has a deadline.
	inner := func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
		deadline, ok := ctx.Deadline()
		if !ok {
			t.Error("unmapped tool should still get a deadline from ClassRead default")
		}
		// Budget should be ~5s (ClassRead default).
		remaining := time.Until(deadline)
		if remaining < 3*time.Second || remaining > 6*time.Second {
			t.Errorf("expected ~5s budget, got %v", remaining)
		}
		return &mcpsdk.CallToolResult{}, nil
	}

	h := mw(inner)
	_, err := h(context.Background(), "tools/call", newCallToolReq("totally_unknown_tool"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

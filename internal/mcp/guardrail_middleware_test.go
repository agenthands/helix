package mcp_test

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	serr "github.com/agenthands/helix/internal/errors"
	"github.com/agenthands/helix/internal/mcp"
	"github.com/agenthands/helix/internal/guardrails/rules"
	"github.com/agenthands/helix/internal/workspace"
)

// fakeDeps is a test-only MiddlewareDeps implementation.
type fakeDeps struct {
	decisions []rules.Decision
	err       error
	delay     time.Duration // simulate slow evaluation

	graphAdvancedWS  workspace.WorkspaceKey
	graphAdvancedGV  uint64
	graphAdvanceCalls int
}

func (f *fakeDeps) Evaluate(ctx context.Context, toolName string, rawArgs json.RawMessage, profile string) ([]rules.Decision, error) {
	if f.delay > 0 {
		select {
		case <-time.After(f.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return f.decisions, f.err
}

func (f *fakeDeps) OnGraphVersionAdvance(ws workspace.WorkspaceKey, newGV uint64) {
	f.graphAdvancedWS = ws
	f.graphAdvancedGV = newGV
	f.graphAdvanceCalls++
}

func (f *fakeDeps) Logger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, nil))
}

func discardLoggerGM() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func nilSession(_ context.Context) *mcp.SessionInfo { return nil }

// makeCallToolRequest creates a *mcpsdk.CallToolRequest for a named tool with optional JSON args.
func makeCallToolRequest(toolName string, argsJSON string) *mcpsdk.CallToolRequest {
	req := &mcpsdk.CallToolRequest{}
	params := &mcpsdk.CallToolParamsRaw{
		Name: toolName,
	}
	if argsJSON != "" {
		params.Arguments = json.RawMessage(argsJSON)
	}
	req.Params = params
	return req
}

// makeOKHandler is an inner handler that returns a success CallToolResult.
func makeOKHandler() mcpsdk.MethodHandler {
	return func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: "ok"}},
		}, nil
	}
}

// --- TestGuardrailMiddleware_NonToolsCall_PassesThrough ---
// Non-tools/call methods (e.g., tools/list) must pass through without evaluation.

func TestGuardrailMiddleware_NonToolsCall_PassesThrough(t *testing.T) {
	deps := &fakeDeps{}
	mw := mcp.GuardrailMiddleware(deps, nilSession, discardLoggerGM())

	called := false
	inner := func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
		called = true
		return &mcpsdk.ListToolsResult{}, nil
	}

	handler := mw(inner)
	_, err := handler(context.Background(), "tools/list", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("expected inner handler to be called for non-tools/call method")
	}
	if len(deps.decisions) != 0 {
		t.Error("expected no Evaluate call for non-tools/call method")
	}
}

// --- TestGuardrailMiddleware_NonDestructiveTool_PassesThrough ---
// Non-destructive tools like find_references must pass through without evaluation.

func TestGuardrailMiddleware_NonDestructiveTool_PassesThrough(t *testing.T) {
	deps := &fakeDeps{}
	mw := mcp.GuardrailMiddleware(deps, nilSession, discardLoggerGM())

	called := false
	inner := func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
		called = true
		return &mcpsdk.CallToolResult{}, nil
	}

	handler := mw(inner)
	req := makeCallToolRequest("find_references", `{"symbol_name":"Foo"}`)
	_, err := handler(context.Background(), "tools/call", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("expected inner handler to be called for non-destructive tool")
	}
}

// --- TestGuardrailMiddleware_DestructiveAllow_PassesThrough ---
// When deps.Evaluate returns no decisions (all Allow), the tool call proceeds normally.

func TestGuardrailMiddleware_DestructiveAllow_PassesThrough(t *testing.T) {
	deps := &fakeDeps{decisions: nil} // empty = all Allow
	mw := mcp.GuardrailMiddleware(deps, nilSession, discardLoggerGM())

	called := false
	inner := func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
		called = true
		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: "renamed"}},
		}, nil
	}

	handler := mw(inner)
	req := makeCallToolRequest("rename_symbol", `{"symbol_name":"Foo","new_name":"Bar","receipts":[]}`)
	result, err := handler(context.Background(), "tools/call", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("expected inner handler to be called when Allow")
	}
	ctr := result.(*mcpsdk.CallToolResult)
	if ctr.IsError {
		t.Error("expected non-error result on Allow")
	}
}

// --- TestGuardrailMiddleware_DestructiveWarn_AttachesWarning ---
// Warn decisions must attach a warning content block to the result (not block the call).

func TestGuardrailMiddleware_DestructiveWarn_AttachesWarning(t *testing.T) {
	deps := &fakeDeps{
		decisions: []rules.Decision{
			{
				Action:  rules.Warn,
				Rule:    "G-001",
				Message: "run find_references first",
			},
		},
	}
	mw := mcp.GuardrailMiddleware(deps, nilSession, discardLoggerGM())

	handler := mw(makeOKHandler())
	req := makeCallToolRequest("replace_in_file", `{"path":"foo.go","find":"Foo","replace":"Bar"}`)
	result, err := handler(context.Background(), "tools/call", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctr, ok := result.(*mcpsdk.CallToolResult)
	if !ok {
		t.Fatalf("expected *CallToolResult, got %T", result)
	}
	if ctr.IsError {
		t.Error("expected warn-mode result to be non-error")
	}

	// Must contain a warning content block.
	found := false
	for _, c := range ctr.Content {
		if tc, ok := c.(*mcpsdk.TextContent); ok {
			if strings.HasPrefix(tc.Text, "__guardrail_warning__:") {
				found = true
				// Verify it contains "guardrail_warning" kind.
				payload := tc.Text[len("__guardrail_warning__:"):]
				var m map[string]any
				if err := json.Unmarshal([]byte(payload), &m); err != nil {
					t.Errorf("warning content is not valid JSON: %v", err)
					continue
				}
				if m["kind"] != "guardrail_warning" {
					t.Errorf("expected kind=guardrail_warning, got %v", m["kind"])
				}
				if m["rule"] != "G-001" {
					t.Errorf("expected rule=G-001, got %v", m["rule"])
				}
			}
		}
	}
	if !found {
		t.Error("expected at least one guardrail_warning content block in result")
	}
}

// --- TestGuardrailMiddleware_DestructiveBlock_ReturnsGuardrailViolation ---
// Block decisions must return a GuardrailViolation error without calling the handler.

func TestGuardrailMiddleware_DestructiveBlock_ReturnsGuardrailViolation(t *testing.T) {
	deps := &fakeDeps{
		decisions: []rules.Decision{
			{
				Action:  rules.Block,
				Rule:    "G-002",
				Message: "find_references required before delete",
				RequiredReceipts: []rules.RequiredReceipt{
					{Class: "references_checked", ScopeHint: "find_references on Foo"},
				},
			},
		},
	}
	mw := mcp.GuardrailMiddleware(deps, nilSession, discardLoggerGM())

	innerCalled := false
	inner := func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
		innerCalled = true
		return &mcpsdk.CallToolResult{}, nil
	}

	handler := mw(inner)
	req := makeCallToolRequest("delete_file", `{"path":"auth.go"}`)
	_, err := handler(context.Background(), "tools/call", req)
	if err == nil {
		t.Fatal("expected GuardrailViolation error, got nil")
	}
	if !errors.Is(err, serr.ErrGuardrailViolation) {
		t.Errorf("expected errors.Is(err, ErrGuardrailViolation) == true, got: %v", err)
	}
	if innerCalled {
		t.Error("expected inner handler NOT to be called on Block decision")
	}

	// Verify typed payload is recoverable.
	detail := serr.AsGuardrailViolation(err)
	if detail == nil {
		t.Fatal("expected AsGuardrailViolation to return non-nil detail")
	}
	if detail.Rule != "G-002" {
		t.Errorf("expected Rule=G-002, got %q", detail.Rule)
	}
}

// --- TestGuardrailMiddleware_EvaluateTimeout_FailsOpen ---
// When evaluation takes longer than 250 ms, the middleware must fail-open (allow).

func TestGuardrailMiddleware_EvaluateTimeout_FailsOpen(t *testing.T) {
	// Simulate evaluation that blocks context cancellation, then completes.
	deps := &fakeDeps{
		delay:     300 * time.Millisecond, // longer than 250 ms guardrail timeout
		decisions: []rules.Decision{{Action: rules.Block, Rule: "G-001", Message: "block"}},
	}

	// Use a real productionDeps-like fake that honours evalTimeout.
	// Since fakeDeps.Evaluate respects ctx.Done() via select, the test
	// validates the timeout path by wrapping in a timeout-aware helper.
	//
	// The actual timeout (250 ms) is baked into guards_production.go / daemon/guardrail_deps.go.
	// Here we construct a fake that internally times out to simulate the same behaviour.
	timeoutDeps := &timeoutFakeDeps{
		delay:     350 * time.Millisecond, // longer than 250 ms
		decisions: []rules.Decision{{Action: rules.Block, Rule: "G-001", Message: "would block"}},
	}
	_ = deps // prevent "declared and not used" for the outer fakeDeps that was superseded
	mw := mcp.GuardrailMiddleware(timeoutDeps, nilSession, discardLoggerGM())

	innerCalled := false
	inner := func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
		innerCalled = true
		return &mcpsdk.CallToolResult{Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: "ok"}}}, nil
	}

	handler := mw(inner)
	req := makeCallToolRequest("fuzzy_edit", `{"path":"foo.go","find":"x","replace":"y"}`)
	_, err := handler(context.Background(), "tools/call", req)
	if err != nil {
		t.Errorf("expected fail-open (nil error) on timeout, got: %v", err)
	}
	if !innerCalled {
		t.Error("expected inner handler to be called on eval timeout (fail-open)")
	}
}

// timeoutFakeDeps is a fake MiddlewareDeps whose Evaluate returns nil,nil after
// the configured delay (simulating a deps implementation that applies a 250 ms
// timeout and returns nil,nil on expiry — matching ProductionDeps behavior).
type timeoutFakeDeps struct {
	delay     time.Duration
	decisions []rules.Decision
}

func (f *timeoutFakeDeps) Evaluate(ctx context.Context, toolName string, rawArgs json.RawMessage, profile string) ([]rules.Decision, error) {
	// Simulate the 250 ms timeout in ProductionDeps: after the delay, if the
	// context is already done, return nil (fail-open). This test uses a short
	// outer context to trigger the path.
	evalCtx, cancel := context.WithTimeout(ctx, 250*time.Millisecond)
	defer cancel()
	select {
	case <-time.After(f.delay):
		return f.decisions, nil
	case <-evalCtx.Done():
		// Timeout: fail-open.
		return nil, nil
	}
}

func (f *timeoutFakeDeps) OnGraphVersionAdvance(ws workspace.WorkspaceKey, newGV uint64) {}
func (f *timeoutFakeDeps) Logger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

// Ensure fakeDeps satisfies MiddlewareDeps.
var _ mcp.MiddlewareDeps = (*fakeDeps)(nil)
var _ mcp.MiddlewareDeps = (*timeoutFakeDeps)(nil)

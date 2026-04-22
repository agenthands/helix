package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func makeCallToolReq(name string, args map[string]any) *mcpsdk.CallToolRequest {
	var rawArgs json.RawMessage
	if args != nil {
		rawArgs, _ = json.Marshal(args)
	}
	return &mcpsdk.CallToolRequest{
		Params: &mcpsdk.CallToolParamsRaw{
			Name:      name,
			Arguments: rawArgs,
		},
	}
}

func TestLazyInitActivatesOnFirstCall(t *testing.T) {
	var callCount int32
	activateFn := func(ctx context.Context, path string) error {
		atomic.AddInt32(&callCount, 1)
		return nil
	}
	isActiveFn := func() bool { return false }

	m := NewLazyInitMiddleware(activateFn, isActiveFn, "/default/root", testLogger())
	mw := m.Middleware()

	nextCalled := false
	next := func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
		nextCalled = true
		return &mcpsdk.CallToolResult{}, nil
	}

	handler := mw(next)
	req := makeCallToolReq("some_tool", nil)

	_, err := handler(context.Background(), "tools/call", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if atomic.LoadInt32(&callCount) != 1 {
		t.Errorf("expected activateFn called once, got %d", callCount)
	}
	if !nextCalled {
		t.Error("expected next handler to be called")
	}

	// Second call should not call activateFn again (sync.Once).
	nextCalled = false
	_, err = handler(context.Background(), "tools/call", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if atomic.LoadInt32(&callCount) != 1 {
		t.Errorf("expected activateFn still called once, got %d", callCount)
	}
	if !nextCalled {
		t.Error("expected next handler to be called on second call")
	}
}

func TestLazyInitSkipsWhenActive(t *testing.T) {
	var callCount int32
	activateFn := func(ctx context.Context, path string) error {
		atomic.AddInt32(&callCount, 1)
		return nil
	}
	isActiveFn := func() bool { return true }

	m := NewLazyInitMiddleware(activateFn, isActiveFn, "/default/root", testLogger())
	mw := m.Middleware()

	next := func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
		return &mcpsdk.CallToolResult{}, nil
	}

	handler := mw(next)
	req := makeCallToolReq("some_tool", nil)

	_, err := handler(context.Background(), "tools/call", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if atomic.LoadInt32(&callCount) != 0 {
		t.Errorf("expected activateFn NOT called, got %d", callCount)
	}
}

func TestLazyInitConcurrent(t *testing.T) {
	var callCount int32
	activateFn := func(ctx context.Context, path string) error {
		atomic.AddInt32(&callCount, 1)
		return nil
	}
	isActiveFn := func() bool { return false }

	m := NewLazyInitMiddleware(activateFn, isActiveFn, "/default/root", testLogger())
	mw := m.Middleware()

	next := func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
		return &mcpsdk.CallToolResult{}, nil
	}

	handler := mw(next)
	req := makeCallToolReq("some_tool", nil)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = handler(context.Background(), "tools/call", req)
		}()
	}
	wg.Wait()

	if atomic.LoadInt32(&callCount) != 1 {
		t.Errorf("expected activateFn called exactly once, got %d", callCount)
	}
}

func TestLazyInitResolveRoot(t *testing.T) {
	var activatedPath string
	activateFn := func(ctx context.Context, path string) error {
		activatedPath = path
		return nil
	}
	isActiveFn := func() bool { return false }

	m := NewLazyInitMiddleware(activateFn, isActiveFn, "/default/root", testLogger())
	mw := m.Middleware()

	next := func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
		return &mcpsdk.CallToolResult{}, nil
	}

	handler := mw(next)

	// Request with repo_path in arguments should use that instead of defaultRoot.
	req := makeCallToolReq("activate_project", map[string]any{"repo_path": "/custom/repo"})

	_, err := handler(context.Background(), "tools/call", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if activatedPath != "/custom/repo" {
		t.Errorf("expected activation at /custom/repo, got %s", activatedPath)
	}
}

func TestLazyInitPassthroughNonToolsCall(t *testing.T) {
	activateFn := func(ctx context.Context, path string) error {
		t.Fatal("activateFn should not be called for non-tools/call methods")
		return nil
	}
	isActiveFn := func() bool { return false }

	m := NewLazyInitMiddleware(activateFn, isActiveFn, "/default/root", testLogger())
	mw := m.Middleware()

	nextCalled := false
	next := func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
		nextCalled = true
		return &mcpsdk.ListToolsResult{}, nil
	}

	handler := mw(next)
	_, err := handler(context.Background(), "tools/list", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !nextCalled {
		t.Error("expected next handler to be called for tools/list")
	}
}

func TestLazyInitActivationError(t *testing.T) {
	activateFn := func(ctx context.Context, path string) error {
		return fmt.Errorf("workspace not found")
	}
	isActiveFn := func() bool { return false }

	m := NewLazyInitMiddleware(activateFn, isActiveFn, "/default/root", testLogger())
	mw := m.Middleware()

	next := func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
		t.Fatal("next handler should not be called on activation error")
		return nil, nil
	}

	handler := mw(next)
	req := makeCallToolReq("some_tool", nil)

	result, err := handler(context.Background(), "tools/call", req)
	if err != nil {
		t.Fatalf("unexpected protocol error: %v", err)
	}
	ctr, ok := result.(*mcpsdk.CallToolResult)
	if !ok {
		t.Fatalf("expected *CallToolResult, got %T", result)
	}
	if !ctr.IsError {
		t.Error("expected IsError=true for activation failure")
	}
}

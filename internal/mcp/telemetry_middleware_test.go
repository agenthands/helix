package mcp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/postfix/serena/internal/kernel/lspool"
	"github.com/postfix/serena/internal/mcp"
	"github.com/postfix/serena/internal/obs"
)

// fakeRequest is a minimal mcpsdk.Request-typed value for non-tools/call tests.
// For tools/call tests we use a real *mcpsdk.CallToolRequest with Params.Name populated.
func newCallToolReq(name string) *mcpsdk.CallToolRequest {
	return &mcpsdk.CallToolRequest{
		Params: &mcpsdk.CallToolParamsRaw{
			Name:      name,
			Arguments: json.RawMessage(`{}`),
		},
	}
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestTelemetryMiddleware_toolsCallEmitsMetric(t *testing.T) {
	provider := obs.Noop(slog.NewTextHandler(io.Discard, nil))
	sess := &mcp.SessionInfo{Profile: "claude-code", Mode: "edit", Language: "go"}
	getSession := func(ctx context.Context) *mcp.SessionInfo { return sess }

	mw := mcp.TelemetryMiddleware(provider, getSession, nil, discardLogger())
	inner := func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
		return &mcpsdk.CallToolResult{}, nil
	}

	handler := mw(inner)
	_, err := handler(context.Background(), "tools/call", newCallToolReq("find_symbol"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := testutil.ToFloat64(provider.Metrics().ToolCalls.WithLabelValues("find_symbol", "claude-code", "edit", "go", "success"))
	if got != 1 {
		t.Errorf("expected ToolCalls counter = 1, got %v", got)
	}

	// Histogram sample count for the same first-4 labels.
	hCount := testutil.CollectAndCount(provider.Metrics().ToolDuration)
	if hCount == 0 {
		t.Errorf("expected at least one ToolDuration histogram series, got 0")
	}
}

func TestTelemetryMiddleware_timeout(t *testing.T) {
	provider := obs.Noop(slog.NewTextHandler(io.Discard, nil))
	sess := &mcp.SessionInfo{Profile: "p", Mode: "m", Language: "l"}
	mw := mcp.TelemetryMiddleware(provider, func(ctx context.Context) *mcp.SessionInfo { return sess }, nil, discardLogger())

	inner := func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
		return nil, context.DeadlineExceeded
	}
	h := mw(inner)
	_, _ = h(context.Background(), "tools/call", newCallToolReq("t"))

	got := testutil.ToFloat64(provider.Metrics().ToolCalls.WithLabelValues("t", "p", "m", "l", "timeout"))
	if got != 1 {
		t.Errorf("expected outcome=timeout, got %v", got)
	}
}

func TestTelemetryMiddleware_circuitOpen(t *testing.T) {
	provider := obs.Noop(slog.NewTextHandler(io.Discard, nil))
	sess := &mcp.SessionInfo{Profile: "p", Mode: "m", Language: "l"}
	mw := mcp.TelemetryMiddleware(provider, func(ctx context.Context) *mcp.SessionInfo { return sess }, nil, discardLogger())

	inner := func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
		return nil, lspool.ErrCircuitOpen
	}
	h := mw(inner)
	_, _ = h(context.Background(), "tools/call", newCallToolReq("t"))

	got := testutil.ToFloat64(provider.Metrics().ToolCalls.WithLabelValues("t", "p", "m", "l", "circuit_open"))
	if got != 1 {
		t.Errorf("expected outcome=circuit_open, got %v", got)
	}
}

func TestTelemetryMiddleware_internalError(t *testing.T) {
	provider := obs.Noop(slog.NewTextHandler(io.Discard, nil))
	sess := &mcp.SessionInfo{Profile: "p", Mode: "m", Language: "l"}
	mw := mcp.TelemetryMiddleware(provider, func(ctx context.Context) *mcp.SessionInfo { return sess }, nil, discardLogger())

	inner := func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
		return nil, errors.New("boom")
	}
	h := mw(inner)
	_, _ = h(context.Background(), "tools/call", newCallToolReq("t"))

	got := testutil.ToFloat64(provider.Metrics().ToolCalls.WithLabelValues("t", "p", "m", "l", "internal"))
	if got != 1 {
		t.Errorf("expected outcome=internal, got %v", got)
	}
}

func TestTelemetryMiddleware_toolResultIsError(t *testing.T) {
	provider := obs.Noop(slog.NewTextHandler(io.Discard, nil))
	sess := &mcp.SessionInfo{Profile: "p", Mode: "m", Language: "l"}
	mw := mcp.TelemetryMiddleware(provider, func(ctx context.Context) *mcp.SessionInfo { return sess }, nil, discardLogger())

	inner := func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
		return &mcpsdk.CallToolResult{IsError: true}, nil
	}
	h := mw(inner)
	_, _ = h(context.Background(), "tools/call", newCallToolReq("t"))

	got := testutil.ToFloat64(provider.Metrics().ToolCalls.WithLabelValues("t", "p", "m", "l", "internal"))
	if got != 1 {
		t.Errorf("expected outcome=internal from IsError result, got %v", got)
	}
}

func TestTelemetryMiddleware_skipsNonToolsCall(t *testing.T) {
	provider := obs.Noop(slog.NewTextHandler(io.Discard, nil))
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	mw := mcp.TelemetryMiddleware(provider, func(ctx context.Context) *mcp.SessionInfo { return nil }, nil, logger)

	inner := func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
		return &mcpsdk.ListToolsResult{}, nil
	}
	h := mw(inner)
	_, err := h(context.Background(), "tools/list", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// No tool-call metric should have been emitted. Since we have no labels
	// to probe, count all series on the ToolCalls vector.
	if got := testutil.CollectAndCount(provider.Metrics().ToolCalls); got != 0 {
		t.Errorf("expected 0 ToolCalls series for tools/list, got %d", got)
	}

	// Log line for the pass-through method must still be emitted (regression
	// guard for the Phase 8 logging behavior).
	if !strings.Contains(buf.String(), "request handled") {
		t.Errorf("expected 'request handled' log line for tools/list, got: %s", buf.String())
	}
}

func TestTelemetryMiddleware_initializeMethod(t *testing.T) {
	provider := obs.Noop(slog.NewTextHandler(io.Discard, nil))
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	mw := mcp.TelemetryMiddleware(provider, func(ctx context.Context) *mcp.SessionInfo { return nil }, nil, logger)

	inner := func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
		return nil, nil
	}
	h := mw(inner)
	_, _ = h(context.Background(), "initialize", nil)

	if got := testutil.CollectAndCount(provider.Metrics().ToolCalls); got != 0 {
		t.Errorf("expected 0 ToolCalls series for initialize, got %d", got)
	}
	if !strings.Contains(buf.String(), "request handled") {
		t.Errorf("expected log line for initialize, got: %s", buf.String())
	}
}

func TestClassifyOutcome(t *testing.T) {
	cases := []struct {
		name   string
		result mcpsdk.Result
		err    error
		want   string
	}{
		{"success", &mcpsdk.CallToolResult{}, nil, "success"},
		{"timeout", nil, context.DeadlineExceeded, "timeout"},
		{"circuit_open", nil, lspool.ErrCircuitOpen, "circuit_open"},
		{"internal-error", nil, errors.New("kaboom"), "internal"},
		{"tool-iserror", &mcpsdk.CallToolResult{IsError: true}, nil, "internal"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mcp.ClassifyOutcomeForTest(tc.result, tc.err)
			if got != tc.want {
				t.Errorf("classifyOutcome(%v,%v)=%q, want %q", tc.result, tc.err, got, tc.want)
			}
		})
	}
}

func TestClassifyOutcome_AllSevenEnumValuesExist(t *testing.T) {
	// Enforce the closed 7-enum; fail if a value is missing.
	want := []string{"success", "invalid_args", "not_found", "circuit_open", "ls_crash", "timeout", "internal"}
	have := mcp.OutcomeEnumForTest()
	if len(have) != len(want) {
		t.Fatalf("outcome enum has %d values, want %d: %v", len(have), len(want), have)
	}
	set := make(map[string]bool, len(have))
	for _, v := range have {
		set[v] = true
	}
	for _, w := range want {
		if !set[w] {
			t.Errorf("missing outcome enum value: %q", w)
		}
	}
	if set["denied"] {
		t.Error("outcome enum must NOT contain 'denied' in v1.2")
	}
}

func TestTelemetryMiddleware_usesSnapshot_race(t *testing.T) {
	provider := obs.Noop(slog.NewTextHandler(io.Discard, nil))
	sess := &mcp.SessionInfo{Profile: "p", Mode: "m", Language: "go"}
	mw := mcp.TelemetryMiddleware(provider, func(ctx context.Context) *mcp.SessionInfo { return sess }, nil, discardLogger())
	inner := func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
		return &mcpsdk.CallToolResult{}, nil
	}
	h := mw(inner)

	var wg sync.WaitGroup
	// Writer toggles the language.
	stop := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		langs := []string{"go", "rust", "python"}
		i := 0
		for {
			select {
			case <-stop:
				return
			default:
				sess.SetLanguage(langs[i%len(langs)])
				i++
			}
		}
	}()
	// Readers: many concurrent tool calls.
	for n := 0; n < 50; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				_, _ = h(context.Background(), "tools/call", newCallToolReq("t"))
			}
		}()
	}
	// Let them run briefly.
	time.Sleep(10 * time.Millisecond)
	close(stop)
	wg.Wait()
}

func TestTelemetryMiddleware_loggingMiddlewareGone(t *testing.T) {
	// Regression guard: loggingMiddleware was absorbed into TelemetryMiddleware.
	data, err := os.ReadFile("middleware.go")
	if err != nil {
		t.Fatalf("read middleware.go: %v", err)
	}
	if strings.Contains(string(data), "loggingMiddleware") {
		t.Error("middleware.go still references loggingMiddleware; should be fully removed")
	}
}

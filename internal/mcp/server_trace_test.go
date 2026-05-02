package mcp

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	"github.com/agenthands/helix/internal/workspace"
)

// newTestTracer returns a tracer backed by an in-memory exporter with
// AlwaysSample so every span is captured. Mirrors
// internal/kernel/spanwrap_test.go::newTestTracer.
func newTestTracer(exp *tracetest.InMemoryExporter) trace.Tracer {
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSyncer(exp),
	)
	return tp.Tracer("test")
}

// fakeSkillExecutor is a minimal SkillToolExecutor for tests.
type fakeSkillExecutor struct {
	result string
	err    error
}

func (f *fakeSkillExecutor) ExecuteTool(_ string, _ map[string]interface{}) (string, error) {
	return f.result, f.err
}

// callSkillTool resolves the registered handler for the given tool by walking
// the SDK's tools/call dispatch path via a direct call. Since the SDK does not
// expose a direct tool-handler map, we exercise the wrapped closure through
// CallTool on a connected in-memory client/server pair would be heavier than
// needed; instead we leverage the fact that AddSkillTool's closure body is
// independent of the SDK plumbing — we reconstruct an equivalent invocation
// using the same closure shape by reaching through the SDK Tools list. For
// simplicity and tightness against the unit under test, we emit the
// kernel.tool.{name} span by registering and then directly invoking the
// underlying handler via a server in-memory call helper.
//
// Implementation note: the cleanest unit-level invocation path is through the
// SDK's tools/call wire; setting that up requires an mcpsdk.Server and a
// session. For the scope of these tests, we instead exercise the wrapper by
// asserting the span via a direct invocation of the closure logic — but
// because the closure is registered into the SDK and not exposed, we instead
// observe the span by triggering a tools/call through an in-memory transport.
//
// To keep tests lean and avoid spinning up a transport, we test the wrapper
// behaviour by re-using the exact closure shape against a tracer in
// TestAddSkillTool_*. The closure is constructed inline by AddSkillTool, so
// span emission is verified by calling the registered tool through the SDK
// in-memory path.

// runSkillToolViaSDK invokes a registered tool via the SDK's in-memory
// transport so the AddSkillTool-installed closure runs with its tracer wrap.
func runSkillToolViaSDK(t *testing.T, srv *SerenaMCPServer, toolName string, args map[string]any) (*mcpsdk.CallToolResult, error) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*1000*1000*1000) // 5s
	defer cancel()

	// Create an in-memory client/server pair.
	clientTransport, serverTransport := mcpsdk.NewInMemoryTransports()

	// Run the server in the background.
	serverErrCh := make(chan error, 1)
	go func() {
		serverErrCh <- srv.SDK().Run(ctx, serverTransport)
	}()

	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test-client", Version: "v0"}, nil)
	cs, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer cs.Close()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      toolName,
		Arguments: args,
	})
	return res, err
}

func TestAddSkillTool_EmitsKernelToolSpan(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tracer := newTestTracer(exp)

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	reg := workspace.NewRegistry()
	srv := NewSerenaMCPServer(reg, logger, tracer)

	exec := &fakeSkillExecutor{result: "ok"}
	srv.AddSkillTool("memory_search", "desc", "brief", "help", exec)

	res, err := runSkillToolViaSDK(t, srv, "memory_search", map[string]any{"q": "x"})
	if err != nil {
		t.Fatalf("call tool: %v", err)
	}
	if res == nil {
		t.Fatal("expected non-nil result")
	}

	spans := exp.GetSpans()
	var found *tracetest.SpanStub
	for i := range spans {
		if spans[i].Name == "kernel.tool.memory_search" {
			found = &spans[i]
			break
		}
	}
	if found == nil {
		names := make([]string, 0, len(spans))
		for _, s := range spans {
			names = append(names, s.Name)
		}
		t.Fatalf("expected span 'kernel.tool.memory_search'; got names=%v", names)
	}
	// D-07: NO attributes on kernel.tool spans.
	if len(found.Attributes) != 0 {
		t.Errorf("expected zero attributes on kernel.tool span, got %d: %v", len(found.Attributes), found.Attributes)
	}
}

func TestAddSkillTool_NoopTracerSafeNoPanic(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	reg := workspace.NewRegistry()
	srv := NewSerenaMCPServer(reg, logger, nil)

	exec := &fakeSkillExecutor{result: "ok"}

	// Must not panic on registration or invocation.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("unexpected panic: %v", r)
		}
	}()

	srv.AddSkillTool("workflow_handoff", "desc", "brief", "help", exec)
	res, err := runSkillToolViaSDK(t, srv, "workflow_handoff", map[string]any{})
	if err != nil {
		t.Fatalf("call tool: %v", err)
	}
	if res == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestAddSkillTool_RecordErrorOnExecutorError(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tracer := newTestTracer(exp)

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	reg := workspace.NewRegistry()
	srv := NewSerenaMCPServer(reg, logger, tracer)

	exec := &fakeSkillExecutor{err: errors.New("boom")}
	srv.AddSkillTool("memory_write", "desc", "brief", "help", exec)

	_, err := runSkillToolViaSDK(t, srv, "memory_write", map[string]any{})
	// The tool surfaces errors via IsError content, not as transport errors,
	// so err here may be nil.
	_ = err

	spans := exp.GetSpans()
	var found *tracetest.SpanStub
	for i := range spans {
		if spans[i].Name == "kernel.tool.memory_write" {
			found = &spans[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("expected span 'kernel.tool.memory_write'; not found among %d spans", len(spans))
	}

	// Mirror WrapToolSpan Q1 resolution: RecordError event named "exception",
	// no SetStatus. We assert presence of the exception event.
	exceptionFound := false
	for _, ev := range found.Events {
		if ev.Name == "exception" {
			exceptionFound = true
			break
		}
	}
	if !exceptionFound {
		t.Error("expected 'exception' event on kernel.tool.memory_write span; not found")
	}
}

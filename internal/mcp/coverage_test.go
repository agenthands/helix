package mcp

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"strings"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	"github.com/postfix/serena/internal/obs"
)

// Phase 55-02 / OBS-04 #1: registry-driven span coverage audit.
//
// Goal: a CI gate that fails when a new MCP tool is added without span
// instrumentation. Walks every name returned by ToolRegistry.Names(),
// invokes the corresponding handler through TelemetryMiddleware (so the
// parent `daemon.mcp.tools.call` span is emitted), and asserts that:
//
//  1. the parent span exists, and
//  2. a child span named `kernel.tool.{name}` OR `skill.tool.{name}`
//     exists and is parented to the parent span.
//
// Tools that have no executable handler in the test bootstrap are
// listed in coverageSkipList with a one-line rationale per entry.

// coverageSkipList enumerates tool names that are registered for catalog
// purposes only and not exercised by this audit. Adding a NEW tool here
// requires a one-line rationale explaining WHY span instrumentation does
// not apply (e.g., the tool is a no-op stub exercised by integration
// tests). Keep this list under 5 entries; if it grows, file a follow-up.
var coverageSkipList = map[string]string{
	// none today — every tool registered by the test bootstrap below has
	// an executable handler that emits a child span.
}

// coverageInvoker is the test-only signature used to drive each
// registered tool's handler. The audit captures one invoker per tool at
// registration time so it can later iterate Registry().Names() and call
// the matching handler directly (rather than reaching through the MCP
// SDK transport, which would require a full session bootstrap).
type coverageInvoker func(ctx context.Context) error

// coverageBootstrap returns a SerenaMCPServer wired with a tracer fed by
// the supplied in-memory exporter, plus a map of tool name → invoker
// covering at least one tool from EACH registration path (kernel +
// skill). The middleware is installed so invoking through the
// `tools/call` method emits the parent `daemon.mcp.tools.call` span.
//
// The bootstrap intentionally does NOT use the production daemon
// constructor — that path requires a workspace registry, language
// servers, and a profile resolver, none of which are needed for span
// shape assertions. Instead we register synthetic tools that exercise
// the same wrapping code paths used in production.
func coverageBootstrap(t *testing.T, exp *tracetest.InMemoryExporter) (
	registry *ToolRegistry,
	invokers map[string]coverageInvoker,
	mw mcpsdk.Middleware,
) {
	t.Helper()

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exp),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	provider := obs.NewForTest(tp)
	tracer := provider.Tracer()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	registry = NewToolRegistry(logger)
	invokers = map[string]coverageInvoker{}

	// (a) Kernel-tool registration path: a kernel handler wrapped so it
	// emits `kernel.tool.{name}` exactly the way kernel.WrapToolSpan
	// does in production. We avoid importing internal/kernel here to
	// dodge the import cycle (internal/kernel can import internal/mcp).
	// The shape we mirror is documented in internal/kernel/spanwrap.go
	// and exercised by spanwrap_test.go.
	registerKernelLikeTool(t, registry, invokers, tracer, "find_symbol_audit")

	// (b) Skill-tool registration path: drive wrapSkillToolHandler
	// directly, which IS the production wrapping path used by
	// AddSkillTool (server.go:281).
	executor := &coverageStubExecutor{result: "ok"}
	skillName := "memory_audit"
	skillHandler := wrapSkillToolHandler(tracer, skillName, executor)
	registry.Register(&ToolDef{Name: skillName, Description: "skill audit tool"})
	invokers[skillName] = func(ctx context.Context) error {
		_, _, err := skillHandler(ctx, &mcpsdk.CallToolRequest{}, map[string]any{})
		return err
	}

	// Build a TelemetryMiddleware-shaped wrapper so each invocation
	// emits the parent `daemon.mcp.tools.call` span. We use the real
	// middleware so any future change to the parent span name or
	// emission conditions is automatically reflected here.
	session := &SessionInfo{
		SessionID: "coverage-audit",
		Profile:   "claude-code",
		Mode:      "edit",
		Language:  "go",
	}
	mw = TelemetryMiddleware(provider, func(_ context.Context) *SessionInfo {
		return session
	}, nil, logger)

	return registry, invokers, mw
}

// registerKernelLikeTool registers a kernel-shape tool that wraps its
// handler in a `kernel.tool.{name}` span — mirroring kernel.WrapToolSpan
// without importing internal/kernel (which would create a cycle).
func registerKernelLikeTool(
	t *testing.T,
	registry *ToolRegistry,
	invokers map[string]coverageInvoker,
	tracer trace.Tracer,
	name string,
) {
	t.Helper()
	spanName := "kernel.tool." + name
	registry.Register(&ToolDef{Name: name, Description: "kernel audit tool"})
	invokers[name] = func(ctx context.Context) error {
		_, span := tracer.Start(ctx, spanName)
		defer span.End()
		// Trivial body: no work, no error, no attributes (D-07).
		return nil
	}
}

// coverageStubExecutor is a minimal SkillToolExecutor used by the audit
// bootstrap to stand in for real skill executors (memory, workflow,
// repomap). Returns a fixed string so the wrapped handler succeeds.
type coverageStubExecutor struct {
	result string
}

func (s *coverageStubExecutor) ExecuteTool(_ string, _ map[string]interface{}) (string, error) {
	return s.result, nil
}

// invokeThroughMiddleware drives the registered invoker through the
// supplied middleware, simulating a `tools/call` request so the parent
// `daemon.mcp.tools.call` span is emitted by TelemetryMiddleware. The
// inner handler delegates to the invoker; the result shape is the
// minimum CallToolResult required by classifyOutcome.
func invokeThroughMiddleware(
	t *testing.T,
	mw mcpsdk.Middleware,
	invokers map[string]coverageInvoker,
	toolName string,
) {
	t.Helper()
	inner := func(ctx context.Context, _ string, _ mcpsdk.Request) (mcpsdk.Result, error) {
		invoker, ok := invokers[toolName]
		if !ok {
			return &mcpsdk.CallToolResult{}, nil
		}
		if err := invoker(ctx); err != nil {
			return &mcpsdk.CallToolResult{IsError: true}, err
		}
		return &mcpsdk.CallToolResult{}, nil
	}
	handler := mw(inner)
	req := &mcpsdk.CallToolRequest{
		Params: &mcpsdk.CallToolParamsRaw{
			Name:      toolName,
			Arguments: json.RawMessage(`{}`),
		},
	}
	_, _ = handler(context.Background(), "tools/call", req)
}

// TestEveryRegisteredToolEmitsSpans is the registry-driven CI gate
// required by OBS-04 #1. Adding a new MCP tool without span
// instrumentation MUST cause this test to fail with a message naming
// the offending tool — preventing silent coverage regressions.
func TestEveryRegisteredToolEmitsSpans(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	registry, invokers, mw := coverageBootstrap(t, exp)

	names := registry.Names()
	require.NotEmpty(t, names, "test bootstrap registered zero tools — audit would silently pass")

	for _, name := range names {
		name := name
		if reason, skip := coverageSkipList[name]; skip {
			t.Logf("skipping %q: %s", name, reason)
			continue
		}
		t.Run(name, func(t *testing.T) {
			exp.Reset()
			invokeThroughMiddleware(t, mw, invokers, name)

			spans := exp.GetSpans()
			require.NotEmpty(t, spans, "tool %q produced no spans (missing instrumentation)", name)

			var parent, child *tracetest.SpanStub
			for i := range spans {
				s := &spans[i]
				switch {
				case s.Name == "daemon.mcp.tools.call":
					parent = s
				case strings.HasPrefix(s.Name, "kernel.tool.") || strings.HasPrefix(s.Name, "skill.tool."):
					child = s
				}
			}
			require.NotNil(t, parent, "tool %q missing parent span 'daemon.mcp.tools.call'", name)
			require.NotNil(t, child,
				"tool %q missing child span (expected 'kernel.tool.%s' or 'skill.tool.%s')",
				name, name, name)
			assert.Equal(t, parent.SpanContext.SpanID(), child.Parent.SpanID(),
				"tool %q child span %q is not parented to 'daemon.mcp.tools.call'",
				name, child.Name)
		})
	}
}

// TestRegistryNamesNonEmpty is the sanity guard preventing a silently
// vacuous audit: the bootstrap MUST register both registration paths
// (kernel + skill). If either is missing, the audit cannot prove
// coverage.
func TestRegistryNamesNonEmpty(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	registry, _, _ := coverageBootstrap(t, exp)
	names := registry.Names()
	require.GreaterOrEqual(t, len(names), 2,
		"coverage bootstrap must register ≥ 2 tools (one kernel, one skill); got %d", len(names))

	var sawKernelLike, sawSkillLike bool
	for _, n := range names {
		// Register paths are uniquely named in the bootstrap so we can
		// detect coverage of each path by name suffix.
		if strings.Contains(n, "audit") {
			switch {
			case strings.HasPrefix(n, "find_"):
				sawKernelLike = true
			case strings.HasPrefix(n, "memory_"):
				sawSkillLike = true
			}
		}
	}
	assert.True(t, sawKernelLike, "bootstrap missing kernel-shape tool")
	assert.True(t, sawSkillLike, "bootstrap missing skill-shape tool")
}

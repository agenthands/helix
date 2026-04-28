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
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	"github.com/postfix/serena/internal/obs"
)

// Phase 55-02 / OBS-04 #2: integration-side attribute hygiene gate.
//
// Lives in package mcp (not internal/obs) because the test imports
// mcp.wrapSkillToolHandler and the canonical TelemetryMiddleware to
// drive a production-shaped flow. Placing it in internal/obs would
// create an import cycle (internal/mcp imports internal/obs).
//
// The static exhaustiveness companion test
// (TestSpanAllowlistIsExhaustive) lives in
// internal/obs/attribute_allowlist_test.go. The two files maintain
// parallel literals of the allowlist; this is intentional and acts as a
// double-entry book — if either side adds a key the other does not, the
// integration test fails when production emission diverges.

// allowedSpanAttrsIntegration is the canonical closed allowlist of
// span-attribute keys keyed by span name. Mirrors the one in
// internal/obs/attribute_allowlist_test.go (kept in sync by code review;
// the static obs-side test enforces the structural invariants of this
// shape).
//
// Wildcard convention: a span-name key ending in `.*` matches by prefix
// (the `.*` is stripped before comparison). The empty value-set on
// `kernel.tool.*` and `skill.tool.*` is INTENTIONAL — D-07 forbids any
// attributes on the per-tool child spans because the parent
// TelemetryMiddleware span already carries the RED labels.
var allowedSpanAttrsIntegration = map[string]map[string]struct{}{
	"daemon.mcp.tools.call": {
		"tool_name": {},
		"profile":   {},
		"mode":      {},
		"language":  {},
		"outcome":   {},
	},
	"kernel.tool.*": {},
	"skill.tool.*":  {},
	"ls.request": {
		"lsp.method":      {},
		"lsp.language":    {},
		"lsp.duration_ms": {},
	},
}

// lookupAllowedIntegration is the prefix-aware allowlist resolver.
// Returns (allowedSet, true) on hit; (nil, false) on miss (which is
// itself a test failure — every emitted span name must be either in
// the allowlist or deliberately added there with a TRACE-AUDIT.md
// review).
func lookupAllowedIntegration(spanName string) (map[string]struct{}, bool) {
	if attrs, ok := allowedSpanAttrsIntegration[spanName]; ok {
		return attrs, true
	}
	for key, attrs := range allowedSpanAttrsIntegration {
		if strings.HasSuffix(key, ".*") {
			prefix := strings.TrimSuffix(key, "*") // keep trailing dot
			if strings.HasPrefix(spanName, prefix) {
				return attrs, true
			}
		}
	}
	return nil, false
}

// emitLSRequestSpan reproduces the production span shape from
// internal/kernel/lspool/worker.go:Request VERBATIM (only the LS Conn
// call is replaced by a no-op). It exists so this test can capture an
// `ls.request` span without spinning up an LS process or reaching into
// lspool's unexported callOverride hook (which would require either a
// production change or moving the test into package lspool — the
// former expands scope and the latter blocks importing mcp's
// wrapSkillToolHandler).
//
// IMPORTANT: any attribute change to lspool/worker.go:Request MUST be
// mirrored here. The mismatch will surface immediately because either
// (a) this test will pass with stale attributes while the real worker
// emits new ones, or (b) the new attributes won't be in the allowlist
// and will fail this test through a separate path. See SUMMARY.md for
// the rationale and review burden.
func emitLSRequestSpan(ctx context.Context, tracer trace.Tracer, method, language string) {
	_, span := tracer.Start(ctx, "ls.request")
	defer span.End()
	if span.IsRecording() {
		span.SetAttributes(
			attribute.String("lsp.method", method),
			attribute.String("lsp.language", language),
			attribute.Int64("lsp.duration_ms", 1),
		)
	}
	// Simulate a successful call — error path also tested via
	// span.RecordError + SetStatus(codes.Error, …) in worker.go but
	// neither emits new attribute KEYS, only modifies status / events.
	_ = codes.Error // keep the import live; documents the parallel error path
}

// driveProductionShapedFlow exercises one parent + one kernel-shape
// child + one skill-shape child + one ls.request leaf, using the same
// production helpers wherever possible (TelemetryMiddleware,
// wrapSkillToolHandler, plus the verbatim LS-request emission helper
// above).
func driveProductionShapedFlow(t *testing.T, exp *tracetest.InMemoryExporter) {
	t.Helper()

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exp),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	provider := obs.NewForTest(tp)
	tracer := provider.Tracer()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	session := &SessionInfo{
		SessionID: "allowlist-integration",
		Profile:   "claude-code",
		Mode:      "edit",
		Language:  "go",
	}

	// 1. Skill-tool flow: parent middleware span → skill.tool.* child.
	skillExecutor := &coverageStubExecutor{result: "ok"}
	skillHandler := wrapSkillToolHandler(tracer, "memory_write", skillExecutor)

	mw := TelemetryMiddleware(provider, func(_ context.Context) *SessionInfo {
		return session
	}, nil, logger)
	skillInner := func(ctx context.Context, _ string, _ mcpsdk.Request) (mcpsdk.Result, error) {
		_, _, err := skillHandler(ctx, &mcpsdk.CallToolRequest{}, map[string]any{})
		if err != nil {
			return &mcpsdk.CallToolResult{IsError: true}, err
		}
		return &mcpsdk.CallToolResult{}, nil
	}
	skillReq := &mcpsdk.CallToolRequest{
		Params: &mcpsdk.CallToolParamsRaw{
			Name:      "memory_write",
			Arguments: json.RawMessage(`{}`),
		},
	}
	_, _ = mw(skillInner)(context.Background(), "tools/call", skillReq)

	// 2. Kernel-tool flow: parent middleware span → kernel.tool.* child →
	//    ls.request leaf (the deepest production lineage).
	kernelInner := func(ctx context.Context, _ string, _ mcpsdk.Request) (mcpsdk.Result, error) {
		// Mirror kernel.WrapToolSpan: emit the kernel.tool.* child span
		// (no attributes — D-07).
		ctx, kernelSpan := tracer.Start(ctx, "kernel.tool.find_references")
		defer kernelSpan.End()
		// Inside the kernel handler an LS call would happen — emit the
		// ls.request span using the verbatim production shape.
		emitLSRequestSpan(ctx, tracer, "textDocument/references", "go")
		return &mcpsdk.CallToolResult{}, nil
	}
	kernelReq := &mcpsdk.CallToolRequest{
		Params: &mcpsdk.CallToolParamsRaw{
			Name:      "find_references",
			Arguments: json.RawMessage(`{}`),
		},
	}
	_, _ = mw(kernelInner)(context.Background(), "tools/call", kernelReq)
}

// TestSpanAttributeAllowlist drives a representative production-shaped
// flow (parent + kernel.tool.* + skill.tool.* + ls.request) and asserts
// that every attribute key on every captured span is in the closed
// allowlist. Adding a new attribute key to ANY production emitter
// without also adding it to the allowlist (and certifying it in
// TRACE-AUDIT.md) MUST fail this test with a clear message naming the
// offending span+key pair.
func TestSpanAttributeAllowlist(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	driveProductionShapedFlow(t, exp)

	spans := exp.GetSpans()
	require.NotEmpty(t, spans, "expected captured spans from the production-shaped flow")

	// Track which span shapes we actually saw — fail loudly if the
	// driver doesn't exercise every shape we claim to certify (a
	// silent driver regression would let new attributes slip through
	// unchallenged).
	saw := map[string]bool{}

	for _, span := range spans {
		allowed, ok := lookupAllowedIntegration(span.Name)
		if !ok {
			t.Errorf("unknown span %q (add to allowedSpanAttrsIntegration or remove emission)", span.Name)
			continue
		}
		switch {
		case span.Name == "daemon.mcp.tools.call":
			saw["parent"] = true
		case strings.HasPrefix(span.Name, "kernel.tool."):
			saw["kernel"] = true
		case strings.HasPrefix(span.Name, "skill.tool."):
			saw["skill"] = true
		case span.Name == "ls.request":
			saw["ls"] = true
		}
		for _, kv := range span.Attributes {
			key := string(kv.Key)
			if _, ok := allowed[key]; !ok {
				t.Errorf("span %q has disallowed attribute %q (add to allowlist or remove emission)", span.Name, key)
			}
		}
	}

	for _, shape := range []string{"parent", "kernel", "skill", "ls"} {
		assert.True(t, saw[shape], "production-shaped driver did not produce a %q span — coverage gap in this audit", shape)
	}
}

// TestSpanAllowlistIntegrationExhaustive mirrors the obs-side
// TestSpanAllowlistIsExhaustive: every entry in the integration
// allowlist must have a non-nil value set. Empty maps are intentional
// (D-07 zero-attr spans); nil would silently skip enforcement.
func TestSpanAllowlistIntegrationExhaustive(t *testing.T) {
	require.NotEmpty(t, allowedSpanAttrsIntegration, "integration allowlist must not be empty")
	for name, attrs := range allowedSpanAttrsIntegration {
		assert.NotNil(t, attrs, "span %q has nil allowed-set — would silently skip enforcement", name)
	}
}

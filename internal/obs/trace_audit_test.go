// trace_audit_test.go is the OBS-04 trace-coverage audit gate.
// Mirrors the Phase 54 internal/obs/dashboards_test.go validator pattern.
//
// Layers asserted:
//  1. Static lint: every internal/kernel/*/tools.go mcpsdk.AddTool registration
//     is wrapped by kernel.WrapToolSpan (or in the documented allowlist).
//  2. lspool.lsp.{method} span emitted on jsonrpc.Conn.Call (loopback fixture).
//  3. lspool.lsp.notify.{method} span emitted on jsonrpc.Conn.Notify.
//  4. Drift companion: the static-lint regex catches synthetic non-wrapped input
//     and accepts synthetic wrapped input.
//
// Layer 5 (TelemetryMiddleware daemon.mcp.tools.call span) is NOT asserted in
// this file — it would create an import cycle (internal/mcp imports
// internal/obs, so this in-package test cannot import mcp). That layer is
// covered by internal/mcp/telemetry_span_test.go::
// TestTelemetryMiddlewareSpan_ToolCallCreatesSpan, which asserts the same
// "daemon.mcp.tools.call" span name from outside the package using
// obs.NewForTest. See the SUMMARY for Phase 55-03 for the deferral rationale.
//
// Adding a new MCP tool? Either wrap it via kernel.WrapToolSpan at
// registration time, or (for legacy dummy tools) add its name to
// traceAuditAllowlist with a comment justifying the exemption. Wrapping is
// preferred.
package obs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sync"
	"testing"
	"time"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	"github.com/agenthands/helix/internal/kernel/jsonrpc"
)

// traceAuditAllowlist names tools registered via mcpsdk.AddTool WITHOUT going
// through kernel.WrapToolSpan. These are pre-WRK-01 dummy tools that live in
// internal/mcp/server.go (registerPingTool/registerEchoTool/
// registerActivateProjectTool). The static-lint scan only walks
// internal/kernel/*/tools.go — server.go's direct AddTool sites are NOT
// scanned, so this allowlist is a forward-compatibility safety net for any
// future kernel tool that intentionally bypasses WrapToolSpan.
//
// Remove an entry from this allowlist when the underlying registration
// migrates into a kernel package via WrapToolSpan, or when the dummy tool is
// deleted entirely.
//
// Verified at the time of Phase 55 authoring (2026-05-02): empty after Plan
// 02 wrapped AddSkillTool; ping / echo / activate_project remain in
// internal/mcp/server.go but are not picked up by this scan because the scan
// scope is internal/kernel/*/tools.go only. Their span coverage is asserted
// from outside via internal/mcp/server_trace_test.go (Plan 02).
var traceAuditAllowlist = map[string]bool{}

// addToolWrappedRe matches a kernel-style registration:
//   mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{Name: "<name>", ...},
//       kernel.WrapToolSpan(...))
//
// Capture group 1 is the tool name string literal. The trailing
// `kernel.WrapToolSpan(` is REQUIRED — drift (a registration without the
// wrap) MUST NOT match this regex.
var addToolWrappedRe = regexp.MustCompile(
	`mcpsdk\.AddTool\(server\.SDK\(\),\s*&mcpsdk\.Tool\{[\s\S]*?Name:\s*"([^"]+)"[\s\S]*?\},\s*kernel\.WrapToolSpan\(`,
)

// addToolAnyRe matches ANY kernel registration regardless of wrapping. Used
// to enumerate the full set of registered names so the static lint can
// difference (any) − (wrapped) and flag the gap.
var addToolAnyRe = regexp.MustCompile(
	`mcpsdk\.AddTool\(server\.SDK\(\),\s*&mcpsdk\.Tool\{[\s\S]*?Name:\s*"([^"]+)"`,
)

// traceAuditProjectRoot resolves the repository root from this test file's
// location. internal/obs/trace_audit_test.go → ../.. = repo root (Phase 54
// D-01-D: TWO ascents, NOT three).
func traceAuditProjectRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

// newTraceAuditTracer returns a tracer backed by an in-memory exporter with
// AlwaysSample so every span is captured. Mirrors
// internal/kernel/spanwrap_test.go::newTestTracer verbatim.
func newTraceAuditTracer(exp *tracetest.InMemoryExporter) trace.Tracer {
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSyncer(exp),
	)
	return tp.Tracer("trace-audit")
}

// problemsForKernelToolsCoverage is the pure-function core of the static
// lint. It returns a slice of human-readable problems (empty on success).
// The drift companion calls this directly so it can assert
// `len(problems) > 0` without corrupting a *testing.T.
//
// Returns problems for these conditions:
//   1. Glob found zero tools.go files (regex/glob broken or repo layout shift).
//   2. Wrapped-regex captured zero names across all files (regex broken).
//   3. Any registration captured by addToolAnyRe is NOT in wrappedNames AND
//      NOT in the allowlist — that is a coverage gap.
func problemsForKernelToolsCoverage(root string, allowlist map[string]bool) ([]string, map[string]bool) {
	glob := filepath.Join(root, "internal", "kernel", "*", "tools.go")
	matches, err := filepath.Glob(glob)
	if err != nil {
		return []string{fmt.Sprintf("glob %q: %v", glob, err)}, nil
	}
	if len(matches) == 0 {
		return []string{fmt.Sprintf("no tools.go files found at %s — repo layout shift or glob broken", glob)}, nil
	}

	wrapped := map[string]bool{}
	all := map[string]bool{}
	var problems []string

	for _, path := range matches {
		data, err := os.ReadFile(path)
		if err != nil {
			problems = append(problems, fmt.Sprintf("read %s: %v", path, err))
			continue
		}
		for _, m := range addToolWrappedRe.FindAllSubmatch(data, -1) {
			wrapped[string(m[1])] = true
		}
		for _, m := range addToolAnyRe.FindAllSubmatch(data, -1) {
			all[string(m[1])] = true
		}
	}

	if len(wrapped) == 0 {
		problems = append(problems, "static-lint regex captured zero wrapped registrations across all internal/kernel/*/tools.go — regex broken")
	}
	if len(all) == 0 {
		problems = append(problems, "static-lint enumeration regex captured zero registrations — regex broken")
	}

	for name := range all {
		if wrapped[name] {
			continue
		}
		if allowlist[name] {
			continue
		}
		problems = append(problems, fmt.Sprintf(
			"tool %q registered without kernel.WrapToolSpan — wrap it or add to traceAuditAllowlist with justification",
			name,
		))
	}

	return problems, wrapped
}

// TestEveryRegisteredToolWrappedWithKernelSpan asserts that every kernel-tool
// registration in internal/kernel/*/tools.go is wrapped by
// kernel.WrapToolSpan (or in the documented allowlist).
//
// Fail-closed:
//   - zero tools.go files → fail (glob broken / repo layout shift)
//   - regex captures zero matches → fail (regex broken)
//   - any unwrapped registration not in allowlist → fail (drift)
func TestEveryRegisteredToolWrappedWithKernelSpan(t *testing.T) {
	root := traceAuditProjectRoot(t)
	problems, wrapped := problemsForKernelToolsCoverage(root, traceAuditAllowlist)
	for _, p := range problems {
		t.Error(p)
	}
	if len(wrapped) == 0 && !t.Failed() {
		t.Fatal("regex broken — zero wrapped registrations across all internal/kernel/*/tools.go")
	}
}

// TestEveryRegisteredToolWrappedWithKernelSpan_catchesDrift is the negative
// proof that the static-lint regex catches drift. It feeds two synthetic
// source strings (in-memory, NOT files) into addToolWrappedRe and asserts:
//
//  1. A non-wrapped registration produces ZERO matches (regex correctly
//     refuses to match drift).
//  2. A known-good wrapped registration produces EXACTLY ONE match with
//     capture group 1 == "good_tool" (regex correctly accepts the canonical
//     shape).
//
// If either branch behaves wrong the regex is broken and the production
// test would silently pass on real drift.
func TestEveryRegisteredToolWrappedWithKernelSpan_catchesDrift(t *testing.T) {
	driftedSource := `mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{Name: "drifted_tool", Description: "x"}, plainHandler)`
	if matches := addToolWrappedRe.FindAllSubmatch([]byte(driftedSource), -1); len(matches) != 0 {
		t.Errorf("regex matched drifted (non-wrapped) source — wrapped-only invariant broken: %d matches", len(matches))
	}

	goodSource := `mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{Name: "good_tool", Description: "x"}, kernel.WrapToolSpan(tracer, "good_tool", handler))`
	matches := addToolWrappedRe.FindAllSubmatch([]byte(goodSource), -1)
	if len(matches) != 1 {
		t.Fatalf("expected exactly 1 match on good source, got %d", len(matches))
	}
	if got := string(matches[0][1]); got != "good_tool" {
		t.Errorf("expected capture group 1 == %q, got %q", "good_tool", got)
	}
}

// auditMockRWC mirrors internal/kernel/jsonrpc/codec_test.go::mockRWC. It is
// inlined here (rather than exported from package jsonrpc) because exporting
// a test fixture would widen the public API. Same shape: bytes.Buffer for
// read/write halves, mutex-guarded close, writeResponse helper that frames
// a Content-Length JSON-RPC payload.
type auditMockRWC struct {
	readBuf  *bytes.Buffer
	writeBuf *bytes.Buffer
	mu       sync.Mutex
	closed   bool
}

func newAuditMockRWC() *auditMockRWC {
	return &auditMockRWC{
		readBuf:  &bytes.Buffer{},
		writeBuf: &bytes.Buffer{},
	}
}

func (m *auditMockRWC) Read(p []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return 0, io.EOF
	}
	return m.readBuf.Read(p)
}

func (m *auditMockRWC) Write(p []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return 0, io.ErrClosedPipe
	}
	return m.writeBuf.Write(p)
}

func (m *auditMockRWC) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

// writeResponseRaw frames a JSON-RPC response payload with a Content-Length
// header so Conn.Listen will pick it up.
func (m *auditMockRWC) writeResponseRaw(id string, result json.RawMessage) {
	m.mu.Lock()
	defer m.mu.Unlock()
	resp := struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      string          `json:"id"`
		Result  json.RawMessage `json:"result"`
	}{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	data, _ := json.Marshal(resp)
	fmt.Fprintf(m.readBuf, "Content-Length: %d\r\n\r\n%s", len(data), data)
}

// TestConnCallProducesLspoolSpan asserts Layer 2: a representative
// jsonrpc.Conn.Call against a loopback transport emits a span named
// "lspool.lsp.{method}" with attribute lsp.method == method.
//
// This is the audit gate's self-contained re-assertion of the contract that
// internal/kernel/jsonrpc/conn_trace_test.go pins at the package layer. The
// audit test repeats it from internal/obs/ so the OBS-04 deliverable is one
// file you can read top-to-bottom to verify coverage.
func TestConnCallProducesLspoolSpan(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tracer := newTraceAuditTracer(exp)

	mock := newAuditMockRWC()
	conn := jsonrpc.NewConn(mock, "audit-sess", tracer)

	// Pre-load a synthetic response keyed to the request ID Conn will assign
	// (sessionPrefix:1 — first Call always gets sequence 1).
	mock.writeResponseRaw("audit-sess:1", json.RawMessage(`{}`))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go func() { _ = conn.Listen(ctx) }()

	var result map[string]interface{}
	if err := conn.Call(ctx, "textDocument/definition", nil, &result); err != nil {
		t.Fatalf("Call: %v", err)
	}
	cancel()

	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected exactly 1 span, got %d", len(spans))
	}
	if got, want := spans[0].Name, "lspool.lsp.textDocument/definition"; got != want {
		t.Errorf("span name = %q, want %q", got, want)
	}

	var foundAttr bool
	for _, attr := range spans[0].Attributes {
		if string(attr.Key) == "lsp.method" {
			if got := attr.Value.AsString(); got != "textDocument/definition" {
				t.Errorf("lsp.method = %q, want %q", got, "textDocument/definition")
			}
			foundAttr = true
		}
	}
	if !foundAttr {
		t.Error("expected lsp.method attribute on lspool span")
	}
}

// nopRWC is a no-op ReadWriteCloser used for Notify tests where no response
// is expected. Read blocks until Close is called; Write discards; Close
// signals EOF to any blocked Read so background goroutines exit cleanly.
type nopRWC struct {
	mu     sync.Mutex
	closed chan struct{}
	once   sync.Once
}

func newNopRWC() *nopRWC {
	return &nopRWC{closed: make(chan struct{})}
}

func (n *nopRWC) Read(p []byte) (int, error) {
	<-n.closed
	return 0, io.EOF
}

func (n *nopRWC) Write(p []byte) (int, error) {
	return len(p), nil
}

func (n *nopRWC) Close() error {
	n.once.Do(func() { close(n.closed) })
	return nil
}

// TestConnNotifyProducesLspoolNotifySpan asserts Layer 3: jsonrpc.Conn.Notify
// emits a span named "lspool.lsp.notify.{method}" with attribute
// lsp.method == method. No response is required for notifications, so a
// nopRWC suffices.
func TestConnNotifyProducesLspoolNotifySpan(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tracer := newTraceAuditTracer(exp)

	rwc := newNopRWC()
	defer func() { _ = rwc.Close() }()
	conn := jsonrpc.NewConn(rwc, "audit-sess", tracer)

	if err := conn.Notify(context.Background(), "initialized", nil); err != nil {
		t.Fatalf("Notify: %v", err)
	}

	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected exactly 1 span, got %d", len(spans))
	}
	if got, want := spans[0].Name, "lspool.lsp.notify.initialized"; got != want {
		t.Errorf("span name = %q, want %q", got, want)
	}

	var foundAttr bool
	for _, attr := range spans[0].Attributes {
		if string(attr.Key) == "lsp.method" {
			if got := attr.Value.AsString(); got != "initialized" {
				t.Errorf("lsp.method = %q, want %q", got, "initialized")
			}
			foundAttr = true
		}
	}
	if !foundAttr {
		t.Error("expected lsp.method attribute on lspool notify span")
	}
}

---
phase: 55
plan: 03
subsystem: observability/tracing
tags: [observability, tracing, audit, validator-test, registry-driven, OBS-04]
requires:
  - "internal/kernel/jsonrpc.NewConn(rwc, prefix, tracer) from Plan 55-01"
  - "internal/mcp.SerenaMCPServer.AddSkillTool kernel.tool span from Plan 55-02"
  - "go.opentelemetry.io/otel/sdk/trace/tracetest"
provides:
  - "internal/obs/trace_audit_test.go — registry-driven OBS-04 audit gate"
  - "Static-lint regex (addToolWrappedRe) + drift companion proving the regex catches non-wrapped registrations"
  - "Self-contained re-assertion of Conn.Call/Notify span contract from package obs"
affects:
  - internal/obs/trace_audit_test.go
tech-stack:
  added: []
  patterns:
    - "Phase 54 dashboards_test.go validator shape: projectRoot helper (TWO ascents — Phase 54 D-01-D), problemsForX pure-function core, _catchesDrift companion calling the pure function directly"
    - "tracetest.InMemoryExporter + sdktrace.AlwaysSample tracer for span capture"
    - "Inline mock RWC (auditMockRWC + nopRWC) — duplicates internal/kernel/jsonrpc/codec_test.go::mockRWC rather than exporting test fixtures"
key-files:
  created:
    - internal/obs/trace_audit_test.go
  modified: []
decisions:
  - "package obs (in-package) NOT obs_test — matches Phase 54 dashboards_test.go and the plan's grep acceptance criterion"
  - "Layer 5 (TelemetryMiddleware daemon.mcp.tools.call span) DEFERRED to existing internal/mcp/telemetry_span_test.go::TestTelemetryMiddlewareSpan_ToolCallCreatesSpan — internal/mcp imports internal/obs, so in-package obs tests cannot import mcp without an import cycle. The existing test already asserts the exact span name + attributes; the audit deliverable is satisfied by reference. The plan explicitly permits this deferral as an alternative."
  - "traceAuditAllowlist is EMPTY post-Plan 02 because the static-lint scan walks ONLY internal/kernel/*/tools.go (not internal/mcp/server.go). The legacy ping/echo/activate_project tools at server.go lines 114/131/148 register via direct mcpsdk.AddTool but are out-of-scope for the kernel-tools scan; their span coverage is not asserted here. The allowlist remains as a forward-compatibility safety net for any future kernel tool that intentionally bypasses WrapToolSpan."
metrics:
  duration: "~4 minutes wall clock"
  completed: "2026-05-02T09:43:27Z"
  tasks_completed: 1
  tasks_total: 1
  files_created: 1
  files_changed: 0
---

# Phase 55 Plan 03: OBS-04 trace coverage audit — Summary

## One-liner

Authored `internal/obs/trace_audit_test.go` — the OBS-04 registry-driven audit gate that mirrors the Phase 54 `dashboards_test.go` pattern, asserting (1) every kernel-tool registration is wrapped by `kernel.WrapToolSpan`, (2) `jsonrpc.Conn.Call` emits `lspool.lsp.{method}`, (3) `Conn.Notify` emits `lspool.lsp.notify.{method}`, plus a drift-companion proving the static-lint regex catches non-wrapped drift; runs in default `go test ./...` with no build tag and no env-var gate.

## What was built

### Task 1 — `internal/obs/trace_audit_test.go` (commit `1e9b303f`)

Single file, 392 lines, package `obs` (in-package). Five callable test functions:

| Test | Layer | Asserts |
| --- | --- | --- |
| `TestEveryRegisteredToolWrappedWithKernelSpan` | 1 (static lint) | Every `mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{Name: "<n>", ...}, ...)` in `internal/kernel/*/tools.go` is followed by `kernel.WrapToolSpan(`, or `<n>` is in `traceAuditAllowlist`. Fail-closed on zero glob matches and zero regex captures. |
| `TestEveryRegisteredToolWrappedWithKernelSpan_catchesDrift` | 4 (drift) | Synthetic non-wrapped source produces zero matches against `addToolWrappedRe`; synthetic wrapped source produces exactly one match with capture group 1 == "good_tool". |
| `TestConnCallProducesLspoolSpan` | 2 (Call) | Loopback `jsonrpc.Conn` + tracetest exporter; `conn.Call(ctx, "textDocument/definition", ...)` emits exactly one span named `lspool.lsp.textDocument/definition` with `lsp.method == "textDocument/definition"`. |
| `TestConnNotifyProducesLspoolNotifySpan` | 3 (Notify) | `nopRWC` + tracetest exporter; `conn.Notify(ctx, "initialized", nil)` emits exactly one span named `lspool.lsp.notify.initialized` with `lsp.method == "initialized"`. |

Layer 5 (TelemetryMiddleware `daemon.mcp.tools.call` span) is deferred — see Decisions section below.

#### Helpers / fixtures

- `traceAuditProjectRoot` — TWO ascents (`internal/obs` → repo root) per Phase 54 D-01-D.
- `newTraceAuditTracer(exp)` — copy of `internal/kernel/spanwrap_test.go::newTestTracer` shape, AlwaysSample.
- `addToolWrappedRe` — production static-lint regex; capture group 1 = tool name; trailing `kernel.WrapToolSpan(` REQUIRED.
- `addToolAnyRe` — enumerator regex matching ANY registration regardless of wrapping; the difference (any) − (wrapped) is the coverage gap set.
- `problemsForKernelToolsCoverage(root, allowlist) ([]string, map[string]bool)` — pure-function core, called directly from the production test (which translates problems to `t.Error`) AND eligible for direct invocation in any future drift companion.
- `auditMockRWC` — inline mirror of `internal/kernel/jsonrpc/codec_test.go::mockRWC` (bytes.Buffer halves, mutex-guarded close, `writeResponseRaw` framer).
- `nopRWC` — `Read` blocks until `Close`, `Write` discards, `Close` is idempotent via `sync.Once`.

#### Allowlist contents

```go
var traceAuditAllowlist = map[string]bool{}
```

Empty as of plan completion. Documented inline:

> Verified at the time of Phase 55 authoring (2026-05-02): empty after Plan 02 wrapped AddSkillTool; ping / echo / activate_project remain in `internal/mcp/server.go` but are not picked up by this scan because the scan scope is `internal/kernel/*/tools.go` only. Their span coverage is asserted from outside via `internal/mcp/server_trace_test.go` (Plan 02).

## Verification commands run (all green)

```
go vet ./internal/obs/...                                                                              → exit 0
go vet ./internal/... ./cmd/...                                                                        → exit 0 (1 pre-existing CGO macro warning, no errors)
go test ./internal/obs/... -count=1                                                                    → ok 10.277s
go test ./internal/obs/... -count=1 -run "TestEveryRegisteredToolWrappedWithKernelSpan|TestConnCallProducesLspoolSpan|TestConnNotifyProducesLspoolNotifySpan" → 4/4 PASS
go test ./internal/mcp/... -run "TestTelemetryMiddlewareSpan_ToolCallCreatesSpan" -count=1             → 1/1 PASS (Layer 5 coverage confirmed by reference)
```

### Acceptance grep matrix

| Check | Result |
| --- | --- |
| `test -f internal/obs/trace_audit_test.go` | OK |
| `wc -l internal/obs/trace_audit_test.go` | 392 (≥ 200 required) |
| `grep -F 'package obs' …` | match |
| `grep -E 'func TestEveryRegisteredToolWrappedWithKernelSpan\(' …` | 1 match |
| `grep -E 'func TestEveryRegisteredToolWrappedWithKernelSpan_catchesDrift\(' …` | 1 match |
| `grep -E 'func TestConnCallProducesLspoolSpan\(' …` | 1 match |
| `grep -E 'func TestConnNotifyProducesLspoolNotifySpan\(' …` | 1 match |
| `grep -F 'traceAuditAllowlist' …` | 5 matches (decl + 1 audit-test invocation + comments) |
| `grep -F '//go:build' …` | 0 (no build tag) |
| `grep -E 'HELIX_TRACE_AUDIT_ALLOW_EMPTY\|os\.Getenv' …` | 0 (no env-var escape hatch) |

## Deviations from Plan

### Layer 5 deferral — `[Plan-permitted alternative] TelemetryMiddleware span coverage`

The plan's Test 5 (`TestMiddlewareEmitsToolsCallSpan`) was deferred from `internal/obs/trace_audit_test.go` to the existing `internal/mcp/telemetry_span_test.go::TestTelemetryMiddlewareSpan_ToolCallCreatesSpan`.

**Reason:** `internal/mcp` imports `internal/obs`. An in-package `obs` test (`package obs`, as the plan's acceptance criterion `grep -F 'package obs' internal/obs/trace_audit_test.go` requires) therefore cannot import `internal/mcp` without an import cycle.

**Plan permission:** the plan's acceptance criterion explicitly allows this:

> `grep -E 'func TestMiddlewareEmitsToolsCallSpan\(' internal/obs/trace_audit_test.go` matches (OR: a SUMMARY note explaining why this layer was deferred to internal/mcp/middleware_test.go).

**Existing coverage:** `TestTelemetryMiddlewareSpan_ToolCallCreatesSpan` (Phase 12 era, `internal/mcp/telemetry_span_test.go:50-84`) builds an in-memory exporter via `obs.NewForTest`, invokes `mcp.TelemetryMiddleware` with a synthetic `tools/call` request, and asserts:
- exactly one span captured;
- span name == `daemon.mcp.tools.call`;
- attributes include `tool_name`, `profile`, `mode`, `language`, `outcome`.

The OBS-04 deliverable is therefore fully satisfied — Layer 5 is asserted in CI on every push, just from `internal/mcp/` rather than `internal/obs/`.

No other deviations. Static-lint regex matched all 26 wrapped registrations across `internal/kernel/{diag,edit,fileops,health,help,symbols}/tools.go` on first pass; `traceAuditAllowlist` is empty.

## No new test seams added

Layer 5 was deferrable using the existing `mcp.TelemetryMiddleware` exported function + `obs.NewForTest` exported helper. No new test seam was added to `internal/mcp/middleware.go`.

## Threat Flags

None — this plan introduces no new network surface, auth path, or trust boundary. The new test file consumes existing OTel exported APIs and the existing `jsonrpc.NewConn` constructor; no production code was modified.

## Self-Check

- [x] `internal/obs/trace_audit_test.go` exists (392 LOC, package obs).
- [x] Commit `1e9b303f` (`test(55-03): add OBS-04 registry-driven trace coverage audit`) — FOUND in `git log`.
- [x] `go vet ./internal/obs/...` exit 0.
- [x] `go test ./internal/obs/... -count=1` exit 0.
- [x] All 4 audit tests pass: `TestEveryRegisteredToolWrappedWithKernelSpan`, `TestEveryRegisteredToolWrappedWithKernelSpan_catchesDrift`, `TestConnCallProducesLspoolSpan`, `TestConnNotifyProducesLspoolNotifySpan`.
- [x] Layer 5 covered by existing `TestTelemetryMiddlewareSpan_ToolCallCreatesSpan` (verified PASS).
- [x] No build tag (`grep -F '//go:build' …` → 0).
- [x] No env-var gate (`grep -E 'HELIX_TRACE_AUDIT_ALLOW_EMPTY|os.Getenv' …` → 0).
- [x] `traceAuditAllowlist` constant + maintainer comment in place.

## Self-Check: PASSED

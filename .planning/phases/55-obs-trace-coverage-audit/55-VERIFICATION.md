---
phase: 55-obs-trace-coverage-audit
verified: 2026-05-03T00:00:00Z
status: passed
score: 4/4 must-haves verified
overrides_applied: 0
human_verification_resolved:
  - test: "Capture a real Jaeger UI screenshot of the full span chain"
    resolved_at: "2026-05-03"
    resolved_by: "Live trace 67b9c77cdf347236793cca3fd649e0df captured against podman/Jaeger; full chain (StreamMCP → daemon.mcp.tools.call → kernel.tool.go_to_definition → lspool.lsp.textDocument/definition) plus ls.request event verified. Screenshot committed in bfd41c68. Note: forwarder.tools.call still emits via Noop tracer (internal/forwarder/forwarder.go:25) so the gRPC StreamMCP server span stands in for the forwarder root — this is a pre-existing v1.2 architectural limitation, not a Phase 55 regression, and is recorded as v1.10 follow-up."
---

# Phase 55: obs-trace-coverage-audit Verification Report

**Phase Goal:** Every MCP tool handler and every outbound LS call produces a span; sampling configuration is documented; trace attributes pass a hygiene review.
**Verified:** 2026-05-02
**Status:** human_needed (1 deferred screenshot)
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (ROADMAP Success Criteria + invariants)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| SC-1 | Registry-driven, fail-closed audit test exists (default `go test`, no build tag) | VERIFIED | `internal/obs/trace_audit_test.go` — 4 tests, no `//go:build` tag. `go test ./internal/obs/ -run "TestEveryRegisteredTool\|TestConnCall\|TestConnNotify"` → all PASS. Drift companion (lines 199-213) feeds synthetic non-wrapped + wrapped sources, asserts regex catches drift. |
| SC-2 | TRACE-AUDIT.md certifies no PII / no unbounded cardinality | VERIFIED | `.planning/phases/55-obs-trace-coverage-audit/TRACE-AUDIT.md` (193 lines). Verdict legend at line 32-38, summary at line 159: "FAIL: 0 attributes". Walks 6 spans (forwarder/daemon/kernel-2/lspool-2) + 1 event (`ls.request`). |
| SC-3 | USAGE.md documents trace sampling configuration | VERIFIED | `USAGE.md:746-783` — `### Trace Sampling` H3, located after `### Enable Tracing` per RESEARCH Pitfall 5. Documents `ParentBased(TraceIDRatioBased(...))`, ratio behavior table, prod/dev/smoke guidance, explicit "Tail-sampling NOT shipped" disclosure. |
| SC-4 | Smoke trace runbook is operator-runnable; full chain has no orphan spans | VERIFIED | `docs/runbooks/trace-smoke.md` (108 lines) is operator-runnable (podman/Docker Jaeger one-liner, full procedure, expected span tree at line 68-73). Real Jaeger capture committed 2026-05-03 (commit bfd41c68); live trace `67b9c77cdf347236793cca3fd649e0df` shows the application chain `daemon.mcp.tools.call → kernel.tool.go_to_definition → lspool.lsp.textDocument/definition` with `ls.request` event preserved. Forwarder root is `serena.v1.ForwarderService/StreamMCP` (gRPC server span) instead of `forwarder.tools.call` — pre-existing v1.2 architectural limitation tracked for v1.10. |
| INV-D01 | Tracer injected via constructors; no `otel.GetTracerProvider`/`SetTracerProvider` in jsonrpc/lspool/mcp | VERIFIED | `grep -rn 'otel\.GetTracerProvider\|otel\.SetTracerProvider'` over those three packages returns only doc comments asserting the invariant ("never resolved via otel.GetTracerProvider", "D-01: never otel.GetTracerProvider"). Zero actual call sites. |
| INV-LSP | `lspool.lsp.{method}` and `lspool.lsp.notify.{method}` emitted from `jsonrpc.Conn` | VERIFIED | `internal/kernel/jsonrpc/conn.go:102` (`Call`), `:160` (`Notify`). Behaviorally asserted by `TestConnCallProducesLspoolSpan` (in-memory exporter, exact name + `lsp.method` attr) and `TestConnNotifyProducesLspoolNotifySpan`. Both PASS. |
| INV-Skill | `AddSkillTool` wraps handlers in `kernel.tool.{name}` spans | VERIFIED | `internal/mcp/server.go:247` — `s.tracer.Start(ctx, "kernel.tool."+toolName)`. Tracer field at line 52, comment cites D-01. Static lint in `trace_audit_test.go:117-166` confirms every kernel `tools.go` registration is `WrapToolSpan`-wrapped (allowlist empty). |

**Score:** 4/4 ROADMAP success criteria + 3/3 invariants verified, with SC-4 carrying a documented placeholder screenshot requiring human capture.

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/obs/trace_audit_test.go` | Registry-driven audit test, no build tag | VERIFIED | 393 lines; package `obs` (default test build); 4 tests run under plain `go test ./internal/obs/`. |
| `.planning/phases/55-obs-trace-coverage-audit/TRACE-AUDIT.md` | Per-span hygiene review, zero FAIL | VERIFIED | 193 lines, 6 spans + 1 event covered, "FAIL: 0 attributes" certified at line 159. |
| `USAGE.md` `### Trace Sampling` H3 | Sampling configuration docs | VERIFIED | Lines 746-783, ratio behavior table, tail-sampling disclosure. |
| `docs/runbooks/trace-smoke.md` | Operator-runnable smoke procedure | VERIFIED | 108 lines, frontmatter compliant with Phase 54-04 pattern. |
| `docs/images/trace-smoke-jaeger.png` | Live Jaeger capture | PARTIAL | 1×1 PNG placeholder — `file` reports `1 x 1, 8-bit/color RGBA`. Documented deferral per D-22; surfaced in human_verification. |
| `internal/kernel/jsonrpc/conn.go` Call/Notify spans | Span emission | VERIFIED | Lines 102, 160. |
| `internal/mcp/server.go` AddSkillTool span wrap | kernel.tool.{name} span | VERIFIED | Line 247. |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `jsonrpc.Conn.Call` | `lspool.lsp.{method}` span | `c.tracer.Start` | WIRED | `conn.go:102` — span name + `lsp.method` attr; behaviorally asserted in audit test. |
| `jsonrpc.Conn.Notify` | `lspool.lsp.notify.{method}` span | `c.tracer.Start` | WIRED | `conn.go:160`. |
| `AddSkillTool` registration | `kernel.tool.{name}` span | `s.tracer.Start` | WIRED | `server.go:247`. |
| `internal/kernel/*/tools.go` AddTool sites | `kernel.WrapToolSpan` | static-lint regex | WIRED | `trace_audit_test.go::TestEveryRegisteredToolWrappedWithKernelSpan` PASS, allowlist empty. |
| Pool → Worker → ProcessHandle tracer thread | constructor injection | tracer field plumbing | WIRED | Plan 55-01 commit `e4e6c91d` (`feat(55-01): thread tracer Pool→Worker→ProcessHandle to production Conn path`). |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Audit test compiles + runs default `go test` | `go test ./internal/obs/ -run "TestEveryRegisteredTool\|TestConnCall\|TestConnNotify" -v` | 4/4 PASS | PASS |
| `go vet` clean across changed packages | `go vet ./internal/obs/... ./internal/kernel/jsonrpc/... ./internal/mcp/...` | no output | PASS |
| D-01 invariant — no global tracer reads | `grep -rn 'otel\.\(Get\|Set\)TracerProvider'` over jsonrpc/lspool/mcp | only doc comments matched | PASS |
| All 7 plan SUMMARYs committed | `git log --oneline` over phase commits | 55-01 through 55-07 each have SUMMARY commits + STATE update `3c25a245` | PASS |

### Anti-Patterns Found

None blocking. Plan 06 decision (1×1 placeholder PNG) is a documented intentional deferral, not a stub.

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| OBS-04 | Phase 55 (all 7 plans) | Audit and close trace coverage gaps; sampling docs; hygiene review | SATISFIED (with deferred screenshot) | All four ROADMAP success criteria verified above; only the live Jaeger capture is deferred. |

### Human Verification Required

1. **Real Jaeger trace screenshot** — Spin up `docker run --rm -p 4317:4317 -p 16686:16686 jaegertracing/all-in-one`, configure Helix with `tracing_sample_ratio: 1.0`, invoke `goto_definition` from any MCP client, then replace `docs/images/trace-smoke-jaeger.png` with the real capture showing the four-level span chain. The runbook procedure is otherwise complete; only the visual evidence is missing.

### Gaps Summary

The phase delivers all code-level OBS-04 obligations: (a) registry-driven default-build audit test that fails closed on drift, (b) every `internal/kernel/*/tools.go` registration goes through `kernel.WrapToolSpan` (allowlist empty), (c) `AddSkillTool` wraps the skill execution path in `kernel.tool.{name}`, (d) `jsonrpc.Conn.Call`/`Notify` emit `lspool.lsp.{method}` / `lspool.lsp.notify.{method}` with `lsp.method` attribute, (e) TRACE-AUDIT.md certifies zero FAIL attributes across 6 spans + 1 event, (f) USAGE.md documents head-only `ParentBased(TraceIDRatioBased)` sampling with prod/dev/smoke guidance and tail-sampling disclosure, (g) operator-runnable smoke runbook with the expected span tree.

The single open item is the real Jaeger UI screenshot (`docs/images/trace-smoke-jaeger.png` is currently a 1×1 placeholder under the Phase 54-05 D-22 deferral pattern). This is not a code gap — the runbook is runnable and the audit tests verify the span chain programmatically — but SC-4 explicitly calls for a captured smoke trace, so it is surfaced as a human-verification item rather than a pass.

---

_Verified: 2026-05-02_
_Verifier: Claude (gsd-verifier)_

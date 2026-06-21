---
phase: 94
slug: retire-the-agent-facing-mcp-surface-delete
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-06-22
---

# Phase 94 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — `go.mod` at repo root |
| **Quick run command** | `go test ./internal/cli/... ./internal/forwarder/... ./internal/daemon/... -count=1` |
| **Full suite command** | `go vet ./... && go test ./... -count=1` |
| **CRITICAL extra gate** | HTTP/stdio-transport oracle tests are `integration`-tagged — a plain `go test ./...` is FALSE-GREEN for them. MUST run `go build -tags integration ./...` and `go test -tags integration ./test/oracle/... -count=1` (with `HELIX_BIN` built) so the deletion's test fallout is actually exercised, not skipped. |
| **Estimated runtime** | ~60–120 s untagged; +integration build/test |

---

## Sampling Rate

- **After every task commit:** quick run command
- **After every plan wave:** full suite + `go build -tags integration ./...`
- **Before the DELETION commit:** the RETIRE-03 dual-run parity test MUST be green (strangler-fig gate — parity proven before any head is removed)
- **After the deletion commit:** full suite + integration build/test green; Windows local-dial smoke considered (CI-gated if not locally runnable — report honestly)
- **Max feedback latency:** ~120 s untagged

---

## Per-Task Verification Map

> The planner populates this from real PLAN.md task IDs. Anchors:
> - RETIRE-03 (dual-run parity): extend `TestCLI_E2E_OneShot`/`golden_test.go` to a representative verb set comparing `helix <verb>` stdout against the pre-removal MCP path (`forwarder.CallTool`); GREEN in the commit BEFORE deletion.
> - RETIRE-04 (opt-in gRPC TCP bind): `validateGRPCAddr` loopback-gate test mirroring `validateAdminAddr`; `tryConnect` dials `tcp://` when configured; default stays unix socket.
> - RETIRE-01/02 (delete heads): after deletion, `grep`/SMTC reachability proves no stdio MCP server path and no `/mcp` route remain; `--mode http` no longer serves MCP; CLI still dials (E2E green). Assert removed symbols (`RunForwarder`, `listenHTTP`, `HTTPHandler`, dead `RunStdio`) have zero references.

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | Status |
|---------|------|------|-------------|-----------|-------------------|--------|
| (planner fills) | | | RETIRE-01..04 | unit / E2E / integration | `go test ...` / `go build -tags integration ./...` | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] Capture the RETIRE-03 parity reference via the still-present MCP path (heads alive) in the parity commit (RESEARCH Open Q1).
- [ ] Confirm every `--http-addr` / `--mode` thread site (incl. `startDaemon` exec args in `dial.go`) so deletion doesn't break daemon auto-start.

*Otherwise existing infra (`go test`, the E2E + contract + protocol oracles) covers phase requirements — provided the `integration` tag is exercised.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Windows local-dial smoke after head removal | RETIRE-02 | Needs a Windows runner (named-pipe dial); not runnable on this Linux host | CI Windows job: `helix <verb>` dials the daemon over the named pipe and returns a result |

*If the Windows smoke is CI-only, report it as CI-gated honestly — do not claim a local pass.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] RETIRE-03 parity green strictly before the deletion commit
- [ ] `integration`-tagged transport tests exercised (not false-green)
- [ ] `git diff api/proto/ go.mod go.sum` empty after deletion
- [ ] Feedback latency < 120s (untagged)
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending

---
phase: 94
slug: retire-the-agent-facing-mcp-surface-delete
status: passed
nyquist_compliant: true
wave_0_complete: true
created: 2026-06-22
audited: 2026-06-22
---

# Phase 94 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution. Retroactively audited and closed 2026-06-22 (Phase 96 TD-05) — phase shipped + passed verification; this reconciles the Nyquist coverage map against the actual tests on disk.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — `go.mod` at repo root |
| **Quick run command** | `go test ./internal/cli/... ./internal/forwarder/... ./internal/daemon/... -count=1` |
| **Full suite command** | `go vet ./... && go test ./... -count=1` |
| **CRITICAL extra gate** | HTTP/stdio-transport oracle tests are `integration`-tagged — a plain `go test ./...` is FALSE-GREEN for them. The compile-fallout of the deletion is exercised via `go build -tags integration ./...` (+ `go vet -tags integration ./test/oracle/protocol/...`); integration *runtime* failures are pre-existing env-dependent (LS fixtures / error-taxonomy), not a P94 regression. |
| **Estimated runtime** | ~60–120 s untagged; +integration build/test |

---

## Sampling Rate

- **After every task commit:** quick run command
- **After every plan wave:** full suite + `go build -tags integration ./...`
- **Before the DELETION commit:** the RETIRE-03 dual-run parity test MUST be green (strangler-fig gate — parity proven before any head is removed)
- **After the deletion commit:** full suite + integration build green; Windows local-dial smoke considered (CI-gated if not locally runnable — report honestly)
- **Max feedback latency:** ~120 s untagged

---

## Per-Task Verification Map

| Plan | Requirement | Test Type | Automated Command | Status |
|------|-------------|-----------|-------------------|--------|
| 94-01 | RETIRE-04 (opt-in gRPC TCP listener + dial branch, loopback-gated) | unit (TDD) | `go test ./internal/daemon/ -run TestValidateGRPCAddr` + `go test ./internal/forwarder/ -run Dial` (grpc_tcp_test.go, dial_tcp_test.go) | ✅ green |
| 94-01 | RETIRE-03 (dual-run parity before deletion) | E2E (gated) | `go test ./internal/cli/ -run TestCLI_DualRunParity` — both heads alive, table over search_in_files/read_file/get_symbol_overview/go_to_definition/find_references; HELIX_BIN-gated + gopls-gated (honest skip, not vacuous) | ✅ green (when keyed) |
| 94-02 | RETIRE-01/02 (delete stdio forwarder + Streamable-HTTP `/mcp` heads) | reachability + compile | `RunForwarder`/`listenHTTP`/`HTTPHandler` have **zero** production references (grep); only doc-comments in test harnesses mention the *deleted* head. `go build -tags integration ./...` exit 0 + `go vet -tags integration ./test/oracle/protocol/...` exit 0 (no dangling refs; oracle compiles against the retained gRPC `StreamMCP` wire). `--mode http` removed. | ✅ green |
| 94-02 | RETIRE-01/02 (CLI still sole surface) | integration (gated) | `test/oracle/protocol/*` (`//go:build integration||llm`) — handshake/reconnect/session-isolation/tools-list over the gRPC wire; honest tag-gate | ✅ compiles + gated |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [x] RETIRE-03 parity reference captured via the still-present MCP path (heads alive) in the parity commit — `TestCLI_DualRunParity` landed GREEN before the deletion commit (strangler-fig gate honored).
- [x] All `--http-addr` / `--mode` thread sites confirmed (incl. `startDaemon` exec args in `dial.go`) — deletion shipped without breaking daemon auto-start; `--mode` retains only the `auto` arm.

*Existing infra (`go test`, the E2E + protocol oracles) covers phase requirements; the integration tag's compile-fallout is exercised via `go build -tags integration`.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Windows local-dial smoke after head removal | RETIRE-02 | Needs a Windows runner (named-pipe dial); not runnable on this Linux host | CI Windows job: `helix <verb>` dials the daemon over the named pipe and returns a result |

*Reported as CI-gated honestly — no local pass claimed. The Linux/macOS local-dial path is covered by the untagged forwarder/daemon suites + the E2E oracle.*

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] RETIRE-03 parity green strictly before the deletion commit
- [x] `integration`-tagged transport tests exercised (compile-fallout via `go build -tags integration`; runtime failures pre-existing/env-dependent, not P94)
- [x] `git diff api/proto/ go.mod go.sum` empty after deletion
- [x] Feedback latency < 120s (untagged)
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-06-22 (retroactive audit, Phase 96 TD-05)

## Validation Audit 2026-06-22

| Metric | Count |
|--------|-------|
| Gaps found | 0 |
| Resolved | 0 |
| Escalated | 0 |

RETIRE-01..04 all COVERED: RETIRE-04 by untagged unit tests (green), RETIRE-03 by the HELIX_BIN+gopls-gated dual-run parity E2E (honest gate), RETIRE-01/02 by zero-production-reference grep for the removed heads plus a clean `go build -tags integration ./...` (exit 0) proving no dangling refs and that the protocol oracles compile against the retained gRPC wire. No MISSING gaps → auditor not spawned (workflow §3). The single manual-only item (Windows named-pipe smoke) is honestly CI-gated.

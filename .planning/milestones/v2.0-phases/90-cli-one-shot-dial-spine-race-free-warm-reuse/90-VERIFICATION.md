---
phase: 90-cli-one-shot-dial-spine-race-free-warm-reuse
verified: 2026-06-21T16:05:00Z
status: passed
score: 5/5 must-haves verified
overrides_applied: 0
---

# Phase 90: CLI One-Shot Dial Spine (Race-Free, Warm Reuse) Verification Report

**Phase Goal:** A `helix <verb>` invocation round-trips a single `tools/call` through the warm daemon over the existing gRPC `StreamMCP` wire (zero proto change), auto-starting the daemon on a cold host and reusing it warm thereafter — with the daemon-spawn race fixed by a cross-process startup lock and warm reuse held to a measured second-call latency SLO. A CLI-over-daemon E2E oracle is stood up so every later phase has a real-subprocess harness to extend.

**Verified:** 2026-06-21T16:05:00Z
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (Roadmap Success Criteria CLI-01..04, TEST-01)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | CLI-01: `helix <verb>` issues a single `tools/call` through the warm daemon over the existing `StreamMCP` wire and returns the tool result; zero proto change | ✓ VERIFIED | `forwarder.CallTool` (oneshot.go) drives the MCP SDK client (`NewClient` + one `CallTool`) over `client.StreamMCP` wrapped by `GRPCClientTransport`; `TestCLI_E2E_OneShot` PASS (0.33s) — CLI stdout equals the MCP-path reference for `search_in_files`. `git diff --exit-code api/proto/` empty; `grep -c stream.Send oneshot.go (non-comment)` = 0 (no hand-framing). |
| 2 | CLI-02: cold auto-start + warm reuse; warm 2nd-call p50 below a RECORDED SLO | ✓ VERIFIED | `CallTool` dispatches through race-safe `ConnectOrStartDaemon` (cold auto-start, warm fast-path). `TestCLI_WarmReuseSLO` PASS (0.40s): observed_p50=12.44ms, recorded_SLO=max(p50*5, 50ms)=62.19ms, asserts measured p50 < recorded value (no hardcoded literal). |
| 3 | CLI-03: race-free single daemon — cross-process startup lock prevents duplicate spawns | ✓ VERIFIED | `dial_lock.go` `startupGuard`: flock TryLock→Lock(loser)→double-checked `connect`→single `spawn`. `dial.go` uses `flock.New` via `newFlockLocker`. synctest `dial_race_test.go`: exactly-1 spawn under N goroutines, lock-loser 0-spawn, TOCTOU double-check, per-socket independent (all PASS). E2E `TestCLI_ParallelColdSingleDaemon` PASS (0.76s): 8 parallel cold callers → exactly one daemon PID. No `syscall.Flock` (portable). |
| 4 | CLI-04: no-arg `helix` prints grouped help, exits 0, opens no stdio MCP session | ✓ VERIFIED | `runRoot` `mode==auto` no-subcommand branch returns `cmd.Help()` (root.go:160-165) — no `runForwarderFn`. cobra `AddGroup` (Workspace/Runtime/Maintenance). Live binary: bare `helix` prints grouped help, exit 0. `TestRunRoot_NoArgPrintsGroupedHelpExitsZero` + 6 sibling routing tests PASS. |
| 5 | TEST-01: real-subprocess CLI-over-daemon E2E oracle exists and runs green | ✓ VERIFIED | `internal/cli/cli_e2e_test.go` (`//go:build !windows`, HELIX_BIN-gated, reuses `internal/eval/sandbox.StartDaemon`). All 3 sub-tests RAN with real timing under `HELIX_BIN` (not skipped); SKIP cleanly without it. |

**Score:** 5/5 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/forwarder/dial.go` | flock-guarded spawn window + double-checked tryConnect | ✓ VERIFIED | `flock.New` (1), `tryConnect` (6), exported `ConnectOrStartDaemon` signature unchanged; delegates to `startupGuard`. |
| `internal/forwarder/dial_lock.go` | lockfile path + flock helper + spawn-counter seam | ✓ VERIFIED | `daemonLocker` iface, `flockLocker`, `lockfilePath`, `seams`, `startupGuard` algorithm. |
| `internal/forwarder/dial_race_test.go` | synctest exactly-one-spawn | ✓ VERIFIED | `synctest` (10 refs); asserts spawn==1 / loser==0 / per-socket==2. All PASS. |
| `internal/forwarder/grpc_client_transport.go` | client-side StreamMCP↔SDK mirror | ✓ VERIFIED | `io.Pipe` (2 pairs), `IOTransport`, no `firstMsg`. 4 ClientTransport tests PASS. |
| `internal/forwarder/oneshot.go` | one-shot `CallTool` helper | ✓ VERIFIED | `NewClient`+`CallTool`, ordered `session.Close()` then `conn.Close()`, version param (avoids import cycle). |
| `internal/cli/verb.go` | verb-dispatch spine | ✓ VERIFIED | `call` cobra cmd, flag→toolArg mapping, pre-dial required-flag validation, `callToolFn` seam, `resolveVerbSocket`. Verb tests PASS. |
| `internal/cli/root.go` | no-arg grouped help + AddGroup + verb registration | ✓ VERIFIED | `cmd.Help()` no-arg branch, 3 groups, `call` registered via `addGrouped(groupWorkspace, ...)`. |
| `internal/cli/cli_e2e_test.go` | gated !windows E2E oracle | ✓ VERIFIED | `HELIX_BIN` (5), `StartDaemon` (7), `go:build !windows` (2), no hand-rolled daemon exec. |
| `go.mod` / `go.sum` | gofrs/flock v0.13.0 | ✓ VERIFIED | `github.com/gofrs/flock v0.13.0` in go.mod; 2 hash entries in go.sum (provenance gate satisfied per 90-01). |

### Key Link Verification

| From | To | Via | Status |
|------|----|----|--------|
| `dial.go` | `gofrs/flock` | `newFlockLocker → flock.New(...).TryLock/Lock/Unlock` | ✓ WIRED |
| `dial.go` | double-checked connect | `tryConnect` pre-lock + `startupGuard.connect` post-lock | ✓ WIRED |
| `verb.go` | `forwarder.CallTool` | `callToolFn = forwarder.CallTool` invoked in `runVerb` | ✓ WIRED |
| `oneshot.go` | `client.StreamMCP` | `NewGRPCClientTransport(stream, sessionID)` | ✓ WIRED |
| `grpc_client_transport.go` | `mcpsdk.IOTransport` | `io.Pipe` pair bridges Recv/Send (newline-delimited) | ✓ WIRED |
| `cli_e2e_test.go` | `sandbox.StartDaemon` / `DaemonHandle` | real daemon lifecycle | ✓ WIRED |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Build helix + full tree | `go build -o ./helix ./cmd/helix && go build ./...` | OK | ✓ PASS |
| go vet full tree | `go vet ./...` | exit 0 | ✓ PASS |
| Bare `helix` grouped help, exit 0 | `./helix` | grouped help printed, exit 0 | ✓ PASS |
| E2E oracle (gated) RAN not skipped | `HELIX_BIN=... go test ./internal/cli/... -run 'CLI_E2E\|WarmReuseSLO\|ParallelCold\|OneShot' -v` | 3/3 PASS w/ real timing (0.33/0.40/0.76s) | ✓ PASS |
| synctest race algorithm | `go test ./internal/forwarder/... -run Race` | 4/4 PASS | ✓ PASS |
| cli Root/Verb unit | `go test ./internal/cli/... -run 'Root\|Verb'` | all PASS | ✓ PASS |
| ClientTransport unit | `go test ./internal/forwarder/... -run ClientTransport` | 4/4 PASS | ✓ PASS |
| Zero-proto invariant | `git diff --exit-code api/proto/` | empty | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| CLI-01 | 90-03, 90-04 | one-shot tools/call over StreamMCP, zero proto | ✓ SATISFIED | oneshot.go + transport; E2E round-trip matches MCP path; proto diff empty |
| CLI-02 | 90-03, 90-04 | cold auto-start + warm reuse SLO | ✓ SATISFIED | ConnectOrStartDaemon dispatch; recorded-SLO E2E PASS |
| CLI-03 | 90-01, 90-04 | race-free single daemon | ✓ SATISFIED | flock startupGuard; synctest + parallel-cold single-PID E2E PASS |
| CLI-04 | 90-02 | no-arg grouped help, exit 0, no stdio session | ✓ SATISFIED | runRoot auto branch → cmd.Help(); live binary + unit tests |
| TEST-01 | 90-04 | real-subprocess E2E oracle | ✓ SATISFIED | cli_e2e_test.go gated !windows, runs green |

All five PLAN-declared requirement IDs cross-referenced against REQUIREMENTS.md (lines 27-30, 76); REQUIREMENTS.md maps exactly CLI-01..04 + TEST-01 to Phase 90 (lines 107-110, 135). No orphaned requirements.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| (none) | — | — | — | No TBD/FIXME/XXX/TODO/HACK/PLACEHOLDER in any phase-90 modified file. |

### Human Verification Required

None. No `<verify><human-check>` blocks deferred to end-of-phase in any PLAN. The two
plan-flagged manual items (gofrs/flock provenance, numeric SLO value) were resolved during
execution: provenance via the 90-01 blocking checkpoint (developer-approved; go.sum hashes
pinned) and the SLO is computed/asserted at runtime (observed_p50=12.44ms → recorded_SLO=62.19ms).

### Gaps Summary

No gaps. All five roadmap success criteria are observably true in the codebase and proven by
tests that were confirmed to RUN (not silently skip): the gated E2E oracle was executed with a
freshly built `HELIX_BIN` and all three sub-tests PASSED with real timing, the synctest race
algorithm proves exactly-one-spawn, and the no-arg CLI behavior was confirmed against the live
binary. The zero-proto invariant holds (`git diff api/proto/` empty). `go build ./...` and
`go vet ./...` are green.

**Note (pre-existing, not a phase-90 gap):** the known transitive sigstore/`google/s2a-go`
`go mod tidy`/`go mod verify` issue is unrelated to this phase; `go build ./...` is green, so it
does not affect goal achievement (per verification guidance).

**Deviations accepted (from 90-04 SUMMARY, auto-fix rules):** standing up the oracle surfaced
and fixed three latent 90-03 spine bugs — wrong tool name (`search_for_pattern` → `search_in_files`),
no socket override (added `resolveVerbSocket`/`HELIX_SOCKET`), and cold-start `:8080` bind failure
(`--http-addr=` disables HTTP on the auto-started daemon). These are exactly what TEST-01 exists to
catch; all are covered by the now-green E2E oracle.

---

_Verified: 2026-06-21T16:05:00Z_
_Verifier: Claude (gsd-verifier)_

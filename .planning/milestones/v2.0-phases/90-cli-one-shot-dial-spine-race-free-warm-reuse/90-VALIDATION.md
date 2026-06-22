---
phase: 90
slug: cli-one-shot-dial-spine-race-free-warm-reuse
status: approved
nyquist_compliant: true
wave_0_complete: true
created: 2026-06-21
---

# Phase 90 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib `testing`, `testing/synctest` GA on toolchain go1.26) |
| **Config file** | none — uses repo `go.mod` |
| **Quick run command** | `go test ./internal/forwarder/... ./internal/cli/...` |
| **Full suite command** | `go vet ./... && go test ./...` |
| **E2E oracle command** | `HELIX_BIN="$(pwd)/helix" go test ./internal/cli/... -run CLI_` (real-subprocess, gated on HELIX_BIN; `//go:build !windows`) |
| **Estimated runtime** | ~30–90 seconds (unit); E2E adds daemon spawn cost |

---

## Sampling Rate

- **After every task commit:** Run `go test` on the touched package(s)
- **After every plan wave:** Run `go vet ./... && go test ./...`
- **Before `/gsd-verify-work`:** Full suite + E2E oracle (with `HELIX_BIN` set) must be green
- **Max feedback latency:** ~90 seconds (unit); E2E excluded from per-commit loop

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 90-01-T1 | 90-01 | 1 | CLI-03 (gofrs/flock dep — provenance gate before install) | T-90-SC | supply-chain: maintainer/repo/version confirmed before `go get`; go.sum pins hashes | checkpoint + build | `grep -c 'github.com/gofrs/flock v0.13.0' go.mod go.sum && go build ./...` | ✅ planned | ⬜ pending |
| 90-01-T2 | 90-01 | 1 | CLI-03 (cross-process startup lock, double-checked connect) | T-90-01, T-90-02, T-90-03 | exactly-one spawn under N goroutines; portable lock (no syscall.Flock) | synctest unit | `go test ./internal/forwarder/... -run Race -count=1` | ✅ scaffold in-plan | ⬜ pending |
| 90-02-T1 | 90-02 | 1 | CLI-04 (`helix` no-args → grouped help, exit 0, no stdio session) | T-90-04, T-90-05 | no MCP session opened on the no-arg path | unit | `go test ./internal/cli/... -run Root -count=1` | ✅ scaffold in-plan | ⬜ pending |
| 90-03-T1 | 90-03 | 2 | CLI-01 (client-side gRPC↔MCP-SDK transport mirror, zero proto) | T-90-07 | zero-proto invariant (`git diff api/proto/` empty) | unit | `go test ./internal/forwarder/... -run ClientTransport -count=1 && git diff --exit-code api/proto/` | ✅ scaffold in-plan | ⬜ pending |
| 90-03-T2 | 90-03 | 2 | CLI-01, CLI-02 (one-shot CallTool + verb spine; SDK handshake, clean teardown) | T-90-06, T-90-08, T-90-09, T-90-10 | initialize handshake (no hand-framing); ordered defers (no `error` session) | unit | `go test ./internal/cli/... -run Verb -count=1 && git diff --exit-code api/proto/` | ✅ scaffold in-plan | ⬜ pending |
| 90-04-T1 | 90-04 | 3 | TEST-01, CLI-01 (E2E oracle scaffold + one-shot round-trip vs MCP path) | T-90-12 | sandbox env allowlist + isolated process group + Cleanup | e2e (real subprocess) | `HELIX_BIN="$(pwd)/helix" go test ./internal/cli/... -run CLI_E2E -count=1` | ✅ scaffold in-plan | ⬜ pending |
| 90-04-T2 | 90-04 | 3 | CLI-02 (warm-reuse 2nd-call SLO — measured p50, recorded multiple, asserted) | — | SLO asserted against recorded value, not a guess | e2e (real subprocess) | `HELIX_BIN="$(pwd)/helix" go test ./internal/cli/... -run CLI_WarmReuseSLO -count=1` | ✅ scaffold in-plan | ⬜ pending |
| 90-04-T3 | 90-04 | 3 | CLI-03 (real) (N parallel cold → exactly one daemon PID) | T-90-11, T-90-13 | single daemon PID under N parallel cold callers; no orphans | e2e (real subprocess) | `HELIX_BIN="$(pwd)/helix" go test ./internal/cli/... -run CLI_ParallelColdSingleDaemon -count=1` | ✅ scaffold in-plan | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*
*Task IDs finalized by the planner; this map is the requirement→test contract.*

**Requirement coverage:** CLI-01 (90-03-T1, 90-03-T2, 90-04-T1) · CLI-02 (90-03-T2, 90-04-T2) · CLI-03 (90-01-T1, 90-01-T2, 90-04-T3) · CLI-04 (90-02-T1) · TEST-01 (90-04-T1). All five phase requirements have at least one automated verification.

---

## Wave 0 Requirements

- [x] CLI-over-daemon E2E harness scaffold (reuse `internal/eval/sandbox` + `bench/runtime` real-subprocess pattern) — gated on `HELIX_BIN` — **created in-plan: 90-04 Task 1 (`internal/cli/cli_e2e_test.go` scaffold + `resolveHelixBin`), shared by 90-04 Tasks 2 & 3**
- [x] synctest-based race test scaffold for the cross-process startup lock algorithm — **created in-plan: 90-01 Task 2 (`internal/forwarder/dial_race_test.go`, synctest fan-out against the `spawnDaemon` seam)**
- [x] `go get github.com/gofrs/flock@v0.13.0` added to go.mod (cross-process lock dep) — **created in-plan: 90-01 Task 1 (blocking provenance checkpoint, then `go get` + `go mod tidy`)**

All Wave-0 scaffolds (test files + the new dependency) are created within Phase 90's own
plans before they are depended upon — there is no out-of-phase Wave-0 prerequisite.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Numeric 2nd-call latency SLO value | CLI-02 | SLO threshold is measured empirically, then recorded; the *assertion* is automated once the number is set | Run warm-reuse oracle (90-04-T2), record observed p50, set SLO as a multiple of median in SUMMARY |
| `gofrs/flock` package provenance | CLI-03 | Research flagged provenance not seam-verified | Confirm `github.com/gofrs/flock@v0.13.0` is the intended well-known library before `go get` (90-01-T1 blocking checkpoint) |

*All other phase behaviors have automated verification.*

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 90s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-06-21

---
phase: 90
slug: cli-one-shot-dial-spine-race-free-warm-reuse
status: draft
nyquist_compliant: false
wave_0_complete: false
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
| **E2E oracle command** | `HELIX_BIN="$(pwd)/helix" go test ./internal/cli/... -run E2E` (real-subprocess, gated on HELIX_BIN; `//go:build !windows`) |
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
| 90-xx | TBD | — | CLI-01 (one-shot tools/call over StreamMCP, zero proto) | — | N/A | unit | `go test ./internal/forwarder/...` | ❌ W0 | ⬜ pending |
| 90-xx | TBD | — | CLI-02 (auto-start cold, reuse warm) | — | N/A | integration | `HELIX_BIN=... go test ./internal/cli/... -run Warm` | ❌ W0 | ⬜ pending |
| 90-xx | TBD | — | CLI-03 (race-free single daemon under N parallel) | — | exactly-one daemon PID | synctest + subprocess | `go test ./internal/forwarder/... -run Race` | ❌ W0 | ⬜ pending |
| 90-xx | TBD | — | CLI-04 (`helix` no-args → grouped help, exit 0, no stdio session) | — | no MCP session opened | unit | `go test ./internal/cli/... -run Root` | ❌ W0 | ⬜ pending |
| 90-xx | TBD | — | TEST-01 (CLI-over-daemon E2E oracle, real subprocess) | — | N/A | e2e | `HELIX_BIN=... go test ./internal/cli/... -run E2E` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*
*Task IDs finalized by the planner; this map is the requirement→test contract.*

---

## Wave 0 Requirements

- [ ] CLI-over-daemon E2E harness scaffold (reuse `evalsandbox` / `bench/runtime/subprocess` real-subprocess pattern) — gated on `HELIX_BIN`
- [ ] synctest-based race test scaffold for the cross-process startup lock algorithm
- [ ] `go get github.com/gofrs/flock@v0.13.0` added to go.mod (cross-process lock dep)

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Numeric 2nd-call latency SLO value | CLI-02 | SLO threshold is measured empirically, then recorded; the *assertion* is automated once the number is set | Run warm-reuse oracle, record observed p50, set SLO as a multiple of median in SUMMARY |
| `gofrs/flock` package provenance | CLI-03 | Research flagged provenance not seam-verified | Confirm `github.com/gofrs/flock@v0.13.0` is the intended well-known library before `go get` |

*All other phase behaviors have automated verification.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 90s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending

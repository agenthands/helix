---
phase: 92
slug: terse-output-renderer-re-targeted-contract-oracle
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-06-21
---

# Phase 92 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib `testing`); CLI stdout goldens |
| **Quick run command** | `go test ./internal/cli/...` |
| **Full suite command** | `go vet ./... && go test ./...` |
| **Goldens / oracle** | `go test ./internal/cli/... -run 'Render|Golden|Terse'` |
| **Behavioral oracle** | `HELIX_BIN="$(pwd)/helix" go test ./internal/cli/... -run 'E2E|Chain'` (real subprocess) |
| **Estimated runtime** | ~30–90 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test` on the touched package(s)
- **After every plan wave:** Run `go vet ./... && go test ./...`
- **Before `/gsd-verify-work`:** Full suite green + per-verb stdout goldens green + behavioral chaining oracle RAN (HELIX_BIN)
- **Max feedback latency:** ~90 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Requirement | Test Type | Automated Command | Status |
|---------|------|-------------|-----------|-------------------|--------|
| 92-xx | TBD | OUT-01 (terse `relpath:line:col<TAB>payload`) | render unit | `go test ./internal/cli/... -run Render` | ⬜ pending |
| 92-xx | TBD | OUT-02 (1-based workspace-relative coords; --abs) | render unit | `go test ./internal/cli/... -run 'Coord|Abs'` | ⬜ pending |
| 92-xx | TBD | OUT-03 (deterministic sort+dedup; byte-identical N runs) | golden | `go test ./internal/cli/... -run 'Golden|Determinism'` | ⬜ pending |
| 92-xx | TBD | OUT-04/06 (no ANSI off-TTY; NO_COLOR; --color) | unit | `go test ./internal/cli/... -run Color` | ⬜ pending |
| 92-xx | TBD | OUT-05 (typed-error stderr prefix + per-kind exit code) | unit + e2e | `go test ./internal/cli/... -run 'Error|ExitCode'` | ⬜ pending |
| 92-xx | TBD | OUT-07 (global --json/--color/--abs flags honored) | unit | `go test ./internal/cli/... -run 'Flags|JSON'` | ⬜ pending |
| 92-xx | TBD | OUT (SC#2 nav: locus+enclosing+snippet; copy-paste chain) | behavioral e2e | `HELIX_BIN=... go test ./internal/cli/... -run Chain` | ⬜ pending |
| 92-xx | TBD | TEST-02 (contract oracle re-targeted to CLI stdout goldens) | golden | `go test ./internal/cli/... -run Contract` | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*
*Task IDs finalized by the planner; this map is the requirement→test contract.*

---

## Wave 0 Requirements

- [ ] per-verb CLI stdout golden fixtures (re-targeted from the MCP JSON goldens)
- [ ] render-class map test scaffold (locus-list / tree / opaque per tool)
- [ ] error-kind → stderr-prefix + exit-code table test scaffold

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Real-TTY color emission | OUT-04 | Genuine TTY behavior is hard to assert in CI (tests assert the piped/no-color path); a quick manual check confirms color renders on a real terminal | Run a verb in an interactive terminal, confirm color; pipe it, confirm zero ANSI |

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

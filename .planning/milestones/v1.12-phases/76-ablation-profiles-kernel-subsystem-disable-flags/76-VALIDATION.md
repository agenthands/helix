---
phase: 76
slug: ablation-profiles-kernel-subsystem-disable-flags
status: approved
nyquist_compliant: true
wave_0_complete: false
created: 2026-06-16
---

# Phase 76 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Derived from 76-RESEARCH.md §"Validation Architecture". Two hard-fail gates
> (zero `lspool.lsp.*` OTel spans under no_lsp; green→red `make vet` flip) are
> first-class automatable tests.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go standard `testing` + `golang.org/x/tools/go/analysis/analysistest` (analyzer) + in-memory OTel span recorder (trace-tap, model on `internal/kernel/jsonrpc/conn_trace_test.go`) |
| **Config file** | none (Go convention) |
| **Quick run command** | `go vet ./... && go test ./internal/lint/ablationleakage/... ./internal/profile/... -count=1` |
| **Full suite command** | `make test` (runs `make vet` then `go test ./...`) |
| **Estimated runtime** | ~60–120 seconds (quick); full suite minutes (LSP fixtures) |

---

## Sampling Rate

- **After every task commit:** Run `go vet ./... && go test ./internal/lint/ablationleakage/... ./internal/profile/... -count=1`
- **After every plan wave:** Run `make vet && go test ./internal/kernel/... ./internal/daemon/... -count=1`
- **Before `/gsd-verify-work`:** `make test` fully green
- **Max feedback latency:** ~120 seconds (quick command)

---

## Per-Task Verification Map

> Task IDs are assigned by the planner; this map keys behaviors to phase
> requirements and the Wave-0 test files that must exist. The planner threads
> each row's automated command into the matching task's `<acceptance_criteria>`.

| Behavior | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|----------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 4 bench YAMLs load + golden tool-surface per arm | 1 | ABLATE-02 | — | Each arm's tool surface matches golden | unit | `go test ./internal/profile/ -run TestBenchProfiles -count=1` | ❌ W0 | ⬜ pending |
| Loader rejects unknown mode name (fail-closed) | 1 | ABLATE-02 | T-tampering | Unknown mode → typed error, not silent no-op | unit | `go test ./internal/profile/ -run TestLoaderRejectsUnknownMode -count=1` | ❌ W0 | ⬜ pending |
| no_lsp emits zero `lspool.lsp.*` spans | 2 | ABLATE-05 | T-infodisc | Structural: no live LSP to emit spans | integration (trace-tap) | `go test ./internal/daemon/ -run TestNoLSPZeroSpans -count=1` | ❌ W0 | ⬜ pending |
| `SetEnrichFn` skipped + no-op notifier under flag | 2 | ABLATE-05 | — | Null-object injection at daemon init | unit | `go test ./internal/daemon/ -run TestNoLSPWiring -count=1` | ❌ W0 | ⬜ pending |
| Structured-edit tools return `Unsupported` under flag | 2 | ABLATE-07 | T-eop (ablation integrity) | Back-channel call → typed `Unsupported` | unit | `go test ./internal/kernel/edit/ -run TestStructuredEditDisabled -count=1` | ❌ W0 | ⬜ pending |
| `replace_in_file` exact-match-only under flag | 2 | ABLATE-07 (D-05) | — | Fuzzy cascade skipped; plain-return path | unit | `go test ./internal/kernel/fileops/ -run TestReplaceInFileNoFuzzyWhenDisabled -count=1` | ❌ W0 | ⬜ pending |
| Analyzer fires on deliberate testdata violation | 1 | ABLATE-08 | T-eop | Forbidden import edge → diagnostic | unit (analysistest) | `go test ./internal/lint/ablationleakage/ -count=1` | ❌ W0 | ⬜ pending |
| `make vet` green→red flip demonstrable | 3 | ABLATE-08 | — | Regression flips `make vet` red | gate (analysistest fixture) | `make vet` | n/a | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/profile/bench_profiles_test.go` — golden tool-surface per bench arm (ABLATE-02)
- [ ] `internal/profile/loader_test.go` — extend with unknown-mode-rejection case (ABLATE-02)
- [ ] `internal/daemon/no_lsp_wiring_test.go` — zero-span trace-tap + `SetEnrichFn`-skip (ABLATE-05); model on `internal/kernel/jsonrpc/conn_trace_test.go` (in-memory span recorder)
- [ ] `internal/kernel/edit/structured_edit_disabled_test.go` — `Unsupported` guard (ABLATE-07)
- [ ] `internal/kernel/fileops/replace_in_file_disabled_test.go` — exact-match-only (ABLATE-07/D-05)
- [ ] `internal/lint/ablationleakage/analyzer_test.go` + `testdata/src/...` fixtures — green→red (ABLATE-08)

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| — | — | — | — |

*All phase behaviors have automated verification. The green→red `make vet` flip
is proven non-manually via `analysistest` `// want` fixtures in `testdata/`.*

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references (test files created RED-first inside each TDD task)
- [x] No watch-mode flags
- [x] Feedback latency < 120s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-06-16 (plan-checker VERIFICATION PASSED)

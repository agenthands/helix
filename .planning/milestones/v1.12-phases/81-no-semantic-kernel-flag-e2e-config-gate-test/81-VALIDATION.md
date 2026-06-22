---
phase: 81
slug: no-semantic-kernel-flag-e2e-config-gate-test
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-06-20
---

# Phase 81 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go standard `testing` + `golang.org/x/tools/go/analysis/analysistest` (vet analyzer) |
| **Config file** | none (Go convention) |
| **Quick run command** | `go test ./internal/daemon/... ./internal/lint/ablationleakage/... ./internal/profile/... -count=1` |
| **Full suite command** | `go test ./... && make vet` |
| **Estimated runtime** | ~60–120 seconds (quick); full suite longer |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/daemon/... ./internal/lint/ablationleakage/... ./internal/profile/... -count=1`
- **After every plan wave:** Run `go test ./... -count=1 && go vet ./...`
- **Before `/gsd-verify-work`:** `go test ./... && make vet` fully green
- **Max feedback latency:** ~120 seconds

---

## Per-Task Verification Map

| Behavior | Requirement | Crit | Test Type | Automated Command | File Exists | Status |
|----------|-------------|------|-----------|-------------------|-------------|--------|
| `bench_disabled: true` → `effSemanticDisabled` resolved at composition root | ABLATE-06 | #1 | unit | `go test ./internal/daemon/ -run TestEffSemanticDisabled -count=1` | ❌ W0 | ⬜ pending |
| `get_repo_map`/`get_context` return `source == "tree_sitter"` under gate | ABLATE-06 | #1 | unit | `go test ./internal/semantic/integ/ -run TestChooseSource -count=1` (+ new daemon-wiring test) | ✅ ladder / ❌ wiring W0 | ⬜ pending |
| `symbolsLookupFn` returns `NoopLookup`, cfgGate disabled when gated (build-but-block) | ABLATE-06 | #1 | unit | `go test ./internal/daemon/ -run TestSemanticGateForcesNoop -count=1` | ❌ W0 | ⬜ pending |
| SemanticSkill accessors left nil under gate (no direct DuckDB read path) | ABLATE-06 | #1 | unit | `go test ./internal/daemon/ -run TestSemanticSkillAccessorsGated -count=1` | ❌ W0 | ⬜ pending |
| `helix_semantic_store_reads_total` increments on read; stays 0 when gated | ABLATE-06 | #2 | unit | `go test ./internal/semantic/store/ -run TestReadCounter -count=1` | ❌ W0 (net-new) | ⬜ pending |
| E2E `no_semantic` cell: counter == 0 else fail cell | ABLATE-06 | #2 | integration | `go test ./bench/runtime/ -run TestNoSemanticZeroReads -count=1` | ❌ W0 | ⬜ pending |
| vet call-site check fires on direct read outside gate (green→red) | ABLATE-06 | #3 | unit (analysistest) | `go test ./internal/lint/ablationleakage/ -count=1` | ✅ harness / ❌ fixtures W0 | ⬜ pending |
| `make vet` fails on the red testdata violation | ABLATE-06 | #3 | smoke | `make vet` | ✅ chain / ❌ new check W0 | ⬜ pending |
| `bench-no-semantic.yaml` carries `disable_semantic_subsystem: true` | ABLATE-06 | #1 | unit | `go test ./internal/profile/ -run TestBenchNoSemanticProfile -count=1` | ⚠️ assertion must FLIP false→true | ⬜ pending |
| MODE.md documents gate key + consumer enumeration | ABLATE-06 | #4 | doc/manual | review `bench/runners/your_agent_no_semantic/MODE.md` | ✅ exists / needs rewrite | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/daemon/semantic_gate_test.go` — composition-root resolution + Noop/disabled-gate forcing + SemanticSkill accessor gating
- [ ] `internal/semantic/store/reads_counter_test.go` — net-new read counter increments / stays-0
- [ ] register `helix_semantic_store_reads_total` in `internal/obs/metrics.go` (covered by store test)
- [ ] `internal/lint/ablationleakage/testdata/src/{badgate,goodgate}/` — green→red call-site fixtures with `// want` comments
- [ ] `bench/runtime/no_semantic_zero_reads_test.go` — E2E counter==0 assertion + fail-cell on violation
- [ ] **Assertion flip:** `internal/profile/bench_profiles_test.go:142` — update from "no kernel flag" to assert `disable_semantic_subsystem: true` (deliberate behavior change, not a regression)
- [ ] Framework install: none (Go stdlib + already-vendored `analysistest`)

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| `MODE.md` accurately documents the gate key + strangler-fig consumer enumeration | ABLATE-06 (#4) | Prose/doc quality — no automated assertion of completeness | Review `bench/runners/your_agent_no_semantic/MODE.md`: confirm it names `disable_semantic_subsystem`, the precedence rule, and lists all gated consumers |

---

## Validation Sign-Off

- [ ] All tasks have automated verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 120s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending

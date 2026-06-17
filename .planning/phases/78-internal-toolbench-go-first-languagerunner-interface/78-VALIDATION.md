---
phase: 78
slug: internal-toolbench-go-first-languagerunner-interface
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-06-17
---

# Phase 78 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Seeded from RESEARCH.md "## Validation Architecture". The Per-Task map is filled
> by the planner/executor as `IT-go-*` tasks land; the success-criteria map below
> is the fixed contract this phase is graded against.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` (go 1.23) + `go test -json` for the bench corpus |
| **Config file** | none — Go convention |
| **Quick run command** | `go test ./bench/... -count=1` |
| **Full suite command** | `go vet ./... && go test ./...` |
| **Bench gate** | `make bench-quick` (build helix, scripted smoke, ≤90s, ≥1 cell succeeds) |
| **Estimated runtime** | quick ~seconds; `make bench-quick` ≤ 90 s (hard budget) |

---

## Sampling Rate

- **After every task commit:** Run `go test ./bench/... -count=1` (+ `gofmt -l` clean, `go vet ./bench/...` clean)
- **After every plan wave:** Run `go vet ./... && go test ./...` + `make bench-quick`
- **Before `/gsd-verify-work`:** Full Go corpus green — `go run ./cmd/helix-bench run --benchmarks=internal-toolbench --languages=go --agent=scripted` exits 0 with 10/10 cells
- **Max feedback latency:** ≤ 90 s (the `bench-quick` budget)

---

## Success Criteria → Test Map (the 4 criteria + 3 requirements, independently verifiable)

| Criterion / Req | Behavior | Test Type | Automated Command / Assertion | Exists |
|------|----------|-----------|-------------------------------|--------|
| C1 / TOOLBENCH-01 | 10 capability classes documented + ≥1 Go fixture each | doc + corpus check | Go test reads every `internal-toolbench/go/*/task.json`, asserts all 10 `capability` values covered; `CAPABILITIES.md` lists 10 classes | ❌ W0 |
| C2 / TOOLBENCH-02 | Go full run passes; coverage 10/10 | integration | `go run ./cmd/helix-bench run --benchmarks=internal-toolbench --languages=go --agent=scripted` exits 0, 10/10 cells; coverage = `Capabilities()` ∩ `task.json` = 10/10 | ❌ W0 |
| C2 / TOOLBENCH-02 | `RunTests` wraps `go test ./... -json` | unit | `GoRunner.RunTests(seedRepo)` returns `Passed=true`, parses ≥1 `TestResult` from the test2json stream | ❌ W0 |
| C3 / TOOLBENCH-10 | `LanguageRunner` defined; conformance + vet | compile-time + unit | `var _ languages.LanguageRunner = (*golang.GoRunner)(nil)` compiles; `go vet ./bench/...` clean | ❌ W0 |
| C4 | `IT-go-*` namespace, zero `T-67-*` collision | static | test asserting every task id matches `^IT-go-` and none matches `^T-67-`; `PHASE67_CROSSWALK.md` exists | ❌ W0 |
| D-03 | per-cell store isolation under `--parallel` | integration | run 2 store-on cells `--parallel=2`; assert both succeed, distinct `.helix/semantic.duckdb` files, `RejectedForeignPid == 0` | ❌ W0 |
| D-09 | `bench-quick` green post-cutover | gate | `make bench-quick` exits 0 ≤ 90 s, pinned to migrated store-OFF seed | ✅ target exists; must stay green |
| D-10 | verify.sh fallback when no runner registered | unit | a cell with `RunnerFor() == nil` falls back to `runVerify`; extend existing `cell_test.go` | partial — extend |

---

## Per-Task Verification Map

> Populated by the planner/executor as `IT-go-<capability>-<n>` tasks land. Each task
> maps to a row above. Format retained for execution-time tracking.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 78-NN-NN | NN | N | TOOLBENCH-NN | — | N/A | unit/integration | `go test ./bench/...` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `bench/languages/runner.go` — `LanguageRunner` interface + registry + `TestOutcome`/`TestResult` types (none exist; dir is `.gitkeep`)
- [ ] `bench/languages/go/runner.go` + `runner_test.go` — Go runner + interface-conformance test
- [ ] `bench/datasets/internal-toolbench/CAPABILITIES.md` — 10-class documentation
- [ ] `bench/datasets/internal-toolbench/PHASE67_CROSSWALK.md` — inspiration map (no code migration)
- [ ] Corpus-coverage test — reads all `task.json` `capability` fields, asserts 10/10 vs declared `Capabilities()`
- [ ] `--parallel` store-isolation integration test for the incremental-update fixture

*Framework install: none — Go testing is built in.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| `CAPABILITIES.md` / `PHASE67_CROSSWALK.md` prose accuracy | TOOLBENCH-01 / C4 | Human-facing documentation quality | Reviewer reads both docs; confirms 10 classes described and crosswalk maps `T-67-*` → `IT-go-*` as inspiration only |

*All graded behaviors have automated verification; the docs above also carry an automated existence/structure check (C1, C4).*

---

## Validation Sign-Off

- [ ] All tasks have `<acceptance_criteria>` with automated verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references (runner.go, go/runner.go, CAPABILITIES.md, crosswalk, coverage test, parallel test)
- [ ] No watch-mode flags
- [ ] Feedback latency < 90 s (`bench-quick` budget protected)
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending

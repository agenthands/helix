---
phase: 78
slug: internal-toolbench-go-first-languagerunner-interface
status: approved
nyquist_compliant: true
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

> Seeded from the 5 landed plans (commit `d85078cf`). `File Exists` = is the test/target on disk
> *yet* — all ❌ W0 at plan time; execute-phase flips to ✅ as each wave lands.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 78-01-01 | 01 | 1 | TOOLBENCH-10, TOOLBENCH-02 | — | N/A | tdd/unit | `go test ./bench/languages/... -count=1` | ❌ W0 | ⬜ pending |
| 78-01-02 | 01 | 1 | TOOLBENCH-02 | T-78-01 / T-78-02 | additive `WithWorkingDir`; ctx-cancel kills `go test` subprocess | unit | `go build ./... && go test ./internal/eval/sandbox/...` | ❌ W0 | ⬜ pending |
| 78-02-01 | 02 | 2 | TOOLBENCH-02, TOOLBENCH-10 | T-78-03 | `<lang>` segment V5-validated (`validateMatrixID`/`validateCellKey`) | tdd/unit | `go test ./bench/runtime/... -count=1` | ❌ W0 | ⬜ pending |
| 78-02-02 | 02 | 2 | TOOLBENCH-02 | T-78-05 | single clean cutover, no stale `toolbench-go` | integration/gate | `go test ./... && make bench-quick` | ❌ W0 | ⬜ pending |
| 78-03-01 | 03 | 3 | TOOLBENCH-01, TOOLBENCH-02 | T-78-06 / T-78-07 | hermetic dependency-free fixtures, offline | corpus/integration | `go vet ./...` per fixture; pre-fail/post-pass authored | ❌ W0 | ⬜ pending |
| 78-03-02 | 03 | 3 | TOOLBENCH-01, TOOLBENCH-02 | T-78-06 | hermetic; `expect_error`+recovery for failure_handling | corpus/integration | `go vet ./...` per fixture; pre-fail/post-pass authored | ❌ W0 | ⬜ pending |
| 78-04-01 | 04 | 3 | TOOLBENCH-01, TOOLBENCH-02 | T-78-10 | store-ON fixture hermetic; in-process refresh, no network | integration | `grep refresh_semantic_graph` + `go vet ./...` in fixture | ❌ W0 | ⬜ pending |
| 78-04-02 | 04 | 3 | TOOLBENCH-02 | T-78-04 / T-78-09 | per-cell `.helix/semantic.duckdb` isolation under `--parallel=2` | tdd/integration | `go test ./bench/runtime/ -run 'StoreIsolation\|Parallel'` | ❌ W0 | ⬜ pending |
| 78-05-01 | 05 | 4 | TOOLBENCH-01, TOOLBENCH-02, TOOLBENCH-10 | T-78-11 / T-78-12 | coverage from `capability` field (not id); gap-detection proves non-vacuous 10/10 | tdd/unit | `go test ./bench/languages/... -count=1` | ❌ W0 | ⬜ pending |
| 78-05-02 | 05 | 4 | TOOLBENCH-01 | — | N/A (docs) | doc-existence | `test -f CAPABILITIES.md && test -f PHASE67_CROSSWALK.md` | ❌ W0 | ⬜ pending |
| 78-05-03 | 05 | 4 | TOOLBENCH-01, TOOLBENCH-02, TOOLBENCH-10 | — | N/A | checkpoint (blocking human-verify) | `helix-bench run --benchmarks=internal-toolbench --languages=go` = 10/10 + `go test ./bench/languages/ -run Coverage` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

> Note: `wave_0_complete: false` in frontmatter is correct at plan time — Wave 0 (Plan 01's
> `runner.go`/`go/runner.go`) is *specified and assigned* but not yet *built*; execute-phase
> sets it `true` once Wave 1 lands. `nyquist_compliant: true` certifies the validation
> *strategy* (every task has an automated verify; sampling continuity holds; all MISSING
> references are assigned to a Wave-0 task), which is satisfiable at plan time.

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

- [x] All tasks have `<acceptance_criteria>` with automated verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify (every task carries an automated verify)
- [x] Wave 0 covers all MISSING references (runner.go + go/runner.go → Plan 01; CAPABILITIES.md + crosswalk + coverage test → Plan 05; parallel store-isolation test → Plan 04)
- [x] No watch-mode flags
- [x] Feedback latency < 90 s (`bench-quick` budget protected; pinned to the store-OFF seed)
- [x] `nyquist_compliant: true` set in frontmatter
- [ ] `wave_0_complete: true` — deferred to execute-phase (Wave 0 not yet built at plan time)

**Approval:** approved 2026-06-17 (strategy approved at plan time; `wave_0_complete` flips during execution)

---
phase: 78-internal-toolbench-go-first-languagerunner-interface
plan: 05
subsystem: testing
tags: [bench, internal-toolbench, coverage-aggregator, capabilities, phase67-crosswalk, namespace, D-06, D-11, C1, C2, C4, tdd]
requires:
  - phase: 78-01
    provides: GoRunner.Capabilities() — the 10 declared Capability enum constants (coverage "declared" set)
  - phase: 78-02
    provides: internal-toolbench/<lang>/ corpus layout + task.json capability field convention (D-06 source of truth)
  - phase: 78-03
    provides: 8 store-off Go capability fixtures (semantic_view, lsp_diagnostics, rename_safety, fuzzy_search, call_graph, dependency_graph, context_minimization, failure_handling)
  - phase: 78-04
    provides: store-ON IT-go-incremental-update-1 + patch_apply seed — completing the 10/10 corpus
provides:
  - "bench/languages/coverage.go — Coverage() aggregator: declared (Capabilities()) ∩ covered (task.json capability fields), reports CoverageReport{Declared,Covered,Missing} (D-11)"
  - "bench/languages/coverage_test.go — Go 10/10 assertion, synthetic-gap detection (D-11), ^IT-go-/not-^T-67- namespace test (C4), docs-exist test"
  - "bench/datasets/internal-toolbench/CAPABILITIES.md — all 10 capability classes documented with tool(s)+fixture (criterion C1)"
  - "bench/datasets/internal-toolbench/PHASE67_CROSSWALK.md — inspiration-only T-67-*→IT-go-* map, zero-collision preamble, no code migration (criterion C4)"
  - "Phase 78 closure: full Go corpus run 10/10 cells green (criterion C2 phase gate, human-verify approved)"
affects:
  - Phase 79 (evaluators consume the documented 10-capability corpus + the coverage aggregator)
  - Phase 85 (forward language tiers reuse Coverage() per-language; CAPABILITIES.md documents the ≥8 / ≥6 thresholds)
tech-stack:
  added: []
  patterns:
    - "Per-language coverage as declared ∩ covered: runner Capabilities() (declared) intersected with task.json capability fields (covered, D-06 source of truth — NOT the id string), gaps listed in Missing (D-11)"
    - "Gap-detection TDD guard: a t.TempDir() synthetic corpus omitting one capability asserts it surfaces in Missing — proves the aggregator detects absence, not a rubber-stamp 10/10 (T-78-12)"
    - "Namespace invariant as a static test: every corpus task id matches ^IT-go- and none matches ^T-67- (C4 zero-collision enforced in code, not just docs)"
key-files:
  created:
    - bench/languages/coverage.go
    - bench/languages/coverage_test.go
    - bench/datasets/internal-toolbench/CAPABILITIES.md
    - bench/datasets/internal-toolbench/PHASE67_CROSSWALK.md
  modified: []
key-decisions:
  - "Coverage decodes ONLY the task.json `capability` field (D-06 source of truth), deliberately not the id string — mitigates T-78-11 (id/capability disagreement still counts by capability)"
  - "Declared count derives from the runner's Capabilities() set (len of the dedup set); a task tagged with a capability the runner does not declare cannot inflate Covered"
  - "Helix native tool names in CAPABILITIES.md (get_symbol_overview singular, get_call_hierarchy) verified against the registered Helix tool registry (internal/kernel/symbols/skill.go) + README inventory during the human-verify gate — NOT the serena/SMTC plugin names; left as-is"
  - "PHASE67_CROSSWALK.md is inspiration-only: T-67-* are Phase 67 PLANNING task IDs (no on-disk corpus, RESEARCH §Phase 67 Crosswalk); the IT-go-* fixtures were authored fresh on the Phase 77 bench spine, zero code migration"
patterns-established:
  - "Coverage aggregator (bench/languages/coverage.go): reusable per-language declared-vs-covered report; Phase 85 calls it for Python/Rust/TS/C#/C++ against their thresholds"
  - "Corpus namespace test: ^IT-go- / not ^T-67- regex assertion over every task.json id — the model for future language namespaces (IT-py-*, IT-rust-*, ...)"
requirements-completed: [TOOLBENCH-01, TOOLBENCH-02, TOOLBENCH-10]

duration: ~30min
completed: 2026-06-17
---

# Phase 78 Plan 05: Capability Docs + Corpus-Coverage Aggregator (Go 10/10) Summary

**Phase-closing plan: the `Coverage()` aggregator reports Go 10/10 (declared `Capabilities()` ∩ covered `task.json` fields, D-06/D-11), CAPABILITIES.md documents all 10 capability classes (C1), PHASE67_CROSSWALK.md is an inspiration-only T-67-*→IT-go-* map with a zero-collision namespace test (C4), and the full Go corpus run is green at 10/10 cells (C2 phase gate, human-verify approved).**

## Performance

- **Duration:** ~30 min
- **Tasks:** 3 (Task 1 TDD RED+GREEN, Task 2 docs, Task 3 human-verify gate)
- **Files created:** 4

## Accomplishments

- **Corpus-coverage aggregator (D-06/D-11):** `bench/languages/coverage.go` walks `<root>/internal-toolbench/<lang>/*/task.json`, decodes each cell's `capability` field (the D-06 source of truth — deliberately NOT the id string, mitigating T-78-11), intersects it with the runner's declared `Capabilities()`, and returns `CoverageReport{Declared, Covered, Missing}`. Go reports **10/10, Missing empty** — the criterion C2 gate.
- **Gap-detection guard (D-11 / T-78-12):** the aggregator is proven NOT a rubber stamp — a synthetic `t.TempDir()` corpus omitting `CapFailureHandling` reports `Covered=9, Missing=[failure_handling]`.
- **Namespace invariant in code (C4):** every Go fixture id matches `^IT-go-` and none matches `^T-67-` — a static test, not just doc prose.
- **CAPABILITIES.md (C1):** all 10 capability classes documented — each names its enum value, what it exercises, the Helix MCP tool(s) the scripted agent drives, and the covering Go fixture dir; plus the per-language threshold table (Go 10/10; Phase 85 forward ≥8 / ≥6).
- **PHASE67_CROSSWALK.md (C4):** inspiration-only map of Phase 67 `T-67-*` planning tasks → the `IT-go-*` fixtures they conceptually inspired, with an explicit no-code-migration / zero-collision preamble and a boundary statement.
- **Phase 78 closed:** the full Go corpus run reports **10/10 cells succeeded** (independently reproduced) and both docs were verified accurate at the human-verify gate.

## Task Commits

1. **Task 2: Author CAPABILITIES.md + PHASE67_CROSSWALK.md** — `f1c14e03` (docs)
2. **Task 1 (TDD RED): failing coverage aggregator + namespace tests** — `41f746a9` (test)
3. **Task 1 (TDD GREEN): corpus-coverage aggregator** — `2a04213b` (feat)
4. **Task 3: human-verify the full Go corpus run (10/10) + doc prose** — checkpoint gate (no code commit; operator approved)

**Plan metadata:** this SUMMARY + STATE.md + ROADMAP.md + REQUIREMENTS.md committed as `docs(78-05): ...`.

_Note: Task 2 (docs) was committed before Task 1's TDD pair so the docs-exist test (`TestDocsExist`) had its target files present; the RED→GREEN ordering for the aggregator itself is preserved (`test(78-05)` `41f746a9` → `feat(78-05)` `2a04213b`)._

## Files Created/Modified

- `bench/languages/coverage.go` — `Coverage(corpusRoot, lang, declared) (CoverageReport, error)` + `coveredCapabilities` helper; reads the `capability` field only (D-06), computes `Missing = declared − covered` (D-11).
- `bench/languages/coverage_test.go` — `TestCoverageGoIsTenOfTen` (live corpus 10/10), `TestCoverageDetectsGap` (synthetic gap, D-11), `TestNamespaceIsITGoZeroT67` (C4), `TestDocsExist` (C1/C4 existence).
- `bench/datasets/internal-toolbench/CAPABILITIES.md` — 10-class capability documentation (criterion C1).
- `bench/datasets/internal-toolbench/PHASE67_CROSSWALK.md` — inspiration-only crosswalk (criterion C4).

## Decisions Made

- **D-06 source of truth:** `coverage.go` decodes only the `capability` field, never parses the `IT-go-*` id — a fixture whose id and capability disagree is still counted by its capability (mitigates T-78-11).
- **Declared from the runner:** `Declared` is the size of the dedup'd `Capabilities()` set; a task tagged with a non-declared capability cannot inflate `Covered` (the loop iterates `declared`, not the corpus).
- **Tool names verified, not changed (human-verify gate):** CAPABILITIES.md uses the *registered Helix native* tool names — `get_symbol_overview` (singular) and `get_call_hierarchy` — confirmed against `internal/kernel/symbols/skill.go` and the README inventory. These are intentionally NOT the serena/SMTC plugin spellings; no doc edit was made (changing them would have introduced errors).
- **Crosswalk is inspiration-only:** `T-67-*` are Phase 67 planning task IDs with no on-disk corpus (RESEARCH §"Phase 67 Crosswalk"); the `IT-go-*` fixtures were authored fresh on the Phase 77 bench spine. No code ported; namespaces disjoint by construction.

## Deviations from Plan

None — plan executed exactly as written. (Task 2 docs were committed ahead of the Task 1 TDD pair so the docs-existence test had its targets; the aggregator's own RED→GREEN order is intact. This is commit ordering, not a scope change.)

## Issues Encountered

None — coverage 10/10 on first GREEN; the full corpus run reported 10/10 cells; both docs passed the operator's accuracy read.

## Human-Verify Gate (Task 3)

The plan's single Manual-Only Verification (VALIDATION.md). Before pausing, the full Go
corpus was run end-to-end. Operator-approved outcome (independently reproduced):

- **Full corpus run:** `helix-bench run --benchmarks=internal-toolbench --languages=go --agent=scripted` → exit 0, **10/10 cells succeeded** (all 10 capability fixtures, incl. the store-ON `incremental_update` cell).
- **Coverage 10/10:** `go test ./bench/languages/ -run Coverage` → Go reports Declared=10, Covered=10, Missing=[] (criterion C2).
- **Docs accurate:** CAPABILITIES.md (all 10 classes, correct tools+fixtures) and PHASE67_CROSSWALK.md (inspiration-only, disjoint namespaces) both confirmed accurate. The orchestrator double-checked the Helix native tool names against the actual tool registry — `get_symbol_overview` / `get_call_hierarchy` are correct; no edits needed.
- **Resume signal:** operator typed "approved".

## TDD Gate Compliance

Task 1 followed RED→GREEN: `test(78-05)` `41f746a9` (failing coverage + namespace + docs-exist tests) precedes `feat(78-05)` `2a04213b` (the aggregator that makes them pass). No REFACTOR commit was needed (the GREEN implementation was already clean: `go vet`/`gofmt` clean). The gap-detection test (`TestCoverageDetectsGap`) was authored in the RED commit and is load-bearing for D-11 (proves real gap detection, not a vacuous 10/10).

## Success Criteria Met

- **C1 (TOOLBENCH-01):** CAPABILITIES.md documents all 10 capability classes with enum value + tool(s) + fixture.
- **C2 (TOOLBENCH-02):** coverage reports Go 10/10; full corpus run exits 0 with 10/10 cells.
- **C4:** `IT-go-*` namespace with zero `T-67-*` collision proven by `TestNamespaceIsITGoZeroT67`; PHASE67_CROSSWALK.md is inspiration-only.
- **D-06/D-11:** aggregator reads the `capability` field (source of truth) and reports declared-vs-covered with explicit `Missing`.

## Threat Surface

- **T-78-11 (coverage from id instead of capability field)** — mitigated: `coverage.go` decodes only the `capability` field; the test `TestCoverageGoIsTenOfTen` counts by capability, never the `IT-go-*` id.
- **T-78-12 (silently-vacuous 10/10 rubber-stamp)** — mitigated: `TestCoverageDetectsGap` feeds a synthetic corpus missing one capability and asserts it appears in `Missing` (D-11 "gaps explicit").
- **T-78-SC (package installs)** — accept/verified: this plan installs zero external packages (two docs + one stdlib-only Go file).

No new external-input surface (a stdlib-only aggregator reading repo-tracked corpus files + two docs).

## Next Phase Readiness

- The internal-toolbench Go corpus is complete and documented: 10/10 capability fixtures, the `Coverage()` aggregator, and the two human-facing docs. Phase 79 evaluators can consume the full corpus.
- The `Coverage()` aggregator and the namespace-test pattern are reusable for Phase 85 (forward language tiers IT-py-*/IT-rust-*/… against the documented ≥8 / ≥6 thresholds).
- No blockers; Phase 78 closes at 5/5 plans (100%).

## Self-Check: PASSED

All 4 files present on disk (`coverage.go`, `coverage_test.go`, `CAPABILITIES.md`, `PHASE67_CROSSWALK.md`); commits `f1c14e03`, `41f746a9`, `2a04213b` present in git log; `go test ./bench/languages/ -run Coverage` green (Go 10/10).

---
*Phase: 78-internal-toolbench-go-first-languagerunner-interface*
*Completed: 2026-06-17*

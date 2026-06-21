---
phase: 86-crosscodeeval-repobench-adapters-multi-oracle-completion-gat
verified: 2026-06-21T05:05:39Z
status: passed
score: 4/4 must-haves verified
overrides_applied: 0
re_verification:
  previous_status: issues_found
  previous_score: code-review (1 BLOCKER + 4 WARNING)
  gaps_closed:
    - "CR-01: zero-value GateConfig{} now floors ESThreshold to DefaultESThreshold (gate fails closed)"
    - "WR-01: missing/short gold_snippet_index on a retrieval row now errors precisely instead of hard-failing the whole decode"
    - "WR-02: gold_snippet_index int64->int bound-checked before narrowing"
    - "WR-03: wholly-mismatched language tag now surfaces a distinct error instead of generic zero-tasks"
    - "WR-04: dataset cache written via temp+atomic rename so partial writes never become a hit"
  gaps_remaining: []
  regressions: []
---

# Phase 86: CrossCodeEval + RepoBench Adapters + Multi-Oracle Completion Gate Verification Report

**Phase Goal:** Two completion-only `dataset-loader-only` public-benchmark adapters (CrossCodeEval Py/Java/TS/C#; RepoBench-R/-C/-P Py/Java) + a multi-oracle completion gate (EM + edit-similarity + identifier-match all required, per-oracle configurable threshold, abstain → verified_correctness=false). Phase BUILT the HF parquet fetcher (net/http + arrow-go) + a minimal contamination-canary probe.
**Verified:** 2026-06-21T05:05:39Z
**Status:** passed
**Re-verification:** Yes — after code-review-fix (CR-01 BLOCKER + WR-01..04)

## Goal Achievement

### Observable Truths (ROADMAP Success Criteria)

| # | Truth | Status | Evidence |
| --- | --- | --- | --- |
| SC#1 (ADAPTER-CCE-01) | CrossCodeEval adapter scores Py/Java/TS/C#; EM + edit-sim + identifier scorers unit-tested vs CCE paper examples; HF fetched via the cached parquet pipeline this phase built; live ≥1-task/lang smoke honestly network-gated | ✓ VERIFIED | `bench/evaluators/{exactmatch,editsim,identmatch}` expose `EM`/`ES`/`Match` (pure, no I/O); hermetic tests pass (`go test ./bench/evaluators/... ok`). `crosscodeeval/loader.go` `Languages = [python,java,typescript,csharp]`; all 4 fixtures on disk (`fixtures/{python,java,typescript,csharp}/task.json`); `TestFixturesAreFourLanguages` loops over all 4. `fetch.go` is net/http+arrow-go into `$HELIX_CACHE_DIR`. Live fetch gated: `if os.Getenv("HELIX_BENCH_NETWORK")=="" { t.Skip(...) }` (fetch_test.go:142). |
| SC#2 (ADAPTER-REPO-01) | RepoBench-R (acc@k) / -C (EM/ES) / -P (pipeline) for Python+Java; metrics match published reference (network-gated); hermetic fixtures + honest gating | ✓ VERIFIED | `repobench/loader.go` models `TaskRetrieval`/`TaskCompletion`/`TaskPipeline`; `score.go` `AccAtK` for -R, reuses Plan-01 EM/ES for -C/-P (key-link confirmed). Fixtures `fixtures/{python,java}/{retrieval,completion}.json` on disk. Live reference-match gated: `score_test.go:91` documents the HELIX_BENCH_NETWORK-gated subset is "never the sole proof"; `fetch_test.go:152` skips offline. Tests pass (`bench/datasets/repobench ok`). |
| SC#3 (VERIFIED-03) | Multi-oracle gate in `bench/evaluators/VERIFIED.md`; EM+ES+identifier all required; per-oracle threshold CONFIGURABLE; abstain → verified_correctness=false; **CR-01 zero-value fail-OPEN FIXED** | ✓ VERIFIED | `gate.go:119-121` floors `cfg.ESThreshold<=0` to `DefaultESThreshold` (0.9) — CR-01 fixed in live source. `TestGrade_ZeroValueConfig_ESOracleStillBites` PASS (run directly) — `GateConfig{}` now bites the ES oracle. `TestGrade_Abstain_ExplicitFalse` PASS — abstain returns non-nil `&false`. `ESThreshold` is a live `GateConfig` field (configurable). `VERIFIED.md` documents all-three-required (line 19), configurable threshold (line 44), abstain→&false (line 56). `make verify-verified-md` clean. |
| SC#4 (canary) | Both adapters use the canary-emission probe + flag contaminated tasks; aggregator canary-pass-rate column populated; additive, goldens byte-identical | ✓ VERIFIED | `bench/canary/canary.go` exposes `InjectPrompt`/`IsContaminated`. Applied at SCORE TIME in aggregator (`aggregate.go:295 rowCanary`, `:316 reduceCanaryRate`), NOT in loader/fetch sources (grep of loaders/fetchers for canary = empty, per Plan 05 design). `report.go:70 CanaryPassRate ciValue` additive column; `aggregate.go:123` wires it after determinism-locked reductions. `aggregate_canary_test.go` proves 0.5 pass-rate + NULL em-dash on no-data. Goldens unchanged (`git status bench/aggregator` shows no golden/testdata diff); aggregator tests pass. |

**Score:** 4/4 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
| --- | --- | --- | --- |
| `bench/evaluators/exactmatch/exactmatch.go` | `func EM` | ✓ VERIFIED | `func EM(pred, gold string) bool` :21 |
| `bench/evaluators/editsim/editsim.go` | `func ES` | ✓ VERIFIED | `func ES(pred, gold string) float64` :33, stdlib-only (no suggest_lev import) |
| `bench/evaluators/identmatch/identmatch.go` | `func Match` | ✓ VERIFIED | `func Match(pred, gold string) (em bool, f1 float64)` :83 |
| `bench/evaluators/completion_gate/gate.go` | `func Grade` | ✓ VERIFIED | `func Grade` :103; imports the 3 scorers; ES default floored (:119) |
| `bench/evaluators/VERIFIED.md` | gate doc | ✓ VERIFIED | all-three-required / configurable threshold / abstain→false / tokenizer all documented |
| `bench/datasets/crosscodeeval/{fetch,loader,pin}.go` | Fetch/Load/PinnedRev | ✓ VERIFIED | net/http+arrow-go, SSRF-pinned, temp+rename atomic cache |
| `bench/datasets/repobench/{fetch,loader,score}.go` | Fetch/Load/AccAtK | ✓ VERIFIED | R/C/P modeled; AccAtK; reuses EM/ES |
| `bench/canary/canary.go` | probe | ✓ VERIFIED | InjectPrompt + IsContaminated |
| `bench/aggregator/{report,aggregate}.go` | CanaryPassRate | ✓ VERIFIED | additive column + reduceCanaryRate, wired |
| `cmd/helix-bench/main.go` | fetch-datasets | ✓ VERIFIED | `newFetchDatasetsCmd` registered (:99); RunE invokes both `Fetch` funcs (:382) |
| testdata/fixtures (CCE 4-lang, RepoBench py+java R/C) | committed | ✓ VERIFIED | all present on disk |

### Key Link Verification

| From | To | Via | Status |
| --- | --- | --- | --- |
| `completion_gate/gate.go` | `evaluators/{exactmatch,editsim,identmatch}` | import 3 scorers | ✓ WIRED |
| `gate.go` | `evaluators.Metrics.VerifiedCorrectness` | `ApplyToMetrics` *bool | ✓ WIRED |
| `repobench/score.go` | `evaluators/{exactmatch,editsim}` | -C/-P next_line scoring | ✓ WIRED |
| `aggregate.go` | `Leaderboard[*].CanaryPassRate` | additive reduce over canary key | ✓ WIRED |
| `cmd/helix-bench/main.go` | `crosscodeeval.Fetch + repobench.Fetch` | fetch-datasets RunE | ✓ WIRED |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| --- | --- | --- | --- |
| Build | `go build ./...` | exit 0 | ✓ PASS |
| Bench + CLI tests | `go test -count=1 ./bench/... ./cmd/helix-bench/...` | all packages ok | ✓ PASS |
| CR-01 zero-value gate | `go test -run ZeroValue ./bench/evaluators/completion_gate/` | PASS (ES oracle bites under GateConfig{}) | ✓ PASS |
| Abstain invariant | `go test -run Abstain ./bench/evaluators/completion_gate/` | PASS (explicit non-nil &false) | ✓ PASS |
| Full vet gate | `make vet` (6 custom vettools) | clean | ✓ PASS |
| VERIFIED.md gate | `make verify-verified-md` | all required sections present | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
| --- | --- | --- | --- | --- |
| ADAPTER-CCE-01 | 86-01, 86-03, 86-05 | CCE adapter, EM+ES+identifier, py/java/ts/c# | ✓ SATISFIED | SC#1 verified; scorers unit-tested (the SOLE authoritative proof per REQUIREMENTS acceptance); live smoke honestly network-gated |
| ADAPTER-REPO-01 | 86-04, 86-05 | RepoBench-R/-C/-P, py+java | ✓ SATISFIED | SC#2 verified; reference-match network-gated as REQUIREMENTS acceptance allows ("on a sampled subset") |
| VERIFIED-03 | 86-02, 86-05 | Multi-oracle gate, configurable threshold, abstain | ✓ SATISFIED | SC#3 verified; gate documented in VERIFIED.md; CR-01 fail-open hole closed |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
| --- | --- | --- | --- | --- |
| `cmd/helix-bench/main.go` | 88 | help-string says "fetch-datasets ... (not yet implemented)" | ℹ️ Info | Stale top-level Long help text; the command IS implemented and registered (`newFetchDatasetsCmd` :99, RunE invokes real Fetch :382). No `TBD`/`FIXME`/`XXX` markers anywhere in phase files; the `notYetImplemented` helper remains legitimately used only by `report` (:424, a genuinely deferred subcommand). Cosmetic doc drift, not a functional gap. |

No `TBD`/`FIXME`/`XXX` debt markers in any phase-modified file. No stub/hollow data paths: all scorers are total pure functions with hermetic proofs; canary reductions read real doc keys; CanaryPassRate emits NULL (em-dash) rather than a fabricated 0 when no data.

### Gaps Summary

None. All four ROADMAP success criteria are observably true in the live codebase. The one code-review BLOCKER (CR-01, gate fail-open on zero-value config) is fixed in source (`gate.go:119-121`) and proven by `TestGrade_ZeroValueConfig_ESOracleStillBites`; the four WARNINGs (WR-01 retrieval gold-index decode, WR-02 int64 truncation, WR-03 silent language-mismatch drop, WR-04 non-atomic cache write) are all fixed in source with their commits present in git history (6460be4e, 08a0f355, 73e8f324, 501c192e, 00d0f2ee). No regressions: full bench+CLI suite, `make vet`, and `make verify-verified-md` are clean; aggregator goldens are byte-identical (no testdata diff).

For SC#1/SC#2, `passed` is appropriate because the scorers, loaders, and gate are hermetically proven and the live reference-match is honestly recorded as `HELIX_BENCH_NETWORK`-gated (skips cleanly offline), exactly matching the REQUIREMENTS.md acceptance phrasing ("unit-tested against CCE paper examples" / "on a sampled subset").

---

_Verified: 2026-06-21T05:05:39Z_
_Verifier: Claude (gsd-verifier)_

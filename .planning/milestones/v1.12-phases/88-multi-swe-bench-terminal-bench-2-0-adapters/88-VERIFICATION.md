---
phase: 88-multi-swe-bench-terminal-bench-2-0-adapters
verified: 2026-06-21T00:00:00Z
status: passed
score: 4/4 must-haves verified
overrides_applied: 0
re_verification:
  previous_status: none
  previous_score: none
gaps: []
deferred: []
---

# Phase 88: Multi-SWE-bench + Terminal-Bench 2.0 Adapters Verification Report

**Phase Goal:** Two subprocess-shellout adapters — Multi-SWE-bench (Mini set; per-language slicing for Java/TS/JS/Go/Rust/C/C++) and Terminal-Bench 2.0 (≥5-task `tb run` smoke, container-isolation) — plus a long-wall scheduler with per-cell checkpointing so a >24h task resumes after harness restart.
**Verified:** 2026-06-21
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (ROADMAP Success Criteria)

| # | Truth (SC) | Status | Evidence |
| --- | --- | --- | --- |
| 1 | SC#1 (ADAPTER-MULTI-01): Multi-SWE-bench adapter via subprocess `python -m multi_swe_bench.harness.run_evaluation --config <config.json>`; path-validated atomic config.json producer; per-language slicing through the EXISTING aggregator (zero new aggregator code) | ✓ VERIFIED | `RunArgs` (harness.go:41-49) builds the fixed argv `["-m","multi_swe_bench.harness.run_evaluation","--config",cfgPath]`; `WriteConfig` (config.go) validates every path field with `isValidPredictionsPath` before marshal, atomic temp+rename; `Ingest` (ingest.go:36-56) gates `TaskSuccess` on the authoritative `resolved` bool via distinct `&taskSuccess`, stamps `Language` from the cell arg (Pitfall 1). `reduceLanguageRows`/`rowLanguage` are EXISTING (Phase 85 commit 9bdc12bb; aggregate.go last touched in Phase 87) — `TestAggregateByLanguageMultiSWE` proves 7 ByLanguage rows over {go,java,ts,js,rust,c,cpp} with zero new production code. `go test ./bench/evaluators/multiswebench/ ./bench/aggregator/` green. Live Mini-set run is Docker/python-gated (honest, ROADMAP-sanctioned). |
| 2 | SC#2 (ADAPTER-TERM-01): Terminal-Bench adapter via `tb run`/`harbor run` (runnerKind seam wired to resolved kind per WR-01); container-isolation (no bench/container import); `is_resolved` → result.v2 | ✓ VERIFIED | `Harness.kind` set by `Detect` (harness.go:214) and `RunArgs(h.kind, r)` uses the resolved kind (harness.go:229) — WR-01 fixed. `go test ./bench/evaluators/terminalbench/ -run Kind` passes (TestRunnerKindSeam, TestRunArgsHonorsKind, TestRunHonorsResolvedKind). `go list -deps ./bench/evaluators/terminalbench/ \| grep bench/container` → empty (SC#2 import-level gate). `Ingest` (ingest.go:33-52) gates `TaskSuccess` on `is_resolved` via distinct pointer, benchmarkName=="terminal-bench". `ParseTBResults`/`ParseTBTrial` exist and are size-capped. Live ≥5-task smoke is tb/Docker-gated (honest). |
| 3 | SC#3: long-wall scheduler + per-cell checkpointing; >24h task resumes after restart — proven HERMETICALLY via injected clock (NO real 24h run) | ✓ VERIFIED | `bench/longwall/` is a pure leaf (`go list -deps` shows no helix/bench imports). `Store` carries injected `now func() time.Time` (checkpoint.go:41-52); `writeCheckpoint` is atomic temp+rename. `go test ./bench/longwall/` green incl. TestCheckpointRoundTrip, TestResumeSkipsDone (runner invoked 0× on seeded done), TestIdempotentReentry, TestFullRestart (+48h clock advance, 0 cells re-run), TestFailedCellReruns, and the WR-03 fix tests TestCellKeyRejectsMalformed + TestMalformedCellDegradesNotAborts. No real 24h dependence — UpdatedAt sourced solely from the injected clock. |
| 4 | SC#4: per-language ablation slicing in the reporter (not upstream harness); run-all-tests carry-forward | ✓ VERIFIED | Per-language slicing lives in the EXISTING reporter `reduceLanguageRows` (aggregate.go:255) keyed on the `language` open key (`rowLanguage`, aggregate.go:517) — NOT the upstream harness; proven by `TestAggregateByLanguageMultiSWE` (7 rows). Run-all-tests carry-forward "where applicable": Multi-SWE/TB's native harness runs the full suite and emits the authoritative `resolved`/`is_resolved` verdict, which both adapters consume as the sole `TaskSuccess` gate (never a row count) — the SWE-bench-specific UTBoost differential overlay correctly does not apply (RESEARCH Pitfall 5). |

**Score:** 4/4 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
| --- | --- | --- | --- |
| `bench/evaluators/multiswebench/config.go` | WriteConfig path-validating atomic producer | ✓ VERIFIED | `WriteConfig` validates every path field before marshal; atomic temp+rename; leaf (no bench/runtime import) |
| `bench/evaluators/multiswebench/harness.go` | fixed `--config` argv + Detect + setGroupKillCancel | ✓ VERIFIED | RunArgs fixed argv; Detect PATH-probes python; WR-02 `setGroupKillCancel(cmd)` wired (line 110) |
| `bench/evaluators/multiswebench/ingest.go` | resolved-gate → ResultInput, Language stamp | ✓ VERIFIED | distinct `&taskSuccess`; Language from cell arg; missing instance → error |
| `bench/evaluators/multiswebench/report.go` | size-capped strict parsers | ✓ VERIFIED | ParseFinalReport present; maxReportBytes cap |
| `bench/evaluators/multiswebench/testdata/config.golden.json` | committed golden contract | ✓ VERIFIED | present (805 bytes); TestWriteConfigGoldenStable byte-stable |
| `bench/evaluators/terminalbench/harness.go` | tb run argv + runnerKind seam + WR-01/WR-02 | ✓ VERIFIED | kind on Harness, RunArgs(h.kind), setGroupKillCancel wired |
| `bench/evaluators/terminalbench/ingest.go` | is_resolved gate | ✓ VERIFIED | distinct pointer; no bench/container import |
| `bench/evaluators/terminalbench/report.go` | size-capped parsers | ✓ VERIFIED | ParseTBResults + ParseTBTrial; null-body rejection |
| `bench/longwall/checkpoint.go` | atomic checkpoint + injected clock + fallible cellKey | ✓ VERIFIED | WR-03: cellKey returns (string,error), no panic |
| `bench/longwall/scheduler.go` | resume-aware Scheduler.Run | ✓ VERIFIED | WR-03: c.Key() error → Failed+continue, never aborts the pass |
| `bench/datasets/multi-swe-bench-mini/{pin,fetch}.go` | SSRF-safe network-gated fetcher | ✓ VERIFIED | leaf; isHexSHA1 mutable-ref refusal; LimitReader cap; atomic cache write |
| `bench/LICENSES.md` | Multi-SWE (CC0) + Terminal-Bench (Apache-2.0) rows | ✓ VERIFIED | both rows + full-set INFRA-02 deferral note present |

### Key Link Verification

| From | To | Via | Status |
| --- | --- | --- | --- |
| multiswebench WriteConfig | isValidPredictionsPath | path-field validation before marshal | ✓ WIRED |
| multiswebench Ingest | ResultInput{Language,TaskSuccess} | resolved → &taskSuccess; Language from cell arg | ✓ WIRED |
| aggregator reduceLanguageRows | result.v2 `language` open key | Cell.Language → per-language ByLanguage slice (EXISTING) | ✓ WIRED |
| terminalbench RunArgs | resolved runnerKind (h.kind) | Detect records kind; Run/RunArgs use it (WR-01) | ✓ WIRED |
| terminalbench Ingest | ResultInput{ContainerID,TaskSuccess} | IsResolved → &taskSuccess | ✓ WIRED |
| longwall writeCheckpoint | atomic temp+rename | os.Rename | ✓ WIRED |
| longwall Scheduler.Run | readCheckpoint Status==done → skip | per-cell resume gate; bad key → Failed (WR-03) | ✓ WIRED |
| longwall CellState.UpdatedAt | injected now func() time.Time | golden-stable timestamps | ✓ WIRED |

### Review Warning Fixes (no SC regression)

| Warning | Fix verified in live code | Regression test |
| --- | --- | --- |
| WR-01: runnerKind seam mis-wired | `Harness.kind` set by Detect; `RunArgs(h.kind,r)` uses resolved kind (harness.go:214,229) | TestRunHonorsResolvedKind exercises tb vs harbor argv via runShim |
| WR-02: Setpgid without cmd.Cancel orphans Docker | `setGroupKillCancel(cmd)` (negative-pid SIGKILL + WaitDelay) wired in both adapters' Run; build-tag-split unix/windows; swebench parity commit clean | unix procgroup helper; full ./bench/... green incl. swebench |
| WR-03: cellKey panic aborts whole run | `cellKey` returns (string,error); Scheduler.Run counts bad cell Failed + continue (scheduler.go:81-89) | TestCellKeyRejectsMalformed + TestMalformedCellDegradesNotAborts |

### Requirements Coverage

| Requirement | Source Plan | Status | Evidence |
| --- | --- | --- | --- |
| ADAPTER-MULTI-01 | 88-01, 88-04 | ✓ SATISFIED | multiswebench adapter + Mini-set fetcher + per-language slicing; SC#1/SC#4 verified; live run Docker-gated (honest) |
| ADAPTER-TERM-01 | 88-02, 88-03, 88-04 | ✓ SATISFIED | terminalbench adapter (SC#2) + longwall resume (SC#3) + license clearance; live smoke tb-gated (honest) |

Plan frontmatter requirement IDs cross-reference cleanly against REQUIREMENTS.md Phase 88 (ADAPTER-MULTI-01, ADAPTER-TERM-01). Plan 03 (longwall) declaring ADAPTER-TERM-01 is correct — SC#3 long-horizon resume is the Terminal-Bench requirement.

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| --- | --- | --- | --- |
| All packages build | `go build ./...` | exit 0 | ✓ PASS |
| All bench tests pass | `go test -count=1 ./bench/...` | all `ok` | ✓ PASS |
| terminalbench Kind seam (WR-01) | `go test ./bench/evaluators/terminalbench/ -run Kind` | 3/3 PASS | ✓ PASS |
| longwall hermetic invariants (SC#3 + WR-03) | `go test ./bench/longwall/ -v` | 10/10 PASS incl. malformed-degrades | ✓ PASS |
| 7-language aggregator slicing (SC#1) | `go test ./bench/aggregator/ -run TestAggregateByLanguage` | 3/3 PASS | ✓ PASS |
| Full custom-vettool gate | `make vet` | exit 0 (6 vettools clean) | ✓ PASS |
| terminalbench has no bench/container | `go list -deps ... \| grep bench/container` | empty (SC#2) | ✓ PASS |
| longwall is a pure leaf | `go list -deps ./bench/longwall/` | no helix/bench imports | ✓ PASS |

### Probe Execution

No `scripts/*/tests/probe-*.sh` declared or implied for this phase; the phase's verification contract is the hermetic Go test suite (run above). Not applicable.

### Anti-Patterns Found

| File | Pattern | Severity | Impact |
| --- | --- | --- | --- |
| (none) | TBD/FIXME/XXX in phase-88 files | — | grep over multiswebench/terminalbench/longwall/multi-swe-bench-mini found NONE in production or test files |

Note (Info, not a gap): IN-01 (`_ = s.store.writeCheckpoint(...)` swallows the write error so a `Ran` count can imply durability that did not persist) was explicitly out of scope in 88-REVIEW-FIX. It is conservative-safe (a lost `done` write simply re-runs the cell next pass — never a false skip) and does not affect SC#3 correctness. IN-02/03/04 are likewise documented Info-level nits, not blockers.

### Honest Live-Run Boundaries (ROADMAP-sanctioned, not gaps)

- SC#1 live Multi-SWE Mini-set run is Docker + `multi_swe_bench` gated; `Detect()` resolves python but the live test `t.Skip`s cleanly — the hermetic fixtures are the sole proof.
- SC#2 live ≥5-task `tb run` smoke is tb/harbor + Docker gated; `Detect()` returns the unavailable sentinel and the smoke test `t.Skip`s cleanly.
- A1-A7 [ASSUMED] upstream contracts (config field spellings, final_report.json keys, tb results.json keys, Mini-set HF repo id, tb-vs-harbor binary, PinnedSHA/digests) are pinned as hermetic golden contracts and the live confirmation was resolved APPROVED-WITH-DEFERRAL by the orchestrator (Phase 87 precedent), recorded honestly in pin.go doc-comments and bench/LICENSES.md (40-zero placeholder PinnedSHA, never falsely claimed live-confirmed).

These boundaries are the same deferred-to-live posture sanctioned for Phase 87 and are not counted as gaps.

### Gaps Summary

None. All four ROADMAP success criteria are observably true in the live codebase: the two subprocess adapters exist with hermetically-proven decision logic, per-language slicing flows through the existing aggregator with zero new production code, the long-wall resume guarantee is proven via an injected clock with no real 24h run, and all three code-review warnings (WR-01/02/03) are fixed in live code with dedicated regression tests and no SC regression. `go build ./...`, `go test -count=1 ./bench/...`, and `make vet` all pass.

---

_Verified: 2026-06-21_
_Verifier: Claude (gsd-verifier)_

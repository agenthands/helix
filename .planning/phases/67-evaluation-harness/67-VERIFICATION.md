---
phase: 67-evaluation-harness
verified: 2026-05-10T13:05:00Z
status: passed
score: 10/10 must-haves verified
overrides_applied: 0
---

# Phase 67: Evaluation Harness — Verification Report

**Phase Goal:** Ship an out-of-process evaluation harness that proves Helix improves agent success / cost / safety by running the same task corpus under four modes and emitting per-task evidence.

**Verified:** 2026-05-10T13:05:00Z
**Status:** PASS
**Re-verification:** No — initial verification

---

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Baseline profile exists and exposes zero Helix tools (A1) | VERIFIED | `internal/profile/profiles/baseline.yaml` skills:[] tools:[]; `TestBaselineExposesZeroHelixTools` PASSES |
| 2 | Per-(task,mode) sandbox isolation (HOME/socket/config) | VERIFIED | `sandbox.go:86,96,206-227` HomeFor/SocketFor/HOME env var per mode; tests pass |
| 3 | Agent runner shells out to `claude --bare --strict-mcp-config` | VERIFIED | `internal/eval/agent/claude.go:79-81` |
| 4 | Budget watchdog enforces four axes (input/output tokens, seconds, tool_calls) | VERIFIED | `internal/eval/budget/budget.go:22-25`; tests pass |
| 5 | Trace tap + merger produces trace_daemon.jsonl + trace_cc.json + merged trace.json | VERIFIED | `internal/eval/trace/{tap,merge,schema}.go`; tests pass |
| 6 | Heuristic scorer + DSL + 10-task seed corpus (Go/TS/Python) | VERIFIED | `internal/eval/score/`, `eval/corpus/` (10 dirs); tests pass |
| 7 | Runner + 5 reporters (eval_report.json/md, cost_summary, safety_compliance, run_metadata) + `helix-eval run` | VERIFIED | `internal/eval/report/` (eval_report, cost_summary, safety_compliance, run_metadata); `cmd/helix-eval/main.go`; tests pass |
| 8 | eval-quick in-process variant: scripted-agent, 9 fixtures, <30s wall-time | VERIFIED | `internal/eval/runner/inprocess.go`; `make eval-quick` = 3.4s; 9 fixtures; test asserts >=9 at `inprocess_fixtures_test.go:40` |
| 9 | LLM judge informational only; never gates merges | VERIFIED | `internal/eval/judge/`; go-test.yml `forbid judge in CI` step grep-gates `tool_behavior_judge` from all workflows |
| 10 | CI runs `make eval-quick` only; `make eval` excluded | VERIFIED | go-test.yml: `eval-quick` step present; comment explicitly excludes `make eval` from CI |

**Score:** 10/10 truths verified

---

## Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/profile/profiles/baseline.yaml` | Profile #6, strips all tools | VERIFIED | 55 lines, skills:[], tools:[], A1 comment |
| `internal/eval/sandbox/` | Per-mode FS/socket isolation | VERIFIED | sandbox.go, sandbox_test.go |
| `internal/eval/agent/claude.go` | claude CLI subprocess wrapper | VERIFIED | --bare --strict-mcp-config at lines 79-81 |
| `internal/eval/budget/budget.go` | Four-axis budget enforcement | VERIFIED | MaxInputTokens, MaxOutputTokens, MaxSeconds, MaxToolCalls |
| `internal/eval/trace/` | tap.go + merge.go + schema.go | VERIFIED | All 3 files present; tests pass |
| `internal/eval/score/` | rules.go, score.go | VERIFIED | DSL parser + scorer + starter rules |
| `eval/corpus/` | 10 seed tasks (Go/TS/Python) | VERIFIED | 10 task directories |
| `eval/fixtures/` | ~10 quick fixtures | VERIFIED | 9 directories (within "~10" tolerance per D-05) |
| `internal/eval/runner/` | runner.go, inprocess.go, scripted_agent.go | VERIFIED | All present; tests pass |
| `internal/eval/report/` | 5 reporters | VERIFIED | eval_report.go, cost_summary.go, safety_compliance.go, run_metadata.go |
| `internal/eval/judge/` | client.go, judge.go, prompts/ | VERIFIED | All present; tests pass |
| `cmd/helix-eval/main.go` | helix-eval binary with `run` subcommand | VERIFIED | Builds; `run` + `validate-rules` subcommands present |
| `eval/EVAL.md` | TOS attestation, retention policy | VERIFIED | Present at eval/EVAL.md |
| `.github/workflows/go-test.yml` | eval-quick step + judge grep gate | VERIFIED | Both steps present |

---

## Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| All eval package tests pass | `go test ./internal/eval/... ./cmd/helix-eval/...` | 9 packages pass | PASS |
| Baseline A1 test passes | `go test ./internal/profile/ -run TestBaselineExposesZeroHelixTools` | PASS | PASS |
| eval-quick completes <30s | `make eval-quick` | 3.4s wall-time, 36/36 tasks | PASS |
| helix-eval binary invocable | `go run ./cmd/helix-eval --help` | shows `run` subcommand | PASS |
| `make eval` not in CI | `grep -rE 'make eval([^-])' .github/workflows/` | only comment line found | PASS |
| `tool_behavior_judge` grep gate present | workflow step `forbid judge in CI` | grep gate wired | PASS |

---

## Requirements Coverage

| Requirement | Status | Evidence |
|-------------|--------|----------|
| EVAL-01: EvalResult captures success, tokens, guardrail compliance, etc. | SATISFIED | `internal/eval/report/eval_result.go`; tests pass |
| EVAL-02: Each mode in isolated subprocess with baseline using --profile=baseline | SATISFIED | `sandbox.go` isolation + `agent/claude.go` --bare; baseline.yaml profile |
| EVAL-03: `make eval-quick` in-process <30s | SATISFIED | 3.4s wall-time in real run |
| EVAL-04: 5 report files emitted | SATISFIED | eval_report.json/md, cost_summary, tool_behavior, safety_compliance in `internal/eval/report/` |
| EVAL-05: Tool-behavior scoring +1/-1 for rename/delete/public-API | SATISFIED | `internal/eval/score/rules.go` DSL + scorer; corpus covers all 5 families |
| EVAL-06: Synthetic-only corpus; TOS attestation in EVAL.md | SATISFIED | eval/EVAL.md present; ZDR gate warns on OSS secondary corpus |
| EVAL-07: LLM judge informational, never gates CI | SATISFIED | judge grep gate in go-test.yml; judge package returns informational-only results |

---

## Anti-Patterns Found

None blocking. One noted:

| File | Pattern | Severity | Impact |
|------|---------|----------|--------|
| `eval/fixtures/` (count=9) | D-05 targets "~10" quick fixtures; 9 delivered | FLAG | Test asserts `>=9`; within stated tolerance. No behavioral gap. |

---

## Gaps Summary

No blocking gaps. One acceptable deviation:

- **Fixture count:** `eval/fixtures/` contains 9 quick fixtures vs. the plan's "~10" target (D-05 phrasing is "~10"). The test at `internal/eval/runner/inprocess_fixtures_test.go:40` accepts this explicitly with `assert >= 9`. The `make eval-quick` run produces 36/36 successes (9 fixtures x 4 modes). This is within plan tolerance.

---

## Recommendation

**PASS**

All 10 must-haves are verified with codebase evidence. All 7 EVAL-0x requirements are satisfied. All tests pass. `make eval-quick` runs in 3.4s (well under the 30s budget). The 9-vs-10 fixture deviation is within the "~10" tolerance documented in D-05 and acknowledged by the test suite itself. The phase goal — a working evaluation harness with in-process fast validation, full reporting pipeline, informational LLM judge, and CI integration — is achieved.

---

_Verified: 2026-05-10T13:05:00Z_
_Verifier: Claude (gsd-verifier)_

---
phase: 67
plan: "01"
subsystem: eval
tags: [eval, scaffolding, profile, baseline, tdd]
dependency_graph:
  requires: []
  provides:
    - internal/profile/profiles/baseline.yaml (baseline profile, eval control case)
    - internal/eval/* (package skeletons, import boundaries locked)
    - internal/eval/budget/types.go (BreachReason cross-plan type contract)
    - eval/ (top-level tree, TOS attestation, .gitignore)
    - cmd/helix-eval (cobra entrypoint binary)
    - Makefile eval + eval-quick targets
  affects:
    - internal/profile/loader_test.go (profile count 5→6)
    - Makefile (.PHONY extended, two new targets)
tech_stack:
  added:
    - cmd/helix-eval (new binary, cobra-based, no new deps)
  patterns:
    - TDD RED→GREEN for baseline profile A1 assumption verification
    - doc.go-only package skeletons to lock import boundaries
    - Cross-plan type contract (budget.BreachReason) in Wave-0 package
key_files:
  created:
    - internal/profile/profiles/baseline.yaml
    - internal/profile/baseline_test.go
    - eval/EVAL.md
    - eval/corpus/.gitkeep
    - eval/gen/.gitkeep
    - eval/fixtures/.gitkeep
    - eval/reports/.gitkeep
    - eval/.gitignore
    - internal/eval/runner/doc.go
    - internal/eval/sandbox/doc.go
    - internal/eval/agent/doc.go
    - internal/eval/trace/doc.go
    - internal/eval/score/doc.go
    - internal/eval/judge/doc.go
    - internal/eval/report/doc.go
    - internal/eval/budget/types.go
    - internal/eval/budget/types_test.go
    - internal/eval/pipeline.go
    - cmd/helix-eval/main.go
    - cmd/helix-eval/main_test.go
  modified:
    - internal/profile/loader_test.go (profile count assertion 5→6, sorted names list)
    - Makefile (.PHONY + eval/eval-quick targets appended)
decisions:
  - "A1 MECHANISM: empty-lists (not disable_all_tools flag). ResolveTools([],[],[]) returns empty non-nil slice; ProfileFilterMiddleware sees AllowedTools!=nil and filters to zero tools."
  - "baseline.yaml uses skills:[] tools:[] guardrails.enforcement:off default_mode:edit"
  - "cmd/helix-eval is a standalone binary (not a helix subcommand) to isolate eval deps from daemon binary"
  - "BreachReason type landed in Wave-0 budget package so Plan-02 watchdog and Plan-03 trace merger can run in parallel Wave-1"
metrics:
  duration: "~7 minutes"
  completed: "2026-05-10"
  tasks: 3
  files_created: 20
  files_modified: 2
---

# Phase 67 Plan 01: Baseline Profile and Skeleton Summary

Baseline eval profile A1 verified + internal/eval/* scaffolding + cmd/helix-eval cobra binary + Makefile targets + EVAL.md TOS attestation

## Assumption A1 Outcome

**A1 MECHANISM USED: empty-lists (no `disable_all_tools` flag was needed)**

The research hypothesis held. `skill.ResolveTools([], [], [])` returns an empty but non-nil `[]*mcp.ToolDef` slice. `session.SetAllowedTools([]string{})` stores an empty non-nil slice as `AllowedTools`. `ProfileFilterMiddleware` checks `snap.AllowedTools != nil` — true for empty slice — enters filtering, and allows zero tools through. The `disable_all_tools: true` fallback in `profile.go` / `middleware.go` was NOT needed.

This is documented:
- In `baseline.yaml` header comments (for future phases)
- In `TestBaselineExposesZeroHelixTools` assertion message
- In the commit message for the GREEN commit (d3c0737e)

## Tasks Executed

### Task 1: baseline.yaml + A1 test (TDD RED→GREEN)

**RED commit:** f3702dfd — four failing tests before baseline.yaml existed.

**GREEN commit:** d3c0737e — baseline.yaml created; four tests pass; loader_test.go updated (Rule 1 auto-fix, see Deviations).

Tests:
- `TestBaselineProfileLoads` — "baseline" present in registry
- `TestBaselineExposesZeroHelixTools` — A1 verified, zero tools exposed
- `TestBaselineGuardrailsOff` — `guardrails.enforcement: off`
- `TestBaselineDescriptionMentionsEval` — description contains "eval"

### Task 2: eval/ tree + EVAL.md

**Commit:** 359e97ba

Files created: `eval/EVAL.md`, `eval/corpus/.gitkeep`, `eval/gen/.gitkeep`, `eval/fixtures/.gitkeep`, `eval/reports/.gitkeep`, `eval/.gitignore`

EVAL.md sections:
1. Phase-67 charter paragraph
2. Provider Retention Attestation (all three Anthropic citation links, retrieval date 2026-05-10)
3. Eval-run policy (synthetic default / ZDR gate / refusal rule)
4. ZDR M5 caveat (account-level, no per-request header)
5. eval-quick vs eval distinction (Pitfall 6 mitigation)
6. Operator ZDR checklist (Pitfall 7 mitigation)

### Task 3: internal/eval/* skeletons + cmd/helix-eval + Makefile

**Commit:** 458e79fc

doc.go skeletons: runner, sandbox, agent, trace, score, judge, report (import boundaries locked)

Budget cross-plan contract: `internal/eval/budget/types.go` — `BreachReason` struct + `String()`. Tests: `TestBreachReasonShape`, `TestBreachReasonString` — GREEN.

Pipeline re-export: `internal/eval/pipeline.go` — `var Phases []phasegraph.PhaseSpec = pipelines.EvalPhases`

cobra binary: `cmd/helix-eval/main.go` — `helix-eval run` (flags: --corpus, --mode, --run-id, --quick, --judge-model, --no-judge, --out) and `helix-eval validate-rules`. Both return "not yet implemented" sentinels.

Tests: `TestHelixEvalCommandHelp`, `TestHelixEvalRunNotImplemented`, `TestPipelineExportsEvalPhases` — all GREEN.

Makefile: `eval-quick` and `eval` targets appended; `.PHONY` extended.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Updated loader_test.go profile count assertions**

- **Found during:** Task 1 (GREEN phase)
- **Issue:** `TestLoadEmbedded_ReturnsAllProfilesAndModes` hardcoded `"expected 5 profiles"` and `TestProfileStore_ProfileNames_Sorted` had the sorted names list without "baseline". After adding baseline.yaml, both tests failed.
- **Fix:** Updated count assertion 5→6; added "baseline" to the sorted names slice.
- **Files modified:** `internal/profile/loader_test.go`
- **Commit:** d3c0737e

None other — plan executed as designed.

## Verification Results

```
go vet ./... → exit 0 (pre-existing C warning in treesitter/bindings/swift, not a Go vet error)
go test ./internal/profile/... → PASS
go test ./internal/eval/budget/... → PASS (race-clean)
go test ./cmd/helix-eval/... → PASS
go build ./cmd/helix-eval ./internal/eval/... → OK
```

## Known Stubs

The following are intentional Wave-0 stubs, documented in each file:

| File | Stub | Reason |
|------|------|--------|
| `cmd/helix-eval/main.go` RunE bodies | `return fmt.Errorf("not yet implemented...")` | Wave 1+ fills real bodies |
| `internal/eval/runner/doc.go` | doc.go only | Wave 1+ adds runner.go |
| `internal/eval/sandbox/doc.go` | doc.go only | Wave 1+ adds sandbox.go |
| `internal/eval/agent/doc.go` | doc.go only | Wave 1+ adds claude.go |
| `internal/eval/trace/doc.go` | doc.go only | Wave 1+ adds tap.go/merge.go |
| `internal/eval/score/doc.go` | doc.go only | Wave 2+ adds rules.go |
| `internal/eval/judge/doc.go` | doc.go only | Wave 3+ adds judge.go |
| `internal/eval/report/doc.go` | doc.go only | Wave 3+ adds report writers |

These stubs are intentional and do not prevent the plan's goal. Later waves fill them per the DAG-02 contract.

## Threat Surface Scan

No new network endpoints, auth paths, file access patterns, or schema changes at trust boundaries beyond what the plan's threat model covers. T-67-01 (baseline.yaml tampering) is mitigated by the RED→GREEN assertion test. T-67-06 (ZDR attestation) is documented in EVAL.md operator checklist.

## Self-Check: PASSED

All key files verified present. All 4 commits verified in git log.

| Check | Result |
|-------|--------|
| `internal/profile/profiles/baseline.yaml` | FOUND |
| `internal/profile/baseline_test.go` | FOUND |
| `eval/EVAL.md` | FOUND |
| `internal/eval/budget/types.go` | FOUND |
| `cmd/helix-eval/main.go` | FOUND |
| `internal/eval/pipeline.go` | FOUND |
| RED commit f3702dfd | FOUND |
| GREEN commit d3c0737e | FOUND |
| eval/ commit 359e97ba | FOUND |
| skeletons commit 458e79fc | FOUND |

---
phase: 98-multi-agent-coverage-stronger-steering
plan: 01
subsystem: agent-adoption / CLI steering
status: complete
tags: [steering, nudge, prehook, session-priming, tdd, defer-97-01]
requires:
  - "internal/cli/nudge.go (isGrepReadTool / classifyBashTarget / bashSteerMessage steering engine)"
  - "internal/cli/activate.go (SessionStart hook stdout)"
  - "internal/cli/verb.go VerbToolNames() (frozen-verb authority)"
  - "internal/cli/skills/helix/SKILL.md decision matrix (priming source-of-truth)"
provides:
  - "Bash sed/cat over a code file steers to the specific helix verb (STEER-01)"
  - "SessionStart priming matrix emitted on `helix activate` stdout (STEER-02)"
  - "STEER-03 negative-control golden rows (prose/log/config stay silent)"
  - "DEFER-97-01 closed non-vacuously (silent loop removed; firing rows on specific verb)"
affects:
  - "internal/cli/nudge.go"
  - "internal/cli/nudge_test.go"
  - "internal/cli/prime.go (new)"
  - "internal/cli/prime_test.go (new)"
  - "internal/cli/activate.go"
tech-stack:
  added: []
  patterns:
    - "Token-anchored Bash recognition on strings.Fields(cmd)[0] (parity with classifyBashTarget)"
    - "Single-constant size-capped priming surface (SKILL-04-style idle-cost bound)"
    - "Pure/fail-open SessionStart stdout emission (no daemon dial, best-effort write)"
    - "Anti-vacuity revert-and-fail proof keyed on the specific verb, not a `helix` substring"
key-files:
  created:
    - "internal/cli/prime.go"
    - "internal/cli/prime_test.go"
  modified:
    - "internal/cli/nudge.go"
    - "internal/cli/nudge_test.go"
    - "internal/cli/activate.go"
decisions:
  - "STEER-01 edits the GATE (isGrepReadTool), token-anchored on fields[0] ∈ {grep,rg,ag,egrep,fgrep,find,sed,cat} — NOT classifyBashTarget/bashSteerMessage (those already handle sed/cat operands+verbs and stay untouched)."
  - "Token-anchor (fields[0]) over strings.Contains to avoid a spurious match on a path containing the substring cat/sed (e.g. concatenate.go) — 98-RESEARCH Open Question 1."
  - "STEER-02 extends activate stdout (no new hook entry / setup change) — 98-RESEARCH Open Question 2."
  - "primingMatrix is a compiled package constant (single source of truth, cannot drift at runtime); sessionPrimingText() is pure so it cannot fail the session (fail-open)."
  - "primingMatrixMaxBytes = 2048 (SKILL-04-comparable bound); the matrix is well under it."
metrics:
  tasks: 2
  files-created: 2
  files-modified: 3
  duration: ~14m
  completed: 2026-06-22
---

# Phase 98 Plan 01: STEER thrust (broaden steering + SessionStart priming) Summary

Broadened the PreToolUse nudge so Bash `sed`/`cat` over a CODE file steers to the
specific `helix` verb (`replace-in-file`/`read-file`), flipped the Phase 97
DEFER-97-01 golden from "asserted silent" to "asserted firing on the specific verb"
non-vacuously, added STEER-03 negative-control rows proving prose/log/config targets
stay silent (exit-0, no deny path), and added a size-capped, fail-open SessionStart
priming surface (`prime.go`) that emits the terse "use X not Y" decision matrix once
per session from a single compiled constant.

## What was built

### Task 1 — STEER-01 + DEFER-97-01 flip + STEER-03 (commit 2f90cfb6)
- **`isGrepReadTool` broadened** (the ONLY production edit): its Bash arm now
  tokenizes the command with `strings.Fields` and matches `fields[0] ∈
  {grep, rg, ag, egrep, fgrep, find, sed, cat}` (was a `strings.Contains` on
  grep/find/rg/ag). This makes Bash `sed -i`/`cat` reach `steerMessage` →
  `classifyBashTarget` → `bashSteerMessage`'s already-correct `sed→replace-in-file` /
  `cat→read-file` branches, which were dead for Bash callers until now. An
  empty/zero-field command stays false (fail-open).
- `classifyBashTarget` and `bashSteerMessage` were **NOT touched** — the prose/log/
  config SILENCE is already enforced by `classifyBashTarget` returning non-code and
  `steerMessage` staying silent when `!isCode`, so STEER-03 passes without classifier
  changes.
- **DEFER-97-01 flip (non-vacuous):** added `bash-sed-i`→`helix replace-in-file` and
  `bash-cat`→`helix read-file` rows to `nudgeShapeGoldenCases()`, bumped the
  empty-bucket floor `5→7`, and DELETED the obsolete `bash-sed-i-silent`/
  `bash-cat-silent` loop (leaving it asserting silence would contradict the firing
  rows and fail).
- **STEER-03 negative controls:** `grep TODO README.md`, `sed -i … README.md`,
  `cat app.log`, `cat config.yaml` each assert NO advisory + exit-0 (one `t.Run` each).
- **Anti-vacuity:** new `TestNudgeGoldenRevertFails_BashSedCat` proves the cat shape
  steers to `helix read-file` (NOT `rename-symbol`) and the sed shape steers to
  `helix replace-in-file` (NOT `read-file`) — the two broadened shapes key on their
  SPECIFIC verb, not a bare `helix` substring, and are not collapsed to one verb.
- sed/cat true-cases (and token-anchor precision negatives like `go build … concatenate.go`)
  added to `TestIsGrepReadTool`; the new sed/cat shapes added to
  `TestNudgeAdvisory_AlwaysExitZero`.

### Task 2 — STEER-02 SessionStart priming (commit cf5bb109)
- **`internal/cli/prime.go` (new):** `primingMatrixMaxBytes` (2048), `primingMatrix`
  (a curated use-X-not-Y subset of SKILL.md's decision matrix, a compiled constant),
  and `sessionPrimingText() string` — a PURE function returning the constant (no
  daemon dial, no file read), so it cannot fail the session.
- **`activate.go` wired:** `runActivate` emits `sessionPrimingText()` to stdout after
  the activation status line, best-effort (a write error is ignored on purpose;
  `runActivate`'s return value is unchanged — fail-open).
- **`prime_test.go` (new) `TestSessionStartPriming`:** non-empty + sourced verbatim
  from `primingMatrix` (single source of truth) + `len ≤ primingMatrixMaxBytes` +
  does NOT inline the `Code generated by helix-refgen` banner + terser than the full
  SKILL body + deterministic on repeated calls + a drift guard mapping every cited
  `helix <verb>` (kebab) to its `VerbToolNames()` snake form (so a verb rename
  surfaces here, not silently).

## Deviations from Plan

None — plan executed exactly as written. Both tasks followed the TDD RED→GREEN cycle:
RED assertions were written first and confirmed failing against the unmodified source
(Task 1: `bash-sed-i`/`bash-cat` emitted nothing; Task 2: `prime.go` symbols
undefined → build failure), then the minimal production edit made them GREEN.

One in-task adjustment worth noting (not a plan deviation): the priming-matrix header
was reworded from "Prefer these helix verbs …" to "Prefer the verbs below …" because
the drift-guard regex (`helix ([a-z-]+)`) correctly flagged the prose phrase
"helix verbs" as a non-existent verb. Rewording keeps every `helix <token>` in the
matrix a real frozen verb — the drift guard working as intended.

## Verification

- `go test ./internal/cli/ -run 'TestIsGrepReadTool|TestNudgeShapeGolden|TestNudgeAdvisory_AlwaysExitZero|TestNudgeGoldenRevertFails|TestClassifyBashTarget_NonCodeTargets'` — green.
- `go test ./internal/cli/ -run 'TestSessionStartPriming|TestNewActivateCommand'` — green.
- `go test ./internal/cli/...` — green (3.4s).
- Per-wave merge gate: `go vet ./...` clean + `go test ./...` green (exit 0, no failures).
- `go run ./cmd/helix-refgen --check` — "reference.md is up to date" (untouched by this plan).
- `git diff go.mod` — empty (zero-dep invariant held).

## Success criteria (all met)

- STEER-01: Bash `sed -i …code.go`→`helix replace-in-file`; Bash `cat …code.go`→`helix read-file`; gate is `isGrepReadTool` token-anchored on `fields[0]`; classifier/message-builder untouched. ✓
- STEER-03: `sed -i … README.md`, `cat app.log`, `cat config.yaml` produce NO advisory; every broadened shape returns exit-0; NO exit-2/deny path added. ✓
- STEER-02: SessionStart priming emits the terse matrix, byte-capped, single-constant-sourced, fail-open; every matrix verb is a real frozen verb. ✓
- DEFER-97-01 closed non-vacuously: golden flipped to firing on the specific verb, silent loop removed, revert-and-fail proven for sed/cat shapes, floor bumped 5→7. ✓

## TDD Gate Compliance

This plan is `type: tdd` with both tasks `tdd="true"`. RED was established before each
production edit (Task 1 RED: `bash-sed-i`/`bash-cat` shapes emitted "" against the
unmodified gate; Task 2 RED: `prime.go` symbols undefined → compile failure), then
GREEN with the minimal production change. Per the plan's per-task atomic-commit
contract, RED and GREEN are folded into one `feat(...)` commit per task rather than
separate `test(...)`/`feat(...)` commits — the RED state was observed and recorded in
this summary at each step.

## Self-Check: PASSED

- internal/cli/prime.go — FOUND
- internal/cli/prime_test.go — FOUND
- internal/cli/nudge.go (modified) — FOUND
- internal/cli/nudge_test.go (modified) — FOUND
- internal/cli/activate.go (modified) — FOUND
- commit 2f90cfb6 — FOUND
- commit cf5bb109 — FOUND

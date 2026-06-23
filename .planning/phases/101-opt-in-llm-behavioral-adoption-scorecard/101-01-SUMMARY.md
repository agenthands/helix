---
phase: 101-opt-in-llm-behavioral-adoption-scorecard
plan: 01
subsystem: test/oracle (LLM-behavioral adoption scorecard, hermetic leg)
tags: [adopt-02, anti-vacuity, hermetic, tdd, scorecard]
requires:
  - internal/cli.EmbeddedSkillBody (Phase 97)
  - internal/cli/skills/helix/SKILL.md "## Decision matrix" heading (Phase 97/98)
provides:
  - adopt.FirstCommand
  - adopt.ClassifyChoice
  - adopt.StripDecisionMatrix
  - adopt.Scorecard
  - adopt.Bucket
  - adopt.ScorecardResult
  - adopt.MinTasks
  - adopt.MaterialDrop
  - test/oracle/adopt/testdata/transcripts (12 committed fixtures)
affects:
  - test/oracle/llm (Plan 02 tagged live leg will import adopt.*)
  - test/oracle/judge (Plan 02 negative exemplar)
tech-stack:
  added: []
  patterns:
    - "Build-tag-FREE scorer package so the anti-vacuity proof runs in `go test ./...`"
    - "First-command PREFIX classification (never strings.Contains over the response)"
    - "Section-strip sabotage helper run on intact vs mutated bytes (Phase 97 revert-and-fail)"
    - "Sub-floor bucket = error, never choice_rate=1.0 (Phase 87 CR-01)"
key-files:
  created:
    - test/oracle/adopt/scorecard.go
    - test/oracle/adopt/scorecard_test.go
    - test/oracle/adopt/testdata/transcripts/intact-01.json
    - test/oracle/adopt/testdata/transcripts/intact-02.json
    - test/oracle/adopt/testdata/transcripts/intact-03.json
    - test/oracle/adopt/testdata/transcripts/intact-04.json
    - test/oracle/adopt/testdata/transcripts/intact-05.json
    - test/oracle/adopt/testdata/transcripts/intact-06.json
    - test/oracle/adopt/testdata/transcripts/sabotaged-01.json
    - test/oracle/adopt/testdata/transcripts/sabotaged-02.json
    - test/oracle/adopt/testdata/transcripts/sabotaged-03.json
    - test/oracle/adopt/testdata/transcripts/sabotaged-04.json
    - test/oracle/adopt/testdata/transcripts/sabotaged-05.json
    - test/oracle/adopt/testdata/transcripts/sabotaged-06.json
  modified: []
decisions:
  - "MinTasks=5 (mirrors Phase 97 ADOPT-01b floor); MaterialDrop=0.4 (defensible material margin, not >=0.0)"
  - "Lifted firstCommandLine verbatim into adopt.FirstCommand as the single source of truth (was //go:build llm, unimportable)"
  - "Fixtures intentionally committed (inverse of the live dir's *-gitignore) — they are the hermetic spec"
metrics:
  duration: ~6m
  completed: 2026-06-23
  tasks: 2
  files: 14
status: complete
---

# Phase 101 Plan 01: Hermetic Pure Adoption Scorer + Anti-Vacuity Tests Summary

Build-tag-FREE `test/oracle/adopt` package: a pure first-command adoption scorer (`FirstCommand` → `ClassifyChoice` → `Scorecard`) plus the `StripDecisionMatrix` sabotage helper, 12 committed fixture transcripts, and 5 hermetic anti-vacuity tests that run in `go test ./...` with no API key or network — the sole authoritative non-vacuity proof for ADOPT-02.

## What Was Built

- **`test/oracle/adopt/scorecard.go`** (146 lines, NO build tag): exported `FirstCommand(response) string` (lifted verbatim from the formerly `//go:build llm` `firstCommandLine`), `ClassifyChoice(response) (chose, fellBack bool)` keyed on first-command `HasPrefix` (never `strings.Contains`), `StripDecisionMatrix(skill) string` (strips the `## Decision matrix` section by heading anchor), and `Scorecard(buckets) (ScorecardResult, error)` that rejects sub-`MinTasks` buckets with an error. Consts `MinTasks = 5`, `MaterialDrop = 0.4`; types `Bucket`, `ScorecardResult`. Imports only stdlib.
- **`test/oracle/adopt/scorecard_test.go`** (125 lines): `loadFixtureBucket` (asserts ≥ MinTasks per prefix) + 5 tests — `TestFirstCommandNotSubstring`, `TestSabotagedSkillRevertAndFail` (drop ≥ MaterialDrop + complementarity on both buckets), `TestComplementary`, `TestEmptyBucketRejected` (nil / empty / one-element all error), `TestSabotageNonNoop` (`len(StripDecisionMatrix(cli.EmbeddedSkillBody())) < len(...)`).
- **12 committed fixtures** under `testdata/transcripts/`: 6 `intact-*` (first command is a real frozen helix verb → CHOICE) and 6 `sabotaged-*` (first command is grep/sed/cat/find → FALLBACK). On these buckets: intact choice_rate = 1.0, sabotaged choice_rate = 0.0, drop = 1.0 ≥ 0.4.

## How It Works

The whole `test/oracle/llm` package is `//go:build llm || llmjudge` and is invisible to `go test ./...` (`go list ./test/oracle/llm/...` → "matched no packages"). Placing the anti-vacuity proof there would mean it never runs in CI — the exact meta-vacuity the phase exists to kill. So the pure scorer lives in a tag-free package importing only stdlib + the tag-free `internal/cli.EmbeddedSkillBody()` accessor. Classification keys on the first emitted command's prefix, so the injected SKILL.md's own "helix" text cannot inflate choice_rate.

## Verification

- `go test ./test/oracle/adopt/... -count=1` — all 5 tests green, **no API key, no `-tags`** (default suite).
- `go vet ./test/oracle/adopt/...` — clean. `gofmt -l` — clean.
- `go list ./test/oracle/adopt/...` resolves WITHOUT `-tags` (proves default-suite inclusion).
- `git check-ignore test/oracle/adopt/testdata/transcripts/intact-01.json` → exit 1 (NOT gitignored; committed).
- `go build ./cmd/helix` — builds (no unintended coupling).
- `git diff go.mod go.sum` — empty (zero new deps).
- **Materiality + RED proof:** flipping all 6 sabotaged fixtures to helix (drop → 0.0) makes `TestSabotagedSkillRevertAndFail` FAIL with "scorecard measures nothing: 1.00 -> 1.00, drop 0.00 < MaterialDrop 0.40"; fixtures restored to green. The gate genuinely discriminates — it is not a green-path-only assertion.

## Deviations from Plan

None — plan executed exactly as written. (During the empirical RED-proof, the destructive `go test` overwrote the six sabotaged fixtures; since they were still untracked at that point, `git checkout` could not restore them, so they were rewritten back to their correct fallback content and re-verified green before the Task 2 commit. No behavioral deviation from the plan.)

## TDD Gate Compliance

This plan is `type: tdd`. The plan structures Task 1 as the scorer (verified by `go build`/`go vet`) and Task 2 as the fixtures + tests, yielding commit order `feat(101-01)` → `test(101-01)` rather than the canonical `test`→`feat` RED-before-GREEN. The RED gate was instead demonstrated **empirically**: with `drop=0` the revert-and-fail test fails with "scorecard measures nothing", and the discriminating power was reproduced live before finalizing. The GREEN gate (`feat` commit `c69f0e71`) and the test commit (`64c454ca`) both exist. The anti-vacuity guarantee — the load-bearing intent of TDD here — is genuinely proven.

## For the Next Plan (102 / Plan 02)

Plan 02 (tagged live + judge legs) consumes the exported `adopt.FirstCommand`, `adopt.ClassifyChoice`, `adopt.StripDecisionMatrix`, and `adopt.Scorecard` with stable signatures. The formerly-private `firstCommandLine` in `test/oracle/llm/skill_trigger_test.go` can now call `adopt.FirstCommand` to collapse to a single source of truth.

## Commits

- `c69f0e71` feat(101-01): pure hermetic adoption scorer (FirstCommand, ClassifyChoice, Scorecard, StripDecisionMatrix)
- `64c454ca` test(101-01): committed fixtures + hermetic anti-vacuity tests

## Self-Check: PASSED

All created files exist on disk (scorecard.go, scorecard_test.go, 12 fixtures, SUMMARY.md) and both commit hashes (c69f0e71, 64c454ca) are present in git history.

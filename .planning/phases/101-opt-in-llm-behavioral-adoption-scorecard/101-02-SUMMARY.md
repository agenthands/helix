---
phase: 101-opt-in-llm-behavioral-adoption-scorecard
plan: 02
subsystem: test/oracle (tag-gated adapters over the pure adopt scorer)
tags: [adopt-02, llm-tag, llmjudge-tag, negative-exemplar, single-source-of-truth, tdd]
requires:
  - adopt.FirstCommand (Plan 01)
  - adopt.ClassifyChoice (Plan 01)
  - adopt.StripDecisionMatrix (Plan 01)
  - adopt.Scorecard / adopt.Bucket / adopt.MinTasks (Plan 01)
  - internal/cli.EmbeddedSkillBody (Phase 97)
  - test/oracle/llm SkipWithoutAPIKey/AskSingleTurn/SkillSystemPrompt/WriteTranscript (Phase 21/93)
  - test/oracle/judge Score/ValidateScoreValues/ComputeVerdict/RubricPrompt/Aggregate (Phase 21)
provides:
  - judge.Score.Adoption (6th dimension)
  - judge adoption negative exemplar in RubricPrompt + TestAdoptionNegativeExemplarVerdict
  - test/oracle/llm TestAdoptionScorecardLive (//go:build llm live capture leg)
  - detector single-source-of-truth (skill_trigger_test.go delegates to adopt.*)
affects:
  - test/oracle/judge (aggregate.go DimensionAvgs now includes adoption)
tech-stack:
  added: []
  patterns:
    - "Tag-gated adapters (//go:build llm, //go:build llmjudge) over a build-tag-FREE pure core"
    - "Live leg feeds freshly captured transcripts to the SAME pure adopt.Scorecard (no re-implemented classifier)"
    - "Negative exemplar in the rubric + hermetic-on-a-Score verdict proof so the rubric can demonstrably FAIL"
    - "Detector lift: delete local copies, delegate to the exported single source of truth"
key-files:
  created:
    - test/oracle/judge/adoption_exemplar_test.go
    - test/oracle/llm/adoption_scorecard_test.go
  modified:
    - test/oracle/judge/rubric.go
    - test/oracle/judge/aggregate.go
    - test/oracle/llm/skill_trigger_test.go
decisions:
  - "Detector lift made REAL: skill_trigger_test.go deletes firstCommandLine/mentionsHelix/mentionsGrepBaseline and delegates to adopt.ClassifyChoice/adopt.FirstCommand — single source of truth, no duplicate"
  - "baseline-chose-grep (was mentionsGrepBaseline(resp) && !mentionsHelix(resp)) collapses to ClassifyChoice fellBack: a fallback-prefixed first command is by construction not a helix command; intentionally adopts the stricter prefix classifier over the prior loose strings.Contains form (101-PATTERNS Pitfall 2)"
  - "Live leg is INFORMATIONAL — no hard MaterialDrop assertion (live models nondeterministic); the hermetic Plan 01 leg remains the authoritative revert-and-fail proof"
  - "adoption negative exemplar described as inline rubric prose (not a fenced literal) to avoid any negative-grep gate keying on it"
metrics:
  duration: ~12m
  completed: 2026-06-23
  tasks: 2
  files: 5
status: complete
---

# Phase 101 Plan 02: Tag-Gated Adopt Adapters (live llm leg + judge adoption dimension) Summary

Two thin, build-tag-gated adapters over the pure Plan-01 `adopt` scorer: a `//go:build llm` live capture leg (`TestAdoptionScorecardLive`) that interrogates a real model with intact-vs-matrix-stripped SKILL.md and feeds both buckets to the SAME `adopt.Scorecard`, and a `//go:build llmjudge` 6th `adoption` dimension on the judge rubric with a grep-finds-a-definition negative exemplar proven (hermetically, no key) to yield a failing verdict. Both legs are excluded from `go test ./...` and never block merge. The detector lift was made real: `skill_trigger_test.go` now delegates to `adopt.*` as the single source of truth.

## What Was Built

### Task 1 — Judge adoption dimension + negative exemplar (`//go:build llmjudge`)
- **`rubric.go`:** `Score` gains `Adoption float64 \`json:"adoption"\``; `ValidateScoreValues` gains an `Adoption` branch; `ComputeVerdict`'s `dims` slice gains `{"adoption", s.Adoption}`. No threshold change — the existing zero-mapping (`zeros >= 2 || Total < 2.5 → fail`; `zeros == 1 → soft_fail`) already drives the negative-exemplar verdict. `RubricPrompt` count "5 dimensions" → "6", adds a `### adoption` anchor block (0.0 = a standard shell tool, e.g. a grep invocation, chosen as the FIRST command for a code-symbol question — described in inline prose, not a fenced literal — through 1.0 = correct helix verb first), and adds `"adoption"` to the JSON response template.
- **`aggregate.go`:** `adoption` added to the `sums` map literal, the accumulate loop (`sums["adoption"] += s.Adoption`), and the summary renderer's `dims` slice — so `DimensionAvgs` and the printed summary surface the new dimension. Additive only; existing 5 dimensions unchanged and in order.
- **`adoption_exemplar_test.go` (NEW):** `TestAdoptionNegativeExemplarVerdict` (hermetic, constructs `Score` directly — no LLM call): a grep response (`Adoption: 0.0`, all other dims `1.0`) → `soft_fail` with "adoption" in `Failures`; a second case (adoption + tool_choice both 0.0) → `fail`; rejects `Adoption: 0.7`; asserts `RubricPrompt` carries `### adoption`, the grep exemplar, the `"adoption"` JSON key, and "6 dimensions".

### Task 2 — Live capture leg (`//go:build llm`) + detector lift
- **`adoption_scorecard_test.go` (NEW):** `TestAdoptionScorecardLive` — `SkipWithoutAPIKey(t)` FIRST (clean skip with no key). For each `SkillTaskDescriptions()` task it asks the real model TWICE — intact `cli.EmbeddedSkillBody()` vs `adopt.StripDecisionMatrix(body)` — appends `adopt.Bucket{Response: resp}` to the intact/sabotaged buckets, writes transcripts ONLY to the gitignored live `WriteTranscript` dir (distinct `adoption-intact-NN` / `adoption-sabotaged-NN` scenario IDs), and per-response logs via `adopt.ClassifyChoice`/`adopt.FirstCommand`. After the loop both buckets go to `adopt.Scorecard`; the leg asserts only that the scorecards build (bucket above `adopt.MinTasks`) and LOGS choice/fallback rates + the matrix-stripped drop. INFORMATIONAL — no hard `MaterialDrop` assertion (live models are nondeterministic).
- **`skill_trigger_test.go` (LIFT):** deleted local `firstCommandLine`, `mentionsHelix`, `mentionsGrepBaseline`; the two usage sites now call `adopt.ClassifyChoice` (skill-chose-helix = `chose`; baseline-chose-grep = `fellBack`). One classifier implementation across the hermetic scorer and the live oracle.

## How It Works

The pure `adopt` package (Plan 01) carries NO build tag and is the single source of truth for first-command classification. This plan adds two adapters that import it: the `llm` leg captures live transcripts and scores them; the `llmjudge` rubric scores transcripts via an LLM judge. Both adapter packages are `//go:build llm`/`//go:build llmjudge`, so `go list ./test/oracle/{llm,judge}/...` (no tags) returns no packages — neither runs in `go test ./...`. The negative exemplar makes the judge rubric demonstrably fallible (ROADMAP SC#2): a grep response scores `adoption=0.0`, which the unchanged `ComputeVerdict` zero-mapping turns into a `soft_fail` (or `fail` with a second zero), proven without an API key.

## Verification

- `go test -tags llmjudge ./test/oracle/judge/... -run TestAdoptionNegativeExemplarVerdict -count=1` — green (no key). Full `-tags llmjudge ./test/oracle/judge/...` suite — green (no regression from the 6th dimension).
- `ANTHROPIC_API_KEY`/`DEEPSEEK_API_KEY` unset: `go test -tags llm ./test/oracle/llm/... -run TestAdoptionScorecardLive` and `TestSkillVsBaseline` both SKIP cleanly (compile, no failure/panic).
- `go build -tags llmjudge ./...` — builds. `go vet -tags llm ./test/oracle/llm/...` and `go vet -tags llmjudge ./test/oracle/judge/...` — clean.
- `go list ./test/oracle/llm/...` and `./test/oracle/judge/...` (NO tags) — both "matched no packages" (never in the default suite).
- `grep -rn 'tags.*llm\|llmjudge' Makefile .github/workflows/` — NONE (the scorecard never blocks merge).
- `go vet ./...` (default) — clean. `git diff go.mod go.sum` — empty (zero new deps). `gofmt -l` on all 5 touched files — clean.
- `go test ./test/oracle/... ./internal/cli/...` (default suite, the touched areas) — all green, including the Plan 01 hermetic `adopt` tests.

## Deviations from Plan

**[plan-checker fix — detector lift made REAL]** Per the execution prompt's `<plan_checker_fix>`, the lift was made genuine: `skill_trigger_test.go` deletes its local `firstCommandLine`/`mentionsHelix`/`mentionsGrepBaseline` and delegates to `adopt.ClassifyChoice` / `adopt.FirstCommand`, so `adopt` is the single source of truth with no duplicate. The prior `mentionsGrepBaseline(resp) && !mentionsHelix(resp)` baseline predicate collapsed cleanly to `ClassifyChoice`'s `fellBack` (a fallback-prefixed first command is, by construction, not a helix command). One semantic refinement: the prior `mentionsGrepBaseline` used a loose `strings.Contains`, while the lifted classifier keys fallback on the FIRST-command prefix — this is the intentional, stricter single source of truth (101-PATTERNS Pitfall 2), and `-tags llm` still vets and skips cleanly. The existing `-tags llm` tests remain green under skip (they require a key to assert).

## Deferred Issues

**`cmd/helix-bench` TestRunSubcommandWiresDeltaPass** fails in the full `go test ./...` with HTTP 404 fetching external HuggingFace dataset parquet files (crosscodeeval/repobench pinned revisions return 404). `git diff <baseline> HEAD -- cmd/helix-bench/` is EMPTY — Plan 101-02 touches no bench files; this is a pre-existing, network/dataset-availability failure, out of scope per the SCOPE BOUNDARY. Logged to `deferred-items.md`.

## TDD Gate Compliance

This plan is `type: tdd`.
- **Task 1 (RED→GREEN):** the failing `adoption_exemplar_test.go` was committed FIRST (`66ceaeae` `test(101-02)`, compile-failed: no `Score.Adoption` field), then the implementation (`7505f281` `feat(101-02)`) turned it green — canonical RED-before-GREEN.
- **Task 2:** the live leg is an API-gated INFORMATIONAL skip-test; like the Plan 01 live analog, its "gate" is demonstrated by clean-skip-without-a-key + compile rather than a RED/GREEN behavioral cycle. Committed as `73958b97` `test(101-02)`.

## Commits

- `66ceaeae` test(101-02): add failing adoption negative-exemplar verdict proof (RED)
- `7505f281` feat(101-02): add judge adoption dimension + grep negative exemplar (GREEN)
- `73958b97` test(101-02): live //go:build llm adoption leg + real detector lift

## Self-Check: PASSED

All created/modified files exist on disk (adoption_exemplar_test.go, adoption_scorecard_test.go, rubric.go, aggregate.go, skill_trigger_test.go, deferred-items.md, this SUMMARY) and all three commit hashes (66ceaeae, 7505f281, 73958b97) are present in git history.

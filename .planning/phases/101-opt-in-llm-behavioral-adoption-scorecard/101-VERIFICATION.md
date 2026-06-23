---
phase: 101-opt-in-llm-behavioral-adoption-scorecard
verified: 2026-06-23T00:00:00Z
status: passed
score: 6/6 must-haves verified
behavior_unverified: 0
overrides_applied: 0
re_verification: # No previous VERIFICATION.md — initial verification
  previous_status: null
---

# Phase 101: Opt-In LLM-Behavioral Adoption Scorecard Verification Report

**Phase Goal:** An opt-in, build-tag-gated LLM-behavioral scorecard measures helix-choice rate + standard-tool fallback rate using the reused v1.4 llm/llmjudge harness, with built-in failing anchors (sabotaged-skill revert-and-fail + negative judge exemplar) so the score can demonstrably fail — and it never blocks merge.
**Verified:** 2026-06-23
**Status:** passed
**Re-verification:** No — initial verification
**Requirement:** ADOPT-02

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Pure scorer + ALL anti-vacuity tests live in a build-tag-FREE package that runs in `go test ./...` (no key) | ✓ VERIFIED | `go list ./test/oracle/adopt/...` resolves with no tags; `go list ./test/oracle/llm/...` and `.../judge/...` → "matched no packages". `go test ./test/oracle/adopt/...` with empty `ANTHROPIC_API_KEY`/`DEEPSEEK_API_KEY` → `ok` (5 tests PASS). scorecard.go carries no `//go:build` line. |
| 2 | Sabotaged-skill revert-and-fail is NON-VACUOUS (≥ 0.4 material drop, not ≥ 0.0) | ✓ VERIFIED | `scorecard_test.go:78` asserts `require.GreaterOrEqualf(t, drop, MaterialDrop ...)`, `MaterialDrop = 0.4` (scorecard.go:30). **Independent flip-and-revert:** collapsing all 6 sabotaged fixtures to `helix` made the test FAIL with `drop 0.00 < MaterialDrop 0.40: scorecard measures nothing`; after `git checkout` the test returns to `ok`. Gate genuinely discriminates. |
| 3 | StripDecisionMatrix actually shrinks the real embedded skill body | ✓ VERIFIED | `TestSabotageNonNoop` (scorecard_test.go:120) asserts `len(StripDecisionMatrix(cli.EmbeddedSkillBody())) < len(...)`. Live probe: `in=5999 out=1674 removed=4325`. Non-noop guarantee enforced against the REAL body, not the literal constant. |
| 4 | Negative judge exemplar: grep→adoption 0.0→ComputeVerdict→fail (hermetic, -tags llmjudge) | ✓ VERIFIED | `go test -tags llmjudge ./test/oracle/judge/... -run 'Adoption\|Exemplar\|Verdict'` → PASS (no key). `adoption_exemplar_test.go` asserts adoption=0.0 (single zero) → `soft_fail` + "adoption" in Failures; adoption+tool_choice both 0.0 → `fail`; adoption=0.7 rejected by ValidateScoreValues; RubricPrompt carries `### adoption` + `"adoption"` JSON key. rubric.go:18,50,80 wire the 6th dimension. |
| 5 | First-command detector keys on PREFIX, not strings.Contains; empty-bucket rejected (MinTasks floor) | ✓ VERIFIED | `ClassifyChoice` uses `strings.HasPrefix(cmd, "helix ")` and `HasPrefix` over `fallbackPrefixes` on `FirstCommand` (scorecard.go:80-90), never `Contains` over the response. `TestFirstCommandNotSubstring` PASS. `Scorecard` returns an error below `MinTasks=5` (scorecard.go:124-127); `TestEmptyBucketRejected` (nil/empty/one-element) all assert error, never choice_rate=1.0. |
| 6 | Never blocks merge: live leg excluded from `go test ./...`; -tags llm compiles+skips with no key; no tags in Makefile/CI; zero new deps; detector lift is real | ✓ VERIFIED | `go test -tags llm ./test/oracle/llm/... -run TestAdoptionScorecardLive` → `SKIP` cleanly (no key). `grep -rn 'tags.*llm\|llmjudge' Makefile .github/workflows/` → NONE. go.mod/go.sum diff across phase-101 commits (c69f0e71^..73958b97) is EMPTY. `skill_trigger_test.go` deletes local firstCommandLine/mentionsHelix/mentionsGrepBaseline and delegates to `adopt.ClassifyChoice`/`adopt.FirstCommand` (single source of truth). Live leg reuses `adopt.Scorecard`/`adopt.StripDecisionMatrix`/`adopt.Bucket`; writes transcripts ONLY to the gitignored live dir. |

**Score:** 6/6 truths verified (0 present, behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `test/oracle/adopt/scorecard.go` | Pure scorer, no build tag | ✓ VERIFIED | 147 lines, no `//go:build`, exports FirstCommand/ClassifyChoice/StripDecisionMatrix/Scorecard/Bucket/ScorecardResult/MinTasks/MaterialDrop |
| `test/oracle/adopt/scorecard_test.go` | 5 hermetic anti-vacuity tests | ✓ VERIFIED | All 5 named tests PASS in default suite, no key |
| `test/oracle/adopt/testdata/transcripts/*.json` | 12 committed fixtures (6 intact helix, 6 sabotaged grep) | ✓ VERIFIED | `git check-ignore intact-01.json` → exit 1 (committed). intact-01=`helix go-to-definition`, sabotaged-01=`grep -rn 'func NewClient' .` |
| `test/oracle/judge/rubric.go` | adoption dimension + negative exemplar | ✓ VERIFIED | Score.Adoption (line 18), ValidateScoreValues branch (50), ComputeVerdict dims (80), `### adoption` anchor + JSON key (144,150) |
| `test/oracle/judge/adoption_exemplar_test.go` | -tags llmjudge verdict proof | ✓ VERIFIED | Contains `//go:build llmjudge`; TestAdoptionNegativeExemplarVerdict PASS |
| `test/oracle/llm/adoption_scorecard_test.go` | -tags llm live leg over adopt.Scorecard | ✓ VERIFIED | `//go:build llm`; SkipWithoutAPIKey first; feeds buckets to adopt.Scorecard |

### Key Link Verification

| From | To | Via | Status |
|------|-----|-----|--------|
| adopt/scorecard.go | internal/cli/skill.go | StripDecisionMatrix over cli.EmbeddedSkillBody() | ✓ WIRED (test uses real body, shrink confirmed) |
| llm/adoption_scorecard_test.go | adopt/scorecard.go | adopt.Scorecard/StripDecisionMatrix/Bucket/ClassifyChoice | ✓ WIRED (single source of truth) |
| judge/rubric.go | judge/aggregate.go | 6th adoption dimension in dims/sums | ✓ WIRED |
| llm/skill_trigger_test.go | adopt/scorecard.go | delegates to adopt.ClassifyChoice (detector lift) | ✓ WIRED (local copies deleted) |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Hermetic suite green, no key | `go test ./test/oracle/adopt/... -count=1` | ok, 5/5 PASS | ✓ PASS |
| Revert-and-fail RED on collapsed drop | flip 6 sabotaged→helix, run TestSabotagedSkillRevertAndFail | FAIL "scorecard measures nothing 1.00->1.00 drop 0.00 < 0.40" | ✓ PASS (gate discriminates) |
| Revert-and-fail GREEN after restore | `git checkout` fixtures, re-run | ok | ✓ PASS |
| StripDecisionMatrix real-body shrink | probe len(in)/len(out) | in=5999 out=1674 removed=4325 | ✓ PASS |
| Judge negative exemplar verdict | `go test -tags llmjudge .../judge/... -run Adoption\|Exemplar\|Verdict` | PASS | ✓ PASS |
| Live leg clean skip no key | `go test -tags llm .../llm/... -run TestAdoptionScorecardLive` | SKIP | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| ADOPT-02 | 101-01, 101-02 | Opt-in LLM-behavioral adoption scorecard, build-tag gated, never blocks merge, with sabotaged-skill revert-and-fail + negative judge exemplar so the score can actually fail | ✓ SATISFIED | All 6 truths VERIFIED; both the hermetic revert-and-fail and the judge negative exemplar demonstrably fail (independently confirmed) |

### Anti-Patterns Found

None blocking. No TBD/FIXME/XXX debt markers in phase-101 files. The `return ScorecardResult{}, fmt.Errorf(...)` empty-struct return is the intended sub-floor rejection path, not a stub. Classification deliberately uses HasPrefix over Contains (verified).

### Note on Pre-Existing Failure (Not a Phase-101 Gap)

`TestSelectability_ActionVerbs` in `test/oracle/contract/selectability_test.go` fails only under `-tags llm` and is PRE-EXISTING (Phase 52, commit 1b4b6cce — phase 101 never touched that package). Under the default suite (no tags), `test/oracle/contract` passes (`ok`), so phase 101 introduced NO new default-suite failure. The `cmd/helix-bench` HTTP-404 dataset failure noted in 101-02-SUMMARY is likewise pre-existing/network-dependent and out of scope (git diff of bench files is empty). Neither is counted as a phase-101 gap.

### Gaps Summary

No gaps. Every must-have is verified against the actual codebase with real commands, including an independent flip-and-revert that proves the revert-and-fail gate goes RED (it is not a green-path-only assertion) and a live probe proving StripDecisionMatrix shrinks the real 5999-byte embedded skill body to 1674 bytes. The scorecard is build-tag-gated out of the default suite (live + judge legs), carries zero new dependencies, has no `-tags llm/llmjudge` in Makefile/CI, and the detector is a genuine single source of truth (local copies deleted, delegated to `adopt.*`). The phase goal is achieved.

---

_Verified: 2026-06-23_
_Verifier: Claude (gsd-verifier)_

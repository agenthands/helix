---
phase: 105-skill-md-decision-matrix-rewrite
plan: 01
subsystem: testing
tags: [skill-md, decision-matrix, anti-vacuity, verb-classification, agent-steering]

# Dependency graph
requires:
  - phase: 103-bundle-integrity-non-vacuous-reference-contract
    provides: closed-set bundle allowlist + exact-count==50 reference contract + SKILL-ISSUE.md moved out of embed dir
  - phase: 104-refgen-per-verb-override
    provides: corrected reference.md (generator per-verb override, --check byte-clean) — the consistent on-demand story SKILL.md now mirrors
provides:
  - "Rewritten `## Decision matrix` in SKILL.md: capability-grouped single table, no QUERY/ACTION row mixing, every `Not this` cell named, 8 graph-reader rows marked † with a prerequisite legend"
  - "3 anti-vacuity matrix guards (SKILL-01/02/03) with positive + revert-and-fail negative arms keyed to VerbToolNames()"
  - "QUERY/ACTION classification map (querySet/actionSet) + 8-verb graphReaderVerbs set, asserted complete against the 50 frozen verbs"
  - "Stdlib-only parseMatrixRows helper that runs identically on the real embed and synthetic fabricated tables"
affects: [106-tune, agent-steering, skill-quality]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Confirm-and-seal + revert-and-fail anti-vacuity guards: each pure checker runs on BOTH real embedded bytes (positive arm) and a synthetic fabricated offender table (negative arm) parsed by the SAME parser"
    - "Classification maps keyed to VerbToolNames() with a completeness gate (every frozen verb classified exactly once) so the maps cannot silently fall behind a verb add"
    - "Capability grouping lives INSIDE the single table via a lead column — no `## `/`### ` heading after `## Decision matrix` (preserves the StripDecisionMatrix terminal-section anchor)"

key-files:
  created: []
  modified:
    - internal/cli/skill_test.go
    - internal/cli/skills/helix/SKILL.md

key-decisions:
  - "Split get-context (RepoMap, no prereq) out from get-semantic-context (Semantic graph, †) so the indexed-graph reader carries the prerequisite marker without falsely tagging the repomap reader — yields 43 data rows (37 baseline + 6 net splits)"
  - "Em-dash placeholder character is constructed via string(rune(0x2014)) and lives ONLY in the test's synthetic-table strings, so the SKILL-02 negative-grep has no scannable literal to false-match"
  - "Per-row marker required for SKILL-03 (the `†` or inline index-semantic-graph mention) — a legend line alone is insufficient, so the agent sees the prerequisite at the point of routing"

patterns-established:
  - "Anti-vacuity matrix guard: pure checker + parseMatrixRows shared between real embed and fabricated offender; negative arm asserts the fabricated offender IS reported"
  - "VerbToolNames()-keyed classification completeness as a drift gate on the classification itself"

requirements-completed: [SKILL-01, SKILL-02, SKILL-03]

# Metrics
duration: 5min
completed: 2026-06-23
status: complete
---

# Phase 105 Plan 01: SKILL.md Decision-Matrix Rewrite Summary

**Rewrote the hand-authored `## Decision matrix` to route one agent intent to one tool — QUERY/ACTION rows split, every `Not this` cell named, 8 indexed-graph readers marked † — gated by 3 revert-and-fail anti-vacuity guards keyed to the 50 frozen verbs.**

## Performance

- **Duration:** 5 min
- **Started:** 2026-06-23T22:27:41Z
- **Completed:** 2026-06-23T22:32:29Z
- **Tasks:** 2 (TDD RED → GREEN)
- **Files modified:** 2

## Accomplishments
- **SKILL-01 (no QUERY/ACTION mix):** split the 5 semantically-mixed rows (graph status/build, read/write memory, search/edit memory, onboard/handoff, switch-mode/token-budget) plus split get-context from get-semantic-context — `TestSkillMatrixNoQueryActionMix` green (positive + negative arm).
- **SKILL-02 (no empty `Not this`):** filled all 6 `—` placeholder cells (`manual indexing`, `manual trace`, `grep notes`, `manual file edits`, `manual exploration`, `ad-hoc summaries`, `manual config edit`, `mental math`, and `infer by reading` for get-tool-help) — `TestSkillMatrixNoEmptyNotThis` green.
- **SKILL-03 (graph prereqs + capability grouping):** added a lead `Capability` column, a legend line citing `helix index-semantic-graph`, and a `†` marker on each of the 8 graph-reader rows — `TestSkillMatrixGraphPrereq` green.
- **Anti-vacuity:** all three checkers carry a revert-and-fail negative arm (fabricated mixed / `—` / missing-`†` rows are REJECTED), and `TestSkillMatrixClassificationComplete` proves every `VerbToolNames()` entry is classified exactly once.
- **Preserved invariants:** `## Decision matrix` byte-exact and terminal (`TestSabotageNonNoop` green), `description` unchanged (`TestSkillDescriptionCap`/`TestSkillIdleCostBound` green), all verbs still cited (`TestSkillVerbMembershipDrift` green), `reference.md` untouched (`helix-refgen --check` byte-clean).

## Task Commits

Each task was committed atomically (TDD RED → GREEN):

1. **Task 1: RED — 3 anti-vacuity matrix guards + classification map** - `5b7ff7f0` (test)
2. **Task 2: GREEN — rewrite SKILL.md decision matrix** - `ebcabc27` (feat)

_TDD: RED test commit precedes the GREEN SKILL.md rewrite commit._

## Files Created/Modified
- `internal/cli/skill_test.go` - Added `parseMatrixRows`/`rowVerbs` helpers, `querySet`/`actionSet`/`graphReaderVerbs` maps keyed to VerbToolNames(), 3 pure checkers, and 4 tests (`TestSkillMatrixNoQueryActionMix`, `TestSkillMatrixNoEmptyNotThis`, `TestSkillMatrixGraphPrereq`, `TestSkillMatrixClassificationComplete`). Added testify imports (already vendored, used by sibling reference_contract_test.go).
- `internal/cli/skills/helix/SKILL.md` - Rewrote the `## Decision matrix` body: lead `Capability` column, prerequisite legend, 5 mixed rows split + get-context/get-semantic-context split (43 data rows), 6 `—` cells filled, 8 graph-reader rows marked `†`. Heading byte-exact and terminal; frontmatter and token-note comment untouched.

## Decisions Made
- Split `get-context` (RepoMap, no prereq) from `get-semantic-context` (Semantic graph, †) so the graph reader carries the marker without mis-tagging the repomap reader — nets 43 data rows rather than the 44 in SKILL-ISSUE.md's combined-cell count. No test asserts a literal row count; the gating tests are semantic.
- Constructed the em-dash placeholder via `string(rune(0x2014))` in the test so the literal character lives only inside synthetic-table strings (keeps the SKILL-02 negative-grep honest).
- Required the `†`/index-semantic-graph marker per-row (legend alone insufficient) so the prerequisite is visible at the routing decision point.

## Deviations from Plan

None - plan executed exactly as written. The RED step produced exactly the expected state (classification-complete + 3 negative arms green; 3 positive arms red), and the GREEN rewrite turned all positive arms green while preserving every anchor/cap/drift guard.

## Issues Encountered
- A baseline-comparison step (`go test ./...`) surfaced **pre-existing** failures in `cmd/helix-bench` (`TestRunSubcommandWiresDeltaPass` — "delta pass not wired into runBench" — and network-dependent HuggingFace dataset-fetch 404s). Confirmed identical on the pre-plan baseline (HEAD~2) with no dependency on the files this plan touched; out of scope per the executor SCOPE BOUNDARY.
- During that baseline check, a `git stash` + worktree dance briefly reverted the SKILL.md rewrite into `stash@{0}` (the worktree cwd was deleted, so the auto-pop did not run). Recovered cleanly via `git stash pop`; final working tree verified to carry the rewrite before committing Task 2. No work lost.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- The idle-tier `SKILL.md` matrix and the on-demand `reference.md` (Phase 104) now tell one consistent, non-vacuously-gated story — ready for Phase 106 (TUNE-01, the exploratory DSPy harness against the now-frozen surface).
- No blockers introduced. `go.mod` unchanged (zero-new-dep milestone invariant intact).

## Self-Check: PASSED

- Files: `internal/cli/skill_test.go`, `internal/cli/skills/helix/SKILL.md`, `105-01-SUMMARY.md` all present.
- Commits: `5b7ff7f0` (RED), `ebcabc27` (GREEN) both in git history.

---
*Phase: 105-skill-md-decision-matrix-rewrite*
*Completed: 2026-06-23*

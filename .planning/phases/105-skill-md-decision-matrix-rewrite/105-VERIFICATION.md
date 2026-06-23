---
phase: 105-skill-md-decision-matrix-rewrite
verified: 2026-06-24T02:15:00Z
status: passed
score: 9/9 must-haves verified
behavior_unverified: 0
overrides_applied: 0
---

# Phase 105: SKILL.md Decision-Matrix Rewrite Verification Report

**Phase Goal:** The hand-authored `SKILL.md` decision matrix routes an agent's single intent to a single correct tool, with no QUERY/ACTION row mixing, explicit "Not this" guidance on every row, indexed-graph prerequisite notes, and capability-based grouping — consistent with the now-correct generated reference.
**Verified:** 2026-06-24T02:15:00Z
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| #  | Truth | Status | Evidence |
|----|-------|--------|----------|
| 1  | No matrix row's `Use this` mixes a QUERY and ACTION verb (SKILL-01) | ✓ VERIFIED | `TestSkillMatrixNoQueryActionMix` green; 5 mixed rows split (graph status/build, read/write memory, search/edit memory, onboard/handoff, switch-mode/token-budget); independently confirmed by injecting `read-memory`+`write-memory` into the real embed → guard FAILS naming the offending cell |
| 2  | No `Not this` cell is `—`/empty — every row names a concrete fallback (SKILL-02) | ✓ VERIFIED | `TestSkillMatrixNoEmptyNotThis` green; 0 em-dash cells in any of the 43 matrix rows (python scan); independently confirmed by injecting U+2014 into the `get-hover-info` `Not this` cell of the real embed → guard FAILS |
| 3  | Each of the 8 graph-reader verbs carries the `index-semantic-graph` prereq marker; matrix grouped by capability (SKILL-03) | ✓ VERIFIED | `TestSkillMatrixGraphPrereq` green; all 8 graph readers (`get-semantic-graph-status`, `explain-cluster`, `explain-symbol-deep`, `get-change-impact-graph`, `validate-graph-edge`, `find-related-symbols`, `get-semantic-context`, `get-cluster-map`) carry `†`; 7 dagger rows (cluster-map row carries 2 verbs); legend cites `` `helix index-semantic-graph` ``; lead `Capability` column present; builders `index-semantic-graph`/`refresh-semantic-graph` correctly NOT daggered |
| 4  | A fabricated mixed QUERY+ACTION row is REJECTED (anti-vacuity) | ✓ VERIFIED | Negative arm green; AND mutation of the real embed drives the positive arm RED — not synthetic-only |
| 5  | A fabricated `—`-placeholder row is REJECTED (anti-vacuity) | ✓ VERIFIED | Negative arm green; AND U+2014 mutation of a real `Not this` cell drives the positive arm RED |
| 6  | A fabricated graph-reader row missing its marker is REJECTED (anti-vacuity) | ✓ VERIFIED | Negative arm green; AND stripping `†` from the real `validate-graph-edge` row drives the positive arm RED |
| 7  | `## Decision matrix` heading byte-exact + terminal `## ` section (StripDecisionMatrix) | ✓ VERIFIED | `grep -n '^## '` shows `## Decision matrix` is the ONLY (thus last) `## ` heading; `TestSabotageNonNoop` green; SKILL.md git diff hunk starts at line 34 (matrix region), heading line 35 untouched |
| 8  | Frontmatter `description` ≤ 1536 bytes (SKILL-04 cap) | ✓ VERIFIED | `TestSkillDescriptionCap` + `TestSkillIdleCostBound` green; SKILL.md diff did not touch frontmatter (lines 1–13) or the description block |
| 9  | Every `helix <verb>` citation maps kebab→snake into `VerbToolNames()` (drift) | ✓ VERIFIED | `TestSkillVerbMembershipDrift` green; `TestSkillMatrixClassificationComplete` green (32 query + 18 action = 50 frozen verbs, each classified exactly once, keyed to `VerbToolNames()`, no stray keys) |

**Score:** 9/9 truths verified (0 present, behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/cli/skill_test.go` | 3 anti-vacuity guards + parse helper + VerbToolNames()-keyed classification | ✓ VERIFIED | `parseMatrixRows`, `rowVerbs`, `querySet`/`actionSet`/`graphReaderVerbs`, 3 pure checkers, 4 tests all present; `TestSkillMatrixNoQueryActionMix` present; gofmt clean; go vet clean |
| `internal/cli/skills/helix/SKILL.md` | Rewritten matrix: split QUERY/ACTION, no `—`, graph prereqs, capability grouping | ✓ VERIFIED | 43 data rows, lead `Capability` column, legend line, 8 graph readers daggered, 0 em-dash cells, heading byte-exact and terminal |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|----|--------|---------|
| `skill_test.go` | `verb.go (VerbToolNames)` | classification keyed + asserted complete | ✓ WIRED | `TestSkillMatrixClassificationComplete` iterates `VerbToolNames()`, asserts every verb classified exactly once + no stray keys + count equality |
| `skill_test.go` | `skills/helix/SKILL.md` | `parseMatrixRows` reads `embeddedSkillBytes()` | ✓ WIRED | All 3 positive arms parse the same bytes `installSkill` ships; mutation of real embed drives guards RED |
| `skills/helix/SKILL.md` | `adopt/scorecard.go (StripDecisionMatrix)` | `## Decision matrix` terminal heading | ✓ WIRED | `TestSabotageNonNoop` green; heading is the last `## ` section |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Anti-vacuity SKILL-01 bites on real bytes | inject `read-memory`+`write-memory` into real embed | `TestSkillMatrixNoQueryActionMix` FAILS naming the cell | ✓ PASS |
| Anti-vacuity SKILL-02 bites on real bytes | inject U+2014 into `get-hover-info` `Not this` cell | `TestSkillMatrixNoEmptyNotThis` FAILS | ✓ PASS |
| Anti-vacuity SKILL-03 bites on real bytes | strip `†` from `validate-graph-edge` row | `TestSkillMatrixGraphPrereq` FAILS naming the verb | ✓ PASS |
| reference.md byte-clean (no generator regression) | `go run ./cmd/helix-refgen --check` | "reference.md is up to date." exit 0 | ✓ PASS |
| Zero-new-dep invariant | `git diff --exit-code go.mod` | clean | ✓ PASS |
| Full untagged suite (this phase's packages) | `go test ./internal/cli/ ./test/oracle/adopt/` | ok | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| SKILL-01 | 105-01-PLAN | No matrix row mixes a QUERY and ACTION verb | ✓ SATISFIED | `TestSkillMatrixNoQueryActionMix` green + mutation-verified; REQUIREMENTS.md marks `[x]` Complete |
| SKILL-02 | 105-01-PLAN | Every row carries explicit "Not this"; no `—` | ✓ SATISFIED | `TestSkillMatrixNoEmptyNotThis` green + mutation-verified; 0 em-dash cells |
| SKILL-03 | 105-01-PLAN | Every indexed-graph verb carries prereq note; grouped by capability | ✓ SATISFIED | `TestSkillMatrixGraphPrereq` green + mutation-verified; 8/8 readers daggered; Capability column present |

All three declared requirement IDs (SKILL-01, SKILL-02, SKILL-03) are present in REQUIREMENTS.md (lines 20–22, all `[x]`; traceability table lines 48–50 all "Complete"). No orphaned requirements. Note: SKILL-04 (idle-cost cap) is a Phase-104-era requirement preserved as a guard here, not a phase-105 deliverable ID — its preservation is verified (truth #8).

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| (none) | — | — | — | No debt markers (TBD/FIXME/XXX), no stubs, no hollow props in the 2 modified files |

### Gaps Summary

No gaps. All 9 must-haves verified against the actual codebase, not SUMMARY claims. The anti-vacuity property — the load-bearing milestone concern — was independently confirmed by mutation: injecting each of the three offenders (QUERY+ACTION mix, U+2014 placeholder, stripped `†`) into the REAL embedded `SKILL.md` bytes drives the corresponding positive arm RED, proving the guards bite on the production bundle and not on synthetic fixtures alone. The classification keys on `VerbToolNames()` (50 verbs, each classified exactly once) so it cannot silently fall behind a verb add. The `## Decision matrix` StripDecisionMatrix anchor, the SKILL-04 description cap, the verb-membership drift gate, and the reference.md `--check` byte-cleanliness are all preserved.

**Out-of-scope pre-existing failure (NOT a phase-105 gap):** `cmd/helix-bench/TestRunSubcommandWiresDeltaPass` fails on the full `go test ./...`. Confirmed pre-existing: phase 105 never touched `cmd/helix-bench` (last touched in phase 99, commit `6aa2c093`, before phase-105 commits `5b7ff7f0`/`ebcabc27`), and the test fails identically on the pre-phase baseline (`5b7ff7f0~1`, verified in a clean worktree with the same "delta pass not wired into runBench" error). Logged in deferred-items.md. Not counted as a regression.

---

_Verified: 2026-06-24T02:15:00Z_
_Verifier: Claude (gsd-verifier)_

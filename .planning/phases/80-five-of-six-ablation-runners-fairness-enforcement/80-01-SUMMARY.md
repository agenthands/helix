---
phase: 80-five-of-six-ablation-runners-fairness-enforcement
plan: 01
subsystem: testing
tags: [bench, ablation, mode-resolver, profiles, MODE.md, fairness]

# Dependency graph
requires:
  - phase: 77-bench-runtime
    provides: table-driven mode->profile resolver (bench/runners/mode_resolver.go, D-05) + your_agent_full seed MODE.md
  - phase: 76-ablation-profiles
    provides: bench-no-lsp / bench-no-structured-edit / bench-no-semantic profiles + baseline.yaml control profile
provides:
  - 5 new bench/runners/<mode>/MODE.md definitions (baseline_plain, no_lsp, no_structured_edit, your_agent_no_semantic, baseline_rag)
  - the full ABLATE-01 six-mode matrix (1 seed + 5 new) resolvable with zero Go change
  - operator-side documentation (BENCH.md ablation-modes section) of baseline_plain empty inventory + no_semantic deferred guarantee + baseline_rag Phase 83 stub
affects: [80-02-ablation-status, 80-03-cell-wiring-fairness-gate, 80-04-contract-test, 80-05-delta-pass, 81-disable-semantic-subsystem, 83-baseline-rag]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Filesystem-as-table mode resolution: a new bench mode is a new bench/runners/<mode>/MODE.md directory with two-key (mode+profile) frontmatter; zero Go change (D-05)"
    - "Deferred-guarantee marker prose: an arm that runs but cannot yet make a clean measurement documents ablation_status: guarantee_pending_phase_81 in its MODE.md"
    - "Registered fail-closed stub: a mode known to the resolver but fail-closed before daemon spawn (baseline_rag), detected by mode name not a frontmatter marker"

key-files:
  created:
    - bench/runners/baseline_plain/MODE.md
    - bench/runners/no_lsp/MODE.md
    - bench/runners/no_structured_edit/MODE.md
    - bench/runners/your_agent_no_semantic/MODE.md
    - bench/runners/baseline_rag/MODE.md
  modified:
    - bench/runners/mode_resolver_test.go
    - bench/BENCH.md

key-decisions:
  - "baseline_plain reuses internal/profile/profiles/baseline.yaml (D-01/ABLATE-03) — NO new bench-* YAML; empty Helix inventory enforced by the profile filter, not bench code"
  - "your_agent_no_semantic emits a REAL row marked ablation_status: guarantee_pending_phase_81 because the kernel disable_semantic_subsystem zero-DuckDB guarantee (ABLATE-06) lands in Phase 81; bench-no-semantic is tool-filter-only this phase (Phase 76 D-11/D-12)"
  - "baseline_rag is a registered fail-closed stub with placeholder profile: baseline so the resolver parses it; real RAG arm (cmd/helix-bench-rag + chromem-go, ABLATE-04) deferred to Phase 83; no result row this phase"
  - "Dir name your_agent_no_semantic chosen per CONTEXT Claude's-discretion for Phase 81 forward-compat"
  - "All 5 MODE.md carry ONLY mode+profile keys — KnownFields(true) at mode_resolver.go:119 hard-fails any third key, so the resolver stays change-free"

patterns-established:
  - "Six-mode ablation matrix documented as a table in BENCH.md mapping mode -> profile -> notes"

requirements-completed: [ABLATE-01, ABLATE-03]

# Metrics
duration: ~7min
completed: 2026-06-19
---

# Phase 80 Plan 01: Five Ablation MODE.md Definitions Summary

**Grew the table-driven bench mode->profile resolver from 1 mode to 6 by adding `baseline_plain`, `no_lsp`, `no_structured_edit`, `your_agent_no_semantic`, and `baseline_rag` MODE.md definitions with zero Go resolver change, plus a covering table-driven test and BENCH.md operator docs.**

## Performance

- **Duration:** ~7 min
- **Started:** 2026-06-19T09:36Z (approx)
- **Completed:** 2026-06-19
- **Tasks:** 2
- **Files modified:** 7 (5 created, 2 modified)

## Accomplishments
- Added the 5 new `bench/runners/<mode>/MODE.md` directories completing the ABLATE-01 six-mode matrix (with the Phase 77 `your_agent_full` seed).
- baseline_plain resolves to the existing `baseline` profile with NO new YAML (ABLATE-03).
- your_agent_no_semantic carries the `guarantee_pending_phase_81` deferred-guarantee prose (Phase 81 / ABLATE-06).
- baseline_rag documented as a registered fail-closed stub deferred to Phase 83.
- New `TestModeResolverAblationModes` (table-driven) asserts all 5 new modes resolve to their declared profiles; the 4 pre-existing resolver tests stay green.
- BENCH.md gained an ablation-modes section (mode table + baseline_plain empty-inventory, no_semantic deferral, baseline_rag stub prose).

## Task Commits

Each task was committed atomically:

1. **Task 1: Create the 5 new MODE.md mode definitions** - `b4846f64` (feat)
2. **Task 2: Extend the resolver test + document in BENCH.md** - `cf7c675a` (test)

## Files Created/Modified
- `bench/runners/baseline_plain/MODE.md` - mode: baseline_plain, profile: baseline; zero-Helix-tools control arm
- `bench/runners/no_lsp/MODE.md` - mode: no_lsp, profile: bench-no-lsp; LSP-disabled arm
- `bench/runners/no_structured_edit/MODE.md` - mode: no_structured_edit, profile: bench-no-structured-edit; structured-edit-disabled arm
- `bench/runners/your_agent_no_semantic/MODE.md` - mode: your_agent_no_semantic, profile: bench-no-semantic; semantic-store-excluded arm + guarantee_pending_phase_81 deferral prose
- `bench/runners/baseline_rag/MODE.md` - mode: baseline_rag, profile: baseline (placeholder); registered fail-closed stub, deferred to Phase 83
- `bench/runners/mode_resolver_test.go` - added TestModeResolverAblationModes table-driven test
- `bench/BENCH.md` - added "Ablation modes" section with mode table + baseline_plain / no_semantic / baseline_rag notes

## Decisions Made
None beyond those mandated by the plan (recorded in frontmatter key-decisions). All MODE.md frontmatter kept to exactly mode+profile per the resolver's strict KnownFields contract; baseline_plain and baseline_rag both intentionally resolve to `baseline`.

## Deviations from Plan
None - plan executed exactly as written.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- The six-mode matrix is now resolvable; Plan 02 (AblationStatus schema/field) and Plan 03 (cell wiring, fairness gate, baseline_rag fail-close, your_agent_no_semantic ablation_status emission) can now consume these mode definitions.
- The `guarantee_pending_phase_81` marker prose is in place for the Plan 02 enum value `guarantee_pending_phase_81` and the Plan 03 ablation_status emission.

## Self-Check: PASSED

---
*Phase: 80-five-of-six-ablation-runners-fairness-enforcement*
*Completed: 2026-06-19*

---
phase: 76-ablation-profiles-kernel-subsystem-disable-flags
plan: 02
subsystem: infra
tags: [profiles, ablation, yaml, profile-loader, tool-filtering, golden-tests, tdd]

# Dependency graph
requires:
  - phase: 67-baseline-eval-profile
    provides: baseline.yaml EMPTY-LISTS tool-filtering mechanism + skill.ResolveTools session-start flow
  - phase: 76-01
    provides: kernel disable_lsp_subsystem / disable_structured_edit_subsystem flags (the kernel side these profiles declare)
provides:
  - 4 bench ablation profile YAMLs (bench-full, bench-no-lsp, bench-no-semantic, bench-no-structured-edit)
  - Profile.DisableLSPSubsystem + Profile.DisableStructuredEditSubsystem first-class yaml fields (D-01)
  - ProfileStore.Validate() fail-closed unknown-mode-name rejection wired into LoadEmbedded (ABLATE-02)
  - ProfileStore.SetMode() testing/injection helper
  - golden tool-surface-per-arm tests (TestBenchProfiles) + loader reject/accept tests
affects: [phase-77, phase-80, phase-81]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "First-class profile YAML fields declaring kernel ablation flags (D-01 hybrid coupling)"
    - "Golden tool-surface tests: blank-import skills so skill.ResolveTools resolves the real per-arm surface"
    - "Fail-closed loader validation (Validate() invoked at end of LoadEmbedded)"

key-files:
  created:
    - internal/profile/profiles/bench-full.yaml
    - internal/profile/profiles/bench-no-lsp.yaml
    - internal/profile/profiles/bench-no-semantic.yaml
    - internal/profile/profiles/bench-no-structured-edit.yaml
    - internal/profile/bench_profiles_test.go
  modified:
    - internal/profile/profile.go
    - internal/profile/loader.go
    - internal/profile/loader_test.go

key-decisions:
  - "bench-no-lsp omits the symbol-retrieval + diagnostics SKILLS (skill-selection) rather than exclude_tools per-tool (D-09 / RESEARCH Q3)"
  - "bench-no-semantic + bench-no-structured-edit keep full skill set and use exclude_tools to drop specific tools"
  - "bench-no-semantic sets NO kernel flag this phase (tool-filter-only, D-11/D-12); kernel guard is Phase 81"
  - "Used real tool name format_code (not RESEARCH's stale format_file) for the no-lsp exclude assertion"
  - "Validate() also rejects unknown transition SOURCE keys, not just default_mode + targets (defense-in-depth)"

patterns-established:
  - "Bench ablation arm = profile YAML (tool surface + disable flags) as the single reviewable artifact (D-01)"
  - "Golden surface tests blank-import skill packages to populate the global registry under test"

requirements-completed: [ABLATE-02]

# Metrics
duration: 4min
completed: 2026-06-16
---

# Phase 76 Plan 02: Bench Ablation Profiles + Fail-Closed Loader Summary

**4 bench ablation profile YAMLs (full/no-lsp/no-semantic/no-structured-edit) with first-class kernel disable-flag fields, golden per-arm tool-surface tests, and a fail-closed profile loader that rejects unknown mode names.**

## Performance

- **Duration:** ~4 min
- **Started:** 2026-06-16T16:38:00Z
- **Completed:** 2026-06-16T16:42:00Z
- **Tasks:** 2 (both TDD: RED → GREEN)
- **Files modified:** 8 (5 created, 3 modified)

## Accomplishments
- Added `Profile.DisableLSPSubsystem` + `Profile.DisableStructuredEditSubsystem` yaml-tagged bool fields (D-01 hybrid coupling; default-off opt-in disables, D-02), read by the daemon composition root in Plan 76-04.
- Shipped the 4 bench profile YAMLs under `internal/profile/profiles/` with the exact ablation semantics:
  - **bench-full** — control arm; full surface; neither flag set.
  - **bench-no-lsp** — omits symbol-retrieval + diagnostics skills; `disable_lsp_subsystem: true`.
  - **bench-no-semantic** — excludes the 10 semantic-store tools via `exclude_tools`; keeps get_repo_map/get_context; NO kernel flag (D-11/D-12).
  - **bench-no-structured-edit** — excludes replace_symbol_body/fuzzy_edit/insert_before_symbol/insert_after_symbol; keeps replace_in_file; `disable_structured_edit_subsystem: true`.
- Golden tool-surface tests (`TestBenchProfiles`) pin each arm's resolved surface + disable flags (mitigates T-76-04 leakage).
- `ProfileStore.Validate()` fail-closes `LoadEmbedded()` on any profile referencing an unknown mode (default_mode, transition source, or transition target), naming the offending profile + mode (mitigates T-76-03 Tampering).

## Task Commits

1. **Task 1 RED: golden tool-surface tests** - `d4ddc810` (test)
2. **Task 1 GREEN: 4 bench profiles + Profile disable-flag fields** - `5b1da2f6` (feat)
3. **Task 2 RED: loader unknown-mode rejection + 10-profile count** - `52397fa3` (test)
4. **Task 2 GREEN: ProfileStore.Validate() + SetMode()** - `b846dbba` (feat)

**Plan metadata:** see final docs commit.

## Files Created/Modified
- `internal/profile/profiles/bench-full.yaml` - control ablation arm (full surface, no flags)
- `internal/profile/profiles/bench-no-lsp.yaml` - LSP-disabled arm (skills omit symbol-retrieval+diagnostics; disable_lsp_subsystem)
- `internal/profile/profiles/bench-no-semantic.yaml` - semantic-tools-excluded arm (10 tools via exclude_tools; no kernel flag)
- `internal/profile/profiles/bench-no-structured-edit.yaml` - structured-edit-disabled arm (4 tools excluded; disable_structured_edit_subsystem)
- `internal/profile/bench_profiles_test.go` - TestBenchProfiles golden surface tests (blank-imports skills to populate registry)
- `internal/profile/profile.go` - added 2 disable-flag fields + SetMode() helper
- `internal/profile/loader.go` - added Validate() + LoadEmbedded() validation call
- `internal/profile/loader_test.go` - reject/accept tests; count 6→10; sorted-names list updated

## Decisions Made
- **Skill-selection vs exclude_tools per arm:** bench-no-lsp drops whole skills (symbol-retrieval, diagnostics) per D-09/RESEARCH Q3; bench-no-semantic and bench-no-structured-edit keep the full skill set and use `exclude_tools` to drop individual tools (semantic 10 tools; the 4 structured-edit tools). This matches each arm's natural granularity.
- **Real tool name `format_code`:** RESEARCH referenced `format_file`, but the diagnostics skill registers `format_code` (diag/skill_adapter.go). Used the real name in the no-lsp exclusion assertion.
- **Validate() rejects transition SOURCE keys too** (not only default_mode + targets) — closes the full unknown-mode surface.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Updated a SECOND hardcoded profile-count assertion the plan did not flag**
- **Found during:** Task 2 (loader count update)
- **Issue:** The plan/landmine called out only `TestLoadEmbedded_ReturnsAllProfilesAndModes` (6→10). `TestProfileStore_ProfileNames_Sorted` (loader_test.go:93) also hardcodes the exact 6-profile sorted slice and would have gone red once the 4 bench profiles loaded.
- **Fix:** Updated the sorted-names assertion to the full 10-profile list (bench-full/no-lsp/no-semantic/no-structured-edit sort between "baseline" and "ci-bot").
- **Files modified:** internal/profile/loader_test.go
- **Verification:** `go test ./internal/profile/ -count=1` passes (full suite).
- **Committed in:** `52397fa3` (Task 2 RED commit)

---

**Total deviations:** 1 auto-fixed (1 bug).
**Impact on plan:** Necessary to keep the existing suite green; no scope creep.

## Issues Encountered
- `skill.ResolveTools` returns an empty surface unless skill packages are registered (global registry populated via `init()`). The golden tests would have been vacuous in the `profile` test package, which imports no skills. Resolved by blank-importing the same skill packages the daemon imports (`internal/daemon/imports.go`) in `bench_profiles_test.go`, so the resolved surface is the real one. No import cycle (no skill package imports `internal/profile`).

## Known Stubs
None.

## Threat Flags
None — no new network/auth/file surface introduced; this plan is profile YAML + loader validation only.

## TDD Gate Compliance
Both tasks followed RED → GREEN with separate commits:
- Task 1: `d4ddc810` (test, RED) → `5b1da2f6` (feat, GREEN).
- Task 2: `52397fa3` (test, RED) → `b846dbba` (feat, GREEN).
No REFACTOR commits were needed (code was clean at GREEN).

## Self-Check: PASSED
- All 5 created files exist on disk; all 3 modified files updated.
- All 4 task commits present in git log.
- `go vet ./internal/profile/...` clean; `go test ./internal/profile/ -count=1` passes; `go build ./cmd/helix` succeeds.

## Next Phase Readiness
- `--profile=bench-full` is loadable for Phase 77's daemon subprocess launch.
- Phase 76-04 (daemon composition root) can read `Profile.DisableLSPSubsystem` / `Profile.DisableStructuredEditSubsystem` to configure the kernel.
- Phase 81 will add the kernel `disable_semantic_subsystem` guard that pairs with the bench-no-semantic profile (deferred per D-11/D-12).

---
*Phase: 76-ablation-profiles-kernel-subsystem-disable-flags*
*Completed: 2026-06-16*

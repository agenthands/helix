---
phase: 76-ablation-profiles-kernel-subsystem-disable-flags
plan: 03
subsystem: testing
tags: [go-analysis, vettool, import-boundary, analysistest, ablation, lint, make-vet]

# Dependency graph
requires:
  - phase: 57-semantic-store-foundation-pipeline-dag-library
    provides: internal/lint/noduckdb house import-boundary analyzer pattern (copy-shape source)
  - phase: 60-live-enrichment
    provides: internal/lint/nokernel2semantic slash-boundary discipline (exact-OR-prefix+"/")
provides:
  - "vet-ablation-leakage static analyzer pinning the architectural edge: bench/runners must not import internal/kernel/lspool or internal/semantic/store (ABLATE-08)"
  - "cmd/vet-ablation-leakage singlechecker entrypoint"
  - "analysistest fixtures proving a demonstrable green->red flip (badrunner fires, goodrunner silent, siblingrunner slash-boundary control)"
  - "make vet wiring running all 5 analyzers green on the real tree"
affects: [80-bench-runners, ablation-profiles, kernel-subsystem-disable-flags]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "House import-boundary lint: go/analysis Analyzer + singlechecker cmd + analysistest testdata + make vet -vettool chaining"
    - "Multi-prefix forbidden-import loop (two disabled subsystems) with exact-OR-slash-suffix matching"

key-files:
  created:
    - internal/lint/ablationleakage/analyzer.go
    - internal/lint/ablationleakage/analyzer_test.go
    - cmd/vet-ablation-leakage/main.go
    - internal/lint/ablationleakage/testdata/src/github.com/agenthands/helix/bench/runners/badrunner/imports.go
    - internal/lint/ablationleakage/testdata/src/github.com/agenthands/helix/bench/runners/goodrunner/imports.go
    - internal/lint/ablationleakage/testdata/src/github.com/agenthands/helix/bench/runners/siblingrunner/imports.go
  modified:
    - Makefile

key-decisions:
  - "Pinned the architectural edge bench/runners -> {lspool, store}, NOT the flag-conditional kernel->fuzzy edge (RESEARCH Pitfall 5 / D-08)"
  - "Used a forbiddenImportPrefixes slice (two subsystems) looped against each import, preserving noduckdb's exact-OR-slash-suffix slash boundary"
  - "Placed testdata fixtures under the full bench/runners/* path so the checkedPkgPrefix gate fires; forbidden-import targets and the storehouse lookalike exist as testdata-GOPATH stubs"

patterns-established:
  - "Ablation import-boundary guard: compile-time static complement to the kernel Unsupported runtime guard"
  - "siblingrunner slash-boundary regression fixture (storehouse lookalike) mirroring noduckdb's duckdb-go-sibling guard"

requirements-completed: [ABLATE-08]

# Metrics
duration: ~12min
completed: 2026-06-16
---

# Phase 76 Plan 03: vet-ablation-leakage Import-Boundary Analyzer Summary

**Static go/analysis vettool pinning the architectural edge `bench/runners` must not import `internal/kernel/lspool` or `internal/semantic/store`, wired into `make vet`, with analysistest fixtures proving a demonstrable green->red flip (ABLATE-08).**

## Performance

- **Duration:** ~12 min
- **Completed:** 2026-06-16
- **Tasks:** 2
- **Files modified:** 10 (7 created in Task 1, Makefile in Task 2; 3 testdata stub packages + 3 runner fixtures)

## Accomplishments
- `internal/lint/ablationleakage/analyzer.go`: a near-clone of `noduckdb` that gates on `checkedPkgPrefix = github.com/agenthands/helix/bench/runners` and reports any import of the two disabled-subsystem prefixes `internal/kernel/lspool` / `internal/semantic/store` (exact-OR-`prefix+"/"` slash boundary).
- `cmd/vet-ablation-leakage/main.go`: `singlechecker.Main(ablationleakage.Analyzer)`.
- analysistest harness + testdata fixtures: `badrunner` (forbidden import + `// want`, fires), `goodrunner` (os/exec, silent), `siblingrunner` (storehouse lookalike, silent — slash-boundary control).
- `make vet` extended to build and run a 5th analyzer; green on the real tree (the violation lives only under `testdata/`, which the go tool ignores — Pitfall 3).

## Task Commits

1. **Task 1 (RED): analysistest + testdata fixtures** - `9a7dea02` (test)
2. **Task 1 (GREEN): analyzer + singlechecker** - `80bfe584` (feat)
3. **Task 2: make vet wiring** - `9c70f024` (build)

_TDD: RED test commit precedes the GREEN feat commit. The green->red flip was demonstrated live before commit by removing the `// want` directive (analysistest then reported the diagnostic as unexpected -> FAIL), then restored._

## Files Created/Modified
- `internal/lint/ablationleakage/analyzer.go` - import-boundary Analyzer (ABLATE-08 architectural edge)
- `internal/lint/ablationleakage/analyzer_test.go` - 3 analysistest cases (reject / allow / slash-boundary)
- `cmd/vet-ablation-leakage/main.go` - singlechecker entrypoint
- `internal/lint/ablationleakage/testdata/.../bench/runners/badrunner/imports.go` - deliberate forbidden import + `// want`
- `internal/lint/ablationleakage/testdata/.../bench/runners/goodrunner/imports.go` - permitted os/exec import (silent control)
- `internal/lint/ablationleakage/testdata/.../bench/runners/siblingrunner/imports.go` - storehouse lookalike (slash-boundary control)
- `internal/lint/ablationleakage/testdata/.../internal/kernel/lspool/lspool.go` - stub forbidden-import target
- `internal/lint/ablationleakage/testdata/.../internal/semantic/store/store.go` - stub forbidden-import target
- `internal/lint/ablationleakage/testdata/.../internal/semantic/storehouse/storehouse.go` - stub lookalike target
- `Makefile` - `VETTOOL_ABLATION_LEAKAGE` var + vet prereq + vettool line + install rule

## Decisions Made
- **Forbidden edge (load-bearing):** pinned the genuinely-architectural `bench/runners -> {internal/kernel/lspool, internal/semantic/store}` edge. Did NOT pin `kernel -> internal/fuzzy`, which is flag-conditional (structured-edit tools legitimately import fuzzy in the enabled build) — Pitfall 5 / D-08 honest scope. Confirmed: the analyzer has no `internal/fuzzy` constant (`grep` matches only the explanatory doc comment, not a forbidden prefix).
- **Two forbidden subsystems:** used `var forbiddenImportPrefixes []string` looped per-import rather than a single const, while preserving the exact-OR-`prefix+"/"` slash-boundary check from `noduckdb`/`nokernel2semantic`.
- **Testdata layout:** the `checkedPkgPrefix` gate requires the analyzed package's own path to start with `bench/runners`, so fixtures live under `testdata/src/github.com/agenthands/helix/bench/runners/<pkg>/`; forbidden-import targets and the `storehouse` lookalike exist as testdata-GOPATH stub packages so type-checking resolves.

## Deviations from Plan
None - plan executed exactly as written. The plan's `files_modified` listed short fixture paths (`testdata/src/badrunner/...`); the actual paths use the full `bench/runners/...` namespace required by the `checkedPkgPrefix` gate (and an added `siblingrunner` + stub packages for the Test-3 slash-boundary case and import resolution, all explicitly described in the plan's Task 1 `<action>`/`<behavior>`). This is the layout the plan's behavior section mandates, not a scope change.

## TDD Gate Compliance
- RED gate: `9a7dea02` `test(76-03): ...` (analysistest + `// want` fixtures).
- GREEN gate: `80bfe584` `feat(76-03): ...` (analyzer implementation), after RED.
- No REFACTOR commit needed (verbatim copy-shape clone).
- Green->red flip demonstrated non-manually via the analysistest fixture (removing `// want` flips the test red), per success criterion #4.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- The analyzer is active in `make vet` now; the real `bench/runners/*` consumer packages land in Phase 80 and will be enforced automatically the moment they exist.
- Closes the ABLATE-08 row in 76-VALIDATION.md (Wave 0 gap closure).

## Self-Check: PASSED

All created files present on disk; all 3 task commits present in git history.

---
*Phase: 76-ablation-profiles-kernel-subsystem-disable-flags*
*Completed: 2026-06-16*

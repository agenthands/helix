---
phase: 75-schema-fairness-contract-tree-skeleton
plan: 01
subsystem: infra
tags: [bench, git-mv, semantic, bleve, duckdb-fts, relocation, phase-64-microbench]

# Dependency graph
requires:
  - phase: 64-semantic-store-foundation
    provides: "Phase 64-01 bleve-vs-DuckDB-FTS5 gate microbench (semantic_bench_test.go, _blevprobe, synthetic_50k_go fixture, REPORT)"
provides:
  - "Phase 64 semantic microbench relocated to internal/semantic/bench/ (package bench, benchfts tag intact, rename history preserved)"
  - "Cleared top-level bench/ namespace for the v1.12 bench stack (Wave 0 hard ordering prerequisite for all later Phase 75 plans)"
  - "Cross-refs (store probe doc-comment + fixture README) repointed to the new path; git grep is clean"
affects: [75-02, 75-03, 75-04, 75-05, "any plan writing under bench/", "Phase 77 BENCH-05 (make bench name-collision, deferred)"]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "git mv per-path relocation (R-detected renames) to preserve git log --follow continuity"
    - "Atomic cross-ref update: external + in-file doc-comment path edits land in the same commit as the move so the repo never has a dangling intermediate state"

key-files:
  created:
    - internal/semantic/bench/semantic_bench_test.go
    - internal/semantic/bench/semantic_bench_fts_test.go
    - internal/semantic/bench/semantic_bench_REPORT.md
    - internal/semantic/bench/_blevprobe/probe.go
    - internal/semantic/bench/fixtures/synthetic_50k_go/gen.go
    - internal/semantic/bench/fixtures/synthetic_50k_go/README.md
    - internal/semantic/bench/fixtures/synthetic_50k_go/.gitignore
  modified:
    - internal/semantic/store/bench_fts_probe.go

key-decisions:
  - "Used per-path git mv so each artifact is recorded as a rename (R090/R100); doc-comment edits account for the sub-100% similarity"
  - "Kept package bench and the //go:build benchfts / //go:build ignore tags unchanged — the only intra-repo import is the absolute module path internal/semantic/store, unaffected by the directory move (no go.mod/replace edit)"
  - "Did NOT touch the make bench Makefile target (runs ./test/bench/..., unrelated to the moved dir) — the future Phase 77 BENCH-05 name-collision is deferred, not fixed here"

patterns-established:
  - "Wave 0 relocation pattern: move + repoint all cross-refs (external and in-file prose) in one atomic commit, verified by an empty git grep for the old paths"

requirements-completed: [INFRA-01, INFRA-03]

# Metrics
duration: 10min
completed: 2026-06-14
---

# Phase 75 Plan 01: Wave 0 Microbench Relocation Summary

**Phase 64 semantic gate microbench relocated from top-level `bench/` to `internal/semantic/bench/` via per-path `git mv` (renames preserved), with the store-probe doc-comment and fixture README cross-refs repointed in the same commit — clearing `bench/` for the v1.12 bench stack.**

## Performance

- **Duration:** 10 min
- **Started:** 2026-06-14T13:54:22Z
- **Completed:** 2026-06-14T14:04:30Z
- **Tasks:** 1
- **Files modified:** 8 (7 relocated + 1 external cross-ref)

## Accomplishments
- Moved all Phase 64 microbench artifacts to `internal/semantic/bench/` (`semantic_bench_test.go`, `semantic_bench_fts_test.go`, `semantic_bench_REPORT.md`, the `_blevprobe/` probe, and the `fixtures/synthetic_50k_go/` deterministic corpus) with git rename detection (`R090`) so `git log --follow` traverses pre-relocation history.
- Top-level `bench/` now contains none of the Phase 64 artifacts — the Wave 0 ordering prerequisite for every later Phase 75 plan is satisfied.
- Repointed both external cross-references in the same commit: `internal/semantic/store/bench_fts_probe.go` doc-comment and the fixture `README.md` (intended-consumers line + build-tag invocation line), plus the self-referential prose paths inside the moved test file and REPORT.
- `git grep` for the old `bench/semantic_bench` / `bench/_blevprobe` / `bench/fixtures` paths (excluding `.planning`/`legacy`) returns zero hits.
- `go vet ./...` is green; `go test -tags=benchfts ./internal/semantic/bench/...` passes; the whole Go suite passes except one pre-existing, unrelated `test/bench` tool-inventory failure (see Deviations / Deferred).

## Task Commits

Each task was committed atomically:

1. **Task 1: git mv Phase 64 microbench to internal/semantic/bench/ and update cross-refs (atomic)** - `661c73ed` (refactor)

**Plan metadata:** `__HASH_META__` (docs: complete plan)

## Files Created/Modified
- `internal/semantic/bench/semantic_bench_test.go` - Relocated bleve-vs-DuckDB-FTS5 gate microbench (package bench); in-file fixture/REPORT prose paths rewritten to `internal/semantic/bench/...`
- `internal/semantic/bench/semantic_bench_fts_test.go` - Relocated benchfts-gated DuckDB FTS5 baseline benchmark (no path refs; pure rename)
- `internal/semantic/bench/semantic_bench_REPORT.md` - Relocated Phase 64 D-08 gate report; `_blevprobe`/fixture/invocation paths rewritten
- `internal/semantic/bench/_blevprobe/probe.go` - Relocated bleve binary-size probe (pure rename)
- `internal/semantic/bench/fixtures/synthetic_50k_go/{gen.go,README.md,.gitignore}` - Relocated deterministic 50k-symbol Go fixture; README consumer/build-tag paths rewritten
- `internal/semantic/store/bench_fts_probe.go` - Cross-ref doc-comment updated `bench/semantic_bench_test.go` → `internal/semantic/bench/semantic_bench_test.go` (file otherwise unchanged; not moved, honors STORE-06 boundary)

## Decisions Made
- Per-path `git mv` (not delete+recreate) so renames are detected and `git log --follow` continuity holds.
- `package bench` and build tags (`//go:build benchfts`, `//go:build ignore`) kept unchanged; the sole intra-repo import is the absolute module path `github.com/agenthands/helix/internal/semantic/store`, unaffected by the move — no `go.mod`/`replace` change needed (single module).
- Left the `make bench` Makefile target alone (it runs `./test/bench/...`, not the moved dir); the Phase 77 BENCH-05 name-collision is documented downstream, not fixed in this Wave 0 relocation.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Repaired incompletely-extracted Go module cache**
- **Found during:** Task 1 (verification step — `go vet ./...` / `go test ./...`)
- **Issue:** The sandbox module cache had two partially-extracted modules — `go.opentelemetry.io/auto/sdk@v1.2.1` (only `internal/telemetry/` present, missing `tracer.go`/`span.go`/etc.) and `github.com/google/jsonschema-go@v0.4.2` (only `meta-schemas/` + `testdata/` present, missing the `jsonschema` package sources). This made `go vet`/`go test` fail repo-wide with "no required module provides package …", including on packages untouched by this plan (e.g. `internal/profile`).
- **Fix:** Re-ran `go mod download` for both modules to re-extract the full package sources, then `go clean -cache` to drop poisoned build-cache entries. `go.mod`/`go.sum` were left byte-unchanged (any transient `-mod=mod` side effects were reverted).
- **Files modified:** None in the repo (environment-only repair).
- **Verification:** `go vet ./...` → exit 0; `go test -tags=benchfts ./internal/semantic/bench/...` → ok.
- **Committed in:** N/A (no repo files changed — environment cache repair only)

---

**Total deviations:** 1 (1 blocking, environment-only — no repo files changed)
**Impact on plan:** None. The relocation itself executed exactly as written; the only deviation was repairing a corrupted module cache so the verification suite could run. No scope creep.

## Issues Encountered

- **Pre-existing `test/bench` failure (out of scope, NOT fixed):** `go test ./test/bench/` fails on `TestBenchToolsManifestMatchesRegistry` (live registry reports **53 tools; manifest expects 47**) and `TestToolDescriptionsGoldenFile` (stale golden). Proven pre-existing by stashing all of this plan's changes and re-running against the pristine HEAD (`582a6a57`) — both fail identically. This plan registers zero tools and edits zero tool descriptions, so it cannot be the cause. Logged to `.planning/phases/75-schema-fairness-contract-tree-skeleton/deferred-items.md` for the tool-registration owner; the fix is `go test ./test/bench/... -run TestToolDescriptionsGoldenFile -update` plus bumping the expected count in `test/bench/tools_manifest_test.go`.
- **`git stash` mishap during pre-existing-failure verification:** stashing/popping to isolate the `test/bench` failure produced a transient `DU` conflict on the moved files; resolved by re-staging the working-tree versions, after which git correctly re-recorded all moves as renames (`R bench/... -> internal/semantic/bench/...`). Final tree is intact and verified.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Top-level `bench/` is cleared; later Phase 75 plans (75-02 … 75-05) may now write the v1.12 foundation skeleton under `bench/` without colliding with the Phase 64 microbench.
- The microbench builds and tests under its new `internal/semantic/bench/` home with rename history preserved.
- No blockers introduced by this plan. (Unrelated, pre-existing: the `test/bench` tool-inventory golden needs a refresh — tracked in deferred-items.md.)

## Self-Check: PASSED

- All 7 relocated files + the cross-ref file verified present on disk.
- Task 1 commit `661c73ed` exists in git history.
- `git log --follow internal/semantic/bench/semantic_bench_test.go` reaches the pre-relocation commit `674cc64d` (rename history preserved).
- `git grep` for old `bench/semantic_bench|bench/_blevprobe|bench/fixtures` top-level paths (excluding `.planning`/`legacy`) returns zero matches.
- `go vet ./...` exit 0; `go test -tags=benchfts ./internal/semantic/bench/...` ok; full suite green except the pre-existing, unrelated `test/bench` tool-inventory failure (deferred).

---
*Phase: 75-schema-fairness-contract-tree-skeleton*
*Completed: 2026-06-14*

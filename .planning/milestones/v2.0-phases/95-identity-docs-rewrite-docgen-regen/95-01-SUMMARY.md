---
phase: 95-identity-docs-rewrite-docgen-regen
plan: 01
subsystem: tooling
tags: [docgen, ci, makefile, drift-gate, readme, cli-verbs, blank-imports]

# Dependency graph
requires:
  - phase: 94 (CLI-first surface)
    provides: the 50 frozen `helix` CLI verbs (internal/cli/verbs_gen.go) and the verb == toolName.replace("_","-") mapping
provides:
  - Verb-keyed README tool table (regenerated via cmd/docgen, lists `helix <verb>` names)
  - `make verify-docs` HARD-FAIL drift gate (mirrors verify-cligen)
  - CI `docgen drift gate (DOCS-02)` step in .github/workflows/go-test.yml (closes the v1.12 docgen-drift hole)
  - Verb-form docgen tests
  - Reciprocal blank-import parity cross-ref comments (docgen <-> daemon)
affects: [95-02 (CLI-first identity rewrite + routing matrix), future tool registrations]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Drift-gate mirrored from cligen: $(GO) run ./cmd/<tool> --check as both a make target and a CI step"
    - "Generated-doc re-key at the emitter (cmd/docgen), never hand-edit between <!-- BEGIN/END TOOLS -->"
    - "Blank-import parity via gate + reciprocal cross-ref comments, NOT literal import-list equality"

key-files:
  created: []
  modified:
    - cmd/docgen/main.go
    - cmd/docgen/main_test.go
    - Makefile
    - .github/workflows/go-test.yml
    - internal/daemon/imports.go
    - README.md

key-decisions:
  - "Derive the verb inline via strings.ReplaceAll(tool.Name, \"_\", \"-\") in docgen rather than importing internal/cli (zero new package dependency; the seam alternative adds an import for no gain here)."
  - "Kept the table header as `| Tool | Category | Description |` (not `Verb`) so TestGenerateToolTable's row-count filter that skips lines containing `Tool` keeps working."
  - "Left the 51st README row (analyze_blast_radius dual-categorized under symbol-retrieval AND symbols) as honest registry output; did not dedupe."
  - "Reconciled docgen <-> daemon blank imports via the --check gate + reciprocal comments; did NOT force literal list equality (health/help non-blank in daemon by design; guardrails table-neutral; D-02 forbids semantic/extract)."

patterns-established:
  - "Pattern 1: docgen drift gate — go run ./cmd/docgen --check wired as make verify-docs + CI step, HARD-FAIL on stale README table"
  - "Pattern 2: parity-via-gate cross-ref comments documenting why import lists legitimately diverge"

requirements-completed: [DOCS-02]

# Metrics
duration: 3min
completed: 2026-06-22
status: complete
---

# Phase 95 Plan 01: docgen Verb Re-key + CI Drift Gate Summary

**Re-keyed the auto-generated README tool table to `helix <verb>` names and closed the v1.12 docgen-drift hole by wiring `go run ./cmd/docgen --check` into both `make verify-docs` and CI.**

## Performance

- **Duration:** ~3 min
- **Started:** 2026-06-21T23:09:33Z
- **Completed:** 2026-06-22T00:00:00Z (approx)
- **Tasks:** 3
- **Files modified:** 6

## Accomplishments
- docgen `generateToolTable` now emits `| \`helix <verb>\` | <category> | <desc> |` rows (verb = MCP tool name with `_`→`-`), proven mechanical for all 50 frozen verbs.
- README tool table regenerated to verb forms (51 rows for 50 unique tools — `analyze_blast_radius` dual-categorized, left as honest output).
- New `make verify-docs` HARD-FAIL drift gate mirroring `verify-cligen`.
- New CI step `docgen drift gate (DOCS-02)` in `.github/workflows/go-test.yml` — the load-bearing deliverable; the gate did not exist in CI before (only a comment referenced docgen).
- docgen tests flipped to verb-form assertions (RED→GREEN).
- Reciprocal blank-import parity cross-ref comments added to `cmd/docgen/main.go` and `internal/daemon/imports.go`, documenting the parity-via-gate rule and the D-02 semantic/extract exclusion.

## Task Commits

Each task was committed atomically:

1. **Task 1: Re-key docgen tool table to `helix <verb>` (TDD RED→GREEN)** - `def471a8` (test, RED), `99251a1d` (feat, GREEN)
2. **Task 2: Add `make verify-docs` + CI `docgen --check` drift gate** - `d3762da0` (feat)
3. **Task 3: Regenerate README table + blank-import cross-ref comments** - `55ab96cb` (docs)

_Note: Task 1 is TDD with two commits (test → feat)._

## Files Created/Modified
- `cmd/docgen/main.go` - Row emitter derives kebab verb and emits `helix <verb>`; added blank-import parity cross-ref comment.
- `cmd/docgen/main_test.go` - TestToolTableContainsKnownTools asserts verb forms (`helix go-to-definition`, etc.).
- `Makefile` - Added `verify-docs` target + `.PHONY` entry (mirrors `verify-cligen`).
- `.github/workflows/go-test.yml` - Added `docgen drift gate (DOCS-02)` step running `go run ./cmd/docgen --check`.
- `internal/daemon/imports.go` - Added reciprocal blank-import parity cross-ref comment.
- `README.md` - Tool table regenerated to verb-keyed rows (between `<!-- BEGIN/END TOOLS -->`).

## Decisions Made
- Inline `strings.ReplaceAll` verb derivation in docgen (zero new deps) over importing `internal/cli`.
- Header stays `Tool` (not `Verb`) to preserve the row-count test filter.
- 51st dual-categorized row left as-is per the honest-counts invariant.
- Blank-import reconciliation via the gate + cross-ref comments, not literal equality.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None. All gates green: `go run ./cmd/docgen --check` exits 0; `make verify-docs` exits 0; `go test ./cmd/docgen/ ./internal/cli/ -count=1` passes; `go build ./...` and `go vet ./...` clean; `git diff go.mod` empty.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- The verb-keyed table and the live drift gate are in place; 95-02 (CLI-first identity rewrite + routing matrix) can proceed against an enforced, verb-named README table.
- No blockers.

## Self-Check: PASSED
- Files: cmd/docgen/main.go, cmd/docgen/main_test.go, Makefile, .github/workflows/go-test.yml, internal/daemon/imports.go, README.md — all FOUND.
- Commits: def471a8, 99251a1d, d3762da0, 55ab96cb — all FOUND.

---
*Phase: 95-identity-docs-rewrite-docgen-regen*
*Completed: 2026-06-22*

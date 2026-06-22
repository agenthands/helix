---
phase: 92-terse-output-renderer-re-targeted-contract-oracle
plan: 01
subsystem: cli
tags: [terse-renderer, locus-parse, exit-codes, render-class, serr, tdd]

# Dependency graph
requires:
  - phase: 91
    provides: typed-error taxonomy (internal/errors.Kind) + permission_denied CLI exit round-trip precedent
provides:
  - "renderClassFor: tool-name -> render class (locus-list / tree / opaque) covering all 50 generated verbs, classOpaque safe default"
  - "parseLocusLine: both daemon locus grammars (file://…:L:C[ — payload] and fileops relpath:L: text) -> (relpath, line, col, payload); abs/rel + ToSlash; no coord re-conversion"
  - "sortDedupLoci: deterministic (relpath, line, col) sort + exact-dup removal"
  - "exitCodeForKind / parseKind / stderrPrefixForKind: 9 serr.Kind -> frozen distinct non-zero exit codes + stable stderr prefixes"
affects: [92-02-renderer-integration, 92-03-contract-oracle, 93-skill-md]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Parse-don't-convert at the render boundary: CLI parser consumes the daemon's already-1-based loci (tools.go:189-190) and never re-shifts coordinates"
    - "Defensive passthrough sentinel (ok=false) for malformed daemon text — never panic (T-92-01)"
    - "Error-kind-spoofing guard: only the 9 enum serr.Kind values match as <kind>: tokens, never arbitrary word: prefixes (T-92-02)"
    - "Frozen exit-code contract published as a unit before integration (codes cited by Phase 93 SKILL.md)"

key-files:
  created:
    - internal/cli/locus.go
    - internal/cli/locus_test.go
    - internal/cli/exitcode.go
    - internal/cli/exitcode_test.go
  modified: []

key-decisions:
  - "parseSearchForm distinguishes grammar (a) <path>:L:C from grammar (b) <path>:L: <text> by checking whether the post-colon remainder is all-digits (a column) vs free text"
  - "Backslash normalization is explicit (strings.ReplaceAll) on top of filepath.ToSlash, since ToSlash is a no-op for backslashes on POSIX hosts — guarantees golden stability cross-OS"
  - "parseKind takes the rightmost (innermost) recognized <kind>: token so the real kind wins over runVerb's 'calling <tool>:' wrapper, with a left word-boundary guard"

patterns-established:
  - "Pure leaf functions (no cobra/network) built TDD RED->GREEN as testable units before 92-02 integration"
  - "renderClassByTool keyed by toolName; coverage test (Task 1) iterates verbSpecs so a future verb cannot ship unclassified"

requirements-completed: [OUT-01, OUT-02, OUT-05]

# Metrics
duration: ~20min
completed: 2026-06-21
status: complete
---

# Phase 92 Plan 01: Terse Renderer Foundation Units Summary

**Three pure, dependency-free foundation units for the Phase 92 terse renderer: a 50-verb render-class map, a locus parse + path-normalization + sort/dedup core, and the 9-kind serr.Kind → frozen exit-code + stderr-prefix mapper — all built TDD with no cobra/network dependency.**

## Performance

- **Duration:** ~20 min
- **Started:** 2026-06-21 (resumed mid-plan; Task 1 pre-completed)
- **Completed:** 2026-06-21
- **Tasks:** 3 (Task 1 pre-completed before resume; Tasks 2 & 3 executed this session)
- **Files modified:** 6 (2 pre-existing from Task 1; 4 created this session)

## Accomplishments
- `internal/cli/locus.go` — `parseLocusLine` handles both daemon grammars (file://…:L:C with optional " — payload", and fileops `relpath:L: text` with col defaulted to 1), strips `file://`, relativizes vs workspace root or keeps abs, ToSlash-normalizes; returns a passthrough sentinel (ok=false) for `(no results)`, markdown fences, and truncated lines without panicking. `sortDedupLoci` is order-independent and collapses exact duplicates.
- `internal/cli/exitcode.go` — `parseKind` recognizes the 9 documented kinds across the bare `kind: message` form and the runVerb-wrapped `calling <tool>: <kind>: <msg>` form (innermost wins), rejecting spoofed `word:` prefixes. `exitCodeForKind` returns the frozen distinct non-zero codes (invalid_args=2 … guardrail_violation=9, internal=70, generic fallback=1). `stderrPrefixForKind` returns the Kind value verbatim.
- Render-class map (Task 1, pre-completed) verified still green and untouched.

## Task Commits

Each task committed atomically (TDD RED → GREEN):

1. **Task 1: Render-class map (50 verbs)** — `275f7cf3` (test RED), `635f4682` (feat GREEN) — pre-completed before resume; verified green, not re-committed
2. **Task 2: Locus parse + sort/dedup** — `18b2ad91` (test RED), `db9d08ac` (feat GREEN)
3. **Task 3: serr.Kind exit-code + prefix mapper** — `ad30afa2` (test RED), `b94623c3` (feat GREEN)

## Files Created/Modified
- `internal/cli/locus.go` - `locus` struct + `parseLocusLine` (both grammars, abs/rel, ToSlash, no coord re-conversion) + `sortDedupLoci` (deterministic sort + dedup)
- `internal/cli/locus_test.go` - RED suite: both grammars, payload, passthrough sentinels, abs, ToSlash, order-independence (`-count=5`), identical-collapse
- `internal/cli/exitcode.go` - `parseKind` (9-kind scan, innermost-wins, spoofing guard) + `exitCodeForKind` (frozen codes) + `stderrPrefixForKind`
- `internal/cli/exitcode_test.go` - RED suite: both wire forms, spoofing rejection, frozen-code contract, distinct-non-zero, unknown fallback, prefix verbatim
- `internal/cli/render_policy.go` / `render_policy_test.go` - (Task 1, pre-completed) unchanged

## Decisions Made
- **Grammar disambiguation:** `parseSearchForm` checks whether the text after `<L>:` is all-digits (→ a column, so grammar (a)) vs free text (→ grammar (b)); this cleanly separates `main.go:5:6` from `main.go:5: func main() {`.
- **Explicit backslash normalization:** `toSlash` wraps `filepath.ToSlash` with an unconditional `\`→`/` replacement, because `filepath.ToSlash` is a no-op for backslashes on POSIX — required to make the synthetic backslash golden-stability test pass on the Linux CI host.
- **Innermost kind wins:** `parseKind` uses `strings.LastIndex` with a left word-boundary check so the real `<kind>:` token survives runVerb's `calling <tool>:` prefix.

## Deviations from Plan

None - plan executed exactly as written. Two GREEN-phase iterations were normal TDD debugging (not deviations): the first `parseLocusLine` implementation mis-classified `main.go:5:6` as the search grammar and `filepath.ToSlash` did not convert backslashes on POSIX; both were fixed before the Task 2 GREEN commit, with the RED tests driving the fixes.

## Issues Encountered
- Initial Task 2 GREEN failed two tests (grammar (a)/(b) ambiguity and POSIX ToSlash no-op). Resolved within the same GREEN phase via the all-digits column check and explicit backslash replacement; final `-count=5` run is green and deterministic.

## User Setup Required
None - no external service configuration required. Phase 92 installs zero new packages (go.mod diff empty); zero-proto invariant untouched (api/proto diff empty).

## Next Phase Readiness
- All three foundation units are testable, deterministic, and free of cobra/network dependencies — 92-02 can wire `renderClassFor` + `parseLocusLine` + `sortDedupLoci` into `renderResult` and `exitCodeForKind` into the CLI exit path without re-deriving any algorithm.
- The exit-code numbering is frozen and ready to be cited by Phase 93 SKILL.md.

## Self-Check: PASSED

- FOUND: internal/cli/locus.go, internal/cli/locus_test.go, internal/cli/exitcode.go, internal/cli/exitcode_test.go
- FOUND commits: 18b2ad91, db9d08ac, ad30afa2, b94623c3
- Verification gate green: `go vet ./internal/cli/...` clean; `go test ./internal/cli/ -run 'RenderClass|RenderPolicy|Locus|SortDedup|ExitCode|ParseKind' -count=1` ok; `git diff go.mod` empty; `git diff api/proto/` empty; full `go test ./internal/cli/` green (no regressions to Task 1).

---
*Phase: 92-terse-output-renderer-re-targeted-contract-oracle*
*Completed: 2026-06-21*

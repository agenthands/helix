---
phase: 106-exploratory-dspy-offline-tuning-harness-spike
plan: 01
subsystem: testing
tags: [go-analysis, vet, analysistest, parity-corpus, dspy, import-boundary, anti-vacuity]

# Dependency graph
requires:
  - phase: 101-llm-behavioral-adoption-scorecard
    provides: "test/oracle/adopt classifier (FirstCommand + ClassifyChoice) — the single source of truth the parity corpus is pinned to"
  - phase: 76-ablation-leakage
    provides: "internal/lint/ablationleakage import-boundary analyzer + singlechecker + Makefile vet: wiring pattern (copied/inverted)"
provides:
  - "Shared golden parity corpus (tools/dspy-tune/golden/parity_cases.json) read by BOTH the Go parity test and Plan 02's Python pytest — single committed file, never duplicated"
  - "Go parity test TestPythonGoParityCorpus pinning the corpus to FirstCommand + ClassifyChoice per case (>=8 floor)"
  - "toolsquarantine go/analysis analyzer: no package outside tools/ may import github.com/agenthands/helix/tools/..."
  - "cmd/vet-tools-quarantine singlechecker + make vet wiring (installs + runs the analyzer on every vet)"
affects: [106-02-PLAN, dspy-offline-tuning, tools-quarantine, parity-contract]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Inverted import-boundary analyzer: opt a namespace OUT (self-import exempt) and check all others, vs ablationleakage opting a namespace IN"
    - "Corpus-as-assertion: a shared JSON corpus is the cross-language parity contract; a wrong expected field fails the Go test (non-vacuous)"

key-files:
  created:
    - tools/dspy-tune/golden/parity_cases.json
    - test/oracle/adopt/parity_test.go
    - internal/lint/toolsquarantine/analyzer.go
    - internal/lint/toolsquarantine/analyzer_test.go
    - internal/lint/toolsquarantine/testdata/src/github.com/agenthands/helix/internal/leakyruntime/imports.go
    - internal/lint/toolsquarantine/testdata/src/github.com/agenthands/helix/internal/goodruntime/imports.go
    - internal/lint/toolsquarantine/testdata/src/github.com/agenthands/helix/internal/toolsupport/imports.go
    - internal/lint/toolsquarantine/testdata/src/github.com/agenthands/helix/tools/dspytune/dspytune.go
    - cmd/vet-tools-quarantine/main.go
  modified:
    - Makefile

key-decisions:
  - "Stored first_command in the corpus as the RAW FirstCommand output (NOT lowercased): the Go test asserts FirstCommand(response) == first_command directly; lowercasing happens only inside ClassifyChoice."
  - "Analyzer is import-boundary-ONLY (no pip/exec.Command scan) so internal/langregistry/installer.go's legit pip/pipx LS shell-out is never flagged."
  - "Self-import exemption uses exact-OR-slash-boundary matching (not bare HasPrefix) so a tools/-rooted package is exempt but internal/toolsupport is still analyzed."

patterns-established:
  - "Anti-vacuity discriminators baked into the corpus: a substring-trap case (prose mentioning 'helix' then grep -> chose=false,fell_back=false) and an lsp-lookalike ('lsp restart' -> NOT an 'ls ' fallback) make the parity test non-vacuous."
  - "RED analysistest fixture (planted runtime->tools/ import + // want) is the load-bearing break-the-invariant proof: deleting the // want directive fails the test."

requirements-completed: [TUNE-01]

# Metrics
duration: 5min
completed: 2026-06-24
status: complete
---

# Phase 106 Plan 01: Go-side authoritative gates for the DSPy quarantine spike Summary

**Shared golden parity corpus pinned to the test/oracle/adopt Go classifier, plus an inverted `toolsquarantine` go/analysis import-boundary analyzer (with RED/GREEN/lookalike fixtures) wired into `make vet` to keep the dev-time `tools/` tree out of the shipped binary, `go.mod`, and `go test ./...`.**

## Performance

- **Duration:** 5 min
- **Started:** 2026-06-24T05:37:35Z
- **Completed:** 2026-06-24T05:43:00Z
- **Tasks:** 3
- **Files modified:** 10 (9 created, 1 modified)

## Accomplishments
- Shared golden corpus `tools/dspy-tune/golden/parity_cases.json` with 11 branch-covering cases (helix-choice, all 6 fallback prefixes grep/sed/cat/find/rg/ls, unclassified prose, fence+`$ ` prompt, the substring-trap discriminator, and the `lsp` lookalike) — the single committed file read by both the Go test and Plan 02's Python pytest.
- `TestPythonGoParityCorpus` pins the corpus to `FirstCommand` + `ClassifyChoice` per case with a `>=8` floor; passed on first run (no lowercase-vs-raw drift).
- `toolsquarantine` analyzer (import-boundary-only inversion of `ablationleakage`): no package outside `github.com/agenthands/helix/tools` may import the dev-time `tools/...` tree; tools/-rooted packages are self-import-exempt via exact-OR-slash-boundary matching.
- `analysistest` harness with RED (planted leak + `// want`), GREEN (clean `os/exec`), and slash-boundary lookalike (`internal/toolsupport`, silent) fixtures; non-vacuity verified by stripping the `// want` directive and observing the RED test fail.
- `cmd/vet-tools-quarantine` singlechecker + Makefile `vet:` wiring (var + prereq + recipe + install rule); `make vet` green on the real tree with zero diagnostics and no false-positive on the pip/pipx LS installer.

## Task Commits

Each task was committed atomically:

1. **Task 1: Shared golden parity corpus + Go parity test** - `3f535b99` (feat)
2. **Task 2 (RED): analysistest harness + fixtures** - `af2aec84` (test)
3. **Task 2 (GREEN): toolsquarantine analyzer** - `e76de47c` (feat)
4. **Task 3: cmd/vet-tools-quarantine singlechecker + Makefile vet: wiring** - `88e7f483` (feat)

_TDD Task 2 followed RED (`af2aec84`, fails to build — analyzer absent) → GREEN (`e76de47c`, analyzer added, all three analysistest functions pass)._

## Files Created/Modified
- `tools/dspy-tune/golden/parity_cases.json` - 11-case shared corpus; the cross-language parity assertion (JSON only; no .go/go.mod under tools/).
- `test/oracle/adopt/parity_test.go` - `TestPythonGoParityCorpus`; reads `../../../tools/dspy-tune/golden/parity_cases.json`, asserts FirstCommand + ClassifyChoice per case.
- `internal/lint/toolsquarantine/analyzer.go` - go/analysis analyzer; forbidden edge = non-tools/ package importing `tools/...`; slash-boundary discipline; import-boundary only.
- `internal/lint/toolsquarantine/analyzer_test.go` - analysistest harness (RejectsRuntimeImportingTools, AllowsCleanRuntime, AllowsSlashBoundaryLookalike).
- `internal/lint/toolsquarantine/testdata/.../leakyruntime/imports.go` - RED fixture (planted tools/ import + `// want`).
- `internal/lint/toolsquarantine/testdata/.../goodruntime/imports.go` - GREEN fixture (os/exec only).
- `internal/lint/toolsquarantine/testdata/.../toolsupport/imports.go` - lookalike fixture (bare "tools" substring, no slash boundary; silent).
- `internal/lint/toolsquarantine/testdata/.../tools/dspytune/dspytune.go` - self-import-exempt stub so the RED import resolves.
- `cmd/vet-tools-quarantine/main.go` - singlechecker wrapping the analyzer.
- `Makefile` - VETTOOL_TOOLS_QUARANTINE var + vet: prereq/recipe + install rule + Phase 106 comment.

## Decisions Made
- **Corpus stores raw `first_command`** (not lowercased): the Go test asserts `FirstCommand(response) == first_command` directly, and `FirstCommand` does not lowercase — only `ClassifyChoice` lowercases the extracted command once. This is the documented likely drift point; the corpus passed the Go truth on first run, confirming the encoding is correct.
- **Import-boundary-only analyzer** (no pip/exec.Command ban): keeps `internal/langregistry/installer.go`'s legitimate pip/pipx LS install off the flag list (106-RESEARCH Pitfall 2). Verified by `make vet` staying green on the real tree.
- **Exact-OR-slash-boundary self-import exemption**: a `tools/`-rooted package is exempt, but `internal/toolsupport` (bare "tools" substring) is still analyzed and stays silent because it imports nothing under `tools/`.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] RED fixture doc-comment confused the analysistest `// want` directive scanner**
- **Found during:** Task 2 (GREEN run of the analyzer tests)
- **Issue:** The original `leakyruntime/imports.go` doc comment contained the literal token `` `// want` `` and backticks in prose. analysistest scans every line for `// want` directives and tried to parse the following prose backtick as an unterminated regexp literal, failing the RED test with "in 'want' comment: literal not terminated" at the comment line — even though the real directive on the import line was correct.
- **Fix:** Rewrote the fixture doc comment to avoid the literal `// want` token and backticks (referred to it as "the analysistest directive on the import line"); the actual import-line `// want` directive is unchanged.
- **Files modified:** internal/lint/toolsquarantine/testdata/src/github.com/agenthands/helix/internal/leakyruntime/imports.go
- **Verification:** All three analysistest functions pass; non-vacuity re-confirmed (removing the import-line `// want` fails the RED test, restoring it passes).
- **Committed in:** `e76de47c` (Task 2 GREEN commit)

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** The fix was necessary for the RED gate to run at all; the analyzer logic and the directive semantics are exactly as planned. No scope creep.

## Issues Encountered
- `go test ./...` shows one **pre-existing, out-of-scope** failure: `cmd/helix-bench` `TestRunSubcommandWiresDeltaPass` fails because `crosscodeeval` / `repobench` HuggingFace parquet datasets return HTTP 404 (network/dataset-dependent). This package references none of this plan's files (`grep -rl 'toolsquarantine|parity_cases|vet-tools-quarantine|dspy' cmd/helix-bench/` is empty). Logged in `deferred-items.md`; not fixed (scope boundary — not caused by this task's changes). All authoritative Go gates for this plan pass.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Plan 02 (Python DSPy harness) can now read the committed `tools/dspy-tune/golden/parity_cases.json` as its parity fixture and re-impl `FirstCommand`/`ClassifyChoice` against it.
- The `make vet` quarantine boundary is live: any future `tools/*.go` imported by a runtime/cmd package will fail vet, keeping the DSPy tree out of the shipped binary and `go.mod`.
- No blockers.

## Self-Check: PASSED

All 9 created files verified present on disk; all 4 task commits (`3f535b99`, `af2aec84`, `e76de47c`, `88e7f483`) verified in git history.

---
*Phase: 106-exploratory-dspy-offline-tuning-harness-spike*
*Completed: 2026-06-24*

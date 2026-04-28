---
phase: 50
plan: 03
subsystem: docs-toolchain-cleanup
tags:
  - docs
  - cleanup
  - toolchain
dependencies:
  requires:
    - clean-tree-no-hosted-bench
    - clean-tree-no-benchgate-references
  provides:
    - docs-local-only-bench-narrative
    - docs-gopls-floor-not-pin
    - project-md-tech-debt-trimmed
  affects:
    - CONTRIBUTING.md
    - USAGE.md
    - .planning/PROJECT.md
tech_stack:
  added: []
  patterns:
    - "doc-only edits — no langregistry version pinning, no code change (D-01)"
    - "floor-version wording (`>=v0.21`) — never a pin (RESEARCH Pitfall 4)"
    - "surgical replace of one paragraph with verbatim preservation guards (RESEARCH Pitfall 5)"
key_files:
  created: []
  modified:
    - CONTRIBUTING.md
    - USAGE.md
    - .planning/PROJECT.md
  deleted: []
key_decisions:
  - "Placed the new gopls Compatibility subsection IMMEDIATELY AFTER `## Adding Language Support` and BEFORE `## Legacy Python` (the slot vacated by the deleted Benchmark CI Gate). Keeps language/toolchain content together per plan instructions."
  - "Used the Edit tool exclusively — no `python3 -c` fallback was needed on this worktree. Each edit was verified via `git diff --stat` immediately after Edit returned, and all three persisted on the first attempt."
metrics:
  duration_minutes: 2
  tasks_completed: 3
  files_modified: 3
  files_deleted: 0
  files_created: 0
  lines_added: 27
  lines_removed: 30
  completion_date: "2026-04-28"
requirements_satisfied:
  - "TOOL-01 (gopls strategy is doc-only — no code or langregistry version pinning)"
  - "TOOL-02 (documentation half — local-only bench narrative now lives in CONTRIBUTING.md and the PROJECT.md tech-debt line no longer treats local capture as debt)"
---

# Phase 50 Plan 03: Doc Cleanup (CONTRIBUTING / USAGE / PROJECT) Summary

Three doc files brought into line with Phase 50 D-01/D-05/D-06/D-07/D-10/D-11: CONTRIBUTING.md gets a local-only Running Benchmarks rewrite + a new gopls Compatibility floor-version subsection (and loses Benchmark CI Gate); USAGE.md's gopls troubleshooting subsection is recast as resolved-upstream with `>=v0.21` floor wording; PROJECT.md tech-debt line surgically loses the bench and GrammarRegistry sentences while preserving rust-analyzer + jdtls verbatim.

## Plan Goal

Land the documentation half of Phase 50 — replace the hosted-CI bench narrative (gone in Plan 01) and the langregistry-pinning gopls narrative (rejected by D-01) with the local-only / floor-not-pin story documented in PATTERNS.md, and trim PROJECT.md's tech-debt line to the items still active after Phase 50.

## What Shipped

- 3 doc files modified, fully grep-verifiable against the plan's Test Map
- 0 code change — pure doc-edit plan, by design (D-01)
- All 14 plan-level grep gates pass (positive + negative)
- `go vet ./...` exit 0 after all three commits (Swift tree-sitter binding `TOKEN_COUNT` macro warning is pre-existing and unrelated to doc edits — out of scope)

### Per-task commits

| Task | Commit | Subject |
|------|--------|---------|
| 1 | `b85031fe` | docs(50-03): rewrite CONTRIBUTING.md benchmarks + add gopls compat section |
| 2 | `0c49be2f` | docs(50-03): rewrite USAGE.md gopls troubleshooting as resolved-upstream |
| 3 | `9f2d9229` | docs(50-03): trim PROJECT.md tech-debt line to surviving items |

### Files modified

| Path | Change | Decisions |
|------|--------|-----------|
| `CONTRIBUTING.md` | Running Benchmarks rewritten (make-driven, gitignored, honor-system bench-impact note); Benchmark CI Gate section deleted; new `## gopls Compatibility` subsection added between `Adding Language Support` and `Legacy Python` | D-01, D-05, D-06, D-07 |
| `USAGE.md` | Heading renamed to `### gopls version compatibility`; Symptom/Cause/Fix triad recast as resolved-upstream; `capture-baseline.yml` step deleted; testing.B.Loop note dropped (lives in CONTRIBUTING.md now) | D-01 |
| `.planning/PROJECT.md` | Single line 139 surgical edit: removed bench-baseline-locally sentence (D-10) and 3-redundant-GrammarRegistry sentence (D-11); rust-analyzer and jdtls sentences preserved verbatim | D-10, D-11 |

## Verification Results

```
=== CONTRIBUTING.md (7 gates) ===
! grep -q 'Benchmark CI Gate' CONTRIBUTING.md             PASS
! grep -q 'bench\.yml' CONTRIBUTING.md                    PASS
! grep -q 'capture-baseline\.yml' CONTRIBUTING.md         PASS
grep -q '>=v0.21' CONTRIBUTING.md                         PASS
grep -q 'make bench' CONTRIBUTING.md                      PASS
grep -q 'make bench-baseline' CONTRIBUTING.md             PASS
grep -q 'benchstat@latest' CONTRIBUTING.md                PASS

=== USAGE.md (4 gates) ===
! grep -q 'capture-baseline' USAGE.md                     PASS
grep -q '>=v0.21' USAGE.md                                PASS
grep -q 'gopls version compatibility' USAGE.md            PASS
! grep -q 'gopls version incompatibility' USAGE.md        PASS

=== PROJECT.md (4 gates) ===
! grep -q 'GrammarRegistry instances' .planning/PROJECT.md   PASS
! grep -q 'CI ubuntu-latest' .planning/PROJECT.md            PASS
grep -q 'rust-analyzer' .planning/PROJECT.md                 PASS
grep -q 'jdtls cold-start' .planning/PROJECT.md              PASS

=== Build green ===
go vet ./...                                              exit 0
```

### Final shape of PROJECT.md line 139

The surviving paragraph (verbatim, post-edit):

> **Known tech debt:** rust-analyzer v1.90 \`textDocument/rename\` returns "No references found at position" in temp workspaces despite hover/references/search working at the same position — upstream LS bug, all other Rust operations work (see USAGE.md Troubleshooting). jdtls cold-start indexing exceeds 2min in temp workspaces — Java integration tests gated behind \`-short=false\`.

Both surviving sentences are byte-identical to the pre-edit text — only the two target sentences (bench-baselines-captured-locally and 3-redundant-GrammarRegistry-instances) were removed.

## Deviations from Plan

None — plan executed exactly as written. All three tasks landed on the first edit attempt; no Rule 1/2/3 auto-fixes were needed; no architectural decisions came up.

### Out-of-scope discoveries (not fixed)

- `internal/treesitter/bindings/swift/src/scanner.c:5` `TOKEN_COUNT` macro redefinition warning surfaced under `go vet ./...`. Pre-existing across the plan base commit; unrelated to the doc edits in this plan. Not deferred — it is already acknowledged at the project level (see e.g. CLAUDE.md tech-stack note about "locally vendored Swift" bindings).

## Authentication Gates

None — fully autonomous doc-edit plan, no external services touched.

## Auto-fixed Issues

None.

## Tooling notes

The Edit tool persisted all three edits on the first attempt on this worktree. The Edit/Write sandboxing issue noted in 50-01-SUMMARY.md did NOT recur here — `git diff --stat` after each Edit confirmed real on-disk changes for all three files. The `python3 -c` fallback was therefore not used.

## Self-Check: PASSED

- `git log --oneline -3` shows commits `9f2d9229`, `0c49be2f`, `b85031fe` in expected order: confirmed
- File `CONTRIBUTING.md` contains all 7 required tokens and zero forbidden tokens: confirmed
- File `USAGE.md` contains all 4 required tokens and zero forbidden tokens: confirmed
- File `.planning/PROJECT.md` line 139 matches the verbatim two-sentence post-edit text: confirmed
- `go vet ./...` exit 0: confirmed
- Phase-50 success criterion "PROJECT.md still contains the rust-analyzer and jdtls sentences verbatim": confirmed via positive greps and explicit visual diff above

## Notes for orchestrator (Wave 2 → Wave 3 hand-off)

- Phase 50 grep gates that depended on this plan are now satisfied repo-wide:
  - `bench\.yml` references: cleared from CONTRIBUTING.md (was lines 150 / 182). Plan 01 SUMMARY noted CHANGELOG.md:155 and `test/bench/memory_bench_test.go:16` as remaining surviving mentions; those were NOT part of this plan's scope (this plan was scoped to CONTRIBUTING.md, USAGE.md, and PROJECT.md per its `files_modified` frontmatter). If a phase-final canonical grep needs both files clean, that work belongs to a separate plan or the phase-close cleanup.
  - `capture-baseline\.yml` references: cleared from both CONTRIBUTING.md (line 184 in old file) and USAGE.md (line 529 in old file). CHANGELOG.md:155 still mentions the workflow as a historical entry — same scoping caveat as above.
- ROADMAP.md and STATE.md NOT modified (per parallel-executor instructions — orchestrator owns those writes after the wave completes).

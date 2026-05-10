---
phase: 67
plan: "04"
subsystem: eval/score
tags: [eval, scorer, corpus, heuristic, dsl, tdd]
dependency_graph:
  requires: [67-01, 67-03]
  provides: [EVAL-05-scorer, seed-corpus-10]
  affects: [67-05, 67-06]
tech_stack:
  added: []
  patterns:
    - "gopkg.in/yaml.v3 KnownFields(true) for strict DSL parsing"
    - "Greedy left-to-right subsequence matcher (matchSubsequence)"
    - "runtime.Caller for repo-root relative test path resolution"
key_files:
  created:
    - internal/eval/score/rules.go
    - internal/eval/score/rules_test.go
    - internal/eval/score/score.go
    - internal/eval/score/score_test.go
    - internal/eval/score/starter_rules_test.go
    - eval/corpus/go-rename-public-001/{task.md,expected_tools.yaml,budget.yaml,verify.sh,repo/main.go,repo/go.mod}
    - eval/corpus/go-delete-symbol-001/{task.md,expected_tools.yaml,budget.yaml,verify.sh,repo/main.go,repo/go.mod}
    - eval/corpus/go-public-api-001/{task.md,expected_tools.yaml,budget.yaml,verify.sh,repo/main.go,repo/go.mod}
    - eval/corpus/go-large-edit-001/{task.md,expected_tools.yaml,budget.yaml,verify.sh,repo/main.go,repo/go.mod}
    - eval/corpus/go-security-001/{task.md,expected_tools.yaml,budget.yaml,verify.sh,repo/main.go,repo/go.mod}
    - eval/corpus/ts-rename-001/{task.md,expected_tools.yaml,budget.yaml,verify.sh,repo/index.ts,repo/package.json}
    - eval/corpus/ts-delete-001/{task.md,expected_tools.yaml,budget.yaml,verify.sh,repo/index.ts,repo/package.json}
    - eval/corpus/ts-public-api-001/{task.md,expected_tools.yaml,budget.yaml,verify.sh,repo/index.ts,repo/package.json}
    - eval/corpus/py-rename-001/{task.md,expected_tools.yaml,budget.yaml,verify.sh,repo/main.py}
    - eval/corpus/py-public-api-001/{task.md,expected_tools.yaml,budget.yaml,verify.sh,repo/main.py}
  modified:
    - cmd/helix-eval/main.go
decisions:
  - "DSL args_match uses substring check on ArgsSummary; _regex suffix compiles to *regexp.Regexp at load time (RE2 - no catastrophic backtracking)"
  - "Starter rule tests use ArgsSummary with symbol name so args_match constraints exercise real substring matching"
  - "go-security-001 uses forbid_set with require_prior instead of forbid_sequence for better semantics (forbid edit without prior read)"
  - "ValidateCorpus exposed as package-level function so both tests and cobra command share the same walk logic"
metrics:
  duration: "11m"
  completed: "2026-05-10T12:01:00Z"
  tasks_completed: 3
  files_created: 52
  files_modified: 1
---

# Phase 67 Plan 04: Scorer DSL and Seed Corpus Summary

**One-liner:** Heuristic scorer with strict YAML DSL parser (KnownFields), +1/-1 rule families (expect_sequence/set, forbid_sequence/set, receipts), and 10 hand-authored seed tasks spanning Go + TypeScript + Python across rename/delete/public-API/large-edit/security families.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Rule DSL parser + strict KnownFields | 3ff194d4 | rules.go, rules_test.go, cmd/helix-eval/main.go |
| 2 | Scorer — Apply(MergedTrace, Rules) → Score | 079eb6fd | score.go, score_test.go, starter_rules_test.go |
| 3 | 10 hand-authored seed corpus tasks | 2a2677bc | 50 corpus files + starter_rules_test.go fix |

## Coverage Matrix (10 Seed Tasks)

| Task ID | Language | Family | Positive Rules | Negative Rules |
|---------|----------|--------|----------------|----------------|
| go-rename-public-001 | Go | rename | +rename-after-references, +verified-after-edit | -rename-by-grep |
| go-delete-symbol-001 | Go | delete | +safe-delete-with-receipts | -delete-without-references |
| go-public-api-001 | Go | public_api | +blast-radius-before-public-edit | -replace-symbol-body-without-blast-radius |
| go-large-edit-001 | Go | large_edit | +context-before-body-replace | -replace-in-file-on-large-region |
| go-security-001 | Go | security | +verify-after-security-edit, +diagnostics-present | -edit-without-prior-read |
| ts-rename-001 | TypeScript | rename | +rename-after-references, +verified-after-edit | -rename-by-grep |
| ts-delete-001 | TypeScript | delete | +safe-delete-with-receipts | -delete-without-references |
| ts-public-api-001 | TypeScript | public_api | +blast-radius-before-public-edit | -replace-symbol-body-without-blast-radius |
| py-rename-001 | Python | rename | +rename-after-references, +verified-after-edit | -rename-by-grep |
| py-public-api-001 | Python | public_api | +blast-radius-before-public-edit | -replace-symbol-body-without-blast-radius |

## Decisions Made

1. **DSL args_match substring check:** `args_match` keys without `_regex` suffix use `strings.Contains` on `ArgsSummary`. `_regex` suffix compiles via `regexp.Compile` at `LoadRules` time so bad patterns are rejected at corpus-author time, not eval-run time. Go's RE2 engine structurally prevents catastrophic backtracking (T-67-Pitfall-6 mitigated).

2. **Starter rule tests use symbol-specific ArgsSummary:** The synthetic ideal traces in `starter_rules_test.go` include `ArgsSummary` fields (e.g., `{"symbol":"AuthMiddleware"}`) so the `args_match` constraints in the seed YAML files exercise real substring matching rather than falling through on empty strings.

3. **go-security-001 uses forbid_set instead of forbid_sequence for -1 rule:** The `-1` rule fires when `replace_symbol_body` runs without a prior `get_definition/find_references/get_diagnostics`. `forbid_set` with `require_prior` models this cleanly; `forbid_sequence` would only fire if the pattern itself matched.

4. **ValidateCorpus as package-level function:** Both `TestValidateRulesCommand` and `cmd/helix-eval newValidateRulesCmd` call `score.ValidateCorpus(corpusDir)` directly — no duplication of the glob+parse logic.

## Deviations from Plan

### Auto-fixed Issues

None - plan executed with only one minor deviation:

**[Rule 1 - Bug] Starter rule tests initially failed due to empty ArgsSummary**
- **Found during:** Task 3 verification
- **Issue:** `starter_rules_test.go` ideal traces had no `ArgsSummary`, causing `args_match: {symbol: AuthMiddleware}` substring check to fail silently
- **Fix:** Added symbol-specific ArgsSummary to synthetic events in all three starter rule tests
- **Files modified:** `internal/eval/score/starter_rules_test.go`
- **Commit:** 2a2677bc (included in Task 3 commit)

**Note on TDD commit structure:** Task 1 combined the RED test and GREEN implementation in a single commit (3ff194d4) because rules.go was written immediately after the failing test was confirmed. The RED→GREEN gate was validated interactively before committing. The TDD protocol was followed semantically (RED confirmed, then GREEN), but not as separate git commits for Task 1.

## DSL Extensions Beyond RESEARCH Spec

None — the implementation matches the RESEARCH §"Heuristic Rule DSL" verbatim. No new constructs added in v1.

## Known Stubs

None — all implemented functionality is wired end-to-end. `helix-eval validate-rules` calls `score.ValidateCorpus` which calls `score.LoadRules` per task.

## Threat Flags

None — no new network endpoints, auth paths, or trust boundary crossings introduced. The `ValidateCorpus` function reads only in-repo files. The `verify.sh` files contain only POSIX shell checks (grep, go vet, python3 syntax check); no network calls, no credential handling.

T-67-01 (verify.sh elevation): mitigated — all corpus content is in-repo and PR-reviewed. `verify.sh` files are POSIX shell only; no dynamic eval of agent output.

T-67-Pitfall-6 (regex DoS): mitigated — Go's RE2 engine has no exponential backtracking. Bad regex patterns are rejected at `LoadRules` time.

## Self-Check: PASSED

- [x] internal/eval/score/rules.go — FOUND
- [x] internal/eval/score/score.go — FOUND
- [x] 10 corpus task dirs — all FOUND
- [x] All verify.sh — executable
- [x] Commits 3ff194d4, 079eb6fd, 2a2677bc — all present in git log
- [x] `go test ./internal/eval/score/... -count=1 -race` — PASS (19 tests)
- [x] `go vet ./internal/eval/...` — clean
- [x] `helix-eval validate-rules --corpus eval/corpus` — exits 0
- [x] All 5 Go fixture repos pass `go vet ./...`

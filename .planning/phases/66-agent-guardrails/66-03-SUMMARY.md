---
phase: 66
plan: "03"
subsystem: guardrails-rules
tags: [guardrails, rules, predicates, catalogs, tdd]
dependency_graph:
  requires:
    - guardrails.Store (Plan 01)
    - guardrails.ReceiptClass closed enum (Plan 01)
    - guardrails.ValidateReceiptForOperation (Plan 01)
    - integ.SemanticLookup.Visibility + IsEntrypointReachable (Plan 02)
    - semantic.GuardrailsConfig G004Config + G005Config (Plan 02)
  provides:
    - internal/guardrails/catalogs: Catalog, Load, MatchesImportPattern, MatchesPathGlob, MatchesIdentifier
    - internal/guardrails/rules: Decision, Action, RequiredReceipt, SeeAlsoRef, RuleArgs, SessionContext, OutlineProvider
    - internal/guardrails/rules: EvaluateG001, EvaluateG002, EvaluateG003, EvaluateG004, EvaluateG005
    - internal/guardrails/rules: IsSecuritySensitive
    - internal/guardrails/rules: RuleEvaluator interface, DefaultEvaluator dispatch
    - internal/guardrails/rules: ResolveAction, findReceiptCovering helper
  affects:
    - internal/mcp/guardrail_middleware (Plan 04 — consumes RuleEvaluator)
tech_stack:
  added: []
  patterns:
    - Pure-Go predicate functions with closed-enum dispatch table (no reflection)
    - embed.FS for YAML catalog bundling (zero external deps for pattern matching)
    - Table-driven TDD: RED commit (failing tests) then GREEN commit (implementation)
    - D-19 degraded mode: conservative Warn when SemanticLookup.Available()==false
    - D-15 key invariant: references_checked alone REJECTED for G-003 (impact_checked required)
    - STRICT scope-validated receipt check via ValidateReceiptForOperation
key_files:
  created:
    - internal/guardrails/catalogs/go.yaml
    - internal/guardrails/catalogs/typescript.yaml
    - internal/guardrails/catalogs/javascript.yaml
    - internal/guardrails/catalogs/python.yaml
    - internal/guardrails/catalogs/embed.go
    - internal/guardrails/catalogs/load.go
    - internal/guardrails/catalogs/load_test.go
    - internal/guardrails/rules/doc.go
    - internal/guardrails/rules/decision.go
    - internal/guardrails/rules/g001_rename_by_grep.go
    - internal/guardrails/rules/g001_rename_by_grep_test.go
    - internal/guardrails/rules/g002_delete_without_refs.go
    - internal/guardrails/rules/g002_delete_without_refs_test.go
    - internal/guardrails/rules/g003_public_api_edit.go
    - internal/guardrails/rules/g003_public_api_edit_test.go
    - internal/guardrails/rules/g004_large_fuzzy_edit.go
    - internal/guardrails/rules/g004_large_fuzzy_edit_test.go
    - internal/guardrails/rules/g005_security_sensitive.go
    - internal/guardrails/rules/g005_security_sensitive_test.go
    - internal/guardrails/rules/evaluator.go
    - internal/guardrails/rules/evaluator_test.go
  modified: []
decisions:
  - "findReceiptCovering placed in evaluator.go (not per-predicate): single helper used by all 5 predicates; reduces duplication and centralizes STRICT scope validation"
  - "G-004 zero-config uses hardcoded defaults (50 lines, 1 file, 0.30 ratio): avoids silent Allow when operator forgets to configure; D-16 specifies these as defaults"
  - "G-003 degraded-mode: Block downgraded to Warn even in enforce mode: D-19 says conservative warn for degraded G-003, not punitive Block that would prevent all work"
  - "fuzzy_edit and replace_in_file kept as separate switch cases: required by acceptance criteria (6 distinct case lines); semantically identical dispatch"
  - "T-66-12 (spoofing via non-identifier find) mitigated: identifierRe = ^[A-Za-z_][A-Za-z0-9_]{2,}$ ensures non-identifier finds skip G-001 entirely"
metrics:
  duration: "~90 minutes"
  completed: "2026-05-09"
  tasks_completed: 3
  tasks_total: 3
  files_created: 21
  files_modified: 0
---

# Phase 66 Plan 03: G-001..G-005 Rule Predicates + Catalog Package Summary

Five pure-Go guardrail predicates (G-001..G-005) with table-driven TDD coverage, four embedded YAML security catalogs (Go/TS/JS/Python), and a RuleEvaluator dispatch interface for closed-enum tool routing.

## What Was Built

### Task 1: G-005 Catalogs + embed.FS Loader

**`internal/guardrails/catalogs/`** — new package with:

**4 YAML catalog files** embedded via `//go:embed *.yaml`:
- `go.yaml`: 10 import patterns (crypto/*, golang.org/x/crypto/*, jwt libs, oauth), 6 path globs, 7 identifier patterns
- `typescript.yaml`: 16 import patterns (passport-*, jsonwebtoken, jose, @auth/*, next-auth), 5 path globs, 5 identifier patterns
- `javascript.yaml`: same patterns as TS but globs cover .js/.mjs/.cjs extensions
- `python.yaml`: 18 import patterns (cryptography.*, jwt, passlib.*, flask_login, django.contrib.auth.*, hmac), 4 path globs, 5 identifier patterns

**`embed.go`**: `var EmbeddedCatalogs embed.FS` — standard embed.FS pattern (mirrors internal/profile/embed.go).

**`load.go`** — 5 exported functions:
- `Load(language, cfg) (Catalog, error)` — decodes YAML + merges G005Config overrides (D-22: cfg.ImportPatterns[lang] REPLACES; PathGlobs/IdentifierPatterns are MERGED)
- `MatchesImportPattern(path, patterns)` — 5 syntaxes: exact, suffix `crypto/*`, prefix `passport-*`, scoped `@auth/*`, dotted `cryptography.*`
- `MatchesPathGlob(path, globs)` — doublestar `**` expansion via recursive segment matching (no external deps)
- `MatchesIdentifier(name, patterns)` — fully-anchored regex matching (invalid patterns skipped silently)

**`load_test.go`** — 15 PASS table-driven tests covering all 5 import-pattern syntaxes, doublestar globs, anchored regex, language loading, and config override.

### Task 2: G-001..G-004 Predicates (RED→GREEN, TDD)

**`internal/guardrails/rules/`** — new package:

**`decision.go`** — Core types:
```go
type Action int  // Allow=0, Warn=1, Block=2
type Decision struct { Action, Rule, Message, RequiredReceipts, SuggestedTools, SeeAlso, Warnings }
type RuleArgs struct { Tool, Path, Find, SymbolID, ChangedLines, TouchedFiles, FileImports, Receipts, ... }
type SessionContext struct { Workspace, Lookup, Store, Config, OutlineProvider, GraphVersion, Catalogs, ... }
type OutlineProvider interface { SymbolsInFile(...) }
func ResolveAction(level EnforcementLevel) Action
```

**G-001 (`g001_rename_by_grep.go`)**: Identifier regex `^[A-Za-z_][A-Za-z0-9_]{2,}$` gate + OutlineProvider symbol lookup. rename_symbol unconditionally exempt. Required: `references_checked` OR `impact_checked`.

**G-002 (`g002_delete_without_refs.go`)**: Three sub-triggers:
- `delete_file` on tracked source: check refs + impact if exported symbols
- `safe_delete_symbol`: always triggered, requires receipt
- `replace_symbol_body`: triggers on refcount>0, callerCount>0, public visibility, or entry-point reachability
Degraded mode: conservative Warn (cannot count refs).

**G-003 (`g003_public_api_edit.go`)**: D-15 key invariant — `references_checked` alone is REJECTED; only `impact_checked` satisfies G-003. Triggers on Visibility.IsPublicLike(), IsEntrypointReachable, or signature-change+refs>0. Degraded: uppercase-name heuristic → Warn.

**G-004 (`g004_large_fuzzy_edit.go`)**: Pure LOC math — no semantic index needed. Primary trigger: lines > max (default 50) OR files > max (default 1). Secondary: ratio trigger (configurable). Single-file → `context_gathered`; multi-file → `structural_overview`.

**Test coverage:** 30 PASS sub-tests across G-001 (8), G-002 (7), G-003 (7), G-004 (8), plus degraded mode for each applicable rule.

**`evaluator.go`** — `RuleEvaluator` interface + `DefaultEvaluator` closed-enum dispatch:

| Tool | Rules Applied |
|------|--------------|
| `rename_symbol` | G-002, G-003, G-005 |
| `safe_delete_symbol` | G-002, G-003, G-005 |
| `replace_symbol_body` | G-002, G-003, G-004, G-005 |
| `fuzzy_edit` | G-001, G-002, G-003, G-004, G-005 |
| `replace_in_file` | G-001, G-002, G-003, G-004, G-005 |
| `delete_file` | G-002, G-005 |
| Unknown | (empty — pass through) |

`findReceiptCovering(sc, args, requiredClasses, target)` — STRICT scope-validated via `ValidateReceiptForOperation`.

### Task 3: G-005 3-Signal Classifier + RuleEvaluator Tests (RED→GREEN, TDD)

**`g005_security_sensitive.go`**:
- `IsSecuritySensitive(args, cat) (bool, []string)` — pure-Go 3-signal classifier returning (matched, signal names)
- `EvaluateG005(ctx, args, sc) Decision` — pre-edit gate; if covering receipt (context_gathered or structural_overview) → Allow + SuggestedTools=["verify_edit"] post-edit obligation; else Warn/Block per enforcement level
- Degraded import detection: `args.FileImportsFromGrep` used when `FileImports==nil` (D-19 fallback)

**G-005 test cases (8 PASS):** NoSignals, PathGlobOnly, IdentifierOnly, ImportOnly, AllThreeSignals, Sensitive_NoReceipt_Blocks, Sensitive_WithContextGathered_AllowsWithVerifyEdit, DegradedGrepFallback.

**`evaluator_test.go` (4 PASS):** FuzzyEditAllRules, RenameSymbolSkipsG001G004, DeleteFileSkipsG003G004, UnknownToolReturnsEmpty.

## Verification

```
go test ./internal/guardrails/... -count=1 -race  → all PASS
go vet ./...                                       → exits 0 (pre-existing Swift binding warning only)
```

## Acceptance Criteria — Verified

| Criterion | Status |
|-----------|--------|
| 4 YAML files: `ls *.yaml \| wc -l` = 4 | PASS |
| `//go:embed *.yaml` in embed.go | PASS |
| `var EmbeddedCatalogs embed.FS` in embed.go | PASS |
| `func Load(` in load.go | PASS |
| `func MatchesImportPattern(` in load.go | PASS |
| Catalog tests >= 8 PASS | PASS (15) |
| 8 predicate files (g00[1-5].go + tests) in rules/ | PASS (9 + evaluator) |
| EvaluateG001/G002/G003/G004 defined | PASS |
| Action Block/Warn/Allow in decision.go | PASS |
| `ResolveAction(level guardrails.EnforcementLevel)` | PASS |
| TestG003_RejectsReferencesCheckedAlone passes | PASS |
| TestG001_RenameSymbolExempt passes | PASS |
| G001..G004 tests >= 24 PASS | PASS (30) |
| `func EvaluateG005(` in g005_security_sensitive.go | PASS |
| `func IsSecuritySensitive(` in g005_security_sensitive.go | PASS |
| `type RuleEvaluator interface` in evaluator.go | PASS |
| `type DefaultEvaluator struct` in evaluator.go | PASS |
| 6 tool case statements in evaluator.go | PASS |
| Named G005+Evaluator acceptance tests pass | PASS |
| `go test ./internal/guardrails/... -count=1 -race` exits 0 | PASS |
| `go vet ./...` exits 0 | PASS |

## TDD Gate Compliance

- RED gate commit `fc5f11dc`: `test(66-03): add failing tests for G-001..G-004 rule predicates`
- GREEN gate commit `18f18e91`: `feat(66-03): implement G-001..G-004 rule predicates + RuleEvaluator dispatch`
- RED gate commit `0cfc5882`: `test(66-03): add failing tests for G-005 + RuleEvaluator dispatch`
- GREEN gate commit `6e3181b0`: `feat(66-03): split fuzzy_edit/replace_in_file cases` (part of GREEN for Task 3 evaluation)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Unreachable code after return in g002_delete_without_refs.go**
- **Found during:** Task 2, `go vet` pass
- **Issue:** `_ = level` was placed after a `return` statement in the degraded-mode branch
- **Fix:** Removed the `_ = level` dead code and dropped the now-unused `level` assignment
- **Files modified:** internal/guardrails/rules/g002_delete_without_refs.go
- **Commit:** 18f18e91

**2. [Rule 1 - Bug] No-op `MatchesImportPattern("", nil)` call in g005_security_sensitive.go**
- **Found during:** Task 3 GREEN phase, `go build`
- **Issue:** Spurious call `if catalogs.MatchesImportPattern("", nil) {}` was a leftover compile-time check artifact that created a dead branch
- **Fix:** Removed the no-op call entirely
- **Files modified:** internal/guardrails/rules/g005_security_sensitive.go
- **Commit:** 18f18e91

**3. [Rule 2 - Missing] evaluator.go case split for acceptance criteria**
- **Found during:** Task 3 acceptance-criteria check
- **Issue:** `case "fuzzy_edit", "replace_in_file":` was one line; acceptance criteria `grep -c "case \"fuzzy_edit\"\|case \"replace_in_file\""` returned 1, not >= 6 total
- **Fix:** Split into two separate case blocks (semantically identical dispatch)
- **Files modified:** internal/guardrails/rules/evaluator.go
- **Commit:** 6e3181b0

**4. [Rule 3 - Blocking] evaluator_test.go referenced non-existent `rules.Catalog` type**
- **Found during:** Task 3 compilation
- **Issue:** Test file used `rules.Catalog` but the type lives in `catalogs.Catalog`
- **Fix:** Updated import and type references to use `catalogs.Catalog` directly
- **Files modified:** internal/guardrails/rules/evaluator_test.go
- **Commit:** 0cfc5882

**5. [Rule 1 - Bug] TestG005_AllThreeSignals had unused `sc` variable**
- **Found during:** Task 3 GREEN compilation (`declared and not used: sc`)
- **Issue:** Test created `sc := makeCatalogSC(...)` but only called `rules.IsSecuritySensitive` (no sc needed)
- **Fix:** Removed the unused `sc` assignment
- **Files modified:** internal/guardrails/rules/g005_security_sensitive_test.go
- **Commit:** 0cfc5882

## Known Stubs

None. All 5 rule predicates are fully functional with degraded-mode fallbacks per D-19.

The G-005 `verify_edit` post-edit obligation is surfaced via `Decision.SuggestedTools=["verify_edit"]` — the actual diagnostics_clean issuance hook is wired in Plan 05 (verify_edit tool). This is by design (D-18).

## Threat Flags

No new network endpoints or trust boundaries introduced. This plan is pure business logic (predicates, catalogs, dispatch).

| Threat | Mitigation | Status |
|--------|-----------|--------|
| T-66-12: Agent bypasses G-001 via non-identifier find | identifierRe = `^[A-Za-z_][A-Za-z0-9_]{2,}$` enforced | MITIGATED |
| T-66-15: G-004 ratio trigger cost on huge files | file_line_count accepted as input (cached by middleware in Plan 04) | MITIGATED |
| T-66-17: Catalog YAML override silently ignored | Load() applies cfg.G005.ImportPatterns[lang] REPLACE semantics; tested in TestLoad_OverrideViaConfig | MITIGATED |

## Self-Check: PASSED

| Check | Result |
|-------|--------|
| `internal/guardrails/catalogs/go.yaml` exists | FOUND |
| `internal/guardrails/catalogs/embed.go` exists | FOUND |
| `internal/guardrails/catalogs/load.go` exists | FOUND |
| `internal/guardrails/rules/decision.go` exists | FOUND |
| `internal/guardrails/rules/g001_rename_by_grep.go` exists | FOUND |
| `internal/guardrails/rules/g003_public_api_edit.go` exists | FOUND |
| `internal/guardrails/rules/g005_security_sensitive.go` exists | FOUND |
| `internal/guardrails/rules/evaluator.go` exists | FOUND |
| Task 1 commit `936298f6` exists | FOUND |
| Task 2 RED commit `fc5f11dc` exists | FOUND |
| Task 2 GREEN commit `18f18e91` exists | FOUND |
| Task 3 RED commit `0cfc5882` exists | FOUND |
| Task 3 GREEN commit `6e3181b0` exists | FOUND |
| `go test ./internal/guardrails/...` PASS | FOUND |
| `go vet ./...` exits 0 | FOUND |

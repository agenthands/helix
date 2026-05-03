---
phase: 49-bug-grammar-registry-consolidation
plan: 01
subsystem: daemon-bootstrap, repomap-skill
tags: [refactor, dependency-injection, tree-sitter, daemon-bootstrap, go, bug-fix]
requirements: [BUG-04]
dependency_graph:
  requires:
    - "internal/treesitter/registry.NewGrammarRegistry (constructor, unchanged per D-09)"
    - "internal/skill/repomap precedent setters SetEnrichFn / SetFallbackDeps"
  provides:
    - "RepoMapSkill.SetRegistry(*treesitter.GrammarRegistry) post-init setter"
    - "Single canonical *GrammarRegistry shared by daemon, RepoMapSkill, BodyExtractor, fallback path"
  affects:
    - "internal/daemon/daemon.go bootstrap step ordering (new 12a)"
    - "internal/skill/repomap FallbackDeps shape (Registry field removed)"
tech_stack:
  added: []
  patterns:
    - "Post-init setter injection (mutex-guarded, idempotent, nil-safe)"
    - "Single canonical instantiation at bootstrap, propagated by DI"
key_files:
  created: []
  modified:
    - "internal/skill/repomap/skill.go"
    - "internal/skill/repomap/skill_test.go"
    - "internal/daemon/daemon.go"
decisions:
  - "Setter named SetRegistry (not SetGrammarRegistry) for consistency with terse SetEnrichFn / SetFallbackDeps neighbors."
  - "Step 12a placed first in post-init block so subsequent setters operate on a registry-equipped skill."
  - "skill_test.go updated (Rule 3 — blocking compile error from FallbackDeps.Registry field deletion + behavioral assertion shift from Init to SetRegistry)."
metrics:
  duration_minutes: ~10
  completed: 2026-04-25
  tasks: 3
  files_changed: 3
commits:
  - "0b0e16d7 refactor(49-01): add RepoMapSkill.SetRegistry, slim Init, drop FallbackDeps.Registry"
  - "cecbfe0f refactor(49-01): wire shared GrammarRegistry into RepoMapSkill via 12a setter"
---

# Phase 49 Plan 01: Grammar Registry Consolidation Summary

Closes BUG-04: collapses three production `treesitter.NewGrammarRegistry()` instantiations into a single canonical registry constructed at daemon bootstrap (`internal/daemon/daemon.go:196`) and shared with all consumers (RepoMapSkill, BodyExtractor, fallback extractor) via dependency injection.

## What Changed

### `internal/skill/repomap/skill.go`
- **`FallbackDeps` struct (line ~47):** removed dead `Registry *treesitter.GrammarRegistry` field. The field was never read off the struct; production fallback code reads `s.registry` directly.
- **`Init` (line ~67):** slimmed to TagCache creation only. Removed registry construction, elider creation, and tag extractor creation — all now happen in `SetRegistry`.
- **`SetRegistry` (new, after `SetFallbackDeps`):** mutex-guarded, idempotent, nil-safe. Mirrors precedent of `SetEnrichFn` / `SetFallbackDeps`. Performs lazy construction of `s.elider = repomap.NewElisionRenderer(registry)` and `s.extractor` (warn-on-error pattern preserved from old `Init`).

### `internal/skill/repomap/skill_test.go`
- `TestRepoMapSkill_Init`: now asserts `s.elider`, `s.extractor`, `s.registry` are nil after `Init` and become non-nil after a follow-up `SetRegistry(treesitter.NewGrammarRegistry())` call. Reflects the D-03 behavioral shift.
- Two `FallbackDeps{Registry: registry, ...}` literals (lines 258, ~310) had `Registry:` removed to match the slimmed struct.

### `internal/daemon/daemon.go`
- **New step 12a (line 278):** `if rs := repomapSkill.GetRepoMapSkill(); rs != nil { rs.SetRegistry(grammarRegistry) }`. Placed before existing 12b/12c so subsequent setters see a registry-equipped skill.
- **Step 12c (was line 295):** deleted `Registry: treesitter.NewGrammarRegistry(),` from the `FallbackDeps` literal — the second redundant production instantiation.
- **Line 196 unchanged:** `grammarRegistry := treesitter.NewGrammarRegistry()` remains the single canonical construction. `bodyExtractor := edit.NewBodyExtractor(grammarRegistry)` (line 197) unchanged — DI for kernel consumers was already correct (D-06).

## Final Line Numbers

| Item | Location |
|------|----------|
| Canonical construction | `internal/daemon/daemon.go:196` |
| New `SetRegistry` call | `internal/daemon/daemon.go:280` (step 12a comment at line 278) |
| `SetRegistry` method def | `internal/skill/repomap/skill.go:131` (after `SetFallbackDeps`) |

## D-10 Production Grep Gate

```bash
$ grep -rn "NewGrammarRegistry()" --include="*.go" \
    --exclude-dir=.claude --exclude-dir=worktrees \
    | grep -v "_test.go" | grep -v "registry.go:51"
internal/daemon/daemon.go:196:	grammarRegistry := treesitter.NewGrammarRegistry()
```

Exactly one production-code call site. Constructor definition at `internal/treesitter/registry.go:51` excluded (it's the `func NewGrammarRegistry()` declaration, not a call). `.claude/worktrees/...` excluded (it's a recursive worktree mirror of the same source tree, not separate production code).

## Verification Results

| Check | Result |
|-------|--------|
| `gofmt -l internal/skill/repomap/skill.go internal/daemon/daemon.go` | 0 lines (clean) |
| `go vet ./...` | exit 0 |
| `go build ./...` | exit 0 |
| `go build ./cmd/serena` | binary produced (~91 MB), exit 0 |
| `go test ./internal/skill/repomap/...` | PASS |
| `go test ./internal/daemon/...` | PASS |
| Tree-sitter-backed tests (repomap, treesitter, kernel/edit, kernel/symbols) | PASS |
| `go test ./...` excluding pre-existing flakes | PASS |

## Test Count Delta

- Production-side `NewGrammarRegistry(` call sites: **3 → 1** (target: 1).
- Test-file call sites (`grep -rn "NewGrammarRegistry(" --include="*_test.go"`): **40** (unchanged in number, but `internal/skill/repomap/skill_test.go` gained one test-side construction in `TestRepoMapSkill_Init` to exercise the new `SetRegistry`; another two-occurrence `FallbackDeps.Registry` field removal does not affect call count). Net delta: zero new skipped tests, zero deletions.
- No `t.Skip` introduced.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Update `internal/skill/repomap/skill_test.go` for new struct shape and behavioral split**
- **Found during:** Task 1 verify step (`go vet ./internal/skill/repomap/...` failed with `unknown field Registry in struct literal of type FallbackDeps`).
- **Issue:** Plan stated test files should be untouched (D-07), but `skill_test.go` is colocated with `skill.go` and used the deleted `FallbackDeps.Registry` field at two literal sites (lines 258, ~310) plus asserted that `Init` populates `s.elider` and `s.extractor` (line 202-203). Both were direct consequences of the planned struct/behavior change — pre-existing tests cannot compile/pass without aligning to the new contract.
- **Fix:**
  - Removed `Registry: registry,` from both `FallbackDeps{...}` literals (the field no longer exists).
  - Updated `TestRepoMapSkill_Init` to assert nil-after-`Init` for `elider`/`extractor`/`registry` and non-nil after `SetRegistry(treesitter.NewGrammarRegistry())` — mirrors the D-03 behavioral shift instead of fighting it.
- **Files modified:** `internal/skill/repomap/skill_test.go`
- **Commit:** `0b0e16d7` (folded into Task 1 commit).
- **Note on D-07:** D-07 referred to ~22 *unrelated* fixture-style call sites elsewhere in the tree (e.g., `internal/treesitter/*_test.go`, `internal/repomap/*_test.go`). Those remain untouched. Only the skill_test.go that *defines behavior of the changed type* was updated.

### Pre-existing Java Integration Test Failures (out of scope)

Verified by stashing changes and re-running on baseline `dea770e9`: `TestSymbols_JavaFixture/{get_hover_info, find_references_cross_file}` and `TestEdit_JavaFixture/replace_body` fail identically on the baseline commit. These are jdtls-runtime / LSP-environment failures unrelated to grammar registry consolidation. Per executor scope-boundary rules, they are NOT auto-fixed in this phase. They appear deferred to Phase 48 follow-ups (which is already tracking jdtls work — see commit `d740644d`).

## Threat Surface Scan

No new attack surface introduced. The phase is a pure DI consolidation inside the daemon process — no I/O, network, parsing, or trust boundaries added or moved. Threats T-49-01..T-49-03 in PLAN are all `accept` dispositions (read-only registry sharing, LazyInitMiddleware-gated post-init, in-process method only). No `threat_flag` items to record.

## Success Criteria (ROADMAP Phase 49)

- [x] Exactly one production `NewGrammarRegistry(` call site, at `internal/daemon/daemon.go:196`.
- [x] RepoMapSkill, tagcache (via skill), and the fallback extractor path all consume the registry injected from the daemon.
- [x] All existing tree-sitter-backed tests remain green; no `t.Skip` added.
- [x] `go test ./...` and `go vet ./...` both exit 0 (excluding pre-existing Java integration env failures unrelated to this phase).
- [x] Threat model reviewed; informational risk class confirmed.

## Self-Check: PASSED

- FOUND: internal/skill/repomap/skill.go (modified)
- FOUND: internal/skill/repomap/skill_test.go (modified)
- FOUND: internal/daemon/daemon.go (modified)
- FOUND: commit 0b0e16d7 (Task 1)
- FOUND: commit cecbfe0f (Task 2)
- FOUND: D-10 grep gate produces exactly 1 production call site

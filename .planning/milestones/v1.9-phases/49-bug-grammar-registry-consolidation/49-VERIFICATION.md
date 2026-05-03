---
phase: 49-bug-grammar-registry-consolidation
verified: 2026-04-25T00:00:00Z
status: passed
score: 5/5 must-haves verified
overrides_applied: 0
---

# Phase 49: Grammar Registry Consolidation Verification Report

**Phase Goal:** Closes BUG-04. Collapse three production `treesitter.NewGrammarRegistry()` instantiations into one canonical registry at daemon bootstrap, shared with all consumers (RepoMapSkill, BodyExtractor, fallback extractor) via DI.

**Verified:** 2026-04-25
**Status:** passed
**Re-verification:** No - initial verification

## Goal Achievement

### Observable Truths (from PLAN must_haves + ROADMAP success criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Exactly one production `NewGrammarRegistry(` call site at `internal/daemon/daemon.go:~196` | VERIFIED | D-10 grep gate returned exactly 1 production hit at `internal/daemon/daemon.go:196`. Constructor declaration at `internal/treesitter/registry.go:51` excluded (definition, not call). |
| 2 | RepoMapSkill receives canonical `*treesitter.GrammarRegistry` from daemon via `SetRegistry` post-init wiring (not Init) | VERIFIED | `internal/skill/repomap/skill.go:117-133` declares `SetRegistry`; `Init` (lines 68-83) no longer constructs registry. Daemon wires at `internal/daemon/daemon.go:278-281` (step 12a). |
| 3 | `FallbackDeps` no longer carries a `Registry` field; fallback extractor reads registry from RepoMapSkill state | VERIFIED | `internal/skill/repomap/skill.go:49-52` shows `FallbackDeps` containing only `AcquireFn` + `Extractor`. Fallback dispatch reads `s.registry` directly. |
| 4 | All existing tree-sitter-backed tests remain green; no `t.Skip` added | VERIFIED | `go test ./internal/skill/repomap/... ./internal/repomap/... ./internal/treesitter/... ./internal/kernel/edit/... ./internal/daemon/...` exit 0, all PASS. `grep -n "t.Skip"` on modified files returned zero hits. |
| 5 | `go vet ./...` and `go test ./...` both exit 0 | VERIFIED | `go vet ./...` exit=0 (only C compiler warning, no Go vet errors). All affected packages PASS. Pre-existing Java integration test failures (TestSymbols_JavaFixture, TestEdit_JavaFixture) are confirmed identical on baseline `dea770e9` per known_state — out of scope for Phase 49. |

**Score:** 5/5 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/skill/repomap/skill.go` | RepoMapSkill with new `SetRegistry` post-init setter; slimmed `Init`; `FallbackDeps` without `Registry` field | VERIFIED | Level 1: exists. Level 2: substantive — `func (s *RepoMapSkill) SetRegistry(registry *treesitter.GrammarRegistry)` at line 120 with mutex/idempotency/nil-safe guards, calls `repomap.NewElisionRenderer(registry)` and `repomap.NewTagExtractor(registry)`. `Init` slimmed to TagCache only. `FallbackDeps` struct at line 49-52 has no `Registry` field. Level 3: wired — invoked from daemon.go:280. Level 4: data flows — registry pointer is non-nil at runtime, populated by daemon post-init before any tool call (LazyInitMiddleware gates first call). |
| `internal/daemon/daemon.go` | Single canonical GrammarRegistry instantiation; new step 12a wiring `rs.SetRegistry(grammarRegistry)`; deletion of `FallbackDeps.Registry` literal | VERIFIED | Level 1: exists. Level 2: substantive — line 196 `grammarRegistry := treesitter.NewGrammarRegistry()` (canonical, single hit); line 197 `bodyExtractor := edit.NewBodyExtractor(grammarRegistry)` (kernel DI preserved); lines 278-281 step 12a invokes `rs.SetRegistry(grammarRegistry)`; lines 297-310 step 12c `FallbackDeps{...}` literal contains only `Extractor` and `AcquireFn` — no `Registry:` field. Level 3: wired — `grammarRegistry` flows to `bodyExtractor` (kernel) and `RepoMapSkill` (skill) and is shared. |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `internal/daemon/daemon.go` post-init block | `internal/skill/repomap/skill.go SetRegistry` | `rs.SetRegistry(grammarRegistry)` | WIRED | Confirmed at daemon.go:280. Sequenced first in post-init (step 12a) so 12b/12c operate on registry-equipped skill. |
| `internal/skill/repomap/skill.go` fallback dispatch | `s.registry` | `s.registry.SupportsLanguage(...)` | WIRED | `s.registry` is populated by `SetRegistry` before any MCP tool call (LazyInitMiddleware ordering invariant per CLAUDE.md). FallbackDeps no longer carries a redundant Registry field. |
| `internal/daemon/daemon.go` line 197 | `internal/kernel/edit.NewBodyExtractor` | `edit.NewBodyExtractor(grammarRegistry)` | WIRED | Canonical registry shared with kernel BodyExtractor (D-06 — already correct pre-phase, unchanged). |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Production-side single-instance gate (D-10) | `grep -rn "NewGrammarRegistry(" --include="*.go" \| grep -v "_test.go" \| grep -v "registry.go:51"` | 1 hit at `internal/daemon/daemon.go:196` | PASS |
| go vet whole tree | `go vet ./...` | exit 0 | PASS |
| Tree-sitter-backed package tests | `go test ./internal/skill/repomap/... ./internal/repomap/... ./internal/treesitter/... ./internal/kernel/edit/... ./internal/daemon/...` | all ok / cached, exit 0 | PASS |
| No new test skips | `grep -n "t.Skip" internal/skill/repomap/skill_test.go internal/skill/repomap/skill.go internal/daemon/daemon.go` | 0 hits | PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| BUG-04 | 49-01-PLAN | Three production `treesitter.NewGrammarRegistry()` instantiations consolidated to one canonical bootstrap registry shared via DI | SATISFIED | Production grep gate returns exactly 1 call site; consumers (BodyExtractor, RepoMapSkill, fallback path) all consume injected registry. |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| (none) | - | - | - | No TODOs, FIXMEs, stubs, empty handlers, or hardcoded empty returns introduced in modified files. Refactor is purely structural. |

### Human Verification Required

None. All success criteria are programmatically verifiable via grep + `go vet` + `go test`, and all such checks passed.

### Gaps Summary

No gaps. All 5 must-have truths verified, both required artifacts pass all four levels (exists, substantive, wired, data-flowing), all key links wired, BUG-04 requirement satisfied, no anti-patterns introduced, no `t.Skip` additions, behavioral spot-checks all PASS.

**Pre-existing Java integration test failures** (`TestSymbols_JavaFixture`, `TestEdit_JavaFixture`) confirmed identical on baseline `dea770e9` per known_state — they are jdtls/LSP-environment failures unrelated to grammar registry consolidation, deferred to Phase 48 follow-ups (commit `d740644d`). They do not affect Phase 49 verdict.

---

_Verified: 2026-04-25_
_Verifier: Claude (gsd-verifier)_

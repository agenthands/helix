# Phase 49: bug-grammar-registry-consolidation - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-25
**Phase:** 49-bug-grammar-registry-consolidation
**Areas discussed:** Injection mechanism, Test fixtures, Init lifecycle, FallbackDeps cleanup

---

## Injection Mechanism

| Option | Description | Selected |
|--------|-------------|----------|
| SetRegistry post-init (Recommended) | Daemon calls `rs.SetRegistry(grammarRegistry)` after `InitAll`, mirroring existing `SetEnrichFn`/`SetFallbackDeps`. Init defers registry-dependent setup. SkillDeps untouched. | ✓ |
| Add Registry to SkillDeps | Extend `skill.SkillDeps` with a Registry field; daemon populates before `InitAll`. Couples a tree-sitter type into the generic skill framework. | |
| Fold registry into FallbackDeps | Reuse existing `SetFallbackDeps` wiring. Smallest change but conflates "fallback path deps" with "core grammar dep". | |

**User's choice:** SetRegistry post-init
**Notes:** Matches existing post-init wiring pattern; keeps `SkillDeps` minimal.

---

## Test Fixtures

| Option | Description | Selected |
|--------|-------------|----------|
| Keep as-is (Recommended) | Tests construct local registries directly. Phase scope is production consolidation. Success criterion (one production call site) is satisfied. | ✓ |
| Add a testutil shared helper | Introduce `treesittertest.NewRegistry()` to centralize. Reduces duplication but expands scope. | |

**User's choice:** Keep as-is
**Notes:** Test fixtures intentionally independent; deferred for future cleanup if needed.

---

## Init Lifecycle

| Option | Description | Selected |
|--------|-------------|----------|
| Lazy in SetRegistry (Recommended) | `Init()` stores cache only; `SetRegistry` builds elider + extractor. Safe because `LazyInitMiddleware` gates first tool call. | ✓ |
| Lazy on first tool use | Defer construction until first `get_repo_map` call, guarded by `sync.Once`. More resilient but adds runtime branching. | |
| Require Registry in Init via SkillDeps | Reverses earlier decision; couples skill framework to tree-sitter. | |

**User's choice:** Lazy in SetRegistry
**Notes:** Daemon post-init wiring guarantees `SetRegistry` runs before any tool can fire.

---

## FallbackDeps Cleanup

| Option | Description | Selected |
|--------|-------------|----------|
| Drop the field (Recommended) | Remove `FallbackDeps.Registry` entirely. Fallback extractor reads registry from RepoMapSkill instead. | ✓ |
| Keep field, pass canonical instance | Daemon populates `FallbackDeps.Registry = grammarRegistry`. Satisfies criterion but leaves redundant plumbing. | |

**User's choice:** Drop the field
**Notes:** Eliminates the second redundant reference and the "same pointer in two places" bookkeeping risk.

---

## Claude's Discretion

- Exact wiring method name (`SetRegistry` vs `SetGrammarRegistry`) — planner's call based on local naming consistency.
- Defensive guard style for `SetRegistry` (nil checks, idempotency).
- Sequencing of `SetRegistry` relative to `SetEnrichFn` / `SetFallbackDeps`.

## Deferred Ideas

- Shared `treesittertest.NewRegistry()` helper.
- Making `NewGrammarRegistry` package-private.
- Pluggable per-workspace grammar lists.

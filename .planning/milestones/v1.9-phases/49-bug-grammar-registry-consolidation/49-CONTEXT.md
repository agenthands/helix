# Phase 49: bug-grammar-registry-consolidation - Context

**Gathered:** 2026-04-25
**Status:** Ready for planning

<domain>
## Phase Boundary

Collapse the three production `treesitter.NewGrammarRegistry()` instantiations into a single canonical registry constructed at daemon bootstrap and shared via dependency injection with all downstream consumers (RepoMapSkill, BodyExtractor, FallbackDeps). Behavior must remain identical; all tree-sitter-backed tests stay green.

**In scope:** Production call sites only — `internal/daemon/daemon.go:196`, `internal/daemon/daemon.go:295`, `internal/skill/repomap/skill.go:84`.
**Out of scope:** Test fixtures' own `NewGrammarRegistry()` calls (intentionally independent), grammar additions/removals, registry API changes beyond what consolidation requires.
</domain>

<decisions>
## Implementation Decisions

### Injection Mechanism
- **D-01:** RepoMapSkill receives the canonical registry via a new `SetRegistry(*treesitter.GrammarRegistry)` post-init wiring method, mirroring the existing `SetEnrichFn` / `SetFallbackDeps` pattern. `skill.SkillDeps` is NOT modified — keeps the generic skill framework free of tree-sitter coupling.
- **D-02:** Daemon bootstrap order: construct `grammarRegistry` once at `daemon.go:196`, run `skill.InitAll(...)`, then call `repomapSkill.SetRegistry(grammarRegistry)` alongside the existing `SetEnrichFn` / `SetFallbackDeps` wiring (step 12b/12c region).

### RepoMapSkill Lifecycle
- **D-03:** `RepoMapSkill.Init()` constructs only the TagCache. Registry-dependent setup (`s.registry`, `s.elider`, `s.extractor`) moves into `SetRegistry()` and runs lazily there. Safe because `LazyInitMiddleware` gates first tool call — no tool can fire before daemon completes post-init wiring.

### FallbackDeps Cleanup
- **D-04:** Remove the `Registry` field from `repomap.FallbackDeps` entirely. The fallback extractor reads the registry from the RepoMapSkill instance instead. Eliminates the second redundant reference and avoids a "same pointer in two places" bookkeeping risk.
- **D-05:** Eliminate the `treesitter.NewGrammarRegistry()` call at `daemon.go:295` — daemon passes only `Extractor` and `AcquireFn` into `SetFallbackDeps`.

### BodyExtractor
- **D-06:** `bodyExtractor` at `daemon.go:197` already takes `grammarRegistry` as an arg — no change needed; it is already DI-wired correctly. Verify only.

### Tests
- **D-07:** All 22+ test-file `NewGrammarRegistry()` call sites stay as-is. They are intentionally independent fixtures, not redundancies. Success criterion 1 ("exactly one `NewGrammarRegistry` call site in production code") is satisfied by changes in production paths only.
- **D-08:** The phase introduces no new testutil package. Test ergonomics are out of scope.

### Constructor Visibility
- **D-09:** `treesitter.NewGrammarRegistry()` stays exported. Test files across multiple packages depend on it; making it package-private would force a testutil layer (rejected per D-08).

### Verification Checks
- **D-10:** After consolidation, planner must include a verification step: `grep -rn "NewGrammarRegistry(" --include="*.go" | grep -v "_test.go"` returns exactly one hit (the daemon bootstrap call).

### Claude's Discretion
- Exact name of the new wiring method (`SetRegistry` vs. `SetGrammarRegistry`) — planner picks based on consistency with surrounding `SetEnrichFn` / `SetFallbackDeps` naming.
- Whether the `SetRegistry` no-op guard (called twice, called with nil) follows the same defensive style as existing setters.
- Sequencing of the `SetRegistry` call relative to `SetEnrichFn` / `SetFallbackDeps` within the daemon post-init block.
</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Roadmap & Requirements
- `.planning/ROADMAP.md` §"Phase 49" — phase goal and 4 success criteria
- `.planning/REQUIREMENTS.md` — BUG-04 ("A single canonical `GrammarRegistry` instance is constructed at daemon bootstrap and shared across repomap, tagcache, and grammar consumers")

### Production Call Sites (the consolidation targets)
- `internal/daemon/daemon.go:196` — canonical instantiation (kept)
- `internal/daemon/daemon.go:295` — `FallbackDeps.Registry` instantiation (deleted per D-05)
- `internal/skill/repomap/skill.go:84` — `RepoMapSkill.Init` instantiation (deleted per D-03)

### Wiring Precedent
- `internal/skill/repomap/skill.go` — existing `SetEnrichFn` (line ~117), `SetFallbackDeps` (line ~125) post-init setters. New `SetRegistry` follows identical pattern.
- `internal/daemon/daemon.go` lines 278-307 (steps 12b, 12c) — daemon post-init wiring block where `SetRegistry` call lands.

### Constructor Definition
- `internal/treesitter/registry.go:50-51` — `NewGrammarRegistry()` constructor (registers Go, Python, TypeScript, TSX, Rust grammars). Stays exported per D-09.

### Skill Framework
- `internal/skill/skill.go:12-19` — `SkillDeps` struct (NOT modified per D-01).

### Project-Level
- `CLAUDE.md` §"Daemon Bootstrap" — describes the `daemon.go` step ordering and post-init wiring philosophy that this phase preserves.
</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `RepoMapSkill.SetEnrichFn` / `SetFallbackDeps` — pattern templates for the new `SetRegistry` setter (mutex-guarded, idempotent, called once during daemon post-init).
- Daemon post-init wiring block (steps 12b/12c) — natural insertion point; no new orchestration scaffolding needed.
- `BodyExtractor` already accepts `*GrammarRegistry` via constructor arg — proves the DI pattern works for kernel consumers.

### Established Patterns
- Post-init wiring via setters (vs. expanding `SkillDeps`) is the project's convention for cross-cutting deps that only some skills need. Confirmed by existing `SetEnrichFn` and `SetFallbackDeps` precedents.
- `LazyInitMiddleware` ensures no MCP tool fires until daemon post-init completes — this is what makes lazy registry-dependent setup in `SetRegistry` safe (D-03).

### Integration Points
- Production: `daemon.New()` is the single entrypoint touched (apart from RepoMapSkill internals).
- `RepoMapSkill` field set: add `registry`, `elider`, `extractor` setup gated on `SetRegistry`. The existing `Cache()` accessor stays; consider symmetric `Registry()` accessor only if the fallback path needs it.
</code_context>

<specifics>
## Specific Ideas

- The success criterion grep is the acceptance test: exactly one production `NewGrammarRegistry(` call after the change.
- Behavioral parity is the bar — no test changes beyond what compilation forces (e.g., if RepoMapSkill tests construct the skill manually, they may need to call the new `SetRegistry`).
</specifics>

<deferred>
## Deferred Ideas

- Shared test helper for `treesittertest.NewRegistry()` — out of scope; revisit if test count grows further or fixtures diverge.
- Making `NewGrammarRegistry` package-private — blocked by cross-package test usage; would require the deferred testutil layer first.
- Pluggable grammar lists per workspace (e.g., language-specific subsets) — not in scope; consolidation preserves the current "all-grammars-at-construction" model.
</deferred>

---

*Phase: 49-bug-grammar-registry-consolidation*
*Context gathered: 2026-04-25*

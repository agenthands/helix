# Phase 46: bug-repomap-lua-fixture - Context

**Gathered:** 2026-04-24
**Status:** Ready for planning

<domain>
## Phase Boundary

Fix `get_repo_map` so that invoking it on Serena's own workspace returns a ranked view of `internal/` Go sources instead of the single deep Lua testdata fixture at `legacy/test/resources/repos/lua/...`. Scope is a bug fix in the repomap pipeline (extraction, ranking, elision, or render) backed by a regression test that prevents silent recurrence.

Not in scope: broader repomap feature work, cross-language scoring overhauls, new `get_repo_map` flags, or changes outside `internal/repomap/` unless RCA forces it.
</domain>

<decisions>
## Implementation Decisions

### Fix Strategy
- **D-01:** Fix the underlying ranking/extraction/elision bug rather than adding path-based heuristics. No hard-coded exclude lists for `testdata/`, `legacy/test/resources/`, `*/test/resources/*`, or `fixtures/`. The invariant we want is: when a polyglot repo contains real source plus testdata fixtures, the ranked output is dominated by the real source *because of the ranking math*, not because paths were filtered out. Path filters are rejected as a fix — they'd mask the underlying bug and regress on repos where testdata-shaped paths contain real code.
- **D-02:** No prior hypothesis locked. Researcher investigates all four candidate root causes on equal footing and follows the evidence:
  1. PageRank starvation (Go tags under-referenced in the graph)
  2. Extractor bias (tree-sitter / LSP `documentSymbol` extractor dropping Go tags or over-weighting Lua)
  3. Workspace root / walk scope (walker treating `legacy/` the same as `internal/`, or misidentifying the root)
  4. Elision / render (token budget fitter picking the single deepest file instead of diversifying across top-ranked files)
  RCA outcome (which of the four was at fault, or a fifth cause) is documented in the phase review per success criterion 3.

### Scope of Fix
- **D-03:** Start narrow — fix the specific symptom. If RCA surfaces a low-risk adjacent bug inside the phase's M sizing budget, planner may propose including it; user reviews the plan before execute. Out-of-budget adjacent issues go to the v1.9 backlog, not this phase.

### Regression Test
- **D-04:** Land BOTH a unit test and an oracle smoke test:
  - **Unit test in `internal/repomap/`** — build a minimal synthetic polyglot fixture tree (Go package with a handful of cross-referenced symbols + a nested Lua testdata subdir containing a deep fixture) and assert the ranked output contains ≥1 Go symbol AND is not dominated by the Lua fixture. This is the invariant guard — fast, hermetic, survives repo churn.
  - **Oracle smoke test in `test/oracle/`** — pin the actual Serena-workspace symptom: calling `get_repo_map` on the Serena repo root must surface at least one `internal/` Go symbol in the top N results. Keep it tolerant enough to survive normal `internal/` churn but strict enough that the Lua-fixture regression would fail it.
- **D-05:** If oracle test proves brittle in practice (frequent unrelated failures as `internal/` evolves), planner may propose downgrading it to a single-shot verification rather than a committed test — but the unit test is non-negotiable.

### Claude's Discretion
- **D-06:** Exact synthetic fixture shape for the unit test (package layout, number of symbols, depth of Lua fixture) — planner/executor chooses whatever best reproduces the rank-dominance behavior reliably.
- **D-07:** Whether RCA documentation lives in a dedicated review section, inline commit message, or a separate note — planner decides based on phase-review conventions already in the repo.
- **D-08:** Whether to refactor any repomap internals touched during the fix — allowed if directly in the fix path, otherwise deferred.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase & Requirements
- `.planning/ROADMAP.md` §"Phase 46: bug-repomap-lua-fixture" — goal, success criteria, notes on orphan directory promotion
- `.planning/REQUIREMENTS.md` — BUG-01 (the requirement this phase closes)

### Repomap Pipeline (primary investigation surface)
- `internal/repomap/extractor.go` — tree-sitter tag extraction; primary candidate for extractor bias
- `internal/repomap/fallback.go` — LSP `documentSymbol` fallback extractor
- `internal/repomap/graph.go` — tag graph construction used by PageRank
- `internal/repomap/pagerank.go` — ranking math; primary candidate for PageRank starvation
- `internal/repomap/elide.go` — scope-aware elision and token-budget fitter; primary candidate for render-time dominance
- `internal/repomap/render.go` — tree renderer
- `internal/repomap/cache.go`, `internal/repomap/tags.go`, `internal/repomap/schema.go` — SQLite tag cache and schema
- `internal/repomap/queries/` — tree-sitter queries per language (including Lua)
- `internal/skill/repomap/` — `get_repo_map` / `get_context` MCP tool wiring
- `internal/repomap/*_test.go` and `internal/repomap/pagerank_bench_test.go` — existing test patterns to mirror

### Test Harness
- `test/oracle/` — integration harness for the oracle smoke test

### Prior Art
- `.planning/phases/999.1-repomap-returns-lua-fixture-instead-of-go-sources/` — orphan backlog directory; currently empty except `.gitkeep`. Planner should promote/rename this directory to `46-bug-repomap-lua-fixture/` per ROADMAP.md notes. No investigation notes to carry over.

### Project-Level
- `CLAUDE.md` §"Code Intelligence Kernel" / §"Fuzzy Editing & RepoMap" — architectural context for the repomap subsystem

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- Full repomap test suite already in `internal/repomap/` — unit test should follow the same pattern (`*_test.go` alongside production file).
- `test/oracle/` already exists and runs against real workspaces — oracle smoke test slots into the existing harness.
- Tree-sitter grammar registry (23 languages including Lua) is shared across the daemon; bug is unlikely to be in grammar registration itself.

### Established Patterns
- Repomap pipeline: walk → extract (tree-sitter or LSP fallback) → cache (SQLite, mtime-invalidated) → graph → PageRank → elide → render. The bug lives somewhere in this pipeline; researcher traces which stage drops/biases Go tags.
- Ranking is path-agnostic by design. Any path-based special-casing would be a new pattern — explicitly rejected (D-01).
- Tag cache is mtime-invalidated and persistent; investigation should account for stale-cache effects (clear the cache when reproducing).

### Integration Points
- `get_repo_map` MCP tool registered via `internal/skill/repomap/` adapter; it's a thin wrapper over the repomap engine, so the fix almost certainly lives in `internal/repomap/` rather than the skill adapter.
- Orphan phase directory at `.planning/phases/999.1-repomap-returns-lua-fixture-instead-of-go-sources/` must be renamed to `46-bug-repomap-lua-fixture/` by the planner (ROADMAP.md directive).

</code_context>

<specifics>
## Specific Ideas

- Reproduction target: running `get_repo_map` on `/Users/Janis_Vizulis/go/src/github.com/postfix/serena` currently surfaces the Lua testdata fixture as the dominant / only output. That's the concrete symptom the regression tests must prevent.
- Success criterion 3 (RCA documented in the phase review) is a hard deliverable, not optional — researcher should plan for the RCA write-up from the start.

</specifics>

<deferred>
## Deferred Ideas

- Path-based testdata exclusion (via an `include_testdata` flag or similar) — explicitly rejected as a fix mechanism (D-01). If users later want to scope `get_repo_map` to exclude testdata-shaped paths for their own reasons, that's a separate feature phase, not a bug fix.
- Broader repomap ranking overhauls or cross-language scoring changes — out of scope; if RCA suggests they're warranted, file as new v1.9 backlog items.
- Cross-file reference capture improvements beyond what's needed to fix this symptom.

</deferred>

---

*Phase: 46-bug-repomap-lua-fixture*
*Context gathered: 2026-04-24*

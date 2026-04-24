# Phase 46: bug-repomap-lua-fixture - Research

**Researched:** 2026-04-24
**Domain:** Serena `internal/repomap/` pipeline — tree-sitter tag extraction, file-graph construction, PageRank ranking, token-budgeted tree rendering.
**Confidence:** HIGH on mechanism identification; MEDIUM on "this is THE single root cause" (four-way investigation is by design).

## Summary

The reported symptom — `get_repo_map` on Serena's repo surfacing the Lua testdata fixture at `legacy/test/resources/repos/lua/test_repo/` as dominant output — is best explained by **name-collision-driven PageRank sink behavior in `internal/repomap/graph.go:BuildGraph`** (HIGH confidence), with a secondary amplifier in the token-budget binary search of `internal/repomap/render.go:RenderBudgeted` (MEDIUM confidence). Extractor-bias and workspace-root hypotheses are largely ruled out on first-pass evidence but still warrant executor-level confirmation before the fix is locked.

The core mechanism: `BuildGraph` indexes definitions and references by **bare symbol name only** (`tag.Name`) — references are not language-, package-, or qualified-name-scoped. When Go code in `internal/` calls methods like `add`, `log`, `new`, `info`, `debug`, `warn`, `error`, `read`, `write`, or references types named `Logger`, and the Lua fixture defines `calculator.add`, `utils.Logger`, `utils.read_file`, etc., the extractor captures the Lua defs as bare names (`add`, `Logger`, `read_file`, ...) via the `dot_index_expression field: (identifier) @name` pattern in `queries/lua_tags.scm:6,14`. Every matching Go ref then creates an edge **Go-file → Lua-file**. With 4 Lua def-files and ~200 Go ref-files, the Lua files accumulate in-degree from the entire Go graph and become classic PageRank sinks.

**Primary recommendation:** Fix the edge-creation logic in `BuildGraph` to reduce cross-language false-positive edges without resorting to path filters. Three fix-shapes are viable (§Fix-Shape Constraints) — prefer qualifying refs (symmetric with qualified defs, minimal math changes) or weighting edges by ambiguity factor (information-theoretic, no per-language special cases). Confirm RCA by executor-run reproduction before committing.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|--------------|----------------|-----------|
| Tag extraction (Go, Lua) | `internal/repomap/extractor.go` + `queries/*.scm` | `internal/repomap/fallback.go` (LSP documentSymbol for languages without grammars) | Tree-sitter is the primary path for both Go and Lua since both have registered grammars. |
| Tag persistence | `internal/repomap/cache.go` (SQLite + mtime) | — | Cache is transparent — bug manifests whether cache is warm or cold, but stale cache can mask reproduction. |
| Workspace walk / root resolution | `internal/skill/repomap/skill.go` (`walkAndExtract`, `resolveRoot`) | — | Walker decides what files get tagged; `skipDirs` is the only existing filter. |
| Graph construction | `internal/repomap/graph.go` (`BuildGraph`) | `EnrichFromLSP` (opportunistic cross-file edges via LSP references) | Bare-name matching between def and ref sets is the primary bug surface. |
| Ranking | `internal/repomap/pagerank.go` (`PageRank`, `RankFiles`) | — | Standard power-iteration PageRank with damping=0.85, uniform teleport for `get_repo_map`. Math is correct; inputs are the problem. |
| Budget-fit rendering | `internal/repomap/render.go` (`RenderBudgeted`, `renderTree`, elided content) | `internal/repomap/elide.go` | Binary search on file count; no diversity constraint → a single top-ranked file can monopolize small budgets. |

## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Fix the underlying ranking/extraction/elision bug — **NO path-based heuristics**. No exclude lists for `testdata/`, `legacy/test/resources/`, `*/test/resources/*`, or `fixtures/`. Path filters are rejected as a fix mechanism.
- **D-02:** No prior hypothesis locked. Researcher investigates all four candidate root causes on equal footing:
  1. PageRank starvation (Go tags under-referenced in the graph)
  2. Extractor bias (tree-sitter / LSP `documentSymbol` dropping Go tags or over-weighting Lua)
  3. Workspace root / walk scope (walker treating `legacy/` the same as `internal/`, or misidentifying the root)
  4. Elision / render (token-budget fitter picking the single deepest file instead of diversifying)
  RCA outcome documented in phase review per success criterion 3.
- **D-03:** Start narrow — fix the specific symptom. Adjacent low-risk cleanup allowed only if it falls inside the phase's M sizing budget; user reviews plan before execute. Out-of-budget issues go to v1.9 backlog.
- **D-04:** Land BOTH:
  - Unit test in `internal/repomap/` with a synthetic polyglot fixture (Go package + nested Lua testdata subdir with deep fixture) asserting the ranked output contains ≥1 Go symbol AND is not dominated by the Lua fixture.
  - Oracle smoke test in `test/oracle/` asserting `get_repo_map` on Serena repo root surfaces at least one `internal/` Go symbol in the top-N.
- **D-05:** If the oracle test proves brittle in practice, planner may propose downgrading it to a single-shot verification — but the unit test is **non-negotiable**.

### Claude's Discretion
- **D-06:** Exact synthetic fixture shape for the unit test (package layout, symbol count, depth of Lua fixture) — planner/executor chooses whatever best reproduces the rank-dominance behavior reliably.
- **D-07:** Whether RCA documentation lives in a dedicated review section, inline commit message, or a separate note — planner decides based on existing phase-review conventions.
- **D-08:** Whether to refactor any repomap internals touched during the fix — allowed if directly in the fix path, otherwise deferred.

### Deferred Ideas (OUT OF SCOPE)
- Path-based testdata exclusion via an `include_testdata` flag or similar — explicitly rejected as a fix mechanism.
- Broader repomap ranking overhauls or cross-language scoring changes — out of scope. If RCA suggests warranted, file as new v1.9 backlog items.
- Cross-file reference capture improvements beyond what's needed to fix this symptom.

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| BUG-01 | `get_repo_map` returns Go sources (not the Lua fixture) when invoked on a Go workspace that contains polyglot testdata fixtures — closes backlog Phase 999.1 | §Root-Cause Investigation identifies the bare-name edge mechanism in `graph.go:BuildGraph`; §Fix-Shape Constraints enumerates viable interventions; §Validation Architecture gives the unit + oracle test plan. |

## Project Constraints (from CLAUDE.md)

- **Always run `go vet ./...` and `go test ./...` before completing any Go task.** — this is a success criterion for this phase.
- **Single Go binary, no CGO hot path.** Fixes must not introduce CGO or non-Go dependencies. The existing `go-tree-sitter` dependency is already CGO; do not deepen it.
- **Format with `gofmt -w .` and lint with `go vet`.**
- **Tree-sitter grammar registry is shared.** Do not create a new grammar registry instance for this fix — Phase 49 (BUG-04) is consolidating registries, and this phase should not regress that effort.
- **Project-level skills in `.claude/skills/` or `.agents/skills/`** — neither directory exists in this repo. No project skills to apply.
- **GSD workflow:** all edits flow through a GSD command; this phase runs via `/gsd-plan-phase 46`.

## Root-Cause Investigation (Four Candidates on Equal Footing)

Per D-02, the researcher investigates all four candidates and documents evidence for/against each.

### Candidate 1 — PageRank starvation (Go tags under-referenced)  [CONFIDENCE: HIGH this is the primary mechanism, but not via "starvation"]

**Hypothesis reframed:** Not that Go is under-referenced, but that **Lua is over-referenced relative to what should connect it to the Go ref pool**. The effect on PageRank is identical: Go files sit at the base of the rank distribution while Lua files sit at the top.

**Evidence from code (file:line):**
- `internal/repomap/graph.go:82-95` builds `defs: name → []file` and `refs: name → file → count` maps keyed **only on `tag.Name`**. No language, file-extension, or qualified-name scope is applied.
- `internal/repomap/graph.go:100-118` creates an edge `refFile → defFile` for every (name, refFile, defFile) tuple where the name appears as both def and ref. `math.Sqrt(refCount)` is the edge weight.
- `internal/repomap/extractor.go:204-207` calls `buildQualifiedName` **only for definitions** (capture names starting with `definition.`). References remain bare (`add`, not `calculator.add`).
- `internal/repomap/queries/lua_tags.scm:5,14` extracts `calculator.add`-style function defs as the bare `add` via the `dot_index_expression field: (identifier) @name` capture. Same for `utils.trim`, `utils.Logger`, etc. The container name (`calculator`, `utils`) is discarded.
- `internal/repomap/queries/go_tags.scm:12-16` extracts Go call-site names (`selector_expression field: (field_identifier) @name`) — again, bare (e.g. `add`, not `x.add`).

**Collision surface (measured):** In the 4 Lua files, definition names include `add`, `subtract`, `multiply`, `divide`, `power`, `factorial`, `mean`, `median`, `trim`, `split`, `starts_with`, `ends_with`, `deep_copy`, `table_contains`, `table_merge`, `read_file`, `write_file`, `Logger`, `new`, `log`, `debug`, `info`, `warn`, `error`, `set_level`. A grep pass over `internal/` found 27 matching call-sites for the sample names `new|add|log|read|write|trim|split|info|debug|warn|error` — and this is a lower bound (Go qualifies as `x.log(...)` → ref name `log`, which hits regardless of receiver type).

**Graph topology consequence:** With 4 Lua def-files and ≈200 Go ref-files, each Lua def acquires dozens of inbound Go→Lua edges. PageRank with damping 0.85 propagates 85% of each Go file's rank mass along outbound edges; with Lua files as the outbound targets and no outbound edges leaving Lua back to Go (Lua's `require("src.calculator")` does not produce cross-language ref tags that land back in Go), the Lua files are **rank sinks**. Dangling-node redistribution (pagerank.go:106-114) applies when Lua has NO outbound edges — but Lua DOES have outbound edges (`main.lua` refs `calculator.add`, `utils.trim`, etc., which point to the other Lua files). So: **Lua is a closed subgraph receiving rank from Go but not returning it.** Pure sink behavior.

**Confirmation by executor (Step 1 of reproduction below):** rank-sort all files and observe top-N should be dominated by Lua; log the in-degree and out-degree per file; observe Go files with out-degree >> 0 (they emit refs to Lua) and Lua files with in-degree >> out-degree.

### Candidate 2 — Extractor bias  [CONFIDENCE: LOW this is the primary cause, but cannot be ruled out without reproduction]

**Hypothesis:** Tree-sitter's Go tag query drops Go def tags, or the Lua query over-extracts defs.

**Evidence against:**
- `internal/repomap/queries/go_tags.scm` is a standard tag query (matches `function_declaration`, `method_declaration`, `type_spec`) and its tests in `internal/repomap/extractor_test.go` cover these. An executor can verify by extracting tags on a single `internal/` Go file and confirming expected def/ref counts.
- `NewTagExtractor` (`extractor.go:122-134`) is error-tolerant: a failed query compilation logs a warning but does not disable other languages. The daemon startup log for the installed Serena daemon would show a warning if Go's query failed — executor should check daemon logs to rule this out.

**Evidence for (weak):**
- If the Go grammar version is ahead of the query and `field_identifier` node kind has shifted, call-site ref extraction could silently degrade. Unlikely but non-zero.

**Confirmation:** Run `TestTagExtractor_*` tests with `-v`. Compare def/ref counts between a representative Go file and a representative Lua file normalized by LOC.

### Candidate 3 — Workspace root / walk scope  [CONFIDENCE: LOW this is the root cause, but a secondary amplifier exists]

**Hypothesis:** The walker is not treating `internal/` as part of the workspace, or it is misidentifying the root, or `legacy/` is being given undue weight.

**Evidence against root misidentification:**
- `resolveRoot` (`skill.go:438-447`) falls back to `os.Getwd()`. When invoked on `/Users/Janis_Vizulis/go/src/github.com/postfix/serena`, it returns that path. `walkAndExtract` (`skill.go:341-404`) recursively walks from that root; `internal/` is a direct child and is NOT in `skipDirs` (`skill.go:308-311` lists `.git`, `node_modules`, `__pycache__`, `.serena`, `vendor`, `.venv`, `dist`, `build`). So `internal/` is walked and its ~217 Go files are tagged.
- `legacy/` is also walked. That's the symptom's precondition, not a cause.

**Evidence for a secondary amplifier (`.claude/worktrees/`):**
- Filesystem enumeration shows **13 worktree copies** of the Lua fixture under `.claude/worktrees/agent-*/legacy/test/resources/repos/lua/` = 13 × 4 = **52 additional Lua files**. None of these are excluded by `skipDirs`.
- This inflates the Lua def-name count and the sink attractor strength. A principled fix for this candidate (not a path filter) is harder — the walker conceptually shouldn't descend into other git worktrees (each worktree is a separate repo). But touching the walker to add `.claude/worktrees/` is a near-path-filter and may conflict with D-01's intent. **Recommendation:** treat this as out-of-scope for the fix; the graph-math fix should make this moot. If executor finds the rank-dominance persists even with worktrees excluded from consideration, promote this to in-scope and discuss with user.

**Confirmation:** Log the workspace root and the list of `.lua` files tagged before `BuildGraph`. Observe 4 + 13×4 = 56 Lua file tags in cache (or only 4 if worktrees are coincidentally skipped by some other means).

### Candidate 4 — Elision / render (token budget fitter)  [CONFIDENCE: MEDIUM as a compounding amplifier, LOW as the primary cause]

**Hypothesis:** Tag extraction and ranking are fine, but `RenderBudgeted` picks the single deepest file instead of diversifying across top-ranked files.

**Evidence from code:**
- `render.go:41-77` performs binary search on `mid = (lower+upper)/2` over the sorted `ranked []RankedFile`, calling `renderTree(ranked[:mid])`. Budget is 4096 tokens by default; min budget is 64. The binary search picks the `mid` that best fits budget — it does NOT enforce diversity (e.g., no constraint like "at least one file from each of the top-3 directories").
- If ranked[0] is a Lua file that, once fully elided, is ≈4000 tokens on its own, then `mid=1` fits but `mid=2` exceeds budget → final output is just the single top-ranked file. That is precisely the "single deep Lua fixture" symptom.
- Lua `main.lua` is 114 LOC with many function bodies; elided content is ~1000-2000 chars ≈ 250-500 tokens per file. A small default budget (4096) could still fit several top-ranked files — but if rank ordering puts 4 Lua files at positions [0..3] before any Go file appears, even a multi-file budget fit only shows Lua.

**Evidence against this being the primary cause:**
- If ranking is fixed (Go files at top), the render loop naturally diversifies because the binary search picks larger N when budget allows.
- Render diversity has architectural merit as a hardening measure (defensive layer) but is orthogonal to the bug. Putting the fix here would only mask a ranking bug.

**Recommendation:** Do not fix in render alone. Consider a diversity hardening pass (e.g., "ensure top-N includes at least one file per top-K directories") as a Phase 46 stretch goal **only** if the ranking fix is simple and budget permits. Otherwise defer to a future repomap-polish phase.

## Reproduction Plan (Executor Runbook)

Before locking RCA, the executor MUST reproduce the symptom locally and capture evidence. Steps below assume running from `/Users/Janis_Vizulis/go/src/github.com/postfix/serena`.

1. **Clear stale tag caches** (mandatory — mtime invalidation is per-file and may hide fix effects):
   ```bash
   rm -f /Users/Janis_Vizulis/go/src/github.com/postfix/serena/index/tags.db*
   rm -f /Users/Janis_Vizulis/.serena/index/tags.db*
   find /Users/Janis_Vizulis/go/src/github.com/postfix/serena -name 'tags.db*' -delete
   ```
2. **Run a harness test** that calls `RepoMapSkill.execGetRepoMap` on the Serena root. Easiest: write a temporary test in `internal/skill/repomap/` that sets `s.rootDir = /Users/.../serena`, calls `ensureGraph` then `RankFiles(0.85, nil)`, and prints the top-20 files. Assert no `internal/` Go file appears in the top-5.
3. **Capture graph statistics:**
   - Total nodes: should be >200.
   - Total edges: should be in the thousands.
   - For each of the 4 canonical Lua files: in-degree, out-degree, PageRank score.
   - For representative Go files (`internal/daemon/daemon.go`, `internal/mcp/middleware.go`, `internal/kernel/lspool/pool.go`): in-degree, out-degree, PageRank score.
4. **Inspect the cross-language edge set:** dump all edges where source has extension `.go` and destination has extension `.lua`. Count = primary evidence for Candidate 1.
5. **Confirm walker did not skip `internal/`:** log the number of `.go` files tagged vs. `.lua` files tagged. Expected: ~217 Go files, 4-56 Lua files (depending on worktree inclusion).

Record outputs in the phase review (success criterion 3 — RCA documentation).

## Fix-Shape Constraints (viable shapes given D-01)

Path-based filters are forbidden. For each candidate, the shapes of fix that remain:

### For Candidate 1 (PageRank via bare-name edges) — PRIMARY

- **F1-A: Qualify references symmetrically with defs.** Change `extractor.go:buildQualifiedName` to also run on `reference.*` capture names. Go call `x.add(...)` becomes ref `?.add` or — if receiver type is locally known — `ReceiverType.add`. The graph match is then `Type.method → Type.method`. This is the most principled and path-agnostic fix. Downside: ref qualification requires type inference, which tree-sitter alone cannot provide for Go receivers; the fallback would be "qualify only with receiver IDENTIFIER" (e.g. `x.add`), which gives weaker matching but zero LSP dependency. Evaluate carefully.
- **F1-B: Weight edges by ambiguity.** In `BuildGraph`, count `degree(name) = len(defs[name])` across languages. Use `weight = sqrt(refCount) / sqrt(1 + degree(name))` — a name that appears as a def in many files weighs less per-edge. This dampens common-name collisions (`log`, `add`, `new`) naturally. Symmetric, purely information-theoretic, no language-specific logic.
- **F1-C: Language-scoped edges.** Only create an edge if `LangFromExt(refFile) == LangFromExt(defFile)`. Simple, effective, but semantically wrong for true cross-language projects (bindings, FFI). **Recommend against** unless F1-A/F1-B prove intractable.
- **F1-D: Lower damping for high-in-degree sinks.** Detect nodes with in-degree > k·avg and apply a per-node rank penalty. Hacky; treats symptom not cause. **Recommend against.**

**Recommended sequence:** Try F1-B first (smallest diff, language-agnostic). If Lua still dominates, combine with F1-A (ref-qualification for Go specifically).

### For Candidate 2 (extractor bias)
- **F2-A:** Fix the offending query or adjust the extractor. Low probability; only act if reproduction shows it.

### For Candidate 3 (walker / workspace scope)
- **F3-A:** If the root cause involves `.claude/worktrees/` inflation, add a walker guard that detects git worktree roots (presence of `.git` **file**, not directory) and treats them as separate repos. This is NOT a path filter — it's a workspace-boundary heuristic grounded in git's actual topology. **Requires user sign-off** before including since it expands beyond pure repomap internals.

### For Candidate 4 (render)
- **F4-A:** Add a diversity constraint in `RenderBudgeted` — e.g., require that the first N files span at least K distinct top-level directories before the binary search kicks in. Defensive hardening only; should NOT be the sole fix.

## Synthetic Fixture Design (for D-04 unit test)

The unit test must reliably reproduce rank-dominance with a minimal, hermetic fixture. Proposed shape:

```
synthetic-polyglot/
├── pkg/
│   ├── server.go            // package pkg; defines Server, Start, Stop, Log, Add; refs logger.Log, store.Add
│   ├── store.go             // package pkg; defines Store, Add, Get, Delete; refs logger.Log
│   ├── logger.go            // package pkg; defines Logger, Log, Info, Debug, Warn, Error; refs nothing cross-file
│   ├── handler.go           // package pkg; defines Handler, Handle; refs server.Start, store.Add, logger.Log
│   └── util.go              // package pkg; defines Trim, Split; refs logger.Log
└── testdata/
    └── fixtures/
        └── lua/
            └── deep/
                └── nested/
                    └── fixture/
                        ├── main.lua         // defines main; refs calc.add, utils.log, etc.
                        ├── calculator.lua   // defines calculator.add, subtract, multiply (bare names: add, subtract, multiply)
                        └── utils.lua        // defines utils.log, trim, split, new, Logger (bare names: log, trim, split, new, Logger)
```

**Why this shape reliably reproduces:**
- 5 Go files with rich internal cross-file refs (high Go↔Go edge density → rank mass should stay in Go subgraph).
- Bare Lua names (`add`, `log`, `trim`, `split`, `Logger`, `new`) collide with Go defs and refs — the same collision pattern as the real Serena repo.
- Deep nesting in the Lua path mirrors the real symptom's depth.
- Lua files form a closed subgraph (main.lua → calculator.lua + utils.lua; no edges back to `pkg/`), so they become sinks.
- Small enough to keep the test fast (<1s extraction + graph build).

**Assertions (D-04):**
- `len(ranked) >= 1` and the top-1 file MUST have a `.go` extension under `pkg/` (strong form).
- OR — softer — `ranked[0..4]` MUST contain at least one `.go` file from `pkg/` (allows some Lua interleaving if fix is partial).
- The rendered output of `RenderBudgeted(ranked, 2048)` MUST NOT be 100% Lua content (guards the render amplifier).

**Negative control:** Before applying the fix, assert the test **fails** (i.e., top-1 is a Lua file). This confirms the fixture actually reproduces the bug. Without the negative control, a passing test proves nothing.

## Landmines

- **Tag cache masking:** `TagCache.GetOrExtract` uses file mtime as the invalidation key. Changing extractor behavior does NOT invalidate cached tags for unchanged files. Every reproduction run and every `go test ./...` run after touching extractor.go or queries/*.scm needs either: a `t.TempDir()`-scoped cache (existing tests do this, `cache_test.go:newTestCache`), OR `cache.Clear()` at test start, OR manual `rm` of the on-disk cache. **The oracle test in `test/oracle/scenario/repomap_test.go` already uses `harness.PrepareFixture` which creates fresh temp dirs — good.** A new oracle test running against the real Serena repo root MUST clear or bypass the persistent cache.
- **`.claude/worktrees/` inflation:** 13 agent worktrees each contain a full copy of `legacy/`. Walker does not skip them. Multiplies Lua sink strength by ~13×. Tests that reproduce the symptom on the REAL repo will see different numbers depending on how many worktrees exist at test time — this makes oracle tests brittle (D-05 anticipates this). The synthetic fixture must stand alone.
- **`go-tree-sitter` query compilation quirks:** `NewTagExtractor` (extractor.go:122-134) swallows per-language query compile errors with a log warning. A silent Go-query compilation failure would manifest as "Go produces zero tags," which looks like Candidate 2 (extractor bias). Always confirm Go extraction is healthy (count of non-zero Go tags) before accepting any RCA.
- **Qualified name asymmetry between defs and refs:** `buildQualifiedName` (extractor.go:235-261) runs only for definitions. Ref-qualification is deliberately absent. Any fix that qualifies refs must also consider: do existing unit tests assert bare ref names? (Yes — `graph_test.go:TestBuildGraph_CrossFileEdges` creates refs with bare names like `Foo`.) Such tests may need updating if ref names become qualified.
- **Canonical `GrammarRegistry` consolidation (BUG-04 / Phase 49):** `RepoMapSkill.Init` currently creates its own `GrammarRegistry` instance (skill.go:84). Phase 49 will consolidate this to a single bootstrap instance. Changes to grammar handling in Phase 46 should use the registry passed in via `SkillDeps` where possible, or at minimum not deepen the instance-divergence. Check in with Phase 49 planner before introducing new registry creation sites.
- **LSP enrichment is opportunistic (`EnrichFromLSP`, graph.go:126-137):** when `enrichFn` is wired (daemon-driven), Go files get additional cross-file ref edges from LSP `references` results. In bare `go test` runs and in oracle tests with `SkipLS: true`, enrichment is NOT active, so the graph is purely tree-sitter-derived. This means the symptom is REPRODUCIBLE without LSP — which is good, but also means a fix that relies on LSP enrichment would not close the bug in LSP-less environments. Keep the fix tree-sitter-pure.

## Validation Architecture

Nyquist validation is enabled (config.json `workflow.nyquist_validation: true`). This section drives `46-VALIDATION.md`.

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go `testing` + `github.com/stretchr/testify/{assert,require}`; integration via existing `test/harness` (build tag `integration`) |
| Config file | None — standard Go test discovery |
| Quick run command | `go test ./internal/repomap/... ./internal/skill/repomap/...` |
| Full suite command | `go test ./...` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| BUG-01 | On a synthetic polyglot workspace (Go pkg + deeply nested Lua testdata fixture), `RankFiles` returns at least one `.go` file in the top-1 and is not dominated by Lua. | unit | `go test ./internal/repomap/ -run TestRepomap_PolyglotRanking -v` | ❌ Wave 0 — new file `internal/repomap/polyglot_rank_test.go` |
| BUG-01 | Rendered `RenderBudgeted` output for the same synthetic fixture contains at least one Go symbol from `pkg/` and is not 100% Lua content. | unit | `go test ./internal/repomap/ -run TestRepomap_PolyglotRender -v` | ❌ Wave 0 — same new file |
| BUG-01 (success criterion 1) | Invoking `get_repo_map` on the real Serena workspace surfaces an `internal/` Go symbol in the top-N output. | integration (oracle) | `go test -tags=integration ./test/oracle/scenario/ -run TestScenario_SerenaRepoMap -v` | ❌ Wave 0 — new file `test/oracle/scenario/serena_repomap_test.go` |
| BUG-01 (regression guard) | Phase-review RCA: tests must fail against the unfixed code and pass after the fix. | manual + CI | captured in phase review; CI validates via the two tests above | ❌ Wave 0 — phase-review entry |
| Project-level (success criterion 4) | `go test ./...` and `go vet ./...` pass cleanly. | smoke | `go vet ./... && go test ./...` | ✅ existing |

### Sampling Rate
- **Per task commit:** `go test ./internal/repomap/... ./internal/skill/repomap/...` (fast, <5s).
- **Per wave merge:** `go test ./... && go vet ./...` (full suite).
- **Phase gate:** Full suite green, integration tag run green (`go test -tags=integration ./test/oracle/...`), and the oracle serena-root test green before `/gsd-verify-work`.

### Wave 0 Gaps
- [ ] `internal/repomap/polyglot_rank_test.go` — unit test + synthetic polyglot fixture setup (in-test `t.TempDir()` tree OR under `internal/repomap/testdata/polyglot_rank/`). Must include a **negative control** run against the pre-fix behavior to prove the fixture reproduces the symptom.
- [ ] `test/oracle/scenario/serena_repomap_test.go` — oracle smoke test against `runtime.ProjectRoot()`; must clear or bypass persistent tag cache before running. Asserts top-N contains ≥1 file with path under `internal/` and `.go` extension. Per D-05, planner may downgrade to a single-shot verification if brittleness is observed.
- [ ] Phase review entry capturing RCA (which of the 4 candidates was at fault, and the concrete mechanism) — location decided by planner per D-07.

## Code Examples

### Reading graph topology (for executor reproduction step)

```go
// Pseudo-code — place in a throwaway test in internal/skill/repomap/
func TestRepro_SerenaRanking(t *testing.T) {
    t.Skip("repro only; remove before merging")

    s := &RepoMapSkill{}
    require.NoError(t, s.Init(skill.SkillDeps{
        ProjectDir: t.TempDir(), // fresh cache dir
        Logger:     slog.Default(),
    }))
    s.SetWorkspaceRoot("/Users/Janis_Vizulis/go/src/github.com/postfix/serena")

    require.NoError(t, s.ensureGraph())

    // Top-20 files
    ranked := s.graph.RankFiles(0.85, nil)
    for i, rf := range ranked[:20] {
        in := s.graph.Edges[rf.Path] // OUTgoing from rf.Path
        t.Logf("%2d  score=%.6f  out=%d  %s", i, rf.Score, len(in), rf.Path)
    }

    // Cross-language edge count
    goToLua := 0
    for src, targets := range s.graph.Edges {
        if filepath.Ext(src) != ".go" { continue }
        for dst := range targets {
            if filepath.Ext(dst) == ".lua" { goToLua++ }
        }
    }
    t.Logf("Go→Lua edges: %d", goToLua)
}
```
[VERIFIED: code walked via Read tool on internal/repomap/*.go and internal/skill/repomap/skill.go on 2026-04-24]

### Proposed F1-B fix shape (ambiguity-weighted edges)

```go
// internal/repomap/graph.go — BuildGraph inner loop
// ... after defs and refs maps are built ...

// Count how many files define this name (ambiguity signal).
defDegree := make(map[string]int, len(defs))
for name, files := range defs {
    defDegree[name] = len(files)
}

g := NewFileGraph()
for ident, definers := range defs {
    ambiguityScale := 1.0 / math.Sqrt(1.0 + float64(defDegree[ident]))
    refFiles, hasRefs := refs[ident]
    if !hasRefs {
        for _, defFile := range definers {
            g.addEdge(defFile, defFile, 0.1)
        }
        continue
    }
    for refFile, count := range refFiles {
        for _, defFile := range definers {
            if refFile == defFile { continue }
            g.addEdge(refFile, defFile, math.Sqrt(float64(count))*ambiguityScale)
        }
    }
}
```
[ASSUMED: this diff shape is viable; executor must validate it improves top-N without regressing existing `graph_test.go` tests, which may hard-code edge weights.]

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Cross-language edge creation is the dominant mechanism producing the symptom, not a tree-sitter extractor bug. | Candidate 1 | If extractor is actually broken, reproduction will surface it and fix-shape F1-B alone won't close the bug. Low risk — reproduction catches this. |
| A2 | F1-B (ambiguity-weighted edges) will close the symptom on the synthetic fixture. | Fix-Shape Constraints | May require F1-B + F1-A combined, or tuning of the ambiguity scale. Medium risk — executor iterates. |
| A3 | Existing tests in `graph_test.go` do not hard-code edge weights in a way that F1-B breaks. | Fix-Shape Constraints | If they do, updating them is allowed under D-08 provided the semantic assertions still hold. |
| A4 | `.claude/worktrees/` inflation is secondary. Fixing it is out of scope. | Candidate 3 | If reproduction shows the bug disappears entirely after skipping worktrees (a near-path-filter), the user may reconsider D-01. Researcher flags this explicitly for user review at plan time. |
| A5 | The oracle smoke test can bypass the persistent tag cache by using a throwaway `ProjectDir` in `SkillDeps`. | Validation Architecture | If cache path is wired elsewhere (e.g., user home dir), oracle will need explicit cache clearing. Executor verifies. |
| A6 | Go 1.x toolchain + existing `go-tree-sitter` deps compile cleanly on the Mac dev machine where reproduction runs. | Reproduction Plan | Phase 50 is fixing Go 1.25 + gopls on Linux CI, but local reproduction is on darwin/arm64 which is known-green today. Low risk. |

## Open Questions

1. **Does ref-qualification (F1-A) require LSP for Go receiver type inference?**
   - What we know: `extractor.go:qualifyGoMethod` walks the tree-sitter AST to get receiver type for defs (e.g. `func (s *Server) Run` → `Server.Run`). For refs, the call site is `s.Run(...)` — tree-sitter knows the IDENTIFIER `s` but NOT its type without semantic analysis.
   - What's unclear: whether qualifying refs as `s.Run` (identifier-based, no type inference) is a useful signal or just noise.
   - Recommendation: prefer F1-B (no qualification needed) first. If F1-A is needed, start with identifier-only qualification; LSP-backed type inference is a separate enhancement.

2. **Is the persistent tag cache a source of reproducibility flakiness in oracle tests?**
   - What we know: tag cache at `$ProjectDir/index/tags.db` is mtime-invalidated per file. Oracle test uses `harness.PrepareFixture` which creates fresh temp dirs — but a new `TestScenario_SerenaRepoMap` pointing at the real Serena repo root would share the on-disk cache.
   - What's unclear: whether the planner/executor can safely use `t.TempDir()` as the `ProjectDir` (where tags.db lives) while still walking the real Serena workspace. Reading `RepoMapSkill.Init` and `NewTagCache`, this looks feasible — `ProjectDir` only controls where `index/tags.db` lives, orthogonal to `SetWorkspaceRoot`.
   - Recommendation: document this decoupling in the test.

3. **Does fixing the graph with F1-B regress any existing unit tests?**
   - What we know: `graph_test.go:TestBuildGraph_CrossFileEdges` asserts `weight = sqrt(1) = 1.0` for a single-ref, single-def case. With F1-B, the weight becomes `sqrt(1) * 1/sqrt(2) ≈ 0.707`.
   - What's unclear: how many tests hard-code exact edge weights.
   - Recommendation: executor runs existing tests pre-fix, audits the failures, updates weight constants symmetrically (the relative ordering of ranks is what matters for correctness).

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | building & testing | ✓ (assumed — repo is active) | — | — |
| go-tree-sitter + Go/Lua grammars | tag extraction | ✓ (vendored Go module deps, already in use) | — | — |
| `gopls` | oracle LSP enrichment (optional) | not required for this phase's tests | — | `SkipLS: true` in harness.RunnerOptions (already used in existing oracle repomap tests) |
| SQLite (modernc) | tag cache | ✓ (pure-Go, no CGO) | — | — |

**Missing dependencies with no fallback:** none — this phase is fully doable with existing tooling.

**Missing dependencies with fallback:** the oracle smoke test does NOT require a live LS; follow the existing pattern of `SkipLS: true`.

## Security Domain

`security_enforcement` is not explicitly disabled in config.json. However, this phase is a pure bugfix in ranking math + test additions. Security surface is minimal; the only pre-existing guards touched are already sound:

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | — |
| V3 Session Management | no | — |
| V4 Access Control | no | — |
| V5 Input Validation | yes (lightly) | `get_context` already validates path traversal (`skill.go:266-273`) — must NOT regress. |
| V6 Cryptography | no | — |

### Known Threat Patterns for This Change

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Path traversal via graph / render inputs | Tampering | Existing `get_context` check; ranking fix does not take new external inputs. |
| SQL injection in tag cache | Tampering | Existing parameterized queries (`cache.go:71-75,110,145,174-177,200-223`); fix does not touch SQL. |
| Denial via unbounded PageRank iterations | Availability | Existing `maxIter` cap (pagerank.go:104); fix does not remove it. |

No new threat vectors introduced by the recommended fix shapes.

## Sources

### Primary (HIGH confidence)
- Serena codebase (walked via Read tool 2026-04-24):
  - `internal/repomap/graph.go` — confirmed bare-name edge mechanism
  - `internal/repomap/pagerank.go` — confirmed power iteration + damping + dangling redistribution
  - `internal/repomap/render.go` — confirmed binary-search budget fit with no diversity constraint
  - `internal/repomap/extractor.go` — confirmed def-only qualification asymmetry
  - `internal/repomap/queries/{go,lua}_tags.scm` — confirmed bare-name capture patterns
  - `internal/skill/repomap/skill.go` — confirmed walker `skipDirs` list, lazy cache population, post-init wiring
  - `internal/repomap/cache.go` — confirmed mtime-based invalidation
  - `test/oracle/scenario/repomap_test.go` + `test/harness/fixture.go` — confirmed existing oracle test patterns and `PrepareFixture`
  - `testdata/fixtures/polyglot/` — confirmed existing polyglot fixture shape (not reused for this phase because it has no Lua and no deep nesting)
- Phase inputs (authoritative):
  - `.planning/phases/46-bug-repomap-lua-fixture/46-CONTEXT.md` (D-01..D-08)
  - `.planning/phases/46-bug-repomap-lua-fixture/46-DISCUSSION-LOG.md`
  - `.planning/REQUIREMENTS.md` (BUG-01)
  - `.planning/ROADMAP.md` Phase 46
  - `./CLAUDE.md` (project-level invariants)

### Secondary (MEDIUM confidence)
- PageRank sink behavior and damping-factor dynamics — standard graph-theoretic result, applied here to the observed Go↔Lua edge topology.
- aider's `repomap` is the cited inspiration for this pipeline (comments in `graph.go:74-121`, `render.go:49-50`). Its `refine_refs` step in the Python implementation qualifies references with more context than this Go port does — that informs fix-shape F1-A. [CITED: aider project architecture, general knowledge — not re-fetched this session]

### Tertiary (LOW confidence)
- Exact Go-ref / Lua-def collision count in the live repo (estimated from a single grep of 11 common names; full enumeration deferred to executor reproduction).

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new deps, all identified components exist in the repo.
- Architecture (4-candidate investigation): HIGH — direct code-walk evidence for each.
- Primary mechanism identification: HIGH — bare-name edge creation is directly readable in `graph.go:82-118`.
- Fix-shape viability: MEDIUM — F1-B is mathematically principled; empirical validation pending executor run.
- Synthetic fixture design: MEDIUM-HIGH — mirrors the real symptom precisely; negative control will confirm.

**Research date:** 2026-04-24
**Valid until:** 2026-05-24 (30 days — repomap code is stable in v1.8; Phase 49 registry consolidation is the only upcoming structural change in `internal/repomap/`'s neighborhood.)

## RESEARCH COMPLETE

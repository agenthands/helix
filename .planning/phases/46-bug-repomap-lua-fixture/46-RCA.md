# Phase 46 — Root Cause Analysis (BUG-01)

**Phase:** 46-bug-repomap-lua-fixture
**Requirement:** BUG-01
**Filed:** 2026-04-24
**Status:** Resolved

## Symptom

`get_repo_map` invoked on `/Users/Janis_Vizulis/go/src/github.com/postfix/serena`
returned a ranked view dominated by
`legacy/test/resources/repos/lua/.../main.lua` and sibling Lua fixture
files instead of Go sources under `internal/`. The user-visible output
surfaced the deepest Lua testdata file at top-1, with the entire Go
subsystem pushed out of the token-budgeted render.

## Reproduction

### Unit reproduction (synthetic polyglot fixture)

`internal/repomap/polyglot_rank_test.go` — a minimal synthetic polyglot
tree built from five Go files under `pkg/` with cross-file qualified
refs plus three deeply-nested Lua files under
`testdata/fixtures/lua/deep/nested/fixture/` whose bare-name defs
(`add`, `log`, `trim`, `Logger`, `new`) collide with the bare names
Go ref call-sites emit pre-fix. Before the fix, `FileGraph.RankFiles`
placed a `.lua` file at top-1. After the fix, a `.go` file under
`pkg/` is top-1 and the top-3 contains at least one Go file.

### Oracle reproduction (full MCP-tool boundary)

`test/oracle/scenario/repomap_polyglot_test.go` — invokes `get_repo_map`
via the in-process MCP client on `testdata/fixtures/polyglot_lua/` and
asserts the rendered output references at least one Go file under
`pkg/`, contains at least one Go symbol, and is not monopolized by Lua.

### Negative-control failure output (captured in Plan 01 SUMMARY)

```
=== RUN   TestRepomap_PolyglotRanking
    polyglot_rank_test.go:31:
        Error: Not equal:
          expected: ".go"
          actual:   ".lua"
        Messages: top-ranked file must be Go, got
                  .../testdata/fixtures/lua/deep/nested/fixture/utils.lua
--- FAIL: TestRepomap_PolyglotRanking (0.01s)
```

Pre-fix rank order (from Plan 01 debug harness):

```
[0] 0.553324  .../fixture/utils.lua
[1] 0.324657  .../fixture/calculator.lua
[2] 0.023325  pkg/logger.go
[3] 0.020833  pkg/store.go
[4] 0.019802  pkg/util.go
[5] 0.019698  pkg/server.go
[6] 0.019384  pkg/handler.go
[7] 0.018977  .../fixture/main.lua
```

## Four Candidates Evaluated (D-02)

Per CONTEXT.md D-02, all four candidate causes were investigated on
equal footing. Verdicts below reflect evidence from Plans 01 and 02.

| # | Candidate | Verdict | Evidence |
|---|-----------|---------|----------|
| 1 | PageRank starvation / bare-name cross-language edges | **CONFIRMED primary** (co-equal with Candidate 2) | `internal/repomap/graph.go:BuildGraph` indexed both `defs[name]` and `refs[name]` on bare identifier with no language, package, or qualified-name scoping. Every Go bare ref that collided with a Lua bare def produced a cross-language edge into the Lua subgraph. With many Go ref files and few Lua def files, the Lua subgraph formed a closed PageRank sink. Synthetic fixture (Plan 01) reproduced the symptom; the combined F1-B + F1-A fix (Plan 02) closed it. |
| 2 | Extractor bias — asymmetric qualification of Go defs vs refs | **CONFIRMED primary** (co-equal with Candidate 1) | `extractor.go:buildQualifiedName` ran only on definition captures. Go defs became qualified (`pkg.Store`, `Store.Add`, `Logger.Log`); Go call-site refs (`s.Add(...)`) emitted bare `Add`. The asymmetry itself is the mechanism that forces Go bare refs to leak into the Lua bare-name keyspace. F1-A (identifier-qualified Go call-site refs) fixes this bias at the source. |
| 3 | Workspace root / walk scope | **REJECTED as root cause** (real-repo amplifier only) | `resolveRoot()` in `internal/skill/repomap/skill.go` returns the correct root; `internal/` is walked. The `.claude/worktrees/` tree multiplies the Lua fixture's presence 13× in the real developer workspace, which amplifies the symptom but does not cause it — the bare-name ambiguity alone is sufficient to produce rank-dominance on a single-copy fixture (Plan 01 proves this). Left out-of-scope for this phase; revisit as a separate walker-hardening ticket. |
| 4 | Elision / render (token-budget fitter picks one deep file) | **REJECTED as root cause** (defensive hardening deferred) | `TreeRenderer.RenderBudgeted`'s binary search diversifies naturally once the ranking upstream is correct. `TestRepomap_PolyglotRender` passes post-fix. Render-level diversity is an orthogonal hardening measure deferred to a future repomap-polish phase per RESEARCH.md §Candidate 4. |

## Confirmed Cause

Candidates 1 and 2 together — **bare-name ambiguity amplified by
extractor asymmetry**, both resident in `internal/repomap/`. Neither
mechanism alone is sufficient to fix the adversarial synthetic fixture
(Plan 01 demonstrated F1-B alone left Lua still dominating because Go
had no incoming cross-file rank at all — every Go bare ref pointed
into Lua).

Mechanism:

1. `graph.go:BuildGraph` indexed `defs[name] = []file` and
   `refs[name] = file → count` on bare identifier.
2. `extractor.go:buildQualifiedName` was invoked for def captures
   only, so Go defs carried qualified names (`Store.Add`) while Go
   refs at call sites stayed bare (`Add`).
3. Lua's `queries/lua_tags.scm` emits bare field identifiers
   (`add` for `calculator.add`, `Logger` for `utils.Logger`, etc.).
4. Bare Go ref `Add` ≡ bare Lua def `add` in the key space → cross-
   language edge, weighted only by reference count.
5. Many Go ref files, few Lua def files → Lua files accumulate rank
   mass → Lua sink dominates `RankFiles`.

## Fix Applied

Combined F1-B (graph weight) plus F1-A (extractor qualification).
Both are language-agnostic in shape — F1-A is *gated* on
`lang == "go"` but introduces zero path-, extension-, or language-
whitelist logic; it simply uses Go's tree-sitter `selector_expression`
to produce a qualified key where one exists.

### F1-B — Ambiguity-weighted edges (commit `c1ff7a06`)

`internal/repomap/graph.go:BuildGraph` now scales each cross-file
ref→def edge by an inverse-square-root ambiguity factor:

```go
defDegree := make(map[string]int, len(defs))
for name, files := range defs {
    defDegree[name] = len(files)
}
// per edge:
ambiguityScale := 1.0 / math.Sqrt(1.0 + float64(defDegree[ident]))
weight := math.Sqrt(float64(count)) * ambiguityScale
```

Rationale: a name defined in many files is a weaker per-edge signal
than a unique name. `1/sqrt(1 + defDegree)` is information-theoretic,
symmetric across languages, and path-agnostic.

F1-B is necessary but insufficient on the adversarial fixture — all
`defDegree` values are 1 on a minimal reproduction, so the scale is
uniform and cannot re-order the ranks.

### F1-A — Identifier-qualified Go call-site refs (commit `a8f9be80`)

`internal/repomap/extractor.go:Extract` now invokes a new helper
`qualifyGoRef` when the capture is `reference.*` and language is Go.
The helper walks to the name node's parent: if it is a
`selector_expression` whose `field` child is the name node and whose
`operand` child is an `identifier`, the returned qualified name is
`{operand}.{field}` (e.g. `s.Add`). Otherwise it returns empty and the
caller keeps the bare name (e.g. `trim` from a bare `trim(x)` call).

The fix is gated entirely by `lang == "go"` and `kind == TagRef` —
Lua and every other language keep their pre-F1-A behavior.

F1-A breaks the Go→Lua bare-name collision by making Go ref keys
distinct from bare Lua def keys. Combined with F1-B the adversarial
fixture produces a Go top-1 rank and Go in the top-3.

## Why This Is Not A Path Filter (D-01 compliance)

No branch on file extension, path prefix, or language/file whitelist
was introduced. Audit:

```bash
grep -cE 'filepath\.Ext|skipDirs\b|\.(go|lua)' \
    internal/repomap/graph.go internal/repomap/extractor.go
```

Output:

```
internal/repomap/graph.go:0
internal/repomap/extractor.go:0
```

The fix is a pure math change to edge weights (F1-B) and a Go-
dispatched tree-sitter qualification pass that doesn't filter anything
(F1-A). It would behave identically on a repository where Lua paths
contain real source and Go paths contain testdata.

D-01 honored.

## Evidence

- **Plan 01 negative-control (pre-fix):**
  `go test ./internal/repomap/ -run TestRepomap_Polyglot` FAILED with
  `top-ranked file must be Go, got .../utils.lua`. See
  `46-01-SUMMARY.md` lines 42-70 for rank table and failure excerpt.
- **Plan 02 green flip (post-F1-B+F1-A):**
  Same command PASSED. Rank table post-fix:
  ```
  RANK 0.4954  pkg/logger.go          <-- Go, top-1 ✓
  RANK 0.1778  ...fixture/calculator.lua
  RANK 0.1778  ...fixture/utils.lua
  RANK 0.0453  pkg/store.go           <-- Go in top-4
  RANK 0.0417  pkg/server.go
  ```
  See `46-02-SUMMARY.md` lines 111-133.
- **Plan 03 oracle:**
  `go test -tags=integration ./test/oracle/scenario/ -run TestScenario_RepoMap_Polyglot -v`
  PASSES, proving the fix holds at the MCP-tool boundary as well.
- **D-01 audit:**
  `grep -cE 'filepath\.Ext|skipDirs' internal/repomap/graph.go` returns
  `0`; same for `extractor.go`. Zero path/language/extension heuristics.
- **Full suite invariant:**
  `go test ./...` green modulo two pre-existing unrelated failures
  (`TestClientRegistryContainsAll` in `internal/cli`,
  `TestToolDescriptionsComplete`/`Golden` in `test/bench`) noted in
  Plan 02 SUMMARY and verified independent of this plan.

## Links

- `internal/repomap/graph.go` — fix site F1-B
- `internal/repomap/extractor.go` — fix site F1-A
- `internal/repomap/polyglot_rank_test.go` — unit reproduction + regression guard
- `test/oracle/scenario/repomap_polyglot_test.go` — oracle regression guard
- `testdata/fixtures/polyglot_lua/` — oracle fixture
- Phase research: `.planning/phases/46-bug-repomap-lua-fixture/46-RESEARCH.md`
- Phase context: `.planning/phases/46-bug-repomap-lua-fixture/46-CONTEXT.md`
- Phase summaries: `46-01-SUMMARY.md`, `46-02-SUMMARY.md`
- Fix commits: `c1ff7a06` (F1-B), `a8f9be80` (F1-A)

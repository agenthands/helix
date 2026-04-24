# Phase 46: bug-repomap-lua-fixture - Pattern Map

**Mapped:** 2026-04-24
**Files analyzed:** 3 (1 modified, 2 new)
**Analogs found:** 3 / 3

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/repomap/graph.go` (modify) | core graph builder | transform (tags → weighted edges) | self (existing `BuildGraph`) | exact (in-place edit) |
| `internal/repomap/polyglot_rank_test.go` (new) | unit test | transform + assertion | `internal/repomap/graph_test.go` + `internal/repomap/pagerank_test.go` | exact (same package, same helpers) |
| `test/oracle/scenario/serena_repomap_test.go` (new) | oracle integration test | request-response (MCP tool call) | `test/oracle/scenario/repomap_test.go` | exact (same package, same fixture flow) |
| `internal/repomap/render.go` (optional tweak, deferred per RESEARCH §F4-A) | tree renderer | transform | self | N/A — touched only if ranking fix is insufficient |

## Pattern Assignments

### `internal/repomap/graph.go` (core graph builder, transform) — MODIFY

**Analog:** self (same file, surgical edit to `BuildGraph`).

**Existing edge-creation loop to be modified** (`internal/repomap/graph.go:98-118`):
```go
g := NewFileGraph()

// Create edges: referencer -> definer, weight = sqrt(refCount).
for ident, definers := range defs {
    refFiles, hasRefs := refs[ident]
    if !hasRefs {
        // Self-loop for isolated definitions (aider pattern, weight 0.1).
        for _, defFile := range definers {
            g.addEdge(defFile, defFile, 0.1)
        }
        continue
    }
    for refFile, count := range refFiles {
        for _, defFile := range definers {
            if refFile == defFile {
                continue // skip same-file refs
            }
            g.addEdge(refFile, defFile, math.Sqrt(float64(count)))
        }
    }
}
```

**Fix-shape to apply** (from `RESEARCH.md` §Fix-Shape Constraints → F1-B — ambiguity-weighted edges):
```go
// Count how many files define this name (ambiguity signal).
defDegree := make(map[string]int, len(defs))
for name, files := range defs {
    defDegree[name] = len(files)
}

g := NewFileGraph()
for ident, definers := range defs {
    ambiguityScale := 1.0 / math.Sqrt(1.0+float64(defDegree[ident]))
    refFiles, hasRefs := refs[ident]
    if !hasRefs {
        for _, defFile := range definers {
            g.addEdge(defFile, defFile, 0.1)
        }
        continue
    }
    for refFile, count := range refFiles {
        for _, defFile := range definers {
            if refFile == defFile {
                continue
            }
            g.addEdge(refFile, defFile, math.Sqrt(float64(count))*ambiguityScale)
        }
    }
}
```

**Invariants to preserve:**
- `math` import already present (line 4).
- Self-loop weight for isolated defs stays at `0.1` (keeps `TestBuildGraph_IsolatedDefinition` passing).
- Same-file ref skip at line 112-114 stays.
- No new language-specific logic, no path filters (honors **D-01**).

**Test impact (from RESEARCH §Open Question 3):**
- `graph_test.go:TestBuildGraph_CrossFileEdges` asserts `sqrt(1)=1.0`; with F1-B it becomes `1.0 * 1/sqrt(2) ≈ 0.707`.
- `graph_test.go:TestBuildGraph_WeightByRefCount` asserts `sqrt(4)=2.0`; with F1-B it becomes `2.0 * 1/sqrt(2) ≈ 1.414`.
- Update these weight constants symmetrically; relative ordering is unchanged.

---

### `internal/repomap/polyglot_rank_test.go` (unit test, transform+assertion) — NEW

**Analog:** `internal/repomap/graph_test.go` (same package, same helpers) — for fixture construction via `newTestCacheWithTags`; `internal/repomap/pagerank_test.go` — for ranking assertion style.

**Package declaration + imports pattern** (`graph_test.go:1-11`):
```go
package repomap

import (
    "math"
    "os"
    "path/filepath"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)
```

**Fixture-construction helper pattern to reuse** (`graph_test.go:15-46`, `newTestCacheWithTags`):
```go
// newTestCacheWithTags creates a TagCache populated with the given file->tags mapping.
// Each file gets a real temp file on disk (required for mtime checks in GetOrExtract).
func newTestCacheWithTags(t *testing.T, files map[string][]Tag) (*TagCache, map[string]string) {
    t.Helper()
    cache := newTestCache(t)
    dir := t.TempDir()
    paths := make(map[string]string, len(files))
    for name, tags := range files {
        fp := filepath.Join(dir, name)
        require.NoError(t, os.MkdirAll(filepath.Dir(fp), 0o755))
        require.NoError(t, os.WriteFile(fp, []byte("// "+name), 0o644))
        paths[name] = fp
        realTags := make([]Tag, len(tags))
        for i, tag := range tags {
            realTags[i] = tag
            realTags[i].File = fp
        }
        tagsCopy := realTags
        _, err := cache.GetOrExtract(fp, func() ([]Tag, error) { return tagsCopy, nil })
        require.NoError(t, err)
    }
    return cache, paths
}
```
**How to extend for polyglot:** the keys in the `files` map are relative names like `"pkg/server.go"` or `"testdata/fixtures/lua/deep/nested/fixture/main.lua"`. `filepath.Dir(fp)` + `MkdirAll` already handle nested paths. No new helper is needed.

**Test case structure pattern** (`graph_test.go:48-64`, `TestBuildGraph_CrossFileEdges`):
```go
func TestBuildGraph_CrossFileEdges(t *testing.T) {
    cache, paths := newTestCacheWithTags(t, map[string][]Tag{
        "fileA.go": {{Name: "Foo", Kind: TagRef, Line: 5, Column: 1}},
        "fileB.go": {{Name: "Foo", Kind: TagDef, Line: 1, Column: 5}},
    })

    g, err := BuildGraph(cache)
    require.NoError(t, err)
    require.Contains(t, g.Edges, paths["fileA.go"])
    assert.InDelta(t, math.Sqrt(1), g.Edges[paths["fileA.go"]][paths["fileB.go"]], 0.001)
}
```

**Ranking assertion pattern** (`pagerank_test.go:117-146`, `TestFileGraph_RankFiles_SortedDescending`):
```go
ranked := g.RankFiles(0.85, nil)
require.Len(t, ranked, 3)
for i := 1; i < len(ranked); i++ {
    assert.GreaterOrEqual(t, ranked[i-1].Score, ranked[i].Score, ...)
}
```

**Fixture design** (from RESEARCH §Synthetic Fixture Design):
- 5 Go files under `pkg/` with cross-file refs (`Server`, `Store`, `Logger`, `Handler`, `Add`, `Log`, ...).
- 3 Lua files under `testdata/fixtures/lua/deep/nested/fixture/` with bare-name collisions (`add`, `log`, `trim`, `Logger`, `new`).
- Lua files form a closed subgraph (main.lua refs calc+utils; no edges back to `pkg/`).

**Required tests** (from VALIDATION map):
- `TestRepomap_PolyglotRanking` — `ranked[0]` has `.go` extension under `pkg/`.
- `TestRepomap_PolyglotRender` — `RenderBudgeted(ranked, 2048)` output contains a Go symbol from `pkg/` and is not 100% Lua.

**Negative control requirement (D-04, explicit in RESEARCH line 208):**
Before committing the fix, confirm the test FAILS against the unfixed `BuildGraph` (i.e., top-1 is Lua). This proves the fixture actually reproduces the bug. Document the pre-fix failure in the phase review.

---

### `test/oracle/scenario/serena_repomap_test.go` (oracle integration test) — NEW

**Analog:** `test/oracle/scenario/repomap_test.go` — same package, same `harness` API, same build tag.

**Build tag + package + imports pattern** (`repomap_test.go:1-11`):
```go
//go:build integration || llm || llmjudge

package scenario_test

import (
    "testing"

    "github.com/stretchr/testify/assert"

    "github.com/postfix/serena/test/harness"
)
```

**Runner setup pattern** (`repomap_test.go:16-30`, `TestScenario_GetRepoMap`):
```go
func TestScenario_GetRepoMap(t *testing.T) {
    fixtureDir := harness.PrepareFixture(t, "go")
    runner := harness.StartRunner(t, harness.RunnerOptions{
        WorkspaceDir: fixtureDir,
        SkipLS:       true,
    })

    result := harness.CallTool(t, runner.Session, "get_repo_map", map[string]any{
        "token_budget": float64(4096),
    })
    text := harness.TextContent(result)
    assert.NotEmpty(t, text, "get_repo_map should return non-empty output")
}
```

**Fixture preparation note:**
- Existing oracle tests use `harness.PrepareFixture(t, "go")` which copies `testdata/fixtures/go/`. A new polyglot serena-shaped fixture may be needed — simplest option: create `testdata/fixtures/polyglot_lua/` (Go pkg + nested lua testdata) and invoke `harness.PrepareFixture(t, "polyglot_lua")`.
- Alternative (per CONTEXT.md success criterion 1): point `WorkspaceDir` at `harness.ProjectRoot()` (real Serena repo). Follows RESEARCH Landmine #3 — must clear persistent tag cache first. Planner decides based on D-05 brittleness trade-off.

**Assertions to add** (new, derived from VALIDATION map):
- `text` contains at least one path prefix `internal/` **and** `.go` extension, OR is not dominated by `.lua`.
- Downgrade path (D-05): if repeatedly brittle, reduce to single-shot verification recorded in phase review.

**Cache-clearing pattern (RESEARCH Landmine #1, Open Question 2):**
The harness runner uses a per-test `t.TempDir()` for `ProjectDir`, so `index/tags.db` is isolated. When pointing at the real repo root with `WorkspaceDir: harness.ProjectRoot()`, the `ProjectDir` still lives under `t.TempDir()` — cache isolation is preserved. No manual `rm` of `tags.db` needed.

---

## Shared Patterns

### Error-handling / assertion style
**Source:** `internal/repomap/graph_test.go`, `internal/repomap/pagerank_test.go`
**Apply to:** new unit test file.
- `require.NoError(t, err)` for setup failures (abort on failure).
- `assert.InDelta(t, expected, actual, 0.001)` for floating-point comparisons.
- `require.Len`, `require.Contains` for structural preconditions before deeper asserts.

### Testify imports
**Source:** every `*_test.go` file in `internal/repomap/`
```go
"github.com/stretchr/testify/assert"
"github.com/stretchr/testify/require"
```

### Oracle test build tag
**Source:** `test/oracle/scenario/*_test.go`
**Apply to:** new oracle test file.
```go
//go:build integration || llm || llmjudge
```
Combined with `package scenario_test` (external test package) and `harness.*` helpers.

### Table-driven ranking assertion
**Source:** `pagerank_test.go:117-146` (`TestFileGraph_RankFiles_SortedDescending`)
**Apply to:** unit test — reuse the descending-order sweep loop to confirm monotonic rank ordering in the polyglot fixture.

### No-path-filter invariant
**Source:** D-01 (CONTEXT) + RESEARCH §Fix-Shape Constraints
**Apply to:** graph.go edit only.
- Do NOT add to `skipDirs` in `internal/skill/repomap/skill.go:308-311`.
- Do NOT branch on extension / path prefix inside `BuildGraph`.
- Fix must be a pure weighting change (F1-B) or a symmetrical qualification change (F1-A).

## No Analog Found

None — every new artifact has a direct, high-quality analog in the existing codebase.

## Metadata

**Analog search scope:** `internal/repomap/`, `internal/skill/repomap/`, `test/oracle/scenario/`, `test/harness/`, `testdata/fixtures/`.
**Files scanned:** 11 (graph.go, graph_test.go, pagerank.go, pagerank_test.go, render.go, cache_test.go, repomap_test.go, polyglot_test.go, fixture.go, lua_tags.scm, go_tags.scm listings).
**Pattern extraction date:** 2026-04-24

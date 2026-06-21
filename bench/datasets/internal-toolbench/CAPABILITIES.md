# Internal ToolBench — Capability Classes

The internal ToolBench suite (`bench/datasets/internal-toolbench/`) exercises Helix's
semantic MCP tools through **deterministic, hermetic, solve-then-`go test` fixtures**.
Each fixture seeds a deliberately-incomplete module; a scripted agent drives the
capability's named Helix tool(s) to solve it, and the fixture's own `go test ./...`
grades the result (via the Go `LanguageRunner`, `bench/languages/go`).

This document enumerates the **10 capability classes** (criterion C1 / TOOLBENCH-01).
The enum string values are the source of truth for each fixture's `task.json`
`capability` field (`bench/languages/runner.go`, decision D-06); the corpus-coverage
aggregator (`bench/languages/coverage.go`) reads those fields and cross-references the
runner's `Capabilities()` declared set to report per-language coverage (D-11).

## Coverage thresholds (per language)

Coverage is `Capabilities()` (declared) ∩ `task.json` `capability` fields (covered).

| Language | Threshold | Phase |
|----------|-----------|-------|
| Go       | **10/10** (all classes) | Phase 78 (this phase) |
| Python / Rust / TypeScript | ≥ 8 | Phase 85 (forward) |
| C# / C++ | ≥ 6 | Phase 85 (forward) |

Go is the first-class reference tier (full LSP-backed resolution via gopls), so it
carries all 10 classes. The forward-looking thresholds above are documented here for
readers extending the corpus to additional languages in Phase 85; they are NOT enforced
by this phase.

## The 10 capability classes

Each section names the capability enum value (the `capability` string), what it
exercises, the Helix MCP tool(s) the scripted agent drives, and the Go fixture
directory that covers it.

### 1. `semantic_view`

- **Exercises:** Enumerating a file's symbol structure to discover every member that
  must be implemented.
- **Helix tool(s):** `get_symbol_overview`
- **Go fixture:** `go/IT-go-semantic-view-1` — `Calculator` in `calc.go` has four methods
  (`Add`, `Sub`, `Mul`, `Div`) that all `panic("not implemented")`; the overview
  enumerates them and all must be implemented for `go test` to pass.

### 2. `lsp_diagnostics`

- **Exercises:** Surfacing compiler/LSP diagnostics and fixing every reported error.
- **Helix tool(s):** `get_diagnostics`
- **Go fixture:** `go/IT-go-lsp-diagnostics-1` — `broken.go` has three compile errors
  (an unused import, a wrong return type, an undefined variable); the package builds and
  `TestGreeting` runs only once all are fixed.

### 3. `rename_safety`

- **Exercises:** Finding every reference to a symbol, then renaming it safely across
  files so callers still resolve.
- **Helix tool(s):** `find_references` → `rename_symbol`
- **Go fixture:** `go/IT-go-rename-safety-1` — exported `Foo` (defined in `widget.go`,
  called from `consumer.go`) must be renamed to `Bar` everywhere; `rename_test.go`
  imports the new name.

### 4. `fuzzy_search`

- **Exercises:** Whitespace-tolerant editing where an exact patch may not reproduce the
  seed's indentation; the fuzzy match cascade applies the edit anyway.
- **Helix tool(s):** `fuzzy_edit` / `replace_in_file` (fuzzy fallback)
- **Go fixture:** `go/IT-go-fuzzy-search-1` — `Discount` in `price.go` must apply a 10%
  discount (`price * 9 / 10`); the tab-indented body is edited via the
  whitespace-tolerant matcher.

### 5. `call_graph`

- **Exercises:** Enumerating every caller of a function via the incoming call hierarchy,
  then changing each call site.
- **Helix tool(s):** `get_call_hierarchy` (`direction: incoming`)
- **Go fixture:** `go/IT-go-call-graph-1` — `Compute(divisor int)` panics on a zero
  divisor; all three callers (`RunA`, `RunB`, `RunC`) must guard against it. The test
  panics unless every caller is guarded.

### 6. `dependency_graph`

- **Exercises:** Discovering which packages depend on a target package, then wiring a
  helper into each dependent.
- **Helix tool(s):** `get_repo_map` / `get_context`
- **Go fixture:** `go/IT-go-dependency-graph-1` — `core.Tag` returns a version stamp;
  three packages (`alpha`, `beta`, `gamma`) import `core` but return an empty `Label`
  and must each be wired to `core`.

### 7. `patch_apply`

- **Exercises:** Applying a targeted edit to a symbol body, the canonical
  edit-then-`go test` shape.
- **Helix tool(s):** `replace_symbol_body` / `replace_in_file`
- **Go fixture:** `go/IT-go-patch-apply-1` — `Double` in `sum.go` returns its argument
  unchanged; it must return `x * 2` so `TestDouble` passes (the migrated Phase 77 seed).

### 8. `context_minimization`

- **Exercises:** Surfacing only the relevant symbol(s) from a large file so the agent
  fixes the targeted function without touching unrelated helpers.
- **Helix tool(s):** `get_context`
- **Go fixture:** `go/IT-go-context-min-1` — `shipping.go` has many helpers, but only
  `ShippingCost` is wrong (returns 0 instead of `weight * ratePerKg`); `get_context`
  surfaces the relevant symbol.

### 9. `incremental_update`

- **Exercises:** Editing a file, then driving the Phase 70 overlay-drain refresh so the
  live semantic graph reflects the edit, then issuing a semantic query against the
  refreshed state. **This is the one store-ON fixture** (`semantic_index.enabled=true`
  per cell, D-01); it is excluded from `make bench-quick` to protect the ≤90s budget
  (D-01/Pitfall 3).
- **Helix tool(s):** `refresh_semantic_graph` (+ an edit, then `get_semantic_context`)
- **Go fixture:** `go/IT-go-incremental-update-1` — `registry.go` registers under the
  wrong token (`"legacy"`); the agent flips it to `"incremental"`, refreshes the
  semantic graph, then a semantic query reflects the post-refresh state.

### 10. `failure_handling`

- **Exercises:** Recovering from a deliberately-failing first tool call (e.g. a wrong
  path) and completing the task on the corrected call.
- **Helix tool(s):** any tool with `expect_error: true` in the scripted step
- **Go fixture:** `go/IT-go-failure-handling-1` — `Negate` in `math.go` returns `n`
  instead of `-n`; a first edit against a wrong path (`typo.go`) is expected to fail,
  and recovery edits the correct file `math.go`.

## Namespace

Every Go fixture id matches `^IT-go-` and is deliberately **disjoint** from the Phase 67
`T-67-*` planning task IDs (zero collision — criterion C4). See
[`PHASE67_CROSSWALK.md`](./PHASE67_CROSSWALK.md) for the inspiration-only mapping. The
namespace invariant is enforced by a static test in `bench/languages/coverage_test.go`.

# Phase 73: P1 Tools Integration & E2E Verification - Pattern Map

**Mapped:** 2026-05-21
**Files analyzed:** 4 (2 new test files, 6 help-const edits in existing files, 1 skill.go edit)
**Analogs found:** 4 / 4 (every new artifact has an exact in-tree analog)

This is a verification/integration phase. No RESEARCH.md exists — all patterns
are grounded in the existing `internal/skill/semantic/` codebase. Every new file
has a direct sibling analog; the planner should copy structure verbatim.

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/skill/semantic/profile_filter_test.go` (MODIFY — add P1 cases) | test (golden) | transform / table-driven | itself (extend P0 pattern) | exact (self-extension) |
| `internal/skill/semantic/p1_e2e_external_test.go` (NEW — `buildP1E2EFixture` + 6 tests) | test (E2E fixture + suite) | request-response over real `*Store` | `production_adapter_e2e_test.go` + `status_e2e_external_test.go` | exact |
| `internal/skill/semantic/wrapper_consistency_test.go` (NEW — static gate) | test (static source-scan gate) | file-I/O / batch | `readonly_gate_test.go` | exact |
| `internal/skill/semantic/tool_help_test.go` (NEW — `get_tool_help` coverage) | test (coverage) | transform | `profile_filter_test.go` (table-driven, in-package) | role-match |
| `internal/skill/semantic/tools_change_impact.go` (MODIFY — standardize `*Help`) | help-text const | n/a (doc string) | `tools_index.go` `indexHelp` const | exact |
| `tools_{explain_symbol,find_related,validate_edge,cluster_map,explain_cluster}.go` (MODIFY — normalize `*Help` section layout) | help-text const | n/a (doc string) | `tools_index.go` `indexHelp` const | exact |
| `internal/skill/semantic/skill.go` (MODIFY — add 6 P1 tools to `Tools()`) | skill ToolProvider list | config / registration | `skill.go` `Tools()` (extend in place) | exact (self-extension) |

## Pattern Assignments

### `profile_filter_test.go` — add P1 golden cases (D-01, SC#1)

**Analog:** itself — extend the existing P0 table-driven pattern.

**Slice-declaration pattern to mirror** (`profile_filter_test.go:28-43`):
```go
// semanticToolNames is the four-tool surface this skill exposes via Tools().
var semanticToolNames = []string{
	"index_semantic_graph",
	"refresh_semantic_graph",
	"get_semantic_graph_status",
	"get_semantic_context",
}
```
Add a parallel `p1ToolNames` slice for the 6 P1 tools and a `p1ReviewPlusOnlyTools`
subset. From the handler reads: `explain_symbol_deep`, `find_related_symbols`,
`validate_graph_edge`, `get_cluster_map`, `explain_cluster` all call
`checkMode(snap, modeTierRead)` (read+ tier — visible in all 4 modes).
`get_change_impact_graph` calls `checkMode(snap, modeTierReview)`
(`tools_change_impact.go:148`) — it is **review+** and MUST be excluded from
`read` and `edit` modes, exactly like `index_semantic_graph`.

**Resolution helper to reuse verbatim** (`profile_filter_test.go:50-87`) —
`resolveForProfileMode(t, store, profile, mode)` already drives
`skill.ResolveTools` over the merged (skill ∪ mode) sets. Do NOT reimplement it.

**Assertion helpers to reuse** (`profile_filter_test.go:90-115`) — `hasAll` and
`hasNone`. The leakage test (per `<specifics>`) must use `hasNone` against
`get_change_impact_graph` in `read`/`edit` modes, mirroring
`TestProfileFilter_ReadMode_IndexExcluded` (`profile_filter_test.go:164-175`):
```go
for _, p := range []string{"full", "claude-code", "codex", "ide-assistant", "ci-bot"} {
	got := resolveForProfileMode(t, store, p, "read")
	hasAll(t, p+"/read", got, readPlusOnlyTools)
	hasNone(t, p+"/read", got, []string{"index_semantic_graph"})
}
```

**CRITICAL prerequisite (Integration Point — most likely SC#1 blocker):**
`SemanticSkill.Tools()` (`skill.go:325-352`) currently returns ONLY the 4 P0
tools. `skill.ResolveTools` operates on the `Tools()` ToolProvider surface, so
the 6 P1 tools are invisible to profile filtering as-is. The P1 golden test will
assert presence of tools that the resolver never sees. The planner MUST first
extend `Tools()` (see skill.go assignment below) — otherwise SC#1 is unverifiable.

---

### `p1_e2e_external_test.go` — `buildP1E2EFixture` real-store E2E suite (D-02, D-02a, SC#4)

**Analogs:** `production_adapter_e2e_test.go` (real `*Store` setup) +
`status_e2e_external_test.go` (real `*Store` + bleve + handler invocation).

**Package + import-cycle pattern** (`production_adapter_e2e_test.go:18`,
`status_e2e_external_test.go:15`) — MUST be black-box `package semantic_test`
because the file imports `internal/daemon` for production-adapter constructors
(`internal/daemon` imports `internal/skill/semantic`, so same-package would cycle):
```go
package semantic_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/agenthands/helix/internal/daemon"
	"github.com/agenthands/helix/internal/mcp"
	"github.com/agenthands/helix/internal/obs"
	semanticpkg "github.com/agenthands/helix/internal/semantic"
	semanticstore "github.com/agenthands/helix/internal/semantic/store"
	"github.com/agenthands/helix/internal/skill"
	"github.com/agenthands/helix/internal/skill/semantic"
	"github.com/agenthands/helix/internal/workspace"
)
```

**Real `*Store` + tempdir setup** — copy from `production_adapter_e2e_test.go:55-74`
(duckdb store config) and `status_e2e_external_test.go:77-83` (bleve engine +
Recoverer):
```go
wsDir := t.TempDir()
t.Chdir(wsDir)
logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelInfo}))
provider := obs.Noop(logger.Handler())
storeCfg := semanticpkg.Config{
	Enabled: true,
	Store: semanticpkg.StoreConfig{
		Kind: "duckdb", Path: filepath.Join(".helix", "semantic.duckdb"),
		MemoryLimit: "256MiB", Threads: 2,
	},
}
store, err := semanticstore.Open(context.Background(), storeCfg, logger, provider.Metrics())
require.NoError(t, err)
t.Cleanup(func() { _ = store.Close() })

bleveDir := filepath.Join(wsDir, ".helix", "semantic.bleve")
engine, err := retrieval.New(bleveDir)
require.NoError(t, err)
t.Cleanup(func() { _ = engine.Close() })
```

**Snapshot-commit pattern** — copy the multi-language symbol/edge seeding from
`production_adapter_e2e_test.go:82-135` (BeginSnapshot → WriteSnapshotFacts →
CommitSnapshot) and `status_e2e_external_test.go:111-167` (`buildFn` shape).
The fixture must seed Go / TypeScript / Java symbols + CALLS / RESOLVES_TO /
USES_TYPE edges + cluster rows (mirror the in-memory layout in
`populated_graph_fixture_test.go:26-32` so the E2E data matches the unit-test
shape). The cluster rows go in via the overlay-tx pattern from
`status_e2e_external_test.go:335-356` (`BeginOverlayTx` → `BumpGraphVersion` →
`UpsertClusters` → `UpsertClusterMembers` → `Commit`).

**Shared-builder discipline (D-02 + the `populated_graph_fixture_test.go:1-8`
"DO NOT inline-copy" norm)** — `buildP1E2EFixture(t)` is one function invoked by
all 6 tool E2E tests. Do NOT do per-tool tempdir setup; do NOT mutate
`productionAdapterFixture` in place (couples Phase 65 tests to Phase 73 data).

**Production-adapter wiring** — `daemon.NewIntegSemanticLookupForTest(store, ws)`
(`production_adapter_e2e_test.go:156`) and the scheduler/retrieval factory
constructors `daemon.NewSchedulerAccessorForStore(store)` /
`daemon.NewRetrievalAccessorForStore(store, engine)`
(`status_e2e_external_test.go:378-379`). For the 6 P1 tools, wire the Phase 71/72
seam accessors via the `Set*` setters on `*semantic.SemanticSkill`
(`skill.go:148-239`: `SetSymbolByName`, `SetTypeChain`, `SetSymbolEdges`,
`SetEdgeEvidence`, `SetClusterMap`, `SetClusterMember`, `SetClusterPageRank`,
`SetImpactLookup`, plus `SetSessionAccessor` for mode).

**Handler invocation + envelope decode** — use the `*ForTest` exports as in
`status_e2e_external_test.go:317` / `:384` (`semantic.HandleIndexSemanticGraphForTest`,
`semantic.HandleGetSemanticGraphStatusForTest`). Phase 73 needs the analogous
`Handle*ForTest` exports for the 6 P1 handlers — check `export_status_test.go`
(`package semantic`) for the export idiom; if a P1 `*ForTest` export is missing,
the planner adds it in a `package semantic` `export_*_test.go` file (NOT in the
external file). Result decoding pattern (`status_e2e_external_test.go:282-291`):
```go
func extractText(t *testing.T, res *mcpsdk.CallToolResult) string {
	t.Helper()
	for _, c := range res.Content {
		if tc, ok := c.(*mcpsdk.TextContent); ok { return tc.Text }
	}
	t.Fatalf("no TextContent in result %+v", res)
	return ""
}
```

**Closed-enum envelope assertions (per `<specifics>` — enum membership, not nil)** —
each of the 6 tests must decode the result JSON and assert `freshness.status` ∈
`{current, stale, unknown}` (from `tools_explain_symbol.go:357-405`
`assembleFreshness`: `FreshnessStatusCurrent/Stale/Unknown`), `freshness.source`
is set (`FreshnessSourceGraph`), `fallback_reason` is empty OR a declared
closed-enum value, and `confidence` is within `[0.0, 1.0]` (and ≤ 0.6 on degraded
type-resolver paths per `tools_explain_symbol.go:308-321`).

---

### `wrapper_consistency_test.go` — static SC#3 gate (D-04, D-04a)

**Analog:** `readonly_gate_test.go` — copy its structure exactly.

**Package** — in-tree `package semantic` (NOT `_test`) — it only reads source
files via `os.Open`, no daemon import needed (`readonly_gate_test.go:21`).

**File-list pattern** (`readonly_gate_test.go:46-53`) — reuse the SAME 6-entry
list; the Phase 72 extension already covers all 6 P1 handlers:
```go
var gatedHandlerFiles = []string{
	"tools_explain_symbol.go",
	"tools_find_related.go",
	"tools_validate_edge.go",
	"tools_cluster_map.go",
	"tools_explain_cluster.go",
	"tools_change_impact.go",
}
```
Use a distinct identifier (e.g., `p1WrapperGatedFiles`) to avoid colliding with
`readonly_gate_test.go`'s `gatedHandlerFiles` in the same package.

**Comment-stripping scanner pattern to copy verbatim** (`readonly_gate_test.go:67-104`):
```go
f, err := os.Open(file)
...
scanner := bufio.NewScanner(f)
lineNum := 0
for scanner.Scan() {
	lineNum++
	raw := scanner.Text()
	trimmed := strings.TrimLeft(raw, " \t")
	if strings.HasPrefix(trimmed, "//") { continue } // skip comment lines
	... // assertion logic
}
```

**SC#3 required-token logic** — instead of `gateForbiddenTokens` (forbidden),
the wrapper gate asserts each file CONTAINS the required wrapper tokens. From
the handler reads:
- `checkMode(` — present in every handler (`tools_explain_symbol.go:187`,
  `tools_change_impact.go:148`, etc.). Assert the substring `checkMode(` appears
  on a non-comment line. NOTE: do NOT pin the tier — 5 handlers use
  `modeTierRead`, `tools_change_impact.go` uses `modeTierReview`. Check the
  helper is called, not which tier.
- `FreshnessV2` — every handler emits it (`tools_explain_symbol.go:94`,
  `tools_find_related.go:65`, `tools_change_impact.go:97`, etc.). Assert the
  `FreshnessV2` token appears on a non-comment line.
- `RegisterAll` coverage — assert each `register<Tool>` function name appears in
  `register.go`'s `RegisterAll` body (`register.go:23-32` lists all 10
  `register*` calls). Read `register.go` once and check the 6 P1 register-fn
  names are present.

Per-file `t.Run(file, ...)` subtests (`readonly_gate_test.go:70`) for granular
failure reporting.

---

### `tool_help_test.go` — `get_tool_help` coverage for 6 P1 tools (D-03, SC#2)

**Analog:** `profile_filter_test.go` table-driven in-package test structure.

**Mechanism (locked by SC#2 — do not re-litigate)** — `get_tool_help` extracts
param docs from the typed-args struct via `jsonschema.For[T]`; the extraction
side is `help.ExtractParamDocs(inputSchema)` (`internal/kernel/help/help.go:19-67`)
which reads `properties` + `required` + `enum` from the marshalled JSON schema.
Each P1 args struct already carries `json` + `jsonschema` tags (e.g.,
`ExplainSymbolDeepArgs` at `tools_explain_symbol.go:33-36`,
`IndexSemanticGraphArgs` at `tools_index.go:69-73`).

**Coverage assertion** — for each of the 6 P1 tools, assert `get_tool_help`
returns non-empty parameter documentation. Drive it through the same registry
surface the tool registration uses: each `register*` calls
`server.Registry().Register(&mcp.ToolDef{... HelpText: ...})`
(`tools_explain_symbol.go:155-160`). Build a table `{toolName, argsType}` and
assert `ExtractParamDocs` over the generated schema yields ≥ 1 `ParamDoc` with a
non-empty `Description`.

---

### Help-const standardization — 6 `*Help` consts (D-03, D-03a, SC#2)

**Analog (the canonical template):** `tools_index.go:15-63` `const indexHelp`.

**Template sections** (Claude's Discretion picks exact wording, but all 6 must be
structurally identical):
```
## Usage Examples
  <tool>(...)            ← 1-3 worked examples

## Parameters
- name (type, required|optional): description

## Return Shape
- field (type): description

## Mode Tier
read+  — or  review+ ...
```

**Per-file status (from the grep scan):**
- `explainSymbolDeepHelp` (`tools_explain_symbol.go:100-143`) — ALREADY structured
  (`## Usage Examples` / `## Parameters` / `## Return Shape` / `## Mode Tier` /
  `## Caps` / `## Determinism`). Use as the richest reference.
- `findRelatedSymbolsHelp` (`tools_find_related.go:70`) — `## Usage Examples`
  starter present; verify it carries all template sections.
- `validateGraphEdgeHelp` (`tools_validate_edge.go:126`) — `## Usage Examples`
  starter present.
- `getClusterMapHelp` (`tools_cluster_map.go:80`) — `## Usage Examples` starter
  present.
- `explainClusterHelp` (`tools_explain_cluster.go:85`) — `## Usage Examples`
  starter present.
- **`getChangeImpactGraphHelp` (`tools_change_impact.go:103-104`) — THE GAP.**
  It is a single-line string:
  `var getChangeImpactGraphHelp = "Returns the pre-edit blast-radius subgraph (nodes + edges) for a seed symbol (review+)."`
  This is the one file that needs real standardization work — expand it to the
  full `## Usage Examples / ## Parameters / ## Return Shape / ## Mode Tier`
  template matching `indexHelp`. The handler at `tools_change_impact.go` and the
  `GetChangeImpactGraphResult` struct (`:96-97`) provide the field list.

**Editing constraint:** `skill.go:1-9` declares it FINAL — the only allowed edit
to `skill.go` historically was deleting stub lines. Phase 73's `skill.go` edit
(adding P1 tools to `Tools()`) is a deliberate, scoped exception; the help-const
edits live entirely in the `tools_*.go` files, not `skill.go`.

---

### `skill.go` — add 6 P1 tools to `Tools()` (Integration Point, SC#1 enabler)

**Analog:** the existing 4-entry `Tools()` body (`skill.go:325-352`).

**Pattern to extend** (`skill.go:326-332`):
```go
func (s *SemanticSkill) Tools() []*mcp.ToolDef {
	return []*mcp.ToolDef{
		{
			Name:             "index_semantic_graph",
			Description:      "Build or refresh a committed semantic snapshot.",
			BriefDescription: "Index the semantic graph",
			HelpText:         indexHelp,
		},
		// ... 3 more P0 entries ...
	}
}
```
Append 6 entries for the P1 tools. The `Name` / `Description` / `BriefDescription`
/ `HelpText` values for each are already authored inside the
`server.Registry().Register(&mcp.ToolDef{...})` block of each P1 `register*`
function — copy them verbatim:
- `explain_symbol_deep` — `tools_explain_symbol.go:155-160`
- `find_related_symbols` — `tools_find_related.go:137-142`
- `validate_graph_edge` — `tools_validate_edge.go:185`+ block
- `get_cluster_map` — `tools_cluster_map.go:115`+ block
- `explain_cluster` — `tools_explain_cluster.go:129`+ block
- `get_change_impact_graph` — `tools_change_impact.go:107`+ block (uses
  `HelpText: getChangeImpactGraphHelp`)

Also update `Description()` (`skill.go:75-77`) — currently says "(4 tools)";
should read "(10 tools)" or similar. The doc comment at `skill.go:1-2`
("contributing 4 MCP tools") is stale prose; update for accuracy.

**Verification note:** Before locking the planner approach, the planner/researcher
must confirm whether the daemon's profile-filter path actually consumes
`SemanticSkill.Tools()` or whether P1 tools flow through a separate
`RegisterAll`-only path that the filter never sees. `register.go:19-33` registers
all 10 tools directly with the MCP server; `skill.ResolveTools` operates on the
ToolProvider `Tools()` list. If these are two disjoint surfaces, adding to
`Tools()` is mandatory for SC#1. This is flagged as the most likely real gap in
73-CONTEXT.md `<code_context>` Integration Points.

## Shared Patterns

### Mode-tier check
**Source:** `internal/skill/semantic/mode_check.go:33` `checkMode(snap, tier)`
**Apply to:** all 6 P1 handlers (already done) + the `wrapper_consistency_test.go`
gate (asserts presence) + the `profile_filter_test.go` golden test (determines
which tier → which modes show the tool).
```go
snap := s.sessionSnapshot(ctx)
if err := checkMode(snap, modeTierRead); err != nil { // or modeTierReview
	return errorResult(err.Error())
}
```
Tier map: 5 P1 tools are `modeTierRead` (read+, all 4 modes);
`get_change_impact_graph` is `modeTierReview` (review+, excluded from read/edit).

### FreshnessV2 envelope assembly
**Source:** `internal/skill/semantic/tools_explain_symbol.go:357-405`
`assembleFreshness(ctx, repoID)` — shared helper (extracted in 71-03, reused by
all P1 handlers).
**Apply to:** the E2E suite's closed-enum assertions. Status ∈
`{FreshnessStatusCurrent, FreshnessStatusStale, FreshnessStatusUnknown}`;
`Source = FreshnessSourceGraph`. The closed-enum `Freshness` type and its 4
values live in `envelope.go:16-32`.

### In-tree static gate (no external CI linter)
**Source:** `internal/skill/semantic/readonly_gate_test.go` — the repo's
established enforcement mechanism for cross-file invariants (71-01 confirmed no
`.github/workflows`, `scripts/`, or `Makefile` linter exists).
**Apply to:** `wrapper_consistency_test.go` — line-by-line `bufio.Scanner`,
strip `//`-prefixed lines, assert required tokens present.

### Black-box test package for daemon imports
**Source:** `production_adapter_e2e_test.go:18`, `status_e2e_external_test.go:15`
**Apply to:** `p1_e2e_external_test.go` — MUST be `package semantic_test`.
Any `Handle*ForTest` export the suite needs is added in a `package semantic`
`export_*_test.go` file (the `export_status_test.go` idiom).

### Shared fixture builder (no inline copies)
**Source:** `populated_graph_fixture_test.go:1-8` "DO NOT inline-copy" header;
`buildPopulatedGraphFixture(t)` invocation pattern.
**Apply to:** `buildP1E2EFixture(t)` — one builder, 6 callers.

## No Analog Found

None. Every Phase 73 artifact has a direct in-tree analog. This is a pure
verification/integration phase — the planner copies existing structure rather
than authoring novel patterns.

## Metadata

**Analog search scope:** `internal/skill/semantic/` (test files, handler files,
skill.go, register.go, envelope.go, mode_check.go), `internal/kernel/help/help.go`
**Files scanned:** 12
**Pattern extraction date:** 2026-05-21

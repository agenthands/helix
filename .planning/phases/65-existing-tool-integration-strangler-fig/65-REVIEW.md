---
phase: 65-existing-tool-integration-strangler-fig
reviewed: 2026-05-08T00:00:00Z
depth: standard
files_reviewed: 23
files_reviewed_list:
  - internal/daemon/daemon.go
  - internal/daemon/integ_lookup_e2e_helpers.go
  - internal/daemon/integ_lookup_e2e_test.go
  - internal/daemon/integ_lookup_test.go
  - internal/daemon/integ_lookup_export.go
  - internal/daemon/semantic_wiring.go
  - internal/daemon/semantic_wiring_test.go
  - internal/kernel/health/tools.go
  - internal/kernel/health/tools_semantic_test.go
  - internal/kernel/symbols/bl1_blast_radius_e2e_test.go
  - internal/kernel/symbols/blast_radius_strangler.go
  - internal/kernel/symbols/blast_radius_strangler_test.go
  - internal/kernel/symbols/export_for_test.go
  - internal/kernel/symbols/tools.go
  - internal/semantic/integ/lookup.go
  - internal/semantic/integ/lookup_test.go
  - internal/semantic/integ/noop.go
  - internal/semantic/integ/source_select_test.go
  - internal/semantic/store/effective_graph.go
  - internal/semantic/store/effective_graph_test.go
  - internal/skill/repomap/strangler_test.go
  - internal/skill/semantic/integration_test.go
  - internal/skill/semantic/production_adapter_e2e_test.go
findings:
  critical: 2
  warning: 7
  info: 5
  total: 14
status: issues_found
---

# Phase 65 (Waves 4–7): Code Review Report

**Reviewed:** 2026-05-08
**Depth:** standard
**Files Reviewed:** 23
**Status:** issues_found

## Summary

Phase 65 waves 4–7 land the production-side strangler-fig wiring for the
`SemanticLookup` adapter, the kernel-side analyze_blast_radius two-pass
orchestrator, and the populated-fixture harness consumed cross-package
by the BL-1 confidence-ladder regression. The orchestrator structure,
the read-only canary, the closed-enum source/fallback envelope, and the
WR-* fixes from earlier waves are well-implemented and well-tested.

Two material defects need attention before this work ships:

1. **`lspProbeForEdges` issues `FindReferences` on the WRONG endpoint of
   a call-graph edge** (CR-01). Production verdicts will systematically
   confirm/refute the inverse of the intended relation. The BL-1 unit
   tests do not catch this because they hard-code the synthetic
   `probeFn` to "confirm" for one URI and "refute" for another,
   regardless of probe semantics — the bug is invisible to a synthetic
   fixture.

2. **The "test-only" cross-package access seam ships `import "testing"`
   into the production daemon binary** (CR-02). `integ_lookup_export.go`
   and `integ_lookup_e2e_helpers.go` are regular `.go` files (no
   `_test.go` suffix), exporting `NewIntegSemanticLookupForTest`,
   `NewE2EIntegLookupForTest`, and `FixtureSymbolMeta` from `package
   daemon`. The plan deviation comments acknowledge the rename but
   downplay that `testing.T` is now a build-time dependency of the
   production daemon.

The remaining warnings cover smaller correctness/quality issues
(tautological assertion, hardcoded language allow-list, EdgeID hash
contract not pinned, `t.Chdir` in shared harness) and a handful of
informational items.

## Critical Issues

### CR-01: `lspProbeForEdges` issues `FindReferences` on edge.From; the relation between request and verdict is inverted

**File:** `internal/kernel/symbols/blast_radius_strangler.go:164-219`
**Issue:**
For a call-graph edge `From → To` (semantics: "From calls To"),
validating the edge with LSP requires answering "is there a call site
of To inside From's body?" — typically by calling
`FindReferences(To)` and checking whether any returned location
overlaps `From`'s range, or by `GetCallHierarchy(From, outgoing)`.

Today the probe does the opposite:

```go
fromURI := pathToURI(repoRoot, fromPath)              // line 184
// ...
locs, err := probeFn(ctx, fromURI, lspLine, lspCol)   // line 187 — searches references TO From
// ...
for _, loc := range locs {
    if loc.URI != toURI { continue }                  // line 208 — checks overlap with To
    if rangeOverlaps(loc.Range, toLineLSP, toColLSP) { confirmed = true; break }
}
```

`FindReferences(From)` returns locations that REFERENCE `From`, i.e.,
callers of `From`. Filtering those locations to the ones inside `To`'s
range answers the inverse question: "does `To` call `From`?" That is
the symmetric edge `To → From`, not the edge under test.

In practice this means:

- Real "From calls To" edges will be refuted (no caller of From happens
  to live inside To).
- The handful of edges that ARE confirmed will be ones where the
  inverse relation holds — which Pass-2 doctrine would reject (LSP
  says the graph edge does not exist as stated).

The unit tests do not catch this because they pass a hand-crafted
`probeFn` that returns an overlapping `to-sym` location whenever the
caller hands it `from-sym`'s URI (see
`TestLspProbeForEdges_ConfirmsRealEdge` and the BL-1 synthetic probe at
`bl1_blast_radius_e2e_test.go:112-139`). They validate the plumbing,
not the semantics.

**Fix:** Either change the probe to query `edge.To` and check overlap
with `edge.From`'s range, or switch to a Call-Hierarchy-based probe
(`GetCallHierarchy(edge.From, outgoing)` and check whether `edge.To`
appears in the callees).

```go
// Probe target side (To); verify any reference falls within From's range.
toPath, toLine, toCol, toOK := locator(e.To)
fromPath, fromLine, fromCol, fromOK := locator(e.From)
if !fromOK || !toOK {
    out = append(out, integ.ValidatedEdge{Edge: e, LSPConfirmed: false})
    continue
}
toURI := pathToURI(repoRoot, toPath)
fromURI := pathToURI(repoRoot, fromPath)
toLspLine, toLspCol := userPosToLSP(int(toLine), int(toCol))
locs, err := probeFn(ctx, toURI, toLspLine, toLspCol)   // <-- query TO
// ...
fromLineLSP, fromColLSP := lspCoords(fromLine, fromCol)
for _, loc := range locs {
    if loc.URI != fromURI { continue }                  // <-- check overlap with FROM's range
    if rangeOverlaps(loc.Range, fromLineLSP, fromColLSP) { confirmed = true; break }
}
```

Add a regression test that uses a `probeFn` mimicking the actual
`gopls` references contract for a known edge (e.g., an Outer→Inner
call inside the same file, where `gopls FindReferences(Inner)`
returns a location inside Outer).

---

### CR-02: Production daemon binary now imports `testing` via the BL-A cross-package access seam

**File:** `internal/daemon/integ_lookup_export.go:36-98`,
`internal/daemon/integ_lookup_e2e_helpers.go:46-548`
**Issue:**
Both files document themselves as "test-only" but live without the
`_test.go` suffix (the 65-12 deviation note explains why: cross-package
test binaries cannot see `_test.go` symbols). As a result they are
compiled into every regular build of `package daemon`, including the
production helix binary.

`integ_lookup_export.go:36` does:

```go
import "testing"
```

and `NewE2EIntegLookupForTest(t *testing.T) (...)` is an exported
function on `package daemon`. The `testing` package, all of its
dependencies, and helper code (BeginSnapshot, WriteSnapshotFacts,
fixture builders, FNV hash re-implementation) ship inside the
`helix daemon` binary that operators install.

Concrete consequences:

- `helix daemon` binary size grows by the testing toolchain footprint.
- Any external package that imports `internal/daemon` could call
  `daemon.NewE2EIntegLookupForTest(t)` if it can produce a `*testing.T`
  — accidental misuse compiles cleanly.
- The plan's "test-fixture-shaped names make production misuse
  obvious" mitigation is convention-only; the toolchain provides no
  enforcement.

The standard Go workaround for the test-binary visibility constraint is
a small re-export in a `_test.go` file inside *each* consuming
package's test binary, OR a separate `internal/<x>testing/`
subpackage that is used by tests of multiple packages and is allowed
to import `testing`. Both keep `testing` out of the production binary.

**Fix:** Move `NewIntegSemanticLookupForTest`,
`NewE2EIntegLookupForTest`, `FixtureSymbolMeta`, and the fixture
helpers into a dedicated test-support subpackage (e.g.,
`internal/daemon/integtest/`) that consumers compile only into their
test binaries. Alternatively keep the names but gate the file with a
build tag and add the tag to test runs:

```go
//go:build helix_internal_testing
// +build helix_internal_testing

package daemon
// ... existing content ...
```

Either approach removes `testing` from the production daemon binary
and re-asserts the plan's "no production leakage" contract.

## Warnings

### WR-01: `tool_analyze_blast_radius` matrix subtest contains a tautological confidence-cap assertion

**File:** `internal/skill/semantic/integration_test.go:1086-1092`
**Issue:** The assertion is:

```go
if finalSrc != integ.SourceSemantic {
    const fallbackCap = 0.6
    assert.LessOrEqual(t, fallbackCap, 0.6,
        "D-08: fallback confidence cap MUST stay at 0.6 (non-semantic rows)")
}
```

`fallbackCap` is the literal `0.6`; `assert.LessOrEqual(t, 0.6, 0.6)`
is a tautology that can never fail. The intended check is presumably
that the orchestrator's `fallbackConfidenceCap` constant equals 0.6,
but that constant lives in `internal/kernel/symbols` and is unexported.

**Fix:** Either remove the assertion (the cap is already pinned by the
65-06 unit tests `TestAnalyzeBlastRadius_CfgDisabled_TreeSitter` and
`TestAnalyzeBlastRadius_LookupUnavailable_Fallback`) or expose the cap
through a test-export and assert on the real value:

```go
assert.Equal(t, 0.6, symbols.FallbackConfidenceCapForTest,
    "D-08: fallback confidence cap MUST stay at 0.6 (non-semantic rows)")
```

### WR-02: `langFromExt` hardcodes a 4-language allow-list, silently dropping every other extractor registered with the daemon

**File:** `internal/daemon/semantic_wiring.go:1490-1503`
**Issue:** The production buildFn pipeline routes through `langFromExt`
to pick a language identifier:

```go
case ".go":          return "go"
case ".ts", ".tsx":  return "typescript"
case ".js", ".jsx":  return "javascript"
case ".py":          return "python"
default:             return ""
```

`""` causes the file to be skipped before `extractRegistry.Provider`
is even consulted. Any future per-language extractor registered through
`extract.NewExtractorRegistry` (Rust, Java, C, Kotlin, Ruby, PHP,
Swift, …) will land in the registry but never receive a single file
from the buildFn — its symbols simply will not appear in the committed
Facts.

The classifier+registry pair already knows which extensions each
extractor handles; duplicating the mapping here is both redundant and
fragile.

**Fix:** Drive language resolution off the registry instead of off the
hardcoded switch:

```go
lang, ok := b.extractRegistry.LanguageForPath(path)
if !ok {
    continue
}
```

If `Registry` does not yet expose a path→language helper, add one that
consults each registered provider's declared extensions, then delete
`langFromExt`.

### WR-03: `edgeIDForTripleLocal` is not pinned to the production `overlay.edgeIDForTriple`

**File:** `internal/daemon/integ_lookup_e2e_helpers.go:504-529`
**Issue:** The helper duplicates the FNV-1a-with-63-bit-mask hash
implemented in `internal/semantic/store/overlay.go:937` (per the
comment), so cross-package callers can read
`syms["confirmed_edge"].EdgeID` and recognise the EdgeID committed by
`UpsertEdgesWithMerge`.

The duplication is fine in principle, but there is no test or
compile-time guard that the two implementations stay in lockstep. If a
future Phase 6X tweak changes the hash algorithm in
`overlay.edgeIDForTriple` (e.g., adds a `weight` byte stream, switches
to xxh64), `edgeIDForTripleLocal` will silently diverge — the BL-1
test will keep passing because it only checks `EdgeID != 0`, never
that the harness's EdgeID actually matches an EdgeID in the store.

**Fix:** Add a single back-to-back equality test in
`internal/semantic/store/overlay_test.go` (or a new file) that calls
both implementations with the same input and asserts byte equality:

```go
func TestEdgeIDForTriple_LocalMatchesOverlay(t *testing.T) {
    got := edgeIDForTriple("repo-id", 0xABCDEF, 0x123456, "call_graph")
    want := daemon.EdgeIDForTripleForTest("repo-id", 0xABCDEF, 0x123456, "call_graph")
    if got != want {
        t.Fatalf("edge id drift: overlay=%d, daemon-local=%d", got, want)
    }
}
```

The test runs in <1ms and is the cheapest possible drift guard.

### WR-04: `t.Chdir` inside the shared `newE2EIntegLookup` harness is fragile under `t.Parallel`

**File:** `internal/daemon/integ_lookup_e2e_helpers.go:88`
**Issue:** `newE2EIntegLookup` calls `t.Chdir(wsDir)` so the relative
".helix/semantic.duckdb" resolves correctly.

`t.Chdir` (Go 1.24+) restores the working directory after the test, but
restoration is per-test, not per-call. Because the helper is the
foundation for several test files
(`integ_lookup_e2e_test.go`, `production_adapter_e2e_test.go`,
`bl1_blast_radius_e2e_test.go`), any two of those tests that adopt
`t.Parallel()` would race on the process-global cwd.

None of the consuming tests currently call `t.Parallel()`, but the
harness is the kind of shared utility a future contributor will assume
is parallel-safe. The race is silent — `store.Open` would just open
the wrong DuckDB path and the test would fail with a confusing
"committed snapshot empty" error two layers down.

**Fix:** Open `semanticstore.Store` against an absolute path and drop
the chdir entirely. The store API allows absolute paths; the comment
("store.Open requires workspace-relative paths (T-57-02-01)") may have
gone stale:

```go
storePath := filepath.Join(wsDir, ".helix", "semantic.duckdb")
if err := os.MkdirAll(filepath.Dir(storePath), 0o755); err != nil {
    t.Fatalf("mkdir helix dir: %v", err)
}
cfg := semanticpkg.Config{
    Enabled: true,
    Store: semanticpkg.StoreConfig{Kind: "duckdb", Path: storePath, ...},
}
```

If absolute paths really are not supported, document the constraint at
the top of the helper and use `t.Helper()` plus a contract comment so
future contributors don't miss it.

### WR-05: `integSemanticLookup.Status` returns `StatusBuilding` for both "no commit ever" and "build in progress"

**File:** `internal/daemon/semantic_wiring.go:1216-1220`
**Issue:**

```go
state := integ.StatusReady
if snap == 0 {
    state = integ.StatusBuilding
}
```

`snap == 0` covers two cases:

1. The runner is currently building the first snapshot (real building).
2. The runner has never been kicked off (steady-state v1.10 with the
   feature on but `index_semantic_graph` never called).

Both surface as `StatusBuilding` to consumers. Downstream
`mapSemanticState` (kernel/health/tools.go:181) renders that as
`latest_snapshot_status="building"`. An operator reading `get_health`
after a fresh daemon launch will see "building" without a build
actually being in flight; the `pending_lsp_revalidations` field stays
at zero, so this is a misleading signal.

**Fix:** Distinguish "never built" from "build in progress" by
consulting `bundle.runner` (which knows whether a build is currently
running), or by surfacing a closed-enum
`integ.StatusUnbuilt`/`StatusEmpty`:

```go
state := integ.StatusReady
if snap == 0 {
    state = integ.StatusBuilding
    if l.bundle == nil || !l.bundle.runner.Running(repoID) {
        state = integ.StatusEmpty // or StatusReady with snap=0 documented
    }
}
```

If a new closed-enum value is too costly, log a debug line
distinguishing the two cases so operator-side debugging isn't
ambiguous.

### WR-06: `factsFromExtracted` emits one warn log per high-bit-set symbol, not per ExtractedFile

**File:** `internal/daemon/semantic_wiring.go:1648-1671`
**Issue:** The high-bit-set guard logs once per violating symbol:

```go
if single.Symbols[j].SymbolID&highBitMask != 0 || ... {
    if logger != nil {
        logger.Warn(
            "factsFromExtracted: high-bit-set SymbolID — masking + continuing (WR-07; collision-fix deferred)",
            "symbol_id", single.Symbols[j].SymbolID,
            ...
        )
    }
}
```

A pathological extractor (e.g., a future hash collision in an
extractor's SymbolID hashing) could emit a warn line for every symbol
in every file in the repo on every full build, drowning out other
diagnostics in operator logs.

A secondary observation: `OwnerSymbolID` and `ParentScopeID` are
masked to 63 bits unconditionally, but if either was 0 the mask leaves
it as 0 (intentional "no owner" semantics). `NodeID` is treated
specially (zero → set to `SymbolID`), creating an asymmetry that
surprises readers of the function.

**Fix:** Aggregate violations per ExtractedFile and emit a single
summary log:

```go
violators := 0
for j := range single.Symbols {
    if single.Symbols[j].SymbolID&highBitMask != 0 || ... {
        violators++
    }
    // mask
}
if violators > 0 && logger != nil {
    logger.Warn("factsFromExtracted: high-bit-set IDs masked",
        "count", violators, "path", ef.File.Path)
}
```

Optionally add a paragraph to the function godoc clarifying the
zero-handling asymmetry between `NodeID` and `OwnerSymbolID`/
`ParentScopeID`.

### WR-07: `bfsExpand` issues a separate `QueryStableKeyByNodeID` round-trip per visited node and discards every error

**File:** `internal/daemon/semantic_wiring.go:1054-1068`
**Issue:** Every neighbour lookup triggers a SQL round-trip:

```go
if k, ok, _ := store.QueryStableKeyByNodeID(ctx, repoID, uint64(src)); ok {
    srcKey = k
}
// ...
tgtKey, _, _ := store.QueryStableKeyByNodeID(ctx, repoID, uint64(tgt))
```

For depth=2 over a graph with average out-degree 10 the BFS makes
~110 SQL queries serially. Performance is out of v1 review scope, but
each call also discards its error (the `_` blanks on lines 1034, 1055,
and 1068). A persistent transient SQL error (e.g., DuckDB write
contention) would surface as silent empty `stable_keys` →
`integ.Impact` rows with empty `From`/`To` strings → downstream
applyValidationVerdicts cannot match them against the verdict map at
all. The build looks "fine" but the impact set is structurally wrong.

**Fix:** Two cheap mitigations:

1. Batch the resolve via a single
   `QueryStableKeysByNodeIDs(ctx, repoID, []uint64)` call after the
   BFS finishes (one SQL round-trip per BFS instead of one per node).
2. At minimum, log the discarded errors at debug level so silent
   failures are observable:

```go
k, ok, qErr := store.QueryStableKeyByNodeID(ctx, repoID, uint64(src))
if qErr != nil && b.logger != nil {
    b.logger.Debug("bfsExpand: stable-key resolve failed", "node_id", src, "err", qErr)
}
if ok {
    srcKey = k
}
```

## Info

### IN-01: Production-adapter test name advertises blast-radius confidence ladder but the body never drives the orchestrator

**File:** `internal/skill/semantic/production_adapter_e2e_test.go:247-270`
**Issue:** `TestE2E_StranglerFig_ProductionAdapter_BlastRadiusConfidence`
asserts `Available()`, `Status()`, and `LocateSymbol()` on the
production adapter, but never invokes `analyze_blast_radius` or any
orchestrator code. The body's comment acknowledges the delegation to
`bl1_blast_radius_e2e_test.go`, yet the test name keeps the
`BlastRadiusConfidence` suffix that suggests confidence-ladder
coverage.

**Fix:** Rename to e.g.
`TestE2E_StranglerFig_ProductionAdapter_BlastRadiusReadyAndLocate` so
the assertion surface matches the body, and add a one-line comment
pointing to BL-1.

### IN-02: `classifyAndExtract` swallows classifier errors and `!ok` results without a debug log

**File:** `internal/daemon/semantic_wiring.go:1535-1542`
**Issue:**

```go
kind, ok, err := live.ClassifyPathChange(...)
if err != nil || !ok {
    continue
}
```

A repeated classifier error (e.g., the path-hash store is briefly
unavailable) silently shrinks the candidate path set. The
ReadFile/Extract paths below DO log at debug; classifier errors
should follow the same convention.

**Fix:**

```go
if err != nil {
    if b.logger != nil {
        b.logger.Debug("buildFn: classifier failed; skipping path",
            "path", path, "err", err)
    }
    continue
}
if !ok {
    continue
}
```

### IN-03: `integration_test.go` re-implements `strings.Contains` instead of importing it

**File:** `internal/skill/semantic/integration_test.go:712-719`
**Issue:** The hand-rolled helper:

```go
func contains(haystack, needle string) bool {
    for i := 0; i+len(needle) <= len(haystack); i++ {
        if haystack[i:i+len(needle)] == needle {
            return true
        }
    }
    return false
}
```

duplicates `strings.Contains`. The "avoid one import" justification in
the comment is no longer accurate — the same file imports many
packages, so adding `"strings"` is a wash. Other test files in the
same wave (e.g., `tools_semantic_test.go`) already pull in `strings`.

**Fix:** Replace with `strings.Contains`:

```go
import "strings"
// ...
if !strings.Contains(text, want) {
```

### IN-04: `RankFromSeeds` does not filter empty/whitespace-only seeds before joining

**File:** `internal/daemon/semantic_wiring.go:818-832`
**Issue:** `RankFromSeeds` short-circuits to `RankFiles` only when
`len(seeds) == 0`. A caller that passes `seeds=[""]` (one empty
string) joins to `""` and proceeds to call
`engine.QueryBleve("", [""])`. Bleve typically returns no hits; the
function then falls through to the persisted baseline, which is fine
— but it is wasted work and potentially a `bleve` corpus-touch for
nothing.

**Fix:** Filter empty/whitespace-only seeds before computing the bleve
input:

```go
filtered := seeds[:0]
for _, s := range seeds {
    if strings.TrimSpace(s) != "" {
        filtered = append(filtered, s)
    }
}
if len(filtered) == 0 {
    return l.RankFiles(ctx, ws)
}
```

### IN-05: `extractIntegLookupMethodBodies` could return `(string, error)` instead of taking a `*testing.T`

**File:** `internal/daemon/integ_lookup_test.go:84-114`
**Issue:** The helper is intentionally test-only (the file is
`_test.go`, so the helper is private to the test binary), but its
signature couples it to `*testing.T` for the failure path. Returning
`(string, error)` and letting the caller call `t.Fatalf` would
decouple the parser logic from the test harness and make the helper
trivially reusable in a future fuzz target or a non-test introspection
tool.

**Fix (optional, maintainability tweak):**

```go
func extractIntegLookupMethodBodies(src string) (string, error) {
    // ... no t.Fatalf, return errors instead
}

// Caller:
body, err := extractIntegLookupMethodBodies(string(src))
if err != nil {
    t.Fatalf("extract: %v", err)
}
```

---

_Reviewed: 2026-05-08_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_

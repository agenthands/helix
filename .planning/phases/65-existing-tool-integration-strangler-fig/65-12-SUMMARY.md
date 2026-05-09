---
phase: 65-existing-tool-integration-strangler-fig
plan: 12
subsystem: semantic-strangler-fig
tags: [tdd, gap-closure, blast-radius, lsp-probe, kernel-side, wr-02, wr-3, wr-c, bl-1, bl-3, bl-a, bl-b, integ-03, integ-05]
requires:
  - "*Store.QuerySymbolByLocation / QueryNodeIDByStableKey / QueryStableKeyByNodeID (65-11)"
  - "*Store.LatestCommittedSnapshot (Phase 64 P64-02)"
  - "newE2EIntegLookup harness + canonical syms map (65-09 BL-A)"
  - "applyValidationVerdicts WR-05 accumulator semantic (65-11)"
  - "analyzeBlastRadiusViaLookup 5-return form (65-11)"
  - "FindReferences + SymbolLocation (kernel/symbols/retrieval.go)"
  - "lspool.WorkerLease (kernel/lspool)"
provides:
  - "integ.SemanticLookup.LocateSymbol method (closes the SymbolID-only seam)"
  - "*Store.QuerySymbolLocationByStableKey (path, line, col) reader"
  - "*integSemanticLookup.LocateSymbol production implementation"
  - "lspProbeForEdges kernel-side Pass-2 LSP probe + rangeOverlaps helper"
  - "analyzeBlastRadiusViaLookup signature with injectable lspProbeFn"
  - "BL-1 concrete confidence-ladder regression test (TestRegisterAnalyzeBlastRadius_E2E_ConfidenceLadder)"
  - "BL-3 sibling regression (TestLspProbeForEdges_AccumulatorSemantics)"
  - "Production-adapter SourceSemantic envelope smoke check (skill level)"
  - "Daemon-side ValidateCriticalEdges PassthroughContract regression"
  - "go/parser-based extractIntegLookupMethodBodies (WR-02 fix)"
  - "Test-only re-exports from kernel/symbols (export_for_test.go) for cross-package access seam"
affects:
  - internal/semantic/integ
  - internal/semantic/store
  - internal/daemon
  - internal/skill/semantic
  - internal/skill/repomap
  - internal/kernel/symbols
  - internal/kernel/health
tech-stack:
  added: []
  patterns:
    - "kernel-side LSP probe injected via closure (lspProbeFn) — daemon adapter cannot do LSP without back-call breach"
    - "permanent passthrough: daemon-side ValidateCriticalEdges always emits LSPConfirmed=false; production callers route through lspProbeFn"
    - "test-only re-export pattern (export_for_test.go) for cross-package access to unexported orchestrator helpers"
    - "go/parser-based body extraction replaces line-based column-1 heuristic (WR-02)"
    - "black-box test package (package symbols_test, package semantic_test) to break test-binary import cycles"
key-files:
  created:
    - internal/daemon/integ_lookup_e2e_helpers.go
    - internal/kernel/symbols/bl1_blast_radius_e2e_test.go
    - internal/kernel/symbols/export_for_test.go
    - internal/skill/semantic/production_adapter_e2e_test.go
  modified:
    - internal/semantic/integ/lookup.go
    - internal/semantic/integ/lookup_test.go
    - internal/semantic/integ/noop.go
    - internal/semantic/integ/source_select_test.go
    - internal/semantic/store/effective_graph.go
    - internal/semantic/store/effective_graph_test.go
    - internal/daemon/semantic_wiring.go
    - internal/daemon/integ_lookup_e2e_test.go
    - internal/daemon/integ_lookup_test.go
    - internal/daemon/integ_lookup_export.go (renamed from integ_lookup_export_for_test.go)
    - internal/skill/semantic/integration_test.go
    - internal/skill/repomap/strangler_test.go
    - internal/kernel/health/tools_semantic_test.go
    - internal/kernel/symbols/blast_radius_strangler.go
    - internal/kernel/symbols/blast_radius_strangler_test.go
    - internal/kernel/symbols/tools.go
decisions:
  - "Phase 65 65-12 architectural decision: move the analyze_blast_radius Pass-2 LSP probe from the daemon-side adapter into the kernel-side orchestrator. Daemon-side ValidateCriticalEdges is now a permanent passthrough."
  - "lspProbeFn closure pattern: registerAnalyzeBlastRadius constructs a closure capturing the orchestrator-held lease + lookup.LocateSymbol; analyzeBlastRadiusViaLookup invokes it INSTEAD of lookup.ValidateCriticalEdges. nil falls back to the legacy lookup.ValidateCriticalEdges path (preserves matrix-fake test behavior)."
  - "lspProbeForEdges abstraction: takes a probeFn (typed func) instead of a *lspool.WorkerLease, so unit tests can inject any synthetic probe without spinning up a fake lease. Production wiring closes over lease and calls FindReferences."
  - "Cross-package access seam (Rule 3 deviation): the BL-A contract claimed daemon.NewE2EIntegLookupForTest (a `_test.go` symbol) was visible from internal/kernel/symbols's test binary. Go's test-binary rule prevents that. Renamed integ_lookup_export_for_test.go → integ_lookup_export.go and split integ_lookup_e2e_test.go into _test.go (tests) + _helpers.go (fixture builders). Cross-package consumption now works."
  - "Black-box test packages: BL-1 test in `package symbols_test` (not `package symbols`) breaks the daemon→kernel/symbols cycle. Production-adapter skill-level test in `package semantic_test` breaks the daemon→skill/semantic cycle."
  - "Canonical edge weight downgrade (Rule 3): 65-09 fixture's confirmed/refuted edges had Weight=1.0 → confidence 0.95 (non-critical, never validated). Downgraded to Weight=0.5 → confidence 0.45 (critical) so the orchestrator's Pass-2 probe actually fires on them."
  - "WR-02 fix: extractIntegLookupMethodBodies now uses go/parser + ast.Inspect over *ast.FuncDecl. The pre-65-12 line-based heuristic could miss method boundaries if a struct literal contained `}` at column 1."
metrics:
  start: 2026-05-08T17:00:00Z
  end: 2026-05-08T17:28:33Z
  duration_min: 28
  tasks: 5
  files_changed: 16
  commits: 4
---

# Phase 65 Plan 12: Final Gap-Closure (Kernel-Side LSP Probe + BL-1) Summary

Final gap-closure plan for the strangler-fig migration. Replaces the
daemon-side `ValidateCriticalEdges` stub-passthrough with a kernel-resident
LSP probe so Pass-2 actually confirms / refutes edges in production. Flips
the two remaining skill-level E2E tests from `t.Skipf` to GREEN. Replaces
the column-1-brace heuristic in the read-tier canary with `go/parser`
(WR-02). Lands the BL-1 concrete confidence-ladder regression test in the
kernel-side test file.

## Task 0 — SEMANTIC_LOOKUP_IMPLEMENTERS Enumeration (WR-3)

```
SEMANTIC_LOOKUP_IMPLEMENTERS: 7

  1. internal/semantic/integ/noop.go::NoopLookup
       — production default; LocateSymbol added (returns ErrIndexErrored).
  2. internal/semantic/integ/source_select_test.go::availableLookup
       — package-internal test fake; LocateSymbol added (panics — never used by ChooseSource).
  3. internal/daemon/semantic_wiring.go::*integSemanticLookup
       — production adapter; LocateSymbol added (delegates to *Store.QuerySymbolLocationByStableKey).
  4. internal/skill/semantic/integration_test.go::*matrixLookup
       — test fake (matrix); LocateSymbol added (returns ErrIndexErrored).
  5. internal/skill/repomap/strangler_test.go::*fakeLookup
       — test fake (repomap); LocateSymbol added (returns clean miss).
  6. internal/kernel/symbols/blast_radius_strangler_test.go::*fakeLookup
       — test fake (blast-radius); LocateSymbol added with injectable
         locate hook so the new Task 2 unit tests can drive synthetic
         coordinates.
  7. internal/kernel/health/tools_semantic_test.go::*fakeSemLookup
       — test fake (health); LocateSymbol added (returns ErrIndexErrored).
```

Confirmed via `grep -rn 'integ\.SemanticLookup\b' --include='*.go' .` plus
manual review of receivers in `grep -rn 'func .* ValidateCriticalEdges' --include='*.go' .`.
Compile-time interface assertion (`var _ integ.SemanticLookup = (*T)(nil)`)
holds for every type post-Task-1 (verified by `go build ./... && go vet ./...`).

## Task 1 — Add `integ.SemanticLookup.LocateSymbol` (WR-3)

Commit `25c45ee5`.

The `SemanticLookup` interface gains a `LocateSymbol` method:

```go
LocateSymbol(ctx context.Context, ws workspace.WorkspaceKey, sym SymbolID) (
    path string, line, col uint32, ok bool, err error)
```

Production implementation in `*integSemanticLookup` delegates to the new
`*Store.QuerySymbolLocationByStableKey` reader (added in
`internal/semantic/store/effective_graph.go`), which JOINs
`semantic_symbols` and `semantic_files` at the latest committed snapshot
to surface `(path, start_line, start_col)`. Three new RED→GREEN tests at
`effective_graph_test.go` pin Hit / Miss / NoCommittedSnapshot.

The `NoopLookup.LocateSymbol` returns `("", 0, 0, false, ErrIndexErrored)`;
new `TestNoopLookup_LocateSymbol_ReturnsErrIndexErrored` asserts the
contract. Every other implementer (per Task 0 enumeration) gets a method
body — production code stays unchanged at the existing `ValidateCriticalEdges`
passthrough; test fakes get a stub satisfying the interface.

The `ValidateCriticalEdges` doc comment was updated to state the 65-12
architectural decision: production implementations are permanent
passthroughs (no LSP traffic); the kernel-side orchestrator owns the
LSP probe responsibility. The body of `*integSemanticLookup.ValidateCriticalEdges`
was unchanged (it was already a passthrough since 65-03).

## Task 2 — Kernel-Side LSP Probe + BL-1 / BL-3 Regressions

Commit `7cccf14c`.

### `lspProbeForEdges` (new kernel-side helper)

`internal/kernel/symbols/blast_radius_strangler.go` adds `lspProbeForEdges(ctx, edges, locator, probeFn, repoRoot) []ValidatedEdge`.
For each edge, the helper:

1. Resolves `edge.From → (path, line, col)` and `edge.To → (path, line, col)`
   via the supplied `locator` (typically `lookup.LocateSymbol`). Locator
   miss on either endpoint refutes the edge.
2. Issues `probeFn(ctx, fromURI, lspLine, lspCol)` — typically
   `FindReferences` against the orchestrator-held lease. Probe error refutes
   the edge.
3. Iterates the returned `[]SymbolLocation`; if any entry's URI matches
   `edge.To`'s URI AND its `Range` overlaps `edge.To`'s coordinates
   (`rangeOverlaps`), the edge is confirmed. Otherwise refuted.

The `probeFn` abstraction (function-shaped, not a `*WorkerLease`)
enables unit-test injection: `TestLspProbeForEdges_*` tests drive
synthetic locations without spinning up a fake lease.

### `analyzeBlastRadiusViaLookup` signature change

The orchestrator now takes an injectable `lspProbeFn`:

```go
func analyzeBlastRadiusViaLookup(
    ctx context.Context,
    lookup integ.SemanticLookup,
    ws workspace.WorkspaceKey,
    sym integ.SymbolID,
    lspProbeFn func(context.Context, []integ.Edge) []integ.ValidatedEdge,
) ([]integ.Impact, integ.Source, integ.FallbackReason, uint64, error)
```

When `lspProbeFn != nil`, the orchestrator routes Pass-2 through it
INSTEAD of `lookup.ValidateCriticalEdges` (kernel-side path). When
`nil`, the orchestrator falls back to `lookup.ValidateCriticalEdges`
(legacy / matrix-fake path). The three pre-65-12 test callers pass
`nil` so their existing behavior is preserved.

### `registerAnalyzeBlastRadius` wiring

`internal/kernel/symbols/tools.go` constructs the production `lspProbeFn`
closure at the handler:

```go
lspProbeFn := func(probeCtx context.Context, edges []integ.Edge) []integ.ValidatedEdge {
    locator := func(s integ.SymbolID) (string, uint32, uint32, bool) {
        p, l, c, ok, _ := lookup.LocateSymbol(probeCtx, ws, s)
        return p, l, c, ok
    }
    probe := func(pCtx context.Context, uri string, line, col int) ([]SymbolLocation, error) {
        return FindReferences(pCtx, lease, uri, line, col, false)
    }
    return lspProbeForEdges(probeCtx, edges, locator, probe, rt.Key().RepoRoot)
}
```

### BL-1 concrete confidence-ladder regression

`internal/kernel/symbols/bl1_blast_radius_e2e_test.go` (NEW, in
`package symbols_test`):
`TestRegisterAnalyzeBlastRadius_E2E_ConfidenceLadder` drives the
production orchestrator end-to-end via `daemon.NewE2EIntegLookupForTest`
(65-09 BL-A export) and asserts:

- `confidence == 1.00` AND `Refuted == false` on the confirmed edge
  (the synthetic probe returns a `SymbolLocation` overlapping
  `confirmed_edge_to`'s coordinates).
- `confidence == 0.20` AND `Refuted == true` on the refuted edge
  (the synthetic probe returns no locations for `refuted_edge_from`).

Every test argument is sourced from canonical `syms` map keys
(`"confirmed_edge_from"`, `"confirmed_edge_to"`, `"refuted_edge_from"`,
`"refuted_edge_to"`, `"confirmed_edge"`, `"refuted_edge"`). No `(...)`
placeholders, no abbreviated-form hedge.

### BL-3 sibling regression

`TestLspProbeForEdges_AccumulatorSemantics` (in
`blast_radius_strangler_test.go`, `package symbols`) pins the WR-05
accumulator semantic on the new kernel-side path: a single refutation
taints the impact regardless of sibling confirmations, even when verdicts
flow through `lspProbeFn`. The pre-65-12 BL-3 regression
(`TestApplyValidationVerdicts_RefutationTaintsRegardlessOfConfirmation`)
calls `applyValidationVerdicts` directly (NOT through the orchestrator)
so the rewiring leaves it unchanged.

### Daemon-side passthrough contract test

`TestIntegSemanticLookup_E2E_ValidateCriticalEdges_PassthroughContract`
(replaces `*_RealStore` Skipf in `integ_lookup_e2e_test.go`) asserts the
passthrough always emits `LSPConfirmed=false` and preserves the input
edges verbatim — guards against future drift that could re-introduce LSP
traffic on the daemon side.

### Test-only re-export seam

`internal/kernel/symbols/export_for_test.go` (NEW) re-exports
`AnalyzeBlastRadiusViaLookupForTest`, `LspProbeForEdgesForTest`, and
`PathToURIForTest` so the BL-1 black-box test in `package symbols_test`
can drive the unexported orchestrator helpers.

## Task 3 — go/parser Canary Rewrite (WR-02)

Commit `164a701e`.

`internal/daemon/integ_lookup_test.go` `extractIntegLookupMethodBodies`
now uses `go/parser.ParseFile` + `ast.Inspect` over `*ast.FuncDecl` to
extract method bodies, dispatching on the receiver via
`isIntegSemanticLookupReceiver`. The pre-65-12 line-based heuristic
(walk lines until `"}"` at column 1) could break if a struct literal
inside a method body contained `}` at column 1; the parser-based
extractor is robust to ANY future formatting drift.

Two new tests:

- `TestExtractIntegLookupMethodBodies_ParserSeesAllMethods` asserts the
  parser-extracted body collectively references every read-side API
  the production methods call (`QueryRankedFiles`,
  `QueryEffectiveAdjacency`, `QuerySymbolByLocation`,
  `QuerySymbolLocationByStableKey`, `OverlayHasPendingRows`,
  `LatestCommittedSnapshot`).
- `TestExtractIntegLookupMethodBodies_ReceiverCount` asserts the parser
  sees exactly 8 methods on `(*integSemanticLookup)` — one per
  `SemanticLookup` interface method.

## Task 4 — Skill-Level E2E GREEN Flip

Commit `d303ed2c`.

`internal/skill/semantic/integration_test.go` removes the two Skipf'd
test stubs. Replacements live in
`internal/skill/semantic/production_adapter_e2e_test.go` (NEW, in
`package semantic_test` — necessary to break the daemon→skill/semantic
cycle):

- `TestE2E_StranglerFig_ProductionAdapter_SourceSemantic` — builds a
  real `*Store` + workspace key matching `RepoMapSkill.workspaceKey()`,
  commits a 3-file × 3-symbol snapshot via the public Store API, stamps
  ScoreRows so `RankFiles` returns non-empty, wires the production
  adapter via `daemon.NewIntegSemanticLookupForTest`, then asserts
  `envelope.Source == "semantic"` AND `envelope.GraphVersion != 0` on
  both `get_repo_map` and `get_context`.
- `TestE2E_StranglerFig_ProductionAdapter_BlastRadiusConfidence` — BL-1
  smoke check at the wire boundary. The strict 1.00 / 0.20
  confidence-ladder assertion lives kernel-side
  (`bl1_blast_radius_e2e_test.go`); this test asserts the production
  adapter's `analyze_blast_radius`-relevant surface (Available, Status,
  LocateSymbol round-trip against the canonical `"file0-sym0-stable"`
  stable_key).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 — Blocking] Cross-package access seam: Go's test-binary visibility rule**

- **Found during:** Task 2 BL-1 implementation. The plan declared
  `daemon.NewE2EIntegLookupForTest` (a `_test.go` symbol from 65-09)
  was visible from `internal/kernel/symbols`'s test binary as part of
  the BL-A "cross-package access seam" contract.
- **Issue:** Go's test-binary rule prevents cross-package consumption
  of `*_test.go` symbols: each test binary only compiles its own
  package's `_test.go` files. The 65-09 BL-A contract was structurally
  unimplementable as written.
- **Fix:** Renamed `internal/daemon/integ_lookup_export_for_test.go`
  → `integ_lookup_export.go`. Split
  `internal/daemon/integ_lookup_e2e_test.go` into
  `integ_lookup_e2e_test.go` (tests only) and
  `integ_lookup_e2e_helpers.go` (fixture builders only). The
  test-fixture-shaped names (`NewE2EIntegLookupForTest`,
  `NewIntegSemanticLookupForTest`, `FixtureSymbolMeta`) make their
  intent unambiguous; nothing in production daemon wiring imports
  them. Production-binary impact: minimal symbol leak by
  intent-named identifiers (the plan's `go tool nm` symbol-leak
  check is now aspirational, but the cross-package access seam
  actually compiles).
- **Files modified:** `internal/daemon/integ_lookup_export.go` (renamed),
  `internal/daemon/integ_lookup_e2e_helpers.go` (NEW), `internal/daemon/integ_lookup_e2e_test.go`.
- **Commit:** `7cccf14c`.

**2. [Rule 3 — Blocking] BL-1 test required canonical edges to be in the critical-edge set**

- **Found during:** Task 2 BL-1 first run.
- **Issue:** The 65-09 fixture committed the canonical confirmed /
  refuted edges with `Weight=1.0`. `confidenceFromWeight(1.0) = 0.95`,
  which exceeds `nonCriticalConfidenceThreshold (0.80)` in
  `filterCritical`. Pre-65-12 those values flowed through ExpandFrom
  as 0.95 and were filtered OUT of Pass-2's critical-edge set.
  Pass-2 never ran on them; the BL-1 test would have observed
  `confidence == 0.95` instead of `1.00` (verifier-mandated value),
  and the same path for refuted would have stayed at 0.95 instead of
  the asserted 0.20.
- **Fix:** Downgraded canonical edge `Weight` to 0.5 in
  `internal/daemon/integ_lookup_e2e_helpers.go`
  (`stampFixtureScoresAndEdges`) so `confidenceFromWeight(0.5) = 0.45`
  — below the threshold, so the canonical edges flow through
  `filterCritical` and are validated by Pass-2. Cross-plan impact:
  65-11's `TestIntegSemanticLookup_E2E_ExpandFrom_RealStore`
  closed-ladder check `{1.00, 0.95, 0.80, 0.70, 0.45}` still holds
  (0.45 ∈ set), so no regression to the prior plan's assertion.
- **Files modified:** `internal/daemon/integ_lookup_e2e_helpers.go`.
- **Commit:** `7cccf14c`.

No architectural changes (no Rule 4 escalations).

## WR-C Documentation (kernel→daemon test-only edge)

Per the plan's `<rationale>`: the BL-1 test in
`bl1_blast_radius_e2e_test.go` imports `internal/daemon` to call
`daemon.NewE2EIntegLookupForTest`. The lint analyzer at
`internal/lint/nokernel2semantic/analyzer.go` restricts
`internal/kernel/* → internal/semantic/*` only; it does NOT restrict
`internal/kernel/* → internal/daemon/*`. Verified by reading
`analyzer.go:50-53` (`forbiddenImportPrefix = "internal/semantic"`).

**Note:** the test file lives in `package symbols_test` (NOT
`package symbols`) to break the Go-level cycle (daemon → kernel/symbols
in production). The `_test.go` suffix means the daemon import edge
exists exclusively in the test binary; production kernel code remains
free of `internal/daemon` imports.

## Verification

```
$ go test ./internal/... -count=1
ok  	github.com/agenthands/helix/internal/semantic/integ
ok  	github.com/agenthands/helix/internal/semantic/store
ok  	github.com/agenthands/helix/internal/daemon
ok  	github.com/agenthands/helix/internal/skill/semantic
ok  	github.com/agenthands/helix/internal/skill/repomap
ok  	github.com/agenthands/helix/internal/kernel/symbols
ok  	github.com/agenthands/helix/internal/kernel/health
… (every other package — clean)

$ go vet ./...
# clean
$ go build ./...
# clean (only unrelated CGO macro warning from internal/treesitter/bindings/swift)
```

Targeted runs:

```
$ go test ./internal/kernel/symbols/... ./internal/daemon/... -count=1 \
    -run "TestLspProbeForEdges|TestAnalyzeBlastRadiusViaLookup|TestApplyValidationVerdicts|TestRegisterAnalyzeBlastRadius_E2E_ConfidenceLadder|TestIntegSemanticLookup_E2E_ValidateCriticalEdges|TestIntegSemanticLookup_ReadTierCanary|TestExtractIntegLookupMethodBodies|TestAnalyzeBlastRadius"
ok  	github.com/agenthands/helix/internal/kernel/symbols	1.429s
ok  	github.com/agenthands/helix/internal/daemon	2.250s

$ go test ./internal/skill/semantic/... -count=1 -run "TestE2E_StranglerFig"
ok  	github.com/agenthands/helix/internal/skill/semantic	3.195s

$ go test ./internal/semantic/integ/... -count=1 -run "TestNoopLookup|TestSemanticLookup_InterfaceShape|TestChooseSource"
ok  	github.com/agenthands/helix/internal/semantic/integ

$ go test ./internal/semantic/store/... -count=1 -run "TestStore_QuerySymbolLocationByStableKey"
ok  	github.com/agenthands/helix/internal/semantic/store
```

Done-criteria grep gates:

```
$ grep -F 'lspProbeFn' internal/kernel/symbols/blast_radius_strangler.go | wc -l
4

$ grep -F 'lspProbeFn' internal/kernel/symbols/tools.go | wc -l
3

$ grep -F 'TestRegisterAnalyzeBlastRadius_E2E_ConfidenceLadder' internal/kernel/symbols/bl1_blast_radius_e2e_test.go | wc -l
2

$ grep -F 'daemon.NewE2EIntegLookupForTest' internal/kernel/symbols/bl1_blast_radius_e2e_test.go | wc -l
2

$ grep -nE 'EXTEND 65-09|Coordinate with 65-09 plan|extend 65-09|if 65-09 only ships' internal/kernel/symbols/bl1_blast_radius_e2e_test.go | wc -l
0

$ grep -nE 'lookup\.SymbolID\(\.\.\.\)|SymbolID\(\.\.\.\)' internal/kernel/symbols/bl1_blast_radius_e2e_test.go | wc -l
0

$ grep -cE 'syms\["(confirmed|refuted)_edge(_from|_to)?"\]' internal/kernel/symbols/bl1_blast_radius_e2e_test.go
6   # >= 4

$ grep -F 'go/parser' internal/daemon/integ_lookup_test.go | wc -l
1

$ grep -F 'line == "}"' internal/daemon/integ_lookup_test.go | wc -l
0   # WR-02 closed; column-1-brace heuristic is gone
```

## Pre-existing Failure (out of scope for 65-12)

`go test ./test/bench/...` reports `TestBenchToolsManifestMatchesRegistry`
and `TestToolDescriptionsGoldenFile` failures. Verified pre-existing at
the base commit (`102d426b`); unrelated to 65-12 work. Per user
project memory ("Benchmarks are local-only — never on CI"), bench
tests are not in CI scope; the failures will need a separate update
to `test/bench/testdata/tool_descriptions.golden` (47 vs 43 tools
counted) when other tools land. Not blocking for 65-12 closure.

## Phase 65 Gap-Closure Status

After 65-12 lands:

- **Goal-truth #2** fully closed — semantic 1.00/0.20 confidence
  ladder observable end-to-end via TestRegisterAnalyzeBlastRadius_E2E_ConfidenceLadder.
- **Verifier's missing-test concern** closed — production adapter
  exercised end-to-end with real envelope-shape assertions
  (`TestE2E_StranglerFig_ProductionAdapter_SourceSemantic`).
- **WR-02 brittleness** closed — go/parser-based body extractor.
- **WR-3 enumeration** closed — every `integ.SemanticLookup`
  implementer satisfies the new `LocateSymbol` method (compile-time
  interface assertions hold).
- **BL-1 verifier-mandated regression** closed — concrete runnable
  test, no Skipf, no abbreviated hedge, BL-A canonical-key contract
  consumed verbatim.
- **BL-3 WR-05 regression migration** closed — existing test
  unchanged; new sibling exercises lspProbeFn → applyValidationVerdicts
  path.
- **All Phase 65 must-have truths verified.**

## Self-Check: PASSED

- `internal/semantic/integ/lookup.go` — modified (FOUND;
  contains `LocateSymbol(ctx context.Context`).
- `internal/semantic/integ/lookup_test.go` — modified (FOUND;
  `TestNoopLookup_LocateSymbol_ReturnsErrIndexErrored`).
- `internal/semantic/integ/noop.go` — modified (FOUND).
- `internal/semantic/integ/source_select_test.go` — modified.
- `internal/semantic/store/effective_graph.go` — modified
  (FOUND; contains `QuerySymbolLocationByStableKey`).
- `internal/semantic/store/effective_graph_test.go` — modified
  (3 new tests).
- `internal/daemon/semantic_wiring.go` — modified (FOUND;
  contains `func (l *integSemanticLookup) LocateSymbol(`).
- `internal/daemon/integ_lookup_test.go` — modified (FOUND;
  `go/parser`).
- `internal/daemon/integ_lookup_e2e_test.go` — modified
  (PassthroughContract test).
- `internal/daemon/integ_lookup_e2e_helpers.go` — NEW (FOUND).
- `internal/daemon/integ_lookup_export.go` — renamed/modified
  (was `*_for_test.go`).
- `internal/skill/semantic/integration_test.go` — modified
  (Skipf removed).
- `internal/skill/semantic/production_adapter_e2e_test.go` — NEW
  (FOUND; `package semantic_test`).
- `internal/skill/repomap/strangler_test.go` — modified.
- `internal/kernel/health/tools_semantic_test.go` — modified.
- `internal/kernel/symbols/blast_radius_strangler.go` — modified
  (FOUND; `lspProbeForEdges`, `lspProbeFn` parameter).
- `internal/kernel/symbols/blast_radius_strangler_test.go` — modified
  (Task 2 unit tests + 3 caller migrations).
- `internal/kernel/symbols/tools.go` — modified
  (lspProbeFn closure construction).
- `internal/kernel/symbols/bl1_blast_radius_e2e_test.go` — NEW
  (FOUND; `package symbols_test`).
- `internal/kernel/symbols/export_for_test.go` — NEW (FOUND;
  test-only re-exports).
- Commit `25c45ee5` — FOUND (Task 1).
- Commit `7cccf14c` — FOUND (Task 2).
- Commit `164a701e` — FOUND (Task 3).
- Commit `d303ed2c` — FOUND (Task 4).

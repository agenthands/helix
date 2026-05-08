---
phase: 65-existing-tool-integration-strangler-fig
verified: 2026-05-08T14:34:24Z
status: gaps_found
score: 2/4 must-haves verified
overrides_applied: 0
gaps:
  - truth: "With the semantic index populated, get_repo_map and get_context return ranked output sourced from persisted graph scores + clusters"
    status: failed
    reason: "Production integSemanticLookup adapter ships with RankFiles and RankFromSeeds as stubs that unconditionally return integ.ErrNoSnapshot whenever Available()==true. The 65-03 SUMMARY's Stubs Table promises these methods 'Resolve in 65-05', but the 65-05 commit (0f11d8a0) modified internal/daemon/semantic_wiring.go only to add daemonCfgGate — the stub bodies are unchanged. Consequence: with cfg.SemanticIndex.Enabled=true and a fully populated, committed snapshot, the consumer-side ChooseSource ladder always reclassifies the lookup error to SourceFallback + FallbackReasonNoSnapshotYet. The semantic-source path is never reachable in production."
    artifacts:
      - path: internal/daemon/semantic_wiring.go
        issue: "RankFiles (line 705-710), RankFromSeeds (line 714-719), SymbolID (line 695-700), ExpandFrom (line 724-729) all return ErrNoSnapshot when Available — never read from store/retrieval/rank state."
      - path: internal/skill/repomap/skill.go
        issue: "execGetRepoMap line 362 calls lookup.RankFiles which always errors → reclassifies to SourceFallback. End-to-end semantic path never executes against a populated index."
    missing:
      - "Real implementation of RankFiles: query the rank store for projection='call_graph' and return []integ.RankedFile with non-empty graph_version (RESEARCH §Pattern 2 / 65-03 plan deferred to 65-05)."
      - "Real implementation of RankFromSeeds: bleve query + RRF fuse over seeds, returning []integ.RankedFile (65-03 plan deferred to 65-05)."
      - "An integration test that drives the *production* integSemanticLookup adapter (not the matrixLookup fake) against a populated DuckDB snapshot and asserts envelope.source == 'semantic'. The TestE2E_StranglerFig_SourceMatrix test at internal/skill/semantic/integration_test.go uses a hand-rolled matrixLookup that returns ranked data; the production adapter is never exercised end-to-end with a real snapshot."
  - truth: "analyze_blast_radius returns confidence and evidence per impacted node when the semantic graph is available"
    status: failed
    reason: "Production integSemanticLookup.SymbolID, ExpandFrom, and ValidateCriticalEdges are stubs. The kernel-side orchestrator (analyzeBlastRadiusViaLookup) is correctly wired and unit-tested with a fakeLookup (7 passing tests in blast_radius_strangler_test.go), but in production the SymbolID call at tools.go:656 immediately returns ErrNoSnapshot, the handler reclassifies via ClassifyLookupErr, and falls back to the LSP-only path with capped confidence ≤ 0.6. The semantic two-pass orchestrator is never reachable in production. Confidence + evidence per impacted node IS produced by the LSP fallback path (capped at 0.6) but the semantic 1.00/0.20 ladder is unreachable."
    artifacts:
      - path: internal/daemon/semantic_wiring.go
        issue: "SymbolID line 695: 'return integ.SymbolID(\"\"), integ.ErrNoSnapshot' on the success branch when Available; ExpandFrom line 728 same; ValidateCriticalEdges line 738 returns input edges with LSPConfirmed=false (no LSP traffic)."
      - path: internal/kernel/symbols/tools.go
        issue: "Line 656 lookup.SymbolID error always triggers fbReason path → never enters analyzeBlastRadiusViaLookup in production."
    missing:
      - "Real SymbolID lookup against the store's symbol index by (repoID, path, line, col) returning Phase 59 EXTRACT-02 stable IDs (deferred from 65-03 to 65-06; never landed)."
      - "Real ExpandFrom: BFS over store.QueryEffectiveAdjacency to depth=2 mapping rows → Impact with Phase 62 confidence ladder (deferred from 65-03 to 65-06; never landed)."
      - "Real ValidateCriticalEdges: kernel-side LSP probe over critical edges (deferred from 65-03 to 65-06; never landed)."
      - "End-to-end integration test that exercises the *production* analyzeBlastRadiusViaLookup path against a populated graph and asserts source==semantic with non-zero non-fallback confidence values (1.00 confirmed, 0.20 refuted)."
human_verification: []
overrides: []
---

# Phase 65: Existing-Tool Integration (Strangler Fig) Verification Report

**Phase Goal:** `get_repo_map`, `get_context`, `analyze_blast_radius`, and `get_health` consult the semantic graph when available — with zero source change to `internal/repomap` engine — and fall back to v1.9 behavior automatically when the index is disabled, building, or errored.

**Verified:** 2026-05-08T14:34:24Z
**Status:** gaps_found (2/4 must-haves verified — 2 BLOCKERs from production-stub method bodies)
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth (ROADMAP Success Criterion) | Status | Evidence |
|---|-----------------------------------|--------|----------|
| 1 | With the semantic index populated, `get_repo_map` and `get_context` return ranked output sourced from persisted graph scores + clusters; with `semantic_index.enabled=false` they return v1.9 tree-sitter + PageRank output (index-disabled goldens preserved). | **FAILED (BLOCKER)** | Index-disabled half VERIFIED (goldens at `internal/skill/repomap/testdata/goldens/index_disabled_*.txt` exist; `TestEnvelope_IndexDisabledIsTreeSitter` passes; `git log -- internal/repomap/` confirms zero source change to engine). Index-populated half FAILS: production `integSemanticLookup.RankFiles` and `RankFromSeeds` return `ErrNoSnapshot` when Available — `internal/daemon/semantic_wiring.go:705-710, 714-719`. The matrix test exercises only a fake `matrixLookup`. |
| 2 | `analyze_blast_radius` returns `confidence` and `evidence` per impacted node when the semantic graph is available; on fallback, confidence drops to ≤ 0.6 and the result envelope says so. | **FAILED (BLOCKER)** | Fallback half VERIFIED: `capConfidences(br, 0.6)` invoked on both `SourceTreeSitter` and `SourceFallback` arms (`internal/kernel/symbols/tools.go:645`); `TestAnalyzeBlastRadius_CfgDisabled_TreeSitter` and `TestAnalyzeBlastRadius_LookupUnavailable_Fallback` pin the cap. Semantic half FAILS: production `integSemanticLookup.SymbolID` returns `ErrNoSnapshot` at `semantic_wiring.go:695-700`, so the handler at `tools.go:656-657` always reclassifies to fallback before reaching `analyzeBlastRadiusViaLookup`. The 1.00 / 0.20 confidence ladder is never observable in production. |
| 3 | `get_health` includes a `semantic_index` section: store kind, latest snapshot status, graph version, overlay active flag, pending LSP count, last live-update latency, and last error. | ✓ VERIFIED | `SemanticIndexBlock` struct at `internal/kernel/health/tools.go:161-170` contains all 8 SPEC §24.5 fields (`Enabled`, `Store`, `LatestSnapshotStatus`, `GraphVersion`, `OverlayActive`, `PendingLSPRevalidations`, `LastLiveUpdateMs`, `LastError`). `ComputeSemanticIndexBlock` (`tools.go:202`) maps `integ.SemanticStatus` → wire JSON; daemon adapter implemented; envelope omitempty preserves SC-1 (`TestGetHealth_SC1Preserved` passes). Production `integSemanticLookup.Status` is REAL (not stubbed) — reads `LatestCommittedSnapshot`, `CurrentGraphVersion`, `OverlayHasPendingRows`, `queue.DepthAll`, `live.LastFlushAt`. `LastErrorReason` is hard-coded to `""` (acknowledged by 65-03 as Phase 65 65-07 follow-up; SPEC §24.5 contract upheld via Status err-return path). |
| 4 | Every MCP envelope from a semantic-aware tool returns `source: semantic | tree_sitter | fallback` so callers can detect path drift. | ✓ VERIFIED | `integ.MarshalEnvelope` invoked in all four tool paths: `internal/skill/repomap/skill.go:407, 511` (get_repo_map, get_context); `internal/kernel/symbols/blast_radius_strangler.go:278, 303` (analyze_blast_radius); `internal/kernel/health/tools.go:244 (Source string field)` (get_health). Closed-enum constants at `internal/semantic/integ/source.go`; `ChooseSource` priority ladder uniformly used. `TestEnvelope_ClosedEnum`, `TestChooseSource_PriorityLadder`, and `TestE2E_StranglerFig_SourceMatrix` pin the field across all 24 (row × tool) cells. NB: which value the source field carries in production is governed by Truths 1+2 above. |

**Score:** 2/4 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/lint/nokernel2semantic/analyzer.go` | Allowlist for `internal/semantic/integ` | ✓ VERIFIED | `allowedSemanticIntegPath` constant present; tests `TestAnalyzer_AllowsKernelImportingSemanticInteg`, `TestAnalyzer_RejectsKernelImportingSemantic`, `TestAnalyzer_RejectsKernelImportingSemanticIntegLookalike` all pass. |
| `internal/semantic/integ/{doc,lookup,source,status,noop,envelope,source_select}.go` | Types-only seam | ✓ VERIFIED | All files exist; `SemanticLookup` interface, closed-enum `Source`/`FallbackReason`, value types, `NoopLookup`, `Envelope`, `ChooseSource`, `ClassifyLookupErr` all present. `go test ./internal/semantic/integ/... -count=1` passes. |
| `internal/daemon/semantic_wiring.go` (production buildFn) | Real walk → classify → extract → ToStoreFacts → WriteSnapshotFacts pipeline | ✓ VERIFIED | D-09 carryover #1 closed at `feat(65-01)` commit `8ac60229`. `ClassifyPathChange` at line 1073, `ToStoreFacts` at line 1166, `WriteSnapshotFacts` at line 957. `TestProductionBuildFn_WritesNonEmptyFacts` and `TestE2E_IndexThenContext_SymbolCount` pass with real Facts. |
| `internal/daemon/semantic_wiring.go` (semSessionAdapter wsKeyFn) | Workspace key from daemon active closure | ✓ VERIFIED | D-09 carryover #2 closed at `feat(65-02)` commit `6348c3db`. `Workspace()` returns `a.wsKeyFn()` (line 542-547). `TestSemSessionAdapter_WorkspaceResolved` passes. |
| `internal/daemon/semantic_wiring.go` (integSemanticLookup adapter) | Production read-tier adapter wrapping store + retrieval + scheduler | ⚠️ STUB | Type exists (line 668); compile-time assertion `var _ integ.SemanticLookup = (*integSemanticLookup)(nil)` at line 1211. `Available()` and `Status()` are real. **`SymbolID`, `RankFiles`, `RankFromSeeds`, `ExpandFrom` return `ErrNoSnapshot` unconditionally when `Available()==true`** (lines 695-728). `ValidateCriticalEdges` returns input edges with `LSPConfirmed=false` (no actual LSP probe). The 65-03 SUMMARY's "Resolves in 65-05/65-06" deferral was never delivered: 65-05 commit `0f11d8a0` modified `semantic_wiring.go` only to add `daemonCfgGate`; 65-06 did not touch the file. |
| `internal/skill/repomap/skill.go` (SetSemanticLookup wiring) | Setter + lookup() normalizer + ChooseSource + JSON envelope | ✓ VERIFIED | `SetSemanticLookup` at line 172; `SetConfigGate`; `lookup()` nil-normalizer; `execGetRepoMap`/`execGetContext` route through `ChooseSource`; output JSON-wrapped via `MarshalEnvelope`. Skill code is correct and well-tested with a fake lookup. |
| `internal/skill/repomap/strangler.go` | adaptRankedFiles + computeFreshness | ✓ VERIFIED | Both helpers present; sort-before-iterate doctrine preserved; closed-enum freshness mapping correct. |
| `internal/skill/repomap/testdata/goldens/index_disabled_*.txt` | Byte-identical v1.9 tree text fixtures | ✓ VERIFIED | Both files exist (372 bytes each). `TestEnvelope_IndexDisabledIsTreeSitter` passes; `INTEG-01` fail-loud comment present. |
| `internal/kernel/symbols/skill_adapter.go` | SymbolsSkill ToolProvider | ✓ VERIFIED | `SymbolsSkill` registers via `init()`; `Tools()` returns `analyze_blast_radius` ToolDef. |
| `internal/kernel/symbols/blast_radius_strangler.go` | Two-pass orchestrator + filterCritical + applyValidationVerdicts + capConfidences | ✓ VERIFIED | All helpers present at lines 79-243. `analyzeBlastRadiusViaLookup` two-pass logic correct (Pass 1 ExpandFrom; copy-before-mutate; filterCritical; Pass 2 ValidateCriticalEdges; verdicts applied). 7/7 tests pass. **Note:** correct in isolation; never reached in production due to upstream stubs (see Truth 2). |
| `internal/kernel/symbols/tools.go` (registerAnalyzeBlastRadius widened) | Accepts lookupFn + cfgGate; ChooseSource ladder at handler entry | ✓ VERIFIED | Signature widened (line 593-600); `integ.ChooseSource` at line 636; three-arm switch at lines 637-653; cap applied uniformly on tree_sitter + fallback arms. |
| `internal/kernel/health/tools.go` (semantic_index block) | SemanticIndexBlock + ComputeSemanticIndexBlock + RegisterTools widened | ✓ VERIFIED | Block struct (8 fields per SPEC §24.5); `ComputeSemanticIndexBlock` real; envelope adds top-level `Source` field; `nilIfDisabled` preserves omitempty. SC-1 envelope unchanged (`TestSemanticStoreStatus_JSONShape` passes). |
| `internal/skill/semantic/integration_test.go` (matrix) | TestE2E_StranglerFig_SourceMatrix table-driven | ⚠️ FAKE-ONLY | 6 rows × 4 tools = 24 subtests pass, but the matrix uses a hand-rolled `matrixLookup` SemanticLookup fake (line 748+). The production `integSemanticLookup` adapter is never exercised end-to-end with a real DuckDB snapshot. The test pins the closed-enum contract correctly but does not exercise the goal SC #1 / #2 against the production code path. |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| RepoMapSkill.execGetRepoMap | integ.SemanticLookup.RankFiles | `lookup.RankFiles(ctx, ws)` | ⚠️ WIRED-TO-STUB | Skill side wired correctly; production lookup returns ErrNoSnapshot unconditionally. |
| RepoMapSkill.execGetContext | integ.SemanticLookup.RankFromSeeds | `lookup.RankFromSeeds(ctx, ws, files)` | ⚠️ WIRED-TO-STUB | Same. |
| analyze_blast_radius handler | integ.SemanticLookup.SymbolID | `lookup.SymbolID(ctx, ws, args.Path, lspLine, lspCol)` | ⚠️ WIRED-TO-STUB | Same — handler always falls back. |
| analyzeBlastRadiusViaLookup | integ.SemanticLookup.ExpandFrom + ValidateCriticalEdges | direct call | ⚠️ UNREACHABLE | Orchestrator is correct but unreachable because SymbolID upstream errors. |
| Daemon post-init | RepoMapSkill.SetSemanticLookup | `repomapSkill.SetSemanticLookup(sBndl.integLookupAccessor())` | ✓ WIRED | `internal/daemon/daemon.go:679`. |
| Daemon post-init | symbols.RegisterTools (lookupFn + cfgGate) | parameters threaded | ✓ WIRED | New signature accepts both; daemon constructs and threads. |
| Daemon post-init | health.RegisterTools (cfgGate, semLookup, semIndex) | parameters threaded | ✓ WIRED | All three threaded; daemonSemIndexAccessor wraps integSemanticLookup.Status. |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|---------------------|--------|
| `execGetRepoMap` (semantic arm) | `ranked []integ.RankedFile` | `lookup.RankFiles` (production) | **No — always errors with ErrNoSnapshot** | ✗ HOLLOW (semantic arm unreachable) |
| `execGetRepoMap` (fallback arm) | `treeText` from `renderV19` | `s.graph.RankFiles(0.85, ...)` over FileGraph | Yes (existing v1.9 path) | ✓ FLOWING |
| `execGetContext` (semantic arm) | `ranked []integ.RankedFile` | `lookup.RankFromSeeds` (production) | **No — same** | ✗ HOLLOW |
| `analyze_blast_radius` (semantic arm) | `impacts []integ.Impact` | `lookup.SymbolID` then `lookup.ExpandFrom` | **No — SymbolID errors first** | ✗ HOLLOW |
| `analyze_blast_radius` (fallback arm) | `br.PerNode` | `AnalyzeBlastRadius` (LSP) → `capConfidences(br, 0.6)` | Yes (LSP-derived) | ✓ FLOWING |
| `get_health` (semantic_index block) | `SemanticIndexBlock` fields | `accessor.Status` → `integSemanticLookup.Status` (REAL) → store accessors | Yes | ✓ FLOWING |

**Note on Level-4 finding:** Per the goal-backward methodology, an artifact that is wired (Level 3) but does not produce real data (Level 4) is HOLLOW. The semantic arms of three of the four tools are wired-but-hollow in production.

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Phase 65 packages compile | `go build ./internal/... ./cmd/...` | exit 0 (Swift binding macro warning only) | ✓ PASS |
| Phase 65 packages pass vet | `go vet ./internal/lint/nokernel2semantic/... ./internal/semantic/integ/... ./internal/kernel/symbols/... ./internal/kernel/health/... ./internal/skill/repomap/... ./internal/daemon/...` | exit 0 | ✓ PASS |
| Lint analyzer tests | `go test ./internal/lint/nokernel2semantic/... -count=1` | PASS | ✓ PASS |
| integ package tests | `go test ./internal/semantic/integ/... -count=1` | PASS | ✓ PASS |
| Symbols + health + repomap tests | `go test ./internal/kernel/symbols/... ./internal/kernel/health/... ./internal/skill/repomap/... -count=1` | PASS | ✓ PASS |
| Daemon phase 65 tests | `go test ./internal/daemon/... -count=1 -run "TestProductionBuildFn|TestSemSessionAdapter|TestIntegSemanticLookup"` | PASS | ✓ PASS |
| Strangler-fig matrix (24 subtests) | `go test ./internal/skill/semantic/... -run "TestE2E_StranglerFig_SourceMatrix"` | PASS — but uses fake `matrixLookup`, not production adapter | ⚠️ PASS (FAKE-ONLY) |
| 15-symbol fixture (post-65-01) | `go test ./internal/skill/semantic/... -run "TestE2E_IndexThenContext_SymbolCount"` | PASS | ✓ PASS |
| End-to-end production-adapter `source==semantic` | (no such test exists) | — | ✗ MISSING |
| End-to-end production-adapter `confidence==1.00` (validated edge) | (no such test exists) | — | ✗ MISSING |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| INTEG-01 | 65-00, 65-01, 65-03, 65-05, 65-08 | get_repo_map consults SetSemanticLookup; falls back on disabled/building/errored; zero source change to internal/repomap engine. | **PARTIAL — BLOCKED** | Falls-back half VERIFIED (config-disabled goldens, engine untouched). Semantic-on half BLOCKED — RankFiles is a stub. |
| INTEG-02 | 65-01, 65-02, 65-03, 65-05, 65-08 | get_context delegates to semantic retrieval engine when available with same fallback contract. | **PARTIAL — BLOCKED** | Same — RankFromSeeds is a stub. |
| INTEG-03 | 65-00, 65-03, 65-06, 65-08 | analyze_blast_radius uses semantic graph expansion + LSP validation when available; per-node confidence + evidence; falls back when disabled. | **PARTIAL — BLOCKED** | Fallback path with 0.6 cap VERIFIED. Semantic two-pass orchestrator correct in isolation but unreachable: SymbolID/ExpandFrom/ValidateCriticalEdges are stubs. |
| INTEG-04 | 65-00, 65-07, 65-08 | get_health includes semantic_index section (store kind, snapshot status, graph version, overlay active, pending LSP count, last live-update latency, last error). | ✓ SATISFIED | All 8 SPEC §24.5 fields present and populated by real `integSemanticLookup.Status` (Status is the one real method). LastErrorReason hard-coded to "" but covered by err-return path. |
| INTEG-05 | 65-03, 65-04, 65-05, 65-06, 65-07, 65-08 | Every MCP envelope from a semantic-aware tool returns source field (semantic | tree_sitter | fallback). | ✓ SATISFIED | Closed-enum source field stamped on all four envelopes via `MarshalEnvelope`/struct tag. ChooseSource ladder uniform across consumers. Note: in production the `semantic` value is unreachable (see INTEG-01/02/03). |

**Orphaned requirements:** None — all 5 IDs (INTEG-01..INTEG-05) appear in plan frontmatter.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `internal/daemon/semantic_wiring.go` | 695-728 | `RankFiles`, `RankFromSeeds`, `SymbolID`, `ExpandFrom` all return `ErrNoSnapshot` on the `Available==true` branch. Method parameters all underscore-discarded. Comments say "65-05 wires the persisted-score reader" / "65-06 wires BFS over QueryEffectiveAdjacency" — those wires never landed. | 🛑 BLOCKER | Goal SC #1 and #2 unreachable in production. |
| `internal/daemon/semantic_wiring.go` | 734-742 | `ValidateCriticalEdges` returns input edges with `LSPConfirmed=false` (no LSP traffic). | 🛑 BLOCKER (contributes to INTEG-03 gap) | Pass-2 verdicts can never confirm/refute in production. |
| `internal/kernel/health/tools.go` | 75-83 | `classifySemanticProbeError` substring-matches `"DB handle nil"` (not emitted by daemon) and `"store unavailable"` (would catch any future wrapped DuckDB error). REVIEW.md CR-02 BLOCKER. | ⚠️ WARNING | Misclassifies real DB errors as `nil_handle`; violates WR-NEW-01 doctrine. Does not block goal but corrupts operator diagnostics. |
| `internal/daemon/semantic_wiring.go` | 794 | `LastErrorReason` hard-coded to `""` even on `state==StatusReady` with transient error. | ℹ️ INFO | Acknowledged in 65-03 SUMMARY as 65-07 follow-up that did not land; SPEC §24.5 contract upheld via err-return path so wire surface is honest. |
| `internal/kernel/symbols/blast_radius_strangler.go` | 284-307 | `formatBlastRadiusEnvelopeFromImpacts` does not stamp `graph_version` (REVIEW WR-03). Other three tools do stamp it. | ⚠️ WARNING | Asymmetric envelope between four tools; semantic envelope has empty graph_version. Would not surface in production today because semantic arm is unreachable. |
| `internal/kernel/symbols/blast_radius_strangler.go` | 209-226 | `applyValidationVerdicts` `break`s after first matching edge per impact, so refutations after a confirmation are silently lost (REVIEW WR-05). | ⚠️ WARNING | Order-dependent verdict; non-deterministic if evidence-edges order is non-stable. Not exercised in production today. |

### Code Review Cross-Reference (REVIEW.md)

REVIEW.md catalogues 14 findings (2 critical, 7 warning, 5 info). Of those, two interact with goal verification:

- **CR-01:** Re-classified to INFO (IN-04) by reviewer; not a goal-violation. CONFIRMED — Status err-return path preserves wire contract.
- **CR-02:** `classifySemanticProbeError` substring sentinel mismatch. BLOCKER from review's perspective on diagnostic correctness, but **not a goal-violation** for Phase 65 — get_health still returns a `semantic_store` block per SC-1; the misclassification corrupts operator diagnostics rather than breaking the success criterion. Surfaced here as ⚠️ WARNING per the verifier prompt's guidance.

### Carryover from Phase 64

| Carryover | Status | Evidence |
|-----------|--------|----------|
| D-09 #1 — production buildFn empty-Facts placeholder | ✓ CLOSED | 65-01 commit `8ac60229` replaced placeholder with real walk → classify → ToStoreFacts → WriteSnapshotFacts pipeline. `TestProductionBuildFn_WritesNonEmptyFacts` passes; `TestE2E_IndexThenContext_SymbolCount` reports 15 symbols. |
| D-09 #2 — zero-value WorkspaceKey from session adapter | ✓ CLOSED | 65-02 commit `6348c3db` threaded `wsKeyFn` closure through `newSemanticBundle` into `semSessionAdapter`. `Workspace()` now returns `a.wsKeyFn()`. `TestSemSessionAdapter_WorkspaceResolved` passes. |

### Human Verification Required

None — the gaps are observable programmatically (stub method bodies returning unconditional sentinel errors).

### Gaps Summary

The phase ships a clean architectural seam (`internal/semantic/integ`), correct consumer-side wiring in all four tools, a uniform closed-enum source field, a populated `semantic_index` block in `get_health`, and the two Phase 64 carryover items. The lint analyzer, envelope marshaller, source-selection helper, two-pass orchestrator, confidence cap, and JSON wrapper are all correct and well-tested.

The blocking gap is on the daemon side: the production `integSemanticLookup` adapter ships with five of its seven methods as stubs that unconditionally return `ErrNoSnapshot` (or pass through edges with `LSPConfirmed=false`). The 65-03 plan and SUMMARY explicitly defer the real ranking/expansion/symbol-translation logic to 65-05 (RankFiles/RankFromSeeds) and 65-06 (ExpandFrom/SymbolID/ValidateCriticalEdges); but the 65-05 commit modified `semantic_wiring.go` only to add `daemonCfgGate`, and 65-06 did not touch the file at all. The deferral was never honoured.

Consequence: with `cfg.SemanticIndex.Enabled=true` and a fully populated, committed snapshot, the consumer-side `ChooseSource` ladder always reclassifies the lookup error to `SourceFallback` + `FallbackReasonNoSnapshotYet`. The semantic arms of `get_repo_map`, `get_context`, and `analyze_blast_radius` are unreachable. The closed-enum `semantic` source value cannot appear on the wire — only `tree_sitter` (cfg off) or `fallback + no_snapshot_yet` (cfg on, populated index, but stubbed lookup). This violates ROADMAP success criteria #1 and #2 directly.

The matrix integration test passes only because it uses a hand-rolled `matrixLookup` fake; it does not drive the production `integSemanticLookup` adapter against a real DuckDB snapshot. There is no end-to-end regression harness for the semantic-source path.

The two gap entries in the YAML frontmatter group these defects by goal-truth (one truth covers RankFiles/RankFromSeeds for SC#1; one covers SymbolID/ExpandFrom/ValidateCriticalEdges for SC#2). The structure is suitable for `/gsd-plan-phase --gaps` consumption.

---

_Verified: 2026-05-08T14:34:24Z_
_Verifier: Claude (gsd-verifier)_

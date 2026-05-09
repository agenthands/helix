---
phase: 65
plan: 06
subsystem: kernel/symbols
tags:
  - blast_radius
  - lsp
  - two-pass
  - kernel
  - strangler-fig
requirements_completed:
  - INTEG-03
  - INTEG-05
dependency_graph:
  requires:
    - 65-00  # vet allowlist for internal/semantic/integ from kernel/*
    - 65-03  # SemanticLookup interface (lookup.go) and Status types
    - 65-04  # ChooseSource + Envelope + ClassifyLookupErr
  provides:
    - analyzeBlastRadiusViaLookup (two-pass orchestrator)
    - filterCritical / crossesPublicAPI / applyValidationVerdicts helpers
    - capConfidences (D-08 fallback hard cap)
    - formatBlastRadiusEnvelope / formatBlastRadiusEnvelopeFromImpacts
    - SymbolsSkill ToolProvider adapter
  affects:
    - internal/kernel/symbols (BlastRadius gains PerNode field)
    - internal/daemon (symbols.RegisterTools widens to lookupFn + cfgGate)
tech_stack:
  added:
    - none (reuses Phase 62/65 confidence ladder + integ envelope)
  patterns:
    - "ChooseSource priority ladder at handler entry (cfg first, lookup second, err third)"
    - "Pitfall §6 copy-before-mutate before applying Pass-2 verdicts"
    - "Uniform confidence cap on every non-semantic path (tree_sitter AND fallback)"
    - "Critical-edge filter: low-conf | cross-pkg+exported target (OR, not AND)"
    - "Catalog-only ToolProvider mirroring kernel/health/skill_adapter.go"
key_files:
  created:
    - internal/kernel/symbols/blast_radius_strangler.go
    - internal/kernel/symbols/blast_radius_strangler_test.go
    - internal/kernel/symbols/skill_adapter.go
  modified:
    - internal/kernel/symbols/blast.go (BlastRadius gains PerNode []NodeImpact)
    - internal/kernel/symbols/tools.go (registerAnalyzeBlastRadius widens; ChooseSource at entry)
    - internal/daemon/daemon.go (symbols.RegisterTools wiring; skill skip list)
decisions:
  - "Per plan must_haves: BlastRadius gains a PerNode []NodeImpact field rather than wrapping the existing struct in a new envelope-only type. Synthesize from DirectRefs + Implementations + Callers with confidence 1.0 (LSP-derived) so capConfidences(0.6) drops them uniformly to the D-08 cap on every non-semantic path."
  - "lookupFn (closure) over fixed lookup parameter: mirrors RepoMapSkill's SetSemanticLookup post-init wiring style and lets the daemon swap adapters without re-registering tools. Closure is constructed at line ~544 of daemon.go from the existing sBndl.integLookupAccessor()."
  - "applyValidationVerdicts breaks after the first matching edge per impact (avoids later edges in the same impact overwriting the verdict). Pitfall §6 still satisfied because the orchestrator passes a COPY of the Pass-1 slice."
  - "isExported uses ASCII-uppercase heuristic on the qualified-name suffix. Conservative — Go-language convention; follow-up enhancement to consult richer EXTRACT-02 visibility metadata."
metrics:
  duration_minutes: ~35
  tasks_completed: 2  # RED + GREEN (REFACTOR not strictly required per plan)
  files_changed: 5
completed_date: 2026-05-08
---

# Phase 65 Plan 06: existing-tool-integration-strangler-fig (analyze_blast_radius) Summary

## One-liner

Wraps the v1.9 pure-LSP `analyze_blast_radius` with a two-pass orchestrator that consults `integ.SemanticLookup` first (Pass 1 = `ExpandFrom` over the persisted graph; Pass 2 = `ValidateCriticalEdges` for LSP confirm/refute), routed through `integ.ChooseSource` so the cfg-disabled path emits `source=tree_sitter` (NOT `source=fallback`) and every non-semantic path hard-caps confidence at 0.6 (D-08 / ROADMAP SC #2).

## What landed

### Two-pass orchestrator (`internal/kernel/symbols/blast_radius_strangler.go`)

- **`analyzeBlastRadiusViaLookup(ctx, lookup, ws, sym)`** — D-07 two-pass dispatcher. Pass 1 calls `lookup.ExpandFrom(..., depth=2)`. The result is *copied* before any Pass-2 mutation (Pitfall §6). Pass 2 calls `lookup.ValidateCriticalEdges` on edges flagged by `filterCritical`. A Pass-1 error returns `(nil, SourceFallback, ClassifyLookupErr(err), err)` so the caller drops to the LSP fallback path. A Pass-2 error is non-fatal — the orchestrator keeps Pass 1 verdicts and reports `source=semantic`.
- **`filterCritical(impacts)`** — returns the subset where `confidence < 0.80 OR crossesPublicAPI(impact)`. Boolean OR, not AND: a low-confidence in-package edge is still critical because the LSP can confirm/refute cheaply.
- **`crossesPublicAPI(im)`** — true iff any evidence edge has different from/to package paths AND the target symbol is exported (capitalized first ASCII rune of the qualified-name suffix).
- **`applyValidationVerdicts(impacts, verdicts)`** — confirmed → confidence 1.00; refuted → confidence 0.20 + Refuted=true. Mutates in place; caller is responsible for the copy-before-mutate contract.
- **`capConfidences(br, cap)`** — clamps `br.PerNode[i].Confidence` to ≤ cap. Used on every non-semantic path (D-08 + ROADMAP SC #2; M-confcap mitigation).
- **`formatBlastRadiusEnvelope` / `formatBlastRadiusEnvelopeFromImpacts`** — render the closed-enum (source, fallback_reason) header via `integ.MarshalEnvelope` plus a `per_node` payload carrying SymbolID, confidence, refuted flag, and evidence (edges + lsp_locations).

### Source-selection at handler entry (`internal/kernel/symbols/tools.go`)

- **`registerAnalyzeBlastRadius` widened** to accept `lookupFn func() integ.SemanticLookup` and `cfgGate integ.ConfigGate`.
- Handler routes through `integ.ChooseSource(cfgGate, lookup, nil)` BEFORE any semantic call. The four switch arms:
  - `SourceTreeSitter` (cfg disabled) → LSP path with `capConfidences(0.6)`. Envelope: `source="tree_sitter"`, no `fallback_reason`.
  - `SourceFallback` (cfg enabled, !Available) → LSP path with `capConfidences(0.6)`. Envelope: `source="fallback"`, `fallback_reason="index_disabled"` (defensive D-05).
  - `SourceSemantic` → resolve cursor → SymbolID via `lookup.SymbolID`; on err re-classify and drop to LSP. On success run two-pass orchestrator. On Pass-1 err drop to LSP with classified reason. On success render via `formatBlastRadiusEnvelopeFromImpacts`.
- `nil` lookup is normalized to `integ.NoopLookup{}` so the priority-ladder gate always sees a valid interface value.

### BlastRadius extended (`internal/kernel/symbols/blast.go`)

- New `NodeImpact{SymbolID string, Confidence float64, Evidence integ.Evidence, Refuted bool}` mirrors `integ.Impact` shape.
- `BlastRadius.PerNode []NodeImpact` ships on every result. `AnalyzeBlastRadius` synthesizes entries from `DirectRefs + Implementations + Callers` with confidence 1.0 so `capConfidences(0.6)` drops them uniformly on every non-semantic path.

### Skill adapter (`internal/kernel/symbols/skill_adapter.go`)

- `SymbolsSkill` ToolProvider mirroring `kernel/health/skill_adapter.go` and `kernel/help/skill_adapter.go`. Catalog-only registration (`RegisterFn` nil) — daemon owns MCP-SDK registration via `RegisterTools` (D-01).

### Daemon wiring (`internal/daemon/daemon.go`)

- `symbolsLookupFn` closure constructed from `sBndl.integLookupAccessor()` (nil-tolerant — `NoopLookup{}` when semantic disabled).
- `symbolsCfgGate := &daemonCfgGate{enabled: cfg.SemanticIndex.Enabled}` (constructed once, fed to both repomap and symbols wiring).
- `symbols.RegisterTools(mcpServer, k, wsKeyFn, symbolsLookupFn, symbolsCfgGate)`.
- Skill skip list at line 558 extended with `"symbols"` so the new ToolProvider does not double-catalog `analyze_blast_radius`.
- New import: `internal/semantic/integ` (covered by the wave-0 vet allowlist).

## Tests

Seven new tests in `internal/kernel/symbols/blast_radius_strangler_test.go`, all passing under `-race`:

| Test | Pins |
|------|------|
| `TestAnalyzeBlastRadius_SemanticTwoPass` | D-07: validated→1.00, refuted→0.20+Refuted=true, non-critical retained byte-identical |
| `TestAnalyzeBlastRadius_CfgDisabled_TreeSitter` | D-04 + Pitfall §3: cfg-off ⇒ source=tree_sitter (NOT fallback+index_disabled) |
| `TestAnalyzeBlastRadius_LookupUnavailable_Fallback` | Defensive D-05: cfg-on + !Available ⇒ source=fallback + index_disabled |
| `TestAnalyzeBlastRadius_Pass1Error_FallsBackToLSP` | Pass-1 err re-classifies via ClassifyLookupErr; fallback cap applied |
| `TestAnalyzeBlastRadius_Pass2Error_KeepsPass1` | Pass-2 err keeps Pass 1; source stays semantic |
| `TestFilterCritical_PublicAPIBoundary` | low-conf | (cross-pkg AND exported target) |
| `TestApplyValidationVerdicts_NoMutation` | Pitfall §6 copy-before-mutate |

Tests use a hand-rolled `fakeLookup` SemanticLookup test double + `fakeCfg` ConfigGate test double, exercising the orchestrator helpers directly without spinning up an MCP server.

## Verification

| Check | Result |
|-------|--------|
| `go test ./internal/kernel/symbols/... -count=1 -race` | PASS (all 7 new tests + existing) |
| `go test ./internal/daemon/... -count=1 -race` | PASS |
| `go vet ./...` | PASS |
| `make vet` (incl. vet-nokernel2semantic, vet-nosemantic2kernel, vet-noduckdb, vet-compact-uses-store) | PASS |
| `go build ./...` | PASS |
| `grep -c "integ.ChooseSource" internal/kernel/symbols/tools.go` | 4 (≥1 ✓) |
| `grep -c "lookup.ExpandFrom" internal/kernel/symbols/blast_radius_strangler.go` | 3 (≥1 ✓) |
| `grep -c "lookup.ValidateCriticalEdges" internal/kernel/symbols/blast_radius_strangler.go` | 3 (≥1 ✓) |
| `grep -c "0.6" internal/kernel/symbols/blast_radius_strangler.go internal/kernel/symbols/tools.go` | 3 (≥1 ✓; via `fallbackConfidenceCap` constant) |
| M-readtier write-token canary | OK (no `BeginSnapshot/CommitSnapshot/AbortSnapshot/WriteSnapshotFacts/OnFlush/BumpGraphVersion` in strangler files) |

Wider regression tests (`./internal/kernel/...`, `./internal/daemon/...`, `./internal/skill/...`, `./internal/semantic/integ/...`) all pass.

## Acceptance criteria — all satisfied

- **D-04 + Pitfall §3** — `TestAnalyzeBlastRadius_CfgDisabled_TreeSitter` pins `source="tree_sitter"`, `fallback_reason==""`, all `PerNode.Confidence ≤ 0.6`.
- **D-05 defensive** — `TestAnalyzeBlastRadius_LookupUnavailable_Fallback` pins `source="fallback"`, `fallback_reason="index_disabled"`, cap applied.
- **D-07 two-pass** — `TestAnalyzeBlastRadius_SemanticTwoPass` pins confirmed→1.00, refuted→0.20+Refuted=true, non-critical retained byte-identical.
- **D-08 + ROADMAP SC #2** — `capConfidences(br, fallbackConfidenceCap)` invoked on BOTH SourceTreeSitter and SourceFallback branches in `tools.go`, plus on Pass-1 error / SymbolID error paths.
- **INTEG-03** — per-node confidence + evidence in semantic-on envelope (`per_node` payload key with `confidence`, `evidence`, optional `refuted`); fallback caps at ≤ 0.6.
- **INTEG-05** — `source` field stamped on every envelope via `integ.MarshalEnvelope`.
- **Pitfall §6** — `TestApplyValidationVerdicts_NoMutation` enforces copy-before-mutate.

## Deviations from Plan

None — plan executed as written. The optional REFACTOR step was skipped per plan guidance ("not strictly required"); `formatBlastRadiusEnvelope` was already extracted in the GREEN commit.

## Threat Model — disposition

| Threat ID | Mitigation | Status |
|-----------|------------|--------|
| T-65-06-01 (Pass 1 in-place mutation) | TestApplyValidationVerdicts_NoMutation + impactsCopy in analyzeBlastRadiusViaLookup | mitigated |
| T-65-06-02 (confidence > 0.6 leaks on non-semantic path) | capConfidences on both SourceTreeSitter + SourceFallback + Pass-1 error paths; tests pin | mitigated |
| T-65-06-03 (refuted edge silently flips) | applyValidationVerdicts sets Refuted=true; envelope renders refuted flag | mitigated |
| T-65-06-04 (kernel imports semantic concretions) | Imports only internal/semantic/integ; vet-nokernel2semantic passes | mitigated |
| T-65-06-05 (cold start triggers indexing) | Available() is cheap; orchestrator never calls indexer | mitigated |
| T-65-06-06 (orchestrator writes a snapshot) | M-readtier grep canary OK; no write tokens in strangler files | mitigated |
| T-65-06-07 (raw error.Error() leaks to envelope) | All envelope reasons go through ClassifyLookupErr; raw err only flows through existing errorResult on hard LSP failure | mitigated |
| T-65-06-08 (cfg.Enabled=false misclassified as fallback+index_disabled) | integ.ChooseSource at handler entry; TestAnalyzeBlastRadius_CfgDisabled_TreeSitter pins | mitigated |

## Threat Flags

None — no new security-relevant surface beyond what the plan's threat model anticipated.

## TDD Gate Compliance

- **RED:** `fc189dd1 test(65-06): add failing tests for analyze_blast_radius two-pass orchestrator + ChooseSource integration` — compile-failure RED state confirmed.
- **GREEN:** `60f42709 feat(65-06): graph-first + LSP-validates-critical-edges for analyze_blast_radius (INTEG-03)` — all 7 tests pass.
- **REFACTOR:** Skipped per plan (not strictly required).

## Self-Check: PASSED

- Files exist: blast_radius_strangler.go ✓, blast_radius_strangler_test.go ✓, skill_adapter.go ✓, blast.go modified ✓, tools.go modified ✓, daemon.go modified ✓.
- Commits exist: fc189dd1 (RED) ✓, 60f42709 (GREEN) ✓.
- All seven tests pass under `-race`.
- All four vet tools pass.
- Wider regression suite (kernel/symbols, daemon, skill/repomap, skill/semantic, semantic/integ) green.

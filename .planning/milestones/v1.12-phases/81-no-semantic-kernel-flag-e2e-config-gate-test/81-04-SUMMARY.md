---
phase: 81-no-semantic-kernel-flag-e2e-config-gate-test
plan: 04
subsystem: daemon/composition-root
tags: [ablation, semantic, no_semantic, ABLATE-06, D-02, D-04, build-but-block]
requires:
  - "semantic.Config.BenchDisabled + Profile.DisableSemanticSubsystem (Plan 81-02)"
  - "internal/semantic/integ.ChooseSource priority ladder + NoopLookup (Phase 65)"
  - "vet-ablation-leakage gate allowlist incl. internal/daemon (Plan 81-03)"
provides:
  - "effSemanticDisabled resolved ONCE at the daemon composition root (resolveSemanticDisabled, D-02)"
  - "gatedSymbolsLookupFn / gatedCfgGate helpers forcing NoopLookup{} + disabled ConfigGate (Pitfall 4)"
  - "all 4 integLookupAccessor hand-out sites gated (symbols/health, repomap, guardrail, SemanticSkill accessor block)"
  - "SemanticSkill Set*Accessor block gated + explicitly nulled under the gate (Pitfall 3 + Pitfall 5)"
  - "TestEffSemanticDisabled / TestSemanticGateForcesNoop / TestSemanticGateChoosesTreeSitter / TestSemanticSkillAccessorsGated"
affects:
  - "internal/daemon (composition root now governs every semantic back-channel read consumer from a single flag)"
  - "Plan 81-05 (depends on the gate being LIVE to assert zero semantic-store reads on the no_semantic arm)"
tech-stack:
  added: []
  patterns:
    - "Single-resolution-point gate (D-02, mirror Phase 76 effDisableLSP) — no scattered per-callsite flag checks"
    - "Build-but-block (D-04): bundle/store STILL built under the gate; only READ consumers forced to Noop"
    - "ChooseSource-first (Pitfall 4): disable the GATE, not just the lookup -> source=tree_sitter not fallback"
    - "Idempotent null-object reset (Pitfall 5): explicitly null repomap lookup + all SemanticSkill accessors on the process-global singletons"
key-files:
  created:
    - internal/daemon/semantic_gate.go
    - internal/daemon/semantic_gate_test.go
  modified:
    - internal/daemon/daemon.go
    - internal/daemon/semantic_wiring.go
    - internal/daemon/daemon_test_export_test.go
decisions:
  - "effSemanticDisabled := resolveSemanticDisabled(cfg, activeProfile) = cfg.SemanticIndex.BenchDisabled || activeProfile.DisableSemanticSubsystem, resolved once at daemon.go:294 adjacent to effDisableLSP (D-02/D-03 default-off)"
  - "Disable the GATE (gatedCfgGate sets enabled = cfg.SemanticIndex.Enabled && !effSemanticDisabled) so ChooseSource priority-1 yields SourceTreeSitter, NOT SourceFallback+index_disabled (Pitfall 4)"
  - "Under the gate, repomap SetSemanticLookup(nil) + the entire SemanticSkill accessor block cleared to nil (16 setters) rather than skipped — process-global singletons retain stale wiring across re-init otherwise (Pitfall 5)"
  - "Discovered + gated a FOURTH integLookupAccessor hand-out the plan's A5 did not enumerate: the guardrail middleware SessionContext.Lookup (daemon.go:855). Gated under && !effSemanticDisabled (deviation Rule 2, T-81-04-01 mitigation)"
  - "Bundle-build guards (daemon.go:324 store-Open, daemon.go:518 newSemanticBundle) left UNTOUCHED — store stays built (D-04 build-but-block, what makes the Plan 05 zero-read assertion non-vacuous)"
metrics:
  duration: ~30m
  completed: 2026-06-20
---

# Phase 81 Plan 04: no_semantic Kernel Gate (effSemanticDisabled) Summary

Resolved `effSemanticDisabled` ONCE at the daemon composition root (OR'ing the Plan 02 config field + profile field, D-02) and threaded it into every back-channel semantic read consumer so the `no_semantic` arm sees `integ.NoopLookup{}` + a DISABLED `ConfigGate` (→ `source == tree_sitter`, Pitfall 4) — while the bundle/store stays BUILT (D-04 build-but-block). This un-wires Phase 65's `SetSemanticLookup` strangler-fig for all 8 enumerated consumers (`get_repo_map`, `get_context`, `find_related_symbols`, `explain_symbol_deep`, `validate_graph_edge`, `analyze_blast_radius`, `RankFiles`, `ExpandFrom`) plus a fourth hand-out (the guardrail middleware lookup) the plan's A5 had not enumerated.

## What Was Built

### Task 1 (RED→GREEN): effSemanticDisabled resolution + Noop/disabled-gate on ChooseSource consumers
- **RED (commit 124b8b36):** `internal/daemon/semantic_gate_test.go` with `TestEffSemanticDisabled` (resolution truth table), `TestSemanticGateForcesNoop` (Noop forcing + disabled cfgGate under a NON-NIL bundle, D-04), and `TestSemanticGateChoosesTreeSitter` (gated ladder yields `tree_sitter`, not `fallback`). Added the `semanticBundleForTest()` export seam. RED was a compile failure (helpers undefined) — the correct pre-implementation state.
- **GREEN (commit 2b3092b9):** new `internal/daemon/semantic_gate.go` with three pure helpers — `resolveSemanticDisabled`, `gatedSymbolsLookupFn`, `gatedCfgGate`. Wired into `daemon.go`:
  - **Resolution (daemon.go:294):** `effSemanticDisabled := resolveSemanticDisabled(cfg, activeProfile)`, added to the "subsystem ablation flags resolved" log block.
  - **Gate point 1 — symbols (daemon.go:624-625):** `symbolsLookupFn = gatedSymbolsLookupFn(sBndl, effSemanticDisabled)` (returns `NoopLookup{}` first under the gate even with a non-nil bundle); `symbolsCfgGate = gatedCfgGate(...)` (disables the GATE, Pitfall 4).
  - **Gate point 2 — health (daemon.go:639):** `healthLookup` inherits the gated closure; `healthCfgGate = gatedCfgGate(...)`.
  - **Gate point 3 — repomap (daemon.go:795-799):** `SetConfigGate(gatedCfgGate(...))`; under the gate `rs.SetSemanticLookup(nil)` (explicit idempotent null-object, Pitfall 5), else the real adapter.

### Task 2 (RED→GREEN): gate the SemanticSkill Set*Accessor block (non-ChooseSource consumers)
- **RED:** extended the test file with `TestSemanticSkillAccessorsGated` (committed in 124b8b36 alongside Task 1's RED). It boots `daemon.New` with `BenchDisabled: true` + `Enabled: true` and asserts the Phase 74 `WiredAccessorsForTest` snapshot shows ALL accessors nil under the gate. RED confirmed (accessors still wired).
- **GREEN (commit 160d3d76):** threaded `effSemanticDisabled` as a new param into `newSemanticBundle` (semantic_wiring.go:218); gated the whole `Set*Accessor` block (`if b.skill != nil && !effSemanticDisabled`, line 262). Added an `else if b.skill != nil` branch (line 288) that EXPLICITLY clears all 16 accessors to nil — required because `SemanticSkill` is a process-global singleton (a prior non-gated `daemon.New` in the same process leaves a stale real accessor; skipping the setters is insufficient). This was caught by the full-suite run (order-dependent failure) and is the Pitfall-5 reset applied to the skill singleton.

### style (commit ae616817)
gofmt realigned the daemon.go import block and the semantic_wiring.go interface-guard var block — column shifts caused by the new `integ` import and the `effSemanticDisabled` identifier length.

## A5 Verification (no ungated 4th read-construction site)

The four `integLookupAccessor()` hand-out sites are ALL gated:
1. `internal/daemon/semantic_gate.go:49` — `gatedSymbolsLookupFn` (symbols + health), gated via the `effSemanticDisabled` param.
2. `internal/daemon/daemon.go:799` — repomap, behind the `else if` after `if effSemanticDisabled { SetSemanticLookup(nil) }`.
3. `internal/daemon/daemon.go:855` — guardrail middleware (`SessionContext.Lookup`), behind `if sBndl != nil && !effSemanticDisabled`. **This is the fourth hand-out the plan's A5 did not enumerate** — see Deviations.
4. `internal/daemon/semantic_wiring.go:276` — `SetImpactLookup(b.integLookupAccessor())`, inside the gated `if b.skill != nil && !effSemanticDisabled` block.

The ONLY production `integSemanticLookup{}` constructor in the read path is `semantic_wiring.go:1401` (inside `integLookupAccessor()`). The other two constructors (`integ_lookup_export.go:69`, `integ_lookup_e2e_helpers.go:170`) are documented `*ForTest` fixture builders invoked only from test packages, not production wiring. No ungated 4th construction site exists.

## D-04 (build-but-block) Confirmation

The bundle-build guards are UNTOUCHED — neither references `effSemanticDisabled`:
- `daemon.go:324` (`if cfg.SemanticIndex.Enabled {` — `semanticstore.Open`)
- `daemon.go:518` (`sBndl = newSemanticBundle(...)`, inside `if semanticStore != nil {`)

`TestSemanticSkillAccessorsGated` asserts `d.semanticBundleForTest() != nil` under the gate, proving the store/bundle is still built (otherwise the Noop forcing would be vacuous and Plan 05's zero-read assertion meaningless).

## Verification

- `go test ./internal/daemon/ -run 'TestEffSemanticDisabled|TestSemanticGateForcesNoop|TestSemanticGateChoosesTreeSitter|TestSemanticSkillAccessorsGated' -count=1` — green
- `go test ./internal/daemon/... ./internal/semantic/integ/... -count=1` — green (incl. the order-dependent singleton-staleness case)
- `go test ./...` — ALL PASS (0 FAIL)
- `go vet ./...` — clean
- `make vet` — exits 0 (incl. the Plan 03 `vet-ablation-leakage` call-site gate; production read wiring stays inside the `internal/daemon` allowlist)
- AC greps: `effSemanticDisabled` count in daemon.go = 14 (≥4); `SetSemanticLookup(nil)` present (daemon.go:797); `effSemanticDisabled` in semantic_wiring.go = 3 (≥1); bundle-build guard regions reference effSemanticDisabled = 0 (D-04); `test(81-04)` precedes both `feat(81-04)` commits
- gofmt — clean on all touched files

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing critical functionality] Gated a FOURTH integLookupAccessor hand-out (guardrail middleware)**
- **Found during:** Task 2 A5 verification (enumerating `integLookupAccessor()` call sites).
- **Issue:** The plan's A5 enumerated exactly three hand-out sites (daemon.go:614, daemon.go:780, semantic_wiring.go:266). A fourth exists at daemon.go:830-832: the guardrail middleware passes `sBndl.integLookupAccessor()` into `newGuardrailDeps`, where it becomes `rules.SessionContext.Lookup` consumed by rule predicates G-001..G-005 (a genuine back-channel semantic read). Leaving it ungated would leak a real semantic read onto the no_semantic arm — the central integrity threat the phase exists to close (T-81-04-01).
- **Fix:** Added `&& !effSemanticDisabled` to the guard (`if sBndl != nil && !effSemanticDisabled`) so the guardrail lookup falls back to the already-default `integ.NoopLookup{}` under the gate.
- **Files modified:** internal/daemon/daemon.go
- **Commit:** 2b3092b9

**2. [Rule 1 - Bug] Explicit nil-clear of the SemanticSkill accessor block (process-global singleton staleness)**
- **Found during:** Task 2 GREEN (full-suite `go test ./internal/daemon/...` — order-dependent failure; the test passed in isolation but failed after `TestSemanticBundleWiresP1Accessors` ran first).
- **Issue:** `SemanticSkill` is a process-global singleton (`semantic.GetSemanticSkill()`). Merely SKIPPING the setters under the gate leaves a stale real accessor wired by an earlier non-gated `daemon.New` in the same process — `ImpactLookup` was still non-nil under the gate. This is the same Pitfall-5 class the plan flags for the repomap singleton, applied to the skill.
- **Fix:** Added an `else if b.skill != nil` branch clearing all 16 accessors to nil (idempotent null-object reset), mirroring the `rs.SetSemanticLookup(nil)` repomap reset.
- **Files modified:** internal/daemon/semantic_wiring.go
- **Commit:** 160d3d76

The plan's own doctrine ("idempotent null-object, mirror SetEnrichFn(nil)") anticipated this for the repomap path; extending it to the skill singleton is the precise realization for the accessor block.

## Self-Check: PASSED
- internal/daemon/semantic_gate.go — FOUND (resolveSemanticDisabled + gatedSymbolsLookupFn + gatedCfgGate)
- internal/daemon/semantic_gate_test.go — FOUND (4 test funcs)
- internal/daemon/daemon.go — FOUND (effSemanticDisabled resolution + 4 gated hand-outs + SetSemanticLookup(nil))
- internal/daemon/semantic_wiring.go — FOUND (effSemanticDisabled param + gated/cleared accessor block)
- internal/daemon/daemon_test_export_test.go — FOUND (semanticBundleForTest)
- Commit 124b8b36 (test) — FOUND
- Commit 2b3092b9 (feat, Task 1) — FOUND
- Commit 160d3d76 (feat, Task 2) — FOUND
- Commit ae616817 (style) — FOUND

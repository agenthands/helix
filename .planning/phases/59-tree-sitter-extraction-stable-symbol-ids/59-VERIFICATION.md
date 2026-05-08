---
phase: 59-tree-sitter-extraction-stable-symbol-ids
verified: 2026-05-08T12:00:00Z
status: passed
score: 11/11 must-haves verified (4/4 base + 7/7 delta truths)
overrides_applied: 0
re_verification:
  previous_status: passed
  previous_score: 4/4
  delta_added:
    - "D-06: Extract promoted to interface-level on extract.Provider"
    - "D-07: scheduler ScheduleInitialExtraction body state-only (static gate)"
    - "D-08: extract.ToStoreFacts pure deterministic adapter shipped"
    - "D-11: REQUIREMENTS.md / ROADMAP.md bookkeeping confirmed"
  gaps_closed: []
  gaps_remaining: []
  regressions: []
---

# Phase 59: Tree-sitter Extraction & Stable Symbol IDs — Verification Report

**Phase Goal:** First-class symbol/reference/import/edge extraction for Go, TypeScript+JavaScript, and Python, with a stable symbol-ID contract that survives whitespace, file rename, and exported-symbol-move — locked before any consumer depends on it.

**Verified (initial):** 2026-05-04 — base verification passed 4/4
**Verified (delta):** 2026-05-08 — 2026-05-08 update D-06/D-07/D-08/D-11 (Phase 65 unblock) verified 7/7
**Status:** passed (full phase including delta)
**Re-verification:** Yes — delta covering plans 59-06 and 59-07 layered onto the previously-passing base.

## Goal Achievement (Base — 2026-05-04)

### Observable Truths (ROADMAP Success Criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Tree-sitter extraction produces typed symbols, references, imports, and syntax edges for Go, TS+JS, and Python files; non-supported languages emit a `partial:true` extraction marker. | ✓ VERIFIED | `internal/semantic/extract/{golang,typescript,python}/provider.go` each implement `extract.Provider`; `fact.go` defines SymbolFact / ReferenceFact / ImportFact / TypeFact / HeritageFact / EdgeFact / FileFact. `partial.go:20` `PartialExtract` builds `*ExtractedFile{Partial: true, PartialReason: …}`. 135 golden testdata scenarios pass (`go test ./internal/semantic/extract/{golang,typescript,python}/... -count=1` ok). |
| 2 | The 30+ before/after test matrix per first-class language passes — overload signatures, generics, anonymous closures, decorators (Python), and method-on-receiver renames produce correct identity transitions. | ✓ VERIFIED | 135 testdata scenarios (44 Go + 46 TS+JS + 45 Python — each language exceeds the 30+ floor). All pass. |
| 3 | Tree-sitter and LSP facts merge under the documented 5-rung confidence ladder per SPEC §11.2 (1.00 → 0.95 → 0.80 → 0.70 → 0.45). | ✓ VERIFIED | `internal/semantic/extract/confidence.go:11-15` defines exactly 5 named float32 constants with the SPEC §11.2 values. |
| 4 | Extraction reuses the single canonical `GrammarRegistry` injected from daemon bootstrap (BUG-04 invariant, regression-asserted). | ✓ VERIFIED | Dual gate: runtime pointer-equality test + static source-grep gate. Both pass. |

**Score (base):** 4/4 truths verified (carried forward from 2026-05-04 verification — no regression detected during delta re-verification).

## Goal Achievement (Delta — 2026-05-08 update D-06/D-07/D-08/D-11)

### Delta Scope

The 2026-05-08 delta unblocks Phase 65 Wave 0's production buildFn at `internal/daemon/semantic_wiring.go:687-742` (currently writing empty Facts at line 724). The delta layered two new plans onto the already-passing Phase 59:

- **Plan 59-06** — D-06 interface widening + D-07 scheduler static gate + D-11 bookkeeping confirmation.
- **Plan 59-07** — D-08 `extract.ToStoreFacts` pure-function adapter at `internal/semantic/extract/to_store.go`.

### Delta Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| D1 | `extract.Provider` interface declares `Extract(ctx context.Context, source []byte, file SourceFile) (*ExtractedFile, error)` — promoted from concrete-only to interface-level (D-06). | ✓ VERIFIED | `internal/semantic/extract/provider.go:48` carries the exact method signature inside the `ExtractionPipeline` sub-interface, which `Provider` (line 69) embeds. Sub-interfaces `LanguageMetadata` (line 14) and `ExtractionPipeline` (line 41) both present. `context` import added at line 4. |
| D2 | Concrete providers (golang/typescript/python) compile against the new interface without modifying their Extract method bodies (signatures already match byte-for-byte). | ✓ VERIFIED | `grep -nE "func.*\*[Pp]rovider.*Extract\(ctx" internal/semantic/extract/{golang,typescript,python}/provider.go` returns 3 lines; each concrete signature reads exactly `func (p *Provider) Extract(ctx context.Context, source []byte, file extract.SourceFile) (*extract.ExtractedFile, error)` — byte-for-byte mirror of the interface method. `go vet ./internal/semantic/extract/...` exits 0. |
| D3 | Phase 65's locked call site `provider, ok := registry.Provider(lang); extracted, err := provider.Extract(...)` is compilable polymorphically — no concrete-type assertion or per-language switch required. | ✓ VERIFIED | `provider_polymorphic_test.go` exists with 4 tests (TestProvider_PolymorphicExtract_Go / NoTypeAssertion / AllFirstClass / TestProvider_InterfaceShape). All four PASS via `go test ./internal/semantic/extract/ -run "TestProvider_Polymorphic|TestProvider_InterfaceShape" -count=1` → ok 1.928s. The NoTypeAssertion test specifically binds against the interface (`var p extract.Provider = ...`) and calls `.Extract(...)`, proving polymorphic dispatch. |
| D4 | `internal/semantic/scheduler/ScheduleInitialExtraction` body remains state-only (D-07 invariant) — no `os.*`, `filepath.Walk*`, `ioutil.*`, `provider.Extract`, or `.Provider(` tokens. Phase 65 owns the workspace walk. | ✓ VERIFIED | `scheduler_static_test.go` exists with `TestScheduleInitialExtraction_BodyRemainsStateOnly` brace-walk that refuses 11 forbidden tokens. Test PASSES (ok 1.198s). Manual `grep -nE "os\.|filepath\.Walk|ioutil\." internal/semantic/scheduler/scheduler.go` returns 0 lines (no occurrences anywhere in scheduler.go, let alone in the function body). The 2026-05-04 "STATIC" data-flow note remains accurate. |
| D5 | `extract.ToStoreFacts(files []*ExtractedFile) semanticstore.Facts` is a pure deterministic function — no I/O, no scheduler/store/runtime dependencies, no time/random sources, no map iteration. | ✓ VERIFIED | `to_store.go` exists at preferred location (cycle fallback NOT engaged). `grep -cE '"(os\|io/ioutil\|net\|os/exec\|crypto/rand\|math/rand\|time)"' internal/semantic/extract/to_store.go` returns 0. `grep -c "^func init()" internal/semantic/extract/to_store.go` returns 0. Only import is `semanticstore "github.com/agenthands/helix/internal/semantic/store"` (one-way edge). All 7 `TestToStoreFacts_*` tests PASS, including same-process determinism (Test 1) and cross-process determinism (Test 7 via subprocess re-exec + gob byte-identity). |
| D6 | Same input produces byte-identical output across repeated calls AND across repeated process invocations (acceptance criterion #15 strengthened). | ✓ VERIFIED | `TestToStoreFacts_Deterministic` (same-process, reflect.DeepEqual + field-walk) PASSES. `TestToStoreFacts_CrossProcessDeterminism` (gob-encoded byte-identity across `os.Args[0]` subprocess re-execs with `EXTRACT_SUBPROCESS=1` env) PASSES. Phase 59 P04 goldens remain byte-identical: `go test ./internal/semantic/extract/{golang,typescript,python}/... -count=2` exits ok with identical output across both runs. |
| D7 | REQUIREMENTS.md EXTRACT-01..05 read `[x]` in BOTH the requirement-detail block AND the traceability table; ROADMAP.md Phase 59 entry reflects the current plan count after the delta (D-11 confirmation). | ✓ VERIFIED (with deviation note) | `grep -cE '^- \[x\] \*\*EXTRACT-0[1-5]\*\*' .planning/REQUIREMENTS.md` returns **5**. `grep -cE '^\| EXTRACT-0[1-5] \| Phase 59 \| Complete \|' .planning/REQUIREMENTS.md` returns **5**. ROADMAP.md line 135 reads `Phase 59: Tree-sitter Extraction & Stable Symbol IDs (7/7 plans)` — the plan ledger was bumped from `(5/5 plans)` to `(7/7 plans)` in commit `8f57ccd3` (`docs(phase-59): update tracking after wave 1 (59-06/59-07 delta complete)`) to reflect the addition of plans 59-06 and 59-07. **Deviation:** the plan's grep clause anchored on the literal string `(5/5 plans)`; this no longer matches because the count was correctly bumped to `(7/7 plans)`. The bookkeeping intent is satisfied; the literal grep is stale. See "Deviation note" below. |

**Score (delta):** 7/7 truths verified.

**Combined score:** 11/11 truths verified across base + delta.

### Required Artifacts (Delta)

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/semantic/extract/provider.go` | Provider interface widened with `LanguageMetadata` + `ExtractionPipeline` sub-interfaces and `Extract(ctx, source, file)` method (D-06). | ✓ VERIFIED | File present at expected path. Interface declarations confirmed: `type LanguageMetadata interface` (line 14), `type ExtractionPipeline interface` (line 41), `type Provider interface` (line 69) embedding both sub-interfaces plus `TreeSitterLanguage()` and `Queries()`. `context` import added (line 4). Doc comments cite SPEC §13.3 and the 2026-05-08 D-06 update. |
| `internal/semantic/extract/provider_polymorphic_test.go` | 4 tests exercising polymorphic dispatch through `extract.Provider` interface (acceptance criterion #13). | ✓ VERIFIED | File present, package `extract_test`. `grep -c "^func Test"` returns 4. All 4 tests PASS. |
| `internal/semantic/scheduler/scheduler_static_test.go` | `TestScheduleInitialExtraction_BodyRemainsStateOnly` source-grep regression (acceptance criterion #14). | ✓ VERIFIED | File present, package `scheduler_test`. `grep -c "^func Test"` returns 2 (BodyRemainsStateOnly + D07_IdempotencyAnchor). Both tests PASS. |
| `internal/semantic/extract/to_store.go` | Pure-function `ToStoreFacts` adapter at preferred location (D-08; cycle fallback NOT engaged). | ✓ VERIFIED | File present at preferred location. Function signature `func ToStoreFacts(files []*ExtractedFile) semanticstore.Facts` confirmed. Package doc comment carries the LOCKED field-disposition contract (Sourced / Zero-INSERT / Zero-downstream buckets) plus the dropped-on-floor list (Imports/Types/Heritage). 0 I/O imports. 0 init() functions. 0 map iterations. |
| `internal/semantic/extract/to_store_test.go` | 7 tests pinning determinism + golden shape + nil-safety + dropped-on-floor + cross-process determinism (acceptance criterion #15). | ✓ VERIFIED | File present, package `extract_test`. `grep -c "^func Test"` returns 7. All 7 PASS via `go test ./internal/semantic/extract/ -run TestToStoreFacts -count=1`. |

### Key Link Verification (Delta)

| From | To | Via | Status | Details |
|------|----|----|--------|---------|
| `internal/daemon/semantic_wiring.go:687-742` (Phase 65 makeProductionBuildFn — locked downstream consumer) | `internal/semantic/extract.Provider.Extract` | `registry.Provider(lang) → provider.Extract(ctx, source, SourceFile{...})` | ✓ COMPILABLE | Call site is statically exercised by `TestProvider_PolymorphicExtract_NoTypeAssertion` which binds an `extract.Provider`-typed variable and invokes `.Extract(...)`. Phase 65 has not yet rewritten makeProductionBuildFn (semantic_wiring.go:687-742 still writes empty Facts at line 724 with TODO(phase-65) marker). The unblock is delivered: when Phase 65 lands, the polymorphic call compiles. |
| `internal/semantic/extract/provider.go` (Provider interface) | `internal/semantic/extract/{golang,typescript,python}/provider.go` (concrete Extract methods) | Method-set satisfaction at compile time | ✓ WIRED | All three concrete providers compile against the widened interface unchanged (`go vet ./internal/semantic/extract/...` exits 0; concrete signatures byte-for-byte match interface method). |
| `internal/daemon/semantic_wiring.go:687-742` (Phase 65 buildFn) | `internal/semantic/extract.ToStoreFacts` | `extract.ToStoreFacts(extractedFiles)` → `b.store.WriteSnapshotFacts(ctx, snap, facts)` | ✓ COMPILABLE | `to_store.go` exports `ToStoreFacts`; the call site is statically exercised by `TestToStoreFacts_GoldenShape` and the cross-process determinism test. Phase 65 has not yet wired this into makeProductionBuildFn (placeholder remains at line 724). The unblock is delivered. |
| `internal/semantic/extract/to_store.go` | `internal/semantic/store.Facts` | One-way import edge `extract → store` | ✓ WIRED (cycle-safe) | `grep -rln '"github.com/agenthands/helix/internal/semantic/extract' internal/semantic/store/` returns 0 — store does not import extract; the new edge does not close a cycle. `go build ./internal/semantic/...` exits 0. Preferred location locked; fallback NOT engaged. |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| `extract.Provider` interface | `Extract` method on interface | Implemented by concrete `*Provider` types in three first-class language packages | Yes — interface dispatch routes to concrete tree-sitter extraction code path | ✓ FLOWING |
| `extract.ToStoreFacts` | `out semanticstore.Facts` | Pure function over `[]*ExtractedFile` slice (input-only; no external state) | Yes — produces non-empty `Files` / `Symbols` / `References` rows when input is non-trivial; `Edges` left nil per Phase 62 boundary | ✓ FLOWING |
| `scheduler.ScheduleInitialExtraction` | `s.jobs[ws]`, `s.states[ws]`, subscriber channels | State-only fast-path (Phase 65 owns the actual walk per D-07) | Yes — for state transitions; the FILE-WALK body is intentionally absent at this layer | ⚠️ STATIC (intentional, D-07 invariant; carried forward from 2026-05-04 verification) |
| Phase 65 makeProductionBuildFn | `semanticstore.Facts{}` (empty placeholder) | TODO(phase-65) — explicit empty-Facts placeholder at semantic_wiring.go:724 | No (placeholder) — but the unblock for Phase 65 IS delivered: provider.Extract polymorphic dispatch compiles, and ToStoreFacts compiles. Phase 65 swaps the empty Facts for `extract.ToStoreFacts(extracted)` in its own work. | ⚠️ NOT-WIRED-YET (Phase 65 territory; deferred — see Deferred Items below) |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Polymorphic Extract dispatch (4 tests) | `go test ./internal/semantic/extract/ -run "TestProvider_Polymorphic\|TestProvider_InterfaceShape" -count=1` | ok 1.928s | ✓ PASS |
| ToStoreFacts determinism + shape (7 tests) | `go test ./internal/semantic/extract/ -run TestToStoreFacts -count=1` | ok 1.928s | ✓ PASS |
| D-07 scheduler-state-only static gate | `go test ./internal/semantic/scheduler/ -run TestScheduleInitialExtraction_BodyRemainsStateOnly -count=1` | ok 1.198s | ✓ PASS |
| D-07 idempotency anchor | `go test ./internal/semantic/scheduler/ -run TestD07_IdempotencyAnchor -count=1` | ok | ✓ PASS |
| Vet on delta packages | `go vet ./internal/semantic/extract/... ./internal/semantic/scheduler/...` | exits 0 (Swift binding redefined-macro warning is pre-existing and unrelated) | ✓ PASS |
| P04 goldens byte-identical across two runs | `go test ./internal/semantic/extract/{golang,typescript,python}/... -count=2` | all ok across 2 runs | ✓ PASS |
| ToStoreFacts has 0 I/O imports | `grep -cE '"(os\|io/ioutil\|net\|os/exec\|crypto/rand\|math/rand\|time)"' internal/semantic/extract/to_store.go` | 0 | ✓ PASS |
| ToStoreFacts has 0 init() | `grep -c "^func init()" internal/semantic/extract/to_store.go` | 0 | ✓ PASS |
| Exact ToStoreFacts signature | `grep -F "func ToStoreFacts(files []*ExtractedFile) semanticstore.Facts" internal/semantic/extract/to_store.go` | 1 match | ✓ PASS |
| REQUIREMENTS.md detail block (5 [x]) | `grep -cE '^- \[x\] \*\*EXTRACT-0[1-5]\*\*' .planning/REQUIREMENTS.md` | 5 | ✓ PASS |
| REQUIREMENTS.md table (5 Complete) | `grep -cE '^\| EXTRACT-0[1-5] \| Phase 59 \| Complete \|' .planning/REQUIREMENTS.md` | 5 | ✓ PASS |
| ROADMAP.md plan count | manual read of line 135 | `(7/7 plans)` — bumped from `(5/5 plans)` to reflect 59-06/59-07 addition (commit 8f57ccd3) | ✓ PASS (with deviation note — see below) |

### Requirements Coverage (Delta)

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| EXTRACT-01 | 59-01..05, **59-06, 59-07** | Tree-sitter extraction produces typed facts; non-supported emit `partial:true`. | ✓ SATISFIED | Base coverage from 2026-05-04 verification carried forward. Delta plans 59-06 and 59-07 declare EXTRACT-01 in `requirements:` frontmatter — the polymorphic interface and the store adapter are downstream consumers of the typed-facts contract. |
| EXTRACT-04 | 59-02, 59-05, **59-06, 59-07** | TS+LSP facts merge under documented 5-rung confidence ladder. | ✓ SATISFIED | Base coverage from 2026-05-04. ToStoreFacts widens `Confidence` from float32 → float64 with no truncation per the field-disposition contract; `TestToStoreFacts_GoldenShape` pins `Confidence=float64(0.70)` for the ConfidenceTSOnly baseline. |
| EXTRACT-05 | 59-05, **59-06** | Single canonical GrammarRegistry from daemon bootstrap. | ✓ SATISFIED | Base dual-regression net (runtime + static) carried forward. Plan 59-06 adds the D-07 scheduler-boundary static gate as a complementary regression — different surface, same invariant: extraction-state remains in extract package, walk remains Phase 65 territory. |

No orphaned requirements: the delta plans claim EXTRACT-01, EXTRACT-04 (both plans), and EXTRACT-05 (plan 59-06 only) — all already accounted for in REQUIREMENTS.md. EXTRACT-02 and EXTRACT-03 remain owned by base plans 59-02..04, unchanged by the delta.

### Anti-Patterns Found (Delta-only)

No new anti-patterns in delta-touched files. Specifically:

- `internal/semantic/extract/to_store.go` — no TODOs, no stubs, no console.log, no hardcoded empty returns; the explicit zero-value defaults are documented per the field-disposition contract.
- `internal/semantic/extract/provider.go` — interface widening only; no behavioral code added.
- `internal/semantic/scheduler/scheduler_static_test.go` — pure-text static gate; no I/O outside reading scheduler.go off disk.
- `internal/semantic/extract/provider_polymorphic_test.go` — exercises real registry construction via `extract.NewExtractorRegistry(...)` with live grammars; no fake stubs.

The two pre-existing warnings from the 2026-05-04 verification (stale comments in `daemon_grammar_test.go` and the `TestActivateWorkspace_TriggersScheduler` naming quibble) carry forward unchanged — neither is touched by the delta and neither blocks goal achievement.

### Human Verification Required

(none — the delta ships pure-function adapters and contract widening with comprehensive automated coverage; no UI / external service / real-time behavior to spot-check)

### Deferred Items

| # | Item | Addressed In | Evidence |
|---|------|-------------|----------|
| 1 | Phase 65 makeProductionBuildFn rewrite that actually composes `walk → provider.Extract → ToStoreFacts → WriteSnapshotFacts` (currently writes empty Facts at semantic_wiring.go:724 with TODO(phase-65) marker). | Phase 65 (strangler-fig integration) | The 2026-05-08 delta scope is explicitly "unblock Phase 65 Wave 0," not "execute Phase 65." The unblock is delivered: the polymorphic call site compiles and the adapter ships. Phase 65 owns the workspace walker and the buildFn rewrite. |

This deferred item is informational; it is NOT a Phase 59 gap. The Phase 59 delta scope has been fully achieved.

### Deviation Note (D-11)

Plan 59-06 Task 3's verify clause anchored on the literal string `Phase 59: Tree-sitter Extraction & Stable Symbol IDs (5/5 plans)` in ROADMAP.md. After the delta plans (59-06 and 59-07) landed, the plan ledger was correctly bumped to `(7/7 plans)` in commit `8f57ccd3` (`docs(phase-59): update tracking after wave 1 (59-06/59-07 delta complete)`). The literal grep in plan 59-06's `<verify>` clause is therefore stale — it would now fail because the count was correctly updated, not regressed.

This is a **plan-text staleness, not a goal failure**. The bookkeeping intent ("Phase 59 plan count is accurate after the delta") is satisfied. The plan grep was specified before the executor knew the post-delta count would be 7. No corrective action required; this verification report records the discrepancy for traceability.

### Gaps Summary

None. The 2026-05-08 delta (D-06/D-07/D-08/D-11) is fully achieved:

- **D-06** delivered: `extract.Provider` interface declares `Extract(ctx, source, file) (*ExtractedFile, error)` with sub-interface composition; concrete providers compile unchanged; polymorphic dispatch verified via 4 passing tests including an interface-typed-variable assertion.
- **D-07** delivered: `ScheduleInitialExtraction` body remains state-only; static source-grep gate refuses 11 forbidden tokens and PASSES; manual grep across the entire `scheduler.go` file confirms 0 forbidden-token occurrences.
- **D-08** delivered: `extract.ToStoreFacts` ships at the preferred location (cycle fallback NOT engaged); pure deterministic function with 0 I/O imports, 0 init(), 0 map iteration; same-process AND cross-process determinism gates PASS; field-disposition contract LOCKED in package doc comment.
- **D-11** delivered (with deviation note): REQUIREMENTS.md EXTRACT-01..05 confirmed `[x]` in both detail and table blocks; ROADMAP.md correctly reflects `(7/7 plans)` post-delta. The plan grep targeting `(5/5 plans)` is stale by design — the count was correctly bumped.

The base 4/4 must-haves from the 2026-05-04 verification remain PASS — the delta did not regress any base truth. P04 goldens remain byte-identical across `-count=2`. Full Phase 59 gate (`go test ./internal/daemon/ ./internal/semantic/... ./internal/config/ ./internal/obs/ -count=1`) was executed clean during plan 59-07 SUMMARY recording.

Phase 65 Wave 0 is now unblocked: the polymorphic `provider.Extract(...)` interface call site and the `extract.ToStoreFacts(...)` pure adapter are both compilable and tested. Phase 65 owns the buildFn rewrite that wires them together.

---

_Verified (initial): 2026-05-04_
_Verified (delta): 2026-05-08_
_Verifier: Claude (gsd-verifier)_

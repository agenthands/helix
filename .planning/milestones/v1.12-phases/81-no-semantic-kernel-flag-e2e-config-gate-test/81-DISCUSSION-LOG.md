# Phase 81: `no_semantic` Kernel Flag + E2E Config-Gate Test - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-06-20
**Phase:** 81-no-semantic-kernel-flag-e2e-config-gate-test
**Areas discussed:** Config-gate identity, Index build under ablation, Zero-read enforcement, vet guard depth

---

## Config-gate identity

| Option | Description | Selected |
|--------|-------------|----------|
| Distinct `bench_disabled` flag | Separate gate, independent of `cfg.SemanticIndex.Enabled`; forces NoopLookup + disabled cfgGate into all consumers regardless of bundle existence. Non-vacuous criterion-#2 test, distinct greppable symbol, decoupled from production feature flag. | ✓ |
| Reuse `SemanticIndex.Enabled=false` | Profile/daemon sets Enabled=false, reusing existing path. Zero new kernel code but zero-read assertion becomes trivially true (no store built); can't distinguish production-off from bench-ablation. | |
| `bench_disabled OR Enabled=false` (union gate) | Single resolved predicate threaded everywhere; one enforcement point but still lets bench_disabled keep the store built. | |

**User's choice:** Distinct `bench_disabled` flag
**Notes:** Decisive factor was making criterion #2's zero-read assertion non-vacuous — with a distinct flag the store can stay built and the test proves the gate *blocks reads*, rather than proving nothing reads a nonexistent store. Resolution lands at the daemon composition root (mirrors Phase 76 `effDisableLSP`). Captured as D-01/D-02.

---

## Index build under ablation

| Option | Description | Selected |
|--------|-------------|----------|
| Build-but-block reads | Daemon still builds/maintains the bundle (store healthy); bench_disabled forces every read consumer to NoopLookup + disabled cfgGate. Makes the zero-read test real; build cost is out-of-band so fairness unaffected. | ✓ |
| Skip build entirely | bench_disabled also skips bundle creation. Cheaper startup + no store to read, but weakens the assertion and re-converges with the rejected reuse option. | |
| Build, then poison the handle | Build the store but replace the read handle with one that errors on any query. Healthy index + hard tripwire, but more moving parts; overlaps with the enforcement decision. | |

**User's choice:** Build-but-block reads
**Notes:** Directly coupled to the distinct-flag choice. Store stays built so the runtime assertion proves the gate works. Captured as D-04.

---

## Zero-read enforcement

| Option | Description | Selected |
|--------|-------------|----------|
| Trace-tap counter assertion | Bench cell asserts `helix_semantic_*` counter family == 0 after the run via the existing trace-tap (the no_lsp zero-span mechanism, Phase 76 76-04). Gate is structural guarantee; counter is verification. | ✓ |
| Counter + poisoned store handle | Counter plus wrapping the duckdb read handle to error on any query under bench_disabled. Belt-and-suspenders; more code, and the poisoned handle could mask bugs the counter would surface cleanly. | |
| Counter + metric on gate itself | Counter plus an explicit metric/log when the gate forces a noop (proves the gate fired vs. nothing tried to read). | |

**User's choice:** Trace-tap counter assertion
**Notes:** Reuse the proven no_lsp infrastructure; no new runtime tripwire. The build-but-block gate is the structural guarantee, the counter is the independent verification. Captured as D-05.

---

## vet guard depth

| Option | Description | Selected |
|--------|-------------|----------|
| Extend with call-site gate check | Extend `vet-ablation-leakage` so every semantic read site must route through ChooseSource/the cfgGate. Criterion #3 ("flag any path that conditionally bypasses the gate") needs more than import-boundary; this pays down Phase 76 D-08's deferred runtime precision — Phase 81 is "that real case." | ✓ |
| Import-boundary only (Phase 76 floor) | Keep compile-time import leakage; rely on the gate + counter as runtime backstop. Less work, consistent with D-08's shipped scope, but under-delivers criterion #3's wording. | |
| Boundary + greppable-gate canary test | Import-boundary analyzer plus a unit/canary test pinning that the gate symbol is consulted at each enumerated consumer. Cheaper than a full call-site analyzer. | |

**User's choice:** Extend with call-site gate check
**Notes:** Criterion #3's "conditionally bypasses the gate" language requires call-site awareness that import-boundary alone can't provide. Ship with a deliberate green→red testdata violation. Captured as D-06.

---

## Claude's Discretion

- Exact config key spelling / koanf path / struct field / CLI override flag name for the gate (working name `semantic_index.bench_disabled`).
- Internal daemon/kernel config-struct threading shape for the effective-disabled boolean.
- Exact `helix_semantic_*` counter/metric names the assertion checks (and whether they exist from Phase 65 or need adding).
- Precise analyzer check shape (S-expression/SSA) and allowlist for the call-site extension.
- `MODE.md` wording/structure (must enumerate gate key + strangler-fig consumers).
- Shape of the E2E `no_semantic` smoke task (reuse Phase 77 seed task vs. dedicated fixture).

## Deferred Ideas

- Poisoned/erroring DuckDB read handle — rejected in favor of the clean counter assertion; revisit only if a real bypass survives the D-06 analyzer + counter.
- Skip-build-entirely under ablation — rejected (makes the zero-read test vacuous); revisit only if bench-arm startup cost becomes a measured problem.
- General runtime call-graph leakage analysis beyond the semantic gate — out of scope; D-06 is scoped to the semantic read sites / ChooseSource invariant.
</content>

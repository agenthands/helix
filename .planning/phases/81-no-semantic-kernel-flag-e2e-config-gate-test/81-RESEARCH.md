# Phase 81: `no_semantic` Kernel Flag + E2E Config-Gate Test - Research

**Researched:** 2026-06-20
**Domain:** Daemon composition-root config gating; strangler-fig consumer enumeration; go/analysis call-site linting; bench-cell runtime assertion
**Confidence:** HIGH (all findings verified against in-repo source with file:line; no external dependencies)

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Distinct `semantic_index.bench_disabled` gate — NOT a reuse of `cfg.SemanticIndex.Enabled=false`. Introduce a separate, opt-in disable flag, independent of the production `SemanticIndex.Enabled` feature flag. (Rationale: reusing `Enabled=false` makes the zero-read assertion vacuous because the daemon already skips bundle creation when `Enabled=false`. A distinct gate decouples ablation from production-off and gives the vet analyzer a greppable symbol. The roadmap explicitly names `semantic_index.bench_disabled`.)
- **D-02:** Resolve the gate once at the daemon composition root, mirroring Phase 76's `effDisableLSP` pattern in `internal/daemon/daemon.go`. Compute an effective "semantic disabled" boolean and thread it into the `symbolsLookupFn` closure, the `daemonCfgGate`, the repomap-skill `SetSemanticLookup`/`SetConfigGate` wiring (`daemon.go:764`), and the health wiring — single resolution point, not scattered per-callsite flag checks.
- **D-03:** Precedence: CLI override > profile YAML field > default-off, reusing Phase 76 D-01/D-02. First-class field on the existing `internal/profile/profiles/bench-no-semantic.yaml` AND a daemon CLI override. Default (no flag) = subsystem ENABLED.
- **D-04:** Build-but-block reads. Under `bench_disabled`, the daemon STILL creates/maintains the semantic bundle (DuckDB store healthy, index built) — but the gate forces every SemanticLookup read consumer to `integ.NoopLookup{}` and a disabled `ConfigGate`. Makes criterion #2 a real test. Index-build cost is out-of-band (daemon-side, not counted against agent tokens).
- **D-05:** Trace-tap counter assertion as the verification mechanism. The bench cell asserts the `helix_semantic_*` counter family == 0 after the `no_semantic` run, via the existing trace-tap (the exact mechanism the `no_lsp` arm uses to assert zero `lsp.*` spans, Phase 76 76-04). On violation, log and FAIL the bench cell. Build-but-block (D-04) is the structural guarantee; the counter assertion is the independent verification. No "poisoned store handle" tripwire.
- **D-06:** Extend `vet-ablation-leakage` (`internal/lint/ablationleakage`) with a call-site gate check — not just the import-boundary check Phase 76 shipped. Assert that every semantic-store read site routes through `integ.ChooseSource` / the wired `ConfigGate` rather than calling a lookup directly. Ship a deliberate green→red `testdata/` violation. Pays down Phase 76 D-08's deferred runtime precision.

### Claude's Discretion
- Exact config key name/path (`semantic_index.bench_disabled` is the working name; precise koanf key, struct field, CLI override flag spelling are planner discretion within D-01/D-03).
- Internal kernel/daemon config-struct threading shape for the effective-disabled boolean (D-02).
- The exact `helix_semantic_*` counter/metric names the assertion checks, and whether they already exist from Phase 65 or need to be added (D-05) — cross-checked during research (see §"Metric/Counter Surface").
- The precise S-expression / SSA shape of the call-site analyzer check and its allowlist (D-06).
- `MODE.md` wording and structure (criterion #4).
- The shape of the E2E `no_semantic` smoke task (reuse Phase 77 seed toolbench-go task vs. a dedicated fixture).

### Deferred Ideas (OUT OF SCOPE)
- Poisoned/erroring DuckDB read handle (rejected for this phase; revisit only if a real gate-bypass leak survives D-06 + counter assertion).
- Skip-build-entirely under ablation (rejected — D-04; makes zero-read test vacuous).
- General runtime call-graph leakage analysis beyond the semantic gate (D-06 scopes the call-site check to semantic read sites / the `ChooseSource` invariant only).
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| ABLATE-06 | Kernel-level `disable_semantic_subsystem` flag prevents any back-channel semantic-store read (incl. Phase 65 SemanticLookup, RankFiles, ExpandFrom); E2E `no_semantic` task makes zero DuckDB queries; runtime assertion logs and fails. | §"Config Key + Struct Threading" (where/how to add the flag), §"Consumer Enumeration + Single-Point Wiring" (the 8 consumers + the single gate point), §"Metric/Counter Surface" (the read-counter to assert — must be ADDED), §"vet Call-Site Extension" (D-06 analyzer), §"E2E Smoke Task Shape" (bench cell), §"Validation Architecture". |
</phase_requirements>

## Summary

Phase 81 closes the last ablation arm. The seam to disable already exists and was deliberately built to be switched off: `integ.NoopLookup{}`, `integ.ConfigGate`, and the `integ.ChooseSource(cfg, lookup, err)` priority ladder (which renders `source="tree_sitter"` the moment `cfg.SemanticIndexEnabled()` returns false) are all in `internal/semantic/integ/`. Phase 65 wired the production adapter (`integSemanticLookup`) and the production gate (`daemonCfgGate{enabled: cfg.SemanticIndex.Enabled}`) at the daemon composition root. Phase 76 supplied the **exact structural template** to copy: `effDisableLSP := cfg.DisableLSPSubsystem || activeProfile.DisableLSPSubsystem` at `daemon.go:287`, threaded into kernel config and every LSP-touching seam, with a CLI override flag (`--disable-lsp-subsystem`) and a profile YAML field, plus the `vet-ablation-leakage` analyzer.

The work is therefore: (1) add a distinct `bench_disabled` flag with the same CLI > profile > default-off precedence; (2) compute `effSemanticDisabled` once at the composition root; (3) thread it into the gate so all consumers see `NoopLookup` + a disabled `ConfigGate` — but keep the bundle built (D-04); (4) **add a read-path counter** (the `helix_semantic_*` family today counts writes/extraction/enrichment but has NO query/read counter — this phase must add one and emit it from the DuckDB read sites); (5) wire a bench-cell assertion that the counter == 0; (6) extend the `ablationleakage` analyzer with a call-site check; (7) write the MODE.md.

**Two findings that change the plan shape materially:**
1. **There is NO existing `helix_semantic_*` read/query counter.** The family (metrics.go:334-504) covers store-open, quarantine, extraction, live-updates, lsp-enrichment, pagerank, graph-version, types-resolution, compaction, vacuum — all writes/maintenance. None counts a *read*. D-05's assertion has nothing to assert against today. **This phase must add a read counter** (e.g. `helix_semantic_store_reads_total`) and emit it at the DuckDB query sites in `internal/semantic/store/`. This is a real deliverable, not just wiring.
2. **There is NO existing bench-cell zero-event runtime assertion** for the `no_lsp` arm either. The no_lsp guarantee today is **purely structural** (the daemon never wires the LSP seams, so no spans are emitted). The bench `MergedTrace` is built from **daemon-log line tapping** (`internal/eval/trace/tap.go` — `msg=="tool call"`/`msg=="receipt issued"`), NOT from OTel spans, and contains no span/counter inventory. The `your_agent_no_semantic` cell is currently marked `guarantee_pending_phase_81` (cell.go:53-59) precisely because this assertion does not exist yet. So D-05 ("reuse the no_lsp zero-span mechanism") is reusing a *pattern intent*, not lifting existing code — the planner must build the counter-scraping + assertion path. The simplest faithful implementation: scrape the daemon's Prometheus `/metrics` (admin listener) or parse a daemon-log emission of the counter after the run, assert == 0, fail the cell.

**Primary recommendation:** Add `bench_disabled bool koanf:"bench_disabled"` to `internal/semantic/config.go`'s `Config`; resolve `effSemanticDisabled := cfg.SemanticIndex.BenchDisabled || activeProfile.DisableSemanticSubsystem || <CLI override>` next to `effDisableLSP` at `daemon.go:287`; gate the four wiring points (symbolsLookupFn, symbolsCfgGate, healthCfgGate/healthLookup, repomap SetSemanticLookup/SetConfigGate at 764) AND the `b.skill.Set*Accessor` block in `newSemanticBundle` (semantic_wiring.go:252-269) so the SemanticSkill read tools get nil/Noop accessors; add a `helix_semantic_store_reads_total` counter emitted from the DuckDB read sites; build the bench-cell counter==0 assertion; extend `ablationleakage` with an SSA call-site check; write MODE.md.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Resolve `bench_disabled` → `effSemanticDisabled` | API/Backend (daemon composition root) | Config | Same tier `effDisableLSP` resolves; single decision point governs all consumers (D-02). |
| Force NoopLookup/disabled gate into read consumers | API/Backend (daemon wiring) | — | The gate is injected at wiring time; consumers themselves stay tier-agnostic. |
| Emit `helix_semantic_store_reads_total` | Database/Storage (`internal/semantic/store`) | Observability (`internal/obs`) | The counter must fire at the actual DuckDB read site to be a faithful "a read happened" signal. |
| Counter==0 assertion + fail-cell | Bench harness (`bench/runtime`) | Observability scrape | The assertion lives in the cell orchestrator, independent verification of the structural gate. |
| Call-site routing lint | Build/tooling (`internal/lint`, `make vet`) | — | Static, compile-time complement to the runtime assertion. |
| MODE.md doc | Bench docs (`bench/runners/your_agent_no_semantic`) | — | Operator/contributor-facing enumeration of the gate + consumers. |

## Standard Stack

This phase adds **zero external dependencies**. All work is in-tree Go against existing libraries.

### Core (already in `go.mod`, verified present)
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `golang.org/x/tools/go/analysis` | v0.43.0 [VERIFIED: go.mod] | go/analysis Analyzer framework for the D-06 vet extension | Already used by all 5 in-repo vet analyzers (noduckdb, nokernel2semantic, nosemantic2kernel, compactusesstore, ablationleakage). |
| `golang.org/x/tools/go/analysis/passes/buildssa` | v0.43.0 (same module) [CITED: pkg.go.dev/golang.org/x/tools/go/analysis/passes/buildssa] | Provides SSA form for a call-site (not just import) analyzer | Standard go/analysis way to get `*ssa.Function` bodies; required if D-06 needs call-graph precision beyond AST. |
| `golang.org/x/tools/go/analysis/analysistest` | v0.43.0 | green→red `testdata/` harness | Already the test driver for noduckdb/ablationleakage (`analysistest.Run`). |
| `github.com/prometheus/client_golang` (via `internal/obs`) | (in go.mod) [VERIFIED: internal/obs/metrics.go] | counter registration/emission for the new read counter | The `helix_semantic_*` family is already prometheus counters in metrics.go. |

### Supporting (in-tree packages — the seams to touch)
| Package | Purpose | When to Use |
|---------|---------|-------------|
| `internal/semantic/integ` | `NoopLookup`, `ConfigGate`, `ChooseSource`, `SourceTreeSitter` | The null-objects/ladder the gate forces — DO NOT reinvent. |
| `internal/daemon` (`daemon.go`, `semantic_wiring.go`) | composition root + bundle wiring | The single gate-resolution + threading point (D-02). |
| `internal/config`, `internal/semantic/config.go`, `internal/profile` | the flag's home + precedence | Add the field; mirror `DisableLSPSubsystem`. |
| `internal/cli/root.go` | CLI override flag | Mirror `--disable-lsp-subsystem`. |
| `internal/lint/ablationleakage` | D-06 analyzer extension | Add call-site check + testdata. |
| `internal/semantic/store` | DuckDB read sites | Emit the new read counter. |
| `bench/runtime` (`cell.go`), `bench/runners/your_agent_no_semantic` | E2E assertion + MODE.md | D-05 + criterion #4. |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Distinct `bench_disabled` flag | `SemanticIndex.Enabled=false` | REJECTED by D-01 — vacuous zero-read test (no bundle built to read). |
| SSA call-site analyzer (D-06) | AST-only direct-call detection | AST can find a direct method call by name but cannot prove cross-package that the receiver is a `SemanticLookup` and that the call is not preceded by a `ChooseSource` gate. SSA gives receiver types + call graph. **However**, the existing analyzers are all AST import-boundary (simpler). See §"vet Call-Site Extension" for the recommended middle path. |
| New `helix_semantic_store_reads_total` counter | Assert on an existing counter | No existing counter covers reads (verified). A write/extraction counter would be a false negative source. |
| Counter-scrape assertion | OTel span assertion | The bench trace pipeline taps **daemon log lines**, not OTel spans (verified tap.go). There is no span inventory to assert on; the semantic store emits no spans at all (verified: no `tracer.Start` in `internal/semantic/store`). A counter is the faithful signal. |

**Installation:** none (no new modules).

## Package Legitimacy Audit

> Not applicable — this phase installs **zero** external packages. All work is in-tree Go against modules already in `go.mod` (`golang.org/x/tools` v0.43.0, `prometheus/client_golang` already vendored). No `go get` is required.

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

## Architecture Patterns

### System Architecture Diagram

```
                       ┌─────────────────────────────────────────────┐
  CLI --no-semantic ──▶│  daemon.go composition root (step ~287)      │
  (override)           │  effSemanticDisabled =                       │
  profile YAML  ──────▶│     cfg.SemanticIndex.BenchDisabled          │
  bench_disabled:true  │   || activeProfile.DisableSemanticSubsystem  │
                       │   || <CLI override>                          │
                       └───────────────────┬─────────────────────────┘
                                           │ (one boolean)
            ┌──────────────────────────────┼───────────────────────────────────┐
            │                              │                                     │
            ▼                              ▼                                     ▼
  symbolsLookupFn()              repomap SetSemanticLookup /          newSemanticBundle:
  + symbolsCfgGate              SetConfigGate (daemon.go:764)        b.skill.Set*Accessor block
  (daemon.go:612-619)                                                (semantic_wiring.go:252-269)
            │                              │                                     │
   if effSemanticDisabled:       if effSemanticDisabled:            if effSemanticDisabled:
   lookupFn → NoopLookup{}       SKIP SetSemanticLookup             pass nil / Noop accessors
   cfgGate.enabled = false       cfgGate.enabled = false            (find_related_symbols,
            │                              │                         explain_symbol_deep,
            ▼                              ▼                         validate_graph_edge,
  find_references,            get_repo_map / get_context             cluster/impact tools)
  analyze_blast_radius,       ──▶ ChooseSource → SourceTreeSitter            │
  get_health                                                                  ▼
            │                                                        (D-04: store STILL built;
            ▼                                                         bundle exists but reads
  ChooseSource(cfgGate,…)                                            forced to Noop)
  → SourceTreeSitter
  (gate is FIRST priority)
            │
            ▼  (independent verification, D-05)
  ┌──────────────────────────────────────────────────────┐
  │ bench cell (bench/runtime/cell.go)                    │
  │  run no_semantic task → scrape helix_semantic_store_  │
  │  reads_total → assert == 0 → else LOG + FAIL cell     │
  └──────────────────────────────────────────────────────┘
            │
            ▼  (static complement, D-06)
  ┌──────────────────────────────────────────────────────┐
  │ make vet → vet-ablation-leakage call-site check:      │
  │  every semantic read site must be preceded by         │
  │  ChooseSource / gated by ConfigGate (green→red testdata)│
  └──────────────────────────────────────────────────────┘
```

### Recommended Project Structure (files touched/added)
```
internal/semantic/config.go            # ADD: BenchDisabled bool `koanf:"bench_disabled"`
internal/profile/profile.go            # ADD: DisableSemanticSubsystem bool `yaml:"disable_semantic_subsystem"`
internal/config/config.go              # ADD (optional): top-level DisableSemanticSubsystem for CLI override symmetry
internal/cli/root.go                   # ADD: --disable-semantic-subsystem (or --no-semantic) flag + override (mirror lines 75-76, 151-181)
internal/daemon/daemon.go              # ADD effSemanticDisabled @ ~287; gate the 4 wiring points (612-619, 627-629, 661, 764-782)
internal/daemon/semantic_wiring.go     # GATE the b.skill.Set*Accessor block (252-269) under effSemanticDisabled
internal/semantic/store/*.go           # ADD: emit helix_semantic_store_reads_total at DuckDB read sites
internal/obs/metrics.go                # ADD: helix_semantic_store_reads_total counter + helper
internal/lint/ablationleakage/         # EXTEND analyzer with call-site check; ADD testdata green→red fixtures
internal/profile/profiles/bench-no-semantic.yaml  # ADD: disable_semantic_subsystem: true
bench/runtime/cell.go                  # ADD: counter==0 assertion for no_semantic; remove guarantee_pending_phase_81
bench/runners/your_agent_no_semantic/MODE.md      # REWRITE: gate key + consumer enumeration (criterion #4)
```

### Pattern 1: Composition-root effective-disable boolean (the D-02 template, copy verbatim)
**What:** Resolve one boolean OR-ing CLI override + profile field at `daemon.go`, thread it down.
**When to use:** This is the canonical Phase 76 pattern; replicate exactly.
**Example:**
```go
// Source: internal/daemon/daemon.go:287-294 (effDisableLSP — the template)
effDisableLSP := cfg.DisableLSPSubsystem || activeProfile.DisableLSPSubsystem
effDisableSE := cfg.DisableStructuredEditSubsystem || activeProfile.DisableStructuredEditSubsystem
// Phase 81 ADD (next to the above):
effSemanticDisabled := cfg.SemanticIndex.BenchDisabled || activeProfile.DisableSemanticSubsystem
if effDisableLSP || effDisableSE || effSemanticDisabled {
    logger.Info("subsystem ablation flags resolved",
        "disable_lsp_subsystem", effDisableLSP,
        "disable_structured_edit_subsystem", effDisableSE,
        "disable_semantic_subsystem", effSemanticDisabled, // ADD
    )
}
```

### Pattern 2: Gating the symbols + repomap + health wiring (force Noop + disabled gate)
**What:** When `effSemanticDisabled`, the lookupFn returns `NoopLookup{}` and the cfgGate reports disabled — even though `sBndl != nil` (D-04 build-but-block).
**Example:**
```go
// Source: internal/daemon/daemon.go:612-619 (CURRENT) — gate added by Phase 81
symbolsLookupFn := func() integ.SemanticLookup {
    if effSemanticDisabled {        // ADD: D-04 build-but-block — bundle exists, reads forced Noop
        return integ.NoopLookup{}
    }
    if sBndl != nil {
        return sBndl.integLookupAccessor()
    }
    return integ.NoopLookup{}
}
// cfgGate: report disabled so ChooseSource → SourceTreeSitter as the FIRST priority.
symbolsCfgGate := &daemonCfgGate{enabled: cfg.SemanticIndex.Enabled && !effSemanticDisabled}
symbols.RegisterTools(mcpServer, k, wsKeyFn, symbolsLookupFn, symbolsCfgGate)
```
Apply the same `&& !effSemanticDisabled` to `healthCfgGate` (629), `healthLookup` (627, via the gated symbolsLookupFn), and the repomap wiring (777-782):
```go
// Source: internal/daemon/daemon.go:777-782 (CURRENT) — gate added by Phase 81
if rs := repomapSkill.GetRepoMapSkill(); rs != nil {
    rs.SetConfigGate(&daemonCfgGate{enabled: cfg.SemanticIndex.Enabled && !effSemanticDisabled})
    if sBndl != nil && !effSemanticDisabled {   // ADD: skip the production lookup under the gate
        rs.SetSemanticLookup(sBndl.integLookupAccessor())
    }
    // (when effSemanticDisabled and a prior wiring set a lookup, explicitly
    //  SetSemanticLookup(nil) for idempotent null-object — mirrors SetEnrichFn(nil) at 710)
}
```

### Pattern 3: Gating the SemanticSkill read-tool accessors (the non-ChooseSource consumers)
**What:** `find_related_symbols`, `explain_symbol_deep`, `validate_graph_edge`, and the Phase 72 cluster/impact tools do NOT route through `ChooseSource` — they read DuckDB directly through ~15 `Set*Accessor` setters wired in `newSemanticBundle` (semantic_wiring.go:252-269). To honor criterion #1 ("all SemanticLookup paths see NoopLookup"), the accessor block must be skipped/nil'd under the gate.
**Why it matters:** These tools are profile-*excluded* on the no_semantic arm (the agent can't call them), but criterion #1 enumerates them and ABLATE-06 says "any back-channel read" — so the structural guard must extend here too. The handlers already guard nil accessors (they degrade gracefully — see skill.go:42 "nil means the seam is unwired (handler MUST guard)").
**Example:**
```go
// Source: internal/daemon/semantic_wiring.go:252-269 (CURRENT)
// Plumb effSemanticDisabled into newSemanticBundle (new param) and:
b.skill = semantic.GetSemanticSkill()
if b.skill != nil && !effSemanticDisabled {   // ADD the gate
    b.skill.SetStore(storeAcc)
    // ... all 15 setters ...
}
// When effSemanticDisabled: leave accessors nil → handlers degrade; SetImpactLookup
// (ExpandFrom) + the integLookupAccessor RankFiles path are likewise never wired.
```
**Note:** `RankFiles` and `ExpandFrom` (the named back-channel consumers in ABLATE-06) are methods of `integSemanticLookup` (semantic_wiring.go:865, 1076). They are reached via `integLookupAccessor()`. Gating the symbolsLookupFn + the repomap SetSemanticLookup + the `b.skill.SetImpactLookup` covers every path that hands out an `integLookupAccessor`. Verify no other call site constructs `integSemanticLookup` directly (grep `integLookupAccessor()` — three call sites: daemon.go:614, daemon.go:780, semantic_wiring.go:266; all gated by the patterns above).

### Anti-Patterns to Avoid
- **Per-callsite flag checks** (`if effSemanticDisabled { ... }` scattered in each handler) — D-02 mandates one resolution point. The handlers should be flag-agnostic; the gate is applied at wiring time by handing them Noop/nil.
- **Reusing `Enabled=false`** — D-01 explicitly forbids (vacuous test).
- **Skipping bundle construction** — D-04 explicitly forbids (the store must be built so the zero-read test is real). Do NOT add `effSemanticDisabled` to the `if cfg.SemanticIndex.Enabled` guard at daemon.go:317 or the `if semanticStore != nil` guard at 506.
- **Asserting on a write/extraction counter** — false-negative source; the read counter must be net-new.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Null SemanticLookup | A custom no-op struct | `integ.NoopLookup{}` (internal/semantic/integ/noop.go:25) | Already the canonical null-object; `ChooseSource` is tested against it. |
| Source-selection ladder | A new `if disabled { tree_sitter }` | `integ.ChooseSource(cfgGate, lookup, err)` (source_select.go:56) | Config-gate is already its FIRST priority → `SourceTreeSitter`. |
| CLI override precedence | New precedence logic | Copy `--disable-lsp-subsystem` handling (root.go:75-76, 151, 176-181) | Exact opt-in-OR pattern, only-apply-when-set semantics. |
| Profile YAML field | New loader code | Copy `DisableLSPSubsystem` (profile.go:46-51) — `yaml:` tag, zero-value=enabled | First-class field already round-trips via the existing loader. |
| go/analysis test harness | Custom AST walker test | `analysistest.Run` + `testdata/src/...` (noduckdb_test.go) | The house pattern; green→red via `// want` comments. |
| Prometheus counter | Manual atomic int | `internal/obs` counter registration (metrics.go:334-504 family) | The `helix_semantic_*` family is already registered there with helpers. |

**Key insight:** The entire null-object/gate machinery was pre-built in Phase 65 to be switched off. This phase wires a new flag to force it; it invents almost nothing on the gate side. The genuinely-new code is (a) the read counter (no read counter exists), (b) the bench-cell counter assertion (no zero-event runtime assertion exists), and (c) the SSA call-site lint extension.

## Runtime State Inventory

> This is a config-gate/wiring phase, not a rename/migration. No stored data, OS-registered state, or build artifacts carry a renamed string. The only "state" is config keys and a profile field — listed below for completeness.

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | None — the DuckDB store is built but NOT mutated by this phase (D-04 keeps it built; the gate only blocks reads). | none |
| Live service config | The `bench-no-semantic.yaml` profile gains `disable_semantic_subsystem: true`. This profile is in git (not a UI/DB). | code edit (YAML field) |
| OS-registered state | None. | none — verified: no OS task/daemon registration touches semantic config. |
| Secrets/env vars | None. | none |
| Build artifacts | The `vet-ablation-leakage` binary (`cmd/vet-ablation-leakage`) is rebuilt by `make vet` from source (Makefile:60-61); no stale artifact risk (the Makefile rule depends on `internal/lint/ablationleakage/*.go`). | none — `make vet` rebuilds automatically. |

## Common Pitfalls

### Pitfall 1: `helix_semantic_*` has no read counter — D-05 cannot assert anything today
**What goes wrong:** Planner assumes the counter exists and writes an assertion against a phantom metric; assertion is vacuously green forever.
**Why it happens:** The family name `helix_semantic_*` suggests broad coverage, but metrics.go:334-504 lists only store-open, quarantine, extraction, live-updates, lsp-enrichment, pagerank, graph-version, types-resolution, compaction, vacuum — all writes/maintenance, zero reads.
**How to avoid:** This phase MUST add a read counter (`helix_semantic_store_reads_total` recommended) and emit it from the actual DuckDB query sites in `internal/semantic/store/`. Make this an explicit Wave-0/early task. The assertion target is the counter you add.
**Warning signs:** A grep for `reads_total|query.*total|Read.*Inc` in metrics.go returns nothing.

### Pitfall 2: The no_lsp "zero-span trace-tap" precedent does not exist as reusable code
**What goes wrong:** Planner treats D-05 as "lift the no_lsp span-assertion helper" — but there is none. The no_lsp guarantee is purely structural (no seams wired → no spans). The bench trace is built from daemon-log line taps (tap.go), not OTel spans.
**Why it happens:** CONTEXT/roadmap phrasing ("reuse the no_lsp zero-span mechanism") describes intent, not an existing function.
**How to avoid:** Build the counter-scrape + assert path from scratch. Recommended: have the daemon expose the counter on its admin/Prometheus endpoint (already supported via `observability.Metrics()`), or emit a single `msg="semantic reads total"` daemon-log line at shutdown that the existing tap can parse; the cell scrapes/parses, asserts == 0, logs + fails on violation.
**Warning signs:** `grep -rn "lsp\.\*\|zero.*span\|assertZero" bench/` returns nothing (confirmed).

### Pitfall 3: SemanticSkill read tools bypass ChooseSource entirely
**What goes wrong:** Planner gates only the four `ChooseSource` consumers (symbols/repomap/health) and believes criterion #1 is satisfied — but `find_related_symbols`/`explain_symbol_deep`/`validate_graph_edge`/cluster/impact tools read DuckDB through `Set*Accessor` setters (semantic_wiring.go:252-269), not through `ChooseSource`. A back-channel read could still happen if these accessors stay wired.
**Why it happens:** Two different consumer families: (a) ChooseSource-gated (symbols, repomap, health), (b) direct-accessor (SemanticSkill 10 tools). CONTEXT enumerates both but they wire differently.
**How to avoid:** Gate the `b.skill.Set*Accessor` block under `effSemanticDisabled` (Pattern 3). Handlers already guard nil accessors. Note these tools are also profile-excluded on the arm, so this is defense-in-depth — but criterion #1 + ABLATE-06 demand it.
**Warning signs:** Counter > 0 in the assertion despite source==tree_sitter on get_repo_map.

### Pitfall 4: Breaking the source="tree_sitter" vs source="fallback" distinction
**What goes wrong:** If the cfgGate is left `enabled=true` but the lookup is forced to Noop, `ChooseSource` takes the defensive D-05 path → `SourceFallback` + `index_disabled`, NOT `SourceTreeSitter`. Criterion #1 demands `source == "tree_sitter"`.
**Why it happens:** `ChooseSource` priority: gate-disabled FIRST → tree_sitter; gate-enabled-but-Noop → fallback (source_select.go:59-65).
**How to avoid:** Under `effSemanticDisabled`, set `cfgGate.enabled = false` (so the FIRST priority fires) AND hand out Noop. Both. The cfgGate-disabled is what produces `tree_sitter`.
**Warning signs:** get_repo_map returns `source="fallback"`/`fallback_reason="index_disabled"` instead of `tree_sitter`.

### Pitfall 5: Idempotent null-object wiring on process-global singletons
**What goes wrong:** RepoMapSkill is a process-global singleton (skill.go:263 `GetRepoMapSkill`). If a prior code path called `SetSemanticLookup(real)` and the gate path just skips the setter, the stale real lookup persists.
**Why it happens:** Skip-the-setter ≠ clear-the-setter. Phase 76 handles this for SetEnrichFn by explicitly calling `SetEnrichFn(nil)` under the gate (daemon.go:710).
**How to avoid:** Under `effSemanticDisabled`, explicitly `rs.SetSemanticLookup(nil)` (the lookup() accessor normalizes nil → NoopLookup, skill.go:212-220) rather than just skipping. Mirror the daemon.go:710 idempotent-null pattern.
**Warning signs:** Gate works on a fresh daemon but leaks after a re-wire.

### Pitfall 6: AST-only call-site analyzer can't prove the ChooseSource invariant
**What goes wrong:** D-06 wants "every semantic read site routes through ChooseSource". A pure-AST check (like the existing import-boundary analyzers) can detect a *named* method call but cannot resolve that the receiver is a `SemanticLookup` across packages, nor that a `ChooseSource` guard precedes it.
**Why it happens:** AST has no type/call-graph info; the existing analyzers only match import paths (string compare).
**How to avoid:** Use `buildssa` to get typed call sites, OR scope the AST check narrowly: flag any direct call to a `SemanticLookup` interface method (e.g. `.ExpandFrom(`, `.RankFiles(`, `.ValidateCriticalEdges(`) outside an allowlisted set of files (the integ adapter + the gated wiring), with a `// want` testdata fixture. The narrow-AST approach matches the house style and is sufficient for the green→red criterion; SSA is the higher-precision option if the planner wants call-graph proof. **Recommendation:** start narrow-AST (matches existing analyzers, lower risk), document the SSA upgrade as future work — this is exactly the "until a real case appears" precision D-08 deferred.
**Warning signs:** Analyzer either over-flags legitimate adapter code or can't see cross-package receivers.

## Code Examples

### Adding the config field (mirror Enabled at config.go:31)
```go
// Source: internal/semantic/config.go:27-31 (Enabled is the sibling)
type Config struct {
    Enabled bool `koanf:"enabled"`
    // Phase 81 ABLATE-06 ADD: distinct ablation gate (NOT a reuse of Enabled).
    // When true, the daemon STILL builds the store/bundle (D-04) but forces
    // every SemanticLookup read consumer to integ.NoopLookup{} + disabled
    // ConfigGate. Default false = subsystem ENABLED. Resolved at the daemon
    // composition root OR'd with the profile field + CLI override (D-02/D-03).
    BenchDisabled bool `koanf:"bench_disabled"`
    // ... rest unchanged ...
}
```

### Profile field (mirror DisableLSPSubsystem at profile.go:46-51)
```go
// Source: internal/profile/profile.go:46-51 (DisableLSPSubsystem is the template)
// Phase 81 ABLATE-06 ADD to type Profile:
DisableSemanticSubsystem bool `yaml:"disable_semantic_subsystem"`
```

### CLI flag + override (mirror root.go:75-76, 151, 176-181)
```go
// Source: internal/cli/root.go:75-76 (flag decl) + 151 (read) + 176-181 (override)
rootCmd.Flags().Bool("disable-semantic-subsystem", false,
    "Ablation override: force-disable semantic-store reads (NoopLookup, tree-sitter source)")
// ...
disableSemantic, _ := cmd.Flags().GetBool("disable-semantic-subsystem")
// ...
if disableSemantic {
    overrides["semantic_index.bench_disabled"] = true
}
```

### New read counter (mirror the family at metrics.go:334-350)
```go
// Source pattern: internal/obs/metrics.go:334-350 (helix_semantic_store_open_total)
// Register in the same block:
{
    Name: "helix_semantic_store_reads_total",
    Help: "Semantic store DuckDB read/query operations (Phase 81 ABLATE-06: must be 0 on the no_semantic arm).",
}
// Helper (mirror SemanticStoreOpenInc at metrics.go:713):
func (m *Metrics) SemanticStoreReadsInc() { /* counter.Inc() */ }
// Emit at the actual DuckDB read sites in internal/semantic/store/*.go
// (the SELECT execution path — wrap the query helper so EVERY read increments).
```

### vet call-site analyzer test (mirror noduckdb_test.go)
```go
// Source: internal/lint/noduckdb/analyzer_test.go (green→red shape)
func TestAnalyzer_RejectsDirectSemanticReadOutsideGate(t *testing.T) {
    analysistest.Run(t, analysistest.TestData(), ablationleakage.Analyzer, "badgate")
}
func TestAnalyzer_AllowsReadBehindChooseSource(t *testing.T) {
    analysistest.Run(t, analysistest.TestData(), ablationleakage.Analyzer, "goodgate")
}
// testdata/src/badgate/imports.go contains a direct `.ExpandFrom(` call with a
// `// want "must route through integ.ChooseSource"` comment (the red fixture);
// testdata/src/goodgate/imports.go gates the same call behind ChooseSource (green).
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Scattered per-callsite subsystem flag checks | Single composition-root effective-disable boolean (`effDisableLSP`) + null-object injection at wiring | Phase 76 (v1.11) | The D-02 template; copy it exactly. |
| Tool-filter-only no_semantic (profile excludes 10 tools) | Kernel-level gate forcing Noop + disabled cfgGate (this phase) | Phase 81 (v1.12) | Closes the back-channel get_repo_map/get_context + RankFiles/ExpandFrom reads. |
| Import-boundary lint only (`ablationleakage` v1) | Call-site routing lint (this phase, D-06) | Phase 81 | Pays down Phase 76 D-08's deferred runtime precision. |

**Deprecated/outdated:**
- `your_agent_no_semantic/MODE.md`'s `ablation_status: guarantee_pending_phase_81` section (cell.go:53-59 + MODE.md) — this phase REMOVES the deferral and lands the real guarantee. The MODE.md must be rewritten to document the gate key + consumer enumeration (criterion #4), dropping the "deferred guarantee" section.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | No existing `helix_semantic_*` read/query counter exists (verified by grep of metrics.go; family is writes/maintenance only). | Pitfall 1, Metric Surface | If a read counter exists elsewhere, the planner could assert on it instead of adding one — lower-effort. Low risk: grep was exhaustive over internal/obs + internal/semantic. |
| A2 | The bench MergedTrace is built from daemon-log line tapping, not OTel spans, so D-05 needs a net-new counter-scrape assertion. | Pitfall 2 | If an OTel span pipeline exists in bench, the assertion could hook spans. Low risk: tap.go + schema.go inspected; no span inventory present. |
| A3 | Narrow-AST call-site check (flagging direct `.ExpandFrom(`/`.RankFiles(`/`.ValidateCriticalEdges(` outside an allowlist) satisfies D-06's green→red criterion without full SSA. | Pitfall 6, vet Extension | If reviewers demand call-graph proof, SSA is needed (more effort). Medium risk — confirm acceptable precision with the planner/discuss-phase. SSA via buildssa is available (v0.43.0) as the upgrade. |
| A4 | `disable_semantic_subsystem` is the best CLI/profile spelling (mirrors `disable_lsp_subsystem`); the koanf key is `semantic_index.bench_disabled` per roadmap. | Config threading | Naming is D-01/D-03 discretion; no functional risk. |
| A5 | Gating the 3 `integLookupAccessor()` call sites (daemon.go:614, 780; semantic_wiring.go:266) + the `b.skill.Set*` block covers EVERY back-channel read path. | Consumer Enumeration | If a 4th construction site exists, a leak survives. Verified by grep of `integLookupAccessor` + `integSemanticLookup{` — three accessor call sites; one struct-literal constructor (in integLookupAccessor itself). Low risk. |

## Open Questions (RESOLVED)

All three open questions are substantively resolved by plan-owned Wave-0 spikes (Q1, Q2) and a locked plan decision (Q3). The technical content below is unchanged from research; each item now carries its resolution + owning task.

1. **Where exactly are the DuckDB read sites to instrument with the new counter?**
   - What we know: reads happen in `internal/semantic/store/*.go` (the `SELECT` execution path) and are surfaced via `rankStoreAdapter`, `retrievalAdapter`, and the `integSemanticLookup` methods (RankFiles, ExpandFrom, etc.).
   - What's unclear: whether to instrument at the lowest `store.Query`/`store.Exec` helper (one site, catches all reads) vs. per-method (more labels). The single-chokepoint approach is recommended for a faithful "any read" signal.
   - Recommendation: wrap the store's read-query helper once; add a `helix_semantic_store_reads_total` counter there. A Wave-0 task should locate the exact helper (grep `db.Query`/`conn.Query` in internal/semantic/store).
   - **RESOLVED:** deferred to **81-01 Task 0 (Wave-0 spike)** — locate the exact single read chokepoint in `internal/semantic/store/duckdb.go` (the `db.Query`/`conn.Query` helper) and wrap it once with `helix_semantic_store_reads_total`. The single-chokepoint approach is the chosen design.

2. **Counter exposure mechanism for the bench cell — Prometheus scrape vs. daemon-log emission?**
   - What we know: the daemon has `observability.Metrics()` (Prometheus) and an admin listener; the bench cell taps daemon.log lines.
   - What's unclear: whether the bench daemon subprocess enables the admin/metrics endpoint (cell.go starts it over a Unix socket with HTTP disabled — D-06 no-TCP). A Prometheus HTTP scrape may not be reachable.
   - Recommendation: emit a single `msg="semantic store reads total" count=N` daemon-log line at shutdown (parseable by the existing tap.go pattern), OR expose the counter value over the gRPC/admin path the cell already uses. Confirm the daemon's metrics reachability in the bench sandbox during planning.
   - **RESOLVED:** deferred to **81-05 Task 0 (Wave-0 spike)** — confirm the reachable exposure path in the HTTP-disabled Unix-socket sandbox and pick one of (A) a single daemon-log line at shutdown (recommended given HTTP is disabled) or (B) the existing gRPC/admin path. If path (A) is chosen, the daemon-side emission lands at the daemon shutdown path (`internal/daemon/daemon.go` — the `d.shutdown()` path reached after `g.Wait()`, ~line 1252); 81-05 declares that file as a contingency scope expansion (see 81-05 `files_modified` + Task 0 acceptance criteria).

3. **Does criterion #1's enumerated list require the SemanticSkill direct-accessor tools to be gated, given they're already profile-excluded?**
   - What we know: criterion #1 names them; ABLATE-06 says "any back-channel read"; they're excluded from the agent surface by bench-no-semantic.yaml.
   - Recommendation: gate them anyway (Pattern 3) — defense-in-depth + literal criterion #1 compliance. The handlers already nil-guard.
   - **RESOLVED: YES** — locked in **81-04 Task 2**: the SemanticSkill direct accessors (`RankFiles`/`ExpandFrom` via `internal/skill/semantic/accessors.go`) ARE gated under `bench_disabled` for defense-in-depth + literal criterion #1 compliance, not relying on profile exclusion alone.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain (CGO=1) | build + test | ✓ | per CLAUDE.md | — |
| `golang.org/x/tools` | go/analysis vet extension | ✓ | v0.43.0 [VERIFIED: go.mod] | — |
| DuckDB (modernc/duckdb-go) | semantic store (built under D-04) | ✓ | in go.mod | platform-stubbed on windows/arm64 (daemon.go:311 — store nil, gate still valid) |
| `make vet` toolchain | D-06 analyzer wiring | ✓ | Makefile:60-61 | — |

**Missing dependencies with no fallback:** none
**Missing dependencies with fallback:** windows/arm64 has no DuckDB (store is nil); on that platform the gate is trivially satisfied (no store to read) — the no_semantic E2E test should be skipped or marked N/A there, mirroring the existing D-14 platform-stub handling.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go standard `testing` + `golang.org/x/tools/go/analysis/analysistest` (for the vet analyzer) |
| Config file | none (Go convention) |
| Quick run command | `go test ./internal/daemon/... ./internal/lint/ablationleakage/... ./internal/semantic/integ/... -count=1` |
| Full suite command | `go test ./... && make vet` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| ABLATE-06 | `bench_disabled: true` → `effSemanticDisabled` resolved at composition root | unit | `go test ./internal/daemon/ -run TestEffSemanticDisabled -count=1` | ❌ Wave 0 |
| ABLATE-06 | get_repo_map/get_context return `source == "tree_sitter"` under gate (ChooseSource gate-first) | unit | `go test ./internal/semantic/integ/ -run TestChooseSource -count=1` (existing source_select_test.go covers the ladder) + a new daemon-wiring test | ✅ ladder / ❌ wiring (Wave 0) |
| ABLATE-06 | symbolsLookupFn returns NoopLookup, cfgGate disabled, when gated (build-but-block) | unit | `go test ./internal/daemon/ -run TestSemanticGateForcesNoop -count=1` | ❌ Wave 0 |
| ABLATE-06 | SemanticSkill accessors left nil under gate (no direct DuckDB read path) | unit | `go test ./internal/daemon/ -run TestSemanticSkillAccessorsGated -count=1` | ❌ Wave 0 |
| ABLATE-06 | `helix_semantic_store_reads_total` increments on a real read; stays 0 when gated | unit | `go test ./internal/semantic/store/ -run TestReadCounter -count=1` | ❌ Wave 0 (counter is net-new) |
| ABLATE-06 (crit #2) | E2E no_semantic cell: counter == 0, else fail cell | integration | `go test ./bench/runtime/ -run TestNoSemanticZeroReads -count=1` | ❌ Wave 0 |
| ABLATE-06 (crit #3) | vet call-site check fires on direct read outside gate (green→red) | unit (analysistest) | `go test ./internal/lint/ablationleakage/ -count=1` | ✅ harness exists / ❌ new fixtures (Wave 0) |
| ABLATE-06 (crit #3) | `make vet` fails on the red testdata violation | smoke | `make vet` (after adding the analyzer to the vettool chain) | ✅ chain exists / ❌ new check (Wave 0) |
| ABLATE-06 (crit #4) | MODE.md documents gate key + consumer enumeration | manual/doc | review `bench/runners/your_agent_no_semantic/MODE.md` | ✅ file exists / needs rewrite |
| ABLATE-06 | bench-no-semantic.yaml carries `disable_semantic_subsystem: true` | unit | `go test ./internal/profile/ -run TestBenchNoSemanticProfile` (extend bench_profiles_test.go:142 which currently asserts NO kernel flag) | ✅ test exists / ⚠️ assertion must FLIP from false→true |

### Sampling Rate
- **Per task commit:** `go test ./internal/daemon/... ./internal/lint/ablationleakage/... ./internal/profile/... -count=1`
- **Per wave merge:** `go test ./... -count=1 && go vet ./...`
- **Phase gate:** `go test ./... && make vet` fully green before `/gsd-verify-work`.

### Wave 0 Gaps
- [ ] `internal/daemon/semantic_gate_test.go` — covers ABLATE-06 composition-root resolution + Noop/disabled-gate forcing + SemanticSkill accessor gating
- [ ] `internal/semantic/store/reads_counter_test.go` — the net-new read counter increments/stays-0
- [ ] `internal/obs/metrics.go` — register `helix_semantic_store_reads_total` (no test file change; covered by store test)
- [ ] `internal/lint/ablationleakage/testdata/src/badgate/` + `goodgate/` — green→red call-site fixtures with `// want` comments
- [ ] `bench/runtime/no_semantic_zero_reads_test.go` — E2E counter==0 assertion + fail-cell on violation
- [ ] **Assertion flip:** `internal/profile/bench_profiles_test.go:142` currently asserts `bench-no-semantic` sets NO kernel flag — this MUST be updated to assert `disable_semantic_subsystem: true` (this is a deliberate behavior change, not a regression).
- [ ] Framework install: none (Go stdlib + already-vendored analysistest)

## Security Domain

> `security_enforcement` is not explicitly configured; this is a Go-native, single-binary, local-daemon project with no auth/session/crypto surface introduced by this phase (config gate + lint + counter). The change reduces attack surface (blocks a read path); it adds no input-validation, auth, or crypto.

### Applicable ASVS Categories
| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | — (no auth surface) |
| V3 Session Management | no | — |
| V4 Access Control | no | — |
| V5 Input Validation | no | the config key is koanf-typed bool; no untrusted input |
| V6 Cryptography | no | — |

### Known Threat Patterns for this stack
| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Gate-bypass leak (a read path that skips the gate) | Tampering / Information Disclosure (vs. the ablation contract) | D-06 call-site lint (static) + D-05 counter==0 assertion (runtime) — belt-and-braces. |
| Stale singleton wiring (real lookup persists after gate) | Tampering | Idempotent null-object wiring (Pitfall 5; explicit `SetSemanticLookup(nil)`). |

## Sources

### Primary (HIGH confidence — in-repo source, file:line verified this session)
- `internal/daemon/daemon.go:280-302` (effDisableLSP template), `:506-540` (bundle construction), `:612-661` (symbols/health wiring), `:702-782` (repomap wiring) — the D-02 resolution + threading points.
- `internal/daemon/semantic_wiring.go:200-269` (newSemanticBundle + SemanticSkill accessor block), `:1357-1399` (daemonCfgGate, integLookupAccessor), `:865, 1076` (RankFiles, ExpandFrom).
- `internal/semantic/integ/source_select.go:32-74` (ConfigGate, ChooseSource priority ladder), `noop.go:25` (NoopLookup), `source.go:19-21` (SourceTreeSitter).
- `internal/semantic/config.go:27-31` (Config.Enabled — where BenchDisabled goes).
- `internal/config/config.go:28-42` (DisableLSPSubsystem field template); `internal/profile/profile.go:46-58` (profile field template); `internal/cli/root.go:75-76, 151, 176-181` (CLI override template).
- `internal/obs/metrics.go:334-504` (helix_semantic_* family — NO read counter), `:701-763` (Inc helpers).
- `internal/lint/ablationleakage/analyzer.go` + `analyzer_test.go`; `internal/lint/noduckdb/analyzer.go` + `analyzer_test.go`; `Makefile:30-61` (vet chain).
- `internal/skill/semantic/skill.go:22-180` (SemanticSkill setters), `accessors.go:401-412` (ImpactLookupAccessor / ExpandFrom).
- `internal/kernel/symbols/tools.go:83-109` (RegisterTools lookupFn/cfgGate), `blast_radius_strangler.go` (analyze_blast_radius consumer); `internal/kernel/health/tools.go:293-351` (health ChooseSource consumer).
- `bench/runtime/cell.go:53-59` (guarantee_pending_phase_81), `:662-690` (writeCellConfig); `bench/runtime/subprocess/daemon.go` (StartDaemon); `bench/runners/mode_resolver.go` (mode→profile); `bench/runners/your_agent_no_semantic/MODE.md` + `no_lsp/MODE.md`.
- `internal/eval/trace/tap.go:23-56` + `schema.go:104-149` (MergedTrace from daemon-log lines, NOT spans).
- `internal/profile/profiles/bench-no-semantic.yaml`; `internal/profile/bench_profiles_test.go:142`.
- `go.mod` (golang.org/x/tools v0.43.0).

### Secondary (MEDIUM confidence)
- `.planning/REQUIREMENTS.md:50` (ABLATE-06), `:49` (ABLATE-05 no_lsp precedent), `:52` (ABLATE-08 analyzer).
- Phase 76 artifacts under `.planning/phases/76-.../` (D-08 deferred precision context — read titles, not full bodies).

### Tertiary (LOW confidence)
- `buildssa` upgrade path for D-06 SSA precision — `[CITED: pkg.go.dev/golang.org/x/tools/go/analysis/passes/buildssa]`, not exercised in-repo (no SSA analyzer exists yet).

## Metadata

**Confidence breakdown:**
- Config/flag threading: HIGH — exact Phase 76 template exists in-repo; mechanical copy.
- Consumer enumeration + single-point wiring: HIGH — all 3 integLookupAccessor call sites + the SemanticSkill accessor block located and verified.
- Metric/counter surface: HIGH — exhaustively grepped; the read counter is confirmed absent (a deliverable, not wiring).
- Bench-cell assertion mechanism: MEDIUM — the trace pipeline is daemon-log-based (verified), but the exact counter-exposure path (Prometheus scrape vs. log line) needs a Wave-0 spike (Open Question 2).
- vet call-site extension: MEDIUM — analyzer harness + green→red pattern verified; narrow-AST vs SSA precision is a discretion call (Assumption A3 / Pitfall 6).

**Research date:** 2026-06-20
**Valid until:** 2026-07-20 (stable internal codebase; re-verify if daemon.go composition root or semantic_wiring.go accessor block is refactored before planning).

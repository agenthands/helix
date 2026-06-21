# Phase 81: `no_semantic` Kernel Flag + E2E Config-Gate Test - Context

**Gathered:** 2026-06-20
**Status:** Ready for planning

<domain>
## Phase Boundary

The last ablation arm that cannot be expressed by profile-YAML tool-filtering alone. This phase lands the
**kernel-level `disable_semantic_subsystem` config gate** that un-wires v1.10 Phase 65's `SetSemanticLookup`
strangler-fig integration, so that on the `no_semantic` arm:
- `get_repo_map` / `get_context` fall back to the v1.9 tree-sitter path (`source == "tree_sitter"`);
- the SemanticLookup consumers (`find_related_symbols`, `explain_symbol_deep`, `validate_graph_edge`,
  `analyze_blast_radius`, plus `RankFiles` / `ExpandFrom`) all see `integ.NoopLookup{}` + a disabled `ConfigGate`;
- **zero DuckDB semantic-store reads** occur during a `no_semantic` run, verified by a runtime assertion that
  logs and fails the bench cell on violation;
- a vet boundary guard flags any code path that bypasses the gate;
- `bench/runners/your_agent_no_semantic/MODE.md` documents the gate + the strangler-fig consumer enumeration.

**Requirement locked here:** ABLATE-06 (see `.planning/REQUIREMENTS.md` line 50). This phase delivers the kernel
gate + its E2E config-gate test — it is the Phase 76 deferral (D-11/D-12) being paid down. Phase 76 already
shipped the `bench-no-semantic.yaml` *profile* (tool-filter arm only); this phase adds the kernel back-channel
guard. WHAT/WHY are locked upstream in REQUIREMENTS.md and v1.12-ROADMAP.md; the decisions below are the HOW.
</domain>

<decisions>
## Implementation Decisions

### Config-gate identity (ABLATE-06; roadmap criterion #1)
- **D-01:** **Distinct `semantic_index.bench_disabled` gate — NOT a reuse of `cfg.SemanticIndex.Enabled=false`.**
  Introduce a separate, opt-in disable flag, independent of the production `SemanticIndex.Enabled` feature flag.
  Rationale: reusing `Enabled=false` would make criterion #2's zero-read assertion **vacuous** (the daemon already
  skips bundle creation when `Enabled=false`, so "zero reads" would prove only that nothing reads a store that was
  never built). A distinct gate also (a) decouples "ablation arm" from "production feature off," and (b) gives the
  vet analyzer a distinct, greppable symbol to anchor on. The roadmap explicitly names `semantic_index.bench_disabled`.
- **D-02:** **Resolve the gate once at the daemon composition root**, mirroring Phase 76's `effDisableLSP` pattern
  in `internal/daemon/daemon.go`. Compute an effective "semantic disabled" boolean and thread it into the
  `symbolsLookupFn` closure, the `daemonCfgGate`, the repomap-skill `SetSemanticLookup`/`SetConfigGate` wiring
  (`daemon.go:764`), and the health wiring — so a single resolution point governs all consumers, not scattered
  per-callsite flag checks.
- **D-03:** **Precedence: CLI override > profile YAML field > default-off**, reusing Phase 76 D-01/D-02. The flag is a
  first-class field on the existing `internal/profile/profiles/bench-no-semantic.yaml` (which Phase 76 created as a
  tool-filter-only arm) AND accepts a daemon CLI override for ad-hoc runs. Default state (no flag) = subsystem ENABLED.

### Index build vs. read seam (ABLATE-06; roadmap criterion #2 test-meaningfulness)
- **D-04:** **Build-but-block reads.** Under `bench_disabled`, the daemon STILL creates/maintains the semantic bundle
  (DuckDB store healthy, index built) — but the gate forces every SemanticLookup read consumer to `integ.NoopLookup{}`
  and a disabled `ConfigGate`. This is what makes criterion #2 a *real* test: the store exists and *could* be queried,
  yet the assertion proves zero reads happen — i.e. it proves the gate blocks, not that there was nothing to read.
  Index-build cost is out-of-band (daemon-side, not counted against agent tokens), so bench fairness is unaffected.

### Zero-read enforcement (ABLATE-06; roadmap criterion #2)
- **D-05:** **Trace-tap counter assertion as the verification mechanism.** The bench cell asserts the
  `helix_semantic_*` counter family == 0 after the `no_semantic` run, via the existing trace-tap — the exact
  mechanism the `no_lsp` arm uses to assert zero `lsp.*` spans (Phase 76 76-04). On violation, it logs and FAILS the
  bench cell. The build-but-block gate (D-04) is the **structural guarantee**; the counter assertion is the
  **independent verification**. No separate "poisoned store handle" runtime tripwire is added in this phase
  (rejected: extra moving parts that could mask bugs the clean counter assertion would otherwise surface).

### vet boundary guard (ABLATE-06; roadmap criterion #3)
- **D-06:** **Extend `vet-ablation-leakage` (Phase 76 `internal/lint/`) with a call-site gate check** — not just the
  import-boundary check Phase 76 shipped. Criterion #3 requires flagging "any code path that conditionally bypasses
  the `bench_disabled` gate," which an import-boundary analyzer alone cannot catch (an in-package direct lookup call
  that skips `ChooseSource` would compile clean). The extension asserts that every semantic-store read site routes
  through `integ.ChooseSource` / the wired `ConfigGate` rather than calling a lookup directly. This is precisely the
  "runtime-path precision" Phase 76 D-08 deferred *"until a real case appears"* — **Phase 81 is that case.** Ship with
  a deliberate green→red `testdata/` violation so a regression makes `make vet` fail with a clear diagnostic.

### Claude's Discretion
- Exact config key name/path for the gate (`semantic_index.bench_disabled` is the working name from the roadmap; the
  precise koanf key, struct field, and CLI override flag spelling are planner discretion within D-01/D-03).
- Internal kernel/daemon config-struct threading shape for the effective-disabled boolean (D-02).
- The exact `helix_semantic_*` counter/metric names the assertion checks, and whether they already exist from Phase 65
  or need to be added (D-05) — cross-check the existing semantic-store metric surface during research.
- The precise S-expression / SSA shape of the call-site analyzer check and its allowlist (D-06).
- `MODE.md` wording and structure (roadmap criterion #4) — must enumerate the gate key + the strangler-fig consumer
  list so a future contributor cannot accidentally rip out the bypass.
- The shape of the E2E `no_semantic` smoke task (reuse the Phase 77 seed toolbench-go task vs. a dedicated fixture).

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Milestone scope & requirements (locked WHAT/WHY — do not re-derive)
- `.planning/REQUIREMENTS.md` line 50 — **ABLATE-06** acceptance: kernel-level `disable_semantic_subsystem` flag
  prevents any back-channel semantic-store read (incl. Phase 65 SemanticLookup, RankFiles, ExpandFrom); E2E
  `no_semantic` task makes zero DuckDB queries; runtime assertion logs and fails.
- `.planning/milestones/v1.12-ROADMAP.md` §"Phase 81" — Goal, Depends-on, and the **4 Success Criteria** this phase
  is graded against (the enumerated consumer list lives in criterion #1).
- `.planning/ROADMAP.md` §"Phase 81" — abbreviated active-roadmap entry.

### The seam being disabled (Phase 65 strangler-fig — the thing un-wired)
- `internal/daemon/daemon.go:602-635` — composition-root wiring of `symbolsLookupFn`, `daemonCfgGate{enabled:
  cfg.SemanticIndex.Enabled}`, `symbols.RegisterTools(...)`, and the health lookup/cfgGate. **The D-02 resolution
  point.** Note nil `sBndl` is already normalized to `integ.NoopLookup{}`.
- `internal/daemon/daemon.go:764` — repomap-skill `SetSemanticLookup` + `SetConfigGate` post-init wiring (the
  `get_repo_map`/`get_context` consumer; must see the gate).
- `internal/daemon/semantic_wiring.go` — `daemonCfgGate` (line ~1357, `SemanticIndexEnabled()`), the
  `integSemanticLookup` production adapter, and the bundle (`sBndl`) lifecycle. The single biggest file to read.
- `internal/skill/repomap/skill.go:183-230` — `SetSemanticLookup` / `SetConfigGate` / `lookup()` /`configGate()`
  nil-normalization to `NoopLookup{}`, and the `ChooseSource` priority ladder (cfgGate consulted FIRST → tree_sitter).
- `internal/kernel/symbols/tools.go:87-97,380,633-664` — `lookupFn func() integ.SemanticLookup` + `cfgGate
  integ.ConfigGate` threading into `registerFindReferences` and the analyze-blast-radius / reachability consumers.
- `internal/kernel/symbols/blast_radius_strangler.go` — `analyze_blast_radius` SemanticLookup / ValidateCriticalEdges
  path (named consumer in criterion #1).
- `internal/kernel/health/tools.go:144-317` — health `semLookup`/`cfgGate` (the `get_health` semantic_index block;
  must reflect the disabled gate honestly, not crash).
- `internal/skill/semantic/accessors.go:402-407` — the `ExpandFrom` (graph) accessor that wraps
  `*integSemanticLookup` (named back-channel consumer in ABLATE-06: RankFiles / ExpandFrom).

### Reused-pattern source (Phase 76 — the analog ablation phase)
- `.planning/phases/76-ablation-profiles-kernel-subsystem-disable-flags/76-CONTEXT.md` — D-01/D-02 (hybrid
  coupling + precedence), D-09 (null-object injection at daemon init), D-07/D-08 (import-boundary analyzer +
  deferred runtime precision — the deferral this phase pays down). **Read in full.**
- `internal/lint/noduckdb/` (`analyzer.go`, `analyzer_test.go`, `testdata/`) and the existing
  `internal/lint/ablationleakage/` (Phase 76) — copy-shape source for the D-06 call-site extension; `make vet`
  `-vettool=` wiring at root `Makefile` lines ~33-38.
- `internal/profile/profiles/bench-no-semantic.yaml` — the Phase 76 tool-filter arm that gains the `bench_disabled`
  first-class field (D-03).
- Phase 76 76-04 `no_lsp` zero-span trace-tap + null-object daemon wiring — the direct precedent for D-04/D-05.

### Bench runtime (where the E2E test + MODE.md live)
- `.planning/phases/77-bench-runtime-first-e2e-smoke/77-CONTEXT.md` + the bench cell orchestrator / CCTapResult /
  trace-tap (Phase 77 77-02/77-03) — where the `helix_semantic_*` counter assertion (D-05) is wired into the cell.
- `bench/runners/your_agent_no_semantic/MODE.md` (criterion #4 — to be written/extended this phase).

### Milestone research (architectural verdicts)
- `.planning/research/SUMMARY.md`, `ARCHITECTURE.md`, `PITFALLS.md` — semantic-store / strangler-fig failure modes.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `integ.NoopLookup{}` and `integ.ConfigGate` (with `SemanticIndexEnabled()`) **already exist** — the seam was built
  Phase 65 to be switched off. This phase wires the new gate to force them, it does not invent the null object.
- `integ.ChooseSource(cfgGate, lookup, ...)` — the priority ladder that already renders `source="tree_sitter"` when
  the gate reports disabled. The gate change flows through it for free at every consumer that already uses it.
- `daemonCfgGate{enabled: ...}` (`semantic_wiring.go`) — the production ConfigGate; the gate resolution extends its
  `enabled` input to `(Enabled && !bench_disabled)` or threads a separate effective-disabled bool (D-02 discretion).
- Phase 76's `effDisableLSP` composition-root pattern in `daemon.go` — the structural template for D-02.

### Established Patterns
- **Null-object injection at daemon init** (Phase 76 D-09) — structurally guarantees the criterion (no live subsystem
  to emit reads/spans), rather than scattered per-callsite flag checks. Reused here for semantic.
- **Import-boundary lint + `make vet` `-vettool=` chaining** (`noduckdb`, `nokernel2semantic`, `ablationleakage`) —
  the house structural-enforcement pattern; D-06 extends `ablationleakage` with call-site awareness.
- **Trace-tap zero-event assertion** (`no_lsp` → zero `lsp.*` spans) — reused as `no_semantic` → zero `helix_semantic_*`.
- **`ChooseSource`-first routing** — cfgGate consulted before any semantic call; the invariant D-06's analyzer pins.

### Integration Points
- `internal/daemon/daemon.go` (composition root, ~602-635 and ~764) — gate resolution + threading into all consumers.
- `internal/daemon/semantic_wiring.go` — `daemonCfgGate` input; bundle stays built (D-04), reads gated.
- `internal/profile/profiles/bench-no-semantic.yaml` — gains the `bench_disabled` field (D-03).
- `internal/lint/ablationleakage/` (+ root `Makefile` `vet` target) — call-site gate-check extension + testdata (D-06).
- The bench cell harness (Phase 77) — `helix_semantic_*` counter assertion + `your_agent_no_semantic/MODE.md` (D-05, criterion #4).

</code_context>

<specifics>
## Specific Ideas

- Working config key from the roadmap: `semantic_index.bench_disabled: true` (exact spelling is D-01 discretion).
- The enumerated consumers that MUST see `NoopLookup` + disabled gate (roadmap criterion #1 + ABLATE-06):
  `get_repo_map`, `get_context`, `find_related_symbols`, `explain_symbol_deep`, `validate_graph_edge`,
  `analyze_blast_radius`, `RankFiles`, `ExpandFrom`. Treat this list as the test/canary checklist.
- Expected post-gate envelope: `get_repo_map`/`get_context` return `source == "tree_sitter"` (the v1.9 path).
- Counter family to assert at zero: `helix_semantic_*` (confirm exact metric names against the Phase 65 surface).

</specifics>

<deferred>
## Deferred Ideas

- **Poisoned/erroring DuckDB read handle** — considered for zero-read enforcement; rejected for this phase in favor of
  the clean trace-tap counter assertion (D-05). Revisit only if a real gate-bypass leak survives the D-06 analyzer +
  counter assertion.
- **Skip-build-entirely under ablation** — rejected (D-04) because it makes the zero-read test vacuous. Not revisited
  unless daemon startup cost on the bench arm becomes a measured problem.
- **General runtime call-graph leakage analysis beyond the semantic gate** — D-06 scopes the call-site check to the
  semantic read sites / `ChooseSource` invariant; broader interprocedural leak analysis stays out of scope.

None of the above expand Phase 81 scope — discussion stayed within the ABLATE-06 kernel-gate boundary.

### Planner notes (apply during plan-phase)
1. Gate is a **distinct `bench_disabled` flag**, resolved once at the `daemon.go` composition root (D-01/D-02),
   precedence CLI > profile YAML > default-off (D-03). Do NOT conflate with `cfg.SemanticIndex.Enabled`.
2. **Build-but-block** (D-04): bundle/store still built; reads forced to `NoopLookup` + disabled `cfgGate`. This is
   what makes the criterion-#2 test non-vacuous.
3. **Verification = trace-tap `helix_semantic_*` == 0** in the bench cell (D-05), mirroring `no_lsp`'s zero-span check.
4. **vet = call-site extension of `ablationleakage`** (D-06) with a green→red `testdata/` violation; this pays down
   Phase 76 D-08's deferred runtime precision.
5. Wire all enumerated consumers (see Specific Ideas) and add `your_agent_no_semantic/MODE.md` (criterion #4).
6. TDD mode is ON: the gate-routing logic, the counter assertion, and the analyzer are all `type: tdd`-eligible
   (defined I/O, testdata green→red); MODE.md / config-field glue is `type: execute`.

</deferred>

---

*Phase: 81-no-semantic-kernel-flag-e2e-config-gate-test*
*Context gathered: 2026-06-20*
</content>
</invoke>

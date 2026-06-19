# Phase 81: `no_semantic` Kernel Flag + E2E Config-Gate Test - Pattern Map

**Mapped:** 2026-06-20
**Files analyzed:** 12 (10 modified, 2+ created)
**Analogs found:** 12 / 12 (every target has an in-repo template; this is a Phase-76 copy phase)

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/semantic/config.go` | config | transform (koanf bind) | `Config.Enabled` field (same file, config.go:31) | exact |
| `internal/profile/profile.go` | config | transform (yaml bind) | `Profile.DisableLSPSubsystem` (same file, profile.go:46-51) | exact |
| `internal/cli/root.go` | config/CLI | request-response | `--disable-lsp-subsystem` flag (same file, root.go:75-76,151,176-181) | exact |
| `internal/daemon/daemon.go` | service (composition root) | event-driven (wiring) | `effDisableLSP` resolution + gating (same file, daemon.go:287-302,643-655,706-723) | exact |
| `internal/daemon/semantic_wiring.go` | service (bundle wiring) | event-driven | `newSemanticBundle` accessor block (same file, semantic_wiring.go:251-269) | exact |
| `internal/obs/metrics.go` | observability/util | transform (counter) | `SemanticStoreOpen` counter + `SemanticStoreOpenInc` (same file, metrics.go:340-347,713-722) | exact |
| `internal/semantic/store/duckdb.go` (read site) | model/storage | CRUD (read) | the DuckDB `.Query`/`QueryContext` read helper (duckdb.go) | role-match |
| `internal/lint/ablationleakage/analyzer.go` | utility (analyzer) | transform (AST) | `noduckdb/analyzer.go` + existing `ablationleakage/analyzer.go` (same file) | exact |
| `internal/lint/ablationleakage/testdata/src/{badgate,goodgate}/` | test (fixtures) | — | existing `ablationleakage/testdata/.../badrunner,goodrunner,siblingrunner` | exact |
| `internal/profile/profiles/bench-no-semantic.yaml` | config | — | `bench-no-lsp.yaml` / existing `bench-no-semantic.yaml` | exact |
| `bench/runtime/cell.go` + `bench/runtime/no_semantic_zero_reads_test.go` | test/harness | event-driven (assertion) | `ablationStatusFor` (cell.go:52-63); no_lsp structural precedent (no liftable code — build new) | role-match |
| `bench/runners/your_agent_no_semantic/MODE.md` | doc | — | `bench/runners/no_lsp/MODE.md` + existing `your_agent_no_semantic/MODE.md` | exact |
| `internal/profile/bench_profiles_test.go:142` | test (assertion flip) | — | the assertion itself (false→true) + sibling at line 159 | exact |

## Pattern Assignments

### `internal/semantic/config.go` (config, koanf bind)

**Analog:** `Config.Enabled` field, config.go:27-31. Add `BenchDisabled` as a sibling bool. **Critical (D-01):** distinct from `Enabled` — do NOT reuse `Enabled=false`.

**Field to mirror** (config.go:27-31):
```go
type Config struct {
	// Enabled gates the entire semantic-graph subsystem. When false, the
	// daemon's bootstrap step 6b skips Open and downstream consumers MUST
	// guard `daemon.SemanticStore() == nil`.
	Enabled bool `koanf:"enabled"`
	// Phase 81 ABLATE-06 ADD — distinct ablation gate (NOT a reuse of Enabled).
	// When true the store/bundle is STILL built (D-04); reads forced to Noop.
	BenchDisabled bool `koanf:"bench_disabled"`
```
Note the file header comment (config.go:18-26): any koanf key add requires lockstep handling — coverage tests reference SPEC §25 keys verbatim, so check `config_test.go` / labels tests if a key set is asserted.

---

### `internal/profile/profile.go` (config, yaml bind)

**Analog:** `DisableLSPSubsystem`, profile.go:46-51 (and `DisableStructuredEditSubsystem`, 53-58). Copy the doc-comment shape verbatim, swap LSP→Semantic.

**Field to mirror** (profile.go:46-51):
```go
// DisableLSPSubsystem requests the daemon disable the LSP subsystem for this
// profile (Phase 76 ABLATE-05). ... Zero-value false = subsystem ENABLED;
// the flag is an opt-in disable (D-02). Read at the daemon composition root,
// OR'd with any CLI override.
DisableLSPSubsystem bool `yaml:"disable_lsp_subsystem"`
```
Phase 81 add: `DisableSemanticSubsystem bool \`yaml:"disable_semantic_subsystem"\``.

---

### `internal/cli/root.go` (CLI override)

**Analog:** `--disable-lsp-subsystem`, three sites. Copy all three.

**Flag decl** (root.go:75-76):
```go
rootCmd.Flags().Bool("disable-lsp-subsystem", false, "Ablation override: force-disable the LSP subsystem (no LS workers, no enrichment)")
rootCmd.Flags().Bool("disable-structured-edit-subsystem", false, "...")
```

**Read in `runDaemon`** (root.go:151-152):
```go
disableLSP, _ := cmd.Flags().GetBool("disable-lsp-subsystem")
disableStructuredEdit, _ := cmd.Flags().GetBool("disable-structured-edit-subsystem")
```

**Override apply — only-when-set** (root.go:172-181):
```go
// only apply the disable overrides when the flag was explicitly set so a
// blank invocation cannot wipe a profile/config value (opt-in force-disables).
if disableLSP {
	overrides["disable_lsp_subsystem"] = true
}
```
Phase 81: add `--disable-semantic-subsystem` flag, read it, and on set do `overrides["semantic_index.bench_disabled"] = true`. **Note the koanf-path asymmetry:** LSP uses the top-level key `disable_lsp_subsystem`; the semantic gate's koanf key is nested `semantic_index.bench_disabled` (D-01 roadmap name). Match the field's actual koanf path in `internal/semantic/config.go` (the `SemanticIndex` sub-config).

---

### `internal/daemon/daemon.go` (composition root — the D-02 resolution + threading point)

**Analog:** `effDisableLSP` end-to-end. **This is the highest-fidelity template in the phase.**

**Resolve once** (daemon.go:287-294):
```go
effDisableLSP := cfg.DisableLSPSubsystem || activeProfile.DisableLSPSubsystem
effDisableSE := cfg.DisableStructuredEditSubsystem || activeProfile.DisableStructuredEditSubsystem
if effDisableLSP || effDisableSE {
	logger.Info("subsystem ablation flags resolved",
		"disable_lsp_subsystem", effDisableLSP,
		"disable_structured_edit_subsystem", effDisableSE,
	)
}
```
Phase 81 add adjacent: `effSemanticDisabled := cfg.SemanticIndex.BenchDisabled || activeProfile.DisableSemanticSubsystem` and add it to the log block.

**Anti-pattern guard (D-04):** do NOT add `effSemanticDisabled` to the `if cfg.SemanticIndex.Enabled` bundle-build guard (daemon.go:317). The store MUST still be built — build-but-block.

**Gate point 1 — symbolsLookupFn + cfgGate** (daemon.go:612-619). Mirror Phase 76's `leaseFn` if/else neutralization style (daemon.go:643-655):
```go
symbolsLookupFn := func() integ.SemanticLookup {
	if sBndl != nil {
		return sBndl.integLookupAccessor()
	}
	return integ.NoopLookup{}
}
symbolsCfgGate := &daemonCfgGate{enabled: cfg.SemanticIndex.Enabled}
symbols.RegisterTools(mcpServer, k, wsKeyFn, symbolsLookupFn, symbolsCfgGate)
```
Phase 81: add `if effSemanticDisabled { return integ.NoopLookup{} }` as the FIRST line of the closure, and set `enabled: cfg.SemanticIndex.Enabled && !effSemanticDisabled`. **Pitfall 4 (verified, source_select.go ladder):** cfgGate MUST be disabled so `ChooseSource` hits its FIRST priority → `SourceTreeSitter`; disabling only the lookup yields `SourceFallback`+`index_disabled`, which fails criterion #1.

**Gate point 2 — health** (daemon.go:627-629,661). `healthLookup := symbolsLookupFn()` already inherits the gate; mirror `&& !effSemanticDisabled` onto `healthCfgGate` (629).

**Gate point 3 — repomap SetSemanticLookup/SetConfigGate** (daemon.go:777-782):
```go
if rs := repomapSkill.GetRepoMapSkill(); rs != nil {
	rs.SetConfigGate(&daemonCfgGate{enabled: cfg.SemanticIndex.Enabled})
	if sBndl != nil {
		rs.SetSemanticLookup(sBndl.integLookupAccessor())
	}
}
```
Phase 81: gate `enabled` with `&& !effSemanticDisabled`; guard the SetSemanticLookup with `&& !effSemanticDisabled`; **Pitfall 5 (idempotent null-object on the process-global singleton):** under the gate explicitly call `rs.SetSemanticLookup(nil)` rather than just skipping — mirror the exact `SetEnrichFn(nil)` / `SetFallbackDeps(nil)` pattern Phase 76 uses at daemon.go:706-723:
```go
// Phase 76 idempotent-null template (daemon.go:706-711):
if rs := repomapSkill.GetRepoMapSkill(); rs != nil {
	if effDisableLSP {
		rs.SetEnrichFn(nil) // explicit null-object: clear any previously-set fn
	} else { /* wire real */ }
}
```

---

### `internal/daemon/semantic_wiring.go` (SemanticSkill accessor block — Pattern 3 / Pitfall 3)

**Analog:** `newSemanticBundle` accessor block, semantic_wiring.go:251-269. Thread a new `effSemanticDisabled` param into `newSemanticBundle` and gate the whole 14-setter block (the non-`ChooseSource` consumers: `find_related_symbols`, `explain_symbol_deep`, `validate_graph_edge`, cluster/impact tools, and the `SetImpactLookup`/ExpandFrom path).

**Block to gate** (semantic_wiring.go:251-268):
```go
b.skill = semantic.GetSemanticSkill()
if b.skill != nil {                       // Phase 81: add `&& !effSemanticDisabled`
	b.skill.SetStore(storeAcc)
	// ... 12 more setters ...
	b.skill.SetImpactLookup(b.integLookupAccessor())   // the ExpandFrom back-channel
	b.skill.SetSymbolEdges(b.symbolEdgesAccessor())
	b.skill.SetClusterMembership(b.clusterMembershipAccessor())
}
```
When gated, leave accessors nil — handlers already nil-guard (skill.go: "nil means the seam is unwired; handler MUST guard"). **A5 (verified):** the three `integLookupAccessor()` call sites (daemon.go:614, daemon.go:780, semantic_wiring.go:266) are the complete set; gating all three closes every RankFiles/ExpandFrom hand-out path.

---

### `internal/obs/metrics.go` + `internal/semantic/store/duckdb.go` (NET-NEW read counter)

**Analog:** `SemanticStoreOpen` counter + `SemanticStoreOpenInc` helper. **Finding (verified, A1): NO read counter exists** — the `helix_semantic_*` family is writes/maintenance only. This is a real deliverable.

**Counter registration to mirror** (metrics.go:340-347):
```go
SemanticStoreOpen: prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "helix_semantic_store_open_total",
		Help: "Semantic store open attempts by outcome (opened/quarantined/created). Phase 57.",
	},
	[]string{"workspace_label", "outcome"},
),
```
Phase 81 add `helix_semantic_store_reads_total` in the same block.

**Helper to mirror** (metrics.go:713-722):
```go
func (m *Metrics) SemanticStoreOpenInc(workspaceLabel, outcome string) {
	switch outcome {
	case "opened", "quarantined", "created":
	default:
		return
	}
	m.SemanticStoreOpen.WithLabelValues(workspaceLabel, outcome).Inc()
}
```
Phase 81: add `SemanticStoreReadsInc()`. **Emit at a single chokepoint** — the DuckDB `.Query`/`QueryContext` read helper in `internal/semantic/store/duckdb.go` (Open Question 1: prefer wrapping the lowest read-query helper once so EVERY read increments, vs. per-method). Wave-0 task locates the exact helper via `grep db.Query|QueryContext internal/semantic/store/duckdb.go`. **Watch the labels test:** new metric names may need allowlist entries — check `internal/obs/metrics_labels_test.go`.

---

### `internal/lint/ablationleakage/analyzer.go` (D-06 call-site extension)

**Analog:** existing `ablationleakage/analyzer.go` (import-boundary) + `noduckdb/analyzer.go`. **Recommendation (Pitfall 6 / A3): start narrow-AST**, matching the house style — flag direct calls to `SemanticLookup` interface methods (`.ExpandFrom(`, `.RankFiles(`, `.ValidateCriticalEdges(`) outside an allowlist (the integ adapter + gated wiring). SSA via `buildssa` is the documented future upgrade, not this phase.

**Existing analyzer skeleton to extend** (analyzer.go:48-69):
```go
var Analyzer = &analysis.Analyzer{
	Name: "ablationleakage",
	Doc:  "...",
	Run: func(pass *analysis.Pass) (interface{}, error) {
		if !strings.HasPrefix(pass.Pkg.Path(), checkedPkgPrefix) {
			return nil, nil
		}
		for _, file := range pass.Files {
			for _, imp := range file.Imports {
				path := strings.Trim(imp.Path.Value, `"`)
				for _, forbidden := range forbiddenImportPrefixes {
					if path == forbidden || strings.HasPrefix(path, forbidden+"/") {
						pass.Reportf(imp.Pos(), "...")
					}
				}
			}
		}
		return nil, nil
	},
}
```
Note the slash-boundary discipline (analyzer.go:18-22) — preserve it for any new prefix/allowlist match. The Makefile rebuilds the vettool from `internal/lint/ablationleakage/*.go` (Makefile:60-61), so no stale-artifact risk.

---

### `internal/lint/ablationleakage/testdata/src/{badgate,goodgate}/` (green→red fixtures)

**Analog:** existing testdata dirs `badrunner` (red, `// want` comment), `goodrunner` (green), `siblingrunner` (lookalike-silent), driven by `analysistest.Run`.

**Test driver to mirror** (analyzer_test.go:14-24):
```go
func TestAnalyzer_RejectsBenchRunnerImportingDisabledSubsystem(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), ablationleakage.Analyzer,
		"github.com/agenthands/helix/bench/runners/badrunner")
}
func TestAnalyzer_AllowsBenchRunnerWithoutForbiddenImport(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), ablationleakage.Analyzer,
		"github.com/agenthands/helix/bench/runners/goodrunner")
}
```
Phase 81: add `badgate` (direct `.ExpandFrom(` call with `// want "must route through integ.ChooseSource"`) and `goodgate` (same call gated behind `ChooseSource`) fixtures + their two test funcs.

---

### `internal/profile/profiles/bench-no-semantic.yaml` (add the field)

**Analog:** existing `bench-no-semantic.yaml` (header at lines 1-8 currently says "this arm sets NO kernel flag" — rewrite that) + `bench-no-lsp.yaml`/`bench-no-structured-edit.yaml` which carry their disable field. Add `disable_semantic_subsystem: true` and update the header comment to reflect that Phase 81 now lands the kernel flag (drop the D-11/D-12 "tool-filter-only" caveat).

---

### `bench/runtime/cell.go` + `bench/runtime/no_semantic_zero_reads_test.go` (E2E assertion)

**Analog (intent only — A2/Pitfall 2):** no liftable no_lsp zero-event assertion exists; the no_lsp guarantee is purely structural. Build the counter-scrape + assert path from scratch. The directly-affected existing code is the deferral marker:

**`ablationStatusFor` to remove** (cell.go:52-63):
```go
func ablationStatusFor(mode string) string {
	if mode == "your_agent_no_semantic" {
		return "guarantee_pending_phase_81"
	}
	return ""
}
```
Phase 81 removes the `guarantee_pending_phase_81` branch (now satisfied). Then add the cell assertion: run the `no_semantic` task → obtain `helix_semantic_store_reads_total` (Open Question 2 — likely a `msg="semantic store reads total" count=N` daemon-log line parsed by the existing `internal/eval/trace/tap.go` `msg==` tap, since the bench daemon runs over a Unix socket with HTTP/Prometheus likely unreachable) → assert == 0, else LOG + FAIL the cell. Confirm the daemon metrics-exposure path in the sandbox during Wave 0.

---

### `internal/profile/bench_profiles_test.go:142` (assertion FLIP)

**Analog:** the assertion itself + the sibling positive assertion at line 159 (`bench-no-structured-edit must declare disable_structured_edit_subsystem: true`).

**Current (to flip)** (bench_profiles_test.go:141-143):
```go
// Tool-filter-only this phase: NEITHER kernel flag set (D-11/D-12).
assert.False(t, p.DisableLSPSubsystem, "bench-no-semantic must NOT set a kernel flag (D-11/D-12)")
assert.False(t, p.DisableStructuredEditSubsystem, "bench-no-semantic must NOT set a kernel flag (D-11/D-12)")
```
Phase 81: add `assert.True(t, p.DisableSemanticSubsystem, "bench-no-semantic must declare disable_semantic_subsystem: true")` (mirroring line 159). Keep the LSP/structured-edit `False` asserts (this arm still only ablates semantic). Also update the comment at line 134 ("kernel guard is Phase 81") to reflect it's now landed.

---

### `bench/runners/your_agent_no_semantic/MODE.md` (REWRITE — criterion #4)

**Analog:** `bench/runners/no_lsp/MODE.md` (the clean, post-deferral shape) + existing `your_agent_no_semantic/MODE.md` (whose "Deferred guarantee" section at lines 16-36 must be DELETED).

**Target structure (mirror no_lsp/MODE.md):** frontmatter (`mode`/`profile`), one-paragraph arm description, then a paragraph documenting the kernel gate. Per criterion #4 the rewrite MUST enumerate (a) the gate key `semantic_index.bench_disabled` and (b) the strangler-fig consumer list so a future contributor cannot accidentally rip out the bypass: `get_repo_map`, `get_context`, `find_related_symbols`, `explain_symbol_deep`, `validate_graph_edge`, `analyze_blast_radius`, `RankFiles`, `ExpandFrom`. Drop the `ablation_status: guarantee_pending_phase_81` section entirely.

**no_lsp/MODE.md shape to copy** (full file, 19 lines):
```markdown
---
mode: no_lsp
profile: bench-no-lsp
---

# no_lsp

The LSP-disabled ablation arm. Resolves to the `bench-no-lsp` profile ...
the kernel-level LSP disable flag is consumed by the daemon, so LSP-dependent
tools fail closed rather than silently degrading.
```

## Shared Patterns

### Composition-root effective-disable boolean (D-02)
**Source:** `internal/daemon/daemon.go:287-294` (`effDisableLSP`)
**Apply to:** the single `effSemanticDisabled` resolution; all gating threads from this one boolean — NO per-callsite flag checks in handlers (anti-pattern, RESEARCH §Anti-Patterns).

### Idempotent null-object wiring on process-global singletons (Pitfall 5)
**Source:** `internal/daemon/daemon.go:706-711` (`SetEnrichFn(nil)`), `:730-734` (`SetFallbackDeps(nil)`)
**Apply to:** `rs.SetSemanticLookup(nil)` under the gate — clear, don't skip.

### ChooseSource-first routing → tree_sitter (Pitfall 4)
**Source:** `internal/semantic/integ/source_select.go` ladder; consumed via `daemonCfgGate{enabled: ...}`
**Apply to:** every cfgGate construction — disable the GATE (not just the lookup) so `source == "tree_sitter"`.

### Prometheus counter + Inc helper
**Source:** `internal/obs/metrics.go:340-347` (register) + `:713-722` (helper)
**Apply to:** the net-new `helix_semantic_store_reads_total`.

### go/analysis green→red testdata harness
**Source:** `internal/lint/ablationleakage/analyzer_test.go:14-24` + `noduckdb` testdata
**Apply to:** the `badgate`/`goodgate` call-site fixtures.

### Only-apply-when-set CLI override
**Source:** `internal/cli/root.go:172-181`
**Apply to:** `--disable-semantic-subsystem` → `overrides["semantic_index.bench_disabled"]`.

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| (none — every target has an in-repo template) | — | — | The bench-cell counter assertion (`bench/runtime/cell.go` + new test) is the only item with NO liftable code; it reuses the *pattern intent* of the no_lsp arm (structural) but the runtime zero-event assertion must be built fresh (Pitfall 2). Counter exposure path (log-line vs Prometheus scrape) is a Wave-0 spike (Open Question 2). |

## Metadata

**Analog search scope:** `internal/daemon`, `internal/cli`, `internal/profile`, `internal/semantic/{config,integ,store}`, `internal/obs`, `internal/lint/{ablationleakage,noduckdb}`, `bench/runtime`, `bench/runners`
**Files scanned:** ~16 (templates verified at file:line against RESEARCH claims)
**Pattern extraction date:** 2026-06-20
**Key risk:** the read counter + bench-cell assertion are net-new (verified absent); everything else is mechanical Phase-76 copy.

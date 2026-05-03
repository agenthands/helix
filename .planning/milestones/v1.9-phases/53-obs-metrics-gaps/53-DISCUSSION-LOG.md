# Phase 53: obs-metrics-gaps - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-30
**Phase:** 53-obs-metrics-gaps
**Areas discussed:** Cache hit metric shape, RepoMap extract histogram, Session lifecycle phases, Edit outcome enum + fuzzy strategy

---

## Cache hit metric shape (lspool + repomap)

### Q1: Counter shape

| Option | Description | Selected |
|--------|-------------|----------|
| Single counter w/ result label | Two metrics: `helix_lspool_lookups_total{result=hit\|miss}` and `helix_repomap_lookups_total{result=hit\|miss}`. Hit-ratio is a single PromQL expression. | ✓ |
| Two separate counters per family | `*_cache_hits_total` + `*_cache_misses_total`. Matches ROADMAP wording verbatim; doubles the family count. | |
| Hits + total counters | `*_cache_hits_total` + `*_cache_lookups_total` (lookups = hits + misses). Three counters per family. | |

**User's choice:** Single counter with `result` label.
**Notes:** Becomes D-01. Implies the ROADMAP success-criterion-1 wording (`*_cache_hits_total`) must be rewritten to the new metric names — captured as D-13.

### Q2: Labels and the lspool "hit" boundary

| Option | Description | Selected |
|--------|-------------|----------|
| lspool: language + result; repomap: language + result | lspool hit = warm-worker share at pool.go:114-122; miss = spawn at :130-141. repomap hit = mtime match at cache.go:76-84. Single new carve-out (`result`). | ✓ |
| Add `dirty` to lspool, `extractor` to repomap cache hit | More signal; 3 carve-outs total; mixes extractor dimension into cache-hit metric. | |
| Result only, no language | Lowest cardinality; loses per-language hit-rate visibility. | |

**User's choice:** language + result for both families.
**Notes:** Becomes D-02, D-03, D-04. `extractor` dimension stays on the histogram only (D-07).

---

## RepoMap extract histogram

### Q1: Instrumentation site

| Option | Description | Selected |
|--------|-------------|----------|
| Wrap extractFn at TagCache.GetOrExtract (cache.go:89) | Times only the actual extraction. Single instrumentation point covers all extractors. | ✓ |
| Per-extractor instrumentation at callsites | Extractor-specific latency without a label; multiple instrumentation points, easier coverage gaps. | |
| Wrap GetOrExtract end-to-end | Mixes hit and miss latency in one histogram; needs `result` label, doubles cardinality. | |

**User's choice:** Wrap extractFn inside GetOrExtract.
**Notes:** Becomes D-05. Forces the `RepoMapMetricsSink` interface (D-15) to keep `internal/repomap/` free of `internal/obs/` imports.

### Q2: Buckets and labels

| Option | Description | Selected |
|--------|-------------|----------|
| Custom 1ms→2.5s exponential, language + extractor | `{0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5}`. Tree-sitter and LSP fallback both visible. `extractor={treesitter,lsp,fallback}` carve-out. | ✓ |
| `prometheus.DefBuckets` + language only | DefBuckets collapses tree-sitter into the first bucket; no extractor breakdown. | |
| Custom buckets, no extractor label | Loses the explicit extractor signal; operators infer from latency tier. | |

**User's choice:** Custom buckets + `language + extractor` labels.
**Notes:** Becomes D-06, D-07.

---

## Session lifecycle phases

### Q1: Phase enum and source of truth

| Option | Description | Selected |
|--------|-------------|----------|
| {started, ended, error} from forwarder stream events | 3 values; emit at daemon.go:606 (started), :620 (ended), and on error returns. | ✓ |
| {started, activated, ended, error} including LazyInit hook | 4 values; adds 'activated' from LazyInitMiddleware sync.Once. Captures cold-start cost. | |
| {started, ended} minimal | 2 values; errors fold into ended; smallest carve-out. | |

**User's choice:** {started, ended, error} from forwarder stream events.
**Notes:** Becomes D-08. Workspace-activation cold-start visibility deferred (see Deferred Ideas).

### Q2: Transport label

| Option | Description | Selected |
|--------|-------------|----------|
| Add `transport={stdio, http}` carve-out | Distinguishes forwarder-driven sessions from direct HTTP sessions; one extra carve-out. | ✓ |
| Phase only, no transport | Smaller surface; cannot separate stdio churn from HTTP churn. | |

**User's choice:** Add transport label.
**Notes:** Becomes D-09. Phase 54 dashboards will compare the two.

---

## Edit outcome enum + fuzzy strategy

### Q1: Outcome enum vocabulary

| Option | Description | Selected |
|--------|-------------|----------|
| {success, no_match, ambiguous_match, validation_failed, ls_error, internal} | 6 values; mirrors `outcomeEnum` discipline; strategy detail in a separate label. | ✓ |
| Roll fuzzy strategy into outcome | 9 values combining strategy and result; conflates 'why succeeded' with 'why failed'. | |
| {success, refused, error} minimal | 3 values; loses the ability to distinguish bad-input from genuine-ambiguity from validation-broke-compilation. | |

**User's choice:** Six-value enum.
**Notes:** Becomes D-10.

### Q2: Strategy label vs. existing rename_strategy_total

| Option | Description | Selected |
|--------|-------------|----------|
| Separate `strategy` label on edit_outcome; keep rename_strategy_total | Strategy ∈ {exact, whitespace_normalized, indentation_flexible, ellipsis, none}. rename_strategy_total preserved per Phase 47 D-07. | ✓ |
| Fold rename into edit_outcome with extended strategy enum | Single counter; breaks Phase 47 D-07 contract; migration cost on existing callers. | |
| No strategy label, outcome only | Loses dashboard-level signal about whether the fuzzy cascade is earning its keep. | |

**User's choice:** Separate strategy label, rename counter unchanged.
**Notes:** Becomes D-11. Rename increments BOTH `helix_edit_outcome_total{tool_name=rename_symbol,strategy=none}` AND `helix_rename_strategy_total{strategy=...}`.

### Q3: Tool scope and tool_name dimension

| Option | Description | Selected |
|--------|-------------|----------|
| All 7 edit tools, reuse `tool_name` | replace_in_file, replace_symbol_body, fuzzy_edit, insert_after_symbol, insert_before_symbol, delete_lines, rename_symbol. tool_name already in AllowedLabels. Cardinality 7×6×5=210. | ✓ |
| Only the 4 fuzzy-capable tools | replace_in_file, replace_symbol_body, fuzzy_edit, rename_symbol. Loses unified visibility for non-fuzzy edit tools. | |
| All 7 tools without tool_name | Drop tool_name; rely on helix_tool_calls_total for per-tool breakdown. Loses 'which edit tool fails most' as a single query. | |

**User's choice:** All 7 tools, reuse tool_name.
**Notes:** Becomes D-12. Cardinality test ceiling 256 leaves headroom for new edit tools.

---

## Claude's Discretion

Wiring patterns (D-14 through D-17) are within Claude's discretion subject to the constraints captured in the decisions:
- lspool extends the existing `MetricsSink` interface.
- repomap follows the same interface pattern via a new `RepoMapMetricsSink` to keep `internal/repomap/` free of obs imports.
- Edit tools wire via a package-level setter in `internal/mcp` (mirroring Phase 47 D-07's `RecordRenameStrategy`).
- Session lifecycle is wired directly via `*obs.Provider` since the daemon and HTTP transport already import `internal/obs`.

The planner picks the exact function signatures and the post-init wiring code in `internal/daemon/daemon.go`.

## Deferred Ideas

- Workspace-activation cold-start visibility (time-from-session-start to first activated workspace). Approximated by traces today; defer to Phase 55 / OBS-04 trace audit.
- `dirty` dimension on lspool lookups. Today dirty-mode acquires count as misses. Add only if hit-rate dashboards show unexplained miss spikes during heavy edit sessions.
- Per-query repomap lookups counter (vs. per-file granularity in D-03). Belongs as its own family; defer until a dashboard requirement surfaces it.

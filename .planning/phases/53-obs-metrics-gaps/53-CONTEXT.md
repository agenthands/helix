# Phase 53: obs-metrics-gaps - Context

**Gathered:** 2026-04-30
**Status:** Ready for planning

<domain>
## Phase Boundary

This phase closes the four v1.2 metrics-coverage gaps by adding new Prometheus metric families to the existing `internal/obs/metrics.go` registry. Operators must be able to observe (via `/metrics`):

1. **lspool cache hit rate** — how often a clean session shares an existing warm worker vs. spawns a new one, broken down by language.
2. **RepoMap cache hit rate** — how often `TagCache.GetOrExtract` returns cached tags (mtime match) vs. invokes the extractor, broken down by language.
3. **RepoMap extraction latency** — histogram of how long the extractor itself takes (cache-miss path only), broken down by language and extractor (treesitter / lsp / fallback).
4. **Session lifecycle** — counter of session phase transitions (started / ended / error) by transport (stdio / http).
5. **Edit-tool outcome** — counter of edit-tool results (success / no_match / ambiguous_match / validation_failed / ls_error / internal) by tool name and fuzzy strategy.

All new vectors live on the same single `*prometheus.Registry` already owned by `*obs.Metrics`, follow the existing AllowedLabels + carve-out discipline, and are zero-cost in the Noop path. The CI cardinality lint (`TestMetricsLabelsAllowlist`) is extended with the new carve-outs and new bounded-cardinality assertions.

**In scope:**
- Adding 5 new metric vectors to `internal/obs/metrics.go` with `helix_*` naming (NOT `serena_*` — see D-09).
- Wiring instrumentation at the canonical observable boundaries:
  - `internal/kernel/lspool/pool.go` AcquireLease (share vs. spawn branch).
  - `internal/repomap/cache.go` `GetOrExtract` (hit vs. extractFn branch + extractFn timing).
  - `internal/daemon/daemon.go:606,620` (forwarder stream open/close + error returns) and the equivalent boundary in the Streamable HTTP transport.
  - All 7 edit tools in `internal/kernel/edit/tools.go` (success/failure classification at the tool-handler return).
- Extending `internal/obs/metrics_labels_test.go` with new carve-outs (`result`, `extractor`, `phase`, `transport`) and adding cardinality-bound tests per family.
- Updating `USAGE.md` Observability table with each new metric, its labels, and its semantics.
- Updating ROADMAP.md success-criteria-1 to use `helix_*` names (Phase 52 rename was not back-applied to forward-looking phase definitions).
- Preserving the noop-default invariant — vectors construct at startup, are unscraped unless `/metrics` is mounted.

**Out of scope (other phases):**
- Grafana dashboards consuming these metrics — Phase 54 (OBS-01, OBS-02).
- Runbooks for the operational failure modes — Phase 54.
- Trace coverage audit (every MCP tool / outbound LS call has a span) — Phase 55 (OBS-04).
- Refactoring `helix_rename_strategy_total` (Phase 47 D-07 contract is preserved unchanged — see D-08).
- Wiring metrics into the legacy Python reference under `legacy/`.

</domain>

<decisions>
## Implementation Decisions

### Cache hit metrics (lspool + repomap)

- **D-01: Single counter per family with a `result={hit,miss}` carve-out label.** Two new vectors total: `helix_lspool_lookups_total{language, result}` and `helix_repomap_lookups_total{language, result}`. Hit-ratio is a pure PromQL expression (`sum(rate({result="hit"}[5m])) / sum(rate(...[5m]))`) — operators do not need to subtract counters across families. Each lookup increments exactly one series, vs. two-counter shapes that double the family count without adding signal. The ROADMAP success-criterion wording (`*_cache_hits_total`) is interpreted as a category, not a literal metric name; the planner must update the ROADMAP wording to `helix_lspool_lookups_total` / `helix_repomap_lookups_total` as part of this phase.
- **D-02: lspool "hit" boundary is share-warm-worker vs. spawn.** A hit is recorded when `Pool.AcquireLease` returns a shared lease via `workerForKeyLocked` (`internal/kernel/lspool/pool.go:114-122`). A miss is recorded when the spawn path runs (`pool.go:130-141`), regardless of whether the spawn ultimately succeeds or fails — circuit-open / max-workers refusals still count as misses, because the cache (the warm-worker pool) did not satisfy the request. Dirty-mode acquires always count as misses (they bypass `workerForKeyLocked` by design); we do NOT add a `dirty` label dimension — operators can correlate via `helix_tool_calls_total` if needed.
- **D-03: repomap "hit" boundary is the mtime-match branch in `TagCache.GetOrExtract`.** Hit at `cache.go:76-84` (cached mtime equals current `info.ModTime().UnixNano()`). Miss at `cache.go:86-92` (no row, mtime mismatch, or load error). Granularity is per-file (one increment per `GetOrExtract` call); per-query rollups happen at the dashboard level via `sum by (language)`. The `language` label comes from the file's detected language, resolved via the existing langregistry path used by callers (planner picks the resolution call — `langregistry.DetectFromPath` is the candidate).
- **D-04: Labels = `{language, result}` for both families.** `language` is already in `AllowedLabels` (no new entry). `result` is added as a closed-enum carve-out for these two families in `metrics_labels_test.go::carveOuts`, with values `{hit, miss}` enforced at emission sites. No `dirty` dimension on lspool, no `extractor` dimension on the cache-hit metric (extractor lives on the histogram — D-06).

### RepoMap extraction histogram

- **D-05: Instrument by wrapping `extractFn` inside `TagCache.GetOrExtract` (cache.go:89).** Single instrumentation point — every extractor (treesitter, lsp-fallback, fallback) flows through it. Cache-hit overhead (SQL load) is excluded. Wiring uses an injected sink (a `RepoMapMetricsSink` interface declared in `internal/repomap/`, implemented by `*obs.Metrics`) to keep `internal/repomap/` free of `internal/obs/` imports — the same decoupling pattern lspool uses (`internal/kernel/lspool/metrics.go::MetricsSink`). Daemon wires the sink at startup; tests get `repomap.NoopMetricsSink{}`.
- **D-06: Custom buckets `{0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5}` (1ms→2.5s).** `prometheus.DefBuckets` (5ms→10s) collapses tree-sitter extracts (typical 1–10ms) into the first bucket and loses the fast-path latency signal. Custom buckets give p50 visibility for tree-sitter, p95 visibility for LSP fallback (50–500ms typical), and a tail bucket for large-file outliers. Metric name: `helix_repomap_extract_duration_seconds`.
- **D-07: Labels = `{language, extractor}`.** `language` already allowed. `extractor` is a closed-enum carve-out with values `{treesitter, lsp, fallback}` enforced at the emission site (the wrapping helper accepts an `Extractor` typed string and drops unknown values, mirroring `RenameStrategyInc`'s rejection-of-unknown pattern). Per-extractor p95 is the diagnostic that closes OBS-03's gap — operators must be able to answer "is the LSP fallback dragging us down for $LANG?" from a single PromQL query.

### Session lifecycle

- **D-08: Closed-enum `phase={started, ended, error}`.** Three values. `started` and `ended` are emitted from the forwarder stream lifecycle (`internal/daemon/daemon.go:606` and `:620` respectively, which already log `session_id`). `error` is emitted when a stream returns with a non-nil error (replacing or augmenting the existing log line at the same boundary). Workspace activation is per-tool-call (LazyInitMiddleware sync.Once) and is NOT a session phase; if cold-start visibility is wanted later, it lives in a separate metric (deferred). Metric name: `helix_session_lifecycle_total`.
- **D-09: Add `transport={stdio, http}` carve-out.** Two values. `stdio` for forwarder-driven sessions (the daemon.go forwarder stream handler is hardcoded to this); `http` for direct Streamable HTTP transport (the planner identifies the matching session-create / session-end boundary in the HTTP path and instruments there). Operators querying `helix_session_lifecycle_total{transport="http"}` see HTTP-specific churn; `transport="stdio"` shows long-lived MCP client sessions. This is a forward-looking dimension because Phase 54 dashboards will want to compare the two.

### Edit-tool outcome counter

- **D-10: Closed-enum `outcome={success, no_match, ambiguous_match, validation_failed, ls_error, internal}`.** Six values, mirroring the discipline of `outcomeEnum` in `internal/mcp/middleware.go:111-119`. `success` = edit applied (any strategy); `no_match` = fuzzy cascade exhausted (`fuzzy.StrategyFailed`); `ambiguous_match` = >1 fuzzy candidate refused with diff (the existing FUZZ refusal path); `validation_failed` = post-edit verifier flagged regression (`internal/kernel/edit/verify.go::VerifyResult`); `ls_error` = upstream LS failure (LS returns error or workspace edit applies fail); `internal` = catch-all for unclassified internal errors. Metric name: `helix_edit_outcome_total`.
- **D-11: Separate `strategy` label distinct from rename's existing dispatcher counter.** `helix_edit_outcome_total{tool_name, outcome, strategy}` where `strategy` ∈ `{exact, whitespace_normalized, indentation_flexible, ellipsis, none}` — the four `fuzzy.Strategy` values plus `none` for non-fuzzy tools (`insert_after_symbol`, `insert_before_symbol`, `delete_lines`) and for failure outcomes where no strategy ran. The existing `helix_rename_strategy_total{strategy=lsp-native|rust-client-side}` (Phase 47 D-07) is **kept unchanged** — it tracks an orthogonal dimension (LSP-native vs. client-side rename dispatcher) not the fuzzy match strategy. Both families coexist; rename increments BOTH `helix_edit_outcome_total{tool_name=rename_symbol,strategy=none}` (rename does not run the fuzzy cascade) AND `helix_rename_strategy_total{strategy=lsp-native|rust-client-side}`.
- **D-12: Tool scope = all 7 edit tools, reuse existing `tool_name` label.** Scope: `replace_in_file`, `replace_symbol_body`, `fuzzy_edit`, `insert_after_symbol`, `insert_before_symbol`, `delete_lines`, `rename_symbol`. `tool_name` is already in `AllowedLabels` — no new carve-out. Cardinality bound: 7 tools × 6 outcomes × 5 strategies = 210 max series (cardinality test ceiling: 256, leaving headroom for future tools without a contract update). For non-fuzzy tools the strategy label is hardcoded to `none` at emission, narrowing actual production cardinality below the worst-case bound.

### Naming + ROADMAP correction

- **D-13: All new metric names use the `helix_*` prefix.** ROADMAP.md success criterion 1 still refers to `serena_lspool_cache_hits_total` etc. — this is a stale string from before the Phase 52 rename. The planner must update the ROADMAP success criterion as part of phase 53 plans (it is in scope because shipping metrics with the `serena_*` prefix would re-introduce the brand inconsistency Phase 52 just removed). Final names: `helix_lspool_lookups_total`, `helix_repomap_lookups_total`, `helix_repomap_extract_duration_seconds`, `helix_session_lifecycle_total`, `helix_edit_outcome_total`.

### Wiring patterns (Claude's discretion within these constraints)

- **D-14: lspool follows existing `MetricsSink` interface pattern** — extend `internal/kernel/lspool/metrics.go::MetricsSink` with an `LSPoolLookup(language, result string)` method; `*obs.Metrics` implements it; `NoopSink` gets a no-op stub.
- **D-15: repomap follows the same pattern** — declare `RepoMapMetricsSink` in `internal/repomap/`, with `RepoMapLookup(language, result string)` and `RepoMapExtractObserve(language, extractor string, seconds float64)` methods. `internal/repomap/` does NOT import `internal/obs/`. Daemon wires the sink at the existing post-init wiring point.
- **D-16: Edit tools wire via the package-level setter pattern** (mirroring `mcp.RecordRenameStrategy` from Phase 47 D-07) — a single `internal/mcp` package-level recorder accepting `(ctx, toolName, outcome, strategy)`, set from `InstallMiddleware`, called by edit tool handlers at their return. Avoids `internal/kernel/edit/` taking a sink dependency.
- **D-17: Session lifecycle wires directly via `*obs.Provider`** (the daemon and HTTP transport already import `internal/obs`, so the sink-interface pattern adds no isolation value). Methods on `*obs.Metrics` named `SessionLifecycleInc(phase, transport string)`.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase scope and roadmap
- `.planning/ROADMAP.md` §"Phase 53: obs-metrics-gaps" (lines 183–193) — phase goal and 4 success criteria; success-criterion-1 metric names are stale (`serena_*`) and must be rewritten to `helix_*` in this phase.
- `.planning/REQUIREMENTS.md` §OBS-03 (line 36) — the requirement closing v1.2 metric gaps.
- `.planning/PROJECT.md` — project-level context for the Helix Go-native rewrite and the noop-default invariant.

### Existing observability contract (must not regress)
- `internal/obs/metrics.go` — single home for all `prometheus/client_golang` imports; the registry, AllowedLabels array, and existing vectors live here. The "Design rules" comment block at the top of this file is binding for all new metrics.
- `internal/obs/obs.go` — `*obs.Provider` semantics; Noop guarantees; the rule that `Metrics()` is never nil.
- `internal/obs/metrics_labels_test.go` — `TestMetricsLabelsAllowlist`, the `carveOuts` map, and the negative-proof drift test. New carve-outs are added here.

### Decoupling patterns to follow
- `internal/kernel/lspool/metrics.go` — existing `MetricsSink` interface + `NoopSink` + closed-enum constants pattern; `internal/kernel/lspool/` does not import `internal/obs/`. Extended in D-14.
- `internal/mcp/middleware.go:18-44` — Phase 47 `renameStrategySink` package-level setter pattern via `atomic.Pointer`. Mirrored in D-16 for the edit-outcome recorder.

### Instrumentation sites
- `internal/kernel/lspool/pool.go:108-147` — `AcquireLease`; the `workerForKeyLocked` branch (line 116) is the lspool-cache hit boundary; the spawn block starting at line 124 is the miss boundary.
- `internal/repomap/cache.go:60-103` — `GetOrExtract`; lines 76-84 are the cache-hit branch, lines 86-92 are the miss + extractFn branch (the `extractFn` call at line 89 is the timing target for D-05).
- `internal/daemon/daemon.go:606,620` — forwarder stream session-start / session-end log lines; canonical session-lifecycle boundary for D-08 (stdio path).
- `internal/kernel/edit/tools.go` — return sites of all 7 edit tool handlers; outcome classification happens here.
- `internal/kernel/edit/verify.go` — `VerifyResult` is the source of truth for the `validation_failed` outcome bucket.
- `internal/fuzzy/types.go:14-28` — `Strategy` enum; the four values plus `StrategyFailed` are the source of truth for the `strategy` label values in D-11.

### Documentation surfaces
- `USAGE.md` §"Prometheus Metrics" (lines 634–678) — the metrics table that must grow to include the 5 new families with their labels and semantics; the example PromQL block should add a hit-ratio query and a per-extractor latency query.

### Prior decisions carried forward
- Phase 11 D-04: `AllowedLabels = {tool_name, profile, mode, language, outcome}`.
- Phase 11 D-13: `helix_lspool_evictions_total{reason}` carve-out as a closed-enum (precedent for D-04, D-07, D-09).
- Phase 47 D-07: `helix_rename_strategy_total{strategy}` as a separate dispatcher counter; preserved unchanged in D-11.
- Phase 52 D-01..D-03: `helix_*` brand prefix; D-13 corrects ROADMAP wording.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `*obs.Metrics` registry pattern (`internal/obs/metrics.go:57-131`) — `newMetrics()` builds every vector once, registers them, returns the struct. Add new vectors here; reuse `MustRegister` block.
- `MetricsSink` interface (`internal/kernel/lspool/metrics.go`) — pattern to copy verbatim for the new `RepoMapMetricsSink`.
- `RecordRenameStrategy` package-level setter (`internal/mcp/middleware.go:23-44`) — exact pattern for `RecordEditOutcome` (D-16).
- `carveOuts` map (`internal/obs/metrics_labels_test.go:22-28`) — extend with the 4 new carve-out entries.
- `outcomeEnum` (`internal/mcp/middleware.go:100-119`) — the discipline of declaring a closed-enum slice for test assertions; copy for `editOutcomeEnum` and `lifecyclePhaseEnum`.

### Established Patterns
- Single-registry-per-Provider — never touch the prometheus global registerer (T-11-05). All new vectors MUST be registered on `m.registry`.
- Vectors constructed in `newMetrics()`, never lazily — keeps double-registration panics impossible (each Provider gets its own registry).
- Closed-enum labels enforced at emission site (drop unknown values silently), CI lint enforces the label NAME bound only.
- Decoupling via interface in upstream-package and adapter in obs (lspool pattern) when the upstream package must not import `internal/obs`.

### Integration Points
- `internal/daemon/daemon.go` post-init wiring section — where `*obs.Metrics` is bound to the lspool `MetricsSink` interface today; new bindings (RepoMapMetricsSink, edit-outcome recorder, session-lifecycle direct calls in the forwarder handler) plug in here.
- `internal/mcp.InstallMiddleware` — currently sets the `renameStrategySink`; will also set the new `editOutcomeSink` (D-16) so kernel/edit/ stays free of obs.
- HTTP transport session boundary — the planner identifies the equivalent of `daemon.go:606,620` in the Streamable HTTP path during research, then instruments there for the `transport="http"` lifecycle counter.

</code_context>

<specifics>
## Specific Ideas

- The hit-ratio PromQL query in USAGE.md should be the first example added to the metrics section after this lands, demonstrating the `result` label in action: `sum(rate(helix_lspool_lookups_total{result="hit"}[5m])) / sum(rate(helix_lspool_lookups_total[5m]))`.
- Cardinality bound test (success criterion 2) must assert per-family ceilings: lspool_lookups ≤ 2 × N_languages; repomap_lookups ≤ 2 × N_languages; repomap_extract_duration ≤ 3 × N_languages × N_buckets; session_lifecycle ≤ 3 × 2 = 6; edit_outcome ≤ 7 × 6 × 5 = 210.

</specifics>

<deferred>
## Deferred Ideas

- **Workspace-activation cold-start visibility** — measuring time-from-session-start to first activated workspace. Requires a second instrumentation point in `LazyInitMiddleware` and either a new histogram or a paired counter. Operators can approximate today via traces (Phase 12). Defer to Phase 54 / OBS-04 when the trace audit lands.
- **`dirty` dimension on lspool lookups** — distinguishing forced-spawn (dirty session) from spawn-because-no-warm-worker. Today both count as misses. If hit-rate dashboards show unexplained miss spikes during heavy edit sessions, revisit and add the dimension. No active need today.
- **Per-query repomap lookups counter** (vs. per-file granularity in D-03) — e.g., counting `get_repo_map` invocations as single events. Belongs as its own family alongside, not folded into `helix_repomap_lookups_total`. Not requested by OBS-03; defer until a dashboard requirement surfaces it.

</deferred>

---

*Phase: 53-obs-metrics-gaps*
*Context gathered: 2026-04-30*

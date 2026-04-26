# Phase 53: obs-metrics-gaps - Context

**Gathered:** 2026-04-26
**Status:** Ready for planning

<domain>
## Phase Boundary

Close the v1.2 metrics gaps (REQUIREMENTS.md OBS-03) by adding five new Prometheus metric families on top of the existing `*obs.Metrics` infrastructure: lspool cache hit/miss, repomap cache hit/miss, repomap extraction-latency histogram, session lifecycle counter, and edit-tool outcome counter. All labels stay bounded under the existing CI label-allowlist gate; the noop-default invariant from Phase 11 is preserved (zero-alloc when observability is disabled). `USAGE.md` Observability section is updated to document each new metric.

**In scope:**
- Five new metric vectors registered on the same private `*prometheus.Registry` owned by `*obs.Metrics`.
- Per-package `MetricsSink` interfaces in `internal/repomap`, `internal/kernel/edit`, and `internal/kernel` (session lifecycle), each with a NoopSink — mirroring Phase 11's lspool sink boundary.
- New helper methods on `*obs.Metrics` that satisfy each sink interface.
- Daemon wire-up + a `wiring_test.go` compile-time assertion pinning sink signatures (mirrors Phase 11 pattern).
- Closed-enum carve-outs in `metrics_labels_test.go` for new label values: `result` ∈ {hit, miss}, `scope` ∈ {clean, dirty, crashed}, `phase` ∈ {activate, deactivate, timeout, shutdown}, `outcome` ∈ {success, fuzzy_applied, refused_ambiguous, failed}, plus a closed allowlist of edit-tool names for the `tool` label on `serena_edit_outcome_total`.
- Cardinality test asserting max series per metric family (D-04 enforcement).
- `USAGE.md` Observability section: documentation of each new metric, its labels, and semantics.

**Out of scope:**
- Grafana dashboards and runbooks (Phase 54).
- Trace/span coverage audit (Phase 55).
- Trace integration of new metrics (covered when Phase 55 audits exemplar attachment, if needed).
- Migrating any existing v1.2 metric families (no breaking renames of `serena_tool_*`, `serena_lspool_*`, `serena_rename_strategy_total`).

</domain>

<canonical_refs>
## Canonical References

Downstream agents (researcher, planner) MUST read these:

- `.planning/REQUIREMENTS.md` — OBS-03 acceptance language.
- `.planning/ROADMAP.md` — Phase 53 success criteria. **Note:** roadmap success criterion 1 names `serena_lspool_cache_hits_total` and `serena_repomap_cache_hits_total` (hits-only). This phase deliberately diverges to a `result={hit,miss}` shape (D-01); the planner must propose either a roadmap-text amendment or a metric-name reconciliation as part of plan 53-01.
- `internal/obs/metrics.go` — Phase 11 owned-registry pattern, `AllowedLabels` array, frozen sink-helper signatures.
- `internal/obs/metrics_labels_test.go` — CI label-allowlist gate; this is where new closed-enum carve-outs land.
- `internal/kernel/lspool/metrics.go` — Phase 11 sink interface + NoopSink prior art that this phase mirrors for `repomap.MetricsSink`, `edit.MetricsSink`, and the session-lifecycle sink.
- `.planning/phases/11-*` (prior CONTEXT.md / SUMMARY.md if present) — D-08 (no upstream consumer imports `internal/obs`), D-13 (closed-enum carve-out pattern), D-16 (Go runtime + Process collectors registered alongside).
- `internal/kernel/lspool/worker.go` and pool acquire path — emission points for `serena_lspool_cache_total`.
- `internal/repomap/` (TagCache lookup, walkAndExtract) — emission points for `serena_repomap_cache_total` and `serena_repomap_extract_duration_seconds`.
- `internal/kernel/` (WorkspaceRuntime activate/deactivate/timeout) and `internal/daemon/daemon.go` (signal-first shutdown) — emission points for `serena_session_lifecycle_total`.
- `internal/kernel/edit/` (six edit tools) and `internal/fuzzy/` (4-strategy cascade with ambiguity refusal) — emission points and outcome classification source for `serena_edit_outcome_total`.
- `USAGE.md` — Observability section updated per success criterion 3.

</canonical_refs>

<decisions>
## Implementation Decisions

### Cache-hit metric shape (lspool + repomap)

- **D-01 (metric family + names):** Use a single counter family per subsystem with a `result` enum, not hits-only counters. Final names:
  - `serena_lspool_cache_total{language, result, scope}`
  - `serena_repomap_cache_total{language, result}`

  This deliberately renames the families from the roadmap text (`*_cache_hits_total`). PromQL hit-rate becomes `rate(family{result="hit"}[5m]) / rate(family[5m])` without a multi-family join.

- **D-02 (labels):**
  - lspool: `{language, result, scope}`. `result` ∈ {hit, miss}; `scope` ∈ {clean, dirty, crashed} — describes WHY a worker share decision was made (clean = share-until-dirty match, dirty = file changes invalidated reuse, crashed = circuit/restart blocked reuse).
  - repomap: `{language, result}`. `result` ∈ {hit, miss}; mtime-invalidation drives the hit/miss distinction.
  - `result` is a NEW closed-enum carve-out in `metrics_labels_test.go`. `scope` is a second carve-out applied only to the lspool family (mirrors Phase 47 D-07 strategy carve-out).

- **D-03 (cardinality bound):** language is bounded by the registered LS catalog (~52); 52 × 2 (result) × 3 (scope) = 312 max series for lspool, 52 × 2 = 104 for repomap. The cardinality test asserts these caps.

### Session lifecycle (`serena_session_lifecycle_total`)

- **D-04 (phase enum):** Closed set: {activate, deactivate, timeout, shutdown}. Each phase fires once per workspace lifecycle event.
  - `activate` from kernel.WorkspaceRuntime activation (both LazyInitMiddleware first-call and explicit `serena activate` paths share the same call site).
  - `deactivate` from explicit `serena deactivate` and on client-disconnect / handle teardown.
  - `timeout` from adaptive-TTL / idle eviction in the workspace runtime.
  - `shutdown` from the daemon's signal-first shutdown sweep, emitted once per still-active workspace before kernel teardown.
- **D-05 (labels):** `{language, phase}`. `language` already in `AllowedLabels`. `phase` is a NEW closed-enum carve-out in `metrics_labels_test.go`.
- **D-06 (workspace_path NOT a label):** Explicitly rejected — workspace_path is unbounded user-disk-path data and would violate the D-04 cardinality bound. If per-workspace insight is needed later, route it through traces/exemplars (Phase 55), not labels.

### Edit-tool outcome (`serena_edit_outcome_total`)

- **D-07 (outcome enum):** Closed set: {success, fuzzy_applied, refused_ambiguous, failed}.
  - `success` — exact-match strategy applied (the happy path).
  - `fuzzy_applied` — one of the fuzzy cascade strategies (whitespace-normalized / indentation-flexible / ellipsis-placeholder) succeeded. Operators want this rate as a leading indicator of LLM drift.
  - `refused_ambiguous` — fuzzy cascade hit multiple candidates and refused per the ambiguity guard.
  - `failed` — catch-all failure: no match, IO/write error, LSP rejection. Phase keeps a single failure bucket; finer breakdowns can be added later if dashboards demand it.
- **D-08 (tool allowlist):** `tool` is a closed allowlist of edit-tool names (e.g., `replace_in_file`, `replace_symbol_body`, `fuzzy_edit`, `insert_at_line_in_file`, `delete_lines`, `create_text_file`, `replace_regex` — research will confirm the final set against the registry). Unknown values are dropped at the sink (`return` early), mirroring `RenameStrategyInc` (Phase 47 D-07). The allowlist lives next to the sink and is asserted in `metrics_labels_test.go`.
- **D-09 (label set):** `{tool, outcome}`. No `language` here — the file-path / language inference inside edit tools is non-trivial and the outcome breakdown is the load-bearing dimension. If per-language signal is wanted later, infer from the surrounding `serena_tool_*` RED metrics by `tool_name` correlation.

### RepoMap extraction histogram (`serena_repomap_extract_duration_seconds`)

- **D-10 (buckets):** `prometheus.DefBuckets` (consistent with `serena_tool_duration_seconds`). No custom buckets in this phase. If real-world p95 telemetry from Phase 54 dashboards justifies finer resolution at sub-ms or multi-second tails, that is a future tuning patch, not a phase-53 bikeshed.
- **D-11 (labels):** `{language}`. Bounded by LS catalog. Per-language p95 falls out for free in PromQL.

### Sink wiring boundary

- **D-12 (per-package sinks + NoopSink — mirrors Phase 11 D-08):** Three new sink interfaces:
  - `internal/repomap/metrics.go` — `repomap.MetricsSink` with `RepoMapCacheInc(language, result string)` and `RepoMapExtractObserve(language string, seconds float64)`. Plus `NoopSink`.
  - `internal/kernel/edit/metrics.go` — `edit.MetricsSink` with `EditOutcomeInc(tool, outcome string)`. Plus `NoopSink`.
  - `internal/kernel/session_metrics.go` (or under an existing kernel file — researcher to choose) — `kernel.SessionMetricsSink` with `SessionLifecycleInc(language, phase string)`. Plus `NoopSink`.

  Each consumer package depends ONLY on its own sink interface, never on `internal/obs` directly (D-08 invariant from Phase 11).

- **D-13 (helper methods on *obs.Metrics):** Add the corresponding helpers on `*obs.Metrics` so it satisfies all three new interfaces (`RepoMapCacheInc`, `RepoMapExtractObserve`, `EditOutcomeInc`, `SessionLifecycleInc` — plus the lspool cache helpers `LSPoolCacheInc(language, result, scope string)`). lspool's existing `MetricsSink` is extended, not replaced — adding `LSPoolCacheInc` is the only breaking change to that frozen interface, and it lands in this phase.

- **D-14 (wiring_test):** A `internal/daemon/wiring_test.go` (or extension to the existing one) compile-time-asserts:
  ```go
  var _ repomap.MetricsSink = (*obs.Metrics)(nil)
  var _ edit.MetricsSink = (*obs.Metrics)(nil)
  var _ kernel.SessionMetricsSink = (*obs.Metrics)(nil)
  var _ lspool.MetricsSink = (*obs.Metrics)(nil) // extended
  ```

### Noop-default invariant

- **D-15:** All five new metric vectors are constructed unconditionally in `newMetrics()` and registered against the owned registry. Noop providers still hand consumers a real `*obs.Metrics` whose vectors go unscraped — no `if metrics != nil` branches at call sites. This is the Phase 11 invariant; this phase preserves it.

### CI / test gates

- **D-16:** New tests added in this phase:
  1. `metrics_labels_test.go` — extend `TestMetricsLabelsAllowlist` (or a sibling) to assert: (a) base labels stay in `AllowedLabels`; (b) `result`, `scope`, `phase`, `outcome`, edit-`tool` carve-outs are explicitly listed and bounded.
  2. A cardinality test that registers worst-case label values and asserts max series per family (per D-03 numbers and analogous bounds for the new families).
  3. A `wiring_test.go` block per D-14.
  4. Per-subsystem unit tests for sink emission (one per sink interface, asserts the right vector & labels are touched).

</decisions>

<code_context>
## Reusable Assets and Patterns

- **Owned-registry pattern** (`internal/obs/metrics.go:57-131`): construct vectors once, register against a private `*prometheus.Registry`. Reused for all five new families.
- **Closed-enum carve-out pattern** (`internal/obs/metrics.go:84-93` for `reason`; `internal/obs/metrics.go:108-116` and `RenameStrategyInc:171-176` for `strategy`): drop unknown values at the sink + carve out from `AllowedLabels` in `metrics_labels_test.go`. Used for `result`, `scope`, `phase`, `outcome`, and edit-`tool`.
- **MetricsSink interface + NoopSink + compile-time assertion** (`internal/kernel/lspool/metrics.go:1-63`): the prior art the three new sinks copy verbatim in shape.
- **DefBuckets convention** (`internal/obs/metrics.go:69-76`): mirrored for `serena_repomap_extract_duration_seconds`.
- **Daemon wiring_test compile-time assertion** (Phase 11 pattern referenced from `internal/kernel/lspool/metrics.go:8-9`): the proven mechanism for keeping sink signatures in lockstep across packages.

</code_context>

<deferred>
## Deferred Ideas

- **Per-workspace lifecycle insight** — if operators want to see which specific workspace is churning, route through trace exemplars in Phase 55 instead of adding `workspace_path` as a metric label. Captured here so it isn't lost.
- **Custom histogram buckets for repomap extraction** — revisit only if Phase 54 dashboards show DefBuckets is too coarse at the sub-ms or multi-second extremes. Not bikeshedding it now.
- **Finer edit-failure outcome breakdown** — split `failed` into {not_found, io_error, lsp_error} if Phase 54 runbook authoring shows operators need it. Single bucket for now.
- **Roadmap text reconciliation** — the rename of `*_cache_hits_total` → `*_cache_total{result=...}` requires either a ROADMAP success-criterion edit or a documented divergence note. Planner picks the form during plan 53-01.

</deferred>

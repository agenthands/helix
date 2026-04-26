# Phase 53 Discussion Log

**Date:** 2026-04-26
**Mode:** discuss (default)

## Areas Selected by User

All four offered gray areas:
1. Cache-hit shape (hits-only vs hit/miss)
2. Session lifecycle phase enum
3. Edit outcome enum + tool allowlist
4. Sink wiring + histogram buckets

## Q&A Trail

### Area 1: Cache-hit shape

**Q1: Counter shape for lspool + repomap cache hit/miss?**
Options:
- `result={hit,miss}` on one counter family per subsystem (Recommended)
- Hits-only counters matching ROADMAP wording
- Separate hits and misses counter families

**Selected:** `result={hit,miss}` on one counter family per subsystem.
**Note:** This renames the families from the roadmap text. Captured as a deliberate divergence; planner will reconcile.

**Q2: What labels accompany the cache-hit counter(s)?**
Options (multi-select):
- `language`
- `scope` ∈ {clean, dirty, crashed} — lspool only
- no labels

**Selected:** `language` AND `scope` (lspool only).

### Area 2: Session lifecycle

**Q1: Which `phase` enum values?**
Options (multi-select):
- activate, deactivate, timeout, shutdown

**Selected:** All four.

**Q2: Label set?**
Options:
- `language` only (Recommended)
- no labels beyond `phase`
- `language` + `workspace_path` (UNBOUNDED)

**Selected:** `language` only.

### Area 3: Edit outcome

**Q1: Which `outcome` enum values?**
Options (multi-select):
- success, fuzzy_applied, refused_ambiguous, failed

**Selected:** All four.

**Q2: `tool` label scope?**
Options:
- Closed allowlist of edit-tool names (Recommended)
- Open string
- Reuse `tool_name` from RED metrics

**Selected:** Closed allowlist of edit-tool names.

### Area 4: Sink wiring + histogram buckets

**Q1: How are sinks exposed to consumers?**
Options:
- Per-package `MetricsSink` interfaces + NoopSink (Recommended) — mirrors Phase 11 D-08
- Fold helpers onto *obs.Metrics directly
- One unified MetricsSink in internal/obs/sink

**Selected:** Per-package `MetricsSink` interfaces + NoopSink.

**Q2: Histogram buckets and labels for `serena_repomap_extract_duration_seconds`?**
Options:
- `DefBuckets`, labeled by `language` (Recommended)
- Custom buckets, labeled by `language`
- `DefBuckets`, no labels

**Selected:** `DefBuckets`, labeled by `language`.

## Deferred Ideas (captured for backlog)

- Per-workspace lifecycle insight via trace exemplars (Phase 55+)
- Custom repomap histogram buckets if dashboards demand it (post-Phase 54)
- Finer edit-failure outcome breakdown ({not_found, io_error, lsp_error}) if Phase 54 runbooks demand it
- Roadmap text reconciliation for `*_cache_total` rename

## Claude's Discretion

- Specific helper-method names (`LSPoolCacheInc`, `RepoMapCacheInc`, `RepoMapExtractObserve`, `EditOutcomeInc`, `SessionLifecycleInc`) — fixed in CONTEXT.md as a starting point; planner may rename if a stronger convention emerges in research.
- Final closed allowlist of edit-tool names — researcher to enumerate from `internal/kernel/edit/`.
- Exact location of `kernel.SessionMetricsSink` (new file vs existing) — left to researcher / planner.

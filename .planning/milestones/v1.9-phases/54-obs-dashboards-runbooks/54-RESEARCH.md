# Phase 54: obs-dashboards-runbooks - Research

**Researched:** 2026-05-01
**Domain:** Operator-facing observability artifacts (Grafana dashboards + Markdown runbooks + PromQL validation test)
**Confidence:** HIGH on technical surface (parser API, Grafana JSON, file:line targets, registry); MEDIUM on dashboard-specific Grafana 10 schema minutiae (inferred from web sources, will be verified by actual import test in execution).

## Summary

Phase 54 ships four artifact buckets on top of the metric inventory that landed in Phase 53: two Grafana dashboards (`deploy/grafana/{helix-overview,helix-engine}.json`), four runbooks (`docs/runbooks/{ErrCircuitOpen,deadline-timeouts,ls-crash-restart,memory-pressure-eviction}.md`), one screenshot (`docs/images/helix-overview-dashboard.png`), and one CI test (`internal/obs/dashboards_test.go`) that validates every PromQL expression in those JSON+MD files against the live registered Prometheus registry. USAGE.md grows two new H3 subsections before the existing `### Prometheus Metrics`.

The technical novelty is concentrated in the validation test, which adds **one new top-level Go dependency** — `github.com/prometheus/prometheus@v3.11.x` — to use its `promql/parser` subpackage. This is import-only (server code is not pulled in), but the module is large; the planner must verify `go.sum` size impact and that `tools/audit-embed.go` (Phase 52) is not impacted. Beyond the parser, all panel JSON, runbook PromQL, and capture procedures are content authoring against an inventory that is fully known and stable.

**Primary recommendation:** Treat this as a content-authoring phase with one dependency-management gate at the start (Wave 0: add `prometheus/prometheus`, write the validation test scaffold against an empty `deploy/grafana/` to confirm fail-closed behavior, then build out dashboards + runbooks against the test). All four runbooks reference code at known `file:line` targets enumerated in §A.5 below.

## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-01–D-04 Dashboard split & panels:** Two dashboards, `helix-overview.json` (RED + sessions, screenshot target) + `helix-engine.json` (lspool + repomap + edits, on-call deep-dive). 6–9 panels each. Three-dashboard split rejected; mega-dashboard rejected.
- **D-02 overview owns:** tool-call rate, tool-call error rate by `outcome`, p50/p95/p99 of `helix_tool_duration_seconds`, session lifecycle rate by `phase` and `transport`, plus a `text` panel restating the Phase 53 D-09 best-effort caveat for HTTP `ended`.
- **D-03 engine owns:** lspool worker count by language, eviction rate by `reason`, circuit state, restart rate, lspool cache hit-ratio (Phase 53 D-01 PromQL verbatim), repomap cache hit-ratio, per-extractor p95 RepoMap extract latency (Phase 53 D-05/D-06 PromQL verbatim), edit-tool outcome rate by `outcome` and `tool_name`, rename strategy share.
- **D-05 Datasource:** `${DS_PROMETHEUS}` import-template pattern with `__inputs` + `__requires` blocks. Hardcoded UIDs and provisioning sidecar rejected.
- **D-06 Schema version:** ≥ 38 (Grafana 10 baseline).
- **D-07:** No alerting blocks in panels.
- **D-08 Template variables:** `$language` (from `label_values(helix_lspool_workers, language)`) + `$instance` (from `label_values(up{job=~".*helix.*"}, instance)`). Both Multi-select, Include-All, default = All.
- **D-09:** No `$tool`, `$outcome`, `$strategy`, `$extractor` dropdowns.
- **D-10–D-14 Validation test:** `internal/obs/dashboards_test.go`. Uses `*obs.NewProvider(ctx, obs.Config{})` to build registry, `Registry().Gather()` to enumerate families. Uses `github.com/prometheus/prometheus/promql/parser` for AST-based extraction. Reads dashboards + runbooks from disk (resolved via `runtime.Caller(0)` + `filepath.Join(..., "../../..")` from `internal/obs/`). Histogram suffix stripping (`_bucket`/`_count`/`_sum`). Label NAME (not value) validation against `AllowedLabels` + `carveOuts`.
- **D-15 Frontmatter:** `title`, `severity` ∈ {warning, critical}, `metric`, `since_phase`, `last_reviewed`.
- **D-16 H2 order (fixed):** Symptoms → Triage → Likely Causes → Remediation.
- **D-17:** Each runbook ends with `## Code references` (bulleted `file:line — context`).
- **D-18/D-19:** Runbooks author PromQL independently of dashboards; drift caught by validation test, not text-equivalence.
- **D-20 Screenshot:** Manual one-time PNG at `docs/images/helix-overview-dashboard.png` at 1600×900. Re-capture on visible drift only.
- **D-21/D-22 USAGE.md:** New `### Grafana Dashboards` and `### Runbooks` H3 subsections inserted immediately before existing `### Prometheus Metrics` (currently at line 634).
- **D-23 Metric set frozen:** Only the 12 helix_* families registered as of end of Phase 53.
- **D-24 Panel kit:** `timeseries`, `stat`, `gauge`, `barchart` only.
- **D-25 File layout:** Two top-level files in `deploy/grafana/`, no subdirs, no umbrella file.

### Claude's Discretion

- Final panel count per dashboard within 6–9.
- Exact PromQL strings (must pass D-10–D-14 validation).
- Exact `file:line` numbers (this RESEARCH provides current values; planner re-verifies at task time).
- Whether `prometheus/prometheus` is already a transitive dep (RESEARCH below: **NO**, it must be added explicitly).
- Stat-panel thresholds for hit-ratio panels (sketch: green ≥ 0.7 / amber 0.4–0.7 / red < 0.4).
- `timeseries` vs. `barchart` vs. `stat` selection per panel within the Grafana stock kit.

### Deferred Ideas (OUT OF SCOPE)

- Provisioning sidecar (docker-compose with Grafana + Prometheus pre-wired).
- Alerting rules / Alertmanager integration.
- Per-tool latency dashboards / per-language deep-dive dashboards.
- Recapture screenshot via CI.
- Embed runbook content in the daemon binary (`helix runbook ErrCircuitOpen`).
- Cross-link USAGE.md ↔ runbooks from error messages (belongs in v1.3 typed-error work).

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| OBS-01 | Grafana JSON dashboards in `deploy/grafana/` covering RED + lspool worker health + workspace activity | §A.1 (metric inventory), §A.2 (panel ↔ metric map), §C.1 (dashboard JSON skeleton), §C.3 (template variables) |
| OBS-02 | Written runbooks in `docs/runbooks/` for the four operational failure modes | §A.5 (code locations + file:line), §C.4 (runbook skeleton + frontmatter), §A.6 (per-runbook PromQL queries) |

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|--------------|----------------|-----------|
| Dashboard JSON authoring | Operator artifact (versioned in repo, not runtime) | — | Static files in `deploy/grafana/`; consumed by an external Grafana instance, not the daemon |
| Runbook authoring | Operator artifact (Markdown) | — | Static files in `docs/runbooks/`; consumed by humans during incidents |
| PromQL validation test | Daemon test (`internal/obs/`) | — | Co-located with the registry it validates; runs in `go test ./...` |
| Screenshot capture | Manual operator workflow (one-time) | — | Out of CI; `last_reviewed` checkpoint pattern from runbooks mirrors |
| `${DS_PROMETHEUS}` parameterization | Grafana import wizard | — | UID-coupling deliberately avoided; works on any Prometheus datasource name |

## §A — Domain Knowledge

### A.1 Metric inventory (frozen as of end of Phase 53)

`[VERIFIED: internal/obs/metrics.go:82-197]` Twelve helix-owned metric families plus the `go_*` and `process_*` runtime families from `collectors.NewGoCollector` + `collectors.NewProcessCollector`. Helix families:

| Family | Type | Labels | Closed enums |
|--------|------|--------|--------------|
| `helix_tool_calls_total` | counter | `tool_name`, `profile`, `mode`, `language`, `outcome` | `outcome` ∈ {success, invalid_args, not_found, circuit_open, ls_crash, timeout, internal} (only {success, circuit_open, timeout, internal} actually emitted in v1.2 per USAGE.md) |
| `helix_tool_duration_seconds` | histogram | `tool_name`, `profile`, `mode`, `language` | DefBuckets |
| `helix_lspool_workers` | gauge | `language` | — |
| `helix_lspool_evictions_total` | counter | `language`, `reason` | `reason` ∈ {idle, pressure, crash, shutdown} |
| `helix_lspool_circuit_state` | gauge | `language` | values 0=closed, 1=half-open, 2=open |
| `helix_lspool_restarts_total` | counter | `language` | — |
| `helix_rename_strategy_total` | counter | `strategy` | `strategy` ∈ {lsp-native, rust-client-side} |
| `helix_lspool_lookups_total` | counter | `language`, `result` | `result` ∈ {hit, miss} |
| `helix_repomap_lookups_total` | counter | `language`, `result` | `result` ∈ {hit, miss} |
| `helix_repomap_extract_duration_seconds` | histogram | `language`, `extractor` | `extractor` ∈ {treesitter, lsp, fallback}; custom buckets {1ms, 2.5ms, 5ms, 10ms, 25ms, 50ms, 100ms, 250ms, 500ms, 1s, 2.5s} |
| `helix_session_lifecycle_total` | counter | `phase`, `transport` | `phase` ∈ {started, ended, error}; `transport` ∈ {stdio, http}; `transport=http,phase=ended` is best-effort |
| `helix_edit_outcome_total` | counter | `tool_name`, `outcome`, `strategy` | `outcome` ∈ {success, no_match, ambiguous_match, validation_failed, ls_error, internal}; `strategy` ∈ {exact, whitespace_normalized, indentation_flexible, none} |

`AllowedLabels` (from `internal/obs/metrics.go:28`): `[tool_name, profile, mode, language, outcome]`. All other label names must appear in `carveOuts` (from `internal/obs/metrics_labels_test.go:22-45`):

```go
"helix_lspool_evictions_total":           {"reason": true},
"helix_rename_strategy_total":            {"strategy": true},
"helix_lspool_lookups_total":             {"result": true},
"helix_repomap_lookups_total":            {"result": true},
"helix_repomap_extract_duration_seconds": {"extractor": true},
"helix_session_lifecycle_total":          {"phase": true, "transport": true},
"helix_edit_outcome_total":               {"strategy": true},
```

### A.2 Panel → metric mapping (planner authoring guide)

**`helix-overview.json` (6–9 panels):**

| Panel | Type | Metric | PromQL sketch |
|-------|------|--------|---------------|
| Tool-call rate | timeseries | `helix_tool_calls_total` | `sum by (tool_name) (rate(helix_tool_calls_total{language=~"$language"}[5m]))` |
| Error rate by outcome | timeseries | `helix_tool_calls_total` | `sum by (outcome) (rate(helix_tool_calls_total{outcome!="success", language=~"$language"}[5m]))` |
| p50 / p95 / p99 latency | timeseries (3 quantiles) | `helix_tool_duration_seconds` | `histogram_quantile(0.95, sum by (le) (rate(helix_tool_duration_seconds_bucket{language=~"$language"}[5m])))` |
| Session start rate | timeseries | `helix_session_lifecycle_total` | `sum by (transport) (rate(helix_session_lifecycle_total{phase="started"}[5m]))` |
| Session error rate | stat | `helix_session_lifecycle_total` | `sum (rate(helix_session_lifecycle_total{phase="error"}[5m]))` |
| Best-effort caveat callout | text panel | — | (Phase 53 D-09 caveat — no query) |

**`helix-engine.json` (6–9 panels):**

| Panel | Type | Metric | PromQL sketch |
|-------|------|--------|---------------|
| LS workers by language | timeseries | `helix_lspool_workers` | `helix_lspool_workers{language=~"$language", instance=~"$instance"}` |
| Eviction rate by reason | barchart or timeseries | `helix_lspool_evictions_total` | `sum by (reason) (rate(helix_lspool_evictions_total{language=~"$language"}[5m]))` |
| Circuit state per language | stat (per-lang) or timeseries | `helix_lspool_circuit_state` | `helix_lspool_circuit_state{language=~"$language"}` |
| Restart rate | timeseries | `helix_lspool_restarts_total` | `sum by (language) (rate(helix_lspool_restarts_total[5m]))` |
| LSPool cache hit-ratio | stat (with sparkline) | `helix_lspool_lookups_total` | `sum(rate(helix_lspool_lookups_total{result="hit"}[5m])) / sum(rate(helix_lspool_lookups_total[5m]))` (Phase 53 D-01 verbatim) |
| RepoMap cache hit-ratio | stat | `helix_repomap_lookups_total` | `sum(rate(helix_repomap_lookups_total{result="hit"}[5m])) / sum(rate(helix_repomap_lookups_total[5m]))` |
| Per-extractor p95 RepoMap latency | timeseries | `helix_repomap_extract_duration_seconds` | `histogram_quantile(0.95, sum by (le, extractor) (rate(helix_repomap_extract_duration_seconds_bucket[5m])))` (Phase 53 D-05/D-06 verbatim) |
| Edit-tool outcome rate | timeseries | `helix_edit_outcome_total` | `sum by (outcome, tool_name) (rate(helix_edit_outcome_total[5m]))` |
| Rename strategy share | stat or barchart | `helix_rename_strategy_total` | `sum by (strategy) (rate(helix_rename_strategy_total[5m]))` |

`[VERIFIED: USAGE.md:670-704]` Phase 53's hit-ratio and per-extractor p95 PromQL examples are reused verbatim.

### A.3 Grafana 10 schema minimum (`__inputs` + `__requires` import-template)

`[CITED: grafana.com docs, community.grafana.com forum]` `[ASSUMED]` for exact `schemaVersion` field value at Grafana 10.0/10.4/11.

The `${DS_PROMETHEUS}` import-template pattern requires three top-level dashboard JSON keys:

```json
{
  "__inputs": [
    {
      "name": "DS_PROMETHEUS",
      "label": "Prometheus",
      "description": "",
      "type": "datasource",
      "pluginId": "prometheus",
      "pluginName": "Prometheus"
    }
  ],
  "__requires": [
    { "type": "grafana", "id": "grafana", "name": "Grafana", "version": "10.0.0" },
    { "type": "datasource", "id": "prometheus", "name": "Prometheus", "version": "1.0.0" },
    { "type": "panel", "id": "timeseries", "name": "Time series", "version": "" },
    { "type": "panel", "id": "stat", "name": "Stat", "version": "" }
  ],
  "schemaVersion": 38,
  "panels": [ ... ]
}
```

`[ASSUMED]` `schemaVersion: 38` is the Grafana 10.0 baseline; 39 is Grafana 10.4+. Targeting 38 maximizes import-cleanly compatibility; planner may bump to 39 if a needed feature requires it.

`[VERIFIED: web search, prometheus.io/docs/visualization/grafana]` On import, Grafana prompts the operator to bind `DS_PROMETHEUS` to a Prometheus datasource of any UID/name. This works on Grafana 10/11/12 (the import wizard prompt wording is "Select a Prometheus data source").

### A.4 Template variable JSON (D-08)

`[CITED: grafana.com docs]`

```json
{
  "templating": {
    "list": [
      {
        "name": "language",
        "label": "Language",
        "type": "query",
        "datasource": { "type": "prometheus", "uid": "${DS_PROMETHEUS}" },
        "query": { "query": "label_values(helix_lspool_workers, language)", "refId": "language" },
        "refresh": 2,
        "multi": true,
        "includeAll": true,
        "allValue": ".*",
        "current": { "selected": false, "text": "All", "value": "$__all" }
      },
      {
        "name": "instance",
        "label": "Instance",
        "type": "query",
        "datasource": { "type": "prometheus", "uid": "${DS_PROMETHEUS}" },
        "query": { "query": "label_values(up{job=~\".*helix.*\"}, instance)", "refId": "instance" },
        "refresh": 2,
        "multi": true,
        "includeAll": true,
        "allValue": ".*",
        "current": { "selected": false, "text": "All", "value": "$__all" }
      }
    ]
  }
}
```

`refresh: 2` = "On Time Range Change". Filter usage in panel queries: `{language=~"$language", instance=~"$instance"}`. Metrics that don't carry one of these labels (e.g., `helix_session_lifecycle_total` — neither `language` nor `instance`) just omit the matcher.

### A.5 Code locations for runbook `## Code references` (D-17)

`[VERIFIED: SMTC + grep at 2026-05-01]` These are the canonical anchors. Line numbers will drift; planner re-verifies at task time.

| Runbook | File | Line | Symbol/context |
|---------|------|------|----------------|
| `ErrCircuitOpen.md` | `internal/kernel/lspool/circuit.go` | 33 | `NewCircuitBreaker(language, maxBackoff, restartBudget, sink)` — `restartBudget` defaults to 3 (D-05); circuit stays open permanently when budget exhausted |
| `ErrCircuitOpen.md` | `internal/kernel/lspool/circuit.go` | 64 | `RecordFailure` — decorrelated jitter backoff (`prevSleep*3` capped at `maxBackoff`) |
| `ErrCircuitOpen.md` | `internal/kernel/lspool/circuit.go` | 130 | `OpenError` — flattens (language, failures, backoff, retry-after) into Detail |
| `ErrCircuitOpen.md` | `internal/kernel/lspool/pool.go` | 22 | `RestartBudget` config knob (default 3) |
| `ErrCircuitOpen.md` | `internal/kernel/lspool/pool.go` | 320 | `cb = NewCircuitBreaker(language, 5*time.Minute, p.config.RestartBudget, p.metrics)` |
| `deadline-timeouts.md` | `internal/mcp/middleware.go` | 113 | `BudgetFunc` type — nil disables deadline injection |
| `deadline-timeouts.md` | `internal/mcp/middleware.go` | 277 | `TelemetryMiddleware` — captures `m := provider.Metrics()` and `tracer` once |
| `deadline-timeouts.md` | `internal/mcp/middleware.go` | 304-313 | D-01 deadline injection block — `context.WithTimeout(ctx, budget)` |
| `deadline-timeouts.md` | `internal/config/config.go` | 28-32 | `DegradationConfig` — `TimeoutRead` / `TimeoutSearch` / `TimeoutEdit` / `TimeoutIndex` / `TimeoutDiagnostics` |
| `deadline-timeouts.md` | `internal/config/defaults.go` | 25-29 | Default values: 5s/15s/10s/120s/20s |
| `deadline-timeouts.md` | `USAGE.md` | 818-820 | Operator-facing knob table |
| `ls-crash-restart.md` | `internal/kernel/lspool/pool.go` | 414-416 | Crash booking — `evictWorkerLocked(id, w, EvictCrash)` |
| `ls-crash-restart.md` | `internal/kernel/lspool/pool.go` | 306 | `RecordSuccess` resets the failure counter after a successful op |
| `ls-crash-restart.md` | `internal/kernel/lspool/worker.go` | 434 | Handler-panic recovery (panics are recovered, logged, never crash daemon) |
| `ls-crash-restart.md` | `internal/kernel/lspool/circuit.go` | 64 | Decorrelated jitter backoff governs the restart cadence |
| `memory-pressure-eviction.md` | `internal/kernel/lspool/pressure_linux.go` | 22 | `os.ReadFile("/proc/pressure/memory")` — Linux PSI |
| `memory-pressure-eviction.md` | `internal/kernel/lspool/pressure_darwin.go` | 24 | `exec.Command("vm_stat").Output()` — macOS pressure detector |
| `memory-pressure-eviction.md` | `internal/kernel/lspool/pool.go` | 403, 432, 452 | `evictWorkerLocked(..., EvictPressure)` call sites |
| `memory-pressure-eviction.md` | `internal/kernel/lspool/pressure.go` | 9-15 | `PressureLevel` enum (None/Low/Medium/High/Critical) |
| `memory-pressure-eviction.md` | `internal/kernel/lspool/metrics.go` | 38 | `EvictPressure = "pressure"` constant — emitted to `helix_lspool_evictions_total{reason="pressure"}` |

### A.6 Per-runbook diagnostic PromQL (planner authoring guide)

**`ErrCircuitOpen.md`** triage queries:
```promql
# Which language's circuit is currently open? (state 2 = open)
helix_lspool_circuit_state == 2

# Recent crash + pressure evictions — likely cause class
sum by (language, reason) (rate(helix_lspool_evictions_total{reason=~"crash|pressure"}[5m]))

# Cross-reference: tool calls that hit a circuit-open outcome
sum by (language) (rate(helix_tool_calls_total{outcome="circuit_open"}[5m]))

# Restart pressure — top languages by restart rate (5min)
topk(5, sum by (language) (rate(helix_lspool_restarts_total[5m])))
```

**`deadline-timeouts.md`** triage queries:
```promql
# Tools currently hitting timeout outcome
sum by (tool_name) (rate(helix_tool_calls_total{outcome="timeout"}[5m]))

# Timeout rate as a fraction of total calls per tool
sum by (tool_name) (rate(helix_tool_calls_total{outcome="timeout"}[5m]))
  / sum by (tool_name) (rate(helix_tool_calls_total[5m]))

# p95 latency by tool — look for tools brushing their deadline
histogram_quantile(0.95, sum by (le, tool_name) (rate(helix_tool_duration_seconds_bucket[5m])))
```

**`ls-crash-restart.md`** triage queries:
```promql
# Crash eviction rate by language (5min)
sum by (language) (rate(helix_lspool_evictions_total{reason="crash"}[5m]))

# Restart rate alongside — crashes that successfully restarted
sum by (language) (rate(helix_lspool_restarts_total[5m]))

# Worker count drop-off — gauge view
helix_lspool_workers{language=~"$language"}
```

**`memory-pressure-eviction.md`** triage queries:
```promql
# Pressure-driven eviction rate by language
sum by (language) (rate(helix_lspool_evictions_total{reason="pressure"}[5m]))

# Compare to all eviction reasons — what fraction is pressure?
sum (rate(helix_lspool_evictions_total{reason="pressure"}[5m]))
  / sum (rate(helix_lspool_evictions_total[5m]))
```

Platform-specific shell commands for the Remediation section:
- Linux: `cat /proc/pressure/memory` — interpret avg10 column
- macOS: `vm_stat 5` — watch free vs. inactive page counts

### A.7 Roadmap success-criterion 1 wording

`[VERIFIED]` Success criterion 1 says "covering RED metrics, lspool worker health, AND workspace activity". CONTEXT.md D-02/D-03 split: overview owns RED + sessions (sessions = workspace activity); engine owns lspool. The "and" is satisfied across the SET of dashboards, not within each. This is consistent with the discussion-log ratification of D-02.

## §B — External Dependencies

### B.1 New top-level Go dependency: `github.com/prometheus/prometheus`

`[VERIFIED: go list -m]` `prometheus/prometheus` is **NOT** currently a direct or transitive dependency of helix. Current Prometheus-family deps in `go.mod`:
- `github.com/prometheus/client_golang v1.23.2` (direct)
- `github.com/prometheus/client_model v0.6.2` (direct)
- `github.com/prometheus/common v0.66.1` (indirect)
- `github.com/prometheus/procfs v0.16.1` (indirect)

**Recommended version:** `[VERIFIED: github.com/prometheus/prometheus releases]` v3.11.x (latest as of 2026-04, e.g. v3.11.2 from 2026-04-13). Pinning `v3.11.0` is acceptable for minimum-version semantics; `v3.11.x` patches are non-breaking.

**Verification step at planning time:**
```bash
go get github.com/prometheus/prometheus@v3.11.0
go mod tidy
go list -m all | grep prometheus
```

**Import-only scope:** `[VERIFIED: pkg.go.dev/github.com/prometheus/prometheus/promql/parser]` The `promql/parser` subpackage's transitive imports are bounded:
- `github.com/prometheus/prometheus/model/labels`
- `github.com/prometheus/prometheus/util/features`
- `github.com/prometheus/prometheus/promql/parser/posrange`

The TSDB, server, scrape, rules, web, and storage subpackages are NOT pulled in. Disk impact in `~/go/pkg/mod` is single-digit MB at most for the parser tree.

**`go.sum` impact:** `[ASSUMED]` adds ~20–50 lines (parser + label-matcher transitive). Planner verifies at Wave 0.

**License:** `[VERIFIED: Prometheus repo]` Apache-2.0 — same as existing `client_golang`. No audit concern.

**`tools/audit-embed.go` (Phase 52):** `[ASSUMED]` This script audits embedded binaries via `embed.FS`; it should NOT be impacted by adding a Go module dep. Planner double-checks the script's actual scope at Wave 0 (10-second `grep`).

### B.2 No other new dependencies

JSON parsing (`encoding/json`) and Markdown fence extraction are stdlib. `runtime.Caller` is stdlib. `dto.MetricFamily` is already imported by `metrics_labels_test.go`.

## §C — Implementation Patterns + Code Sketches

### C.1 Dashboard JSON skeleton (planner copy-paste base)

```json
{
  "__inputs": [
    { "name": "DS_PROMETHEUS", "label": "Prometheus", "description": "", "type": "datasource", "pluginId": "prometheus", "pluginName": "Prometheus" }
  ],
  "__requires": [
    { "type": "grafana", "id": "grafana", "name": "Grafana", "version": "10.0.0" },
    { "type": "datasource", "id": "prometheus", "name": "Prometheus", "version": "1.0.0" },
    { "type": "panel", "id": "timeseries", "name": "Time series", "version": "" },
    { "type": "panel", "id": "stat", "name": "Stat", "version": "" }
  ],
  "annotations": { "list": [] },
  "editable": true,
  "graphTooltip": 0,
  "schemaVersion": 38,
  "tags": ["helix", "observability"],
  "templating": { "list": [ /* §A.4 */ ] },
  "time": { "from": "now-1h", "to": "now" },
  "title": "Helix Overview",
  "uid": "helix-overview",
  "version": 1,
  "panels": [
    {
      "id": 1,
      "type": "timeseries",
      "title": "Tool call rate",
      "datasource": { "type": "prometheus", "uid": "${DS_PROMETHEUS}" },
      "gridPos": { "h": 8, "w": 12, "x": 0, "y": 0 },
      "targets": [
        {
          "refId": "A",
          "datasource": { "type": "prometheus", "uid": "${DS_PROMETHEUS}" },
          "expr": "sum by (tool_name) (rate(helix_tool_calls_total{language=~\"$language\"}[5m]))",
          "legendFormat": "{{tool_name}}",
          "range": true
        }
      ],
      "options": {},
      "fieldConfig": { "defaults": {}, "overrides": [] }
    }
  ]
}
```

**Stat panel with thresholds (hit-ratio):**
```json
{
  "id": 5,
  "type": "stat",
  "title": "LSPool cache hit-ratio",
  "datasource": { "type": "prometheus", "uid": "${DS_PROMETHEUS}" },
  "gridPos": { "h": 6, "w": 6, "x": 0, "y": 8 },
  "targets": [
    {
      "refId": "A",
      "expr": "sum(rate(helix_lspool_lookups_total{result=\"hit\"}[5m])) / sum(rate(helix_lspool_lookups_total[5m]))",
      "legendFormat": "hit-ratio"
    }
  ],
  "options": {
    "reduceOptions": { "calcs": ["lastNotNull"], "fields": "", "values": false },
    "graphMode": "area",
    "colorMode": "background"
  },
  "fieldConfig": {
    "defaults": {
      "unit": "percentunit",
      "min": 0,
      "max": 1,
      "thresholds": {
        "mode": "absolute",
        "steps": [
          { "color": "red", "value": null },
          { "color": "yellow", "value": 0.4 },
          { "color": "green", "value": 0.7 }
        ]
      }
    },
    "overrides": []
  }
}
```

`[CITED: grafana stat panel docs]` `reduceOptions.calcs: ["lastNotNull"]` is mandatory for stat panels — Pitfall #2.

**Text panel (best-effort caveat, no query):**
```json
{
  "id": 99,
  "type": "text",
  "title": "Note: HTTP session ended is best-effort",
  "gridPos": { "h": 4, "w": 12, "x": 0, "y": 24 },
  "options": {
    "mode": "markdown",
    "content": "`helix_session_lifecycle_total{transport=\"http\", phase=\"ended\"}` only fires on a client-issued `DELETE /mcp`. Sessions that disappear due to server-side timeout do NOT register. See Phase 53 D-09 / USAGE.md."
  }
}
```

### C.2 Validation test scaffold (`internal/obs/dashboards_test.go`)

```go
package obs

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/prometheus/prometheus/promql/parser"
)

// projectRoot resolves the repository root from this test file's location.
// internal/obs/dashboards_test.go → ../../.. = repo root.
func projectRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}

// stripHistogramSuffix maps "*_bucket"/"*_count"/"*_sum" → base family name.
func stripHistogramSuffix(name string) string {
	for _, sfx := range []string{"_bucket", "_count", "_sum"} {
		if strings.HasSuffix(name, sfx) {
			return strings.TrimSuffix(name, sfx)
		}
	}
	return name
}

// registeredFamilies enumerates the live Prometheus registry via *obs.Provider.
func registeredFamilies(t *testing.T) map[string]bool {
	t.Helper()
	provider := NewProvider(context.Background(), Config{})
	mfs, err := provider.Metrics().Registry().Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	// Prime every vector first so Gather returns non-empty families.
	// (See metrics_labels_test.go:107-125 for the priming pattern.)
	primeAllVectors(t, provider.Metrics())
	mfs, err = provider.Metrics().Registry().Gather()
	if err != nil {
		t.Fatalf("Gather (primed): %v", err)
	}
	out := map[string]bool{}
	for _, mf := range mfs {
		out[mf.GetName()] = true
	}
	return out
}

// extractPromQLFromDashboard walks panels (and nested row panels) to collect
// every targets[].expr string.
type dashFile struct {
	Panels []dashPanel `json:"panels"`
}
type dashPanel struct {
	Type    string      `json:"type"`
	Targets []target    `json:"targets,omitempty"`
	Panels  []dashPanel `json:"panels,omitempty"` // nested rows
}
type target struct {
	Expr string `json:"expr"`
}

func extractPromQLFromDashboard(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var d dashFile
	if err := json.Unmarshal(data, &d); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	var out []string
	var walk func(panels []dashPanel)
	walk = func(panels []dashPanel) {
		for _, p := range panels {
			for _, tgt := range p.Targets {
				if tgt.Expr != "" {
					out = append(out, tgt.Expr)
				}
			}
			if len(p.Panels) > 0 {
				walk(p.Panels)
			}
		}
	}
	walk(d.Panels)
	return out
}

// extractPromQLFromRunbook scans Markdown for ```promql fenced blocks.
var promqlFenceRe = regexp.MustCompile("(?s)```promql\\s*\\n(.*?)```")

func extractPromQLFromRunbook(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var out []string
	for _, m := range promqlFenceRe.FindAllSubmatch(data, -1) {
		// Each fenced block may contain multiple `\n\n`-separated queries.
		// Split on blank line, trim, skip comment-only blocks.
		blocks := strings.Split(string(m[1]), "\n\n")
		for _, b := range blocks {
			// Strip leading comment-only lines
			lines := strings.Split(b, "\n")
			var kept []string
			for _, ln := range lines {
				if strings.HasPrefix(strings.TrimSpace(ln), "#") {
					continue
				}
				kept = append(kept, ln)
			}
			expr := strings.TrimSpace(strings.Join(kept, "\n"))
			if expr != "" {
				out = append(out, expr)
			}
		}
	}
	return out
}

// validateExpr parses a PromQL expression and asserts that every metric name
// (after histogram-suffix stripping) is in `families`, and every label name
// is in AllowedLabels or carveOuts[familyName].
func validateExpr(t *testing.T, expr string, families map[string]bool) {
	t.Helper()
	ast, err := parser.ParseExpr(expr)
	if err != nil {
		t.Errorf("parse %q: %v", expr, err)
		return
	}
	allowed := map[string]bool{}
	for _, l := range AllowedLabels {
		allowed[l] = true
	}
	parser.Inspect(ast, func(n parser.Node, _ []parser.Node) error {
		vs, ok := n.(*parser.VectorSelector)
		if !ok {
			return nil
		}
		base := stripHistogramSuffix(vs.Name)
		// Allow `up` (Grafana template variable convention) and runtime families.
		if base == "up" || strings.HasPrefix(base, "go_") || strings.HasPrefix(base, "process_") {
			// validate labels below
		} else if !families[base] {
			t.Errorf("metric %q (base %q) in expr %q not registered", vs.Name, base, expr)
		}
		carve := carveOuts[base]
		for _, lm := range vs.LabelMatchers {
			ln := lm.Name
			if ln == "__name__" || ln == "le" || ln == "job" {
				continue
			}
			if allowed[ln] || carve[ln] {
				continue
			}
			t.Errorf("label %q on %s in expr %q not in AllowedLabels or carveOuts", ln, vs.Name, expr)
		}
		return nil
	})
}

func TestDashboardsAndRunbooksReferenceRegisteredMetrics(t *testing.T) {
	root := projectRoot(t)
	families := registeredFamilies(t)

	dashGlob := filepath.Join(root, "deploy", "grafana", "*.json")
	dashes, err := filepath.Glob(dashGlob)
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(dashes) == 0 {
		t.Fatalf("no dashboards found at %s", dashGlob)
	}
	for _, path := range dashes {
		for _, expr := range extractPromQLFromDashboard(t, path) {
			validateExpr(t, expr, families)
		}
	}

	rbGlob := filepath.Join(root, "docs", "runbooks", "*.md")
	rbs, err := filepath.Glob(rbGlob)
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(rbs) == 0 {
		t.Fatalf("no runbooks found at %s", rbGlob)
	}
	for _, path := range rbs {
		for _, expr := range extractPromQLFromRunbook(t, path) {
			validateExpr(t, expr, families)
		}
	}
}
```

**Notes on the sketch:**
- `parser.Inspect` is the convenient single-callback form of `parser.Walk` (per pkg.go.dev).
- `up` and runtime families (`go_*`, `process_*`) need an allowlist — `up` is part of the `$instance` template variable query.
- The carveOuts map already exists in `internal/obs/metrics_labels_test.go` — same package, no import needed.
- Histogram suffix stripping handles `histogram_quantile(..., rate(helix_repomap_extract_duration_seconds_bucket[5m]))`.
- `parser.Inspect` traverses into `MatrixSelector.VectorSelector` via the children iterator; explicit handling of `MatrixSelector` is not needed.

### C.3 Path resolution depth check

`internal/obs/dashboards_test.go` → `../../..` → repo root. `[VERIFIED]`: `/Users/.../helix/internal/obs/` is two levels deep; `..` × 2 + one more for `internal` = three. `filepath.Join(dir, "..", "..", "..")` is correct.

**Windows compatibility:** `filepath.Join` and `runtime.Caller` produce native separators on Windows; using `filepath.Glob` instead of any string-comparison hack avoids the path-separator pitfall.

### C.4 Runbook file skeleton (planner copy-paste base)

```markdown
---
title: Circuit breaker open (ErrCircuitOpen)
severity: critical
metric: helix_lspool_circuit_state
since_phase: 11
last_reviewed: 2026-05-01
---

# Circuit breaker open (ErrCircuitOpen)

Helix's per-language LS worker pool trips a circuit breaker after consecutive worker failures exceed `degradation.restart_budget` (default 3). When open, all `tools/call` to that language fail with `ErrCircuitOpen` until backoff expires and a half-open probe succeeds.

## Symptoms

- Tool calls return `circuit_open` outcome (`helix_tool_calls_total{outcome="circuit_open"}` rises).
- `helix_lspool_circuit_state{language="<lang>"} == 2` on the engine dashboard.
- Daemon logs include `circuit breaker open` at WARN.

## Triage

```promql
# Which language's circuit is currently open?
helix_lspool_circuit_state == 2
```
*Filter the result to find the affected language.*

```promql
# Recent crash + pressure evictions — likely cause class
sum by (language, reason) (rate(helix_lspool_evictions_total{reason=~"crash|pressure"}[5m]))
```
*High `crash` rate → LS process is dying repeatedly. High `pressure` rate → memory pressure is forcing eviction.*

## Likely Causes

- **LS process crash storm** — confirmed if `helix_lspool_evictions_total{reason="crash"}` is rising. See `ls-crash-restart.md`.
- **Memory pressure eviction** — confirmed if `helix_lspool_evictions_total{reason="pressure"}` is rising. See `memory-pressure-eviction.md`.
- **Slow LS responses exceeding deadlines** — confirmed if `helix_tool_calls_total{outcome="timeout"}` co-occurs. See `deadline-timeouts.md`.

## Remediation

1. **(operator)** Identify root cause via the triage queries above.
2. **(operator)** If pressure-driven: free memory or raise `degradation.memory_limit_mb`.
3. **(operator)** If crash-driven: check LS version compatibility and restart the daemon.
4. **(engineer)** If neither: increase `degradation.restart_budget` temporarily and file an issue with logs.

## Code references

- `internal/kernel/lspool/circuit.go:33` — `NewCircuitBreaker`; restartBudget default = 3
- `internal/kernel/lspool/circuit.go:64` — `RecordFailure`; decorrelated jitter backoff (capped at maxBackoff = 5min per `pool.go:320`)
- `internal/kernel/lspool/circuit.go:130` — `OpenError`; flattens (language, failures, backoff, retry-after) into Detail
- `internal/kernel/lspool/pool.go:22` — `RestartBudget` config knob
```

## §D — Risk Surface / Pitfalls

### Pitfall #1: `prometheus/prometheus` module size
**What:** Adding a top-level dep on the Prometheus monorepo can balloon `go.sum`.
**Why:** Even though we only import `promql/parser`, `go mod` resolves the entire module's `go.sum` lines.
**Mitigation:** Verify `go.sum` line growth at Wave 0; if > 100 lines, document. Do NOT pull `prometheus/promql` (without the `/parser` suffix) — the parent package brings the engine.

### Pitfall #2: Stat panel `reduceOptions.calcs` mandatory
**What:** First-time stat-panel JSON authors omit `reduceOptions.calcs: ["lastNotNull"]` and the panel renders blank.
**Mitigation:** Use the C.1 stat-panel skeleton verbatim.

### Pitfall #3: `${DS_PROMETHEUS}` in v11+
**What:** Grafana periodically reworks the import wizard.
**Mitigation:** `[VERIFIED: 2026-05]` `${DS_PROMETHEUS}` import-template still works on Grafana 10/11/12 (community-published dashboards continue to use it). Planner verifies during the Wave that ships the screenshot by importing into a Grafana 10.4+ instance.

### Pitfall #4: `runtime.Caller` path on Windows
**What:** Forward-slash hardcoding in path joins breaks Windows tests.
**Mitigation:** Use `filepath.Join` and `filepath.Glob` exclusively; avoid `strings.Split("/")`.

### Pitfall #5: `parser.ParseExpr` and complex queries
**What:** Some Prometheus extensions (experimental functions, duration expressions) require `parser.NewParser(Options{...})` rather than the package-level `ParseExpr`.
**Mitigation:** Phase 53's two example queries (`sum / sum`, `histogram_quantile(0.95, sum by (le, extractor) (rate(...)))`) are vanilla PromQL and parse with the default `ParseExpr`. If a future query uses experimental features, switch to `parser.NewParser(parser.Options{EnableExperimentalFunctions: true}).ParseExpr(expr)`.

### Pitfall #6: Empty registries in test
**What:** Prometheus `Registry().Gather()` omits metric families with zero observations, producing a false negative ("metric not registered").
**Mitigation:** Prime every vector before `Gather()` — mirror the pattern in `metrics_labels_test.go:107-125`. The C.2 sketch above factors this into a `primeAllVectors` helper.

### Pitfall #7: `up` and runtime families
**What:** The `$instance` template variable queries `up{job=~".*helix.*"}` — `up` is a Prometheus-internal series, not in our registry.
**Mitigation:** Allowlist `up`, `go_*`, `process_*` in `validateExpr`.

### Pitfall #8: Markdown fence variants
**What:** ```` ```PromQL ```` vs. ```` ```promql ```` — case matters for the regex.
**Mitigation:** Author standard: lowercase `promql`. Document this in CONTEXT.md follow-up; the regex is `(?s)\x60\x60\x60promql\s*\n(.*?)\x60\x60\x60` (case-sensitive).

### Pitfall #9: Comment-only PromQL blocks
**What:** A block that's entirely `# comment lines` would parse as empty.
**Mitigation:** The C.2 extractor strips comment lines and skips empty residuals.

### Pitfall #10: Dashboard nested row panels
**What:** Some Grafana JSON has `panels[].panels[]` for `row` containers. Missing the recursion misses validation.
**Mitigation:** C.2 walks recursively. Note: D-24 limits panel kit to `timeseries`/`stat`/`gauge`/`barchart` — no `row` panels expected, but the recursion is cheap insurance.

### Pitfall #11: Screenshot data scarcity
**What:** A fresh daemon with no traffic produces flat-zero graphs unsuitable for the screenshot.
**Mitigation:** Capture procedure: run `make bench` (`./test/bench/...`) locally + exercise a few MCP tools (e.g., `mcp__helix__find_references` against the helix repo itself). `make bench` is local-only by user policy — never on CI.

### Pitfall #12: `tools/audit-embed.go` impact
**What:** Phase 52 audit-embed script may scan `go.sum`.
**Mitigation:** `[ASSUMED]` script audits `embed.FS` content, not module deps. Planner runs `make verify-embed-pubkey` (the only embed-related Make target) after `go mod tidy` to confirm no regression. 30-second check.

## §E — Open Questions for Planner

1. **Exact `schemaVersion` value:** 38 (Grafana 10.0) vs. 39 (Grafana 10.4+). Recommendation: start with 38; bump only if a needed feature requires it. Test by importing into Grafana 10.0 and 11.x.
2. **Specific stat-panel thresholds:** CONTEXT sketches green ≥ 0.7 / amber 0.4–0.7 / red < 0.4 for hit-ratio panels. Use as default; planner may revise after running a few real workloads.
3. **`circuit_state` panel type:** `stat` per language (small-multiples) vs. `timeseries` (multi-line). Recommendation: `timeseries` since `language` cardinality is bounded but operator-extensible.
4. **Eviction-rate panel type:** `barchart` (better for closed-enum reasons) vs. `timeseries`. Recommendation: `timeseries` stacked, since operators care about both rate and trend.
5. **`primeAllVectors` location:** Add as a helper in the new `dashboards_test.go`, OR refactor `metrics_labels_test.go:107-125` priming into a shared internal helper. Recommendation: copy-paste in this phase (avoid touching Phase 53 test surface); refactor in a later cleanup phase.
6. **Runbook intro length:** CONTEXT D-spec says "one-paragraph intro". Planner enforces ≤ 4 sentences in review.
7. **Capture-procedure documentation home:** the screenshot capture procedure lives "inline in the phase plan" per D-20, but should it be a permanent doc (e.g., `docs/runbooks/_screenshot-capture.md`)? Recommendation: keep in PLAN.md; not a runbook for an operational failure.

## §F — Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` (existing) |
| Config file | `Makefile` — `make test` runs `go test ./...` |
| Quick run command | `go test ./internal/obs/... -run TestDashboardsAndRunbooksReferenceRegisteredMetrics -count=1` |
| Full suite command | `go test ./... -count=1` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| OBS-01 | Dashboards exist in `deploy/grafana/` | unit (file presence) | `go test ./internal/obs/... -run TestDashboards -count=1` | ❌ Wave 0 |
| OBS-01 | Every panel `targets[].expr` references a registered metric | unit (AST parse + registry lookup) | same | ❌ Wave 0 |
| OBS-01 | Every label name on a vector selector is allowed/carved | unit | same | ❌ Wave 0 |
| OBS-01 | Dashboards import cleanly into Grafana 10+ | manual | (operator imports JSON via UI; checked at screenshot time) | manual |
| OBS-02 | Four runbooks exist in `docs/runbooks/` | unit (file presence) | same | ❌ Wave 0 |
| OBS-02 | Every ` ```promql ` fenced block parses + references registered metrics | unit | same | ❌ Wave 0 |
| OBS-02 | Each runbook has correct frontmatter (title, severity, metric, since_phase, last_reviewed) | unit (frontmatter parse) | optional second test `TestRunbookFrontmatter` | ❌ Wave 0 |
| Success-criterion 4 | Validation test fails closed if dashboards or runbooks dirs are empty | unit | same — `t.Fatalf("no dashboards found")` | ❌ Wave 0 |
| USAGE.md update | New H3 subsections inserted before line 634 | manual review | (no automated test; PLAN-CHECK flags) | manual |
| Screenshot | `docs/images/helix-overview-dashboard.png` exists | manual review | `ls docs/images/helix-overview-dashboard.png` | manual |

**Test failure modes that need positive proof:**
- Add a dashboard panel with a typo metric name → test must fail (positive proof of metric-name validation).
- Add a label with a typo (`results=` instead of `result=`) → test must fail (positive proof of label-name validation).
- Delete all files from `deploy/grafana/` → test must fail-closed (positive proof of fail-closed empty-dir behavior).
- Add a runbook with a `promql` fence containing an unregistered metric → test must fail.

These are the four mutation-test patterns that prove the test catches drift. Implement at least one as an explicit `_catchesDrift` test (mirror `TestMetricsLabelsAllowlist_catchesDrift`) so future-Claude knows the test wasn't silently disabled.

### Sampling Rate
- **Per task commit:** `go test ./internal/obs/... -count=1` (~1s)
- **Per wave merge:** `go test ./... -count=1` (full suite, < 60s)
- **Phase gate:** Full suite green before `/gsd-verify-work`. Plus a manual import test of both dashboards into Grafana 10.4+.

### Wave 0 Gaps
- [ ] Add `github.com/prometheus/prometheus@v3.11.x` to `go.mod`; run `go mod tidy`; verify `go.sum` line growth.
- [ ] Confirm `tools/audit-embed.go` is unaffected (`make verify-embed-pubkey` passes).
- [ ] Create empty stub directories: `deploy/grafana/`, `docs/runbooks/`, `docs/images/` (with `.gitkeep` so the validation test isn't tested against an entirely missing path during Wave 0).
- [ ] Author `internal/obs/dashboards_test.go` skeleton WITH a `TestDashboardsAndRunbooksReferenceRegisteredMetrics_catchesDrift` companion test (drift detection proof).
- [ ] Confirm `.gitignore` does not exclude `deploy/`, `docs/`.
- [ ] Verify `make test` picks up the new test (it does, via `go test ./...`).

## Project Constraints (from CLAUDE.md)

- **Go-native single binary.** All test code uses Go stdlib + the existing prometheus deps + the new `prometheus/prometheus` parser sub-tree. No new languages, no Docker dep.
- **`go vet ./...` and `go test ./...` are mandatory before completion.** Both run by `make vet` and `make test`.
- **GSD workflow gate.** Edits live behind a `/gsd-execute-phase` invocation; planner authors PLAN.md files for the parent agent to invoke.
- **SMTC-first tool routing.** When verifying `file:line` anchors at task time, executor uses `mcp__smtc__goto_definition` / `mcp__smtc__find_declarations` not `Grep`. Already used during this research to find `RestartBudget`, `RecordFailure`, `EvictPressure` anchors.
- **No CI benchmarks** (user auto-memory). The screenshot capture procedure references `make bench` as a LOCAL-only step — must not propose adding any bench invocation to `.github/workflows/`.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|-------------|-----------|---------|----------|
| Go toolchain | All Go code | ✓ | 1.25.1 (per go.mod) | — |
| `go test` | Validation test | ✓ | builtin | — |
| `make` | Build/test entrypoints | ✓ | — | direct `go test` |
| `github.com/prometheus/prometheus/promql/parser` | Validation test | ✗ | — | NONE — must `go get`. NO viable fallback (regex-based extraction was explicitly rejected at D-11). |
| Grafana 10+ instance | Manual import test + screenshot | ✗ (operator-local) | — | docker-compose Grafana for one-shot use; `[ASSUMED]` user has access |
| Prometheus instance | Screenshot dataflow | ✗ (operator-local) | — | docker-compose Prometheus or local binary |

**Missing dependencies with no fallback:**
- `github.com/prometheus/prometheus@v3.11.x` — block until added in Wave 0.

**Missing dependencies with fallback:**
- Grafana + Prometheus instances are needed only for the screenshot wave; planner schedules that wave last and documents the docker-compose one-liner inline.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `schemaVersion: 38` is acceptable for Grafana 10.0 import | A.3, C.1 | Dashboards may need re-export at 39; minor — fix is one-line edit |
| A2 | `${DS_PROMETHEUS}` continues to work on Grafana 11/12 | D, Pitfall #3 | Dashboards may need rework; verified at screenshot time |
| A3 | `tools/audit-embed.go` is unaffected by adding a Go module dep | B.1, Pitfall #12 | Build break; 30s to verify at Wave 0 |
| A4 | `prometheus/prometheus` v3.11.x is compatible with Go 1.25.1 + client_golang v1.23.2 | B.1 | Build break; verified at Wave 0 by `go mod tidy` |
| A5 | `go.sum` growth is bounded at ~50 lines | B.1 | Larger growth is acceptable but worth noting; not blocking |
| A6 | Comment-only PromQL fenced blocks should be skipped (not validated) | C.2, Pitfall #9 | If wrong: pure documentation snippets fail validation; mitigation already in extractor |
| A7 | `parser.Inspect` covers `MatrixSelector.VectorSelector` via children iteration | C.2 | If wrong: range-vector queries (`rate(...)`) skip validation; verifiable in unit test |
| A8 | The capture procedure (`make bench` + manual MCP tool exercise) produces non-trivial graphs | D, Pitfall #11 | Screenshot looks empty; iterate at capture time |
| A9 | Phase 53's `tools/audit-embed.go` only checks `embed.FS` content, not module graph | Pitfall #12 | Slight risk; trivial to verify |
| A10 | Lowercase `promql` fence is the convention to enforce; `PromQL` is rejected | C.2, Pitfall #8 | Author may use uppercase; mitigation: document as a CONTEXT amendment if needed |
| A11 | `up` series is exposed by Prometheus on every scrape and the `$instance` query works against any helix scrape job named `helix*` | A.4 | Template variable shows empty list; operator can edit manually |
| A12 | `parser.ParseExpr` (default options) handles all PromQL used in Phase 53 USAGE.md examples | Pitfall #5 | Switch to `parser.NewParser(Options{...})`; one-line change |

## Sources

### Primary (HIGH confidence)
- `internal/obs/metrics.go:1-306` — registered metric family inventory (read in full)
- `internal/obs/metrics_labels_test.go:1-308` — `AllowedLabels` + `carveOuts` + priming pattern (read in full)
- `internal/kernel/lspool/circuit.go:1-150` — circuit breaker state machine, `RecordFailure` jitter, `OpenError`
- `internal/kernel/lspool/pool.go:22, 320, 403, 414-432, 452, 457` — `RestartBudget`, `evictWorkerLocked` call sites
- `internal/kernel/lspool/pressure_linux.go`, `pressure_darwin.go` — platform-specific pressure detection
- `internal/mcp/middleware.go:113, 277, 304-369` — `BudgetFunc`, `TelemetryMiddleware`, deadline injection block
- `internal/config/config.go:23-32`, `internal/config/defaults.go:25-31` — degradation timeout knobs
- `USAGE.md:600-706` — existing observability docs
- `Makefile` (entire) — `make test`, `make vet`, `make bench` (bench is local-only)
- `.github/workflows/go-test.yml:96-97` — `go test ./... -count=1` (no bench invocation)
- `.planning/phases/53-obs-metrics-gaps/53-CONTEXT.md` — D-01 through D-17 metric inventory and closed-enum decisions
- `pkg.go.dev/github.com/prometheus/prometheus/promql/parser` — full parser API, `Walk`/`Inspect`/`VectorSelector`/`MatrixSelector` signatures

### Secondary (MEDIUM confidence)
- `github.com/prometheus/prometheus` releases (latest stable v3.11.x, April 2026) — version recommendation for `go get`
- Grafana docs for dashboard JSON model + `__inputs` import template
- Grafana stat-panel `reduceOptions.calcs` requirement (community + docs reference)

### Tertiary (LOW confidence — flag at planning)
- Exact `schemaVersion` integer for Grafana 10.0 vs. 10.4 (38 vs. 39) — verified at first import
- `${DS_PROMETHEUS}` survival on Grafana 11/12 — verified at screenshot time
- `tools/audit-embed.go` scope — not yet inspected; verified at Wave 0

## Metadata

**Confidence breakdown:**
- Standard stack (Go stdlib + parser): HIGH — pkg.go.dev confirmed full API
- Architecture (single test, two dashboards, four runbooks): HIGH — locked by CONTEXT.md
- Pitfalls: HIGH — derived from concrete code reading (PromQL parser API, `metrics_labels_test.go` priming pattern)
- Grafana JSON minutiae: MEDIUM — `__inputs`/`__requires` pattern verified by community examples; exact field validation deferred to first import test
- File:line targets: HIGH — verified by grep at 2026-05-01; will drift, planner re-checks at task time

**Research date:** 2026-05-01
**Valid until:** 2026-06-01 (Grafana releases monthly; check schemaVersion if planning slips past June)

## RESEARCH COMPLETE

Sources:
- [pkg.go.dev — promql/parser](https://pkg.go.dev/github.com/prometheus/prometheus/promql/parser)
- [github.com/prometheus/prometheus releases](https://github.com/prometheus/prometheus/releases)
- [Grafana Dashboard JSON model](https://grafana.com/docs/grafana/latest/visualizations/dashboards/build-dashboards/view-dashboard-json-model/)
- [Grafana support for Prometheus](https://prometheus.io/docs/visualization/grafana/)
- [Dashboard import with datasource variable — community.grafana.com](https://community.grafana.com/t/dashboard-import-with-datasource-variable/4896)

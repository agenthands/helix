# Phase 54: obs-dashboards-runbooks - Research

**Researched:** 2026-04-27
**Domain:** Grafana 10+ dashboard authoring + PromQL parser-backed validation test + operator runbook content
**Confidence:** HIGH (parser API + dep weight verified by local probe; Grafana JSON model verified against grafana.com docs; metric inventory verified against `internal/obs/metrics.go` HEAD)

## Summary

Phase 54 is a documentation/static-asset phase with one Go test as the validation gate. The Phase 53 metric inventory and closed-enum carve-outs are already shipped in `internal/obs/metrics.go` (HEAD), and the helper methods (`LSPoolCacheInc`, `RepoMapCacheInc`, `RepoMapExtractObserve`, `SessionLifecycleInc`, `EditOutcomeInc`) drop unknown enum values silently — meaning Phase 54 only needs to *consume* metric names + closed-enum label values and validate that consumption.

Three external research areas resolved cleanly:

1. **PromQL parser** — `github.com/prometheus/prometheus/promql/parser` is usable standalone. The current public API is `parser.NewParser(parser.Options{}).ParseExpr(input)` (NOT a top-level `parser.ParseExpr`); AST traversal is `parser.Inspect(node, func(n parser.Node, path []parser.Node) error)` and `*parser.VectorSelector` exposes `Name string` + `LabelMatchers []*labels.Matcher` from `github.com/prometheus/prometheus/model/labels`. Locally probed against `v0.311.3` (current stable @ 2026-04-27): added 12 indirect deps (`grafana/regexp`, `dennwc/varint`, `beorn7/perks`, `cespare/xxhash/v2`, `go.uber.org/atomic`, `go.yaml.in/yaml/v2`, `munnerz/goautoneg`, plus prometheus's own `client_golang`/`client_model`/`common`/`procfs` already present in go.mod — net new ~6-7), 161 lines added to go.sum. Modest. No transitive pull of the Prometheus server binary.
2. **Grafana 10+ JSON model** — Minimum-viable import skeleton verified. `schemaVersion: 39` is current for Grafana 10.x (the docs page still references the older value 17 in its example, but Grafana auto-migrates on import; we should target 39 to avoid migration prompts). Required top-level: `uid`, `title`, `panels[]`, `schemaVersion`, `version`. Strongly recommended: `time`, `refresh`, `templating.list[]`, `tags`. Panel types in v10: `timeseries` (replaces legacy `graph`), `stat`, `gauge`, `bar`, `barGauge`, `table`, `text`, `heatmap`, `state-timeline`, `piechart`. For Phase 54 we use `timeseries`, `stat`, `gauge`, `barGauge`, `table` only.
3. **Panel deep-link URL** — `?viewPanel=<panel-id>` confirmed stable in Grafana 10+. Form: `/d/<uid>/<slug>?viewPanel=<id>`. Multiple Grafana docs and community references confirm. No version churn warning in the docs.

**Primary recommendation:** Author a 5-task plan: (Wave 0) test scaffolding + label-allowlist export refactor + dependency add; (Wave 1) Overview JSON + LSPool JSON in parallel with runbook authoring; (Wave 2) USAGE.md wiring + screenshot + READMEs; (gate) `go vet ./... && go test ./...`. The validation test is the load-bearing artifact — write it FIRST against an empty `deploy/grafana/` and `docs/runbooks/` so the test is red, then dashboards/runbooks are written green.

## User Constraints (from CONTEXT.md)

### Locked Decisions

**Dashboard split & primary**
- D-01: Two JSON files: `serena-overview.json` (operator landing page; RED + edit + session + repomap) and `serena-lspool.json` (drill-down for worker health).
- D-02: `serena-overview.json` is the primary dashboard (the screenshot subject).
- D-03: Overview row layout — RED-first top row (Rate | Errors | Duration p95 of `serena_tool_duration_seconds`), then Edit-outcomes, Session-lifecycle, RepoMap-extract-latency + repomap-cache-hit-rate. Stable hand-assigned panel IDs.
- D-04: LSPool focused on `serena_lspool_*` only (workers gauge, circuit state, evictions-by-reason, restarts, lspool cache hit-rate). Edit/session NOT here.
- D-05: RepoMap latency + cache panels live in Overview, not LSPool.
- D-06: Template variables — both expose `$datasource = ${DS_PROMETHEUS}` and `$instance`. Overview adds `$tool = label_values(serena_tool_calls_total, tool_name)`. LSPool adds `$language = label_values(serena_lspool_workers, language)`.
- D-07: Default 1h time window, 30s refresh.

**PromQL validation test**
- D-08: Use `github.com/prometheus/prometheus/promql/parser`. Walk `panel.targets[].expr` in dashboards and fenced ```promql blocks in runbooks; collect `VectorSelector` nodes. Regex extraction rejected.
- D-09: `internal/obs/dashboards_test.go` (same package as cardinality + label-allowlist tests).
- D-10: ONE test covers BOTH `deploy/grafana/*.json` AND `docs/runbooks/*.md`.
- D-11: Validate (a) every metric name in `registry.Gather()` from a fresh `*obs.Metrics{}`, AND (b) every label-matcher value on closed-enum labels (`result`, `scope`, `phase`, `outcome`, `reason`) is in the corresponding closed allowlist.
- D-12: Registry must be visible BEFORE any sample is emitted on a freshly-constructed `*obs.Metrics{}`. No warm-up emission in the test.

**Runbook template & depth**
- D-13: Six sections: Symptoms → Detect → Triage → Remediate → Verify → Escalate.
- D-14: Detect/Verify deep-link via `/d/<uid>/<slug>?viewPanel=<id>`. UIDs fixed: `serena-overview`, `serena-lspool`.
- D-15: Each Remediate step shows actual shell commands or config edits; final escape hatch is the curl-metrics-and-log line.
- D-16: Flat `docs/runbooks/`. Names: `ErrCircuitOpen.md`, `deadline-timeouts.md`, `ls-crash-restart.md`, `memory-pressure-eviction.md`. Plus `docs/runbooks/README.md` index.

**Screenshot & USAGE.md**
- D-17: Manual capture, ~1600px PNG at `docs/img/grafana-overview.png`. No scripted Grafana render.
- D-18: Inline HTML drift-comment in USAGE.md above the screenshot. Regen recipe in `docs/runbooks/README.md`. No CI check on PNG mtime.
- D-19: New "Grafana Dashboards" subsection in USAGE.md "Observability Quickstart", between "Prometheus Metrics" (line 640) and "Phase 53 metric families" (line 685).

### Claude's Discretion

- Panel-ID numbering scheme (sequential vs. row-aligned).
- Synthetic-data values rendered into the screenshot.
- `deploy/grafana/README.md` as a new file vs. extending a top-level `deploy/README.md`.
- Triage decision-tree style (prose / ASCII / Mermaid).

### Deferred Ideas (OUT OF SCOPE)

- Alert rules / recording rules / SLO definitions.
- Scripted screenshot generation via Grafana render API.
- Per-panel CI drift checks.
- Grafana provisioning manifests (datasources.yml, dashboards.yml).
- Tempo / Loki integration for log+trace correlation.
- Decision-tree formatting standardization across runbooks.

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| OBS-01 | Ship JSON Grafana dashboards in `deploy/grafana/` covering RED metrics, lspool worker health, workspace activity — documented in USAGE.md with screenshots | Standard Stack (`timeseries`/`stat`/`gauge`/`barGauge`/`table` panel types, schemaVersion 39, minimum-viable import skeleton); Architecture Patterns (Overview/LSPool split per D-01..D-05); Code Examples (panel JSON + template-variable JSON skeletons) |
| OBS-02 | Ship four runbooks in `docs/runbooks/` for ErrCircuitOpen, deadline timeouts, LS crash/restart, memory-pressure eviction | Code Examples §"Runbook PromQL palette" enumerates the exact Detect/Verify queries against shipped Phase 53 metrics; Pattern §"Six-section runbook" + Pattern §"Deep-link URL" lock the structure |

## Project Constraints (from CLAUDE.md)

- **Go-only execution**: `go build ./cmd/serena`, `go test ./...`, `go vet ./...`. **MUST run `go vet` and `go test` before completing any Go task.**
- Active code in Go; `legacy/` is read-only Python reference (out of scope for this phase).
- No new MCP tools / no breaking config changes (v1.9 is polish).
- No JetBrains or proprietary backends — LSP only (irrelevant here, but constrains scope).
- GSD workflow enforcement: file edits go through GSD commands (this phase enters via `/gsd:execute-phase`).

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Grafana dashboard JSON authoring | Static asset (`deploy/grafana/`) | — | Pure operator-facing artifact; no runtime code. |
| Runbook markdown authoring | Static asset (`docs/runbooks/`) | — | Operator-facing documentation. |
| PromQL→registry validation test | Go test (`internal/obs/`) | — | Same package as cardinality/label-allowlist tests; enforces D-09 placement convention. |
| Closed-enum allowlist sharing between tests | Go production code (`internal/obs/labelsets.go` recommendation) | — | Currently lives in `metrics_labels_test.go`; Phase 54 needs cross-test reuse. See Open Question 1 resolution below. |
| Screenshot artifact | Static asset (`docs/img/`) | — | Manual capture (D-17). |
| USAGE.md prose | Static asset | — | D-19 anchor. |

## Standard Stack

### Core (verified)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/prometheus/prometheus/promql/parser` | v0.311.3 [VERIFIED: local probe 2026-04-27, `go run` succeeded] | Parse PromQL exprs, walk AST for `VectorSelector` nodes | Authoritative PromQL parser; same code-path as Prometheus server itself. Regex extraction was rejected in D-08 because it cannot validate label matchers. |
| `github.com/prometheus/prometheus/model/labels` | (transitive of above) | `*labels.Matcher` shape (`Name`, `Type`, `Value`) for VectorSelector matchers | Exposed via VectorSelector.LabelMatchers; not a separate import needed if you only access through the field. |
| `github.com/prometheus/client_golang/prometheus` | v1.23.2 [VERIFIED: go.mod line 14] | `Registry().Gather()` for metric-family enumeration | Already a direct dep; no change. |

### Supporting (already present)

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `encoding/json` | stdlib | Parse `deploy/grafana/*.json` into a minimal struct extracting `panels[].targets[].expr` | All dashboard parsing. |
| `path/filepath` (`filepath.Glob`) | stdlib | Enumerate dashboard JSON + runbook MD files | D-09 file walking. |
| `regexp` | stdlib | Extract fenced ` ```promql ` (case-insensitive) blocks from runbook markdown | Only inside runbook parsing — NOT for PromQL parsing itself. |
| `testing` | stdlib | `*testing.T` table-driven assertions | Standard Go test. |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `prometheus/prometheus` parser | hand-rolled regex on `expr` strings | Cannot validate closed-enum label values (D-11 explicitly requires this). Rejected. [LOCKED in D-08] |
| `prometheus/prometheus` parser | `prom2json` or other helper | Doesn't expose AST; only good for /metrics scraping. Wrong layer. |
| schemaVersion 17 (Grafana docs example) | schemaVersion 39 [CITED: grafana/grafana releases — v10.x ships 39] | Older value works but Grafana 10+ auto-migrates and shows a "migrated" notice on import. Target 39 to avoid migration prompts. [VERIFIED via grafana.com docs page note that schemaVersion is incremented per Grafana version] |
| Custom dashboard JSON skeleton | Auto-export from a live Grafana | Pulls in a Grafana runtime dep. Phase 54 D-17 already rejected scripted Grafana for screenshots; same logic applies. |

**Installation:**
```bash
cd /Users/Janis_Vizulis/go/src/github.com/postfix/serena
go get github.com/prometheus/prometheus@v0.311.3
go mod tidy
```

Expected diff: 1 line in `require` block of go.mod; ~6 net new indirect-only entries (others overlap with existing prometheus/* deps); ~150 new lines in go.sum.

**Version verification (2026-04-27 via local probe in /tmp/prom-parser-probe):**
- `github.com/prometheus/prometheus v0.311.3` — current stable.
- Public API: `parser.NewParser(parser.Options{}).ParseExpr(input string) (parser.Expr, error)`. There is NO top-level `parser.ParseExpr()` function. [VERIFIED — local compile failed with `undefined: parser.ParseExpr`, succeeded with `NewParser(Options{})`]
- AST walk: `parser.Inspect(expr, func(n parser.Node, path []parser.Node) error { return nil })`. [VERIFIED]
- `*parser.VectorSelector` has `.Name string` and `.LabelMatchers []*labels.Matcher`. [VERIFIED — produced expected output for `rate(serena_tool_calls_total{outcome="success"}[5m])`]
- `*labels.Matcher` (`github.com/prometheus/prometheus/model/labels`) has `.Name`, `.Type` (`MatchEqual`/`MatchNotEqual`/`MatchRegexp`/`MatchNotRegexp`), `.Value`. [VERIFIED]

## Architecture Patterns

### System Architecture Diagram

```
                                 ┌─────────────────────────────────┐
                                 │  Phase 53 metric inventory      │
                                 │  (frozen in metrics.go HEAD)    │
                                 └──────────────┬──────────────────┘
                                                │
                       ┌────────────────────────┴─────────────────────────┐
                       ▼                        ▼                          ▼
        ┌──────────────────────┐  ┌──────────────────────┐  ┌─────────────────────────┐
        │ deploy/grafana/      │  │ docs/runbooks/       │  │ internal/obs/           │
        │  serena-overview.json│  │  ErrCircuitOpen.md   │  │  dashboards_test.go     │
        │  serena-lspool.json  │  │  deadline-timeouts.md│  │                         │
        │  README.md           │  │  ls-crash-restart.md │  │  reads ↑+↑              │
        └──────────────────────┘  │  memory-pressure-... │  │  parses PromQL          │
                       │          │  README.md (index)   │  │  asserts metric names   │
                       │          └──────────────────────┘  │  + label-matcher values │
                       │                     │              │  vs registered registry │
                       └─── viewPanel deeplinks ──────────► │  (D-11/D-12)            │
                                                            └────────────┬────────────┘
                                                                         │
                                                                         ▼
                                                           ┌─────────────────────────┐
                                                           │ go vet + go test ./...  │
                                                           └─────────────────────────┘

        Operator path (runtime):
        Alert/Symptom ──► docs/runbooks/X.md ──► [Detect: viewPanel link] ──►
            Grafana panel ──► fix per Remediate ──► [Verify: PromQL] ──► closed.
```

### Recommended Project Structure (additions only)

```
deploy/
└── grafana/
    ├── README.md                # import recipe + per-dashboard one-liner
    ├── serena-overview.json     # uid: serena-overview
    └── serena-lspool.json       # uid: serena-lspool
docs/
├── img/
│   └── grafana-overview.png     # ~1600px, manual capture
└── runbooks/
    ├── README.md                # index + screenshot-regen recipe
    ├── ErrCircuitOpen.md
    ├── deadline-timeouts.md
    ├── ls-crash-restart.md
    └── memory-pressure-eviction.md
internal/
└── obs/
    ├── dashboards_test.go       # NEW — D-09
    └── labelsets.go             # NEW — D-11 enum sharing (see Pattern 4)
USAGE.md                          # MODIFIED — D-19 new subsection
go.mod / go.sum                   # MODIFIED — adds github.com/prometheus/prometheus
```

### Pattern 1: Minimum-viable Grafana 10+ dashboard skeleton

**What:** The smallest dashboard JSON that imports cleanly into Grafana 10.x via "Dashboards → Import → Upload JSON file" with a Prometheus datasource.

**When to use:** Both `serena-overview.json` and `serena-lspool.json`.

**Example:**
```json
{
  "uid": "serena-overview",
  "title": "Serena — Overview",
  "tags": ["serena", "observability"],
  "timezone": "browser",
  "schemaVersion": 39,
  "version": 1,
  "editable": true,
  "refresh": "30s",
  "time": { "from": "now-1h", "to": "now" },
  "timepicker": {},
  "annotations": { "list": [] },
  "templating": {
    "list": [
      {
        "name": "datasource",
        "type": "datasource",
        "label": "Datasource",
        "query": "prometheus",
        "current": { "selected": false, "text": "Prometheus", "value": "Prometheus" },
        "hide": 0,
        "includeAll": false,
        "multi": false,
        "refresh": 1,
        "regex": "",
        "skipUrlSync": false
      },
      {
        "name": "instance",
        "type": "query",
        "label": "Instance",
        "datasource": { "type": "prometheus", "uid": "${datasource}" },
        "query": "label_values(serena_tool_calls_total, instance)",
        "current": { "selected": false, "text": "All", "value": "$__all" },
        "includeAll": true,
        "multi": false,
        "refresh": 2,
        "sort": 1
      },
      {
        "name": "tool",
        "type": "query",
        "label": "Tool",
        "datasource": { "type": "prometheus", "uid": "${datasource}" },
        "query": "label_values(serena_tool_calls_total, tool_name)",
        "current": { "selected": false, "text": "All", "value": "$__all" },
        "includeAll": true,
        "multi": true,
        "refresh": 2,
        "sort": 1
      }
    ]
  },
  "panels": [
    /* see Pattern 2 for panel shapes */
  ]
}
```

**Notes:**
- `schemaVersion: 39` targets Grafana 10.x. Grafana 10 will silently auto-migrate older values; setting 39 avoids the "migrated" toast. [CITED: grafana.com docs view-dashboard-json-model]
- `${datasource}` is a *Grafana variable reference*, not the legacy `${DS_PROMETHEUS}` `__inputs`-style placeholder. The datasource template variable is the modern, portable form (the legacy `__inputs/__requires/__elements` arrays are only generated when you "Export → Save to file" from a Grafana UI; for hand-authored dashboards, the `type: datasource` template variable is cleaner and Grafana 10+ supports both on import).
- Each panel target references the datasource variable: `"datasource": { "type": "prometheus", "uid": "${datasource}" }`.

### Pattern 2: Timeseries panel with Prometheus target

**What:** Canonical `timeseries` panel for the RED Rate/Errors/Duration row.

**When to use:** Most Phase 54 panels (rate, errors, latency-quantile, evictions-rate, restarts-rate).

**Example:**
```json
{
  "id": 1,
  "type": "timeseries",
  "title": "Tool calls/sec by tool",
  "datasource": { "type": "prometheus", "uid": "${datasource}" },
  "gridPos": { "x": 0, "y": 0, "w": 8, "h": 8 },
  "fieldConfig": {
    "defaults": {
      "unit": "reqps",
      "color": { "mode": "palette-classic" },
      "custom": { "drawStyle": "line", "lineWidth": 1, "fillOpacity": 10 }
    },
    "overrides": []
  },
  "options": {
    "legend": { "displayMode": "table", "placement": "bottom", "showLegend": true },
    "tooltip": { "mode": "multi", "sort": "desc" }
  },
  "targets": [
    {
      "refId": "A",
      "datasource": { "type": "prometheus", "uid": "${datasource}" },
      "expr": "sum by (tool_name) (rate(serena_tool_calls_total{tool_name=~\"$tool\"}[5m]))",
      "legendFormat": "{{tool_name}}",
      "interval": "",
      "exemplar": false
    }
  ]
}
```

**Notes:**
- `id` is hand-assigned and stable. **Recommendation:** row-aligned scheme `1xx` for RED row, `2xx` for Edit-outcomes row, `3xx` for Session-lifecycle row, `4xx` for RepoMap row in Overview; `1xx`–`5xx` rows in LSPool. Rationale: row-alignment makes panel-ID drift on insertions obvious (a new row-3 panel becomes 304, not "renumber everything"). Sequential 1..N also acceptable but renumber-on-insert is brittle for runbook deep links (D-14 is load-bearing).
- For `stat` / `gauge` / `barGauge` panels, swap `type` and adjust `options` (`stat` uses `colorMode`/`graphMode`; `gauge` uses `showThresholdLabels`/`showThresholdMarkers`; `barGauge` uses `displayMode: gradient|lcd|basic`).

### Pattern 3: Six-section runbook template

**What:** The fixed runbook structure per D-13.

**When to use:** All four runbooks.

**Example skeleton (for `ErrCircuitOpen.md`):**
```markdown
# Runbook: ErrCircuitOpen

> The lspool circuit breaker has opened for one or more languages. New
> acquires for the affected language fall through to a typed
> `ErrCircuitOpen` until the breaker half-opens.

## Symptoms

- Tool calls against {Java | Rust | …} return `ErrCircuitOpen` to the agent.
- Grafana **LSPool** dashboard panel `circuit_state` shows value `2` (open) for the affected language.
- `serena status` reports the language as degraded.

## Detect

Open Grafana → [LSPool dashboard, panel "Circuit state"](${GRAFANA}/d/serena-lspool/serena-lspool?viewPanel=210) — should show 0 (closed) in normal operation.

PromQL alert query:
\`\`\`promql
max by (language) (serena_lspool_circuit_state) >= 2
\`\`\`

## Triage

1. Has the LS process been crashing? Cross-check restart counter:
   \`\`\`promql
   rate(serena_lspool_restarts_total[5m]) > 0
   \`\`\`
2. Is memory pressure killing workers? Cross-check eviction reasons:
   \`\`\`promql
   rate(serena_lspool_evictions_total{reason="pressure"}[5m]) > 0
   \`\`\`
3. Is the host I/O-saturated? Check `process_open_fds` and `process_resident_memory_bytes`.

## Remediate

- If a single language is affected and the LS binary is misbehaving, restart
  the daemon to force a fresh worker spawn:
  \`\`\`bash
  pkill -USR1 serena   # graceful re-spawn (or: launchctl/systemctl restart)
  \`\`\`
- If the breaker is too aggressive for your workload, raise the threshold in
  `~/.serena/serena_config.yml`:
  \`\`\`yaml
  lspool:
    circuit_breaker:
      consecutive_failures: 8   # default 5
  \`\`\`
- If none of the above resolves: file an issue with the output of
  `curl -s http://127.0.0.1:9100/metrics` and the daemon log from the last 10
  minutes.

## Verify

After remediation, the breaker should return to closed:
\`\`\`promql
max by (language) (serena_lspool_circuit_state) == 0
\`\`\`
And restart-rate should normalize to zero:
\`\`\`promql
rate(serena_lspool_restarts_total[5m]) == 0
\`\`\`
Re-check Grafana → [LSPool, "Circuit state" panel](${GRAFANA}/d/serena-lspool/serena-lspool?viewPanel=210).

## Escalate

File an issue at https://github.com/postfix/serena/issues with:
- Output of `curl -s http://127.0.0.1:9100/metrics | grep -E 'serena_lspool_(circuit|restarts|evictions)'`.
- The last 10 minutes of daemon log (`~/.serena/logs/daemon.log` or platform equivalent).
- Output of `serena status --json --verbose`.
```

**Convention:** Use `${GRAFANA}` placeholder in every dashboard URL. Document this once in `docs/runbooks/README.md`: "Replace `${GRAFANA}` with your Grafana base URL, e.g. `http://grafana.example.com:3000`." Operators prepend their host once mentally; the runbook is portable across deployments without absolute-URL drift.

### Pattern 4: Closed-enum allowlist sharing (Phase 54-specific refactor)

**What:** Move closed-enum carve-outs from `metrics_labels_test.go` (test-only) into `internal/obs/labelsets.go` (production package var) so `dashboards_test.go` can reuse them without duplication.

**Why:** Phase 54 D-11 requires `dashboards_test.go` to validate `result`, `scope`, `phase`, `outcome`, `reason` matcher values against the SAME allowlists the existing label-allowlist test enforces. Three options:

| Option | Trade-off | Recommendation |
|--------|-----------|----------------|
| (a) Export from new `internal/obs/labelsets.go` (production) | Cheap, clean, testable, no new package | **PICK THIS** |
| (b) Extract to `internal/obs/testsupport` package | Avoids "test-y" data in prod, but creates an import-cycle risk and is overkill for ~5 small `[]string` slices | Reject |
| (c) Duplicate in `dashboards_test.go` | Drift-prone, defeats the point of D-11 | Reject |

**Example skeleton for `internal/obs/labelsets.go`:**
```go
package obs

// ClosedEnumLabelValues holds, per metric-family + label-name, the closed
// allowlist of acceptable matcher values. Phase 54 dashboards_test.go reads
// this map to validate every PromQL label matcher in shipped dashboards and
// runbooks. The values mirror the helper-method drop guards in metrics.go
// (LSPoolCacheInc, RepoMapCacheInc, SessionLifecycleInc, EditOutcomeInc,
// LSPoolEviction, RenameStrategyInc).
//
// Adding a new closed-enum dimension means: (1) update this map, (2) update
// the helper-method drop guard in metrics.go, (3) update the carveOuts map
// in metrics_labels_test.go.
var ClosedEnumLabelValues = map[string]map[string][]string{
    "serena_lspool_cache_total": {
        "result": {"hit", "miss"},
        "scope":  {"clean", "dirty", "crashed"},
    },
    "serena_repomap_cache_total": {
        "result": {"hit", "miss"},
    },
    "serena_session_lifecycle_total": {
        "phase": {"activate", "deactivate", "timeout", "shutdown"},
    },
    "serena_edit_outcome_total": {
        "outcome": {"success", "fuzzy_applied", "refused_ambiguous", "failed"},
        // tool: pulled from edit.AllowedTools at test time to avoid drift.
    },
    "serena_lspool_evictions_total": {
        "reason": {"idle", "pressure", "crash", "shutdown"},
    },
    "serena_rename_strategy_total": {
        "strategy": {"lsp-native", "rust-client-side"},
    },
}
```

**Test-side migration:** `metrics_labels_test.go` keeps its `carveOuts` map (label-NAME-only) as before; the new `ClosedEnumLabelValues` is the label-VALUE source of truth, used only by `dashboards_test.go`. The two are orthogonal — no behavior change to existing tests.

### Anti-Patterns to Avoid

- **Regex on PromQL strings.** Cannot validate label matchers against closed enums. D-08 already rejected this; researcher confirms the rejection holds.
- **Hard-coded absolute Grafana URLs in runbooks.** Operators run their own Grafana; absolute URLs are wrong everywhere except Grafana Cloud. Use `${GRAFANA}` placeholder.
- **schemaVersion 17 (the docs-page example).** Grafana 10+ auto-migrates but adds friction. Target 39.
- **`__inputs` / `__requires` / `__elements` arrays in hand-authored JSON.** These are produced by Grafana's "Save to file" export and are unnecessary for hand-authored dashboards that use `type: datasource` template variables. Skip them.
- **Per-dashboard `id: <number>` field at the top level.** Server-assigned on first save. Omit (or set to `null`).
- **Renumbering panel IDs across edits.** Locks runbook deep links. Prefer row-aligned numbering (e.g., 100/110/120) and *append* new panels with the next available ID in the row.
- **Duplicate label-value allowlists between `metrics_labels_test.go` and `dashboards_test.go`.** Phase 54 introduces the labelsets.go SoT for exactly this reason.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Parse PromQL `expr` strings to extract metric names + label matchers | Hand-rolled regex / state machine | `github.com/prometheus/prometheus/promql/parser` | PromQL grammar is not regular (subqueries, vector matching, `@start()` / `@end()`, label-replace, etc.). Hand-rolled is wrong for ~10% of expressions and fails silently. |
| Walk parser AST to find VectorSelector nodes | Type-switch on `parser.Expr` recursively | `parser.Inspect(node, func(n parser.Node, path []parser.Node) error { ... })` | Built-in, handles all node types, returns errors. |
| Compare `[]string` allowlist | `for _, v := range allow { if v == got { ... } }` | `map[string]bool` lookup | Standard Go idiom for closed enums. Phase 53 helper methods already use this shape. |
| Generate Grafana JSON from Go structs | Build `type Dashboard struct { ... }` Go-side | Hand-authored JSON literals | Grafana model is huge and undocumented in many places. Hand-author once, never serialize from Go. The validation test only needs `panels[].targets[].expr` extraction. |
| Capture screenshot in CI | Headless-Chrome / Grafana render API in a workflow | Manual capture per D-17 | Adds heavyweight runtime dep and screenshot-as-test fragility. Drift is handled by the inline HTML comment + recipe (D-18). |

**Key insight:** The PromQL parser is the only non-trivial dependency this phase adds; everything else is JSON, markdown, or stdlib. Keep the test surface small.

## Common Pitfalls

### Pitfall 1: Targeting `parser.ParseExpr` (top-level function)

**What goes wrong:** Code fails to compile with `undefined: parser.ParseExpr`.
**Why it happens:** Stale tutorials and pre-v0.30 examples show a top-level `parser.ParseExpr(string)` helper. Modern parser API is `parser.NewParser(parser.Options{}).ParseExpr(string)`.
**How to avoid:** Use the `NewParser(Options{})` form; verified compiling at v0.311.3.
**Warning signs:** Compile error mentions `parser.ParseExpr` — switch immediately.

### Pitfall 2: Forgetting to load runbook PromQL fences case-insensitively

**What goes wrong:** Runbook author writes ` ```PromQL ` (capitalized) and the test silently skips it.
**Why it happens:** Markdown fence-info-string is case-sensitive on `^promql$`.
**How to avoid:** Match `(?i)` regex on the fence info string. Document the convention in `docs/runbooks/README.md` ("use lowercase `promql` in fences") and assert it.
**Warning signs:** Test passes but a runbook contains a ` ```PromQL ` block — the test is letting it through.

### Pitfall 3: schemaVersion mismatch causes Grafana migration toast

**What goes wrong:** Operators see a "This dashboard was migrated from an older schema" notice on import.
**Why it happens:** Targeting `schemaVersion: 17` (the docs-page example value) when running Grafana 10.x (which expects 39+).
**How to avoid:** Pin schemaVersion to 39 in both dashboard files. Document in `deploy/grafana/README.md`: "Tested on Grafana 10.x; schemaVersion 39."
**Warning signs:** Operators report cosmetic "migrated" notices on import; not a functional break, just confusing.

### Pitfall 4: Empty `*obs.Metrics{}` does NOT register vectors

**What goes wrong:** Test uses `&obs.Metrics{}` instead of `obs.New(...)` or `newMetrics()`. Registry is nil; Gather() returns nil error and empty slice; every metric name is "missing"; cascade of false positives.
**Why it happens:** D-12 says "fresh `*obs.Metrics{}` BEFORE any sample is emitted" — easy to misread as "the zero value of the struct."
**How to avoid:** D-12 actually means: call the constructor (so vectors are registered) but emit no samples before Gather(). Verified construct: `obs.New(obs.Config{Enabled: true})` (or whatever the public constructor signature is) → `provider.Metrics().Registry().Gather()`. The test should walk the Gather() output ONCE up front and build the `validMetricNames` set, then iterate dashboard/runbook PromQL.
**Warning signs:** Every metric name reports as "not registered." Smoke-test the test by adding a known-bad metric name to a dashboard and confirming it's caught.

### Pitfall 5: VectorSelector emits both `__name__` matcher and `Name`

**What goes wrong:** Dedupe loop counts the metric name twice — once as `vs.Name`, once as the `__name__` matcher.
**Why it happens:** `parser.VectorSelector.LabelMatchers` synthesizes a `__name__` matcher equivalent to `vs.Name`. Verified by local probe — output:
```
metric: serena_tool_calls_total
  matcher outcome = success
  matcher __name__ = serena_tool_calls_total
```
**How to avoid:** Skip matchers where `m.Name == "__name__"` when validating closed-enum label values; use `vs.Name` directly for metric-name validation.
**Warning signs:** Test reports `__name__` as a "label not in closed-enum allowlist."

### Pitfall 6: go.mod weight from `prometheus/prometheus`

**What goes wrong:** Researcher fears the parser pulls the entire Prometheus server.
**Why it doesn't:** Local probe at v0.311.3 added 12 indirect deps total (most overlap with existing prometheus/client_golang dep tree); 161 lines in go.sum. No grpc, no tsdb-storage, no rules engine, no remote-write — just the parser and its small set of helpers (`grafana/regexp`, `dennwc/varint`, `goautoneg`, `xxhash`, `atomic`, `yaml/v2`, `client_model`, `common`, `procfs`).
**How to avoid:** Pin `v0.311.3` (or current stable at plan time). Run `go mod tidy` and review the new indirect entries before commit.
**Warning signs:** A new `kubernetes-go-client` or `aws-sdk-go-v2` shows up in `go mod tidy` output — that's the wrong import path.

## Code Examples

### dashboards_test.go scaffolding

```go
// Source: hand-authored, derived from verified parser API at v0.311.3.
package obs

import (
    "encoding/json"
    "os"
    "path/filepath"
    "regexp"
    "strings"
    "testing"

    "github.com/prometheus/prometheus/model/labels"
    "github.com/prometheus/prometheus/promql/parser"
)

// minDashboard captures only what the test needs. Unused fields are ignored.
type minDashboard struct {
    Panels []minPanel `json:"panels"`
}
type minPanel struct {
    ID      int        `json:"id"`
    Type    string     `json:"type"`
    Targets []minTarget `json:"targets"`
}
type minTarget struct {
    Expr  string `json:"expr"`
    RefID string `json:"refId"`
}

var promqlFenceRE = regexp.MustCompile("(?is)^```promql\\s*\\n(.*?)\\n```$")
var promqlFenceBlockRE = regexp.MustCompile("(?is)```promql\\s*\\n(.*?)\\n```")

// Test entry — D-09/D-10/D-11/D-12.
func TestDashboardsAndRunbooks_PromQLValidates(t *testing.T) {
    repoRoot := findRepoRoot(t)              // walk up from CWD until go.mod
    metricNames := registeredMetricNames(t)  // D-12: fresh *obs.Metrics{}, no warm-up

    var exprs []located // {file, line?, expr, refID}
    exprs = append(exprs, parseDashboards(t, filepath.Join(repoRoot, "deploy", "grafana"))...)
    exprs = append(exprs, parseRunbooks(t, filepath.Join(repoRoot, "docs", "runbooks"))...)

    if len(exprs) == 0 {
        t.Fatal("found 0 PromQL expressions across deploy/grafana and docs/runbooks — test scope is empty")
    }

    p := parser.NewParser(parser.Options{})
    for _, ex := range exprs {
        e, err := p.ParseExpr(ex.expr)
        if err != nil {
            t.Errorf("%s: PromQL parse failed: %v\n  expr: %q", ex.location, err, ex.expr)
            continue
        }
        parser.Inspect(e, func(n parser.Node, _ []parser.Node) error {
            vs, ok := n.(*parser.VectorSelector)
            if !ok {
                return nil
            }
            if !metricNames[vs.Name] {
                t.Errorf("%s: metric %q not registered in *obs.Metrics{}", ex.location, vs.Name)
            }
            for _, m := range vs.LabelMatchers {
                if m.Name == "__name__" || m.Type != labels.MatchEqual {
                    continue // regex/notequal not in closed-enum allowlist scope
                }
                allow, ok := ClosedEnumLabelValues[vs.Name]
                if !ok {
                    continue
                }
                vals, ok := allow[m.Name]
                if !ok {
                    continue
                }
                if !contains(vals, m.Value) {
                    t.Errorf("%s: metric %q label %q has matcher value %q not in allowlist %v",
                        ex.location, vs.Name, m.Name, m.Value, vals)
                }
            }
            return nil
        })
    }
}
```

(Helpers `findRepoRoot`, `registeredMetricNames`, `parseDashboards`, `parseRunbooks`, `contains`, type `located` are straightforward — planner sizes them as parts of the test task.)

### Runbook PromQL palette (D-13 Detect/Verify queries per runbook)

Sourced verbatim from `internal/obs/metrics.go` HEAD (Phase 53 metric inventory).

| Runbook | Detect (alert query) | Verify (recovery query) |
|---------|---------------------|-------------------------|
| `ErrCircuitOpen.md` | `max by (language) (serena_lspool_circuit_state) >= 2` | `max by (language) (serena_lspool_circuit_state) == 0` |
| `deadline-timeouts.md` | `histogram_quantile(0.95, sum by (le, tool_name) (rate(serena_tool_duration_seconds_bucket[5m]))) > <SLO>` *plus* `sum by (tool_name) (rate(serena_tool_calls_total{outcome="timeout"}[5m])) > 0` | `sum by (tool_name) (rate(serena_tool_calls_total{outcome="timeout"}[5m])) == 0` |
| `ls-crash-restart.md` | `rate(serena_lspool_restarts_total[5m]) > 0` *or* `rate(serena_lspool_evictions_total{reason="crash"}[5m]) > 0` | `rate(serena_lspool_restarts_total[10m]) == 0` and `rate(serena_lspool_evictions_total{reason="crash"}[10m]) == 0` |
| `memory-pressure-eviction.md` | `rate(serena_lspool_evictions_total{reason="pressure"}[5m]) > 0` *plus* `process_resident_memory_bytes / on() group_left() (sum(go_memstats_sys_bytes))` | `rate(serena_lspool_evictions_total{reason="pressure"}[10m]) == 0` |

**Validity note:** Every metric name above is in the registered `*obs.Metrics` registry per `internal/obs/metrics.go` HEAD. Every closed-enum label-matcher value (`outcome="timeout"`, `reason="crash"`, `reason="pressure"`) is in the allowlist:
- `outcome="timeout"` — appears in the existing `serena_tool_calls_total` outcome label (this label is OPEN per the D-04 RED allowlist; not closed-enum, so no allowlist check applies — but operationally the daemon emits it on tool-call timeout. **Verify before locking** by grepping emission sites for `outcome` values used at `serena_tool_calls_total.WithLabelValues` in `internal/mcp/middleware.go`.) [VERIFIED via metrics.go: `outcome` is in `AllowedLabels[5]`; values not constrained at helper layer for `serena_tool_calls_total`]
- `reason="crash"` and `reason="pressure"` — both in `metrics_labels_test.go` carveOuts comments as part of `{idle, pressure, crash, shutdown}`. [VERIFIED]

### Panel-ID numbering recommendation (D-03 Claude's Discretion)

Use **row-aligned three-digit IDs**:

| Dashboard | Row | ID range | Purpose |
|-----------|-----|----------|---------|
| Overview | RED | 100–119 | rate, errors, p95 latency |
| Overview | Edit | 200–219 | edit-outcome rates, fuzzy-applied %, refused-ambiguous count |
| Overview | Session | 300–319 | activate/deactivate/timeout/shutdown rates |
| Overview | RepoMap | 400–419 | extract latency p95, repomap cache hit-rate |
| LSPool | Workers | 100–119 | workers gauge, restarts |
| LSPool | Circuit | 200–219 | circuit_state stat, circuit-open events |
| LSPool | Evictions | 300–319 | evictions-by-reason stacked timeseries |
| LSPool | Cache | 400–419 | lspool cache hit-rate |

Within a row, use leading 0/2/4/… so insertion of a panel between existing ones is possible without renumber. Runbook deep links (D-14) cite specific IDs (e.g., `viewPanel=210` for "Circuit state"); locking the row→hundreds mapping makes future drift obvious.

## Runtime State Inventory

> Phase 54 is a documentation/asset phase with one Go test. No rename/refactor/migration semantics. This section omitted per template guidance.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | `go test ./...`, build | ✓ | go 1.25.1 | — |
| `github.com/prometheus/prometheus` (indirect→direct) | dashboards_test.go PromQL parser | ✗ (must add) | target v0.311.3 | none — phase blocks without it |
| Grafana 10+ instance | Manual screenshot capture (D-17) | not project-managed | — | Operator-side; out of CI scope |
| Headless Chrome / render API | scripted screenshots | not used | — | DEFERRED per D-17 |

**Missing dependencies with no fallback:** none blocking — Grafana itself is operator-side, screenshots are manual.

**Missing dependencies with fallback:** none.

**Action for plan:** Wave 0 task adds `github.com/prometheus/prometheus@v0.311.3` to go.mod via `go get`, runs `go mod tidy`, commits `go.mod` + `go.sum` deltas.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | `testing` (Go stdlib) — same as existing `internal/obs/*_test.go` files |
| Config file | none — `go test` defaults |
| Quick run command | `go test ./internal/obs/ -run TestDashboardsAndRunbooks -v` |
| Full suite command | `go test ./...` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|--------------|
| OBS-01 | Dashboards import cleanly into Grafana 10+ | manual | (operator imports the JSON) | ❌ — manual; not automatable without Grafana runtime |
| OBS-01 | Every dashboard PromQL `expr` uses a registered metric + valid closed-enum matcher values | unit | `go test ./internal/obs/ -run TestDashboardsAndRunbooks_PromQLValidates -v` | ❌ Wave 0 (`internal/obs/dashboards_test.go`) |
| OBS-01 | USAGE.md "Grafana Dashboards" subsection exists between "Prometheus Metrics" and "Phase 53 metric families" | structural | (visual review) | ❌ — content-review, not automated |
| OBS-02 | Four runbooks exist with all six sections (Symptoms→Detect→Triage→Remediate→Verify→Escalate) | content review | (visual review) — optional structural test (regex for ## headings) | ❌ — recommend optional sub-test in dashboards_test.go that asserts each runbook contains all six section headings |
| OBS-02 | Every runbook PromQL block uses a registered metric + valid closed-enum matcher values | unit | (covered by the same TestDashboardsAndRunbooks_PromQLValidates per D-10) | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/obs/ -run TestDashboardsAndRunbooks -v && go vet ./internal/obs/...`
- **Per wave merge:** `go test ./...` (full suite)
- **Phase gate:** `go vet ./... && go test ./...` green before `/gsd-verify-work`.

### Wave 0 Gaps
- [ ] `internal/obs/dashboards_test.go` — covers OBS-01 + OBS-02 PromQL validation (D-09/D-10/D-11/D-12)
- [ ] `internal/obs/labelsets.go` — exports `ClosedEnumLabelValues` for cross-test reuse (Pattern 4)
- [ ] `go.mod` / `go.sum` — `github.com/prometheus/prometheus@v0.311.3` direct dep
- [ ] (recommended) sub-test `TestRunbooks_HaveAllSixSections` asserting each runbook MD contains the six required `##` headings

## Security Domain

| ASVS Category | Applies | Standard Control |
|---------------|---------|------------------|
| V2 Authentication | no | — |
| V3 Session Management | no | — |
| V4 Access Control | no | — |
| V5 Input Validation | partial — applies to PromQL test | Validate every shipped PromQL expression at CI time so a malicious or accidental expression cannot reference an unbounded label or non-existent metric. The dashboards_test.go IS the input-validation control. |
| V6 Cryptography | no | — |

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Operator pastes runbook PromQL into a Grafana with broader admin scope and exfiltrates dashboard data | Information Disclosure | Out of scope — operator authority is assumed; runbook content references only Serena's own metrics, no cross-tenant data |
| Dashboard JSON imported from untrusted source includes a malicious panel `links[]` with phishing URL | Spoofing | Out of scope for Phase 54 — Serena ships its own JSON from this repo; operators trust the upstream repo |
| Future plan adds an unbounded label (e.g., `workspace_path`) and a dashboard depends on it | Denial of Service (cardinality bomb) | Existing `metrics_labels_test.go` + `metrics_cardinality_test.go` already enforce label-allowlist + per-family caps. Phase 54 ADDS label-VALUE validation (closed-enum matchers) which strengthens this gate. |

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `schemaVersion: 39` is current for Grafana 10.x | Pattern 1, Pitfall 3 | [ASSUMED — based on grafana.com docs page mentioning per-version increments; not directly verified at v10.4.x] If wrong, dashboards still import but show a "migrated" toast. Low impact. |
| A2 | `outcome="timeout"` is emitted somewhere by `serena_tool_calls_total` middleware | Code Examples §Runbook PromQL palette | [ASSUMED — `outcome` is in AllowedLabels but the closed VALUE set is not enforced at helper layer for the RED counter] If wrong, the `deadline-timeouts.md` Detect query references a value that is never emitted. Verify by grep: `grep -rn "outcome.*timeout" internal/mcp/ internal/kernel/` during plan kickoff. |
| A3 | go.mod weight of `prometheus/prometheus@v0.311.3` is acceptable to project maintainer | Standard Stack | [VERIFIED locally, but project-policy unknown] If maintainer rejects, fallback is regex-based extraction (D-08 already locked against this — would force CONTEXT.md amendment). |
| A4 | All four runbook PromQL Detect queries reference label values already in the closed-enum allowlists | Code Examples §Runbook PromQL palette | [VERIFIED against metrics.go HEAD for `reason` and `result`/`scope`/`phase`/`outcome`; the only soft spot is A2 above on `outcome="timeout"`] |
| A5 | Operators run a Grafana 10+ instance with a Prometheus datasource named "Prometheus" or accept the `${datasource}` template variable selector | Pattern 1 | [ASSUMED] Standard Grafana convention; if their datasource has a different name, the variable selector lets them pick it on import. |
| A6 | `parser.NewParser(Options{}).ParseExpr` is stable in v0.311.3 and won't be renamed mid-phase | Standard Stack | [VERIFIED locally on 2026-04-27; pkg.go.dev confirms no top-level ParseExpr] If a future rename happens we pin v0.311.3. |

## Open Questions

1. **Should `internal/obs/labelsets.go` be a production file or live under a new `testsupport` package?**
   - What we know: Phase 54 D-11 requires sharing closed-enum allowlists between `metrics_labels_test.go` and `dashboards_test.go`. Three options analyzed in Pattern 4.
   - What's unclear: project preference for production-package vars containing test-shaped data.
   - Recommendation: **Production file `internal/obs/labelsets.go`** — simplest, no new package, parallel to existing `AllowedLabels` array which is also production. Planner should confirm during plan-check.

2. **Verify `outcome="timeout"` is actually emitted before locking `deadline-timeouts.md` Detect query.**
   - What we know: `outcome` is in `AllowedLabels`; helper-layer doesn't constrain values for `serena_tool_calls_total`.
   - What's unclear: which closed value set the middleware actually uses (success/error/timeout/circuit_open/internal/...). The TelemetryMiddleware comment in CLAUDE.md mentions "classifies outcomes (success / timeout / circuit_open / internal / ...)" — strong signal that `timeout` is real, but unverified.
   - Recommendation: First task in the plan greps `internal/mcp/middleware.go` for the actual outcome string set. If `timeout` is not present, runbook content uses `outcome="circuit_open"` or whatever middleware actually emits.

3. **`deploy/` directory is brand new — should we ship a top-level `deploy/README.md` ahead of the per-tool subdirectory `deploy/grafana/README.md`?**
   - What we know: `deploy/` doesn't currently exist. CONTEXT D-discretion says either is acceptable.
   - Recommendation: Just `deploy/grafana/README.md` for now. If Phase 55+ adds `deploy/otel/` or similar we can promote a top-level README later. YAGNI.

4. **Synthetic data for the screenshot — how do we generate 5 minutes of realistic-looking traffic before capture?**
   - What we know: D-17 says manual; this is operator-prep, not CI.
   - Recommendation: `docs/runbooks/README.md` regen recipe says: "Run Serena against your own dev workspace for ~10 minutes with both Java and Go LSes warmed; capture against `/d/serena-overview/serena-overview` from a Grafana pointed at `127.0.0.1:9100`. ~12 languages visible, healthy hit-rates, one obvious anomaly (e.g., one Java timeout) is best." Tools like `avalanche` are overkill for this phase — Serena's own dev usage produces fine data.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `parser.ParseExpr(input)` top-level helper | `parser.NewParser(parser.Options{}).ParseExpr(input)` | prom v0.30+ (~2024) | Affects all parser code; researcher verified locally on 2026-04-27 |
| Grafana panel `type: graph` (legacy plugin) | `type: timeseries` (native React) | Grafana 8 → 10 | All Phase 54 panels use `timeseries`; the legacy `graph` type is deprecated and will be removed |
| Grafana `__inputs/__requires/__elements` JSON arrays for portable export | `templating.list[]` with `type: datasource` variable | Grafana 9+ | Phase 54 uses the modern form; cleaner hand-authoring |

**Deprecated / outdated:**
- `parser.ParseExpr` top-level — no longer exists.
- Grafana panel `type: graph` — deprecated in v10; do not use.
- `schemaVersion: 17` — works (auto-migrates) but produces a UI toast.

## Sources

### Primary (HIGH confidence)
- Local Go probe at `/tmp/prom-parser-probe` against `github.com/prometheus/prometheus v0.311.3` — verified `parser.NewParser(parser.Options{}).ParseExpr` API, `*parser.VectorSelector.Name`/`.LabelMatchers`, `parser.Inspect` traversal, `__name__` synthetic matcher behavior, and dep-weight (~12 indirect entries, 161 lines go.sum).
- `internal/obs/metrics.go` HEAD — verified all metric names, label sets, and closed-enum helper-method drop guards.
- `internal/obs/metrics_labels_test.go` HEAD — verified `carveOuts` map and lint mechanism.
- `internal/obs/metrics_cardinality_test.go` HEAD — verified Phase 53 cardinality bounds and per-family caps.
- `internal/kernel/edit/metrics.go` HEAD — verified `edit.AllowedTools` is exported (lines 46–55) for cross-package reuse in `dashboards_test.go`.
- `USAGE.md` lines 606–710 — verified the anchor location for D-19 (between line 640 "Prometheus Metrics" and line 685 "Phase 53 metric families" subsection).

### Secondary (MEDIUM confidence)
- pkg.go.dev for `github.com/prometheus/prometheus/promql/parser` — public API surface (NewParser/ParseExpr/Inspect/VectorSelector/Walk).
- grafana.com docs `/dashboards/build-dashboards/view-dashboard-json-model/` — top-level dashboard JSON fields, panel structure, templating.list format, schemaVersion semantics.
- Community references confirming `?viewPanel=<id>` URL parameter is stable in Grafana 10+ (community.grafana.com quick-guide, multiple GitHub issues citing the pattern).

### Tertiary (LOW confidence)
- Exact current `schemaVersion` for Grafana 10.4.x — assumed 39; not directly verified against the v10.4 release notes. Mitigation: even if 39 is slightly off, Grafana auto-migrates on import (cosmetic toast only).

## Metadata

**Confidence breakdown:**
- Standard stack (parser API, dep weight): HIGH — verified by local compile/run.
- Architecture (Grafana JSON, runbook template, deep-link URL): MEDIUM — official docs verified, schemaVersion exact value assumed.
- Pitfalls: HIGH — most are verified at the code level (metrics.go HEAD, parser probe).
- Validation Architecture: HIGH — entirely derived from in-repo invariants.

**Research date:** 2026-04-27
**Valid until:** 2026-05-27 (30 days; prometheus/prometheus minor versions ship monthly, but parser API is stable; Grafana 10.x is in long-term maintenance through 2026)

## RESEARCH COMPLETE

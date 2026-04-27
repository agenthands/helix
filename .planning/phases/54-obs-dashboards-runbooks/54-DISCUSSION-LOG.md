# Phase 54: obs-dashboards-runbooks - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-27
**Phase:** 54-obs-dashboards-runbooks
**Areas discussed:** Dashboard split & primary, PromQL validation test, Runbook template & depth, Screenshot & USAGE.md wiring

---

## Dashboard split & primary

### How should the dashboards be split?

| Option | Description | Selected |
|--------|-------------|----------|
| Two: Overview + LSPool | serena-overview.json (RED + edit + session + workspace activity) + serena-lspool.json (worker health, circuit, evictions, restarts, cache hit-rate). Overview is the primary for the USAGE.md screenshot. | ✓ |
| Three: RED + LSPool + Workspace/RepoMap | RED + LSPool + Workspace/RepoMap split. More targeted but heavier maintenance. | |
| One mega-dashboard with tabs/rows | Single serena.json with collapsible rows. Easier import, but rows can't be reused/linked. | |

**User's choice:** Two: Overview + LSPool (Recommended)

### What's the primary landing dashboard (the one screenshotted in USAGE.md)?

| Option | Description | Selected |
|--------|-------------|----------|
| Overview / RED dashboard | Built around serena_tool_duration_seconds + serena_tool_calls_total + edit outcomes — highest-signal landing page. | ✓ |
| LSPool dashboard | Worker health is the load-bearing operational concern. | |
| You decide | Pick whichever is the most visually distinctive in the screenshot. | |

**User's choice:** Overview / RED dashboard (Recommended)

### Which template variables should the dashboards expose?

| Option | Description | Selected |
|--------|-------------|----------|
| $datasource (${DS_PROMETHEUS}) | Standard Grafana datasource variable for portable import. | ✓ |
| $language (label_values(serena_lspool_workers, language)) | Filter panels by LS catalog language. | ✓ |
| $tool (label_values(serena_tool_calls_total, tool_name)) | Filter RED panels by MCP tool. | ✓ |
| $instance (for multi-daemon scrapes) | Useful when multiple daemons scrape into one Prom. | ✓ |

**User's choice:** All four (multi-select).

### Default time range and refresh interval?

| Option | Description | Selected |
|--------|-------------|----------|
| 1h / 30s refresh | Matches typical operator-debugging session. Cheap for loopback Prom. | ✓ |
| 6h / 1m refresh | Better for trends; more burst-tolerant. | |
| You decide | Use Grafana defaults. | |

**User's choice:** 1h / 30s refresh (Recommended)

### Layout style of the Overview dashboard?

| Option | Description | Selected |
|--------|-------------|----------|
| RED-first rows | Top: Rate / Errors / Duration. Then Edit-outcomes, Session-lifecycle, RepoMap rows. | ✓ |
| Single mixed grid | One unstructured 6–8 panel grid, no rows. | |
| By subsystem (tool / edit / repomap / session) | Group by code subsystem. RED gets buried. | |

**User's choice:** RED-first rows (Recommended)

### Do edit-outcome and session-lifecycle panels live in Overview or LSPool?

| Option | Description | Selected |
|--------|-------------|----------|
| Both in Overview | LSPool stays focused on worker health; edit + session belong with RED. | ✓ |
| Session in Overview, edit outcomes in third dashboard | Punt edit outcomes to a third file. | |
| You decide | Group by what produces the most coherent screenshot. | |

**User's choice:** Both in Overview (Recommended)

### RepoMap extract latency — which dashboard?

| Option | Description | Selected |
|--------|-------------|----------|
| Overview | Tool-level latency signal; pairs with cache hit-rate. | ✓ |
| LSPool dashboard | RepoMap touches LS calls for documentSymbol fallback. | |

**User's choice:** Overview (Recommended)

---

## PromQL validation test

### How should the PromQL→registry validation test work?

| Option | Description | Selected |
|--------|-------------|----------|
| Parse JSON + extract metrics via promql parser | github.com/prometheus/prometheus/promql/parser. Catches metric-name + label typos. New direct dep. | ✓ |
| Regex extract metric names + check registry | Regex /(serena_[a-z_]+)/g, dedupe, assert in registry.Gather(). No new deps; misses label typos. | |
| Static allowlist file | Hand-maintained list. Drifts. | |

**User's choice:** Parse JSON + extract metrics via promql parser (Recommended)

### Where does the test live and what does it scan?

| Option | Description | Selected |
|--------|-------------|----------|
| internal/obs/dashboards_test.go | Lives next to the registry; consistent with Phase 11/53 placement. | ✓ |
| deploy/grafana/dashboards_test.go | Test next to artifacts; new package. | |
| test/oracle/grafana_test.go | Treat as oracle/integration test. | |

**User's choice:** internal/obs/dashboards_test.go (Recommended)

### Should runbook PromQL also be validated?

| Option | Description | Selected |
|--------|-------------|----------|
| Yes — same test, same parser | One test walks dashboards/*.json + runbooks/*.md. Roadmap success criterion 4 mandates "in the dashboards and runbooks." | ✓ |
| Yes, but separate test | Split for clarity: dashboards_test.go + runbooks_test.go. | |
| Dashboards only | Skip runbook validation. | |

**User's choice:** Yes — same test, same parser (Recommended)

### What should the parser do with PromQL functions and label matchers?

| Option | Description | Selected |
|--------|-------------|----------|
| Validate metric names + closed-enum label values | Assert label values for closed-enum labels (result, scope, phase, outcome, reason) are in the allowlist. | ✓ |
| Metric names only | Just check VectorSelector names. | |
| Full PromQL semantic check | Evaluate against synthetic series. Overkill. | |

**User's choice:** Validate metric names + closed-enum label values (Recommended)

### How should the test handle metrics that aren't pre-registered until first emission?

| Option | Description | Selected |
|--------|-------------|----------|
| Pre-register all families at metric construction | Phase 53 already does this; test asserts it on a fresh *obs.Metrics{} with no warm-up emission. | ✓ |
| Force a sample emission for each family before gathering | Defensive but masks lazy-registration regressions. | |

**User's choice:** Pre-register all families at metric construction (Recommended)

---

## Runbook template & depth

### Common runbook structure (front-matter / sections)?

| Option | Description | Selected |
|--------|-------------|----------|
| Symptoms → Detect → Triage → Remediate → Verify → Escalate | Six sections. Matches Google SRE runbook conventions. | ✓ |
| Symptoms / Triage / PromQL / Remediation only | Four sections, matches roadmap text literally. Loses Verify. | |
| Free-form Markdown | Each author picks structure. Inconsistent. | |

**User's choice:** Symptoms → Detect → Triage → Remediate → Verify → Escalate (Recommended)

### Should runbooks cross-link to dashboard panels?

| Option | Description | Selected |
|--------|-------------|----------|
| Yes — deep links by panel ID | Link via ?viewPanel=N. Stable panel IDs. One-click jump from runbook to panel. | ✓ |
| Yes — link to dashboard, not panel | Just /d/serena-overview/serena-overview. Simpler, less precise. | |
| No cross-links | PromQL only, in code blocks. | |

**User's choice:** Yes — deep links by panel ID (Recommended)

### Runbook depth — how prescriptive on remediation?

| Option | Description | Selected |
|--------|-------------|----------|
| Concrete commands + escape hatch | Real shell commands per step; final step is "file an issue with /metrics + log." | ✓ |
| Concept-level guidance | Describe what to look for, not exact commands. | |
| Decision tree only | Pure if/else with PromQL conditions. | |

**User's choice:** Concrete commands + escape hatch (Recommended)

### Where do runbooks live in the directory tree?

| Option | Description | Selected |
|--------|-------------|----------|
| docs/runbooks/{slug}.md, flat | Roadmap-named filenames; flat directory + README.md index. | ✓ |
| docs/runbooks/operational/ subdir | Nest under category. Premature for 4 runbooks. | |

**User's choice:** docs/runbooks/{slug}.md, flat (Recommended)

---

## Screenshot & USAGE.md wiring

### How should the dashboard screenshot be produced and stored?

| Option | Description | Selected |
|--------|-------------|----------|
| Manual capture, committed PNG at docs/img/grafana-overview.png | Author exports ~1600px PNG with synthetic data. Simple; no runtime dep. | ✓ |
| Scripted via Grafana render API | Make target boots Grafana, hits /render. Heavy. | |
| Describe-only (no PNG) | Prose + ASCII layout. Rejects roadmap success criterion 3. | |

**User's choice:** Manual capture, committed PNG at docs/img/grafana-overview.png (Recommended)

### How is screenshot drift caught?

| Option | Description | Selected |
|--------|-------------|----------|
| Comment + CONTRIBUTING note | Inline HTML comment in USAGE.md + regeneration recipe in docs/runbooks/README.md. | ✓ |
| CI check that PNG mtime > overview JSON mtime | Trips on every rebase. | |
| Don't track drift | Accept staleness. | |

**User's choice:** Comment + CONTRIBUTING note (Recommended)

### Where in USAGE.md does the dashboard section land?

| Option | Description | Selected |
|--------|-------------|----------|
| New "Grafana Dashboards" subsection inside Observability Quickstart | Slot between "Prometheus Metrics" and the existing "Phase 53 metric families" subsection. | ✓ |
| Top-level new section after Observability Quickstart | Promote to a peer-level section. Breaks existing flow. | |

**User's choice:** New "Grafana Dashboards" subsection inside Observability Quickstart (Recommended)

---

## Claude's Discretion

- Exact panel-ID numbering scheme (sequential vs row-aligned). Either works as long as IDs are stable.
- Specific synthetic-data values rendered into the screenshot (operator-friendly: ~12 langs, healthy hit-rates, one anomaly).
- Whether `deploy/grafana/README.md` is a new file or extends a top-level `deploy/README.md` (deploy/ does not currently exist; new file is fine).
- Decision-tree style in Triage sections (prose vs ASCII flowchart vs Mermaid) — author's discretion per runbook.

## Deferred Ideas

- Alert rules / recording rules — out of scope; future `obs-alerting` phase.
- SLO definitions (e.g., p95 tool latency target) — needs alerting infra first.
- Scripted screenshot generation via Grafana render API — revisit if drift becomes painful.
- Per-panel CI drift checks (PNG mtime vs JSON mtime) — too noisy on rebase.
- Grafana provisioning manifests (datasources.yml, dashboards.yml) — out of scope.
- Tempo / Loki integration for log+trace correlation — downstream of Phase 55 trace audit.
- Standardizing decision-tree formatting across all runbooks — author's discretion for v1.

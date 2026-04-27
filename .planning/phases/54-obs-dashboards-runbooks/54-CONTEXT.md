# Phase 54: obs-dashboards-runbooks - Context

**Gathered:** 2026-04-27
**Status:** Ready for planning

<domain>
## Phase Boundary

Turn the v1.2 + Phase 53 metric inventory into operator-facing artifacts: ship two Grafana 10+ JSON dashboards under `deploy/grafana/`, four runbooks under `docs/runbooks/`, a USAGE.md "Grafana Dashboards" subsection with a committed screenshot, and a parser-backed test that asserts every PromQL expression shipped (in dashboards AND runbooks) references a metric that exists in the registered `*obs.Metrics` Prometheus registry, with closed-enum label values validated against the same allowlists used in `internal/obs/metrics_labels_test.go`.

**In scope:**
- `deploy/grafana/serena-overview.json` — primary dashboard. RED-first row layout (Rate / Errors / Duration), Edit-outcomes row, Session-lifecycle row, RepoMap extract-latency + cache-hit-rate row.
- `deploy/grafana/serena-lspool.json` — focused on worker health: workers gauge, circuit state, evictions by reason, restarts, lspool cache hit-rate, with `language` template-variable filtering.
- Four runbooks at `docs/runbooks/{ErrCircuitOpen.md,deadline-timeouts.md,ls-crash-restart.md,memory-pressure-eviction.md}` — six-section template (Symptoms → Detect → Triage → Remediate → Verify → Escalate), deep-linked back to specific dashboard panels via `#viewPanel=N`.
- `docs/runbooks/README.md` — index linking each runbook with a one-line summary, plus the "regenerate the screenshot" recipe.
- `deploy/grafana/README.md` — import instructions and a one-line summary per dashboard.
- USAGE.md "Grafana Dashboards" subsection inside Observability Quickstart (between "Prometheus Metrics" and the existing "Phase 53 metric families" subsection), with a relative link to `docs/img/grafana-overview.png`.
- Committed screenshot at `docs/img/grafana-overview.png` (~1600px wide; manual capture from a Grafana 10+ render against synthetic data).
- `internal/obs/dashboards_test.go` — single test that scans `deploy/grafana/*.json` and `docs/runbooks/*.md`, parses every PromQL expression via `github.com/prometheus/prometheus/promql/parser`, asserts each `VectorSelector` metric name is in `registry.Gather()` from a fresh `*obs.Metrics`, and asserts every label-matcher value on the closed-enum labels (`result`, `scope`, `phase`, `outcome`, `reason`) is in the corresponding allowlist.
- A single new go.mod direct dependency: `github.com/prometheus/prometheus` (parser package only).

**Out of scope:**
- Alert rules, recording rules, SLO definitions (future phase).
- Tempo / Loki / OTel-collector integration (separate observability work).
- Trace coverage audit (Phase 55).
- New metric families. Phase 53 closed the inventory.
- Renaming or restructuring existing metric names. Phase 54 only consumes them.
- Scripted/automated screenshot generation (Grafana render API). Manual capture only.
- Per-panel CI drift checks beyond the PromQL→registry test. Drift on the screenshot is handled by an inline HTML comment + CONTRIBUTING note, not CI.

</domain>

<decisions>
## Implementation Decisions

### Dashboard split & primary

- **D-01 (split):** Two JSON files: `serena-overview.json` and `serena-lspool.json`. Overview is the operator landing page (RED + edit + session + repomap). LSPool is the drill-down for worker health.
- **D-02 (primary):** `serena-overview.json` is the "primary" dashboard for USAGE.md success criterion 3 — its RED rows produce the most informative landing screenshot.
- **D-03 (Overview layout):** RED-first rows. Top row: Rate (calls/s by tool) | Errors (% error rate) | Duration (p95 of `serena_tool_duration_seconds`). Then Edit-outcomes row, Session-lifecycle row, RepoMap extract-latency + repomap cache-hit-rate row. Each row has a stable, hand-assigned panel ID so runbooks can deep-link.
- **D-04 (LSPool layout):** Focused on `serena_lspool_*`. Workers gauge, circuit state, evictions-by-reason, restarts, lspool cache hit-rate (using the `result={hit,miss}` enum from Phase 53 D-01). Edit and session metrics do NOT appear here — they belong in Overview.
- **D-05 (RepoMap location):** RepoMap extract latency and repomap cache hit-rate panels live in Overview. RepoMap is a tool-level concern; LSPool stays pure to worker health.
- **D-06 (template variables):** Both dashboards expose `$datasource = ${DS_PROMETHEUS}` (required for portable import). Overview also exposes `$tool = label_values(serena_tool_calls_total, tool_name)`. LSPool exposes `$language = label_values(serena_lspool_workers, language)`. Both expose `$instance` for multi-daemon scrapes.
- **D-07 (default time / refresh):** Both dashboards default to a 1h time window with a 30s refresh interval.

### PromQL validation test

- **D-08 (parser):** Use `github.com/prometheus/prometheus/promql/parser`. Walk every `panel.targets[].expr` in dashboards and every fenced ```promql block in runbooks, parse, and collect `VectorSelector` nodes. Adds one new direct go.mod dep (`github.com/prometheus/prometheus`); the upside (label-value validation, future-proofing) outweighs the dep weight. Regex extraction was rejected because it cannot validate label matchers.
- **D-09 (test location):** `internal/obs/dashboards_test.go` — same package as cardinality and label-allowlist tests (Phase 11 / 53 placement convention). Test reads `deploy/grafana/*.json` and `docs/runbooks/*.md` via filepath.Walk relative to the repo root.
- **D-10 (coverage = dashboards + runbooks):** A single test covers both artifact directories. Roadmap success criterion 4 explicitly requires "in the dashboards and runbooks" — split tests would invite divergence.
- **D-11 (validation depth):** Validate (a) every metric name exists in `registry.Gather()` from a fresh `*obs.Metrics{}`, AND (b) every label-matcher value on closed-enum labels (`result`, `scope`, `phase`, `outcome`, `reason`) is in the corresponding closed allowlist (sourced from `internal/obs/metrics_labels_test.go` carve-outs). Catches both metric-name typos and enum-value typos like `result="hits"` or `scope="crash"`.
- **D-12 (registry assumption):** All metric families must be visible in `registry.Gather()` on a freshly-constructed `*obs.Metrics{}` BEFORE any sample is emitted. Phase 53 already MustRegisters every family at construction time. The test asserts this with no warm-up emission so any future regression to lazy registration is caught.

### Runbook template & depth

- **D-13 (sections):** Six-section template per runbook: **Symptoms** (what the operator sees) → **Detect** (PromQL alert query / dashboard panel link) → **Triage** (decision tree of likely causes) → **Remediate** (concrete commands) → **Verify** (PromQL that should now show recovery) → **Escalate** (when to file an issue, with required attachments).
- **D-14 (deep linking):** Each Detect and Verify section links to the specific Grafana panel via `/d/<dashboard-uid>/<slug>?viewPanel=<panel-id>`. Stable panel IDs are assigned in the JSON. `dashboard-uid` is fixed at `serena-overview` and `serena-lspool`.
- **D-15 (depth):** Each Remediate step shows actual shell commands or config edits — not just concepts. Example for `ls-crash-restart.md`: `pkill -USR1 serena` (or platform-specific equivalent) to force a worker re-spawn, plus a `serena_config.yml` edit to raise the circuit-breaker threshold. The final step in every Remediate section is the escape hatch: "If none of the above resolves: file an issue with the output of `curl -s http://127.0.0.1:9100/metrics` and the daemon log from the last 10 minutes."
- **D-16 (location & naming):** Flat `docs/runbooks/`. The four files use the exact names from the roadmap: `ErrCircuitOpen.md`, `deadline-timeouts.md`, `ls-crash-restart.md`, `memory-pressure-eviction.md`. Plus a `docs/runbooks/README.md` index that lists each with a one-line summary.

### Screenshot & USAGE.md wiring

- **D-17 (screenshot production):** Manual capture, committed PNG at `docs/img/grafana-overview.png`. Author imports `serena-overview.json` into a Grafana 10+ instance with realistic synthetic data, exports a ~1600px-wide PNG, and commits it. Scripted Grafana render was rejected (heavyweight runtime dep for one image).
- **D-18 (drift handling):** Inline HTML comment in USAGE.md immediately above the screenshot — `<!-- Regenerate when serena-overview.json panel set changes. Recipe: docs/runbooks/README.md#regenerating-the-screenshot. -->` — plus the regeneration recipe lives in `docs/runbooks/README.md`. No CI check on PNG mtime (would trip on every rebase).
- **D-19 (USAGE.md location):** New "Grafana Dashboards" subsection nested inside the existing "Observability Quickstart" section, slotted between "Prometheus Metrics" and the existing "Phase 53 metric families" subsection. Includes (1) Grafana import instructions (UI: Dashboards → Import → Upload JSON file → pick Prometheus datasource), (2) the screenshot, (3) links to `deploy/grafana/` and `docs/runbooks/`.

### Claude's Discretion

- Exact panel-ID numbering scheme (sequential 1..N, or row-aligned 100/110/120). Either is fine as long as IDs are stable and runbook deep links match.
- Specific synthetic-data values rendered into the screenshot. Operator-friendly values (~12 langs visible, healthy hit-rates, one obvious anomaly) are best.
- Whether `deploy/grafana/README.md` is a new file or extends an existing top-level `deploy/README.md`. Either is acceptable — `deploy/` does not currently exist, so a new `deploy/grafana/README.md` is fine.
- Choice of decision-tree style in Triage sections (bulleted prose vs ASCII flowchart vs Mermaid). Pick whatever produces the cleanest GitHub render.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Roadmap & requirements
- `.planning/ROADMAP.md` — Phase 54 entry: goal, success criteria 1–4, dependency on Phase 53.
- `.planning/REQUIREMENTS.md` — OBS-01 (Grafana dashboards) and OBS-02 (runbooks) acceptance language.

### Prior phase context (carried forward)
- `.planning/phases/53-obs-metrics-gaps/53-CONTEXT.md` — locked metric names, label sets, closed enums (D-01..D-13). Single source of truth for what Phase 54 dashboards/runbooks may reference.
- `.planning/phases/53-obs-metrics-gaps/53-SUMMARY.md` (and 53-01/02/03 SUMMARY.md if present) — what shipped vs what was deferred. Phase 54 must NOT reference any deferred metric.

### Metric registry & label allowlists
- `internal/obs/metrics.go` — owned `*prometheus.Registry`, all `MustRegister` calls, closed-enum helper signatures. The PromQL test calls `Registry().Gather()` here.
- `internal/obs/metrics_labels_test.go` — closed-enum carve-outs for `result`, `scope`, `phase`, `outcome`, `reason`. The Phase 54 test imports/extracts these allowlists.
- `internal/obs/metrics_cardinality_test.go` — Phase 53 cardinality bounds; Phase 54 test does NOT replace these, only extends.
- `internal/obs/handler.go` — admin listener `/metrics` endpoint. USAGE.md instructions reference `127.0.0.1:9100/metrics`.

### USAGE.md anchor points
- `USAGE.md` §"Observability Quickstart" — lines ~606+ today; the new "Grafana Dashboards" subsection slots inside this section between "Prometheus Metrics" and "Phase 53 metric families."
- `USAGE.md` §"Phase 53 metric families" (~lines 690+) — operator-facing metric documentation that Phase 54 dashboards visualize. Treat as the canonical metric reference for runbook prose.

### PromQL parser dependency
- `go.mod` — Phase 54 adds `github.com/prometheus/prometheus` as a direct dependency. Researcher must verify the smallest acceptable import path (`promql/parser` only, not the full server stack) and pin a stable release.

### External Grafana conventions
- Grafana 10+ dashboard JSON model documentation — researcher should pull current schema (panels, targets, templating, links) via Context7 or web fetch before authoring JSON.
- Grafana panel-link URL syntax (`?viewPanel=<id>`) for runbook deep links.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `*obs.Metrics.Registry()` (`internal/obs/metrics.go:191`) — direct accessor to the owned registry. Phase 54 test calls `obs.New(...).Registry().Gather()` to enumerate metric families.
- Closed-enum allowlists in `internal/obs/metrics_labels_test.go` — already-validated test fixtures Phase 54 reuses verbatim for label-value validation.
- `internal/obs/handler.go` — admin `/metrics` endpoint. Runbook PromQL queries are tested against the same registry it serves.

### Established Patterns
- Phase 11 / Phase 53 keep all observability-test enforcement in `internal/obs/*_test.go` (cardinality, label allowlist, alloc). Phase 54 test follows the same convention.
- USAGE.md uses GitHub-flavored-markdown tables for metric documentation (see "Phase 53 metric families" §). New dashboard panels/runbook PromQL are documented in the same style.
- Stable, owner-controlled `prometheus.Registry` (Phase 11 D-08 invariant): no upstream consumer imports `internal/obs` directly. Phase 54 dashboards/runbooks consume metric NAMES (strings), so the boundary holds.

### Integration Points
- Test target: `internal/obs/dashboards_test.go` runs as part of `go test ./...`. No new test target needed.
- Build target: `deploy/grafana/*.json` and `docs/runbooks/*.md` and `docs/img/grafana-overview.png` are static assets — no Makefile changes required.
- Documentation surfaces: USAGE.md (operator quickstart), `deploy/grafana/README.md` (import recipe), `docs/runbooks/README.md` (runbook index + screenshot regeneration recipe).

</code_context>

<specifics>
## Specific Ideas

- The operator's incident path is "alert/symptom → runbook → dashboard panel → fix → verify." Every runbook must include both the Detect-side and Verify-side PromQL so the operator can confirm recovery without leaving the runbook.
- Edit-outcome and session-lifecycle visualizations belong on the Overview because they answer "is Serena healthy from the user's perspective?" — LSPool answers "is the language-server pool healthy?" Different audiences (UX-level vs infra-level), different dashboards.
- The screenshot's value is recognition, not detail. ~1600px is wide enough to make panels readable at-a-glance without bloating the repo.

</specifics>

<deferred>
## Deferred Ideas

- **Alert rules / recording rules.** Operators have asked informally; out of scope for this phase. Belongs in a follow-on `obs-alerting` phase.
- **SLO definitions** (e.g., "p95 tool latency < 200ms"). Same — needs alerting infra first.
- **Scripted screenshot generation via Grafana render API.** Drops manual-capture friction but adds a Grafana-as-test runtime dep. Revisit if dashboards start drifting frequently.
- **Per-panel CI drift checks** (e.g., asserting screenshot mtime > overview JSON mtime). Noisy on rebase; rejected for this phase.
- **Grafana provisioning manifests** (datasources.yml, dashboards.yml). Useful for users running Grafana via docker-compose; out of scope.
- **Tempo / Loki integration** for log+trace correlation from dashboard panels. Phase 55 audits trace coverage; correlated-pane work is downstream of that.
- **Decision-tree formatting standardization across runbooks** (forcing all four to use Mermaid, say). Author's discretion per runbook for v1; revisit if inconsistency hurts readability.

</deferred>

---

*Phase: 54-obs-dashboards-runbooks*
*Context gathered: 2026-04-27*

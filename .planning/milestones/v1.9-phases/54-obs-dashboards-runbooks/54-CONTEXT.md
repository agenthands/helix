# Phase 54: obs-dashboards-runbooks - Context

**Gathered:** 2026-05-01
**Status:** Ready for planning

<domain>
## Phase Boundary

This phase ships operator-facing artifacts that consume the v1.2 metrics that landed in Phase 53. Two things are delivered:

1. **Two Grafana dashboards** in `deploy/grafana/` (a new top-level directory created by this phase) that import cleanly into Grafana 10+.
2. **Four operational runbooks** in `docs/runbooks/` (also a new top-level directory) covering the four most common Helix failure modes: `ErrCircuitOpen.md`, `deadline-timeouts.md`, `ls-crash-restart.md`, `memory-pressure-eviction.md`.

In addition, `USAGE.md`'s Observability section is extended with a link to `deploy/grafana/` and a screenshot of the primary dashboard, and a CI test asserts that every PromQL expression in the JSON dashboards and Markdown runbooks references a metric that actually exists in the registered Prometheus registry.

**In scope:**
- `deploy/grafana/helix-overview.json` — RED metrics + workspace activity (sessions). Primary dashboard, screenshot target.
- `deploy/grafana/helix-engine.json` — lspool health + repomap + edits. On-call deep-dive.
- Both dashboards use the `${DS_PROMETHEUS}` import-template pattern (Grafana's `__inputs` + `__requires` blocks) and expose `$language` and `$instance` template variables for drill-down.
- `docs/runbooks/ErrCircuitOpen.md`, `docs/runbooks/deadline-timeouts.md`, `docs/runbooks/ls-crash-restart.md`, `docs/runbooks/memory-pressure-eviction.md`. Shared YAML-frontmatter + 4-section H2 skeleton (Symptoms / Triage / Likely Causes / Remediation) plus a Code references section with `file:line` pointers.
- `internal/obs/dashboards_test.go` — PromQL validation test that walks `deploy/grafana/*.json` and `docs/runbooks/*.md`, parses every PromQL expression with `github.com/prometheus/prometheus/promql/parser`, and asserts every metric name is present in the registry returned by `*obs.Metrics`.
- `docs/images/helix-overview-dashboard.png` — manual one-time screenshot of the populated overview dashboard.
- `USAGE.md` Observability section update — link to `deploy/grafana/`, embed the screenshot, document the import flow (`${DS_PROMETHEUS}` prompt) and the `$language`/`$instance` template variables.

**Out of scope (other phases or future work):**
- New metric families. All PromQL must reference metrics already registered as of end of Phase 53 (see `<canonical_refs>` for the full inventory).
- Trace coverage / span audit — Phase 55 (OBS-04).
- Per-tool latency dashboards beyond the RED rollup — possible later phase.
- Alerting rules (`alerting/`, `rules.yml`) — not in OBS-01/OBS-02; deferred.
- Provisioning sidecar (docker-compose with Grafana + Prometheus pre-wired) — deferred (see `<deferred>`).
- Re-capturing the screenshot on every panel change — manual, on visible drift only.
- Translating runbooks for the legacy Python tree under `legacy/`.

</domain>

<decisions>
## Implementation Decisions

### Dashboard split, scope, and panel ownership

- **D-01: Two dashboards: `helix-overview.json` + `helix-engine.json`.** Hits the roadmap's `≥2` minimum cleanly. Overview is the operator-facing summary and screenshot target; engine is the on-call deep-dive. Three-dashboard split (RED / lspool / repomap+edits) was rejected as more navigation cost than signal — most operators want one screen for "is it healthy" and one for "what's broken inside". A single mega-dashboard with collapsed rows was rejected because it violates the roadmap's `≥2 dashboards` literal requirement without a stub second file.
- **D-02: `helix-overview.json` owns RED + workspace activity.** Panels: tool-call rate, tool-call error rate by `outcome` (the 7-value enum from `helix_tool_calls_total`), p50/p95/p99 of `helix_tool_duration_seconds`, session lifecycle rate by `phase` (`started`/`ended`/`error`) and `transport` (`stdio`/`http`), plus a callout panel of the HTTP-`ended`-is-best-effort caveat documented in Phase 53 D-09. This is the primary dashboard — the screenshot lives here.
- **D-03: `helix-engine.json` owns lspool + repomap + edits.** Panels: lspool worker count by language, lspool eviction rate by `reason` (`idle`/`pressure`/`crash`/`shutdown`), lspool circuit state, lspool restart rate, lspool cache hit-ratio (the Phase 53 D-01 PromQL: `sum(rate(helix_lspool_lookups_total{result="hit"}[5m])) / sum(rate(helix_lspool_lookups_total[5m]))`), repomap cache hit-ratio (same shape), per-extractor p95 RepoMap extract latency (the Phase 53 D-05/D-06 PromQL), edit-tool outcome rate by `outcome` and `tool_name`, and rename strategy share (`helix_rename_strategy_total{strategy=lsp-native|rust-client-side}`).
- **D-04: Each dashboard targets 6–9 panels.** Below 6 = thin; above 9 = scroll fatigue. Planner picks final panel count within this band based on visual coherence; small-multiples (one panel per language) only where the metric is bounded ≤ 8 languages, otherwise use `sum by (language)` with a multi-line graph.

### Datasource and import portability

- **D-05: Use `${DS_PROMETHEUS}` import-template pattern.** Both JSON files declare an `__inputs` block with one entry of type `datasource` named `DS_PROMETHEUS`, plus a `__requires` block listing `grafana >= 10.0.0` and `prometheus` datasource. Every panel `targets[].datasource` references `${DS_PROMETHEUS}`. On import via Grafana UI or API, Grafana prompts the operator to bind `DS_PROMETHEUS` to their actual Prometheus datasource — no UID coupling. This is the same pattern that grafana.com community dashboards use, and it survives operators who name their datasource anything (`prometheus`, `Prometheus`, `prom-prod-1`, etc.). Hardcoded UIDs and provisioning sidecar were rejected — UIDs break custom-named datasources, sidecar is heavier scope (Prometheus container + scrape config) than OBS-01 demands.
- **D-06: Schema version targets Grafana 10+.** Set `schemaVersion: 38` (Grafana 10.0 baseline) at minimum. The dashboards must `import cleanly into Grafana 10+` per success-criterion 1; the planner verifies by importing into a Grafana 10.4+ test instance during plan verification.
- **D-07: No alerting rules in this phase.** Dashboard panels do not embed `alert` blocks. OBS-01 is observability surface only; alerting (`alerting/` rules, Alertmanager wiring) is its own scope and deferred.

### Template variables (drill-down)

- **D-08: Both dashboards expose `$language` and `$instance` template variables.** `$language` sourced from `label_values(helix_lspool_workers, language)` (Multi-select, Include-All, default = All). `$instance` sourced from `label_values(up{job=~".*helix.*"}, instance)` (Multi-select, Include-All, default = All). Panel queries filter via `{language=~"$language", instance=~"$instance"}` where the metric carries those labels; metrics without one of those labels just omit the matcher (e.g., `helix_session_lifecycle_total` has neither — its panels skip both vars). Operators can scope to a single language or instance without editing JSON.
- **D-09: No additional template variables.** No `$tool`, `$outcome`, `$strategy`, `$extractor` dropdowns — those carve-outs are too narrow to deserve a top-level dropdown and live as panel-level legend filters instead. Keeps the two variable bars minimal and consistent across both dashboards.

### PromQL validation test

- **D-10: Test lives at `internal/obs/dashboards_test.go`.** Co-located with the metrics registry it validates. Uses `*obs.NewProvider(ctx, obs.Config{})` to build a fresh registry, calls `provider.Metrics().Registry().Gather()` to enumerate registered metric families, builds a `set[string]` of family names, then walks the dashboard + runbook tree and asserts every PromQL VectorSelector name is in the set (after stripping histogram suffixes).
- **D-11: Use `github.com/prometheus/prometheus/promql/parser` for AST extraction.** `parser.ParseExpr(expr)` returns an `parser.Expr`; `parser.Walk` visits every node; on `*parser.VectorSelector`, capture `.Name`. Also walk `*parser.MatrixSelector.VectorSelector` for range-vector queries (`rate()`, `increase()`, etc.). Histogram-suffix handling: if the captured name ends in `_bucket`, `_count`, or `_sum`, strip the suffix and check the base name against the registry — this matches how histogram families are registered (one base name, three suffix series). Custom regex was rejected — it cannot reliably distinguish metric names from label names, function names, or recording-rule references.
- **D-12: Test enumerates inputs from disk, not embed.** The test reads `deploy/grafana/*.json` and `docs/runbooks/*.md` from disk relative to the project root (resolved via `runtime.Caller(0)` + `filepath.Join(..., "../..")`). Reading from disk means adding a new dashboard or runbook file is automatically covered — no test-list edits needed. The test fails closed if either directory is empty (catches accidental deletion).
- **D-13: PromQL extraction targets:**
  - **JSON dashboards:** every panel's `targets[].expr` field, recursively across `panels[]` and any nested `panels[]` inside `row` panels. Use `encoding/json` with a minimal struct that captures `panels[].targets[].expr` and `panels[].panels[].targets[].expr`.
  - **Markdown runbooks:** every fenced ```` ```promql ```` code block. Plain ```` ``` ```` blocks are skipped (treated as prose / shell snippets). The fence language identifier is the contract — runbook authors MUST use ```` ```promql ```` for queries that should be validated.
- **D-14: Validation also covers label filters.** When a VectorSelector has matchers (e.g., `{result="hit"}`), the test asserts that every label name on the right-hand side (`result`, `language`, `outcome`, etc.) is either in `obs.AllowedLabels` or in a registered carve-out (the `carveOuts` map in `internal/obs/metrics_labels_test.go`). This catches typos like `helix_lspool_lookups_total{results="hit"}` (extra `s`) that would silently produce empty graphs in production. Scope is limited to label NAMES — label VALUES (`hit`/`miss`) are not checked because PromQL allows regex matchers (`result=~"hit|miss"`) and validating value literals adds complexity without a corresponding failure mode.

### Runbook structure

- **D-15: Shared YAML frontmatter on every runbook.** Fields:
  ```yaml
  ---
  title: <human-readable runbook title>
  severity: <one of: warning | critical>
  metric: <primary metric this runbook keys off, e.g., helix_lspool_circuit_state>
  since_phase: <phase that introduced the metric, for context>
  last_reviewed: <ISO date>
  ---
  ```
  Severity is a closed-enum (warning|critical) — `warning` for "operator should investigate when metric drifts" (eviction rate, hit-ratio drop), `critical` for "operator should act now" (circuit open, repeated LS crashes). Machine-readable so future tooling (alert generator, runbook index page) can ingest it.
- **D-16: Shared 4-section H2 skeleton in fixed order.** Every runbook has, after the frontmatter and a one-paragraph intro:
  1. `## Symptoms` — what operators see (PromQL alert condition, dashboard panel that turns red, log line)
  2. `## Triage` — diagnostic PromQL queries ranked by usefulness, in fenced ```` ```promql ```` blocks, with a one-line interpretation under each
  3. `## Likely Causes` — bullet list of root causes ordered by frequency, each with a one-sentence "how to confirm"
  4. `## Remediation` — concrete steps (config knob to flip, daemon restart, code fix), each labeled with required privilege (operator vs. engineer)
- **D-17: Every runbook ends with a `## Code references` section.** Format: bulleted `file:line — brief context` entries pointing to the relevant `internal/...` paths. Examples:
  - `ErrCircuitOpen.md` → `internal/kernel/lspool/circuit.go:47 — failure threshold; tunable via lspool.circuit.failure_threshold`
  - `ls-crash-restart.md` → `internal/kernel/lspool/worker.go:<line> — restart loop and backoff`
  - `memory-pressure-eviction.md` → `internal/kernel/lspool/eviction.go:<line> — pressure scoring`
  - `deadline-timeouts.md` → `internal/mcp/middleware.go:<line> — TelemetryMiddleware budget injection`
  Line numbers will drift; that's acceptable — the `last_reviewed` frontmatter field gives an explicit checkpoint, and a broken `file:line` link breaks loudly when an engineer follows it.

### Runbook ↔ dashboard PromQL relationship

- **D-18: Runbooks author PromQL independently of dashboards.** Runbook queries are diagnostic / narrative-shaped (e.g., `topk(5, ...)`, narrowly-scoped histogram quantiles, per-language drill-downs) while dashboard queries are panel-shaped (rolled-up, intended for a graph). Forcing text-equivalence between the two would either pollute dashboards with diagnostic queries or impoverish runbooks. The Phase 53 PromQL examples in `USAGE.md` (the `lspool` hit-ratio and `repomap_extract` p95 queries) ARE good runbook material and should be reused verbatim where they fit.
- **D-19: Drift is caught by the validation test (D-10–D-14), not text-equivalence.** Both dashboard PromQL and runbook PromQL pass the same metric-name + label-name validation; that is the contract. Text equivalence is not enforced.

### Screenshot and USAGE.md update

- **D-20: One manual PNG, committed once, re-captured on visible drift.** File: `docs/images/helix-overview-dashboard.png`. Capture procedure documented inline in the phase plan: spin up Grafana 10+ locally, wire it to a Prometheus scraping the helix admin listener, run `go test ./internal/repomap/... -bench=.` and exercise a few MCP tools to populate the registry with realistic data, import `helix-overview.json` (Grafana prompts for the datasource), screenshot the rendered dashboard at 1600×900 (the default Grafana viewport), commit. The scripted/docker-compose path was rejected — adds a Grafana+Prometheus dev dependency that wasn't requested by OBS-01.
- **D-21: USAGE.md Observability section grows by one subsection.** Add a new H3 `### Grafana Dashboards` immediately before the existing `### Prometheus Metrics` subsection (so the flow goes admin listener → dashboards → metrics table → tracing). Subsection contents:
  1. The screenshot (`![Helix overview dashboard](docs/images/helix-overview-dashboard.png)`).
  2. Two-sentence description of what each dashboard shows (overview = RED + sessions; engine = lspool + repomap + edits).
  3. A bulleted list linking the two JSON files: `- [helix-overview.json](deploy/grafana/helix-overview.json) — primary dashboard.` etc.
  4. Import instructions: "In Grafana, **Dashboards → New → Import**, upload the JSON, select your Prometheus datasource when prompted for `DS_PROMETHEUS`."
  5. A note that `$language` and `$instance` template variables let operators scope panels.
- **D-22: Runbook section in USAGE.md is also added.** A new `### Runbooks` H3 immediately after the dashboards subsection lists the four runbook files with a one-line description each. Roadmap success-criterion 3 only requires the dashboard link/screenshot, but linking runbooks from the same Observability section makes them discoverable to operators reading USAGE.md top-down.

### Wiring and packaging discretion (planner picks within these constraints)

- **D-23: The planner authors PromQL using only the metric families registered as of end of Phase 53.** Authoritative inventory in `<canonical_refs>`. The validation test enforces this — but the planner should avoid even attempting expressions that reference future metrics, since they'd break the test before review.
- **D-24: Panel kit is Grafana stock.** `timeseries`, `stat`, `gauge`, `barchart` panel types only — no custom plugins or non-default visualizations. Keeps imports painless across Grafana 10/11/12.
- **D-25: File ordering inside `deploy/grafana/`.** Two top-level JSON files (`helix-overview.json`, `helix-engine.json`) in the directory root. No subdirectories, no `helix.json` umbrella. Future dashboards added as siblings.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase scope and roadmap
- `.planning/ROADMAP.md` §"Phase 54: obs-dashboards-runbooks" (lines 195+) — phase goal and 4 success criteria. The 4th success criterion (every PromQL references a real metric, validated by test) is non-negotiable and locks D-10–D-14.
- `.planning/REQUIREMENTS.md` §OBS-01 (line 33) — Grafana JSON dashboards in `deploy/grafana/` covering RED + lspool worker health + workspace activity.
- `.planning/REQUIREMENTS.md` §OBS-02 (line 34) — written runbooks in `docs/runbooks/` for the four operational failure modes.
- `.planning/PROJECT.md` §"Current State" / §"Constraints" — Helix is a single Go binary; observability is opt-in (admin listener disabled by default); noop-default invariant must not regress.

### Phase 53 — metric inventory (the metrics this phase consumes)
- `.planning/phases/53-obs-metrics-gaps/53-CONTEXT.md` — Phase 53 implementation decisions. D-01 through D-17 lock the closed-enum label values (`result`, `extractor`, `phase`, `transport`, `outcome`, `strategy`) that runbook + dashboard PromQL filters MUST use.
- `internal/obs/metrics.go` — registered metric families. Authoritative inventory of names + labels + types:
  - `helix_tool_duration_seconds` (histogram; labels `tool_name`, `profile`, `mode`, `language`)
  - `helix_tool_calls_total` (counter; labels `tool_name`, `profile`, `mode`, `language`, `outcome`)
  - `helix_lspool_workers` (gauge; label `language`)
  - `helix_lspool_evictions_total` (counter; labels `language`, `reason` ∈ `{idle, pressure, crash, shutdown}`)
  - `helix_lspool_circuit_state` (gauge; label `language`)
  - `helix_lspool_restarts_total` (counter; label `language`)
  - `helix_rename_strategy_total` (counter; label `strategy` ∈ `{lsp-native, rust-client-side}` — Phase 47 D-07)
  - `helix_lspool_lookups_total` (counter; labels `language`, `result` ∈ `{hit, miss}` — Phase 53 D-01/D-02)
  - `helix_repomap_lookups_total` (counter; labels `language`, `result` ∈ `{hit, miss}` — Phase 53 D-03)
  - `helix_repomap_extract_duration_seconds` (histogram; labels `language`, `extractor` ∈ `{treesitter, lsp, fallback}`; custom buckets 1ms→2.5s — Phase 53 D-05/D-06)
  - `helix_session_lifecycle_total` (counter; labels `phase` ∈ `{started, ended, error}`, `transport` ∈ `{stdio, http}`; HTTP `ended` is best-effort — Phase 53 D-08/D-09)
  - `helix_edit_outcome_total` (counter; labels `tool_name`, `outcome` ∈ `{success, no_match, ambiguous_match, validation_failed, ls_error, internal}`, `strategy` ∈ `{exact, whitespace_normalized, indentation_flexible, none}` — Phase 53 D-10/D-11/D-12)
- `internal/obs/metrics_labels_test.go` — `AllowedLabels` slice + `carveOuts` map; D-14 validation reuses these as the source of truth for legitimate label names per family.

### Existing observability surfaces (must extend, not duplicate)
- `USAGE.md` §"Prometheus Metrics" (lines 634–706) — existing metrics table + the two example PromQL blocks for hit-ratio and per-extractor p95. Extended in D-21/D-22 with a `### Grafana Dashboards` and `### Runbooks` subsection inserted before line 634.
- `USAGE.md` §"Observability Quickstart" (lines 600–632) — admin listener / `/metrics` / `/healthz` / `/readyz` already documented; do not regress these.

### Code locations referenced by the four runbooks (file:line targets for D-17)
- `internal/kernel/lspool/circuit.go` — circuit-breaker state machine; trip threshold and recovery; canonical reference for `ErrCircuitOpen.md`.
- `internal/kernel/lspool/worker.go` and `internal/kernel/lspool/pool.go` — LS worker lifecycle, restart loop with exponential backoff; canonical reference for `ls-crash-restart.md`.
- `internal/kernel/lspool/eviction.go` (or wherever the platform-aware pressure eviction lives — confirmed during research) — Linux cgroups + macOS vm_stat pressure scoring; canonical reference for `memory-pressure-eviction.md`.
- `internal/mcp/middleware.go` `TelemetryMiddleware` — per-tool deadline injection via `BudgetFunc`; canonical reference for `deadline-timeouts.md`.
- `internal/config/` and `~/.helix/helix_config.yml` schema — degradation timeouts (`degradation.timeout_read`, `degradation.timeout_search`, etc.) referenced from `deadline-timeouts.md`.

### Test infrastructure to mirror
- `internal/obs/metrics_labels_test.go` — `TestMetricsLabelsAllowlist` style: enumerate registered families via the registry, walk asserts, fail-loud diffs. D-10's new test follows the same shape.
- `github.com/prometheus/prometheus/promql/parser` — already a transitive dependency of `prometheus/client_golang` in many projects; planner verifies it's in `go.mod` (or addable cleanly) during research, since this is the only new external dependency this phase introduces.

### Prior decisions carried forward
- Phase 11 D-04: `AllowedLabels = {tool_name, profile, mode, language, outcome}` — used by the D-14 label-name validation.
- Phase 11 D-13: closed-enum carve-outs as the test pattern — D-14 mirrors this.
- Phase 47 D-07: `helix_rename_strategy_total{strategy=lsp-native|rust-client-side}` is orthogonal to the Phase 53 edit-strategy label; both coexist on the engine dashboard.
- Phase 52 D-01..D-03: all metric names use `helix_*` prefix; this phase ships only `helix_*` queries.
- Phase 53 D-13: ROADMAP success-criterion-1 metric names already corrected to `helix_*` in Phase 53; nothing further to fix.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `*obs.Metrics` registry (`internal/obs/metrics.go:57-186`) — `newMetrics()` builds every vector and registers them on a per-Provider `*prometheus.Registry`. The validation test (D-10) uses this same constructor to enumerate the inventory; no need to duplicate the family list.
- `obs.NewProvider(ctx, obs.Config{})` — returns a fully-wired Provider whose `Metrics()` is never nil (PROJECT.md noop-default invariant). The validation test calls this with a default Config to get the registry.
- `metrics_labels_test.go::carveOuts` map — already enumerates legitimate label-name carve-outs per family (`reason`, `result`, `extractor`, `phase`, `transport`, `outcome`, `strategy`). The D-14 label-name check imports this directly.
- `USAGE.md` §"Prometheus Metrics" — existing PromQL examples (hit-ratio for lspool, per-extractor p95 for repomap) are reusable verbatim as runbook Triage queries.
- `internal/obs/obs.go` `*obs.Provider` semantics — `Metrics()`/`Tracer()` always non-nil; the test does NOT need to gate on whether the admin listener is enabled.

### Established Patterns
- Single-registry-per-Provider — never touch the prometheus global registerer (T-11-05). The validation test instantiates a fresh Provider via `NewProvider(ctx, Config{})` per test run; do NOT use `prometheus.DefaultRegisterer`.
- Closed-enum carve-outs enforced at emission site (Phase 53) — this phase's PromQL filters use only the values that emission sites can produce. Planner verifies during research that PromQL like `helix_edit_outcome_total{strategy="ellipsis"}` (rejected in Phase 53 D-11 amendment) is NOT authored.
- Label NAME bound is enforced by CI lint; label VALUE bound is enforced at emission. PromQL can use regex (`result=~".*"`) without breaking either invariant.
- Test-from-disk pattern (D-12) — co-locate the test with what it validates; resolve project-root paths via `runtime.Caller(0)` so the test is invariant under `go test` invocation directory.

### Integration Points
- `deploy/` does not yet exist. This phase creates the top-level `deploy/grafana/` directory. Planner verifies `.gitignore` does not exclude it.
- `docs/` does not yet exist. This phase creates `docs/runbooks/` and `docs/images/`. Planner verifies `.gitignore` does not exclude them.
- `internal/obs/dashboards_test.go` (new) — co-located with `metrics.go` and `metrics_labels_test.go`. Plug into the existing test packaging — `go test ./internal/obs/...` runs it automatically.
- `USAGE.md` insertion point for D-21/D-22 is just before line 634 (`### Prometheus Metrics`). Insert two new H3 subsections (`### Grafana Dashboards`, `### Runbooks`) so Observability flows: Quickstart → Admin Listener → Dashboards → Runbooks → Prometheus Metrics → Tracing → pprof.
- No new external Go dependencies expected beyond `github.com/prometheus/prometheus/promql/parser` (D-11). Planner confirms during research whether it's already a transitive dep or needs an explicit `go get`. If a fresh top-level dependency, planner uses `go.mod` minimum-version pinning consistent with other prometheus deps already in `go.mod`.

</code_context>

<specifics>
## Specific Ideas

- **Hit-ratio panel** on `helix-engine.json` reuses the Phase 53 D-01 PromQL verbatim and is the single most-watched panel for "is caching working". It deserves a stat panel with thresholds (green ≥ 0.7, amber 0.4–0.7, red < 0.4) and a sparkline beside it.
- **Per-extractor p95 panel** on `helix-engine.json` reuses the Phase 53 D-05/D-06 PromQL verbatim. This is the panel that closes the OBS-03 narrative — operators answer "is the LSP fallback dragging us down for $LANG?" without leaving the dashboard.
- **Best-effort caveat panel** on `helix-overview.json` — a `text` panel (not a query panel) embedded in the session-lifecycle row, restating the Phase 53 D-09 caveat that `helix_session_lifecycle_total{transport="http",phase="ended"}` is best-effort. Operators reading the session graph in isolation should not be blindsided by this.
- **Runbook intros are short.** The H1 + frontmatter + one-paragraph intro gives the on-call engineer the context they need before the four H2 sections start. Treat the intro as the "what this is" sentence; everything else is the action surface.
- **`ErrCircuitOpen.md` triage example** should include the per-language circuit query: `helix_lspool_circuit_state == 2` to find which language's circuit is open, and the recent-eviction query: `rate(helix_lspool_evictions_total{reason=~"crash|pressure"}[5m])` to find the cause class.
- **`memory-pressure-eviction.md` remediation** should reference both the Linux (`/proc/pressure/memory`) and macOS (`vm_stat`) check commands, with a "your platform" note — the eviction code is platform-aware and operators on each platform need different shell commands.

</specifics>

<deferred>
## Deferred Ideas

- **Provisioning sidecar (docker-compose with Grafana + Prometheus pre-wired).** Operator-friendly first-run experience but heavier scope than OBS-01 demands. Revisit if first-time-user feedback says JSON-import-via-UI is too rough.
- **Alerting rules / Alertmanager integration.** Severity frontmatter on runbooks (D-15) is the on-ramp to a future alerting pack — runbooks already classify warning vs. critical. Belongs in a dedicated alerting phase.
- **Per-tool latency dashboard / per-language deep-dive dashboards.** Out of OBS-01 scope. The two-dashboard split (D-01) is intentionally minimal; specialized dashboards land later if operator demand surfaces.
- **Recapture screenshot via CI.** Today the screenshot is committed manually (D-20). Could be automated via a Grafana image-renderer in CI to keep it always-fresh, but adds CI infrastructure that isn't requested.
- **Embed runbook content in the daemon binary.** A `helix runbook ErrCircuitOpen` subcommand that prints the runbook from an embedded copy of `docs/runbooks/`. Nice ergonomic but no requirement; defer.
- **Cross-link USAGE.md ↔ runbooks directly from error messages.** Some daemon errors (`ErrCircuitOpen`, deadline timeouts) could include a runbook URL in their error text. Belongs in the typed-error work referenced in the metrics table comment ("v1.3 typed-error work").

</deferred>

---

*Phase: 54-obs-dashboards-runbooks*
*Context gathered: 2026-05-01*

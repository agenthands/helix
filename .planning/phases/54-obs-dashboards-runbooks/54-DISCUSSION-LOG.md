# Phase 54: obs-dashboards-runbooks - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-05-01
**Phase:** 54-obs-dashboards-runbooks
**Areas discussed:** Dashboard split + datasource, PromQL validation test, Runbook template + PromQL sync, Screenshot workflow

---

## Gray-area selection

| Option | Description | Selected |
|--------|-------------|----------|
| Dashboard split + datasource | How many dashboards, how sliced, datasource wiring | ✓ |
| PromQL validation test | How to satisfy success-criterion 4 | ✓ |
| Runbook template + PromQL sync | Shared skeleton + drift handling | ✓ |
| Screenshot workflow | Manual / scripted / skip | ✓ |

**User's choice:** All four areas.
**Notes:** No areas deferred. The four areas cover all four roadmap success criteria.

---

## Dashboard split

| Option | Description | Selected |
|--------|-------------|----------|
| Two dashboards: RED-overview + LSPool/Engine | Hits ≥2 minimum cleanly; primary + deep-dive split | ✓ |
| Three dashboards: RED + LSPool + RepoMap/Edits | More navigable, triples maintenance | |
| One mega-dashboard with row-folding | Single import, but violates ≥2 spec without a stub | |

**User's choice:** Two dashboards (`helix-overview.json` + `helix-engine.json`).
**Notes:** Captured as D-01. The follow-up question (workspace-activity panel ownership) was asked at the end and locked overview = RED + sessions, engine = lspool + repomap + edits.

---

## Datasource wiring

| Option | Description | Selected |
|--------|-------------|----------|
| `${DS_PROMETHEUS}` import template | Standard Grafana wizard pattern, portable | ✓ |
| Datasource by name + uid:prometheus | Hardcoded UID; fails on custom datasource names | |
| Provisioning sidecar (docker-compose) | First-run friendly, much heavier scope | |

**User's choice:** `${DS_PROMETHEUS}` import template.
**Notes:** Captured as D-05. Provisioning sidecar moved to Deferred Ideas.

---

## Template variables (drill-down)

| Option | Description | Selected |
|--------|-------------|----------|
| `$language` + `$instance` | Multi-select dropdowns for both | ✓ |
| `$language` only | Skip $instance because most operators run one daemon | |
| No template variables — flat panels with sum-by | Simplest JSON, worst drill-down | |

**User's choice:** `$language` + `$instance`.
**Notes:** Captured as D-08/D-09. No `$tool`/`$outcome`/`$strategy`/`$extractor` dropdowns — those are panel-level legend filters.

---

## PromQL validation test

| Option | Description | Selected |
|--------|-------------|----------|
| Parse JSON+MD with prometheus/prometheus parser, check names against live registry | AST-based, ~150 LOC | ✓ |
| Simple regex extraction + name allowlist | Lighter, no AST validation, drifts | |
| Run an in-process Prometheus + scrape + query each expression | Strongest, but heavy and slow | |

**User's choice:** prometheus/promql parser + live registry.
**Notes:** Captured as D-10–D-14. Test lives at `internal/obs/dashboards_test.go`. Label-NAME validation included (D-14); label-VALUE validation excluded (regex matchers complicate it without a clear failure mode).

---

## Runbook structure

| Option | Description | Selected |
|--------|-------------|----------|
| Shared 4-section template + YAML frontmatter | Predictable on-call experience, machine-readable metadata | ✓ |
| Shared 4-section template, no frontmatter | Saves ~5 lines per file | |
| Per-incident bespoke shape | Most flexible, no consistent on-call experience | |

**User's choice:** Shared 4-section template + YAML frontmatter.
**Notes:** Captured as D-15/D-16. Frontmatter fields locked: `title`, `severity` (warning|critical), `metric`, `since_phase`, `last_reviewed`. H2 order fixed: Symptoms → Triage → Likely Causes → Remediation. Each runbook also gets a final `## Code references` section.

---

## Runbook ↔ dashboard PromQL relationship

| Option | Description | Selected |
|--------|-------------|----------|
| Independent authoring + validation test catches drift | Runbooks and dashboards serve different needs | ✓ |
| Lint requires runbook PromQL = dashboard PromQL verbatim | Forces compromise on both sides | |
| Runbooks reference dashboards by panel ID, no inline PromQL | Useless without Grafana open mid-incident | |

**User's choice:** Independent authoring; validation test catches metric/label drift.
**Notes:** Captured as D-18/D-19. Runbook queries can use `topk()`, narrow histogram quantiles, etc., that wouldn't make a good panel. Phase 53's existing PromQL examples (hit-ratio, per-extractor p95) are reusable verbatim where they fit.

---

## Screenshot workflow

| Option | Description | Selected |
|--------|-------------|----------|
| Manual one-time PNG, committed to docs/images/ | Re-capture on visible drift, no CI infra | ✓ |
| Scripted: docker-compose Grafana + image-renderer | Reproducible, slow, adds Grafana dev dependency | |
| Skip the screenshot — link only | Violates success-criterion 3 explicitly | |

**User's choice:** Manual PNG in `docs/images/helix-overview-dashboard.png`.
**Notes:** Captured as D-20. Capture procedure documented in the phase plan: spin up Grafana 10+, populate registry by running benchmarks + exercising tools, screenshot at 1600×900. Re-capture on visible drift only — `last_reviewed` frontmatter on runbooks gives an explicit checkpoint pattern that the screenshot can mirror.

---

## Runbook code-reference depth

| Option | Description | Selected |
|--------|-------------|----------|
| File:line references with brief context | Operators learn where to look without grepping | ✓ |
| Filenames only, no line numbers | No line drift, less precise | |
| No code section — operator-facing only | Cleaner separation, loses engineer on-ramp | |

**User's choice:** File:line references with brief context.
**Notes:** Captured as D-17. Line drift is acceptable — broken `file:line` link breaks loudly when an engineer follows it, and `last_reviewed` frontmatter gives an explicit refresh checkpoint.

---

## Workspace-activity panel ownership

| Option | Description | Selected |
|--------|-------------|----------|
| Overview owns sessions; engine owns lspool+repomap+edits | Maps cleanly to roadmap RED+activity framing | ✓ |
| Overview = RED only; engine owns sessions + everything else | Cleaner conceptual split, pushes sessions off screenshot | |

**User's choice:** Overview owns sessions.
**Notes:** Captured as D-02/D-03. Session churn is a legitimate at-a-glance signal for operators; pushing it to the engine dashboard would weaken the overview's value as the screenshot target.

---

## Claude's Discretion

The following details are within the planner's authority once research is complete:

- Final panel count per dashboard (within the 6–9 band, D-04).
- Exact PromQL strings for each panel (must use only the registered metrics in `<canonical_refs>` and pass the D-10–D-14 validation test).
- Exact `file:line` numbers for the runbook code-references sections — confirmed during research.
- Whether `github.com/prometheus/prometheus/promql/parser` is already a transitive dependency or needs an explicit `go get`.
- Specific stat-panel thresholds for the hit-ratio panels (currently sketched as green ≥ 0.7, amber 0.4–0.7, red < 0.4 in `<specifics>`).
- Choice of `timeseries` vs. `barchart` vs. `stat` for individual panels, within the Grafana stock kit (D-24).

## Deferred Ideas

- Provisioning sidecar (docker-compose with Grafana + Prometheus pre-wired) — heavier scope, revisit on first-run feedback.
- Alerting rules / Alertmanager integration — runbook severity frontmatter is the on-ramp; lands in a dedicated alerting phase.
- Per-tool latency dashboards / per-language deep-dive dashboards — out of OBS-01 scope.
- Recapture screenshot via CI — Grafana image-renderer in CI; not requested.
- Embed runbook content in the daemon binary (`helix runbook ErrCircuitOpen` subcommand) — nice ergonomic, no requirement.
- Cross-link USAGE.md ↔ runbooks from error messages — belongs in v1.3 typed-error work.

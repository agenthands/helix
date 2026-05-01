# Phase 54: obs-dashboards-runbooks - Pattern Map

**Mapped:** 2026-05-01
**Files analyzed:** 10 (8 new + 2 modified)
**Analogs found:** 5 / 10 (5 establish new patterns; in-repo precedent does not exist for those)

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/obs/dashboards_test.go` | test (validation) | batch / file-read | `internal/obs/metrics_labels_test.go` | exact (same package, same registry pattern) |
| `deploy/grafana/helix-overview.json` | operator artifact (Grafana JSON) | static-config | RESEARCH §C.1 skeleton (no in-repo precedent) | new pattern |
| `deploy/grafana/helix-engine.json` | operator artifact (Grafana JSON) | static-config | `deploy/grafana/helix-overview.json` (sibling, authored same wave) | sibling-mirror |
| `docs/runbooks/ErrCircuitOpen.md` | operator artifact (Markdown) | doc | RESEARCH §C.4 skeleton (no in-repo precedent) | new pattern |
| `docs/runbooks/deadline-timeouts.md` | operator artifact (Markdown) | doc | `docs/runbooks/ErrCircuitOpen.md` (sibling) | sibling-mirror |
| `docs/runbooks/ls-crash-restart.md` | operator artifact (Markdown) | doc | `docs/runbooks/ErrCircuitOpen.md` (sibling) | sibling-mirror |
| `docs/runbooks/memory-pressure-eviction.md` | operator artifact (Markdown) | doc | `docs/runbooks/ErrCircuitOpen.md` (sibling) | sibling-mirror |
| `docs/images/helix-overview-dashboard.png` | binary asset (manual) | static | none | new pattern |
| `USAGE.md` (modified) | doc | doc | existing `### Health Checks` / `### Prometheus Metrics` H3 blocks | role-match |
| `go.mod` / `go.sum` (modified) | build manifest | static | existing `prometheus/client_golang` direct-dep entries | role-match |

## Pattern Assignments

### `internal/obs/dashboards_test.go` (test, validation)

**Analog:** `internal/obs/metrics_labels_test.go` — same package, same registry-priming pattern, same fail-loud diff style.

**Imports pattern** (mirror lines 1-9 of `metrics_labels_test.go`, augmented):
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
```
Note: same package (`obs`, NOT `obs_test`) — gives direct access to `AllowedLabels` (metrics.go:28), `carveOuts` (metrics_labels_test.go:22-45), `runtimeFamilyPrefixes` (metrics_labels_test.go:51), `isRuntimeFamily` (line 53), `newMetrics()` (metrics.go:82). Reuse, do NOT duplicate these.

**Registry construction pattern** (mirror metrics_labels_test.go:107):
```go
m := newMetrics()
```
NOT `obs.NewProvider(ctx, obs.Config{})` — CONTEXT.md D-10 mentions `NewProvider` but `metrics_labels_test.go` already uses the lower-level `newMetrics()` constructor (in-package access). RESEARCH §C.2 confirms the equivalent. Pick one and stick with it; `newMetrics()` is the lower-friction match for the in-package test. **Decision for planner:** use `newMetrics()` — it is the verbatim sibling pattern and avoids importing `context`/`Config{}` for a function call that produces an identical registry.

**Vector priming (MANDATORY — Pitfall #6)** (verbatim from metrics_labels_test.go:111-125):
```go
m.ToolCalls.WithLabelValues("t", "p", "m", "go", "success").Inc()
m.ToolDuration.WithLabelValues("t", "p", "m", "go").Observe(0.001)
m.LSPoolWorkers.WithLabelValues("go").Set(1)
m.LSPoolEvictions.WithLabelValues("go", "idle").Inc()
m.LSPoolCircuitState.WithLabelValues("go").Set(0)
m.LSPoolRestarts.WithLabelValues("go").Inc()
m.RenameStrategy.WithLabelValues("lsp-native").Inc()
m.LSPoolLookups.WithLabelValues("go", "hit").Inc()
m.RepoMapLookups.WithLabelValues("go", "hit").Inc()
m.RepoMapExtract.WithLabelValues("go", "treesitter").Observe(0.001)
m.SessionLifecycle.WithLabelValues("started", "stdio").Inc()
m.EditOutcome.WithLabelValues("replace_symbol_body", "success", "exact").Inc()
```
Open Question 5 in RESEARCH resolves to: copy-paste this block into the new test (do NOT refactor metrics_labels_test.go in this phase). Consider extracting to an unexported `primeAllVectors(t, m)` helper used by both tests in a future cleanup phase, but NOT now.

**Gather → family-set pattern** (mirror metrics_labels_test.go:67-83 from `lintLabels`):
```go
mfs, err := m.Registry().Gather()
if err != nil {
    t.Fatalf("Gather: %v", err)
}
families := map[string]bool{}
for _, mf := range mfs {
    families[mf.GetName()] = true
}
```

**AllowedLabels reuse pattern** (verbatim from metrics_labels_test.go:73-76):
```go
allowed := map[string]bool{}
for _, l := range AllowedLabels {
    allowed[l] = true
}
```
The test then consults `carveOuts[familyName]` (metrics_labels_test.go:22-45) per-vector-selector — same lookup the existing test uses. Add `"le"`, `"job"`, `"__name__"` to a per-test internal allowlist for VectorSelector matchers (these are PromQL-internal, not application labels — RESEARCH Pitfall #7 + §C.2 sketch).

**Drift-detection companion test** (mirror metrics_labels_test.go:139-164 `TestMetricsLabelsAllowlist_catchesDrift`):
The new file MUST include a `TestDashboardsAndRunbooksReferenceRegisteredMetrics_catchesDrift` companion that proves the validator catches at least ONE failure mode (e.g., a synthetic invalid PromQL string with a typo'd metric name produces a `t.Errorf`). Pattern from line 139:
```go
// _catchesDrift is the negative proof: deliberate bad input must be flagged.
// If this test ever passes silently, the validator is broken.
```
RESEARCH §F lists four mutation patterns; pick one (typo'd metric name is simplest) and codify it as the drift test.

**Fail-closed empty-dir pattern** (no direct analog; new):
```go
if len(dashes) == 0 {
    t.Fatalf("no dashboards found at %s", dashGlob)
}
```
Mirrors the spirit of `gatherFamily` returning `nil` followed by `t.Fatal` (metrics_labels_test.go:184-194). Same fail-loud-on-missing pattern.

**Path resolution** (verbatim from RESEARCH §C.2):
```go
_, file, _, ok := runtime.Caller(0)
// internal/obs/dashboards_test.go → ../../.. = repo root
return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
```
NOT `os.Getwd()` (varies by `go test` invocation dir). Use `filepath.Glob` not string-split (Pitfall #4 — Windows compat).

**Histogram suffix stripping** (no analog; new — RESEARCH §C.2):
```go
for _, sfx := range []string{"_bucket", "_count", "_sum"} {
    if strings.HasSuffix(name, sfx) {
        return strings.TrimSuffix(name, sfx)
    }
}
```

**Runtime-family allowlist** (reuse metrics_labels_test.go:51-60):
```go
// `up`, `go_*`, `process_*` are not in the helix registry but appear in
// dashboard PromQL (template variables, runtime panels). Reuse isRuntimeFamily
// for the prefix check; add a small literal allowlist for `up`.
if base == "up" || isRuntimeFamily(base) { /* skip family check */ }
```

---

### `deploy/grafana/helix-overview.json` (Grafana JSON)

**Analog:** none in repo. Establishes the `deploy/grafana/` template. Use RESEARCH §C.1 skeleton verbatim.

**Mandatory top-level keys** (RESEARCH §A.3, §C.1):
- `__inputs` — single `DS_PROMETHEUS` datasource entry
- `__requires` — Grafana 10.0.0 + Prometheus datasource + every panel-type plugin used
- `schemaVersion: 38` (Grafana 10.0 baseline; bump to 39 only if a needed feature requires it)
- `tags: ["helix", "observability"]`
- `uid: "helix-overview"` (unique per dashboard; second file uses `helix-engine`)
- `templating.list` — two entries: `language`, `instance` (RESEARCH §A.4 verbatim)

**Per-target datasource pattern** (verbatim from §C.1):
```json
"datasource": { "type": "prometheus", "uid": "${DS_PROMETHEUS}" }
```
EVERY `targets[]` entry AND the panel-level `datasource` field must use this template — never a hardcoded UID.

**Stat panel `reduceOptions.calcs` (MANDATORY — Pitfall #2):**
```json
"reduceOptions": { "calcs": ["lastNotNull"], "fields": "", "values": false }
```
Omitting this renders the panel blank.

**Panel kit constraint** (D-24):
Use only `timeseries`, `stat`, `gauge`, `barchart`. No `row` containers, no custom plugins.

**Panel inventory** (RESEARCH §A.2 — copy PromQL verbatim):
- Tool-call rate (timeseries)
- Error rate by outcome (timeseries)
- p50/p95/p99 latency (timeseries — three quantile targets in one panel)
- Session start rate (timeseries)
- Session error rate (stat)
- Best-effort caveat callout (text panel — RESEARCH §C.1)

**Best-effort caveat text panel** (RESEARCH §C.1 verbatim) — content lifted directly from USAGE.md:706 prose:
```
helix_session_lifecycle_total{transport="http", phase="ended"} only fires
on a client-issued DELETE /mcp. Sessions that disappear due to server-side
timeout do NOT register. See Phase 53 D-09 / USAGE.md.
```

---

### `deploy/grafana/helix-engine.json` (Grafana JSON)

**Analog:** `deploy/grafana/helix-overview.json` (sibling — same skeleton, different panels and uid).

**Differences from sibling:**
- `uid: "helix-engine"`
- `title: "Helix Engine"`
- Different `panels[]` (RESEARCH §A.2 — engine row): LS workers by language, eviction rate by reason, circuit state per language, restart rate, lspool hit-ratio (Phase 53 D-01 PromQL **verbatim** — see USAGE.md:696-697), repomap hit-ratio, per-extractor p95 (Phase 53 D-05/D-06 **verbatim** — USAGE.md:703), edit-tool outcome rate, rename strategy share.

**Verbatim PromQL re-use (D-18 reuse Phase 53 examples):**
```promql
# Reused as-is from USAGE.md:696-697
sum(rate(helix_lspool_lookups_total{result="hit"}[5m])) / sum(rate(helix_lspool_lookups_total[5m]))

# Reused as-is from USAGE.md:703
histogram_quantile(0.95, sum by (le, extractor) (rate(helix_repomap_extract_duration_seconds_bucket[5m])))
```

**Hit-ratio stat panel thresholds** (RESEARCH §C.1, Open Q 2 default):
```json
"thresholds": { "mode": "absolute", "steps": [
  { "color": "red", "value": null },
  { "color": "yellow", "value": 0.4 },
  { "color": "green", "value": 0.7 }
]}
```

---

### `docs/runbooks/ErrCircuitOpen.md` (Markdown runbook)

**Analog:** none in repo. Establishes the runbook template. RESEARCH §C.4 is the verbatim copy-paste base.

**Mandatory frontmatter (D-15) — verbatim from RESEARCH §C.4:**
```yaml
---
title: <human-readable runbook title>
severity: warning | critical
metric: <primary helix_* metric this runbook keys off>
since_phase: <phase number>
last_reviewed: 2026-05-01
---
```
Severity is a closed-enum: `warning` for "investigate" (eviction-rate drift, hit-ratio drop), `critical` for "act now" (circuit open, repeated crashes).

**Mandatory H2 order (D-16):**
```
# <title>

<one-paragraph intro, ≤ 4 sentences per Open Q 6>

## Symptoms
## Triage
## Likely Causes
## Remediation
## Code references
```

**PromQL fence convention (Pitfall #8):** lowercase ```` ```promql ```` ONLY. Uppercase variant is rejected by the validator regex.

**Triage block format** (RESEARCH §C.4):
````markdown
```promql
# Which language's circuit is currently open?
helix_lspool_circuit_state == 2
```
*Filter the result to find the affected language.*
````
One-line italic interpretation under each fenced query. Comments inside the fence (lines starting `#`) are stripped by the extractor before AST parsing — safe to use freely (Pitfall #9).

**Code references format** (D-17, RESEARCH §A.5):
```
- `internal/kernel/lspool/circuit.go:33` — NewCircuitBreaker; restartBudget default = 3
- `internal/kernel/lspool/pool.go:22` — RestartBudget config knob
```
Bulleted `path:line — context`. Line numbers will drift; `last_reviewed` frontmatter is the explicit checkpoint.

**Triage queries (RESEARCH §A.6 — copy verbatim):**
```promql
helix_lspool_circuit_state == 2
sum by (language, reason) (rate(helix_lspool_evictions_total{reason=~"crash|pressure"}[5m]))
sum by (language) (rate(helix_tool_calls_total{outcome="circuit_open"}[5m]))
topk(5, sum by (language) (rate(helix_lspool_restarts_total[5m])))
```

**Code references (RESEARCH §A.5 — re-verify line numbers at task time via `mcp__smtc__find_declarations`):**
- `internal/kernel/lspool/circuit.go:33` — NewCircuitBreaker
- `internal/kernel/lspool/circuit.go:64` — RecordFailure (decorrelated jitter)
- `internal/kernel/lspool/circuit.go:130` — OpenError
- `internal/kernel/lspool/pool.go:22` — RestartBudget config knob
- `internal/kernel/lspool/pool.go:320` — `cb = NewCircuitBreaker(...)` call site

---

### `docs/runbooks/deadline-timeouts.md` (Markdown runbook)

**Analog:** `docs/runbooks/ErrCircuitOpen.md` (sibling — same frontmatter + 4-section H2 + Code references structure).

**Frontmatter values:**
- `title: Deadline / timeout outcomes`
- `severity: warning`
- `metric: helix_tool_calls_total{outcome="timeout"}`
- `since_phase: 11`

**Triage queries (RESEARCH §A.6 verbatim):**
```promql
sum by (tool_name) (rate(helix_tool_calls_total{outcome="timeout"}[5m]))
sum by (tool_name) (rate(helix_tool_calls_total{outcome="timeout"}[5m])) / sum by (tool_name) (rate(helix_tool_calls_total[5m]))
histogram_quantile(0.95, sum by (le, tool_name) (rate(helix_tool_duration_seconds_bucket[5m])))
```

**Code references (RESEARCH §A.5):**
- `internal/mcp/middleware.go:113` — `BudgetFunc` type
- `internal/mcp/middleware.go:277` — `TelemetryMiddleware`
- `internal/mcp/middleware.go:304-313` — deadline injection block (`context.WithTimeout`)
- `internal/config/config.go:28-32` — `DegradationConfig` knobs
- `internal/config/defaults.go:25-29` — default values (5s/15s/10s/120s/20s)
- `USAGE.md:818-820` — operator-facing knob table

---

### `docs/runbooks/ls-crash-restart.md` (Markdown runbook)

**Analog:** `docs/runbooks/ErrCircuitOpen.md`.

**Frontmatter values:**
- `title: LS worker crash / restart`
- `severity: critical`
- `metric: helix_lspool_evictions_total{reason="crash"}`
- `since_phase: 11`

**Triage queries (RESEARCH §A.6):**
```promql
sum by (language) (rate(helix_lspool_evictions_total{reason="crash"}[5m]))
sum by (language) (rate(helix_lspool_restarts_total[5m]))
helix_lspool_workers{language=~"$language"}
```

**Code references (RESEARCH §A.5):**
- `internal/kernel/lspool/pool.go:414-416` — crash booking via `evictWorkerLocked(id, w, EvictCrash)`
- `internal/kernel/lspool/pool.go:306` — `RecordSuccess` resets failure counter
- `internal/kernel/lspool/worker.go:434` — handler-panic recovery
- `internal/kernel/lspool/circuit.go:64` — backoff cadence

---

### `docs/runbooks/memory-pressure-eviction.md` (Markdown runbook)

**Analog:** `docs/runbooks/ErrCircuitOpen.md`.

**Frontmatter values:**
- `title: Memory pressure eviction`
- `severity: warning`
- `metric: helix_lspool_evictions_total{reason="pressure"}`
- `since_phase: 11`

**Platform-specific Remediation block** (specific to this runbook — RESEARCH §A.6):
```markdown
- **Linux:** `cat /proc/pressure/memory` — interpret avg10 column
- **macOS:** `vm_stat 5` — watch free vs. inactive page counts
```

**Code references (RESEARCH §A.5):**
- `internal/kernel/lspool/pressure_linux.go:22` — `os.ReadFile("/proc/pressure/memory")` (PSI)
- `internal/kernel/lspool/pressure_darwin.go:24` — `exec.Command("vm_stat")`
- `internal/kernel/lspool/pool.go:403, 432, 452` — `evictWorkerLocked(..., EvictPressure)` call sites
- `internal/kernel/lspool/pressure.go:9-15` — `PressureLevel` enum
- `internal/kernel/lspool/metrics.go:38` — `EvictPressure = "pressure"` constant

---

### `docs/images/helix-overview-dashboard.png` (binary asset)

**Analog:** none. Manual one-time capture per D-20.

**Capture procedure** (CONTEXT D-20, RESEARCH Pitfall #11):
1. Local Grafana 10+ wired to Prometheus scraping helix admin listener.
2. Run `make bench` (`./test/bench/...`) **locally only** — never CI per global memory.
3. Exercise a few MCP tools (e.g., `mcp__helix__find_references` on the helix repo).
4. Import `helix-overview.json` (Grafana prompts for `DS_PROMETHEUS`).
5. Screenshot at 1600×900 (default Grafana viewport).
6. Commit. Re-capture only on visible drift.

---

### `USAGE.md` (modified — insert two H3 subsections)

**Analog:** existing `### Health Checks` (USAGE.md:622) and `### Prometheus Metrics` (USAGE.md:634) H3 blocks under `## Observability Quickstart`.

**Insertion point:** immediately BEFORE `### Prometheus Metrics` (line 634). Resulting flow:
```
## Observability Quickstart
### Enable the Admin Listener
### Health Checks
### Grafana Dashboards    ← NEW (D-21)
### Runbooks              ← NEW (D-22)
### Prometheus Metrics    ← existing line 634
### Enable Tracing        ← existing line 708
```

**`### Grafana Dashboards` content pattern** (D-21):
```markdown
### Grafana Dashboards

![Helix overview dashboard](docs/images/helix-overview-dashboard.png)

Two dashboards live in `deploy/grafana/`:

- [`helix-overview.json`](deploy/grafana/helix-overview.json) — RED metrics + workspace activity. Primary operator view.
- [`helix-engine.json`](deploy/grafana/helix-engine.json) — lspool, repomap, edit-tool internals. On-call deep-dive.

In Grafana, **Dashboards → New → Import**, upload the JSON, and select your
Prometheus datasource when prompted for `DS_PROMETHEUS`.

The dashboards expose `$language` and `$instance` template variables for
drill-down — leave both at `All` for a global view.
```

**`### Runbooks` content pattern** (D-22):
```markdown
### Runbooks

Operational runbooks for the four most common Helix failure modes live in `docs/runbooks/`:

- [`ErrCircuitOpen.md`](docs/runbooks/ErrCircuitOpen.md) — circuit breaker tripped on a language pool.
- [`deadline-timeouts.md`](docs/runbooks/deadline-timeouts.md) — tool calls hitting the configured deadline.
- [`ls-crash-restart.md`](docs/runbooks/ls-crash-restart.md) — language server process crashing.
- [`memory-pressure-eviction.md`](docs/runbooks/memory-pressure-eviction.md) — workers evicted under memory pressure.

Each runbook lists symptoms, triage PromQL, likely causes, and remediation steps.
```

**H3 style mirror** (USAGE.md:622-632 `### Health Checks` shape): heading, one-paragraph framing, bullet/code block, no nested H4. Match the surrounding markdown register.

---

### `go.mod` / `go.sum` (modified — add `github.com/prometheus/prometheus@v3.11.x`)

**Analog:** existing direct-dep entries `github.com/prometheus/client_golang v1.23.2`, `github.com/prometheus/client_model v0.6.2` (RESEARCH §B.1).

**Pattern:**
```bash
go get github.com/prometheus/prometheus@v3.11.0
go mod tidy
go list -m all | grep prometheus
```
Pin to `v3.11.0` minimum; `v3.11.x` patches are non-breaking. Apache-2.0 license — same as existing prom deps. Verify `make verify-embed-pubkey` and `tools/audit-embed.go` are unaffected (RESEARCH Pitfall #12). Document `go.sum` line growth at Wave 0; flag if > 100 lines added.

---

## Shared Patterns

### Closed-enum label discipline (validator + emission)
**Source:** `internal/obs/metrics.go:248-305` (drop-unknown helpers like `LSPoolLookup`, `RepoMapLookup`, `RepoMapExtractObserve`, `SessionLifecycleInc`, `EditOutcomeInc`).
**Apply to:** every PromQL expression authored in dashboards and runbooks. Filter values MUST come from the closed enums these helpers emit. Do NOT invent new values like `result="warm"` or `strategy="ellipsis"` (the latter explicitly rejected per Phase 53 D-11 amendment).

### Single-registry-per-Provider, never the prom default
**Source:** `internal/obs/metrics.go:82-83` (`reg := prometheus.NewRegistry() // owned; NOT the prometheus global registerer`).
**Apply to:** `dashboards_test.go` — call `newMetrics()` for a fresh isolated registry per test run. Never touch `prometheus.DefaultRegisterer`. T-11-05 mitigation.

### Vector priming before Gather
**Source:** `internal/obs/metrics_labels_test.go:107-125` (12 priming calls covering every helix vector).
**Apply to:** `dashboards_test.go` (same priming list — copy verbatim). Without priming, Gather() drops empty families and the validator produces false negatives (Pitfall #6).

### `_catchesDrift` companion test convention
**Source:** `internal/obs/metrics_labels_test.go:139-164` (`TestMetricsLabelsAllowlist_catchesDrift`).
**Apply to:** `dashboards_test.go` — add `TestDashboardsAndRunbooksReferenceRegisteredMetrics_catchesDrift` so the validator's negative path is proven. Future-Claude reads the suffix as "this is the proof the validator works".

### `runtime.Caller(0)` + `filepath.Join(..., "../../..")` for repo-root resolution
**Source:** No exact in-repo precedent (the existing tests use registry-only inputs); pattern lifted from RESEARCH §C.2.
**Apply to:** `dashboards_test.go` — invariance under `go test` invocation cwd; use `filepath.Glob` for cross-platform path handling (Pitfall #4).

### USAGE.md H3 subsection style
**Source:** `USAGE.md:622-632` (`### Health Checks`), `USAGE.md:634-666` (`### Prometheus Metrics`).
**Apply to:** the two new H3 subsections. Heading, one-paragraph framing, bullet list or fenced code block, no nested H4.

## No Analog Found

| File | Role | Reason |
|------|------|--------|
| `deploy/grafana/helix-overview.json` | Grafana JSON | No prior Grafana JSON in repo. Establishes the template; sibling dashboard mirrors it. RESEARCH §C.1 is the verbatim base. |
| `docs/runbooks/ErrCircuitOpen.md` | Markdown runbook | No prior runbook in repo. Establishes the YAML-frontmatter + 4-section H2 template; the other three runbooks mirror it. RESEARCH §C.4 is the verbatim base. |
| `docs/images/helix-overview-dashboard.png` | binary asset | First screenshot in repo. Manual capture per D-20 procedure. |
| `deploy/` directory itself | top-level dir | Net-new top-level. Verify `.gitignore` does not exclude (CONTEXT integration note). |
| `docs/` directory itself | top-level dir | Net-new top-level. Verify `.gitignore` does not exclude. |

## Metadata

**Analog search scope:**
- `internal/obs/` — registry, providers, label tests
- `internal/kernel/lspool/` — runbook code-reference targets (verified by RESEARCH SMTC pass)
- `internal/mcp/middleware.go` — runbook code-reference target
- `internal/config/` — degradation timeout knobs
- `USAGE.md` lines 600-708 — Observability Quickstart layout
- `deploy/`, `docs/` — confirmed empty (these directories do not yet exist)

**Files scanned:** ~12 source files + USAGE.md + RESEARCH/CONTEXT.

**Pattern extraction date:** 2026-05-01.

**Re-verification reminder for planner:** `file:line` anchors in §A.5 of RESEARCH and the runbook Code-references blocks above are valid as of 2026-05-01 grep. Re-verify at task time via `mcp__smtc__find_declarations` / `mcp__smtc__goto_definition` (CLAUDE.md SMTC-first routing).

## PATTERNS COMPLETE

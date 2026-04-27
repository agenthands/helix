---
phase: 54
slug: obs-dashboards-runbooks
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-27
---

# Phase 54 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib) |
| **Config file** | `go.mod` (Go modules; no separate test config) |
| **Quick run command** | `go test ./internal/obs/... -run TestDashboards -count=1` |
| **Full suite command** | `go vet ./... && go test ./... -count=1` |
| **Estimated runtime** | ~30s quick / ~3 min full |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/obs/... -run TestDashboards -count=1`
- **After every plan wave:** Run `go vet ./... && go test ./... -count=1`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds (quick) / 180 seconds (full)

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 54-01-01 | 01 | 0 | OBS-01, OBS-02 | — | Closed-enum allowlist exported once, no test-file imports | unit | `go test ./internal/obs/... -run TestClosedEnumLabelValues -count=1` | ❌ W0 | ⬜ pending |
| 54-01-02 | 01 | 0 | OBS-01, OBS-02 | — | dashboards_test.go skeleton fails (proves wiring) | unit | `go test ./internal/obs/... -run TestDashboards -count=1` (must fail RED) | ❌ W0 | ⬜ pending |
| 54-01-03 | 01 | 0 | OBS-02 | — | `outcome="timeout"` confirmed emitted in middleware before runbook locks query | grep | `grep -rn 'outcome.*"timeout"' internal/mcp/` | ✅ | ⬜ pending |
| 54-02-01 | 02 | 1 | OBS-01 | — | serena-overview.json imports cleanly into Grafana 10+ schemaVersion 39 | unit | `go test ./internal/obs/... -run TestDashboards -count=1` | ❌ W0 | ⬜ pending |
| 54-02-02 | 02 | 1 | OBS-01 | — | serena-lspool.json same | unit | `go test ./internal/obs/... -run TestDashboards -count=1` | ❌ W0 | ⬜ pending |
| 54-03-01 | 03 | 1 | OBS-02 | — | Each runbook PromQL fence parses + every metric in registry | unit | `go test ./internal/obs/... -run TestDashboards -count=1` | ❌ W0 | ⬜ pending |
| 54-04-01 | 04 | 2 | OBS-01 | — | USAGE.md links to deploy/grafana/ + screenshot ref present | grep | `grep -n 'docs/img/grafana-overview.png' USAGE.md` | ✅ | ⬜ pending |
| 54-05-01 | 05 | 3 | OBS-01, OBS-02 | — | Full suite green; PromQL test counts >0 expressions | unit+manual | `go vet ./... && go test ./... -count=1` | ✅ | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/obs/labelsets.go` — production file exporting `ClosedEnumLabelValues` map (result/scope/phase/outcome/reason). Refactor existing carve-outs in `metrics_labels_test.go` to consume it (no behavior change to existing tests).
- [ ] `internal/obs/dashboards_test.go` — initial skeleton: walk `deploy/grafana/*.json` + `docs/runbooks/*.md`, parse PromQL via `parser.NewParser(parser.Options{}).ParseExpr`, assert metric names in `obs.New(...).Registry().Gather()`, assert closed-enum label values in `ClosedEnumLabelValues`. Skeleton runs with zero artifacts and exits cleanly (or fails RED with a clear "no dashboards/runbooks found yet" message that turns into pass once artifacts exist).
- [ ] `go.mod` — add `github.com/prometheus/prometheus` (parser package only) at the version pinned in research (v0.311.3 or later stable).

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| `serena-overview.json` imports cleanly into Grafana 10+ via Dashboards → Import → Upload JSON file | OBS-01 | Cannot run a real Grafana instance from `go test` without heavyweight runtime dep (rejected in CONTEXT D-17) | (1) Stand up Grafana 10.x locally (docker run grafana/grafana:10.4.0). (2) Add Prometheus datasource. (3) Import each JSON file. (4) Confirm no parse errors and panels render against the chosen Prometheus. (5) Capture `docs/img/grafana-overview.png` (~1600px wide). |
| Runbook deep links resolve when prefixed with `${GRAFANA}` host | OBS-02 | URL routing depends on the operator's Grafana base URL | Document the convention in `docs/runbooks/README.md`; spot-check at least one `?viewPanel=N` link against an imported dashboard. |
| Screenshot drift comment present and recipe documented | OBS-01 | Subjective drift judgment | Visual inspection: `<!-- Regenerate when serena-overview.json panel set changes. -->` precedes the image; recipe exists in `docs/runbooks/README.md`. |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references (labelsets.go, dashboards_test.go skeleton, go.mod dep)
- [ ] No watch-mode flags
- [ ] Feedback latency < 180s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending

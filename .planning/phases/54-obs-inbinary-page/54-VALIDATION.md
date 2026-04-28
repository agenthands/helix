---
phase: 54
slug: obs-inbinary-page
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-28
---

# Phase 54 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — stdlib testing |
| **Quick run command** | `go test ./internal/daemon/... -run TestStatusPage -count=1` |
| **Full suite command** | `go test ./... && go vet ./...` |
| **Estimated runtime** | ~30 seconds (quick) / ~3 minutes (full) |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/daemon/... -run TestStatusPage -count=1`
- **After every plan wave:** Run `go test ./... && go vet ./...`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 54-01-01 | 01 | 1 | OBS-01 | — | render HTML on `/`, no PII in output | unit | `go test ./internal/daemon -run TestStatusPageRender -count=1` | ❌ W0 | ⬜ pending |
| 54-01-02 | 01 | 1 | OBS-01 | — | route narrows `/` to literal path, returns 404 for other unknown paths | unit | `go test ./internal/daemon -run TestStatusPageRouting -count=1` | ❌ W0 | ⬜ pending |
| 54-01-03 | 01 | 1 | OBS-01 | — | walks live `prometheus.Registry` (same as `/metrics`) | unit | `go test ./internal/daemon -run TestStatusPageRegistryShared -count=1` | ❌ W0 | ⬜ pending |
| 54-02-01 | 02 | 2 | OBS-02 | — | runbook files exist, contain required sections, free of forbidden Grafana/PromQL strings | unit | `go test ./docs/runbooks -run TestRunbookCompliance -count=1` | ❌ W0 | ⬜ pending |
| 54-03-01 | 03 | 2 | OBS-01, OBS-02 | — | USAGE.md observability section references `http://127.0.0.1:9100/` and `docs/runbooks/`; no third-party prerequisites | unit | `go test ./docs -run TestUSAGEObservability -count=1` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/daemon/status_page_test.go` — stubs for OBS-01 (render, routing, shared registry)
- [ ] `docs/runbooks/runbooks_test.go` — compliance test stubs (file existence + forbidden-string grep)
- [ ] `docs/usage_test.go` — USAGE.md observability section assertions

*Existing test infrastructure (`go test`) covers; no new framework required.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Visual rendering of HTML page in a real browser | OBS-01 | Visual fidelity, inline CSS layout, readability | Start daemon with `--admin-addr 127.0.0.1:9100`, open `http://127.0.0.1:9100/` in Chrome/Safari/Firefox, confirm all 7 sections render and refresh on F5 |
| Runbook readability for an on-call operator | OBS-02 | Subjective — "could a stressed operator triage from this?" | Manual reading pass on the 4 runbooks against criterion #3 |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending

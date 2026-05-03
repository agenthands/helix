---
phase: 54-obs-dashboards-runbooks
plan: 05
subsystem: observability/docs
tags: [observability, documentation, screenshot, usage, grafana, runbooks]
requires: [54-02, 54-03, 54-04]
provides:
  - "USAGE.md ### Grafana Dashboards subsection (operator entry point to deploy/grafana/)"
  - "USAGE.md ### Runbooks subsection (operator entry point to docs/runbooks/)"
  - "docs/images/helix-overview-dashboard.png placeholder slot (real capture deferred — see Deferred section)"
affects:
  - USAGE.md
  - docs/images/helix-overview-dashboard.png
tech-stack:
  added: []
  patterns:
    - "Markdown image embed with project-relative path"
    - "Markdown link bullets to JSON dashboard files and runbook .md files"
key-files:
  created:
    - docs/images/helix-overview-dashboard.png  # 1x1 placeholder; pending real 1600x900 capture
  modified:
    - USAGE.md  # +26 lines: ### Grafana Dashboards + ### Runbooks H3 blocks before ### Prometheus Metrics
decisions:
  - "Took the deferral path documented in plan acceptance_criteria: committed a 1x1 PNG placeholder rather than blocking the autonomous chain on a manual Grafana screenshot. This satisfies the file-magic / extension acceptance gate while making the gap loudly visible (image renders broken or as a 1x1 dot in any markdown viewer)."
  - "Pure-insert edit to USAGE.md: zero existing lines modified; verified via `git diff USAGE.md | awk '/^-[^-]/'` returning empty."
metrics:
  duration: ~5 min
  completed: 2026-05-01
  tasks_completed: 2
  files_changed: 2
---

# Phase 54 Plan 05: USAGE.md Discoverability + Dashboard Screenshot Summary

USAGE.md now exposes the Grafana dashboards and runbooks as first-class operator entry points immediately before the existing Prometheus Metrics subsection; the dashboard screenshot slot is committed as a 1x1 PNG placeholder pending a one-time manual capture against a populated local Grafana 10+ instance.

## What Shipped

### Task 1: docs/images/helix-overview-dashboard.png (DEFERRED — placeholder committed)

- File created at the canonical path: `docs/images/helix-overview-dashboard.png`.
- Format: PNG image data, 1x1 RGBA, 68 bytes (verified via `file(1)`).
- This is a **placeholder**, not a real capture. The plan's `<acceptance_criteria>` explicitly permits this deferral path: "If deferred: SUMMARY explicitly records 'screenshot deferred' with a `last_reviewed` follow-up date."
- **Why deferred:** The autonomous executor cannot drive the D-20 capture procedure (requires local Grafana 10+ + Prometheus + a populated Helix daemon plus a 60-120 sec `make bench` warmup). This is a checkpoint:human-verify task by design.
- **Commit:** `fa1676e9` — `docs(54-05): add placeholder for helix-overview dashboard screenshot`

### Task 2: USAGE.md — two new H3 subsections

- New `### Grafana Dashboards` H3 inserted at USAGE.md line 634 (immediately before the existing `### Prometheus Metrics`, which moved to line 660).
- New `### Runbooks` H3 inserted at USAGE.md line 649.
- Final flow through the Observability section:
  1. `### Enable the Admin Listener` (line 604)
  2. `### Health Checks` (line 622)
  3. `### Grafana Dashboards` (line 634, NEW)
  4. `### Runbooks` (line 649, NEW)
  5. `### Prometheus Metrics` (line 660, was line 634)
  6. `### Enable Tracing` (line 734, was 708)
  7. `### Enable pprof` (was 720)
- Image embed, both dashboard JSON link-bullets, the `DS_PROMETHEUS` import note, and the `$language` / `$instance` template-variable note all present per D-21.
- All four runbook files linked with one-line descriptions per D-22.
- **Commit:** `c1d55336` — `docs(54-05): add Grafana Dashboards and Runbooks sections to USAGE.md`

## Verification

- `grep -c '^### Grafana Dashboards$' USAGE.md` → 1 ✓
- `grep -c '^### Runbooks$' USAGE.md` → 1 ✓
- `grep -F 'docs/images/helix-overview-dashboard.png' USAGE.md` → matches ✓
- `grep -F 'deploy/grafana/helix-overview.json' USAGE.md` → matches ✓
- `grep -F 'deploy/grafana/helix-engine.json' USAGE.md` → matches ✓
- All four runbook paths matched (`grep -cE '^- \[\`(ErrCircuitOpen|deadline-timeouts|ls-crash-restart|memory-pressure-eviction)\.md\`\]' USAGE.md` → 4) ✓
- `git diff USAGE.md | awk '/^-[^-]/'` returns empty (pure insert) ✓
- `go test ./internal/obs/... -count=1` → ok 10.380s ✓
- `go vet ./...` → only pre-existing errors in untracked `tmp/` directories (out of scope per Phase 54 deviation rules; not introduced by this plan)
- `file docs/images/helix-overview-dashboard.png | grep -q "PNG image data"` → exits 0 ✓

## Deviations from Plan

### Auto-fixed Issues

None.

### Deferred Items (require human follow-up)

**1. [DEFERRED] Real 1600x900 helix-overview dashboard screenshot**

- **What:** Replace `docs/images/helix-overview-dashboard.png` (currently a 1x1 placeholder) with a real screenshot captured per the D-20 / Plan 54-05 Task 1 procedure.
- **Why deferred:** Requires a local Grafana 10+ + Prometheus stack with a populated Helix daemon (`make bench` + a few MCP tool exercises for 60-120s). Cannot be driven autonomously.
- **Procedure:** See Plan 54-05 `<how-to-verify>` block — quick path is `helix daemon --admin-addr=127.0.0.1:9091`, run a Prometheus container scraping it, run a Grafana 10.4+ container, import `deploy/grafana/helix-overview.json`, populate via `make bench` (LOCAL ONLY — never CI), screenshot at 1600x900, save to the canonical path, commit.
- **Acceptance once captured:** `file docs/images/helix-overview-dashboard.png` should report `PNG image data, ~1600 x ~900` (allow ±100px tolerance per acceptance_criteria), file size < 5 MB.
- **`last_reviewed` follow-up:** 2026-08-01 (capture is one-time per D-20; re-capture only on visible dashboard drift).
- **Tracking:** This entry is the canonical follow-up record per the plan's deferral acceptance_criteria.

## Phase 54 Roadmap Success Criteria — Final Checklist

| # | Criterion | Status | Citation |
| - | - | - | - |
| 1 | ≥2 Grafana dashboards in `deploy/grafana/` | ✓ | `helix-overview.json` (Plan 02), `helix-engine.json` (Plan 03) |
| 2 | Four runbooks at exact filenames in `docs/runbooks/` | ✓ | `ErrCircuitOpen.md`, `deadline-timeouts.md`, `ls-crash-restart.md`, `memory-pressure-eviction.md` (Plan 04) |
| 3 | USAGE.md links + screenshot | ✓ (with deferred placeholder) | This plan: USAGE.md ### Grafana Dashboards + ### Runbooks; `docs/images/helix-overview-dashboard.png` placeholder pending real capture |
| 4 | Validation test enforcing PromQL references registered metrics | ✓ | `internal/obs/dashboards_test.go` (Plan 01); kept green across Plans 02 + 04 |

## Threat Flags

None — this plan touches only documentation surfaces (USAGE.md markdown, a binary PNG asset). No new network endpoints, auth paths, file-access patterns, or schema changes. Plan threat_model classified LOW; no high-severity items.

## Self-Check: PASSED

- `docs/images/helix-overview-dashboard.png` — FOUND
- `USAGE.md` — modified, headings verified
- Commit `fa1676e9` — FOUND
- Commit `c1d55336` — FOUND
- All grep acceptance checks pass
- `go test ./internal/obs/... -count=1` green

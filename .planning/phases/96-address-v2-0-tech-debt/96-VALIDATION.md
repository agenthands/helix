---
phase: 96
slug: address-v2-0-tech-debt
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-06-22
---

# Phase 96 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution. Source: 96-RESEARCH.md § Validation Architecture. Four small, well-bounded fixes — validation is lightweight, concrete, and per-item.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go standard `testing` (table-driven) |
| **Config file** | none — Go toolchain |
| **Quick run command** | `go test ./internal/cli/... ./internal/daemon/...` |
| **Full suite command** | `go test ./...` |
| **Doc gate** | `make verify-docs` (= `go run ./cmd/docgen --check`) |
| **Estimated runtime** | ~30–60 seconds (full suite) |

---

## Sampling Rate

- **After every task commit:** Run the item's targeted `-run` command (sub-second).
- **After every plan wave:** `go build ./... && go vet ./... && go test ./internal/cli/... ./internal/daemon/... ./internal/kernel/help/...` + `make verify-docs`.
- **Before phase verification:** Full suite green — `go build ./...`, `go vet ./...`, `go test ./...` all pass; `make verify-docs` green; `git diff --stat api/proto/` empty; `internal/cli/verbs_gen.go` unchanged (frozen verb set).
- **Max feedback latency:** ~60 seconds.

---

## Per-Task Verification Map

| Task Ref | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|----------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| TD-01 | 1 | TD-01 | T-96-01 | `validateAdminAddr` refuses empty-host (`:9090`) + wildcard/unspecified bind; accepts loopback — admin instrumentation never binds `0.0.0.0`/`::` | unit (table) | `go test ./internal/daemon/ -run TestValidateAdminAddr` | ✅ extend `telemetry_test.go` | ⬜ pending |
| TD-02 | 1 | TD-02 | — | N/A (dead-code removal; live sibling `removeFromJSONConfig` preserved) | build/vet + suite | `go build ./... && go vet ./... && go test ./internal/cli/...` | ✅ remove merge tests from `setup_test.go` | ⬜ pending |
| TD-03 | 1 | TD-03 | T-96-02 | grep/rg/ag/egrep/fgrep PATTERN not counted as file operand; hook stays fail-open / exit-0 | unit (table) | `go test ./internal/cli/ -run TestClassifyBashTarget` | ✅ extend `nudge_test.go` | ⬜ pending |
| TD-04 | 1 | TD-04 | — | N/A (generated-doc correction; README regenerated via docgen, not hand-edited) | drift gate | `make docs && make verify-docs` | ✅ docgen `--check` | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

*Existing infrastructure covers all phase requirements.* No new framework or fixture required — the work extends `telemetry_test.go` (TD-01) and `nudge_test.go` (TD-03), removes merge tests from `setup_test.go` (TD-02), and reuses the existing docgen `--check` gate (TD-04).

---

## Manual-Only Verifications

*All phase behaviors have automated verification.* (TD-04's doc correctness is enforced by the docgen drift gate; no manual review required.)

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references (none for this phase)
- [ ] No watch-mode flags
- [ ] Feedback latency < 60s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending

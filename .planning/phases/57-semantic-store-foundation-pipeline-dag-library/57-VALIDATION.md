---
phase: 57
slug: semantic-store-foundation-pipeline-dag-library
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-05-03
---

# Phase 57 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (Go 1.x) |
| **Config file** | none — repo-wide `go.mod` + `Makefile` |
| **Quick run command** | `go test ./internal/phasegraph/... ./internal/semantic/... ./internal/config/... ./internal/lint/noduckdb/...` |
| **Full suite command** | `make vet && make test` |
| **Estimated runtime** | ~30 seconds (quick) / ~120 seconds (full) |

---

## Sampling Rate

- **After every task commit:** Run quick command for the package(s) touched
- **After every plan wave:** Run `make vet && make test`
- **Before `/gsd-verify-work`:** Full suite must be green; `make vet` must pass (proves analyzer is wired)
- **Max feedback latency:** 30 seconds for quick; 120 seconds for full

---

## Per-Task Verification Map

> Populated by the planner during plan generation. Each task in a PLAN.md must declare an `<automated>` block citing the command from the table below; otherwise it must list a Wave 0 dependency on a missing test file.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| TBD-by-planner | — | — | STORE-01..06, DAG-01..04 | — | see PLAN.md | unit/integration | per task | — | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/phasegraph/phasegraph_test.go` — fixtures for happy path, duplicate ID, missing dep, single-node cycle, multi-node cycle (DAG-03)
- [ ] `internal/semantic/store/store_test.go` — fixtures for fresh open, existing+clean reopen, corrupt-header quarantine+rebuild, schema-version stamping (STORE-01, STORE-03, STORE-06)
- [ ] `internal/semantic/store/duckdb_nocgo_test.go` — CGO=0 stub returns `serr.ErrUnsupported` (STORE-02)
- [ ] `internal/config/loader_test.go` extensions — `TestLoad_SemanticIndexDefaults`, `TestLoad_SemanticIndexPrecedence` (STORE-04, STORE-05)
- [ ] `internal/lint/noduckdb/analyzer_test.go` + fixture under `testdata/src/badimport/` — `analysistest`-driven (STORE-04 vet rule)

*Existing infrastructure (`go test`, `analysistest`, `t.TempDir()`, `internal/obs/` metric helpers) covers everything else.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| First daemon start with `semantic_index.enabled=true` opens fresh `<workspace>/.helix/semantic.duckdb` | Success Criterion 1 | Cross-process daemon bootstrap; covered by integration test in P02 but worth a one-time human smoke run | `helix start` against an empty workspace; verify `.helix/semantic.duckdb` appears and `helix status` does not error |
| Daemon refuses CGO=0 build cleanly with `Kind: Unsupported` | Success Criterion 2, STORE-02 | The CGO=0 path is exercised by unit tests but the human-readable refusal banner deserves visual confirmation | `CGO_ENABLED=0 go build ./cmd/helix && ./helix start`; expect remediation banner |

*All other phase behaviors have automated verification.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s for quick command
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending

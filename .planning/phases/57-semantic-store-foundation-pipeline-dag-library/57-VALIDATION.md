---
phase: 57
slug: semantic-store-foundation-pipeline-dag-library
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-05-03
revised: 2026-05-03
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

> Populated by the planner during plan generation (revision 2026-05-03 closed W-04). Each task in a PLAN.md declares an `<automated>` block; cross-referenced below.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 57-01-00 | P01 | 1 | DAG-03 | T-57-01-01 | DAG validation rejects malformed graphs by API shape | unit (RED) | `! go test ./internal/phasegraph/... 2>&1 \| grep -E "(no Go files\|cannot find package\|undefined: phasegraph)" -q` | yes | ⬜ pending |
| 57-01-01 | P01 | 1 | DAG-01,02,03 | T-57-01-01..04 | stdlib-only library validates+runs DAG; fail-fast cycle detection | unit (GREEN) | `go test ./internal/phasegraph/... -count=1` | yes | ⬜ pending |
| 57-01-02 | P01 | 1 | DAG-03 | T-57-01-03 | DOT-on-failure for cycle debugging; pipeline cardinality matches SPEC | unit (REFACTOR) | `go test ./internal/phasegraph/... -count=1 -run "TestValidate_WritesDOTOnCycleWhenWriterProvided\|TestPipelineCount_MatchesSpec"` | yes | ⬜ pending |
| 57-02-01 | P02 | 1 | STORE-06, DAG-04 | T-57-02-04 | duckdb-go dep added at D-12-locked path; DAG-04 marker placed | unit/build | `go build ./internal/semantic/... && grep -q "TODO(v1.11): migrate to phasegraph.RunPhaseGraph(BootstrapPhases)" internal/daemon/daemon.go && grep -q 'github.com/duckdb/duckdb-go/v2 v2.10502.0' go.mod` | yes | ⬜ pending |
| 57-02-02a | P02 | 1 | STORE-01,02,03,06 | T-57-02-01..06 | RED gate: 11 tests fail because stubs return ErrUnsupported | unit (RED) | `go test ./internal/semantic/store/... -count=1 2>&1; test $? -ne 0` | yes | ⬜ pending |
| 57-02-02b | P02 | 1 | STORE-01,02,03,06 | T-57-02-01..06 | GREEN gate: three-tier open + Schema 1 + CGO=0 stub + 2 metrics | unit/integration (GREEN) | `go test ./internal/semantic/store/... ./internal/obs/... -race -count=1 && CGO_ENABLED=0 go build ./internal/semantic/store/... && CGO_ENABLED=0 go test ./internal/semantic/store/... -count=1` | yes | ⬜ pending |
| 57-02-03 | P02 | 1 | STORE-01, DAG-04 | T-57-02-04 | daemon step 6b wired; SerenaConfig stub field present; wave-1 build green | unit/build | `go build ./... && go vet ./... && grep -c '6b. Open semantic fact store' internal/daemon/daemon.go \| grep -q '^1$' && grep -q 'semanticstore.Open' internal/daemon/daemon.go && grep -q 'SemanticIndex semantic.Config' internal/config/config.go` | yes | ⬜ pending |
| 57-03-01 | P03 | 2 | STORE-04, STORE-05 | T-57-03-01..04 | RED gate: TestLoad_SemanticIndex* tests fail because defaults missing and koanf tag missing | unit (RED) | `! go test ./internal/config/... -run TestLoad_SemanticIndex -count=1 2>/dev/null` | yes | ⬜ pending |
| 57-03-02 | P03 | 2 | STORE-04, STORE-05 | T-57-03-01..04 | GREEN gate: koanf tag added, every SPEC §25 default present, tests pass | unit (GREEN) | `go test ./internal/config/... -count=1 -run "TestLoad_SemanticIndex\|TestLoad_Observability\|TestLoad_ProjectConfigOverridesGlobal"` | yes | ⬜ pending |
| 57-03-03 | P03 | 2 | STORE-01,02 | T-57-02-04 | daemon integration test exercises YAML→koanf→Open path | integration | `go test -tags 'cgo integration' ./internal/daemon/... -run TestDaemon_SemanticStore -count=1` | yes | ⬜ pending |
| 57-04-01 | P04 | 2 | STORE-06 | T-57-04-01..06 | RED gate: analyzer test fails to build (Analyzer undefined) | unit (RED) | `! go build ./internal/lint/noduckdb/... 2>/dev/null` | yes | ⬜ pending |
| 57-04-02 | P04 | 2 | STORE-06 | T-57-04-01..06 | GREEN gate: analyzer rejects badpkg, allows goodpkg, full-tree vet clean | unit (GREEN) | `go test ./internal/lint/noduckdb/... -count=1 && go install ./cmd/vet-noduckdb && go vet -vettool=$(go env GOPATH)/bin/vet-noduckdb ./...` | yes | ⬜ pending |
| 57-04-03 | P04 | 2 | STORE-06 | T-57-04-04 | Makefile vet target replaced + chained from test; clean-state build works | build/integration | `rm -f $(go env GOPATH)/bin/vet-noduckdb && make vet && make test` | yes | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

**Sampling continuity check (W-04):** No 3 consecutive task IDs above lack `<automated>` verify. Each task has a runnable command. Runtime estimates per quick command: < 30 s.

---

## Wave 0 Requirements

- [ ] `internal/phasegraph/phasegraph_test.go` — fixtures for happy path, duplicate ID, missing dep, single-node cycle, multi-node cycle (DAG-03) — created by 57-01-00
- [ ] `internal/semantic/store/store_test.go` — fixtures for fresh open, existing+clean reopen, corrupt-header quarantine+rebuild, schema-version stamping (STORE-01, STORE-03, STORE-06) — created by 57-02-02a
- [ ] `internal/semantic/store/duckdb_nocgo_test.go` — CGO=0 stub returns `serr.ErrUnsupported` (STORE-02) — created by 57-02-02a
- [ ] `internal/config/loader_test.go` extensions — `TestLoad_SemanticIndexDefaults`, `TestLoad_SemanticIndexPrecedence` (STORE-04, STORE-05) — created by 57-03-01
- [ ] `internal/lint/noduckdb/analyzer_test.go` + fixtures (badpkg + deep-path goodpkg) — `analysistest`-driven (STORE-06) — created by 57-04-01

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

- [x] All tasks have `<automated>` verify or Wave 0 dependencies (verified 2026-05-03 — every task ID in the per-task map carries an `<automated>` command)
- [x] Sampling continuity: no 3 consecutive tasks without automated verify (verified 2026-05-03 — every row in the per-task map has a command)
- [ ] Wave 0 covers all MISSING references (executor flips after wave 0 completes — see Wave 0 checklist above)
- [x] No watch-mode flags (verified 2026-05-03 — all commands are one-shot, no `--watch` / `-watch` / `--reload` flags present)
- [ ] Feedback latency < 30s for quick command (executor confirms after first run)
- [ ] `nyquist_compliant: true` set in frontmatter (executor flips after sampling continuity is empirically confirmed end-to-end)

**Approval:** pending — three checkboxes flipped at planner-revision time (2026-05-03); remaining three flip during execution as the executor verifies wave-0 completion + measured feedback latency, then flips `nyquist_compliant: true` in this file's frontmatter.

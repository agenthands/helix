---
phase: 75
slug: schema-fairness-contract-tree-skeleton
status: draft
nyquist_compliant: true
wave_0_complete: false
created: 2026-06-14
---

# Phase 75 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — standard `go test ./...` |
| **Quick run command** | `go test ./bench/... ./cmd/helix-bench/...` |
| **Full suite command** | `go vet ./... && go test ./...` |
| **Estimated runtime** | ~60 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./bench/... ./cmd/helix-bench/...`
- **After every plan wave:** Run `go vet ./... && go test ./...`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 60 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 75-01-01 | 01 | 0 | INFRA-01, INFRA-03 | T-75-01, T-75-02 | git-mv continuity + no dangling cross-refs | integration/script | `git grep -n "bench/semantic_bench\|bench/_blevprobe\|bench/fixtures" -- ':!*.planning*' ':!legacy*'; test $(git grep -c "bench/semantic_bench\|bench/_blevprobe\|bench/fixtures" -- ':!*.planning*' ':!legacy*' | wc -l) -eq 0 && go vet ./... && go test ./...` | ❌ W0 | ⬜ pending |
| 75-02-01 | 02 | 1 | BENCH-01, BENCH-06, INFRA-01, INFRA-02, INFRA-03 | T-75-03, T-75-04 | display-only attestation fields, no injection sink | unit/script (doc render) | `test "$(find bench -maxdepth 1 -mindepth 1 -type d | wc -l)" -eq 6 && grep -q "eval/EVAL.md" bench/BENCH.md && grep -q "bench/BENCH.md" eval/EVAL.md && grep -Eq "^benchmarking_permitted:" bench/PROVIDERS.md && grep -q "datasets,runners,languages,evaluators,reports,schema" .planning/REQUIREMENTS.md && grep -q "datasets,runners,languages,evaluators,reports,schema" .planning/milestones/v1.12-ROADMAP.md && grep -q "single permitted change" .planning/REQUIREMENTS.md && grep -q "single permitted change" .planning/milestones/v1.12-ROADMAP.md && go vet ./...` | ❌ W0 | ⬜ pending |
| 75-02-02 | 02 | 1 | INFRA-01 | T-75-03 | TOS-flag legal accuracy (human-only) | manual checkpoint | MANUAL — `checkpoint:human-verify` (maintainer confirms tos_url / benchmarking_permitted / publish_permitted against live provider TOS); see Manual-Only Verifications | ❌ W0 | ⬜ pending |
| 75-03-01 | 03 | 1 | BENCH-03, FAIR-03 (schema substrate only) | T-75-05, T-75-06 | schema validator rejects malformed instance, no crash (ASVS V5) | unit (TDD) | `go test ./bench/schema/... && grep -q '"const": *"v2"' bench/schema/result.v2.schema.json && grep -q "tokens_input_cached_read" bench/schema/result.v2.schema.json && grep -q "overrides" bench/schema/result.v2.schema.json && go vet ./...` | ❌ W0 | ⬜ pending |
| 75-04-01 | 04 | 1 | FAIR-01, FAIR-02 | T-75-07, T-75-08, T-75-09 | empty-WaiverReason fatal; dated snapshot pin; injected-clock 30d gate; free-text fields never exec'd | unit (TDD) | `go test ./bench/runners/... && grep -q "var DefaultContract = FairnessContract{" bench/runners/fairness_contract.go && grep -Eq "ModelID:\s*\"[a-z0-9-]+-[0-9]{8}\"" bench/runners/fairness_contract.go && grep -q "func (c FairnessContract) DeprecationGate(today time.Time" bench/runners/fairness_contract.go && go vet ./...` | ❌ W0 | ⬜ pending |
| 75-05-01 | 05 | 2 | BENCH-02, COST-01 | — | RunE error propagation; path arg is file-read only, not a shell sink | unit/build | `go build ./cmd/helix-bench && go test ./cmd/helix-bench/... && go run ./cmd/helix-bench doctor && go run ./cmd/helix-bench --help 2>&1 | grep -q validate-cost-table && go vet ./...` | ❌ W0 | ⬜ pending |
| 75-05-02 | 05 | 2 | BENCH-06, COST-01, FAIR-02 | T-75-10, T-75-11, T-75-12 | strict yaml decode (KnownFields); hard-fail gates; no continue-on-error; injected clock | unit (TDD) | `go test ./cmd/helix-bench/... && make validate-cost-table && make verify-tos && ! go run ./cmd/helix-bench validate-cost-table bench/datasets/testdata/cost-table-stale.yaml && grep -q "func validateCostTable(today time.Time" cmd/helix-bench/validate_cost_table.go && grep -q "func verifyTOS(today time.Time" cmd/helix-bench/verify_tos.go && go vet ./...` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

**Sampling continuity check:** 7 tasks total. Every code-producing task carries an `<automated>`
verify block; the only manual task (75-02-02) is a `checkpoint:human-verify` for legal-TOS
accuracy and is bracketed by automated tasks on both sides (75-02-01 automated before, 75-03-01
automated after). No run of 3 consecutive tasks lacks an automated verify → nyquist 8c satisfied.

---

## Wave 0 Requirements

- [ ] Relocation of Phase 64 semantic microbench to `internal/semantic/bench/` (D-05/D-06) lands before any new `bench/` file — owned by Plan 75-01 (Wave 0), which every later plan `depends_on`.
- [ ] After relocation, `git grep "bench/semantic_bench\|bench/_blevprobe\|bench/fixtures"` (excluding `.planning`/`legacy`) is empty and `git log --follow` continuity holds on the moved files.
- [ ] Existing `go test` infrastructure covers all phase requirements — no new framework install (all libs vendored: `santhosh-tekuri/jsonschema/v6`, `gopkg.in/yaml.v3`, `spf13/cobra`).

*All MISSING test references in this phase are net-new files created within the phase itself
(schema test, fairness_contract test, helix-bench tests). Each is authored test-first (RED) inside
its TDD plan; there is no pre-existing test file to scaffold in a separate Wave 0 beyond the Plan
75-01 relocation.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| TOS attestation accuracy (PROVIDERS.md content reflects real provider TOS) | INFRA-01 / BENCH-06 | Requires human reading of provider Terms of Service — `verify-tos` only checks date freshness + frontmatter parse, not factual correctness of the flags | Maintainer confirms each `tos_url` + `benchmarking_permitted` + `publish_permitted` flag against the live TOS at attestation time (Plan 75-02 Task 2, `checkpoint:human-verify`) |

*All other phase behaviors have automated verification.*

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or are a designated manual checkpoint (75-02-02) bracketed by automated tasks
- [x] Sampling continuity: no 3 consecutive tasks without automated verify (only one manual task, surrounded by automated ones)
- [x] Wave 0 covers all MISSING references (Plan 75-01 relocation gates every later plan; all other MISSING files are net-new test-first artifacts within their own TDD plans)
- [x] No watch-mode flags (all commands are single-shot `go test` / `go vet` / `make` / `go run`)
- [x] Feedback latency < 60s (full suite ~60s; per-package quick runs faster)
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved (planner — revision pass populated the per-task map and confirmed nyquist 8a/8c compliance from the five PLAN.md `<automated>` verify blocks)

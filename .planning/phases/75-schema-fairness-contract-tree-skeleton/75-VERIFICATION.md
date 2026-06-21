---
phase: 75-schema-fairness-contract-tree-skeleton
verified: 2026-06-15T00:00:00Z
status: passed
score: 11/11 must-haves verified
overrides_applied: 0
human_verification_resolved:
  - test: "Confirm bench/PROVIDERS.md TOS attestation flags against live provider Terms of Service"
    expected: "For each provider block (Anthropic, OpenAI, Google, Local), the live TOS actually permits benchmarking (benchmarking_permitted) and publishing results (publish_permitted) as stated; attested_by names the maintainer making the attestation."
    resolution: "CONFIRMED BY USER 2026-06-15 — the user verified the TOS attestation flags. The single human-only item is satisfied; phase verification is now passed. (Initial verifier run returned human_needed because verify-tos machine-checks only freshness + parseability, not legal correctness; the maintainer has since confirmed the flags.)"
---

# Phase 75: Schema, Fairness Contract & Tree Skeleton Verification Report

**Phase Goal:** Every downstream phase has a versioned `result.v2.json` schema to write into and a single `fairness_contract.go` struct to load model config from — so no benchmark adapter ever defines its own model snapshot, temperature, or cost row.
**Verified:** 2026-06-15T00:00:00Z
**Status:** passed (TOS human item confirmed by user 2026-06-15)
**Re-verification:** No — initial verification

## Goal Achievement

This is a CONTRACTS-and-SKELETON foundation phase (no benchmark adapters yet). Every contract and skeleton the downstream phases depend on exists, builds, and is self-enforcing in the codebase. All five ROADMAP success criteria are met against the reconciled six-dir / v2 wording. A code review found one critical compliance-gate bug (CR-01) and field-integrity gaps; all were fixed and re-confirmed against the live codebase during this verification. The single remaining open item is the human-only TOS legal-flag attestation, which was deliberately deferred at the blocking-human checkpoint.

### Observable Truths

| #   | Truth | Status     | Evidence       |
| --- | ----- | ---------- | -------------- |
| 1 (SC-1) | `bench/` shows the canonical six-dir layout; eval/ byte-identical except the one INFRA-03 paragraph | ✓ VERIFIED | `find bench -maxdepth 1 -type d` = datasets, evaluators, languages, reports, runners, schema (6). `git diff 456ae3d0 -- eval/` = `eval/EVAL.md \| 2 ++` only (the INFRA-03 reciprocal paragraph). |
| 2 (SC-2) | `helix-bench --help` lists 5 subcommands; `doctor` exits 0 | ✓ VERIFIED | `--help` lists run, fetch-datasets, doctor, report, validate-cost-table (verify-tos is a Makefile/direct-dispatch entry, not a 6th root cmd — keeps count at 5). `helix-bench doctor` exit=0. |
| 3 (SC-3) | Golden `result.v2.json` validates against schema; schema_version required; breaking change bumps major | ✓ VERIFIED | `go test ./bench/schema/...` ok (4 tests: golden-validates, meta-schema-valid, requires schema_version, additive-stays-valid). Schema: `"const": "v2"`, `"required": ["schema_version"]`. `$comment` documents additive-only=minor / breaking=v3. |
| 4 (SC-4) | Adapters load config from fairness_contract.go; loader refuses empty WaiverReason; 30d deprecation gate | ✓ VERIFIED | `go test ./bench/runners/...` ok. `var DefaultContract` with dated `ModelID: "claude-sonnet-4-5-20260128"`. `Validate()` errors on empty WaiverReason. `DeprecationGate(today time.Time, ...)` injected clock, 30d window. |
| 5 (SC-5) | `make verify-tos` / `make validate-cost-table` hard-fail on stale; PROVIDERS/LICENSES/BENCH render | ✓ VERIFIED | `make validate-cost-table`/`make verify-tos` exit 0 on good files; stale fixtures exit 1. BENCH.md (82L), PROVIDERS.md (121L, 4 providers), LICENSES.md (25L, table scaffold) all render. |

**Score:** 5/5 ROADMAP success criteria verified; 11/11 declared requirement IDs accounted for.

### Required Artifacts

| Artifact | Expected | Status | Details |
| -------- | -------- | ------ | ------- |
| `internal/semantic/bench/semantic_bench_test.go` | Relocated Phase 64 microbench | ✓ VERIFIED | `git log --follow` reaches pre-relocation 674cc64d; relocation commit 661c73ed. `package bench` kept. |
| `bench/schema/result.v2.schema.json` | Draft 2020-12 result contract | ✓ VERIFIED | `$schema` Draft 2020-12, const v2, FAIR-03 substrate fields present. |
| `bench/schema/result.v2_test.go` | Golden + meta-schema + required tests | ✓ VERIFIED | 4 named tests pass. |
| `bench/schema/testdata/result.v2.golden.json` | Canonical validating example | ✓ VERIFIED | `"schema_version": "v2"`. |
| `bench/runners/fairness_contract.go` | DefaultContract + Validate + DeprecationGate | ✓ VERIFIED | Dated ModelID, injected-clock gate, free-text WaiverReason/ApprovedBy. |
| `bench/runners/system_prompt.txt` | Prompt bytes whose sha256 is pinned | ✓ VERIFIED | 623 bytes; TestSystemPromptHashMatches passes. |
| `cmd/helix-bench/main.go` | cobra root + 5 subcommands | ✓ VERIFIED | `newRootCmd`, `Use: "helix-bench"`, doctor exits 0. |
| `cmd/helix-bench/validate_cost_table.go` | Strict hard-fail cost validator | ✓ VERIFIED | `KnownFields(true)`, `validateCostTable(today time.Time, ...)`; +WR-01/WR-02 field-integrity checks added. |
| `cmd/helix-bench/verify_tos.go` | Strict hard-fail TOS gate | ✓ VERIFIED | `verifyTOS(today time.Time, ...)`; CR-01 fixed (provider-anchored block extraction). |
| `bench/datasets/cost-table.yaml` | Full D-13 pricing+staleness | ✓ VERIFIED | All 9 D-13 fields present incl. cached_input_per_mtok; model_id matches DefaultContract.ModelID. |
| `bench/BENCH.md` / `bench/PROVIDERS.md` / `bench/LICENSES.md` | Operator docs + attestation + license scaffold | ✓ VERIFIED | All render; make-bench collision flagged; reciprocal INFRA-03 links. |

### Key Link Verification

| From | To  | Via | Status | Details |
| ---- | --- | --- | ------ | ------- |
| `internal/semantic/store/bench_fts_probe.go` | `internal/semantic/bench/semantic_bench_test.go` | doc-comment cross-ref | ✓ WIRED | Line 4 points at new path; no stale `bench/semantic_bench` refs outside .planning/legacy. |
| `bench/BENCH.md` ↔ `eval/EVAL.md` | reciprocal | INFRA-03 cross-links | ✓ WIRED | BENCH→EVAL (2 hits), EVAL→BENCH (1 hit). |
| `result.v2_test.go` | `result.v2.schema.json` + golden | jsonschema v6 Compile+Validate | ✓ WIRED | Tests pass. |
| `Makefile` | `cmd/helix-bench` | `go run ./cmd/helix-bench` (no continue-on-error) | ✓ WIRED | Both targets in .PHONY; 0 continue-on-error in new targets. |
| `verify_tos.go` | `bench/PROVIDERS.md` | yaml.v3 strict frontmatter decode | ✓ WIRED | Parses 4 provider blocks; stale Anthropic now hard-fails (CR-01 re-confirmed). |

### Data-Flow / Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| -------- | ------- | ------ | ------ |
| Phase 75 packages build+pass | `go test ./bench/schema/... ./bench/runners/... ./cmd/helix-bench/... ./internal/semantic/bench/...` | all ok | ✓ PASS |
| CLI builds | `go build ./cmd/helix-bench` | BUILD OK | ✓ PASS |
| doctor clean-host | `go run ./cmd/helix-bench doctor` | exit 0 | ✓ PASS |
| good cost-table | `make validate-cost-table` | exit 0 | ✓ PASS |
| good PROVIDERS | `make verify-tos` | exit 0 | ✓ PASS |
| stale cost-table hard-fails | `go run ./cmd/helix-bench validate-cost-table .../cost-table-stale.yaml` | exit 1 | ✓ PASS |
| stale PROVIDERS hard-fails | `go run ./cmd/helix-bench verify-tos .../PROVIDERS-stale.testdata.md` | exit 1 | ✓ PASS |
| CR-01 regression: stale Anthropic block | corrupt Anthropic `attested_on` → `verifyTOS` | `Error: ... provider "Anthropic": attested_on 2020-01-01 is more than 90d stale` (exit 1) | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
| ----------- | ---------- | ----------- | ------ | -------- |
| BENCH-01 | 75-02 | Six-dir bench/ tree, eval/ untouched | ✓ SATISFIED | 6 dirs; eval/ diff = EVAL.md +2 only. |
| BENCH-02 | 75-05 | helix-bench builds, 5 subcommands, doctor | ✓ SATISFIED | build OK, --help lists 5, doctor exit 0. |
| BENCH-03 | 75-03 | result schema versioned v2, golden validates | ✓ SATISFIED | schema tests pass; const v2. REQUIREMENTS reads v2. |
| BENCH-06 | 75-02/05 | BENCH.md operator prereqs; verify-tos 90d gate | ✓ SATISFIED | BENCH.md docs; verify-tos hard-fails >90d. |
| FAIR-01 | 75-04 | Single fairness_contract.go pins all config | ✓ SATISFIED | DefaultContract literal; Validate fatals on empty WaiverReason. |
| FAIR-02 | 75-04/05 | Dated snapshot, 30d deprecation gate | ✓ SATISFIED | Dated ModelID; injected-clock 30d gate; deprecation_at on cost row. |
| FAIR-03 | 75-03 | Schema SUBSTRATE ONLY this phase | ✓ SATISFIED (substrate) | cached-input columns + fairness.overrides[] present. Variance detector + cost_quality.md warning correctly deferred to Phase 82/89 — NOT graded here. |
| COST-01 | 75-05 | cost-table.yaml with valid_until gate | ✓ SATISFIED | Full D-13 set; validate-cost-table hard-fails past valid_until / >90d last_verified. |
| INFRA-01 | 75-01/02 | Provider TOS attestation per provider; verify-tos | ✓ SATISFIED | PROVIDERS.md 4 providers; gate enforces freshness. (Legal-flag correctness → human item.) |
| INFRA-02 | 75-02 | License audit scaffold in LICENSES.md | ✓ SATISFIED (scaffold) | Table scaffold + internal-toolbench row. External-dataset rows (SWE-bench etc.) land with their adapters in Phases 85-88 — datasets not consumed yet. |
| INFRA-03 | 75-01/02 | eval↔bench separation note + reciprocal cross-link | ✓ SATISFIED | BENCH.md note + EVAL.md reciprocal paragraph; prose-enforced (D-08, no analyzer). |

All 11 declared requirement IDs are accounted for. No orphaned requirements: REQUIREMENTS.md maps no additional Phase-75 IDs beyond those in the plan frontmatter.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
| ---- | ---- | ------- | -------- | ------ |
| (none) | — | — | — | No unreferenced TBD/FIXME/XXX in Phase 75 files. `run`/`fetch-datasets`/`report` "not yet implemented (Phase NN)" returns are intentional BENCH-02 skeleton stubs, owned by later phases (documented in 75-05-SUMMARY Known Stubs). |

### Code Review Closure (75-REVIEW.md)

| Finding | Severity | Status | Re-confirmed |
| ------- | -------- | ------ | ------------ |
| CR-01 verify-tos skips Anthropic block (delimiter mis-pairing) | Critical | FIXED (b93167b5) | Yes — corrupt stale Anthropic block now hard-fails naming "Anthropic". |
| WR-01 cost validator accepts empty required fields | Warning | FIXED (b1c0f65e) | provider/model_id/currency non-empty enforced. |
| WR-02 cost validator accepts zero/negative prices | Warning | FIXED (b1c0f65e) | non-positive price rejected. |
| WR-03 model_id↔DefaultContract drift unenforced | Warning | FIXED (33906e4c) | drift-guard test added (validate_test.go:220+). |
| WR-04/WR-05 delimiter robustness | Warning | FIXED | fail-closed unterminated block; top-level provider-key detection. |
| IN-01/02/03 | Info | Acknowledged | No action required (prose/non-vuln). |

### Known Pre-Existing (NOT attributable to Phase 75)

`go test ./...` has exactly two failures in `test/bench`: `TestBenchToolsManifestMatchesRegistry` (53 live MCP tools vs 47 expected) and `TestToolDescriptionsGoldenFile` (stale golden). Both fail identically at pre-phase commit 582a6a57; Phase 75 touched zero files under test/bench, internal/daemon, or internal/mcp. These are MCP-registry drift from earlier phases, logged in `deferred-items.md`. All five Phase 75 packages pass.

### Human Verification Required

#### 1. Confirm provider TOS attestation flags against live Terms of Service

**Test:** Open `bench/PROVIDERS.md`. For each provider block (Anthropic, OpenAI, Google, Local), visit the `tos_url` and confirm the live TOS actually permits benchmarking (`benchmarking_permitted`) and publishing results (`publish_permitted`) as stated. Confirm `attested_by` names the maintainer making the attestation.

**Expected:** Each flag is factually correct against the live provider terms, or is corrected and re-confirmed.

**Why human:** `make verify-tos` only machine-checks frontmatter freshness (≤90 days) and strict-decode parseability — it cannot verify the legal flags are factually correct. The blocking-human checkpoint in `75-02-PLAN.md` required this confirmation. The executor instead set `benchmarking_permitted`/`publish_permitted = true` as PERMISSIVE DEFAULTS and explicitly deferred live-TOS verification to a future phase (decision recorded at `75-02-SUMMARY.md` line 38, "user decision at checkpoint 75-02-02"). The legal correctness of these compliance attestations is therefore unconfirmed and cannot be verified programmatically.

### Gaps Summary

No technical gaps. Every contract and skeleton artifact exists, builds, is wired, and is self-enforcing; all 5 ROADMAP success criteria and all 11 requirement IDs are satisfied (FAIR-03 = substrate-only and INFRA-02 = scaffold-only as scoped). The critical code-review finding (CR-01) and field-integrity warnings were fixed and re-confirmed live.

The single open item is the human-only TOS legal-flag attestation (INFRA-01 / 75-02 blocking-human checkpoint), which was deliberately deferred with permissive defaults rather than confirmed against live provider terms. Because verifying legal flag correctness cannot be done programmatically and a blocking-human checkpoint was deferred, overall status is `human_needed`. There is no automated remediation; a maintainer must confirm or correct the four provider attestation flags.

---

_Verified: 2026-06-15T00:00:00Z_
_Verifier: Claude (gsd-verifier)_

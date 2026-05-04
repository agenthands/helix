---
phase: 59
slug: tree-sitter-extraction-stable-symbol-ids
status: signed-off
nyquist_compliant: true
wave_0_complete: true
created: 2026-05-04
audited: 2026-05-04
---

# Phase 59 — Validation Strategy

> Per-phase validation contract. Audited post-execution against the merged tree.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (table-driven + golden-file `testdata/`) |
| **Config file** | none — built-in `go test` |
| **Quick run command** | `go test ./internal/semantic/extract/... -run TestStableSymbolKey -count=1` |
| **Full suite command** | `go test ./internal/semantic/... ./internal/daemon/... -count=1` |
| **Estimated runtime** | ~30s quick / ~120s full |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/semantic/extract/... -count=1` (or the package modified by that task)
- **After every plan wave:** Run full suite + `go vet ./...`
- **Before `/gsd-verify-work`:** Full suite must be green AND the 30+ before/after matrix per first-class language passes
- **Max feedback latency:** 60 seconds (per-package quick) / 120 seconds (full)

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 59-01-01 | 01 | 0 | EXTRACT-01..05 (schema migration) | T-59-01-01/02 | forward-incompat refused; partial-migration detected | integration | `go test ./internal/semantic/store/... -run TestMigration -count=1` | ✅ `internal/semantic/store/migrations_test.go` | ✅ green |
| 59-02-01 | 02 | 1 | EXTRACT-01 (stable symbol-ID) | T-59-02-03 | symbol IDs survive whitespace/rename/move; field-order pinned | unit + property | `go test ./internal/semantic/extract/... -run "TestStableSymbolKey\|TestCanonicalize" -count=1` | ✅ `internal/semantic/extract/stable_id_test.go` | ✅ green |
| 59-02-02 | 02 | 1 | EXTRACT-04 (5-rung confidence) | — | 5-rung ladder enforced | unit | `go test ./internal/semantic/extract/... -run TestConfidence -count=1` | ✅ `internal/semantic/extract/confidence_test.go` | ✅ green |
| 59-03-01 | 03 | 1 | EXTRACT-02 (extraction scheduler) | T-59-03-01/02/03 | RequireReady best-partial on timeout; cap-8 buffered Subscribe; one-job-per-workspace | integration | `go test ./internal/semantic/scheduler/... -count=1` | ✅ `scheduler_test.go`, `ready_test.go`, `priority_test.go`, `export_test.go` | ✅ green |
| 59-04-01 | 04 | 2 | EXTRACT-03 (Go provider) | T-59-04-02 | 30+ before/after matrix; `//go:embed` queries | golden-file | `go test ./internal/semantic/extract/golang/... -count=1` | ✅ `provider_test.go`, `stable_id_test.go`, `smoke_test.go` | ✅ green (61 PASS) |
| 59-04-02 | 04 | 2 | EXTRACT-03 (TS+JS shared provider) | T-59-04-02 | TS + TSX + JS + JSX + MJS + CJS via two grammars; 30+ matrix | golden-file | `go test ./internal/semantic/extract/typescript/... -count=1` | ✅ `provider_test.go`, `stable_id_test.go`, `smoke_test.go` | ✅ green (60 PASS) |
| 59-04-03 | 04 | 2 | EXTRACT-03 (Python provider) | T-59-04-02 | 30+ matrix passes | golden-file | `go test ./internal/semantic/extract/python/... -count=1` | ✅ `provider_test.go`, `stable_id_test.go`, `smoke_test.go` | ✅ green (61 PASS) |
| 59-04-04 | 04 | 2 | EXTRACT-04 (partial:true marker) | — | 4 partial-reason paths (unsupported, file-too-large, binary/generated, parse-error) | unit | `go test ./internal/semantic/extract/... -run TestPartialMarker -count=1` | ✅ `internal/semantic/extract/partial_test.go` | ✅ green |
| 59-05-01 | 05 | 2 | EXTRACT-05 (GrammarRegistry singleton) | T-59-05-01 | dual gate: pointer-equality + static grep | regression | `go test ./internal/daemon/... -run TestBootstrap_GrammarRegistrySingleton -count=1` AND `go test ./internal/semantic/extract/... -run TestNoNewGrammarRegistry -count=1` | ✅ `internal/daemon/daemon_grammar_test.go`, `internal/semantic/extract/registry_grep_test.go` | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [x] `internal/semantic/extract/` package skeleton with `extract_test.go` stubs for EXTRACT-01..04
- [x] `internal/semantic/extract/golang/testdata/` directory with at least 30 before/after scenario fixtures (40+ delivered)
- [x] `internal/semantic/extract/typescript/testdata/` directory with at least 30 before/after scenario fixtures (40+ delivered)
- [x] `internal/semantic/extract/python/testdata/` directory with at least 30 before/after scenario fixtures (40+ delivered)
- [x] `internal/daemon/daemon_grammar_test.go` — singleton-injection regression test for EXTRACT-05
- [x] `internal/semantic/store/migrations_test.go` — Apply()-roundtrip + forward-incompat tests for the new schema migration

*Sampling rationale:* The 30+ before/after matrix per first-class language is the contract test for stable symbol-IDs. Wave 0 stubs let property tests fail-loud on canonicalization regressions before any provider lands.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| ROADMAP success criterion #3 corrected from "7-rung" → "5-rung" | EXTRACT-04 | Documentation amend; verified by reading the file post-edit | `grep "5-rung" .planning/milestones/v1.10-ROADMAP.md` returns the Phase 59 success criterion |
| SPEC §25 updated to add ExtractionConfig sub-struct | (config layout decision) | Documentation amend | Grep SPEC for `ExtractionConfig` and confirm the four new keys are documented |
| File-walk worker absent in merged tree (T-59-04-01 / T-59-04-03 transferred to Phase 60) | EXTRACT-02 (Phase 60 scope) | No production caller of `Provider.Extract` exists today; size+timeout gates have no enforcement point until Phase 60 lands the worker | `grep -rn "\.Extract(ctx" internal/semantic/ internal/daemon/ internal/kernel/ \| grep -v _test.go` returns 0 matches; documented in 59-SECURITY.md "TRANSFERRED to Phase 60" section |

---

## Validation Audit 2026-05-04

| Metric | Count |
|--------|-------|
| Tasks audited | 9 |
| COVERED | 9 |
| PARTIAL | 0 |
| MISSING | 0 |
| Manual-Only added | 1 (Phase 60 file-walk worker handoff) |

All requirements have automated verification. Full suite green: `go test ./internal/semantic/... ./internal/daemon/... -count=1`. Per-language matrix totals — Go: 61 PASS · TS+JS: 60 PASS · Python: 61 PASS — exceed the 30-scenario-per-language Wave 0 contract.

The Manual-Only entry for the file-walk worker is informational: the worker is out-of-scope for Phase 59. Phase 60's PLAN.md must inherit T-59-04-01 / T-59-04-03 as acceptance criteria (recorded in 59-SECURITY.md).

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 120s (full suite ~120s)
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** signed-off 2026-05-04

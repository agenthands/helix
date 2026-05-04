---
phase: 59
slug: tree-sitter-extraction-stable-symbol-ids
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-05-04
---

# Phase 59 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

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
| 59-01-01 | 01 | 0 | EXTRACT-01..05 (schema migration) | — | N/A | integration | `go test ./internal/semantic/store/... -run TestMigration -count=1` | ❌ W0 | ⬜ pending |
| 59-02-01 | 02 | 1 | EXTRACT-01 (stable symbol-ID) | — | symbol IDs survive whitespace/rename/move | unit + property | `go test ./internal/semantic/extract/... -run "TestStableSymbolKey\|TestCanonicalization" -count=1` | ❌ W0 | ⬜ pending |
| 59-02-02 | 02 | 1 | EXTRACT-04 (5-rung confidence) | — | 5-rung ladder enforced | unit | `go test ./internal/semantic/extract/... -run TestConfidenceLadder -count=1` | ❌ W0 | ⬜ pending |
| 59-03-01 | 03 | 1 | EXTRACT-02 (extraction scheduler) | — | RequireReady gate respected | integration | `go test ./internal/semantic/extract/scheduler/... -count=1` | ❌ W0 | ⬜ pending |
| 59-04-01 | 04 | 2 | EXTRACT-03 (Go provider) | — | 30+ before/after matrix passes | golden-file | `go test ./internal/semantic/extract/golang/... -count=1` | ❌ W0 | ⬜ pending |
| 59-04-02 | 04 | 2 | EXTRACT-03 (TS+JS provider) | — | 30+ matrix passes | golden-file | `go test ./internal/semantic/extract/typescript/... -count=1` | ❌ W0 | ⬜ pending |
| 59-04-03 | 04 | 2 | EXTRACT-03 (Python provider) | — | 30+ matrix passes | golden-file | `go test ./internal/semantic/extract/python/... -count=1` | ❌ W0 | ⬜ pending |
| 59-04-04 | 04 | 2 | EXTRACT-04 (partial:true marker for non-supported langs) | — | partial envelope shape | unit | `go test ./internal/semantic/extract/... -run TestPartialMarker -count=1` | ❌ W0 | ⬜ pending |
| 59-05-01 | 05 | 2 | EXTRACT-05 (GrammarRegistry singleton) | — | single canonical registry; pointer-equality + static grep | regression | `go test ./internal/daemon/... -run TestGrammarRegistrySingleton -count=1` AND `go test ./internal/semantic/extract/... -run TestNoNewGrammarRegistry -count=1` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/semantic/extract/` package skeleton with `extract_test.go` stubs for EXTRACT-01..04
- [ ] `internal/semantic/extract/golang/testdata/` directory with at least 30 before/after scenario fixtures
- [ ] `internal/semantic/extract/typescript/testdata/` directory with at least 30 before/after scenario fixtures
- [ ] `internal/semantic/extract/python/testdata/` directory with at least 30 before/after scenario fixtures
- [ ] `internal/daemon/daemon_grammar_test.go` — singleton-injection regression test stub for EXTRACT-05
- [ ] `internal/semantic/store/migrations_test.go` — Apply()-roundtrip test stub for the new schema migration

*Sampling rationale:* The 30+ before/after matrix per first-class language is the contract test for stable symbol-IDs. Wave 0 stubs let property tests fail-loud on canonicalization regressions before any provider lands.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| ROADMAP success criterion #3 corrected from "7-rung" → "5-rung" | EXTRACT-04 | Documentation amend; verified by reading the file post-edit | `grep "5-rung" .planning/milestones/v1.10-ROADMAP.md` returns the Phase 59 success criterion |
| SPEC §25 updated to add ExtractionConfig sub-struct | (config layout decision) | Documentation amend | Grep SPEC for `ExtractionConfig` and confirm the four new keys are documented |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 120s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending

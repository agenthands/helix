---
phase: 95
slug: identity-docs-rewrite-docgen-regen
status: passed
nyquist_compliant: true
wave_0_complete: true
created: 2026-06-22
audited: 2026-06-22
---

# Phase 95 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution. Retroactively audited and closed 2026-06-22 (Phase 96 TD-05) — phase shipped + passed verification; this reconciles the Nyquist coverage map against the actual gates on disk.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test + docgen/--check drift gate + grep-based doc assertions |
| **Config file** | none — `go.mod`, `Makefile`, `.github/workflows/go-test.yml` |
| **Quick run command** | `go run ./cmd/docgen --check` (drift gate) + `go test ./cmd/docgen/... -count=1` |
| **Full suite command** | `go vet ./... && go test ./... -count=1` + `make verify-docs` |
| **Estimated runtime** | ~30–60 s |

---

## Sampling Rate

- **After every task commit:** `go run ./cmd/docgen --check` + `go test ./cmd/docgen/...`
- **After the docs rewrite:** grep assertions that no doc claims MCP as the *primary agent interface* (DOCS-01) and that the CLAUDE.md Helix routing cites real `helix` verbs (DOCS-02)
- **Before `/gsd-verify-work`:** `make verify-docs` green; full `go test ./...` green
- **Max feedback latency:** ~60 s

---

## Per-Task Verification Map

| Plan | Requirement | Test Type | Automated Command | Status |
|------|-------------|-----------|-------------------|--------|
| 95-01 | DOCS-02 (README tool table re-keyed to `helix <verb>`; docgen drift gate wired into CI + `make verify-docs`) | drift-gate + unit | `make verify-docs` (= `go run ./cmd/docgen --check`, "README.md is up to date") + `go test ./cmd/docgen/ -run TestToolTableContainsKnownTools` (asserts verb forms `helix go-to-definition` etc.) | ✅ green |
| 95-02 | DOCS-01 (no doc claims MCP as the primary agent interface; CLI-first framing) | grep assertion | `grep -inE 'primary (agent )?interface\|53 MCP tools' README.md CLAUDE.md .planning/PROJECT.md` → zero MCP-as-primary hits (true "MCP-SDK/gRPC retained internally" statements preserved) | ✅ green |
| 95-02 | DOCS-03 (CLAUDE.md Helix-CLI routing matrix cites `helix` verbs end-to-end; external SMTC matrix left intact) | grep / structural | Helix-CLI routing matrix present in CLAUDE.md citing real `cli.VerbToolNames()` verbs; `mcp__smtc__*` SMTC matrix byte-unchanged with a non-conflation clarifier | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [x] `docgen --check` CI step + `make verify-docs` target added — shipped in 95-01 (CI step "docgen drift gate (DOCS-02)" in `.github/workflows/go-test.yml`, mirroring `verify-cligen`); closes the v1.12 docgen-drift hole.
- [x] `cmd/docgen/main_test.go` raw-tool-name assertions updated to kebab verb forms — `TestToolTableContainsKnownTools` asserts `helix go-to-definition` etc.

*Existing infra (`go test`, `cmd/docgen --check`) covers phase requirements; no new framework needed.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Prose reads as genuinely CLI-first (not just keyword-swapped) | DOCS-01 | Editorial quality is not grep-checkable | Human skim of README/CLAUDE.md/PROJECT.md intros after the rewrite |

*The MCP-as-primary claim removal (the measurable part) is grep-asserted; the editorial-quality check is the manual backstop.*

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] `docgen --check` drift gate green AND wired into CI (`make verify-docs`)
- [x] No doc claims MCP as the primary agent interface (grep-asserted)
- [x] `git diff go.mod` empty
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-06-22 (retroactive audit, Phase 96 TD-05)

## Validation Audit 2026-06-22

| Metric | Count |
|--------|-------|
| Gaps found | 0 |
| Resolved | 0 |
| Escalated | 0 |

DOCS-01..03 all COVERED: DOCS-02 by the `make verify-docs` drift gate (green, "README.md is up to date") + `TestToolTableContainsKnownTools` verb-form assertion (green); DOCS-01 by the grep gate (zero MCP-as-primary claims across the 3 shipped docs); DOCS-03 by the added Helix-CLI routing matrix. Both Wave-0 items (CI drift-gate step + verb-form test assertions) were shipped by the phase itself. No MISSING gaps → auditor not spawned (workflow §3). The single manual-only item (editorial CLI-first prose quality) is a documented backstop with a grep-asserted measurable proxy.

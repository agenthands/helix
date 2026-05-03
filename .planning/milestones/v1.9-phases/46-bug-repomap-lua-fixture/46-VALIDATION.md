---
phase: 46
slug: bug-repomap-lua-fixture
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-24
---

# Phase 46 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — stdlib `testing` |
| **Quick run command** | `go test ./internal/repomap/...` |
| **Full suite command** | `go test ./... && go vet ./...` |
| **Estimated runtime** | ~60 seconds (quick) / ~3-5 min (full) |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/repomap/...`
- **After every plan wave:** Run `go test ./... && go vet ./...`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 60 seconds (quick), 300s (full)

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 46-01-xx | 01 | 1 | BUG-01 | — | N/A | unit (reproduction) | `go test ./internal/repomap/ -run TestPolyglotRankDominance` | ❌ W0 | ⬜ pending |
| 46-02-xx | 02 | 2 | BUG-01 | — | N/A | unit (fix) | `go test ./internal/repomap/...` | ❌ W0 | ⬜ pending |
| 46-03-xx | 03 | 2 | BUG-01 | — | N/A | oracle/integration | `go test ./test/oracle/...` | ❌ W0 | ⬜ pending |

---

## Wave 0 Requirements

- [ ] `internal/repomap/polyglot_rank_test.go` — synthetic Go+Lua polyglot fixture; asserts ≥1 Go symbol in ranked output AND Lua fixture does not dominate. Must fail pre-fix (negative control) and pass post-fix.
- [ ] `test/oracle/` smoke test — pins the Serena-workspace reproduction. Top-N `get_repo_map` output on repo root contains ≥1 `internal/` Go symbol.
- [ ] `testdata/` directory for synthetic fixture (Go `pkg/` + nested `testdata/fixtures/lua/deep/nested/fixture/*.lua`).

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Real-world reproduction on Serena repo | BUG-01 success criterion 1 | Requires live workspace + MCP invocation | Run `get_repo_map` via MCP on `/Users/Janis_Vizulis/go/src/github.com/postfix/serena` after clearing tag cache; confirm `internal/` Go symbols dominate top-N. |
| RCA write-up | BUG-01 success criterion 3 | Human-authored narrative | Documented in phase review / SUMMARY.md noting which of the 4 candidate causes was confirmed and the evidence. |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 60s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending

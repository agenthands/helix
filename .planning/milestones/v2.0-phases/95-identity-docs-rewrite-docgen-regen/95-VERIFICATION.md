---
phase: 95-identity-docs-rewrite-docgen-regen
verified: 2026-06-22T00:00:00Z
status: passed
score: 13/13 must-haves verified
behavior_unverified: 0
overrides_applied: 0
deferred:
  - truth: "README:332 `helix get-tool-help` row says 'any MCP tool' (off-message)"
    addressed_in: "Follow-up source-Description task (IN-01, deferred in 95-REVIEW-FIX)"
    evidence: "Auto-generated row sourced from internal/kernel/help/ tool Description; hand-editing README is reverted by the docgen --check CI gate (DOCS-02). Not a Phase-95 doc-edit defect; requires a source change + docgen regen."
---

# Phase 95: Identity & Docs Rewrite + docgen Regen Verification Report

**Phase Goal:** Rewrite Helix's identity to CLI-first across README, CLAUDE.md, PROJECT.md so no doc claims MCP as the primary agent interface; update CLAUDE.md tool-routing guidance to reference `helix <verb>`; regenerate the auto-generated tool table against the frozen CLI surface behind a green drift gate.
**Verified:** 2026-06-22
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
| --- | --- | --- | --- |
| 1 | No doc claims MCP as the primary agent interface | ✓ VERIFIED | `grep 'primary interface' README.md CLAUDE.md .planning/PROJECT.md` → 0 matches |
| 2 | CLI-first framing consistent across README/CLAUDE.md/PROJECT.md | ✓ VERIFIED | README:19,47; CLAUDE.md:20,179; PROJECT.md:5,9 all CLI-first ("agents drive `helix <verb>` CLI") |
| 3 | MCP-SDK/gRPC described as retained internal plumbing (NOT removed) | ✓ VERIFIED | CLAUDE.md:20,94,179; README:19,47; PROJECT.md:5 — "retained as internal daemon plumbing"; no "MCP removed" over-claim |
| 4 | README tool table lists `helix <verb>` names (not raw MCP names) | ✓ VERIFIED | 51 `\| \`helix ` rows in table; 0 raw underscored tool names inside BEGIN/END TOOLS region |
| 5 | `go run ./cmd/docgen --check` exits 0 | ✓ VERIFIED | "README.md is up to date." exit 0 |
| 6 | `make verify-docs` exits 0 | ✓ VERIFIED | runs `go run ./cmd/docgen --check`, exit 0 |
| 7 | CI go-test.yml contains `go run ./cmd/docgen --check` step (v1.12 hole closed) | ✓ VERIFIED | go-test.yml:108-112 "docgen drift gate (DOCS-02)" |
| 8 | docgen tests pass with verb-form assertions | ✓ VERIFIED | `go test ./cmd/docgen/ -count=1` → ok; asserts `helix go-to-definition` |
| 9 | CLAUDE.md has Helix-CLI routing matrix citing real `helix <verb>` names | ✓ VERIFIED | "## Helix CLI tool routing" header; 24 `\| \`helix ` rows; all cited verbs ∈ verbs_gen.go (kebab) |
| 10 | External SMTC (mcp__smtc__*) matrix left intact | ✓ VERIFIED | `grep -c mcp__smtc__ CLAUDE.md` = 31 = HEAD baseline; clarifier present |
| 11 | Stale tool counts (53 / 41+) reconciled to 50 frozen verbs | ✓ VERIFIED | 0 "53 callable/MCP tools" in README/CLAUDE; only 41+ match is a dated v1.8 historical changelog line (PROJECT.md:83, left intact per plan) |
| 12 | `git diff go.mod` empty (zero new deps) | ✓ VERIFIED | clean |
| 13 | Build + full untagged test suite green | ✓ VERIFIED | `go build ./...` exit 0; `go test ./...` no failures |

**Score:** 13/13 truths verified (0 present, behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
| --- | --- | --- | --- |
| `cmd/docgen/main.go` | Verb-keyed row emitter | ✓ VERIFIED | `verb := strings.ReplaceAll(tool.Name, "_", "-")` at line 129; blank-import parity cross-ref comment at line 23 |
| `cmd/docgen/main_test.go` | Verb-form assertions | ✓ VERIFIED | asserts `helix go-to-definition`; tests pass |
| `Makefile` | verify-docs HARD-FAIL gate | ✓ VERIFIED | target at line 83 + `.PHONY` entry line 1 |
| `.github/workflows/go-test.yml` | CI docgen drift-gate step | ✓ VERIFIED | step at lines 108-112 |
| `internal/daemon/imports.go` | Reciprocal parity cross-ref + D-02 preserved | ✓ VERIFIED | cross-ref comment line 6; D-02 invariant comment; 0 active semantic/extract imports |
| `README.md` | CLI-first hero + verb table + clean config | ✓ VERIFIED | hero/How-it-Works CLI-first; no `--mode=stdio/http`; no "Generic MCP client"/"client registration" |
| `CLAUDE.md` | CLI-first identity + Helix routing matrix | ✓ VERIFIED | identity/Constraints reframed; routing matrix added; stale lines fixed |
| `.planning/PROJECT.md` | CLI-first What-This-Is/Core Value/Constraints | ✓ VERIFIED | line 5/9 CLI-first; canonical "only surface an agent touches" Core Value |

### Key Link Verification

| From | To | Via | Status | Details |
| --- | --- | --- | --- | --- |
| cmd/docgen/main.go | README.md | replaceSection BEGIN/END TOOLS writes verb rows | ✓ WIRED | --check confirms table matches live registry |
| go-test.yml | cmd/docgen/main.go | CI invokes `go run ./cmd/docgen --check` | ✓ WIRED | step present + run line confirmed |
| CLAUDE.md routing matrix | internal/cli/verbs_gen.go | cites real frozen verb names | ✓ WIRED | all 24 routing verbs ∈ frozen set (kebab); `helix status` is in arch prose, a real CLI cmd (status.go) |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
| --- | --- | --- | --- | --- |
| DOCS-01 | 95-02 | Identity rewrite (no MCP-as-primary, CLI-first consistent) | ✓ SATISFIED | Truths 1-3, 11; "all four docs" acceptance reconciled — 3 shipped docs reframed, STATE.md (planning source) already CLI-first |
| DOCS-02 | 95-01 | docgen table regen to helix verbs + drift gate green | ✓ SATISFIED | Truths 4-8, 12; CI gate ADDED (did not exist before) |
| DOCS-03 | 95-02 | CLAUDE.md routing → helix verbs, SMTC intact | ✓ SATISFIED | Truths 9-10; new matrix cites real verbs end-to-end |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| --- | --- | --- | --- |
| docgen drift gate green | `go run ./cmd/docgen --check` | "README.md is up to date." exit 0 | ✓ PASS |
| make gate green | `make verify-docs` | exit 0 | ✓ PASS |
| docgen verb-form tests | `go test ./cmd/docgen/ -count=1` | ok | ✓ PASS |
| build clean | `go build ./...` | exit 0 | ✓ PASS |
| full suite green | `go test ./...` | no failures | ✓ PASS |
| zero new deps | `git diff go.mod` | empty | ✓ PASS |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
| --- | --- | --- | --- | --- |
| Makefile | 179 | `mktemp -t helix-smoke-XXXXXX` | ℹ️ Info | False positive — mktemp template, not a debt marker |
| README.md | 332 | "any MCP tool" in get-tool-help row | ℹ️ Info | Auto-generated from source Description; deferred (IN-01) — cannot hand-edit (docgen gate reverts) |

No `TODO`/`FIXME`/`XXX`/`TBD` debt markers in phase-modified files.

### Human Verification Required

None. All truths are presence/wiring/gate-verifiable for a docs-only phase; gates run green in-process. (95-02 plan suggested an optional human skim that intros read as genuinely CLI-first — confirmed by inspection of README:19/47, CLAUDE.md:20, PROJECT.md:5/9, which are substantive CLI-first prose, not keyword swaps.)

### Gaps Summary

No gaps. Phase 95 goal is achieved. All three requirements (DOCS-01, DOCS-02, DOCS-03) are satisfied:

- Identity is CLI-first across README/CLAUDE.md/PROJECT.md with zero MCP-as-primary-agent-interface claims, while accurately retaining the MCP-SDK + gRPC `StreamMCP` as internal daemon plumbing (no over-claim of removal).
- The auto-generated README tool table is re-keyed to `helix <verb>` rows behind a real, newly-added CI drift gate (`go run ./cmd/docgen --check`) and `make verify-docs`, both green; go.mod unchanged.
- CLAUDE.md carries a new Helix-CLI routing matrix citing only real frozen verbs, with the external SMTC matrix preserved byte-for-byte (31 occurrences, unchanged).

The code-review's 1 BLOCKER + 5 WARNINGS (incomplete staleness sweep) are all confirmed fixed in commits 46207709 and 35841b39 — CLAUDE.md no longer says "one-command MCP registration", "Streamable HTTP (direct)", "claude mcp add-json", or "53 callable tools"; README no longer says "Generic MCP client"/"client registration".

Two acceptance-text nuances, both benign and not goal-blocking:
1. DOCS-01 acceptance says "all four docs" but only 3 shipped docs carry the claim; STATE.md (a planning source) is already CLI-first and supplies the canonical Core Value the others reuse. Documented reconciliation in 95-02-SUMMARY.
2. README:332 "any MCP tool" is an auto-generated row from a source-level tool Description; correctly deferred (IN-01) as it cannot be hand-edited under the docgen gate.

---

_Verified: 2026-06-22_
_Verifier: Claude (gsd-verifier)_

---
phase: 95-identity-docs-rewrite-docgen-regen
plan: 02
subsystem: docs
tags: [cli-first, identity, mcp-retirement, claude-md, readme, routing-matrix]

# Dependency graph
requires:
  - phase: 95-01
    provides: verb-keyed README tool table + docgen --check drift gate (verify-docs)
  - phase: 94
    provides: deletion of the agent-facing MCP heads (stdio forwarder + Streamable-HTTP /mcp); SDK/gRPC retained internally
provides:
  - CLI-first identity prose across README.md, CLAUDE.md, and .planning/PROJECT.md (zero MCP-as-primary-agent-interface claims)
  - A new "Helix CLI tool routing" decision matrix in CLAUDE.md citing real frozen helix verbs
  - Stale README config blocks (removed --mode=stdio/--mode=http//mcp) rewritten to helix setup + CLI usage
  - Honest count reconciliation (53/41+ -> 50 frozen verbs / count-free phrasing)
affects: [future-docs-updates, onboarding, milestone-summary]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "CLI-first identity framing: agents drive `helix <verb>` via Bash; MCP Go SDK + gRPC IPC are retained internal daemon plumbing (NOT removed)"
    - "Helix-own routing matrix mirrors the SMTC decision-matrix FORMAT but cites real frozen verbs from internal/cli/verbs_gen.go"

key-files:
  created: []
  modified:
    - README.md
    - CLAUDE.md
    - .planning/PROJECT.md

key-decisions:
  - "DOCS-03 satisfied by ADDING a Helix-CLI routing matrix (none existed to rewrite), not by editing the external SMTC matrix"
  - "External SMTC (mcp__smtc__*) matrix left byte-for-byte intact; mcp__smtc__ count unchanged at 31 vs HEAD"
  - "Reused canonical v2.0 Core Value verbatim from STATE.md:26 in CLAUDE.md + PROJECT.md"
  - "Reworded my own explanatory text to avoid literal dead-flag strings (--mode=stdio etc.) so the strict DOCS-01 grep gate stays green"
  - "Dated historical PROJECT.md line 83 (v1.8 Phase 39 '41+ tools') left intact per plan directive (not a present-tense identity claim)"

patterns-established:
  - "Pattern 1: When a doc must mention a removed flag to explain its removal, describe it ('transport modes were removed') rather than printing the literal flag, to satisfy negative grep gates"
  - "Pattern 2: A product routing matrix and an external dev-tool routing matrix coexist in CLAUDE.md with an explicit clarifier preventing conflation"

requirements-completed: [DOCS-01, DOCS-03]

# Metrics
duration: 18min
completed: 2026-06-21
status: complete
---

# Phase 95 Plan 02: Identity Docs Rewrite (CLI-First) Summary

**Rewrote Helix's identity from MCP-primary to CLI-first across README/CLAUDE.md/PROJECT.md and added a Helix-CLI tool-routing decision matrix to CLAUDE.md citing the 50 real frozen verbs, with the external SMTC matrix left byte-for-byte intact.**

## Performance

- **Duration:** ~18 min
- **Started:** 2026-06-21T23:01Z
- **Completed:** 2026-06-21T23:19Z
- **Tasks:** 3
- **Files modified:** 3

## Accomplishments
- Every present-tense MCP-as-primary-agent-interface claim in the three docs reframed to CLI-first; the MCP Go SDK + gRPC IPC are accurately described as retained internal daemon plumbing (NOT removed).
- New `## Helix CLI tool routing` decision matrix added to CLAUDE.md — 24 rows, all citing real frozen `helix <verb>` names verified against `internal/cli/verbs_gen.go`.
- Stale README config blocks documenting the Phase-94-deleted `--mode=stdio`/`--mode=http`/`/mcp` rewritten to `helix setup` + per-verb CLI usage.
- Stale counts (CLAUDE.md "53 MCP tools", README/PROJECT "41+ MCP tools") reconciled to the 50 frozen verbs / count-free phrasing per STATE.md:84.
- The external SMTC routing matrix preserved exactly (`mcp__smtc__` count unchanged at 31 vs HEAD); a one-line clarifier now distinguishes the two surfaces.

## Task Commits

Each task was committed atomically:

1. **Task 1: CLI-first identity rewrite across README/CLAUDE.md/PROJECT.md** - `617e07bc` (docs)
2. **Task 2: Add the Helix-CLI tool-routing matrix to CLAUDE.md** - `e40f7ae9` (docs)
3. **Task 3: Phase-wide consistency + drift-gate re-verification** - verification-only, no file changes (all gates already green after Tasks 1-2)

## Files Created/Modified
- `README.md` - CLI-first hero/intro + How-Helix-Works; manual-config blocks rewritten to `helix setup`; internal-detail lines (admin listener, architecture runtime) reworded off stale MCP transports.
- `CLAUDE.md` - identity/Core Value/Constraints reframed CLI-first; both `Protocol` lines reframed; NEW Helix-CLI routing matrix added above the (untouched) SMTC section with a clarifier.
- `.planning/PROJECT.md` - What-This-Is / Core Value / Constraints `Protocol` line reframed CLI-first; the "## Context" product summary count reconciled to 50 verbs; the v2.0 milestone meta-line reworded to drop the literal old "primary interface" quote.

## Decisions Made
- **DOCS-03 via ADD, not rewrite:** there is no Helix-own routing matrix in CLAUDE.md today, so DOCS-03's "routing matrix cites CLI verbs end-to-end" is satisfied by authoring one. Rewriting the external SMTC matrix would be factually wrong (Helix ships no taint/CFG/IR tools).
- **Reused canonical Core Value** verbatim from STATE.md:26 to keep wording consistent across STATE/CLAUDE/PROJECT.
- **No over-claim:** docs say the agent-facing heads were removed and the SDK/gRPC are retained internally — never "MCP removed."

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Reworded explanatory text + a meta/historical line to pass strict negative grep gates**
- **Found during:** Task 1 (identity rewrite) and confirmed in Task 3 (gate re-run)
- **Issue:** (a) My new README explanatory sentence named the removed flags (`--mode=stdio`, `--mode=http`, `/mcp`) to explain they were removed — which tripped the Task 3 `! grep -- '--mode=stdio\|--mode=http'` and `/mcp` gates. (b) PROJECT.md:190, a v2.0 milestone meta-line, quoted the literal old `"Protocol: MCP — primary interface"` phrase, tripping the `primary interface` gate. (c) PROJECT.md:203 ("## Context") carried a stale "41+ MCP tools" product-state count.
- **Fix:** (a) Reworded to "legacy agent-facing daemon transport modes and HTTP endpoint were removed; only the internal auto daemon remains" (no literal dead-flag strings). (b) Reworded the milestone line to describe the rewrite without quoting the dead phrase. (c) Reconciled the "## Context" count to "50 frozen `helix` CLI verbs". The dated `✓ ... — v1.8 Phase 39` historical line (PROJECT.md:83, "41+ tools") was left intact per the plan's explicit "do NOT rewrite history" directive.
- **Files modified:** README.md, .planning/PROJECT.md
- **Verification:** Task 3 gates re-run green: zero `primary interface`, zero `--mode=stdio/http`, zero `/mcp`, zero `53/41+ MCP tools` across the three docs.
- **Committed in:** `617e07bc` (Task 1 commit)

**2. [Rule 1 - Bug] Removed an accidental `mcp__smtc__*` token that bumped the protected SMTC count 31 -> 32**
- **Found during:** Task 2 (routing matrix add)
- **Issue:** My clarifier note used the literal `mcp__smtc__*`, which incremented the `mcp__smtc__` occurrence count to 32 — violating the critical invariant that the count must equal the HEAD baseline (31).
- **Fix:** Reworded the clarifier to "the `smtc` MCP tools" (no `mcp__smtc__` literal).
- **Files modified:** CLAUDE.md
- **Verification:** `grep -c 'mcp__smtc__' CLAUDE.md` == 31 == `git show HEAD:CLAUDE.md | grep -c 'mcp__smtc__'`.
- **Committed in:** `e40f7ae9` (Task 2 commit)

---

**Total deviations:** 2 auto-fixed (1 blocking gate-compliance reword, 1 invariant-protecting fix)
**Impact on plan:** Both fixes were necessary to satisfy the plan's own acceptance gates and critical invariants. No scope creep; all changes remained docs-only.

## REQUIREMENTS reconciliation note
The success criteria flagged a possible "four docs" wording in REQUIREMENTS. In practice only THREE shipped docs carry the MCP-as-primary claim and were rewritten (README.md, CLAUDE.md, .planning/PROJECT.md). STATE.md is a planning source and is already CLI-first (its line 26 is the canonical v2.0 Core Value that the other docs now reuse), so it required no identity rewrite. The CLI-first identity is therefore consistent across the three shipped docs and the planning source.

## Threat Flags
None. Pure documentation edits; no runtime code path, network, or auth surface touched. T-95-02 (Information Disclosure) mitigated: docs use the precise framing (internal gRPC over a unix socket by default) and do not misstate listener exposure or claim MCP was removed. T-95-SC: zero new dependencies (`git diff go.mod` empty).

## Known Stubs
None.

## Issues Encountered
None beyond the two auto-fixed deviations above. The docgen drift gate (`go run ./cmd/docgen --check`) and `make verify-docs` both stayed green after the prose edits, confirming the README tool table (between BEGIN/END TOOLS markers) was never touched.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- This is the FINAL v2.0 plan. The docs now describe the frozen, MCP-head-free CLI-first shape after Phase 94.
- All v2.0 DOCS requirements (DOCS-01, DOCS-03; DOCS-02 in plan 95-01) are satisfied; milestone is ready for completion/audit.

## Self-Check: PASSED

- FOUND: `.planning/phases/95-identity-docs-rewrite-docgen-regen/95-02-SUMMARY.md`
- FOUND commits: `617e07bc` (Task 1), `e40f7ae9` (Task 2), `ddaf880b` (SUMMARY)

---
*Phase: 95-identity-docs-rewrite-docgen-regen*
*Completed: 2026-06-21*

# Phase 44: Re-Verify Phase 41 and USAGE-02 - Research

**Researched:** 2026-04-23
**Domain:** Documentation verification / audit-trail closure (procedural)
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-01:** Re-run Phase 40's verification by overwriting `40-VERIFICATION.md` in place. Flip the three `gaps_found` items (get_repo_map parameter name, fuzzy_edit named-args example, rust-analyzer rename troubleshooting) to `SATISFIED` with new evidence citing the Phase 43 commits that fixed them. Git history preserves the prior version; do not create a `-v2.md`.
- **D-02:** Phase 41's new `41-VERIFICATION.md` mirrors the `40-VERIFICATION.md` template exactly — requirement-ID-by-requirement-ID table, same section order, same evidence-citation style. Covers INST-01, INST-02, CONT-01, CONT-02.
- **D-03:** Verification evidence must cite real artifacts (file:line, commit SHA, or `serena setup --output` output). No claims without a citation.
- **D-04:** Re-run the full v1.8 integration check — all 11 findings (F-01 through F-13), not just the three criticals. Produces a `status_total` comparable to the original check, which is what SC-3 ("no remaining critical findings") actually requires to assert.
- **D-05:** Overwrite `.planning/v1.8-INTEGRATION-CHECK.md` with the re-run. Git preserves the prior version. A single canonical milestone integration check is the intended artifact.
- **D-06:** Any residual critical findings that surface in the re-run are deferred to Phase 45, not fixed in Phase 44.
- **D-07:** SC-3 interpretation: zero open criticals OR every critical explicitly `deferred_to: 45` in the integration-check frontmatter counts as resolved.
- **D-08:** Three plans, executed sequentially — 44-01 (41-VERIFICATION.md), 44-02 (re-run 40-VERIFICATION.md), 44-03 (re-run integration check).
- **D-09:** Sequencing is required: 44-03 reads both verification files to assert `phases_verified: [39, 40, 41, 42]`.

### Claude's Discretion

- Exact phrasing, section ordering, and evidence formatting within each VERIFICATION.md (provided the template mirrors Phase 40).
- Which commits from Phase 43 to cite as evidence for which re-flipped gap.
- Whether to re-use the existing integration-check section structure verbatim or adjust headings to make re-run deltas obvious.
- Commit granularity inside each plan (atomic per-file vs. combined).

### Deferred Ideas (OUT OF SCOPE)

- README manual-config cross-reference polish (F-02) — Phase 45.
- Architecture-terminology alignment across CLAUDE.md / CONTRIBUTING.md (F-10, F-11 if residual) — Phase 45 candidate.
- CHANGELOG v1.7 client-count fixup (F-07) — Phase 45 if not absorbed by Phase 43.
- Nyquist-compliant upgrade of `40-VALIDATION.md` / `39-VALIDATION.md` — dedicated validation phase.
- Human UAT scenarios from `39-HUMAN-UAT.md` — deferred indefinitely.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| USAGE-02 | USAGE.md documents all v1.7 features (setup CLI, health tool, hooks, smart errors, progressive descriptions, lazy init) — re-opened by v1.8 audit | Phase 40 already rated USAGE-02 as SATISFIED (40-VERIFICATION.md row 7 in Requirements Coverage). The three `gaps_found` items were attributed to USAGE-01/USAGE-03 in the per-truth table. Phase 43 closed the cross-doc client-list drift for USAGE via commits `fb53c7fb` / `067f7ec4`. See §Evidence Map → USAGE-02. |
| INST-01 | INSTALL.md reflects current install paths and `serena setup <client>` as primary method | Phase 41-01 rewrote INSTALL.md (`b3932958`); Phase 43-02 added opencode + generic to Quick Start (`3491ba0d`); Phase 41 restoration commit (`7ce77d23`) brought back manual configs for Codex/OpenCode/Cursor/Antigravity. See §Evidence Map → INST-01. |
| INST-02 | INSTALL.md has accurate MCP configs for all 7 supported clients | Current INSTALL.md Quick Start enumerates all 7 registered clients at lines 36–42. Manual Configuration block covers 9 clients (the 7 registered plus Cursor + Antigravity). See §Evidence Map → INST-02. |
| CONT-01 | CONTRIBUTING.md reflects current Go codebase structure and dev workflow | Phase 41-02 SUMMARY lists 10 added packages + test/oracle + test/harness; `internal/kernel/fileops/` corrected to "7 file operation tools" (CONTRIBUTING.md:53). See §Evidence Map → CONT-01. |
| CONT-02 | CONTRIBUTING.md references current test harness (oracle tests, integration tags, benchmark gates) | CONTRIBUTING.md:105–125 documents 6-layer oracle hierarchy (protocol, contract, runtime, scenario, llm, judge). See §Evidence Map → CONT-02. |
</phase_requirements>

## Summary

This phase is procedural re-verification of already-shipped work — no new code, no new documentation. The three artifacts produced are (1) a new `41-VERIFICATION.md` written from scratch against the existing `40-VERIFICATION.md` template, (2) an in-place overwrite of `40-VERIFICATION.md` flipping three `gaps_found` items to `SATISFIED`, and (3) an in-place overwrite of `.planning/v1.8-INTEGRATION-CHECK.md` re-running all 11 findings.

All evidence needed to close these verifications is already on disk. Phase 40's three original gaps were closed by Phase 40-03 commits (`f2c835bb`, `036baae2`) BEFORE Phase 43 existed; the verification metadata simply went stale. Phase 43's commits additionally closed cross-doc drift that the integration check flagged (F-01, F-07, F-08, F-10, F-11). Phase 41's evidence surface maps cleanly to INST-01/INST-02/CONT-01/CONT-02 via `b3932958` (INSTALL+CONTRIBUTING rewrite) plus `3491ba0d` / `7ce77d23` (INSTALL additions).

**Primary recommendation:** Copy the `40-VERIFICATION.md` YAML frontmatter + section structure verbatim for `41-VERIFICATION.md`. Use the commit SHA + `file:line` citation style already in use. Cite the Phase 43 commits by SHA short-form (8 chars) exactly as this research does.

## Architectural Responsibility Map

Not applicable — this phase writes no code and touches no application tiers. All three artifacts are planning documents under `.planning/`. Omitted per template guidance.

## Template Fidelity (for 44-01)

The `40-VERIFICATION.md` structure the planner must mirror exactly:

### YAML Frontmatter Schema
```yaml
---
phase: NN-slug-name                 # e.g., 41-install-contributing
verified: 2026-04-23TXX:XX:XXZ      # ISO 8601 UTC
status: passed | gaps_found         # final verdict
score: N/M must-haves verified      # free-form string
overrides_applied: 0                # integer (zero unless waivers used)
gaps:                               # list; empty or omitted when status=passed
  - truth: "Observable truth text"
    status: failed | satisfied
    reason: "…"
    artifacts:
      - path: "USAGE.md"
        issue: "Line N: description"
    missing:
      - "Required follow-up item"
---
```
[VERIFIED: .planning/phases/40-usage-refresh/40-VERIFICATION.md:1-33]

### Body Section Order (MUST be reused verbatim)

1. `# Phase N: <Name> Verification Report` (H1 title)
2. Header bullets: **Phase Goal**, **Verified**, **Status**, **Re-verification**
3. `## Goal Achievement`
   - `### Observable Truths` — numbered table with columns `# | Truth | Status | Evidence`
   - `**Score:** N/M truths verified`
   - `### Required Artifacts` — table `Artifact | Expected | Status | Details`
   - `### Key Link Verification` — table `From | To | Via | Status | Details`
   - `### Data-Flow Trace (Level 4)` — "Not applicable -- documentation-only phase…" for doc phases
   - `### Behavioral Spot-Checks` — "Step 7b: SKIPPED (documentation-only phase…)" for doc phases
   - `### Requirements Coverage` — table `Requirement | Source Plan | Description | Status | Evidence`
   - `### Anti-Patterns Found` — table `File | Line | Pattern | Severity | Impact`
   - `### Human Verification Required` — "None" for doc phases
   - `### Gaps Summary` — narrative; omit or "None — all truths verified." when status=passed
4. `---`
5. Footer: `_Verified: <timestamp>_` and `_Verifier: Claude (gsd-verifier)_`

[VERIFIED: .planning/phases/40-usage-refresh/40-VERIFICATION.md:35-119]

### Evidence Citation Style

- **File:line form:** `USAGE.md:173`, `internal/cli/setup_clients.go:39-45`
- **Commit form (Phase 43 carry-over):** append commit SHA short-form, e.g., `USAGE.md:25 (closed by fb53c7fb)`
- Where multiple citations combine (code + doc), stack them inside the Evidence cell: `USAGE.md:146 — closed by f2c835bb; code source internal/kernel/fileops/tools.go`

## Evidence Map (for 44-01 and 44-02)

### USAGE-02 / USAGE-01 / USAGE-03 Gap Closure (for 44-02)

Every original Phase 40 `gaps_found` item has an identifiable closing commit. Two of the three were closed by **Phase 40-03** (same phase, later plan) and one was further refined by Phase 43:

| Original Gap (40-VERIFICATION.md gaps[N]) | Closing Commit | File Evidence (post-fix) | Status Flip |
|------|----------------|-------------------------|-------------|
| `gaps[0]` — get_repo_map parameter `max_tokens` → `token_budget` | `f2c835bb` — `docs(40-03): fix RepoMap parameter name and fuzzy_edit code example` | `USAGE.md:153` ("Accepts a `token_budget` parameter (default 4096, max 32768)"), `USAGE.md:159` (`get_repo_map(token_budget=4096)`) | failed → satisfied |
| `gaps[1]` — fuzzy_edit positional args → named params | `f2c835bb` (same commit) | `USAGE.md:146` (`fuzzy_edit(path="src/handler.go", search="func HandleRequest(\n...\n)", replacement="func HandleRequest(ctx context.Context,\n...\n)")`) | failed → satisfied |
| `gaps[2]` — rust-analyzer rename troubleshooting missing | `036baae2` — `docs(40-03): add rust-analyzer rename troubleshooting entry` | `USAGE.md:531-537` (Symptom/Cause/Workaround block) | failed → satisfied |

[VERIFIED: `git log --oneline -- USAGE.md`; file content inspected via Read]

**Additional USAGE-02 drift closed by Phase 43** (relevant for the re-run's cross-doc consistency claim):

| Integration Finding | Closing Commit (Phase 43) | Evidence |
|---------------------|--------------------------|----------|
| F-01 — Supported-clients list undercount | `fb53c7fb` — `docs(43-03): add opencode to USAGE supported clients` | `USAGE.md:25`, `USAGE.md:173` both list all 7 clients including `opencode` |
| F-08 — fuzzy-strategy naming drift (Exact/Whitespace/IndentFlex/Failed → canonical) | `067f7ec4` — `docs(43-03): use canonical fuzzy strategy names in USAGE` | `USAGE.md:141` now reads "exact match, whitespace-normalized, indentation-flexible, ellipsis-placeholder" |

[VERIFIED: `grep -n "opencode\|indentation-flexible" USAGE.md` returns both matches]

**Note for 44-02 planner:** The existing `40-VERIFICATION.md` Requirements Coverage row for USAGE-02 is already marked `SATISFIED`. The phase-level `status: gaps_found` is driven by USAGE-01 and USAGE-03 per-truth failures (truths 1, 2, 9 in the Observable Truths table). The re-run flips those truths to VERIFIED, flips USAGE-01 and USAGE-03 Requirements Coverage rows from PARTIAL → SATISFIED, clears the gaps list, drops the Anti-Patterns Found table to empty, and flips `status: gaps_found` → `passed`.

### Phase 41 Evidence Surface (for 44-01)

| REQ | Evidence Anchor | Closing Commit(s) | Observable Truths to Enumerate |
|-----|-----------------|-------------------|---------------------------------|
| INST-01 | `INSTALL.md:1-47` (Prerequisites + Quick Start) | `b3932958` (41-01 rewrite), `3491ba0d` (43-02 opencode+generic in Quick Start) | (a) `serena setup <client>` is the primary install path (INSTALL.md:32); (b) `go install` + PATH verification present (INSTALL.md:7-30); (c) Quick Start enumerates all 7 registered clients verbatim (INSTALL.md:36-42). |
| INST-02 | `INSTALL.md:55-216` (Manual Configuration section) | `b3932958`, `7ce77d23` (restore Codex/OpenCode/Cursor/Antigravity), `3491ba0d` | (a) Manual-config section has blocks for Claude Code, Codex, Gemini CLI, VS Code, JetBrains, Claude Desktop, OpenCode, Cursor, Antigravity, Generic (INSTALL.md:60-216); (b) Quick Start covers all 7 `clientRegistry()` entries from `internal/cli/setup_clients.go:39-45`; (c) HTTP Mode covered as separate section. Reconcile with canonical registry (7 clients) — INST-02 is met because all 7 are in Quick Start; manual-config superset is additive and documents unmanaged agents. |
| CONT-01 | `CONTRIBUTING.md` project-structure section | `b3932958` (same commit group — 41-02 executed in parallel with 41-01) | (a) 4-layer architecture described; (b) `internal/cli/`, `internal/errors/`, `internal/fuzzy/`, `internal/kernel/health/`, `internal/kernel/help/`, `internal/kernel/jsonrpc/`, `internal/repomap/`, `internal/skill/repomap/`, `internal/treesitter/`, `internal/workspace/`, `internal/memory/` all listed; (c) `internal/kernel/fileops/` line reads "7 file operation tools (includes fuzzy_edit)" (CONTRIBUTING.md:53); (d) `test/harness/` and `test/oracle/` listed. |
| CONT-02 | `CONTRIBUTING.md:105-125` (Oracle test suite section) + existing integration/bench sections | `b3932958` | (a) 6-layer oracle table enumerates protocol/contract/runtime/scenario/llm/judge with paths and commands; (b) integration test section retained (`test/integration/`); (c) benchmark / benchstat CI gate documented; (d) test harness package referenced. |

[VERIFIED: file contents inspected via Read; commit stats via `git log --oneline -- CONTRIBUTING.md INSTALL.md`]

### Cross-Phase Causality Note

The commit `b3932958` appears in both Phase 41-01-SUMMARY.md and Phase 41-02-SUMMARY.md because the two plans ran in parallel and the SUMMARY was written after a single-commit merge. In practice, the INSTALL.md and CONTRIBUTING.md diffs are separate code paths inside that commit. `41-VERIFICATION.md` can cite `b3932958` for both INST-* and CONT-* requirements without ambiguity. [VERIFIED: both SUMMARY.md files' Task tables]

## Integration Re-Check Mechanics (for 44-03)

### Finding Status Delta (Original → Re-run)

| ID | Severity | Original Status | Closing Commit(s) | Re-run Status | Notes |
|----|----------|-----------------|-------------------|---------------|-------|
| F-01 | critical | open | `9f5c1bb9` (README), `3491ba0d` (INSTALL), `fb53c7fb` (USAGE), `71eb4406` (CHANGELOG) | **resolved** | All four docs now enumerate 7 clients including `opencode`. Verify with `grep opencode README.md INSTALL.md USAGE.md CHANGELOG.md`. |
| F-02 | warning | open | — | **open → deferred_to: 45** | README manual-config cross-index to INSTALL not yet added. Explicitly deferred (CONTEXT.md). |
| F-03 | info | open | `28052801` (README 41+) | **resolved** | README now says "41+" matching CLAUDE.md. [VERIFIED: `git log 28052801 --name-only`] |
| F-04 | critical (carry-over) | open | `f2c835bb` | **resolved** | `token_budget` parameter name used consistently (USAGE.md:153, :159). |
| F-05 | critical (carry-over) | open | `f2c835bb` | **resolved** | Named-args form at USAGE.md:146. |
| F-06 | warning (carry-over) | open | `036baae2` | **resolved** | Rust-analyzer rename entry at USAGE.md:531-537. |
| F-07 | warning | open | `71eb4406` — `docs(43-04): correct CHANGELOG v1.7 to 7 setup clients` | **resolved** | CHANGELOG.md:8 now reads "7 clients — Claude Code, VS Code, JetBrains, Claude Desktop, Gemini CLI, OpenCode, and generic MCP clients". |
| F-08 | warning | open | `067f7ec4` (USAGE), `65691ab5` (CLAUDE.md) | **resolved** | All three docs use canonical names `exact match, whitespace-normalized, indentation-flexible, ellipsis-placeholder`. |
| F-09 | (no finding — healthy) | healthy | — | healthy | — |
| F-10 | warning | open | `28052801` (README Layer 3 label), plus CLAUDE.md already correct | **resolved — VERIFY** | Need to re-check whether README Layer 1 one-liner now mentions RepoMap/fuzzy/health/help (F-10 had two prongs: Layer 3 label + Layer 1 reductive). [ASSUMED] If Layer 1 prose was NOT updated, F-10 is only partially resolved and the residue belongs in Phase 45. The planner should re-verify by reading README.md:315-327 during 44-03 execution. |
| F-11 | warning | open | `0609a550` — CLAUDE.md fileops 6→7 | **resolved** | CLAUDE.md:57 now reads "7 file operation tools (read, write, list, find, search, replace, fuzzy_edit)". |
| F-12 | info | open | — | **open → deferred_to: 45** | Asymmetric cross-linking (README↛CHANGELOG, USAGE↛INSTALL). Explicitly deferred. |
| F-13 | critical | open | 44-01 output (`41-VERIFICATION.md` created by this phase) | **resolved by this phase** | Resolution is internal to Phase 44. 44-03 must run AFTER 44-01 commits so the artifact exists on disk. |

[VERIFIED: all commit SHAs cross-checked against `git log --oneline -- <file>` output]
[CITED: `.planning/v1.8-INTEGRATION-CHECK.md` for original finding statuses]

### Frontmatter Delta for Re-run

| Field | Original | Re-run |
|-------|----------|--------|
| `status` | `gaps_found` | `passed` (assuming no fresh criticals surface) |
| `critical_count` | 3 | 0 (or N with all entries carrying `deferred_to: 45`) |
| `warning_count` | 6 | 0-2 depending on F-10 prong + any fresh drift |
| `phases_verified` | `[39, 40, 42]` | `[39, 40, 41, 42]` |
| `phases_unverified` | `[41]` | `[]` |

Add a new scalar field `rerun_of: 2026-04-23` (or equivalent) in frontmatter, plus a short prologue noting which findings flipped and which are deferred — CONTEXT.md D-08/D-09 allow this adjustment under Claude's Discretion.

### Per-Finding Deferral Schema

For residual findings (F-02, F-12, and F-10 if Layer 1 prose unresolved), add machine-readable deferral to the finding block's metadata. The planner can choose between two formats (both count under D-07):

```yaml
# Option A — inline in findings: block
findings:
  - id: F-02
    severity: warning
    status: deferred
    deferred_to: 45
    rationale: "README manual-config pointer polish is Phase 45 scope (CONTEXT.md:106)."
```

Or a Markdown-level note after the finding's Recommendation paragraph:

> **Deferred:** tagged `deferred_to: 45`. Remediation scheduled for Phase 45 (Cross-link & Manual-Config Polish).

Either satisfies the "machine-readable inbox" criterion in CONTEXT.md:100.

## Canonical Source of Truth for F-01 Reconciliation

The integration-check re-run MUST reconcile client-count claims against the canonical registry, not against any other doc:

```go
// internal/cli/setup_clients.go:36-46
// clientRegistry returns all 7 registrars keyed by client name.
func clientRegistry() map[string]ClientRegistrar {
    return map[string]ClientRegistrar{
        "claude-code":    &ClaudeCodeRegistrar{},
        "vscode":         &VSCodeRegistrar{},
        "jetbrains":      &JetBrainsRegistrar{},
        "claude-desktop": &ClaudeDesktopRegistrar{},
        "gemini-cli":     &GeminiCLIRegistrar{},
        "opencode":       &OpenCodeRegistrar{},
        "generic":        &GenericRegistrar{},
    }
}
```
[VERIFIED: file read 2026-04-23; line 36 comment corrected by `8005d4b4`]

The canonical 7-client ordered list to use in verification evidence: `claude-code, vscode, jetbrains, claude-desktop, gemini-cli, opencode, generic`.

## Risks / Drift Watchlist

Before 44-03 is written, the re-runner should re-verify the following surfaces in case post-Phase-43 drift has crept in:

1. **README Layer 1 one-liner** (`README.md:315-327`) — F-10 had two prongs; Phase 43-01 fixed the Layer 3 label but I did not confirm whether the Layer 1 "LS worker pool, symbol ops, file ops, diagnostics" line was also expanded. If it was NOT, the re-run either (a) keeps F-10 as a warning tagged `deferred_to: 45`, or (b) the integration check classifies this as "partial" and splits F-10 into F-10a (resolved) + F-10b (open-deferred). [ASSUMED — risk of missed scope]
2. **README → CHANGELOG cross-link** (F-12) — low-stakes, explicitly deferred.
3. **Fresh criticals from drift** — since Phase 43, was any doc touched that re-introduced a count mismatch? `git log --since=2026-04-22 --oneline -- README.md USAGE.md INSTALL.md CHANGELOG.md CLAUDE.md CONTRIBUTING.md` should return only Phase 43 + Phase 44 commits before re-run. If any other commit appears, re-audit that file. [VERIFIED: at research time only Phase 43 commits appear in scope]
4. **40-VERIFICATION.md Observable Truths table semantics** — the re-run needs to decide whether to keep 9 rows (flipping 1, 2, 9 to VERIFIED) or re-number. Safest default: keep the 9 rows unchanged, flip Status column cells, update Evidence column to reference closing commits. Preserves diffability in git history.

## Plan Sequencing Constraints (for the planner)

Because 44-03 frontmatter asserts `phases_verified: [39, 40, 41, 42]`, it cannot run until both 44-01 and 44-02 commits exist on disk. D-08/D-09 already require sequential execution. The planner should NOT place these three plans in the same execution wave — they are intrinsically ordered:

```
Wave 1: 44-01 (write 41-VERIFICATION.md)
Wave 2: 44-02 (overwrite 40-VERIFICATION.md)   [depends on Wave 1 only for narrative symmetry, not file-level; could also run in Wave 1]
Wave 3: 44-03 (overwrite v1.8-INTEGRATION-CHECK.md)
```

Alternatively, 44-01 and 44-02 could run in parallel in Wave 1 (they touch different files and have no data dependency), with 44-03 in Wave 2. Either structure honors D-09.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead |
|---------|-------------|-------------|
| Verification schema | A new YAML frontmatter shape for 41-VERIFICATION.md | The existing 40-VERIFICATION.md frontmatter verbatim. D-02 mandates mirroring. |
| Commit attribution | A new `evidence_commits:` field in frontmatter | Plain prose in the Evidence column, with SHA short-form (8 chars) suffixed. Matches the existing citation convention. |
| Residual-finding tracking | A separate `.planning/v1.8-BACKLOG.md` file | Inline `deferred_to: 45` per-finding tag in `.planning/v1.8-INTEGRATION-CHECK.md` frontmatter. D-07 already permits this. |

## Common Pitfalls

### Pitfall 1: Writing evidence without a citation

**What goes wrong:** An Evidence cell reads "RepoMap parameter is correct" without a file:line or commit anchor.
**Why it happens:** Verification-author fatigue — restating the truth instead of anchoring it.
**How to avoid:** Every Evidence cell MUST contain at minimum `path/to/file:line` OR a commit SHA. D-03 is explicit. A reviewer should be able to run one `grep` or `git show` to validate each row.

### Pitfall 2: Flipping USAGE-02 without addressing USAGE-01 / USAGE-03 truths

**What goes wrong:** 44-02 targets USAGE-02 but the original `gaps_found` was driven by USAGE-01 and USAGE-03 truths (rows 1, 2, 9). Flipping only USAGE-02 leaves phase status stuck at `gaps_found`.
**Why it happens:** CONTEXT.md names USAGE-02 as the re-verification target, which misleads a reader about which per-truth rows to flip.
**How to avoid:** 44-02 flips ALL three `gaps` entries regardless of REQ-ID affinity. The phase status key is `status: gaps_found → passed`, which requires zero `gaps[]` entries — not a specific REQ-ID being satisfied.

### Pitfall 3: Re-introducing stale counts while writing 41-VERIFICATION.md

**What goes wrong:** 44-01's author reads Phase 41-CONTEXT.md, which says "6 supported clients", and writes `INST-02 satisfied — INSTALL.md documents 6 clients`. That is stale; Phase 43 updated the registry to 7.
**Why it happens:** Phase 41-CONTEXT.md predates Phase 43.
**How to avoid:** Phase 41 success criteria shifted post-hoc to "7 clients". Always reconcile against `internal/cli/setup_clients.go:36-46` (current: 7 clients). The ROADMAP.md:149 also says "6 supported clients" — that is also stale. Canonical source is the Go file.

### Pitfall 4: Double-citing Phase 43 commits when Phase 40-03 is the true closer

**What goes wrong:** 44-02's author cites Phase 43 commits for the three original Phase 40 gaps. Wrong: the gaps were closed by Phase 40-03 (`f2c835bb`, `036baae2`) before Phase 43 existed. Phase 43 only closed cross-doc drift (F-01, F-08), not the three original gaps.
**How to avoid:** Use the Evidence Map table above. Phase 40-03 commits are the original closers. Phase 43 commits are an additional remediation layer.

## Project Constraints (from CLAUDE.md)

- **GSD workflow enforcement:** Edits must flow through a GSD command (this phase is already running under `/gsd-plan-phase`, so compliant).
- **Go Development Commands:** `go vet ./...` and `go test ./...` before completing Go tasks. *Not applicable — this phase writes no Go code.*
- **Legacy framing:** Python legacy kept to "Originally inspired by" phrasing. *Relevant only if 41-VERIFICATION.md references LEGC-01; it does not — Phase 41 scope is INST/CONT only.*

## Environment Availability

Skipped — this phase has no external dependencies. All work is Markdown authoring + `git` operations.

## Validation Architecture

Skipped — Nyquist validation is not applicable to doc-authoring phases. The doc-phase convention is `VALIDATION.md: draft, nyquist_compliant: false` (as Phase 39 and 40 already record). If the project-level config requires this section, the planner may emit a one-line "N/A — documentation-only phase" entry.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | README Layer 1 one-liner expansion (F-10 prong 2) was NOT explicitly addressed by Phase 43 — risk it remains a warning | §Integration Re-Check Mechanics → F-10 row, §Risks 1 | If actually addressed, one fewer residual; re-run integration check simply flips F-10 to `resolved` without deferral. If still open, it's explicitly deferred per CONTEXT.md — net effect: trivial. 44-03 author verifies by reading README.md:315-327 before writing the status. |

No other claims in this research are assumed. Every commit SHA, file path, and line number was verified via `Read` tool or `git log`.

## Sources

### Primary (HIGH confidence)
- `.planning/phases/40-usage-refresh/40-VERIFICATION.md` — full read, template source
- `.planning/v1.8-INTEGRATION-CHECK.md` — full read, all 11 findings
- `.planning/v1.8-MILESTONE-AUDIT.md` — full read, audit context
- `.planning/phases/41-install-contributing/41-CONTEXT.md`, `41-01-SUMMARY.md`, `41-02-SUMMARY.md` — full read
- `.planning/phases/43-cross-doc-sync/43-03-SUMMARY.md`, `43-05-SUMMARY.md`, `43-06-SUMMARY.md` — full read
- `.planning/REQUIREMENTS.md`, `.planning/ROADMAP.md` — full read
- `internal/cli/setup_clients.go:36-46` — source of truth for client count
- `git log --oneline` filtered per file — verified all cited commit SHAs
- Current file contents of `INSTALL.md`, `USAGE.md`, `CONTRIBUTING.md`, `CHANGELOG.md`, `README.md`, `CLAUDE.md` — spot-read via `grep` + `Read`

### Secondary (MEDIUM confidence)
- None — every finding in this research was verified against a primary artifact.

### Tertiary (LOW confidence)
- A1 in Assumptions Log only.

## Metadata

**Confidence breakdown:**
- Template fidelity: HIGH — 40-VERIFICATION.md read in full, structure reproduced.
- Evidence map: HIGH — all commits verified via `git log --oneline -- <file>`.
- Integration delta: HIGH-MEDIUM — one prong of F-10 marked ASSUMED (A1); all others verified.

**Research date:** 2026-04-23
**Valid until:** 2026-05-23 (30 days — procedural phase, stable)

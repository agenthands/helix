---
phase: 45-polish-crosslinks
plan: 01
subsystem: docs
tags: [docs, cross-links, readme, usage, v1.8]
requirements_closed: [README-03, USAGE-03, INST-02]
findings_closed: [F-02, F-06, F-12]
dependency_graph:
  requires: []
  provides:
    - "Explicit 7-client manual-config pointer in README"
    - "README → CHANGELOG Release Notes link"
    - "USAGE → INSTALL Prerequisites pointer"
    - "F-06 rust-analyzer currency confirmation + block hash"
  affects:
    - README.md
    - USAGE.md
    - .planning/phases/45-polish-crosslinks/45-VERIFICATION.md
tech_stack:
  added: []
  patterns:
    - "Inline [Text](path.md#anchor) Markdown link style (matches existing doc tone)"
    - "Content-hash pinning of troubleshooting blocks for future drift detection"
key_files:
  created:
    - .planning/phases/45-polish-crosslinks/45-VERIFICATION.md
  modified:
    - README.md
    - USAGE.md
decisions:
  - "D-02 locked sentence used verbatim for README 7-client pointer"
  - "D-04 CHANGELOG link placed above closing footer `---` rule"
  - "D-05 USAGE INSTALL pointer merged into existing line-5 paragraph to preserve rust-analyzer block byte-position (deviation from 'distinct new line' instruction; presentation-only)"
  - "D-07/D-08 rust-analyzer block requires no edit; confirmed via read-only content check + SHA-256 hash capture"
metrics:
  started: 2026-04-24T21:43:36Z
  completed: 2026-04-24T21:46:20Z
  duration: "~3 min"
  tasks: 4
  files_modified: 2
  files_created: 1
---

# Phase 45 Plan 01: Cross-link & Manual-Config Polish Summary

Three deterministic markdown polish edits closing non-blocking v1.8 integration-check findings F-02, F-06, F-12 — no code, no scope expansion, verified by grep + content hashing.

## Objective Recap

Close three Phase-44-deferred v1.8 integration-check findings:

- **F-02** — Replace vague README manual-config pointer with explicit 7-client sentence naming Cursor, Antigravity, VS Code, JetBrains, Claude Desktop, Gemini CLI, and OpenCode.
- **F-12** — Add README → CHANGELOG Release Notes link + USAGE → INSTALL Prerequisites link (docs cross-link symmetry).
- **F-06** — Read-only confirmation that USAGE.md:531–537 rust-analyzer troubleshooting block still reflects current (v1.90) behavior; no edit expected.

## Tasks Completed

| # | Task                                                              | Commit    | Files                                                     |
| - | ----------------------------------------------------------------- | --------- | --------------------------------------------------------- |
| 1 | F-02 — Replace README vague pointer with 7-client sentence        | `762f3c9a` | README.md                                                 |
| 2 | F-12a — Add README Release Notes link to CHANGELOG                | `33a2555e` | README.md                                                 |
| 3 | F-12b — Add USAGE intro pointer to INSTALL                        | `2958451f` | USAGE.md                                                  |
| 4 | F-06 — rust-analyzer currency confirmation + 45-VERIFICATION.md  | `005d4c20` | .planning/phases/45-polish-crosslinks/45-VERIFICATION.md  |

## Exact Diffs Applied

### README.md (2 edits — 1 replacement + 1 insertion)

```diff
@@ -132,7 +132,7 @@ serena --serve --http-addr=:9091
 # Connect your client to http://localhost:9091/mcp
 ```

-For detailed configuration options, see [INSTALL.md](INSTALL.md).
+For Cursor, Antigravity, VS Code, JetBrains, Claude Desktop, Gemini CLI, and OpenCode — see [INSTALL.md#manual-configuration](INSTALL.md#manual-configuration) for full examples.

 </details>

@@ -338,6 +338,8 @@ A significant part of Serena...
 We are very grateful for the many contributors...
 what it is today.

+See [CHANGELOG.md](CHANGELOG.md) for release notes.
+
 ---

 <sub>Originally inspired by [Python Serena](https://github.com/lks-ai/serena).</sub>
```

### USAGE.md (1 merged-sibling edit)

```diff
@@ -2,7 +2,7 @@

 This guide covers operational usage of Serena...

-For installation and feature overview, see [README.md](README.md).
+For installation and feature overview, see [README.md](README.md). If you haven't installed Serena yet, see [INSTALL.md](INSTALL.md) first.

 ## Quick Tutorials
```

### 45-VERIFICATION.md (created)

New 207-line verification record capturing grep outputs, the rust-analyzer block SHA-256 hash, ROADMAP success-criterion mapping, and the Task-3 deviation rationale.

## rust-analyzer Block Hash (F-06)

```
sed -n '531,537p' USAGE.md | shasum -a 256
→ 7586749f007a7237f56612bb45aa184b8448c387bff49964be8bed40679c0f26
```

Block is byte-identical pre- and post-phase. Hash recorded in `45-VERIFICATION.md` for future drift detection.

## 45-VERIFICATION.md Confirmation

`.planning/phases/45-polish-crosslinks/45-VERIFICATION.md` written (commit `005d4c20`, 207 lines). Contains:

- F-02 positive + negative greps and anchor-integrity check.
- F-12 greps for README → CHANGELOG and USAGE → INSTALL.
- F-06 token-by-token content verification table + SHA-256 hash.
- ROADMAP success-criterion mapping (3/3 SATISFIED).
- REQ-ID traceability (README-03, USAGE-03, INST-02 closed).
- Task-3 deviation documentation.

## REQ IDs Closed

- **README-03** — README correctly points users to INSTALL.md for clients not documented inline. Closed by Task 1.
- **USAGE-03** — USAGE cross-links to INSTALL and retains accurate troubleshooting. Closed by Tasks 3 and 4.
- **INST-02** — INSTALL `## Manual Configuration` is canonical target for 7-client docs. Closed by Task 1 (anchor link `#manual-configuration` validated).

## Findings Closed

- **F-02** (warning) — vague README manual-config pointer → **RESOLVED**.
- **F-06** (warning) — rust-analyzer troubleshooting currency → **RESOLVED** (read-only + hash).
- **F-12** (info) — missing README→CHANGELOG and USAGE→INSTALL cross-links → **RESOLVED**.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Sentence insertion position adjusted to preserve rust-analyzer block byte-position**

- **Found during:** Task 3 (USAGE → INSTALL pointer).
- **Issue:** Plan instructed inserting the new INSTALL-pointer sentence as a **distinct new line** immediately after USAGE.md line 5. Doing so shifted all subsequent content down by one, moving the rust-analyzer troubleshooting block from 531–537 to 532–538. This broke:
  - Plan must_have truth: "USAGE.md lines 531–537 rust-analyzer troubleshooting block is unchanged."
  - Task 4 acceptance greps using `sed -n '531,537p' USAGE.md | grep -q "Workaround:"` / `replace_symbol_body` (these tokens moved out of the 531–537 window).
- **Fix:** Merged the new sentence as a sibling on the SAME line as the existing README backlink (USAGE.md line 5), producing one two-sentence paragraph rather than two one-sentence paragraphs. Preserves:
  - Task-3 acceptance criteria (head-20 contains INSTALL link, exact sentence present, README backlink preserved, count == 1).
  - Byte-stable rust-analyzer block (hash unchanged: `7586749f...`).
  - Higher-priority must_have truth about block stability.
- **Trade-off:** Presentation is a single two-sentence paragraph instead of two paragraphs. Semantic content and reading order are equivalent; markdown renderers treat both forms identically.
- **Files modified:** USAGE.md.
- **Commit:** `2958451f`.

No other deviations. Everything else matches the plan exactly.

### Auth Gates

None — docs-only phase, no tools requiring authentication.

## Verification Outcomes

All phase-level acceptance greps exit 0:

```text
F-02a OK — 7-client sentence + anchor link present in README
F-02b OK — old vague pointer removed
F-12a OK — README Release Notes link present
F-12b OK — USAGE head-20 contains INSTALL link
F-12c OK — exact USAGE→INSTALL sentence present
F-06 v1.90 OK
F-06 Symptom: OK
F-06 Cause: OK
F-06 Workaround: OK
F-06 replace_symbol_body OK
anchor OK — INSTALL.md ## Manual Configuration still exists
VERIFICATION.md OK
```

## Success Criteria

- [x] README manual-config block points to INSTALL.md#manual-configuration and explicitly names all 7 clients (ROADMAP #1, REQ README-03, INST-02).
- [x] README contains Release Notes link to CHANGELOG.md in footer area AND USAGE contains INSTALL.md link in Prerequisites/Setup intro (ROADMAP #2, REQ README-03, USAGE-03).
- [x] USAGE.md:531–537 rust-analyzer block confirmed current (v1.90 + Symptom/Cause/Workaround + replace_symbol_body) and byte-unchanged (ROADMAP #3, REQ USAGE-03).
- [x] 45-VERIFICATION.md records all four task outcomes, block SHA-256 hash, ROADMAP→REQ mapping.
- [x] All verification greps exit 0.

No Go source modified — `go vet` / `go test` not applicable.

## Self-Check: PASSED

Verified file existence and commit hashes:

- `.planning/phases/45-polish-crosslinks/45-VERIFICATION.md` — FOUND.
- README.md and USAGE.md — FOUND in the diff range (commits `762f3c9a`, `33a2555e`, `2958451f`).
- Commit `762f3c9a` (Task 1) — FOUND.
- Commit `33a2555e` (Task 2) — FOUND.
- Commit `2958451f` (Task 3) — FOUND.
- Commit `005d4c20` (Task 4) — FOUND.

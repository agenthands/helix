# Phase 45: Cross-link & Manual-Config Polish - Research

**Researched:** 2026-04-24
**Domain:** Markdown documentation edits (README, USAGE, INSTALL, CHANGELOG)
**Confidence:** HIGH

## Summary

Phase 45 is a narrow, text-only documentation polish phase closing three non-blocking v1.8 integration-check findings (F-02, F-06, F-12). All decisions (D-01..D-09) are locked in CONTEXT.md. Research was limited to verifying current file state, exact edit targets, anchor names, and grep-based verification recipes.

All four target files (README.md, USAGE.md, INSTALL.md, CHANGELOG.md) were inspected directly. Anchor targets, line ranges, and surrounding-context patterns have been verified against the live repo state as of 2026-04-24. Drift from CONTEXT.md line references was minimal (the README:134 vague pointer is now at README:135 after a whitespace/content shift from Phase 39/40/44 commits).

**Primary recommendation:** Plan as a single small wave — three targeted edits to README.md, one to USAGE.md, and one verification-only read of USAGE.md:531–537. Verification = grep patterns spelled out in D-09 + a line-range hash/diff check on the rust-analyzer block.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**F-02 — README Manual-Config Pointer**
- **D-01:** Keep the existing 3 JSON examples (Claude Code, Codex, generic IDE Assistant) + HTTP mode in README's manual-config `<details>` block unchanged. Do NOT add more JSON examples.
- **D-02:** Replace the existing vague line `README.md:134` (`For detailed configuration options, see [INSTALL.md](INSTALL.md).`) with a single explicit sentence that names the other clients: "For Cursor, Antigravity, VS Code, JetBrains, Claude Desktop, Gemini CLI, and OpenCode — see [INSTALL.md#manual-configuration](INSTALL.md#manual-configuration)."
- **D-03:** Keep it one sentence. No bullet list, no table.

**F-12 — Cross-Links**
- **D-04:** Add README → CHANGELOG link. Place it at the end of README (near existing footer/docs-index area, where CONTRIBUTING and INSTALL are already referenced). Label: "Release Notes".
- **D-05:** Add USAGE → INSTALL link. Place it in USAGE's early Prerequisites / Setup section (natural reading order — "Install Serena first, see INSTALL.md").
- **D-06:** Do NOT add CONTRIBUTING → USAGE/README/INSTALL back-links. Defer.

**F-06 — rust-analyzer Currency**
- **D-07:** Text review only. Read USAGE.md:531–537, confirm the v1.90 reference and Symptom/Cause/Workaround structure still accurately describe the known rust-analyzer rename limitation. No live re-test.
- **D-08:** If text is already accurate: no edit needed — criterion #3 is satisfied by reading. Record the check in VERIFICATION.md.

**Verification Approach**
- **D-09:** Targeted grep + link checks at end of phase. No full v1.8 integration-check re-run. Checklist:
  - `grep "Cursor.*Antigravity.*VS Code.*JetBrains.*Claude Desktop.*Gemini CLI.*OpenCode" README.md` succeeds on one line with an `INSTALL.md` link.
  - `grep -i "release notes\|CHANGELOG" README.md` finds a link to `CHANGELOG.md`.
  - `grep "INSTALL.md" USAGE.md` finds a link in USAGE's setup/prereqs area.
  - `diff` confirms USAGE.md:531–537 text unchanged (or if updated, still matches rust-analyzer v1.90 behavior).

### Claude's Discretion
- Exact sentence wording for D-02, D-04, D-05 — Claude picks natural phrasing.
- Section anchor targets in INSTALL.md — verify actual anchor (see below, verified `#manual-configuration`).
- CHANGELOG link: whether to link to `CHANGELOG.md` root or to the latest release heading.

### Deferred Ideas (OUT OF SCOPE)
- CONTRIBUTING → USAGE/README/INSTALL back-links.
- Expanding README manual-config to 7 or 9 clients.
- Full v1.8 integration-check re-run after edits.
- Live rust-analyzer rename re-test.
</user_constraints>

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| README-03 | README correctly points users to INSTALL.md for clients not documented inline | D-02 edit replaces vague line with explicit 7-client sentence; anchor `#manual-configuration` verified |
| USAGE-03 | USAGE cross-links to INSTALL for installation and retains accurate troubleshooting | D-05 adds INSTALL link near USAGE line 5 area; D-07/D-08 confirms USAGE.md:531–537 rust-analyzer block unchanged |
| INST-02 | INSTALL's Manual Configuration section is the canonical target for 7-client documentation | Verified `## Manual Configuration` heading at INSTALL.md:55 → slug `#manual-configuration` |

## Project Constraints (from CLAUDE.md)

- **GSD Workflow Enforcement:** All edits must go through a GSD command (this phase uses `/gsd-execute-phase`). No direct repo edits outside the workflow.
- **No code changes:** This phase touches only `.md` files. CLAUDE.md rules about `go vet`/`go test` do not apply (no Go code modified).
- **Doc tone:** Existing docs use `[text](path.md)` / `[text](path.md#anchor)` inline Markdown links (no reference-style). New sentences must match.

## Current State Snapshot

### README.md (343 lines total)

| Target | Actual Line | Verified Text |
|--------|-------------|---------------|
| CONTEXT says "line 134" vague pointer | **Line 135** | `For detailed configuration options, see [INSTALL.md](INSTALL.md).` |
| Manual-config `<details>` block | Lines 90–137 | `<summary>` at 91, closing `</details>` at 137 |
| Existing 3 JSON blocks + HTTP (D-01 keep unchanged) | Lines 93–133 | Claude Code, Codex, IDE Assistant, HTTP mode |
| Footer / acknowledgements area | Lines 335–343 | `## Acknowledgements` at 335; closing horizontal rule + Python-Serena attribution at 341–343 |
| Existing CHANGELOG link | **NONE** | Grep for "CHANGELOG\|Release\|release notes" → 0 matches. Clean slate for D-04. |

**D-02 target:** Replace README.md line 135 (off by one from CONTEXT's "134" — drift from a prior edit). Plan must cite line 135.

**D-04 target:** Add "Release Notes" line. Best placement candidates, in order:
1. **Between the `## Architecture` block and `## Acknowledgements`** — awkward, Acknowledgements is emotional/credit copy.
2. **New footer line just before the final `<sub>` attribution at line 343** — clean, follows "docs-index" footer pattern.
3. **At the top of `## Acknowledgements`** — off-topic.

Recommendation: option 2 — add a new line after line 340 (after the horizontal rule, or replace/adjust the closing block) OR immediately before the rule at line 341. Exact spot left to planner + discretion; both satisfy D-04's "near existing footer/docs-index area."

**D-04 link target:** `CHANGELOG.md` root (not a specific heading). CHANGELOG.md line 1 = `# Changelog`. Latest release is `## v1.7 — Developer Experience & Auto-Setup (2026-04-22)` at line 5 — pinning the link to a version would require maintenance on every release, so root is safer.

### USAGE.md (817 lines total)

| Target | Actual Line | Verified Text |
|--------|-------------|---------------|
| CONTEXT says rust-analyzer block at "531–537" | **Lines 531–537 (UNCHANGED)** | Header line 531 `### rust-analyzer rename fails in fresh workspaces`; body through line 537 "The `replace_symbol_body` tool uses tree-sitter parsing rather than LSP rename, bypassing the rust-analyzer limitation entirely." Symptom/Cause/Workaround triple intact. v1.90 reference intact at line 535. |
| Existing README back-link | Line 5 | `For installation and feature overview, see [README.md](README.md).` |
| Existing INSTALL link | **NONE** | Grep shows zero `INSTALL.md` references in USAGE.md. |
| "Step 1: Install Serena" in Tutorial 1 | Line 13 | Mentions `go install` at line 16; does not link to INSTALL.md |
| No explicit "Prerequisites" heading exists | — | USAGE jumps from line 1 title → line 3 intro → line 5 README backlink → line 7 `## Quick Tutorials`. No dedicated Prerequisites section. |

**D-05 target (Claude's discretion on exact line):** Most natural placement is **immediately after the existing README backlink at line 5**, e.g., a new line 6 (or convert line 5's single sentence into a two-sentence paragraph) pointing to INSTALL.md. Alternative: inside Tutorial 1 Step 1 at line 13, adjacent to `go install`. Option 1 is cleaner because it mirrors the README-backlink pattern and is visible before the tutorial list.

**F-06 (D-07/D-08) outcome:** Text at USAGE.md:531–537 is verified present, structurally sound (Symptom / Cause / Workaround), mentions v1.90. No edit needed. The phase's verification task records a read-only confirmation and the current text hash for future drift detection.

### INSTALL.md (246 lines total)

| Target | Verified |
|--------|----------|
| Heading for D-02 link target | `## Manual Configuration` at line 55 |
| GitHub-flavored-markdown slug | `#manual-configuration` (lowercase, spaces → hyphen) |
| Seven client subsections present | `### Claude Code` (62), `### Gemini CLI` (72), `### VS Code` (82), `### JetBrains` (100), `### Claude Desktop` (115), `### Codex` (134), `### OpenCode` (151), `### Cursor` (169), `### Antigravity` (184), `### Generic (any MCP client)` (201) — all 7 D-02-named clients confirmed plus Codex + Generic. |

**Anchor verification:** `#manual-configuration` is the correct GFM slug. D-02's exact URL `[INSTALL.md#manual-configuration](INSTALL.md#manual-configuration)` is VALID. No ambiguity — no other `## Manual*` heading exists in INSTALL.md.

### CHANGELOG.md (206 lines total)

| Target | Verified |
|--------|----------|
| Top heading | `# Changelog` at line 1 |
| Latest release | `## v1.7 — Developer Experience & Auto-Setup (2026-04-22)` at line 5 |
| Stable top anchor? | `#changelog` via the `# Changelog` heading, but linking to file root `CHANGELOG.md` is equivalent and simpler |

## Exact Edit Targets (per decision)

| Decision | File | Line(s) | Action |
|----------|------|---------|--------|
| D-02 | README.md | 135 | Replace entire line with: `For Cursor, Antigravity, VS Code, JetBrains, Claude Desktop, Gemini CLI, and OpenCode — see [INSTALL.md#manual-configuration](INSTALL.md#manual-configuration) for full examples.` |
| D-04 | README.md | after 340 (before closing `<sub>` at 343) | Insert one-line "Release Notes" link. Suggested wording: `See [CHANGELOG.md](CHANGELOG.md) for release notes.` |
| D-05 | USAGE.md | after 5 | Insert one-line INSTALL pointer. Suggested wording: `If you haven't installed Serena yet, see [INSTALL.md](INSTALL.md) first.` |
| D-07/D-08 | USAGE.md | 531–537 | READ-ONLY confirmation. No edit. Record text verification in VERIFICATION.md. |

## Grep Verification Recipes

These exact commands should appear in plan acceptance_criteria and VERIFICATION.md:

```bash
# D-02: README has 7-client sentence with INSTALL anchor link
grep -E "Cursor.*Antigravity.*VS Code.*JetBrains.*Claude Desktop.*Gemini CLI.*OpenCode" README.md | grep -q "INSTALL.md#manual-configuration"

# D-04: README has CHANGELOG link (case-insensitive match on label OR filename)
grep -iE "\[release notes\]|CHANGELOG\.md" README.md | grep -q "CHANGELOG.md"

# D-05: USAGE links to INSTALL.md in early setup area (first 20 lines of file)
head -20 USAGE.md | grep -q "INSTALL.md"

# D-07/D-08: USAGE.md:531-537 rust-analyzer block unchanged content-wise
sed -n '531,537p' USAGE.md | grep -q "v1.90"
sed -n '531,537p' USAGE.md | grep -q "Symptom:"
sed -n '531,537p' USAGE.md | grep -q "Cause:"
sed -n '531,537p' USAGE.md | grep -q "Workaround:"
sed -n '531,537p' USAGE.md | grep -q "replace_symbol_body"

# Bonus: vague-pointer line removed from README (D-02 negative check)
! grep -qE "^For detailed configuration options, see \[INSTALL\.md\]\(INSTALL\.md\)\.$" README.md

# Link integrity (optional): INSTALL.md has the anchor target
grep -qE "^## Manual Configuration$" INSTALL.md
```

Optional (stronger F-06 stability): capture a hash of the block before/after the phase to detect accidental edits:

```bash
sed -n '531,537p' USAGE.md | shasum -a 256
# Record this hash in VERIFICATION.md; any future phase can diff it.
```

## Established Patterns (to match in new text)

From inspection of README.md and USAGE.md:

- **Link style:** inline `[Text](path.md)` / `[Text](path.md#anchor)`. No reference-style (`[Text][ref]`). D-02/D-04/D-05 wording all match.
- **Pointer-sentence form:** imperative "For X, see Y.md" (INSTALL uses this; README:135 uses it today). D-02 draft preserves it.
- **Em-dash usage:** README already uses `—` in prose (line 88 "lazy initialization — workspaces..."). D-02's em-dash is consistent.
- **Footer block in README:** line 341 `---` horizontal rule, line 343 `<sub>` attribution. Any insertion should land above the rule or between the rule and the `<sub>`, not after the `<sub>`.
- **USAGE intro pattern:** short backlink sentence at line 5 (README). D-05 mirrors this exactly as a sibling sentence.

## Risk Notes

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Anchor drift — INSTALL.md heading renamed after edit | LOW | Verified `## Manual Configuration` at line 55 today; planner's acceptance grep includes the anchor integrity check. |
| Line number drift between plan-time and execute-time | LOW | Planner should cite *content* (`For detailed configuration options...`) for the D-02 target string rather than line 135 alone. Edits should use string match, not line-addressing. |
| `<details>` block whitespace quirks | LOW | D-02 replaces a line *outside* the `</details>` tag (line 135 is after `</details>` at 137? — re-check). **Verified:** line 135 is the vague pointer, `</details>` is at line 137. Line 135 sits between line 133 (end of HTTP mode bash fence) and line 137 (`</details>`). The vague pointer is INSIDE the `<details>` block. Replacement stays inside — no GFM rendering change. |
| D-04 placement too far from CHANGELOG content context | LOW | Footer is explicitly where D-04 mandates; Acknowledgements/Attribution block is the canonical "docs index" tail. |
| D-05 disturbing Tutorial 1 flow | LOW | Placing after line 5 keeps it in the intro paragraph zone, well before `## Quick Tutorials` at line 7. |
| rust-analyzer v1.90 reference becoming stale during the phase window | NEGLIGIBLE | Phase window is ~1 day; F-06 text reflects a persistent LSP quirk, not a version-sensitive bug. |
| Markdown auto-link lint breakage | NEGLIGIBLE | No linter configured for these files; repo has no markdown CI gate. |

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | GitHub renders the slug `#manual-configuration` for heading `## Manual Configuration` (GFM default) | Anchor verification | Link silently broken. Mitigation: verify by clicking in GitHub after merge, or use repo's rendered preview. [CITED: GitHub docs — GFM auto-ids lowercase & replace spaces with hyphens] |
| A2 | Line 135 is the current vague-pointer line (drift of +1 from CONTEXT's "134") | Current State Snapshot | None — planner should match the pointer by string content, not line number. `[VERIFIED: Read of README.md lines 85–145]` |
| A3 | No other `## Manual*` heading exists in INSTALL.md to cause slug collision | Anchor verification | `[VERIFIED: grep of INSTALL.md headings]` |
| A4 | USAGE.md:531–537 byte-identical to the text added by Phase 40-03 commit `036baae2` | F-06 review | [ASSUMED] — did not diff against the commit; a before/after hash in VERIFICATION.md handles future drift. |

**Confirmation needed:** A4 only. The planner can upgrade A4 to `[VERIFIED]` by running `git show 036baae2 -- USAGE.md | diff - <(sed -n '531,537p' USAGE.md)` during verification.

## Open Questions

1. **Should D-04's Release Notes link sit above or below the horizontal rule at README.md:341?**
   - What we know: Line 341 is `---`, line 343 is the `<sub>` Python-Serena attribution.
   - What's unclear: Whether "footer/docs-index" per D-04 includes the attribution line.
   - Recommendation: Place ABOVE line 341 (as a new paragraph after `## Acknowledgements` content ends) so it reads as a documentation pointer, not a sub-footer aside. Leave `<sub>` attribution as the literal final line.

2. **Should the D-05 pointer replace line 5 or live as a new line 6?**
   - What we know: Line 5 already points to README.md for feature overview.
   - Recommendation: Keep line 5 as-is, add a new sentence at line 6 or turn line 5 into a 2-sentence paragraph. Either is fine; planner's discretion.

## Sources

### Primary (HIGH confidence)
- Direct `Read` of `/Users/Janis_Vizulis/go/src/github.com/postfix/serena/README.md` (lines 85–145, 315–343)
- Direct `Read` of `/Users/Janis_Vizulis/go/src/github.com/postfix/serena/USAGE.md` (lines 1–60, 525–545)
- Direct `Read` of `/Users/Janis_Vizulis/go/src/github.com/postfix/serena/INSTALL.md` (lines 50–65, heading grep)
- Direct `Read` of `/Users/Janis_Vizulis/go/src/github.com/postfix/serena/CHANGELOG.md` (heading grep)
- Direct `Read` of `.planning/phases/45-polish-crosslinks/45-CONTEXT.md` (locked decisions)

### Secondary (MEDIUM confidence)
- GFM heading-anchor convention — lowercase, spaces → hyphens — well-documented behavior in GitHub's markdown renderer.

### Tertiary (LOW confidence)
- None — no WebSearch needed for this scope.

## Metadata

**Confidence breakdown:**
- Current file state: HIGH — directly read.
- Anchor correctness: HIGH — heading verified, GFM slug rules well-known.
- Verification recipes: HIGH — patterns match live file content.
- F-06 text-review outcome: HIGH — text matches the structure and version reference CONTEXT describes.

**Research date:** 2026-04-24
**Valid until:** ~7 days (docs files change frequently around active development; planner should re-verify line numbers at plan-execute time if > 3 days elapse)

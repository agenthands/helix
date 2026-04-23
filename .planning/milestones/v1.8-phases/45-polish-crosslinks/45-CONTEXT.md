# Phase 45: Cross-link & Manual-Config Polish - Context

**Gathered:** 2026-04-24
**Status:** Ready for planning

<domain>
## Phase Boundary

Close the three non-blocking v1.8 integration-check findings deferred to Phase 45:

- **F-02** (warning): README manual-config pointer to INSTALL.md is vague — clients-not-in-README must be named explicitly.
- **F-06** (warning): USAGE rust-analyzer rename troubleshooting entry (USAGE.md:531–537) must still reflect current behavior.
- **F-12** (info): Two missing cross-links — README → CHANGELOG, USAGE → INSTALL.

Deliverables are narrow, text-only doc edits. No architecture, no new features, no scope expansion. Out of scope for this phase: adding more README client JSON blocks, adding CONTRIBUTING back-links, spinning up a live Rust workspace to re-test rename.

</domain>

<decisions>
## Implementation Decisions

### F-02 — README Manual-Config Pointer
- **D-01:** Keep the existing 3 JSON examples (Claude Code, Codex, generic IDE Assistant) + HTTP mode in README's manual-config `<details>` block unchanged. Do NOT add more JSON examples.
- **D-02:** Replace the existing vague line `README.md:134` (`For detailed configuration options, see [INSTALL.md](INSTALL.md).`) with a single explicit sentence that names the other clients: "For Cursor, Antigravity, VS Code, JetBrains, Claude Desktop, Gemini CLI, and OpenCode — see [INSTALL.md#manual-configuration](INSTALL.md#manual-configuration)."
- **D-03:** Keep it one sentence. No bullet list, no table.

### F-12 — Cross-Links
- **D-04:** Add README → CHANGELOG link. Place it at the end of README (near existing footer/docs-index area, where CONTRIBUTING and INSTALL are already referenced). Label: "Release Notes".
- **D-05:** Add USAGE → INSTALL link. Place it in USAGE's early Prerequisites / Setup section (natural reading order — "Install Serena first, see INSTALL.md").
- **D-06:** Do NOT add CONTRIBUTING → USAGE/README/INSTALL back-links. That's additive polish beyond F-12's stated recommendation; defer.

### F-06 — rust-analyzer Currency
- **D-07:** Text review only. Read USAGE.md:531–537, confirm the v1.90 version reference and the Symptom/Cause/Workaround structure still accurately describe the known rust-analyzer rename limitation. No live re-test.
- **D-08:** If text is already accurate: no edit needed — criterion #3 is satisfied by reading. Record the check in VERIFICATION.md.

### Verification Approach
- **D-09:** Targeted grep + link checks at end of phase. No full v1.8 integration-check re-run for this phase. Verification checklist:
  - `grep "Cursor.*Antigravity.*VS Code.*JetBrains.*Claude Desktop.*Gemini CLI.*OpenCode" README.md` succeeds on one line with an `INSTALL.md` link.
  - `grep -i "release notes\|CHANGELOG" README.md` finds a link to `CHANGELOG.md`.
  - `grep "INSTALL.md" USAGE.md` finds a link in USAGE's setup/prereqs area.
  - `diff` confirms USAGE.md:531–537 text unchanged (or if updated, still matches rust-analyzer v1.90 behavior).

### Claude's Discretion
- Exact sentence wording for D-02, D-04, D-05 — Claude picks natural phrasing that fits existing README/USAGE tone, preserving the decisions above.
- Section anchor targets in INSTALL.md (e.g., `#manual-configuration` vs `#manual-config`) — Claude verifies the actual anchor against INSTALL.md.
- CHANGELOG link: whether to link to `CHANGELOG.md` root or to the latest release heading.

### Folded Todos
_None — no pending todos matched this phase._

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### v1.8 Audit (source of truth for what must close)
- `.planning/v1.8-INTEGRATION-CHECK.md` — F-02, F-06, F-12 finding blocks with Affected files, Evidence, and Recommendation. This is the authoritative statement of what Phase 45 must close. See `deferred_findings:` frontmatter for current status.
- `.planning/v1.8-MILESTONE-AUDIT.md` — broader v1.8 audit context; cross-references F-02/F-06/F-12.

### Roadmap & Requirements
- `.planning/ROADMAP.md` §"Phase 45" — success criteria (3 items), goal statement, REQ mapping (README-03, USAGE-03, INST-02).
- `.planning/REQUIREMENTS.md` — REQ-IDs README-03, USAGE-03, INST-02 definitions.

### Files being edited
- `README.md` §"Manual configuration" (lines 88–135 at check time) — target for D-01, D-02, D-03.
- `README.md` footer area — target for D-04 (Release Notes link).
- `USAGE.md` Prerequisites/Setup section — target for D-05 (INSTALL link).
- `USAGE.md` lines 531–537 — target for D-07, D-08 (rust-analyzer entry, likely read-only).
- `INSTALL.md` §"Manual Configuration" (lines 55–216 at check time) — link target; verify anchor name.
- `CHANGELOG.md` — link target for D-04.

### Prior phase context (relevant decisions)
- `.planning/phases/44-reverify-p41-usage02/44-CONTEXT.md:106` — established that "README manual-config pointer polish" is explicitly Phase 45 scope (referenced by F-02/F-12 deferral rationale).
- `.planning/phases/39-readme-rewrite/` — the current README structure, don't undo choices made there.
- `.planning/phases/40-usage-refresh/40-VERIFICATION.md` — F-06 history; confirms USAGE.md:531–537 entry was added by Phase 40-03 (commit `036baae2`).

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- Existing README footer-area links to INSTALL, USAGE, CONTRIBUTING — Release Notes link follows the same Markdown pattern.
- USAGE's existing "Getting Started" / Prerequisites area (early in file) is the natural home for an INSTALL.md link — follows the pattern of Tutorial 1 already referencing setup flow.

### Established Patterns
- Markdown link style across docs: `[Link Text](path.md)` or `[Link Text](path.md#anchor)`. No reference-style links.
- Pointer sentences across INSTALL/USAGE follow an imperative form: "For X, see Y.md" — D-02's phrasing matches this.
- Rust-analyzer troubleshooting block uses **Symptom / Cause / Workaround** triple — any edit MUST preserve this structure (verified in USAGE.md:531–537).

### Integration Points
- None beyond the four files (README.md, USAGE.md, INSTALL.md, CHANGELOG.md). No code changes. No CI changes. No LSP/kernel/tool changes.

</code_context>

<specifics>
## Specific Ideas

- **D-02 draft wording** (Claude may refine): `For Cursor, Antigravity, VS Code, JetBrains, Claude Desktop, Gemini CLI, and OpenCode — see [INSTALL.md#manual-configuration](INSTALL.md#manual-configuration) for full examples.`
- **D-04 draft wording**: A "Release Notes" line in the README footer: `See [CHANGELOG.md](CHANGELOG.md) for release notes.` — Claude picks the exact placement consistent with surrounding footer content.
- **D-05 draft wording**: An early USAGE sentence: `If you haven't installed Serena yet, see [INSTALL.md](INSTALL.md) first.` — placed in Prerequisites/Setup area.

</specifics>

<deferred>
## Deferred Ideas

- **CONTRIBUTING → USAGE/README/INSTALL back-links** (F-12 "optional" part) — deferred. Not required by F-12's stated recommendation or Phase 45 success criteria. Can be picked up in a future docs polish pass if the asymmetry proves annoying.
- **Expanding README manual-config to 7 or 9 clients** — rejected in favor of keeping README light and letting INSTALL.md own the full matrix.
- **Full v1.8 integration-check re-run** after edits — not scheduled here; if needed, Phase 46 or a milestone-audit step can re-verify F-02/F-12 flip from deferred to resolved.
- **Live rust-analyzer rename re-test** — overkill for polish scope. If rust-analyzer v1.90 assumption becomes stale, a future phase can re-test and update USAGE.md:531–537.

### Reviewed Todos (not folded)
_No pending todos matched this phase — nothing to defer._

</deferred>

---

*Phase: 45-polish-crosslinks*
*Context gathered: 2026-04-24*

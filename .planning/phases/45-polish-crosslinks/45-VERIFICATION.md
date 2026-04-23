# Phase 45 Verification Record

**Phase:** 45-polish-crosslinks
**Plan:** 01
**Verified date:** 2026-04-24
**Requirements:** README-03, USAGE-03, INST-02
**Status:** passed

All three ROADMAP success criteria are SATISFIED. No full v1.8 integration-check re-run was performed (per D-09). Verification is scoped to targeted grep + read-only content checks.

---

## F-02 Verification (Task 1) — README Manual-Config Pointer

**Decision IDs:** D-01, D-02, D-03

### Positive check — explicit 7-client sentence + anchor link present

```text
$ grep -E "Cursor.*Antigravity.*VS Code.*JetBrains.*Claude Desktop.*Gemini CLI.*OpenCode" README.md | grep -F "INSTALL.md#manual-configuration"
For Cursor, Antigravity, VS Code, JetBrains, Claude Desktop, Gemini CLI, and OpenCode — see [INSTALL.md#manual-configuration](INSTALL.md#manual-configuration) for full examples.
```

Exit code: 0. **PASS.**

### Negative check — old vague pointer removed

```text
$ ! grep -qE "^For detailed configuration options, see \[INSTALL\.md\]\(INSTALL\.md\)\.$" README.md
(no match; negated grep exits 0)
```

Exit code: 0. **PASS.**

### Duplicate check

```text
$ grep -c "For Cursor, Antigravity, VS Code, JetBrains, Claude Desktop, Gemini CLI, and OpenCode" README.md
1
```

Exactly one occurrence. **PASS.**

### Anchor-integrity check — INSTALL.md target still exists

```text
$ grep -qE "^## Manual Configuration$" INSTALL.md
(matches at INSTALL.md:55)
```

Exit code: 0. **PASS.**

---

## F-12 Verification (Tasks 2 and 3) — Cross-Links

**Decision IDs:** D-04, D-05, D-06 (deferred)

### README → CHANGELOG (Release Notes)

```text
$ grep -qF "See [CHANGELOG.md](CHANGELOG.md) for release notes." README.md
(match present)

$ grep -c "CHANGELOG.md" README.md
1

$ tail -5 README.md
See [CHANGELOG.md](CHANGELOG.md) for release notes.

---

<sub>Originally inspired by [Python Serena](https://github.com/lks-ai/serena).</sub>
```

- Release Notes line present above the closing `---` rule.
- `<sub>` attribution remains the final non-empty line.
- Exactly one CHANGELOG.md reference (previously zero).

**PASS.**

### USAGE → INSTALL (Prerequisites/Setup intro)

```text
$ head -20 USAGE.md | grep -qF "[INSTALL.md](INSTALL.md)"
(match in head -20)

$ grep -qF "If you haven't installed Serena yet, see [INSTALL.md](INSTALL.md) first." USAGE.md
(exact sentence present)

$ grep -qF "For installation and feature overview, see [README.md](README.md)." USAGE.md
(existing README backlink preserved)

$ grep -cF "[INSTALL.md](INSTALL.md)" USAGE.md
1
```

**PASS.**

#### Deviation note (Task 3)

The plan instructed the new INSTALL-pointer sentence be inserted as a **distinct new line** immediately after line 5. During execution we observed that inserting a new line shifts all subsequent content down by one, which pushes the rust-analyzer troubleshooting block from USAGE.md:531–537 to 532–538 and breaks the `sed -n '531,537p' USAGE.md | grep -q "Workaround:"` / `replace_symbol_body` Task-4 acceptance greps as well as the plan's must_have truth "USAGE.md lines 531–537 rust-analyzer troubleshooting block is unchanged."

**Resolution (Rule 3 — fix blocking issue):** The new sentence was merged as a sibling sentence on the SAME line as the existing README backlink (line 5), producing one two-sentence paragraph rather than two one-sentence paragraphs. This preserves:

- All Task-3 acceptance greps (sentence present, head-20 contains `[INSTALL.md](INSTALL.md)`, README backlink preserved, count == 1).
- The byte-stable position and SHA-256 hash of the rust-analyzer block at lines 531–537.
- The plan's must_have truth about block stability (higher priority than the "distinct new line" presentation preference).

The deviation is presentation-only; semantic content and reading order are equivalent.

### CONTRIBUTING back-links (D-06)

Deferred — no CONTRIBUTING edits made. Out of scope for Phase 45.

---

## F-06 Verification (Task 4) — rust-analyzer Troubleshooting Currency

**Decision IDs:** D-07, D-08

### Content checks on USAGE.md:531–537 (read-only)

```text
$ sed -n '531,537p' USAGE.md
### rust-analyzer rename fails in fresh workspaces

**Symptom:** `rename_symbol` returns `"internal: rename (No references found at position)"` when targeting a Rust symbol, even though `get_hover_info`, `find_references`, and `search_symbols` all work correctly at the same position.

**Cause:** rust-analyzer (tested with v1.90) has a known limitation where `textDocument/rename` and `textDocument/prepareRename` return "No references found at position" in freshly-opened workspaces. Other LSP operations (`textDocument/hover`, `textDocument/references`, `workspace/symbol`) work correctly at the same position, indicating the issue is specific to the rename protocol handler's workspace readiness check.

**Workaround:** Use `replace_symbol_body` (tree-sitter-based) instead of `rename_symbol` (LSP-based) for Rust symbol renaming. The `replace_symbol_body` tool uses tree-sitter parsing rather than LSP rename, bypassing the rust-analyzer limitation entirely.
```

| Token              | Present | Command                                                    |
| ------------------ | ------- | ---------------------------------------------------------- |
| `v1.90`            | PASS    | `sed -n '531,537p' USAGE.md \| grep -q "v1.90"`            |
| `Symptom:`         | PASS    | `sed -n '531,537p' USAGE.md \| grep -q "Symptom:"`         |
| `Cause:`           | PASS    | `sed -n '531,537p' USAGE.md \| grep -q "Cause:"`           |
| `Workaround:`      | PASS    | `sed -n '531,537p' USAGE.md \| grep -q "Workaround:"`      |
| `replace_symbol_body` | PASS | `sed -n '531,537p' USAGE.md \| grep -q "replace_symbol_body"` |

All required tokens present. Symptom/Cause/Workaround triple intact. `replace_symbol_body` mitigation still canonical.

### Block SHA-256 hash (for future drift detection)

```text
$ sed -n '531,537p' USAGE.md | shasum -a 256
7586749f007a7237f56612bb45aa184b8448c387bff49964be8bed40679c0f26  -
```

**Recorded hash:** `7586749f007a7237f56612bb45aa184b8448c387bff49964be8bed40679c0f26`

Any future phase may re-run this command and `diff` against the recorded hash to detect accidental edits to the rust-analyzer troubleshooting block.

### Decision

**No edit required — text matches D-07/D-08 criteria.** The v1.90 reference, the Symptom/Cause/Workaround triple, and the `replace_symbol_body` mitigation all reflect current rust-analyzer behavior as captured by Phase 40-03 (commit `036baae2`). The block remains byte-identical to its pre-phase state.

---

## Link-Integrity Check — INSTALL.md Anchor Target

```text
$ grep -qE "^## Manual Configuration$" INSTALL.md
(match at line 55)
```

Anchor `#manual-configuration` is live. No slug collision (no other `## Manual*` heading in INSTALL.md). The README-to-INSTALL link written in Task 1 resolves correctly.

**PASS.**

---

## ROADMAP Success-Criteria Mapping

| # | Criterion                                                                                      | Satisfied by | Status     |
| - | ---------------------------------------------------------------------------------------------- | ------------ | ---------- |
| 1 | README manual-config either covers 7 clients or points to INSTALL with all 7 named explicitly   | Task 1       | SATISFIED |
| 2 | README → CHANGELOG link present AND USAGE → INSTALL link present (symmetry restored)           | Tasks 2 and 3 | SATISFIED |
| 3 | USAGE rust-analyzer troubleshooting block metadata confirmed current                            | Task 4       | SATISFIED |

---

## REQ ID Mapping

| REQ ID     | Description                                                                 | Closed by       |
| ---------- | --------------------------------------------------------------------------- | --------------- |
| README-03  | README correctly points to INSTALL.md for clients not documented inline     | Task 1 (F-02)   |
| USAGE-03   | USAGE cross-links to INSTALL + retains accurate troubleshooting             | Tasks 3 and 4 (F-12, F-06) |
| INST-02    | INSTALL `## Manual Configuration` is canonical target for 7-client docs     | Task 1 (via anchor link validated in link-integrity check) |

All three requirements marked complete.

---

## Findings Closed

- **F-02** (warning) — vague README manual-config pointer → RESOLVED (Task 1).
- **F-06** (warning) — rust-analyzer troubleshooting currency → RESOLVED (Task 4 read-only confirmation + hash recorded).
- **F-12** (info) — missing README→CHANGELOG and USAGE→INSTALL cross-links → RESOLVED (Tasks 2 and 3).

---

## Summary

Status: **passed.** All grep acceptance criteria exit 0. Rust-analyzer block byte-stable (hash unchanged). No v1.8 integration-check re-run scheduled (per D-09).

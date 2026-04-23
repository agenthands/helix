# Phase 41: Install & Contributing - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-23
**Phase:** 41-install-contributing
**Areas discussed:** INSTALL restructuring, Client list accuracy, CONTRIBUTING test harness, CONTRIBUTING features
**Mode:** --auto (all decisions auto-selected)

---

## INSTALL Restructuring

| Option | Description | Selected |
|--------|-------------|----------|
| Setup CLI primary | Lead with `serena setup <client>`, manual configs secondary | ✓ |
| Keep current structure | Manual JSON configs remain primary, add setup CLI note | |
| Dual equal paths | Both methods presented equally | |

**User's choice:** [auto] Setup CLI as primary method (aligns with Phase 39 D-11)
**Notes:** Carries forward the product direction established in Phase 39 README rewrite

---

## Client List Accuracy

| Option | Description | Selected |
|--------|-------------|----------|
| Match setup CLI registry | Use actual clientRegistry() as source of truth (6 clients) | ✓ |
| Keep all current agents | Preserve all 7 current manual config sections | |
| Superset | Include both setup CLI clients and additional manual-only configs | |

**User's choice:** [auto] Match setup CLI registry (claude-code, vscode, jetbrains, claude-desktop, gemini-cli, generic)
**Notes:** Codex/OpenCode/Cursor/Antigravity use generic profile — no need for dedicated sections

---

## CONTRIBUTING Test Harness

| Option | Description | Selected |
|--------|-------------|----------|
| Full oracle hierarchy | Document all 6 oracle test layers + harness package | ✓ |
| Minimal update | Mention oracle tests exist, keep focus on integration tests | |

**User's choice:** [auto] Full oracle hierarchy documentation (recommended for contributor onboarding)
**Notes:** Oracle tests represent the primary test strategy since v1.4

---

## CONTRIBUTING Features

| Option | Description | Selected |
|--------|-------------|----------|
| Add missing packages | Update structure listing with v1.5-v1.7 additions | ✓ |
| Minimal changes | Only update tool counts, keep structure as-is | |

**User's choice:** [auto] Add missing packages to structure listing
**Notes:** RepoMap, fuzzy, CLI, smart errors, progressive descriptions all missing from current structure

---

## Claude's Discretion

- Exact wording of setup CLI examples
- Manual config section presentation (collapsed vs flat)
- Oracle test layer description depth
- Whether to add installation troubleshooting subsection

## Deferred Ideas

None

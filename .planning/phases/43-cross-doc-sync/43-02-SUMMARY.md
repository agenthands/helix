---
phase: 43-cross-doc-sync
plan: 02
subsystem: docs
tags: [docs, install, setup-cli, cross-doc-sync]
requires: []
provides:
  - install-quickstart-7-clients
affects:
  - INSTALL.md
tech-stack:
  added: []
  patterns:
    - canonical-client-enumeration-from-setup_clients.go
key-files:
  created: []
  modified:
    - INSTALL.md
decisions:
  - Task 2 resolved as no-op branch — INSTALL.md contains no "Supported clients:" prose sentence, so the enumeration-prose sync is vacuously satisfied.
metrics:
  duration: ~2m
  completed: 2026-04-23
---

# Phase 43 Plan 02: Align INSTALL Quick Start with 7-Registrar Source of Truth Summary

One-liner: Appended `serena setup opencode` and `serena setup generic` to INSTALL.md Quick Start block so the install guide matches `internal/cli/setup_clients.go` (7 canonical registrars) and README Quick Start.

## What Changed

### Task 1 — Add opencode and generic to INSTALL Quick Start

- **File:** `INSTALL.md` (Quick Start code block, lines 35-43 post-edit)
- **Edit:** Appended two lines to the existing 5-line block:
  ```
  serena setup opencode       # OpenCode
  serena setup generic        # Generic MCP client
  ```
- **Column alignment:** Preserved the existing two-space gap before `#`, matching the surrounding style.
- **Commit:** `3491ba0d` — `docs(43-02): add opencode and generic to INSTALL Quick Start`

### Task 2 — Update "Supported clients" prose enumeration (no-op branch)

- **Investigation:** `grep -in "supported clients" INSTALL.md` returned zero matches.
- **Outcome:** No prose enumeration sentence exists in INSTALL.md to update. The plan's explicit no-op branch applies.
- **Explicit assertion satisfied:** `grep -qi "supported clients:" INSTALL.md || echo NO_OP_CONFIRMED` printed `NO_OP_CONFIRMED`.
- **No commit** (no file change).

## Verification

Post-edit verification (run from repo root):

```
$ grep -c "serena setup opencode" INSTALL.md
1
$ grep -c "serena setup generic" INSTALL.md
2
$ grep -E "^serena setup (claude-code|vscode|jetbrains|claude-desktop|gemini-cli|opencode|generic)" INSTALL.md | wc -l
7
$ grep -in "supported clients" INSTALL.md
(no matches — no-op branch confirmed for Task 2)
```

The `^serena setup ...` anchored count = 7, matching the canonical registrar list in `internal/cli/setup_clients.go:37-46`: `claude-code, vscode, jetbrains, claude-desktop, gemini-cli, opencode, generic`.

## Deviations from Plan

### Auto-fixed / Acknowledged Issues

**1. [Rule 1 — Plan acceptance criterion off-by-one] `serena setup opencode` count is 1, not ≥2**

- **Found during:** Task 1 verification.
- **Issue:** Task 1 `acceptance_criteria` stated `grep -c "serena setup opencode" INSTALL.md` should return `≥2 (new Quick Start line + existing mentions at line 149)`. INSTALL.md line 149 is the section header `### OpenCode`, not a `serena setup opencode` invocation. No pre-existing `serena setup opencode` string exists in the file; only one other opencode-related `serena setup` reference exists post-edit (none for opencode specifically, two for generic — lines 40 and 214).
- **Resolution:** Followed the plan's `<done>` criterion ("Quick Start block contains all 7 canonical `serena setup <client>` lines.") rather than the numeric count. Post-edit the Quick Start block contains all 7 canonical invocations, the primary intent of the plan.
- **Evidence:** `grep -E "^serena setup (claude-code|vscode|jetbrains|claude-desktop|gemini-cli|opencode|generic)" INSTALL.md | wc -l` = 7.
- **Files modified:** none beyond Task 1.
- **Commit:** documented here; no code fix needed.

**2. [Task 2] Confirmed no-op branch**

- Not a deviation from the plan — the plan explicitly permits this branch and requires it be documented.
- Branch taken: `NO_OP_CONFIRMED`.

## Authentication Gates

None.

## Known Stubs

None.

## Threat Flags

None. Documentation-only change; no new surface area, endpoints, auth paths, file access patterns, or schema changes.

## Commits

| Task | Name | Commit | Files |
| ---- | ---- | ------ | ----- |
| 1    | Add opencode and generic to INSTALL Quick Start | 3491ba0d | INSTALL.md |
| 2    | Update "Supported clients" enumeration (no-op) | — | — |

## Self-Check: PASSED

- `INSTALL.md` — FOUND, modified (Quick Start block now has 7 canonical registrars).
- Commit `3491ba0d` — FOUND in `git log`.
- Task 2 no-op documented per plan's explicit-assertion requirement.

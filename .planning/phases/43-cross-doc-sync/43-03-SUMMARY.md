---
phase: 43-cross-doc-sync
plan: 03
subsystem: docs
tags: [docs, usage, cross-doc-sync, fuzzy, setup]
requires: []
provides:
  - canonical-client-list-in-usage
  - canonical-fuzzy-strategy-names-in-usage
affects:
  - USAGE.md
tech-stack:
  added: []
  patterns: []
key-files:
  created: []
  modified:
    - USAGE.md
decisions:
  - Aligned USAGE.md Supported-clients enumeration with the canonical 7-client list (adds opencode between gemini-cli and generic).
  - Replaced legacy strategy vocabulary (Exact / Whitespace / IndentFlex / Failed) with canonical names matching strategies.go and CHANGELOG.md:43.
  - Clarified that "Failed" is NOT a strategy but the fallback unified-diff envelope when all four strategies miss; ellipsis-placeholder IS the real fourth strategy.
metrics:
  duration: ~3m
  tasks_completed: 2
  files_changed: 1
  completed: 2026-04-23
---

# Phase 43 Plan 03: USAGE.md Client List and Fuzzy Strategy Sync Summary

Aligned USAGE.md with canonical client enumeration (7 clients including opencode) and canonical fuzzy-strategy names (exact match, whitespace-normalized, indentation-flexible, ellipsis-placeholder), closing F-01 and F-08 for USAGE.md per v1.8 integration check.

## Tasks

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Add opencode to USAGE Supported clients list | fb53c7fb | USAGE.md |
| 2 | Rewrite USAGE fuzzy-strategy paragraph with canonical names | 067f7ec4 | USAGE.md |

## What Changed

### Task 1 — Supported clients (USAGE.md lines 25, 173)
Inserted `` `opencode` `` between `gemini-cli` and `generic` in both Supported-clients sentences. USAGE.md now matches the canonical enumeration in `internal/cli/setup_clients.go` (claude-code, vscode, jetbrains, claude-desktop, gemini-cli, opencode, generic).

### Task 2 — Fuzzy strategy vocabulary (USAGE.md line 141)
Rewrote the 4-strategy enumeration sentence. Before:

- **Exact** (byte-for-byte match)
- **Whitespace** (ignores leading/trailing whitespace per line)
- **IndentFlex** (tabs and spaces interchangeable)
- **Failed** (returns a unified-diff showing the nearest match)

After (matches `strategies.go` and CHANGELOG.md:43):

- **exact match** (byte-for-byte)
- **whitespace-normalized** (ignores leading/trailing whitespace per line)
- **indentation-flexible** (tabs and spaces interchangeable at line start)
- **ellipsis-placeholder** (allows `...` in the search block to skip intermediate content)

Also clarified the fallback behavior: if all four strategies miss, the tool returns a unified-diff envelope showing the nearest candidate match. "Failed" was removed as a strategy label because it was never an actual matching strategy — it was the all-miss fallback response.

## Verification

```bash
grep -c "opencode" USAGE.md               # 2 (was 0)
grep "IndentFlex" USAGE.md                # empty (was 1)
grep -c "indentation-flexible" USAGE.md   # 1 (was 0)
grep -c "ellipsis-placeholder" USAGE.md   # 1 (was 0)
grep -c "whitespace-normalized" USAGE.md  # 1 (was 0)
grep '\*\*Failed\*\*' USAGE.md            # empty (was 1)
```

All acceptance criteria pass.

## Deviations from Plan

None — plan executed exactly as written.

## Self-Check: PASSED

- FOUND: USAGE.md Supported-clients sentences contain `opencode` (2 occurrences)
- FOUND: USAGE.md contains `indentation-flexible`, `ellipsis-placeholder`, `whitespace-normalized`
- FOUND: `IndentFlex` and `**Failed**` removed from USAGE.md
- FOUND commit: fb53c7fb (Task 1)
- FOUND commit: 067f7ec4 (Task 2)

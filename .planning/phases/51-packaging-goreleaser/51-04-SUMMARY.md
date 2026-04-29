---
phase: 51-packaging-goreleaser
plan: 04
subsystem: packaging-docs
tags: [packaging, documentation, repo-identity, gap-closure, cr-01, wr-03, wr-04]
requires: [51-01, 51-02]
provides:
  - "README.md Quick Start install path no longer publishes the broken postfix/serena go-install URL"
  - "README.md HTTP-mode example matches INSTALL.md byte-for-byte"
  - "README.md upstream attribution is internally consistent (single canonical URL)"
affects:
  - "README.md (the only file modified by this plan)"
tech_stack:
  added: []
  patterns:
    - "doc-vs-binary-flag verification (grep cobra StringVar/Bool defs in internal/cli/root.go)"
    - "doc-vs-doc consistency (cross-grep README.md and INSTALL.md for canonical commands)"
    - "loopback-by-default in user-facing examples (127.0.0.1 over `:port`)"
key_files:
  created:
    - .planning/phases/51-packaging-goreleaser/51-04-SUMMARY.md
  modified:
    - README.md
decisions:
  - "Removed the postfix/serena `go install` line entirely rather than annotating it with a comment (CR-01: the comment was documentation theater)"
  - "Aligned README HTTP-mode example to INSTALL.md verbatim using `serena --mode=http --http-addr=127.0.0.1:8080`; kept `--serve` discoverable via an equivalence comment"
  - "Chose `oraios/serena` as the canonical upstream URL based on (a) the original README author's email being @oraios-ai.de, (b) `oraios/serena` resolves with HTTP 200 while `lks-ai/serena` returns 404, (c) `.planning/research/SUMMARY.md` independently references `oraios.github.io/serena`"
metrics:
  duration_seconds: 81
  duration_human: "~1 minute"
  completed_at: "2026-04-29T08:45:49Z"
  tasks_completed: 3
  files_modified: 1
  files_created: 1
  commits: 3
---

# Phase 51 Plan 04: README Identity & Flags Summary

One-liner: Closed VERIFICATION/REVIEW gaps CR-01, WR-03, and WR-04 in README.md by removing the broken `go install github.com/postfix/serena/cmd/serena@latest` primary install line, aligning the HTTP-mode example to INSTALL.md (`--mode=http --http-addr=127.0.0.1:8080`), and reconciling the two conflicting upstream URLs to a single canonical `github.com/oraios/serena`.

## What Changed

| # | Task | File | Commit |
|---|------|------|--------|
| 1 | Replace Quick Start install: pre-built binaries lead linking to INSTALL.md and the Releases page; build-from-source kept as secondary path with `agenthands/helix` repo | README.md | `749b3de3` |
| 2 | Align HTTP-mode example with INSTALL.md and binary defaults: `--mode=http --http-addr=127.0.0.1:8080`, `--serve` documented as equivalent in a comment | README.md | `e7123c6b` |
| 3 | Reconcile upstream attribution: footnote URL changed from `lks-ai/serena` (404) to `oraios/serena` (200), now matching the lead paragraph | README.md | `1784c0b7` |

## Upstream URL Decision (Task 3)

**Chosen URL:** `https://github.com/oraios/serena`

**Evidence trail:**

1. **Git log** (`git log --all --diff-filter=A -- README.md`): the initial README commit (`e93169bc`, 2025-03-23) was authored by `Michael Panchenko <michael.panchenko@oraios-ai.de>` — the original codebase came from oraios.
2. **HTTP resolution check**:
   - `curl -sI -L https://github.com/oraios/serena` → **200**
   - `curl -sI -L https://github.com/lks-ai/serena` → **404**
3. **Project planning history** (`grep -rn "oraios\|lks-ai" .planning/`):
   - `.planning/research/SUMMARY.md:150` independently references `oraios.github.io/serena` as the Python Serena reference
   - `lks-ai/serena` only appears in later README rewrites (phases 39 and 45) and was carried forward unchecked

**Rule applied:** "If exactly ONE URL resolves to a real repo, use that URL in BOTH lines 12 and 348." Plan 51-04 leaves line 12 as-is (already `oraios/serena`) and updates line 348 from `lks-ai/serena` to `oraios/serena`.

After the edit: `grep -c "oraios/serena" README.md` = 2, `grep -c "lks-ai/serena" README.md` = 0.

## Verification

| Check | Status |
|-------|--------|
| README.md does NOT contain `go install github.com/postfix/serena/cmd/serena@latest` | PASS |
| README.md links to `[INSTALL.md](INSTALL.md)` and `[Releases page](https://github.com/agenthands/helix/releases)` | PASS |
| README.md uses `git clone https://github.com/agenthands/helix.git` and `cd helix` | PASS |
| README.md HTTP-mode example uses `serena --mode=http --http-addr=127.0.0.1:8080` (matching INSTALL.md byte-for-byte) | PASS |
| README.md contains zero occurrences of port `:9091` | PASS |
| Attribution count: 2 occurrences of `oraios/serena`, 0 of `lks-ai/serena` | PASS |
| INSTALL.md was NOT modified by this plan (`git diff b451ac1b -- INSTALL.md` empty) | PASS |
| Only README.md changed (`git diff --name-only b451ac1b HEAD` = `README.md`) | PASS |
| `go vet ./...` exit code 0 (no Go source touched) | PASS |
| `go test ./...` baseline preserved (no Go source touched, not re-run) | N/A |

## Deviations from Plan

None — plan executed exactly as written. The decision branches in Task 3 (which URL to choose) followed the plan's documented decision rule.

## Threat Model Compliance

| Threat ID | Disposition | Mitigation Realized |
|-----------|-------------|---------------------|
| T-51-21 (broken install path) | mitigate | Task 1 removed the `go install postfix/serena` line entirely; INSTALL.md is now the canonical install path. |
| T-51-22 (doc/flag drift) | mitigate | Task 2 aligned the example to `--mode=http --http-addr=127.0.0.1:8080`, verified against `internal/cli/root.go:30,38` (cobra defs). |
| T-51-23 (non-loopback default) | mitigate | New example binds to `127.0.0.1:8080` (loopback only) instead of the previous `:9091` (all interfaces). |
| T-51-24 (internal attribution conflict) | mitigate | Task 3 reconciled both attribution lines to `oraios/serena` (canonical, evidenced). |

No new threat surface introduced.

## Cross-References

- **Closes** VERIFICATION gap: "README.md still publishes `go install github.com/postfix/serena/cmd/serena@latest` as the PRIMARY install path" (`51-VERIFICATION.md:122`)
- **Closes** REVIEW finding **CR-01**: README repo identity drift (`51-REVIEW.md`)
- **Closes** REVIEW finding **WR-03**: README/INSTALL HTTP-mode flag mismatch
- **Closes** REVIEW finding **WR-04**: two-URL upstream attribution conflict
- **Reverses** Plan 51-02's "Open Question 1" deferral (the explanatory in-fence comment was confirmed by the verifier as documentation theater)

## Note for Downstream

A future phase should rename the Go module path in `go.mod` from `github.com/postfix/serena` to `github.com/agenthands/helix` (separate decision per Plan 51-02 Open Question 1). Once that lands and the new module path is published to the Go proxy, the README.md Install subsection can re-add a `go install github.com/agenthands/helix/cmd/serena@latest` one-liner without the original safety concern.

## Self-Check: PASSED

- README.md exists with the expected changes (Quick Start, HTTP mode, footnote)
- All three commit hashes verified in `git log`:
  - `749b3de3` — Task 1 (Install subsection rewrite)
  - `e7123c6b` — Task 2 (HTTP-mode reconciliation)
  - `1784c0b7` — Task 3 (attribution reconciliation)
- INSTALL.md unchanged (verified via `git diff b451ac1b -- INSTALL.md`)
- Worktree base verified at start: `b451ac1b5d485708dae17c4dabda806fc1428fa4`

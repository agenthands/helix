---
phase: 43-cross-doc-sync
plan: 06
subsystem: cli-setup
tags: [docs, comment-fix, cross-doc-sync]
requires: []
provides:
  - "Accurate registrar count in setup_clients.go source comment"
affects:
  - internal/cli/setup_clients.go
tech_stack:
  added: []
  patterns: []
key_files:
  created: []
  modified:
    - internal/cli/setup_clients.go
decisions: []
metrics:
  duration_minutes: 1
  completed_date: 2026-04-23
  tasks_completed: 1
  files_modified: 1
requirements_completed:
  - INST-02
---

# Phase 43 Plan 06: Registrar Count Comment Fix Summary

One-line comment correction at `internal/cli/setup_clients.go:36` so that the doc-comment's registrar count matches the map cardinality below it (7 entries).

## Changes

- `internal/cli/setup_clients.go:36` — `// clientRegistry returns all 6 registrars keyed by client name.` → `// clientRegistry returns all 7 registrars keyed by client name.`

## Verification

- `grep "all 6 registrars" internal/cli/setup_clients.go` — empty
- `grep -c "all 7 registrars keyed by client name" internal/cli/setup_clients.go` — `1`
- `go build ./cmd/serena` — exit 0
- `go vet ./internal/cli/...` — exit 0 (pre-existing Swift tree-sitter binding warnings are unrelated)

## Deviations from Plan

None — plan executed exactly as written.

## Commits

- `8005d4b4` docs(43-06): fix registrar count comment in setup_clients.go

## Self-Check: PASSED

- FOUND: internal/cli/setup_clients.go (modified, line 36 updated)
- FOUND: commit 8005d4b4 in `git log`

# Phase 104 — Deferred Items

Out-of-scope discoveries logged during execution (not fixed by this phase).

## TestRunSubcommandWiresDeltaPass (cmd/helix-bench) — PRE-EXISTING FAILURE

- **Found during:** Task 3 full-suite `go test ./...`.
- **Failure:** `run_cmd_test.go:144` — `real mode ...: row ... missing ablation_deltas (delta pass not wired into runBench)`.
- **Pre-existing:** Confirmed identical failure on the pre-phase baseline commit `1e382f5c` (checked out via a throwaway worktree). The phase 104 commits touch only `cmd/helix-refgen/` and `internal/cli/skills/helix/reference.md` — nothing under `cmd/helix-bench`.
- **Scope:** Out of scope for REFGEN-01. The ablation-delta pass is not wired into `runBench`; unrelated to reference-generator per-verb prose.
- **Action:** None taken (deviation-rule scope boundary — only auto-fix issues directly caused by this phase's changes).

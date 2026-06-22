---
phase: 98-multi-agent-coverage-stronger-steering
plan: 02
subsystem: cli-setup
status: complete
tags: [agent-adoption, setup, instruction-files, codex, gemini, nudge-reuse]
requires:
  - internal/cli/skill.go (embeddedSkillFS, EmbeddedSkillBody)
  - internal/cli/setup_hooks.go (helixHookConfig, mergeHooksIntoSettings pattern)
  - internal/cli/setup_clients.go (ClientRegistrar, clientRegistry, teardownOnlyRegister)
  - internal/cli/nudge.go (runNudge — the ONE steering engine, reused by codex)
provides:
  - "EmbeddedReference() — surfaces reference.md bytes to non-Claude agents (AGENT-01)"
  - "writeAgentInstructions() — idempotent sentinel-delimited instruction-file writer, 32-KiB cap, atomic, path-contained (AGENT-02)"
  - "removeAgentInstructions() — strips the Helix block for uninstall"
  - "writeCodexHooks() — Codex PreToolUse hooks.json invoking `helix nudge` (AGENT-03)"
  - "CodexRegistrar — new `codex` setup target (AGENTS.md + hooks.json)"
  - "gemini-cli/generic flipped to also-write instruction files (no hook)"
affects:
  - internal/cli/setup.go (ValidArgs gains codex)
  - internal/cli/setup_clients.go (clientRegistry gains codex; Gemini/Generic Register flipped)
tech-stack:
  added: []
  patterns:
    - "Idempotent sentinel-delimited markdown append (filter-then-rewrite, mirrors mergeHooksIntoSettings)"
    - "Atomic temp+rename single-file write (mirrors saveSessionStats)"
    - "JSON via encoding/json Marshal, never string-concat (T-34-01)"
    - "filepath path-traversal containment guard (Security V12)"
    - "One steering engine, two runtimes — Codex hook reuses runNudge"
key-files:
  created:
    - internal/cli/setup_agents.go
    - internal/cli/setup_agents_test.go
  modified:
    - internal/cli/skill.go
    - internal/cli/setup_clients.go
    - internal/cli/setup.go
    - internal/cli/setup_test.go
decisions:
  - "Codex hooks.json command pinned as an array [<bin>,\"nudge\"] with type:\"command\", matcher \"Bash\", written to <project>/.codex/hooks.json (global: ~/.codex/hooks.json) per documented Codex hooks convention (STACK.md A1)."
  - "Codex AGENTS.md → <project>/AGENTS.md (global: ~/.codex/AGENTS.md), 32-KiB cap; Gemini GEMINI.md → <project>/GEMINI.md (global: ~/.gemini/GEMINI.md), no cap; generic → <project>/AGENTS.md (agents.md standard)."
  - "32-KiB cap trims the APPENDED block to a pointer (agentInstructionPointer), never user content; an impossible cap errors rather than truncating user bytes."
  - "Path containment refuses any destination containing a raw `/../` traversal segment (destinations derive from fixed conventions, never raw user input)."
  - "Scoped the flip to codex + gemini-cli + generic only (A5); vscode/jetbrains/opencode remain teardown-only."
metrics:
  duration: ~30m
  completed: 2026-06-22
  tasks_completed: 3
  files_created: 2
  files_modified: 4
  lines_changed: "+841 / -12 (internal/cli)"
---

# Phase 98 Plan 02: Multi-Agent Coverage (AGENT thrust) Summary

Surfaced the generated verb reference to non-Claude agents and flipped four
non-Claude setup clients from teardown-only to also-writing per-agent instruction
files — including a brand-new `codex` client that reuses the existing `helix
nudge` steering engine via a Codex `PreToolUse` hooks.json — using an idempotent
sentinel-delimited, atomic, path-contained, 32-KiB-capped writer that never
clobbers a user's existing `AGENTS.md`/`GEMINI.md`/generic file.

## What Was Built

### AGENT-01 — `EmbeddedReference()` (Task 1)
`internal/cli/skill.go` gains `EmbeddedReference()` (and unexported
`embeddedReferenceBytes()`) reading `skills/helix/reference.md` from the same
`//go:embed skills/helix/*` FS that already ships the Claude bundle. It mirrors
`EmbeddedSkillBody()`/`embeddedSkillBytes()` exactly (panic on a missing embed —
a build-time error). One generated source, the same `helix-refgen --check`-gated
bytes, now surfaceable to non-Claude agents — no per-agent bespoke engine, no
drift.

### AGENT-02 — idempotent sentinel-delimited instruction-file writer (Task 1)
`internal/cli/setup_agents.go` (NEW):
- `helixBlockBegin`/`helixBlockEnd` — HTML-comment sentinels (invisible in
  rendered markdown).
- `agentInstructionBody()` — the terse "use X not Y" matrix (sourced from
  SKILL.md's table) + a pointer to `helix get-tool-help` / the installed
  reference. The full 32-KB reference is NEVER inlined (that would blow the cap).
- `agentInstructionPointer()` — the minimal fallback block used when the full
  body would exceed the cap.
- `writeAgentInstructions(path, body, maxBytes)` — read → strip any prior Helix
  block (filter-then-rewrite, mirroring `mergeHooksIntoSettings`) → append a fresh
  sentinel block → if over `maxBytes` trim the APPENDED block to a pointer (never
  user content; error rather than truncate user bytes if even the pointer cannot
  fit) → atomic temp+rename write. Path-contained against `/../` traversal.
- `stripHelixBlock`/`ensureTrailingNewline`/`removeAgentInstructions` helpers.

### AGENT-03 — Codex client + nudge reuse + flips (Task 3)
- `writeCodexHooks(path, binaryPath)` in `setup_agents.go` — builds the Codex
  PreToolUse hooks.json via `map[string]any` + `encoding/json` (never
  string-concat). The command is `[<binaryPath>, "nudge"]` — the SAME `runNudge`
  engine Claude uses (one engine, two runtimes). Advisory-only: emits the shared
  `additionalContext` envelope at exit 0, NEVER `permissionDecision:"deny"`.
  Atomic, path-contained, byte-stable on re-run.
- `CodexRegistrar` in `setup_clients.go` — implements `ClientRegistrar`; `Register`
  writes `AGENTS.md` (≤32 KiB) + `.codex/hooks.json`; `Unregister` strips the block
  + removes the Helix-managed hooks.json; `teardownMCP` is a documented best-effort
  no-op (codex was never an MCP-registered client). Honors DryRun.
- `GeminiCLIRegistrar.Register` / `GenericRegistrar.Register` flipped: keep the
  MCP teardown, then ALSO write `GEMINI.md` / `AGENTS.md` with the Helix block —
  NO hook (Gemini has no PreToolUse-equivalent; fabricating one is the locked
  anti-feature). DryRun honored.
- `clientRegistry()` gains `"codex"`; `setup.go` `ValidArgs` gains `"codex"`.
- `vscode`/`jetbrains`/`opencode` remain teardown-only (Assumption A5).

## How It Was Verified

| Gate | Result |
|------|--------|
| `go test ./internal/cli/...` | PASS (green) |
| `go vet ./...` | clean |
| `go build ./cmd/helix` | builds |
| `go test ./...` (merge gate) | PASS (rc=0) |
| `go run ./cmd/helix-refgen --check` | clean (reference.md untouched) |
| `git diff go.mod` | empty (zero-dep invariant holds) |
| `gofmt -l` on touched files | clean |

Anti-vacuity teeth shipped and green:
- `TestWriteAgentInstructions/idempotent-rerun-no-duplicate` — re-run produces
  EXACTLY ONE block; a duplicated block fails.
- `TestWriteAgentInstructions/32-KiB-cap-trims-block-not-user-content` — every
  user line preserved verbatim under cap; only the appended block trimmed.
- `TestWriteAgentInstructions/cap-too-small-errors-rather-than-truncating-user` —
  an impossible cap errors and leaves the user file untouched.
- `TestWriteAgentInstructions/path-containment-refuses-escape` — a raw `/../`
  destination is refused before any write.
- `TestGeminiNoHookArtifact` — GEMINI.md written, NO hook artifact anywhere.
- `TestCodexRegistrar` / `TestWriteCodexHooks` — hooks.json invokes `<bin> nudge`,
  no `permissionDecision`/deny default.
- `TestCodexInValidArgsAndRegistry` — codex resolvable; vscode/jetbrains/opencode
  write no instruction file (stay teardown-only).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 — Test breakage] Pre-existing generic-registrar tests leaked an `AGENTS.md` into the package dir**
- **Found during:** Task 3 (after flipping `GenericRegistrar.Register` to write an instruction file).
- **Issue:** `TestGenericRegistrarRegister` / `TestGenericRegistrarStdout` set no `ProjectDir`, so the new flip wrote `AGENTS.md` into the working directory (the package dir).
- **Fix:** Set `ProjectDir` to a `t.TempDir()` in both tests so the instruction-file write is contained; removed the leaked file.
- **Files modified:** `internal/cli/setup_test.go`.
- **Commit:** 5ddd38be.

**2. [Rule 1 — Exact-membership test] `TestClientRegistryContainsAll` asserted 7 clients**
- **Found during:** Task 3 (after adding `codex` to `clientRegistry()`).
- **Issue:** The test asserts the exact registry membership and length; adding `codex` made it 8.
- **Fix:** Added `"codex"` to the expected list.
- **Files modified:** `internal/cli/setup_test.go`.
- **Commit:** 5ddd38be.

## Checkpoint Resolution (Task 2 — blocking human-verify)

The blocking human-verify checkpoint (Codex/Gemini conventions, research flags
A1/A2) was PRE-AUTHORIZED by the orchestrator (autonomous mode) and resolved by
pinning the implementation + golden bytes against the OFFICIAL conventions
documented at HIGH confidence in `.planning/research/STACK.md`
(developers.openai.com/codex AGENTS.md + hooks; geminicli.com GEMINI.md):

- Codex `AGENTS.md` (≤32 KiB) at `<project>/AGENTS.md` (global `~/.codex/AGENTS.md`).
- Codex `PreToolUse` `hooks.json` at `<project>/.codex/hooks.json` (global
  `~/.codex/hooks.json`), shape
  `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":["<bin>","nudge"]}]}]}}`,
  advisory `additionalContext` only — no deny default.
- Gemini `GEMINI.md` (instruction-file only, NO hook) at `<project>/GEMINI.md`
  (global `~/.gemini/GEMINI.md`).

The tests assert only the parts Helix controls (block present, command invokes
`nudge`, no deny default, no Gemini hook).

## Known Stubs

None. No placeholder/empty-data stubs introduced; the instruction-file body is a
real terse matrix sourced from SKILL.md's table.

## human_needed UAT Items

> The following requires a live external runtime and could NOT be self-verified
> against an automated test this session. It is recorded as a clean human-verify
> item per the plan's autonomous routing — NOT asserted as automated-passed.

- **Codex/Gemini live round-trip (A1/A2 end-to-end):** Install into a real Codex
  config dir (`helix setup codex`) and a real Gemini config dir (`helix setup
  gemini-cli`), then confirm: (a) Codex actually loads `AGENTS.md` and fires the
  `.codex/hooks.json` PreToolUse hook, invoking `helix nudge` and surfacing the
  `additionalContext` advisory; (b) Gemini actually loads `GEMINI.md` into context.
  The exact hooks.json byte-shape, key names, and discovery path are pinned from
  documented conventions (STACK.md A1/A2, HIGH confidence) but were not exercised
  against a live Codex/Gemini install. If the live runtime rejects the pinned
  shape, adjust `writeCodexHooks` / the path helpers in `setup_agents.go` and
  re-pin the golden in `TestWriteCodexHooks` / `TestCodexRegistrar`.

## Self-Check: PASSED

- `internal/cli/setup_agents.go` — FOUND
- `internal/cli/setup_agents_test.go` — FOUND
- `internal/cli/skill.go` (EmbeddedReference) — FOUND
- Task 1 commit e87adcdc — FOUND
- Task 3 commit 5ddd38be — FOUND

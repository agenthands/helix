# Phase 96: Address v2.0 tech debt - Context

**Gathered:** 2026-06-22
**Status:** Ready for planning
**Mode:** Auto-generated (discuss skipped via workflow.skip_discuss), enriched from the v2.0 milestone audit

<domain>
## Phase Boundary

Resolve the non-blocking tech debt the v2.0 milestone audit (`.planning/v2.0-MILESTONE-AUDIT.md`) surfaced so the milestone archives with zero open items. The milestone Definition of Done is already MET (31/31 requirements, 6/6 phases passed, integration clean, 4/4 E2E flows wired) — this phase clears accumulated non-blocking items, not blockers.

Four small code/doc fixes plus a Nyquist coverage close. No behavior change to the frozen CLI/verb surface. The frozen 50-verb surface, the docgen drift gate, and the retained gRPC `StreamMCP` internal wire must stay intact.

**In scope (user-confirmed): code/doc debt + Nyquist.**
**Out of scope:** the pre-existing env-gated `cmd/helix-bench` `TestRunSubcommandWires*` failures (carried from v1.12, NOT a v2.0 regression; need a full bench agent/dataset env).
</domain>

<decisions>
## Implementation Decisions

### Claude's Discretion
Implementation choices are at Claude's discretion, guided by the audit findings, the existing patterns each fix mirrors, and codebase conventions. Each fix is small and self-contained; prefer the minimal change that closes the item plus a regression test.

### Confirmed scope (user)
All four actionable code/doc items AND retroactive Nyquist validation on phases 93/94/95. Pre-existing bench failures explicitly excluded.
</decisions>

<code_context>
## Existing Code Insights

The four actionable tech-debt items, each verified to still exist in the tree:

**TD-01 — P94 security follow-up (hardening).**
- `validateGRPCAddr` at `internal/daemon/grpc_tcp.go:85` was hardened by CR-01 to reject empty-host / wildcard binds.
- `validateAdminAddr` at `internal/daemon/telemetry.go:108` shares the same gap and was flagged as a recommended follow-up (94-REVIEW-FIX.md). Apply the equivalent validation; mirror the `validateGRPCAddr` test cases. Lower severity (admin listener exposes instrumentation only, not the tool surface) — but close it for parity.

**TD-02 — P94 dead code removal.**
- `mergeJSONConfig` at `internal/cli/setup_clients.go:60` is dead production code after the Phase-93 `helix setup` flip (the setup path no longer registers an MCP server, so the JSON-merge helper has no callers). Grep confirms only the definition + comment remain. Remove it and any now-orphaned helpers/imports/tests. Keep `go build`/`go vet` green.

**TD-03 — P93 nudge classifier edge case (cosmetic).**
- `classifyBashTarget` at `internal/cli/nudge.go:337` can count a grep *pattern* that looks like a code path (e.g. `grep foo.go`) as a file operand, causing the advisory hook to over-trigger. The hook is fail-open / exit-0, so this is cosmetic only. Tighten the operand classification (e.g. don't treat a regex/pattern argument of search commands as a file) and add a unit test. Do not change the fail-open contract.

**TD-04 — P95 generated-doc correction.**
- The `get_tool_help` tool Description reads "...documentation for any MCP tool...". It lives at source in `internal/kernel/help/tools.go` (two occurrences) and `internal/kernel/help/skill_adapter.go`, and is surfaced into the generated `README.md:332` table. Reword at source to drop the off-message "MCP tool" phrasing (CLI-first identity), then regenerate docs. README is under the docgen drift gate and MUST NOT be hand-edited — run the docgen regen (`make verify-docs` / the docgen `--check` path) so the table is regenerated and the gate stays green. Keep docgen's blank imports == the daemon's (v1.12 docgen-drift lesson).

**TD-05 — Nyquist coverage (companion task, not code).**
- Phases 93/94/95 `VALIDATION.md` carry `nyquist_compliant: false` (Wave-0 checkboxes unflipped). All three already passed verification; this is a discovery-only coverage formality. Closed by running `/gsd-validate-phase 93|94|95` (separate from the Phase 96 code plan), which audits coverage, fills any genuine gap, and flips the flag.
</code_context>

<specifics>
## Specific Ideas

- Each code fix (TD-01..TD-04) ships with a regression/unit test where applicable.
- TD-04 must go through docgen regen, never a hand-edit of README.md (drift gate).
- Verification target: `go build ./...`, `go vet ./...`, `go test ./...` all green; docgen drift gate green; no diff to `api/proto/` and no change to the frozen verb set.
- TD-05 is orchestrated outside the code plan via `/gsd-validate-phase`.
</specifics>

<deferred>
## Deferred Ideas

- Pre-existing `cmd/helix-bench` `TestRunSubcommandWires*` env-gated failures — out of scope; tracked as the v1.12 "live execution honestly gated-skip" deferred item.
</deferred>

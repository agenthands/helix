# Phase 95: Identity & Docs Rewrite + docgen Regen - Context

**Gathered:** 2026-06-22
**Status:** Ready for planning
**Mode:** Auto-generated (discuss skipped via workflow.skip_discuss)

<domain>
## Phase Boundary

Rewrite Helix's identity to CLI-first across README, CLAUDE.md, and PROJECT.md (Core Value; the Constraints "Protocol: MCP — primary interface" line) so no doc claims MCP as the primary agent interface, and update the CLAUDE.md tool-routing guidance to reference `helix <verb>` instead of MCP tool names. Regenerate the auto-generated tool table against the frozen CLI surface (`cmd/docgen` enumerates verbs; docgen blank-imports stay equal to the daemon's) behind a green drift gate.

Requirements: DOCS-01, DOCS-02, DOCS-03. This is the FINAL v2.0 phase; it describes the final, frozen, MCP-head-free shape after Phase 94.

NOTE: This is NOT a frontend/visual-UI phase. The ui-plan-gate false-positives on the word "interface" (as in "MCP as primary *interface*") — plan with `--skip-ui`.
</domain>

<decisions>
## Implementation Decisions

### Claude's Discretion (bounded by carried STATE constraints + requirements)
All implementation choices at Claude's discretion — discuss skipped. Bounded by:
- **DOCS-01:** No doc may claim MCP as the primary agent interface. CLI-first framing must be consistent across README, CLAUDE.md, PROJECT.md (Core Value + the Constraints "Protocol: MCP — primary interface" line). The CLI (`helix <verb>`) is the agent surface; MCP-SDK/gRPC remain as INTERNAL daemon plumbing (do not claim MCP is removed entirely — it is retained internally; only the agent-facing heads were deleted in Phase 94).
- **DOCS-02:** The CLAUDE.md tool-routing matrix must cite `helix` CLI verbs end-to-end instead of MCP tool names (where that matrix refers to Helix's OWN tools). Preserve the meaning; just re-anchor to the real frozen verb names from `internal/cli/verbs_gen.go`.
- **DOCS-03 (docgen drift gate — heed the v1.12 docgen-drift lesson):** Regenerate the auto-generated tool table against the frozen CLI surface; `cmd/docgen` enumerates verbs; docgen's blank imports MUST stay EQUAL to the daemon's (the v1.12 root cause was `cmd/docgen` missing a blank import — `internal/skill/semantic` — so the table drifted from the live registry). Three-way parity: registry ↔ CLI ↔ docgen. The drift gate must be green.
- Honor zero-proto / no-new-deps where applicable.
</decisions>

<code_context>
## Existing Code Insights

Codebase context gathered during plan-phase research. Known anchors: README.md tool table is auto-generated (do NOT hand-edit per CLAUDE.md); `cmd/docgen` generates it; the daemon's blank imports live in `internal/daemon/imports.go` (docgen's must match); the frozen 50-verb surface is `internal/cli/verbs_gen.go` / `cli.VerbToolNames()`; CLAUDE.md carries the identity ("The IDE for your coding agent"), Core Value, Constraints ("Protocol: MCP — primary interface"), and tool-routing guidance to rewrite. PROJECT.md Core Value + Constraints also need the CLI-first reframing.
</code_context>

<specifics>
## Specific Ideas

No specific requirements beyond the ROADMAP — discuss skipped. Refer to the Phase 95 success criteria: (1) no doc claims MCP primary, CLI-first consistent across the three docs; (2) CLAUDE.md routing matrix cites helix verbs end-to-end; (3) generated tool table lists helix verbs + docgen drift gate green with docgen blank-imports == daemon's.
</specifics>

<deferred>
## Deferred Ideas

None — this is the final v2.0 phase. Milestone audit/complete/cleanup follow after.
</deferred>

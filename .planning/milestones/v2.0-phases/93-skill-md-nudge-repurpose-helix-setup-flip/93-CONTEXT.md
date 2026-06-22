# Phase 93: SKILL.md + Nudge Repurpose + `helix setup` Flip - Context

**Gathered:** 2026-06-21
**Status:** Ready for planning
**Mode:** Auto-generated (discuss skipped via workflow.skip_discuss)

<domain>
## Phase Boundary

Ship an embedded `SKILL.md` (go:embed, frontmatter + `| Question | Use this | Not this |` decision table citing the now-frozen Phase 92 verbs and terse output) whose description fires on code-navigation/edit tasks without over-firing; repurpose the PreToolUse nudge to advisory-steer grep/sed/cat toward the equivalent `helix <verb>` (exit 0, fail-open on unparseable Bash and non-code targets); and flip `helix setup <client>` to install the skill + hooks and tear down any prior MCP registration idempotently across all supported clients. Skill + nudge behavior is verified empirically via the LLM behavioral harness.

Requirements: SKILL-01, SKILL-02, SKILL-03, SKILL-04, TEST-03.

This is NOT a frontend/visual-UI phase — SKILL.md is markdown, the nudge is a Go PreToolUse hook, and `helix setup` is CLI. (The roadmap "UI hint" was corrected to `no` on 2026-06-21.)
</domain>

<decisions>
## Implementation Decisions

### Claude's Discretion
All implementation choices are at Claude's discretion — discuss phase was skipped per user setting. Use ROADMAP phase goal, success criteria, and codebase conventions to guide decisions. Honor the locked roadmap constraints carried in STATE.md: SKILL.md must cite the REAL frozen verb names + real terse output from Phase 92; the nudge must be advisory (exit 0, fail-open); setup teardown must be idempotent across all supported clients.
</decisions>

<code_context>
## Existing Code Insights

Codebase context will be gathered during plan-phase research. Known anchors: the existing PreToolUse nudge lives at `internal/cli/nudge.go`; `helix setup <client>` lives across `internal/cli/setup*.go` (7 clients); the frozen Phase 92 verb surface + terse renderer live in `internal/cli/` (verbs_gen.go, render.go).
</code_context>

<specifics>
## Specific Ideas

No specific requirements — discuss phase skipped. Refer to ROADMAP phase description and success criteria (SKILL.md schema validation + behavioral-oracle triggering; code-symbol grep yields a `helix` suggestion via `additionalContext` while README/log grep yields none and the hook never blocks; idempotent setup with no MCP entry; a measured idle-skill-cost vs preloaded-tool-schema number).
</specifics>

<deferred>
## Deferred Ideas

None — discuss phase skipped.
</deferred>

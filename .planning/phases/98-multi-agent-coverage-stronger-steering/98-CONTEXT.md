# Phase 98: Multi-Agent Coverage + Stronger Steering - Context

**Gathered:** 2026-06-22
**Status:** Ready for planning
**Mode:** Auto-generated (discuss skipped via workflow.skip_discuss)

<domain>
## Phase Boundary

Non-Claude agents (Codex, Gemini, generic) receive the shared generated reference plus a per-agent instruction file installed without clobbering user content, Codex gets the reused advisory nudge hook, and the broadened steering classifier reaches more standard-tool shapes while provably never firing on legitimately-correct prose/log/config use.

**Requirements:** STEER-01, STEER-02, STEER-03, AGENT-01, AGENT-02, AGENT-03

</domain>

<decisions>
## Implementation Decisions

### Claude's Discretion
All implementation choices are at Claude's discretion — discuss phase was skipped per user setting. Use ROADMAP phase goal, success criteria, and codebase conventions to guide decisions.

Key constraints carried from research (SUMMARY.md / ARCHITECTURE.md / PITFALLS.md) and the Phase 97 deviation:
- **DEFER-97-01 lands here:** Phase 97's nudge golden pinned Bash `sed -i`→`replace-in-file` and `cat`→`read-file` as *silent* sub-cases because the classifier's Bash arm only matched grep/find/rg/ag. STEER-01 broadens `classifyBashTarget` so those shapes steer; once they fire, flip the Phase 97 golden's `bash-sed-i-silent`/`bash-cat-silent` sub-cases to asserting the specific verb (keep the contract non-vacuous and still green).
- Steering stays **advisory exit-0 / fail-open** — a deny/block (exit 2) path is an explicit anti-feature (grep/sed/cat are legitimately correct for prose/logs/config/build output). STEER-03 negative-control golden rows MUST prove the nudge does NOT fire on those (e.g. `grep TODO README.md`).
- Codex's `PreToolUse` hook uses the SAME `additionalContext` envelope as Claude — reuse the existing `internal/cli/nudge.go` steering engine (one engine, two runtimes), do not build a second.
- Gemini CLI has **NO** PreToolUse-equivalent — instruction-file (`GEMINI.md`) steering only; do NOT fabricate a hook. IDE/generic also instruction-file only.
- AGENT-02: per-agent instruction files via **idempotent sentinel-delimited append** that never clobbers a user's existing `AGENTS.md`/`GEMINI.md`/generic file (Codex `AGENTS.md` ≤32 KiB cap). Re-run must not duplicate the Helix block. Multi-agent is a shared markdown reference + thin per-agent file — NOT a per-agent bespoke skill engine.
- SessionStart priming (STEER-02) presents the terse "use X not Y" matrix once per session, SKILL-04-style size-capped, fail-open.
- Reuse existing setup machinery in `internal/cli/setup_clients.go` / `setup*.go` (today these 4 non-Claude clients get MCP-teardown only — flip to also-install the reference + instruction file).

</decisions>

<code_context>
## Existing Code Insights

Codebase context will be gathered during plan-phase research. Relevant existing files: `internal/cli/nudge.go` (classifyBashTarget, emitAdvisory, the standard-tool→verb map, exit-0 contract), `internal/cli/setup_clients.go` / `setup_detect.go` / `setup_hooks.go` / `setup_output.go` (per-client setup; current teardown-only for codex/gemini/vscode/jetbrains/opencode/generic), `internal/cli/skill.go` (embed.FS bundle + installSkill from Phase 97; reference.md now shippable), the Phase 97 nudge golden in `internal/cli/nudge_test.go` (silent sed/cat sub-cases to flip).

</code_context>

<specifics>
## Specific Ideas

No specific requirements beyond the ROADMAP success criteria — discuss phase skipped. Refer to ROADMAP Phase 98 description and its 4 success criteria.

</specifics>

<deferred>
## Deferred Ideas

None — discuss phase skipped.

</deferred>

---
phase: 17-usage-install-guides
plan: 02
subsystem: docs
tags: [install, mcp, agents, claude-code, codex, opencode, cursor, gemini-cli, antigravity]

# Dependency graph
requires:
  - phase: 14-documentation
    provides: Initial README.md and USAGE.md structure
provides:
  - INSTALL.md with Go binary setup for 6 coding agents + HTTP mode
  - Deletion of stale Python-based llms-install.md
affects: [README.md cross-references, onboarding flow]

# Tech tracking
tech-stack:
  added: []
  patterns: [per-agent MCP config sections with correct JSON formats]

key-files:
  created: [INSTALL.md]
  modified: []

key-decisions:
  - "Followed plan exactly -- used documented agent config formats from research"
  - "OpenCode uses 'mcp' key with array command format, distinct from all other agents"

patterns-established:
  - "Per-agent MCP config documentation pattern: config file path, JSON block, agent-specific notes"

requirements-completed: [INST-01, INST-02]

# Metrics
duration: 2min
completed: 2026-04-11
---

# Phase 17 Plan 02: Create INSTALL.md Summary

**Go binary install guide with per-agent MCP config sections for Claude Code, Codex, OpenCode, Cursor, Gemini CLI, Antigravity, and HTTP mode**

## Performance

- **Duration:** 2 min
- **Started:** 2026-04-11T09:03:34Z
- **Completed:** 2026-04-11T09:05:47Z
- **Tasks:** 1
- **Files modified:** 2

## Accomplishments
- Created INSTALL.md with shared prerequisites (go install, PATH verification) and 7 agent setup subsections
- Deleted stale Python/uv-based llms-install.md (29 lines of entirely outdated content)
- OpenCode section correctly uses `"mcp"` key with array `"command"` format, distinct from other agents
- Codex includes `--profile=codex`, Cursor includes `--profile=ide-assistant`
- Verification and Next Steps sections link to USAGE.md and README.md

## Task Commits

Each task was committed atomically:

1. **Task 1: Create INSTALL.md and delete llms-install.md** - `8174dbd8` (docs)

## Files Created/Modified
- `INSTALL.md` - Multi-agent install guide for Go binary with 7 agent setup sections
- `llms-install.md` - Deleted (stale Python/uv install instructions)

## Decisions Made
None - followed plan as specified. All agent config formats taken directly from research findings.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- INSTALL.md complete and ready for users
- Cross-references to USAGE.md and README.md in place
- No blockers for remaining phase 17 plans

---
*Phase: 17-usage-install-guides*
*Completed: 2026-04-11*

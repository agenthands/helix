# Requirements: Serena

**Defined:** 2026-04-20
**Core Value:** Rock-solid LSP-backed MCP runtime that survives client disconnects, shares warm caches across sessions, and exposes semantic code operations as tools.

## v1.7 Requirements

Requirements for Developer Experience & Auto-Setup milestone. Each maps to roadmap phases.

### Setup & Registration

- [ ] **SETUP-01**: User can run `serena setup claude-code` to register MCP server and install hooks
- [ ] **SETUP-02**: User can run `serena setup vscode` to register MCP server for VS Code
- [ ] **SETUP-03**: User can run `serena setup jetbrains` to register MCP server for JetBrains IDEs
- [ ] **SETUP-04**: User can run `serena setup claude-desktop` to register MCP server for Claude Desktop
- [ ] **SETUP-05**: User can run `serena setup gemini-cli` to register MCP server for Gemini CLI
- [ ] **SETUP-06**: User can run `serena setup generic` to output MCP config for any stdio client
- [ ] **SETUP-07**: Setup command detects project languages and pre-installs available language servers
- [ ] **SETUP-08**: Setup uses client CLIs as subprocess (not direct file manipulation) for config stability

### Hooks

- [ ] **HOOK-01**: Claude Code SessionStart hook auto-activates project workspace on session start
- [ ] **HOOK-02**: Claude Code PreToolUse hook nudges agents toward symbolic tools when overusing grep/read
- [ ] **HOOK-03**: Claude Code Stop hook cleans up session data on session end
- [ ] **HOOK-04**: Hooks are auto-installed by `serena setup claude-code` into user settings

### Health & Observability

- [ ] **HLTH-01**: Agent can call `get_health` MCP tool to see active LSes and their status
- [ ] **HLTH-02**: `get_health` reports indexing state and available capabilities per workspace
- [ ] **HLTH-03**: User can run `serena status` CLI to see workspace health summary
- [ ] **HLTH-04**: Health reporting defaults to error-only mode (suppress noise, surface actionable failures)

### Smart Errors

- [ ] **SERR-01**: Error responses include "did you mean" suggestions when agents misuse tool parameters
- [ ] **SERR-02**: Error responses suggest parameter corrections (never redirect to different tools)
- [ ] **SERR-03**: Error enrichment middleware wraps existing typed error taxonomy with suggestion payloads

### Progressive Descriptions

- [ ] **DESC-01**: Tool descriptions have tiered detail levels (brief for tool listing, detailed on demand)
- [ ] **DESC-02**: Agent can call `get_tool_help` MCP tool for deep documentation on any tool
- [ ] **DESC-03**: Progressive descriptions are gated by behavioral test coverage before deployment

### Lazy Init

- [ ] **LAZY-01**: First MCP tool call triggers workspace activation if setup wasn't run
- [ ] **LAZY-02**: Lazy init is thread-safe under concurrent first calls (sync.Once pattern)

## Future Requirements

Deferred to future milestones. Tracked but not in current roadmap.

### Extended Hooks

- **HOOK-05**: VS Code PreToolUse hooks for Copilot integration
- **HOOK-06**: JetBrains hook system integration

### Advanced Discovery

- **DESC-04**: Meta-tool pattern (discover+execute indirection) for massive tool surfaces
- **DESC-05**: AI-powered error explanations using LLM context

### Platform

- **PLAT-01**: Auto-update mechanism for Serena binary
- **PLAT-02**: GUI/TUI setup wizard for interactive configuration

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
|---------|--------|
| VS Code/JetBrains hooks | Less mature hook APIs than Claude Code; defer until APIs stabilize |
| Meta-tool pattern | High complexity, marginal gain over tiered descriptions for 41 tools |
| AI-powered error explanations | Adds LLM dependency to runtime; "did you mean" pattern sufficient |
| Auto-update | Binary distribution concern, not DX concern; separate milestone |
| GUI/TUI wizard | CLI-first for agent users; GUI adds complexity without agent value |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| SETUP-01 | — | Pending |
| SETUP-02 | — | Pending |
| SETUP-03 | — | Pending |
| SETUP-04 | — | Pending |
| SETUP-05 | — | Pending |
| SETUP-06 | — | Pending |
| SETUP-07 | — | Pending |
| SETUP-08 | — | Pending |
| HOOK-01 | — | Pending |
| HOOK-02 | — | Pending |
| HOOK-03 | — | Pending |
| HOOK-04 | — | Pending |
| HLTH-01 | — | Pending |
| HLTH-02 | — | Pending |
| HLTH-03 | — | Pending |
| HLTH-04 | — | Pending |
| SERR-01 | — | Pending |
| SERR-02 | — | Pending |
| SERR-03 | — | Pending |
| DESC-01 | — | Pending |
| DESC-02 | — | Pending |
| DESC-03 | — | Pending |
| LAZY-01 | — | Pending |
| LAZY-02 | — | Pending |

**Coverage:**
- v1.7 requirements: 24 total
- Mapped to phases: 0
- Unmapped: 24

---
*Requirements defined: 2026-04-20*
*Last updated: 2026-04-20 after initial definition*

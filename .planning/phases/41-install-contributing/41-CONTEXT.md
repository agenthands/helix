# Phase 41: Install & Contributing - Context

**Gathered:** 2026-04-23
**Status:** Ready for planning

<domain>
## Phase Boundary

Update INSTALL.md and CONTRIBUTING.md to accurately reflect the current codebase through v1.7. INSTALL.md must lead with `serena setup <client>` as the primary install method, with accurate MCP configs for the actual supported clients. CONTRIBUTING.md must reflect the current Go project structure including all packages added through v1.7, and document the full test harness (oracle tests, integration, benchmarks).

</domain>

<decisions>
## Implementation Decisions

### INSTALL.md — Setup CLI as Primary Path
- **D-01:** Lead with `serena setup <client>` as the primary installation method — one command configures everything
- **D-02:** Manual JSON configs become a secondary "Manual Configuration" section for users who prefer explicit control or use unsupported agents
- **D-03:** The client list in INSTALL.md must match the actual `clientRegistry()` in `internal/cli/setup_clients.go`: claude-code, vscode, jetbrains, claude-desktop, gemini-cli, generic
- **D-04:** Remove Codex, OpenCode, Cursor, and Antigravity as separate manual config sections — agents not in the setup CLI registry use the `generic` profile or manual JSON

### INSTALL.md — Structure
- **D-05:** Section order: Prerequisites → Quick Start (setup CLI) → Manual Configuration (collapsed or secondary) → HTTP Mode → Verify Installation → Next Steps
- **D-06:** Keep HTTP mode as a separate section — it serves a different use case (shared daemon, multiple agents)
- **D-07:** Verify Installation section should reference `get_health` tool alongside `onboard_project` for health checking

### CONTRIBUTING.md — Test Harness
- **D-08:** Update test structure to document the full oracle hierarchy: `test/oracle/` with protocol, contract, runtime, scenario, llm, judge sub-layers
- **D-09:** Document the test harness package (`test/harness/`) with Runner, tools, golden, fixture helpers
- **D-10:** Keep existing integration test and benchmark sections but update details (oracle tests are the newer, more comprehensive layer)

### CONTRIBUTING.md — Project Structure
- **D-11:** Add missing packages to structure listing: `internal/repomap/`, `internal/fuzzy/`, `internal/cli/` (setup CLI), `internal/health/` (if exists), `internal/mcp/suggest_lev.go`, `internal/mcp/progressive.go`
- **D-12:** Update tool count references (currently says "9 symbol retrieval tools" etc. — verify against actual count)
- **D-13:** Add `test/oracle/` directory to the project structure section

### Tone & Legacy
- **D-14:** Maintain product-aware technical tone (carrying forward Phase 39 D-03)
- **D-15:** Python legacy acknowledgment stays as brief one-liner at bottom (carrying forward Phase 39 D-04)

### Claude's Discretion
- Exact wording of setup CLI examples and help text
- Whether manual JSON configs are in a collapsed `<details>` section or a flat secondary section
- Level of detail in oracle test layer descriptions
- Whether to add a "Troubleshooting Installation" subsection to INSTALL.md

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements
- `.planning/REQUIREMENTS.md` — INST-01, INST-02, CONT-01, CONT-02 mapped to this phase

### Current State
- `INSTALL.md` — Current install guide being updated (143 lines)
- `CONTRIBUTING.md` — Current contributing guide being updated (146 lines)

### Setup CLI Implementation
- `internal/cli/setup.go` — `serena setup <client>` command implementation
- `internal/cli/setup_clients.go` — Client registry with 6 supported clients (claude-code, vscode, jetbrains, claude-desktop, gemini-cli, generic)
- `internal/cli/setup_detect.go` — Client auto-detection
- `internal/cli/setup_health.go` — Post-setup health check
- `internal/cli/setup_hooks.go` — Client hook registration

### Test Structure
- `test/harness/` — Test harness package (Runner, tools, golden, fixtures)
- `test/oracle/` — Oracle test layers (protocol, contract, runtime, scenario, llm, judge)
- `test/integration/` — MCP round-trip integration tests
- `test/bench/` — Benchmark suite with baselines

### Architecture & Features
- `CLAUDE.md` — Authoritative architecture description and tool inventory
- `internal/repomap/` — RepoMap subsystem
- `internal/fuzzy/` — Fuzzy editing strategies
- `internal/mcp/suggest_lev.go` — Smart error suggestions
- `internal/mcp/progressive.go` — Progressive tool descriptions
- `internal/mcp/lazy_init.go` — Lazy workspace initialization

### Prior Phase Context
- `.planning/phases/39-readme-rewrite/39-CONTEXT.md` — Product tone (D-03), legacy framing (D-04), setup CLI as primary path (D-11)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- INSTALL.md has clean structure with per-agent JSON config sections — restructure around setup CLI
- CONTRIBUTING.md has well-organized sections including "Adding a New MCP Tool" and "Adding Language Support" guides
- Setup CLI (`internal/cli/setup_clients.go`) has 6 registered clients that serve as the authoritative client list

### Established Patterns
- INSTALL.md uses fenced JSON code blocks for each agent config
- CONTRIBUTING.md uses tables for development commands
- CONTRIBUTING.md uses nested bullet lists for project structure (directory → description)

### Integration Points
- `serena setup <client>` replaces manual JSON config as primary path
- `get_health` tool can be referenced for verify installation alongside `onboard_project`
- Test oracle structure needs to be reflected in both CONTRIBUTING.md structure section and test running section

</code_context>

<specifics>
## Specific Ideas

No specific requirements beyond the decisions above — open to standard approaches for documentation structure.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 41-install-contributing*
*Context gathered: 2026-04-23*

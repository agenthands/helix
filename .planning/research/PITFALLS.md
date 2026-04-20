# Domain Pitfalls

**Domain:** Developer Experience & Auto-Setup for MCP Server Platform (v1.7)
**Researched:** 2026-04-20
**Confidence:** HIGH (grounded in Claude Code hook specification, MCP protocol docs, Serena codebase analysis, community issue trackers)

## Critical Pitfalls

Mistakes that cause rewrites, broken existing flows, or user-facing regressions.

### Pitfall 1: Client Config File Format Instability

**What goes wrong:** The `serena setup <client>` command writes to client config files (`.mcp.json`, `~/.claude.json`, `claude_desktop_config.json`, VS Code `mcp.json`) whose format and location shift between client versions. Claude Code moved from `~/.claude.json` mcpServers to `.mcp.json` with `--scope` flags. VS Code uses a different path per platform and changed to `mcp.json` user profile files. Claude Desktop now supports `.mcpb` Desktop Extensions alongside JSON config.

**Why it happens:** Each client (Claude Code, VS Code Copilot, Cursor, JetBrains) has its own config schema, file location, and versioning cadence. Hardcoding paths or formats creates brittleness. JSON syntax errors (trailing commas, duplicate keys) in config files silently disable ALL MCP servers.

**Consequences:** Setup command silently writes to wrong file, server never appears in client. Users report "serena setup worked but no tools show up." Worst case: corrupts existing MCP config and disables all other MCP servers the user had configured.

**Prevention:**
- Use `claude mcp add-json --scope project` subprocess call for Claude Code instead of direct file manipulation. This delegates path resolution and format handling to Claude Code itself.
- For VS Code, use `code --list-extensions` or discover settings path from environment. Never hardcode `~/Library/Application Support/Code/User/`.
- Validate JSON round-trip: parse before writing, parse after writing, compare. Never string-concatenate JSON.
- Add `--dry-run` flag that shows what would be written without modifying anything.
- Each client adapter must be its own Go package with integration tests against real config files.
- Version-pin known config formats per client and document the tested version.

**Detection:** Integration test that runs `serena setup claude-code` and verifies with `claude mcp list`. CI should test on macOS and Linux.

**Phase mapping:** Phase 1 (Setup CLI). This is the foundation feature and must be rock-solid before hooks or health tools can function.

---

### Pitfall 2: Hook Exit Code Semantics Violate Unix Conventions

**What goes wrong:** Claude Code hooks use non-standard exit code semantics that clash with developer expectations. Exit code 1 is NON-blocking (shows stderr as warning, continues). Only exit code 2 blocks tool execution. Developers writing hook scripts naturally use `exit 1` for errors, expecting it to stop execution -- but it doesn't.

**Why it happens:** Claude Code follows its own convention: exit 0 = success (parses JSON), exit 2 = blocking error (uses stderr), exit 1 or other = non-blocking warning (shows stderr, continues). This is documented but counterintuitive. Furthermore, JSON output is ONLY parsed when exit code is 0 -- a hook returning exit 1 with JSON on stdout has its JSON silently ignored.

**Consequences:** A PreToolUse hook meant to block dangerous operations (e.g., remind agent to use Serena tools instead of Bash grep) returns exit 1 thinking it blocks -- but the tool call proceeds. The hook becomes security theater. A SessionStart hook that sets environment via JSON but exits with code 1 has its JSON discarded.

**Prevention:**
- Always use exit 2 for blocking decisions. Document this prominently in generated hook scripts.
- Keep hook scripts minimal: single call to `serena hook <event>` that handles logic in Go and returns the correct exit code. Shell logic in hook scripts is fragile.
- Better: use HTTP hooks (`"type": "http"`) calling the serena daemon's admin listener endpoint. No exit codes, no shell, no permissions issues. Return JSON with `decision: "block"` for blocking.
- Test every hook against the actual Claude Code binary. `claude --print-config` verifies hook presence.

**Detection:** Hook appears to run (stderr visible) but tool call proceeds anyway. Agent ignores reminders that should have been blocking.

**Phase mapping:** Phase 3 (Client Hooks). The exit code gotcha must be documented and tested from the first hook implementation.

---

### Pitfall 3: Progressive Descriptions Breaking Existing Tool Discovery

**What goes wrong:** Changing tool descriptions to be "progressive" (shorter initially, expandable) breaks agents that have already calibrated tool selection against the current descriptions. The existing 41 tools have descriptions that LLMs use for intent matching. Shortening them causes agents to pick wrong tools, miss capabilities, or fail to distinguish between similar tools (e.g., `find_symbol` vs `get_symbols_overview` vs `search_for_pattern`).

**Why it happens:** With 41 tools, descriptions consume ~15-20K tokens at session start. The temptation is to shorten aggressively. But research shows that tool description quality directly impacts agent task success rates. The arXiv paper "MCP Tool Descriptions Are Smelly" (Feb 2026) found that augmented descriptions significantly improved efficiency. Shortening goes against this evidence.

**Consequences:** Agent stops using `find_symbol` and falls back to `search_for_pattern` for symbol lookups. Agent never discovers `get_context` because the short description omits "task-focused" or "PageRank." Regression in LLM behavioral test pass rates. The meta-tool pattern paper shows 85x token savings are possible -- but only when done correctly with a discovery layer, not by truncating existing descriptions.

**Prevention:**
- Run LLM behavioral test suite (existing oracle in `test/oracle/`) before AND after any description changes. This is the regression gate.
- Implement progressive disclosure via a meta-tool pattern (`list_tools_detailed`) rather than shortening existing descriptions. Keep full descriptions, add a discovery layer on top. The agent loads short descriptions initially and requests full specs on demand.
- Profile-specific description overrides (already supported via profile YAMLs) are the right mechanism for per-agent tuning. Don't change the base descriptions.
- A/B test: run behavioral tests with both old and new descriptions, compare pass rates before shipping.

**Detection:** LLM behavioral test pass rate drops. Agents start calling tools with wrong arguments. Usage patterns shift (tools that should be used frequently become rare).

**Phase mapping:** Phase 5 (Progressive Descriptions). Must be the LAST DX feature implemented, after behavioral test coverage is strong enough to detect regressions.

---

### Pitfall 4: Smart Errors Creating Infinite Retry Loops

**What goes wrong:** Error responses that suggest "try tool X instead" or "correct format is Y" cause the agent to retry endlessly. The agent follows the suggestion, hits a different error, gets a different suggestion, and loops until context window exhaustion or turn limit.

**Why it happens:** Suggestions are helpful for humans but dangerous for LLMs that follow instructions literally. Tool call errors are injected back into the LLM context window (this is how MCP works -- errors become prompt context). If `find_symbol` returns "did you mean get_symbols_overview?", the agent calls `get_symbols_overview`, which might fail differently, suggesting yet another tool. The MCP error handling guide recommends a three-part template (what happened, why, correct format) -- but tool-to-tool redirections create chains.

**Consequences:** Agent burns 5-10 tool calls accomplishing nothing. Token waste. User frustration. Worst case: agent enters pathological loop and exhausts the session.

**Prevention:**
- Suggestions must be parameter corrections, not tool redirections. "Invalid path: use relative path from workspace root" is safe. "Use find_symbol instead" is dangerous -- it creates redirect chains.
- Error messages should answer three questions: What happened? Why? What is the correct input format? Include an example of correct input. Never suggest a different tool.
- Keep the existing 7-kind error taxonomy (`NotFound`, `InvalidArgs`, `NoWorkspace`, `Unsupported`, `Internal`, `CircuitOpen`, `Timeout`). Add an optional `hint` field -- don't restructure the error type.
- Make suggestions idempotent: the same wrong input always produces the same suggestion. No state-dependent "try this other thing" logic.
- Cap visibility: if the same tool+error kind fires 3x in a session, suppress the hint on subsequent calls to avoid polluting context.

**Detection:** In LLM behavioral tests, inject intentional errors and verify the agent recovers within 2 retries. Monitor tool call sequences for A->B->A loops.

**Phase mapping:** Phase 4 (Smart Errors). Implement after health/status so you can observe the actual error patterns agents hit in practice.

---

## Moderate Pitfalls

### Pitfall 5: Health Tool Becoming Token-Expensive Background Noise

**What goes wrong:** A `get_health` tool that returns comprehensive LS state, indexing progress, memory usage, and capabilities list generates 2-5K tokens of response. Agents call it reflexively at session start (especially if onboarding workflow suggests it), wasting context window on information they never act on. With 41 tools already consuming ~15-20K tokens in descriptions, adding 2-5K of health data per session is significant.

**Prevention:**
- Return a compact summary by default (5-10 lines max). Detailed output only with `verbose: true` parameter.
- Error-only reporting pattern: healthy components produce NO output. Only unhealthy components appear. "All systems operational" is one line. "gopls: not installed" is actionable.
- Health tool should NOT be in the default onboarding workflow. It's a debugging tool, not a startup ritual.
- If the SessionStart hook already activates the workspace, the health tool is only needed for troubleshooting -- don't encourage routine polling.
- Cap response to ~500 tokens. Anything more goes to the admin metrics endpoint, not the MCP tool response.

**Phase mapping:** Phase 2 (Health/Status). Design for minimal token footprint from day 1.

---

### Pitfall 6: Setup CLI Breaking Existing Daemon Lifecycle

**What goes wrong:** `serena setup` installs hooks and config that assume a specific daemon startup sequence. But the existing architecture (forwarder -> gRPC -> daemon) has its own lifecycle. If setup installs a SessionStart hook that calls `serena activate-workspace` but the daemon isn't running yet (cold start via stdio forwarder), the hook races with daemon initialization.

**Prevention:**
- Setup must embed the absolute path to the `serena` binary in hook commands. Use `which serena` or `os.Executable()` at setup time. Relative paths break when Claude Code's working directory differs from where setup was run.
- SessionStart hook must tolerate daemon cold-start. The existing stdio forwarder auto-starts the daemon via gRPC -- the hook should call through the same path, not try to start a separate daemon.
- Add a `--timeout 15` to all hook command configurations so if daemon startup takes >15s, the hook doesn't hang Claude Code.
- Test the full cold-start path: fresh machine, `serena setup claude-code`, start Claude Code, verify first tool call works end-to-end.
- Setup and hooks must be designed as a pair in the same design session. Implementing them in separate phases without shared design creates integration gaps.

**Phase mapping:** Phase 1 (Setup CLI) and Phase 3 (Hooks) must share a design document even if implemented separately.

---

### Pitfall 7: Lazy Init Racing with First Tool Call

**What goes wrong:** "Lazy workspace init on first tool call" introduces a race: the first tool call triggers workspace activation (LS startup, indexing), which takes 2-30 seconds depending on project size and language server. The agent doesn't wait -- it fires the next tool call immediately, which fails because the workspace isn't ready yet. The `NoWorkspace` error kind fires, the agent retries, and you get a flurry of failed calls.

**Prevention:**
- The first tool call must block synchronously until workspace is ready, then return the actual result. No partial responses, no "initializing, please wait" message that the agent doesn't know how to handle.
- Use the existing worker pool's readiness detection. If workspace activation is in progress, queue subsequent calls behind a `sync.WaitGroup` or `sync.Once` rather than failing them.
- Set a 30-second timeout and return a clear `Timeout` error kind if initialization exceeds it. This is better than hanging indefinitely.
- The lazy init path must be tested explicitly: no setup run, no SessionStart hook, cold daemon, first tool call triggers everything. This is the degraded path and must work.

**Phase mapping:** Phase 1 (Setup CLI -- fallback path). Interacts with existing kernel startup code in `internal/kernel/`.

---

### Pitfall 8: Platform-Specific Path Assumptions

**What goes wrong:** Config file paths differ across macOS and Linux. Claude Desktop: `~/Library/Application Support/Claude/` (macOS) vs `~/.config/Claude/` (Linux). VS Code: `~/Library/Application Support/Code/User/` (macOS) vs `~/.config/Code/User/` (Linux). Claude Code CLI: works via subprocess (`claude mcp add-json`) so less path-sensitive, but the hook scripts' `$CLAUDE_PROJECT_DIR` may contain spaces.

**Prevention:**
- Use Go's `os.UserConfigDir()` and `os.UserHomeDir()` as base, then apply client-specific subdirectories. Never hardcode OS-specific paths.
- Quote all paths in generated hook commands. `"command": "\"$CLAUDE_PROJECT_DIR\"/.claude/hooks/serena-hook.sh"` -- note the escaped quotes around `$CLAUDE_PROJECT_DIR`.
- For Claude Code, prefer subprocess CLI (`claude mcp add-json --scope project`) over direct file manipulation. It handles path resolution internally.
- Windows support is out of scope for v1.7 but design path resolution to be extensible via `runtime.GOOS` switch.
- Test with a project path containing spaces. This catches 80% of path quoting bugs.

**Phase mapping:** Phase 1 (Setup CLI). Path resolution is foundational.

---

### Pitfall 9: Hook Architecture -- Scripts vs HTTP vs Inline Commands

**What goes wrong:** Three hook implementation strategies each have failure modes:
- **Shell scripts:** Permissions (`chmod +x` forgotten), wrong shell (bash vs sh vs zsh), shebang issues on different platforms, script not found when CWD changes.
- **HTTP hooks:** Daemon must be running before hook fires. Connection refused on cold start. HTTP hook timeouts default to 30s (shorter than command hooks at 600s).
- **Inline commands:** `"command": "serena hook pre-tool-use"` -- serena binary must be on PATH, which isn't guaranteed if installed via `go install` without PATH configuration.

**Prevention:**
- Use HTTP hooks calling the serena daemon's admin listener. This is the cleanest architecture: no scripts, no permissions, no shell compatibility, Claude Code handles the HTTP call. The admin listener already exists (`internal/obs/`).
- Fallback for cold-start: use inline command hooks with absolute path to serena binary (embedded at setup time). The command starts the daemon if needed via the existing forwarder auto-start.
- Never generate shell script files. They create maintenance burden (need updating when hook logic changes) and platform-specific bugs.
- Document the hook architecture decision explicitly. Mixing strategies across events creates confusion.

**Phase mapping:** Phase 3 (Client Hooks). Architecture decision must happen before any hook implementation.

---

### Pitfall 10: Extending Error Type Without Breaking Serialization Contract

**What goes wrong:** Adding `Hint`, `Suggestion`, or `CorrectUsage` fields to the existing `*Error` struct breaks the JSON serialization contract. The current struct serializes as `{"kind":"not_found","message":"...","tool":"...","detail":"..."}`. Agents or downstream tools parsing this format may break on unexpected fields. The 4 typed golden test files become invalid. The `extractKind` helper needs updating.

**Prevention:**
- Do NOT add suggestion fields to `internal/errors.Error`. Keep the error type clean for programmatic matching.
- Instead, return suggestions as part of the MCP tool response content -- separate content blocks alongside the error. The MCP SDK supports multiple content items in a tool response. Error is one block (`isError: true`), suggestion is another block (`isError: false`, type `text`).
- If extending `*Error` is unavoidable, new fields must have `omitempty` JSON tags. Run all existing error golden tests. The builder pattern (`WithTool().WithDetail()`) extends naturally to `.WithHint()` as long as hint is omitempty.
- Consider a response wrapper: `type ToolResponse struct { Error *Error; Hint string }` at the handler level, keeping the core error type untouched.

**Phase mapping:** Phase 4 (Smart Errors). Design decision (response wrapper vs error extension) needed before implementation starts.

---

## Minor Pitfalls

### Pitfall 11: Setup Command Conflicting with Existing `.serena/` Config

**What goes wrong:** `serena setup` creates/modifies `.serena/project.yml` in the workspace, but the user already has one with custom language server preferences, profile configuration, or ignore patterns. Setup overwrites their customizations.

**Prevention:** Setup must be additive only. Read existing config, merge new settings, never overwrite existing keys. Use `--force` flag for explicit override. Show diff of what would change before applying. Detect and warn if `.serena/project.yml` already exists.

**Phase mapping:** Phase 1 (Setup CLI).

---

### Pitfall 12: Health Tool Exposing Unstable Internal State

**What goes wrong:** Health tool returns pool sizes, worker counts, internal queue depths, specific LSP protocol versions. Users or agents start depending on specific field names and values. Next version changes internal architecture and breaks consumers who scripted against health output.

**Prevention:** Health response schema should report abstract capabilities (languages available, features active, overall status) not implementation details (worker count, queue depth, RSS). Reserve internals for Prometheus metrics endpoint (already exists) and admin-only endpoints. Version the health response format if external tools depend on it.

**Phase mapping:** Phase 2 (Health/Status).

---

### Pitfall 13: PreToolUse Reminder Flooding Agent Context

**What goes wrong:** A PreToolUse hook that injects "Remember to use Serena tools for code operations" on every tool call adds tokens to every turn. After 30 tool calls in a session, that's 30x the reminder consuming context window. The agent either ignores it (wasted tokens) or over-indexes on it (uses Serena when raw Bash would be faster).

**Prevention:**
- Use `matcher` to fire only on relevant tools: `"Bash|Edit|Write|Read|Glob|Grep"` -- tools where Serena alternatives exist.
- Use the `if` field to filter: only remind when Bash command looks like code navigation (`grep -r`, `find . -name`, `cat src/`).
- Keep reminders under 100 characters. Return as `additionalContext`, not a blocking decision.
- Consider using `"once": true` (runs once per session then auto-removes) for session-level reminders rather than per-tool-call reminders.
- Measure: count reminder injections per session and total token cost. Set a budget (e.g., max 500 tokens of reminders per session).

**Phase mapping:** Phase 3 (Client Hooks). Tuning required after observing real agent behavior.

---

### Pitfall 14: Claude Code Hook JSON Output Size Limit

**What goes wrong:** Claude Code imposes a 10,000 character limit on hook stdout. If a hook returns detailed health data, workspace capability listings, or verbose error context, the output is silently truncated and saved to a file instead of being parsed as JSON. The hook appears to succeed but its JSON response is lost.

**Prevention:**
- Keep all hook JSON output under 5,000 characters (conservative margin).
- Health/status data should go through the MCP tool, not through hooks. Hooks are for lightweight signaling (activate workspace, inject reminder, set env vars).
- If a hook must return data, return only the minimal decision JSON: `{"hookSpecificOutput":{"additionalContext":"..."}}` with the context being a short string.

**Phase mapping:** Phase 3 (Client Hooks).

---

## Phase-Specific Warnings

| Phase Topic | Likely Pitfall | Mitigation |
|-------------|---------------|------------|
| Setup CLI | Config file corruption / wrong format (#1) | Use client CLIs as subprocess (`claude mcp add-json`), not direct file manipulation |
| Setup CLI | Breaking existing `.serena/` config (#11) | Additive-only merge, `--force` flag for override |
| Setup CLI | Platform path assumptions (#8) | `os.UserConfigDir()`, test with spaces in paths |
| Setup CLI | Daemon lifecycle mismatch (#6) | Embed absolute binary path, test cold-start |
| Health/Status | Token-expensive responses (#5) | Error-only reporting, compact by default, <500 token cap |
| Health/Status | Exposing unstable internal state (#12) | Abstract capabilities, not implementation details |
| Client Hooks | Exit code 1 != blocking (#2) | Always exit 2 for blocking, prefer HTTP hooks |
| Client Hooks | Script permissions/shell issues (#9) | HTTP hooks to admin listener, no script files |
| Client Hooks | PreToolUse noise flooding context (#13) | Scoped matchers, `if` filters, `once: true` |
| Client Hooks | JSON output size limit (#14) | Keep hook output <5K chars, use MCP tools for data |
| Smart Errors | Retry loops from suggestions (#4) | Parameter corrections only, never tool redirections |
| Smart Errors | Breaking error serialization (#10) | Response wrapper pattern, not error struct modification |
| Progressive Descriptions | Breaking agent tool selection (#3) | LLM behavioral tests as regression gate, meta-tool pattern |
| Lazy Init | Race with first tool call (#7) | Synchronous blocking init, `sync.Once` for concurrent calls |

## Sources

- [Claude Code Hooks Reference](https://code.claude.com/docs/en/hooks) -- HIGH confidence, official specification for all hook events, exit code semantics, JSON format, matchers
- [Claude Code MCP Configuration](https://code.claude.com/docs/en/mcp) -- HIGH confidence, official .mcp.json format and `claude mcp add` CLI
- [VS Code MCP Server Configuration](https://code.visualstudio.com/docs/copilot/customization/mcp-servers) -- HIGH confidence, official VS Code MCP setup docs
- [MCP Tool Descriptions Are Smelly (arXiv 2602.14878)](https://arxiv.org/html/2602.14878v1) -- MEDIUM confidence, peer research on description optimization
- [Progressive Disclosure Meta-Tool Pattern (SynapticLabs)](https://blog.synapticlabs.ai/bounded-context-packs-meta-tool-pattern) -- MEDIUM confidence, 85x token savings benchmark
- [SEP-1576: Token Bloat in MCP](https://github.com/modelcontextprotocol/modelcontextprotocol/issues/1576) -- MEDIUM confidence, official MCP protocol issue
- [Better MCP Error Responses (Alpic AI)](https://alpic.ai/blog/better-mcp-tool-call-error-responses-ai-recover-gracefully) -- MEDIUM confidence, three-part error template
- [MCP Tool Design: Why Your AI Agent Is Failing (DEV)](https://dev.to/aws-heroes/mcp-tool-design-why-your-ai-agent-is-failing-and-how-to-fix-it-40fc) -- MEDIUM confidence, common MCP design mistakes
- [Claude Code Hook Automation Issue #10447](https://github.com/anthropics/claude-code/issues/10447) -- LOW confidence, feature request showing community pain points
- [Claude Code .mcp.json Loading Issue #5037](https://github.com/anthropics/claude-code/issues/5037) -- LOW confidence, real-world config loading bug
- Serena codebase analysis: `internal/errors/errors.go`, `internal/errors/kinds.go`, `internal/mcp/server.go`, `internal/daemon/`, `internal/config/` -- HIGH confidence, direct code review

---
*Pitfalls research for: Developer Experience & Auto-Setup (v1.7)*
*Researched: 2026-04-20*

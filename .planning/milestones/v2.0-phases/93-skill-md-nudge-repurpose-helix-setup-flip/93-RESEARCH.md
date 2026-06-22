# Phase 93: SKILL.md + Nudge Repurpose + `helix setup` Flip - Research

**Researched:** 2026-06-21
**Domain:** Claude Code Agent Skills (go:embed markdown), PreToolUse hook re-targeting (Go CLI), MCP→skill setup migration, LLM behavioral oracle
**Confidence:** HIGH (every claim anchored to a real file in this repo or to official Claude Code docs fetched this session)

## Summary

Phase 93 has three deliverables, all in `internal/cli/` plus one new embedded markdown asset, with **zero new dependencies and zero proto churn** (the invariant carried since Phase 90). The hard product work — the terse `relpath:line:col<TAB>payload` renderer and the 50 frozen kebab verbs — already shipped in Phase 92; this phase *teaches* the agent those verbs (SKILL.md), *steers* it toward them when it reaches for grep/sed/cat (repurposed nudge), and *migrates* `helix setup` from "register an MCP server" to "install skill + hooks, tear down any prior MCP entry."

The codebase is unusually favorable: the PreToolUse nudge already exists (`internal/cli/nudge.go`), reads hook JSON safely, tracks per-session stats, and always exits 0 — Phase 93 changes its *message* (advisory `helix <verb>` substitution) and *trigger logic* (detect a code target vs a README/log), not its lifecycle. `helix setup` already has a 7-client registrar interface with idempotent JSON merge/remove helpers and a hook installer with `helix_managed: true` tagging — Phase 93 flips the Register path to *call Unregister's MCP-removal first* then write the skill, reusing the exact same idempotency machinery. The `go:embed` pattern is used in ~30 places; `internal/kernel/help/embed.go` is the canonical 2-line analog. The LLM behavioral harness exists at `test/oracle/llm/` (build-tagged `llm`/`llmjudge`, gated on `ANTHROPIC_API_KEY`, with a `tool-selection` test that already prompts a model with a tool list and asserts the chosen tool) — TEST-03 extends this, not invents it.

**Primary recommendation:** Add `internal/cli/skill.go` with a `//go:embed skills/helix/SKILL.md` asset and an `installSkill(dir)` writer; repurpose `nudge.go`'s message + add a Bash-command code-target classifier (fail-open); add a shared `teardownPriorMCP(cfg)` that every registrar's `Register` calls before installing the skill; extend `test/oracle/llm/` with a skill-trigger + grep-baseline comparison test and record the idle-skill-cost token number (computed offline with the existing `bench/evaluators/token_meter` accounting model, or a one-shot `count_tokens` call) directly in SKILL.md.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| SKILL.md authoring + go:embed | CLI / Setup (Layer 3) | — | The skill is a static asset shipped in the binary and written to disk by `helix setup`; it is not kernel logic |
| Skill description triggering | Claude Code runtime (external) | SKILL.md content | The agent host decides when to load a skill from its `description`; our job is to write a description that fires on code-nav/edit and stays dormant otherwise |
| PreToolUse nudge (grep→helix steer) | CLI (Layer 3, `nudge.go`) | Claude Code hook runtime | The hook is a `helix nudge` subprocess invoked by Claude Code on Grep/Read/Bash; it emits advisory `additionalContext`, never blocks |
| Bash-command code-target classification | CLI (`nudge.go`) | — | Pure string parsing of the `command` field; no daemon, no LSP |
| `helix setup` skill+hook install / MCP teardown | CLI / Setup (Layer 3, `setup*.go`) | — | Per-client config-file and CLI-subprocess orchestration; reuses existing registrar interface |
| Behavioral verification (TEST-03) | Test (`test/oracle/llm/`) | external LLM API | Empirical; gated on `ANTHROPIC_API_KEY`/`DEEPSEEK_API_KEY`, build tag `llm` |
| Idle-skill-cost token measurement (SKILL-04) | Test/bench accounting | provider `usage` block | Token number is a fact recorded in SKILL.md, derived from a real count |

## User Constraints (from CONTEXT.md)

### Locked Decisions
None explicitly locked — discuss phase was skipped (`workflow.skip_discuss`). All implementation choices are at Claude's discretion, **bounded by** the locked ROADMAP/STATE constraints below.

### Claude's Discretion
All implementation choices. Honor the locked roadmap constraints carried in STATE.md:
- SKILL.md must cite the **REAL frozen verb names + real terse output** from Phase 92 (not invented verbs).
- The nudge must be **advisory** (exit 0, fail-open on unparseable Bash and non-code targets).
- Setup teardown must be **idempotent across all supported clients**.

### Deferred Ideas (OUT OF SCOPE)
- Deleting the MCP heads (stdio forwarder + `/mcp` HTTP) — that is **Phase 94** ("Delete MCP heads LAST"). Phase 93 must NOT remove the forwarder dial path or any tool handler; it only stops *registering* MCP and tears down *client-side* registration entries.
- Identity/docs rewrite + docgen regen — **Phase 95**.
- A hard-deny grep nudge — explicitly out of scope per REQUIREMENTS "Out of Scope" (false positives on log/README/YAML break legit work).

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| SKILL-01 | Ship `SKILL.md` via `go:embed` with frontmatter (`name`/`description`/`allowed-tools: Bash(helix:*)`) + `\| Question \| Use this \| Not this \|` decision table; description fires on code-nav/edit without over-firing. _Acceptance:_ validates against CC skill schema; behavioral oracle shows triggers-on-code / dormant-otherwise. | CC skill schema confirmed via docs (frontmatter fields, `allowed-tools` scoped-Bash form, 1,536-char description cap, install paths). `go:embed` pattern = `internal/kernel/help/embed.go`. Decision-table analog = CLAUDE.md "Decision matrix". 50 frozen verbs enumerated below. |
| SKILL-02 | `helix setup <client>` installs skill + hooks instead of MCP server, idempotently, across supported clients, tearing down prior MCP registration. _Acceptance:_ `helix setup claude-code` leaves skill + hooks, no MCP entry; re-run idempotent. | `internal/cli/setup.go` + `setup_clients.go` (7 registrars, idempotent `mergeJSONConfig`/`removeFromJSONConfig`), `setup_hooks.go` (`mergeHooksIntoSettings`, `helix_managed` tagging). Teardown = call existing per-client MCP-removal before skill write. |
| SKILL-03 | PreToolUse nudge advisory-steers grep/sed/cat → equivalent `helix <verb>` via `additionalContext`, exit 0, fail-open on unparseable Bash + non-code targets. _Acceptance:_ code-symbol grep → `helix` suggestion; README/log grep → none; never blocks. | `internal/cli/nudge.go` (existing PreToolUse handler, exits 0, safe JSON decode, `isGrepReadTool`). Repurpose message + add code-target classifier. |
| SKILL-04 | SKILL.md token-efficiency rationale backed by a real measured idle-skill-cost vs preloaded-full-tool-schema number. _Acceptance:_ before/after token numbers recorded in SKILL.md or referenced doc. | `bench/evaluators/token_meter/token_meter.go` (provider-usage token accounting model); description capped at 1,536 chars = idle cost upper bound; full 53-tool MCP schema = "before" blob. |
| TEST-03 | Skill + nudge verified via the v1.4 LLM behavioral harness — confirms skill shifts tool-selection toward `helix` and nudge fires correctly. _Acceptance:_ behavioral oracle records tool-selection improvement vs grep/sed/cat baseline. | `test/oracle/llm/` (build-tagged `llm`, `SkipWithoutAPIKey`, `TestSelection` pattern, `AskSingleTurn`, transcript writer). Add skill-vs-baseline comparison test. |

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `embed` (stdlib) | Go 1.x | Embed `SKILL.md` into the binary | Already the repo's universal embed mechanism (~30 `go:embed` sites); zero deps `[VERIFIED: codebase grep]` |
| `encoding/json` (stdlib) | Go 1.x | Idempotent config merge/remove + hook JSON parse | Already used in `setup_clients.go`/`setup_hooks.go`/`nudge.go`; "never string concat" rule (T-34-01) `[VERIFIED: setup_clients.go:61]` |
| `github.com/spf13/cobra` | v1.9.1 | `nudge`/`setup` subcommands | Existing CLI framework `[VERIFIED: go.mod / CLAUDE.md]` |
| `github.com/anthropics/anthropic-sdk-go` | v1.35.0 | LLM behavioral oracle (TEST-03) | Already a dep, already used by `test/oracle/llm/client.go` `[VERIFIED: go.mod]` |
| `github.com/openai/openai-go` | v1.12.0 | DeepSeek-compatible behavioral oracle fallback | Already a dep, already used by `test/oracle/llm/client.go` `[VERIFIED: go.mod]` |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/stretchr/testify` | (in go.mod) | Test assertions | New nudge/skill/setup unit tests + behavioral oracle |
| `test/harness` (internal) | — | Runner/sandbox bringup, `ListTools` | Behavioral oracle still needs `tools/list` to enumerate the tool blob for the "before" measurement (the MCP head is alive until Phase 94) |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `go:embed` single file | `embed.FS` over a `skills/` dir | FS form is justified only if shipping supporting files; SKILL.md is one file → a single `//go:embed skills/helix/SKILL.md\nvar skillMD string` is simpler. Use FS only if you later add reference docs alongside it. |
| Reusing the existing registrar `Unregister` for teardown | A new `teardownPriorMCP` helper | `Unregister` also removes hooks (which we now *want* to keep/install). Prefer a focused MCP-only teardown that does NOT strip hooks. See Pitfall 3. |
| Live `count_tokens` API call for SKILL-04 | Static character/token accounting | A one-shot Anthropic `count_tokens` is most accurate but adds a network/API-key dependency to a doc fact. The 1,536-char description cap gives a hard, dependency-free upper bound on idle cost; record both. |

**Installation:** No new packages. `git diff go.mod` MUST stay empty (carried invariant). `git diff api/proto/` MUST stay empty.

**Version verification:** `[VERIFIED: go.mod]` anthropic-sdk-go v1.35.0, openai-go v1.12.0, cobra v1.9.1 are all already present — no install step, no registry lookup needed. There is no `## Package Legitimacy Audit` section because this phase installs **zero** external packages.

## Architecture Patterns

### System Architecture Diagram

```
                         helix binary (single Go binary)
   ┌───────────────────────────────────────────────────────────────────────┐
   │  internal/cli/                                                          │
   │                                                                        │
   │  skill.go  ──(//go:embed skills/helix/SKILL.md)──►  embedded SKILL.md   │
   │     │                                                                  │
   │     │ installSkill(targetDir)                                          │
   │     ▼                                                                  │
   │  setup.go / setup_clients.go ── per-client Register():                 │
   │     1. teardownPriorMCP(cfg)  ── removeFromJSONConfig(... "helix")     │
   │     2. installSkill(<client skills dir>)  ── write SKILL.md            │
   │     3. mergeHooksIntoSettings(...)  ── SessionStart/PreToolUse/Stop    │
   │                                                                        │
   │  nudge.go ── runNudge(stdin JSON):                                     │
   │     parse hookInput ──► isGrepReadTool? ──► classifyBashTarget()       │
   │         code target ─► emit additionalContext: "try helix <verb>"      │
   │         non-code / unparseable ─► exit 0, no suggestion (FAIL-OPEN)    │
   └───────────────────────────────────────────────────────────────────────┘
                 ▲                                   │ (unchanged)
                 │ Claude Code invokes               │ helix <verb> still dials
                 │ `helix nudge` (PreToolUse)        │ daemon over gRPC StreamMCP
                 │ and loads SKILL.md on             ▼
                 │ code-nav/edit tasks        warm daemon (kernel/LSP/RepoMap)
   ┌─────────────┴───────────────────────────────────────────────────────┐
   │  Claude Code host (external)                                          │
   │   • reads .claude/skills/helix/SKILL.md → decides to load by desc     │
   │   • runs PreToolUse hook on Grep|Read|Bash                           │
   └──────────────────────────────────────────────────────────────────────┘

   test/oracle/llm/ (build tag `llm`, API-key gated)
     baseline run (no skill in system prompt) ─┐
     skill run    (SKILL.md in system prompt) ─┴─► judge: tool-selection toward helix?
```

### Recommended Project Structure

```
internal/cli/
├── skill.go                      # NEW: //go:embed + installSkill/removeSkill/skillTargetDir
├── skill_test.go                 # NEW: embed non-empty, frontmatter parses, idempotent write
├── skills/
│   └── helix/
│       └── SKILL.md              # NEW: the embedded asset (frontmatter + decision table)
├── nudge.go                      # MODIFIED: message → helix-verb advisory; add classifyBashTarget
├── nudge_test.go                 # MODIFIED: code-target=suggest, non-code=silent, always exit 0
├── setup.go                      # MODIFIED: wire skill install into runSetup
├── setup_clients.go              # MODIFIED: each Register calls teardownPriorMCP + installSkill
├── setup_hooks.go                # (reused as-is; PreToolUse matcher already covers Grep|Read|Bash)
└── setup_test.go                 # MODIFIED: post-setup no MCP entry, skill+hooks present, idempotent

test/oracle/llm/
├── skill_trigger_test.go         # NEW: skill present vs absent → tool-selection toward helix
└── prompt.go                     # MODIFIED: add skill-system-prompt + helix-verb task descriptions
```

### Pattern 1: go:embed a single markdown asset (string form)

**What:** Embed SKILL.md as a string for both shipping and `helix setup` to write to disk.
**When to use:** SKILL-01.
**Example:**
```go
// Source: pattern from internal/kernel/help/embed.go (embed.FS form) and
// internal/eval/judge/client.go:21 (//go:embed prompts/rubric.md → string)
package cli

import _ "embed"

//go:embed skills/helix/SKILL.md
var embeddedSkillMD string  // shipped in the binary; written verbatim by installSkill
```
`[VERIFIED: codebase grep]` — `internal/eval/judge/client.go` embeds a single `.md` into a var; `internal/kernel/help/embed.go` uses the `embed.FS` form for a glob. Either works; the single-string form matches the single-file SKILL.md.

### Pattern 2: Idempotent client config mutation (already in repo)

**What:** Read JSON → mutate map → marshal → write; remove is a no-op if the key/file is absent.
**When to use:** SKILL-02 teardown of the prior MCP entry.
**Example:**
```go
// Source: internal/cli/setup_clients.go:97 removeFromJSONConfig
// Already idempotent: missing file → nil; missing key → nil.
func removeFromJSONConfig(path, key, serverName string) error { /* ... */ }
// teardownPriorMCP just calls this with each client's (path,key) pair:
//   claude-code: claude mcp remove helix  (CLI) OR .mcp.json mcpServers
//   vscode:      .vscode/mcp.json  key="servers"
//   opencode:    opencode.json     key="mcp"
//   jetbrains:   .junie/mcp/mcp.json key="mcpServers"
//   etc.
```
`[VERIFIED: setup_clients.go]` — every registrar already has the exact `(path,key,"helix")` triple in its `Unregister`. Teardown = invoke that removal logic from `Register`, but DO NOT also call `removeHooksFromSettings` (we want hooks present).

### Pattern 3: Advisory PreToolUse hook (already in repo, repurpose message)

**What:** Read hook stdin JSON, classify the tool/command, print advisory text, exit 0.
**When to use:** SKILL-03.
**Example:**
```go
// Source: internal/cli/nudge.go:52 runNudge — ALREADY exits 0 on every path,
// ALREADY safe-decodes stdin (json.Decoder, never executes the command).
// Phase 93 change: when isGrepReadTool && classifyBashTarget==code, emit a
// helix-verb suggestion. Today it prints a generic find_symbol tip after a
// 5-call threshold; repurpose to per-call advisory steering.
```
`[VERIFIED: nudge.go:52-107]`. **Emission channel:** the current code uses `fmt.Println` (plain stdout). To surface as `additionalContext` per the Claude Code hook contract, emit a JSON object on stdout:
```json
{"hookSpecificOutput": {"hookEventName": "PreToolUse", "additionalContext": "Tip: `helix search-in-files --pattern=...` is symbol-aware and terser than grep."}}
```
`[CITED: code.claude.com/docs/en/hooks]` — PreToolUse hooks return `additionalContext` under `hookSpecificOutput`; exit 0 = non-blocking advisory. Verify the exact key against the hooks doc during planning (the nudge currently predates the structured-output convention — Wave 0 should confirm the JSON shape the installed Claude Code version expects).

### Pattern 4: grep/sed/cat → helix verb mapping (the steer table)

The nudge's substitution map (cite the FROZEN kebab verbs):

| Bash command shape | Advisory suggestion (frozen verb) |
|--------------------|-----------------------------------|
| `grep <pattern> <code files>` (content search) | `helix search-in-files --pattern=<pattern>` |
| `grep "func X"/"class X"/symbol-decl` | `helix search-symbols --query=<name>` |
| `grep -r "Foo("` (callers) | `helix find-references` / `helix get-call-hierarchy` |
| `find . -name '*.go'` | `helix find-files --pattern='**/*.go'` |
| `cat <code file>` | `helix read-file --path=<file>` (or `helix get-symbol-overview` for shape) |
| `sed -n 'N,Mp' file` (read a range) | `helix read-file --path=<file>` |
| `sed -i 's/.../.../' file` (edit) | `helix replace-in-file` / `helix replace-symbol-body` |

### Anti-Patterns to Avoid

- **Hard-blocking the grep (exit 2 / `permissionDecision: deny`):** explicitly out of scope. The nudge MUST exit 0 always (REQUIREMENTS "Out of Scope": "Hard-deny-every-grep nudge"). `[CITED: REQUIREMENTS.md:98]`
- **Re-converting coordinates / re-implementing the renderer in SKILL.md:** the output shape is frozen; the skill *cites* `relpath:line:col<TAB>payload`, it does not redefine it. `[CITED: 92-02-SUMMARY.md]`
- **Stripping hooks during MCP teardown:** `Unregister` removes both MCP and hooks; teardown must remove MCP only. `[VERIFIED: setup_clients.go:227-264]`
- **Over-broad skill description:** a description mentioning generic "files" or "search" fires on log/config/doc tasks too. Scope it to *code symbols / navigation / refactor*. The 1,536-char description+when_to_use cap forces concision anyway. `[CITED: code.claude.com/docs/en/skills]`
- **Inventing verb names:** the verb surface is generated and parity-locked; only the 50 kebab verbs below exist. `[VERIFIED: verbs_gen.go]`

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Idempotent JSON config merge/remove | A custom diff/patch | `mergeJSONConfig`/`removeFromJSONConfig` (`setup_clients.go`) | Already handles missing file/key, parent dir creation, 0644/0755 perms, T-34-01 no-string-concat |
| Hook entry idempotency | Manual dedup | `mergeHooksIntoSettings` + `helix_managed:true` filter (`setup_hooks.go`) | Already filters prior Helix entries before re-adding; preserves user hooks |
| Per-client config paths | A new path table | The 7 existing registrars' `configPath()` methods | Platform-specific paths (darwin/windows/linux) already correct + tested |
| Safe hook-stdin parsing | `os/exec` on the command | `json.NewDecoder(os.Stdin).Decode` (`nudge.go`) | Never executes the command string; treats it as data (T-36-01) |
| LLM API plumbing for TEST-03 | A new client | `test/oracle/llm/client.go` `AskSingleTurn` | Multi-provider (Anthropic + DeepSeek), key-gated, rate-limited (`InterCallDelay`), transcript writer |
| Token accounting for SKILL-04 | A tokenizer dep | provider `usage` model in `bench/evaluators/token_meter` OR a one-shot `count_tokens` | Repo policy: tokens come from the provider usage block, never a hand-rolled counter (token_meter D-02) |

**Key insight:** Phase 93 is ~80% wiring of machinery that already exists and is already tested. The only genuinely new artifacts are the SKILL.md prose, the Bash-target classifier, and the skill-trigger behavioral test.

## Runtime State Inventory

> This is a setup-migration phase (it changes what `helix setup` writes to client config files and disk). The "runtime state" here is **client-side config the previous MCP-registration left behind**, which the teardown must clean.

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | None — no daemon datastore stores skill/nudge state beyond `.helix/session-stats.json` (per-session, ephemeral, regenerated). `[VERIFIED: nudge.go:79]` | None |
| Live service config (client-side, NOT in this repo's git) | **MCP server entries written by prior `helix setup`** across 7 clients: `claude-code` (`claude mcp` registry / `.mcp.json` `mcpServers.helix`), `vscode` (`.vscode/mcp.json` `servers.helix`), `jetbrains` (`.junie/mcp/mcp.json` `mcpServers.helix`), `claude-desktop` (`claude_desktop_config.json` `mcpServers.helix`), `gemini-cli` (`settings.json` `mcpServers.helix` + `mcp-server-enablement.json` `helix.enabled`), `opencode` (`opencode.json` `mcp.helix`), `generic` (stdout/file). `[VERIFIED: setup_clients.go]` | **Teardown each on Register** — call the existing per-client MCP-removal; for gemini also `ensureDisabled()` the enablement entry |
| OS-registered state | None — no Task Scheduler / launchd / systemd registration. Hooks live in `settings.json` (handled by existing hook merge/remove). `[VERIFIED: setup_hooks.go]` | None |
| Secrets/env vars | `ANTHROPIC_API_KEY` / `DEEPSEEK_API_KEY` consumed by the behavioral oracle only (read, never written). `[VERIFIED: test/oracle/llm/client.go:19]` | None |
| Build artifacts | The embedded SKILL.md is compiled into the binary; `go build` picks it up. No stale artifact (it's a new file). | None |

**The canonical question — after `helix setup` runs, does any prior MCP registration linger?** Without teardown, yes: a user who previously ran `helix setup claude-code` (MCP) and then upgrades and re-runs it would have BOTH an MCP `helix` server entry AND the new skill/hooks. SKILL-02's acceptance ("no MCP server entry") requires the teardown step explicitly. The 7 removal code paths already exist in `Unregister`; the work is calling the MCP-only subset from `Register`.

## Common Pitfalls

### Pitfall 1: Skill description over-firing (the central SKILL-01 risk)
**What goes wrong:** A broad description ("search files", "read code") makes Claude load the skill on README edits, log greps, and config tasks, wasting context and annoying users.
**Why it happens:** The host triggers purely on the `description` (+ optional `when_to_use`); body content is irrelevant to triggering. `[CITED: code.claude.com/docs/en/skills]`
**How to avoid:** Scope the description to **symbol-level code navigation and refactoring** ("find where a symbol is defined, who calls it, rename across files, replace a function body"). Mirror the precise verbs in CLAUDE.md's SMTC "Decision matrix" phrasing. Keep description+when_to_use under the 1,536-char listing cap (also doubles as the SKILL-04 idle-cost bound). Verify empirically with the behavioral oracle: a code task loads it, a "summarize this README" / "tail the log" task does not.
**Warning signs:** Behavioral oracle shows the skill triggering on non-code prompts; description contains generic words like "files", "text", "search" without a code qualifier.

### Pitfall 2: Nudge false positives on non-code targets
**What goes wrong:** `grep TODO README.md` or `grep error app.log` yields a spurious "use helix" suggestion, steering the agent toward a tool that can't help (helix is code-aware, not a log grepper).
**Why it happens:** A naive `strings.Contains(cmd, "grep")` (the *current* `isGrepReadTool`) can't tell a `.go` target from a `.log`/`.md` target. `[VERIFIED: nudge.go:186-197]`
**How to avoid:** Add `classifyBashTarget(cmd) -> (isCode bool, ok bool)`: tokenize the command, find file/path operands, check extensions against a code-extension set (reuse the spirit of `langregistry.ByExtension` or a small allowlist). If parsing fails OR no code target is found OR the target is `.md/.log/.txt/.json/.yaml/Dockerfile` → **fail open** (no suggestion, exit 0). Only suggest when a code target is positively identified.
**Warning signs:** Suggestions on README/log/YAML greps in the behavioral oracle; the classifier returning `isCode=true` by default.

### Pitfall 3: Teardown that also nukes the hooks it just installed
**What goes wrong:** Reusing `registrar.Unregister(cfg)` for teardown removes the MCP entry *and* calls `removeHooksFromSettings` — then the skill+hook install re-adds hooks, but a careless ordering (teardown-after-install) leaves no hooks, or a partial failure leaves an inconsistent state.
**Why it happens:** `Unregister` bundles MCP-removal + hook-removal. `[VERIFIED: setup_clients.go:251-261]`
**How to avoid:** Introduce an MCP-only teardown (the `removeFromJSONConfig`/`claude mcp remove` half, *without* `removeHooksFromSettings`). Order: (1) MCP teardown, (2) skill write, (3) hook merge. The hook merge is already idempotent (filters prior `helix_managed` then re-adds), so re-runs converge.
**Warning signs:** Setup-then-setup leaves zero hooks; idempotency test flaps.

### Pitfall 4: `additionalContext` emission channel mismatch
**What goes wrong:** The nudge prints a plain string to stdout (current behavior), but the installed Claude Code version expects structured `hookSpecificOutput.additionalContext` JSON — the advice is dropped or mis-surfaced.
**Why it happens:** `nudge.go` predates the structured hook-output convention (`fmt.Println` of a tip). `[VERIFIED: nudge.go:99]`
**How to avoid:** Wave 0 — confirm the exact PreToolUse output schema against `code.claude.com/docs/en/hooks` for the version Helix targets, then emit JSON. Keep exit code 0 regardless (a malformed advisory must never block).
**Warning signs:** Behavioral/manual test shows the suggestion text appearing as a raw tool-output line rather than injected context.

### Pitfall 5: docgen / cligen drift (the carried v1.12 lesson)
**What goes wrong:** SKILL.md hardcodes a verb list that drifts from `verbs_gen.go`, or someone regenerates verbs without updating the skill's decision table.
**Why it happens:** Two independent lists of the same truth. STATE.md flags the docgen-drift lesson explicitly. `[CITED: STATE.md:75]`
**How to avoid:** SKILL.md cites verbs by capability group (navigation/edit/fileops/diagnostics/repomap/memory) matching `verbs_gen.go`'s `groupID`. Optionally add a lightweight test that asserts every verb named in SKILL.md exists in `cli.VerbToolNames()` (set-membership), catching drift in the default suite. Do NOT hardcode a verb *count* in prose (VERB-01 lesson: count == live registry, not a literal).
**Warning signs:** SKILL.md names a verb not in `VerbToolNames()`; a verb rename leaves SKILL.md stale.

### Pitfall 6: Zero-proto / zero-dep invariant breach
**What goes wrong:** Pulling in a YAML/frontmatter parser or a tokenizer adds a dependency; touching proto for some perceived need.
**How to avoid:** Frontmatter is hand-authored YAML inside the markdown — no parser needed at runtime (the binary just writes the file verbatim; validation is a test concern and can use the already-vendored test libs). Token count uses provider usage / char-cap, not a tokenizer dep. Assert `git diff go.mod` and `git diff api/proto/` empty in the verification gate (same as Phase 92). `[CITED: 92-02-SUMMARY.md:125]`

## Code Examples

### The 50 frozen verbs by capability group (cite these in SKILL.md)
```
# Source: internal/cli/verbs_gen.go (generated, parity-locked to live registry)
navigation:  go-to-definition  find-references  search-symbols  get-hover-info
             find-implementations  get-call-hierarchy  get-type-hierarchy
             get-symbol-overview  analyze-blast-radius
edit:        replace-symbol-body  rename-symbol  safe-delete-symbol
             insert-before-symbol  insert-after-symbol  verify-edit
fileops:     read-file  create-file  find-files  list-directory
             search-in-files  replace-in-file  fuzzy-edit
diagnostics: get-diagnostics  get-code-actions  format-code
repomap:     get-repo-map  get-context  get-semantic-context  get-cluster-map
             explain-cluster  explain-symbol-deep  find-related-symbols
             get-change-impact-graph  get-semantic-graph-status
             index-semantic-graph  refresh-semantic-graph  validate-graph-edge
memory:      read-memory  write-memory  list-memories  search-memories
             rename-memory  edit-memory  delete-memory  onboard-project
             prepare-for-new-conversation  switch-mode  get-token-budget
             get-health  get-tool-help
```
`[VERIFIED: verbs_gen.go grep — 50 kebab verb keys; the 53-tool count includes 3 dynamic-handler tools without derivable flags]`

### Terse output shape to cite in SKILL.md
```
# Source: 92-02-SUMMARY.md / 92-03 goldens — FROZEN
relpath:line:col<TAB>payload          # locus-list verbs (sorted, deduped, 1-based)
internal/cli/render.go:11:6<TAB>func Helper() {   # nav verbs append one snippet line
# --abs → absolute paths; --json → one compact {path,line,col,payload} per line
# --color=never ≡ piped (zero ANSI); per-kind stderr prefix + non-zero exit on error
```
`[VERIFIED: 92-02-SUMMARY.md, 92-03-SUMMARY.md goldens]`

### SKILL.md frontmatter skeleton (SKILL-01)
```markdown
---
name: helix
description: >-
  Symbol-aware code navigation and editing via the `helix` CLI. Use when you need
  to find where a symbol is defined, who calls a function, all references to a
  name, an interface's implementations, a type/call hierarchy, a file's symbol
  outline, or to rename a symbol / replace a function body across files — instead
  of grep/sed/cat. Output is terse `relpath:line:col<TAB>payload`, copy-paste-able
  into the next verb. Dormant for prose/log/config tasks.
allowed-tools: Bash(helix:*)
---

| Question | Use this | Not this |
|---|---|---|
| Where is symbol `X` defined? | `helix go-to-definition --path --line --column` | `grep "X"` |
| Who calls / references `Y`? | `helix find-references` / `helix get-call-hierarchy` | `grep -r "Y("` |
| Find a symbol by name | `helix search-symbols --query=Foo` | `grep "func Foo"` |
| Content search across code | `helix search-in-files --pattern=...` | `grep -r ...` |
| File's shape (outline) | `helix get-symbol-overview` | `cat file` |
| Read a file | `helix read-file --path=...` | `cat` / `sed -n` |
| Rename across files | `helix rename-symbol --new-name=...` | `sed -i` |
| Replace a function body | `helix replace-symbol-body` | `sed -i` |
| Implementations of an interface | `helix find-implementations` | `grep "implements"` |
| Type hierarchy | `helix get-type-hierarchy` | `grep "extends"` |
| Impact of changing a symbol | `helix analyze-blast-radius` | recursive grep |

<!-- Token note (SKILL-04): idle skill cost ≈ <N> tokens (description+table listing,
     ≤1,536 chars); preloaded full MCP tool schema ≈ <M> tokens. Measured <date>. -->
```
`[CITED: code.claude.com/docs/en/skills — allowed-tools scoped-Bash form, description cap]`, `[VERIFIED: CLAUDE.md decision-matrix format]`

### Behavioral oracle: skill vs grep baseline (TEST-03)
```go
// Source: extends test/oracle/llm/selection_test.go pattern
//go:build llm
// Two runs of the same code task:
//   baseline: system prompt WITHOUT SKILL.md → record chosen tool
//   skill:    system prompt WITH SKILL.md body → record chosen tool
// Assert: skill run shifts selection toward a `helix` verb vs grep/sed/cat.
// Gated by SkipWithoutAPIKey(t); uses AskSingleTurn; writes transcripts.
```
`[VERIFIED: test/oracle/llm/selection_test.go, client.go]`

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Custom slash commands in `.claude/commands/*.md` | Merged into Skills (`.claude/skills/<name>/SKILL.md`) | Recent CC | A skill *is* the modern unit; `helix setup` writes a skill dir, not a command file `[CITED: code.claude.com/docs/en/skills]` |
| Nudge = generic "use find_symbol" tip after a 5-call threshold | Per-call advisory `helix <verb>` substitution with code-target gating | This phase | More targeted, fail-open `[VERIFIED: nudge.go vs SKILL-03]` |
| `helix setup` registers an MCP stdio server | Installs skill + hooks, tears down MCP | This phase (SKILL-02) | The agent surface becomes the CLI, not MCP |
| MCP tool descriptions teach tool selection (`tools/list` blob preloaded) | SKILL.md decision table teaches it on-demand | This phase | Idle cost drops from full schema to a ≤1,536-char description (SKILL-04) |

**Deprecated/outdated (do NOT touch in Phase 93):**
- The stdio forwarder head and `/mcp` HTTP transport are deprecated but **removed in Phase 94**, not here. The MCP SDK, gRPC IPC, 5 middlewares, and all tool handlers stay. `[CITED: STATE.md:74]`

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | The installed Claude Code version surfaces PreToolUse advice via `hookSpecificOutput.additionalContext` JSON on stdout (vs the current plain `fmt.Println`). | Pitfall 4 / Pattern 3 | LOW — if the plain-stdout form still works, the nudge is even simpler; Wave 0 confirms the schema. The exit-0 fail-open contract holds either way. |
| A2 | `allowed-tools: Bash(helix:*)` is accepted scoped-Bash syntax (docs show `Bash(git add *)`; REQUIREMENTS specifies `Bash(helix:*)`). | SKILL.md skeleton | LOW — docs confirm scoped Bash; the exact glob (`helix:*` vs `helix *`) should be validated against the schema validator in the skill test. |
| A3 | Idle skill cost is bounded by the description+when_to_use 1,536-char listing cap; the body loads only on trigger. | SKILL-04 | LOW — docs state body loads only when used; the "before" number (full MCP schema) is measurable via `tools/list` while the MCP head is still alive (pre-Phase-94). |
| A4 | A code-extension allowlist (or `langregistry.ByExtension`) is sufficient to classify a Bash grep/cat target as code vs non-code. | Pitfall 2 | MEDIUM — heredocs, pipes, and globbed multi-file commands are hard to parse; the fail-open default makes wrong-but-silent the safe failure mode (no false suggestion). |
| A5 | TEST-03 can run hermetically-skipped in CI (no API key) and only RUN locally with a key, matching the existing `llm` build-tag gating. | TEST-03 | LOW — `SkipWithoutAPIKey` is the established pattern; CI stays green without keys. |

## Open Questions

1. **Exact `additionalContext` JSON schema for the target Claude Code version**
   - What we know: PreToolUse hooks return advisory context; exit 0 = non-blocking.
   - What's unclear: the precise key path (`hookSpecificOutput.additionalContext`) and whether plain stdout still injects context in the installed version.
   - Recommendation: Wave 0 reads `code.claude.com/docs/en/hooks` and confirms; default to the structured JSON form, keep exit 0.

2. **Which clients get the skill written where (per-client skills dir)**
   - What we know: Claude Code uses `.claude/skills/<name>/SKILL.md` (project) or `~/.claude/skills/...` (global). `[CITED: docs]`
   - What's unclear: non-Claude clients (vscode/jetbrains/gemini/opencode) may not consume Agent Skills at all — the skill format is Claude-Code-specific (with an `agentskills.io` open standard).
   - Recommendation: For Claude-family clients (claude-code, claude-desktop) write the skill + hooks and tear down MCP. For non-Claude clients, the "flip" may mean **tear down MCP only** (the skill has no consumer there) OR write to the client's equivalent instruction file if one exists. Decide per client in planning; SKILL-02 acceptance is specified for `claude-code` concretely. Default the global flag to `--global` semantics already supported by every registrar.

3. **SKILL-04 measurement method: live `count_tokens` vs static accounting**
   - What we know: provider-usage is the repo's trusted token source; description capped at 1,536 chars.
   - What's unclear: whether to spend an API call for an exact count or record the char-cap bound + an estimated token ratio.
   - Recommendation: Record BOTH — the hard char-cap bound (dependency-free) and, if a key is present, an exact `count_tokens` of (a) the SKILL.md description and (b) the full `tools/list` schema blob. Store the number in SKILL.md (a referenced doc is also acceptable per acceptance text).

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain (`go build`/`go test`) | All deliverables | ✓ | host CC, CGO=1 | — |
| `anthropic-sdk-go` | TEST-03 (behavioral oracle) | ✓ (vendored) | v1.35.0 | DeepSeek via openai-go |
| `openai-go` | TEST-03 DeepSeek path | ✓ (vendored) | v1.12.0 | — |
| `ANTHROPIC_API_KEY` / `DEEPSEEK_API_KEY` | TEST-03 RUN (not build) | ✗ at rest | — | `SkipWithoutAPIKey` → hermetic skip in CI; RUN locally with a key |
| `HELIX_BIN` (built binary) | Behavioral/E2E that subprocess `helix` | ✗ unless set | — | tests `t.Skip` when unset (established pattern) |
| `claude`/`gemini` client CLIs | setup teardown via CLI subprocess | host-dependent | — | Every registrar already falls back to direct config-file write when the CLI is absent `[VERIFIED: setup_clients.go:162]` |

**Missing dependencies with no fallback:** None — every external is either vendored or gracefully skipped/falls back.
**Missing dependencies with fallback:** API keys (skip), client CLIs (direct file write), HELIX_BIN (skip).

## Validation Architecture

> `workflow.nyquist_validation` is not explicitly false in this milestone's history; treat as enabled.

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go `testing` + `stretchr/testify` |
| Config file | none (standard `go test`); build tags `llm`/`llmjudge`/`integration` for gated tiers |
| Quick run command | `go test ./internal/cli/...` (unit: nudge classifier, skill embed, setup teardown) |
| Full suite command | `go test ./... && go vet ./...` (default suite, hermetic, no keys) |
| Behavioral (gated) | `ANTHROPIC_API_KEY=... go test -tags llm ./test/oracle/llm/...` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| SKILL-01 | embedded SKILL.md non-empty + valid frontmatter + every named verb ∈ `VerbToolNames()` | unit | `go test ./internal/cli/ -run TestSkill` | ❌ Wave 0 (`skill_test.go`) |
| SKILL-01 | description triggers on code task, dormant on prose/log | behavioral | `go test -tags llm -run TestSkillTrigger ./test/oracle/llm/...` | ❌ Wave 0 (`skill_trigger_test.go`) |
| SKILL-02 | post-setup: no MCP `helix` entry, skill + hooks present | unit | `go test ./internal/cli/ -run TestSetup` | ⚠️ extend `setup_test.go` |
| SKILL-02 | re-running setup is idempotent (byte-stable config) | unit | `go test ./internal/cli/ -run TestSetup.*Idempotent` | ⚠️ extend `setup_test.go` |
| SKILL-02 | teardown covers all 7 clients (MCP-removal invoked per client) | unit | `go test ./internal/cli/ -run TestTeardownPriorMCP` | ❌ Wave 0 |
| SKILL-03 | code-symbol grep → `helix` `additionalContext`; README/log grep → none; always exit 0 | unit | `go test ./internal/cli/ -run TestNudge` | ⚠️ extend `nudge_test.go` |
| SKILL-03 | unparseable Bash → fail-open (no suggestion, exit 0) | unit | `go test ./internal/cli/ -run TestNudge.*FailOpen` | ❌ Wave 0 |
| SKILL-04 | a real token number is present in SKILL.md / referenced doc | unit (presence) + manual (measure) | `go test ./internal/cli/ -run TestSkill.*TokenNote` | ❌ Wave 0 |
| TEST-03 | skill shifts tool-selection toward `helix` vs grep baseline | behavioral | `go test -tags llm -run TestSkillVsBaseline ./test/oracle/llm/...` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/cli/... && go vet ./...` (sub-second, hermetic).
- **Per wave merge:** `go test ./... ` (full default suite) + `make verify-cligen` (drift) + `git diff go.mod api/proto/` empty.
- **Phase gate:** default suite green; behavioral oracle RUN once with a key (records the tool-selection improvement number into a transcript/SUMMARY); SKILL.md token number filled in.

### Wave 0 Gaps
- [ ] `internal/cli/skill.go` + `skills/helix/SKILL.md` — the embedded asset (SKILL-01)
- [ ] `internal/cli/skill_test.go` — embed non-empty, frontmatter parses, verb-in-catalog membership, token-note presence
- [ ] `internal/cli/nudge.go` `classifyBashTarget` + repurposed advisory message (SKILL-03)
- [ ] `internal/cli/setup_clients.go` `teardownPriorMCP` (MCP-only, no hook strip) wired into each `Register`
- [ ] `test/oracle/llm/skill_trigger_test.go` + prompt additions (TEST-03, SKILL-01 trigger)
- [ ] SKILL-04 token measurement (offline count or `count_tokens`) recorded in SKILL.md
- [ ] Framework install: none — all test libs already vendored

## Security Domain

> `security_enforcement` is enabled by default. This phase adds no new network endpoint, no new deserialization of untrusted data beyond what already exists, and no new package. The relevant threats are input-handling and config-write integrity.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | No auth surface added |
| V3 Session Management | no | Hook session-stats are ephemeral, non-security |
| V4 Access Control | no | (Profile/mode enforcement landed in Phase 91; unchanged) |
| V5 Input Validation | yes | Hook stdin is parsed with `json.Decoder` as data, never executed (T-36-01, existing). The new Bash-target classifier MUST also treat the command string as data — tokenize, never `exec`. |
| V6 Cryptography | no | None |
| V12 File / Resource | yes | Config writes use `encoding/json` Marshal (never string concat, T-34-01), 0644/0755 perms; skill write to `.claude/skills/` must stay within the resolved target dir (no path traversal from a crafted client/project dir). |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Hook command injection (treating the grep command string as executable) | Tampering / Elevation | Parse stdin/command as data only; the classifier never shells out (`nudge.go` precedent T-36-01) `[VERIFIED]` |
| Config-file corruption on concurrent setup/hook writes | Tampering | Atomic temp-file+rename for stats (`saveSessionStats`); Marshal-then-write for config (existing). Skill write should likewise be atomic. |
| Path traversal writing SKILL.md outside the skills dir | Tampering | Resolve + contain the target path before write (mirror `readSnippetLine`'s `filepath.Rel`+`..`-prefix check from 92-02). |
| API key disclosure in test logs | Info Disclosure | `SkipWithoutAPIKey` never logs the key value (T-21-01, existing). `[VERIFIED]` |
| Advisory nudge becoming a blocking gate (DoS of legit greps) | Denial of Service | Hard invariant: exit 0 on every path, fail-open. `[CITED: REQUIREMENTS Out-of-Scope]` |

## Project Constraints (from CLAUDE.md)

- **Go single binary; no Python/Docker/runtime deps** — SKILL.md ships via `go:embed`, no external asset fetch.
- **`go vet ./...` and `go test ./...` before completing any Go task** — both in the verification gate.
- **GSD workflow enforcement** — edits go through a GSD command (this is plan/execute-phase work).
- **SMTC-first tool routing** — used SMTC-aware reading where applicable; this repo has no security capability (Go), so no `java-security` activation.
- **Carried invariants (STATE.md):** zero-proto (`git diff api/proto/` empty), zero-dep (`git diff go.mod` empty), tool count = live registry (don't hardcode), docgen/cligen blank-imports stay == daemon's, "Delete MCP heads LAST" (Phase 94 — NOT here).

## Sources

### Primary (HIGH confidence)
- `internal/cli/nudge.go`, `setup.go`, `setup_clients.go`, `setup_hooks.go`, `setup_detect.go`, `verbs_gen.go`, `verb.go`, `render.go` — current implementations (read this session)
- `internal/kernel/help/embed.go`, `internal/eval/judge/client.go` — `go:embed` patterns
- `test/oracle/llm/{client,prompt,selection_test,doc}.go` — the LLM behavioral harness
- `bench/evaluators/token_meter/token_meter.go` — token accounting model
- `.planning/REQUIREMENTS.md`, `.planning/STATE.md`, `.planning/ROADMAP.md`, `92-02-SUMMARY.md`, `92-03-SUMMARY.md`, `93-CONTEXT.md`
- `CLAUDE.md` — SMTC "Decision matrix" (decision-table format analog)
- `code.claude.com/docs/en/skills` — Agent Skills frontmatter schema, `allowed-tools` scoped-Bash, 1,536-char description cap, install paths (fetched this session)

### Secondary (MEDIUM confidence)
- `code.claude.com/docs/en/hooks` — PreToolUse `additionalContext` output shape (to be confirmed in Wave 0 for the exact key path)

### Tertiary (LOW confidence)
- None.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — zero new deps; everything vendored and verified in go.mod.
- Architecture: HIGH — all three deliverables map onto existing, tested machinery (nudge, registrars, embed, llm harness).
- Pitfalls: HIGH — derived from the actual current code (over-broad `isGrepReadTool`, `Unregister` bundling hooks, plain-stdout nudge) and the carried STATE constraints.
- Skill schema: HIGH — confirmed against official Claude Code docs this session.
- `additionalContext` exact JSON key: MEDIUM — confirm in Wave 0.

**Research date:** 2026-06-21
**Valid until:** 2026-07-21 (stable; Claude Code skill/hook schema is the only fast-moving external — re-verify if CC ships a schema change)

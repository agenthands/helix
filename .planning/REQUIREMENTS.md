# Milestone v2.0 — CLI-First (MCP Surface Retirement)

**Defined:** 2026-06-21
**Status:** Active (locked 2026-06-21)
**Source of truth:** This file (REQ-IDs are normative for the roadmap)
**Upstream context:** `.planning/PROJECT.md` (Current Milestone v2.0), `.planning/research/{STACK,FEATURES,ARCHITECTURE,PITFALLS,SUMMARY}.md`

**Core Value (this milestone):** The `helix` CLI is the only surface an agent touches — terse, `file:line`-anchored, zero schema-preload tax — driving the unchanged warm LSP/RepoMap kernel behind it, so agents actually use the toolset instead of falling back to grep/sed/cat.

**Locked architecture decision:** "Rip out MCP" = remove the *agent-facing* surface only (stdio forwarder head + Streamable-HTTP transport). The MCP SDK stays inside the daemon as the internal tool-dispatch + 5-middleware engine; the CLI drives it over the existing gRPC `StreamMCP` wire via one-shot `tools/call`. Excising the SDK (hat #2) is out of scope.

**Locked design decisions (2026-06-21):**
- Output flags `--json` / `--color` are **global persistent root flags**; text default, color=auto (off when piped).
- The grep/sed/cat → `helix` nudge is **advisory-only** (exit 0, `additionalContext`), never blocking.
- The retained gRPC IPC **optionally binds a TCP address** for split-host CLI↔daemon use (loopback default; non-loopback opt-in + gated).
- File-location format: **workspace-relative `relpath:line:col`, 1-based** coordinates, deterministic ordering, `--abs` escape hatch — empirically validated against the LLM behavioral oracle (OUT-07).

---

## v1 Requirements

Every REQ has a one-line acceptance test. The roadmap maps each REQ to exactly one phase. REQ-IDs use new v2.0 category prefixes (no prior milestone used these).

### CLI Invocation Spine (CLI-*)

- [ ] **CLI-01**: `helix <verb>` invokes a single tool against the warm daemon via a one-shot `tools/call` over the existing gRPC `StreamMCP` wire, reusing `forwarder.ConnectOrStartDaemon`. _Acceptance:_ a representative verb round-trips through the daemon and returns the same tool result the MCP path returned; `git diff api/proto/` is empty (zero proto changes).
- [ ] **CLI-02**: CLI invocations auto-start the daemon on a cold host and reuse a warm daemon thereafter; second-call latency meets a phase-set SLO. _Acceptance:_ first call spawns the daemon once; warm second-call p50 is below the SLO recorded in the phase.
- [ ] **CLI-03**: Daemon auto-start is race-free under parallel first calls — a cross-process startup lock prevents duplicate daemon spawns and connect storms. _Acceptance:_ N parallel cold `helix` invocations result in exactly one daemon process (integration/synctest).
- [ ] **CLI-04**: No-arg `helix` prints grouped usage/help and no longer launches a stdio MCP server. _Acceptance:_ `helix` with no args exits 0 with grouped command help and opens no MCP stdio session.

### Verb Surface & Codegen (VERB-*)

- [ ] **VERB-01**: Every callable tool in the live registry has a corresponding `helix <verb>` subcommand, code-generated from the typed `*Args` structs (existing `json` + `jsonschema` tags). _Acceptance:_ a parity test asserts generated subcommand count == live registry tool count, enumerated by name.
- [ ] **VERB-02**: A `helix-cligen` generator emits committed `*_gen.go`; a `--check` CI drift gate fails on stale generated code (mirrors the docgen / PromQL gates). _Acceptance:_ editing a tool's args without regenerating fails CI.
- [ ] **VERB-03**: `helix --help` groups verbs by capability (navigation / edit / fileops / diagnostics / repomap / memory) via cobra `AddGroup`; each verb's `--help` shows flags derived from its arg struct with required-ness. _Acceptance:_ grouped help renders; a required arg without a flag value errors before dialing the daemon.
- [ ] **VERB-04**: The tool-name → arg-struct-type mapping the generator needs is available (a `ToolDef` field or a generator-side table). _Acceptance:_ the generator resolves the arg struct for every registered tool with no manual per-tool edits.

### Terse Output Contract (OUT-*) — load-bearing

- [ ] **OUT-01**: Default output is terse `relpath:line:col<TAB>payload` — workspace-relative paths, 1-based line/col (converted from LSP 0-based), no ANSI when non-tty, honoring `NO_COLOR`. _Acceptance:_ piped output contains zero ANSI bytes; coordinates are 1-based; per-verb golden matches.
- [ ] **OUT-02**: Output ordering is deterministic (sorted + deduped) across repeated runs. _Acceptance:_ the same query yields byte-identical output across N repeated runs.
- [ ] **OUT-03**: Each verb's output is self-contained enough to act on in one round-trip (navigation verbs print locus + enclosing symbol + one snippet line; outline verbs print shape only). _Acceptance:_ per-verb goldens; behavioral-oracle check that nav verbs do not force a follow-up `Read`.
- [ ] **OUT-04**: A read-verb's output is a copy-paste-able input to an edit/nav verb (stable symbol locator handle). _Acceptance:_ `helix find-symbol` output feeds `helix replace-symbol-body` / `get-callers` verbatim.
- [ ] **OUT-05**: The v1.5 typed-error taxonomy is preserved on the CLI — each error kind surfaces as a stable stderr prefix plus a per-kind non-zero exit code. _Acceptance:_ each documented error kind maps to a stable prefix + exit code the agent can branch on.
- [ ] **OUT-06**: Global persistent `--json` (opt-in compact JSON lines) and `--color` (auto/always/never) flags; terse text is the default. _Acceptance:_ `--json` emits compact JSON; omitting it yields terse text; `--color=never` ≡ piped behavior.
- [ ] **OUT-07**: The path-format best practice (workspace-relative + `--abs` escape hatch) is empirically validated via the LLM behavioral oracle and the chosen anchor is documented in `SKILL.md`. _Acceptance:_ the behavioral oracle confirms agents resolve `helix`-emitted paths without error; `--abs` produces absolute paths.

### Profile / Mode Enforcement (SEC-*) — security-load-bearing

- [ ] **SEC-01**: Profile/mode filtering is enforced on `tools/call` (not only the removed `tools/list`), so a read-mode or ci-bot agent cannot invoke destructive edit verbs via the CLI. _Acceptance:_ `helix replace-symbol-body` under read mode is refused with a typed error; the same verb succeeds under edit mode.
- [ ] **SEC-02**: The CLI honors the resolved profile tool-subset (`config.ResolveProfile`) — verbs outside the active profile are hidden and refused. _Acceptance:_ per-profile goldens (re-pointed from the MCP `tools/list` goldens) verify the CLI verb surface per profile.

### Skill, Nudge & Setup (SKILL-*)

- [ ] **SKILL-01**: Ship a `SKILL.md` (embedded via `go:embed`) with frontmatter (`name` / `description` / `allowed-tools: Bash(helix:*)`) and a `| Question | Use this | Not this |` decision table; the description fires on code-navigation/edit tasks without over-firing. _Acceptance:_ the skill validates against the Claude Code skill schema; the behavioral oracle shows it triggers on code tasks and stays dormant on unrelated ones.
- [ ] **SKILL-02**: `helix setup <client>` installs the skill + hooks instead of registering an MCP server, idempotently, across the supported clients, and tears down any prior MCP registration (migration path). _Acceptance:_ `helix setup claude-code` leaves skill + hooks present and no MCP server entry; re-running is idempotent.
- [ ] **SKILL-03**: The PreToolUse nudge hook is repurposed to advisory-steer grep/sed/cat → the equivalent `helix <verb>` via `additionalContext`, exiting 0, failing open on unparseable Bash and non-code targets. _Acceptance:_ a code-symbol grep yields a `helix` suggestion; a README/log grep yields none; the hook never blocks.
- [ ] **SKILL-04**: The `SKILL.md` token-efficiency rationale is backed by a real measurement (idle skill cost vs the preloaded full-tool schema blob). _Acceptance:_ measured before/after token numbers are recorded in `SKILL.md` or a referenced doc.

### MCP Surface Retirement (RETIRE-*)

- [ ] **RETIRE-01**: The stdio MCP forwarder head is removed; agents no longer connect via stdio MCP (the forwarder dial path is retained only for CLI→daemon gRPC). _Acceptance:_ no stdio MCP server code path remains reachable; the CLI still dials the daemon.
- [ ] **RETIRE-02**: The Streamable-HTTP MCP transport (`/mcp`, `--mode http`) is removed. _Acceptance:_ the HTTP MCP endpoint is gone and `--mode http` no longer serves MCP.
- [ ] **RETIRE-03**: MCP-head removal happens only after CLI parity is proven via dual-run (strangler-fig) — a parity test compares CLI output against the pre-removal MCP path for a representative tool set. _Acceptance:_ the dual-run parity test is green in the commit immediately before the deletion commit.
- [ ] **RETIRE-04**: The retained gRPC IPC optionally binds a TCP address for split-host CLI↔daemon use (loopback/unix-socket default; non-loopback TCP opt-in, gated and documented per the v1.2 admin-addr loopback pattern). _Acceptance:_ the CLI can target a configured TCP daemon endpoint; the default remains the local unix socket.

### Docs & Identity (DOCS-*)

- [ ] **DOCS-01**: Identity rewrite — README, CLAUDE.md, and PROJECT.md ("Core Value"; Constraints "Protocol: MCP — primary interface") are rewritten to a CLI-first identity. _Acceptance:_ no doc claims MCP as the primary agent interface; the CLI-first framing is consistent across all four docs.
- [ ] **DOCS-02**: The auto-generated tool table is regenerated against the CLI surface (`cmd/docgen` enumerates verbs; docgen blank-imports stay == the daemon's). _Acceptance:_ the generated table lists `helix` verbs and the docgen drift gate is green.
- [ ] **DOCS-03**: The CLAUDE.md "tool routing" guidance is updated to reference `helix <verb>` instead of MCP tool names. _Acceptance:_ the routing matrix cites CLI verbs end-to-end.

### Test & Oracle Migration (TEST-*)

- [ ] **TEST-01**: A CLI-over-daemon end-to-end oracle exercises `helix <verb>` as a subprocess against a real daemon (reusing the v1.12 bench subprocess/sandbox patterns). _Acceptance:_ the E2E suite runs a representative verb set green under `go test`.
- [ ] **TEST-02**: The contract oracle is re-targeted — goldens become CLI stdout goldens (ordering, `file:line`, error-kind prefix); schema meta-validation becomes "typed args → cobra flags" parity. _Acceptance:_ the contract oracle passes against CLI output.
- [ ] **TEST-03**: Skill + nudge behavior is verified via the v1.4 LLM behavioral harness — confirming the skill shifts agent tool-selection toward `helix` and the nudge fires correctly. _Acceptance:_ the behavioral oracle records a tool-selection improvement vs the grep/sed/cat baseline.

## v2 Requirements (deferred to future milestones)

### Remote / Multi-Host (REMOTE-*)

- **REMOTE-01**: Authenticated non-loopback gRPC TCP transport (mTLS / token auth) for true remote daemon access beyond the gated opt-in in RETIRE-04.
- **REMOTE-02**: A multi-client fan-out story to replace the removed HTTP transport's multi-connection scenarios, if a concrete need surfaces.

## Out of Scope

| Feature | Reason |
|---------|--------|
| Excising the MCP SDK from the daemon | Internal dispatch/middleware engine stays; only the external surface is removed. ~5× the work for zero agent-visible benefit. |
| Removing the gRPC IPC layer | The daemon↔CLI wire is retained — it already carries the `tools/call` frames. |
| Re-implementing the 5 middlewares natively | Telemetry / profile-filter / suggest / lazy-init / guardrail keep running inside the daemon unchanged. |
| A compatibility MCP shim / dual MCP+CLI head | Clean retirement, not dual-head; revisit only if a concrete client need surfaces. |
| Playwright-style named sessions (`-s <name>`) | Code navigation is stateless; the warm daemon is the only state that matters. |
| Default-on JSON output | JSON is more tokens and lower fluency for an LLM reader than terse `file:line`; `--json` is opt-in. |
| A generic `helix run <tool> --json` mega-verb | Kills discoverability and reintroduces a schema blob; one explicit verb per tool instead. |
| Hard-deny-every-grep nudge | False positives on log/README/YAML greps break legitimate work; advisory-only. |
| Real authn implementation for non-loopback TCP | RETIRE-04 only gates/documents the opt-in; full auth deferred to REMOTE-01. |

## Traceability

Which phases cover which requirements. Filled in during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| (pending roadmap) | — | Pending |

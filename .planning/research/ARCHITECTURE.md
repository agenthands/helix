# Architecture Research

**Domain:** CLI head for an existing LSP-backed MCP daemon (Go); retirement of the agent-facing MCP surface
**Milestone:** v2.0 CLI-First — MCP Surface Retirement
**Researched:** 2026-06-21
**Confidence:** HIGH (grounded in the actual v1.12 codebase — every integration point below cites a real file/line)

> Scope note: This document answers "how does the v2.0 CLI head integrate with the existing daemon/gRPC/MCP-SDK internals, and what is the safe build order?" It is explicit about NEW / MODIFIED / DELETED code so the roadmapper can carve phases. The architecture decision is already locked in PROJECT.md (lines 173-199): kill MCP hat #1 (agent-facing transport), keep MCP hat #2 (internal dispatch + middleware engine).
>
> This file replaces a stale v1.12 bench-milestone ARCHITECTURE.md (dated 2026-06-13) that predated the v2.0 start.

---

## Standard Architecture

### System Overview — today (v1.12) vs target (v2.0)

```
TODAY (v1.12) — two agent-facing MCP heads
┌──────────────────────────────────────────────────────────────────────┐
│  AGENT / IDE                                                           │
│   │  (A) stdio MCP            │  (B) Streamable-HTTP MCP               │
│   ▼                           ▼                                        │
│  helix (no-arg) ─► forwarder  helix --mode http ─► daemon.listenHTTP   │
│   │  RunForwarder()            │   /mcp  (mcpServer.HTTPHandler())     │
│   ▼  StreamMCP gRPC stream     ▼                                       │
├───┴────────────────────────────┴──────────────────────────────────────┤
│  DAEMON (persistent)                                                   │
│   forwarderServiceHandler.StreamMCP ─► GRPCTransport ─► mcpServer.SDK  │
│   middleware: LazyInit→Guardrail→Suggest→ProfileFilter→Telemetry       │
│   51 mcpsdk.AddTool handlers ─► kernel (LSP pool, RepoMap, edits)      │
└───────────────────────────────────────────────────────────────────────┘

TARGET (v2.0) — one agent-facing head: the CLI
┌──────────────────────────────────────────────────────────────────────┐
│  AGENT (via universal Bash tool, taught by SKILL.md + nudge hook)     │
│   │  helix <verb> --flags                                             │
│   ▼                                                                   │
│  CLI subcommand (NEW, ~code-generated from tool registry)             │
│   1. ConnectOrStartDaemon()  ← REUSED warm-daemon dial                │
│   2. one StreamMCP stream: initialize → tools/call → read result      │
│   3. render MCP content blocks → terse file:line text (NEW renderer)  │
│   4. exit                                                             │
├───────────────────────────────────────────────────────────────────────┤
│  DAEMON (UNCHANGED behind the wire)                                    │
│   StreamMCP ─► GRPCTransport ─► mcpServer.SDK                         │
│   middleware stack UNCHANGED ─► 51 tool handlers ─► kernel            │
│   [DELETED: listenHTTP /mcp head, mcpServer.RunStdio]                 │
└───────────────────────────────────────────────────────────────────────┘
```

### Component Responsibilities (v2.0)

| Component | Responsibility | New/Modified/Deleted | File anchor |
|-----------|----------------|------------------------|-------------|
| CLI verb subcommands | One cobra command per callable tool; parse flags → `tools/call` args | **NEW** (largely generated) | `internal/cli/tools_gen.go` (new) |
| One-shot MCP client | Open stream, `initialize`+`tools/call`, read one result, exit | **NEW** | `internal/cli/toolcall.go` (new) or `internal/forwarder/oneshot.go` |
| `ConnectOrStartDaemon` | Auto-start + warm-reuse the daemon over gRPC | **REUSED as-is** | `internal/forwarder/dial.go:24` |
| Output renderer | MCP `[]Content` → terse, greppable `file:line` text | **NEW** | `internal/cli/render/` (new) |
| `forwarderServiceHandler.StreamMCP` | Bridge gRPC stream ↔ SDK session | **UNCHANGED** | `internal/daemon/daemon.go:1442` |
| `GRPCTransport` | gRPC stream ↔ `mcpsdk.IOTransport` pipe bridge | **UNCHANGED** | `internal/mcp/grpc_transport.go` |
| Middleware stack (5) | telemetry/profile-filter/suggest/lazy-init/guardrail | **UNCHANGED** (run inside daemon) | `internal/daemon/daemon.go:828-942` |
| `mcpServer.RunStdio` | stdio MCP transport for daemon-direct mode | **DELETED** | `internal/mcp/server.go:191` |
| `daemon.listenHTTP` + `/mcp` | Streamable-HTTP MCP head | **DELETED** | `internal/daemon/daemon.go:1360`; `http_session_middleware.go` |
| `runForwarder` no-arg path | no-arg `helix` = stdio MCP forwarder | **DELETED / REPURPOSED** | `internal/cli/root.go:113-114` |
| `helix setup <client>` | register MCP server → install skill + hooks | **MODIFIED** | `internal/cli/setup*.go` |
| `helix nudge` | PreToolUse steer grep/sed/cat → `helix <verb>` | **MODIFIED** | `internal/cli/nudge.go:99` |
| `cmd/docgen` | regenerate tool table against CLI surface | **MODIFIED** | `cmd/docgen/main.go` |

---

## Q1 — One-shot tool invocation: reuse forwarder auto-start, and zero-proto-change feasibility

### The reusable seam already exists

`forwarder.ConnectOrStartDaemon(ctx, socketPath, logger, tp)` (`internal/forwarder/dial.go:24`) is the warm-daemon dial: it `tryConnect`s to the unix socket, and on miss spawns `helix --serve --socket=…` detached and polls up to 10s for readiness (`dial.go:34-39,82-100`). **It is already reused by `helix activate` (`internal/cli/activate.go:53`) and `helix status`** with a noop tracer — those commands prove the CLI→daemon path with no MCP at all. The new tool subcommands reuse the *same* function verbatim. No new auto-start logic is needed.

### Zero-proto-change one-shot — RECOMMENDED, and feasible

The existing `StreamMCP(stream MCPMessage)` RPC carries raw MCP JSON-RPC frames (`api/proto/serena/v1/ipc.proto:12`; `MCPMessage.payload` = raw bytes, `:27`). A one-shot CLI call is a *degenerate forwarder session*:

1. `ConnectOrStartDaemon` → `client.StreamMCP(ctx)`.
2. Send `initialize` request frame, read `initialize` response. The daemon side runs the full MCP handshake inside `mcpServer.SDK().Connect` — `defaultSessionRunner` (`daemon.go:1476-1492`) does exactly this for the forwarder; the daemon cannot tell a CLI stream from a forwarder stream.
3. Send one `tools/call` frame `{name, arguments}`; read the matching response frame.
4. `CloseSend()`, drain, exit.

Every frame flows through the **unchanged** server path: `StreamMCP` (`daemon.go:1442`) → `GRPCTransport.Connect` io.Pipe bridge (`grpc_transport.go:44`) → `IOTransport` → SDK → middleware chain → tool handler. The CLI is just a *minimal MCP client* speaking the same wire the forwarder speaks. **Requires ZERO ipc.proto changes and zero daemon changes.**

**Cost of zero-proto:** the CLI must do the MCP `initialize` handshake + JSON-RPC framing + request/response id-matching. Two clean ways to avoid hand-rolling:
- **(Preferred) Use the MCP Go SDK *client* against a client-side `GRPCTransport`.** `mcpsdk.NewClient(...).Connect(ctx, transport, nil)` yields `session.CallTool(ctx, &CallToolParams{Name, Arguments})`, handling handshake + id-matching for free. The CLI builds the client side of a `GRPCTransport` (mirror of the daemon's `grpc_transport.go`; the forwarder's existing send/recv loop, `forwarder.go:64-123`, refactored into a reusable transport supplies the gRPC↔pipe plumbing). This keeps the MCP SDK as *internal plumbing on the CLI side too* — consistent with "keep the SDK, remove the external surface."
- Hand-roll a 3-message JSON-RPC exchange. Smaller dependency surface but re-implements handshake/id-matching the SDK already gives.

### Unary `CallTool` RPC — NOT warranted now (assessed)

A new `rpc CallTool(CallToolRequest) returns (CallToolResult)` would be ergonomically nicer (no per-call handshake) but:
- It would **bypass the 5-middleware chain** (telemetry/lazy-init/guardrail/suggest, installed `daemon.go:828-942`) unless re-invoked behind the unary handler — i.e. rebuild the dispatch the SDK already does. PROJECT.md line 196 keeps those middlewares running; a parallel unary path risks them silently not applying.
- It is a **proto + generated-code + new server-handler** change, versus zero for the streaming reuse.
- The handshake "cost" is one extra round-trip on an *already-warm unix socket* — sub-millisecond; dominant latency is the tool's LSP work, identical either way.

**Verdict:** Ship v2.0 with the **zero-proto streaming one-shot via the SDK client**. Revisit a unary `CallTool` only if profiling shows the per-call `initialize` handshake is a measurable tax against a warm daemon (it won't be). Confidence: HIGH.

---

## Q2 — Where does output formatting live

### Principle: formatting is a CLI-layer concern, NEVER in the daemon

Tool handlers return `*mcpsdk.CallToolResult{ Content: []mcpsdk.Content{ &TextContent{Text: …} } }` (e.g. `server.go:114-119`, `AddSkillTool` `server.go:262-266`). The daemon stays **content-shape neutral**: it must keep returning structured content so non-CLI consumers (InMemory + HTTP test oracles, the eval harness) are not coupled to terminal formatting. Putting `file:line` prettifying in the daemon would pollute the kernel with presentation concerns and break the existing golden/contract oracles.

### Recommended: a shared renderer package with per-tool render funcs

```
internal/cli/render/          (NEW)
├── render.go      # Render(toolName string, result *mcpsdk.CallToolResult) (string, error)
├── content.go     # generic fallback: flatten TextContent; IsError → stderr + nonzero exit
├── locations.go   # shared "path:line:col  symbol  kind" formatter (the load-bearing terseness)
└── tools/         # per-tool overrides keyed by tool name (only where the generic isn't terse enough)
```

- **One dispatch point**, keyed by tool name, with a **generic fallback** that flattens `TextContent` and maps `IsError==true` (set by tool handlers, e.g. `server.go:161`, `:258`) to stderr + non-zero exit. Most tools work via the fallback + the shared `locations.go` formatter; only a handful (RepoMap tree, blast-radius, diagnostics) need bespoke renderers.
- **Why shared, not purely per-command:** the terse `file:line`-anchored grammar (ripgrep/ast-grep ergonomics, PROJECT.md line 185) is a *product invariant* that must be consistent across all 51 verbs; one `locations.go` enforces it. Per-command logic is reserved for genuinely tool-specific shapes.
- This package is the **load-bearing product work** (PROJECT.md line 185) — it deserves its own phase and golden-output tests (mirror the existing golden pattern used for profile contracts).

**Anti-pattern to avoid:** adding a `--format` field to `tools/call` args or a render hint to the proto. Rendering must not cross the wire. Confidence: HIGH.

---

## Q3 — Profile/mode + middleware once the MCP `tools/list` surface is gone

### Critical finding: ProfileFilterMiddleware ONLY touches `tools/list`

`ProfileFilterMiddleware` (`middleware.go:501`, early-returns for `method != "tools/list"` at `:509`) and the explicit comment at `middleware.go:165`: *"ProfileFilterMiddleware only filters tools/list in v1.2; there is no rejection path at tools/call."* So today **profile filtering is advisory** — it shapes what the client is *told* it may call, but the daemon does NOT reject a `tools/call` for a tool outside the active profile. Brief-descriptions and profile description-overrides also piggyback on the same `tools/list` pass (CLAUDE.md middleware notes; `middleware.go:498-501`).

This is the single most important consequence of removing the MCP surface: **with no `tools/list`, the daemon-side profile filter becomes a no-op for the CLI path.** If profile/mode is to be preserved, the CLI must own it.

| Middleware | Fires on | Still meaningful under CLI `tools/call`? | Action |
|------------|----------|------------------------------------------|--------|
| **LazyInit** (`lazy_init.go`, installed last → runs first) | every `tools/call` | **YES — essential.** Activates the workspace on first call (`daemon.go:902-942`), serializes concurrent first calls. The CLI one-shot relies on this to warm the kernel. | Keep unchanged |
| **Telemetry** (`middleware.go:331`, fires when `method=="tools/call"` `:340`) | `tools/call` | **YES.** RED metrics, per-tool deadline via `BudgetFunc`, outcome classification still apply per CLI call. | Keep unchanged |
| **Guardrail** (`guardrail_middleware.go`) | `tools/call` | **YES.** Receipt/rule enforcement is call-time; unaffected by surface removal. | Keep unchanged |
| **Suggestion** (`suggest.go`) | error responses on `tools/call` | **YES, but** enriches the MCP error *content*; the CLI renderer must surface that as stderr text. | Keep; renderer prints it |
| **ProfileFilter** (`middleware.go:501`) | `tools/list` only | **NO for filtering** (no tools/list) and its description-override work also only runs on tools/list. | Becomes inert for the CLI path |

### Where profile/mode enforcement must move

The profile tool-subsets live in `Profile.Tools` (`internal/profile/profile.go:30`) and `config.Tools` (`internal/config/config.go:131,139`), resolved by `config.ResolveProfile` (`internal/config/loader.go:82`). Two options:

1. **CLI-side gate (recommended).** At subcommand registration / dispatch, resolve the active profile via `config.ResolveProfile` and **omit (or refuse) verbs not in the profile's allowed set + current mode.** This mirrors exactly what `tools/list` filtering did for the agent — the agent now "sees" only the verbs the CLI exposes. Keeps enforcement at the same conceptual layer (the surface the agent touches), needs no daemon change. `switch_mode` becomes `helix switch-mode` writing session/config state the CLI reads.
2. **Promote filtering into a `tools/call` rejection middleware** in the daemon. More invasive (new daemon code, changes call-time semantics for *all* clients), explicitly larger than this milestone wants. Only if defense-in-depth against a hand-typed out-of-profile `helix` call is a hard requirement.

**Verdict:** Enforce profile/mode in the **CLI layer** (option 1), reusing `config.ResolveProfile`. The daemon middleware stack stays byte-for-byte unchanged; `ProfileFilterMiddleware` remains installed (harmless) but dormant for the agent path. Document that the daemon does NOT enforce profile at `tools/call` — the CLI is the enforcement boundary. Confidence: HIGH.

---

## Q4 — Exactly what is removed vs retained

### DELETED (agent-facing MCP surface — hat #1)

| Deleted | File anchor | Notes |
|---------|-------------|-------|
| Streamable-HTTP MCP head | `daemon.listenHTTP` + `/mcp` mux `daemon.go:1360-1392`; `mcpServer.HTTPHandler()` `server.go:196`; `internal/daemon/http_session_middleware.go` | Removes the `--mode http` agent transport + `--http-addr`. |
| stdio MCP transport (daemon-direct) | `mcpServer.RunStdio` `server.go:191` | The forwarder path used `GRPCTransport`, not this; safe to delete. |
| no-arg / `--mode stdio` forwarder MCP head | `runForwarder` dispatch `root.go:113-114`; the stdin↔stdout MCP pump in `forwarder.RunForwarder` `forwarder.go:22-131` | Retire the *stdio MCP head*. **Reuse** `ConnectOrStartDaemon` + the gRPC send/recv loop by refactoring them into the new one-shot transport; **delete** only the stdin/stdout MCP-framing wrapper. |
| `helix setup` MCP-registration registrars | `internal/cli/setup_clients.go` (`claude mcp add-json`-style subprocess) | Replaced by skill+hooks install (below). |

### RETAINED (internal plumbing — hat #2)

- The **daemon process** + bootstrap (`daemon.go`), the **gRPC IPC** (`ForwarderService`, all 4 RPCs — `StreamMCP` now carries CLI one-shots; `GetStatus`/`ActivateWorkspace`/`DeactivateWorkspace` still serve `helix status/activate/deactivate`, `daemon.go:1494-1538`).
- `forwarderServiceHandler.StreamMCP` + `GRPCTransport` + `mcpServer.SDK()` dispatch + **all 5 middlewares** + all 51 `mcpsdk.AddTool` registrations.
- `ConnectOrStartDaemon` auto-start/warm-reuse; `--serve` flag (now internal, used by `dial.go:89`).

### What replaces no-arg `helix`

Today no-arg `helix` launches the stdio forwarder (`root.go:50` comment, `:98-118` dispatches to `runForwarder`). Options, in recommendation order:
1. **Print help/usage** (cobra default once `RunE: runRoot` is removed). Cleanest; matches `git`/`kubectl`/`playwright`. The agent never calls bare `helix`; it calls `helix find-symbol …`.
2. Alias bare `helix` to `helix status` (least surprising for a human poking the daemon).

`--serve` is **retained** (the forwarder/one-shot spawns the daemon via `helix --serve --socket=…`). `--mode` / `--http-addr` are removed from the public surface.

### How `helix setup <client>` flips to skill+hooks install

Half the skeleton already exists for Claude Code: `setup_hooks.go:28-69` installs SessionStart(`activate`)/PreToolUse(`nudge`)/Stop(`deactivate`) hooks. The flip:
- **Remove** the MCP-server registration step (`ClientRegistrar.Register` subprocess in `setup_clients.go`).
- **Add** a `SKILL.md` install step: write/symlink the progressively-disclosed `SKILL.md` (NEW authored asset) into the client's skill location, plus the nudge/activate/deactivate hooks (present for Claude Code; generalize the installer to other hook-capable clients, degrade to "SKILL.md only" elsewhere).
- **Keep** language detection + LS pre-install (`setup_detect.go`, `setup.go:113-138`) — unchanged value.
- `helix setup` becomes "install skill + hooks (+ pre-warm language servers)" rather than "register MCP server."

The **nudge hook** (`nudge.go`) is repurposed: today it nudges *toward* MCP symbolic tool names (`helixSymbolicTools` map `nudge.go:158-168`; message `nudge.go:99`); v2.0 rewrites the suggestion text to redirect `grep`/`sed`/`cat` Bash calls to the equivalent `helix <verb>` (PROJECT.md line 187). `isGrepReadTool` (`nudge.go:186-198`) already detects the Bash grep/find/rg/ag cases — the change is the *suggested replacement text* and the detection set. Confidence: HIGH.

---

## Q5 — Suggested build order (phases continue from 90) + docgen regen point

Dependency-ordered, strangler-fig (same pattern v1.10 used for the semantic store): each phase ships behind the still-live MCP surface; **the MCP heads are deleted LAST**, after the CLI proves parity.

```
Phase 90  CLI one-shot invocation spine (NEW)
          - Refactor forwarder send/recv into a reusable client-side GRPCTransport seam
          - ConnectOrStartDaemon reuse + MCP SDK *client* + initialize→tools/call→read→exit
          - 2-3 hand-wired verbs end-to-end (e.g. find-symbol, goto-definition) as proof
          - ZERO proto change; daemon untouched; MCP heads still live (dual-run)
          DEPENDS: nothing new. GATE: a real CLI call returns a tool result from the warm daemon.

Phase 91  Code-generated verb wiring + profile/mode CLI gate (NEW + MODIFIED)
          - Generate one cobra subcommand per tool from mcpServer.CollectToolSchemas()
            InputSchema + the typed Args structs (the SDK populates InputSchema; suggest.go
            BuildToolSchemaMap at suggest.go:26-38 proves it's introspectable)
          - CLI-side profile/mode enforcement via config.ResolveProfile (Q3 option 1)
          DEPENDS: 90. GATE: all 51 verbs dispatch; out-of-profile verbs refused at the CLI.

Phase 92  Terse output renderer (NEW) — the load-bearing product phase
          - internal/cli/render/ : shared locations.go + per-tool overrides + golden tests
          - IsError → stderr + nonzero exit; surface Suggestion-middleware enrichment
          DEPENDS: 91 (needs verbs to render). GATE: golden file:line output for every verb.

Phase 93  SKILL.md authoring + nudge-hook repurpose + setup flip (NEW + MODIFIED)
          - Author progressively-disclosed SKILL.md
          - nudge.go: redirect grep/sed/cat → helix <verb>
          - setup*.go: drop MCP registration, install skill+hooks across clients
          DEPENDS: 92 (skill teaches the real terse commands). GATE: helix setup claude-code
          installs skill+hooks, no MCP server; nudge points to helix verbs.

Phase 94  Retire the agent-facing MCP surface (DELETED)
          - Delete daemon.listenHTTP + /mcp + http_session_middleware + HTTPHandler
          - Delete RunStdio + the stdio-MCP forwarder framing; repurpose no-arg helix → help/status
          - Remove --mode/--http-addr public flags; keep --serve as internal
          - Middlewares + StreamMCP + GRPCTransport + 51 tools all RETAINED
          DEPENDS: 90-93 (CLI must be the proven sole surface first). GATE: no MCP head
          listens; CLI parity verified; existing daemon/kernel tests green.

Phase 95  Identity & docs rewrite + docgen regen (MODIFIED)
          - README / CLAUDE.md / PROJECT.md Constraints ("Protocol: MCP — primary interface"
            → CLI-first); Core Value rewrite
          - REGENERATE cmd/docgen tool table against the CLI surface (see below)
          DEPENDS: 94 (docs describe the final shape). GATE: docgen --check clean.
```

### Where `cmd/docgen` must be regenerated — and a required change

`cmd/docgen/main.go` builds the README tool table from `skill.ToolProviders()` via blank imports (`cmd/docgen/main.go:21-36`). Two facts the roadmapper must encode:
1. **Regen happens in Phase 95**, after the surface is final: run `go run ./cmd/docgen` then commit — the table is auto-generated, never hand-edited (CLAUDE.md: "do not hand-edit the tool table"). The `--check` mode is the CI gate.
2. **docgen's import set MUST equal the daemon's** (MEMORY "helix-tool-docs-drift": the prior bug was docgen missing the `internal/skill/semantic` blank import, drifting 53-vs-47 tool counts). If any tool package is added/removed during 90-94, mirror it in both `cmd/docgen/main.go` and `internal/daemon/imports.go`. The table semantics also shift from "MCP tool / profile / mode" to "CLI verb / profile / mode" — docgen's `generateToolTable` renderer (`cmd/docgen/main.go:56`) likely needs a column/header update, not just a regen.

---

## Anti-Patterns (specific to this milestone)

### Anti-Pattern 1: Excising the MCP SDK
**What people do:** read "rip out MCP" as "remove the SDK." **Why wrong:** PROJECT.md line 179 locks this — the SDK is the daemon's *dispatch + middleware engine* (51 `AddTool` + 5 middlewares); excising it is ~5× work for zero agent-visible benefit. **Instead:** remove only the two agent-facing transports (HTTP `/mcp`, stdio forwarder framing); keep `mcpServer.SDK()` driving `StreamMCP`.

### Anti-Pattern 2: A unary `CallTool` proto RPC to "simplify" one-shots
**What people do:** add `rpc CallTool(...)` to feel cleaner. **Why wrong:** bypasses the 5-middleware chain (re-implementing dispatch), needs proto/codegen changes, saves only a sub-ms handshake on a warm socket. **Instead:** stream one `tools/call` over the existing `StreamMCP` via the MCP SDK *client* (Q1).

### Anti-Pattern 3: Rendering in the daemon / over the wire
**What people do:** add a `--format` arg or render hint to `tools/call`. **Why wrong:** couples the kernel to presentation, breaks the InMemory/HTTP test oracles, leaks formatting across the IPC boundary. **Instead:** daemon returns structured `[]Content`; `internal/cli/render/` formats (Q2).

### Anti-Pattern 4: Assuming the daemon enforces profile at call time
**What people do:** drop CLI-side profile gating, trusting the daemon's ProfileFilter. **Why wrong:** ProfileFilter only filters `tools/list`, which no longer exists for the CLI — call-time is unenforced (`middleware.go:165`). **Instead:** gate profile/mode in the CLI via `config.ResolveProfile` (Q3).

### Anti-Pattern 5: Deleting MCP heads before CLI parity
**What people do:** remove `listenHTTP`/forwarder framing early. **Why wrong:** loses the dual-run safety net; a CLI gap becomes a regression with no fallback. **Instead:** strangler-fig — heads stay live through Phases 90-93, deleted only in 94.

---

## Integration Points

### Internal Boundaries

| Boundary | Communication | Notes |
|----------|---------------|-------|
| CLI ↔ daemon (tool call) | gRPC `StreamMCP` stream, raw MCP JSON-RPC frames (`ipc.proto:12`) | **UNCHANGED wire.** CLI is a minimal MCP client; daemon can't distinguish it from the forwarder. |
| CLI ↔ daemon (workspace/status) | gRPC unary `GetStatus`/`ActivateWorkspace`/`DeactivateWorkspace` | **UNCHANGED.** Already used by `helix status/activate/deactivate`. |
| CLI ↔ daemon (auto-start) | `ConnectOrStartDaemon` spawns `helix --serve --socket=…` | **REUSED** (`dial.go:24,89`). `--serve` retained as internal flag. |
| daemon ↔ kernel | in-process; SDK dispatch → tool handlers → kernel | **UNCHANGED.** Warm LSP pool, RepoMap, edits all intact. |
| CLI render ↔ tool result | in-process; `*mcpsdk.CallToolResult.Content` → terse text | **NEW** `internal/cli/render/`. Must not cross the wire. |
| CLI ↔ profile/config | `config.ResolveProfile` (`config/loader.go:82`) | **NEW use** — CLI becomes the profile-enforcement boundary. |
| client ↔ helix (steering) | `SKILL.md` + nudge hook (`nudge.go`) | **NEW asset + MODIFIED hook.** |

### External Reference

| Reference | Pattern | Notes |
|-----------|---------|-------|
| `github.com/microsoft/playwright-cli` | CLI + `SKILL.md` taught via Bash tool | Playwright keeps MCP *alongside* the CLI; Helix goes further and retires the agent-facing MCP head. The CLI+SKILL.md+progressive-disclosure pattern and terse-output ergonomics are the borrowable parts. |

---

## Scaling Considerations

| Scale | Architecture adjustment |
|-------|--------------------------|
| Single agent, occasional calls | Per-call `initialize` handshake on a warm socket is negligible; default path. |
| High CLI call rate (tight agent loop) | If handshake overhead ever shows in profiling, consider a unary `CallTool` (Q1) OR a short-lived client connection cache — but only then. The warm daemon + share-until-dirty LS pool is the real performance asset and is untouched. |
| Many concurrent CLI invocations | LazyInit serializes first-call activation per workspace; each one-shot is its own gRPC stream/session — already the forwarder's concurrency model. |

---

## Sources

- `/.planning/PROJECT.md` lines 173-199 (v2.0 locked architecture decision) — HIGH
- `internal/forwarder/dial.go:24,34-39,82-100` (`ConnectOrStartDaemon`, `--serve` spawn) — HIGH
- `internal/forwarder/forwarder.go:22-131` (stdio MCP pump to retire; gRPC send/recv loop to reuse) — HIGH
- `internal/mcp/grpc_transport.go:25-130` (`GRPCTransport` bridge, retained) — HIGH
- `internal/mcp/server.go:191,196,262-266` (`RunStdio`, `HTTPHandler` deleted; content shape) — HIGH
- `internal/mcp/middleware.go:133,165,331,340,498-501,509` (5-middleware install; ProfileFilter tools/list-only; telemetry on tools/call) — HIGH
- `internal/mcp/suggest.go:26-38` (proves `mcpsdk.Tool.InputSchema` is introspectable for verb codegen) — HIGH
- `internal/daemon/daemon.go:828-942,1360-1392,1442-1538` (middleware install order, listenHTTP, StreamMCP, defaultSessionRunner, unary RPC handlers) — HIGH
- `internal/cli/root.go:44-118,134-206` (no-arg/forwarder/daemon dispatch, flags to remove) — HIGH
- `internal/cli/activate.go:53` (ConnectOrStartDaemon CLI reuse precedent) — HIGH
- `internal/cli/nudge.go:99,158-198` (nudge repurpose point), `internal/cli/setup.go:113-138`, `internal/cli/setup_hooks.go:28-69` (setup flip) — HIGH
- `internal/profile/profile.go:30`, `internal/config/config.go:131,139`, `internal/config/loader.go:82` (profile tool-subsets + ResolveProfile) — HIGH
- `cmd/docgen/main.go:21-36,56` + MEMORY "helix-tool-docs-drift" (regen point + import-parity requirement) — HIGH
- `api/proto/serena/v1/ipc.proto:9-30` (StreamMCP + unary RPCs; zero-change feasibility) — HIGH

---
*Architecture research for: CLI head over an existing MCP daemon; agent-facing MCP surface retirement (v2.0)*
*Researched: 2026-06-21*

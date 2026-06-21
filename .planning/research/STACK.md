# Stack Research

**Domain:** CLI head for an existing Go daemon — 53-subcommand cobra CLI + agent SKILL.md, driving the unchanged MCP-SDK/gRPC daemon kernel via one-shot `tools/call`.
**Researched:** 2026-06-21
**Confidence:** HIGH

> **TL;DR for the roadmapper:** This milestone needs **essentially zero new third-party dependencies.** Every load-bearing capability — cobra subcommands at scale, gRPC `StreamMCP` dialing with daemon autostart, JSON-RPC framing, struct→JSON-schema reflection — already ships in the tree and is already exercised. The real work is (1) a `go:generate` codegen that turns the existing typed-arg structs into ~53 cobra subcommands, (2) a one-shot MCP-over-gRPC client adapter (a ~150-LOC refactor of the existing forwarder loop), (3) terse per-tool output formatters, and (4) a `SKILL.md` asset + a `helix setup` flip. The stack recommendation is therefore mostly "reuse what's vendored; add one tiny generator; do not add a TUI framework or a second RPC layer."

---

## Recommended Stack

### Core Technologies (all ALREADY in `go.mod` — reuse, do not re-add)

| Technology | Version (in tree) | Purpose for this milestone | Why it fits a single-binary Go daemon |
|------------|-------------------|----------------------------|----------------------------------------|
| `github.com/spf13/cobra` | **v1.10.2** | The 53-subcommand CLI head, grouped help, per-command flags | Already the root-command framework (`internal/cli/root.go`); `cmd/helix-bench` already runs 6 subcommands. v1.6+ has `AddGroup`/`GroupID` for grouped help — exactly what 53 verbs need. No new dep. |
| `github.com/modelcontextprotocol/go-sdk` | **v1.5.0** | Stays as the daemon's *internal* dispatch+middleware engine; its bundled `jsonschema` reflector is the codegen source of truth | Architecture decision is locked: kill the agent-facing surface, keep the SDK as hat #2. The SDK's `jsonschema` package (struct→schema via reflection, `For[T]`) lets the generator read the same arg structs the daemon registers — zero schema drift. Latest upstream is v1.6.1 (2025-05); v1.5.0 in-tree is current enough and bumping is orthogonal to this milestone. |
| `google.golang.org/grpc` | **v1.80.0** | The CLI→daemon wire (`StreamMCP` bidi stream) — unchanged | The daemon↔CLI wire already carries `tools/call` frames. `internal/forwarder/dial.go` already dials over `unix://` with keepalive + otelgrpc. The CLI reuses this verbatim. **No proto change expected** (PROJECT.md: "likely zero proto changes"). |
| `google.golang.org/protobuf` | **v1.36.11** | `MCPMessage{payload,session_id}` envelope — unchanged | The `serena.v1.MCPMessage` wrapper already carries opaque JSON-RPC bytes; the CLI sends one request frame and reads one response frame. |
| `github.com/knadh/koanf/v2` | **v2.3.4** | Socket-path / profile resolution for the CLI process | Already the 4-layer config engine; CLI subcommands resolve the socket via the same `config.DefaultSocketPath()` the forwarder uses. |
| `encoding/json` (stdlib) | Go **1.25.1** | JSON-RPC request build + response parse in the one-shot client | The forwarder already treats payloads as opaque bytes and sniffs `"method":"tools/call"` with a byte match (`internal/forwarder/forwarder.go:151`). The CLI marshals one request and unmarshals one `CallToolResult`. No JSON-RPC library needed. |

### Supporting Libraries / Internal Packages (reuse-don't-fork)

| Library / package | Version / location | Purpose | When to use |
|-------------------|--------------------|---------|-------------|
| `internal/forwarder` (`ConnectOrStartDaemon`, `tryConnect`, `startDaemon`, `waitForDaemon`) | in-tree | Daemon autostart + warm reuse for every CLI invocation | **The single most important reuse.** `dial.go` is already the gopls-pattern "connect-or-spawn" logic with a 10s readiness poll. The CLI's one-shot client wraps `ConnectOrStartDaemon` then opens one `StreamMCP`. Do **not** write a second autostart. |
| `internal/mcp` `ToolRegistry` + `ToolDef` | in-tree | Enumerate the 53 tools (names, brief/help text) for the generator and for grouped-help text | `ToolDef{Name,Description,BriefDescription,HelpText}` is the existing per-tool metadata; `cmd/docgen` already walks `skill.ToolProviders()` to build the README tool table — the generator reuses that exact enumeration. |
| `<sdk>/jsonschema` (bundled in go-sdk v1.5.0) | in-tree (transitive) | Reflect each `XxxArgs` struct → JSON schema → cobra flags (name, type, required, help) in the generator | The arg structs already carry `json:"..."` + `jsonschema:"description"` tags (e.g. `fileops/tools.go:22`). One reflection pass yields flag name, Go type→pflag type, required-ness, and `--help` text — no hand-written flag wiring per command. |
| `text/template` (stdlib) | Go 1.25.1 | Emit the generated `*_cli_gen.go` subcommand file from the registry | Mirrors the existing `cmd/lspgen` codegen pattern (`protocol/generate.go` `//go:generate go run ../cmd/lspgen`). |
| `os` / `golang.org/x/term` (only if needed) | stdlib + already transitive | TTY detection for `--color=auto` default-off-when-piped | Most CLI invocations from an agent are piped → color must default off. `os.Getenv("NO_COLOR")` + `term.IsTerminal(fd)` is the whole policy; see Output section. **Prefer stdlib** — do not add a color framework. |

### Development Tools

| Tool | Purpose | Notes |
|------|---------|-------|
| `//go:generate go run ./cmd/helix-cligen` (new, tiny) | Generate the 53 subcommand wrappers from the tool registry at build time | Models the existing `protocol/generate.go` → `cmd/lspgen` precedent. Output is committed `*_gen.go` (like `protocol/gen/`), with a `--check` CI mode mirroring `cmd/docgen --check` to fail on drift. |
| `cmd/docgen` (existing, extend) | Regenerate the README tool table against the CLI surface | PROJECT.md target feature: "auto-generated tool table regenerated against the CLI surface." docgen already enumerates the same providers; extend, don't replace. |
| `gofmt` / `go vet ./...` | Format + vet generated code | Generator output must pass `gofmt -w` and `go vet` (project rule: "Always run go vet and go test before completing any Go task"). |

---

## Code-Generation Feasibility for 53 Commands (concrete assessment)

**Verdict: HIGHLY FEASIBLE — the registry already holds everything the generator needs.** This is the load-bearing technical question and the answer is a clear yes.

**What the generator reads (all already in-tree):**
1. `skill.ToolProviders()` → the list of tools (same source `cmd/docgen` uses).
2. Per tool: `ToolDef{Name, BriefDescription, HelpText}` for the cobra `Use`/`Short`/`Long`.
3. Per tool: the typed `XxxArgs` struct, whose fields already carry `json:"name,omitempty"` + `jsonschema:"help text"` tags. Reflection (or the SDK's `jsonschema.For[T]`) yields, per field: flag name (`json` tag), Go type → pflag type (`String`/`Int`/`Bool`/`StringSlice`), required-ness (absence of `,omitempty`), and `--help` string (`jsonschema` tag).

**What the generator emits** (one `*_cli_gen.go`), per tool, roughly:

```go
// Code generated by cmd/helix-cligen. DO NOT EDIT.
func newGoToDefinitionCmd(dial DialFunc) *cobra.Command {
    var args symbols.GoToDefinitionArgs
    cmd := &cobra.Command{
        Use:     "go-to-definition",
        Short:   "Jump to where a symbol is defined",      // from BriefDescription
        Long:    goToDefinitionHelp,                        // from HelpText
        GroupID: "symbols",                                 // grouped help
        RunE: func(c *cobra.Command, _ []string) error {
            return runTool(c.Context(), dial, "go_to_definition", args)
        },
    }
    cmd.Flags().StringVar(&args.Path, "path", "", "File path ...")        // from struct tags
    cmd.Flags().IntVar(&args.Line, "line", 0, "1-indexed line")
    cmd.Flags().IntVar(&args.Col,  "col",  0, "1-indexed column")
    _ = cmd.MarkFlagRequired("path")
    return cmd
}
```

`runTool` marshals `args` into a JSON-RPC `tools/call` request, sends it through the one-shot gRPC client, and hands the `CallToolResult` to the tool's terse formatter.

**Two viable codegen mechanics — pick one in the roadmap:**

| Approach | How | Tradeoff |
|----------|-----|----------|
| **A. Source-reflection at generate time** (recommended) | `cmd/helix-cligen` imports the kernel/skill packages (blank-import style like docgen/daemon), reads the registry + reflects the arg structs, emits Go via `text/template`. Committed output, `--check` CI gate. | Compile-time-safe wrappers, zero per-call reflection, greppable generated code. Matches `lspgen`/`docgen` precedent exactly. **This is the idiomatic Go answer.** |
| **B. Fully dynamic at runtime** | Build `*cobra.Command`s in a loop at `init()` from the live registry; bind flags via a generic `map[string]any` populated by reflection each call. | No generated file, but per-call reflection, weaker `--help`/typing, harder to grep, and it fights cobra's static-flag model. Use only if the tool set were truly dynamic — it isn't (53 compiled-in tools). |

**Grouped help** (53 verbs is too many for a flat list): cobra `AddGroup(&cobra.Group{ID:"symbols",Title:"Symbol Intelligence"})` + `GroupID` on each generated command. Natural groups already exist in the codebase layout: symbols / edit / fileops / diag / memory / repomap / semantic / workflow.

**Caveat for the generator:** the typed-arg structs are registered per-tool via individual `mcpsdk.AddTool` calls (e.g. `registerGoToDefinition` in `symbols/tools.go`), so the link from tool *name* → arg *struct type* is not stored in `ToolDef` today. The generator needs that mapping. Two options: (a) add a `ArgsExample any` (or `reflect.Type`) field to `ToolDef` populated at registration, or (b) keep a small generator-side name→type table. Option (a) is cleaner and makes the registry self-describing; flag it as a small enabling change in the roadmap.

---

## One-Shot `tools/call` Over the Existing Bidi `StreamMCP` (concrete pattern)

**The problem shape:** a short-lived CLI process must do exactly one request→one response over a streaming RPC, then exit. The existing forwarder runs an *infinite* stdin↔stream pump; the CLI needs a *bounded* single exchange.

**Minimal client (reuses `forwarder.ConnectOrStartDaemon`):**

```go
func runTool(ctx context.Context, socket string, toolName string, args any) (*CallToolResult, error) {
    client, conn, err := forwarder.ConnectOrStartDaemon(ctx, socket, logger, tp) // autostart + warm reuse
    if err != nil { return nil, err }
    defer conn.Close()

    stream, err := client.StreamMCP(ctx)
    if err != nil { return nil, err }

    req := jsonrpcRequest{JSONRPC: "2.0", ID: 1, Method: "tools/call",
        Params: callParams{Name: toolName, Arguments: args}}
    payload, _ := json.Marshal(req)
    if err := stream.Send(&serenav1.MCPMessage{Payload: payload, SessionId: newSessionID()}); err != nil {
        return nil, err
    }
    _ = stream.CloseSend() // half-close: we will send nothing further

    // Read frames until the response whose id == our request id (skip notifications/logs).
    for {
        msg, err := stream.Recv()
        if err == io.EOF { return nil, errNoResponse }
        if err != nil { return nil, err }
        var resp jsonrpcResponse
        if json.Unmarshal(msg.Payload, &resp) == nil && resp.ID == 1 {
            return resp.toCallToolResult()
        }
    }
}
```

**Key correctness notes for the roadmapper:**
- **Framing is already opaque bytes** — `MCPMessage.Payload` carries raw JSON-RPC (`internal/forwarder/forwarder.go`), so the CLI does not re-implement MCP; it speaks the same line protocol the forwarder does, just once.
- **MCP handshake**: the daemon's MCP server may require an `initialize` request before `tools/call`. The CLI's one-shot client must either send the `initialize`/`notifications/initialized` preamble on the stream first, or the daemon must accept a bare `tools/call` on a fresh session. **This is the one real protocol question to settle early** — confirm against the daemon's MCP server handshake handling in the first CLI phase; it determines whether `runTool` sends 1 frame or 3.
- **Half-close after the final send** (`CloseSend`) is the clean single-request idiom over a bidi stream; do **not** keep the send side open.
- **Match on JSON-RPC `id`** and skip interleaved notifications (progress, `notifications/message`) so a log frame can't be mistaken for the result — the daemon may emit them on the same stream.
- **Reuse the warm daemon**: `ConnectOrStartDaemon` connects to the running daemon if present and only spawns one if absent — preserving share-until-dirty across CLI calls (PROJECT.md "One-shot daemon dialing with warm reuse").
- **Session id**: generate a fresh one per invocation (the forwarder already does, `generateSessionID()`); the daemon's `LazyInitMiddleware` activates the workspace on first call.
- **No new RPC layer, no new proto message.** The existing bidi RPC is more than sufficient for a single round-trip — there is no reason to add a unary RPC.

---

## SKILL.md as a Shippable Asset (format + install)

**File format (Agent Skills open standard, Claude Code superset):** a directory `<skill-name>/SKILL.md` with YAML frontmatter + markdown body.

**Frontmatter — recommended for Helix:**
```yaml
---
name: helix
description: >
  Semantic code navigation and editing for this repo via the `helix` CLI —
  go-to-definition, find-references, rename, blast-radius, structured edits,
  repo map. Use INSTEAD OF grep/sed/cat when you need symbol-level answers
  (where is X defined, who calls Y, rename across files).
allowed-tools: Bash(helix:*)
---
```
- `name` (optional; defaults to dir name — set it to `helix` for clarity). The invoked command name comes from the directory.
- `description` (the **only** load-bearing field): this is the ~100-token idle footprint scanned every session. **Put the "use instead of grep/sed/cat" trigger first.** Combined `description` + `when_to_use` is truncated at **1,536 characters** in the listing — stay well under.
- `allowed-tools: Bash(helix:*)` lets the agent run `helix <verb>` without a permission prompt — the whole UX point.
- Optional: `when_to_use` for extra trigger phrases.

**Body structure** (mirror playwright-cli's proven layout, progressively disclosed): Quick Start → Commands grouped by category (symbols / edit / fileops / diag) with one terse example each → "Use instead of" decision table (grep→`helix search`, "where defined"→`helix go-to-definition`) → Raw-output/piping notes. The body loads **only when the agent decides the skill is relevant**, so it costs ~nothing idle.

**Install locations (Claude Code):**
| Level | Path written by `helix setup` | Scope |
|-------|-------------------------------|-------|
| Personal | `~/.claude/skills/helix/SKILL.md` | all the user's projects |
| Project | `<repo>/.claude/skills/helix/SKILL.md` | this repo only |
| Plugin | `<plugin>/skills/helix/SKILL.md` | where plugin enabled |

**How `helix setup <client>` flips** (PROJECT.md target): instead of `claude mcp add-json ...` (current `internal/cli/setup_clients.go`), setup now (1) writes the embedded `SKILL.md` to the chosen skills dir, and (2) installs/repurposes the `PreToolUse` nudge hook (`internal/cli/nudge.go`) to redirect `grep`/`sed`/`cat` Bash calls toward `helix` verbs. Ship `SKILL.md` via Go `embed` in the binary (same mechanism as other runtime assets in EMBED-AUDIT.md) so a single binary self-installs its skill. Live change detection means a re-written `SKILL.md` is picked up within the session — no client restart.

---

## Terse, LLM-Oriented Output (conventions, not a framework)

**This is product work, not a dependency.** The convergent convention across ripgrep / ast-grep / gh:

| Convention | Rule for Helix CLI | Source precedent |
|------------|--------------------|------------------|
| **Color default = `auto`, off when piped** | Default `--color=auto`: emit ANSI only if stdout is a TTY **and** `NO_COLOR` unset. Agent invocations are piped → no color by construction. Honor `NO_COLOR` (any value) and a `--color never\|always\|auto` flag. | ripgrep, ast-grep both default `auto`, both honor `NO_COLOR`; ripgrep flips to `never` under `--vimgrep`. |
| **`file:line:col` anchor** | Print locations as `path:line:col` with `:` separators (stable, greppable, the format every model already knows). 1-indexed line/col (Helix already does `userPosToLSP`). | ripgrep `--vimgrep` / grep `-n` format. |
| **One match per line, no decoration** | Avoid boxes/tables/pretty-JSON for results an agent reads; newline-delimited records pipe into `grep`/`awk` and tokenize cheaply. | ast-grep `--json=stream` (one object per line); ripgrep default. |
| **Optional `--json` compact** | For tools whose result is structured (blast radius, repo map), offer `--json` emitting single-line compact JSON (no whitespace) for token efficiency. Text stays the default. | ast-grep `--json=compact` "ideal for LLM agents concerned with token efficiency." |
| **Exit codes carry signal** | `0` = found/ok, `1` = no results (grep convention), `2` = error. Lets the nudge hook and agents branch without parsing prose. | grep/ripgrep exit-code convention. |

**Implementation:** stdlib only — `os.Getenv("NO_COLOR")`, `golang.org/x/term.IsTerminal(int(os.Stdout.Fd()))` (already transitively available), and `fmt`. Per-tool formatters live next to each tool (the existing `formatLocations` in `symbols/tools.go` is the seed). **No color/format library is warranted.**

---

## Installation

```bash
# NOTHING new to `go get` for the core path — everything is already vendored:
#   github.com/spf13/cobra v1.10.2
#   github.com/modelcontextprotocol/go-sdk v1.5.0  (+ bundled jsonschema)
#   google.golang.org/grpc v1.80.0
#   github.com/knadh/koanf/v2 v2.3.4

# Generator scaffold (new internal command, no external deps):
#   cmd/helix-cligen/main.go          # reads registry, emits internal/cli/*_gen.go
#   internal/cli/generate.go          # //go:generate go run ./cmd/helix-cligen

# Regenerate the CLI surface + README tool table:
go generate ./internal/cli/...
go run ./cmd/docgen            # README tool table vs CLI surface
go vet ./... && go test ./...  # project gate

# x/term is already transitive; if `go mod tidy` ever drops it, re-add:
# go get golang.org/x/term   # (only if needed for TTY detection)
```

---

## Alternatives Considered

| Recommended | Alternative | When to use the alternative |
|-------------|-------------|------------------------------|
| `go:generate` source-reflection codegen (Approach A) | Runtime-dynamic command construction (Approach B) | Only if the tool set were genuinely dynamic/plugin-loaded at runtime. Helix's 53 tools are compiled-in → static codegen wins on typing, `--help`, and greppability. |
| Reuse `forwarder.ConnectOrStartDaemon` + `StreamMCP` | A new **unary** gRPC `CallTool(req) returns (resp)` RPC | Only if streaming chunks/progress on a single call become load-bearing for the CLI. Today one-shot over the existing bidi stream needs zero proto change; adding a unary RPC is new surface for no benefit. |
| SDK-bundled `jsonschema` reflection for flag derivation | A separate schema library (e.g. `invopop/jsonschema`) | Never here — the daemon already derives schemas with the SDK's reflector; using the same one guarantees CLI flags match tool params with zero drift. |
| Embedded `SKILL.md` via `go:embed` | Fetch-on-setup from a URL | Never — single-binary, offline-capable distribution is a core constraint (EMBED-AUDIT.md). Embed it. |
| Plain `fmt` + `x/term` for output | `fatih/color`, `lipgloss`, `pterm` | Only for a human-facing TUI, which is explicitly out of scope. Agent output is piped and color-off. |

---

## What NOT to Use

| Avoid | Why | Use instead |
|-------|-----|-------------|
| **Bubble Tea / lipgloss / any TUI framework** | The CLI head is a one-shot, pipe-into-an-agent surface, not an interactive terminal app. A TUI framework adds deps, an event loop, and ANSI by default — the opposite of terse machine-readable output. | Plain `cobra` + `fmt` + newline-delimited `file:line:col`. |
| **A new gRPC service / proto message for `tools/call`** | The `StreamMCP` bidi stream already carries opaque JSON-RPC `tools/call` frames; PROJECT.md locks "likely zero proto changes." A second RPC layer is ~5× work for zero agent-visible benefit (the same logic the milestone rejects for SDK excision). | Reuse `StreamMCP`; send one frame, `CloseSend`, read by `id`. |
| **Excising the MCP Go SDK from the daemon** | Explicitly out of scope (Architecture decision, locked). The SDK is hat #2 (internal dispatch + 5 middlewares). Removing it forfeits the reflector that powers codegen and the middleware stack. | Keep the SDK internal; remove only the stdio-forwarder + HTTP MCP *transports* (hat #1). |
| **A JSON-RPC client library (e.g. `sourcegraph/jsonrpc2`)** | Overkill: the CLI does one request, one response, over a byte-opaque transport. The forwarder already proves `encoding/json` + a byte-match is sufficient. | `encoding/json` struct marshal/unmarshal + `id` match. |
| **`fatih/color` / `mattn/go-colorable` as a dependency** | Color is *off* in the dominant (piped-to-agent) path; a color lib adds a dep to do something `NO_COLOR` + a TTY check do in 3 lines. | `os.Getenv("NO_COLOR")` + `x/term.IsTerminal`. |
| **`go-plugin` / dynamic plugin loading for subcommands** | Tools are compiled-in; the registry is known at build time. Plugins add IPC and versioning overhead for a static set. | `go:generate` over the static registry. |
| **A second autostart/daemon-spawn implementation in the CLI** | `forwarder.dial.go` already implements gopls-pattern connect-or-spawn + readiness poll + keepalive. Duplicating it risks divergent socket/race behavior. | Call `forwarder.ConnectOrStartDaemon`. |

---

## Stack Patterns by Variant

**If the generator should fail CI on drift (recommended):**
- Add `cmd/helix-cligen --check` mirroring `cmd/docgen --check` (exit 1 if generated `*_gen.go` would change). Wire into `make vet`/CI like the existing `vet-noduckdb`/PromQL-validator gates.
- Because the generated CLI surface must match the live 53-tool registry, this also guards the README tool table regen — closing the same class of drift the MEMORY note "Helix tool docs drift" describes (docgen's blank-imports must equal the daemon's, or a tool silently vanishes from the surface).

**If `tools/call` ever needs progress streaming to the CLI:**
- The bidi `StreamMCP` already supports it — read multiple frames, render progress to stderr, keep the final `id`-matched frame as the result. Still **no proto change**; just relax the "first matching id wins" read loop. Defer until a concrete tool needs it.

**If a target client is not Claude Code (e.g. generic / OpenCode):**
- The Agent Skills standard (`agentskills.io`) is cross-tool; `helix setup <client>` writes `SKILL.md` to that client's skills dir. For clients with no skill mechanism, fall back to installing the nudge hook + documenting the `helix` verbs in the client's instruction file. (Setup already special-cases 7 clients in `setup_clients.go`.)

---

## Version Compatibility

| Package A | Compatible With | Notes |
|-----------|-----------------|-------|
| `spf13/cobra@v1.10.2` | Go 1.25.1 | `AddGroup`/`GroupID` available since v1.6.0; already in tree and exercised by `cmd/helix-bench`. |
| `modelcontextprotocol/go-sdk@v1.5.0` | Go 1.25.1, `jsonschema` (bundled) | Bundled reflector caches schemas (perf note in v1.6.x changelog). Upstream latest is v1.6.1 (2025-05); a bump is optional and orthogonal to this milestone — do it in a separate hygiene pass if at all. |
| `google.golang.org/grpc@v1.80.0` | `protobuf@v1.36.11`, `otelgrpc@v0.68.0` | Existing forwarder dial path; `unix://` + keepalive + single `WithStatsHandler` (Pitfall 5 already noted in `dial.go`). No change. |
| `koanf/v2@v2.3.4` | Go 1.25.1 | Existing 4-layer config; CLI resolves socket/profile via the same path. |
| `golang.org/x/term` | Go 1.25.1 | Transitively present; only used for `IsTerminal` in output color policy. |

---

## Sources

- In-tree ground truth (HIGH): `go.mod` (versions), `internal/forwarder/{dial.go,forwarder.go}` (autostart + `StreamMCP` + JSON-RPC byte framing), `internal/cli/root.go` (cobra root + subcommands), `cmd/helix-bench/main.go` (6-subcommand cobra precedent), `protocol/generate.go` + `cmd/lspgen` (`go:generate` precedent), `internal/mcp/registry.go` (`ToolDef`/`ToolRegistry`), `internal/kernel/{symbols,fileops,...}/tools.go` (typed `XxxArgs` structs with `json`+`jsonschema` tags; per-tool `AddTool` registration), `cmd/docgen/main.go` (provider enumeration), `api/proto/serena/v1/ipc.proto` (`StreamMCP(stream MCPMessage)`), `.planning/PROJECT.md` v2.0 milestone section (architecture decision, target features, out-of-scope). — HIGH confidence (read directly).
- [spf13/cobra releases](https://github.com/spf13/cobra/releases) — v1.10.2 latest; `AddGroup`/`GroupID` grouped help since v1.6.0. (verified against in-tree v1.10.2) — HIGH.
- [modelcontextprotocol/go-sdk releases](https://github.com/modelcontextprotocol/go-sdk/releases) — v1.5.0 (in tree) / v1.6.1 latest; bundled `jsonschema` reflector with schema caching. — HIGH.
- [Claude Code: Extend Claude with skills](https://code.claude.com/docs/en/skills) — SKILL.md frontmatter (`name`/`description`/`allowed-tools`/`when_to_use`), 1,536-char description+when_to_use cap, install dirs (`~/.claude/skills/`, `.claude/skills/`, plugin), live change detection. — HIGH.
- [microsoft/playwright-cli SKILL.md](https://github.com/microsoft/playwright-cli/blob/main/skills/playwright-cli/SKILL.md) — reference frontmatter (`name`/`description`/`allowed-tools: Bash(playwright-cli:*)`) and progressively-disclosed body layout (Quick Start → grouped Commands → examples). The reference product for this milestone. — HIGH.
- [Anthropic: Equipping agents with Agent Skills](https://www.anthropic.com/engineering/equipping-agents-for-the-real-world-with-agent-skills) + [Agent Skills overview](https://platform.claude.com/docs/en/agents-and-tools/agent-skills/overview) — progressive disclosure (~100 tokens/skill idle footprint). — HIGH.
- [ast-grep JSON Mode](https://ast-grep.github.io/guide/tools/json.html) + [ast-grep run reference](https://ast-grep.github.io/reference/cli/run.html) — `--json=compact` for token-efficient agent output; `--color=auto` honoring `NO_COLOR`. — HIGH.
- [ripgrep FAQ / rg(1) manpage](https://manpages.debian.org/testing/ripgrep/rg.1.en.html) — `--color=auto` default, `NO_COLOR` honored, `--vimgrep` flips to `never` for machine-readable `file:line:col`. — HIGH.

---
*Stack research for: CLI head over an existing Go MCP daemon (v2.0 CLI-First — MCP Surface Retirement)*
*Researched: 2026-06-21*

# Project Research Summary

**Project:** Helix — v2.0 CLI-First (MCP Surface Retirement)
**Domain:** CLI head for an existing Go MCP daemon — 53-subcommand cobra CLI + agent SKILL.md driving the unchanged LSP/RepoMap kernel over the existing gRPC `StreamMCP` wire
**Researched:** 2026-06-21
**Confidence:** HIGH

## Executive Summary

This milestone retires Helix's *agent-facing* MCP surface (the stdio forwarder head + Streamable-HTTP `/mcp` transport) and replaces it with a single new head: a 53-verb `helix <verb>` CLI, taught to agents via a progressively-disclosed `SKILL.md` and a repurposed PreToolUse nudge hook. The architecture decision is locked and consistent across all four research files: MCP wears two hats — (1) the external transport an IDE connects to, and (2) the daemon's internal tool-dispatch + 5-middleware engine. **Only hat #1 dies.** The SDK, the 5 middlewares, the gRPC IPC, and the warm LSP/RepoMap kernel all stay byte-for-byte behind the wire. Every researcher independently converged on the same low-risk implementation spine: a one-shot `tools/call` over the *existing* `StreamMCP` bidi stream, reusing `forwarder.ConnectOrStartDaemon` for warm-daemon dial — **zero `ipc.proto` changes, zero daemon changes.**

The recommended approach adds essentially no new third-party dependencies. The 53 verbs are **code-generated** from the typed `*Args` structs already carrying `json`+`jsonschema` tags (the same source the SDK reflects for `AddTool` and `cmd/docgen` walks for the README table), shipped behind a `--check` drift gate mirroring docgen. The single load-bearing *product* work is the terse `path:line:col` default renderer (color off when piped / honor `NO_COLOR`, text-default not JSON-default) — and it must precede SKILL.md authoring, because the skill's decision table cites the real verbs and real output shape. Sequencing is strangler-fig: build the dial spine and verbs behind the still-live MCP heads, prove dual-run parity, then **delete the MCP heads last**.

The dominant risks are concentrated and well-understood. The **SECURITY-LOAD-BEARING** finding (surfaced by both ARCHITECTURE and PITFALLS): `ProfileFilterMiddleware` only filters `tools/list` — there is **no `tools/call` rejection path**. With the MCP surface (and its `tools/list`) gone, profile/mode filtering becomes a no-op unless the CLI/daemon enforces it at call time; otherwise a `read`-mode or `ci-bot` agent can invoke destructive edit verbs. This is a regression to *prevent*, not defer. The other foundational risk is the Phase 90 dial layer: per-invocation cold-start tax and a daemon-spawn race / connect-storm — `dial.go` has **no cross-process startup lock**, so parallel first calls into a cold repo spawn duplicate daemons. Both must be fixed in the foundation phase, not patched later.

## Key Findings

### Recommended Stack

Reuse what is vendored; add one tiny generator; add no TUI framework and no second RPC layer. Every load-bearing capability — cobra subcommands at scale (`AddGroup`/`GroupID` for grouped help), gRPC `StreamMCP` dialing with daemon autostart, JSON-RPC byte-framing, struct→JSON-schema reflection — already ships in the tree and is already exercised (`cmd/helix-bench` runs 6 cobra subcommands; `helix activate`/`status` already use `ConnectOrStartDaemon`). The real work is a `go:generate` codegen, a ~150-LOC one-shot MCP-over-gRPC client (a bounded refactor of the forwarder loop), per-tool terse formatters, and a `SKILL.md` asset + setup flip.

**Core technologies (all already in `go.mod`):**
- **`spf13/cobra` v1.10.2** — the 53-subcommand CLI head with grouped help — already the root framework; no new dep.
- **`modelcontextprotocol/go-sdk` v1.5.0** — stays as the daemon's internal dispatch+middleware engine; its bundled `jsonschema` reflector is the codegen source of truth (zero schema drift). Optionally the *client* side too, to get handshake + id-matching for free.
- **`google.golang.org/grpc` v1.80.0 + `protobuf` v1.36.11** — the unchanged CLI→daemon wire; `MCPMessage.payload` carries opaque JSON-RPC bytes. No proto change.
- **`encoding/json` + `golang.org/x/term` (stdlib/transitive)** — one-shot request build/parse; TTY detection for color-off-when-piped. No JSON-RPC lib, no color framework.
- **New: `cmd/helix-cligen` (tiny, no external deps)** — `go:generate` source-reflection codegen emitting committed `*_gen.go`, modeling the existing `cmd/lspgen`/`cmd/docgen` precedent, with a `--check` CI gate.

**Codegen feasibility: HIGHLY FEASIBLE.** The registry already holds tool names + brief/help text; the `*Args` structs already carry `json` (flag name + required-ness via `omitempty`) and `jsonschema` (help) tags. One enabling change: the tool *name* → arg *struct type* mapping is not stored in `ToolDef` today — add a `reflect.Type`/`ArgsExample` field at registration (cleaner) or keep a generator-side table.

### Expected Features

The single reader is an LLM agent invoking the CLI through the Bash tool — categories are tuned for that, not for a human at a terminal.

**Must have (table stakes / v2.0 MVP):**
- Code-generated 53-verb CLI dialing the warm daemon (the parity promise).
- Terse `path:line:col`-anchored default output, per-verb tuned for self-contained action (the load-bearing product work; relative paths, deduped, sorted, no boxes/emoji/color-when-piped).
- Stable exit codes (`0`=results, `1`=no results, `2`=error) + quiet-by-default (results on stdout, diagnostics on stderr).
- SKILL.md: tight third-person `description` (the trigger) + decision-table body (<=500 lines, mirroring the proven CLAUDE.md "SMTC-first routing" matrix), references one level deep.
- PreToolUse steering upgraded to name specific `helix` verbs — advisory-first, code-signal-gated, argument-aware (not substring).
- `helix setup <client>` flip: install skill + hooks (and teardown the old MCP registration) instead of registering an MCP server.

**Should have (differentiators):**
- Self-contained results: locus + enclosing symbol + one snippet line (act in one round-trip, not a bare locator nor the whole file).
- Ref/handle flow (Playwright `e15` analog): a read-verb's output is a copy-paste-able input to an edit/nav verb.
- Idle cost ~= zero (1 skill description) vs MCP's 53-schema always-on tax — the milestone's core "why now."

**Defer (v2.x+):**
- `--json` opt-in (JSON Lines) — add when a concrete script/tool consumer asks; not on the agent path.
- `--stats`/`--count` summarizers; pattern-gated steering *rewrites* (escalate from advisory only after low false-positive rate is measured).
- MCP compatibility shim — only if a concrete laggard-client need surfaces (clean retirement is locked).

**Anti-features to actively avoid each phase:** default-on JSON; a generic `helix run <tool> --args=<json>` mega-verb; pretty/boxed/colored/emoji default output; stuffing all 53 tool docs into SKILL.md; hard-deny-every-grep nudge; steering that fires on free-text/non-code grep; stateful `--session` handles (the warm daemon is the implicit session); a dual MCP+CLI head "just in case."

### Architecture Approach

The new CLI head is a degenerate forwarder session: `ConnectOrStartDaemon` -> one `StreamMCP` stream -> `initialize` -> one `tools/call` -> read the id-matched result -> render -> exit. Every frame flows through the **unchanged** server path (`StreamMCP` -> `GRPCTransport` io.Pipe bridge -> SDK -> 5-middleware chain -> tool handler). Output rendering is a CLI-layer concern that must **never** cross the wire (no `--format` arg on `tools/call`) — the daemon stays content-shape-neutral so the InMemory/HTTP test oracles aren't coupled to terminal formatting.

**Major components:**
1. **CLI verb subcommands** (NEW, generated `internal/cli/*_gen.go`) — one cobra command per tool; flags -> `tools/call` args.
2. **One-shot MCP client** (NEW) — refactor the forwarder send/recv loop into a reusable client-side `GRPCTransport`; preferably drive it with the MCP SDK *client* for free handshake/id-matching.
3. **Terse renderer** (NEW `internal/cli/render/`) — shared `locations.go` `path:line:col` formatter (product invariant across all verbs) + per-tool overrides + golden tests; `IsError`->stderr+nonzero exit; surfaces Suggestion-middleware enrichment.
4. **CLI-side profile/mode gate** (NEW use of `config.ResolveProfile`) — the CLI becomes the enforcement boundary; daemon `tools/call`-side rejection is the defense-in-depth option (see Pitfall 1).
5. **DELETED:** `daemon.listenHTTP`+`/mcp`+`http_session_middleware`, `mcpServer.RunStdio`, the stdio-MCP forwarder framing (no-arg `helix` -> help/status); `--mode`/`--http-addr` public flags. **RETAINED:** daemon, all 4 gRPC RPCs, `GRPCTransport`, SDK dispatch, all 5 middlewares, all tool handlers, `--serve` (now internal).

### Critical Pitfalls

1. **Profile/mode capability regression (SECURITY-LOAD-BEARING).** `ProfileFilterMiddleware` filters `tools/list` only — no `tools/call` rejection path. With the MCP surface gone, filtering becomes a no-op. **Avoid:** enforce profile/mode at the daemon `tools/call` boundary (typed `PermissionDenied`) *and* mirror cosmetically in `helix --help`; re-point the v1.1 profile/mode golden suite at the CLI + daemon refusal. Treat as a regression to prevent, not defer.
2. **Daemon-spawn race / connect-storm (Phase 90 foundational).** `dial.go` has no cross-process startup lock; parallel first calls into a cold repo spawn duplicate daemons, and burst CLI fan-out can hit gRPC accept limits (`ResourceExhausted`). **Avoid:** `flock` around the spawn path (one winner, others wait on readiness); idempotent self-healing `startDaemon`; bound concurrency + client retry-with-jitter; fan-out stress test.
3. **Per-invocation cold-start tax / latency inversion.** Each verb is a fresh process; without airtight warm-reuse the agent re-pays process start + dial + (worst case) LSP cold-index per call. **Avoid:** treat warm-reuse as a measured SLO (2nd-call p95 bench); pre-warm via `SessionStart` `helix activate`; keep the CLI binary path thin; confirm unix-socket dial.
4. **Un-greppable / unstable output.** Unstable ordering, ANSI leakage on piped/hook paths, silent truncation, ambiguous `file:line`, and collapsing the 9-kind error taxonomy into a bare stderr string. **Avoid:** sort-before-print (stable key); plain off-TTY; loud truncation trailer (`... (N more)`); serialize `Kind` to a greppable `error[not_found]:` prefix; golden-output oracle.
5. **Lost deadlines/telemetry/suggestion via cobra short-circuit.** A CLI that validates flags before the gRPC call bypasses the daemon's suggestion enrichment and per-tool budget. **Avoid:** thin pass-through (minimal cobra parse -> typed args JSON); align CLI deadline >= daemon budget; keep otelgrpc trace propagation; render daemon typed errors verbatim.
6. **Setup flip breaks existing MCP client configs (all 7 clients).** Stale MCP registrations point at a dead surface. **Avoid:** idempotent teardown (`claude mcp remove helix`) + skill/hooks install; first-run detect-and-warn; loud CHANGELOG; cover every one of the 7 clients.
7. **Testing harness assumes MCP transport.** The multi-oracle harness tests through MCP — retiring it risks false-green (testing a surface no agent uses). **Avoid:** add a CLI subprocess oracle (reuse `bench/runtime/subprocess`); re-target contract goldens to stdout; demote MCP-dispatch tests to engine units.

## Implications for Roadmap

Strangler-fig, dependency-ordered, phases continue from 90. Each phase ships behind the still-live MCP surface; **the MCP heads are deleted LAST** after dual-run parity is proven.

### Phase 90: CLI one-shot dial spine + warm-reuse / burst foundation
**Rationale:** Everything depends on a correct, race-free dial. The two foundational risks (latency inversion, spawn-race/connect-storm) live here and must be solved before any verb is useful.
**Delivers:** reusable client-side `GRPCTransport` (refactored from forwarder loop); `ConnectOrStartDaemon` reuse + `initialize`->`tools/call`->read->exit; cross-process `flock` startup lock; 2-3 hand-wired verbs end-to-end as proof; 2nd-call p95 warm-reuse benchmark + fan-out stress test.
**Uses:** `forwarder.ConnectOrStartDaemon`, existing `StreamMCP`, MCP SDK client. **Zero proto change; daemon untouched.**
**Avoids:** Pitfalls 2(spawn-race), 3(latency).

### Phase 91: Code-generated 53-verb wiring + CLI/daemon profile-mode gate
**Rationale:** With the spine proven, generate the full surface from the registry; close the SECURITY regression in the same phase that creates the always-visible subcommands.
**Delivers:** `cmd/helix-cligen` source-reflection codegen -> committed `*_gen.go` with `--check` gate; grouped help by tool family; `tools/call`-boundary profile/mode enforcement (typed `PermissionDenied`) + cosmetic CLI mirror; thin-pass-through arg handling preserving daemon suggestion/deadline.
**Implements:** CLI subcommands + profile gate components.
**Avoids:** Pitfalls 1(profile regression — security), 5(lost enrichment/deadlines).

### Phase 92: Terse output renderer (load-bearing product phase)
**Rationale:** This is the milestone's whole rationale and must precede SKILL.md (the skill cites real output). Highest-value, highest-risk product work.
**Delivers:** `internal/cli/render/` shared `locations.go` + per-tool overrides; stable sort; plain off-TTY / `NO_COLOR`; loud truncation; `Kind`-prefixed errors; golden-output oracle per verb.
**Avoids:** Pitfall 4(un-greppable/unstable output).

### Phase 93: SKILL.md authoring + nudge repurpose + setup flip
**Rationale:** Teaching surface depends on the real verbs + terse output existing. Setup flip needs SKILL.md authored and hook command finalized.
**Delivers:** progressively-disclosed SKILL.md (trigger description + decision table); argument-aware nudge redirecting grep/sed/cat -> real `helix` verbs (advisory-first); `setup*.go` flip across all 7 clients with idempotent MCP-registration teardown.
**Avoids:** Pitfalls 6(setup breakage), SKILL trigger, nudge-hazard.

### Phase 94: Retire the agent-facing MCP surface (DELETE)
**Rationale:** Only after CLI parity is the proven sole surface (dual-run). Deleting earlier loses the safety net.
**Delivers:** delete `listenHTTP`+`/mcp`+`http_session_middleware`+`HTTPHandler`+`RunStdio`+stdio forwarder framing; no-arg `helix`->help/status; remove `--mode`/`--http-addr`; explicit remote/multi-client scope decision (does retained gRPC optionally bind TCP?) + Windows local-dial smoke. Middlewares + `StreamMCP` + `GRPCTransport` + tools RETAINED.
**Avoids:** lost HTTP multi-client/remote via explicit scope record.

### Phase 95: Identity & docs rewrite + docgen regen + oracle migration
**Rationale:** Docs describe the final shape; regen is last to avoid churn.
**Delivers:** README/CLAUDE.md/PROJECT.md CLI-first rewrite; `cmd/docgen` re-pointed at CLI surface (column header shift, not just regen) + three-way registry<->CLI<->docgen parity test; CLI subprocess oracle replacing MCP protocol/contract oracles; LLM behavioral oracle for SKILL.md tool-selection.
**Avoids:** Pitfalls 7(harness gap), docgen-drift.

### Phase Ordering Rationale
- **Dial spine first** because every verb and the renderer depend on a correct, race-free one-shot call.
- **Profile gate co-located with verb generation** because generating always-visible subcommands is exactly when the `tools/list`-filter loss bites — security can't trail.
- **Renderer before SKILL.md** because the skill's decision table cites real verbs + real output (FEATURES + research consensus).
- **Delete MCP heads last** (strangler-fig) so a CLI gap is a fallback, not a regression.
- **Docs/docgen/oracle migration last** so they describe the frozen surface.

### Research Flags

Phases likely needing deeper research during planning:
- **Phase 90:** the MCP `initialize` handshake requirement over `StreamMCP` (1 frame vs 3) — confirm whether the daemon accepts a bare `tools/call` on a fresh session or requires `initialize`+`notifications/initialized`. Determines `runTool` shape. Also the cross-process lock design under burst.
- **Phase 91:** the `ToolDef` -> arg-struct-type mapping enabling change, and SDK-client-vs-hand-rolled JSON-RPC tradeoff.
- **Phase 94:** the remote/multi-client scope decision (HTTP MCP was the only network-transparent transport) — whether retained gRPC optionally binds TCP for split-host use.

Phases with standard patterns (lighter research):
- **Phase 92:** terse output conventions are well-documented (ripgrep/ast-grep precedent); risk is execution discipline + golden tests, not unknowns.
- **Phase 95:** docgen regen + parity test follow the existing `--check` precedent.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | Grounded in in-tree `go.mod` + read source; codegen feasibility verified against actual `*Args` tags and SDK reflector. |
| Features | HIGH | Official Claude Code skills/hooks docs + Playwright-CLI + ripgrep/ast-grep man pages + existing nudge read from source. |
| Architecture | HIGH | Every integration point cites a real v1.12 file/line; the "ProfileFilter is tools/list-only" finding anchored at `middleware.go:165,509`. |
| Pitfalls | HIGH | Grounded in actual codebase (dial.go no-lock, nudge substring matcher, error taxonomy) + gopls daemon-mode corroboration + MEMORY drift history. |

**Overall confidence:** HIGH

### Gaps to Address

- **MCP `initialize` handshake requirement** — 1 vs 3 frames over `StreamMCP`; resolve empirically in Phase 90 against the daemon's MCP server handshake handling.
- **Global vs per-command `--json`/`--color` flags** — decide once and apply uniformly (persistent flag vs per-verb); affects codegen template.
- **Absolute vs workspace-relative `file:line`** — decide once, document in SKILL.md, keep consistent across all verbs (Pitfall 4); agents must be able to feed output back into `helix read-file`.
- **Advisory vs blocking nudge** — default advisory (exit 0); if blocking is ever introduced, gate behind opt-in + allowlist + always supply the exact substitute command.
- **Dropping HTTP MCP vs remote access** — whether retained gRPC should optionally bind TCP (`--socket=tcp://host:port`) for split-host use, or remote is a documented removed capability (Phase 94 decision record).
- **53 vs 51 tool count** — research files cite both (51 `AddTool` in some anchors, 53 callable verbs in PROJECT.md); reconcile against the live registry during Phase 91 (the MEMORY docgen-drift lesson applies).

## Sources

### Primary (HIGH confidence)
- In-tree ground truth: `go.mod`, `internal/forwarder/{dial.go,forwarder.go}`, `internal/cli/{root.go,nudge.go,setup*.go,activate.go}`, `internal/mcp/{registry.go,middleware.go,suggest.go,grpc_transport.go,server.go,lazy_init.go}`, `internal/daemon/daemon.go`, `internal/kernel/*/tools.go` (typed `*Args`), `cmd/{docgen,lspgen,helix-bench}`, `api/proto/serena/v1/ipc.proto`, `internal/{profile,config,errors}`, `.planning/PROJECT.md` v2.0 milestone.
- Claude Code Agent Skills (overview + best-practices) — description-as-trigger, char caps, body <=500 lines, progressive disclosure, install dirs, live change detection.
- microsoft/playwright-cli README/SKILL.md — reference frontmatter, `e15` ref flow, grouped commands, token-efficiency rationale.
- ripgrep / ast-grep man pages + docs — `--color=auto`/`NO_COLOR`, `path:line:col`, `--json` opt-in JSON Lines, exit-code `0/1/2`.
- spf13/cobra + modelcontextprotocol/go-sdk releases — `AddGroup`/`GroupID`; bundled `jsonschema` reflector.

### Secondary (MEDIUM confidence)
- gopls daemon-mode reports (go.dev/gopls/daemon, golang/go#48844) — warm-daemon connect can still stall when the *workspace* wasn't pre-warmed; corroborates per-invocation latency risk.

### Tertiary (LOW confidence)
- None load-bearing; the 51-vs-53 tool count is the one figure to reconcile against the live registry during planning.

---
*Research completed: 2026-06-21*
*Ready for roadmap: yes*

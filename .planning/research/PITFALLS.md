# Pitfalls Research

**Domain:** CLI-over-warm-daemon migration + retiring an existing MCP agent surface (Helix v2.0 "CLI-First")
**Researched:** 2026-06-21
**Confidence:** HIGH (grounded in the actual Helix codebase — `internal/forwarder/dial.go`, `internal/cli/nudge.go`, `internal/cli/setup_hooks.go`, `internal/errors/kinds.go`, the `tools/call`-keyed middleware stack — plus external corroboration from gopls daemon-mode reports and Claude Code Agent Skills docs)

This milestone has two intertwined risk surfaces: (A) **adding** a fresh-process-per-call CLI head in front of a daemon that was designed for one long-lived MCP session, and (B) **removing** a working MCP surface whose middleware quietly provided profile-filtering, per-tool deadlines, telemetry, and suggestion enrichment. The pitfalls below are specific to that pairing — not generic CLI advice.

## Critical Pitfalls

### Pitfall 1: Per-invocation cold-start tax — every `helix <verb>` re-paying daemon start or LSP warm-up

**What goes wrong:**
An MCP session connects once and stays warm for the whole agent session; the warm LS pool, RepoMap cache, and gRPC channel persist across hundreds of tool calls. A CLI head inverts this: each `helix find_symbol …` is a fresh OS process that dials the daemon, runs one `tools/call`, prints, and exits. If the warm-reuse contract isn't airtight, the agent pays process startup + gRPC dial + (worst case) daemon spawn + (worst case) LSP cold-index **on every single call**. Even when the daemon is warm, gopls-style reports show that connecting to a warm shared daemon can still stall 10+ seconds if the *workspace* (not the daemon) wasn't already activated — the cold cost is per-workspace, not per-daemon. Helix's existing `LazyInitMiddleware` activates the workspace on *first* `tools/call`; under CLI-per-call there is no persistent session to amortize that first call against, so the very first `helix` verb in a repo eats the full LSP warm-up synchronously while the agent waits.

**Why it happens:**
`ConnectOrStartDaemon` (internal/forwarder/dial.go:24) already does try-connect-then-spawn with a 10s readiness poll — but it was written for *one* forwarder process, not N concurrent short-lived CLI processes. The autostart path (`startDaemon` → `cmd.Start` → `Release`) has no cross-process lock, so two near-simultaneous CLI invocations into a cold repo both observe "socket not found" and both spawn a daemon (see Pitfall 2). Process-startup overhead (Go runtime init, cobra wiring, koanf 4-layer config load, gRPC client construction, otelgrpc stats handler) is paid per invocation and is invisible in single-call benchmarks.

**How to avoid:**
- Treat "warm reuse" as a measured SLO, not an assumption. Establish a benchmark for *second-and-subsequent* `helix <verb>` calls into an already-warm repo and gate it (e.g. p95 < 150 ms process-to-result excluding the tool's own work). The existing `bench/` harness (v1.12) is the natural home.
- Minimize per-process fixed cost: lazy-construct the otelgrpc stats handler and koanf layers only when needed; avoid importing skill packages into the thin CLI binary path (they belong in the daemon).
- Keep the daemon's `LazyInitMiddleware` but add an explicit `helix activate` warm-up verb (already exists as `activate`) and have `setup` wire it into `SessionStart` so the first real verb is never the one paying LSP cold-start — the hook does it ahead of time.
- Reuse of the gRPC channel within a single process is moot (one call per process), so the win must come from daemon warmth + cheap dial. Confirm the Unix-socket dial path (not TCP) is used by default.

**Warning signs:**
- Second `helix find_symbol` in the same repo is not dramatically faster than the first.
- Telemetry shows `LazyInitMiddleware` activation firing on calls other than the session's first.
- p95 latency in the CLI benchmark is dominated by a fixed constant independent of tool work.

**Phase to address:**
Phase 90 (daemon-dial / warm-reuse foundation) owns the reuse contract; a CLI-latency benchmark phase verifies the SLO.

---

### Pitfall 2: Daemon auto-start race & socket connect storm under burst CLI calls

**What goes wrong:**
An agent fires many `helix` calls in quick succession (e.g. a fan-out of `find_references` across files). Three distinct failures emerge: (1) **spawn race** — into a cold repo, several CLI processes simultaneously fail `tryConnect`, each calls `startDaemon`, and you get duplicate daemons fighting over the same socket path; the loser's `cmd.Start` may clobber the socket or leave an orphan. (2) **connect storm** — into a warm daemon, dozens of concurrent `net.DialTimeout` + `grpc.NewClient` calls hit the daemon's accept loop and per-connection goroutine/stream limits at once, causing queueing, timeouts, or `ResourceExhausted`. (3) **thundering-herd on readiness** — during the 10s `waitForDaemon` poll window, all racers poll the socket every 100 ms, and when the daemon finally binds they all reconnect in the same instant.

**Why it happens:**
`tryConnect`→`startDaemon`→`waitForDaemon` in dial.go has zero cross-process mutual exclusion. The forwarder design assumed a single client process; the MCP transport multiplexed many tool calls over *one* gRPC stream. CLI-per-call replaces stream multiplexing with connection multiplicity, which the daemon's listener was never load-tested for.

**How to avoid:**
- Add a cross-process startup lock: an exclusive `flock` (or atomic socket-create-then-bind) around the spawn path so exactly one CLI wins the spawn and the rest wait on readiness. The lock file lives next to the socket (`.helix/daemon.lock`).
- Make `startDaemon` idempotent and self-healing: if the socket exists but is dead (stale after crash), the winner unlinks-and-rebinds; losers detect "now alive" and connect.
- Bound concurrency: the daemon must tolerate a burst of short-lived gRPC connections — set sane `MaxConcurrentStreams`, connection accept backpressure, and confirm one connection per CLI process closes cleanly (no FD leak).
- Consider an OS-level retry-with-jitter on the *client* dial so the herd disperses instead of synchronizing on the 100 ms tick.

**Warning signs:**
- `ps` shows >1 `helix --serve` after a burst.
- Intermittent "daemon did not start within 10s" under parallel calls that never reproduces serially.
- Daemon FD count climbs monotonically across a session (connection leak).
- gRPC `ResourceExhausted` or accept-queue latency spikes in telemetry under fan-out.

**Phase to address:**
Phase 90 (daemon-dial foundation) — the startup lock and burst-tolerance are foundational; a dedicated connect-storm stress test (synctest + fan-out, mirroring the v1.1 three-tier concurrency pattern) verifies it.

---

### Pitfall 3: Capability regression — losing profile/mode tool-filtering when the MCP `tools/list` surface is removed

**What goes wrong:**
`ProfileFilterMiddleware` filters `tools/list` by the active profile and applies brief/override descriptions on that same pass (CLAUDE.md middleware stack). Profiles (claude-code, codex, ci-bot, …) and modes (read/edit/review/admin) gate *which tools an agent may even see*. A CLI head that exposes one cobra subcommand per tool has **no `tools/list` step** — every subcommand is always present on `helix --help`. If nothing replaces the filter, a `read`-mode or `ci-bot` agent can now invoke destructive edit verbs that the MCP surface would have hidden, and mode transitions (`switch_mode`) lose their meaning. This is a silent *security/safety* regression, not just a UX one.

**Why it happens:**
Profile/mode filtering lived entirely in the MCP `tools/list` response path. Cobra registers all subcommands at build time; there is no per-request filtering hook unless you add one. It's easy to assume "the daemon still enforces it" — but the daemon's filter ran on `tools/list`, and the CLI never calls `tools/list`; it goes straight to `tools/call`.

**How to avoid:**
- Enforce profile/mode at the **`tools/call` boundary inside the daemon**, not (only) at CLI subcommand registration. Every gRPC `tools/call` must be checked against the active profile/mode and rejected with a typed `PermissionDenied`/`Unsupported` error if out of profile — so even a hand-typed `helix replace_symbol_body` in read mode is refused by the daemon.
- Mirror the filter in the CLI for UX: hide or grey out out-of-profile subcommands in `helix --help` based on resolved profile, but treat that as cosmetic — the daemon is the enforcement point.
- Preserve `switch_mode` as a CLI verb that mutates daemon session/workspace state, and make mode part of the per-call gRPC metadata.
- Keep the golden-file profile/mode contract tests (v1.1, 19 goldens) alive by re-pointing them at the CLI surface + the daemon enforcement, so a tool leaking into the wrong profile fails CI.

**Warning signs:**
- `helix --help` shows edit verbs while in `read` mode.
- A profile/mode golden test has no CLI-surface equivalent after the migration.
- No `PermissionDenied` path exercised in CLI tests for out-of-profile calls.

**Phase to address:**
The phase that builds CLI parity must add daemon-side `tools/call` profile/mode enforcement; the profile/mode golden suite migration is its verification.

---

### Pitfall 4: Losing per-tool deadlines, telemetry, and suggestion/guardrail enrichment that lived in the middleware stack

**What goes wrong:**
The five middlewares (`Telemetry`, `ProfileFilter`, `Suggestion`, `LazyInit`, `Guardrail`) execute on the `tools/call` path *inside the daemon*. The locked architecture keeps them running — but two things can still regress: (1) **deadlines** — `TelemetryMiddleware`'s `BudgetFunc` injects a per-tool deadline; if the CLI sets its own client-side gRPC timeout that's shorter or longer, you get either premature cancellation (CLI gives up while the daemon is mid-LSP-call) or a hung CLI (CLI waits forever past the daemon's deadline). (2) **suggestion enrichment** — `SuggestionMiddleware`'s "Did you mean?" for parameter typos enriches `tools/call` *errors*; if the CLI does its own cobra-level flag parsing and rejects bad flags *before* reaching the daemon, the agent never sees the daemon's richer suggestion, just cobra's terse "unknown flag" — a downgrade in error quality.

**Why it happens:**
The middleware contract assumed the *client* was a dumb transport that forwarded `tools/call` verbatim and surfaced whatever the daemon returned. A CLI is not a dumb transport: cobra parses, validates, and can short-circuit before the gRPC call, bypassing the daemon's enrichment and deadline logic.

**How to avoid:**
- Make the CLI a **thin pass-through**: do minimal cobra parsing (just map subcommand+flags → typed args JSON), and let the *daemon's* validation/suggestion/guardrail middleware own argument errors. Do not duplicate enum/param validation in cobra where the daemon already does it better.
- Align timeouts: the CLI's gRPC call deadline must be derived from (or strictly longer than) the daemon's per-tool budget, so the daemon's deadline fires first and returns a typed `Timeout` error the CLI can render — rather than the CLI cancelling blind.
- Keep telemetry meaningful: the daemon still emits RED metrics per `tools/call`; verify the per-call CLI invocation still carries trace context (the otelgrpc client stats handler in dial.go must propagate `TraceContext{}` — it does today, don't drop it in the thin CLI).
- Surface the daemon's typed error verbatim — including suggestion text — in CLI stderr.

**Warning signs:**
- CLI exits with cobra's "unknown flag" instead of the daemon's "Did you mean `--symbol`?" suggestion.
- Traces show no span for CLI-driven `tools/call`, or broken trace continuity.
- A long LSP operation gets cancelled by the CLI before the daemon's budget elapses (client-deadline-too-short).

**Phase to address:**
The CLI-parity phase; verified by an error-quality oracle (the daemon's suggestion/typed-error output must survive to CLI stderr unchanged).

---

### Pitfall 5: Un-greppable / unstable CLI output that loses to grep instead of beating it

**What goes wrong:**
The whole product thesis is "terse `file:line`-anchored output beats grep." It's easy to ship output that *looks* terse but fails for an LLM reader: (a) **unstable ordering** — map-iteration or goroutine-completion order makes the same query print results in different orders run-to-run, breaking the agent's ability to diff or reference "the 3rd result"; (b) **color/ANSI leakage** — cobra/term libraries auto-detect a TTY but a hook-spawned or piped invocation may still emit escape codes that pollute the model's context with `\x1b[31m`; (c) **truncation hiding results** — a token-budget or line cap that silently drops matches makes the agent confidently wrong ("no other references"); (d) **ambiguous `file:line`** — relative vs absolute paths, or `path:12` vs `path:12:5`, that the agent can't reliably feed back into `helix read_file`; (e) **losing the typed-error taxonomy** — v1.5's 9-kind structured errors (`not_found`, `invalid_args`, `circuit_open`, `timeout`, `guardrail_violation`, …) collapsing into an undifferentiated stderr string the agent can't branch on.

**Why it happens:**
Designing output "for a human at a terminal" (pretty tables, colors, spinners, truncation-with-ellipsis) is the default instinct, and it's exactly wrong for a model reader. Ordering instability comes from concurrent LS fan-out with unsorted result merging. The structured-error loss happens because gRPC returns an error and the naive CLI just prints `err.Error()`, discarding `Kind` and the `GuardrailViolationDetail`/`SeeAlso` payload (internal/errors/kinds.go).

**How to avoid:**
- **Deterministic ordering**: sort every multi-result output by a stable key (file path, then line, then col) before printing. Make this a tested invariant.
- **No color by default for the agent path**: disable ANSI unless `--color=always`; auto-detection must default to plain when stdout is not an interactive TTY (and the nudge/skill path should pass plain explicitly).
- **Truncation must be loud**: never silently drop; print a machine-readable `… (N more, re-run with --limit=… )` trailer so the agent knows results were elided. Prefer token-budgeted *but complete-count-reported* output.
- **Canonical `file:line[:col]`** with a documented, stable scheme (decide absolute vs workspace-relative once, document it in SKILL.md, keep it consistent across all verbs).
- **Preserve the error taxonomy on the wire and in print**: serialize `Kind` (and the guardrail `see_also`/`required_receipts` detail) across gRPC and render a greppable prefix (e.g. `error[not_found]: …`, reusing the existing `subsystem_disabled:`-style greppable convention) so an agent can branch on kind.

**Warning signs:**
- Running the same query twice yields different result order.
- Escape codes appear in captured CLI output during hook/piped invocation.
- An agent says "no references found" when references exist (silent truncation).
- CLI stderr for a known `not_found` is indistinguishable from an internal error.

**Phase to address:**
A dedicated "terse output / output contract" phase (this is called out as "the load-bearing product work"); verified by golden-output oracle tests asserting ordering, no-ANSI, loud-truncation, and `Kind`-prefixed errors.

---

### Pitfall 6: SKILL.md that doesn't trigger (or triggers on everything)

**What goes wrong:**
The skill's YAML `description` *is the trigger* — Claude reads it every turn and decides relevance. Two failure modes: (a) **under-triggering** — a vague description ("helps with code") never fires, so the agent keeps using grep and the entire migration delivers nothing; (b) **over-triggering** — an over-broad description ("use for any file operation") fires on README edits, log greps, and config reads where `helix` has no advantage, wasting context and annoying users. A third, subtler failure: the skill *fires* but the agent **ignores it and uses grep anyway** because grep is in muscle-memory and the skill didn't give a concrete, lower-friction substitution.

**Why it happens:**
Skill descriptions are easy to write as documentation ("what it does") rather than as a trigger ("when to use it, with specific terms"). Anthropic's guidance is explicit: be specific, include key terms and concrete triggers, write in third person ("This skill should be used when…"). Teams underestimate that the agent's default (grep/cat) is a strong attractor that a weak skill won't overcome.

**How to avoid:**
- Write the description as a trigger with concrete verbs and contexts: name the operations (find definition, find references, rename across files, blast radius) and the *anti-trigger* boundary (NOT for free-text search in comments/docs/logs — exactly the CLAUDE.md "when grep is still correct" list).
- Keep SKILL.md body lean (target ~1,500–2,000 words / <500 lines) and push detail to progressively-disclosed sub-files; an overlong body degrades triggering and wastes context.
- Give the agent a **direct grep→helix substitution table** in the skill (mirroring the CLAUDE.md decision matrix) so the substitution is lower-friction than typing grep.
- **Verify behavior change empirically**, not by inspection: reuse the v1.4 LLM behavioral-test harness + judge scoring to measure "did the agent pick `helix find_references` over `grep -r`" across realistic prompts, with a pass-rate gate. A skill is "done" only when it measurably shifts tool selection.

**Warning signs:**
- Behavioral tests show grep still chosen for symbol-level questions after the skill ships.
- The skill fires on doc/log tasks where `helix` adds nothing (over-trigger).
- SKILL.md body exceeds ~500 lines.

**Phase to address:**
A SKILL.md authoring phase, gated by the LLM behavioral oracle (tool-selection pass-rate), not by author judgement.

---

### Pitfall 7: Nudge-hook hazards — false positives, wrong-verb mapping, and users disabling hooks

**What goes wrong:**
The repurposed `PreToolUse` hook redirects grep/sed/cat → `helix`. Failure modes: (a) **false positives** — `isGrepReadTool` (nudge.go:186) flags any Bash command merely *containing* the substrings `grep`/`find`/`rg`/`ag`, so grepping a *log file*, a path that contains `ripgrep`/`postgres`, a comment mentioning `find`, or a README search trips it. Substring matching is far too coarse and will nudge on legitimate non-code searches (logs, READMEs, YAML), training the user to ignore or disable the hook; (b) **wrong-verb mapping** — mapping a `grep "func "` to `find_symbol` when the user wanted a literal-string search in a Markdown file gives actively bad advice; (c) **blocking vs advising** — the current nudge is advisory (always exit 0, just prints a tip after 5 calls). If v2.0 escalates it to *block* (PreToolUse can deny the tool call), a false positive now *prevents* a legitimate grep, which is rage-inducing and the fastest route to `--no-hooks`; (d) **annoyance → disable** — too-frequent or too-preachy nudging makes users turn hooks off, losing the steering entirely.

**Why it happens:**
The existing matcher is `strings.Contains`-based substring detection over the raw Bash command — fast to write, wrong in the tails. The hook fires on `Grep|Read|Bash` broadly. Escalating from advisory-tip to behavioral-redirect raises the cost of every false positive.

**How to avoid:**
- Replace substring matching with **argument-aware parsing**: detect that the Bash command's *program* is `grep`/`rg`/`ag`/`sed`/`cat` (token at command position, not anywhere in the string), and inspect the *target* — only nudge when the search target is a code file in the workspace, never for `*.log`, `*.md`, `*.yaml`, `/var/log`, paths outside the workspace, or piped-from-stdout greps.
- **Map conservatively**: only suggest a specific `helix` verb when the pattern is unambiguously symbolic (e.g. `grep -rn "func X"` → `find_declarations`); for ambiguous greps, suggest nothing or a generic pointer, never a wrong specific verb.
- **Keep it advisory by default**; if blocking is ever introduced, gate it behind explicit opt-in and an allowlist, and always provide the exact `helix` command to run instead so the redirect is zero-friction.
- **Tune frequency**: keep the "after N calls without symbolic use" threshold and the per-session reset (already in nudge.go), and make the message terse and actionable, not preachy.
- Honor `--no-hooks` and document a one-line disable so frustrated users downgrade gracefully instead of abandoning Helix.

**Warning signs:**
- Users report the nudge firing on `grep` of log files or READMEs.
- Issue reports / telemetry show `--no-hooks` usage rising after the change.
- The hook suggests `find_symbol` for a literal-text search.

**Phase to address:**
The nudge-hook phase; verified by a fixture corpus of Bash commands (code-grep vs log-grep vs doc-grep vs non-grep-containing-substring) asserting nudge fires only on the true positives.

---

### Pitfall 8: Breaking existing users' MCP client configs with no migration path

**What goes wrong:**
Existing users have `helix` registered as an MCP server in Claude Code / VS Code / Gemini / Claude Desktop (via `helix setup <client>` which calls `claude mcp add-json` etc.). When v2.0 retires the stdio forwarder head and HTTP MCP transport, those registrations point at a surface that **no longer answers MCP**. The agent's MCP client will show a dead/failing server, the 53 tools vanish from the agent's tool list, and — because the migration also *retires* MCP — there's no fallback. Without a migration path, every existing user's setup silently breaks on upgrade.

**Why it happens:**
`setup` registered an MCP server *with the client's own config store* (it shelled out to `claude mcp add-json`, it didn't write Helix-owned files). Helix can't unilaterally clean those up; they live in the client. The "clean retirement, no shim" decision (locked, out-of-scope item) means there's deliberately no compatibility MCP head, so a stale registration is a hard break.

**How to avoid:**
- Ship a **migration command**: `helix setup <client>` (the "flip") must both *remove* the old MCP registration (e.g. `claude mcp remove helix`) and *install* the new skill+hooks, idempotently — so re-running setup heals a stale config. The hook installer already has an idempotent `helix_managed: true` pattern (setup_hooks.go) to clone for MCP-registration teardown.
- **Detect-and-warn on first run**: when the daemon or CLI starts and detects an orphaned MCP registration it can no longer serve, emit a one-line "run `helix setup <client>` to migrate" message.
- **Document the breaking change loudly** in CHANGELOG + README (this is a v2.0 major; a hard cut is acceptable but must be announced, like the v1.9 serena→helix rename precedent).
- Cover every one of the 7 setup clients — a migration that flips Claude Code but forgets Gemini/OpenCode leaves those users broken.

**Warning signs:**
- After upgrade, the agent's MCP tool list is empty and no skill/hooks were installed.
- `helix setup <client>` run twice produces duplicate or conflicting entries (non-idempotent teardown).
- A supported client has no teardown path in the flipped `setup`.

**Phase to address:**
The `helix setup` flip phase; verified by a per-client setup/teardown idempotency test (run setup twice, assert old MCP entry gone + skill/hooks present exactly once).

---

### Pitfall 9: Losing HTTP-transport multi-client / remote scenarios

**What goes wrong:**
The Streamable-HTTP MCP transport let *multiple clients* connect to one daemon over HTTP (and enabled remote/containerized setups where the agent and daemon aren't co-located). Retiring it removes that topology. The CLI head assumes the daemon is reachable over a **local Unix socket** (dial.go is unix-socket-first), which means: (a) remote/split-host deployments stop working with no replacement; (b) any user who relied on HTTP for multi-client fan-in loses it silently; (c) Windows named-pipe / cross-platform socket nuances resurface (the forwarder has `dial_windows.go`/`dial_unix.go` for a reason).

**Why it happens:**
HTTP was the only network-transparent transport; the CLI-over-gRPC-unix-socket path is inherently local. It's easy to treat "remove HTTP MCP" as pure subtraction without noticing it also removed the only remote-access story.

**How to avoid:**
- **Confirm the scope**: the milestone retires the HTTP *MCP* transport, not necessarily the daemon's ability to listen on a TCP gRPC socket. If remote access matters, the gRPC layer (retained) can still bind TCP; decide explicitly whether the CLI supports `--socket=tcp://host:port` or whether remote is genuinely out of scope.
- If remote is dropped, **document it as a removed capability** in CHANGELOG so users with that topology aren't surprised.
- Preserve cross-platform dialing (`dial_windows.go` named-pipe path) for the local case — Windows agents still need to reach the daemon.

**Warning signs:**
- A user reports the daemon is on a different host and the CLI can't reach it.
- Windows CLI invocations fail to dial the daemon (named-pipe path regressed).

**Phase to address:**
The MCP-surface-removal phase; verification: explicit decision record on remote/multi-client scope + a Windows local-dial smoke test.

---

### Pitfall 10: docgen tool-table going stale (53 vs N drift)

**What goes wrong:**
The README tool table is auto-generated by `cmd/docgen` from the live tool registry; CLAUDE.md says "do not hand-edit." There's *already* documented drift history (MEMORY.md: a 53-vs-47 tool-count drift from `cmd/docgen` missing a blank import; the v1.12 `test/bench` MCP-registry 53-vs-47 failures). Now the surface *changes shape* — from "53 MCP tools" to "53 `helix` CLI verbs." If docgen still introspects the MCP `tools/list` registry while the agent-facing surface is the CLI, the generated table describes a surface that no longer exists; if it isn't re-pointed at the CLI subcommand set, every count and name can drift.

**Why it happens:**
docgen and the daemon must register the *same* set (the MEMORY note: "Keep docgen's imports == daemon's"). The CLI migration introduces a *third* surface (cobra subcommands) that must stay in lockstep with the registry the daemon exposes and the docgen reads. Three things to keep equal instead of two = more drift surface.

**How to avoid:**
- **Single source of truth**: generate the CLI subcommands *and* the docgen table from the same tool registry (the milestone already plans "code-generated subcommand wiring" from the registry's typed args — extend that generation to docgen).
- Keep the existing CI count gate but re-point it at the CLI surface; assert `len(cli subcommands) == len(registry tools) == docgen rows`.
- Reuse the v1.12 lesson: a missing blank import silently drops tools — keep an explicit registry-completeness test, not just a count.

**Warning signs:**
- README tool table count ≠ `helix --help` verb count ≠ daemon registry size.
- A `test/bench` or docgen count test fails after adding a tool.

**Phase to address:**
The identity/docs-rewrite phase; verified by a three-way count/name parity test (registry ↔ CLI ↔ docgen).

---

### Pitfall 11: Testing strategy gap — the multi-oracle harness assumes an MCP transport

**What goes wrong:**
The v1.1–v1.4 multi-oracle harness (protocol / contract / scenario / LLM-behavioral) tests *through the MCP transport* — protocol oracle checks the MCP handshake and `tools/list`; contract oracle meta-validates MCP tool schemas and golden outputs; the harness uses InMemory + HTTP MCP transports. Retiring the MCP surface **deletes the thing those oracles test against**. If the harness isn't reworked, either the tests break (and get disabled, losing coverage) or they keep passing against an internal MCP path that's no longer the agent's surface (false green — testing a surface no agent uses).

**Why it happens:**
The oracles were built around MCP being *the* surface. The locked architecture keeps MCP as internal dispatch, so the tempting shortcut is "leave the oracles pointed at the internal MCP dispatch" — but that no longer reflects what an agent experiences (a fresh CLI process per call, gRPC, terse stdout, exit codes).

**How to avoid:**
- **Add a CLI end-to-end oracle**: spawn the real `helix` binary as a subprocess (the v1.12 bench harness already does daemon-over-unix-socket subprocess spawning — reuse `bench/runtime/subprocess`), run a verb, assert on stdout/stderr/exit-code. This is the new "protocol" layer for the CLI surface.
- **Re-target the contract oracle**: golden outputs become *CLI stdout* goldens (ordering, `file:line`, error-kind prefix) instead of MCP JSON; schema meta-validation becomes "typed args → cobra flags" parity.
- **Keep the scenario oracle** (multi-language runtime correctness) but drive it through the CLI verbs.
- **Keep the LLM behavioral oracle** and *expand* it — it's now load-bearing for SKILL.md triggering (Pitfall 6).
- Don't delete the internal MCP-dispatch tests; demote them to "internal engine" unit tests (the SDK is still the dispatch core) so the engine stays covered without pretending it's the agent surface.

**Warning signs:**
- After migration, oracle tests still import the MCP HTTP transport as the surface under test.
- No test spawns the real CLI binary and asserts on stdout.
- Coverage of "what the agent actually sees" is zero (only internal dispatch is tested).

**Phase to address:**
A testing-migration phase (likely late in the milestone, after CLI parity + output contract land); verified by a CLI subprocess oracle replacing the MCP protocol/contract oracles.

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| CLI client sets its own short gRPC deadline, ignoring daemon `BudgetFunc` | Simple, no metadata plumbing | Premature cancel of long LSP ops; deadline taxonomy diverges from telemetry | Never — align to daemon budget |
| Keep substring `strings.Contains` nudge matcher | No parser to write | False positives on logs/READMEs → users disable hooks | Only as the current advisory tip; never if nudge escalates to blocking |
| Print `err.Error()` and drop `Kind` | One line | Agent loses the v1.5 typed-error taxonomy it can branch on | Never — serialize Kind across gRPC |
| Leave multi-oracle harness pointed at internal MCP dispatch | Tests stay green | False green — surface no agent uses; real CLI surface untested | Only as a transitional step with a tracked CLI-oracle follow-up |
| No cross-process daemon-start lock | Works in serial tests | Spawn race + duplicate daemons under agent fan-out | Never — agents fan out by design |
| Auto-detect TTY for color without an agent-path override | Pretty for humans | ANSI leaks into model context on piped/hook paths | Only with an explicit plain-by-default for non-TTY |

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| Claude Code MCP registry | Flip `setup` to install skill but forget to `claude mcp remove helix` | Idempotent teardown-then-install; heal stale config on re-run |
| Claude Code PreToolUse hook | Block legitimate grep on a false positive → user runs `--no-hooks` | Advisory-first; argument-aware matcher; always supply the exact `helix` substitute |
| gRPC `StreamMCP` wire | Assume one-shot `tools/call` needs proto changes | Reuse the existing bidi `StreamMCP` frame; likely zero proto changes (per locked architecture) |
| 7 setup clients | Migrate Claude Code only | Cover all 7 (Claude Code, VS Code, JetBrains, Claude Desktop, Gemini, OpenCode, generic) |
| otelgrpc trace propagation | Drop the client stats handler in the thin CLI | Keep `obs.ClientStatsHandler(tp)` + `TraceContext{}` so per-call traces stay continuous |
| Windows daemon dial | Assume Unix socket everywhere | Preserve `dial_windows.go` named-pipe path for local Windows agents |

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| Per-process fixed startup cost (runtime + koanf + gRPC + otel) | p95 dominated by a tool-work-independent constant | Lazy-init heavy components; keep CLI binary path thin | Immediately, on every call; worsens under fan-out |
| Connect storm into warm daemon | gRPC `ResourceExhausted`, accept-queue latency under fan-out | Bound concurrency; tolerate burst of short connections; client retry-with-jitter | Tens of concurrent CLI calls (agent fan-out) |
| First-call LSP cold-index paid synchronously | First `helix` verb in a repo stalls 10s+ (gopls-style) | Pre-warm via `SessionStart` `helix activate` hook | First call into any new workspace |
| FD/connection leak per CLI process | Daemon FD count climbs across a session | One connection per process, asserted closed; leak test | Long sessions with many calls |
| Daemon spawn race | >1 `helix --serve`, intermittent start-timeout | Cross-process `flock` around spawn | Cold repo + parallel first calls |

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| Profile/mode filtering only at CLI registration, not daemon `tools/call` | A `read`-mode/`ci-bot` agent invokes destructive edit verbs hidden by the old MCP filter | Enforce profile/mode at the daemon `tools/call` boundary; reject with typed `PermissionDenied` |
| Guardrail middleware bypassed by cobra short-circuit | Destructive op runs without the v1.6 receipt/guardrail check | Route all `tools/call` through the daemon; never validate-and-execute in the CLI |
| Nudge hook executes values from stdin | Command injection via crafted hook input | Already mitigated (nudge.go reads ToolName/ToolInput as data only) — keep that invariant |
| Stale MCP registration left answering on a port | Confused/dead surface; potential bind confusion | Teardown old registration in `setup` flip |

## UX Pitfalls

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| Preachy / too-frequent nudges | User disables hooks, loses all steering | Terse, actionable, threshold-gated, per-session reset (keep nudge.go pattern) |
| Silent result truncation | Agent confidently wrong ("no references") | Loud `… (N more)` trailer with re-run hint |
| Color codes in piped/hook output | Pollutes model context | Plain by default off-TTY |
| Stale config breaks on upgrade with no message | User thinks Helix is broken | First-run detect-and-warn + idempotent `setup` migration |
| Wrong-verb nudge (`find_symbol` for a doc search) | Actively bad guidance | Suggest specific verb only for unambiguous symbolic patterns |

## "Looks Done But Isn't" Checklist

- [ ] **CLI parity:** All 53 verbs present — verify three-way count parity (registry ↔ `helix --help` ↔ docgen), not just "the common ones work."
- [ ] **Warm reuse:** Second call into a warm repo is fast — verify with a 2nd-call benchmark, not a single-call demo.
- [ ] **Profile/mode enforcement:** Out-of-profile verb is *refused by the daemon* — verify a hand-typed edit verb in `read` mode returns `PermissionDenied`, not success.
- [ ] **Error taxonomy:** `Kind` survives to CLI stderr — verify a known `not_found` prints `error[not_found]:`, distinct from `internal`.
- [ ] **Output determinism:** Same query twice → byte-identical order — verify with a repeated-run golden.
- [ ] **No ANSI off-TTY:** Piped output is plain — verify captured output has no escape codes.
- [ ] **Truncation loud:** Capped output announces elision — verify the trailer appears, not silent drop.
- [ ] **Setup migration:** Re-running `setup` heals a stale MCP registration — verify old entry removed + skill/hooks installed exactly once, for all 7 clients.
- [ ] **Skill behavior change:** Agent actually picks `helix` over grep — verify with the LLM behavioral oracle pass-rate, not by reading SKILL.md.
- [ ] **Daemon-start race:** Parallel first calls spawn exactly one daemon — verify with a fan-out stress test.
- [ ] **Trace continuity:** Per-call CLI invocation produces a continuous trace — verify a `tools/call` span exists end-to-end.
- [ ] **Deadline alignment:** Daemon budget fires before CLI client timeout — verify a long op returns typed `Timeout`, not a client cancel.

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| Daemon spawn race shipped | MEDIUM | Add cross-process `flock`; add fan-out stress test; reap orphan daemons on detect |
| Profile/mode filtering lost | HIGH | Retrofit daemon-side `tools/call` enforcement; re-run/golden the profile-mode suite — security-sensitive, prioritize |
| Stale MCP configs breaking users | MEDIUM | Ship a `helix setup --migrate` patch release; first-run warn; CHANGELOG notice |
| Un-greppable/unstable output | LOW–MEDIUM | Add sort-before-print + plain-default + loud-truncation; re-golden outputs |
| SKILL.md not triggering | LOW | Rewrite description as trigger with key terms + anti-triggers; re-measure with behavioral oracle |
| Nudge false positives | LOW | Swap substring matcher for argument-aware parsing; add fixture corpus |
| docgen drift | LOW | Generate CLI + docgen from one registry; add three-way parity test |
| Multi-oracle harness stale | MEDIUM | Add CLI subprocess oracle; re-target contract goldens to stdout; demote MCP tests to engine units |

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| 1. Per-invocation cold-start tax | Phase 90 (daemon-dial / warm-reuse) + CLI-latency bench | 2nd-call p95 SLO benchmark; LazyInit fires only on session-first |
| 2. Daemon-start race / connect storm | Phase 90 (daemon-dial foundation) | Cross-process lock; fan-out/synctest stress test; single-daemon assertion |
| 3. Profile/mode filter regression | CLI-parity phase (daemon `tools/call` enforcement) | Profile/mode golden suite re-pointed at CLI + daemon refusal test |
| 4. Lost deadlines/telemetry/suggestion | CLI-parity phase | Daemon suggestion/typed-error survives to stderr; trace continuity; deadline alignment |
| 5. Un-greppable/unstable output | Terse-output contract phase (load-bearing) | Golden ordering, no-ANSI, loud-truncation, `Kind`-prefixed errors |
| 6. SKILL.md trigger failure | SKILL.md authoring phase | LLM behavioral oracle tool-selection pass-rate gate |
| 7. Nudge-hook hazards | Nudge-hook phase | Fixture corpus (code-grep vs log/doc/non-grep) asserts true-positive-only firing |
| 8. Breaking MCP client configs | `helix setup` flip phase | Per-client setup/teardown idempotency test (all 7 clients) |
| 9. Lost HTTP multi-client/remote | MCP-surface-removal phase | Explicit remote-scope decision record + Windows local-dial smoke |
| 10. docgen tool-table drift | Identity/docs-rewrite phase | Three-way registry↔CLI↔docgen count/name parity test |
| 11. Multi-oracle harness gap | Testing-migration phase (late) | CLI subprocess oracle replacing MCP protocol/contract oracles |

## Sources

- Helix codebase (HIGH — primary): `internal/forwarder/dial.go` (autostart/poll, no cross-process lock, keepalive, otelgrpc handler), `internal/cli/nudge.go` (substring matcher, advisory exit-0, session stats), `internal/cli/setup_hooks.go` (idempotent `helix_managed` hook pattern, PreToolUse matcher), `internal/errors/kinds.go` (9-kind taxonomy, `subsystem_disabled:` greppable convention, guardrail detail), CLAUDE.md (middleware stack, LIFO order, profile/mode filtering on `tools/list`, docgen "do not hand-edit"), `.planning/PROJECT.md` v2.0 milestone (locked architecture, target features, out-of-scope), MEMORY.md (docgen 53-vs-47 drift history; bench false-green skip).
- [gopls: Running as a daemon](https://go.dev/gopls/daemon) and [golang/go#48844 — slow startup even with shared cache](https://github.com/golang/go/issues/48844) (MEDIUM) — warm-daemon socket connect can still stall 10s+ when the *workspace* wasn't pre-warmed; daemon mode addresses memory sharing, not per-invocation connection latency.
- [Anthropic — Equipping agents with Agent Skills](https://www.anthropic.com/engineering/equipping-agents-for-the-real-world-with-agent-skills), [Skill authoring best practices](https://docs.claude.com/en/docs/agents-and-tools/agent-skills/best-practices), [Extend Claude with skills](https://code.claude.com/docs/en/skills) (HIGH — official) — description is the trigger; be specific with key terms + concrete triggers; third person; body <500 lines / ~1,500–2,000 words; progressive disclosure.

---
*Pitfalls research for: CLI-over-warm-daemon migration + MCP agent-surface retirement (Helix v2.0)*
*Researched: 2026-06-21*

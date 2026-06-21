# Phase 91: Code-Generated Verb Surface + `tools/call` Profile/Mode Enforcement - Research

**Researched:** 2026-06-21
**Domain:** Go codegen (cobra subcommands from typed `*Args` structs), MCP-SDK receiving-middleware authz at the `tools/call` boundary, CI drift gates
**Confidence:** HIGH (all claims grounded in `[VERIFIED: codebase]` reads with file:line; no external packages introduced)

## Summary

Phase 91 has two intertwined deliverables that MUST land together. (1) **Codegen verb surface:** every callable tool in the live registry (53, reconciled at v1.12) gets a generated `helix <verb>` cobra subcommand emitted into committed `*_gen.go`, behind a `helix-cligen --check` drift gate that mirrors the existing `cmd/docgen --check` pattern. The Phase 90 spine (`internal/cli/verb.go`) already has the exact runtime shape the generated verbs plug into: a `verbSpec` registry, `buildVerbArgs` flag→args mapping with **pre-dial** required-flag validation, the `callToolFn` seam, and cobra `AddGroup` scaffolding in `internal/cli/root.go`. The generator's job is to populate `verbSpecs` (the catalog) — the spine itself is done. (2) **`tools/call` enforcement:** today `ProfileFilterMiddleware` filters ONLY `tools/list` (`internal/mcp/middleware.go:509`) and there is **no `tools/call` rejection path** (confirmed by the comment block at `middleware.go:164-166`). The moment generated verbs make every tool always-invocable, a read-mode/ci-bot agent could call destructive edits. The fix is a new receiving middleware modeled byte-for-byte on the existing `GuardrailMiddleware` (`internal/mcp/guardrail_middleware.go`), checking the session's already-resolved `AllowedTools` whitelist and returning a typed `PermissionDenied` error.

The single hardest research finding for **VERB-04** is that **the tool-name → arg-struct-type mapping does not exist yet anywhere in the runtime.** Tools are registered via `mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{Name: "go_to_definition"}, handler-closure-taking GoToDefinitionArgs)` (`internal/kernel/symbols/tools.go:354-357`). The arg struct type is bound only inside the generic `AddTool` type parameter; `mcp.ToolDef` (the thing `skill.ToolProvider.Tools()` returns) carries only `Name/Description/BriefDescription/HelpText/RegisterFn` (`internal/mcp/registry.go:8-17`) — **no arg-type field.** VERB-04 is precisely the task of closing this gap.

**Primary recommendation:** Build `cmd/helix-cligen` mirroring `cmd/docgen`'s blank-import discipline; resolve the tool→args mapping via a **generator-side `go/packages` + AST scan** of the `RegisterTools`/`AddTool` call sites (recommended) OR a hand-maintained `ToolDef.ArgsExample any` field populated at registration (simpler, but reintroduces per-tool manual edits — disfavored by VERB-04 acceptance). Emit `internal/cli/verbs_gen.go` populating `verbSpecs`. Add a `tools/call` profile/mode enforcement middleware (`InstallProfileEnforcementMiddleware`) installed AFTER Guardrail and BEFORE LazyInit; refuse with `serr.New(serr.PermissionDenied, ...)`. Re-point the `testdata/profiles/*.tools.golden` fixtures from MCP `tools/list` to the CLI verb surface.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Enumerate tools by name + arg struct | Build-time generator (`cmd/helix-cligen`) | — | Static catalog must be committed + drift-gated; reflection at runtime cannot run in CI without a daemon |
| Generate cobra subcommands | CLI (`internal/cli/verbs_gen.go`) | — | The verb surface is a CLI concern; spine already lives in `internal/cli/verb.go` |
| Flag validation (required-before-dial) | CLI (`buildVerbArgs`) | — | SC#3 demands the error before any network I/O; CLI-local, no daemon |
| Profile/mode authz on invocation | Daemon middleware (`internal/mcp/`) | — | Authz MUST be server-side; a CLI-only check is bypassable. The CLI never sees the profile/mode authoritatively — the daemon session does |
| Typed-error surfacing to CLI | Daemon (returns typed err) → MCP wire → CLI (`forwarder.CallTool`) | — | Error originates daemon-side, propagates as JSON-RPC error, recovered CLI-side |
| Per-profile verb-surface goldens | Test tier (`testdata/profiles/`, `test/integration/`) | — | Contract oracle; checked-in golden is source of truth |

## Standard Stack

No new external packages. This phase is entirely in-tree Go using already-vendored libraries.

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/spf13/cobra` | v1.10.2 | Subcommand + flag + `AddGroup` grouping | Already the CLI framework (`go.mod`); 90-02 established the group scaffold `[VERIFIED: go.mod, internal/cli/root.go:83-87]` |
| `github.com/modelcontextprotocol/go-sdk/mcp` | (vendored) | `AddReceivingMiddleware`, `CallToolRequest`, typed handler args | The daemon's dispatch + middleware engine; SEC enforcement is a new middleware on it `[VERIFIED: internal/mcp/middleware.go:133-141]` |
| `golang.org/x/tools/go/packages` + `go/ast` | (stdlib + x/tools) | Generator AST scan of `AddTool` call sites to map tool name → `*Args` type | The robust way to recover the name↔args binding the runtime drops (VERB-04) — see Pitfall 1 |
| `internal/errors` (alias `serr`) | in-tree | Typed-error taxonomy; `PermissionDenied` kind already exists | SEC-01 typed refusal `[VERIFIED: internal/errors/kinds.go:31]` |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `go/format` (stdlib) | stdlib | gofmt the generated `*_gen.go` so the drift gate compares formatted bytes | Always — generated output must be `gofmt`-stable or `--check` flaps |
| `reflect` (stdlib) | stdlib | Alternative arg-struct field enumeration IF a `ToolDef.ArgsExample any` is added | Only if AST scan is rejected (disfavored) |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| AST scan to recover tool→args | `ToolDef.ArgsExample any` field set at each `AddTool` site, then `reflect` over it | Simpler generator, but adds one manual field per tool registration → violates VERB-04 "no manual per-tool edits" spirit and re-opens the v1.12 drift class. Use only as fallback. |
| New enforcement middleware | Extend `ProfileFilterMiddleware` to also handle `tools/call` | Mixing list-filtering and call-authz in one middleware muddies the LIFO ordering reasoning; a dedicated middleware (like Guardrail) is cleaner and testable in isolation |
| `helix <verb>` flat | keep the Phase-90 `helix call <verb>` parent | ROADMAP/CONTEXT say `helix <verb>` (no `call` parent) — the generated verbs attach to the ROOT command with group IDs, not under `call`. See Open Question 1. |

**Installation:** None — no `go get`.

**Version verification:** `cobra v1.10.2` confirmed in `go.mod` `[VERIFIED: codebase grep]`. No registry lookups needed (zero new deps).

## Package Legitimacy Audit

**Not applicable — this phase installs zero external packages.** All work uses already-vendored deps (`cobra v1.10.2`, the MCP Go SDK) and stdlib (`go/ast`, `go/format`, `reflect`, `golang.org/x/tools` is already in the module graph via existing codegen tooling). No `go get`, no new `require` lines expected. If the planner finds a task proposing a new dependency, gate it behind a `checkpoint:human-verify` and run `gsd-tools query package-legitimacy check` first.

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| VERB-01 | Every live-registry tool has a `helix <verb>` subcommand, generated from typed `*Args` | Registry enumerable via `skill.ToolProviders()` → `tp.Tools()` → `[]*mcp.ToolDef.Name` (`cmd/docgen/main.go generateToolTable`); parity test counts generated verbs == registry names. Arg structs recovered by generator (VERB-04). |
| VERB-02 | `helix-cligen` emits committed `*_gen.go`; `--check` drift gate | Mirror `cmd/docgen --check` (`cmd/docgen/main.go:48, regenerate→compare→os.Exit(1)`); add Make target alongside `verify-*` family |
| VERB-03 | `helix --help` groups verbs by capability; required flag errors before dial | `AddGroup` scaffold exists (`root.go:83-87`); `buildVerbArgs` already errors on missing required flag BEFORE `callToolFn` (`verb.go:135-170, 175-184`) |
| VERB-04 | Tool-name → arg-struct mapping available to generator | **GAP: does not exist today.** `ToolDef` has no arg-type field (`registry.go:8-17`); arg type bound only in `AddTool` generic call. Generator-side AST scan recommended. |
| SEC-01 | Profile/mode enforced on `tools/call`, typed refusal | **Confirmed gap** (`middleware.go:164-166, 509`). New middleware modeled on `GuardrailMiddleware`; refuse with `serr.PermissionDenied` (`kinds.go:31`) |
| SEC-02 | CLI honors resolved profile tool-subset; verbs hidden + refused | Session already carries resolved `AllowedTools` (`daemon.go:709-716`, `session.go:33`); per-profile goldens at `testdata/profiles/*.tools.golden` |

## Architecture Patterns

### System Architecture Diagram

```
                      BUILD TIME (CI / make)
  ┌──────────────────────────────────────────────────────────────┐
  │  cmd/helix-cligen  (blank-imports == daemon/imports.go)       │
  │    1. skill.ToolProviders() ──► tool NAMES (53)               │
  │    2. go/packages + AST scan of AddTool call sites            │
  │         └─► map[toolName] = *Args struct type + json/jsonschema tags
  │    3. render cobra subcommand per tool ──► verbs_gen.go       │
  │    4. --check: regenerate to buffer, diff committed, exit 1   │
  └──────────────────────────────────────────────────────────────┘
                              │ commits
                              ▼
                      RUN TIME
  agent ──$ helix replace-symbol-body --path … --new-body …
            │
            ▼ (cobra dispatch; generated subcommand on ROOT, grouped)
  internal/cli/verbs_gen.go → verbSpecs[verb]
            │
            ▼ buildVerbArgs  ──[missing required flag]──► ERROR (exit) ◄── BEFORE any dial (SC#3)
            │ args map[string]any
            ▼ callToolFn = forwarder.CallTool
  forwarder.ConnectOrStartDaemon (race-safe, 90-01) ─► StreamMCP (gRPC, zero-proto)
            │
            ▼  DAEMON  MCP-SDK receiving middleware (LIFO exec order):
  LazyInit ─► **ProfileEnforce(NEW)** ─► Guardrail ─► Suggestion ─► ProfileFilter ─► Telemetry ─► handler
                    │
                    ├─ method==tools/call & tool ∉ session.AllowedTools
                    │     └─► return serr.New(PermissionDenied,…)  ──► JSON-RPC error
                    └─ else next()
            │
            ▼ error propagates over MCP wire
  forwarder.CallTool returns non-nil err ─► runVerb ─► typed stderr + non-zero exit
```

### Recommended Project Structure
```
cmd/
└── helix-cligen/          # NEW: generator (mirror of cmd/docgen)
    ├── main.go            #   blank-imports == internal/daemon/imports.go (+ health, help)
    ├── scan.go            #   go/packages AST scan: AddTool callsite → (name, argsType)
    └── render.go          #   emit verbs_gen.go (gofmt'd)
internal/cli/
├── verb.go                # EXISTING spine: verbSpec, buildVerbArgs, callToolFn, groups; 91-01 adds VerbToolNames() accessor
├── verbs_gen.go           # NEW (committed, generated): populates verbSpecs catalog
└── verbs_gen_test.go      # parity test: len(verbSpecs) == registry name count
internal/mcp/
├── profile_enforce.go     # NEW: ProfileEnforcementMiddleware (mirror guardrail_middleware.go)
└── profile_enforce_test.go
testdata/profiles/         # EXISTING goldens — re-pointed to verb surface (SEC-02)
```

### Pattern 1: `tools/call` enforcement middleware (mirror GuardrailMiddleware)
**What:** A receiving middleware that early-outs on non-`tools/call`, extracts the tool name, reads the session's `AllowedTools` snapshot, and refuses with a typed error if the tool is not in the set.
**When to use:** SEC-01/SEC-02 — this is THE security gate.
**Example (model directly on the verified Guardrail pattern):**
```go
// Source: pattern verified in internal/mcp/guardrail_middleware.go:74-167
func ProfileEnforcementMiddleware(getSession func(ctx context.Context) *SessionInfo, logger *slog.Logger) mcpsdk.Middleware {
    return func(next mcpsdk.MethodHandler) mcpsdk.MethodHandler {
        return func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
            if method != "tools/call" {
                return next(ctx, method, req)
            }
            ctr, ok := req.(*mcpsdk.CallToolRequest)
            if !ok || ctr == nil || ctr.Params == nil {
                return next(ctx, method, req)
            }
            sess := getSession(ctx)
            if sess == nil {
                return next(ctx, method, req) // no session → no whitelist → allow (matches ProfileFilter nil-session behavior, middleware.go:514)
            }
            snap := sess.Snapshot()           // consistent profile+mode+AllowedTools read (session.go:53)
            if snap.AllowedTools == nil {
                return next(ctx, method, req) // nil whitelist == "all tools" (baseline_test.go:27)
            }
            allowed := false
            for _, n := range snap.AllowedTools {
                if n == ctr.Params.Name { allowed = true; break }
            }
            if !allowed {
                // SEC-01 typed refusal — surfaces as a non-nil err from session.CallTool CLI-side.
                return nil, serr.New(serr.PermissionDenied,
                    fmt.Sprintf("tool %q is not available in profile=%s mode=%s", ctr.Params.Name, snap.Profile, snap.Mode)).
                    WithTool(ctr.Params.Name)
            }
            return next(ctx, method, req)
        }
    }
}
```
**Install ordering (CRITICAL):** Install AFTER Guardrail (`daemon.go:883`) and BEFORE LazyInit (`daemon.go:942`). Because `AddReceivingMiddleware` composes LIFO, install order `Telemetry → ProfileFilter → Suggestion → Guardrail → ProfileEnforce → LazyInit` yields exec order `LazyInit → ProfileEnforce → Guardrail → Suggestion → ProfileFilter → Telemetry → handler`. **LazyInit MUST stay last-installed/first-executed** (the documented invariant at `lazy_init.go:106-112`). Putting ProfileEnforce just-inside LazyInit means the workspace is activated but the authz check runs before the (more expensive) guardrail receipt evaluation — refuse cheaply. `[CITED: internal/mcp/guardrail_middleware.go:51-62 install-order doc]`

### Pattern 2: generator blank-import discipline (v1.12 lesson)
**What:** `cmd/helix-cligen` MUST blank-import the SAME skill packages as `internal/daemon/imports.go` so `skill.ToolProviders()` enumerates the identical 53-tool set the daemon registers.
**When:** Always — this is the docgen-drift lesson.
**Example:** Copy the import block verbatim from `cmd/docgen/main.go:22-32` (which already adds `health` + `help` over the daemon's set). `[VERIFIED: cmd/docgen/main.go:22-32 vs internal/daemon/imports.go]`
> Note a real divergence to reconcile: `cmd/docgen` blank-imports `internal/kernel/health` and `internal/kernel/help`, which `internal/daemon/imports.go` does NOT (the daemon constructs those kernel-resident tools explicitly, not via init()). The generator must enumerate the daemon's LIVE surface — confirm whether health/help register via init() or explicit construction, and mirror whichever path produces the runtime 53. See Open Question 2.

### Pattern 3: `--check` drift gate (mirror docgen)
**What:** Generate to an in-memory buffer, compare against the committed `verbs_gen.go`, `os.Exit(1)` with a remediation message on mismatch.
**Example:**
```go
// Source: cmd/docgen/main.go:48, 70-75 (verified pattern)
if *check {
    if generated != committed {
        fmt.Fprintln(os.Stderr, "verbs_gen.go is out of date. Run 'go run ./cmd/helix-cligen' to regenerate.")
        os.Exit(1)
    }
}
```
Wire a `make verify-cligen` (or fold into `make docs`/CI) alongside the existing `verify-*` targets (`Makefile:1`).

### Anti-Patterns to Avoid
- **CLI-side-only authz:** Filtering verbs in the CLI process is bypassable (a user can craft a raw `tools/call`). Authz MUST be server-side in the daemon middleware. The CLI hiding of out-of-profile verbs (SEC-02 "hidden") is UX; the daemon refusal (SEC-01 "refused") is the security boundary. Ship BOTH.
- **Returning `IsError: true` result instead of an `error` for refusal:** The Guardrail block path returns a typed `error` (`guardrail_middleware.go:142`), which surfaces as a non-nil err from `session.CallTool` → `forwarder.CallTool` wraps it → CLI gets a clean typed failure + non-zero exit. An `IsError` *result* (like LazyInit uses at `lazy_init.go:92-97`) is for in-band tool failures, not authz denials — use the `error` path so `errors.Is(err, serr.ErrPermissionDenied)` works.
- **Hardcoding the verb count (53):** VERB-01 acceptance is generated-count == live-registry-count enumerated BY NAME (CONTEXT.md / STATE.md line 67). Never assert `== 53`.
- **Flag name verbatim == tool arg name:** Several tools use snake_case arg keys (`max_results`, `context_lines`) mapped to kebab flags (`--max-results`); the spine's `toolArg` indirection (`verb.go:39-52`) already handles this. The generator must emit kebab-case flag names and the snake_case `toolArg` from the json tag.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Subcommand grouping in help | Custom help formatter | cobra `AddGroup` + `GroupID` | Already scaffolded (`root.go:83-87`); 90-02 chose group IDs leaving room for the 6 capability groups |
| Flag→args mapping + required check | New flag layer | Existing `buildVerbArgs` + `verbFlag{required,toolArg,kind}` | Spine done + unit-tested (`verb.go:135-170`, `verb_test.go`); generator just fills `verbSpecs` |
| One-shot dial / daemon autostart | New transport | `forwarder.CallTool` (`callToolFn` seam) | 90-03 built the client transport mirror + race-safe dial; zero-proto |
| Typed error taxonomy + stderr surfacing | New error type | `serr.PermissionDenied` + `errors.Is`/`errors.As` | Kind exists (`kinds.go:31`); Guardrail proves the wire round-trip |
| Drift gate | Custom diff tool | `cmd/docgen --check` pattern + `go/format` | Identical mechanics; CI already runs docgen check |
| Recovering tool→args binding | Manual per-tool table | `go/packages` AST scan (or, fallback, a `ToolDef.ArgsExample` reflected) | Manual table is the exact drift hazard VERB-04 forbids |

**Key insight:** ~90% of this phase is *wiring already-built primitives*, not new construction. The spine, the dial path, the group scaffold, the typed errors, the goldens, and the `--check` pattern all exist. The two genuinely NEW pieces are (a) the generator's name↔args recovery (VERB-04, the only hard problem) and (b) the enforcement middleware (a near-verbatim Guardrail clone).

## Runtime State Inventory

> Not a rename/refactor/migration phase. This phase ADDS a generator, generated files, and one middleware; it does not rename stored state. The one relevant "runtime state" concern: the generated `verbs_gen.go` is a committed build artifact whose staleness is caught by the `--check` gate (no external datastore involved).

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | None — verified: no DB/collection keyed on verb names | None |
| Live service config | None — the daemon reads profiles from embedded YAML (`internal/profile/embed.go`), no external service | None |
| OS-registered state | None | None |
| Secrets/env vars | None new (the `HELIX_SOCKET` hook from 90-04 is unchanged) | None |
| Build artifacts | `internal/cli/verbs_gen.go` (NEW committed generated file) — stale after any `*Args` edit | Regenerate via `helix-cligen`; `--check` gate enforces (VERB-02) |

## Common Pitfalls

### Pitfall 1: VERB-04 — the tool→args binding is not recorded anywhere
**What goes wrong:** A naive generator iterates `skill.ToolProviders()` getting NAMES, but `mcp.ToolDef` has no field pointing at the `*Args` struct, so the generator cannot derive flags.
**Why it happens:** Registration splits the binding: `mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{Name:"go_to_definition"}, handler(... args GoToDefinitionArgs))` binds the type only inside the generic `AddTool` (`symbols/tools.go:354-357`), while `server.Registry().Register(&mcp.ToolDef{Name:"go_to_definition", ...})` (`tools.go:377`) records the name with NO type. `[VERIFIED: internal/kernel/symbols/tools.go:353-378, internal/mcp/registry.go:8-17]`
**How to avoid:** Recover the binding at generate time with a `go/packages` load + `go/ast` walk of every `AddTool(...)` call expression: the 3rd argument's func literal's last param type is the `*Args` struct; the 2nd argument's `&mcpsdk.Tool{Name: "..."}` composite literal gives the matching name. Then read the struct's fields' `json:"..."` and `jsonschema:"..."` tags to emit `verbFlag`s. Fallback: add `ArgsExample any` to `ToolDef`, set it at each registration, and `reflect.TypeOf` it (rejected if VERB-04's "no manual per-tool edits" is read strictly).
**Warning signs:** Generator emits verbs with zero flags, or a flag count that doesn't match the struct.

### Pitfall 2: nested / complex arg-struct field types
**What goes wrong:** `ReplaceSymbolBodyArgs.Receipts` is `[]guardrails.ReceiptID` and other tools carry maps/slices (`edit/tools.go:93`). The spine only supports `flagString/flagInt/flagBool` (`verb.go:23-29`).
**Why it happens:** cobra flags are scalar by nature; non-scalar tool args (slices, structs, maps) have no obvious single flag mapping.
**How to avoid:** Define a generator policy: (a) `[]string`/scalar-slices → `StringSlice` flag (extend `flagKind`); (b) opaque/complex types (`[]guardrails.ReceiptID`, maps) → emit a `--<name>-json` raw-JSON flag OR omit from the generated flag set and document. Decide the policy explicitly so the generator is deterministic. The `Receipts` field is `omitempty` (`edit/tools.go:93`) so it is non-required — safe to default to a `--receipts` JSON flag. `[VERIFIED: internal/kernel/edit/tools.go:88-93]`
**Warning signs:** generator panics on an unhandled `reflect.Kind`, or compile error in `verbs_gen.go`.

### Pitfall 3: flag-name collisions across tools — NOT a real risk here
**What goes wrong:** Worry that two tools both define `--path`.
**Why it's NOT a problem:** Each verb is a SEPARATE cobra subcommand with its OWN flag set (`newVerbSubcommand` calls `sub.Flags()`, `verb.go:119-128`). Flags are per-subcommand-scoped, so `helix go-to-definition --path` and `helix replace-symbol-body --path` never collide. The only collision surface is reserved ROOT persistent flags (`--socket`, `--json`, `--profile`, etc., `root.go:90-118`) — the generator must NOT emit a verb flag whose name shadows a persistent root flag. Maintain a small reserved-name denylist in the generator.
**Warning signs:** cobra panics "flag redefined" at startup → a generated verb flag clashed with a root persistent flag.

### Pitfall 4: the `--check` gate flaps because output isn't gofmt-stable
**What goes wrong:** Map iteration order (`verbSpecs` is a `map`, `verb.go:68`) makes generated output non-deterministic; CI `--check` fails intermittently.
**How to avoid:** Sort tool names before emitting; run `go/format.Source` on the generated bytes before writing/comparing (docgen compares post-transform strings, `docgen/main.go:70`). Emit a `// Code generated by helix-cligen; DO NOT EDIT.` header so `gofmt`/linters treat it as generated.
**Warning signs:** `--check` passes locally, fails in CI (or vice versa) with a whitespace/ordering-only diff.

### Pitfall 5: SC#3 — required-flag error must precede the dial
**What goes wrong:** Validation happens after the daemon is dialed, so a cold autostart spins up before the obvious "missing --path" error.
**Why it's already handled:** `runVerb` calls `buildVerbArgs` (which returns the required-flag error) BEFORE `callToolFn` (`verb.go:175-184`); `buildVerbArgs` does NO network I/O (`verb.go:135-170`). The generator must preserve this ordering — it does automatically by reusing the spine's `runVerb`. Add a hermetic test (no daemon) asserting a missing required flag errors with the `callToolFn` seam swapped to a fail-if-called fake (the 90-03 `verb_test.go` already demonstrates this seam). `[VERIFIED: internal/cli/verb.go:175-184, 135-170]`
**Warning signs:** a daemon process appears (or a dial timeout) when invoking a verb with a missing required flag.

### Pitfall 6: how profile/mode reaches the daemon for a one-shot call
**What goes wrong:** Assuming the CLI must SEND the profile/mode with each `tools/call`.
**Reality:** Profile/mode is **daemon-side session state**, resolved at daemon bootstrap from `cfg.Profile` + `DefaultMode` into `SessionInfo{Profile, Mode, AllowedTools}` (`daemon.go:700-716`) and mutated only by `switch_mode` (`profile/skill.go:178-179`). The one-shot CLI call rides the SAME daemon session; the enforcement middleware reads `getSession(ctx)` exactly as Guardrail/Telemetry do (`daemon.go:821-823`). So the CLI does NOT pass profile/mode per call — the daemon's resolved `AllowedTools` is authoritative. **Implication for SEC-01 testing:** to test "read mode refuses replace-symbol-body," the test daemon must be started with read mode / a read-only profile (the `StartTestDaemon(Options{Profile:...})` harness, `test/integration/profile_golden_test.go:16-20`), then a `helix` subprocess (or `forwarder.CallTool`) attempts the destructive verb and must get a `PermissionDenied`. `[VERIFIED: internal/daemon/daemon.go:700-716, 821-828; internal/profile/skill.go:124-186]`

## Code Examples

### Enumerating the live registry by name (generator step 1)
```go
// Source: cmd/docgen/main.go generateToolTable (verified)
import "github.com/agenthands/helix/internal/skill"
for _, tp := range skill.ToolProviders() {
    for _, tool := range tp.Tools() { // []*mcp.ToolDef
        names = append(names, tool.Name) // VERB-01 enumeration source of truth
    }
}
```

### Destructive-verb set (already enumerated for Guardrail — reuse for SEC tests)
```go
// Source: internal/mcp/guardrail_middleware.go:172-188 (verified, D-08 LOCKED closed-enum)
// rename_symbol, safe_delete_symbol, replace_symbol_body, fuzzy_edit, replace_in_file, delete_file
```
The read mode's `exclude_tools` (`internal/profile/modes/read.yaml`) and ci-bot's (`profiles/ci-bot.yaml`) already exclude the edit tools, so a read/ci-bot session's `AllowedTools` will not contain `replace_symbol_body` — the enforcement middleware refuses it. SC#4's "succeeds under edit mode" holds because edit mode's resolved set includes it. `[VERIFIED: internal/profile/modes/read.yaml, internal/profile/profiles/ci-bot.yaml]`

### Re-pointing per-profile goldens (SEC-02)
```go
// Existing oracle: test/integration/profile_golden_test.go:11-26 (verified)
//   tools := listSessionTools(t, td.Session)            // currently: MCP tools/list
//   assertGoldenTools(t, p+"."+defMode, tools)          // vs testdata/profiles/<p>.<mode>.tools.golden
// Phase 91 re-point: drive the CLI verb surface for that profile and assert the
// same golden — the verb list visible+invokable under profile p == the golden set.
// The CLI verb catalog is read via the 91-01-exported internal/cli.VerbToolNames()
// accessor (91-03 owns only test/integration files; the seam lives in 91-01).
// The golden FILES (e.g. testdata/profiles/ci-bot.read.tools.golden, 25 tools)
// already encode the per-profile allowed set; reuse them as the CLI-surface oracle.
```
`[VERIFIED: test/integration/profile_golden_test.go, testdata/profiles/ci-bot.read.tools.golden]`

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `tools/list` filtering as the only profile gate | `tools/call` enforcement is now required | Phase 91 (this) | Always-visible verbs make list-filtering insufficient; the call-time gate is the new security boundary |
| One representative verb (`search`) in `verbSpecs` | Full generated catalog | Phase 91 | 90-03 shipped exactly one verb as a spine proof (`verb.go:62-86`); this phase generates all |
| `helix call <verb>` parent | `helix <verb>` on root (per ROADMAP) | Phase 91 | The `call` parent (`verb.go:92-106`) may be flattened — see Open Question 1 |

**Deprecated/outdated:**
- The `middleware.go:164-166` comment ("Reintroduce a deny bucket here if per-call filtering lands in v1.3") — Phase 91 IS that landing. The `outcomeInvalidArgs`/`not_found` TODOs remain out of scope, but a refusal outcome may want a metric bucket (see Open Question 3).

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `helix <verb>` attaches generated subcommands to the ROOT command (not under a `call` parent) | Alternatives / SOTA | If the `call` parent is kept, help grouping + docgen enumeration change shape; LOW risk (ROADMAP text is explicit about `helix <verb>`) |
| A2 | A `go/packages` AST scan can robustly recover all 53 name↔args bindings | Pitfall 1 / Stack | If some tools register through an indirection the AST can't follow, those verbs lack flags; MEDIUM — mitigated by a parity test that fails if any verb has unexpected zero flags |
| A3 | Returning a typed `error` (not IsError result) from the new middleware round-trips to the CLI as a recoverable typed failure | Pattern 1 / Anti-patterns | If the SDK swallows middleware errors into a generic result, `errors.Is(PermissionDenied)` CLI-side breaks; LOW — Guardrail's block path proves the round-trip (`guardrail_middleware_test.go:246-258`) |
| A4 | health/help tools are part of the live 53 and must be enumerated by the generator's imports | Pattern 2 / Open Q2 | If the generator's blank imports miss them, parity test fails (caught early); LOW |

## Open Questions (RESOLVED)

1. **`helix <verb>` flat vs `helix call <verb>`?**
   - What we know: ROADMAP/CONTEXT/REQUIREMENTS all write `helix <verb>` (e.g. `helix replace-symbol-body`, `helix find-symbol`). The 90-03 spine uses a `call` parent (`verb.go:92`).
   - What's unclear: whether to flatten (attach generated subcommands directly to root with `GroupID`) or keep `call`.
   - Recommendation: **Flatten to root** to match the documented `helix <verb>` UX and the 6 capability groups; the generator emits `rootCmd.AddCommand(grouped(verb))`. Keep `call` only if a namespacing need surfaces. PLAN should lock this in task 1.
   - **RESOLVED: Flatten verbs onto root grouped by capability — locked in 91-01 Task 2 (Open Q1), which replaces the `call` parent with `registerGeneratedVerbs(rootCmd)` + the 6 capability `cobra.Group`s; the flatten smoke test in 91-01 Task 3 asserts a generated verb is a direct root child with its GroupID.**

2. **Do health/help register via init() (so blank-import enumerates them) or only via explicit daemon construction?**
   - What we know: `cmd/docgen` blank-imports `internal/kernel/health` + `internal/kernel/help`; `internal/daemon/imports.go` does NOT (the daemon constructs them explicitly). Both appear in the goldens (`get_health`, `get_tool_help` in `ci-bot.read.tools.golden`).
   - What's unclear: which import set yields exactly the runtime 53 for the generator.
   - Recommendation: copy `cmd/docgen`'s import block (it already reconciled this for the README table), then assert `len(generated) == registry count` to catch any miss. `[VERIFIED: cmd/docgen/main.go:22-32; internal/daemon/imports.go]`
   - **RESOLVED: Copy cmd/docgen's blank-import block VERBATIM — locked in 91-01 Task 1 (action: "blank-import block COPIED VERBATIM from cmd/docgen/main.go:21-32 ... so init()-registration enumeration matches the daemon's live surface"); the 91-01 Task 3 parity test (`len(verbSpecs) == liveCount` BY NAME) fails CI if the import set misses any tool.**

3. **Should a refused `tools/call` get its own telemetry outcome bucket?**
   - What we know: the outcome enum has no `permission_denied` value (`middleware.go:173-200`); a refusal currently classifies as `outcomeInternal` via `classifyOutcome` (`middleware.go:268-298`).
   - Recommendation: optionally add `outcomePermissionDenied` and classify it (mirrors the Phase 66 guardrail-blocked addition); NICE-TO-HAVE, not required by SEC-01/02 acceptance.
   - **RESOLVED: OUT OF SCOPE for Phase 91 — the telemetry bucket is a NICE-TO-HAVE not required by SEC-01/SEC-02 acceptance; 91-02 (the enforcement middleware) ships the typed `PermissionDenied` refusal without adding an `outcomePermissionDenied` bucket. Deferred (revisit with the `outcomeInvalidArgs`/`not_found` TODOs if a metrics need surfaces).**

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | build, codegen, tests | ✓ | (host) | — |
| `golang.org/x/tools/go/packages` | generator AST scan | ✓ (in module graph via existing codegen) | — | `reflect` over `ToolDef.ArgsExample` (manual field) |
| cobra | CLI | ✓ | v1.10.2 | — |
| MCP Go SDK | daemon middleware | ✓ | vendored | — |
| `HELIX_BIN` + built `helix` | E2E security-refusal subprocess test | ✓ when built | — | gated-skip like 90-04 (`go:build !windows`, `HELIX_BIN`-gated) |

**Missing dependencies with no fallback:** None.
**Missing dependencies with fallback:** Generator AST scan can fall back to a reflected `ToolDef.ArgsExample` if `go/packages` proves awkward (disfavored — manual edits).

## Validation Architecture

> `workflow.nyquist_validation` not set to false in `.planning/config.json` → section included. This phase strongly warrants it (parity, drift, security-refusal, per-profile goldens).

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go `testing` (stdlib) + `go test` |
| Config file | none (standard Go) |
| Quick run command | `go test ./internal/cli/... ./internal/mcp/... -count=1` |
| Full suite command | `go test ./...` then `HELIX_BIN="$(pwd)/helix" go test ./internal/cli/ -run CLI_ -count=1` |
| Build gate | `go build ./cmd/helix ./cmd/helix-cligen && go vet ./...` (per CLAUDE.md) |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| VERB-01 | generated verb count == live registry name count | unit | `go test ./internal/cli/ -run VerbsParity -x` | ❌ Wave 0 (`verbs_gen_test.go`) |
| VERB-02 | editing an `*Args` without regen fails `--check` | integration | `go run ./cmd/helix-cligen --check` (CI) + a test mutating a fixture | ❌ Wave 0 |
| VERB-03 | grouped help renders; missing required flag errors pre-dial | unit | `go test ./internal/cli/ -run 'VerbGroups|RequiredBeforeDial'` | ⚠️ partial (`verb_test.go` has pre-dial test; extend) |
| VERB-04 | generator resolves arg struct for every tool, no manual edits | unit | `go test ./cmd/helix-cligen/ -run ScanAllTools` | ❌ Wave 0 |
| SEC-01 | replace-symbol-body refused under read, succeeds under edit, typed | unit + E2E | `go test ./internal/mcp/ -run ProfileEnforce`; `HELIX_BIN=… go test ./internal/cli/ -run CLI_SecRefusal` | ❌ Wave 0 |
| SEC-02 | per-profile verb surface matches re-pointed goldens | integration | `go test -tags integration ./test/integration/ -run Profile_Contract` | ✅ exists — re-point oracle |

### Sampling Rate
- **Per task commit:** `go test ./internal/cli/... ./internal/mcp/... -count=1 && go vet ./...`
- **Per wave merge:** full `go test ./...` + `go run ./cmd/helix-cligen --check`
- **Phase gate:** full suite green + HELIX_BIN-gated E2E security-refusal test RAN (not skipped) before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `internal/cli/verbs_gen_test.go` — VERB-01 parity (count by name), VERB-04 (every verb has resolved flags)
- [ ] `cmd/helix-cligen/scan_test.go` — AST scan recovers name↔args for all tools
- [ ] `internal/mcp/profile_enforce_test.go` — SEC-01 refuse/allow + typed-error (`errors.Is(PermissionDenied)`), mirror `guardrail_middleware_test.go:221-258`
- [ ] `internal/cli/cli_e2e_test.go` extension — `HELIX_BIN`-gated `CLI_SecRefusal` (read-mode daemon refuses destructive verb)
- [ ] re-point `test/integration/profile_golden_test.go` oracle to the CLI verb surface
- [ ] `make verify-cligen` target + CI wiring for the `--check` drift gate

## Security Domain

> `security_enforcement` absent → enabled. This phase IS a security phase (SEC-01/02). Helix is Go-native; no `java-security` SMTC capability applies (per CLAUDE.md).

### Applicable ASVS Categories
| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V1 Architecture | yes | Authz enforced at the trust boundary (daemon middleware), not the bypassable client (CLI) |
| V4 Access Control | **yes (core)** | Profile/mode `AllowedTools` whitelist enforced on every `tools/call`; deny-by-membership; typed refusal |
| V5 Input Validation | yes | Required-flag validation pre-dial (`buildVerbArgs`); generator emits typed flags from json/jsonschema tags |
| V7 Error Handling | yes | Typed `PermissionDenied` (no message-string matching); stable error kind for agent branching |
| V2/V3/V6 | no | No new auth/session-crypto/secrets surface (one-shot rides existing per-uid unix socket; no TCP added) |

### Known Threat Patterns for {Go daemon + CLI verb surface}
| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Read/ci-bot agent invokes destructive edit verb once verbs are always-visible | Elevation of Privilege | `tools/call` enforcement middleware checks `AllowedTools` → `PermissionDenied` (SEC-01) — **the load-bearing fix; do NOT defer** |
| CLI-side-only verb hiding bypassed by raw `tools/call` | EoP / Spoofing | Server-side daemon middleware is authoritative; CLI hiding is UX only |
| Stale generated catalog hides/exposes wrong verbs | Tampering | `--check` drift gate fails CI on stale `verbs_gen.go` (VERB-02) |
| Refusal leaks internal tool names / args | Information Disclosure | Refusal message names only the requested tool + profile/mode (already public); never echoes other tools' existence |
| Enforcement middleware mis-ordered (runs after handler) | EoP | Install AFTER Guardrail, BEFORE LazyInit; preserve LazyInit-first invariant (`lazy_init.go:106-112`); test the LIFO exec order |

## Sources

### Primary (HIGH confidence — codebase reads, this session)
- `internal/mcp/middleware.go` (ProfileFilter tools/list-only at :509; no-deny-bucket comment :164-166) — the confirmed SEC gap
- `internal/mcp/guardrail_middleware.go` (:74-188) — the verbatim template for the enforcement middleware + destructive-tool enum
- `internal/mcp/registry.go` (:8-17) — `ToolDef` has no arg-type field (VERB-04 gap)
- `internal/mcp/session.go` (:25-69) — `SessionInfo.AllowedTools` + `Snapshot()`
- `internal/cli/verb.go` (full) + `internal/cli/root.go` (full) — the Phase 90 spine + group scaffold
- `internal/kernel/symbols/tools.go` (:340-426) — `AddTool` registration shape; arg type bound only in the generic call
- `internal/kernel/edit/tools.go` (:88-106) — complex arg types (`[]guardrails.ReceiptID`)
- `internal/daemon/daemon.go` (:700-716, 815-955) — session bootstrap, `getSessionFn`, middleware install order
- `internal/daemon/imports.go` + `cmd/docgen/main.go` (:22-75) — blank-import discipline + `--check` drift pattern
- `internal/profile/profile.go`, `internal/profile/skill.go` (:124-186), `internal/config/loader.go` (ResolveProfile) — profile/mode → AllowedTools resolution
- `internal/errors/kinds.go` (:13-48) — typed taxonomy incl. `PermissionDenied`
- `test/integration/profile_golden_test.go` + `testdata/profiles/*.tools.golden` — the re-pointable oracle
- 90-02/03/04 SUMMARYs — spine provenance, seams, E2E harness conventions

### Secondary (MEDIUM)
- ROADMAP.md / REQUIREMENTS.md / STATE.md / 91-CONTEXT.md — requirement IDs, success criteria, carried-forward constraints

### Tertiary (LOW)
- None — no external/web sources needed (no new deps).

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — zero new deps; all libs verified present in `go.mod`
- Architecture (enforcement middleware): HIGH — direct clone of a verified, tested in-tree pattern (Guardrail)
- VERB-04 generator approach: MEDIUM — AST-scan approach is sound but the exact scan robustness across all 53 registration sites is unproven until built (A2)
- Pitfalls: HIGH — each grounded in a specific file:line

**Research date:** 2026-06-21
**Valid until:** 2026-07-21 (stable in-tree domain; revisit only if the tool-registration shape or middleware chain changes)

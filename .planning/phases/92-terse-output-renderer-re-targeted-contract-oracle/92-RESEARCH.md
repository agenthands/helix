# Phase 92: Terse Output Renderer + Re-Targeted Contract Oracle - Research

**Researched:** 2026-06-21
**Domain:** CLI output rendering (terse `relpath:line:col<TAB>payload`), typed-error → exit-code mapping, TTY/color gating, golden-test re-targeting (Go single binary)
**Confidence:** HIGH (all findings verified by direct codebase read; no external packages required)

## Summary

The Phase 90/91 verb spine already issues a one-shot MCP `tools/call` and renders the result via `internal/cli/verb.go:renderResult` (`[VERIFIED: internal/cli/verb.go:247-258]`), which dumps `TextContent` verbatim and any non-text content as compact JSON. **The single most important architectural fact for this phase: every kernel tool already formats its result to plain TEXT on the daemon side** (e.g. `symbols.formatLocations` at `internal/kernel/symbols/tools.go:182-200` emits `file://<abs>/path:line:col — preview` strings, with coordinates already converted to 1-based via `loc.Range.Start.Line + 1`). The CLI therefore receives *opaque, pre-formatted, daemon-flavored text*, not structured location data — and worse, the text format is **inconsistent across tools**: `go_to_definition` emits `file://<WORKSPACE>/main.go:5:6` (absolute `file://` URI), `search_in_files` emits `relpath:line: text` (relative path, line-only, no col), `get_symbol_overview` emits `Kind Name (line N)` (not in locus form at all), and `get_hover_info` emits a markdown blob (no locus). This heterogeneity is the core problem OUT-01/02/03/04 must solve.

The two viable rendering strategies are: **(A) parse-and-normalize CLI-side** — the CLI parses the daemon's text into `(path, line, col, payload)` tuples and re-renders terse; fragile because it must reverse-engineer N text formats. **(B) move structured rendering to the boundary** — change the daemon handlers (or add a structured-content channel) so the CLI receives machine-readable loci. Given the zero-proto invariant and that handlers already return `*mcpsdk.CallToolResult`, the recommended path is a **hybrid**: introduce a per-tool render policy keyed by tool name in `internal/cli`, with a small set of structured "locus-list" tools normalized by parsing the daemon's already-`file://...:L:C` lines (cheapest, since the daemon already emits the locus), and a passthrough class for non-locus tools (hover, repo map, blast-radius JSON envelope). The renderer is the single seam at `runVerb` → `renderResult`; that is the one function to replace.

For OUT-05, the v1.5 typed-error taxonomy (`internal/errors`, alias `serr`) survives over the wire as a **string** in two channels: transport/authz errors arrive as a Go `error` from `session.CallTool` whose `.Error()` is `"kind: message"` (`internal/errors/errors.go:51-56`), and tool-handler errors arrive as `res.IsError==true` with `TextContent` carrying the same `"kind: message"` string (`internal/kernel/symbols/tools.go:121-128, 359-360`). Today `cmd/helix/main.go:18-21` collapses **all** errors to `os.Exit(1)`. Phase 92 must parse the leading `kind:` token and map each `serr.Kind` to a stable stderr prefix and a per-kind non-zero exit code.

**Primary recommendation:** Replace `renderResult` with a tool-name-keyed terse renderer in `internal/cli`; classify tools into `locus-list` (normalize daemon `file://...:L:C` lines → `relpath:line:col<TAB>payload`, sort+dedup), `tree/outline` (passthrough, shape-only), and `opaque/json` (hover, blast-radius envelope; passthrough or `--json`). Add global persistent `--color` and `--abs` root flags (and a separate output `--json` — see the **`--json` collision** pitfall), add `color`/`abs` to the cligen `reservedRootFlags` denylist, and centralize exit-code mapping in `cmd/helix/main.go` keyed off the parsed `serr.Kind`. Re-target the existing `test/oracle/contract` goldens from MCP `TextContent` to CLI stdout.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Terse line formatting (`relpath:line:col<TAB>payload`) | CLI (`internal/cli`) | — | OUT-01: the render boundary is CLI-side, between `CallTool` result and stdout (`verb.go:215`). Daemon must stay format-stable for the retained internal MCP path. |
| LSP 0→1 coord conversion | Daemon (already done) | CLI (verify/parse) | `formatLocations` already `+1`s (`tools.go:189`); CLI parses already-1-based loci. Do NOT double-convert. |
| Workspace-relative path | CLI | Daemon (already relative for fileops) | CLI knows cwd/workspace; daemon emits mixed `file://` abs (symbols) and relpath (fileops). CLI must normalize both to relpath. |
| `--abs` absolute path | CLI | — | OUT-06: escape hatch lives in the renderer, reading the global flag. |
| Sort + dedup determinism | CLI | — | OUT-02/03: applied after parsing, before printing, so byte-identical across runs regardless of daemon ordering. |
| TTY/NO_COLOR/`--color` gating | CLI | — | OUT-04/06: stdout is the CLI's; `fatih/color` already in tree. |
| Typed-error → exit code | CLI (`main.go` + a mapper) | Daemon (emits `kind:`) | OUT-05: daemon emits `serr.Error()` string; CLI parses kind and sets exit code. |
| Global `--json`/`--color`/`--abs` flags | CLI root (`root.go`) | cligen denylist | OUT-06/07: persistent root flags every generated verb inherits; cligen must not shadow them. |
| Contract goldens | Test tier (`test/oracle/contract`) | — | TEST-02: re-target MCP `TextContent` goldens to CLI stdout. |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/spf13/cobra` | v1.10.2 | persistent root flags (`--json`/`--color`/`--abs`), verb subcommands | already the CLI framework `[VERIFIED: go.mod]` |
| `github.com/spf13/pflag` | v1.0.10 | flag types behind cobra | transitive of cobra `[VERIFIED: go.mod]` |
| `github.com/fatih/color` | v1.19.0 | color gating; **honors `NO_COLOR` automatically** | already used by `setup_output.go`/`status_output.go` `[VERIFIED: internal/cli/setup_output.go:7]` |
| `github.com/mattn/go-isatty` | v0.0.20 | TTY detection (`isatty.IsTerminal(fd)`) | transitive (via fatih/color); already resolvable `[VERIFIED: go.mod]` |
| `golang.org/x/term` | v0.41.0 | alt TTY detection (`term.IsTerminal(fd)`) | already in tree if a non-color-coupled check is wanted `[VERIFIED: go.mod]` |
| `github.com/modelcontextprotocol/go-sdk` | v1.5.0 | `*mcpsdk.CallToolResult`, `TextContent` | the result type the renderer consumes `[VERIFIED: go.mod]` |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| stdlib `path/filepath` | — | `filepath.Rel`, `ToSlash` (Windows relpath) | path normalization in the renderer |
| stdlib `sort` | — | deterministic ordering | OUT-02/03 sort by (path, line, col) |
| stdlib `strings` | — | parse `file://` prefix, `kind:` prefix, build `<TAB>` lines | the parse/normalize core |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Parse daemon text CLI-side (Strategy A) | Add a structured-content channel from handlers (Strategy B) | B is cleaner long-term but touches every handler and risks the zero-proto / format-stability invariants the retained MCP path depends on; A is contained to `internal/cli` and is the recommended scope for Phase 92. |
| `fatih/color` | hand-rolled ANSI + `mattn/go-isatty` | fatih/color already honors `NO_COLOR` and is already a dependency; do not hand-roll. |
| `mattn/go-isatty` | `golang.org/x/term` | both present; pick whichever the chosen color approach couples to. isatty is already pulled by fatih/color. |

**Installation:**
```bash
# No new packages required — every dependency is already in go.mod.
```

**Version verification:** All libraries above were confirmed present in `go.mod` by direct read `[VERIFIED: go.mod]`. No `go get` is needed for Phase 92.

## Package Legitimacy Audit

> Phase 92 installs **no new external packages**. All rendering uses the Go stdlib plus packages already vendored in `go.mod` (cobra, fatih/color, mattn/go-isatty, golang.org/x/term, modelcontextprotocol/go-sdk).

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|-------------|---------|-------------|
| (none — no new installs) | — | — | — | — | — | — |

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

## Architecture Patterns

### System Architecture Diagram

```
helix <verb> --flag=val
        │
        ▼
 cobra root + generated verb subcommand (root.go / verbs_gen.go)
        │  buildVerbArgs (pre-dial required-flag validation)  [verb.go:142]
        ▼
 forwarder.CallTool ── one-shot MCP tools/call over gRPC StreamMCP ──► WARM DAEMON
        │                                                                  │
        │                                                  handler formats result to TEXT
        │                                                  (formatLocations: file://abs:L:C — preview,
        │                                                   coords already +1 → 1-based)  [tools.go:182]
        │                                                                  │
        ◄──────────────── *mcpsdk.CallToolResult (TextContent | IsError) ──┘
        │
        ▼
 renderResult  ◄── THE SEAM (replace this)  [verb.go:215,247]
        │
        ├─ classify by tool name (render policy):
        │     ├─ locus-list  → parse "file://…:L:C — payload" lines
        │     │                  → strip file://, make relpath (or --abs),
        │     │                    sort+dedup by (path,line,col),
        │     │                    emit relpath:line:col<TAB>payload   [OUT-01/02/03/04]
        │     ├─ tree/outline → passthrough (shape only)              [OUT-03]
        │     └─ opaque/json  → passthrough text, or compact JSON under --json
        │
        ├─ color gate: NO_COLOR / --color=auto|always|never / isatty(stdout)  [OUT-04/06]
        │
        └─ on IsError OR transport error: parse leading "kind:" token,
             write "<KIND>: msg" to STDERR, return a kind-tagged error
                              │
                              ▼
              cmd/helix/main.go → exitCodeForKind(err) → os.Exit(N)  [OUT-05]
```

File-to-implementation mapping is in the Component Responsibilities note below, not the diagram.

### Recommended Project Structure
```
internal/cli/
├── verb.go            # runVerb + the renderResult SEAM (replace render here)
├── render.go          # NEW: terse renderer — parse, normalize, sort/dedup, color gate
├── render_policy.go   # NEW: tool-name → render class map (locus-list/tree/opaque)
├── render_test.go     # NEW: byte-equivalence, NO_COLOR, --color, --abs, sort/dedup
├── exitcode.go        # NEW: serr.Kind → exit code + stderr prefix (or fold into main)
├── root.go            # add persistent --color, --abs (and output --json); see pitfall
└── verbs_gen.go       # generated (unchanged content; cligen denylist gets color/abs)
cmd/helix/main.go      # exit-code branch keyed off the parsed kind
cmd/helix-cligen/render.go  # add "color","abs" to reservedRootFlags
test/oracle/contract/
├── golden_test.go     # re-target: capture CLI stdout instead of MCP TextContent
└── testdata/golden/   # regenerate per-verb stdout goldens (GOLDEN_UPDATE=1)
```

### Pattern 1: Single render seam, tool-name-keyed policy
**What:** Keep ONE render entrypoint (`renderResult`) and dispatch on `spec.toolName` to a render class. Do not scatter formatting across handlers.
**When to use:** All verb output.
**Example:**
```go
// Source: internal/cli/verb.go:247-258 (current seam to replace)
func renderResult(cmd *cobra.Command, res *mcpsdk.CallToolResult) {
	out := cmd.OutOrStdout()
	for _, c := range res.Content {
		if tc, ok := c.(*mcpsdk.TextContent); ok {
			fmt.Fprintln(out, tc.Text)   // ← today: verbatim daemon text
			continue
		}
		if b, err := json.Marshal(c); err == nil {
			fmt.Fprintln(out, string(b))
		}
	}
}
```

### Pattern 2: Coordinates are already 1-based at the daemon — parse, don't re-convert
**What:** The daemon converts LSP 0-based → 1-based before emitting text.
**Where:** `userPosToLSP` converts inbound 1-based args → 0-based for LSP (`internal/kernel/symbols/tools.go:145-157`); `formatLocations` converts outbound LSP 0-based → 1-based for display (`tools.go:189`: `loc.Range.Start.Line + 1`, `tools.go:190`: `...Start.Character + 1`).
**Implication:** The terse renderer parses already-1-based loci. Re-converting would double-shift. The conversion is a daemon responsibility that already exists; the renderer's job is path normalization + layout, not coord math. (NOTE: CONTEXT/ROADMAP prose phrases this as "convert LSP 0-based at the render boundary" — that requirement is **already satisfied** by the daemon's existing `+1` at `tools.go:189-190`; the CLI renderer PARSES the already-1-based locus and does NOT re-convert. Re-converting CLI-side would double-shift.)

### Pattern 3: Path normalization — handle BOTH daemon formats
**What:** Symbols tools emit absolute `file://<root>/path` (`formatLocations`); fileops tools already emit `relpath` via `filepath.Rel` (`internal/kernel/fileops/search.go:67`, `find.go:50`).
**Recommendation:** The renderer strips a leading `file://` and, given a known workspace root, computes `filepath.Rel(root, abs)`. Use `filepath.ToSlash` so Windows emits `/`-separated relpaths (golden stability). `--abs` reverses this (emit the absolute path). The workspace root CLI-side can be derived from `os.Getwd()` or an explicit flag (activate uses `--workspace` defaulting to `os.Getwd()` — `internal/cli/activate.go:28-36`); the planner must choose and lock the root-resolution rule (see Open Questions).

### Anti-Patterns to Avoid
- **Re-converting coordinates in the renderer:** they are already 1-based at the daemon (`tools.go:189-190`). Double-shift produces off-by-one goldens.
- **Reusing the existing root `--json` (logging) flag for output JSON:** `root.go:113` already defines `--json` as a *log-format* flag passed to `newLogger` (`root.go:211, 225`). OUT-06's `--json` means *output* JSON. They collide. See Pitfall 1.
- **Formatting in N handlers:** keep the terse policy in one CLI seam; do not edit each kernel handler (touches the retained MCP path and risks the format-stability invariant).
- **Stripping ANSI after the fact:** gate color generation up front (color disabled → no codes emitted) rather than emitting then stripping.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| NO_COLOR / color enable | custom env+ANSI logic | `github.com/fatih/color` (already used in `setup_output.go`) | already honors `NO_COLOR`; consistent with the rest of the CLI |
| TTY detection | `os.Stat` mode hacks | `mattn/go-isatty` or `golang.org/x/term.IsTerminal` | both in tree; battle-tested across platforms |
| Workspace-relative path | string-prefix trimming | `filepath.Rel` + `filepath.ToSlash` | handles `..`, separators, Windows |
| Golden compare/update | bespoke diff | `test/harness` `AssertGolden` + `GOLDEN_UPDATE=1` | the contract oracle already uses it (`golden_test.go:7-9`) |
| Typed error matching | substring scan everywhere | `serr` `errors.Is` / parse the `kind:` prefix once | the taxonomy already exists (`internal/errors/kinds.go`) |

**Key insight:** Everything Phase 92 needs (color, isatty, golden harness, typed errors, coord conversion) already exists in the tree. The phase is *integration and normalization*, not new infrastructure.

## Runtime State Inventory

> Not a rename/refactor/migration phase — this is additive product work (a renderer + flags + re-targeted goldens). No stored data, live service config, OS-registered state, secrets, or build artifacts carry a string that changes. The one persisted artifact that changes is the **committed contract golden files** under `test/oracle/contract/testdata/golden/` — these are test fixtures (regenerated via `GOLDEN_UPDATE=1`), not runtime state. **None found in any runtime-state category — verified by reading the phase scope and the contract oracle (`test/oracle/contract/golden_test.go`).**

## Common Pitfalls

### Pitfall 1: `--json` flag collision (logging vs output)
**What goes wrong:** OUT-06 wants a global `--json` for *output* JSON; `root.go:113` already defines `--json` as a *log-format* flag (`Bool("json", false, "Use JSON log format ...")`) consumed by `runForwarder`/`runDaemon` (`root.go:211, 225`).
**Why it happens:** the existing flag predates the output contract; the cligen denylist already reserves `json` because of it (`cmd/helix-cligen/render.go:17`).
**How to avoid:** Decide and LOCK one of: (a) repurpose the existing `--json` to mean output-JSON for verbs while leaving daemon/forwarder paths reading it as log-format (they are mutually exclusive code paths — verbs never run the daemon), or (b) rename the logging flag (e.g. `--log-json`) and free `--json` for output. Option (a) is lower-blast-radius since `runVerb` and `runDaemon` are disjoint dispatch paths. The planner must pick explicitly; this is the single highest-risk ambiguity in the phase.
**Warning signs:** a verb's `--json` produces no JSON, or `helix --serve --json` changes verb behavior.

### Pitfall 2: Inconsistent daemon text formats across tools
**What goes wrong:** A one-size parser breaks because `go_to_definition` emits `file://<abs>:L:C`, `search_in_files` emits `relpath:L: text` (no col), `get_symbol_overview` emits `Kind Name (line N)`, `get_hover_info` emits markdown.
**Why it happens:** each handler hand-formats (`formatLocations`, `formatOutline`, `formatHierarchy`, `search.go`'s own formatter, hover passthrough).
**How to avoid:** a render-class map (locus-list / tree / opaque). Only the locus-list class gets the `relpath:line:col<TAB>payload` treatment; tree/opaque pass through (OUT-03 explicitly says outline verbs print shape only). Enumerate every one of the 50 verbs into a class during planning (the cligen catalog is the authoritative list — `internal/cli/verbs_gen.go`).
**Warning signs:** garbled output for hover/overview; a parser that drops `search_in_files` matches because they lack a column.

### Pitfall 3: Non-locus payloads (hover, repo map, blast-radius envelope)
**What goes wrong:** Forcing `relpath:line:col<TAB>` onto hover (markdown) or the blast-radius JSON envelope (`formatBlastRadiusEnvelope`, `tools.go:681`) produces nonsense.
**How to avoid:** `get_hover_info`, `get_repo_map`, `get_context`, and `analyze_blast_radius` are **opaque/passthrough**. blast-radius already emits a JSON envelope string — treat as `--json`-shaped passthrough. Document this carve-out in the render policy.

### Pitfall 4: Windows path separators in relpath
**What goes wrong:** `filepath.Rel` returns `pkg\greeter.go` on Windows → goldens diverge from the `pkg/greeter.go` form.
**How to avoid:** always `filepath.ToSlash` the relpath before emitting. The fileops tools already emit forward-slash relpaths; match that.
**Warning signs:** Windows CI golden diffs that differ only in `\` vs `/`.

### Pitfall 5: Snippet-line extraction for nav verbs (SC#2 / OUT-03)
**What goes wrong:** OUT-03 wants nav verbs to print "locus + enclosing symbol + one snippet line" so no follow-up `Read` is forced — but `formatLocations` only fills `Preview`/`Name` *when present* (`tools.go:191-197`), and `goto_definition`/`find_references` populate neither (`locationsToSymbolLocations` at `retrieval.go:99-108` leaves `Preview`/`Name` empty). The current `go_to_definition` golden is bare `file://<WORKSPACE>/main.go:5:6` with no snippet.
**Why it happens:** the daemon does not read the source line for definition/reference results.
**How to avoid:** the snippet line must be sourced somewhere. Either (a) the daemon enriches `SymbolLocation.Preview` (touches kernel — wider scope), or (b) the CLI reads the one line from the file at `relpath:line` after parsing the locus (CLI-side, contained, but adds a file read per locus). The planner must choose; (b) keeps Phase 92 in `internal/cli` but the workspace-root resolution (Open Q1) becomes load-bearing for reading the file. This is a genuine scope fork — flag it for a planning decision.
**Warning signs:** SC#2 behavioral-oracle fails because the agent still issues a `Read` after `find-symbol`.

### Pitfall 6: Blanket `os.Exit(1)` erases the typed taxonomy
**What goes wrong:** `cmd/helix/main.go:18-21` exits 1 for every error; OUT-05 needs per-kind codes.
**How to avoid:** parse the leading `kind:` token from the error string (transport channel) or from the `IsError` `TextContent` (handler channel) — `serr.Error.Error()` is `"<kind>: <message>"` (`internal/errors/errors.go:51-56`), and `Kind` values are the stable strings in `kinds.go:13-33`. Map each kind to a fixed exit code. Note `runVerb` currently wraps the handler-error into a generic `fmt.Errorf("tool %s reported an error")` (`verb.go:216-218`) which DROPS the kind — that wrapping must change to carry the kind through to `main`.
**Warning signs:** `helix go-to-definition` on a missing symbol exits 1 same as a permission denial; the agent cannot branch.

### Pitfall 7: Determinism — daemon ordering is not guaranteed stable
**What goes wrong:** LSP `textDocument/references` can return results in server-dependent order; repeated runs differ → goldens flake.
**How to avoid:** sort by `(relpath, line, col)` and dedup AFTER parsing, CLI-side (OUT-02/03). The contract oracle already sorts workspace lines for `search_symbols` (`golden_test.go:48-60, filterWorkspaceLines`) — adopt the same discipline in the renderer so it holds for live runs, not just goldens.

## Code Examples

### Typed error → "kind: message" string (the wire form the CLI parses)
```go
// Source: internal/errors/errors.go:51-56
func (e *Error) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("%s: %s (%s)", e.Kind, e.Message, e.Detail)
	}
	return fmt.Sprintf("%s: %s", e.Kind, e.Message)
}
// Kind values (stable strings): not_found, invalid_args, no_workspace,
// unsupported, internal, circuit_open, timeout, permission_denied,
// guardrail_violation   [internal/errors/kinds.go:13-33]
```

### Daemon-side locus formatting (already 1-based; the CLI parses this)
```go
// Source: internal/kernel/symbols/tools.go:182-200
func formatLocations(locs []SymbolLocation) string {
	if len(locs) == 0 {
		return "(no results)"
	}
	var sb strings.Builder
	for _, loc := range locs {
		uri := loc.URI                          // "file://<abs>/path"
		line := loc.Range.Start.Line + 1        // ← already 1-based
		col := loc.Range.Start.Character + 1    // ← already 1-based
		if loc.Name != "" {
			sb.WriteString(fmt.Sprintf("%s:%d:%d — %s [%s]\n", uri, line, col, loc.Name, loc.Kind))
		} else if loc.Preview != "" {
			sb.WriteString(fmt.Sprintf("%s:%d:%d — %s\n", uri, line, col, loc.Preview))
		} else {
			sb.WriteString(fmt.Sprintf("%s:%d:%d\n", uri, line, col)) // ← goto_def / find_refs hit this
		}
	}
	return sb.String()
}
```

### Existing color helper (NO_COLOR already honored — reuse the pattern)
```go
// Source: internal/cli/setup_output.go:7,18-22
import "github.com/fatih/color"
// fatih/color honors NO_COLOR automatically; color.NoColor can be forced.
green := color.New(color.FgGreen)
fmt.Fprintf(os.Stderr, "%s %s\n", green.Sprint("✓"), msg)
// For --color=never: set color.NoColor = true. For --color=always: color.NoColor = false.
// For --color=auto: leave default (off when stdout is not a TTY / NO_COLOR set).
```

### cligen reserved-flag denylist (must add color, abs)
```go
// Source: cmd/helix-cligen/render.go:17-31  (json already reserved due to the logging flag)
var reservedRootFlags = map[string]bool{
	"socket": true, "json": true, "profile": true, "config": true,
	"mode": true, "serve": true, "http-addr": true, "admin-addr": true,
	"version": true, "disable-lsp-subsystem": true,
	"disable-structured-edit-subsystem": true, "disable-semantic-subsystem": true,
	// Phase 92 MUST add: "color": true, "abs": true
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Agent reads MCP `TextContent` directly | CLI renders terse `relpath:line:col<TAB>payload` to stdout | Phase 92 (this) | output shape FREEZES here; SKILL.md (Phase 93) cites it |
| `renderResult` dumps verbatim text / compact JSON | tool-name-keyed terse renderer | Phase 92 | the one seam to replace (`verb.go:247`) |
| Blanket `os.Exit(1)` | per-kind exit codes from `serr.Kind` | Phase 92 | agents can branch on error kind (OUT-05) |
| MCP `TextContent` goldens (`test/oracle/contract`) | CLI stdout goldens | Phase 92 (TEST-02) | golden capture switches from `harness.CallTool`→`TextContent` to subprocess stdout |

**Deprecated/outdated:**
- Root `--json` as a *logging* flag is in tension with OUT-06's output `--json`; resolve in Phase 92 (Pitfall 1).
- The `call` parent verb was removed in 91-01; verbs flatten onto root (`91-01-SUMMARY.md`). Any renderer wiring attaches at the flat verb level.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Terse rendering can stay CLI-side (Strategy A/hybrid) without editing kernel handlers | Summary / Pattern 1 | If the snippet-line requirement (OUT-03/SC#2) forces daemon enrichment, scope expands into the kernel — see Pitfall 5. |
| A2 | Workspace root for relpath is derivable CLI-side from `os.Getwd()` (mirroring activate's default) | Pattern 3 / Open Q1 | Wrong root → wrong relpaths; goldens and copy-paste chaining (OUT-04) break. |
| A3 | Repurposing the existing `--json` for output (Pitfall 1 option a) is safe because verb and daemon dispatch paths are disjoint | Pitfall 1 | If any verb path also reads the log-format `--json`, behavior changes unexpectedly. |
| A4 | Per-kind exit-code numbering is the planner's to choose (no pre-existing numbering convention found in tree) | OUT-05 | A future external contract could expect specific numbers; none found today. |

## Open Questions (RESOLVED)

> All four forks were resolved during planning and are now LOCKED. The decisions live in the plans (92-01/92-02/92-03); they are recorded here so the research → plan artifact chain is internally consistent. None remain open.

1. **Workspace-root resolution CLI-side for relpath + snippet reads.**
   - What we know: `activate` uses `--workspace` defaulting to `os.Getwd()` (`activate.go:28-36`); the daemon embeds the absolute `RepoRoot` in `file://` URIs (`formatLocations`); fileops already emit relpath.
   - What's unclear: whether the verb should infer root from `os.Getwd()`, a new `--workspace` flag, or by stripping the daemon's `file://<root>` prefix using the longest-common-prefix of returned loci.
   - Recommendation: derive from `os.Getwd()` by default with a `--workspace`/`--abs` override; LOCK this in planning because Pitfall 5 (snippet reads) depends on it.
   - **RESOLVED:** `os.Getwd()` is the default workspace root CLI-side, with `--abs` as the absolute-path override (no separate `--workspace` flag for the renderer). Locked in 92-02 Task 2 (runVerb resolves `workspaceRoot` from `os.Getwd()`); 92-03 sets the subprocess CWD to the workspace root so `os.Getwd()`-derived relpaths resolve.

2. **`--json` collision resolution (repurpose vs rename).**
   - What we know: `--json` is currently a logging flag (`root.go:113`); cligen reserves it.
   - What's unclear: repurpose vs rename the logging flag.
   - Recommendation: repurpose `--json` for verb output (option a, lowest blast radius); rename logging to `--log-json` only if a verb path genuinely reads the log flag.
   - **RESOLVED:** Option (a) — REPURPOSE the existing `--json` for verb output JSON; the daemon/forwarder dispatch paths (disjoint from verbs) keep reading it as log-format unchanged. No rename. Locked in 92-02 Task 2 (root.go change + dual-read comment).

3. **Snippet line: daemon enrichment vs CLI file read (OUT-03 / SC#2).**
   - What we know: nav loci carry no `Preview` today (Pitfall 5).
   - Recommendation: prefer CLI-side single-line read (keeps phase in `internal/cli`) unless the planner accepts kernel scope.
   - **RESOLVED:** Option (b) — CLI-side single-line read (`readSnippetLine`), zero kernel/proto change. The read is clamped to the workspace root (path-traversal guard, T-92-04). Locked in 92-02 Task 1 (`readSnippetLine` helper).

4. **Exit-code numbering scheme.**
   - What we know: 9 `serr.Kind` values; `main.go` only emits 0/1 today.
   - Recommendation: assign a stable distinct non-zero code per kind (e.g. invalid_args=2, no_workspace=3, not_found=4, permission_denied=5, unsupported=6, timeout=7, circuit_open=8, guardrail_violation=9, internal=70); document in SKILL.md (Phase 93 cites it).
   - **RESOLVED:** The per-kind scheme is adopted verbatim: invalid_args=2, no_workspace=3, not_found=4, permission_denied=5, unsupported=6, timeout=7, circuit_open=8, guardrail_violation=9, internal=70; unrecognized/no-kind → 1 (generic fallback). Frozen in 92-01 Task 3 (`exitCodeForKind`) and cited by Phase 93 SKILL.md.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | build/test | ✓ | (repo CGO=1 single-mode) | — |
| `gopls` | LS-backed contract goldens (`with_ls` group) | ✓ if installed; tests `RequireGopls` skip cleanly | — | golden tests skip when gopls absent (`golden_test.go:185`) |
| `HELIX_BIN` | CLI subprocess E2E (`cli_e2e_test.go`, `cli_sec_e2e_test.go`) | gated; must be set to RUN not skip | — | tests skip when unset (intentional; verify step sets it) |
| New external packages | — | n/a | — | none needed |

**Missing dependencies with no fallback:** none.
**Missing dependencies with fallback:** `gopls`-dependent goldens skip when gopls is unavailable; the CLI-stdout golden re-target inherits this skip discipline.

## Validation Architecture

> `workflow.nyquist_validation: true` and `workflow.tdd_mode: true` `[VERIFIED: .planning/config.json]` — full validation section included; plans should be TDD (RED→GREEN).

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` + `test/harness` golden store |
| Config file | none (Go convention); build tags `integration`/`llm` for the contract oracle |
| Quick run command | `go test ./internal/cli/ -run Render -count=1` |
| Full suite command | `go vet ./... && go test ./...` |
| Golden update | `GOLDEN_UPDATE=1 go test -tags integration -run TestGolden -count=1 -timeout 5m ./test/oracle/contract/...` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| OUT-01 | terse `relpath:line:col<TAB>payload`; zero ANSI off-TTY; 1-based | unit | `go test ./internal/cli/ -run Render_Terse -x` | ❌ Wave 0 (`internal/cli/render_test.go`) |
| OUT-02 | byte-identical sorted+deduped across N runs | unit | `go test ./internal/cli/ -run Render_Determinism -count=5` | ❌ Wave 0 |
| OUT-03 | nav prints locus+enclosing+1 snippet; outline shape-only | unit + behavioral | `go test ./internal/cli/ -run Render_NavSelfContained` | ❌ Wave 0 |
| OUT-04 | find-symbol output feeds replace-symbol-body / get-callers verbatim | E2E (behavioral oracle) | `HELIX_BIN=... go test ./internal/cli/ -run E2E_Chain` | ❌ Wave 0 (extend `cli_e2e_test.go`) |
| OUT-05 | per-kind stderr prefix + non-zero exit code | unit + E2E | `go test ./internal/cli/ -run ExitCode` | ❌ Wave 0 (`internal/cli/exitcode_test.go`); SEC e2e (`cli_sec_e2e_test.go`) already proves permission_denied exit |
| OUT-06 | `--json` compact; `--color=never` ≡ piped; default terse | unit | `go test ./internal/cli/ -run Render_Flags` | ❌ Wave 0 |
| OUT-07 | `--abs` absolute paths; relpath default; behavioral validation | unit + behavioral | `go test ./internal/cli/ -run Render_Abs` | ❌ Wave 0 |
| TEST-02 | CLI stdout goldens (ordering/file:line/error-kind); typed-args→cobra-flags parity | golden + parity | `go test -tags integration -run TestGolden ./test/oracle/contract/...`; `make verify-cligen` | ⚠️ EXISTS but targets MCP TextContent — must re-target (`test/oracle/contract/golden_test.go`) |

### Sampling Rate
- **Per task commit:** `go test ./internal/cli/ -run Render -count=1` (+ the touched render/exitcode test)
- **Per wave merge:** `go vet ./... && go test ./...`
- **Phase gate:** full suite green + re-targeted contract goldens green (`-tags integration`) + `make verify-cligen` green before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `internal/cli/render.go` + `internal/cli/render_test.go` — OUT-01/02/03/06/07 unit coverage (terse, sort/dedup, NO_COLOR/--color byte-equivalence, --abs)
- [ ] `internal/cli/render_policy.go` — per-tool render class map covering all 50 verbs (authoritative list = `verbs_gen.go`)
- [ ] `internal/cli/exitcode.go` + `internal/cli/exitcode_test.go` — `serr.Kind` → exit code + stderr prefix (OUT-05)
- [ ] Re-target `test/oracle/contract/golden_test.go` to capture CLI subprocess stdout (TEST-02); regenerate `testdata/golden/*` with `GOLDEN_UPDATE=1`
- [ ] Add `color`,`abs` to `cmd/helix-cligen/render.go reservedRootFlags`; regenerate `verbs_gen.go`; `make verify-cligen` green
- [ ] Behavioral oracle: extend `internal/cli/cli_e2e_test.go` for the copy-paste chain (OUT-04) and no-follow-up-Read (OUT-03/SC#2)

## Security Domain

> No `security_enforcement` key in `.planning/config.json`; Helix is Go-native with no Java `java-security` SMTC capability (per CLAUDE.md, security rows N/A for this repo). Phase 92 adds no network surface, auth path, or trust boundary — it renders existing tool output and maps error kinds to exit codes. The pre-existing SEC-01/02 enforcement (Phase 91) is unchanged.

### Applicable ASVS Categories
| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V5 Input Validation | minor | output-only phase; the only untrusted input is the daemon's own text (parsed for loci/kind) — parse defensively (no panic on malformed lines; fall back to passthrough) |
| V6 Cryptography | no | — |
| V2/V3/V4 (auth/session/access) | no | enforcement already lives at `tools/call` (Phase 91); unchanged here |

### Known Threat Patterns for this phase
| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Malformed daemon text crashes the renderer | Denial of Service | defensive parse: unrecognized line → opaque passthrough, never panic |
| Error-kind spoofing via message text containing `kind:` | Tampering (low) | parse kind from the structured channel where possible; for the text channel, match only the documented `serr.Kind` enum set, not arbitrary `word:` prefixes |
| Path traversal via `--abs`/snippet read outside workspace | Info disclosure (low) | clamp snippet reads to the resolved workspace root (Open Q1/Q3) |

## Sources

### Primary (HIGH confidence)
- `internal/cli/verb.go:201-258` — `runVerb` + `renderResult` (the seam) — read in full
- `internal/cli/root.go:79-160, 207-280` — root flags (`--json` logging), dispatch — read in full
- `internal/kernel/symbols/tools.go:113-200, 353-426, 628-806` — `textResult`/`errorResult`/`formatLocations`/`userPosToLSP`, handler error patterns, blast-radius envelope
- `internal/kernel/symbols/retrieval.go:13-108` — `SymbolLocation`, empty Preview/Name for nav results
- `internal/errors/errors.go:8-78` + `internal/errors/kinds.go:13-48` — typed taxonomy, `Error()` string form, 9 Kind values
- `cmd/helix/main.go:14-22` — blanket `os.Exit(1)`
- `cmd/helix-cligen/render.go:17-31` — `reservedRootFlags` (json reserved; color/abs missing)
- `test/oracle/contract/golden_test.go` (full) — contract oracle to re-target; sort/normalize discipline
- `test/oracle/contract/testdata/golden/{go_to_definition,find_references,search_in_files,get_symbol_overview,get_hover_info}/success.golden` — actual heterogeneous output formats
- `internal/cli/setup_output.go`, `internal/cli/status_output.go` — existing `fatih/color` usage
- `internal/kernel/fileops/{search.go:67,find.go:50}` — fileops already emit `filepath.Rel` relpaths
- `internal/cli/cli_sec_e2e_test.go:1-75` — SEC-01 typed-exit round-trip proof (exit-code precedent)
- `.planning/REQUIREMENTS.md` (OUT-*/TEST-02), `.planning/STATE.md` (carried constraints), `92-CONTEXT.md`, `90-03`/`91-01` SUMMARYs
- `go.mod` — all dependency versions

### Secondary (MEDIUM confidence)
- none — all findings are direct codebase reads.

### Tertiary (LOW confidence)
- none.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — every package verified present in `go.mod`; no new installs.
- Architecture (the render seam, coord/path facts, error channels): HIGH — verified by direct read of the exact lines.
- Pitfalls: HIGH — each is grounded in a specific file:line (json collision, format heterogeneity, blanket exit, Windows separators, empty Preview).
- Open Questions: RESOLVED — the four planning forks (root resolution, snippet sourcing, json repurpose, exit numbering) are all locked in the plans; see Open Questions (RESOLVED) above.

**Research date:** 2026-06-21
**Valid until:** 2026-07-21 (stable; internal codebase, no fast-moving external deps)

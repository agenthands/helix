# Phase 92: Terse Output Renderer + Re-Targeted Contract Oracle - Pattern Map

**Mapped:** 2026-06-21
**Files analyzed:** 8 (5 new, 3 modified) + golden fixtures
**Analogs found:** 8 / 8 (all in-tree; no external infra)

> RESEARCH.md already carries the authoritative file:line map and render-class
> taxonomy. This document distills it into per-file pattern assignments the
> planner can paste into plan actions. Every analog below was re-read and
> verified against the live tree.

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/cli/render.go` (NEW) | utility (renderer) | transform | `internal/kernel/symbols/tools.go` `formatLocations` (locus shape) + `internal/cli/verb.go` `renderResult` (seam) | role+flow match |
| `internal/cli/render_policy.go` (NEW) | config (tool→class map) | transform | `internal/cli/verbs_gen.go` `verbSpecs` catalog (authoritative tool list) | role-match |
| `internal/cli/render_test.go` (NEW) | test | transform | `test/oracle/contract/golden_test.go` (normalize/sort discipline) | flow-match |
| `internal/cli/exitcode.go` (NEW) | utility (error→code mapper) | transform | `internal/errors/kinds.go` + `errors.go` `Error()` | role-match |
| `internal/cli/exitcode_test.go` (NEW) | test | transform | `internal/cli/cli_sec_e2e_test.go` (exit-code round-trip) | exact |
| `internal/cli/verb.go` (MOD) | controller (verb runner) | request-response | self — `renderResult`/`runVerb` replaced in place | self |
| `internal/cli/root.go` (MOD) | config (root flags) | request-response | self — existing `rootCmd.Flags()` block (line 109-137) | self |
| `cmd/helix/main.go` (MOD) | controller (entrypoint) | request-response | self — `os.Exit(1)` branch (line 18-21) | self |
| `cmd/helix-cligen/render.go` (MOD) | config (codegen denylist) | transform | self — `reservedRootFlags` map (line 17-30) | self |
| `test/oracle/contract/golden_test.go` (MOD) + `testdata/golden/*` | test | transform | self — re-targeted MCP→CLI stdout | self |

## Pattern Assignments

### `internal/cli/render.go` (NEW — terse renderer)

**Seam to replace** — `internal/cli/verb.go:247-258` (current `renderResult`):
```go
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
Keep this ONE entrypoint signature; dispatch internally on `spec.toolName` →
render class. Do not scatter formatting across handlers.

**Locus shape the renderer parses** — `internal/kernel/symbols/tools.go:182-200`
(`formatLocations`, coords ALREADY 1-based — parse, do not re-convert):
```go
uri := loc.URI                       // "file://<abs>/path"
line := loc.Range.Start.Line + 1     // ← already 1-based
col := loc.Range.Start.Character + 1 // ← already 1-based
// emits:  file://<abs>:L:C — Name [Kind]   (Name present)
//         file://<abs>:L:C — preview        (Preview present)
//         file://<abs>:L:C                  (goto_def / find_refs: bare)
```
Renderer job: strip `file://`, `filepath.Rel(root, abs)` + `filepath.ToSlash`
(or `--abs` → keep absolute), sort+dedup by `(relpath,line,col)`, emit
`relpath:line:col<TAB>payload`.

**Path-normalization precedent (fileops already emits relpath)** —
`internal/kernel/fileops/search.go:67`, `internal/kernel/fileops/find.go:50`
use `filepath.Rel`; the renderer must normalize BOTH the symbols `file://` abs
form and this relpath form to the same workspace-relative slash output.

**Color gate** — copy `internal/cli/setup_output.go:7,18-22`:
```go
import "github.com/fatih/color"   // honors NO_COLOR automatically
// --color=never → color.NoColor = true ; --color=always → false ; auto → default
```

**Defensive parse (Security V5):** any line not matching the locus grammar →
opaque passthrough, never panic. Match `kind:` only against the 9-value
`serr.Kind` enum, not arbitrary `word:` prefixes.

---

### `internal/cli/render_policy.go` (NEW — tool→render-class map)

**Analog (authoritative tool list):** `internal/cli/verbs_gen.go` `verbSpecs`
catalog — enumerate every one of the ~50 verbs into a class. Classes (from
RESEARCH Pitfall 2/3):
- **locus-list** → `relpath:line:col<TAB>payload` (goto_definition,
  find_references, find/search-symbol, get_callers, etc.)
- **tree/outline** → passthrough, shape-only (OUT-03: outline verbs print shape)
- **opaque/json** → passthrough or `--json` (get_hover_info markdown,
  get_repo_map, get_context, analyze_blast_radius JSON envelope at
  `internal/kernel/symbols/tools.go:681`)

---

### `internal/cli/exitcode.go` (NEW — serr.Kind → exit code + stderr prefix)

**Analog (the enum + wire form):** `internal/errors/kinds.go:13-33` (9 stable
Kind strings) and `internal/errors/errors.go:51-56`:
```go
func (e *Error) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("%s: %s (%s)", e.Kind, e.Message, e.Detail)
	}
	return fmt.Sprintf("%s: %s", e.Kind, e.Message)   // "kind: message"
}
```
Kinds: `not_found, invalid_args, no_workspace, unsupported, internal,
circuit_open, timeout, permission_denied, guardrail_violation`.

**Two wire channels to parse** (RESEARCH Pitfall 6):
- transport error: Go `error` from `callToolFn` whose `.Error()` is `"kind: message"`
- handler error: `res.IsError==true` with `TextContent` carrying `"kind: message"`

**Current kind-dropping bug to fix** — `internal/cli/verb.go:216-218`:
```go
if res.IsError {
	return fmt.Errorf("tool %s reported an error", spec.toolName)  // ← DROPS kind
}
```
Must carry the parsed kind through to `main`. Recommended exit numbering
(RESEARCH Open Q4): invalid_args=2, no_workspace=3, not_found=4,
permission_denied=5, unsupported=6, timeout=7, circuit_open=8,
guardrail_violation=9, internal=70.

---

### `cmd/helix/main.go` (MOD — per-kind exit branch)

**Replace** `cmd/helix/main.go:18-21`:
```go
if err := cmd.Execute(); err != nil {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)            // ← collapses all errors
}
```
with `os.Exit(exitCodeForKind(err))` keyed off the parsed `serr.Kind`, writing
the stable `<KIND>: msg` stderr prefix.

---

### `internal/cli/root.go` (MOD — global persistent flags)

**Analog (existing flag block):** `internal/cli/root.go:109-137`. Add persistent
`--color` and `--abs`. **`--json` COLLISION** (RESEARCH Pitfall 1 — highest-risk
ambiguity): `root.go:113` defines `--json` as a LOG-format flag consumed at
`root.go:211` and `root.go:225` by daemon/forwarder paths only:
```go
rootCmd.Flags().Bool("json", false, "Use JSON log format (default: text)")  // :113
jsonLog, _ := cmd.Flags().GetBool("json")  // :211 runForwarder, :225 runDaemon
```
Planner MUST lock: repurpose `--json` for verb output (option a — verb and
daemon dispatch paths are disjoint, lowest blast radius) OR rename logging flag
to `--log-json`. Recommended: option (a).

---

### `cmd/helix-cligen/render.go` (MOD — codegen denylist)

**Add to** `reservedRootFlags` at `cmd/helix-cligen/render.go:17-30` (currently
reserves `json` due to the logging flag):
```go
"color": true,
"abs":   true,
```
Then regenerate `verbs_gen.go`; `make verify-cligen` green.

---

### `test/oracle/contract/golden_test.go` (MOD — re-target MCP→CLI stdout)

**Analog (self — sort/normalize discipline to keep):**
`test/oracle/contract/golden_test.go:24-60`:
- `normalizeResponse` (line 24) replaces workspace dir with `<WORKSPACE>` and
  abs paths — keep, but the re-targeted form normalizes CLI relpath output.
- `filterWorkspaceLines` (line 48-60) already `sort.Strings` keeps workspace
  lines — the renderer must adopt the same `(relpath,line,col)` sort so live
  output is deterministic, not just goldens.

Change golden capture from `harness.CallTool` → `TextContent` to **CLI
subprocess stdout** (gated on `HELIX_BIN`); regenerate `testdata/golden/*` with
`GOLDEN_UPDATE=1`. Heterogeneous source goldens to regenerate live under
`test/oracle/contract/testdata/golden/{go_to_definition,find_references,search_in_files,get_symbol_overview,get_hover_info}/success.golden`.

---

### `internal/cli/render_test.go` / `exitcode_test.go` (NEW — TDD Wave 0)

**Exit-code E2E analog (exact):** `internal/cli/cli_sec_e2e_test.go:1-75` already
proves a `permission_denied` typed-exit round-trip via subprocess — extend the
same pattern for the other kinds. Behavioral chain (OUT-04) extends
`internal/cli/cli_e2e_test.go`.

## Shared Patterns

### Coordinate convention (apply to render.go + all locus-list tools)
**Source:** `internal/kernel/symbols/tools.go:189-190` (`+1` outbound),
`tools.go:145-157` (`userPosToLSP` inbound). Daemon emits 1-based; renderer
parses 1-based. NEVER re-convert (double-shift = off-by-one goldens).

### Typed-error wire form (apply to exitcode.go + verb.go + main.go)
**Source:** `internal/errors/errors.go:51-56`, `internal/errors/kinds.go:13-33`.
Parse leading `kind:` token from BOTH the transport error string and the
`IsError` `TextContent`; match only the documented 9-Kind enum.

### Color/TTY gating (apply to render.go)
**Source:** `internal/cli/setup_output.go:7` (`github.com/fatih/color`, honors
`NO_COLOR`). Gate generation up front — never emit then strip ANSI.

### Path normalization (apply to render.go)
**Source:** `internal/kernel/fileops/search.go:67`, `find.go:50` (`filepath.Rel`).
Always `filepath.ToSlash` for Windows golden stability.

### Golden harness (apply to golden_test.go + render_test.go)
**Source:** `test/oracle/contract/golden_test.go:24-60` — `normalizeResponse`,
`filterWorkspaceLines` (sort), `harness.AssertGolden` + `GOLDEN_UPDATE=1`.

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| (none) | — | — | All Phase 92 work has an in-tree analog; this is integration/normalization, not new infrastructure. |

## Metadata

**Analog search scope:** `internal/cli/`, `internal/errors/`,
`internal/kernel/symbols/`, `internal/kernel/fileops/`, `cmd/helix/`,
`cmd/helix-cligen/`, `test/oracle/contract/`
**Files scanned:** 12 (RESEARCH.md provided the exact file:line map; verified by direct read)
**Pattern extraction date:** 2026-06-21

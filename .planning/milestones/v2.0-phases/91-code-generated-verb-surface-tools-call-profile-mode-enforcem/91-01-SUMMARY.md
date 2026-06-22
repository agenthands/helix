---
phase: 91-code-generated-verb-surface-tools-call-profile-mode-enforcem
plan: 01
subsystem: cli
tags: [codegen, go-packages, go-ast, cobra, verb-surface, drift-gate, mcp]

# Dependency graph
requires:
  - phase: 90-cli-spine
    provides: "verbSpec/verbFlag/buildVerbArgs spine, callToolFn seam, cobra AddGroup scaffold, forwarder.CallTool one-shot dial"
  - phase: docgen
    provides: "blank-import discipline + --check regenerate-and-diff gate pattern (cmd/docgen)"
provides:
  - "cmd/helix-cligen: build-time generator (scan.go AST recovery, render.go gofmt-stable emit, main.go --check gate)"
  - "internal/cli/verbs_gen.go: committed generated verb catalog — 50 verbs, one per unique live-registry tool"
  - "internal/cli.VerbToolNames(): exported read-only accessor over the verb catalog (the seam 91-03 integration tests consume)"
  - "flagStringSlice + flagJSON flag kinds in the verb spine (non-scalar arg policy)"
  - "make verify-cligen target + CI drift-gate step"
affects: [91-02, 91-03, 91-04]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "go/packages + go/ast AST scan to recover the tool-name -> *Args binding the runtime drops (VERB-04)"
    - "Per-function-body tool-var scoping so AddTool calls passing the tool by variable resolve their Name"
    - "Generated-file drift gate: format.Source-stable emit + --check compare (mirror docgen)"
    - "Catalog = scan-superset INTERSECT live registry (no special-cases); generator owns verbSpecs"

key-files:
  created:
    - cmd/helix-cligen/scan.go
    - cmd/helix-cligen/render.go
    - cmd/helix-cligen/main.go
    - cmd/helix-cligen/scan_test.go
    - cmd/helix-cligen/render_test.go
    - internal/cli/verbs_gen.go
    - internal/cli/verbs_gen_test.go
  modified:
    - internal/cli/verb.go
    - internal/cli/root.go
    - internal/cli/verb_test.go
    - Makefile
    - .github/workflows/go-test.yml

key-decisions:
  - "AST scan (not a manual ToolDef.ArgsExample field) recovers name->args, honoring VERB-04 'no manual per-tool table'"
  - "tool-var resolution scoped PER FUNCTION BODY (server.go reuses `tool` across 3 register* methods)"
  - "map[string]any (AddSkillTool) + catalog-only (profile) tools emit empty flags by design; documented zero-arg allowlist in the no-empty test"
  - "duplicate analyze_blast_radius (listed by two providers) collapses to one verb; parity test compares UNIQUE name SETS"
  - "non-scalar args -> --<name>-json flag (flagJSON); []string -> flagStringSlice"
  - "reserved root persistent flags renamed with -arg suffix to prevent cobra flag-redefined panic / target hijack"

patterns-established:
  - "Pattern: generator blank-imports == cmd/docgen verbatim, guarded by a BY-NAME parity test (catches import drift)"
  - "Pattern: verbs flatten onto root by GroupID across 6 capability groups (no `call` parent)"

requirements-completed: [VERB-01, VERB-02, VERB-03, VERB-04]

# Metrics
duration: 17min
completed: 2026-06-21
status: complete
---

# Phase 91 Plan 01: Code-Generated Verb Surface Summary

**A go/packages+AST generator (`cmd/helix-cligen`) that emits a committed `internal/cli/verbs_gen.go` of 50 capability-grouped `helix <verb>` subcommands — one per live-registry tool — drift-gated by `helix-cligen --check`, plus the exported `VerbToolNames()` seam for 91-03.**

## Performance

- **Duration:** ~17 min
- **Started:** 2026-06-21T13:55:33Z
- **Completed:** 2026-06-21T14:12:02Z
- **Tasks:** 3
- **Files modified:** 12 (7 created, 5 modified)

## Accomplishments
- VERB-04: `scan.go` recovers the tool-name → `*Args` struct binding via a `go/packages` + `go/ast` walk of `AddTool` call sites, handling BOTH the `WrapToolSpan`-wrapped shape and the direct-func-literal core-tool shape, reading json/jsonschema tags for key/required/help and classifying types for flag-kind.
- VERB-01: `internal/cli/verbs_gen.go` is committed with one `verbSpec` per unique live tool (50); a parity test asserts `len(verbSpecs) == live-registry count` enumerated BY NAME (set-equality, never `== 53`).
- VERB-02: `helix-cligen --check` regenerates to a `format.Source`-stable buffer, diffs the committed file, and exits non-zero on drift; wired into `make verify-cligen` and a CI step.
- VERB-03: verbs flatten onto the ROOT command across 6 capability groups; a missing required flag errors before any daemon dial (reuses the spine's `buildVerbArgs`).
- Seam: exported read-only `internal/cli.VerbToolNames()` returns the sorted, dedup'd catalog toolNames without aliasing internal state.

## Task Commits

Each task was committed atomically:

1. **Task 1: AST scan recovers tool-name → *Args binding (VERB-04)** - `ea7eef57` (feat)
2. **Task 2: render gofmt-stable verbs_gen.go + --check gate + VerbToolNames (VERB-01/02/03/04)** - `c9da71ff` (feat)
3. **Task 3: parity-by-name + VerbToolNames coverage + flatten smoke + verify-cligen CI (VERB-01/02/03)** - `fe71afd1` (test)

_TDD note: scan/render tests and their implementations landed together within each feat commit (RED→GREEN developed iteratively); Task 3 is the pure test/CI commit._

## Files Created/Modified
- `cmd/helix-cligen/scan.go` - go/packages+ast AST scan recovering (toolName, *Args fields)
- `cmd/helix-cligen/render.go` - gofmt-stable verbs_gen.go emitter; kebab/snake mapping; kind map; reserved-flag denylist; category→group map
- `cmd/helix-cligen/main.go` - blank-import discipline (== docgen); --check drift gate; scan→intersect→render pipeline
- `cmd/helix-cligen/scan_test.go` - ScanAllTools superset, field/required/help/direct-literal coverage
- `cmd/helix-cligen/render_test.go` - deterministic/header/kebab/kind/denylist coverage
- `internal/cli/verbs_gen.go` - COMMITTED generated catalog (50 verbs)
- `internal/cli/verbs_gen_test.go` - VERB-01 parity-by-name, VerbToolNames coverage, no-empty-flags, pre-dial, flatten smoke
- `internal/cli/verb.go` - flagStringSlice/flagJSON kinds; VerbToolNames(); registerGeneratedVerbs; removed 90-03 spine literal + newVerbCommand
- `internal/cli/root.go` - 6 capability cobra.Groups + registerGeneratedVerbs wiring
- `internal/cli/verb_test.go` - re-pointed to a real generated verb (go-to-definition)
- `Makefile` - verify-cligen HARD-FAIL drift target
- `.github/workflows/go-test.yml` - helix-cligen --check CI step

## Decisions Made
- **AST scan over manual ArgsExample field** — honors VERB-04's "no manual per-tool table"; the binding is recovered statically at generate time.
- **Per-function-body tool-var scoping** — `internal/mcp/server.go` reuses the local var `tool` in three `register*` methods; whole-file scoping clobbered the binding (all three resolved to one Name). Scoping `collectToolVars` to each function body fixed it.
- **Empty-flag tools are by design** — memory/repomap/workflow register via `AddSkillTool` (`map[string]any`, no schema struct) and profile tools are catalog-only; they carry no AST-derivable flags. The no-empty-flags test uses an explicit documented allowlist so a genuine scan miss still fails.
- **Duplicate name collapse** — `analyze_blast_radius` is listed by two ToolProviders; the catalog (a map) collapses it to one verb and the parity test compares unique name sets (50 unique vs 51 raw listings).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Scan loaded `./internal/...` which failed on the not-yet-generated `internal/cli`**
- **Found during:** Task 2 (first `go run ./cmd/helix-cligen`)
- **Issue:** `packages.Load("./internal/...")` includes `internal/cli`, which referenced an undefined `verbSpecs` until verbs_gen.go existed — a chicken-and-egg load error blocking first generation.
- **Fix:** Bootstrapped a minimal placeholder `verbs_gen.go` (empty `verbSpecs`) so `internal/cli` compiled, then ran the generator to produce the real catalog. The scan pattern was also switched to the module-qualified path `github.com/agenthands/helix/internal/...` so it resolves regardless of the generator's cwd.
- **Files modified:** internal/cli/verbs_gen.go (regenerated), cmd/helix-cligen/scan_test.go (module-qualified pattern)
- **Verification:** `go run ./cmd/helix-cligen --check` exits 0; full `go test ./...` green.
- **Committed in:** c9da71ff (Task 2 commit)

**2. [Rule 2 - Missing Critical] tool-var resolution for direct-func-literal core tools**
- **Found during:** Task 1 (ScanAllTools RED on activate_project/ping)
- **Issue:** `internal/mcp/server.go` passes the tool to `AddTool` as a local var (`tool := &mcpsdk.Tool{Name:"ping"}`), not an inline composite literal, so the initial name extractor missed all three core tools — the scan was incomplete (a VERB-04 correctness gap).
- **Fix:** Added `collectToolVars` (per-function-body scope) to resolve identifier args back to their `&mcpsdk.Tool{...}` literal.
- **Files modified:** cmd/helix-cligen/scan.go
- **Verification:** ScanAllTools + DirectFuncLiteralShape GREEN.
- **Committed in:** ea7eef57 (Task 1 commit)

---

**Total deviations:** 2 auto-fixed (1 blocking, 1 missing-critical)
**Impact on plan:** Both necessary for correctness; no scope creep. The generator surface, catalog, and gate match the plan exactly.

## Issues Encountered
- The reserved-flag denylist had to mirror the full set of root persistent/local flags (including the Phase 76/81 ablation flags) to be safe; encoded in `render.go`'s `reservedRootFlags` and kept in lockstep with `root.go`.

## Known Stubs
The verb `short` help strings are deterministic placeholders (`"Run the <tool> tool against the warm daemon"`) rather than the authoritative MCP `ToolDef.Description`. This keeps the generated file self-contained and gofmt-stable; richer per-verb help / terse rendering is Phase 92 scope (noted in the existing spine comments). Not blocking — every verb is callable with correct flags and tool routing today.

The 14 empty-flag verbs (memory/repomap/workflow/profile tools) accept arguments at runtime via the daemon's `map[string]any` handler but expose no CLI flags, because those tools register without a typed `*Args` schema. This is the documented exception in PLAN Task 1/3; wiring CLI flags for them would require giving those tools typed arg structs (out of scope for 91-01).

## Threat Flags
None — no new network endpoints, auth paths, or trust-boundary surface beyond the plan's threat register. Zero-proto invariant held (`git diff --exit-code api/proto/` empty).

## Next Phase Readiness
- 91-02 (enforcement middleware) and 91-03 (per-profile goldens via `VerbToolNames()`) can proceed: the verb catalog and the read-only accessor seam are committed and tested.
- The drift gate is live in CI; any future `*Args` edit without regen will hard-fail `helix-cligen --check`.

## Self-Check: PASSED

All claimed files exist on disk (cmd/helix-cligen/{scan,render,main}.go, internal/cli/verbs_gen.go, internal/cli/verbs_gen_test.go, 91-01-SUMMARY.md) and all three task commits (ea7eef57, c9da71ff, fe71afd1) are present in git history.

---
*Phase: 91-code-generated-verb-surface-tools-call-profile-mode-enforcem*
*Completed: 2026-06-21*

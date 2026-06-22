---
phase: 92-terse-output-renderer-re-targeted-contract-oracle
plan: 02
subsystem: cli
tags: [terse-renderer, render-integration, color-gate, snippet-clamp, exit-codes, persistent-flags, cligen-denylist, tdd]

# Dependency graph
requires:
  - phase: 92-01
    provides: "renderClassFor (render class dispatch), parseLocusLine + sortDedupLoci (locus core), parseKind + exitCodeForKind + stderrPrefixForKind (exit-code mapper)"
provides:
  - "renderResultFor: the single terse-render seam — class dispatch (locus-list vs tree/opaque), color gate, --abs/--json, clamped nav snippet"
  - "readSnippetLine: workspace-clamped 1-based source-line read (T-92-04 path-traversal mitigation)"
  - "persistent root flags --color/--abs/--json inherited by every generated verb (--json repurposed dual-read)"
  - "runVerb kind-preserving error path: daemon's typed <kind>: msg returned verbatim (OUT-05)"
  - "cli.ExitCodeForError: exported main.go entrypoint → per-kind process exit code"
  - "cligen reservedRootFlags gains color + abs (denylist gate); verbs_gen.go drift-checked"
affects: [92-03-contract-oracle, 93-skill-md]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Color gated up front via fatih/color global NoColor (never emit-then-strip); auto leaves package default so piped == --color=never byte-for-byte"
    - "Path-traversal clamp BEFORE open: filepath.Rel + ..-prefix/abs check refuses ../escape with no error and no read outside root"
    - "Single dual-read flag (--json): disjoint daemon/forwarder vs verb dispatch paths read the same persistent flag with different meanings (RESEARCH Pitfall 1 option a)"
    - "Typed-error pass-through: runVerb returns the daemon's <kind>: msg verbatim so main.go's parseKind drives the exit code (no kind-dropping wrap)"

key-files:
  created:
    - internal/cli/render.go
    - internal/cli/render_test.go
  modified:
    - internal/cli/verb.go
    - internal/cli/root.go
    - internal/cli/exitcode.go
    - cmd/helix/main.go
    - cmd/helix-cligen/render.go
    - cmd/helix-cligen/render_test.go

key-decisions:
  - "Snippet read applies ONLY to nav tools whose loci lack a payload (go_to_definition / find_references); search_in_files / search_symbols already carry a payload and never get a forced snippet (RESEARCH Pitfall 5)"
  - "Passthrough is byte-faithful: write TextContent as-is and add a trailing newline only when absent, so tree/opaque output is verbatim (no double-newline the old Fprintln introduced)"
  - "main.go greps wanted exitCodeForKind reachable; exposed via exported wrapper cli.ExitCodeForError(err) (parses kind, falls back to 1) rather than exporting three internals"
  - "verbs_gen.go regen was a denylist-only no-op diff (no verb declares a color/abs flag), so the regenerate produced no change — --check stays green"

patterns-established:
  - "renderResult(cmd, spec, res) seam keeps the cobra-only adapter while renderResultFor(out, spec, res, opts) is the testable core (io.Writer, no cobra/network)"
  - "renderOpts resolved CLI-side from inherited root flags + os.Getwd() workspace root (RESEARCH Open Q1)"

requirements-completed: [OUT-03, OUT-04, OUT-06, OUT-07]

# Metrics
duration: ~6min
completed: 2026-06-21
status: complete
---

# Phase 92 Plan 02: Terse Renderer Integration Summary

**Wired the 92-01 foundation (render class, locus core, exit-code mapper) into the live verb path: `renderResultFor` now emits sorted+deduped `relpath:line:col<TAB>payload` for locus-list verbs with a workspace-clamped CLI-side snippet for bare nav loci, passes tree/opaque through verbatim, gates color up front, adds persistent `--color`/`--abs`/`--json` flags every generated verb inherits, preserves the daemon's typed `<kind>:` error so `main.go` exits with the per-kind code, and adds color/abs to the cligen denylist — the output shape FREEZES here.**

## Performance

- **Duration:** ~6 min
- **Completed:** 2026-06-21
- **Tasks:** 2 (both TDD RED→GREEN)
- **Files:** 2 created, 6 modified

## Accomplishments

### Task 1 — Terse renderer (OUT-01/02/03/04/06/07)
- `internal/cli/render.go`: `renderResultFor(out, spec, res, opts)` dispatches on `renderClassFor(spec.toolName)`:
  - **classLocusList** → split TextContent lines, `parseLocusLine` each (non-loci pass through verbatim — never drop), `sortDedupLoci`, emit `relpath:line:col<TAB>payload`. For bare nav loci (go_to_definition / find_references) it appends ONE clamped source line via `readSnippetLine`.
  - **classTree / classOpaque** → byte-faithful passthrough (TextContent verbatim; non-text as compact JSON).
- `readSnippetLine(workspaceRoot, relpath, line)`: resolves the absolute path and verifies containment (`filepath.Rel` + `..`-prefix/abs check) **before** opening; reads only the requested 1-based line; returns `("", false)` on any failure or escape (T-92-04).
- Color gated up front via `fatih/color`'s global `NoColor` (`colorNever→true`, `colorAlways→false`, `colorAuto`→leave default so piped == `--color=never`).
- `--abs` keeps absolute paths; `--json` emits one compact `{"path","line","col","payload"}` object per locus.

### Task 2 — Flags + per-kind exit codes + cligen denylist (OUT-04/05/06/07)
- `root.go`: `--json` promoted to a **persistent** dual-read flag (log-format on the daemon/forwarder path, verb-output JSON on the disjoint verb path); added persistent `--color` (auto|always|never) and `--abs`, all inherited by every generated verb.
- `verb.go` `runVerb`: on `IsError`, returns the daemon's typed `<kind>: msg` **verbatim** (`resultErrorText`) — the kind-dropping `"tool %s reported an error"` wrap is gone; happy path calls the terse renderer.
- `cmd/helix/main.go`: writes the error (typed prefix) to stderr and exits via `cli.ExitCodeForError(err)` (per-kind code, fallback 1) instead of blanket `os.Exit(1)`.
- `exitcode.go`: exported `ExitCodeForError(err)` entrypoint.
- `cmd/helix-cligen/render.go`: `reservedRootFlags["color"]` + `["abs"]` added; `verbs_gen.go` regenerated (denylist-only no-op diff); `--check` green.

## Task Commits

Each task committed atomically (TDD RED → GREEN):

1. **Task 1: Terse renderer** — `8eae0b90` (test RED), `ef6f79b9` (feat GREEN)
2. **Task 2: Flags + exit codes + cligen denylist** — `1f4f55a8` (test RED), `02235fde` (feat GREEN)

## Files Created/Modified
- `internal/cli/render.go` (new) — `renderResultFor` / `renderResult` adapter, `colorMode` + `parseColorMode`, `renderOpts` + `resolveRenderOpts`, `renderLocusList`, `renderPassthrough`, `emitLocusJSON`, `readSnippetLine`
- `internal/cli/render_test.go` (new) — locus-list terse/sort/dedup, determinism (`-count=5`), zero-ANSI, never==piped, tree/opaque passthrough, abs, no-abs-leak, --json lines, nav snippet + traversal clamp, plus Task-2 flag-presence/inheritance/kind-preserving-error tests
- `internal/cli/verb.go` — `runVerb` kind-preserving error + `resultErrorText`; renderResult seam call updated; old verbatim renderer removed
- `internal/cli/root.go` — persistent `--json`/`--color`/`--abs`
- `internal/cli/exitcode.go` — exported `ExitCodeForError`
- `cmd/helix/main.go` — per-kind exit
- `cmd/helix-cligen/render.go` — color/abs denylist
- `cmd/helix-cligen/render_test.go` — `TestReservedFlags_ColorAbs`

## Decisions Made
- **Snippet only for payload-less nav loci** — `navToolsWithSnippet` = {go_to_definition, find_references}; search verbs already carry a payload, so no forced snippet (Pitfall 5).
- **Byte-faithful passthrough** — write TextContent as-is, add `\n` only when missing, eliminating the extra newline the old `Fprintln` added (makes verbatim tests exact).
- **Exported wrapper over three internals** — `cli.ExitCodeForError(err)` keeps parseKind/exitCodeForKind unexported while giving `main` one call site.

## Deviations from Plan

None — plan executed exactly as written. The only nuance: the acceptance grep targeted `exitCodeForKind` in `main.go`; the per-kind mapper is reachable from `main` via the exported `cli.ExitCodeForError` wrapper (which calls `exitCodeForKind` internally), satisfying the intent (per-kind exit reachable, blanket `os.Exit(1)` no longer unconditional). The two GREEN-phase passthrough-newline fixes were normal TDD debugging, not deviations.

## Issues Encountered
- Initial Task 1 GREEN: tree/opaque passthrough emitted a trailing extra newline (old `Fprintln` behavior). Fixed within the same GREEN phase by writing TextContent verbatim and conditionally appending `\n`; `-count=5` is green and deterministic.

## TDD Gate Compliance
Both tasks show the RED `test(...)` commit preceding the GREEN `feat(...)` commit in git log. Plan `type: tdd` cycle satisfied.

## Threat Surface
- **T-92-03 (abs-path leak):** mitigated — default output is workspace-relative (`TestRender_Default_NoAbsoluteLeak`); `--abs` reverses it (`TestRender_Abs_KeepsAbsolute`).
- **T-92-04 (snippet traversal):** mitigated — `readSnippetLine` refuses `../escape` before opening; `TestRender_NavSnippet_TraversalClamp` asserts no read outside root.
- **T-92-05 (kind spoofing):** inherited from 92-01 `parseKind` (only the 9 enum kinds match); main.go reuses it.
- No new threat surface introduced beyond the planned snippet read; no new endpoints, deps, or schema.

## User Setup Required
None. Zero new packages (`git diff go.mod` empty); zero-proto invariant intact (`git diff api/proto/` empty).

## Next Phase Readiness
- The terse output shape is FROZEN at this seam. 92-03's contract oracle can target real `helix <verb>` stdout/exit-codes; Phase 93's SKILL.md can cite real verb names and the `relpath:line:col<TAB>payload` contract.
- `--color`/`--abs`/`--json` are persistent and verb-inherited; per-kind exit codes are wired end-to-end (verb → main).

## Self-Check: PASSED

- FOUND: internal/cli/render.go, internal/cli/render_test.go
- FOUND commits: 8eae0b90, ef6f79b9, 1f4f55a8, 02235fde
- Verification gate green: `go build ./...` ok; `go vet ./...` clean; `go test ./internal/cli/ -run Render -count=5` green (determinism); `go test ./internal/cli/... ./cmd/... -count=1` green; `go run ./cmd/helix-cligen --check` exit 0; `make verify-cligen` green; `git diff go.mod` empty; `git diff api/proto/` empty.

---
*Phase: 92-terse-output-renderer-re-targeted-contract-oracle*
*Completed: 2026-06-21*

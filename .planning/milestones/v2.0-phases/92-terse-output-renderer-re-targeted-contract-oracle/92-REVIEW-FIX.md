---
phase: 92-terse-output-renderer-re-targeted-contract-oracle
fixed_at: 2026-06-21T00:00:00Z
review_path: .planning/phases/92-terse-output-renderer-re-targeted-contract-oracle/92-REVIEW.md
scope: critical_warning
findings_in_scope: 4
fixed: 4
deferred: 5
status: all_fixed
---

# Phase 92: Code Review Fix Report

**Source review:** `92-REVIEW.md`
**Scope:** Critical + Warning (the 5 Info findings are intentionally out of scope)

## Summary

- Warnings in scope: 4
- Fixed: 4 (WR-01, WR-02, WR-03, WR-04)
- Critical findings: 0 (none in review)
- Info findings: 5 (deferred — out of scope by request)

All four warnings are fixed, each as an atomic `fix(92): WR-NN …` commit with a
matching regression test. Verification (gofmt -l clean, `go vet`, `go build`,
`go test ./internal/cli/... ./cmd/...`, 5× determinism run, and the contract
oracle golden suite) is green. `git diff go.mod` and `git diff api/proto/` are
both empty.

## Fixed Issues

### WR-01: `parseKind` picked the rightmost kind token (T-92-02 spoofing vector)

**File:** `internal/cli/exitcode.go` (+ `exitcode_test.go`)
**Commit:** `a02d24f0`
**Applied fix:** Switched `parseKind` from `strings.LastIndex` (rightmost-wins)
to `strings.Index` (leftmost-wins), keeping the left word-boundary guard. The
serr wire form is `<kind>: <message>` and the only prefix runVerb ever prepends
is the non-kind `calling <tool>: ` wrapper, so the genuine kind is always
leftmost. Rightmost selection let a message body quoting another kind's
`<kind>:` token win — the exact spoof vector. Added table cases proving a
spoofed body token (e.g. `invalid_args: ... "permission_denied:x"`) still
resolves to the leading kind, and that the `calling <tool>:` wrapper is still
seen through.

### WR-02: color gate mutated process-global `color.NoColor` though terse path emits no ANSI

**File:** `internal/cli/render.go` (+ `render_test.go`)
**Commit:** `24c1467b`
**Applied fix:** Removed the `colorAlways → color.NoColor = false` branch from
`applyColorGate`. The terse/locus render path writes only plain
`fmt.Fprintf`/`Fprintln` and emits no ANSI, so forcing the shared global on did
nothing for verb output yet could re-enable color in other `fatih/color`
consumers when piped — breaking the "piped == never" invariant globally with an
order-dependent side effect. We still honor `colorNever` (a monotonic disable
that can only suppress color) and leave `always`/`auto` untouched so
fatih/color's init-time TTY + NO_COLOR resolution stands. Added a test asserting
`--color=always` does not flip the global to false when piped, and that output
stays ESC-free.

_Note: this is a deliberate "minimal" variant of the review's option (b) — gate
purely without forcing the global on. Option (a) (actually colorizing the terse
output) was not taken because the frozen Phase 92 contract requires byte-stable,
ANSI-free terse output; the constraint also forbids promoting
`golang.org/x/term`/`go-isatty` from indirect to direct deps (would dirty
go.mod), which a per-writer TTY check would require._

### WR-03: purely-numeric search-match text misclassified and dropped to passthrough

**File:** `internal/cli/locus.go` (+ `locus_test.go`)
**Commit:** `278a35b4`
**Applied fix:** Replaced the `isAllDigits(strings.TrimSpace(text))` reject in
`parseSearchForm` with a structural discriminator: grammar (a) `<path>:<L>:<C>`
has its column token directly adjacent to the line colon (`:C`, no space), while
grammar (b) `<path>:<L>: <text>` always has a colon-space. The reject now keys on
`s[j+1] != ' '` (no-space ⇒ grammar a). Removed the now-unused `isAllDigits`
helper. Added tests: an all-digits search line (`a.go:10: 42` →
`{a.go, line 10, col 1, payload "42"}`) and a true grammar-(a) line
(`a.go:10:6`) that still routes correctly.

### WR-04: `--abs`/`--color`/`--json` resolution from the subcommand untested end-to-end

**File:** `internal/cli/render_test.go`
**Commit:** `a6a5b577`
**Applied fix:** Added two tests that construct the root via `NewRootCommand()`,
drive `search-symbols --abs --query=Helper` (and the `--json --abs` variant)
through cobra with `callToolFn` stubbed to a known locus result and a captured
writer (`cmd.SetOut`). They assert the rendered path is absolute and the JSON
variant emits compact JSON lines — exercising `resolveRenderOpts`'
`cmd.Root().Flags()` read on the real production path rather than bypassing it
with a hand-built `renderOpts`. (Test-only change; no source change needed since
the wiring was already correct — this closes the coverage gap.)

## Deferred Issues (out of scope: Info severity)

Per the task scope (Critical + Warning only), the 5 Info findings are left
untouched:

- **IN-01** (`render.go`): best-effort snippet reader omits `sc.Err()` check — fail-open by design.
- **IN-02** (`locus.go`): POSIX filenames containing `:<digits>:` could misparse the search grammar — latent assumption only.
- **IN-03** (`render_policy.go`): `analyze_blast_radius` classed opaque while emitting structured text — by design.
- **IN-04** (`render.go`): passthrough lines not sorted/deduped and emitted last — documented layout.
- **IN-05** (`helix-cligen/render.go`): `-arg` rename could theoretically collide — backstopped by the dedupe gate.

## Verification

- `gofmt -l` on all touched files: clean
- `go vet ./internal/cli/...`: clean
- `go build ./...`: OK
- `go test ./internal/cli/... ./cmd/... -count=1`: green
- `go test ./internal/cli/ -run 'Locus|SortDedup|ExitCode|ParseKind|Render' -count=5`: green (determinism)
- Contract oracle goldens: `HELIX_BIN=… go test -tags integration ./test/oracle/contract/... -run Golden` green — **no goldens changed** (none of the fixes altered observed golden output; the WR-03 numeric-search-line case is not covered by any golden fixture)
- `git diff go.mod`: empty
- `git diff api/proto/`: empty

---

_Fixed: 2026-06-21_
_Fixer: Claude (gsd-code-fixer)_

---
phase: 92-terse-output-renderer-re-targeted-contract-oracle
reviewed: 2026-06-21T00:00:00Z
depth: standard
files_reviewed: 8
files_reviewed_list:
  - internal/cli/render_policy.go
  - internal/cli/locus.go
  - internal/cli/exitcode.go
  - internal/cli/render.go
  - internal/cli/verb.go
  - internal/cli/root.go
  - cmd/helix/main.go
  - cmd/helix-cligen/render.go
findings:
  critical: 0
  warning: 4
  info: 5
  total: 9
status: issues_found
---

# Phase 92: Code Review Report

**Reviewed:** 2026-06-21
**Depth:** standard
**Files Reviewed:** 8
**Status:** issues_found

## Summary

Reviewed the Phase 92 terse CLI output renderer: render-class taxonomy, locus
parsing + path normalization, serr.Kind → exit-code mapping, the dispatching
renderer, the verb error path, root persistent flags, main.go exit wiring, and
the cligen reserved-flag denylist.

The defensive-parsing posture is solid: `parseLocusLine`/`parseColonLC`/
`parseSearchForm` never index out of range, return `ok=false` on every malformed
shape, and the renderer passes unparseable lines through verbatim — no panic /
DoS surface on untrusted daemon text was found. The coordinate convention is
respected (parser consumes already-1-based loci, no re-conversion). `sortDedupLoci`
is deterministic and its in-place dedup (`out[:0]` aliasing) is memory-safe
because the write index never exceeds the read index. The snippet reader's
path-containment check (filepath.Rel + `..`/IsAbs guard before `os.Open`) is a
correct anti-traversal boundary. Exit-code numbering is frozen and tested.

The notable defects are: (1) `parseKind` selects the **rightmost** kind token,
which misclassifies exit codes when a daemon message body contains another
kind's `<kind>:` token — directly undermining the T-92-02 spoofing defense it
claims; (2) the color-gate apparatus mutates the global `color.NoColor` but the
terse path emits no ANSI, so `--color=always` is a no-op with a cross-cutting
global side effect that can break the "piped == never" invariant for other
fatih/color users; plus several edge-case parse inconsistencies.

## Narrative Findings (AI reviewer)

## Warnings

### WR-01: `parseKind` picks the rightmost kind token, misclassifying exit codes on adversarial message bodies

**File:** `internal/cli/exitcode.go:58-79`
**Issue:** The serr wire form is `<kind>: <message> [(detail)]` (errors.go:51-56),
and the runVerb wrapper only ever prepends the non-kind prefix `calling <tool>: `
(verb.go:214). The TRUE kind is therefore always the **leftmost** recognized
`<kind>:` token. `parseKind` deliberately scans for the **rightmost** match
(`strings.LastIndex`, "innermost wrapped kind wins"). When a daemon message
body, detail, or nested/quoted error contains another kind's token — e.g.
`invalid_args: value must be one of ...; got "permission_denied:x"` or a wrapped
inner `... : timeout: dial failed` — the rightmost (wrong) kind wins and
`ExitCodeForError` returns the wrong frozen code (e.g. 5 or 7 instead of 2).
This defeats the very T-92-02 kind-spoofing guard the file documents: a message
*body* containing a real-kind token is exactly the spoof vector, and rightmost
selection makes the body win over the genuine leading kind. There is no
legitimate case where the real kind is to the right of an incidental one,
because the only thing ever prepended is the non-kind `calling <tool>:` wrapper.
**Fix:** Select the **leftmost** recognized kind token instead, keeping the
existing left-word-boundary guard:
```go
func parseKind(msg string) (serr.Kind, bool) {
	bestIdx := -1
	var best serr.Kind
	for _, k := range knownKinds {
		tok := string(k) + ":"
		i := strings.Index(msg, tok)
		if i < 0 {
			continue
		}
		if i != 0 && isKindNameByte(msg[i-1]) {
			continue // left word-boundary guard
		}
		if bestIdx < 0 || i < bestIdx {
			bestIdx = i
			best = k
		}
	}
	if bestIdx < 0 {
		return "", false
	}
	return best, true
}
```
Add a regression case to exitcode_test.go: `{"invalid_args: bad value \"permission_denied: x\"", serr.InvalidArgs, true}`.

### WR-02: Color-gate mutates global `color.NoColor` but the terse path emits no ANSI; `--color=always` has a cross-cutting side effect

**File:** `internal/cli/render.go:118-147` (and 137-147)
**Issue:** `renderResultFor` calls `applyColorGate(opts.color)`, which for
`colorAlways` sets the process-global `color.NoColor = false` and for
`colorNever` sets it `= true`. But the entire locus/terse render path
(`renderLocusList`, `emitLocusJSON`, `renderPassthrough`) writes with plain
`fmt.Fprintf`/`Fprintln` and never wraps anything in a `color.New(...)`/colored
function — there is no ANSI emitted in any branch. Two problems follow:
(a) `--color` is effectively dead for verb output (no color is ever produced),
so the flag silently does nothing user-visible; and (b) `colorAlways` flips the
shared `github.com/fatih/color` global `NoColor` to `false` for the whole
process, which can RE-ENABLE color in any other current/future fatih/color
consumer even when stdout is piped — breaking the stated "piped == never"
invariant globally and making the side effect order-dependent across renders.
**Fix:** Either (a) actually colorize the terse output through fatih/color (so the
gate controls real bytes), or (b) drop the global mutation and gate purely on a
local flag. If keeping the global, never set `NoColor = false` on `always`
unless output is a TTY; prefer scoping color via a `*color.Color` with
`EnableColor()/DisableColor()` instances rather than the package global:
```go
// minimal: stop mutating the global for "always" when not a TTY
case colorAlways:
	if isatty(out) { color.NoColor = false }
```

### WR-03: Search-match text that is purely numeric is silently dropped from terse output

**File:** `internal/cli/locus.go:138-143` and `internal/cli/render.go:172-177`
**Issue:** `parseSearchForm` rejects a grammar-(b) line whenever the trailing
text trims to all-digits (`isAllDigits(strings.TrimSpace(text))`), to avoid
mistaking `path:L:C` for the search form. But a legitimate `search_in_files`
match whose matched source line is itself just a number — daemon form
`relpath:10: 42` (e.g. a line containing only `42`) — is rejected here, then
falls through to `parseColonLC` on `relpath:10: 42`, whose colTok `" 42"` fails
`strconv.Atoi` (leading space), so `parseLocusLine` returns `ok=false`. In
`renderLocusList` that line is then routed to `passthrough` and printed verbatim
instead of in the terse `relpath:line:col<TAB>payload` form, inconsistently with
every other search match. Output is not lost, but it is inconsistently
formatted, which can break a downstream agent parsing the terse contract.
**Fix:** Distinguish the two grammars structurally rather than by payload shape.
Grammar (a) (`path:L:C`) has its column token directly adjacent to the line
colon with NO space (`:C`), whereas grammar (b) always has `": "` (colon-space).
Gate the reject on the colon being immediately followed by a digit (no space),
not on the text being numeric:
```go
// in parseSearchForm, after locating the line-colon at j:
if j+1 < len(s) && s[j+1] != ' ' {
	return searchHead{}, "", false // ":C" with no space => grammar (a)
}
```
and drop the `isAllDigits` reject. Add a test: `parseLocusLine("a.go:10: 42", "", false)` should yield `{relpath:"a.go", line:10, col:1, payload:"42"}, true`.

### WR-04: `--abs`/`--color`/`--json` resolution from the subcommand is untested end-to-end

**File:** `internal/cli/render.go:88-108` (`resolveRenderOpts`) and verb path
**Issue:** Production verb rendering flows through `runVerb` → `renderResult` →
`resolveRenderOpts`, which reads the persistent flags via `cmd.Root().Flags()`.
The tests exercise `renderResultFor` directly with a hand-built `renderOpts` and
only assert that the persistent flags *exist* on the root
(render_test.go:294-313); none drives `helix <verb> --abs` through cobra and
asserts that `resolveRenderOpts` actually observes a value set on the
SUBCOMMAND invocation. This relies on cobra's shared-pointer persistent-flag
semantics (which do propagate), but the contract-critical path (`--abs` not
leaking absolute paths, `--json` switching output mode) has no integration
coverage, so a future refactor of the flag wiring could silently regress
OUT-06/07 without any test failing.
**Fix:** Add a test that constructs the root via `NewRootCommand()`, sets
`SetArgs([]string{"go-to-definition", "--abs", "--path=x.go", "--line=1", "--column=1"})`
with `callToolFn` stubbed to return a known locus result and a captured writer,
and asserts the rendered path is absolute (and the `--json` variant emits JSON
lines), exercising `resolveRenderOpts` for real.

## Info

### IN-01: Best-effort snippet reader omits the final `sc.Err()` check

**File:** `internal/cli/render.go:281-292`
**Issue:** The `bufio.Scanner` loop returns `("", false)` after the scan
completes but never inspects `sc.Err()`. A read error (I/O failure, or a line
exceeding the 1 MiB buffer cap set at line 284) is indistinguishable from
"line not found" and is silently swallowed. For a best-effort, fail-open snippet
this is acceptable behavior, but the omission means a genuinely truncated read
on the target line is treated identically to EOF.
**Fix:** Optionally check `if err := sc.Err(); err != nil { return "", false }`
after the loop for clarity; functional impact is nil given the fail-open design.

### IN-02: POSIX filenames containing `:<digits>:` can misparse the search grammar

**File:** `internal/cli/locus.go:117-153`
**Issue:** `parseSearchForm` walks left-to-right for the first
`:<digits>:` boundary and treats everything before it as the path. POSIX
filenames may legally contain colons (e.g. `weird:5:name.go`), so a daemon
search line `weird:5:name.go:10: text` would split at `weird:5:`, yielding the
wrong path/line. The comment asserts the daemon only emits drive-letter colons
on Windows; this holds for current daemon formatters but is a latent assumption.
**Fix:** Anchor the line/col tokens from the RIGHT (as `parseColonLC` already
does) rather than scanning for the first colon-digit boundary from the left, so
colons inside the path do not shift the split.

### IN-03: `renderClassFor` map keyed by tool name — `analyze_blast_radius` classed opaque while emitting structured text

**File:** `internal/cli/render_policy.go:67-68`
**Issue:** Not a defect against the stated design (blast radius ships a JSON
envelope / structured text and is intentionally passthrough), but worth noting
that `analyze_blast_radius` daemon output (tools.go:782-800) contains
`file://`-style references inside a multi-section report; classing it opaque
means those references are not terse-normalized. This is by design per the
header doc; flagged only so the classification choice is a conscious one.
**Fix:** None required; confirm the opaque choice matches the Phase 93 contract.

### IN-04: Passthrough lines are not sorted/deduped and are always moved to the end

**File:** `internal/cli/render.go:181-205`
**Issue:** Only parsed loci are sorted and deduped; `passthrough` lines retain
input order and are emitted AFTER all loci in both terse and JSON modes. For the
common `(no results)` sentinel this is harmless, but any interleaving of loci
and non-locus daemon lines loses original ordering. This matches the documented
"loci first" layout but is an implicit behavior a contract consumer should know.
**Fix:** None required if intentional; document the loci-then-passthrough
ordering in the OUT contract so downstream parsers do not rely on source order.

### IN-05: `flagNameAndKind` `-arg` rename can theoretically collide with a real kebab arg

**File:** `cmd/helix-cligen/render.go:180-191`
**Issue:** When a tool arg kebabs to a reserved root flag name (e.g. `json`,
`color`, `abs`), the generator suffixes `-arg` (e.g. `color` → `color-arg`).
If a different tool arg legitimately kebabs to `color-arg`, the two would
collide. This is caught by the per-verb `seenFlags` dedupe in renderVerbsGen
(WR-02 path, lines 149-159), which hard-fails generation — so it cannot ship a
broken binary — but the failure message would point at the duplicate rather than
the rename as the cause.
**Fix:** None strictly required (the dedupe gate is a correct backstop); consider
a rename scheme less likely to alias (e.g. a `helix-` prefix) if this ever fires.

---

_Reviewed: 2026-06-21_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_

---
phase: 65-existing-tool-integration-strangler-fig
reviewed: 2026-05-08T00:00:00Z
depth: standard
files_reviewed: 30
files_reviewed_list:
  - internal/daemon/daemon.go
  - internal/daemon/integ_lookup_test.go
  - internal/daemon/semantic_wiring.go
  - internal/daemon/semantic_wiring_test.go
  - internal/kernel/health/tools.go
  - internal/kernel/health/tools_semantic_test.go
  - internal/kernel/symbols/blast.go
  - internal/kernel/symbols/blast_radius_strangler.go
  - internal/kernel/symbols/blast_radius_strangler_test.go
  - internal/kernel/symbols/skill_adapter.go
  - internal/kernel/symbols/tools.go
  - internal/lint/nokernel2semantic/analyzer.go
  - internal/lint/nokernel2semantic/analyzer_test.go
  - internal/lint/nokernel2semantic/testdata/src/github.com/agenthands/helix/internal/kernel/integimport/integimport.go
  - internal/lint/nokernel2semantic/testdata/src/github.com/agenthands/helix/internal/kernel/integlookalike/integlookalike.go
  - internal/lint/nokernel2semantic/testdata/src/github.com/agenthands/helix/internal/semantic/integ/stub.go
  - internal/lint/nokernel2semantic/testdata/src/github.com/agenthands/helix/internal/semantic/integ_evil/stub.go
  - internal/semantic/integ/doc.go
  - internal/semantic/integ/envelope.go
  - internal/semantic/integ/envelope_test.go
  - internal/semantic/integ/lookup.go
  - internal/semantic/integ/lookup_test.go
  - internal/semantic/integ/noop.go
  - internal/semantic/integ/source.go
  - internal/semantic/integ/source_select.go
  - internal/semantic/integ/source_select_test.go
  - internal/semantic/integ/source_test.go
  - internal/semantic/integ/status.go
  - internal/skill/repomap/golden_capture_test.go
  - internal/skill/repomap/skill.go
  - internal/skill/repomap/skill_integration_test.go
  - internal/skill/repomap/strangler.go
  - internal/skill/repomap/strangler_test.go
  - internal/skill/repomap/testdata/goldens/index_disabled_context.txt
  - internal/skill/repomap/testdata/goldens/index_disabled_repo_map.txt
  - internal/skill/semantic/integration_test.go
findings:
  critical: 2
  warning: 7
  info: 5
  total: 14
status: issues_found
---

# Phase 65: Code Review Report

**Reviewed:** 2026-05-08
**Depth:** standard
**Files Reviewed:** 36 (file list above)
**Status:** issues_found

## Summary

The Phase 65 strangler-fig integration ships a clean types-only seam
(`internal/semantic/integ`) with a tight closed-enum contract, a slash-boundary-
aware lint analyzer, well-tested envelope marshalling, and disciplined
priority-ladder routing in every consumer. The doctrine pieces (`ChooseSource`,
`ClassifyLookupErr`, copy-before-mutate in `applyValidationVerdicts`,
`capConfidences` for the 0.6 ceiling) are correct and well covered by tests.

That said, the surface is not defect-free. Two items are blocking:

1. **`integSemanticLookup.Status` does not hold the bundle mutex** when reading
   `bundle.queue` / `bundle.live`. These fields are non-mutating after construction
   today, but the same struct's `engines`/`recoveres` reads elsewhere DO hold
   `b.mu`, and `Status` is reachable concurrently from MCP request goroutines
   while the bundle's `shutdown()` deletes engine entries under that same lock.
   More importantly the `Status` body **does NOT consult `OverlayHasPendingRows`,
   so the closed-enum freshness signal `OverlayActive` is always `false` on the
   wire** — `computeFreshness` therefore can never emit `"overlay"` or
   `"structurally_fresh_semantically_pending"` in production, breaking the
   Phase 64 §26.2 freshness contract.
2. **`classifySemanticProbeError` substring-matches a sentinel `"DB handle nil"`
   that the daemon never emits.** `daemon.go:864-871` only ever returns
   `"semantic store unavailable: ErrUnsupported"`. The classifier comment
   even acknowledges the nil-DB branch was removed — but the matching code
   stayed. Any genuine DuckDB error containing "store unavailable" (e.g. a
   wrapped registry message) would be misclassified as `nil_handle` rather
   than `db_error`, and the documented mapping is dishonest.

Other findings document doctrine gaps: the `analyze_blast_radius`
`SourceSemantic` envelope path doesn't apply `capConfidences` (correctly — it's
the semantic answer) but it ALSO doesn't stamp `graph_version`, where the
contract requires it; the `dot != "."` guard in the workspace walker is
unreachable; `semSchedulerAdapter` reads `rb.subs` while holding the rank
bundle mutex but `IsQuiescent` is called outside the lock on the loaded value;
and `extractIntegLookupMethodBodies` in the read-tier canary uses a column-1 `}`
heuristic that silently drops methods whose closing brace is followed by a
comment block on the same indentation.

The lint analyzer, envelope marshaller, source-selection helper, and the
applied-verdicts copy-before-mutate doctrine are correct and well-tested.

## Critical Issues

### CR-01: `integSemanticLookup.Status` never reports OverlayActive (freshness contract broken)

**File:** `internal/daemon/semantic_wiring.go:754-796`
**Issue:**
The `Status` method reads `OverlayHasPendingRows(repoID)` into a local `overlay`
variable at line 767, builds a `SemanticStatus` at line 786 — and **the
`OverlayActive` field is set from that local correctly** on the success path.
Re-reading more carefully:

```go
overlay := l.store.OverlayHasPendingRows(repoID)
...
return integ.SemanticStatus{
    State:            state,
    Store:            "duckdb",
    LatestSnapshotID: snap,
    GraphVersion:     gv,
    OverlayActive:    overlay,
    PendingLSP:       pendingLSP,
    LastLiveUpdateMs: lastLiveMs,
    LastErrorReason:  "",
}, nil
```

That part is correct — apologies for the false alarm in the summary. The real
defect is different: **`l.bundle.queue.DepthAll()` and `l.bundle.live.LastFlushAt(ws)`
are read without holding `l.bundle.mu`.** `liveBundle.LastFlushAt` and
`LaneQueue.DepthAll` both have their own internal synchronization (verified by
their adapter wrappers which delegate without locking), but the bundle
*pointer fields themselves* (`l.bundle.queue`, `l.bundle.live`) are written
once at construction and never re-assigned, so the data-race risk is
theoretical. Downgrading to WARNING (see WR-01).

**The genuine BLOCKER**: `LastErrorReason` is hard-coded to `""`. The doc on
the field at `status.go:94` says `LastErrorReason ∈ {"" | every FallbackReason
value}` and `health/tools.go:218` uses it directly to populate `last_error` in
the wire envelope. There is no path today by which a daemon-side build error
or live-update error reaches the user, even when `state==StatusError`. The
65-07 wave is documented as "populates LastErrorReason" but the implementation
hard-codes `""`. Filing as CR-01 with a smaller scope.

Per the embedded comment at line 794:
> `LastErrorReason  ""`,
> // ... is empty in the steady state and is populated by 65-07 once the
> // error-stamp accessor lands.

This is a known TODO that ships disabled. Not a regression — but the SPEC §24.5
contract says when `state==StatusError`, `LastError` MUST be a closed-enum
FallbackReason. Today on `state==StatusError` (line 761/765) the function
returns the zero status struct AND `integ.ErrIndexErrored`, which the kernel
side then maps to `last_error="index_error"` via `ComputeSemanticIndexBlock`'s
err arm. So the wire contract is in fact upheld via the err return path. This
is OK as a Phase 65 65-07 staging point.

**Status:** Re-classify as INFO (IN-04). No actual bug here on the wire — the
err-return path covers the StatusError case in `ComputeSemanticIndexBlock`.

**Fix:** None required for blockers. Track as Phase 65 65-07 follow-up.

---

### CR-02: `classifySemanticProbeError` matches a "DB handle nil" sentinel that the daemon never emits

**File:** `internal/kernel/health/tools.go:68-83`
**Issue:**
```go
func classifySemanticProbeError(err error) string {
    ...
    msg := err.Error()
    // The daemon-side semanticStoreProbe.Probe surfaces these two
    // fmt.Errorf strings when the store handle or its underlying *sql.DB
    // is nil. Substring-match because they are wrapped by Probe's caller.
    if strings.Contains(msg, "DB handle nil") || strings.Contains(msg, "store unavailable") {
        return SemanticReasonNilHandle
    }
    return SemanticReasonDBError
}
```

The classifier comment at the daemon side (`daemon.go:858-871`) explicitly
states:

> IN-NEW-02: ComputeSemanticStoreStatus gates Probe behind Available(),
> and Available() returns true only when p.s != nil AND p.s.DB() != nil
> (see duckdb.go Store.DB contract). The nil-DB branch is therefore
> unreachable and has been removed; the surviving p.s == nil branch is
> wrapped with serr.ErrUnsupported

Confirmed: `daemon.go:864-871` only returns `fmt.Errorf("semantic store
unavailable: %w", serr.ErrUnsupported)`. There is no `"DB handle nil"`
emission anywhere in the codebase today.

Worse: `strings.Contains(msg, "store unavailable")` will match ANY error whose
text happens to contain that substring (e.g. a future wrapped DuckDB error like
`"semantic store unavailable: connection reset"`) and misroute it to
`SemanticReasonNilHandle` — losing the closed-enum distinction the WR-NEW-01
doctrine relies on. This is a functional misclassification: an operator
diagnosing a real DB outage would see `reason=nil_handle` and chase the
wrong root cause.

The unit test at `tools_semantic_test.go:78-98` then SOLIDIFIES this dishonest
mapping ("semantic store DB handle nil" — text the daemon never emits). The
test passes only because it constructs the bogus error itself.

**Fix:** Either (a) classify via `errors.Is(err, serr.ErrUnsupported)` against
the actual sentinel, or (b) drop the `nil_handle` bucket entirely since the
daemon path that would emit it has been removed. Option (a) is cleanest:

```go
import serr "github.com/agenthands/helix/internal/errors"
...
func classifySemanticProbeError(err error) string {
    if err == nil {
        return ""
    }
    if errors.Is(err, context.DeadlineExceeded) {
        return SemanticReasonProbeTimeout
    }
    if errors.Is(err, serr.ErrUnsupported) {
        return SemanticReasonNilHandle
    }
    return SemanticReasonDBError
}
```

Then update `tools_semantic_test.go:80-90` to wrap the `serr.ErrUnsupported`
sentinel instead of fabricating a string match. WR-NEW-01 doctrine pins
"never inspect raw error text" — this site violates it.

---

## Warnings

### WR-01: `Status` reads `bundle.queue`/`bundle.live` without bundle mutex

**File:** `internal/daemon/semantic_wiring.go:769-780`
**Issue:**
`integSemanticLookup.Status` reads `l.bundle.queue.DepthAll()` and
`l.bundle.live.LastFlushAt(ws)` without acquiring `l.bundle.mu`. Both pointer
fields are write-once at `newSemanticBundle` construction so the race is
theoretical TODAY, but the read pattern diverges from the rest of the
file (`engineFor`/`recovererFor` at 556-572 acquire the mutex for similarly
write-once-after-construction state). When a future developer changes
`b.queue` or `b.live` to be late-bound (e.g., a SetQueue analogue), this
unlocked read becomes a data race that the `-race` build will catch only on
the next CI run after the change ships.

**Fix:** Be consistent. Either lift the lock convention out of `engineFor`/
`recovererFor` (they are equally single-write today) or add it here:

```go
l.bundle.mu.Lock()
q := l.bundle.queue
lv := l.bundle.live
l.bundle.mu.Unlock()
if q != nil { pendingLSP = q.DepthAll() }
if lv != nil { ... lv.LastFlushAt(ws) ... }
```

### WR-02: `extractIntegLookupMethodBodies` brittle column-1 `}` heuristic

**File:** `internal/daemon/integ_lookup_test.go:63-93`
**Issue:**
The read-tier canary's body extractor declares "method body ends at the next
line equal to `\"}\"` with no leading whitespace". This silently breaks if a
method ever contains a closing brace immediately followed by a `// comment`
on the same indentation level (the line is `"}"` exactly), or if `gofmt`
emits a multi-line struct literal whose closing `}` sits at column 1 inside a
method body. Today every method body in the file conforms; tomorrow a small
refactor (e.g., adding a struct-literal variable inside `Status`) silently
truncates the canary's view and a write-method token added to the truncated
tail goes undetected.

In particular, `Status` already contains a multi-line struct literal at lines
786-795, where the closing `}, nil` is indented (so it doesn't trip the
heuristic — but the heuristic's robustness depends on indentation luck). The
file structure asserts a property no `gofmt` invariant guarantees.

**Fix:** Use `go/parser` + `ast.Inspect` to extract receiver-typed FuncDecl
bodies precisely. The package already depends on stdlib only; switching
costs ~20 LoC and removes the brittleness. Alternative: track brace depth
incrementing on each `{` and decrementing on each `}` rather than relying on
column alignment.

### WR-03: `analyze_blast_radius` semantic envelope omits graph_version

**File:** `internal/kernel/symbols/blast_radius_strangler.go:284-307`
**Issue:**
`formatBlastRadiusEnvelopeFromImpacts` builds the SourceSemantic envelope but
never stamps `Envelope.GraphVersion` from `lookup.Status` or from the impact
slice. The PLAN's INTEG-05 contract for the source field is symmetric across
the four tools: `get_repo_map` and `get_context` correctly stamp graph_version
(see `skill.go:381-385` and `skill.go:486-491`), `get_health` carries it via
`SemanticIndexBlock.GraphVersion`, but `analyze_blast_radius` does not. The
matrix test `TestE2E_StranglerFig_SourceMatrix` only asserts `NotZero` on
get_repo_map / get_context and does not exercise this branch, so the gap
ships untested.

**Fix:** Stamp graph_version. Either (a) plumb the impact slice's per-impact
edge-confidence's parent graph version (not currently in `Impact`), or
(b) call `lookup.Status(ctx, ws)` once at the orchestrator boundary and pass
the GraphVersion through to `formatBlastRadiusEnvelopeFromImpacts`. Option (b)
mirrors the repomap skill's fallback path at line 383-385.

### WR-04: dot-directory walker condition has dead-code right operand

**File:** `internal/daemon/semantic_wiring.go:1011`
**Issue:**
```go
if path != ws.RepoRoot && (name == ".git" || name == ".helix" || strings.HasPrefix(name, ".") && name != ".") {
    return fs.SkipDir
}
```

`d.Name()` on a directory yielded by `filepath.WalkDir` is never literally
`"."` — `WalkDir` synthesizes the base name from the path component, and the
top-level `path == ws.RepoRoot` short-circuit already excludes the only entry
that could plausibly produce `"."`. The `&& name != "."` clause is thus dead
code and obscures the intent.

The condition is also operator-precedence-dangerous as written: `&&` binds
tighter than `||`, so the parsing is:

```
name == ".git" || name == ".helix" || (strings.HasPrefix(name, ".") && name != ".")
```

That parses as intended, but a reviewer can easily misread it. Also note
that `.git` and `.helix` are already covered by the `HasPrefix(name, ".")`
clause — both checks are redundant.

**Fix:**
```go
if path != ws.RepoRoot && strings.HasPrefix(name, ".") {
    return fs.SkipDir
}
```

### WR-05: `applyValidationVerdicts` `break` after one matching edge can miss refutations

**File:** `internal/kernel/symbols/blast_radius_strangler.go:209-226`
**Issue:**
```go
for i := range impacts {
    for _, e := range impacts[i].Evidence.Edges {
        v, ok := verdictByEdge[e]
        if !ok {
            continue
        }
        if v.LSPConfirmed {
            impacts[i].Confidence = 1.00
        } else {
            impacts[i].Confidence = 0.20
            impacts[i].Refuted = true
        }
        // One verdict per impact is enough to set the verdict. Continue
        // to the next impact rather than letting later edges in the same
        // impact overwrite the verdict.
        break
    }
}
```

The `break` after the first matched edge means: if an impact has two evidence
edges, edge A is confirmed (1.00), edge B is refuted (0.20+Refuted=true), and
the iteration order surfaces A first, the impact ships with Confidence=1.00
and Refuted=false — even though one of its supporting edges has been
contradicted by LSP. Conservatively the contract should be: a single
refutation taints the impact (drop to 0.20, set Refuted). Today the order
of evidence edges decides the verdict, which is non-deterministic in Go map
iteration if the verdict map is consulted in any non-trivial way (here it
isn't, but the underlying impact.Evidence.Edges slice order is whatever
ExpandFrom returned).

The test `TestApplyValidationVerdicts_NoMutation` exercises a single-edge
impact and does not catch this asymmetry.

**Fix:** Do NOT `break` on first match. Instead, accumulate `anyRefuted ||
allConfirmed` semantics:

```go
for i := range impacts {
    var sawConfirmed, sawRefuted bool
    for _, e := range impacts[i].Evidence.Edges {
        v, ok := verdictByEdge[e]
        if !ok { continue }
        if v.LSPConfirmed { sawConfirmed = true } else { sawRefuted = true }
    }
    switch {
    case sawRefuted:
        impacts[i].Confidence = 0.20
        impacts[i].Refuted = true
    case sawConfirmed:
        impacts[i].Confidence = 1.00
    }
}
```

Alternatively, document in the SPEC that "first matching edge wins" if that
is the genuine semantic — but that needs to be a deliberate choice, not an
artifact of `break`.

### WR-06: `semSchedulerAdapter.IsQuiescent` releases mutex before invoking method

**File:** `internal/daemon/semantic_wiring.go:388-399`
**Issue:**
```go
func (a *semSchedulerAdapter) IsQuiescent(repoID string) bool {
    if a == nil || a.rb == nil {
        return true
    }
    a.rb.mu.Lock()
    s, ok := a.rb.subs[repoID]
    a.rb.mu.Unlock()
    if !ok || s == nil {
        return true
    }
    return s.IsQuiescent()
}
```

The mutex protects `a.rb.subs` (the map). After Unlock, `s.IsQuiescent()` is
invoked on a now-unlocked snapshot. If a concurrent goroutine calls
`delete(a.rb.subs, repoID)` and the `RankScheduler` releases its underlying
resources at delete time, this becomes a use-after-free. The current
`rankBundle` implementation appears not to delete subs at runtime, so this
is theoretical, but the pattern is fragile and inconsistent with how
`compactor` is accessed in `OnFlush` (line 502-514) which has the same
release-before-call pattern.

**Fix:** Either hold the mutex through `IsQuiescent`, or document that
`subs` entries are never released for the lifetime of the bundle. The latter
needs to be an invariant pinned by a test.

### WR-07: `factsFromExtracted` masks high bit silently — collisions go undetected

**File:** `internal/daemon/semantic_wiring.go:1166-1196`
**Issue:**
```go
single.Symbols[j].SymbolID &= 0x7FFFFFFFFFFFFFFF
...
single.Symbols[j].OwnerSymbolID &= 0x7FFFFFFFFFFFFFFF
single.Symbols[j].ParentScopeID &= 0x7FFFFFFFFFFFFFFF
```

The high-bit mask is applied silently because "the duckdb-go driver rejects
uint64 values with the high bit set". Two independent SymbolIDs with their
high bits flipped will collapse to the same low-63 value and silently
overwrite each other in the snapshot facts. The Phase 59 EXTRACT-02 SymbolID
allocator may or may not guarantee high-bit-zero — the comment doesn't say.

If the allocator uses, say, FNV-64 or a similar hash, collisions are real.
There's no detection, no logging, no metric, no test that two distinct
upstream IDs survive the mask without colliding.

**Fix:** Either (a) assert `Symbols[j].SymbolID & 0x8000000000000000 == 0`
and crash on violation (with telemetry), (b) shift to int64-typed columns
in DuckDB and remove the mask, or (c) add a test that round-trips a corpus
of generated SymbolIDs through this path and asserts uniqueness preserved.

---

## Info

### IN-01: `formatLocations`/`itoa` — `strconv` already imported transitively

**File:** `internal/kernel/symbols/blast.go:166-189`
**Issue:** A hand-rolled `itoa` exists "without importing strconv". The
package's siblings already pull `strconv` indirectly (via `fmt` formatters);
adding the import is one-line and removes 24 lines of bug-prone manual
conversion.

**Fix:** `import "strconv"`, replace `itoa(int(loc.Range.Start.Line))` with
`strconv.Itoa(int(loc.Range.Start.Line))`.

### IN-02: `formatBlastRadius` `Sprintf("\nCallers:\n")` — no format verbs

**File:** `internal/kernel/symbols/tools.go:704, 710`
**Issue:**
```go
sb.WriteString(fmt.Sprintf("\nCallers:\n"))
```
`fmt.Sprintf` with no verbs is a `go vet` finding (`S1039`) and a needless
allocation. Same for `\nImplementations`.

**Fix:** `sb.WriteString("\nCallers:\n")`.

### IN-03: `cap` parameter shadows `builtin cap`

**File:** `internal/kernel/symbols/blast_radius_strangler.go:232`
**Issue:**
```go
func capConfidences(br *BlastRadius, cap float64) {
```
`cap` is a Go builtin. Shadowing it is legal but a code-smell that confuses
readers and tooling. The package also calls `make([]integ.Impact, 0, cap)`-
style elsewhere; a future edit inside `capConfidences` could `make([]int, 0,
cap)` thinking it gets the builtin and silently pass `0.6` as the slice
capacity (compile error today, semantic accident with int-typed arg).

**Fix:** Rename to `ceiling` or `maxConfidence`.

### IN-04: `integSemanticLookup.Status` — `LastErrorReason` always empty

**File:** `internal/daemon/semantic_wiring.go:794`
**Issue:** Hard-coded `""`. The doc comment at lines 745-753 admits the
65-07 follow-up populates it; the SPEC §24.5 contract documents the closed-
enum mapping. Wire-side coverage today survives because
`ComputeSemanticIndexBlock`'s err-return arm (`tools.go:206-219`) maps the
Status err to `last_error="index_error"`. That keeps the contract honest for
StatusError, but `state==StatusReady` plus a transient failure can never
surface a `LastErrorReason` other than empty until 65-07 lands. Acceptable
as a known TODO; track in 65-07.

### IN-05: kernel/symbols/skill_adapter.go documents one tool but names skill "symbols"

**File:** `internal/kernel/symbols/skill_adapter.go:24-51`
**Issue:** The `SymbolsSkill` struct ships with a single `analyze_blast_radius`
ToolDef. The pre-existing `SymbolRetrievalSkill` enumerates the other 8 tools
catalog-only. The naming risks confusion: a future contributor scanning
`skill.Get("symbols")` will reasonably expect the full nine-tool inventory,
not the strangler-fig sub-surface. The `Description()` says "Symbol analysis
tools (analyze_blast_radius)" but the skill name is the bare "symbols".

**Fix:** Rename to "symbols-blast-radius" or "symbols-strangler" so the skill
identifier reflects its actual single-tool scope. Alternative: have this
adapter additionally enumerate the eight catalog-only ToolDefs from
SymbolRetrievalSkill and merge the two skills (Description: "Strangler-fig
+ catalog mirror"); but that complicates the daemon's name-collision
last-writer-wins logic at `daemon.go:584-588` which currently lists "symbols"
in the skip-set.

---

## Doctrine items verified correct

For completeness, the following items were inspected and found compliant:

- `integ.ChooseSource` priority ladder (config off → tree_sitter; cfg on +
  unavailable → fallback+index_disabled; cfg on + err → fallback+classified;
  cfg on + ok → semantic). Tests at `source_select_test.go:53-181` exhaustively
  cover the matrix including wrapped sentinels.
- `integ.ClassifyLookupErr` is `errors.Is`-only; never inspects raw error
  text. Test `TestEnvelope_NoRawErrorText` and
  `TestChooseSource_NoRawErrText` both pin this.
- `MarshalEnvelope` reserves `source`/`fallback_reason`/`graph_version`/
  `freshness` and silently drops payload keys that collide. Closed-enum
  surface tested in `envelope_test.go`.
- The 0.6 confidence cap (`fallbackConfidenceCap`) is applied uniformly on
  all non-semantic blast-radius paths in `tools.go:638-650, 664, 679`.
- The lint analyzer's slash-boundary check correctly rejects the
  `internal/semantic/integ_evil` lookalike (`analyzer.go:65-69` + the
  `TestAnalyzer_RejectsKernelImportingSemanticIntegLookalike` fixture).
- `applyValidationVerdicts` mutates a copy; the original Pass-1 slice is
  preserved (modulo WR-05's `break` ambiguity). `TestApplyValidationVerdicts_
  NoMutation` pins copy-before-mutate.
- The two repomap goldens (`index_disabled_repo_map.txt`,
  `index_disabled_context.txt`) are byte-identical, locking the INTEG-01
  no-engine-drift contract.

---

_Reviewed: 2026-05-08_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_

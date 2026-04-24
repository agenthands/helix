# Phase 47: bug-rust-analyzer-rename - Research

**Researched:** 2026-04-24
**Domain:** LSP rename quirks / rust-analyzer adapter / tool-level fallback
**Confidence:** HIGH on code structure (read real files); MEDIUM on RCA trigger hypothesis; MEDIUM on upstream issue pinning (candidates exist, exact match unconfirmed).

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-01 Fix strategy — Hybrid:** Attempt real fix first (detect/wait for rust-analyzer indexing readiness before `textDocument/rename`), then fall back to a workaround if the native call still returns "No references found at position".
- **D-02 RCA required:** Reproduce in a controlled harness (or test-driver trace), capture LSP wire traffic for the failing `rename` vs. succeeding `hover`/`references` at the same position, document the trigger. Satisfies Phase 47 Success Criterion #4.
- **D-03 Fallback mechanism:** Client-side rename via `textDocument/references` — collect response ranges, apply text edits through existing `edit` layer.
- **D-04 Wiring:** New optional `RenameOverride` hook on the `QuirkAdapter` interface; `internal/kernel/edit/rename.go` checks for the override after `prepareRename` and delegates when present.
- **D-05 Semantic accuracy:** Limits of client-side rename (no cross-crate trait-impl discovery, macro-expansion corners) are documented, not eliminated. QuirkAdapter doc comment + `USAGE.md` Troubleshooting both cite `BUG-DEFER-02`.
- **D-06 Failure UX:** When both native rename and workaround fail, `rename_symbol` returns a structured error (`internal/errors` a.k.a. `serr`) pointing agents at `fuzzy_edit`, `replace_symbol_body`, or `search_in_files`. No partial/degraded success — half-renames are refused.
- **D-07 Strategy tag:** Successful rename results carry a `strategy` tag (`"lsp-native"` | `"rust-client-side"`) mirroring `fuzzy.Match`. Bounded-label metric counter emitted by strategy.
- **D-08 Test:** `TestEdit_RustFixture/rename` at `test/integration/rust_test.go:120` is unskipped. Add one new assertion: result exposes `strategy` as `"lsp-native"` or `"rust-client-side"`.
- **D-09 Regression coverage:** Existing `TestSymbols_RustFixture` cases suffice — no extra before/after-rename harness.
- **D-10 Ship gate:** Green default `go test ./...` on developer machine with `rust-analyzer` installed. Hosted-CI runner availability tracked separately (TOOL-01 / Phase 50).

### Claude's Discretion

- The precise readiness signal used for the native-fix attempt (`$/progress` indexing-end vs. polling `rust-analyzer/analyzerStatus` vs. re-issuing `prepareRename`) — guided by RCA.
- Exact `serr` error code name and message wording for D-06.
- Shape of the strategy metric counter (label set, name) under the current telemetry layer.

### Deferred Ideas (OUT OF SCOPE)

- Submitting an upstream rust-analyzer patch — owned by `BUG-DEFER-02`.
- Full cross-crate / macro-expansion semantic parity with native rust-analyzer rename.
- Generalizing `RenameOverride` to other LSes preemptively.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| BUG-02 | `rename_symbol` succeeds against a Rust symbol in a temp workspace via rust-analyzer (or a documented, tool-level workaround is shipped if the upstream bug cannot be closed in-milestone) | Hybrid fix: indexing-readiness gate before `textDocument/rename` (native path) + `RenameOverride` delegating to `FindReferences`-driven client-side rename (fallback), with strategy tag in result and `serr` error when both fail. Documented in `USAGE.md` Troubleshooting + `RustAnalyzerAdapter` doc comment. |
</phase_requirements>

## Project Constraints (from CLAUDE.md)

- Go-only; single binary; no Python at runtime (Python is legacy reference only).
- `go vet ./...` and `go test ./...` MUST be run before completing any Go task.
- Kernel Layer 1 lives in `internal/kernel/`; LS quirks live in `internal/kernel/lspool/quirks.go` behind the `QuirkAdapter` interface.
- Structured errors via `internal/errors` (imported as `serr`) — never plain `fmt.Errorf` at tool boundary.
- Tool-level work must go through a GSD workflow; this phase is already in execution via `/gsd-plan-phase`.

## Summary

Phase 47 ships a hybrid fix for `rename_symbol` failing on Rust symbols in temp workspaces under rust-analyzer 1.90. The failure is deterministic at `test/integration/rust_test.go:120` (currently `t.Skip`ed): rename `helper` → `renamed_helper` at line 4 col 4 in `src/main.rs` returns `"No references found at position"` for `textDocument/rename`, yet `hover`, `references`, and `workspace/symbol` at the same position succeed.

The test harness (`test/integration/harness.go:210 WaitForLS`) already gates on a non-empty `search_symbols` response before running any subtest, so the trigger is **not** simple indexing lag in the normal sense — rust-analyzer's rename code path has an additional readiness check (or an altogether different quirk) that isn't satisfied by `workspace/symbol` succeeding. The strongest hypothesis from current-source research: rust-analyzer's rename path goes through `cargo metadata` / crate-graph resolution that's resolved on a different background job than `workspace/symbol` — the `experimental/serverStatus` notification with `quiescent=true` is the canonical "truly idle" signal.

**Primary recommendation:** Implement the hybrid as three thin tasks: (1) RCA + `experimental/serverStatus`-aware readiness gate in the native path; (2) add `RenameOverride` optional method to `QuirkAdapter` and a `RustAnalyzerAdapter.RenameOverride` that wraps `symbols.FindReferences` + `applyTextEdits` with per-range new-name substitution; (3) thread `strategy` through `RenameResult` and add the error/USAGE.md surfaces. All three ride the existing `internal/kernel/edit/rename.go` call-site.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Rename orchestration (prepareRename → rename → apply) | Kernel / edit (`internal/kernel/edit/rename.go`) | — | Single existing call-site; adding override check here keeps the branch local. |
| LS readiness detection for rename (serverStatus, progress, retry) | Kernel / lspool (quirks + conn) | jsonrpc Conn | Notifications arrive on the JSON-RPC dispatcher; `QuirkAdapter.NotificationHandlers` already exists as the extension point. |
| Client-side rename body (references → text edits) | Kernel / edit + Kernel / symbols | — | Reuse `symbols.FindReferences` for the discovery half; reuse `edit.applyTextEdits` for the mutation half. |
| Strategy/metric surface | Kernel / edit (result struct) + MCP middleware (counter) | Tool handler | `RenameResult.Strategy` flows up to `tools.go:396`; counter lives alongside `serena_tool_calls_total` label vocabulary. |
| Documentation | `USAGE.md` Troubleshooting + adapter doc comment | — | Phase SC #2 explicitly requires both surfaces. |

## Standard Stack

### Core (already in repo — no new dependencies)

| Package | Location | Purpose |
|---------|----------|---------|
| `internal/kernel/lspool` | `quirks.go`, `adapter.go`, `worker.go` | `QuirkAdapter` interface, per-LS adapters, JSON-RPC worker. |
| `internal/kernel/edit` | `rename.go`, `replace.go`, `tools.go` | Rename flow, `FuzzyMatchInfo` strategy-tag precedent, tool registration. |
| `internal/kernel/symbols` | `retrieval.go` | `FindReferences` — the LSP call that DOES succeed at the failing position; pure function over `WorkerLease`. |
| `internal/errors` (alias `serr`) | `errors.go`, `kinds.go` | Structured error with `Kind`, `WithTool`, `WithDetail`. Kinds: `NotFound`, `InvalidArgs`, `NoWorkspace`, `Unsupported`, `Internal`, `CircuitOpen`, `Timeout`. |
| `internal/fuzzy` | `types.go`, `strategies.go` | Strategy-tag pattern to mirror (D-07). |
| `protocol/gen` | `gen/` | LSP types: `Position`, `Range`, `TextEdit`, `WorkspaceEdit`, `ReferenceParams`. |

### No New Dependencies

All work uses packages already in `go.mod`. Do NOT add new LSP libraries, rename engines, or text-diff packages. `applyTextEdits` in `rename.go:119` is the reuse path.

### Version Verification

- `rust-analyzer` target version: `1.90` (as cited in existing `rust_test.go:120` skip message and `USAGE.md:535`). [VERIFIED: repo grep]
- Go SDK MCP: unchanged. No SDK surface change.

## Architecture Patterns

### Pattern 1: Optional-Method QuirkAdapter Hook (type-assertion)

**What:** New hooks are added as *stand-alone interface types* that adapters optionally implement; the call-site uses a type assertion. This is ALREADY the established pattern in `quirks.go:14-19`:

```go
// ArgsModifier is an optional interface that QuirkAdapters can implement
// to inject extra command-line arguments when starting the LS process.
type ArgsModifier interface {
    ExtraArgs(workDir string, args []string) []string
}
```

Used at call-sites as `if mod, ok := adapter.(ArgsModifier); ok { args = mod.ExtraArgs(workDir, args) }`. **Match this pattern exactly for `RenameOverride`.** Do NOT add the method to the base `QuirkAdapter` interface (that would force every existing adapter — `ClangdAdapter`, `GoplsAdapter`, `JdtlsAdapter`, `VueAdapter`, etc. — to stub it).

```go
// RenameOverride is an optional interface that QuirkAdapters can implement
// to bypass the native textDocument/rename path. Called by edit.RenameSymbol
// after prepareRename when the native path fails (or preemptively, per adapter
// discretion). The override is responsible for producing the WorkspaceEdit
// equivalent (files + TextEdits) via whatever LSP calls are known-good for
// the target LS (for rust-analyzer, that is textDocument/references).
//
// Returns (result, nil) on success. Returns (nil, serr.*) on failure; the
// caller (rename.go) surfaces the error verbatim to the MCP tool layer.
type RenameOverride interface {
    RenameOverride(
        ctx context.Context,
        lease *lspool.WorkerLease,
        uri string,
        line, col int,
        newName string,
    ) (*edit.RenameResult, error)
}
```

**Import-cycle note:** `lspool` cannot import `edit` (edit already imports lspool). The clean fix is to define `RenameOverride` and its result type in a neutral package — either move the `RenameResult` struct into `internal/kernel/edit` (already there) and define the interface in `edit/` with a registry lookup, or define a lightweight `RenameOverrideResult` in `lspool` that `edit` converts. The cheapest option: define the interface in `edit/` and have `edit.RenameSymbol` do `if o, ok := adapter.(edit.RenameOverrider); ok { ... }`. The adapter type (`*RustAnalyzerAdapter`) still lives in `lspool`; Go allows cross-package type assertions against interfaces declared elsewhere. The planner should pick one of these and stick with it.

### Pattern 2: Strategy-Tag in Result (mirror fuzzy)

**What:** `internal/fuzzy/types.go:10-28` declares `Strategy` as a typed string with documented constants that are part of the public contract (shown to agents in tool responses). **Mirror this exactly.**

```go
// RenameStrategy enumerates the path the rename succeeded through.
// String values are part of the public contract: they appear in
// rename_symbol tool responses AND in the serena_rename_strategy_total
// metric label set. MUST NOT change without a coordinated update.
type RenameStrategy string

const (
    StrategyLSPNative       RenameStrategy = "lsp-native"
    StrategyRustClientSide  RenameStrategy = "rust-client-side"
)
```

`RenameResult` gains one field:

```go
type RenameResult struct {
    FilesChanged int
    EditsApplied int
    Files        []string
    Strategy     RenameStrategy // NEW — D-07
}
```

Tool-layer rendering (mirror `tools.go:266`):

```go
text := fmt.Sprintf("Renamed to %q: %d files changed, %d edits applied\nFiles: %s\nstrategy: %s",
    args.NewName, result.FilesChanged, result.EditsApplied, strings.Join(result.Files, ", "), result.Strategy)
```

### Pattern 3: PostInitialize-driven LS warmup + NotificationHandlers for runtime signals

`RustAnalyzerAdapter.PostInitialize` already calls `didOpenAllFilesRecursive` (`quirks.go:121`). The `NotificationHandlers()` method returns `map[string]func(json.RawMessage)`, currently nil for rust-analyzer. **This is where `experimental/serverStatus` handling lands** — register a handler that updates a `sync/atomic`-guarded readiness flag on the adapter.

### Anti-Patterns to Avoid

- **Opening all files recursively on every rename call.** That already happens once at `PostInitialize`. The failure is post-warmup.
- **Sleeping for a fixed duration.** Deterministic readiness signal or bounded retry only.
- **Retrying `textDocument/rename` forever.** If rust-analyzer reports `quiescent=true` and rename still returns "No references found at position", fall back to the override — don't loop.
- **Applying partial edits on override failure.** D-06 explicitly forbids half-renames.
- **Adding the override method to the base `QuirkAdapter` interface.** Breaks 12 existing adapter types; use optional-interface type assertion.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Finding all usages of the symbol | Custom text search / tree-sitter walker | `symbols.FindReferences(ctx, lease, uri, line, col, true)` | Already exists at `internal/kernel/symbols/retrieval.go:43`; LSP-accurate; handles cross-file ranges rust-analyzer knows about. |
| Applying edits to files | New write path | `edit.applyTextEdits(uri, edits)` at `rename.go:119` | Reverse-ordered atomic write already correct for multi-edit-per-file case. |
| Structured error construction | `fmt.Errorf` + string matching | `serr.New(kind, msg).WithTool("rename_symbol").WithDetail(...)` | Tool-level convention across the entire codebase. |
| Strategy metric counter | New bespoke counter shape | Label-extend existing `serena_tool_calls_total` vocabulary (`middleware.go:48-78`) OR register a sibling `serena_rename_strategy_total{strategy="lsp-native\|rust-client-side"}` via the same OTel meter. | Closed-enum discipline is already there; adding a bounded-label counter fits. |
| URI handling | New `path→uri` helpers | `pathToURI` (`rename.go:156`) and `uriToPath` (same file) | Already consistent across `edit/`. |

## Runtime State Inventory

> Not applicable — this is a behavior-fix phase, not a rename/refactor phase. No stored data, live service config, OS-registered state, secrets, or build artifacts are affected by the fix. The RCA may alter rust-analyzer's in-memory indexing behavior transiently but nothing persists.

## Common Pitfalls

### Pitfall 1: Import cycle between `lspool` and `edit`

**What goes wrong:** Naive approach puts the `RenameOverride` interface in `lspool` referencing `edit.RenameResult` → import cycle, since `edit` already imports `lspool`.
**Why it happens:** `RenameResult` is the natural return type; it lives in `edit`.
**How to avoid:** Declare the `RenameOverride` interface in `internal/kernel/edit` (package where the call-site lives) with the `*edit.RenameResult` return type. Adapters in `lspool` implement it structurally (Go does not require implementing `import`). Call-site: `if o, ok := any(adapter).(edit.RenameOverrider); ok { ... }`. Type-assertion against an interface from a different package is legal and idiomatic.
**Warning signs:** `go build` fails with "import cycle" at compile time.

### Pitfall 2: `FindReferences` with `includeDeclaration=false` misses the declaration site

**What goes wrong:** Client-side rename misses the `fn helper()` declaration at line 4 col 4 — only the caller at `using_helper` and `main` get renamed — file left syntactically broken.
**Why it happens:** LSP's `ReferenceContext.IncludeDeclaration` defaults to false; callers must opt in.
**How to avoid:** Pass `includeDecl=true` to `symbols.FindReferences`. Verified at `retrieval.go:46` — parameter already plumbed through.
**Warning signs:** The integration test passes by accident (file compiles if helper is still named `helper` at declaration), but a second rename call from the renamed identifier fails because the declaration and callsites disagree.

### Pitfall 3: Reference ranges span the identifier — don't overwrite whole-range with `newName` only when identifier length matches

**What goes wrong:** `textDocument/references` ranges cover the identifier span. Client-side rename must construct a `TextEdit{Range: refRange, NewText: newName}` per reference — trivially correct as long as refRange exactly spans the identifier. For rust-analyzer, it does. For other LSes that might return wider ranges (e.g., method-receiver ranges), this would corrupt the file.
**Why it happens:** Assumption baked into the fallback.
**How to avoid:** Scope the fallback to `RustAnalyzerAdapter.RenameOverride` only. Document the narrow contract ("assumes references-range == identifier-range") in the `RenameOverrider` interface doc. Do NOT generalize.
**Warning signs:** Test on trait methods, re-exports, or macros produces broken source; these are called out in D-05 as documented limits, not fixed.

### Pitfall 4: `experimental/serverStatus` is opt-in via client capabilities

**What goes wrong:** rust-analyzer will not emit the notification unless the client advertises `serverStatusNotification: true` in `ClientCapabilities.experimental`.
**Why it happens:** It's an rust-analyzer LSP extension, not core LSP.
**How to avoid:** Set `ClientCapabilities.Experimental = { "serverStatusNotification": true }` in `RustAnalyzerAdapter.InitOptions` — but be careful: `InitOptions` goes to `initializationOptions`, not `capabilities`. Need to confirm where capabilities are assembled during the LS handshake. If that path is not open to quirk adapters today, a minimal extension to `LSAdapter` or the init handshake is required. (Check `internal/kernel/lspool/adapter.go` for the initialize call.)
**Warning signs:** No `experimental/serverStatus` notifications ever arrive; handler never fires. Silent no-op → fallback always runs.

### Pitfall 5: `WaitForLS` passes ≠ rename is ready

**What goes wrong:** `test/integration/harness.go:210 WaitForLS` already polls `search_symbols` until non-empty — and the subtest STILL hits the rename failure. The rename code path has a different readiness gate than `workspace/symbol`.
**Why it happens:** rust-analyzer dispatches requests to different analysis layers; `workspace/symbol` can succeed before the crate graph / rename-index is fully populated.
**How to avoid:** Native-path readiness gate must be a rename-relevant signal (`experimental/serverStatus.quiescent=true` OR a bounded retry of `prepareRename` OR `rust-analyzer/analyzerStatus` poll). Don't trust `workspace/symbol` for rename-readiness.
**Warning signs:** Test flakes locally but passes in CI (or vice versa). The symptom is the same "No references found at position".

## Code Examples

### Current `rename.go` skeleton (lines 26-62) — what the override check slots into

```go
// After prepareRename (rename.go:41-51), before constructing RenameParams:
if o, ok := any(adapter).(RenameOverrider); ok {
    // Native attempt first (D-01 hybrid), then override on failure:
    nativeResult, nativeErr := tryNativeRename(ctx, lease, uri, pos, newName)
    if nativeErr == nil {
        nativeResult.Strategy = StrategyLSPNative
        return nativeResult, nil
    }
    // Fall through to override.
    overrideResult, overrideErr := o.RenameOverride(ctx, lease, uri, line, col, newName)
    if overrideErr != nil {
        return nil, serr.New(serr.Unsupported,
            "rename cannot proceed for this symbol; use fuzzy_edit, replace_symbol_body, or search_in_files for manual rename").
            WithTool("rename_symbol").WithDetail(overrideErr.Error())
    }
    overrideResult.Strategy = StrategyRustClientSide
    return overrideResult, nil
}
// Default path unchanged (other LSes).
```

### `RustAnalyzerAdapter.RenameOverride` — the client-side-rename body

```go
func (r *RustAnalyzerAdapter) RenameOverride(
    ctx context.Context,
    lease *lspool.WorkerLease,
    uri string,
    line, col int,
    newName string,
) (*edit.RenameResult, error) {
    // Gather references (includes declaration) — this call is known to succeed
    // at the same position where textDocument/rename fails (phase RCA).
    locs, err := symbols.FindReferences(ctx, lease, uri, line, col, true)
    if err != nil {
        return nil, serr.Wrap(serr.Internal, "rust-client-side rename: find references", err)
    }
    if len(locs) == 0 {
        return nil, serr.New(serr.NotFound, "rust-client-side rename: no references at position")
    }
    // Group ranges by file; build TextEdits where NewText == newName.
    byFile := make(map[string][]gen.TextEdit)
    for _, l := range locs {
        byFile[l.URI] = append(byFile[l.URI], gen.TextEdit{Range: l.Range, NewText: newName})
    }
    res := &edit.RenameResult{}
    for fileURI, edits := range byFile {
        if err := edit.ApplyTextEdits(fileURI, edits); err != nil { // export applyTextEdits for reuse
            return nil, serr.Wrap(serr.Internal, "rust-client-side rename: apply edits", err).WithDetail(fileURI)
        }
        res.FilesChanged++
        res.EditsApplied += len(edits)
        res.Files = append(res.Files, fileURI)
    }
    sort.Strings(res.Files)
    return res, nil
}
```

**Note:** `applyTextEdits` at `rename.go:119` is currently lowercase (package-private). Either (a) export it as `ApplyTextEdits`, or (b) keep the `RenameOverride` body inside `internal/kernel/edit` in a rust-specific file and have `RustAnalyzerAdapter.RenameOverride` delegate to that exported function. Option (b) keeps `lspool` free of edit-file-IO concerns.

### Error message for D-06 (suggested, verbatim)

```
unsupported: rename_symbol cannot proceed for this Rust symbol (rust-analyzer rename path unavailable in this workspace state); use fuzzy_edit, replace_symbol_body, or search_in_files for a manual rename
```

Use `serr.New(serr.Unsupported, "...")`. `Unsupported` signals the specific path failed; the detail line carries the upstream LSP error verbatim. Do NOT use `Internal` — that bucket is already noisy in the telemetry outcome classifier.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Trust `workspace/symbol` response as proxy for full LS readiness (`WaitForLS`) | Use `experimental/serverStatus` with `quiescent=true` to detect full-indexing-complete on rust-analyzer | v1.90+ (rust-analyzer extension stable) | Eliminates the race between `workspace/symbol`-ready and `rename`-ready. |
| Hardcoded `textDocument/didOpen` on all files at PostInitialize (current adapter) | Still needed, plus runtime readiness signal for rename specifically | No change in PostInitialize | didOpen solves the first class of quirks; rename needs the second. |

**Deprecated/outdated:** The `USAGE.md:537` workaround advice "Use `replace_symbol_body` instead" is accurate but will be superseded by Phase 47. The Troubleshooting entry should be updated in-place to describe the hybrid: "rename_symbol now works against Rust; if the result reports `strategy: rust-client-side`, be aware of the documented semantic limits (cross-crate, macro-expansion) — see `BUG-DEFER-02`."

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | rust-analyzer 1.90's rename path has a distinct readiness gate from `workspace/symbol` | RCA (Pitfall 5) | [ASSUMED] — strongest hypothesis given `WaitForLS` already gates on symbols. If wrong, the native-path readiness signal choice needs revisiting during implementation. |
| A2 | `experimental/serverStatus.quiescent=true` correlates with rename-ready | Code Examples / Pitfall 4 | [CITED: rust-analyzer LSP extensions docs] — documented as "no pending background work", but not empirically proven to include the rename index specifically. If wrong, fallback still ships and fires; reduces the proportion of `lsp-native` successes. |
| A3 | `textDocument/references` ranges on rust-analyzer cover exactly the identifier span | Pitfall 3 + Code Examples | [ASSUMED] based on LSP conventions and existing `find_references` test behavior at `rust_test.go:42-53`. If wrong, client-side rename corrupts source. Mitigate by scoping strictly to `RustAnalyzerAdapter`. |
| A4 | Client capability `serverStatusNotification: true` can be threaded through the adapter/initialize path | Pitfall 4 | [ASSUMED] — existing `InitOptions` goes to `initializationOptions`, not `capabilities`. May require a small plumbing change to `LSAdapter.initialize`. |
| A5 | No existing upstream rust-analyzer GitHub issue exactly matches "rename fails in fresh temp workspaces while hover/references succeed at same position" | Sources / Upstream Issue | [ASSUMED] — issues #6560 (local variable panic), #15837 (file-indexed?), #10888 (workspace ready notification), and the `rename.rs` source are close but not exact matches. Worth a targeted search during RCA to either pin one OR capture ours as a new one for `BUG-DEFER-02`. |
| A6 | Strategy counter can extend existing OTel meter pattern (`serena_tool_calls_total`) | Don't Hand-Roll / Claude's Discretion | [ASSUMED] — plausible given existing closed-enum counter discipline (`middleware.go:48`). Exact meter wiring needs a quick read of the metric definition file. |

## Open Questions

1. **Is the trigger cargo-metadata resolution vs. indexing?**
   - What we know: `workspace/symbol` succeeds (pre-rename readiness gate), yet rename fails.
   - What's unclear: whether `cargo metadata` on the temp workspace completes, and whether rust-analyzer's rename handler has a hard dependency on the project graph vs. just symbol tables.
   - Recommendation: RCA task captures the wire trace and any rust-analyzer stderr/progress; check whether `experimental/serverStatus.health` reports `warning` when rename fails.

2. **Which readiness signal wins on cost/reliability — `experimental/serverStatus` notification vs. bounded `prepareRename` retry?**
   - What we know: notification is event-driven, but requires capability plumbing; retry is simple but wastes 1-2 round-trips.
   - What's unclear: which produces fewer flakes in the Nyquist harness.
   - Recommendation: plan for notification-first, retry-fallback. If `serverStatus` arrives within the test's 45s `LSTimeout`, use it; otherwise retry `prepareRename` up to N times with small backoff, then trigger the override.

3. **Does an upstream rust-analyzer issue exactly match?**
   - What we know: Several candidate issues exist (#6560, #10888, #15837, #5829); none is an exact symptom match from the surface.
   - What's unclear: whether one exists under a different title.
   - Recommendation: RCA task does a targeted repo search on rust-analyzer with the exact error string; if no match, file one for `BUG-DEFER-02`.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| `rust-analyzer` | Integration test `TestEdit_RustFixture/rename` | Must be on developer machine per D-10 ship gate | 1.90+ (repo pinned) | Test already has `requireLS(t, "rust-analyzer")` — skips on absent. No code-path fallback needed. |
| `cargo` / Rust toolchain | rust-analyzer project loading | Implicitly required | — | None — rust-analyzer self-resolves; fixture may not need a `Cargo.toml` present since `didOpenAllFilesRecursive` is used. |
| Go 1.25 | Build & test | Required per `TOOL-01` and `go.mod` | 1.25 | — |

**Missing dependencies with no fallback:** None for this phase — `rust-analyzer` absence cleanly skips via `requireLS`.

**Missing dependencies with fallback:** None.

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go `testing` + `github.com/stretchr/testify/assert` |
| Config file | None (native `go test`); integration tests gated by `//go:build integration` |
| Quick run command | `go test -tags integration -run TestEdit_RustFixture ./test/integration/...` |
| Full suite command | `go test ./... && go vet ./...` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| BUG-02 | `rename_symbol` on Rust `helper` → `renamed_helper` succeeds end-to-end | integration | `go test -tags integration -run 'TestEdit_RustFixture/rename' ./test/integration/...` | ✅ existing at `rust_test.go:119-141` (unskip) |
| BUG-02 (strategy tag) | Result text includes `strategy: lsp-native` OR `strategy: rust-client-side` | integration | same as above; assertion `assert.Regexp(t, "strategy: (lsp-native\|rust-client-side)", text)` | ✅ same file; new assertion line added |
| BUG-02 (regression) | Hover/references/find_implementations unchanged | integration | `go test -tags integration -run TestSymbols_RustFixture ./test/integration/...` | ✅ existing at `rust_test.go:14-92` |
| BUG-02 (override wiring, no LS) | `RenameSymbol` detects the override and routes to it when a fake adapter returns a synthetic `RenameResult` | unit | `go test ./internal/kernel/edit/...` | ❌ Wave 0 — new test `rename_override_test.go` |
| BUG-02 (serr on both-fail) | When both native rename and override return error, `RenameSymbol` returns `serr.Unsupported` with the expected detail | unit | `go test ./internal/kernel/edit/...` | ❌ Wave 0 — same test file |

### Sampling Rate

- **Per task commit:** `go vet ./... && go test ./internal/kernel/edit/... ./internal/kernel/lspool/...` (fast, no LS required)
- **Per wave merge:** `go test -tags integration -run 'TestEdit_RustFixture|TestSymbols_RustFixture' ./test/integration/...` (requires `rust-analyzer`)
- **Phase gate:** `go vet ./... && go test ./... && go test -tags integration ./test/integration/...` green; strategy tag observed in logs at least once per run.

### Wave 0 Gaps

- [ ] `internal/kernel/edit/rename_override_test.go` — new unit test: mock `QuirkAdapter` implementing `RenameOverrider` (table-driven): (a) native success returns `strategy=lsp-native`, (b) native fails + override success returns `strategy=rust-client-side` with correct file/edit counts, (c) native fails + override fails returns `serr.Unsupported`.
- [ ] `internal/kernel/edit/rename.go` refactor: split the current monolithic `RenameSymbol` into `tryNativeRename` + `RenameSymbol` dispatcher so the adapter-check path is testable without a live LS. Preserves the single public entry point.
- [ ] Export `ApplyTextEdits` (or introduce a new exported function in a `rename_override.go` file) so the `RenameOverride` implementation can live in `lspool` OR stay in `edit` — decide during planning.
- [ ] New file `internal/kernel/lspool/quirks_rename.go` (or inline at `quirks.go`): `RustAnalyzerAdapter.RenameOverride` and the `experimental/serverStatus` notification handler registration.
- [ ] `USAGE.md` Troubleshooting: update the existing §"rust-analyzer rename fails in fresh workspaces" entry at line 531-537 to reflect the hybrid fix and document the `rust-client-side` strategy's semantic limits (cross-crate / macro).

*(Note: no new test framework or fixture is needed — `testdata/fixtures/rust/` already contains the reproduction.)*

## Sources

### Primary (HIGH confidence)

- `internal/kernel/edit/rename.go` (full read) — current `RenameSymbol` flow, `applyTextEdits`, `pathToURI`.
- `internal/kernel/lspool/quirks.go` (full read) — `QuirkAdapter` interface, `RustAnalyzerAdapter`, `ArgsModifier` optional-interface precedent.
- `internal/kernel/symbols/retrieval.go` (lines 30-108) — `FindReferences` signature and `includeDecl` plumbing.
- `internal/kernel/edit/replace.go` (lines 1-110) — `FuzzyMatchInfo` strategy-tag precedent (`replace.go:17-20`).
- `internal/fuzzy/types.go` (full read) — `Strategy` typed-string + public-contract discipline.
- `internal/errors/errors.go` + `kinds.go` (full read) — `serr` API surface; `Unsupported` is the right kind for D-06.
- `internal/mcp/middleware.go` (lines 40-115) — closed-enum counter pattern for the strategy metric.
- `test/integration/rust_test.go` (full read) — reproduction target, existing subtest structure.
- `test/integration/harness.go` (lines 180-230) — `WaitForLS` already polls `workspace/symbol` before any subtest runs; proves the failure is post-symbol-readiness.
- `testdata/fixtures/rust/src/main.rs` (full read) — `helper` at line 4 col 4 confirmed.
- `.planning/ROADMAP.md` §Phase 47 — success criteria (4 items).
- `.planning/REQUIREMENTS.md` BUG-02 and BUG-DEFER-02.
- `USAGE.md` lines 418-537 — existing Troubleshooting entry to update.

### Secondary (MEDIUM confidence)

- [rust-analyzer LSP extensions doc](https://android.googlesource.com/toolchain/rustc/+/HEAD/src/tools/rust-analyzer/docs/dev/lsp-extensions.md) — `experimental/serverStatus` with `health` and `quiescent`; `rust-analyzer/analyzerStatus`; `rust-analyzer/reloadWorkspace`.
- [rust-lang/rust-analyzer issue #10888](https://github.com/rust-lang/rust-analyzer/issues/10888) — motivation for a workspace-ready notification; confirms clients currently lack a clean readiness signal beyond `serverStatus`.
- [rust-lang/rust-analyzer issue #15837](https://github.com/rust-lang/rust-analyzer/issues/15837) — "How to determine whether a file is indexed" — community confirmation that this is a real gap.
- [Zed issue #20348](https://github.com/zed-industries/zed/issues/20348) — another editor integrating `experimental/serverStatus` for the same class of problem.

### Tertiary (LOW confidence)

- [rust-lang/rust-analyzer issue #6560](https://github.com/rust-lang/rust-analyzer/issues/6560) — closest textual match for the symptom string "No references found at position" but about a panic on local-variable rename, not the workspace-state failure we see. Useful as a starting point for RCA, not authoritative.
- [LazyVim issue #5966](https://github.com/LazyVim/LazyVim/issues/5966) — end-user report of the same symptom from Neovim; indicates the bug is reproducible outside Serena but does not identify the upstream root cause.

## Metadata

**Confidence breakdown:**

- Standard stack (package reuse, interface pattern, serr): **HIGH** — all read from real files.
- Architecture (hybrid dispatch, override interface, strategy field): **HIGH** — mirrors existing `ArgsModifier` + `FuzzyMatchInfo` patterns verbatim.
- Pitfalls (import cycle, `IncludeDeclaration`, range-span assumption, capability plumbing): **HIGH** on the first three; **MEDIUM** on capability plumbing (may need a small `LSAdapter` extension that's out of the read sample).
- RCA trigger hypothesis (rename has a distinct readiness gate from symbol/workspace): **MEDIUM** — strong inference from `WaitForLS` already passing, but not empirically confirmed. RCA task nails this down.
- Upstream issue pinning: **LOW** — candidate issues listed, exact match not confirmed; may become a net-new upstream filing under `BUG-DEFER-02`.

**Research date:** 2026-04-24
**Valid until:** 2026-05-24 (30 days — rust-analyzer is slow-moving on LSP extensions; recheck if rust-analyzer ships a 1.91+ that changes `experimental/serverStatus`).

## RESEARCH COMPLETE

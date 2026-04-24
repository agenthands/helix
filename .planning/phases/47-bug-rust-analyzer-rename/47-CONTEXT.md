# Phase 47: bug-rust-analyzer-rename - Context

**Gathered:** 2026-04-24
**Status:** Ready for planning

<domain>
## Phase Boundary

`rename_symbol` succeeds on Rust symbols in temp workspaces via `rust-analyzer`, either because the underlying trigger is neutralized (real fix) OR because a documented, deterministic workaround is routed through the `rust-analyzer` `QuirkAdapter`. The existing skipped rename case in `test/integration/rust_test.go` runs green under default `go test ./...` (no `-short=false`, no skip). Hover / references / find_implementations on the same Rust symbol remain unchanged.

Out of scope: submitting or landing a true upstream rust-analyzer patch (deferred as `BUG-DEFER-02`); fixing CI runner rust-analyzer availability (owned by `TOOL-01` / Phase 50); Java, Go, TypeScript rename behavior.

</domain>

<decisions>
## Implementation Decisions

### Fix Strategy
- **D-01:** Ship a **hybrid**: first attempt a real fix (detect and wait for rust-analyzer indexing readiness before issuing `textDocument/rename`), then fall back to a workaround path if the native LSP call still returns `No references found at position`. The phase is green on either path — the hybrid maximizes the chance of keeping semantic LSP rename while guaranteeing a passing test.
- **D-02:** **RCA is required.** A plan must reproduce the failure in a controlled harness (or in a test-driver trace), capture the LSP wire traffic for the failing `textDocument/rename` vs. the succeeding `hover` / `textDocument/references` at the same position, and document the trigger. Satisfies Phase 47 Success Criterion #4 explicitly.

### Workaround Mechanism
- **D-03:** If the native fix is insufficient, the fallback is **client-side rename via `textDocument/references`**: call references on the symbol position, collect the response ranges, and apply the text edits via the existing `edit` layer. Works because references already succeed in the failing scenario.
- **D-04:** The fallback is wired as a new **`RenameOverride` hook on `QuirkAdapter`** (optional method). `internal/kernel/edit/rename.go` checks for the override after `prepareRename` and delegates to it when present. Matches the existing `InitOptions` / `PostInitialize` / `NotificationHandlers` pattern in `internal/kernel/lspool/quirks.go`.
- **D-05:** Semantic-accuracy limits of the client-side rename (no cross-crate trait-impl discovery, macro-expansion corner cases) are **documented, not eliminated**. The QuirkAdapter doc comment and `USAGE.md` Troubleshooting entry both call this out and reference `BUG-DEFER-02` as the path to true semantic rename parity.

### Failure UX
- **D-06:** When both the native rename and the workaround cannot proceed, `rename_symbol` returns a **structured error** (using the `internal/kernel/serr` package) whose message points agents at `fuzzy_edit`, `replace_symbol_body`, or `search_in_files` for manual rename. No partial / degraded success — half-renames are refused.
- **D-07:** Successful rename results carry a **`strategy` tag** (`"lsp-native"` or `"rust-client-side"`), mirroring the `fuzzy.Match` pattern where the strategy used is reported in the result. A bounded-label metric counter (by strategy) is emitted so upstream-fix adoption can be tracked over time — aligns with the Phase 53 observability direction.

### Test + Verification Shape
- **D-08:** The existing `TestEdit_RustFixture` "rename" subtest at `test/integration/rust_test.go:120` is **unskipped**. Add one new assertion: the result exposes `strategy` as either `"lsp-native"` or `"rust-client-side"`. No new semantic-coverage cases (multi-file, trait methods) in this phase — keeps flake risk down.
- **D-09:** Hover / references / `find_implementations` regression coverage is **satisfied by the existing `TestSymbols_RustFixture` cases** — no extra before/after-rename harness.
- **D-10:** The ship gate is **green default `go test ./...` on a developer machine with `rust-analyzer` installed**. Hosted CI runner availability is tracked separately under `TOOL-01` / Phase 50 and must not block Phase 47 merge.

### Claude's Discretion
- The precise readiness signal used for the native-fix attempt (watching `$/progress` indexing-end vs. polling `rust-analyzer/analyzerStatus` vs. re-issuing `prepareRename` until it resolves) is an implementation detail for planning / research to choose, guided by RCA.
- Exact serr error code naming and message wording for D-06.
- The shape of the metric counter (label set, name) under the current `internal/mcp` telemetry layer.

### Folded Todos
_None — no matching backlog todos._

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Roadmap + Requirements
- `.planning/ROADMAP.md` §Phase 47 — success criteria (4 items), BUG-DEFER-02 relationship.
- `.planning/REQUIREMENTS.md` BUG-02 — core requirement; BUG-DEFER-02 scope boundary.

### Existing code (must read — workaround lives here)
- `internal/kernel/edit/rename.go` — current `RenameSymbol` flow; already sends `prepareRename` before `textDocument/rename`. The `RenameOverride` hook check is added here.
- `internal/kernel/lspool/quirks.go:87-123` — `RustAnalyzerAdapter` definition; `PostInitialize` already calls `didOpenAllFilesRecursive(".rs", "rust")`. The new `RenameOverride` method for the `QuirkAdapter` interface is added alongside `InitOptions` / `PostInitialize` / `NotificationHandlers`.
- `test/integration/rust_test.go:96-140` — `TestEdit_RustFixture`; the `t.Skip` at line 120 is removed and a `strategy` assertion is added.
- `test/integration/rust_test.go:14-94` — `TestSymbols_RustFixture`; the regression guardrail (hover / references / find_implementations unchanged) lives here.
- `internal/kernel/serr/` — structured error package; used for D-06.
- `internal/fuzzy/strategies.go` — reference pattern for strategy tagging in a result (match this shape for D-07).

### Documentation surface
- `USAGE.md` Troubleshooting — gets the documented-workaround entry (Phase 47 Success Criterion #2).

### Upstream
- rust-analyzer 1.90 textDocument/rename failure in temp workspaces — upstream issue identification is part of RCA (D-02); no URL yet — researcher should confirm and pin.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `RustAnalyzerAdapter` (`internal/kernel/lspool/quirks.go:87`) — already registered for language `"rust"` (line 453 in the language→adapter map). Extending it with `RenameOverride` is the minimum delta.
- `didOpenAllFilesRecursive` — used in `PostInitialize`; not the failing layer but rules out "file not opened" as the trigger.
- `internal/fuzzy/strategies.go` — the "strategy used is reported in the result" pattern already exists and should be mirrored for rename (D-07).
- `internal/kernel/edit/` text-edit application path — reused by the client-side rename fallback.

### Established Patterns
- **QuirkAdapter interface lives in `lspool/quirks.go`** with per-LS implementations and a language→factory registry at the bottom. New hooks should be added as optional methods so adapters that don't care can stay no-op.
- **Structured errors via `internal/kernel/serr`** — all tool-layer failures use this package (see `rename.go` existing `serr.Wrap(serr.Internal, ...)` calls).
- **Strategy-tagged results** (fuzzy edit package) — set the precedent for D-07 shape.

### Integration Points
- `internal/kernel/edit/rename.go` is the single call-site for rename routing — the fix + fallback branches both land here behind the QuirkAdapter check.
- `internal/kernel/lspool/quirks.go:453` language→adapter map already wires `"rust"` → `RustAnalyzerAdapter`; no registry change needed.
- MCP middleware telemetry (`internal/mcp/middleware.go`) already emits tool-outcome metrics — the strategy counter piggybacks on this pattern.

</code_context>

<specifics>
## Specific Ideas

- Mirror `internal/fuzzy/strategies.go` for the strategy-tag-in-result pattern (D-07).
- The QuirkAdapter extension is an optional method — legacy adapters (ClangdAdapter, etc.) do not need to implement it.
- The failing test gives us a precise reproduction: `helper` → `renamed_helper` at line 4 col 4 in `src/main.rs` of the rust fixture. The RCA plan should capture the wire trace against this exact call.

</specifics>

<deferred>
## Deferred Ideas

- Submitting an upstream rust-analyzer patch to fix the "No references found at position" rename trigger — `BUG-DEFER-02` already owns this.
- Multi-crate / trait-impl / macro-expansion semantic-correctness parity with rust-analyzer's native rename — documented limitation, not pursued here.
- Generalizing `RenameOverride` to other LSes that may hit similar quirks (e.g. forcing client-side rename on clangd). Revisit only if another language exhibits the same symptom.

</deferred>

---

*Phase: 47-bug-rust-analyzer-rename*
*Context gathered: 2026-04-24*

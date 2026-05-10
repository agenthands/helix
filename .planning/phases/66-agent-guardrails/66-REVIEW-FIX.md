---
phase: 66
fixed_at: 2026-05-10T00:00:00Z
review_path: .planning/phases/66-agent-guardrails/66-REVIEW.md
iteration: 1
findings_in_scope: 17
fixed: 17
skipped: 0
status: all_fixed
---

# Phase 66: Code Review Fix Report

**Fixed at:** 2026-05-10
**Source review:** .planning/phases/66-agent-guardrails/66-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 17 (4 critical + 8 warning + 5 info, `--all` flag active)
- Fixed: 17
- Skipped: 0

## Status table

| ID    | Severity | Status                                    | Commit   | Files |
|-------|----------|-------------------------------------------|----------|-------|
| CR-01 | Critical | fixed                                     | 01b229d2 | `internal/guardrails/rules/g002_delete_without_refs.go`, `internal/guardrails/rules/g003_public_api_edit.go` |
| CR-02 | Critical | fixed                                     | e62299c4 | `internal/guardrails/rules/g001_rename_by_grep.go`, `internal/kernel/help/topics.go` |
| CR-03 | Critical | fixed: requires human verification        | 0da1a3e0 | `internal/daemon/daemon.go`, `internal/daemon/guardrail_deps.go` |
| CR-04 | Critical | fixed: requires human verification        | eaa9fd8d | `internal/kernel/symbols/tools.go` |
| WR-01 | Warning  | fixed                                     | bc22afd2 | `internal/skill/semantic/tools_context.go` |
| WR-02 | Warning  | fixed: requires human verification        | 442c0179 | `internal/guardrails/rules/g001_rename_by_grep.go` |
| WR-03 | Warning  | fixed: requires human verification        | 79fd4f8e | `internal/daemon/daemon.go` |
| WR-04 | Warning  | fixed                                     | f6808169 | `internal/guardrails/rules/g001_rename_by_grep.go` |
| WR-05 | Warning  | fixed                                     | deca5ddb | `internal/guardrails/rules/g005_security_sensitive.go` |
| WR-06 | Warning  | fixed                                     | deca5ddb | `internal/guardrails/rules/g005_security_sensitive.go` |
| WR-07 | Warning  | fixed                                     | a68aacc1 | `internal/guardrails/rules/evaluator.go` |
| WR-08 | Warning  | fixed                                     | 44753e53 | `internal/semantic/config.go` |
| IN-01 | Info     | fixed                                     | 83104e36 | `internal/guardrails/receipt_id.go` |
| IN-02 | Info     | fixed                                     | e32a59d5 | `internal/guardrails/store.go` |
| IN-03 | Info     | fixed                                     | deca5ddb | `internal/guardrails/rules/g005_security_sensitive.go` |
| IN-04 | Info     | fixed                                     | cc6f42c4 | `internal/guardrails/rules/g002_delete_without_refs.go` |
| IN-05 | Info     | fixed                                     | 68033e12 | `Makefile`, `internal/kernel/help/docs/guardrails.md`, `internal/kernel/help/docs/dod.md` |

## Fixed Issues

### CR-01: Use serr.ErrUnsupported sentinel in G-002/G-003

**Files:** `internal/guardrails/rules/g002_delete_without_refs.go`, `internal/guardrails/rules/g003_public_api_edit.go`
**Commit:** 01b229d2
**Applied fix:** Imported `serr "github.com/agenthands/helix/internal/errors"`. Changed `errors.Is(err, errors.ErrUnsupported)` to `errors.Is(err, serr.ErrUnsupported)` so the sentinel comparison hits the project's `*serr.Error` type (Kind == Unsupported) rather than the unrelated stdlib `errors.ErrUnsupported`. The conservative-trigger branch in G-002.evaluateG002ReplaceBody and G-003.EvaluateG003 no longer fires on legitimate "unsupported" errors from the integ.SemanticLookup placeholder methods.

### CR-02: Align rule SeeAlso topics to registered help topics

**Files:** `internal/guardrails/rules/g001_rename_by_grep.go`, `internal/kernel/help/topics.go`
**Commit:** e62299c4
**Applied fix:** G-001 now emits `workflow:rename` (which exists and covers the rename DoD including G-001/G-003). For G-003 the emitted `workflow:public-api-edit` topic was registered as an alias mapping to the existing `docs/dod.md` (which carries the public-API-edit DoD), so the agent's remediation hint resolves until a dedicated doc lands. The `TestTopicRegistry_LoadDefaults_All6Present` test still passes because it asserts presence, not exact count.

### CR-03: Thread active workspace key into receipt issuance and middleware

**Files:** `internal/daemon/daemon.go`, `internal/daemon/guardrail_deps.go`
**Commit:** 0da1a3e0
**Applied fix:** Replaced the literal `workspace.WorkspaceKey{}` in the issuance-sink closure (daemon.go:725) with `activeWSKey`, which the daemon's `lazyActivateFn` already updates on workspace activation; closures capture by reference, so the issuance-sink picks up the updated key on every call. Plumbed a `wsKeyFn func() workspace.WorkspaceKey` parameter through `newGuardrailDeps` and stored it on `guardrailDepsImpl`; the evaluator now resolves the live key at evaluation time and writes it to `SessionContext.Workspace`. Pre-LazyInit calls still see the zero key, but those calls are rare (LazyInit middleware runs first in the LIFO chain) and harmless.

**Human-verification ask:** Confirm that `lazyActivateFn` always runs before the first guardrail evaluation in your real client setups (Claude Code / Codex / etc.). If a client invokes a tool before LazyInit completes, the evaluator will see `workspace.WorkspaceKey{}` and the receipt store will collapse those issuances to a single bucket. The 250 ms guardrail timeout fail-open keeps this from blocking, but the semantic of "scoped per workspace" is broken for those edge calls.

**GraphVersion is still 0:** That sub-mitigation (T-66-02) is a phase-66.x deferred item — the wiring requires a graph-version publish/subscribe seam from Phase 62 that is documented as out of scope for this fix.

### CR-04: Resolve SymbolID for read-tool receipts via lookup

**Files:** `internal/kernel/symbols/tools.go`
**Commit:** eaa9fd8d
**Applied fix:** `find_references` now best-effort calls `lookup.SymbolID(ctx, ws, args.Path, lspLine, lspCol)` before issuing the `references_checked` receipt; on success the receipt's `SymbolID` is populated, and STRICT scope validation (validate.go:91-92) accepts the receipt against a non-empty Target.SymbolID. The LSP-only arm of `analyze_blast_radius` (the `SourceTreeSitter` / `SourceFallback` switch arms) does the same. On lookup miss/error/Noop, both call sites fall back to empty SymbolID — the receipt is still issued (file-anchored validations still match) but symbol-anchored consumers will skip it. Plumbed `lookupFn` into `registerFindReferences` for this resolution.

**Human-verification ask:** This is a semantic logic change. Confirm that the `lookup.SymbolID` call in `find_references` does not block the handler in a way that defeats the `find_references` SLO (it returns `serr.ErrUnsupported` for the NoopLookup, which is fast). On the LSP-only arm of `analyze_blast_radius`, a non-Noop lookup may now do work that the cfg-gate steered the request away from — confirm the latency cost is acceptable, or guard the call on `lookup.Available()`.

### WR-01: Use request context in get_semantic_context receipt issuance

**Files:** `internal/skill/semantic/tools_context.go`
**Commit:** bc22afd2
**Applied fix:** Replaced `context.Background()` at line 318 with the handler's `ctx`, which `handleGetSemanticContext` already accepts as a parameter.

### WR-02: G-001 conservative-warn when outline errors

**Files:** `internal/guardrails/rules/g001_rename_by_grep.go`
**Commit:** 442c0179
**Applied fix:** When `OutlineProvider.SymbolsInFile` returns an error AND the identifier-shape gate has fired, G-001 now emits a degraded-mode `Warn` (with `Block` downgraded to `Warn` per D-19 conservative-not-punitive) rather than silently allowing. Empty-outline-no-error still allows (the find string cannot alias anything if there are no declared symbols). The Warn carries a `find_references` remediation hint and a `workflow:rename` SeeAlso pointer.

**Human-verification ask:** This is a semantic change to the degraded path. Confirm that no production OutlineProvider returns errors for legitimate non-source files (e.g. a Markdown file that the provider doesn't know how to outline). If it does, every `fuzzy_edit` against such a file will now Warn — over-triggering. The current production provider is `noopOutlineProvider{}` which returns `(nil, nil)` (not an error), so this code path is dormant in v1.10 until a real adapter lands.

### WR-03: Surface noopOutlineProvider gap with startup warn-log

**Files:** `internal/daemon/daemon.go`
**Commit:** 79fd4f8e
**Applied fix:** Added a startup `Warn` log immediately after `InstallGuardrailMiddleware` to surface the dead-G-001 condition to operators. Replacing the `noopOutlineProvider{}` with a real tree-sitter adapter is documented as a deferred phase-66.x cross-cutting wiring task and is out of scope for a single-finding fix.

**Human-verification ask:** This fix is documentation/observability only — G-001 remains structurally Allow in production. An operator looking at startup logs will see the gap and can choose to set `guardrails.G-001.enforcement: off` to avoid the misleading "G-001 enforced" config. A real adapter is the proper fix and should be a tracked TODO.

### WR-04: Remove dead containsIdentifier helper

**Files:** `internal/guardrails/rules/g001_rename_by_grep.go`
**Commit:** f6808169
**Applied fix:** Removed `containsIdentifier(symName, find string) bool { return symName == find }` and the redundant `|| containsIdentifier(...)` clause. The match loop now reads `if sym.Name == args.Find { matchedSymbol = sym.Name; break }`. Behavior is unchanged because the OR was a tautology.

### WR-05, WR-06, IN-03: G-005 cleanup

**Files:** `internal/guardrails/rules/g005_security_sensitive.go`
**Commit:** deca5ddb
**Applied fix:**
- WR-05: replaced `joinSignals`'s hand-rolled `+=` accumulator with `strings.Join(signals, ", ")`.
- IN-03: replaced the manual `\` / `/` extension-extraction loop with `filepath.Ext(path) + strings.ToLower(...)` + a small switch.
- WR-06: documented the intentional fail-closed behavior — `langFromPath` returns `""` for any extension outside the four shipped catalogs (go/ts/js/py), and `resolveG005Catalog` returns an empty `Catalog{}` for unknown languages so `IsSecuritySensitive` cannot fire. Operators with Java/Rust/C#/Kotlin/Ruby/PHP/Swift repos should treat G-005 as a no-op until later phases ship those catalogs.

### WR-07: Drop redundant class-precheck in findReceiptCovering

**Files:** `internal/guardrails/rules/evaluator.go`
**Commit:** a68aacc1
**Applied fix:** Removed the `if rcpt.Class != required { continue }` early-out in the inner loop. `ValidateReceiptForOperation` already enforces the class match (validate.go:69-72) and is now the single gate.

### WR-08: Document G-004 zero-value coerce-to-default behavior

**Files:** `internal/semantic/config.go`
**Commit:** 44753e53
**Applied fix:** Added a struct-level comment to `G004Config` explaining that 0/negative threshold values fall back to defaults (50, 1, 0.30) and that `EnableRatioTrigger=false` is the proper toggle for disabling the ratio trigger. The behavior itself is preserved — changing the semantics now would silently flip thresholds for any operator who left fields zero.

### IN-01: Use receiptIDTotalLen as ParseReceiptID fast-path

**Files:** `internal/guardrails/receipt_id.go`
**Commit:** 83104e36
**Applied fix:** Added a length precheck `if len(s) != receiptIDTotalLen` that defers to the more specific `ErrInvalidReceiptIDPrefix` when applicable. The constant is now used.

### IN-02: Rename IssueFields.PendingLSP to PendingLSPFiles

**Files:** `internal/guardrails/store.go`
**Commit:** e32a59d5
**Applied fix:** Renamed the field on the carrier struct so it matches the `Receipt.PendingLSPFiles` field name. No external callers populate `IssueFields.PendingLSP` (a grep confirmed it's only set inside the package).

### IN-04: Inline requiredClasses in G-002 delete-file branch

**Files:** `internal/guardrails/rules/g002_delete_without_refs.go`
**Commit:** cc6f42c4
**Applied fix:** Moved the local `requiredClasses` slice into the `else` branch where it is the only consumer; the `if args.HasExportedSymbols` branch already builds its own inline `[]ReceiptClass{ClassImpactChecked}`.

### IN-05: Add sync-docs Makefile target and resync embedded help docs

**Files:** `Makefile`, `internal/kernel/help/docs/guardrails.md`, `internal/kernel/help/docs/dod.md`
**Commit:** 68033e12
**Applied fix:** Added a `sync-docs` Makefile target that copies the root `GUARDRAILS.md` and `DoD.md` into `internal/kernel/help/docs/`, re-prepending the `<!-- Synced from /…; do not edit directly -->` header. Ran the target — the embedded copies had drifted from the root (468 lines of insertions across the two files). Now byte-equal modulo header.

## Skipped Issues

None.

## Verification

Final verification run inside the worktree at `/tmp/sv-66-reviewfix-NG2OKT`:

| Command | Exit code | Notes |
|---------|-----------|-------|
| `go build ./cmd/... ./internal/...` | 0 | Pre-existing swift macro warning; no errors |
| `go test ./internal/guardrails/... ./internal/mcp/... ./internal/kernel/symbols/... ./internal/kernel/edit/... ./internal/kernel/help/... ./internal/skill/semantic/... -count=1` | 0 | All packages pass: guardrails (0.25s), guardrails/catalogs (0.34s), guardrails/rules (0.51s), mcp (8.91s), kernel/symbols (2.21s), kernel/edit (1.02s), kernel/help (2.45s), skill/semantic (6.80s) |

Per-finding verification was Tier 2 (`go vet` + targeted `go test`) for every code-modifying fix. No rollbacks were needed.

---

_Fixed: 2026-05-10_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_

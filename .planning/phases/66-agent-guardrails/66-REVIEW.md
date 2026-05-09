---
phase: 66-agent-guardrails
reviewed: 2026-05-09T00:00:00Z
depth: standard
files_reviewed: 84
files_reviewed_list:
  - internal/config/defaults.go
  - internal/daemon/daemon.go
  - internal/daemon/guardrail_deps.go
  - internal/daemon/imports.go
  - internal/daemon/semantic_wiring.go
  - internal/errors/kinds_test.go
  - internal/errors/kinds.go
  - internal/guardrails/catalogs/embed.go
  - internal/guardrails/catalogs/go.yaml
  - internal/guardrails/catalogs/javascript.yaml
  - internal/guardrails/catalogs/load_test.go
  - internal/guardrails/catalogs/load.go
  - internal/guardrails/catalogs/python.yaml
  - internal/guardrails/catalogs/typescript.yaml
  - internal/guardrails/doc.go
  - internal/guardrails/enforcement_test.go
  - internal/guardrails/enforcement.go
  - internal/guardrails/issue_sink.go
  - internal/guardrails/receipt_id_test.go
  - internal/guardrails/receipt_id.go
  - internal/guardrails/receipt.go
  - internal/guardrails/rules/decision.go
  - internal/guardrails/rules/doc.go
  - internal/guardrails/rules/evaluator_test.go
  - internal/guardrails/rules/evaluator.go
  - internal/guardrails/rules/g001_rename_by_grep_test.go
  - internal/guardrails/rules/g001_rename_by_grep.go
  - internal/guardrails/rules/g002_delete_without_refs_test.go
  - internal/guardrails/rules/g002_delete_without_refs.go
  - internal/guardrails/rules/g003_public_api_edit_test.go
  - internal/guardrails/rules/g003_public_api_edit.go
  - internal/guardrails/rules/g004_large_fuzzy_edit_test.go
  - internal/guardrails/rules/g004_large_fuzzy_edit.go
  - internal/guardrails/rules/g005_security_sensitive_test.go
  - internal/guardrails/rules/g005_security_sensitive.go
  - internal/guardrails/store_test.go
  - internal/guardrails/store.go
  - internal/guardrails/validate_test.go
  - internal/guardrails/validate.go
  - internal/guardrails/visibility_test.go
  - internal/guardrails/visibility.go
  - internal/kernel/diag/tools.go
  - internal/kernel/edit/receipt_issuance_test.go
  - internal/kernel/edit/tools.go
  - internal/kernel/fileops/tools.go
  - internal/kernel/fileops/write.go
  - internal/kernel/health/tools_semantic_test.go
  - internal/kernel/help/docs/dod.md
  - internal/kernel/help/docs/guardrails.md
  - internal/kernel/help/docs/workflow_delete.md
  - internal/kernel/help/docs/workflow_large_edit.md
  - internal/kernel/help/docs/workflow_rename.md
  - internal/kernel/help/docs/workflow_security_sensitive_edit.md
  - internal/kernel/help/embed.go
  - internal/kernel/help/tools.go
  - internal/kernel/help/topics_test.go
  - internal/kernel/help/topics.go
  - internal/kernel/symbols/blast_radius_strangler_test.go
  - internal/kernel/symbols/receipt_issuance_test.go
  - internal/kernel/symbols/tools.go
  - internal/mcp/guardrail_middleware_test.go
  - internal/mcp/guardrail_middleware.go
  - internal/mcp/integ/doc.go
  - internal/mcp/integ/receipt_issuance_smoke_test.go
  - internal/mcp/middleware_test.go
  - internal/mcp/middleware.go
  - internal/mcp/telemetry_middleware_test.go
  - internal/obs/metrics.go
  - internal/profile/profile.go
  - internal/profile/profiles/ci-bot.yaml
  - internal/profile/profiles/claude-code.yaml
  - internal/profile/profiles/codex.yaml
  - internal/profile/profiles/full.yaml
  - internal/profile/profiles/ide-assistant.yaml
  - internal/semantic/config_test.go
  - internal/semantic/config.go
  - internal/semantic/integ/lookup_stub.go
  - internal/semantic/integ/lookup.go
  - internal/semantic/integ/noop.go
  - internal/semantic/integ/source_select_test.go
  - internal/semantic/integ/visibility_test.go
  - internal/semantic/integ/visibility.go
  - internal/skill/guardrails/doc.go
  - internal/skill/guardrails/skill.go
  - internal/skill/repomap/skill.go
  - internal/skill/repomap/strangler_test.go
  - internal/skill/semantic/integration_test.go
  - internal/skill/semantic/tools_context.go
  - test/harness/context_truncation_test.go
findings:
  critical: 4
  warning: 8
  info: 5
  total: 17
status: issues_found
---

# Phase 66: Code Review Report

**Reviewed:** 2026-05-09
**Depth:** standard
**Files Reviewed:** 84
**Status:** issues_found

## Summary

Phase 66 lands a substantial agent-guardrail surface: the receipt store, five rule
predicates (G-001..G-005), the GuardrailMiddleware, typed errors, embedded YAML
catalogs, and per-profile enforcement. The package-level construction is sound
(closed enums, drop-on-unknown metric discipline, fail-open evaluation timeout,
LRU + TTL store). However, several BLOCKER-class defects undercut the runtime
behaviour: an `errors.Is` package-mix bug forces G-002 / G-003 to over-trigger
on every call in production, two of the rule predicates emit `see_also` topic
names that are not registered in the help registry (so the remediation hint is
a 404), and the daemon-side issuance closure plus middleware both pass
`workspace.WorkspaceKey{}` as the workspace identity, defeating the cross-
workspace forgery check (T-66-06) and the graph-version drift mitigation
(T-66-02) at the same time.

The receipt-issuance call sites in read-side tools were instrumented but
populate `SymbolID: ""` for the LSP-only path of `find_references` and
`analyze_blast_radius`, which means downstream STRICT scope validation in
`validateScope` rejects every receipt these tools issue against any non-empty
target SymbolID — making G-001/G-002/G-003 unsatisfiable from the LSP-only
arm of the read-side tools. This is the dominant correctness defect.

## Critical Issues

### CR-01: G-002 / G-003 always conservatively trigger because `errors.Is(err, errors.ErrUnsupported)` mixes packages

**File:** `internal/guardrails/rules/g002_delete_without_refs.go:174`
**File:** `internal/guardrails/rules/g003_public_api_edit.go:39`

**Issue:** Both files import the standard library `"errors"` package and call
`errors.Is(err, errors.ErrUnsupported)`, where `errors.ErrUnsupported` is the
stdlib Go 1.21 sentinel. The errors actually returned by the lookup
(`integ.UnsupportedSemanticLookup`, `integ.NoopLookup`, and the production
`integSemanticLookup.Visibility` / `IsEntrypointReachable` placeholders) are
the **internal/errors `serr.ErrUnsupported`** sentinel — `*serr.Error{Kind:
Unsupported}`. `(*serr.Error).Is` (internal/errors/errors.go:66-70) only
matches when the target is `*serr.Error`; against the stdlib sentinel it
returns false. Result: in production, with `Available()==true` and the
placeholder `Visibility` returning `(VisUnknown, serr.ErrUnsupported)`, the
"unexpected error: conservative trigger" branch runs for every edit, blocking
or warning on every `replace_symbol_body`, `safe_delete_symbol`,
`fuzzy_edit`, and `replace_in_file` call regardless of actual visibility.

**Fix:**
```go
import (
    "errors"

    serr "github.com/agenthands/helix/internal/errors"
    // ...
)
// ...
if err != nil && !errors.Is(err, serr.ErrUnsupported) {
    triggered = true
}
```
The same correction is needed in `g003_public_api_edit.go:39` and
`g002_delete_without_refs.go:174`. Add a regression test that injects a
`serr.ErrUnsupported`-returning Lookup and asserts `triggered == false`.

---

### CR-02: Rule predicates reference unregistered help topics — `see_also` returns 404

**File:** `internal/guardrails/rules/g001_rename_by_grep.go:96`
**File:** `internal/guardrails/rules/g003_public_api_edit.go:95`

**Issue:** Two of the three rules with a `SeeAlso` reference point to topics
that `internal/kernel/help/topics.go:60-77` never registers:

| Rule | Emitted topic | Registered names |
|------|---------------|------------------|
| G-001 | `workflow:rename-by-grep` | `workflow:rename` (only) |
| G-003 | `workflow:public-api-edit` | (no `public-api-edit` topic) |

The help dispatcher (`internal/kernel/help/tools.go:30-39`) returns an error
result `Topic %q not found. Available topics: ...` for any unknown topic, so
the agent's remediation path on a guardrail violation is broken. G-004 and
G-005 are correct (`workflow:large-edit`, `workflow:security-sensitive-edit`).

**Fix:** Either rename the SeeAlso topics to the registered names
(`workflow:rename` and a new `workflow:public-api-edit`) AND add the missing
docs file + topic-registry entry, or align the topics map to match the rule
emissions. Add a unit test that walks every rule's SeeAlso emissions and
asserts each topic is present in `defaultTopics.Names()`:
```go
// internal/guardrails/rules/decision_topics_test.go
func TestSeeAlsoTopicsAreRegistered(t *testing.T) {
    registered := map[string]bool{}
    for _, n := range help.DefaultTopics().Names() {
        registered[n] = true
    }
    decisions := []rules.Decision{
        rules.EvaluateG001(...), rules.EvaluateG003(...), rules.EvaluateG004(...),
        rules.EvaluateG005(...),
    }
    for _, d := range decisions {
        for _, sa := range d.SeeAlso {
            if topic := sa.Args["topic"]; topic != "" {
                assert.True(t, registered[topic], "unregistered topic %q from rule %s", topic, d.Rule)
            }
        }
    }
}
```

---

### CR-03: All receipts issued/validated under empty `WorkspaceKey{}` — workspace forgery check (T-66-06) and graph-drift invalidation (T-66-02) are inert

**File:** `internal/daemon/daemon.go:724-726`
**File:** `internal/daemon/guardrail_deps.go:165-180`

**Issue:** The production receipt issuance closure stores every receipt under
`workspace.WorkspaceKey{}`:
```go
guardrails.SetReceiptIssueSink(func(ctx context.Context, class ..., scope ..., tool string) (..., error) {
    return guardrailStore.Issue(workspace.WorkspaceKey{}, class, scope, ...)
})
```
The middleware-side `SessionContext.Workspace` is also `workspace.WorkspaceKey{}`
(see TODO at `guardrail_deps.go:166-167`). Two consequences:

1. **Cross-workspace receipt forgery (T-66-06)**: the `Receipt.WorkspaceKey
   != currentWS` check in `validate.go:50` and `store.go:292` can never fail
   because both sides are the zero value. As soon as multi-workspace lands
   without changing this code, a receipt issued in workspace A is accepted
   for an edit in workspace B.
2. **Graph-version drift invalidation (T-66-02)** is a no-op because
   `IssueFields{IssuingTool: tool}` omits `GraphVersion`, leaving it at 0,
   while the middleware also passes `GraphVersion: 0`. `rcpt.GraphVersion <
   currentGV` is `0 < 0 = false`, so receipts never expire on graph advance.

The TODO comments acknowledge this but the behaviour is shipping in v1.10
unguarded — both stated mitigations from the threat model are inert.

**Fix:** Thread the active workspace key through the issuance closure (use
`activeWSKey` already in scope at daemon.go) and through the
`SessionContext.Workspace` (a `wsKeyFn func() workspace.WorkspaceKey` was
already plumbed through `semanticBundle`). At minimum, refuse to install
the middleware when the workspace key cannot be resolved, and document a
hard fail in CHANGELOG / DoD instead of silently inert mitigations.

---

### CR-04: Receipts issued by `find_references` / `analyze_blast_radius` carry `SymbolID: ""` — STRICT scope validation rejects them for any symbol-anchored target

**File:** `internal/kernel/symbols/tools.go:391-397, 661-670`

**Issue:** Both call sites populate the receipt scope with `SymbolID: ""`:
```go
guardrails.IssueReceiptOnSuccess(ctx, guardrails.ClassReferencesChecked,
    guardrails.ReferencesCheckedScope{
        SymbolID:     "",   // <- always empty
        ...
    }, "find_references")
```
Downstream STRICT scope check in `validate.go:91-92`:
```go
if target.SymbolID != "" && s.SymbolID != target.SymbolID {
    return ErrReceiptScopeMismatch
}
```
G-001/G-002/G-003 build a `Target{SymbolID: args.SymbolID, ...}` and pass it
to `findReceiptCovering`. When `args.SymbolID != ""` (any non-empty target
symbol) but the receipt scope's SymbolID is `""`, validation fails with
`ErrReceiptScopeMismatch` — so the receipts these read tools issue cannot
satisfy any rule's symbol-anchored requirement. The agent runs
`find_references`, gets a receipt, hands it back, and the guardrail rejects
it. The `analyze_blast_radius` semantic arm (line 727-736) does carry
`SymbolID: sym`, but the LSP-only fallback arm at 661-670 does not.

**Fix:** Resolve the target SymbolID at the read-tool entry, even when the
LSP-only path renders the body. For `find_references`, translate
`(args.Path, lspLine, lspCol)` via `lookup.SymbolID(...)` when the lookup is
available; on miss/error, omit the receipt or annotate scope with the
position fields the guardrail rules already validate against (path is
already there). For the LSP-only arm of `analyze_blast_radius`, do the same
or refuse to issue a symbol-anchored receipt class.

Alternatively (less ideal): relax the STRICT scope check for the LSP-only
path, but that re-opens T-66-06 cross-workspace concerns and conflicts with
D-09 LOCKED.

---

## Warnings

### WR-01: `IssueReceiptOnSuccess` for `get_semantic_context` uses `context.Background()` instead of the request context

**File:** `internal/skill/semantic/tools_context.go:318`

**Issue:** Every other receipt issuance call site threads the caller's `ctx`,
but `tools_context.go:318` substitutes `context.Background()`. This breaks
deadline propagation, request-scoped tracing (the daemon-side issuance sink
will not be cancellable from the handler), and prevents future tracing-span
attachment.

**Fix:** Pass `ctx` from the enclosing handler:
```go
guardrails.IssueReceiptOnSuccess(ctx, guardrails.ClassContextGathered, ...)
```
Confirm `handleGetSemanticContext` has `ctx` already in scope.

---

### WR-02: G-001 outline-error fail-open contradicts G-002/G-003 conservative-warn invariant

**File:** `internal/guardrails/rules/g001_rename_by_grep.go:48-52`

**Issue:** When `OutlineProvider.SymbolsInFile` returns an error or empty
slice, G-001 returns `Decision{Action: Allow}` ("Cannot determine outline:
safe to allow"). This is the opposite of D-19's conservative-warn rule that
G-002/G-003 follow when their semantic source is unavailable. An agent on a
language whose outline extractor is broken (or simply not wired —
`noopOutlineProvider` from `daemon/guardrail_deps.go:208-212` returns `(nil,
nil)` for every call, currently the production path) gets G-001 silently
disabled across the entire repo.

**Fix:** Match the G-002/G-003 degraded behaviour — issue a conservative
Warn with a remediation hint when the outline cannot be determined and the
identifier shape gate fired. At minimum, document this divergence in a
SKILL.md / GUARDRAILS.md decision row.

---

### WR-03: Production OutlineProvider is `noopOutlineProvider` — G-001 never triggers in production

**File:** `internal/daemon/guardrail_deps.go:208-212` (no-op outline provider)
**File:** `internal/daemon/daemon.go:737`

**Issue:** The daemon wires `noopOutlineProvider{}` as the production
`OutlineProvider`, with a TODO("phase-66.x"). The noop returns `(nil, nil)`
unconditionally; combined with WR-02, G-001 is structurally Allow on every
call in v1.10. The phase ships with the rule predicate present but the
trigger path dead.

**Fix:** Wire a real adapter to the kernel's tree-sitter outline (the same
seam already used by `get_symbol_overview`). Add an integration smoke test
that asserts G-001 fires for a `fuzzy_edit` whose `find` matches a known
symbol. Until then, gate the rule registration on a non-noop provider so
operators are not misled by a "G-001 enforced" config that does nothing.

---

### WR-04: G-001 `containsIdentifier` is a duplicate of the `==` check next to it (dead code)

**File:** `internal/guardrails/rules/g001_rename_by_grep.go:54-58, 106-109`

**Issue:** The matching loop:
```go
if sym.Name == args.Find || containsIdentifier(sym.Name, args.Find) { ... }
```
And `containsIdentifier` is defined as:
```go
func containsIdentifier(symName, find string) bool {
    return symName == find
}
```
The OR is `A || A`. Either the helper was meant to do whole-word substring
matching (the comment "or contains find as an identifier substring" implies
it) and was never finished, or the helper is dead and should be removed.
Tests and trigger semantics will diverge from the documented behaviour
(GUARDRAILS.md §G-001 says "find equals or is a substring of a known
symbol name").

**Fix:** Either implement the substring-with-word-boundary semantics
described in the docstring, or delete the helper and the redundant clause.
Update the doc to match.

---

### WR-05: `g005_security_sensitive.go` `joinSignals` builds string with `+=` (O(n²)) and is used inside an error message

**File:** `internal/guardrails/rules/g005_security_sensitive.go:152-161`

**Issue:** `joinSignals` is functionally `strings.Join(signals, ", ")`. The
hand-rolled `+=` allocates a new string on every iteration. With at most
three signals (`path_glob`, `identifier`, `import_pattern`) this is not a
runtime hazard, but it is a code-quality smell that exists in a security-
sensitive predicate where reviewers will look. Replace with stdlib:
```go
return strings.Join(signals, ", ")
```

---

### WR-06: `langFromPath` silently fails closed for any language whose extension is not in the four-case switch

**File:** `internal/guardrails/rules/g005_security_sensitive.go:128-150`

**Issue:** The function returns `""` for `.java`, `.rs`, `.cs`, `.kt`, `.rb`,
`.swift`, `.php`, `.c`, `.cpp`, `.h`, `.hpp`, etc. `resolveG005Catalog`
then returns an empty `Catalog{}` and `IsSecuritySensitive` cannot fire any
of the three signals. G-005 silently degrades to allow-everything for any
language not in the four embedded YAML catalogs. Combined with the empty
`SemanticLookup` catalogs map (`guardrail_deps.go:160-162`), G-005 is
silently disabled in production for the majority of supported languages
even when operators have configured `enforcement: enforce` for it.

**Fix:** Document this as a known limitation in GUARDRAILS.md (the
"Java/Rust/C# G-005 catalogs deferred" line is partial — it omits Ruby,
Kotlin, PHP, Swift, etc.). Better: emit a one-shot warn-log at startup for
each language present in the workspace that has no catalog, and surface
the gap on `get_health`.

---

### WR-07: `evaluator.go` per-loop class-precheck is dead — STRICT scope check already covers it

**File:** `internal/guardrails/rules/evaluator.go:93-103`

**Issue:** `findReceiptCovering` does:
```go
for _, required := range requiredClasses {
    t := target
    t.RequiredClass = required
    if rcpt.Class != required {
        continue
    }
    if err := guardrails.ValidateReceiptForOperation(rcpt, t, ...); err == nil {
        return true
    }
}
```
`ValidateReceiptForOperation` already enforces `target.RequiredClass != "" &&
rcpt.Class != target.RequiredClass → ErrWrongReceiptClass` (validate.go:69-72).
The `if rcpt.Class != required { continue }` inside the inner loop duplicates
that check. It is harmless but suggests the validation contract is unclear
to maintainers. Remove the duplicate and let the validator be the single
gate; or document why the early-out is intentional (it is not).

---

### WR-08: `MaxFileChangeRatio <= 0` falls back to default — operator cannot set "ratio trigger off via this knob"

**File:** `internal/guardrails/rules/g004_large_fuzzy_edit.go:39-42`

**Issue:** `cfg.G004.MaxFileChangeRatio` defaults to 0.30 when `<= 0`. An
operator who legitimately wants the ratio threshold at e.g. `0.0` (every
ratio over zero triggers) cannot express it; setting `0` silently coerces to
the default. The same antipattern exists for `MaxChangedLines` and
`MaxFiles`. The proper toggle for "ratio trigger off" is
`EnableRatioTrigger=false`, but the rule does not communicate that
constraint, and `0` is a footgun — particularly since `int` zero value is
indistinguishable from "unset" without a `*int` or `Set bool` companion.

**Fix:** Either accept `< 0` as "unset" / use a sentinel (a separate `Set`
flag like `EnforcementLayer.Set`), or document the constraint in the
G004Config struct comment.

---

## Info

### IN-01: `receiptIDTotalLen` declared but unused

**File:** `internal/guardrails/receipt_id.go:21`

**Issue:** `receiptIDTotalLen = len(receiptIDPrefix) + receiptIDBodyLen` is
defined but nothing references it. The total-length invariant
(`len(prefix)+26 == 31`) is enforced indirectly by the prefix and body-length
checks in `ParseReceiptID`. Either delete the constant or use it (e.g.,
`if len(s) != receiptIDTotalLen { return ErrInvalidReceiptIDLength }` as a
fast path before the body length check).

---

### IN-02: `Receipt.PendingLSPFiles` vs `IssueFields.PendingLSP` — field name drift

**File:** `internal/guardrails/receipt.go:51`
**File:** `internal/guardrails/store.go:28`

**Issue:** The `Receipt` struct names the field `PendingLSPFiles`, but the
`IssueFields` carrier names it `PendingLSP`. This is a minor naming drift
that will trip up future readers (and any reflection-based marshaller).
Rename `IssueFields.PendingLSP` → `PendingLSPFiles` for parity.

---

### IN-03: `langFromPath` recurses on a backwards-iterating loop instead of using `filepath.Ext` / `path.Ext`

**File:** `internal/guardrails/rules/g005_security_sensitive.go:128-150`

**Issue:** Hand-rolled extension extraction with manual `\` and `/`
handling. `path.Ext("/foo/bar.go")` returns `.go` and is byte-for-byte
equivalent on POSIX paths; `filepath.Ext` handles OS-native separators.
Replacing the loop with stdlib is one line and avoids the off-by-one
hazards in the manual scan. Cleanup, not correctness.

---

### IN-04: G-002 `evaluateG002DeleteFile` uses `requiredClasses` only in the non-exported branch

**File:** `internal/guardrails/rules/g002_delete_without_refs.go:58-78`

**Issue:** The local `requiredClasses := []ReceiptClass{ClassReferencesChecked,
ClassImpactChecked}` is declared at line 58 but used only in the else branch
at line 75. The `if args.HasExportedSymbols` branch builds its own
`[]ReceiptClass{ClassImpactChecked}` inline. Move the declaration into the
else branch or delete it for clarity.

---

### IN-05: GUARDRAILS topic file (`docs/guardrails.md`) is sourced from the root copy via `make sync-docs`, but no `sync-docs` target was reviewed

**File:** `internal/kernel/help/docs/guardrails.md:1`

**Issue:** Header reads `<!-- Synced from /GUARDRAILS.md; do not edit
directly — edit the root copy and re-run `make sync-docs` -->`. If the
`make sync-docs` target is missing or broken the embedded help drifts from
the root document silently. Verify the Makefile target exists and runs in
CI; otherwise replace the comment with a generated marker that CI
asserts byte-equal.

---

_Reviewed: 2026-05-09_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_

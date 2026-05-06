---
phase: 61-lsp-enrichment-worker
reviewed: 2026-05-06T00:00:00Z
depth: standard
files_reviewed: 6
files_reviewed_list:
  - internal/semantic/lspenrich/cascade_lsp_shim.go
  - internal/semantic/lspenrich/manager.go
  - internal/daemon/live_wiring.go
  - internal/semantic/lspenrich/cascade_lsp_shim_test.go
  - internal/semantic/lspenrich/integration_dispatch_test.go
  - internal/semantic/lspenrich/cascade_integration_test.go
findings:
  critical: 0
  warning: 5
  info: 4
  total: 9
status: warnings
---

# Phase 61 (plan 05): Code Review Report

**Reviewed:** 2026-05-06
**Depth:** standard
**Files Reviewed:** 6 (3 source, 3 tests)
**Status:** warnings

## Summary

Plan 61-05 promotes the test-only `realLSPShim` into production code as `cascadeLSPShim`, threads a `CascadeLSPFactory` injection seam through `Manager`, and wires `live_wiring.go` to supply it. The change is **functionally correct** for the gap-closure objective: the integration test passes, the cascade engine now runs end-to-end against real LSPs from production daemon dispatch, and lease ownership stays with the Manager (B2 invariant respected — the shim never calls `lease.Release` or holds the lease handle).

No BLOCKERs were found — the load-bearing wiring is solid and the gap-closure assertion in `TestManagerProductionDispatch_Go` actually exercises the production path (verified via inspection of `mgr.SetCascadeLSPFactory(...)` at line 177 mirroring `live_wiring.go:296-298`).

The WARNINGs are mostly inherited from the verbatim-promoted `realLSPShim` (string-match error classification, `==` vs `errors.Is`) plus one new minor concurrency-correctness gap on `Manager.SetCascadeLSPFactory`. None are urgent enough to block ship; all should be tracked in a follow-up.

## Notable Strengths

- **Lease lifecycle is provably correct.** The shim NEVER calls `lease.Release` (verified by `grep` — zero matches in `cascade_lsp_shim.go`); the doc-comment B2 invariant at lines 16-18 is accurate. Cascade dispatch is sequential per-file via `Cascade.Run` in `cascade.go:222-449`, so the URI memoization in `cascadeLSPShim` is not exposed to concurrent access in the current code path. The internal `sync.Mutex` is therefore defensive (good).
- **`leaseRequester` interface is the right seam.** Decoupling the shim from `*lspool.WorkerLease` for unit-testability without changing the public `CascadeLSPFactory` signature is a clean piece of design — the public constructor still takes `*lspool.WorkerLease`, the four pre-existing test-only injection sites compile unchanged, and the test seam is unexported.
- **Lazy URI resolution is sound.** The cascade fires `DocumentSymbol(ctx, job.Path)` first (`cascade.go:295`) before any per-symbol calls. The URI is cached BEFORE Hover/CallHierarchy/etc. reach the shim. The fall-back path (per-symbol call without a prior DocumentSymbol) returns `(nil, nil)` gracefully — no panic.
- **TOCTOU on `m.newCascadeLSP` is benign in current usage.** `SetCascadeLSPFactory` writes the field; `Run` reads it. In `live_wiring.go:281-298`, `SetCascadeLSPFactory` is called synchronously before the daemon's errgroup launches `bundle.Run`, providing happens-before. The Warn log at `manager.go:229-234` makes the unwired-factory failure mode loud at startup.
- **Gap-closure test asserts the live path, not a bypass.** `TestManagerProductionDispatch_Go` (`integration_dispatch_test.go:177-179`) calls `SetCascadeLSPFactory` with the same `NewCascadeLSPShim` adapter that production uses; it asserts `FilesDropped == 0` (line 233-236) which would fail under pre-61-05 behavior. The polling loop at lines 201-209 correctly breaks on any outcome (drop included) so a regression cannot silently pass.

---

## Warnings

### WR-01: `Manager.newCascadeLSP` field has no memory synchronization between writer and reader

**File:** `internal/semantic/lspenrich/manager.go:65, 137-139, 229, 244, 254`
**Issue:** `SetCascadeLSPFactory` writes `m.newCascadeLSP` without holding any lock; `Run` reads the same field. The doc-comment at lines 124-127 says "MUST be called BEFORE Run is invoked", but there is no enforcement and no fence. In current production wiring this is safe because `live_wiring.go` calls `SetCascadeLSPFactory` synchronously before the errgroup spawns `bundle.Run` (the goroutine creation provides happens-before). However, a future caller pattern such as:

```go
go mgr.Run(ctx)
mgr.SetCascadeLSPFactory(f) // ← race
```

would be a Go data race that `go test -race` will flag, and the documented "no-op after Run" claim is technically incorrect — the field IS mutated, just unobserved by the running Worker (which already captured the value).

**Fix:** Either guard with a once-token/atomic, or strengthen the doc to "MUST be called from the same goroutine that calls Run, before Run". For example:

```go
type Manager struct {
    // ...
    newCascadeLSPMu sync.Mutex
    newCascadeLSP   CascadeLSPFactory
}

func (m *Manager) SetCascadeLSPFactory(f CascadeLSPFactory) {
    m.newCascadeLSPMu.Lock()
    m.newCascadeLSP = f
    m.newCascadeLSPMu.Unlock()
}

// In Run:
m.newCascadeLSPMu.Lock()
factory := m.newCascadeLSP
m.newCascadeLSPMu.Unlock()
```

Or simpler: panic in `SetCascadeLSPFactory` if called after `m.cancel != nil` (Run started). Either documents intent and traps misuse.

---

### WR-02: `isJSONRPCMethodNotFound` substring matching is fragile in production

**File:** `internal/semantic/lspenrich/cascade_lsp_shim.go:401-409`
**Issue:** `ResponseError.Error()` (`internal/kernel/jsonrpc/message.go:33-35`) returns ONLY `e.Message` — the numeric `-32601` Code never appears in the error string. So the `strings.Contains(s, "-32601")` branch is dead in production; only the literal Message contents `"method not found"` / `"MethodNotFound"` can match. Real LS servers emit varied messages (gopls, jdtls, rust-analyzer all phrase the message differently — e.g., `"method not supported"` would NOT match), and the matcher is case-sensitive on `"method not found"` but `"Method Not Found"` would still match via the `MethodNotFound` substring.

This silent matcher is acceptable in the integration test (where you control the LS), but in production it can MASK real Method-Not-Found errors as generic `OutcomePartialLSPUnavail`, defeating the per-(lang, method) `CapabilityCache` deduplication. The cascade then re-attempts the same unsupported method on every job.

**Fix:** Have the kernel/jsonrpc layer expose a typed `IsMethodNotFound(err error) bool` (or a sentinel + `errors.As(*ResponseError)`) so the shim can match on `Code == -32601` directly. Avoids semantic→kernel/* import constraint by adding the helper to a third package or surfacing it through `*lspool.WorkerLease`. Alternatively, document the matcher's known fragility and add a TODO referencing the typed-error path. Until then, broaden the matcher to be case-insensitive on all variants:

```go
func isJSONRPCMethodNotFound(err error) bool {
    if err == nil {
        return false
    }
    s := strings.ToLower(err.Error())
    return strings.Contains(s, "-32601") ||
        strings.Contains(s, "method not found") ||
        strings.Contains(s, "methodnotfound")
}
```

---

### WR-03: Integration test masks real failures with `t.Skipf` after the skip-gate already passed

**File:** `internal/semantic/lspenrich/integration_dispatch_test.go:113-117`
**Issue:** `skipIfMissing(t, "gopls")` at line 97 already established that gopls is on PATH. After that point, any `pool.AcquireLease` failure is a real bug (e.g., a regression in `*lspool.Pool` or `langregistry`). Demoting it to `t.Skipf("could not acquire warmup lease (gopls install issue?)")` makes the test a no-op for any acquire-side regression that lands in the future — exactly the failure mode the gap-closure test was designed to catch.

**Fix:**

```go
warmLease, err := pool.AcquireLease(bootCtx, "test-prewarm:go", wsKey, false)
bootCancel()
if err != nil {
    t.Fatalf("warmup lease acquire failed (gopls is on PATH per skipIfMissing): %v", err)
}
```

---

### WR-04: Direct equality on `context.Canceled` misses wrapped errors

**File:** `internal/semantic/lspenrich/integration_dispatch_test.go:216`
**Issue:** `if err != nil && err != context.Canceled` uses `==` which only matches the unwrapped sentinel. `Manager.Run` returns `g.Wait()` which can return wrapped context errors. The existing production code at `internal/daemon/live_wiring.go:81` correctly uses `errors.Is(err, context.Canceled)`. Inconsistent treatment within the same plan; one of them is wrong.

**Fix:**

```go
if err != nil && !errors.Is(err, context.Canceled) {
    t.Errorf("mgr.Run returned non-nil non-Canceled err: %v", err)
}
```

(Add `"errors"` import.)

---

### WR-05: `kindLabel` uses magic numbers when typed `gen.SymbolKind` constants are available

**File:** `internal/semantic/lspenrich/cascade_lsp_shim.go:449-460`
**Issue:** The function switches on `case 5: ... case 12: ... case 6:` with a doc comment explaining "5/12/6 are the LSP wire numbers for Class/Function/Method." But `protocol/gen/tsprotocol.go:140-141` ships typed constants `gen.SymbolKindClass = 5`, `gen.SymbolKindMethod = 6`, `gen.SymbolKindFunction = 12`. Using the magic numbers loses type safety, makes the switch silently miscompile if the LSP wire numbers ever shift in the metaModel regen, and removes the at-a-glance grep-ability of the constants.

**Fix:**

```go
func kindLabel(k gen.SymbolKind) string {
    switch k {
    case gen.SymbolKindClass:
        return "Class"
    case gen.SymbolKindFunction:
        return "Function"
    case gen.SymbolKindMethod:
        return "Method"
    default:
        return fmt.Sprintf("Kind(%d)", k)
    }
}
```

---

## Info

### IN-01: `leaseRequester.Notify` is dead in the shim

**File:** `internal/semantic/lspenrich/cascade_lsp_shim.go:48-51`
**Issue:** The `leaseRequester` interface declares both `Request` and `Notify`. The shim never calls `Notify` on the lease (verified by `grep "lease.Notify\|s\.lease\.Notify"` — zero matches in cascade_lsp_shim.go). The `Notify` method is only there because the original `realLSPShim` was constructed against `*lspool.WorkerLease` directly and the integration test invoked `lease.Notify` for `didOpen` BEFORE constructing the shim. After the test seam was introduced, `Notify` became dead surface area. The `fakeLease.Notify` (test line 61) is also unused.

**Fix:** Drop `Notify` from the `leaseRequester` interface. The integration test's `didOpen` calls operate on the concrete `*WorkerLease` directly, not through the shim's interface — removing it from the test seam doesn't affect anything. Smaller interface = clearer contract.

```go
type leaseRequester interface {
    Request(ctx context.Context, method string, params, result any) error
}
```

---

### IN-02: Repeated lock acquisitions in `Hover` / `CallHierarchy` / `TypeHierarchy` / `Implementation`

**File:** `internal/semantic/lspenrich/cascade_lsp_shim.go:186-219, 221-269, 271-312, 314-353`
**Issue:** Each method acquires `s.mu` three times: once in `s.uriOrEmpty()` (early-return guard), once inside `s.pickSymbolPosition(sym)`, and once again in `s.uriOrEmpty()` to populate the params map. The cascade is sequential so this is not racy, but the lock churn is unnecessary. Fetching the URI once and reusing the value is clearer and avoids the (currently theoretical) inconsistency window where the URI could change between the guard read and the param-set read.

**Fix:**

```go
func (s *cascadeLSPShim) Hover(ctx context.Context, sym Symbol) (*Edge, error) {
    uri := s.uriOrEmpty()
    if uri == "" {
        return nil, nil
    }
    pos, ok := s.pickSymbolPosition(sym)
    if !ok {
        return nil, nil
    }
    params := map[string]any{
        "textDocument": map[string]any{"uri": uri},
        "position":     map[string]any{"line": pos.Line, "character": pos.Character},
    }
    // ...
}
```

Apply the same pattern to CallHierarchy / TypeHierarchy / Implementation.

---

### IN-03: `len(e.Source) < 4 || e.Source[:4] != "lsp."` should use `strings.HasPrefix`

**File:** `internal/semantic/lspenrich/integration_dispatch_test.go:256`
**Issue:** Hand-rolled prefix check. `strings.HasPrefix(e.Source, "lsp.")` is one call, one allocation-free comparison, and reads cleanly.

**Fix:**

```go
if !strings.HasPrefix(e.Source, "lsp.") {
    t.Errorf("edge[%d].Source: got %q, want prefix lsp.", i, e.Source)
}
```

(Add `"strings"` import.)

---

### IN-04: Doc-comment overstates "no-op" semantics of `SetCascadeLSPFactory` after `Run`

**File:** `internal/semantic/lspenrich/manager.go:124-127`
**Issue:** The comment says "calling after Run has started is a no-op (the Worker has already been constructed with whatever factory value was set at the moment Run() reached the Worker struct literal)." Strictly: the Worker has indeed captured the value, but `m.newCascadeLSP` is still mutated by the post-Run setter — a subsequent `Run` call (after `Stop`) would observe the new value. So it's "no-op for the currently-running Worker" but not a true no-op. Minor doc imprecision.

**Fix:** Tighten the wording:

```go
// SetCascadeLSPFactory injects the production CascadeLSP factory.
// MUST be called before Run; the factory is captured into the Worker
// struct literal at the moment Run() runs. Calls AFTER Run() has reached
// the Worker construction step are observable only by a subsequent Run()
// invocation (after Stop). Concurrent Run + Set is a data race; see
// WR-01 in the 61-REVIEW.md follow-up.
```

---

_Reviewed: 2026-05-06_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_

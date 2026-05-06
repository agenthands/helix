---
phase: 61-lsp-enrichment-worker
fixed_at: 2026-05-06T08:50:00Z
review_path: .planning/phases/61-lsp-enrichment-worker/61-REVIEW.md
iteration: 1
findings_in_scope: 9
fixed: 9
skipped: 0
status: all_fixed
---

# Phase 61 (plan 05): Code Review Fix Report

**Fixed at:** 2026-05-06
**Source review:** `.planning/phases/61-lsp-enrichment-worker/61-REVIEW.md`
**Iteration:** 1
**Scope:** `--all` (warnings + info)

**Summary:**
- Findings in scope: 9 (5 warning + 4 info)
- Fixed: 9 (one folded — IN-04 merged into WR-01)
- Skipped: 0
- Distinct commits: 8

## Per-finding outcome

| Finding | Status | Commit   | Notes |
|---------|--------|----------|-------|
| WR-01   | FIXED  | `aee8001e` | Added `newCascadeLSPMu sync.Mutex`; SetCascadeLSPFactory takes the lock; Run snapshots `factory := m.newCascadeLSP` under the lock and uses the local for nil-check, Worker struct literal, and startup INFO log. Race detector now sees Set→Run happens-before. |
| WR-02   | FIXED  | `0e18c4b8` | Used the **substring fallback** path: `strings.ToLower(err.Error())` then `Contains` for `"-32601"`, `"method not found"`, `"methodnotfound"`. Verified the typed kernel/jsonrpc path is **rejected** by `internal/lint/nosemantic2kernel/analyzer.go` — only `internal/kernel/lspool` is in `lspoolPkgPath` carve-out; `internal/kernel/jsonrpc` would fail `make vet`. Documented as a TODO targeting phase 62+ for a typed helper surfaced through `*lspool.WorkerLease` or a third package. |
| WR-03   | FIXED  | `3655a64c` | `t.Skipf` → `t.Fatalf` with explanatory message: skipIfMissing already established gopls is on PATH, so AcquireLease failure is a real bug. |
| WR-04   | FIXED  | `a59f2fba` | `err != context.Canceled` → `!errors.Is(err, context.Canceled)`. Added `"errors"` import. |
| WR-05   | FIXED  | `3b698385` | `case 5/12/6` → `gen.SymbolKindClass / SymbolKindFunction / SymbolKindMethod`. Function signature already took `gen.SymbolKind`, no caller change. |
| IN-01   | FIXED  | `c2221ffa` | Dropped `Notify` from `leaseRequester` interface and from test `fakeLease`. Pre-flight grep confirmed zero matches for `lease.Notify` / `s.lease.Notify` in `cascade_lsp_shim.go`. |
| IN-02   | FIXED  | `436c3786` | Hoisted `uri := s.uriOrEmpty()` to the top of Hover, CallHierarchy, TypeHierarchy, Implementation. Early-return on empty; reused local `uri` in the params map. Removed the second `s.uriOrEmpty()` call inside each method. |
| IN-03   | FIXED  | `4fdfe602` | `len(e.Source) < 4 || e.Source[:4] != "lsp."` → `!strings.HasPrefix(e.Source, "lsp.")`. Added `"strings"` import. |
| IN-04   | merged | `aee8001e` | Folded into WR-01 per instructions. Doc-comment on `SetCascadeLSPFactory` rewritten to describe the new locking semantics and the precise "currently-running Worker has snapshotted its factory; subsequent Run() observes new value" behavior. |

## WR-02 path decision

The instruction allowed two paths: typed `errors.As(*jsonrpc.ResponseError)` if the analyzer permits the import, otherwise the case-insensitive substring fallback.

**Chosen: substring fallback.**

**Reason:** `internal/lint/nosemantic2kernel/analyzer.go` permits only `internal/kernel/lspool` (and its sub-packages) as the carve-out (`lspoolPkgPath = ".../kernel/lspool"`, `lspoolSubPkgPrefix = ".../kernel/lspool/"`). Importing `internal/kernel/jsonrpc` would be reported as a violation by the analyzer, and `make vet` runs `vet-nosemantic2kernel` against `./...` per the Makefile. The fallback matcher is documented with a TODO referencing the typed-error surface for a future phase (62+) when the kernel can expose `IsMethodNotFound` through `*lspool.WorkerLease` or a third package outside `internal/kernel/`.

The verification analyzer was run explicitly to confirm the change is clean:

```
$ go vet -vettool=$(go env GOPATH)/bin/vet-nosemantic2kernel ./internal/semantic/...
EXIT: 0
$ go vet -vettool=$(go env GOPATH)/bin/vet-nokernel2semantic ./internal/kernel/...
EXIT: 0
```

## Final verification

All four required verification commands ran clean:

```
$ go list ./... | grep -v '^github.com/agenthands/helix/tmp/' | xargs go build
BUILD EXIT: 0  (only benign Swift treesitter TOKEN_COUNT macro-redefine warning + test/bench info)

$ go vet $(go list ./... | grep -v tmp/)
VET EXIT: 0

$ go test -short -timeout 180s ./internal/semantic/lspenrich/... ./internal/daemon/... ./internal/kernel/lspool/...
ok   github.com/agenthands/helix/internal/semantic/lspenrich  1.134s
ok   github.com/agenthands/helix/internal/daemon              2.332s
ok   github.com/agenthands/helix/internal/kernel/lspool       5.969s
EXIT: 0

$ go test -tags integration -timeout 240s \
    -run "TestManagerProductionDispatch_Go|TestCascade_GoIntegration" \
    ./internal/semantic/lspenrich/...
ok   github.com/agenthands/helix/internal/semantic/lspenrich  5.169s
EXIT: 0
```

Additional:

- **`go test -race -short` on `./internal/semantic/lspenrich/...`** → clean (validates WR-01's mutex closes the documented data race).
- **`vet-nosemantic2kernel`** → clean (validates WR-02's matcher kept the import boundary).
- **`vet-nokernel2semantic`** → clean (no kernel→semantic regression).

## Files modified

- `internal/semantic/lspenrich/manager.go` (WR-01, IN-04)
- `internal/semantic/lspenrich/cascade_lsp_shim.go` (WR-02, WR-05, IN-01, IN-02)
- `internal/semantic/lspenrich/cascade_lsp_shim_test.go` (IN-01)
- `internal/semantic/lspenrich/integration_dispatch_test.go` (WR-03, WR-04, IN-03)

No production wiring outside `internal/semantic/lspenrich/` was touched (`internal/daemon/live_wiring.go` did not need to change — WR-01's mutex change is callsite-transparent).

---

_Fixed: 2026-05-06_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_

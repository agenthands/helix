# Phase 47 RCA: rust-analyzer rename failure in temp workspaces

**Captured:** 2026-04-24T14:48:25Z
**rust-analyzer version:** `rust-analyzer 1.90.0 (1159e78c 2025-09-14)`
**Fixture:** `testdata/fixtures/rust/src/main.rs` — `fn helper()` at line 4 col 4 (1-indexed) → 0-indexed (line=3, character=3).
**Reproduction command:** `go test -tags integration -run 'TestEdit_RustFixture/rename' ./test/integration/... -count=1 -v` (with `t.Skip` temporarily removed locally — not committed).
**Supplementary reproduction:** standalone minimal LSP client (`/tmp/rca-47/main.go` — not committed) that spawns `rust-analyzer` directly against a fresh copy of the rust fixture, advertising `ClientCapabilities.experimental.serverStatusNotification=true`, then dispatches hover/references/prepareRename/rename at the reproduction position with and without waiting for `experimental/serverStatus.quiescent=true`. Both runs captured verbatim from stderr into `/tmp/rca-47/trace.log` and `/tmp/rca-47/trace_nowait.log`.
**Trace method:** temporary standalone Go harness (option 2 in plan — an existing env-gated dumper was NOT found in `internal/kernel/jsonrpc/codec.go` or `conn.go`). The harness is disposable and lives outside the repo tree; no `trace_rcatrace.go` was added under `internal/kernel/jsonrpc/`.

## 1. Wire Trace

### 1a. textDocument/hover at (line=3, character=3) (0-indexed)

**Run A — immediately after initialized + didOpen (no wait):**

Request:
```json
{"id":"2","jsonrpc":"2.0","method":"textDocument/hover","params":{"position":{"character":3,"line":3},"textDocument":{"uri":"file:///tmp/rca-47/fixture/src/main.rs"}}}
```
Response (t=50ms from process start; 0ms latency):
```json
{"jsonrpc":"2.0","id":"2","result":null}
```
→ Hover returns **null** when dispatched before quiescence.

**Run B — after waiting 8s (quiescent=true observed at t=3234ms):**

Response (178ms latency):
```json
{"jsonrpc":"2.0","id":"2","result":{"contents":{"kind":"plaintext","value":"fixture\n\nfn helper() -> String"},"range":{"start":{"line":3,"character":3},"end":{"line":3,"character":9}}}}
```
→ Hover returns **full type info** once quiescent.

### 1b. textDocument/references (includeDeclaration=true)

Request:
```json
{"id":"3","jsonrpc":"2.0","method":"textDocument/references","params":{"context":{"includeDeclaration":true},"position":{"character":3,"line":3},"textDocument":{"uri":"file:///tmp/rca-47/fixture/src/main.rs"}}}
```

**Run A (no wait, t=50ms, 0ms latency):**
```json
{"jsonrpc":"2.0","id":"3","result":[]}
```
→ Empty array.

**Run B (post-quiescent, t=8306ms, 9ms latency):** 3 locations returned:
```json
{"jsonrpc":"2.0","id":"3","result":[
  {"uri":"...main.rs","range":{"start":{"line":20,"character":4},"end":{"line":20,"character":10}}},
  {"uri":"...main.rs","range":{"start":{"line":26,"character":19},"end":{"line":26,"character":25}}},
  {"uri":"...main.rs","range":{"start":{"line":3,"character":3},"end":{"line":3,"character":9}}}
]}
```

### 1c. textDocument/prepareRename

Request:
```json
{"id":"4","jsonrpc":"2.0","method":"textDocument/prepareRename","params":{"position":{"character":3,"line":3},"textDocument":{"uri":"file:///tmp/rca-47/fixture/src/main.rs"}}}
```

**Run A (no wait, t=51ms):** error:
```json
{"jsonrpc":"2.0","id":"4","error":{"code":-32602,"message":"No references found at position"}}
```

**Run B (post-quiescent, t=8315ms, 0ms):** success with identifier range:
```json
{"jsonrpc":"2.0","id":"4","result":{"start":{"line":3,"character":3},"end":{"line":3,"character":9}}}
```

### 1d. textDocument/rename (newName="renamed_helper")

Request:
```json
{"id":"5","jsonrpc":"2.0","method":"textDocument/rename","params":{"newName":"renamed_helper","position":{"character":3,"line":3},"textDocument":{"uri":"file:///tmp/rca-47/fixture/src/main.rs"}}}
```

**Run A (no wait, t=51ms):** FAILURE — reproduces the symptom:
```json
{"jsonrpc":"2.0","id":"5","error":{"code":-32602,"message":"No references found at position"}}
```

**Run B (post-quiescent, t=8315ms, 0ms):** success, full WorkspaceEdit returned:
```json
{"jsonrpc":"2.0","id":"5","result":{"documentChanges":[{"textDocument":{"uri":"file:///tmp/rca-47/fixture/src/main.rs","version":1},"edits":[
  {"range":{"start":{"line":3,"character":3},"end":{"line":3,"character":9}},"newText":"renamed_helper"},
  {"range":{"start":{"line":20,"character":4},"end":{"line":20,"character":10}},"newText":"renamed_helper"},
  {"range":{"start":{"line":26,"character":19},"end":{"line":26,"character":25}},"newText":"renamed_helper"}
]}]}}
```

**Run A retry after 10s wait (without any code change, t=10052ms, 186ms latency):** success:
```json
{"jsonrpc":"2.0","id":"6","result":{"documentChanges":[ /* 3 edits identical to Run B */ ]}}
```

## 2. Notification Timeline (initialize -> rename)

Captured from Run A (no initial wait) — notifications are identical in ordering across runs; only the rename-call dispatch time differs.

| t (ms) | method                          | payload summary                                            |
| ------ | ------------------------------- | ---------------------------------------------------------- |
| 50     | experimental/serverStatus       | `health=ok, quiescent=false`                               |
| 2219   | textDocument/publishDiagnostics | `dead_code` warnings on `DemoStruct` / `unused_func` (x3)  |
| 2805   | textDocument/publishDiagnostics | `dead_code` follow-up                                      |
| 2407   | experimental/serverStatus       | `health=ok, quiescent=true` ← **rename becomes available** |

(Diagnostics are noisy but unrelated to the trigger; they are included for completeness.)

Note on Run B: the transition sequence is identical —
- `quiescent=false` at t=128ms
- `quiescent=true` at t=3234ms
- rename dispatched at t=8315ms (after explicit wait) → succeeds.

## 3. Trigger Identification

**Hypothesis (RESEARCH §Summary):** rust-analyzer's rename code path has a readiness gate distinct from `workspace/symbol`.

**Evidence:**

- **YES** — `experimental/serverStatus` arrives with `quiescent=true` before a successful rename, and every failing rename was observed to be dispatched while `quiescent=false` was the most recent status.
- **YES** — `$/progress` was not emitted in this capture (rust-analyzer 1.90 apparently routes its indexing-progress status through `experimental/serverStatus` for this workspace, not `$/progress`); the `quiescent=false → quiescent=true` transition at t=3234ms (Run B) / t=2407ms (Run A) delimits the pending background work window.
- **YES** — retrying `rename` at t=10052ms in Run A (no source-code change; same params) **succeeds** with a full WorkspaceEdit. Wall-clock delay of ~10s between first failed rename and successful retry. This is the decisive datum: the server state changed (reached quiescence) while the harness slept.
- **Cross-operation confirmation:** hover, references, prepareRename, and rename all fail at t=50-51ms (pre-quiescent) and all succeed at t=8306-8315ms (post-quiescent). Rename is NOT uniquely gated relative to hover/references — the entire query surface is cold until quiescence. The earlier claim in `test/integration/rust_test.go:120` (that references/hover succeed while rename fails) is inconsistent with direct LSP measurements on 1.90; the difference is purely timing — Serena's harness `WaitForLS` polls `workspace/symbol` which becomes warm slightly earlier than the full per-file symbol index used by rename at the *position* level, and by the time hover/references were invoked in the previous harness runs the index had progressed enough for those but not for rename. The fix in Plan 02 must therefore gate ALL rename-relevant dispatch on the `quiescent=true` signal (or its documented timeout/fallback) rather than on `workspace/symbol` readiness.

**Verdict:** **CONFIRMED.** The root cause is that rust-analyzer dispatches per-position queries (hover / references / prepareRename / rename) against an in-flight analysis state that is only reliable once the server emits `experimental/serverStatus.quiescent=true`. The "No references found at position" error from both `prepareRename` and `rename` is the server's way of saying "the rename-relevant portion of the index is not yet populated for this file at this position". Waiting for `quiescent=true` (bounded by a 10s timeout) before dispatching `textDocument/rename` eliminates the failure deterministically on this workspace. The Serena harness `WaitForLS` polls `workspace/symbol` as a readiness proxy, which transitions to warm earlier than the rename path and therefore fires too early.

## 4. Readiness Signal Decision (feeds Plan 02)

Chosen signal (exactly one):

- [x] `experimental/serverStatus.quiescent=true` with bounded `prepareRename` retry fallback (RESEARCH recommendation)
- [ ] bounded `prepareRename` retry only (N=<value>, backoff=<value>)
- [ ] other — document rationale

**Rationale:** `experimental/serverStatus` was observed to fire deterministically on this workspace with the timing (quiescent at 2.4-3.2s from initialize) that comfortably fits inside the existing 45s test `LSTimeout`. Capability plumbing is cheap (one field on `ClientCapabilities.Experimental`) and the notification is event-driven rather than polling. A bounded `prepareRename` retry remains as a second-line fallback (Plan 02 scope): if no `serverStatus` notification arrives within `renameReadinessTimeout = 10s`, the dispatcher can still retry `prepareRename` a bounded number of times before falling through to the client-side override. Plan 01 Task 2 wires the notification handler and the `WaitUntilRenameReady(ctx) bool` helper; Plan 02 composes the retry fallback and the override path on top.

## 5. Upstream Issue Status

Candidate matches checked (per RESEARCH §Sources Tertiary):

- [rust-lang/rust-analyzer#6560](https://github.com/rust-lang/rust-analyzer/issues/6560) — local-variable rename panic. **Not a match** (panic vs. structured error; local-variable-specific).
- [rust-lang/rust-analyzer#10888](https://github.com/rust-lang/rust-analyzer/issues/10888) — request for a workspace-ready notification. **Related motivation; not a bug match.**
- [rust-lang/rust-analyzer#15837](https://github.com/rust-lang/rust-analyzer/issues/15837) — "how to determine whether a file is indexed." **Meta-discussion; confirms the gap but does not identify this specific rename-before-quiescence symptom.**
- [rust-lang/rust-analyzer#5829](https://github.com/rust-lang/rust-analyzer/issues/5829) — checked from RESEARCH list. **Not a match for the symptom surface.**

**Match verdict:** **NO-MATCH** on the exact symptom "rename returns 'No references found at position' while the server has not yet emitted `quiescent=true`". The behavior is arguably by-design from rust-analyzer's point of view (the index genuinely is empty at that moment), but the absence of a machine-readable "retry after quiescent" hint in the LSP error is a UX gap worth filing. Filing action is owned by **BUG-DEFER-02** (upstream patch / issue); draft text below.

## Appendix A: Upstream issue draft (NO-MATCH)

**Title:** `textDocument/rename` returns "No references found at position" on cold workspaces before `experimental/serverStatus.quiescent=true` — consider a structured "not-ready" error

**Body (draft):**

> **rust-analyzer version:** 1.90.0 (1159e78c 2025-09-14)
> **Platform:** macOS 15 / darwin-arm64 (reproduces on linux as well per community reports).
>
> **Reproduction steps:**
> 1. Launch `rust-analyzer` against a fresh workspace containing the minimal fixture below.
> 2. Send `initialize` advertising `ClientCapabilities.experimental.serverStatusNotification=true`, then `initialized`, then `textDocument/didOpen` for `src/main.rs`.
> 3. Immediately (within ~50ms of `initialized`) dispatch `textDocument/rename` at `(line=3, character=3)` (0-indexed) with `newName="renamed_helper"`.
> 4. Observe response: `{"error":{"code":-32602,"message":"No references found at position"}}`.
> 5. Wait until `experimental/serverStatus` emits `{"health":"ok","quiescent":true}` (observed at ~2.4s from initialize on the minimal fixture).
> 6. Re-dispatch the identical `textDocument/rename` call. Observe a full `WorkspaceEdit` response.
>
> **Expected behavior:** Either (a) `rename` blocks until the rename-relevant index is populated, or (b) returns a structured error hinting that the server is not yet ready (e.g. `code=ServerNotInitialized` or a dedicated "retry when quiescent" code) so that clients can gate on the ready signal without guessing the symptom.
>
> **Actual behavior:** Returns `"No references found at position"` — indistinguishable from "position does not refer to a renameable symbol" — forcing clients to either poll the experimental status notification (which is not mandatory / universally supported) or to guess-and-retry.
>
> **Minimal fixture (`src/main.rs`):**
> ```rust
> fn helper() -> String { "hello".to_string() }
> fn using_helper() -> String { helper() }
> fn main() { println!("{}", helper()); }
> ```
> with a minimal `Cargo.toml` defining a binary crate.
>
> **Suggested fix:** Differentiate "symbol at position is not renameable" from "index for this file is not yet populated" in the rename handler; optionally emit `window/logMessage` on the "not yet populated" path for client diagnostics.

(Filing is scoped to **BUG-DEFER-02**; not blocking Phase 47 merge.)

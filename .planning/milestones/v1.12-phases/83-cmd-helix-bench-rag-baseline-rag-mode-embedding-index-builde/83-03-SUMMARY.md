---
phase: 83-cmd-helix-bench-rag-baseline-rag-mode-embedding-index-builde
plan: 03
subsystem: bench
tags: [baseline-rag, rag-control-arm, embedder-id, fairness-contract, delta-operand, stdio-mcp, budget-exclusion, ablate-04]

# Dependency graph
requires:
  - phase: 83
    plan: 01
    provides: "bench/ragindex leaf package (Open/Query/EmbedderID, selectEmbedder, stub-deterministic + HELIX_RAG_FORCE_STUB) the cell builds the index with"
  - phase: 83
    plan: 02
    provides: "cmd/helix-bench-rag standalone 4-tool stdio MCP server the cell spawns in place of the daemon"
  - phase: 77
    provides: "bench/runtime result.v2 builder + fairness contract + cell spine this RAG arm plugs into"
provides:
  - "bench/runtime/result.go embedder_id open-provenance key (ResultInput -> resultDoc omitempty -> BuildResult), additive-minor (schema_version stays v2)"
  - "bench/runtime/subprocess.StartRAGServer + RAGHandle: spawns cmd/helix-bench-rag over its own stdio (D-06 no socket/port), Pid()/Kill() lifecycle sibling of StartDaemon"
  - "bench/runtime/rag.go runRAGCell: real baseline_rag drive leg — out-of-band index build, stdio RAG-server spawn, rag_search drive, same DefaultContract + verify contract, embedder_id-bearing schema-valid result.v2 row"
  - "baseline_rag as an OPTIONAL delta operand (full_minus_baseline_rag) in deltas.go"
  - "real-arm MODE.md + BENCH.md docs (budget exclusion + embedder_id provenance)"
affects: [phase-84+ (baseline_rag is now a real leaderboard control arm with full vs baseline_rag deltas)]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Third subprocess spawn variant (StartRAGServer) alongside StartDaemon/StartClaude: spawns a STDIO MCP server directly (the bench-rag server IS the endpoint — no socket, no forwarder), returning a bench-owned RAGHandle that mirrors DaemonHandle's Pid()/Kill() surface"
    - "Out-of-band index build BEFORE the start:=time.Now() span anchor (mirrors prePatchSnapshot) so embedding-API tokens are excluded from the agent's per-task budget (criterion #4 / Pitfall 3)"
    - "Optional delta operand: baseline_rag participates in full_minus_baseline_rag when its row is present but never gates task completeness (the 4 honest modes still do); a missing baseline_rag omits the comparison rather than skipping the task"
    - "Hermetic HELIX_BIN-gated cell tests via HELIX_RAG_FORCE_STUB + HELIX_CACHE_DIR(t.TempDir()): the deterministic stub embedder makes the drive leg fully no-network and reproducible"

key-files:
  created:
    - bench/runtime/subprocess/ragserver.go
    - bench/runtime/rag.go
    - bench/runtime/baseline_rag_test.go
  modified:
    - bench/runtime/result.go
    - bench/runtime/result_test.go
    - bench/runtime/cell.go
    - bench/runtime/cell_test.go
    - bench/runtime/five_of_six_test.go
    - bench/runtime/deltas.go
    - bench/runtime/deltas_test.go
    - bench/runtime/matrix.go
    - bench/runners/baseline_rag/MODE.md
    - bench/BENCH.md
    - .gitignore

key-decisions:
  - "StartRAGServer returns a NEW bench-side RAGHandle (NOT the planned *evalsandbox.DaemonHandle): the bench-rag server is a STDIO MCP endpoint, not a socketed daemon, and DaemonHandle's unexported fields cannot be constructed outside evalsandbox. RAGHandle mirrors Pid()/Kill() and additionally exposes the Stdin/Stdout MCP pipes the cell drives directly (no forwarder). Deviation from the literal plan signature; the Pid()/Kill() lifecycle contract is preserved verbatim."
  - "The RAG drive leg lives in a dedicated runRAGCell (bench/runtime/rag.go) rather than inline in RunCell: RunCell short-circuits to runRAGCell by mode name right after the unconditional fairness gate, keeping the daemon spine untouched. The two legs share the path layout, fairness gate, verify/RunTests contract, coordinator grading, and result builder."
  - "baseline_rag is an OPTIONAL delta operand: requiredModes (the 4 honest modes) still gate task completeness; operandModes adds baseline_rag for indexing/load/write-back; deltaComparisons marks full_minus_baseline_rag optional so an absent baseline_rag row omits the comparison instead of skipping the whole task. This keeps every pre-existing 4-mode delta test green while adding the headline control comparison."
  - "Hermetic cell tests force the deterministic stub via HELIX_RAG_FORCE_STUB (not by unsetting OPENAI_API_KEY): a live key in the dev env caused a real embedding API call (429 rate-limit) on first attempt. StartRAGServer forwards HELIX_RAG_FORCE_STUB so the server's warm-cache reopen re-supplies the SAME stub embedder, keeping the recorded embedder_id consistent (Pitfall 1)."
  - "five_of_six_test.go was updated (not in the plan's files_modified) because it directly asserted baseline_rag deferral (Deferred==true, no row, 3 deltas) — flipping those assertions was unavoidable once baseline_rag became a real, row-emitting delta operand."

requirements-completed: [ABLATE-04]

# Metrics
duration: ~55min
completed: 2026-06-21
---

# Phase 83 Plan 03: baseline_rag Real Arm — embedder_id + RAG Drive Leg + Delta Operand Summary

**Turns `baseline_rag` from a Phase-80 fail-closed stub into a real retrieval-only control arm: `RunCell` now builds the `bench/ragindex` embedding index OUT-OF-BAND (before the timed span), spawns the standalone `cmd/helix-bench-rag` 4-tool MCP server over its own stdio via the new `subprocess.StartRAGServer`, drives a `rag_search` interaction, and emits a schema-valid `result.v2` row carrying the selected `embedder_id` under `DefaultContract`'s model snapshot + budget VERBATIM (embedding-API cost excluded) — and makes `baseline_rag` an optional `full_minus_baseline_rag` delta operand.**

## Performance
- **Duration:** ~55 min
- **Completed:** 2026-06-21
- **Tasks:** 4 (3 TDD, 1 docs)
- **Files modified:** 14 (3 created)

## Accomplishments
- `embedder_id` is an additive-minor open-provenance key on `result.v2` (ResultInput → resultDoc `omitempty` → BuildResult), mirroring the Outcome/TraceRef/ModelID keys; honest modes drop the key, baseline_rag records it. `schema_version` stays `v2` (additionalProperties OPEN).
- `subprocess.StartRAGServer` spawns `cmd/helix-bench-rag --corpus=<repoDir>` over its own stdio (D-06: no socket, no TCP port) and returns a `RAGHandle` whose `Pid()`/`Kill()` mirror the daemon handle plus the MCP `Stdin`/`Stdout` pipes the cell drives directly.
- `runRAGCell` is a real drive leg: out-of-band `ragindex.Open(repoDir)` BEFORE the `start := time.Now()` span anchor (budget exclusion), RAG-server spawn, MCP handshake + `rag_search` probe, the SAME verify/RunTests + coordinator grading contract as the daemon leg, and a result row with `EmbedderID` set and `Fairness = runners.DefaultContract`.
- The Phase-80 fail-close (`cell.go:429-433`) is replaced by `runRAGCell` (detected by mode name — no MODE.md frontmatter key).
- `baseline_rag` is an OPTIONAL delta operand: `full_minus_baseline_rag` is produced when its row is present (and written into its row too), omitted when absent without skipping the task.
- The deferral tests are flipped: `TestBaselineRagFailClose` removed; `baseline_rag_test.go` asserts a real schema-valid row with non-empty `embedder_id`, `model_id == DefaultContract.ModelID`, and null token metrics (embedding cost excluded); `five_of_six_test.go` now expects a real baseline_rag row + 4 deltas.
- MODE.md + BENCH.md rewritten from "fail-closed stub" to the real 4-tool arm with the same-model-same-budget invariant and embedding-cost exclusion.

## Task Commits
1. **Task 1: embedder_id open key** — `e731a98e` (test RED) → `b4786dd2` (feat GREEN)
2. **Task 2: real drive leg + StartRAGServer + flip deferral tests** — `38516c54` (test RED) → `8fe80180` (feat GREEN)
3. **Task 3: baseline_rag delta operand** — `4f3c9202` (test RED) → `5d90ff2a` (feat GREEN)
4. **Task 4: MODE.md + BENCH.md rewrite** — `0e3a52fa` (docs)

## Files Created/Modified
- `bench/runtime/result.go` (+`embedder_id`), `result_test.go` (omitted-when-empty / emitted-when-set).
- `bench/runtime/subprocess/ragserver.go` — `StartRAGServer`, `RAGHandle`, `ResolveRAGServerBin`, `HELIX_BENCH_RAG_BIN` override.
- `bench/runtime/rag.go` — `runRAGCell`, `driveRAGServer`.
- `bench/runtime/cell.go` — fail-close → `runRAGCell`; Deferred comment now generic.
- `bench/runtime/cell_test.go` — removed `TestBaselineRagFailClose`.
- `bench/runtime/baseline_rag_test.go` — `TestBaselineRagEmitsRow`, `TestBaselineRagSameContractBudget` (HELIX_BIN-gated, hermetic).
- `bench/runtime/five_of_six_test.go` — baseline_rag real row + 4 deltas.
- `bench/runtime/deltas.go` (+`deltas_test.go`) — `full_minus_baseline_rag` optional operand.
- `bench/runtime/matrix.go` — Deferred doc reconciled.
- `bench/runners/baseline_rag/MODE.md`, `bench/BENCH.md` — real-arm rewrite.
- `.gitignore` — `/helix-bench-rag` + `bench/runtime/.helix/` store artifact.

## Decisions Made
See `key-decisions` frontmatter. Headline: `StartRAGServer` returns a bench-owned `RAGHandle` (not `*evalsandbox.DaemonHandle`) because the RAG server is a stdio endpoint, not a socketed daemon; the Pid()/Kill() lifecycle is preserved. baseline_rag is an OPTIONAL operand so pre-existing 4-mode delta tests stay green. Hermetic tests force the stub via `HELIX_RAG_FORCE_STUB`.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] StartRAGServer returns RAGHandle, not *evalsandbox.DaemonHandle**
- **Found during:** Task 2
- **Issue:** The plan specified `StartRAGServer(...) (*evalsandbox.DaemonHandle, error)`. That is infeasible: (a) `DaemonHandle`'s fields (`cmd`/`taskID`/`mode`/`exited`) are unexported, so it cannot be constructed outside `internal/eval/sandbox`; and (b) the eval `StartDaemon` it would delegate to hard-codes the helix binary + a `--socket=` argv, whereas `cmd/helix-bench-rag` is a STDIO MCP server (StdioTransport) with no `--serve`/socket mode.
- **Fix:** Introduced a bench-owned `RAGHandle` in the `subprocess` package that owns the stdio-spawned bench-rag process, mirrors `DaemonHandle.Pid()`/`Kill()` (group SIGKILL via Setpgid, single owner Wait goroutine), and additionally exposes the `Stdin`/`Stdout` MCP pipes the cell drives directly (no forwarder).
- **Files modified:** bench/runtime/subprocess/ragserver.go, bench/runtime/rag.go
- **Verification:** `TestBaselineRagEmitsRow`/`TestBaselineRagSameContractBudget` pass HELIX_BIN-gated.
- **Committed in:** `8fe80180`.

**2. [Rule 1 - Bug] Hermetic cell tests must force the stub embedder**
- **Found during:** Task 2 (first HELIX_BIN run)
- **Issue:** With a live `OPENAI_API_KEY` in the dev env, `ragindex.Open` made a real embedding API call that 429'd (rate-limited), failing the cell tests non-deterministically.
- **Fix:** The cell tests set `HELIX_RAG_FORCE_STUB=1` + `HELIX_CACHE_DIR=t.TempDir()` so `selectEmbedder` takes the deterministic no-network stub regardless of key/Ollama; `StartRAGServer` forwards `HELIX_RAG_FORCE_STUB` so the server's warm-cache reopen re-supplies the SAME stub (consistent embedder_id, Pitfall 1).
- **Files modified:** bench/runtime/subprocess/ragserver.go, bench/runtime/baseline_rag_test.go, bench/runtime/five_of_six_test.go
- **Committed in:** `38516c54` (tests) / `8fe80180` (StartRAGServer forward).

**3. [Rule 3 - Blocking] five_of_six_test.go flipped (not in plan files_modified)**
- **Found during:** Task 2/3
- **Issue:** `five_of_six_test.go` directly asserted baseline_rag deferral (Deferred==true, no row, 3 deltas) — those assertions break once baseline_rag emits a real row and becomes a delta operand.
- **Fix:** Flipped to assert a real baseline_rag row with non-empty `embedder_id` (Task 2) and 4 deltas incl. `full_minus_baseline_rag` (Task 3); gated on the bench-rag binary, hermetic via the stub env.
- **Files modified:** bench/runtime/five_of_six_test.go
- **Committed in:** `38516c54` (row) / `5d90ff2a` (deltas).

**4. [Rule 2 - Hygiene] .gitignore the bench-rag binary + stray store artifact**
- **Found during:** Task 2
- **Issue:** Building `helix-bench-rag` left an untracked binary; a store-on cell test running with cwd=bench/runtime left an untracked `bench/runtime/.helix/semantic.duckdb`.
- **Fix:** Added `/helix-bench-rag` and `/bench/runtime/.helix/` to `.gitignore` (the latter is a pre-existing test side-effect, now ignored).
- **Committed in:** `8fe80180`.

---

**Total deviations:** 4 (2 Rule-3 blocking, 1 Rule-1 bug, 1 Rule-2 hygiene). None expand scope; all required to ship a working, hermetic, correctly-isolated real arm.

## Threat Model Compliance
- **T-83-03-01 (repudiation):** every baseline_rag row records `embedder_id` (asserted by `TestBaselineRagEmitsRow` + `five_of_six`); the secret `OPENAI_API_KEY` is never recorded — only the model string.
- **T-83-03-02 (fairness tampering):** `Fairness = runners.DefaultContract` VERBATIM; the unconditional `DefaultContract.Validate()` gate still runs before the RAG leg; `TestBaselineRagSameContractBudget` asserts `model_id == DefaultContract.ModelID`.
- **T-83-03-03 (budget skew):** the embedding index is built BEFORE the `start := time.Now()` span anchor; token metrics on the scripted RAG row are explicit null (no embedding-API tokens). Asserted by `TestBaselineRagSameContractBudget`.
- **T-83-03-04 (spawn path, accept):** `StartRAGServer` invokes the same trusted local bench-rag binary (resolved from `HELIX_BENCH_RAG_BIN` or the helix-bin sibling) over a per-cell stdio pipe with no TCP port; the binary's no-kernel/no-semantic isolation is enforced by `make vet`'s `vet-bench-rag-leakage` (confirmed green).

## TDD Gate Compliance
Tasks 1–3 followed RED → GREEN. Git log shows the gate commits in order: T1 `e731a98e`→`b4786dd2`, T2 `38516c54`→`8fe80180`, T3 `4f3c9202`→`5d90ff2a`. Task 2's HELIX_BIN-gated cell tests RED'd first on a live-key 429 (the stub fix made them GREEN); Task 3's delta-operand test RED'd against the 3-delta exclusion before the `full_minus_baseline_rag` operand made it GREEN.

## Known Stubs
- None new. The hermetic test path uses the Plan 01 `stub-deterministic` embedder (an INTENTIONAL, documented offline/CI fallback recorded with a distinct `embedder_id`); live/soak runs use OpenAI → Ollama. No production stub introduced by this plan.

## Verification
- `go build ./...` clean; `make build` produces `./helix`; `go build -o helix-bench-rag ./cmd/helix-bench-rag` clean.
- `go test ./bench/runtime/ -run 'TestEmbedderID|TestDelta'` green (pure).
- `HELIX_BIN="$(pwd)/helix" HELIX_BENCH_RAG_BIN="$(pwd)/helix-bench-rag" go test ./bench/runtime/...` green (the baseline_rag cell row + five-of-six tests actually RUN, not skip).
- `go test ./bench/runners/...` green (MODE.md still parses).
- `go vet ./bench/...` clean; `make vet` (incl. `vet-bench-rag-leakage`) EXIT 0.
- `go list -deps ./cmd/helix-bench-rag` exact-boundary check: NO `internal/kernel|semantic|mcp`.

## Self-Check: PASSED

All 3 created files exist on disk (bench/runtime/subprocess/ragserver.go, bench/runtime/rag.go, bench/runtime/baseline_rag_test.go). All 7 commits (e731a98e, b4786dd2, 38516c54, 8fe80180, 4f3c9202, 5d90ff2a, 0e3a52fa) are present in git history. `go build ./...`, `make vet`, and the HELIX_BIN-gated `go test ./bench/runtime/...` all pass.

---
*Phase: 83-cmd-helix-bench-rag-baseline-rag-mode-embedding-index-builde*
*Completed: 2026-06-21*

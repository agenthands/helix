# Phase 78: Internal ToolBench — Go First + LanguageRunner Interface - Context

**Gathered:** 2026-06-17
**Status:** Ready for planning

<domain>
## Phase Boundary

Build the **deterministic ground-truth corpus** that proves "Helix's tools work": **10 capability
test classes** (semantic view, LSP diagnostics, rename safety, fuzzy search, call graph, dependency
graph, patch apply, context minimization, incremental update, failure handling), **all 10 covered
on Go** with ≥ 1 deterministic fixture each under `bench/datasets/internal-toolbench/go/`, a
`CAPABILITIES.md` documenting the classes, a `bench/languages/go/runner.go` wrapping `go test
./... -json`, and a common **`LanguageRunner` interface** (`Detect`, `Setup`, `RunTests`,
`Capabilities`) that Phase 85 extends to 7 more languages. Task IDs move to the
`IT-go-<capability>-<n>` namespace with a `PHASE67_CROSSWALK.md`.

This phase grows the Phase 77 **seed** (`bench/datasets/toolbench-go/sum-doubler`, the single
scripted edit → `go test` task) into the full Go capability matrix, on top of the now-operational
Phase 77 runtime spine (`bench/runtime/` cell + matrix, scripted-agent CI gate, schema-valid
`result.v2.json`, 2-leg merged trace).

**Requirements locked here:** TOOLBENCH-01, TOOLBENCH-02, TOOLBENCH-10 (see
`.planning/REQUIREMENTS.md` lines 32–33, 41). WHAT/WHY are locked upstream in REQUIREMENTS.md and
v1.12-ROADMAP.md (4 success criteria); the decisions below are the HOW.

**Graded against (v1.12-ROADMAP Phase 78 success criteria):**
1. `bench/datasets/internal-toolbench/CAPABILITIES.md` documents the 10 capability classes; each
   has ≥ 1 deterministic test per supported language.
2. ToolBench-Go full run passes locally — all 10 capabilities have ≥ 1 Go fixture under
   `bench/datasets/internal-toolbench/go/`; `bench/languages/go/runner.go` wraps `go test ./...
   -json`; capability coverage reported per language.
3. `LanguageRunner` interface (`Detect`, `Setup`, `RunTests`, `Capabilities`) defined in
   `bench/languages/`; `go vet` + interface-conformance test passes for Go.
4. Task IDs use the new `IT-go-<capability>-<n>` namespace (zero collision with Phase 67 `T-67-*`);
   `bench/datasets/internal-toolbench/PHASE67_CROSSWALK.md` documents inspiration mapping.

**Explicitly OUT of scope (other phases):** the other 7 LanguageRunners + their fixtures (Phase
85); evaluators + the 17 result-schema metrics (Phase 79); container runtime / per-language images
(Phase 84); multi-run aggregation / pass@k (Phase 82).

</domain>

<decisions>
## Implementation Decisions

### Semantic backend per cell (the load-bearing decision)
- **D-01:** **Targeted semantic-index enablement.** The semantic index stays **disabled by
  default** per cell (the fast live-LSP / tree-sitter / RepoMap path, as Phase 77 cell.go does
  today via `semantic_index.enabled=false`). Store-dependent fixtures **opt in** per cell via a
  config knob. Rationale: 9/10 capabilities (semantic view, diagnostics, rename, fuzzy search, call
  graph, dependency graph, patch apply, context minimization, failure handling) can be exercised
  deterministically through live LSP / tree-sitter / RepoMap with **no persistent DuckDB store**;
  only **incremental update** structurally needs the Phase 70 overlay-drain refresh, which requires
  the store enabled. Targeted enablement contains the per-cell DuckDB/indexing latency (protects
  the ≤ 90 s `bench-quick` budget) and limits store/indexing-stability coupling, while still proving
  the differentiator where it matters.
- **D-02:** **Only the `incremental update` capability fixture(s) enable the persistent store.**
  Everything else runs store-off. (If, during planning, a call-graph / dependency-graph fixture is
  found to be deterministically un-testable without the store, it MAY opt in under the same knob —
  but the default rule is **store-on only when the capability cannot be deterministically exercised
  without it**.)
- **D-03:** **The store-enable mechanism is a per-cell daemon working-directory.** The blocker is
  that `internal/semantic/store` **rejects absolute store paths** (T-57-02-01) AND the store is
  opened **eagerly at daemon startup relative to the daemon CWD** (`daemon.go:308/318`), so with the
  cwd-relative default (`.helix/semantic.duckdb`) every parallel cell would open the SAME file and
  deadlock on its lock. The fix is to give the cell daemon a **private working directory** (the
  cell's isolated repo/HOME) so the cwd-relative store path is per-cell. This requires an **additive
  extension to eval's `StartDaemon`** to accept a working-dir (eval's current `StartDaemon` does not
  expose `cmd.Dir`, and bench's `subprocess.StartDaemon` delegates straight into it). This is
  **extending, not forking** — within P77 D-07. The cwd extension is gated behind the opt-in so
  store-off cells are unaffected. Planner: confirm the cleanest additive seam (extend eval
  `StartDaemon` signature vs a sandbox-level wrapper that sets `cmd.Dir`).

### Capability test shape (criteria #1, #2)
- **D-04:** **Uniform "solve-then-`go test`" fixtures.** Every capability is a task the scripted
  agent solves; the resulting **code state is verified by `go test`** (reuse the Phase 77 outcome
  path). There is **one** fixture shape, not two — no golden-output diffing mechanism this phase.
- **D-05:** **Read/query capabilities are engineered so correct completion REQUIRES consuming the
  tool's full output.** E.g. a call-graph fixture: "add a nil-check to every caller of `F`" — miss
  one caller and a `go test` case fails. The fixture's `scripted_agent.yaml` **explicitly calls the
  capability's tool** (e.g. `get_callers`, `get_symbols_overview`, `find_references`) as part of
  solving, so the capability is exercised **by construction** in the hermetic scripted CI gate. The
  "did it use the right tool" signal for the real-`claude` path comes from the merged trace (Phase
  79 `semantic_tool_calls` metric) — NOT graded here.
- **D-06:** **Capability is recorded as an explicit `capability` enum field in `task.json`.** The
  runner aggregates per-language coverage by reading the `capability` field across the corpus.
  `CAPABILITIES.md` is the human-facing documentation (criterion #1). The `IT-go-<capability>-<n>`
  task ID **mirrors** the field for readability but is **NOT** the source of truth (decouples ID
  string format from coverage logic; explicit + greppable).

### Corpus layout & naming (criterion #2; reconciling Phase 77)
- **D-07:** **Add a language axis to the matrix.** On-disk layout is
  `bench/datasets/internal-toolbench/<lang>/<task>` (honors criterion #2's literal
  `internal-toolbench/go/` path). Benchmark name is **`internal-toolbench`**; the `Cell` struct
  (`bench/runtime/matrix.go`) gains a **`Language`** field; `ExpandMatrix` + `validateMatrixID`
  extend to the language segment; `helix-bench run` gains a **`--languages`** flag (default `go`).
  The seed-dir join becomes `<DatasetsRoot>/<benchmark>/<lang>/<task>`. This is exactly the seam
  Phase 85 needs — it adds 7 languages by dropping fixtures + runners in, no matrix rewrite.
  (Constraint: benchmark/lang/task segments stay slash-free so the V5 path-traversal validators
  hold.)
- **D-08:** **Migrate the Phase 77 seed via `git mv`** from `bench/datasets/toolbench-go/sum-doubler`
  into `bench/datasets/internal-toolbench/go/<task>` (preserves git history; matches the Phase 64
  microbench-relocation pattern). Assign it an `IT-go-<capability>-<n>` ID and a `capability` tag
  (its `replace_in_file`-driven edit→`go test` shape maps to **patch apply** or **fuzzy search** —
  exact capability is Claude's discretion).
- **D-09:** **Flip the `toolbench-go` → `internal-toolbench` defaults THIS phase** in `Makefile`
  (`SUITE ?= toolbench-go` → `internal-toolbench`), `cmd/helix-bench/main.go` (`--benchmarks`
  default), and `bench/BENCH.md` (key-names) — plus the cell.go/matrix.go doc-comment examples.
  **Hard constraint: `make bench-quick` stays green throughout the cutover** (single clean cutover,
  no stale `toolbench-go` dir or defaults left behind). Update `bench/LICENSES.md` if its
  `internal-toolbench-go` row needs reconciling with the chosen benchmark name.

### LanguageRunner contract (criterion #3)
- **D-10:** **`RunTests` is the structured outcome source; `verify.sh` becomes a fallback.** The
  cell dispatches by language to a **registered `LanguageRunner`**; `RunTests` runs `go test ./...
  -json`, parses it, and returns a **structured pass/fail + per-test result** (strictly richer than
  an exit code — feeds Phase 79's `test_runner` evaluator + `compile_errors_before/after`). For
  `internal-toolbench` cells, `RunTests` **replaces** the Phase 77 `runVerify(verify.sh)` outcome
  path. `verify.sh` is **retired for internal-toolbench** but **preserved as an escape hatch**: the
  cell falls back to `verify.sh` when no `LanguageRunner` is registered for the benchmark/language
  (keeps the door open for Phase 85+ external adapters with custom verification). Constraint: the
  swap must keep `bench-quick` green.
- **D-11:** **`Capabilities()` returns a STATIC per-language declaration** — the set of capability
  classes the runner is DESIGNED to support (Go = all 10; Phase 85: Python/Rust/TS = 8, C#/C++ = 6).
  The coverage report cross-references this **declared** set against the **fixtures actually present**
  (`task.json` `capability` fields), so "declared-but-missing" gaps are explicit and the per-language
  acceptance thresholds (Go 10/10, Python ≥ 8/10, C# ≥ 6/10) have a real denominator.

### Claude's Discretion
- The exact `LanguageRunner` method **signatures** and the `RunTests` return type / `Capabilities`
  value type, and the `bench/languages/` package layout (D-10/D-11 fix the semantics, not the Go
  syntax). Keep minimal; it is the contract Phase 85 conforms to — design for additive conformance.
- The **per-capability fixture designs** (the 10 Go modules + their `scripted_agent.yaml` tool
  sequences and `go test` assertions), within the D-04/D-05 shape.
- The seed task's exact `capability` assignment and its `IT-go-*` index (D-08).
- **`PHASE67_CROSSWALK.md` scope**: an **inspiration-mapping doc only** (criterion #4 wording: "for
  migrating consumers") — it documents which Phase 67 `T-67-*` tasks inspired which `IT-go-*`
  fixtures. **No code migration of `T-67-*` tasks**; the namespaces are deliberately disjoint (zero
  collision).
- Whether the store opt-in knob lives in `task.json` (e.g. `semantic_index: true`) or is derived
  from the `capability` value (incremental-update ⇒ store-on) — pick the least-surprising during
  planning.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Milestone scope & requirements (locked WHAT/WHY — do not re-derive)
- `.planning/milestones/v1.12-ROADMAP.md` §"Phase 78" — Goal, Depends-on, Requirements, and the
  **4 Success Criteria** this phase is graded against. Also §"Phase 79" (downstream consumer of the
  Go test results), §"Phase 85" (the 7 languages that conform to `LanguageRunner`).
- `.planning/REQUIREMENTS.md` — **TOOLBENCH-01** (line 32: 10 capability classes + `CAPABILITIES.md`),
  **TOOLBENCH-02** (line 33: Go coverage, `go test ./... -json`), **TOOLBENCH-10** (line 41: the
  `LanguageRunner` interface + conformance test). Boundary rows: **TOOLBENCH-03..09** (lines 34–40 →
  **Phase 85**, the other languages + their per-language capability thresholds), **METRIC-01..06**
  (lines 56–61 → **Phase 79**, evaluators consume the Go test output).
- `.planning/ROADMAP.md` §"Phase 78" — active-roadmap entry (promoted from the milestone roadmap on
  2026-06-17 to unblock discuss/plan tooling).
- `.planning/research/PITFALLS.md` — milestone failure modes (daemon-tap PID cross-talk, trace-merge
  clock assumptions, fairness drift, contamination canary).

### Phase 77 runtime this phase builds on (reuse, do not reinvent)
- `bench/runtime/cell.go` — `RunCell` end-to-end spine; **note `writeCellConfig` (lines 446–458)**
  forces `semantic_index.enabled=false` and the cwd-lock comment (lines 432–445) that D-01/D-03
  resolve; `runVerify` (lines 394–413) is the verify.sh outcome path D-10 replaces.
- `bench/runtime/matrix.go` — `Cell` struct, `ExpandMatrix`, `RunMatrix`, `validateMatrixID`, and
  the `DatasetsRoot`/`<benchmark>/<task>` seed-dir join that D-07 extends with a language segment.
- `bench/runtime/subprocess/daemon.go` — `StartDaemon` delegates into the embedded eval sandbox's
  `StartDaemon`; the additive working-dir seam (D-03) threads through here.
- `bench/datasets/toolbench-go/sum-doubler/` — the seed to migrate (`task.json`,
  `scripted_agent.yaml`, `verify.sh`, `sum.go`, `sum_test.go`, `go.mod`) — D-08.
- `bench/runners/fairness_contract.go`, `bench/schema/result.v2.schema.json` — the result contract
  the Go runs still write into (unchanged from Phase 77; metric-sparse until Phase 79).
- `internal/eval/sandbox/sandbox.go` — `StartDaemon(ctx, taskID, mode, profileName, cfgPath)`; the
  signature D-03 extends with a per-cell working-dir.

### Helix tools the capability fixtures exercise (for fixture design — D-05)
- `internal/kernel/symbols/` — `goto_definition`, `find_references`, `get_callers`/`get_callees`,
  call/type hierarchy, blast radius (semantic view, call graph, rename-safety read side).
- `internal/kernel/edit/` — `rename_symbol`, `replace_symbol_body` (rename safety, patch apply).
- `internal/kernel/fileops/` — `replace_in_file`, `fuzzy_edit`, `search` (fuzzy search, patch apply).
- `internal/kernel/diag/` — `get_diagnostics` (LSP diagnostics).
- `internal/repomap/` + `internal/skill/repomap/` — `get_repo_map` / `get_context` (dependency
  graph, context minimization).
- `internal/semantic/` + the Phase 70 refresh overlay-drain — incremental update (store-backed).
- Phase 67 `internal/eval/` tasks (`T-67-*`) — inspiration source for `PHASE67_CROSSWALK.md`.

### Prior-phase context
- `.planning/phases/77-bench-runtime-first-e2e-smoke/77-CONTEXT.md` — the runtime decisions
  (D-01 scripted-gates-CI, D-04 metric-sparse result, D-06 unix-socket/no-TCP, D-07
  reuse-don't-fork sandbox) all carry forward.
- `.planning/phases/75-schema-fairness-contract-tree-skeleton/75-CONTEXT.md` — `bench/` skeleton,
  reuse-don't-fork + eval↔bench separation (D-08/D-15).
- `.planning/phases/76-ablation-profiles-kernel-subsystem-disable-flags/76-CONTEXT.md` — the
  `bench-*.yaml` profiles + locked filenames; `your_agent_full` → `bench-full`.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `bench/runtime/` (cell + matrix + subprocess + sandbox) — the full Phase 77 spine; Phase 78
  extends it (language axis, runner dispatch), does not rewrite it.
- The Phase 77 seed task shape (`task.json` + `scripted_agent.yaml` + `go test` verify) is the
  template every capability fixture follows (D-04).
- `internal/eval/runner/scripted_agent.go` + `runner.LoadScript` (strict KnownFields) — the
  `scripted_agent.yaml` format each fixture's tool sequence is written against.
- `bench/runners/DefaultContract` + `bench/schema` — result.v2 builder, unchanged.

### Established Patterns
- **Scripted agent gates CI; real `claude` wired-not-gating** (P77 D-01) — fixtures must be
  scripted-replayable and hermetic.
- **`git mv` for relocations** to preserve `git log --follow` history (Phase 64 microbench pattern)
  — D-08.
- **Reuse-don't-fork / eval↔bench separation** (P75 D-08/D-15, P77 D-07) — the eval `StartDaemon`
  working-dir change (D-03) is an additive EXTENSION, never a fork.
- **V5 path-traversal validation** (`validateMatrixID` / `validateCellKey`) — every new path
  segment (the language axis) must pass the same slash-free / no-parent-ref checks (D-07).

### Integration Points
- `bench/datasets/internal-toolbench/{CAPABILITIES.md,PHASE67_CROSSWALK.md,go/<task>/...}` (new
  corpus root) — D-06/D-07/D-08.
- `bench/languages/` (currently a `.gitkeep`) — the `LanguageRunner` interface + `go/runner.go`
  (D-10/D-11).
- `bench/runtime/matrix.go` — `Cell.Language`, `ExpandMatrix`, validators, the `<lang>` join (D-07).
- `bench/runtime/cell.go` — runner-dispatch + the store-opt-in config knob + verify.sh fallback
  (D-03/D-10).
- `bench/runtime/subprocess/daemon.go` + `internal/eval/sandbox/sandbox.go` — the working-dir
  extension (D-03).
- `cmd/helix-bench/main.go` — `--languages` flag + flipped `--benchmarks` default (D-07/D-09).
- `Makefile`, `bench/BENCH.md`, `bench/LICENSES.md` — `toolbench-go` → `internal-toolbench` cutover
  (D-09), `bench-quick` stays green.

</code_context>

<specifics>
## Specific Ideas

- Coverage report target shape: **per language, declared (via `Capabilities()`) vs covered (via
  `task.json` capability fields)** — Go must report **10/10** (criterion #2).
- The 10 capability classes are fixed by criterion #1: semantic view, LSP diagnostics, rename
  safety, fuzzy search, call graph, dependency graph, patch apply, context minimization, incremental
  update, failure handling.
- `IT-go-<capability>-<n>` IDs are deliberately disjoint from Phase 67 `T-67-*` (zero collision);
  the crosswalk is inspiration-only, not a code migration.
- The incremental-update fixture is the one place the persistent semantic store is enabled this
  phase — design it to actually drive the Phase 70 overlay-drain refresh, not a store-less proxy.

</specifics>

<deferred>
## Deferred Ideas

- **The other 7 LanguageRunners + their fixtures** (Python, TypeScript, JavaScript, Java, C#, C++,
  Rust) — **Phase 85**. Phase 78 ships Go only; the interface + matrix language axis are the seam
  they slot into.
- **Evaluators + the 17 result-schema metrics** (edit_locality, regression_rate, token sourcing,
  pass@k, etc.) — **Phase 79**, which consumes the structured `go test -json` output `RunTests`
  produces. Phase 78 leaves result.v2 metric-sparse.
- **Golden-output assertion mechanism** for pure-read capabilities — explicitly NOT built (D-04
  chose uniform solve-then-`go test`). Revisit only if a future capability genuinely cannot be
  outcome-graded.
- **Container runtime / per-language toolchain images / GHCR mirror** — Phases 84–88; out of the
  "Go first, no container" Phase 78 boundary.
- **Multi-run repetition / pass@k path threading** (`<task>/<mode>/<run_index>/`) — Phase 79/82
  (the P77 IN-05 single-rep-per-OutDir constraint still holds).
- **Relaxing the semantic store's absolute-path rejection** (T-57-02-01) as an alternative to the
  per-cell-cwd fix — a deeper kernel change, out of scope; D-03 uses the working-dir route instead.

None of the above expand Phase 78 scope — discussion stayed within the Go-corpus + LanguageRunner
boundary.

### Planner notes (apply during plan-phase)
1. **Phase 78 = the Go capability corpus + the LanguageRunner seam.** Resist building other
   languages (P85), metrics/evaluators (P79), or containers (P84).
2. **Targeted store enablement** (D-01/D-02): default store-off; only incremental-update opts in via
   the per-cell working-dir fix (D-03, additive eval `StartDaemon` extension). Confirm the cleanest
   additive seam first.
3. **Uniform solve-then-`go test`** (D-04/D-05): every fixture is a scripted-solvable task whose
   `go test` outcome is the grade; read capabilities force the agent to act on the tool's full
   output; capability tagged in `task.json` (D-06).
4. **Language axis** (D-07): `internal-toolbench/<lang>/<task>`, `Cell.Language`, `--languages`
   flag, V5-validated. **Migrate the seed via `git mv`** (D-08) and **flip defaults keeping
   `bench-quick` green** (D-09).
5. **`RunTests` replaces verify.sh as the structured outcome for internal-toolbench; verify.sh is
   the runner-less fallback** (D-10). `Capabilities()` is a static per-language declaration (D-11).
6. **Honest claims**: Go must hit 10/10 (criterion #2); the `LanguageRunner` conformance test +
   `go vet` must pass for Go (criterion #3); `PHASE67_CROSSWALK.md` is inspiration-only (criterion #4).

</deferred>

---

*Phase: 78-internal-toolbench-go-first-languagerunner-interface*
*Context gathered: 2026-06-17*

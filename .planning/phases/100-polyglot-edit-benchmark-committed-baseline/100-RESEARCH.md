# Phase 100: Polyglot Edit Benchmark + Committed Baseline - Research

**Researched:** 2026-06-23
**Domain:** Go bench harness integration — wiring the existing aider-polyglot `RunExercise` loader to helix EDIT verbs against the warm daemon, a filesystem-table bench mode, an additive `result.v2` open key, and a byte-reproducible HELIX_BIN-gated committed baseline.
**Confidence:** HIGH — every claim below is anchored to a direct in-tree read (`bench/datasets/aider-polyglot/loader.go`, `bench/runtime/{cell,result,drive,cctap,rag,matrix}.go`, `bench/runtime/subprocess/{daemon,claude}.go`, `bench/runners/mode_resolver.go`, `bench/aggregator/{aggregate,byte_reproducible_test}.go`, `Makefile`, `internal/cli/verbs_gen.go`, `internal/forwarder/{drive,oneshot}.go`). File:line anchors are given throughout.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
All implementation choices are at Claude's discretion — discuss phase was skipped per user setting (`workflow.skip_discuss`). Use ROADMAP phase goal, success criteria, and codebase conventions to guide decisions. The following constraints are carried (binding) from research + Phase 99:

- **Reuse `RunExercise` VERBATIM** (`bench/datasets/aider-polyglot/loader.go:230`) — the v2.1 delta is supplying an EDIT-verb `AgentFn` (in `bench/runtime`, daemon-dialing), NOT rebuilding the loader. The WR-01 anti-tamper pristine-test restore (`loader.go:177-184`, `loader.go:243-249`) MUST stay intact.
- **EDITBENCH-01:** the `AgentFn` routes the model's edit through the helix EDIT verbs (`replace-symbol-body` / `fuzzy-edit` / `replace-in-file` / `insert-before-symbol` / `insert-after-symbol`), dialed against the warm daemon (the CLI→gRPC path; reuse the Phase 77 subprocess daemon lifecycle in `bench/runtime`).
- **EDITBENCH-02:** new `bench/runners/aider_edit/MODE.md` via the filesystem-as-table pattern (Phase 80 grew 1→6 modes with ZERO mode-resolver Go change). Use the vendored fixtures from Phase 99 (offline) — `bench/datasets/aider-polyglot/fixtures/` (py/go/rust; java reserved/future).
- **EDITBENCH-03:** additive `edit_format_applied` (`*bool`, `omitempty`) open key on `result.v2.json` — NO `schema_version` v3 bump (mirror the `swebench_raw_resolved`/`embedder_id`/`language` additive-open-key precedent).
- **BASELINE-01 — the load-bearing determinism decision:** a committed baseline must be BYTE-REPRODUCIBLE, but a real LLM is non-deterministic. So the COMMITTED baseline is driven by a DETERMINISTIC/scripted agent (apply the exercism reference `files.example` through the helix EDIT verbs — a fixed, reproducible transformation), NOT a live model. The real-LLM (`--agent=claude`) arm is wired-but-NOT-the-baseline (mirror Phase 77's scripted `CCTapResult` synthesis + the wired-not-gating claude branch). Commit only deterministic metrics (pass/fail, edit_format_applied, file/edit counts) — EXCLUDE latency/tokens-of-a-live-model from the committed baseline (latency → local `bench-micro`).
- **HELIX_BIN fail-not-skip (cross-cutting, MANDATORY):** bench-surface tests must FAIL (not silently SKIP) when `HELIX_BIN` is set but no `result.v2.json` / empty bucket / missing metric is produced. Ship a hermetic golden sibling (no binary, no network) as the sole authoritative proof, plus a "did it RUN" sentinel when HELIX_BIN is set.
- **Determinism guards:** route reports through the existing deterministic `renderAll` / seed any RNG / sort-before-emit; the committed baseline artifact must regenerate byte-identically.
- Respect the `vet-ablation-leakage` leaf-import boundary: the daemon-dialing AgentFn lives in `bench/runtime` (NOT a stdlib leaf); the `aiderpolyglot` loader leaf stays stdlib-only.
- Benches stay local-only (no CI benchstat gate). A built helix binary is available at `/tmp/helix-v2.1-test`; the executor may rebuild a fresh one for HELIX_BIN.

### Claude's Discretion
All implementation choices (concrete file/function names, the exact deterministic-edit verb chosen, the single-exercise-single-mode baseline scope) are at Claude's discretion within the constraints above.

### Deferred Ideas (OUT OF SCOPE)
None — discuss phase skipped. (Roadmap defers: EDITBENCH-04 full 6-track/225-task expansion; REPOEVAL/FUZZBENCH/BASELINE-02 are Phase 102.)
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| EDITBENCH-01 | EDIT-verb `AgentFn` (in `bench/runtime`, daemon-dialing) routes the model's edit through helix edit verbs, plugged into `RunExercise` verb-agnostic seam — loader untouched, WR-01 preserved | Loader seam: `loader.go:204` (`AgentFn` type), `loader.go:230` (`RunExercise`). Daemon-dial precedent: `drive.go:67` (`forwarder.OpenSession` + `CallTool`). New file `bench/runtime/aider_edit_agent.go`. WR-01 at `loader.go:243-249`. §"Pattern 1/2", §"Code Examples". |
| EDITBENCH-02 | New `bench/runners/aider_edit/MODE.md` adds the polyglot-edit bench mode via filesystem-as-table with zero mode-resolver Go change | `mode_resolver.go:83` (`ResolveProfileFromRoot`) reads any `<mode>/MODE.md`; 6 existing MODE.md dirs prove zero-Go-change growth. §"Pattern 3". Mode detected by name in `RunCell` like `baselineRagMode` (`cell.go:434`). |
| EDITBENCH-03 | Additive `edit_format_applied` (`*bool`, `omitempty`) open key on `result.v2.json`, no `schema_version` v3 bump | `result.go:138-139` (`SwebenchRawResolved *bool` precedent) + `result.go:228-229` (json tag). Mirror EXACTLY. §"Pattern 4", §"Code Examples". |
| BASELINE-01 | Committed polyglot-edit baseline (`bench/reports/<run>/BENCH-RESULTS.md` + `result.v2.json`) HELIX_BIN-gated, fail-not-skip, byte-reproducible deterministic metrics only (latency excluded) | Deterministic agent applies `files.example` (`.meta/example.go`) via EDIT verbs. `renderAll` (`aggregate.go:193`) is the shared byte-stable render path; `byte_reproducible_test.go` is the proof pattern. §"Pattern 5/6", §"Validation Architecture". |
</phase_requirements>

## Summary

Phase 100 is a **wiring** phase, not a new-subsystem phase. Three substrates already exist and must be reused verbatim:

1. **The aider loader** (`bench/datasets/aider-polyglot/loader.go`) is a stdlib-only LEAF that already implements the upstream 2-attempt + stderr-reprompt protocol, the WR-01 pristine-test anti-tamper, and the native per-language test commands. It exposes a verb-agnostic `AgentFn` seam (`loader.go:204`) and a `TestFn` seam (`loader.go:200`). **It has NO live runner today** — it is exercised ONLY by `loader_test.go` (hermetic). Phase 100 supplies the missing live wiring.

2. **The Phase 77 cell spine** (`bench/runtime/cell.go`) owns sandbox → per-cell daemon spawn (`subprocess.StartDaemon`, Unix socket, HTTP off) → drive → kill → tap → `BuildResult`/`Validate` → durable write. The `baseline_rag` arm (`cell.go:434` → `runRAGCell` in `rag.go`) is the **exact precedent** for a mode-name-detected branch that diverges from the daemon-spine while reusing sandbox/build/write. Phase 100 adds a sibling `runAiderEditCell`.

3. **The additive open-key contract** (`result.go`) — `swebench_raw_resolved *bool` (`result.go:138-139`, `result.go:228-229`) is the byte-exact template for `edit_format_applied *bool`.

The critical design decision (BASELINE-01) resolves cleanly: the committed baseline is driven by a **deterministic scripted agent** that applies the exercism reference solution `files.example` (e.g. `.meta/example.go`, confirmed present in the vendored tree) through the helix EDIT verbs against the warm daemon — a fixed transformation that always produces the same pass/fail and `edit_format_applied`. The real-LLM `--agent=claude` arm is **wired but not the baseline** (mirror `cell.go:556-583`). Byte-reproducibility is guaranteed by routing the baseline report through the existing zero-RNG `renderAll` (`aggregate.go:193`) and committing only deterministic metrics (pass/fail, `edit_format_applied`, file/edit counts) — never live latency/tokens.

**Primary recommendation:** Add `runAiderEditCell` to `bench/runtime` (a `cell.go` mode-name branch like `baselineRagMode`) that: spawns the warm daemon, builds a deterministic EDIT-verb `AgentFn` (applies `files.example` via `replace-in-file` or `fuzzy-edit` dialing `forwarder.OpenSession`), calls `RunExercise` VERBATIM with that AgentFn + a real `TestFn` (native `pytest`/`cargo test --include-ignored`/`go test ./...`), stamps `edit_format_applied *bool` onto the result, and writes through the standard `BuildResult`/`Validate`/`writeDurable` path. Add `bench/runners/aider_edit/MODE.md` (profile `bench-full`). Capture ONE exercise × ONE mode for the committed baseline — small + hermetic-provable is sufficient.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| 2-attempt protocol + WR-01 anti-tamper | aider loader leaf (`loader.go`) | — | Already correct; REUSED VERBATIM. Stdlib-only, no daemon import. |
| EDIT-verb edit application (daemon dial) | `bench/runtime` AgentFn (`aider_edit_agent.go` NEW) | warm daemon (`subprocess.StartDaemon`) | Daemon-dialing is forbidden in the loader leaf (`vet-ablation-leakage`); lives in `bench/runtime`. |
| Native test execution (`pytest`/`cargo`/`go test`) | aider loader `nativeTestCommand` (`loader.go:280`) wrapped in a live `TestFn` | `bench/runtime` cell | Loader provides the argv table; runtime shells it under `attemptTimeout`. |
| Cell orchestration (sandbox/spawn/build/write) | `bench/runtime` cell (`cell.go` / new `runAiderEditCell`) | — | Mirror `runRAGCell`. |
| `edit_format_applied` provenance | `result.go` `ResultInput`/`resultDoc` (MODIFIED) | — | Additive open key, byte-compat. |
| Mode→profile binding | `bench/runners/aider_edit/MODE.md` (NEW, data) | `mode_resolver.go` (UNCHANGED) | Filesystem-as-table. |
| Deterministic baseline render | `bench/aggregator` `renderAll` (REUSED) | `bench/reports/<run>/` committed | Zero-RNG, sort-before-emit, byte-stable. |

## Standard Stack

This phase adds **NO new Go dependency**. Everything is in-tree reuse.

### Core (in-tree, reused)
| Component | Location | Purpose | Why Standard |
|-----------|----------|---------|--------------|
| `RunExercise` / `AgentFn` / `TestFn` | `bench/datasets/aider-polyglot/loader.go:230,204,200` | 2-attempt + WR-01 + native-test loader | Already correct; rebuilding risks regressing WR-01/WR-02 (`[VERIFIED: in-tree read]`) |
| `RunCell` + mode-name branch | `bench/runtime/cell.go:367,434` | Cell spine; `baseline_rag` branch precedent | `runRAGCell` is the template for `runAiderEditCell` `[VERIFIED]` |
| `subprocess.StartDaemon` | `bench/runtime/subprocess/daemon.go:40` | Per-cell warm daemon, Unix socket, HTTP off | Reused unchanged; the "warm daemon" target `[VERIFIED]` |
| `forwarder.OpenSession` + `session.CallTool` | `internal/forwarder` (used at `drive.go:73,108`) | Dial the daemon's gRPC `StreamMCP` wire in-process | The AgentFn calls EDIT verbs through this, NOT by shelling the binary `[VERIFIED]` |
| `BuildResult` / `Validate` / `writeDurable` | `bench/runtime/result.go:244`, `cell.go:322`(Validate),`923`(writeDurable) | result.v2 builder + schema gate + atomic write | The standard result path `[VERIFIED]` |
| `renderAll` | `bench/aggregator/aggregate.go:193` | SINGLE byte-stable render path (4 reports) | `report --run-id` and `Aggregate` share it; zero-RNG `[VERIFIED]` |
| `mode_resolver.ResolveProfileFromRoot` | `bench/runners/mode_resolver.go:83` | MODE.md frontmatter → profile, zero Go change | Filesystem-as-table; 6 modes already prove it `[VERIFIED]` |

### Supporting
| Component | Location | When to Use |
|-----------|----------|-------------|
| `SynthCCTap` | `bench/runtime/cctap.go:33` | Synthesize the CC (agent-tap) leg from scripted StepResults; zero Usage (no model). For the deterministic arm, record the EDIT-verb calls as steps. `[VERIFIED]` |
| `subprocess.StartClaude` | `bench/runtime/subprocess/claude.go:51` | The wired-not-gating `--agent=claude` arm; never the baseline. `[VERIFIED]` |
| `bench-full` profile | `internal/profile/profiles/bench-full.yaml` | The profile `aider_edit/MODE.md` resolves to (full edit-verb surface). `[VERIFIED]` |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `runAiderEditCell` branch in `cell.go` | A separate `cmd/helix-bench` subcommand | The mode-name branch reuses sandbox/daemon/build/write and the matrix wiring; a separate subcommand duplicates the spine. Mirror `runRAGCell`. |
| Deterministic agent applies `files.example` via `replace-in-file` (whole-file) | `fuzzy-edit` per-symbol | `replace-in-file` with the full reference body is the simplest byte-reproducible transform; `fuzzy-edit` exercises the cascade but adds drift. EDITBENCH-01 lists both — pick `replace-in-file` for the baseline, leave `fuzzy-edit`/`replace-symbol-body` reachable for richer arms. |
| AgentFn dials via `forwarder.OpenSession` (in-process) | Shell `helix replace-in-file` as a subprocess | `drive.go:67-73` already dials in-process over the retained gRPC `StreamMCP` wire (Phase 94 removed the stdio head). Reuse `OpenSession`; do NOT shell the binary per edit. |

**Installation:** None — no new packages. (No `npm`/`pip`/`cargo add`.)

## Package Legitimacy Audit

> Not applicable — this phase installs **no external packages**. All work is in-tree Go reuse against the existing module. No `go get`, no `npm install`, no new `require` directive. `git diff go.mod` must stay empty (Phase 99 verified this discipline; `99-01-SUMMARY.md:100`).

## Architecture Patterns

### System Architecture Diagram

```
helix-bench run --benchmarks=aider-polyglot --modes=aider_edit --agent=scripted --helix-bin=$HELIX_BIN
        │
        ▼
ExpandMatrix (matrix.go:119) ──► Cell{Benchmark:aider-polyglot, Language:go, Task:<ex>, Mode:aider_edit}
        │
        ▼
RunCell (cell.go:367)
   │ validate keys, compute durable paths, resolve profile (bench-full via MODE.md)
   │ fairness gate
   ▼
   if Mode == "aider_edit":  ──► runAiderEditCell   ★ NEW (mirrors runRAGCell, rag.go:57)
        │
        ├─ benchsandbox.New + Prepare + clone the vendored exercise dir into scratch repo
        ├─ subprocess.StartDaemon (Unix socket, HTTP off) ── WARM DAEMON
        │
        ├─ load Exercise:  aiderpolyglot.LoadExercise(<fixtures>/<lang>/.../<ex>, lang)   [loader, leaf]
        │
        ├─ build deterministic EDIT-verb AgentFn  ★ NEW (aider_edit_agent.go, bench/runtime)
        │     for each files.solution stub:
        │       read reference body from files.example (.meta/example.<ext>)
        │       forwarder.OpenSession(sockPath) ──► session.CallTool("replace_in_file"|"fuzzy_edit", {...})
        │       record EDIT verb call as a StepResult; set edit_format_applied
        │
        ├─ build real TestFn:  shell nativeTestCommand(lang) under attemptTimeout   ★ NEW (thin)
        │
        ▼
   aiderpolyglot.RunExercise(ctx, ex, workDir, realTestFn, editVerbAgentFn)   ◆ REUSED VERBATIM
        │   attempt i: AgentFn edits stub ──► restorePristineTests (WR-01) ──► native tests
        │   on pass: break;  on fail: re-prompt attempt 2 with captured output
        ▼
   AttemptResult{Passed, Attempts, LastOutput}
        │
        ├─ kill daemon, PID-gated tap (reuse cell.go:603-622 pattern)
        ├─ SynthCCTap(editVerbSteps)  ──► 2-leg Merge
        ▼
   BuildResult(ResultInput{ ..., EditFormatApplied:&applied, Language:lang })  ◆ +edit_format_applied
        │   Validate (schema gate) ──► writeDurable <out>/<task>/<mode>/<idx>/result.v2.json
        ▼
   bench/aggregator renderAll ──► bench/reports/<run>/BENCH-RESULTS.md  (committed, byte-stable)
```

### Component Responsibilities (delta)

| File | Status | What |
|------|--------|------|
| `bench/datasets/aider-polyglot/loader.go` | **REUSED VERBATIM** | `RunExercise`, `AgentFn`/`TestFn`, WR-01 restore, `nativeTestCommand`. Do NOT touch. May need to **export** a `LoadExercise` wrapper (currently `loadExercise` is unexported, `loader.go:96`) — see Open Question 1. |
| `bench/runtime/aider_edit_agent.go` | **NEW** | The deterministic EDIT-verb `AgentFn` (daemon-dialing). Applies `files.example` → `files.solution` via `replace_in_file`/`fuzzy_edit`. Lives in `bench/runtime` (daemon-dial allowed). |
| `bench/runtime/aider_edit_cell.go` (or add to `cell.go`) | **NEW** | `runAiderEditCell` — the mode-name branch mirroring `runRAGCell`. |
| `bench/runtime/cell.go` | **MODIFIED** | Add `if cfg.Mode == aiderEditMode { return runAiderEditCell(...) }` near `cell.go:434` (the `baselineRagMode` branch). Add `EditFormatApplied` to the `BuildResult` call. |
| `bench/runtime/result.go` | **MODIFIED** | Add `EditFormatApplied *bool` to `ResultInput` + `EditFormatApplied *bool json:"edit_format_applied,omitempty"` to `resultDoc`; wire through `BuildResult`. Mirror `result.go:138-139,228-229`. |
| `bench/runners/aider_edit/MODE.md` | **NEW (data)** | `mode: aider_edit` / `profile: bench-full`. Zero Go change. |
| `bench/reports/<run>/` | **NEW (committed)** | The byte-reproducible baseline artifact (`result.v2.json` + `BENCH-RESULTS.md`). |
| `Makefile` | **MODIFIED (optional)** | A `bench-aider-edit` or extended `bench-quick` target driving the deterministic arm HELIX_BIN-gated. |

### Pattern 1: Verb-agnostic `AgentFn` seam in `RunExercise` (reuse, do NOT touch loader)

**What:** `RunExercise(ctx, ex, workDir, runTests TestFn, agent AgentFn)` (`loader.go:230`). `AgentFn` is `func(ctx, ex, workDir, prompt) error` (`loader.go:204`) — "whatever drives the edit." The loader already does the 2-attempt loop, the WR-01 pristine-test restore (`loader.go:243-249`), and re-prompts attempt 2 with captured failure output (`loader.go:261`).
**When to use:** Supply an `AgentFn` that routes the edit through helix EDIT verbs. The loader stays verb-agnostic and untouched.
**Anti-pattern:** Moving `restorePristineTests` into the AgentFn, or having the AgentFn write the test file. WR-01 restoration runs AFTER the agent edits but BEFORE grading (`loader.go:241-249`), precisely so a test-tampering agent cannot force a green.

### Pattern 2: Daemon-dialing AgentFn via `forwarder.OpenSession` (in-process gRPC, not shelling)

**What:** `drive.go:73` opens ONE MCP session against the per-cell Unix socket via `forwarder.OpenSession(ctx, sockPath, "", logger, "bench-runtime")`, issues an `activate_project` call (`drive.go:87`), then one `session.CallTool(stepCtx, tool, args)` per step (`drive.go:108`). Phase 94 removed the stdio forwarder head; this dials the retained gRPC `StreamMCP` wire directly in-process (`drive.go:29-37`).
**When to use:** The EDIT-verb AgentFn reuses this exact pattern — `OpenSession` → `activate_project` (point the daemon at the cloned exercise repo) → `CallTool("replace_in_file", {...})`. The verb's MCP tool name is the underscore form (`replace_in_file`, `fuzzy_edit`, `replace_symbol_body`, `insert_before_symbol`, `insert_after_symbol`) — confirmed in `internal/cli/verbs_gen.go:108-110,272,282,362,374-376`.
**Trade-off:** This is daemon-dialing → it MUST live in `bench/runtime`, NOT in the `aiderpolyglot` loader leaf (stdlib-only; `loader.go:11-15`). The `vet-ablation-leakage` boundary forbids the leaf from importing the daemon/forwarder.

### Pattern 3: Filesystem-as-table mode (`bench/runners/aider_edit/MODE.md`, zero Go change)

**What:** `ResolveProfileFromRoot(root, mode)` (`mode_resolver.go:83`) reads `bench/runners/<mode>/MODE.md` YAML frontmatter (`mode:` + `profile:`, strict `KnownFields(true)`, `mode_resolver.go:35-38,119`). Six MODE.md dirs already exist; adding a seventh needs NO resolver change.
**When to use:** Drop in `bench/runners/aider_edit/MODE.md` with `mode: aider_edit` / `profile: bench-full`.
**Trade-off:** The `aider_edit` arm is *also* detected BY MODE NAME in `RunCell` (like `baselineRagMode` at `cell.go:434`, `rag.go:37`) — the MODE.md still resolves (validates the frontmatter) but the cell branches before the daemon-spine. Detection is by mode name, NOT a MODE.md frontmatter key (the resolver is strict two-key).

### Pattern 4: Additive open `result.v2` key (`*bool`, no schema v3 bump)

**What:** `result.go` carries open provenance keys (`embedder_id`, `language`, `container_id`, `swebench_raw_resolved`) as `omitempty`; **booleans use `*bool`** so a literal `false` survives marshalling (a value-type `omitempty` would drop a load-bearing `false`). `schema_version` stays `"v2"`; `additionalProperties` stays OPEN; the key is NOT added to `required`.
**When to use:** `edit_format_applied *bool` mirrors `swebench_raw_resolved` EXACTLY (`result.go:138-139` field, `result.go:228-229` json tag).
**Trade-off:** None structurally — this is the project's established additive contract (`result.go:39-42` documents the pinned-key precedent).

### Pattern 5: Deterministic scripted agent (the committed-baseline driver)

**What:** Phase 77's scripted path drives the cell with NO model — `SynthCCTap` (`cctap.go:33`) builds the agent-tap leg from recorded `StepResult`s with **zero Usage** (tokens legitimately 0, never fabricated, `cctap.go:26-27,82`). The deterministic aider-edit baseline does the same: the AgentFn reads the exercism reference solution `files.example` (e.g. `.meta/example.go` — **confirmed present in the vendored tree**, `files.example: [".meta/example.go"]`) and applies it to the stub via a fixed EDIT verb, producing a deterministic pass/fail and `edit_format_applied`.
**When to use:** The committed baseline (`--agent=scripted`, the default). The reference solution is the ground-truth correct edit, so a correctly-wired EDIT verb yields a deterministic PASS.
**Trade-off:** This proves "the EDIT-verb path applies the reference solution and the native tests pass" — a real integration smoke — NOT "an LLM solved it." That is the correct thing to commit (deterministic, reproducible). The LLM arm is separate (Pattern below).

### Pattern 6: Wired-not-gating `--agent=claude` arm (NOT the baseline)

**What:** `cell.go:556-583` (the `case "claude"` branch) spawns the real claude CLI via `subprocess.StartClaude` (`claude.go:51`) against the same per-cell daemon socket. It is reachable ONLY via `--agent=claude`, NEVER the CI/baseline default (`claude.go:1-5`).
**When to use:** Provide the same branch for the aider-edit cell so a maintainer CAN drive a real model locally — but the committed baseline is the scripted arm. A missing `claude` binary surfaces as `subprocess.ErrClaudeNotFound` (`claude.go:31`); it never fails the scripted gate.

### Anti-Patterns to Avoid
- **Rebuilding the loader.** `RunExercise` + WR-01 + WR-02 (`--include-ignored`) already exist and are correct (`loader.go:286-292`). Supply the AgentFn only.
- **Daemon-dialing from the loader leaf.** Breaks `vet-ablation-leakage`. The AgentFn lives in `bench/runtime`.
- **`schema_version` v3 bump for `edit_format_applied`.** Use the additive open key (`result.go` precedent).
- **Value-type `bool` for `edit_format_applied`.** A literal `false` (edit-format NOT applied — a load-bearing signal) would be dropped by `omitempty`. Use `*bool` (`result.go:138` comment).
- **Committing live latency/tokens.** Machine-specific → non-reproducible baseline. Commit only deterministic quality metrics; latency → `make bench-micro` (`Makefile:118-127`).
- **`t.Skip` without a hermetic golden sibling.** The project's most-repeated false-green (Pitfall 2 below).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| 2-attempt + reprompt + anti-tamper | A new attempt loop | `RunExercise` (`loader.go:230`) | WR-01/WR-02 already correct; rebuilding regresses them |
| Per-cell warm daemon over a socket | A new spawn | `subprocess.StartDaemon` (`daemon.go:40`) | HTTP-off Unix-socket invariant already enforced |
| Dialing the daemon to call a verb | Shelling `helix <verb>` per edit | `forwarder.OpenSession`+`CallTool` (`drive.go:73,108`) | In-process gRPC; no per-edit process spawn |
| result.v2 build + schema gate + atomic write | New JSON marshal | `BuildResult`/`Validate`/`writeDurable` (`result.go:244`, `cell.go:923`) | Validate-on-write + atomic temp+rename |
| Byte-stable report render | A new markdown emitter | `renderAll` (`aggregate.go:193`) | Zero-RNG, sort-before-emit, double-render-proven |
| Mode→profile mapping | A Go map | `bench/runners/<mode>/MODE.md` (`mode_resolver.go:83`) | Filesystem-as-table, zero Go change |
| Native test argv per language | A new switch | `nativeTestCommand` (`loader.go:280`) | Includes the WR-02 `cargo test -- --include-ignored` guard |

**Key insight:** Every piece of this phase already exists except (1) the daemon-dialing EDIT-verb `AgentFn`, (2) a thin live `TestFn` that shells `nativeTestCommand`, (3) the `runAiderEditCell` branch, (4) the `edit_format_applied` field, (5) the `MODE.md`, and (6) the committed baseline artifact. Custom solutions for anything else are a regression risk.

## Runtime State Inventory

> Not a rename/refactor/migration phase — this is additive bench wiring. No stored data, live-service config, OS-registered state, secrets, or build artifacts carry a renamed string. **None — verified by:** the phase only ADDS new files (`aider_edit_agent.go`, `aider_edit_cell.go`, `MODE.md`, baseline reports) and adds one additive field to `result.go`; it renames nothing and migrates no datastore. The committed `bench/reports/<run>/` artifact is new content, not a migration of an existing baseline. `git diff go.mod` stays empty.

## Common Pitfalls

### Pitfall 1: HELIX_BIN false-green — the aider-edit surface SKIPs silently
**What goes wrong:** The new runner's only test is `if os.Getenv("HELIX_BIN")=="" { t.Skip }`, so `go test ./bench/...` reports green while never exercising the new code. This is the project's single most-repeated trap (`PITFALLS.md` Pitfall 2; project memory `helix-bench-smoke-false-green`).
**Why it happens:** `t.Skip` is idiomatic; CI lacks the built binary unless built first.
**How to avoid:** Ship a **hermetic golden sibling** (the SOLE authoritative proof) that runs the deterministic AgentFn + `RunExercise` against a hermetic in-memory/fake `TestFn` with NO `HELIX_BIN` and NO network — exactly the shape of `loader_test.go` (which already drives `RunExercise` with a scripted fake tester, `loader.go:228-229` notes the empty-`SrcDir` fake-tester path). The live HELIX_BIN leg may skip; its logic is covered by the sibling. Add a "did it RUN" sentinel test that, when `HELIX_BIN` IS set, asserts the live cell actually produced a `result.v2.json` (Phase 81 precedent `TestNoSemanticReadsTotalLineEmitted`).
**Warning signs:** `go test ./bench/...` passes in seconds with no `HELIX_BIN` and you can't name a non-skipped test that touched `aider_edit_agent.go`.

### Pitfall 2: Fail-OPEN on a missing/empty result (vacuous baseline pass)
**What goes wrong:** The HELIX_BIN leg runs but produces NO `result.v2.json` (daemon failed to dial, empty bucket), and the test reads the absence as a zero/pass — the Phase 82 CR-01 "N-gate failed OPEN on empty run dir" and Phase 81 WR-02 "missing line read as count=0" class.
**How to avoid:** Fail-CLOSED — when the live leg runs, a missing `result.v2.json`, an empty run dir, or a missing `edit_format_applied` metric line MUST be a hard error, never a pass (mirror `scrapeSemanticReadsTotal`'s `present` fail-closed signal, `cell.go:99-141,160-179`). For the baseline, assert the committed `result.v2.json` exists AND carries the expected `outcome` + `edit_format_applied`.

### Pitfall 3: Non-reproducible committed baseline (latency/timestamps/map-order)
**What goes wrong:** The baseline bakes in wall-clock latency, absolute paths, a hostname, or a map-iteration-ordered field (Phase 80 WR-03 `fairness.overrides[]` ordering) → a re-run diffs and every check sees a spurious regression.
**How to avoid:** Route the baseline report through `renderAll` (`aggregate.go:193`) which is zero-RNG and sort-before-emit (`aggregate.go:189-191`, `fairnessBlock` sorts overrides, `result.go:303-313`). Commit ONLY deterministic metrics (pass/fail, `edit_format_applied`, file/edit counts). Exclude latency (`bench-micro` owns it, `Makefile:118-127`). Ship a double-render byte-reproducibility test mirroring `byte_reproducible_test.go:25-54`.
**Warning signs:** Re-running the baseline produces a non-empty diff; the artifact contains a timestamp, absolute path, or millisecond latency.

### Pitfall 4: WR-01 anti-tamper bypassed by the new AgentFn
**What goes wrong:** The EDIT-verb AgentFn (or the live TestFn) writes the test file, or the cell skips `restorePristineTests`, letting a future edit-format change force a spurious green.
**How to avoid:** Call `RunExercise` VERBATIM — it restores the pristine test AFTER the agent edits and BEFORE each grade (`loader.go:241-249`), but ONLY when `ex.SrcDir != ""`. Ensure `LoadExercise` sets `SrcDir` (it does, `loader.go:120`) so the restore actually runs on the real path. The AgentFn must edit ONLY `files.solution` stubs, never `files.test`.

### Pitfall 5: Rust/Java/JavaScript flagged non-hermetic (offline dep resolution)
**What goes wrong:** Picking a rust exercise for the baseline → `flagNonHermetic` marks it non-hermetic (`loader.go:313-326`) because `cargo test` fetches crates unless an offline cache is baked in; the committed-baseline run then needs network and isn't reproducible offline.
**How to avoid:** For the single committed-baseline exercise, prefer a **python or go** exercise (both are hermetic with the committed stubs, `loader.go:321-324`). Go's `go test ./...` and python's `pytest` resolve offline against the vendored fixtures. Reserve rust/java for the (deferred) full expansion.

## Code Examples

### `edit_format_applied` additive open key (mirror `swebench_raw_resolved`)
```go
// Source: bench/runtime/result.go:138-139 (ResultInput field) — mirror EXACTLY.
// In ResultInput:
//   EditFormatApplied is the Phase 100 (EDITBENCH-03) open-provenance key recording
//   whether the model's edit was applied through a helix EDIT verb in the dataset's
//   expected edit format. *bool (NOT bool) WITH omitempty so a nil drops the key but a
//   literal false (edit-format NOT applied — load-bearing) is PRESERVED. Mirrors the
//   SwebenchRawResolved additive-minor discipline: omitempty, schema_version stays "v2",
//   additionalProperties stays OPEN, NOT added to required.
EditFormatApplied *bool

// Source: bench/runtime/result.go:228-229 (resultDoc json tag) — mirror EXACTLY.
EditFormatApplied *bool `json:"edit_format_applied,omitempty"`

// In BuildResult (result.go:264-287), add to the resultDoc literal:
//   EditFormatApplied: in.EditFormatApplied,
```

### Deterministic EDIT-verb AgentFn (NEW, bench/runtime/aider_edit_agent.go)
```go
// Source pattern: bench/runtime/drive.go:67-124 (OpenSession + activate_project + CallTool)
//              +  bench/datasets/aider-polyglot/loader.go:204 (AgentFn signature)
// Lives in bench/runtime (daemon-dialing allowed); NOT in the aiderpolyglot leaf.

func newDeterministicEditAgent(sockPath string, applied *bool) aiderpolyglot.AgentFn {
    return func(ctx context.Context, ex *aiderpolyglot.Exercise, workDir, _ string) error {
        sess, err := forwarder.OpenSession(ctx, sockPath, "", discardLogger(), "aider-edit")
        if err != nil { return fmt.Errorf("aider-edit: open session: %w", err) }
        defer sess.Close()
        // Point the daemon workspace at the cloned exercise repo (drive.go:87).
        if _, err := sess.CallTool(ctx, "activate_project", map[string]any{"repo_path": workDir}); err != nil {
            return fmt.Errorf("aider-edit: activate: %w", err)
        }
        // Deterministic transform: apply the reference solution (files.example,
        // e.g. .meta/example.go) to each solution stub via a fixed EDIT verb.
        for i, stub := range ex.Config.Files.Solution {
            ref := ex.Config.Files.Example[i]               // ".meta/example.<ext>"
            body, err := os.ReadFile(filepath.Join(ex.SrcDir, ref))
            if err != nil { return fmt.Errorf("aider-edit: read reference %q: %w", ref, err) }
            // replace_in_file: deterministic whole-file replacement = byte-reproducible.
            res, err := sess.CallTool(ctx, "replace_in_file", map[string]any{
                "relpath": stub, "content": string(body),  // arg names per the tool schema
            })
            if err != nil { return fmt.Errorf("aider-edit: replace_in_file %q: %w", stub, err) }
            if res != nil && res.IsError { *applied = false; return fmt.Errorf("aider-edit: verb error") }
        }
        t := true; *applied = t   // edit_format_applied = true (verb succeeded)
        return nil
    }
}
```
> NOTE: the exact `replace_in_file` argument keys must be read from the tool's InputSchema in `internal/kernel/fileops/` at plan time — the keys above are illustrative. Confirm against the live schema before coding `[ASSUMED: arg key names]`.

### Live TestFn wrapping nativeTestCommand (NEW, thin)
```go
// Source: bench/datasets/aider-polyglot/loader.go:280 (nativeTestCommand) +
//         bench/runtime/cell.go:821 (runVerify exec pattern under ctx timeout).
func newNativeTestFn() aiderpolyglot.TestFn {
    return func(ctx context.Context, ex *aiderpolyglot.Exercise, workDir string) aiderpolyglot.TestResult {
        argv, err := aiderpolyglot.NativeTestCommand(ex.Language) // export needed (Open Q1)
        if err != nil { return aiderpolyglot.TestResult{Passed: false, Output: err.Error()} }
        cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
        cmd.Dir = workDir
        out, runErr := cmd.CombinedOutput()
        return aiderpolyglot.TestResult{Passed: runErr == nil, Output: string(out)}
    }
}
```

### MODE.md (NEW data, bench/runners/aider_edit/MODE.md)
```markdown
---
mode: aider_edit
profile: bench-full
---

# aider_edit

The polyglot-edit bench mode (EDITBENCH-02). Resolves to the `bench-full`
profile — the full helix EDIT-verb surface. The model's edit is routed through
helix edit verbs against the warm daemon via the reused RunExercise loader.
Detected by mode name in RunCell (like baseline_rag) so the cell branches to
runAiderEditCell; the MODE.md still validates the two-key frontmatter.
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| aider loader is dataset-loader-only (no live runner) | Phase 100 supplies the live AgentFn + cell branch | This phase | `RunExercise` finally driven end-to-end |
| stdio MCP forwarder head (shell `helix --mode=stdio`) | gRPC `StreamMCP` wire dialed in-process via `forwarder.OpenSession` | Phase 94 (`drive.go:29-37`) | The AgentFn dials in-process, never shells a forwarder |
| `result.v2` keys added via schema bump | additive open keys (`*bool`/`omitempty`), schema stays v2 | Phase 83/85/87 | `edit_format_applied` follows the same contract |

**Deprecated/outdated:**
- The stdio forwarder head — removed Phase 94. Do NOT spawn `helix --mode=stdio`; use `forwarder.OpenSession` (`drive.go:29-37`).
- The Phase 80 `ablation_status: "pending"` deferral marker — removed (`cell.go:776-781`). The aider-edit arm needs no deferral marker.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `replace_in_file` arg keys are `relpath`/`content` | Code Examples | LOW — read the live InputSchema in `internal/kernel/fileops/` at plan time; wrong keys = verb error caught fail-closed |
| A2 | `loadExercise`/`nativeTestCommand` need exporting (currently unexported, `loader.go:96,280`) | Open Q1 | LOW — if a live caller already exists elsewhere they may be exported; verified NO live caller exists (`grep RunExercise`), so an exported wrapper in the loader package is likely needed |
| A3 | A python or go exercise is the right baseline pick (hermetic) | Pitfall 5 | LOW — `flagNonHermetic` (`loader.go:321-324`) confirms py/go/cpp are hermetic; rust/java/js are not |
| A4 | `forwarder.OpenSession` signature is `(ctx, sockPath, tcpAddr, logger, name)` | Code Examples | LOW — taken verbatim from `drive.go:73`; stable |

**Note:** No `[ASSUMED]` claim touches compliance, retention, security standards, or performance targets. All assumptions are mechanical (arg keys, export visibility) and verifiable at plan/code time.

## Open Questions

1. **Exporting loader internals for the live runner.**
   - What we know: `RunExercise`, `AgentFn`, `TestFn`, `TestResult`, `Exercise`, `Config`, `AttemptResult` are EXPORTED (`loader.go:204,200,188,58,40,207,230`). But `loadExercise` (`loader.go:96`) and `nativeTestCommand` (`loader.go:280`) are UNEXPORTED.
   - What's unclear: the live `runAiderEditCell` (in `bench/runtime`) needs to load an exercise from a fixture dir and get the native test argv. Today only `loader_test.go` (same package) can call them.
   - Recommendation: add a thin EXPORTED wrapper in the loader package (e.g. `func LoadExercise(dir, language string) (*Exercise, error)` delegating to `loadExercise`, and `func NativeTestCommand(language string) ([]string, error)`). This keeps the daemon-dial logic OUT of the leaf (only adds exported pure-stdlib accessors) and preserves the leaf-import boundary. Confirm with `go vet` + `vet-ablation-leakage` that no daemon import leaks in.

2. **Single exercise × single mode sufficiency for the committed baseline.**
   - What we know: BASELINE-01 wants "small + hermetic-provable." The deterministic agent applying the reference solution yields a guaranteed PASS for a correctly-wired verb.
   - What's unclear: whether the baseline should span >1 exercise.
   - Recommendation: **ONE exercise × ONE mode (`aider_edit`) × one language (go or python) is sufficient** for the committed baseline. It proves the full path (load → daemon → EDIT verb → native test → result.v2 → render) byte-reproducibly. The full 6-track/225-task expansion is explicitly deferred (EDITBENCH-04). Keep the committed tree tiny and auditable (Phase 99 vendored a 9-exercise subset; pick one, e.g. `go/wordy` or `python/wordy`).

3. **Where the `aider-polyglot` benchmark plugs into the matrix.**
   - What we know: `ExpandMatrix` (`matrix.go:119`) builds cells from `<datasets>/<benchmark>/<language>/<task>` seed dirs and `cellSeedDir` (`matrix.go:246`) joins that path; the standard cell expects `scripted_agent.yaml`+`verify.sh`. The aider fixtures live at `bench/datasets/aider-polyglot/fixtures/<lang>/exercises/practice/<ex>` — a DIFFERENT layout.
   - What's unclear: whether to (a) make `runAiderEditCell` resolve the fixture path from the benchmark name directly (bypassing `cellSeedDir`'s seed-dir assumptions), or (b) adapt the matrix.
   - Recommendation: **(a)** — like `runRAGCell`, the aider-edit branch owns its own path resolution to the fixtures tree; the standard `cellSeedDir`/`task.json` assumptions don't apply. Detect `Mode == aider_edit` (or `Benchmark == aider-polyglot`) early in `RunCell` and branch before the seed-dir clone. Decide benchmark-vs-mode keying at plan time; mode-name keying matches the `baseline_rag` precedent.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| `helix` binary (HELIX_BIN) | Live aider-edit cell (daemon spawn) | ✓ (buildable) | v2.1 | Hermetic golden sibling (no binary) is the sole authoritative proof |
| Go toolchain (`go test ./...`) | Native test for a go exercise | ✓ | host | Pick python exercise (`pytest`) |
| `pytest` | Native test for a python exercise | likely ✓ | — | Pick go exercise (`go test`) — always present |
| Vendored fixtures (Phase 99) | Exercise load + reference solution | ✓ | `@7e0611e` | None — committed offline (`99-01-SUMMARY.md:12`) |
| `claude` CLI | `--agent=claude` wired-not-gating arm | ✗ (optional) | — | `ErrClaudeNotFound`; never the baseline |

**Missing dependencies with no fallback:** None. The committed baseline is a go (or python) hermetic exercise; the helix binary is buildable; fixtures are vendored offline.
**Missing dependencies with fallback:** `claude` CLI (optional, never gating); `pytest` (use a go exercise instead).

## Validation Architecture

> `workflow.nyquist_validation` is not explicitly false → section included.

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go `testing` + `testify` (`require`/`assert`) |
| Config file | none — standard `go test` |
| Quick run command | `go test ./bench/runtime/... ./bench/datasets/aider-polyglot/...` |
| Full suite command | `go test ./...` then `go test -tags integration ./bench/...` (note: integration-tagged suite has pre-existing env failures — see project memory) |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| EDITBENCH-01 | Deterministic EDIT-verb AgentFn applies `files.example` via a helix verb; WR-01 preserved | unit (hermetic, no binary) | `go test ./bench/runtime/ -run TestAiderEditAgentHermetic -x` | ❌ Wave 0 |
| EDITBENCH-01 | `RunExercise` reused verbatim, WR-01 restore still runs (test-tamper → still fails) | unit (revert-and-fail) | `go test ./bench/datasets/aider-polyglot/ -run TestRunExerciseAntiTamper -x` | partial (`loader_test.go` exists; add aider-edit-specific) |
| EDITBENCH-02 | `bench/runners/aider_edit/MODE.md` resolves to `bench-full`, zero Go change | unit | `go test ./bench/runners/ -run TestResolveAiderEdit -x` | ❌ Wave 0 |
| EDITBENCH-03 | `edit_format_applied *bool` survives marshalling (false preserved, nil dropped) | unit | `go test ./bench/runtime/ -run TestEditFormatAppliedOpenKey -x` | ❌ Wave 0 |
| EDITBENCH-03 | result.v2 with `edit_format_applied` still schema-valid (`Validate` passes, no v3) | unit | `go test ./bench/runtime/ -run TestEditFormatAppliedSchemaValid -x` | ❌ Wave 0 |
| BASELINE-01 | Committed `result.v2.json` exists, carries expected outcome + `edit_format_applied` | golden (hermetic, no binary) | `go test ./bench/runtime/ -run TestAiderEditBaselineGolden -x` | ❌ Wave 0 |
| BASELINE-01 | Baseline report byte-reproducible (double-render diff-empty) | golden | `go test ./bench/aggregator/ -run TestAiderEditBaselineByteReproducible -x` | ❌ Wave 0 (mirror `byte_reproducible_test.go`) |
| BASELINE-01 | HELIX_BIN "did it RUN" sentinel — live leg produced a result | integration (HELIX_BIN-gated) | `HELIX_BIN=$(pwd)/helix go test ./bench/runtime/ -run TestAiderEditLiveRan -x` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./bench/runtime/... ./bench/datasets/aider-polyglot/... ./bench/runners/...` (the touched packages; hermetic, fast)
- **Per wave merge:** `go test ./...` + `go vet ./...` (CLAUDE.md mandate)
- **Phase gate:** full hermetic suite green AND a manual HELIX_BIN run of the live cell producing a byte-identical `result.v2.json` to the committed baseline (`HELIX_BIN=$(pwd)/helix make bench-aider-edit` or equivalent).

### Wave 0 Gaps
- [ ] `bench/runtime/aider_edit_agent_test.go` — hermetic AgentFn test (no binary; fake session OR a real OpenSession only under HELIX_BIN) covering EDITBENCH-01
- [ ] `bench/runtime/aider_edit_cell_test.go` — `runAiderEditCell` golden + HELIX_BIN sentinel covering BASELINE-01
- [ ] `bench/runtime/result_test.go` — extend with `edit_format_applied` *bool round-trip + schema-valid (EDITBENCH-03)
- [ ] `bench/runners/mode_resolver_test.go` — extend with `aider_edit` → `bench-full` row (EDITBENCH-02)
- [ ] `bench/aggregator/` — aider-edit baseline byte-reproducibility golden (BASELINE-01) mirroring `byte_reproducible_test.go:25-54`
- [ ] Anti-vacuity: a revert-and-fail test that breaks the WR-01 restore (or feeds a wrong reference) and asserts the native test FAILS (so the baseline pass is non-vacuous) — Pitfall 4 + `PITFALLS.md` meta-lesson

## Security Domain

> `security_enforcement` is enabled (absent = enabled). This phase is a local-only bench harness with no auth/session/network-facing surface, but the path-traversal controls are load-bearing.

### Applicable ASVS Categories
| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | local bench harness; no auth surface |
| V3 Session Management | no | — |
| V4 Access Control | no | — |
| V5 Input Validation | yes | Path-segment validation BEFORE any `filepath.Join`: `validatePathSegment` (`loader.go:81`), `validateModeName` (`mode_resolver.go:54`), `validateCellKey` (`cell.go:301`), `copyFile` relpath guard (`loader.go:129-132`). The new AgentFn/cell MUST reuse these — never join a raw exercise/mode/relpath. |
| V6 Cryptography | no | — |

### Known Threat Patterns for the bench harness
| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Path traversal via exercise/mode/relpath into `filepath.Join` | Tampering | Reuse `validatePathSegment`/`validateModeName`/`validateCellKey` BEFORE join (`loader.go:81`, `mode_resolver.go:54`, `cell.go:301`) |
| Test-tampering agent forces a spurious green | Tampering | WR-01 `restorePristineTests` runs after edit, before grade (`loader.go:241-249`); AgentFn edits only `files.solution` |
| Vacuous Rust pass (do-nothing stub compiles) | Tampering | WR-02 `cargo test -- --include-ignored` (`loader.go:286-292`) — irrelevant if baseline is go/python, but preserved by loader reuse |
| Daemon subprocess credential leak | Info disclosure | `--agent=claude` only; strict env allowlist in `internal/eval/agent` (`claude.go:9-11`); not on the baseline path |

## Sources

### Primary (HIGH confidence — direct in-tree read)
- `bench/datasets/aider-polyglot/loader.go` — `RunExercise` (:230), `AgentFn` (:204), `TestFn`/`TestResult` (:200,:188), WR-01 `restorePristineTests` (:177,:243-249), WR-02 (:286-292), `loadExercise` (:96), `nativeTestCommand` (:280), `flagNonHermetic` (:313)
- `bench/runtime/cell.go` — `RunCell` (:367), `baselineRagMode` branch (:434), claude branch (:556-583), kill+tap (:603-622), `BuildResult` call (:764), `writeDurable` (:923), `runVerify` exec pattern (:821)
- `bench/runtime/result.go` — `swebench_raw_resolved *bool` ResultInput (:138-139) + json tag (:228-229), additive-key doctrine (:39-42,:84-90), `BuildResult` (:244), `Validate` (:322), `fairnessBlock` sort (:303-313)
- `bench/runtime/rag.go` — `runRAGCell` mode-name branch precedent (:37,:57)
- `bench/runtime/drive.go` — `forwarder.OpenSession`+`CallTool` in-process gRPC dial (:67,:73,:87,:108), Phase 94 stdio-head removal note (:29-37)
- `bench/runtime/cctap.go` — `SynthCCTap` zero-Usage scripted leg (:33,:73-85)
- `bench/runtime/subprocess/{daemon.go:40, claude.go:51,31}` — `StartDaemon`, `StartClaude`, `ErrClaudeNotFound`
- `bench/runtime/matrix.go` — `ExpandMatrix` (:119), `cellSeedDir` (:246), `runOneCell` (:254)
- `bench/runners/mode_resolver.go` — `ResolveProfileFromRoot` (:83), filesystem-as-table (:5-9), strict frontmatter (:35-38,:119)
- `bench/aggregator/aggregate.go` — `renderAll` shared byte-stable render (:193), sort-before-emit (:189-191,:233)
- `bench/aggregator/byte_reproducible_test.go` — double-render diff-empty proof (:25-54)
- `internal/cli/verbs_gen.go` — EDIT verb→tool-name map (:108-110,:272,:282,:362,:374-376)
- `Makefile` — `bench-quick` HELIX_BIN-gated pattern (:149-158), `bench-micro` latency-only (:118-127), `verify-licenses` vendored-tree gate (:337)
- `bench/datasets/aider-polyglot/fixtures/.../config.json` — `files.example: [".meta/example.<ext>"]` confirmed present in vendored tree

### Secondary (HIGH — this milestone's research artifacts)
- `.planning/research/ARCHITECTURE.md` (Pattern 4/5 seams, build order), `.planning/research/PITFALLS.md` (Pitfall 2/4, the four named vacuous-pass CRITICALs, fail-closed discipline)
- `.planning/phases/99-.../99-01-SUMMARY.md` (vendored fixtures tree, offline, `@7e0611e`)
- Project memory: `helix-bench-smoke-false-green` (HELIX_BIN fail-not-skip), `helix-integration-tagged-suite-preexisting-failures`

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — every reused component read directly with file:line anchors; no new dependency
- Architecture (cell branch / AgentFn seam / additive key): HIGH — `runRAGCell` + `swebench_raw_resolved` are byte-exact precedents
- Pitfalls: HIGH — anchored to named shipped CRITICALs (Phase 82/81/80) and project memory
- Open questions: MEDIUM — export-visibility and matrix-keying are plan-time decisions, not blockers

**Research date:** 2026-06-23
**Valid until:** ~2026-07-23 (stable in-tree bench stack; re-verify if `bench/runtime/cell.go` or `result.go` change)

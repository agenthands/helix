# Architecture Research

**Domain:** Dev-time/offline GEPA prompt-optimization pipeline driving a tool-using LLM agent over the `helix` CLI, scored by real coding-benchmark task-success — integrated into a shipped Go single-binary product without breaching the no-runtime-Python / single-binary boundary.
**Milestone:** v2.3 Task-Success-Driven Skill Optimization
**Researched:** 2026-06-24
**Confidence:** HIGH (all integration points read from source; only the new Python agent module is greenfield)

## Standard Architecture

### System Overview — one task-success evaluation of one GEPA candidate

```
DEV-TIME / OFFLINE (tools/ — NEVER in the helix binary, go.mod, or `go test ./...`)
┌──────────────────────────────────────────────────────────────────────────────┐
│  tools/dspy-tune/optimize.py   (GEPA loop — modified)                          │
│    GEPA proposes candidate SKILL text  ──┐                                     │
│                                          │ injected as system prompt           │
│    metric(gold, pred) ───────────────────┘                                     │
│         │  (in-process Python call — NOT subprocess)                           │
│         ▼                                                                      │
│  tools/dspy-tune/agent/  (NEW Python pkg)   tools/dspy-tune/taskmetric.py (NEW)│
│  ┌────────────────────────────┐             ┌──────────────────────────────┐  │
│  │ ReActAgent                  │            │ task_success_metric(...)      │  │
│  │  - openai client (DeepSeek  │◄───────────│  for each held-out task:      │  │
│  │    primary base_url +       │  candidate │   1. make per-task sandbox    │  │
│  │    OpenAI fallback)         │  skill text│   2. agent.run(task, skill)   │  │
│  │  - system prompt =          │            │   3. grade (Aider/SWE-bench)  │  │
│  │    candidate skill text     │            │   4. pass/fail → score+feedbk │  │
│  │  - ReAct loop:              │            └──────────────┬───────────────┘  │
│  │      think → emit a         │                           │                   │
│  │      `helix <verb> …` argv  │                           │                   │
│  └──────────┬──────────────────┘                          │                   │
└─────────────┼─────────────────────────────────────────────┼───────────────────┘
              │ subprocess (os argv)                         │ subprocess
              ▼                                              ▼
   ┌──────────────────────┐              ┌───────────────────────────────────────┐
   │  helix <verb> CLI     │              │  BENCHMARK GRADER                     │
   │  (the SHIPPED product │              │  A) Aider polyglot: native test argv  │
   │   surface — real      │              │     (pytest / cargo / go test / …)    │
   │   binary, real daemon)│              │     run in per-task work dir          │
   │   inside per-task      │             │  B) SWE-bench: python -m swebench.    │
   │   sandbox/worktree     │             │     harness.run_evaluation via Podman │
   └──────────────────────┘              │     (DOCKER_HOST → podman.sock)       │
                                          └──────────────┬────────────────────────┘
                                                         │ exit 0/non-0 → pass/fail
                                                         ▼
                                          GEPA records score → reflects → next candidate
                                                         │
                                                         ▼ best candidate (output/optimized.json, git-ignored)
─────────────────────────────────────────────────────────────────────────────────
 HUMAN-GATED RE-ENTRY (the ONLY path back to the shipped surface)
   human transcribes adopted text → internal/cli/skills/helix/SKILL.md  (≤1536 chars,
   "## Decision matrix" anchor preserved)  OR a cmd/helix-refgen override
                                          │
                                          ▼
   `go run ./cmd/helix-refgen --check`  (byte-repro drift gate; CI)
                                          │
                                          ▼
   reference.md regenerated + committed → ships in the binary's skill bundle
```

### Component Responsibilities

| Component | Responsibility | New / Modified | Real path |
|-----------|----------------|----------------|-----------|
| ReAct tool-using agent | DeepSeek-primary/OpenAI-fallback LLM driving `helix <verb>` argv in a think→act→observe loop; system prompt = candidate skill text | **NEW** | `tools/dspy-tune/agent/` (Python) |
| `openai`-client wrapper | One `openai.OpenAI(base_url=…)` instance per provider; DeepSeek primary (`base_url=https://api.deepseek.com`), OpenAI fallback | **NEW** | `tools/dspy-tune/agent/llm.py` |
| Task-success metric | Replaces `score_choice_rate` as GEPA's metric: runs the agent per task, grades, returns `dspy.Prediction(score, feedback)` | **NEW** (replaces `adopt_metric` body) | `tools/dspy-tune/taskmetric.py` + `optimize.py` |
| Aider grader bridge | Build the native per-language test argv + run it in the work dir; mirror of `aiderpolyglot.NativeTestCommand` / `newNativeTestFn` | **NEW** (Python parity mirror) | `tools/dspy-tune/grade_aider.py` |
| SWE-bench grader bridge | Shell `python -m swebench.harness.run_evaluation` under `DOCKER_HOST→podman.sock`; mirror the Go harness argv/allowlist | **NEW** (Python; or subprocess to a Go shim) | `tools/dspy-tune/grade_swebench.py` |
| Per-task sandbox/worktree | Isolated work dir per task: pristine solution stub + test files; reset between candidates | **NEW** | `tools/dspy-tune/sandbox.py` |
| GEPA harness loop | Draws train/val from `data/train.jsonl`; sequesters `data/test.jsonl`; wires the new metric | **MODIFIED** | `tools/dspy-tune/optimize.py` |
| `choice_rate` scorer | Retained as a fast pre-screen / secondary signal (parity-pinned to the Go classifier) — NOT deleted | **UNCHANGED** | `tools/dspy-tune/scorer.py` |
| Import-boundary analyzer | Keeps the whole `tools/` tree out of every runtime/cmd package | **UNCHANGED** (boundary holds; no new Go edge) | `internal/lint/toolsquarantine/`, `cmd/vet-tools-quarantine` |
| Adoption gate | `helix-refgen --check` byte-repro drift gate; human transcribes adopted text | **UNCHANGED** | `cmd/helix-refgen/` |
| `helix` CLI / daemon | The product surface the agent shells; the only thing that runs in production | **UNCHANGED** | `cmd/helix`, `internal/...` |

## Recommended Project Structure

The new agent and metric live **as sibling modules inside the existing `tools/dspy-tune/` package**, NOT a separate `tools/agent/`. Rationale: GEPA calls its metric **in-process per candidate** (`optimize.py` imports `score_choice_rate` directly today — `from scorer import score_choice_rate`, optimize.py:69). The agent must be a plain Python import the metric can call in the same process, exactly as the scorer is. A separate top-level package would force a cross-package import for no boundary benefit (both are under the single quarantined `tools/` namespace anyway).

```
tools/dspy-tune/                  # the SINGLE quarantined dev-time tree (unchanged location)
├── optimize.py                   # MODIFIED: metric body swaps choice_rate → task-success
├── scorer.py                     # UNCHANGED: choice_rate classifier (kept as pre-screen)
├── taskmetric.py                 # NEW: task_success_metric() — the GEPA metric callable
├── agent/                        # NEW: the ReAct tool-using agent
│   ├── __init__.py
│   ├── react.py                  #   the think→emit-`helix`-argv→observe loop
│   ├── llm.py                    #   openai client: DeepSeek primary + OpenAI fallback
│   └── tools.py                  #   subprocess shim that runs `helix <verb> …` argv
├── grade_aider.py                # NEW: Python mirror of NativeTestCommand + work-dir run
├── grade_swebench.py             # NEW: shells swebench harness via DOCKER_HOST→podman
├── sandbox.py                    # NEW: per-task isolated work dir (pristine stub+test copy)
├── data/                         # UNCHANGED location: train.jsonl (train/val), test.jsonl (sequestered)
│   └── tasks/                    # NEW (likely): real benchmark task specs the agent runs
├── golden/parity_cases.json      # UNCHANGED: shared Go↔Python parity corpus
├── test_split.py                 # UNCHANGED + EXTEND: TEST ⊄ train∪val (now over task corpus)
├── test_parity.py                # UNCHANGED: choice_rate parity (scorer.py stays pinned)
├── test_degenerate.py            # UNCHANGED (choice_rate gaming guard; advisory)
├── test_agent.py                 # NEW: hermetic agent loop test (fake LLM, fake helix, no network)
├── test_grade.py                 # NEW: hermetic grader test (fake test cmd, no Podman)
├── requirements.txt              # MODIFIED: add `openai` (dev venv only — never go.mod)
├── README.md / REPORT.md         # MODIFIED: document the task-success pivot
└── output/optimized.json         # git-ignored; NEVER pasted into the product
```

### Structure Rationale

- **Same package, not `tools/agent/`:** the metric→agent call is an **in-process Python import** (GEPA invokes `metric(gold, pred)` per candidate inside `optimizer.compile`); keeping the agent under `tools/dspy-tune/` matches the existing `from scorer import …` pattern and stays inside the one quarantined namespace the analyzer already covers.
- **`agent/` as a subpackage:** the agent is the largest new surface (LLM client, ReAct loop, subprocess tool shim) — a subpackage keeps `optimize.py` thin and lets `test_agent.py` drive the loop hermetically with a fake LLM + fake `helix`.
- **Graders as separate modules:** `grade_aider.py` and `grade_swebench.py` isolate the two benchmark backends so the Aider path (no Podman) runs without the SWE-bench path being reachable, and each gets its own hermetic test.

## How the candidate SKILL.md text becomes the agent's system prompt

GEPA optimizes a `dspy.Signature`'s instruction text. Today `optimize.py` defines a `Steer` signature whose docstring is the evolved instruction and a `dspy.Predict(Steer)` program (optimize.py:73-79). For v2.3 the evolved text must reach the agent as its **system prompt** for each candidate:

1. GEPA hands the metric a `pred` whose program carries the current candidate instruction (GEPA mutates the signature/instruction; `track_stats=True` and the reflection LM drive the evolution — optimize.py:124-130).
2. The new `task_success_metric(gold, pred, …)` extracts the candidate's instruction text from the predictor (`pred`/the compiled program's signature instructions) and passes it to `ReActAgent(system_prompt=candidate_text)`.
3. The agent runs the held-out task with that system prompt; the grader returns pass/fail; the metric returns `dspy.Prediction(score=1.0/0.0, feedback=…)` — same return contract GEPA already consumes (optimize.py:91).

**Boundary note:** the candidate text is a *steering instruction*, not the shipped `SKILL.md` file. The mapping from "winning instruction" → committed `SKILL.md` edit stays a **human transcription** (≤1536 chars, `## Decision matrix` anchor preserved) — the optimizer never writes the bundle (optimize.py:132-148, README "artifact re-entry gate").

## Per-task evaluation data flow (component + boundary diagram)

```
GEPA candidate instruction C
   │  (1) in-process Python call: task_success_metric(gold, pred=program(C))
   ▼
taskmetric.py
   │  (2) for task T in held-out task set:
   │        sandbox = make_sandbox(T)        ── pristine solution stub + test files copied in
   ▼
ReActAgent(system_prompt=C).run(task=T, cwd=sandbox)
   │  (3) loop, bounded by max_turns / max_tool_calls:
   │        LLM (DeepSeek primary, OpenAI fallback) emits a `helix <verb> …` argv
   │        ── subprocess boundary ──► helix CLI in sandbox  ──► daemon over per-task socket
   │        observe stdout/exit, feed back into the loop
   │  (4) agent stops (done / budget exhausted) leaving an edited work tree
   ▼
GRADER (one of):
   A) grade_aider.py:  argv = NativeTestCommand(lang)  (pytest|cargo test --include-ignored|
        go test ./...|./gradlew test|./npm-test.sh|./cpp-test.sh)
        ── restore pristine TEST file FIRST (anti-tamper, WR-01) ── run in sandbox ── exit 0 = pass
   B) grade_swebench.py:  build predictions.jsonl from the agent's diff
        ── python -m swebench.harness.run_evaluation --dataset_name princeton-nlp/SWE-bench_Verified …
        ── env: DOCKER_HOST=unix://$XDG_RUNTIME_DIR/podman/podman.sock (+ PATH/HOME/HELIX_CACHE_DIR)
        ── parse the harness report → resolved/unresolved = pass/fail
   │
   ▼  (5) score = 1.0 if pass else 0.0; feedback = test/harness failure tail (for GEPA reflection)
dspy.Prediction(score, feedback)  ──► GEPA records, reflects, proposes next candidate
```

**Boundaries crossed (each is a process/trust boundary):**
- **Metric → agent:** in-process Python import (cheap; no serialization).
- **Agent → `helix`:** `subprocess` argv (the product surface; the agent NEVER imports Go).
- **Grader → Aider native test:** `subprocess` argv in the sandbox work dir (mirror of Go's `newNativeTestFn`).
- **Grader → SWE-bench:** `subprocess` to the upstream Python harness, which itself talks the Docker API → Podman's docker-compatible socket via `DOCKER_HOST`.
- **Optimizer → shipped binary:** NONE automatically. The only edge is the human-reviewed `SKILL.md`/refgen commit gated by `helix-refgen --check`.

## How the optimized artifact crosses back to the runtime

**The agent/optimizer NEVER writes the binary, the skill bundle, or `reference.md`.** This invariant already exists in v2.2 and is unchanged:

- `optimize.py` writes only `output/optimized.json` (git-ignored) and prints a re-entry reminder (optimize.py:132-148).
- The **sole sanctioned path**: a human transcribes the winning steering text into `internal/cli/skills/helix/SKILL.md` (preserving the `## Decision matrix` anchor and the **SKILL-04 1,536-char idle-cost cap** — enforced by `internal/cli/skill_test.go:121-151`), OR into a `cmd/helix-refgen` per-verb override (`outputShapeOverrides` / `useThisNotThatOverrides`, render.go:207/223).
- The gate is **`go run ./cmd/helix-refgen --check`** (helix-refgen/main.go:60-71): byte-for-byte compares the on-disk `reference.md` against a fresh render walking the live `skill.ToolProviders()`; exit 1 on drift. This is a CI gate (REF-03). The gate is on the **committed artifact**, never the optimizer **process** (LLM output is not bit-reproducible — README "artifact re-entry gate").

**v2.3 changes nothing here.** The task-success pivot changes *what signal* selects the candidate; it does not change the re-entry mechanism. Roadmap should treat the adoption-gate phase as a **re-verification** of the v2.2 invariant under the new metric, not a rebuild.

## Invoking the existing Go `bench/` assets from Python without linking Go

Two candidate strategies; **recommendation differs per benchmark**:

### Aider polyglot — replicate a tiny parity-pinned Python mirror (RECOMMENDED)
The Go grader is trivially small and pure-stdlib: `aiderpolyglot.NativeTestCommand(lang)` returns a fixed per-language argv (loader.go:299-323) and `newNativeTestFn` just runs `exec.CommandContext(argv…).CombinedOutput()` in the work dir (aider_edit_agent.go:171-182). Mirroring this in `grade_aider.py` is ~15 lines and avoids a `helix-bench` subprocess + JSON-protocol surface. **This follows the existing `scorer.py` precedent**: the project already accepts a parity-pinned Python re-implementation of a small Go truth, pinned case-for-case against a shared golden corpus (scorer.py header; `golden/parity_cases.json`).
- **MUST mirror the load-bearing details verbatim:** Rust uses `cargo test -- --include-ignored` (a plain `cargo test` is a vacuous false-pass, loader.go:305-311); the pristine **test file is restored AFTER the agent edits, BEFORE grading** (WR-01 anti-tamper, RunExercise loader.go:260-268). Both are anti-vacuity invariants — each needs a break-the-invariant test in `test_grade.py`.
- **Add a parity corpus** the way `scorer.py` has one: a shared `golden/aider_native_cmd.json` asserted by both a new Go `*_test.go` and `test_grade.py`, so the argv table cannot silently drift.

### SWE-bench — call the upstream Python harness directly from Python (RECOMMENDED)
The Go `swebench.Harness` is itself just a **subprocess shim** around `python -m swebench.harness.run_evaluation` (harness.go:11, :179-229). From Python you are already in the harness's native language — call the harness module directly (or `subprocess` the same argv), reusing the Go harness's **validation + env discipline as the spec to mirror**:
- Fixed argv: `-m swebench.harness.run_evaluation --dataset_name <allowlisted> --predictions_path <abs,clean> --run_id <[A-Za-z0-9_-]+> --max_workers <n> --cache_level <none|base|env|instance> --instance_ids <ids…>` (RunArgs, harness.go:179-229).
- Dataset name allowlist: `princeton-nlp/SWE-bench_Verified` + `Bertsekas/SWE-Bench_Verified_UTBoost` (harness.go:30-33).
- Env allowlist (never inherit full env): `PATH, HOME, HELIX_CACHE_DIR, DOCKER_HOST, DOCKER_TLS_VERIFY, DOCKER_CERT_PATH` (harness.go:342-345). Set `DOCKER_HOST=unix://$XDG_RUNTIME_DIR/podman/podman.sock` and start `podman system service --time=0 &` if no socket is up.

### Do NOT subprocess `helix-bench run` as the grader
`helix-bench run` orchestrates the *whole* matrix (daemon-per-cell, scripted/claude agent, result.v2 JSON — main.go:127-311). The v2.3 agent is a *new* agent driver that does not fit the existing `--agent=scripted|claude` switch, and the GEPA metric needs a single task pass/fail, not a matrix run. Reusing `helix-bench` would mean teaching it a third agent and round-tripping result JSON — heavier than the two thin Python mirrors above. **Keep `helix-bench` for its own benchmarking; give GEPA direct, minimal graders.**

## The import-boundary analyzer — what v2.3 introduces and how the quarantine holds

**The quarantine is import-boundary-only and already covers everything v2.3 adds — no new Go edge is introduced, so the analyzer needs NO change.** Confirmed from source:

- `toolsquarantine.Analyzer` fails if any package whose path is **outside** `github.com/agenthands/helix/tools` imports anything under that prefix (analyzer.go:39-65). The `tools/` tree self-imports freely (the exemption at analyzer.go:51).
- The new agent, metric, and graders all live under `tools/dspy-tune/…` → inside the exempt prefix. They `import` each other (Python) and `subprocess` the `helix` binary and the swebench harness — **none of that is a Go import**, so it is invisible to the analyzer and to `go test ./...`.
- The analyzer is **deliberately import-only** — it does NOT scan for `exec.Command`/pip shell-outs (analyzer.go:24-28), because `internal/langregistry/installer.go` legitimately shells pip. So the agent's `subprocess helix` and the grader's `subprocess swebench` raise no flag even though they shell out.

**New boundary edges v2.3 introduces (all benign w.r.t. the quarantine):**

| New edge | Crosses a Go import boundary? | Quarantine action |
|----------|-------------------------------|-------------------|
| Python agent → `openai` lib | No (dev venv only; `requirements.txt`, never `go.mod`) | none — add `openai` to `requirements.txt` only |
| Python agent → `helix` binary (subprocess) | No (argv, not import) | none |
| Python grader → swebench harness (subprocess) | No | none |
| Python grader → Aider native test cmd (subprocess) | No | none |
| New `tools/dspy-tune/*.py` files | No (`tools/` ships zero `.go`) | none — the no-`.go`-under-`tools/` invariant still holds |

**The one thing to GUARD against (anti-vacuity):** the quarantine's *only* failure mode is a future `tools/*.go` file or a runtime package importing `tools/…`. v2.3 must keep the rule honest by ensuring:
1. No `go.mod`/`go.sum` change adds `openai`/`dspy`/Python deps (they are pip-only).
2. No new `tools/*.go` file appears (would be Go-visible). If a Go-side parity test for the Aider argv corpus is added, it lives under `bench/datasets/aider-polyglot/` (a runtime-adjacent package that may NOT import `tools/`) and reads the shared JSON corpus — exactly the `scorer.py`/`parity_test.go` split, which does NOT cross the boundary.
3. Keep the existing `analyzer_test.go` leaky/good testdata pair green (it already plants a deliberate violation under `testdata/`, which `go vet` ignores).

The `make vet` chain (`vet-tools-quarantine` is the 7th vettool, Makefile:55-63) runs on every `make test`. **No Makefile change needed.**

## Suggested Build Order (dependency-driven)

The chain is **agent → metric-rewire → SWE-bench → adoption gate**, matching the quality gate. Each step ships its break-the-invariant test before the next depends on it. Phase numbering continues from 106.

1. **Phase 107 — NEW ReAct agent + LLM client (`tools/dspy-tune/agent/`)**
   - `llm.py` (DeepSeek-primary / OpenAI-fallback `openai` client; key from env, never committed), `react.py` (think→emit-`helix`-argv→observe loop, budget-bounded), `tools.py` (`subprocess helix <verb>` shim, fixed-argv + sandbox cwd).
   - `test_agent.py`: hermetic — fake LLM + fake `helix` stub, no network, asserts the loop emits/observes correctly and respects the budget. Anti-vacuity: a degenerate "always emit grep" agent must score 0.
   - **Why first:** the metric, both graders, and the adoption gate all depend on a working agent. Standalone, no benchmark coupling yet.
   - **Boundary check:** `make vet` stays green; `openai` added to `requirements.txt` only.

2. **Phase 108 — Aider-polyglot grader + per-task sandbox + metric rewire (`grade_aider.py`, `sandbox.py`, `taskmetric.py`, `optimize.py`)**
   - Python mirror of `NativeTestCommand` + the WR-01 pristine-test restore; `sandbox.py` per-task isolated work dir; `taskmetric.py` ties agent→grade→`dspy.Prediction`; `optimize.py` metric body swaps `choice_rate` → task-success (keep `scorer.py` as an optional pre-screen).
   - Add a shared `golden/aider_native_cmd.json` parity corpus + matching Go `*_test.go` under `bench/datasets/aider-polyglot/` and a Python `test_grade.py`. Anti-vacuity: a do-nothing-stub agent fails Rust (proves `--include-ignored`); a test-tampering agent cannot force green (proves restore-before-grade).
   - Extend `test_split.py` to assert TEST ⊄ train∪val over the task corpus.
   - **Why second:** depends on the agent (1); Aider needs no Podman, so it is the cheaper, hermetic-test-friendly grader to land before SWE-bench.

3. **Phase 109 — SWE-bench grader via Podman (`grade_swebench.py`)**
   - Mirror the Go harness's dataset-name allowlist, fixed argv, and env allowlist; set `DOCKER_HOST→podman.sock`; build `predictions.jsonl` from the agent's diff; parse the harness report → pass/fail.
   - Hermetic test: fake harness invocation (assert exact argv + env allowlist, like `swebench.runShim`), no live Podman. Live run is network/Podman-gated and skips cleanly offline.
   - **Why third:** heaviest dependency (Podman + upstream harness); depends on the agent (1) and the metric plumbing (2). Pluggable as a second grader behind the same `taskmetric.py` interface.

4. **Phase 110 — Human-gated adoption re-verification (`cmd/helix-refgen`, `SKILL.md`)**
   - Re-verify the v2.2 invariant under the new metric: optimizer writes only `output/optimized.json`; adopted text re-enters only via human-reviewed `SKILL.md`/refgen edit; `helix-refgen --check` byte-repro gate green; `make vet` green; no `go.mod` drift.
   - **Why last:** it gates the *output* of the whole pipeline; nothing downstream depends on it. Mostly a re-verification + doc/REPORT.md update (ship vs no-ship) — the mechanism is unchanged from v2.2.

## Architectural Patterns

### Pattern 1: In-process metric, subprocess everything-the-LLM-touches
**What:** GEPA calls the metric as a same-process Python function (cheap, per-candidate); the agent reaches the product (`helix`) and the graders reach the toolchains/harness via `subprocess`. **When:** any GEPA/optimizer metric that must call an LLM agent in-loop. **Trade-off:** keeps the optimizer 100% Python with zero Go linkage, at the cost of per-call subprocess spawn overhead (acceptable — LLM latency dominates).

### Pattern 2: Parity-pinned Python mirror of a small Go truth
**What:** when a Go grader is tiny and pure, re-implement it in Python and pin it case-for-case against a shared JSON golden corpus asserted by both sides (the `scorer.py` ↔ `parity_test.go` ↔ `golden/parity_cases.json` precedent). **When:** the Go asset is small (the Aider argv table) and a subprocess+JSON protocol would be heavier than the mirror. **Trade-off:** two implementations to keep in sync — mitigated by the shared corpus + a planted-divergence anti-vacuity test.

### Pattern 3: Direct subprocess to the upstream tool when already in its language
**What:** SWE-bench's harness is Python; the Go `swebench.Harness` is only a shim around it. From Python, skip the shim and call the harness directly, reusing the Go shim's validation/env discipline as the spec. **When:** the existing Go asset adds no logic beyond marshaling a subprocess. **Trade-off:** the Python side must independently re-enforce the dataset/argv/env allowlists (don't trust caller input) — a deliberate duplication of the security discipline, not the orchestration.

## Anti-Patterns to Avoid

- **A new `tools/agent/` top-level package.** Forces a cross-package import for the in-process metric→agent call with no boundary benefit (both are inside the one quarantined `tools/` namespace). Use `tools/dspy-tune/agent/`.
- **Subprocessing `helix-bench run` as the grader.** Pulls in the whole matrix orchestrator (daemon-per-cell, result.v2 JSON, the `scripted|claude` agent switch) when GEPA needs one task pass/fail. Give GEPA minimal direct graders.
- **`"helix" in response` substring scoring carried into task-success.** The choice_rate substring trap (scorer.py header) is irrelevant to task-success but tempting as a shortcut signal — task-success keys on the *test/harness exit*, not on what the agent printed.
- **Auto-writing `SKILL.md`/`reference.md` from the optimizer.** Breaks the human-gated re-entry invariant and the `--check` byte-repro contract. The optimizer PROPOSES; a human transcribes.
- **Adding `openai`/`dspy` to `go.mod` or a Go `tools/*.go` file.** Any of these breaches the single-binary / no-runtime-Python boundary and (for a `tools/*.go`) becomes Go-visible. Keep all Python deps in `requirements.txt` (dev venv only).
- **Skipping the WR-01 restore-before-grade in the Python Aider mirror.** A test-tampering agent would force spurious greens — the exact anti-vacuity failure the Go loader guards against.

## Scalability / Cost Considerations

| Concern | Small corpus (current ~5 tasks) | Real run (val_size > 50, TUNE-FUT-01) |
|---------|---------------------------------|----------------------------------------|
| LLM cost | DeepSeek primary keeps per-candidate ReAct loops cheap; bound `max_tool_calls` | dominant cost; cap candidates × tasks × turns; cache nothing across candidates (each needs the new system prompt) |
| Wall-clock | seconds (Aider native tests) | SWE-bench per-instance Podman build dominates — pin a small `--instance_ids` subset, `--max_workers` to host cores |
| Statistical trust | too small for a held-out delta (the v2.2 no-ship reason) | grow `data/train.jsonl`/task corpus first (TUNE-FUT-01) — the real gate before a tuning run can beat a deterministic baseline |
| Isolation | per-task sandbox dir | SWE-bench instances are container-isolated by the harness; Aider runs `--network=none`-style hermetic where the loader marks the task hermetic |

## Confidence Assessment

| Area | Confidence | Reason |
|------|------------|--------|
| Module placement (`tools/dspy-tune/agent/`) | HIGH | Driven by the existing in-process `from scorer import …` metric pattern (optimize.py:69) |
| Candidate-text → system-prompt injection | HIGH | GEPA signature-instruction evolution is exactly what optimize.py already does; only the consumer changes |
| Per-task data flow + boundaries | HIGH | Aider loader (RunExercise/NativeTestCommand) and swebench harness read in full from source |
| Re-entry / adoption gate | HIGH | Unchanged from v2.2; `helix-refgen --check` and the 1536-char cap verified in source |
| Aider grader = Python mirror | HIGH | Go grader is ~15 lines pure-stdlib; matches the established `scorer.py` parity precedent |
| SWE-bench grader = direct Python harness call | HIGH | Go harness is itself a subprocess shim; Podman socket path confirmed in CLAUDE.md + harness env allowlist |
| Quarantine needs no change | HIGH | Analyzer is import-only; all new edges are Python/subprocess, inside the exempt `tools/` prefix |
| Build order | HIGH | Dependency chain agent→metric→swebench→gate is forced by what each step imports/needs |

## Sources

- `tools/dspy-tune/optimize.py`, `scorer.py`, `README.md`, `requirements.txt` (HIGH — source of truth for the GEPA loop, metric contract, re-entry gate)
- `internal/lint/toolsquarantine/analyzer.go` + `Makefile` vet chain (HIGH — quarantine semantics)
- `cmd/helix-refgen/main.go`, `render.go` (HIGH — `--check` gate, override maps)
- `bench/datasets/aider-polyglot/loader.go` (HIGH — NativeTestCommand table, WR-01 anti-tamper, RunExercise protocol)
- `bench/runtime/aider_edit_agent.go` (`newNativeTestFn`) (HIGH — the live grader shellout to mirror)
- `bench/evaluators/swebench/harness.go` (HIGH — argv, dataset allowlist, env allowlist, Podman/DOCKER_HOST)
- `bench/runtime/subprocess/claude.go`, `cmd/helix-bench/main.go` (HIGH — why NOT to reuse helix-bench as the grader)
- `.planning/PROJECT.md` v2.3 milestone section (HIGH — locked decisions: CLI subprocess transport, Aider+SWE-bench via Podman, DeepSeek-primary, no-runtime-Python)

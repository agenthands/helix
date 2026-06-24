# Pitfalls Research

**Domain:** Task-success-driven skill optimization — a dev-time DeepSeek/OpenAI tool-using agent driving the `helix` CLI, a GEPA metric rewired from `choice_rate` to agent task-success on Aider polyglot + SWE-bench (Podman), human-gated SKILL.md adoption via `helix-refgen --check`. Net-new features added to a shipped Go single-binary product (v2.3, phases 107+).
**Researched:** 2026-06-24
**Confidence:** HIGH (grounded in v2.2 Phase 106 spike artifacts, carried-constraint memories, DeepSeek/SWE-bench primary docs)

> Scope note: these are pitfalls specific to *adding this optimizer-agent loop to Helix*, not generic ML advice. The dominant repeated failure class in this repo is **anti-vacuity** (a gate that passes green without actually testing the invariant); every new gate below carries a concrete break-the-invariant → assert-RED requirement. The v2.2 no-ship root cause (gameable `choice_rate` proxy + tiny corpus) is addressed head-on by Pitfalls 1, 2, and 3.

## Critical Pitfalls

### Pitfall 1: Reward-hacking the task-success oracle (the metric is the milestone — make it honest first)

**What goes wrong:**
The whole reason v2.3 exists is that `choice_rate` was gameable ("always emit `helix` first"). A task-success oracle re-introduces *new* gaming surfaces, any of which silently inflates the optimized delta and ships degenerate steering text:
- **Vacuous pass (zero tests ran).** The harness counts a task "passed" when the test command exited 0 but zero tests actually executed — wrong dataset name → empty test set, a collection error swallowed, a `pytest` that found no matches, or a SWE-bench container that never applied the patch. This is the *exact* shape of the Phase 81 `no_semantic` zero-reads vacuity (missing line read as 0 = "pass") recorded in `helix-bench-smoke-false-green`.
- **Pass without using helix.** The agent solves the task with its own reasoning / `cat`+`sed` and never calls a `helix` verb, yet the optimized skill text gets credit. The metric must not reward task-success that didn't route through the product surface.
- **Test tampering inside the sandbox.** The agent edits/deletes the failing test, weakens an assertion, or `git checkout`s the gold patch, then "passes."
- **Degenerate steering that games the agent.** GEPA evolves SKILL.md text that browbeats the agent into spamming helix verbs (or hard-codes answers) rather than genuinely improving tool selection — the task-success analogue of always-`helix`.

**Why it happens:**
Pass/fail is read from an exit code or a metric line, and the *absence* of a signal is silently coerced to a benign default. The agent runs with write access to the repo under test (it must, to edit code), which is also write access to the oracle.

**How to avoid:**
- **Three-part honest oracle, each independently gated:** (1) **fail-not-skip** — if the test set is empty, the dataset name unresolved, the container missing, or the harness can't *prove* N>0 tests ran, the result is a hard ERROR, never "pass" and never silently 0 (carry the HELIX_BIN fail-not-skip lesson). (2) **FAIL_TO_PASS / PASS_TO_PASS contract** — adopt SWE-bench's own resolution gate: the designated failing tests must flip fail→pass AND the previously-passing tests must stay green; a patch that breaks PASS_TO_PASS is not a pass. (3) **gold-test immutability** — run the gold/oracle test from a path the agent cannot write (apply the agent's diff, then `git checkout` the test files / restore them from the pinned dataset before grading), and reject if the agent's diff touches test files.
- **Attribution of the win to helix use** belongs to the ON/OFF control arm (Pitfall 3) — task-success alone does not prove the *skill* helped.
- **Degenerate-steering inspection on the optimized text** must run on every GEPA output and must be a *flag-no-flag pair* (degenerate text flagged, legitimate conditional text not), carried forward and broadened from `test_degenerate.py`. The honest backstop remains human review (Pitfall 8) — the smell test is documented as not-sound (Phase 106 IN-01).

**Warning signs:**
A suspiciously high pass rate; pass rate identical with and without the agent actually editing; "passed" tasks whose logs show 0 collected tests; an optimized delta that survives only because TEST tasks with empty test sets count as wins; optimized SKILL.md text that is unconditional imperatives.

**Phase to address:**
Earliest task-success-oracle phase (the metric core, ~107–108), before any GEPA run is wired. The oracle's honesty gates are prerequisites for the optimization phase.

---

### Pitfall 2: Overfitting on a tiny corpus — the v2.2 no-ship cause, unfixed

**What goes wrong:**
v2.2 concluded **no-ship** precisely because the corpus was tiny (`MinTasks=5` floor; the actual split was TRAIN=8 / TEST=3) and a meaningful held-out split starves the optimizer of signal, so any "win" is noise. If v2.3 rewires the metric but reuses an Aider/SWE-bench slice of a dozen tasks, it reproduces the no-ship — now at far higher cost per task (real container runs, real API spend). A second failure mode is **TEST-split leakage**: the held-out TEST tasks get seen by `optimizer.compile()` (GEPA's reflective loop, val scoring, or few-shot bootstrap), so the reported held-out delta is optimistic and unattributable.

**Why it happens:**
Growing a task-success corpus is expensive (each task is a container build + multi-turn agent loop), so there's pressure to reuse the small `choice_rate` corpus. Split discipline erodes because GEPA wants val data to reflect against, and it's tempting to feed it everything.

**How to avoid:**
- **Honor the TUNE-FUT-01 power threshold as a hard gate, not a footnote.** `val_size > 50` is the documented floor for trusting a tuned delta; below it the harness must conclude no-ship (a legitimate, success-meeting outcome — keep that framing from Phase 106). Treat `val_size > 50` as a precondition for *adopting* any optimized text, asserted in the harness.
- **TRAIN / VAL / held-out TEST discipline, TEST sequestered from `compile()`.** Keep the Phase 106 pattern verbatim: the held-out split is created as the FIRST harness step (disjoint by construction); `test.jsonl` is loaded only for the final report print and **never** passed to `compile()`; a `test_split.py`-style guard asserts `TEST ∩ (TRAIN∪VAL) == ∅` and RED-fails on a planted leak.
- **Report the delta only on data the optimizer never saw**, and mirror the `MaterialDrop=0.4`-style margin as a *reporting* threshold (adopt only if it clears the margin on held-out TEST), never a CI gate.

**Warning signs:**
Train/val accuracy near 100% with held-out TEST flat or worse; a delta that swings sign when you reshuffle the split; a corpus under ~50 val tasks paired with a "ship" recommendation; GEPA's val score and the held-out TEST score being implausibly close (leakage).

**Phase to address:**
The corpus-construction + split phase (must precede the GEPA run). The `val_size>50` gate and TEST-sequestration guard live here.

---

### Pitfall 3: Attribution failure — the improvement came from the LM, not the skill text

**What goes wrong:**
The optimized agent scores higher on task-success, the team attributes it to the new SKILL.md steering, and adopts it — but the delta actually came from the agent/LM (a better model checkpoint, more turns, a longer context budget, prompt scaffolding unrelated to the skill). Without a control, you cannot tell "the skill text helped" from "DeepSeek-V4 is just better than what v2.2 measured." This is the deeper version of the v2.2 lesson: `choice_rate` at least measured *helix-choice*; task-success measures *task completion*, which is dominated by the LM's raw coding ability, so the skill's marginal contribution is easily drowned out and mis-credited.

**Why it happens:**
There's only one arm in the experiment (optimized skill ON). Task-success is a coarse, high-variance signal where the skill text is a small term; the LM and harness scaffold are the large terms.

**How to avoid:**
- **Mandatory ON-vs-OFF control arm.** Run every evaluation as a paired comparison: skill-steering **present** vs **absent** (same model, same turns, same tasks, same seeds), and report `delta = success(ON) − success(OFF)`. Adopt only if ON beats OFF by a material margin on held-out TEST. A bare ON number is not adoptable. The agent harness must expose a first-class `--steering on|off` (or skill-injected vs baseline-prompt) switch so the two arms are identical except for the skill text.
- The ON/OFF switch is itself a new gate that needs an anti-vacuity test (Pitfall 9): prove the OFF arm genuinely omits the skill text (assert the rendered prompt in OFF mode does NOT contain the steering, and ON mode does).

**Warning signs:**
Reported deltas with no OFF baseline; ON and OFF arms differing in model/turns/seed (confounded); a "win" that doesn't reproduce when you re-run OFF; the delta being within run-to-run variance of the OFF arm alone.

**Phase to address:**
The agent-harness phase (the agent must support ON/OFF from the start) and the metric phase (delta = ON−OFF). Wire the control before the GEPA loop.

---

### Pitfall 4: Gating the optimizer PROCESS instead of the committed ARTIFACT (non-determinism)

**What goes wrong:**
LLM optimization is not bit-reproducible — GEPA's reflective evolution, sampling temperature, API non-determinism, and provider drift mean two runs produce different optimized text. If a phase gate or CI tries to assert "the optimizer produces X" or re-runs `optimize.py` in the merge path, the gate is permanently flaky and pulls Python + API keys + cost into CI — violating the single-binary / off-the-merge-path invariant.

**Why it happens:**
Instinct says "test the thing you built." But the thing built is a stochastic process; only its *output*, once a human accepts it, is a stable artifact.

**How to avoid:**
- **Gate the artifact, never the process.** The committed artifact is `reference.md` (generated) and the human-edited `SKILL.md`; the gate is `helix-refgen --check` (the v2.2 invariant: `reference.md` is generated-not-hand-edited and must be regenerated, not hand-patched). `optimize.py` output stays git-ignored (`output/optimized.json`) and re-enters ONLY via a human-reviewed SKILL.md edit + refgen + `--check`. No CI step runs the optimizer.
- **The GEPA run is dev-time-deferred and key-guarded**: an unset-API-key guard prints an informative message and exits 0 (never crashes, never silently skips into a false pass) — carry the Phase 106 unset-key pattern.
- Keep `optimize.py` off `go test ./...` and `go.mod` via the `toolsquarantine` analyzer (Pitfall 5).

**Warning signs:**
A CI job that needs `DEEPSEEK_API_KEY`/`OPENAI_API_KEY`; a test asserting optimizer output equality; `helix-refgen --check` not in the gate set; flaky "optimizer changed" diffs in PRs.

**Phase to address:**
The refgen/adoption phase and the harness phase. `helix-refgen --check` must be the only adoption gate; reaffirm in the boundary-analyzer phase.

---

### Pitfall 5: Boundary leaks — Python/DSPy/agent code reaching `internal/` or the merge path

**What goes wrong:**
The dev-time agent + DSPy harness accidentally couples into the shipped binary: a `tools/*.go` file gets imported by a runtime/cmd package, a `helix` subcommand shells to Python, `go.mod` gains a dependency, or the agent code lands somewhere `go test ./...` compiles it. Any of these breaks the "no runtime Python / single binary" invariant. A subtler variant: the *Go-side* agent harness (`bench/runtime/...`) is legitimately in-tree and HELIX_BIN-gated, but its tests skip silently without a binary, giving false-green `go test ./...` (the `helix-bench-smoke-false-green` lesson — `resolveHelixBin()` returns "" and tests PASS-as-skipped).

**Why it happens:**
The agent naturally wants to call helix internals; the quarantine is a discipline, not a default. The HELIX_BIN skip is invisible because a skipped test is a green test.

**How to avoid:**
- **Carry the `toolsquarantine` go/analysis analyzer** (Phase 106): no package outside `github.com/agenthands/helix/tools/...` may import the `tools/...` tree; wired into `make vet` via `cmd/vet-tools-quarantine`. The Python DSPy + the offline optimizer agent live under `tools/`; the in-tree Go *bench* agent (if added under `bench/runtime/`) is NOT under the quarantine and must instead respect the existing `nokernel2semantic`/`ablationleakage` boundaries.
- **Decide the agent's home explicitly:** Python optimizer-driver agent → `tools/` (quarantined, off go.mod). A Go subprocess agent that drives `helix` verbs → `bench/runtime/` alongside `aider_edit_agent.go` / `subprocess/claude.go`, in-tree but **HELIX_BIN-gated**. State which, so the boundary analyzer covers the right tree.
- **Bench gate runs with `HELIX_BIN` exported, not bare `go test ./...`.** Any phase touching `bench/` must `go build -o ./helix ./cmd/helix` then `HELIX_BIN="$(pwd)/helix" go test ./bench/runtime/...`; runner tests further need `helix` on PATH. Do not accept "go test ./... all pass" as verification for bench changes.

**Warning signs:**
`go.mod` diff in a v2.3 PR; `grep -rl python cmd/ internal/` hits; `make vet` newly failing; a bench test that "passes" but logs `SKIP` for lack of HELIX_BIN; `cmd/helix-bench` runner tests failing `exec: "helix" not found`.

**Phase to address:**
The agent-scaffolding phase (place it correctly, extend the analyzer) and every bench-touching phase (HELIX_BIN gate in the success criteria).

---

### Pitfall 6: SWE-bench / Podman harness setup gotchas (and the dataset-name drift)

**What goes wrong:**
The SWE-bench arm silently fails to a vacuous pass (feeding Pitfall 1) or is wrongly reported "blocked" because of three concrete footguns:
- **Dataset org-name drift.** Code/docs reference `github.com/SWE-bench/SWE-bench` (the repo moved to the `SWE-bench` org), but the Hugging Face *datasets* are still under `princeton-nlp/` (`princeton-nlp/SWE-bench_Verified`, `princeton-nlp/SWE-bench_Lite`). Mixing the two (`SWE-bench/SWE-bench_Verified` as a dataset id) → 404 → empty task set → vacuous pass. This is already a live tension in-tree (the swebench evaluator cites the `SWE-bench/` repo while datasets are pinned elsewhere), and HuggingFace 404s already cause a pre-existing `cmd/helix-bench` test failure.
- **Podman socket not started / wrong DOCKER_HOST.** The upstream `swebench` Python harness speaks the Docker API; it needs Podman's docker-compat socket: `podman system service --time=0 &` then `DOCKER_HOST=unix://$XDG_RUNTIME_DIR/podman/podman.sock`. If the socket isn't up, the harness errors — which must fail-not-skip.
- **Rootless Podman + `--network=none`.** Rootless permission quirks and the `--network=none` isolation (used for hermetic runs) can make image pulls or test execution fail in ways that look like "0 tests passed."

**Why it happens:**
The repo uses **Podman, not Docker** (`helix-container-engine-podman`); upstream harnesses assume Docker. The dataset/repo org split is a genuine naming trap. None of these surface as an exception by default — they surface as an empty or errored run that the oracle coerces to a benign result.

**How to avoid:**
- **Never report "Docker not installed → blocked"** — `bench/container` auto-detects Podman (`Engine.Detect` / `TestDetectFindsPodmanWhenNoDocker`). Stand up `podman system service --time=0 &` + `DOCKER_HOST=...podman.sock` for the upstream harness; this is configuration, not a blocker.
- **Pin dataset ids to the `princeton-nlp/` org** (or whatever the loader's pinned SHA resolves) and assert resolution: a dataset fetch that 404s is a hard ERROR, and the task-set size must be asserted `> 0` before grading (the fail-not-skip discipline from Pitfall 1).
- **Reuse the existing pinned-SHA loaders** (`bench/datasets/swebench-utboost`, `multi-swe-bench-mini`) which already refuse mutable refs; extend rather than re-fetch by mutable name.

**Warning signs:**
HTTP 404 in dataset fetch logs; `DOCKER_HOST` unset while running the Python harness; `--network=none` runs with 0 collected tests; "blocked for lack of Docker" in a status note; task counts that don't match the dataset's known instance count.

**Phase to address:**
The SWE-bench integration phase (dataset pinning + Podman socket setup + task-count assertion as success criteria).

---

### Pitfall 7: Provider gotchas — DeepSeek deprecation, tool-call flakiness, missing-key silent skip

**What goes wrong:**
- **Dated model deprecation.** DeepSeek's `deepseek-chat` and `deepseek-reasoner` aliases are **retired after 2026-07-24 15:59 UTC** (they map onto `deepseek-v4-flash` non-thinking/thinking modes); after that, requests using those names FAIL. A milestone that hard-codes `deepseek-chat` will break mid-flight.
- **Tool-call flakiness in long loops.** DeepSeek's own docs warn that `deepseek-chat` function-calling can be **unstable → looped calls or empty responses**, and that the model may emit invalid JSON or hallucinate parameters. In a multi-turn ReAct loop driving `helix` verbs, this means stuck loops, empty tool args, or malformed verb invocations.
- **Missing API key → silent skip instead of fail.** `ANTHROPIC_API_KEY` is unset by design; if `DEEPSEEK_API_KEY` is also unset and the harness "skips" (or falls through to OFF-arm scoring) instead of failing loudly, you get a false-green run with no agent activity (a sibling of Pitfall 1's vacuity).

**Why it happens:**
Model aliases are convenient and get hard-coded; long agent loops amplify rare per-call tool-format failures; key-guards are written to be lenient so local runs don't crash.

**How to avoid:**
- **Pin the concrete model id and record the deprecation date.** Prefer the explicit `deepseek-v4-*` id over the soon-retired alias; add a comment/assert tying the chosen id to the 2026-07-24 cutoff so it's not silently stale. Keep **OpenAI as a configured fallback** (per the locked decision) and exercise the fallback path.
- **Defensive tool-call handling:** validate tool-call JSON before dispatching to a `helix` verb (reject/repair invalid args, never pass through), cap the ReAct loop with a max-turns + no-progress detector (kill stuck/empty-response loops), and on repeated empty responses fall back to OpenAI. Treat malformed-args / loop-exhaustion as a task FAILURE, not a pass.
- **Missing-key is fail-not-skip for a real run, exit-0-informative for the hermetic/LM-free gates.** Distinguish the two: the GEPA optimization RUN needs a key and should refuse loudly if asked to actually run without one; the committed unit gates (parity, split, degenerate, ON/OFF-render) must run with NO key and NO network.

**Warning signs:**
`deepseek-chat` literal in code near mid-2026; agent runs that hang or produce empty turns; tasks "completing" in zero turns; a run summary with no recorded tool calls; CI green with no `DEEPSEEK_API_KEY` present.

**Phase to address:**
The agent-harness phase (model pinning, fallback, defensive tool handling, key-guard semantics).

---

### Pitfall 8: Auto-adopting optimized steering (bypassing the human gate)

**What goes wrong:**
`optimize.py` (or a future convenience) writes the optimized text straight into `internal/cli/skills/helix/SKILL.md` or `reference.md`, shipping un-reviewed, possibly degenerate steering into the binary's embedded skill bundle. Compounded by the `helix-skill-embed-ships-whole-dir` lesson: anything dropped into the skill dir leaks into the binary.

**Why it happens:**
Closing the loop end-to-end is tempting; the optimizer "knows" the better text, so why not write it.

**How to avoid:**
- **No auto-adopt, ever.** Carry the Phase 106 invariant: `optimize.py` writes ONLY git-ignored `output/optimized.json`; it must NOT contain a write path to `skills/helix` or `reference.md` (the acceptance grep `grep -E 'skills/helix|reference\.md' optimize.py == 0`). Adoption is a human SKILL.md edit (within the ≤1536-char budget, `## Decision matrix` anchor preserved) followed by `helix-refgen --check`.
- Keep the skill bundle allowlist (`bundleFiles`, v2.2 BL-SKILL-01) so stray optimizer artifacts can't leak into the binary even if mis-placed.

**Warning signs:**
A write-mode `open()` to the skills dir in `optimize.py`; `reference.md` diffs not produced by refgen; `helix-refgen --check` failing in CI; new files in the skill bundle.

**Phase to address:**
The refgen/adoption phase; reaffirm in the boundary-analyzer phase.

---

### Pitfall 9: Anti-vacuity for THIS milestone's new gates (the dominant recurring failure)

**What goes wrong:**
A new gate is written, named like a real invariant check, passes green — and tests nothing. v2.2 shipped two such defects that the plan-checker approved and only code-review/verifier *mutation-testing* caught: a tautological `x==x` Guard B (Phase 104) and a Python↔Go parity bug invisible to a point-wise corpus (Phase 106). The plan-checker reviews test *structure/intent/names*; it cannot tell a discriminator from a tautology. v2.3 adds at least three new gates that are each prone to vacuity.

**Why it happens:**
Green-path-only tests are easy to write and look complete; the break-the-invariant arm is extra work and easy to omit.

**How to avoid:**
**Every new gate ships a deliberate break-the-invariant → assert-RED test, and code-review+fix is folded BEFORE verify** (so the verifier validates post-fix code; the verifier and reviewer independently mutation-test each guard). Concretely, per new gate:

| New v2.3 gate | Break-the-invariant test it MUST ship |
|---|---|
| **Task-success oracle (honest pass)** | Plant a *vacuous* task (empty test set / 0 collected tests / unresolved dataset) → assert the oracle ERRORs, does NOT report "pass". Separately: plant a diff that touches a test file → assert rejected. Plant a FAIL_TO_PASS regression → assert fail. |
| **"Used helix" attribution** | Plant a transcript that passes the task with ZERO `helix` verb calls → assert it does NOT count as a helix-attributed success. |
| **ON/OFF control arm** | Assert the OFF-arm rendered prompt does NOT contain the steering text and the ON-arm DOES (mutate: swap the flag → the prompts swap); a delta computed from two identical arms must be provably ~0. |
| **TEST-split sequestration** | Leak a TEST task into TRAIN → assert the split guard goes RED (carry Phase 106 `test_split.py`). |
| **`val_size>50` adoption gate** | Set `val_size=50` (boundary) → assert no-ship; `val_size=51` with a real delta → assert adoptable. |
| **Degenerate-steering inspection** | Flag/no-flag pair: unconditional always-helix text flagged, conditional steering not (carry + broaden Phase 106 `test_degenerate.py`; honestly scope it as a smell test, not sound). |
| **`toolsquarantine` boundary analyzer (extended)** | Planted runtime→`tools/` import + `// want` analysistest fixture → assert RED; stripping `// want` fails the test (carry Phase 106 fixture). |
| **`helix-refgen --check` adoption gate** | Hand-edit `reference.md` out of sync → assert `--check` fails non-zero. |

**Warning signs:**
A test whose assertion compares a value to itself or to a constant it just computed the same way; a "parity" corpus that only pins agreement points (a point-wise corpus only proves the points it pins — scope the claim or widen the domain); a gate with no corresponding RED fixture; a reviewer/verifier that didn't mutation-test.

**Phase to address:**
EVERY phase that introduces a gate. Make "ships a break-the-invariant test, mutation-confirmed by review+verify" an explicit success criterion on each.

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Reuse the v2.2 8+3 `choice_rate` corpus for task-success | No corpus-building cost | Reproduces the no-ship; wastes real container/API spend confirming noise | Never for an adoption decision; OK only to smoke-test plumbing |
| Read pass/fail from a bare exit code | Simple oracle | Vacuous passes (empty test set, swallowed collection error) | Never — must assert N>0 tests ran + FAIL_TO_PASS/PASS_TO_PASS |
| Hard-code `deepseek-chat` | Works today | Breaks after 2026-07-24; flaky tool-calls | Never — pin `deepseek-v4-*`, keep OpenAI fallback |
| Run the optimizer in CI to "test it" | End-to-end coverage | Permanently flaky, pulls Python+keys+cost into merge path | Never — gate the artifact via `--check`, not the process |
| Lenient missing-key skip on the real run | Local runs don't crash | False-green runs with no agent activity | OK only for the LM-free hermetic gates; the real GEPA run must refuse loudly |
| Single ON arm, no OFF baseline | Half the runs/cost | Mis-attributes LM gains to the skill; adopts noise | Never for an adoption decision |

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| SWE-bench datasets (HF) | Using `SWE-bench/SWE-bench_Verified` as a dataset id (repo org ≠ dataset org) | Pin `princeton-nlp/SWE-bench_*` (or the loader's pinned SHA); assert fetch resolves and task-count > 0 |
| Podman as Docker backend | "Docker not installed → blocked" | `podman system service --time=0 &` + `DOCKER_HOST=unix://$XDG_RUNTIME_DIR/podman/podman.sock`; `bench/container` auto-detects podman |
| DeepSeek API | Hard-coded `deepseek-chat`; trust tool-call JSON | Pin `deepseek-v4-*`; validate/repair tool-call JSON; cap loop turns; OpenAI fallback |
| `helix` CLI from the agent | Driving via MCP or assuming a daemon shape | Drive `helix <verb>` subprocesses (the product surface); reuse `bench/runtime/subprocess` + `forwarder.Session` patterns |
| Bench tests | `go test ./...` and call it verified | `go build -o ./helix`; `HELIX_BIN="$(pwd)/helix" go test ./bench/runtime/...`; runner tests need `helix` on PATH |
| DSPy / Python | Adding to `go.mod` or shelling from a `helix` subcommand | Quarantine under `tools/`; `toolsquarantine` analyzer + `make vet`; git-ignored output |

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| GEPA hot loop × container task = cost/time blowup | Hours-long runs, large API bills | Cap turns + tasks; cache container images; small val for dev, full only for the adoption run; budget per run | As corpus grows toward `val_size>50` |
| API rate-limit / timeout storms in the loop | 429s, hung turns, partial runs scored as fail | Backoff + retry with cap; per-turn timeout; treat exhaustion as task-fail (not pass); checkpoint progress (`bench/longwall` pattern) | Under concurrent task fan-out |
| Container build/pull per task | Slow cold runs | Reuse pinned mirror; `--network=none` for hermetic runs after warm pull | First run / CI cold cache |

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| API key committed to a `tools/` file or test fixture | Leaked DeepSeek/OpenAI credentials | Key from dev env only; no key in any committed file (Phase 106 T-106-05); gitignore output |
| Agent runs with write access to the grading oracle | Test tampering = inflated pass rate | Restore/`git checkout` gold tests before grading; reject diffs touching test files; run grader from agent-unwritable path |
| Untrusted task repos executed locally | Arbitrary code from SWE-bench instances runs on host | Run in Podman containers (`--network=none` where possible); never execute task code on the host |
| Raw optimizer dump shipped | Un-reviewed steering in the binary | No auto-adopt; output git-ignored; human SKILL.md edit + `--check`; bundle allowlist |

## UX Pitfalls

(Dev-facing — the "users" here are Helix maintainers running the harness.)

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| No-ship treated as a failure | Pressure to ship noise to "succeed" | Document no-ship as a legitimate, success-meeting outcome (carry Phase 106 REPORT framing) |
| Silent skip on missing key/binary | Maintainer thinks the suite passed | Fail-not-skip for real runs; informative exit-0 only for hermetic gates; print what ran |
| Unclear ON/OFF/delta reporting | Can't tell if the skill helped | Report ON, OFF, delta, val_size, and adopt/no-ship verdict explicitly in REPORT.md |

## "Looks Done But Isn't" Checklist

- [ ] **Task-success oracle:** often missing the N>0-tests assertion and FAIL_TO_PASS/PASS_TO_PASS contract — verify a vacuous task ERRORs, not "passes."
- [ ] **Pass attribution:** often missing the "did the agent actually call helix?" check — verify a zero-helix-call pass is not credited.
- [ ] **ON/OFF control:** often missing entirely (single arm) — verify the delta is ON−OFF and the OFF prompt provably omits the steering.
- [ ] **TEST sequestration:** often leaks into `compile()` — verify `test.jsonl` is never passed to the optimizer and the split guard RED-fails on a planted leak.
- [ ] **`val_size>50` gate:** often a doc note, not a gate — verify the harness no-ships below it.
- [ ] **Boundary analyzer:** often not extended to the new agent tree — verify a planted runtime→`tools/` import fails `make vet`.
- [ ] **Bench verification:** often "go test ./... passed" — verify it was run with `HELIX_BIN` exported (else it's false-green).
- [ ] **Model id:** often the soon-retired `deepseek-chat` alias — verify a `deepseek-v4-*` pin + OpenAI fallback.
- [ ] **Every new gate:** often green-path-only — verify each ships a break-the-invariant RED test, mutation-confirmed by review+verify.

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| Adopted degenerate/noise steering | MEDIUM | Revert the SKILL.md edit; regenerate `reference.md` via refgen; re-run ON/OFF on held-out TEST to confirm |
| Vacuous-pass oracle shipped | HIGH | Audit all "passed" tasks for N>0 tests + FAIL_TO_PASS; re-grade; add the missing RED gate; discard tainted deltas |
| TEST leakage into compile() | HIGH | Discard the reported delta (unattributable); re-split with sequestration; re-run; add the leak RED guard |
| `deepseek-chat` retired mid-milestone | LOW | Swap to pinned `deepseek-v4-*` id; re-run; OpenAI fallback covers the gap |
| Boundary leak into binary/go.mod | MEDIUM | `make vet` locates it; move code under `tools/` or HELIX_BIN-gate it; revert go.mod |

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| 1. Reward-hacking the oracle | Task-success oracle phase (~107–108) | Vacuous-task ERRORs; FAIL_TO_PASS/PASS_TO_PASS enforced; test-file diffs rejected; degenerate-text flagged |
| 2. Tiny-corpus overfit | Corpus + split phase (pre-GEPA) | `val_size>50` gate no-ships below floor; TEST-leak RED guard fails on a plant |
| 3. Attribution failure | Agent-harness + metric phase | ON/OFF arms identical-except-steering; delta = ON−OFF; OFF prompt provably omits steering |
| 4. Gating the process | Refgen/adoption phase | `helix-refgen --check` is the only adoption gate; no optimizer in CI; unset-key exits 0 |
| 5. Boundary leaks | Agent-scaffolding phase + every bench phase | `toolsquarantine` `make vet` green; `go.mod` 0-diff; bench run with HELIX_BIN exported |
| 6. SWE-bench/Podman/dataset drift | SWE-bench integration phase | Dataset id pinned to `princeton-nlp/`; fetch resolves; task-count>0; Podman socket up |
| 7. Provider gotchas | Agent-harness phase | `deepseek-v4-*` pinned; OpenAI fallback exercised; tool-call JSON validated; loop capped; missing-key fails loudly on real run |
| 8. Auto-adopt | Refgen/adoption phase | `grep -E 'skills/helix|reference\.md' optimize.py == 0`; adoption is human SKILL.md edit + `--check`; bundle allowlist |
| 9. Anti-vacuity (all new gates) | EVERY gate-introducing phase | Each gate ships a break-the-invariant RED test; code-review+fix folded BEFORE verify; reviewer+verifier mutation-test |

## Sources

- Phase 106 spike artifacts (HIGH — in-repo authoritative): `106-01-SUMMARY.md`, `106-02-SUMMARY.md`, `106-REVIEW.md`, `106-VERIFICATION.md`, `tools/dspy-tune/REPORT.md` — no-ship cause (`MinTasks=5`, tiny split), TEST-sequestration pattern, degenerate-steering flag/no-flag pair, `toolsquarantine` analyzer + RED fixture, unset-key guard, no-auto-adopt invariant, parity-corpus point-wise limitation.
- Memory (HIGH — project-specific lessons): `helix-antivacuity-mutation-test-guards` (tautological Guard B 104, Python↔Go parity bug 106, fold review+fix before verify); `helix-bench-smoke-false-green` (HELIX_BIN fail-not-skip, SIGKILL-vacuous-gate); `helix-container-engine-podman` (Podman auto-detect, `DOCKER_HOST` socket, never "Docker → blocked"); `helix-skill-embed-ships-whole-dir` (bundle allowlist).
- PROJECT.md v2.3 milestone section (HIGH — source of truth): locked decisions (CLI-subprocess transport, Aider+SWE-bench scope, no-runtime-Python, DeepSeek primary / OpenAI fallback, human-gated refgen adoption).
- [DeepSeek API — Tool Calls / Function Calling docs](https://api-docs.deepseek.com/guides/tool_calls) (HIGH): `deepseek-chat`/`deepseek-reasoner` alias retirement after 2026-07-24 15:59 UTC; function-calling instability (looped calls / empty responses); validate args before dispatch.
- [SWE-bench datasets (Hugging Face, `princeton-nlp/` org)](https://huggingface.co/datasets/princeton-nlp/SWE-bench_Verified) + [SWE-bench evaluation guide](https://www.swebench.com/SWE-bench/) (HIGH/MEDIUM): datasets remain under `princeton-nlp/` while the repo moved to the `SWE-bench/` org (the naming-drift trap); FAIL_TO_PASS/PASS_TO_PASS resolution contract.
- In-repo grounding for dataset drift: `bench/evaluators/swebench/predictions.go` cites `github.com/SWE-bench/SWE-bench`; `bench/datasets/swebench-utboost/`, `multi-swe-bench-mini/` pin dataset SHAs (mutable-ref refusal).

---
*Pitfalls research for: task-success-driven skill optimization (v2.3, Helix)*
*Researched: 2026-06-24*

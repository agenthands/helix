# Stack Research

**Domain:** Go single-binary CLI tooling + (exploratory) dev-time/offline Python prompt-optimization harness
**Researched:** 2026-06-23
**Confidence:** HIGH (DSPy version/extras/LM-config verified against PyPI + dspy.ai; Go-side surfaces read directly from the tree)

## Scope

v2.2 "Agent-Facing Skill Quality & Prompt Tuning" is **mostly a content/codegen milestone, not a stack milestone.** Three of its four features (SKILL.md rewrite, `cmd/helix-refgen` fixes, `installSkill` allowlist) need **ZERO new dependencies** — they are pure Go edits inside packages that already exist. The only feature that introduces anything new is the **exploratory, dev-time-only DSPy harness**, and even that must stay strictly out of the shipped binary, the Go module graph, and the merge-gating CI path.

The central constraint from `PROJECT.md` (line 192): *"Helix stays a Go single binary with no Python/runtime deps — DSPy is dev-time/offline only; its output is committed and gated by `helix-refgen --check`."* This research's primary job is to honor that.

## Recommended Stack

### Core Technologies

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| Go | 1.25.1 (existing) | All shipped code: SKILL.md rewrite is content; `cmd/helix-refgen` + `installSkill` are Go | No change. The three non-DSPy features touch only existing packages (`cmd/helix-refgen`, `internal/cli/skill.go`, `internal/cli/skills/helix/`). No new Go import is required or wanted. |
| DSPy (Python) | **3.2.1** (stable, PyPI 2026-05; `3.3.0b1` beta available) | The exploratory **offline** prompt-optimizer that tunes SKILL.md decision-matrix / nudge text against the adoption scorecard | DSPy is the de-facto framework for *programmatic* prompt optimization with a measurable metric. It separates the program (signature) from the optimizer and optimizes against a `metric(example, prediction) -> float` — which maps exactly onto the existing `choice_rate`/`fallback_rate` scorecard. It is **dev-time only**; its *output* is committed text, so the runtime stays pure-Go. |
| Python | **>=3.10, <3.15** (DSPy's own pin) | Interpreter for the DSPy harness only | DSPy requires Python ≥3.10. `python3` is already invoked in one Makefile CI helper (`Makefile:297`), so a base interpreter is already assumed in some CI lanes — but `pip`/venv/DSPy must remain an **opt-in dev target**, never on the default `go test ./...` / merge path. |

### Supporting Libraries

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `dspy[anthropic]` extra | pulled by DSPy 3.2.1 | Anthropic provider wiring for the optimization LM | Use because the existing Go harness already keys on `ANTHROPIC_API_KEY` (`test/oracle/llm/client.go:18`). DSPy → LiteLLM reads the **same** `ANTHROPIC_API_KEY` env var, so the dev sets one key for both the Go scorecard and the Python optimizer. |
| GEPA (`dspy.GEPA`, or standalone `gepa`) | bundled with DSPy 3.x (GEPA 0.1.x) | Reflective prompt-evolution optimizer — the recommended optimizer for *instruction/prose* tuning (which is exactly what SKILL.md text is) | Prefer GEPA over MIPROv2 for this task: GEPA optimizes free-form instruction text via reflective evolution (ICLR 2026), needs far fewer rollouts, and does not require few-shot demonstration sets — SKILL.md is prose, not a demo bank. Already integrated as `dspy.GEPA`. |
| `dspy[optuna]` extra (Optuna) | pulled by extra | Bayesian search backend for **MIPROv2** only | Only if you fall back to MIPROv2 (`dspy.MIPROv2`) instead of GEPA. Optuna is a required dep of MIPROv2/BootstrapFewShotWithOptuna. Skip it if you use GEPA. |
| LiteLLM | transitively via DSPy | Provider normalization layer DSPy calls under the hood | Not chosen directly — it is DSPy's transitive dep. Relevant only because it is what reads `ANTHROPIC_API_KEY` and accepts the `anthropic/claude-…` / `deepseek/deepseek-chat` model strings, mirroring the Go harness's provider switch. |

### Development Tools

| Tool | Purpose | Notes |
|------|---------|-------|
| `uv` **or** `python -m venv` + `pip` | Isolate the DSPy install away from the Go build | Recommended: a dedicated venv under e.g. `tools/promptopt/.venv` (git-ignored) created by an **opt-in** `make promptopt-setup`. `uv` is faster but optional; plain venv keeps the bar low. The venv MUST be git-ignored and never referenced by any default `make test` / `go test` target. |
| `requirements.txt` (pinned) | Reproducible harness install: `dspy==3.2.1` (+ extras) | Pin the exact DSPy version so the optimizer is reproducible across dev machines. Lives next to the Python harness (e.g. `tools/promptopt/requirements.txt`), NOT in repo root. |
| `cmd/helix-refgen --check` (existing) | The re-entry gate: optimizer output is committed, then this gate proves it is byte-reproducible | The DSPy harness does NOT write `reference.md`/SKILL.md directly to the embed path as a side effect of CI. The loop is: human runs the optimizer offline → reviews the proposed text → commits it (or feeds tuned prose into the generator templates) → the **existing** `helix-refgen --check` gate (Phase 97) keeps `reference ⊇ VerbToolNames()` true. |

## Integration Points (how the Python harness re-enters the Go world)

This is the load-bearing part of the design — the seam between the offline optimizer and the committed, `--check`-gated artifacts.

1. **Metric source (Go ↔ Python):** the optimizer's metric is the Phase 101 adoption scorecard. Two viable wirings, in preference order:
   - **(Preferred) Re-implement the trivial classifier in Python, validate against the Go scorer.** `test/oracle/adopt` is build-tag-FREE and runs hermetically. Its `ClassifyChoice` rule is ~10 lines: strip code fences/backticks, take the FIRST command line, `HasPrefix("helix ")` for a choice vs the `{"grep ","sed ","cat ","find ","rg ","ls "}` fallback set (`scorecard.go:77,85`). Re-implement that exactly in the Python `metric` so the harness is self-contained, and treat the Go `adopt` package as the **authoritative** scorer the *committed* result is finally validated against.
   - Alternatively, shell the Python `metric` out to a thin Go CLI/test shim over `adopt.Scorecard` for a single source of truth — heavier wiring, only worth it if the classifier ever stops being frozen.
   - Live transcript generation (model emitting a first command given a candidate SKILL.md) reuses the **same** Anthropic/DeepSeek providers as `test/oracle/llm` — same `ANTHROPIC_API_KEY` / `DEEPSEEK_API_KEY` env vars.
2. **Optimization target (what DSPy mutates):** the SKILL.md **decision-matrix text** and/or the `PreToolUse` nudge-steering prose. DSPy proposes candidate instruction strings; the metric scores each candidate by running it through the (model → first-command → scorecard) loop and maximizing `choice_rate` (equivalently minimizing `fallback_rate`).
3. **Output landing zone (Python → Go):** the optimizer writes a **proposed** SKILL.md body / generator-template snippet to a dev scratch path (e.g. `tools/promptopt/out/`). A human reviews it, then either (a) commits the tuned SKILL.md directly, or (b) feeds tuned per-verb prose into `cmd/helix-refgen`'s render templates and regenerates `reference.md`. **The DSPy harness never writes into `internal/cli/skills/helix/` in CI.**
4. **Re-entry gate (the contract that keeps the binary honest):** after the tuned text is committed, the **existing** gates enforce — `helix-refgen --check` (drift), the `reference ⊇ VerbToolNames()` contract (`internal/cli/reference_contract_test.go`), and the new v2.2 `installSkill` bundle allowlist test. DSPy adds **no** new CI gate on the default path.

## Go-side additions for the SKILL/reference rewrite + allowlist

**Confirmed: NONE new.** Verified against the tree:

- `cmd/helix-refgen/{main.go,render.go}` already exists and owns reference.md rendering + the `--check` gate (`main.go` header + blank-import parity rule). The "use this / not that" + "Output" copy-paste fixes (per `SKILL-ISSUE.md`) are edits to `render.go`'s per-verb / per-`groupID` text and templates — no new import.
- `internal/cli/skill.go` already holds `installSkill`, the `//go:embed skills/helix/*` FS (`skill.go:21`), and the containment/atomicity logic. The "bundle allowlist" is a code edit: replace the `embeddedSkillFS.ReadDir("skills/helix")` **walk** (which currently ships *every* file in the dir) with an explicit `{"SKILL.md","reference.md"}` allowlist plus a bundle-contents test. No new dependency — `embed`, `os`, `path/filepath` are already imported.
- The adoption contract test (`internal/cli/reference_contract_test.go`) and scorer (`test/oracle/adopt`) already exist and need no new deps.

So the entire Go surface of v2.2 is edits inside already-vendored packages. `testify`, `cobra`, `jsonschema/v6`, `anthropic-sdk-go` are all already in `go.mod`; nothing is added.

## Installation

```bash
# Go side: nothing new. Existing build/test pipeline is unchanged.
make build && make test

# Reference/skill regen + drift gate (existing, unchanged):
go run ./cmd/helix-refgen            # regenerate reference.md
go run ./cmd/helix-refgen --check    # CI drift gate

# ---- Exploratory DSPy harness: OPT-IN, dev-time only, isolated venv ----
# (lives under e.g. tools/promptopt/, git-ignored .venv, never on default test path)
python3 -m venv tools/promptopt/.venv
. tools/promptopt/.venv/bin/activate
pip install -r tools/promptopt/requirements.txt   # pins: dspy==3.2.1  (+ extras below)
#   requirements.txt content:
#     dspy[anthropic]==3.2.1          # GEPA bundled as dspy.GEPA
#     # dspy[optuna]==3.2.1           # ONLY if falling back to MIPROv2

# Run the optimizer offline (reads the SAME key the Go harness uses):
export ANTHROPIC_API_KEY=...          # or DEEPSEEK_API_KEY for the cheaper leg
python tools/promptopt/optimize_skill.py   # writes proposed text to tools/promptopt/out/
# Human reviews out/, commits tuned SKILL.md / regenerates reference.md, then:
go run ./cmd/helix-refgen --check     # the committed artifact must pass the existing gate
```

DSPy LM config inside the harness (verified format):

```python
import os, dspy
# Same env var the Go scorecard keys on (test/oracle/llm/client.go:18).
lm = dspy.LM("anthropic/claude-…", api_key=os.environ["ANTHROPIC_API_KEY"])
# Cheaper optimization leg, mirroring the Go DeepSeek provider:
# lm = dspy.LM("deepseek/deepseek-chat", api_key=os.environ["DEEPSEEK_API_KEY"])
dspy.configure(lm=lm)
optimizer = dspy.GEPA(metric=adoption_metric)   # metric = choice_rate-driven scorer
```

## Alternatives Considered

| Recommended | Alternative | When to Use Alternative |
|-------------|-------------|-------------------------|
| DSPy `GEPA` optimizer | DSPy `MIPROv2` (+ Optuna) | Use MIPROv2 if you later want to optimize few-shot *demonstrations* (example banks) rather than free-form instruction prose. For SKILL.md text (prose), GEPA is the better fit and needs no Optuna. |
| DSPy framework | Hand-rolled prompt-search loop in Go | A pure-Go loop over the existing `adopt` scorer avoids Python entirely — viable if the team wants zero Python. But it forfeits DSPy's reflective optimization and the GEPA literature. Given the feature is explicitly *exploratory*, DSPy is the right first bet; the Go scorer remains the authority. |
| Isolated venv (`tools/promptopt/.venv`) | Conda / system pip install | Conda/system installs leak DSPy into the dev environment and risk it drifting onto a CI lane. A git-ignored venv keeps the blast radius to one directory. |
| DSPy 3.2.1 (stable) | DSPy 3.3.0b1 (beta) | Pin stable 3.2.1 for reproducibility. Only move to 3.3.x once it leaves beta and you re-verify the GEPA/LM-string API. |
| Reuse `ANTHROPIC_API_KEY`/`DEEPSEEK_API_KEY` | Add an OpenAI leg | OpenAI works via `dspy.LM("openai/…")`, but the Go harness has no OpenAI provider — adding one splits the key surface. Stay on the two providers the scorecard already supports. |

## What NOT to Use / NOT to Add

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| **Any runtime Python dependency in the shipped binary** | Helix's entire identity is a single Go binary with zero Python/Docker/runtime deps (CLAUDE.md, PROJECT.md). DSPy at *runtime* would violate the product thesis. | DSPy strictly **dev-time/offline**; only its *committed text output* enters the binary, gated by `helix-refgen --check`. |
| **New Go module dependencies** | The three non-DSPy features are edits to existing packages; no new import is needed. Adding Go deps for a content/codegen milestone is pure risk. | `embed`, `os`, `path/filepath`, `cobra`, `testify`, `jsonschema/v6`, `anthropic-sdk-go` — all already vendored. |
| **DSPy/pip on the default CI / `go test ./...` path** | A merge-gating job that needs DSPy would (a) require Python+pip+API-key in CI and (b) make merges depend on a live LLM — both forbidden by the milestone's "no runtime/CI Python requirement" intent. The adoption *score* is opt-in and never blocks merge (v2.1 contract). | An **opt-in** `make promptopt-*` target run by a human locally; default CI keeps only the deterministic `helix-refgen --check` + bundle-allowlist + `reference ⊇ VerbToolNames()` gates. |
| **Letting the DSPy harness write directly into `internal/cli/skills/helix/`** | A side-effecting optimizer that mutates the embed path turns generated/committed artifacts into a moving target and can break the `--check` gate non-deterministically. | Optimizer writes to a dev scratch dir (`tools/promptopt/out/`); a human reviews, commits, and the existing gate validates. |
| **Committing the venv / DSPy wheels / `__pycache__`** | Bloats the repo and risks the Python tree being picked up by the `installSkill` embed walk (the very bug the allowlist fixes). | Git-ignore `tools/promptopt/.venv/`, `**/__pycache__/`; keep the embed bundle restricted to the `{SKILL.md, reference.md}` allowlist. |
| **MIPROv2 + Optuna by default** | Optuna is an extra dep that only MIPROv2 needs; the SKILL.md task is prose tuning, not demo-set search. | GEPA (`dspy.GEPA`), which is bundled and needs no Optuna. |
| **Adding an OpenAI provider just for DSPy** | Splits the API-key surface away from the two providers the Go scorecard already supports. | `anthropic/…` (primary) or `deepseek/deepseek-chat` (cheap leg) — same env vars as `test/oracle/llm`. |

## Stack Patterns by Variant

**If the team wants the DSPy harness fully reproducible across machines:**
- Pin `dspy==3.2.1` in `tools/promptopt/requirements.txt`, optionally generate a `requirements.lock` via `pip freeze` / `uv pip compile`.
- Because the optimization LM is non-deterministic, treat the optimizer's *output* (not its run) as the reproducible artifact: the committed SKILL.md/reference.md is what `helix-refgen --check` enforces.

**If the team decides Python is too much surface even for dev-time:**
- Drop DSPy entirely and run a hand-rolled candidate-search loop directly over the Go `test/oracle/adopt` scorer (`go test`-driven), keeping the milestone 100% Go. The other three features are unaffected. This is the clean fallback because DSPy is explicitly the *exploratory* item.

## Version Compatibility

| Package A | Compatible With | Notes |
|-----------|-----------------|-------|
| `dspy==3.2.1` | Python `>=3.10,<3.15` | DSPy's own interpreter pin (PyPI metadata). `python3` already present in one Makefile CI helper, but keep DSPy off the default path. |
| `dspy[anthropic]` | `ANTHROPIC_API_KEY` env (via LiteLLM) | Reuses the **same** env var as `test/oracle/llm/client.go` — one key for Go scorer + Python optimizer. |
| `dspy.GEPA` (GEPA 0.1.x) | bundled with DSPy 3.x | No separate `pip install gepa` needed when using integrated `dspy.GEPA`. |
| Go 1.25.1 / shipped binary | (no DSPy at all) | The binary never links, embeds, or shells to Python. Hard isolation. |

## Sources

- https://pypi.org/project/dspy/ — verified latest stable **3.2.1** (2026-05), Python `>=3.10,<3.15`, extras include `anthropic`/`optuna`/`mcp`/`langchain` — HIGH confidence
- https://dspy.ai/ — `pip install -U dspy`, `dspy.LM("provider/model", api_key=…)` + `dspy.configure(lm=lm)` pattern, Python ≥3.10 — HIGH confidence
- https://dspy.ai/api/models/LM/ — LM string format + explicit `api_key=` and `ANTHROPIC_API_KEY` env-var path via LiteLLM — HIGH
- https://github.com/stanfordnlp/dspy/releases — `3.3.0b1` beta exists; GEPA integrated (`dspy.GEPA`), Optuna required only by MIPROv2 — MEDIUM (release-page snapshot dates appeared stale; reconciled against PyPI)
- https://www.morphllm.com/gepa-prompt-optimization — GEPA is reflective prompt-evolution (ICLR 2026), bundled in DSPy, fewer rollouts than MIPROv2 — MEDIUM
- Repo tree (read directly): `cmd/helix-refgen/{main.go,render.go}`, `internal/cli/skill.go` (`installSkill` + embed walk), `test/oracle/adopt/scorecard.go` (choice_rate/fallback_rate classifier), `test/oracle/llm/client.go` (`ANTHROPIC_API_KEY`/`DEEPSEEK_API_KEY` providers), `go.mod` (Go 1.25.1), `Makefile` (`helix-refgen` targets, lone `python3` CI helper) — HIGH confidence on Go-side "no new deps" claim

---
*Stack research for: v2.2 Agent-Facing Skill Quality & Prompt Tuning (DSPy offline harness + Go-side codegen/skill rewrite)*
*Researched: 2026-06-23*

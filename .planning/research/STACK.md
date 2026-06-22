# Technology Stack

**Project:** Helix v2.1 — Agent Adoption & Aider-Derived Validation
**Researched:** 2026-06-22
**Mode:** Ecosystem / tooling (subsequent milestone — additions to a mature Go product)
**Overall confidence:** HIGH (conventions verified via official Claude Code / Codex / Gemini docs and the in-tree existing adapters; one assumption in the prompt corrected — see below)

> **Bottom line:** This milestone needs **almost no new Go dependencies.** Both
> thrusts are overwhelmingly served by code and libraries that already ship in the
> tree (the v1.12 bench stack, the v1.4 `llm`/`llmjudge` harness, the v2.0 skill +
> nudge). The *new* work is **file-format conventions, vendored data, and Go-native
> glue** — not library acquisition. The few genuinely new things are: (1) a
> bundled `reference.md` (+ optional per-verb refs) under the skill dir, (2)
> per-runtime instruction files (`AGENTS.md`, `GEMINI.md`) + a Codex `PreToolUse`
> hook config, and (3) a vendored snapshot of Aider fixtures with SPDX headers.

---

## Prompt assumptions corrected up front (read this first)

Two premises in the research question are factually wrong against the tree / upstream and must be fixed before requirements:

1. **The exercism polyglot fixtures are MIT, NOT Apache-2.0.** The milestone brief
   says "vendor Aider's fixtures (Apache-2.0)". That conflates two repos:
   - `github.com/Aider-AI/aider` (the **tool**) is **Apache-2.0** (`LICENSE.txt`). [HIGH]
   - `github.com/Aider-AI/polyglot-benchmark` (the **fixtures**) redistributes
     **Exercism** content; all six tracks (cpp/go/java/javascript/python/rust)
     ship an **identical MIT `LICENSE`** (verified byte-for-byte in the existing
     `bench/datasets/aider-polyglot/LICENSE-AUDIT.md`, sha256
     `e52f804e…df44df`). [HIGH — in-tree audit]
   - **Implication:** vendored polyglot fixtures need **MIT** attribution +
     `SPDX-License-Identifier: MIT` headers, not Apache-2.0. Apache-2.0 only
     applies if you also vendor code/prompts *from the aider tool repo* (e.g. the
     edit-format coder prompts) — see Thrust 2.

2. **A polyglot adapter already exists and is substantial (Phase 85).**
   `bench/datasets/aider-polyglot/` already has `clone.go` (pinned-SHA shallow
   clone), `loader.go` (the `.meta/config.json` → solution/test/example mapping +
   the upstream 2-attempt / 180s / stderr-reprompt protocol + anti-tamper pristine
   test restore), `pin.go` (pinned `7e0611e7…` SHA + hex guard), and
   `LICENSE-AUDIT.md`. **Do NOT re-propose the polyglot edit driver.** The v2.1
   surface is the *delta*: vendoring (not just cloning), the RepoMap eval, and the
   edit-format/fuzzy surface. See the "What NOT to add" section.

---

## Thrust 1 — Agent Adoption Layer

### 1A. SKILL reference & per-verb doc conventions (file format, not a library)

**No new dependency.** This is a content/convention change to the existing
`internal/cli/skills/helix/SKILL.md` (go:embed) authoring pipeline. The
authoritative conventions, verified against the live Claude Code skills doc:

| Convention | Value | Source / confidence |
|---|---|---|
| Frontmatter fields | `name`, `description` (recommended), `when_to_use`, `allowed-tools`, `disallowed-tools`, `effort`, `paths`, `disable-model-invocation`, `user-invocable` — **all optional except by recommendation** | code.claude.com/docs/en/skills [HIGH] |
| Idle-cost cap | `description` **+** `when_to_use` is truncated at **1,536 characters** in the skill listing (the only text loaded until the skill fires) | docs [HIGH] — matches the existing SKILL-04 in-tree assertion |
| Body size guidance | **Keep `SKILL.md` under 500 lines**; move detailed reference material to separate files | docs [HIGH] |
| Progressive disclosure | Bundle additional files (`reference.md`, examples, scripts) **in the skill directory**; reference them from `SKILL.md` via markdown links (`see [reference.md](reference.md)`) so the model loads them **on demand**, not at idle | docs [HIGH] |
| `allowed-tools` Bash form | `Bash(helix *)` (space-then-star) — already used correctly in the current SKILL.md | docs [HIGH] |
| Directory layout | `<.claude>/skills/helix/SKILL.md` + sibling reference files; `${CLAUDE_SKILL_DIR}` resolves the dir for bundled-file references | docs [HIGH] — matches `skillTargetDir()` |

**What to build (format, no library):**
- A new bundled **`reference.md`** (and optionally per-capability files, e.g.
  `reference-edit.md`, `reference-nav.md`) in `internal/cli/skills/helix/`,
  embedded via **`go:embed` with a directory glob** (`//go:embed skills/helix/*`)
  — the current code embeds a single file as a string (`//go:embed
  skills/helix/SKILL.md`), so the embed directive + `installSkill` must move to a
  **multi-file copy** (walk the embedded FS, write each file into the target
  `skills/helix/` dir). **Switch `embeddedSkillMD string` → `embed.FS`.** This is
  the one structural code change in 1A.
- Per-verb content (args, output shape, worked examples, "use X not Y") is
  **generated from the existing tool registry** the same way `cmd/docgen`
  produces the README table — reuse `cmd/docgen` plumbing rather than hand-writing
  50 verbs (avoids drift; the README tool table is already auto-generated). [HIGH]

**Why:** the v2.0 skill is deliberately terse (599-byte idle description). The
1,536-char cap means the *richer* per-verb material **cannot** go in frontmatter —
it MUST be progressive-disclosure reference files, which is exactly the documented
pattern. No YAML library is needed (the existing minimal stdlib frontmatter
splitter in `skill.go` already handles the block-scalar `description`).

### 1B. Per-runtime install conventions (config-file formats, no new libraries)

Each target runtime expects a different instruction-file + hook convention. The
existing `internal/cli/setup_clients.go` already does MCP-teardown for 7 clients;
v2.1 adds **instruction-file writing** (and, where supported, a PreToolUse hook).
All are plain file writes — **no new Go dependency.**

| Runtime | Instruction file | Location | Hook capability (PreToolUse-equiv?) | Confidence |
|---|---|---|---|---|
| **Claude Code** | `SKILL.md` (+ bundled `reference.md`) | `<.claude>/skills/helix/` (project) or `~/.claude/skills/helix/` | **Yes** — `PreToolUse` hook already installed (`internal/cli/nudge.go`, `setup_hooks.go`); emits `hookSpecificOutput.additionalContext` at exit 0 | HIGH (in-tree) |
| **OpenAI Codex CLI** | `AGENTS.md` (overridable via `AGENTS.override.md`; fallbacks via `project_doc_fallback_filenames`) | project root → cwd walk; home `~/.codex/AGENTS.md`. **32 KiB/file cap** (`project_doc_max_bytes`) | **Yes** — Codex `PreToolUse` hook intercepts Bash + `apply_patch` + MCP calls; returns `additionalContext` or `permissionDecision:"deny"`. Config: `~/.codex/hooks.json` or `.codex/hooks.json` / `config.toml [hooks]`. **Only `type:"command"` handlers run today.** | HIGH (developers.openai.com/codex/hooks) |
| **Gemini CLI** | `GEMINI.md` (filename configurable via `context.fileName` in settings.json) | `~/.gemini/GEMINI.md` (global); workspace + parent dirs | **No PreToolUse-equivalent.** Steering is via `GEMINI.md` context + optional custom commands (`~/.gemini/commands/*.toml`). MCP config in `~/.gemini/settings.json` | HIGH (geminicli.com docs) |
| **IDE assistants / generic** | No standard skill/hook surface | n/a | **No.** Best Helix can do is write an `AGENTS.md` (the emerging cross-tool standard at agents.md) into the project root and document manual setup | MEDIUM |

**Key design consequence:** Codex's `PreToolUse` hook uses the **same
`additionalContext` / `permissionDecision` envelope shape** as Claude Code's. The
existing `runNudge` / `emitAdvisory` logic in `nudge.go` is **directly portable** —
add a Codex-flavored output struct (camelCase keys are already what the Claude path
emits) and a `helix setup codex` path that writes `AGENTS.md` + a `hooks.json`
pointing at `helix nudge`. **Reuse the nudge command; do not write a second
steering engine.** [HIGH]

For Gemini/IDE/generic there is no hook — adoption relies on the instruction file
plus the (already-shipped) terse CLI ergonomics. Setup for these writes
`GEMINI.md` / `AGENTS.md` and is otherwise teardown-only (mirrors today's
`teardownOnlyRegister`).

### 1C. Adoption-contract test (deterministic, pure Go) + LLM-behavioral score (opt-in)

| Layer | What it is | New deps? | Reuses |
|---|---|---|---|
| **Deterministic CI gate** | Pure-Go table tests: (a) every frozen verb appears in the generated reference with args+example; (b) the nudge fires for each grep/sed/cat/find/Read shape (extend `nudge_test.go`); (c) frontmatter ≤1,536 chars (extend SKILL-04); (d) every runtime's instruction file round-trips (golden files) | **None** — `testing` + existing `test/harness` golden pattern | `internal/cli/nudge_test.go`, the SKILL-04 assertion, `test/harness/golden.go` |
| **LLM-behavioral adoption score (opt-in, never blocks)** | Given a coding task + the installed skill, does a real model emit `helix <verb>` over grep/sed/cat? Scored by a judge model | **None new** — `github.com/anthropics/anthropic-sdk-go v1.35.0` is already a dep; DeepSeek multi-provider path exists | `test/oracle/llm` (build tags `llm`/`llmjudge`), `mentionsHelix`/`firstCommandLine`/`mentionsGrepBaseline` detectors, `AskSingleTurn`, `test/oracle/judge` scorer + rubric. The `skill_trigger_test.go` is the seed — extend it into a multi-task adoption scorecard. |

**Why no new deps:** the v1.4 harness already loads the embedded skill body into a
system prompt (`cli.EmbeddedSkillBody()`), asks the model to choose a command, and
scores "did it pick helix vs grep". The adoption score is a **broadening of
existing tests**, build-tag gated so it stays out of the merge gate (consistent
with the locked "LLM tests never block merge" decision).

---

## Thrust 2 — Aider-Derived Validation

### 2A. Vendoring Aider fixtures (data + SPDX headers, not a library)

**Decision: vendor a pinned snapshot into the tree** (the milestone asks for a
*committed baseline* and a vendored fixture set; the current adapter only
*clones* at runtime, which is network-gated and skips offline). The vendored set
lives alongside the existing two hermetic fixtures under
`bench/datasets/aider-polyglot/fixtures/`.

| Item | Detail | Confidence |
|---|---|---|
| Source repo | `github.com/Aider-AI/polyglot-benchmark` @ pinned SHA (reuse `pin.go`'s `PinnedSHA = 7e0611e7…`; re-pin via `git ls-remote … main`) | HIGH (in-tree) |
| Fixture license | **MIT** per Exercism track (NOT Apache-2.0). Add `SPDX-License-Identifier: MIT` provenance + a `NOTICE`/attribution per track citing `exercism/<lang>@<sha>` | HIGH (in-tree LICENSE-AUDIT.md) |
| Exercise structure | `<lang>/exercises/practice/<name>/` with `.meta/config.json` (`files.solution` / `files.test` / `files.example`) — already modeled by `loader.go`'s `Config` struct. Pristine test-restore anti-tamper invariant already implemented (`restorePristineTests`, WR-01) | HIGH (in-tree) |
| Per-language test argv | `pytest` / `cargo test -- --include-ignored` / `go test ./...` / `./gradlew test` / `./npm-test.sh` / `./cpp-test.sh` — already in `nativeTestCommand()`; **Rust's `--include-ignored` is load-bearing** (WR-02, vacuous-pass guard) | HIGH (in-tree) |
| Attribution gate | Extend the existing `make verify-licenses` HARD-FAIL gate to cover the **vendored** tree (it currently audits the cloned tracks) | HIGH |

**Vendoring scope guard:** vendor only the **subset of exercises actually
exercised** by the three bench surfaces, not all six full tracks (keeps the tree
small and the MIT NOTICE auditable). The selection must be deterministic and
recorded (a manifest), so the committed baseline is reproducible.

**Apache-2.0 only if** you also vendor **edit-format material from the aider TOOL
repo** (`github.com/Aider-AI/aider`, e.g. its `benchmark/` harness logic or coder
edit-format prompt fixtures). Those files carry `Apache-2.0` and would need an
`Apache-2.0` SPDX header + a copy of `LICENSE.txt` + `NOTICE`. **Prefer NOT to
vendor aider tool code** — re-derive the edit-format *drift cases* natively against
Helix's own fuzzy cascade (see 2C) to avoid mixing licenses. [MEDIUM —
recommendation]

### 2B. RepoMap eval surface (reuse, no new library)

Aider's repomap is a tree-sitter PageRank ranked symbol graph — **the same design
as Helix's `internal/repomap`**. The eval measures `get-repo-map` / `get-context`
ranking quality + token-budget fitting.

| Need | Use | New deps? |
|---|---|---|
| Ranking-quality metric (does the right file/symbol rank in top-k for a task?) | A new evaluator under `bench/evaluators/` modeled on the existing `editsim` / `ragindex` leaf pattern (stdlib-only, no kernel import). Standard IR metrics: **recall@k, MRR, nDCG** — all ~30 LOC pure Go, no library | **None** |
| Token-budget correctness | Assert `get-repo-map`'s budget-fit output stays ≤ budget across repo sizes (binary-search fitter already exists) | **None** |
| Optional embedding baseline to compare against | `bench/ragindex` already vendors **`chromem-go`** (embedded vector index) as a leaf — reusable as a retrieval baseline to contrast PageRank vs embedding ranking | **None** (chromem-go already in go.mod) |

**Why no library:** recall@k/MRR/nDCG are trivial pure-Go; the project's bench
philosophy is stdlib-only leaf evaluators (see `editsim`'s deliberate
no-cross-package-reach doc). Adding an IR-metrics dependency would violate that.

### 2C. Edit-format / fuzzy-robustness surface (reuse, no new library)

Measures the 4-strategy fuzzy cascade (`internal/fuzzy`) against LLM output drift —
exactly what aider's edit-format benchmark probes (whitespace drift, ellipsis
placeholders, indentation reflow, ambiguity refusal).

| Need | Use | New deps? |
|---|---|---|
| Edit-similarity scoring of applied vs gold | **`bench/evaluators/editsim`** already implements CM-ES (normalized rune-level Levenshtein, CrossCodeEval) — reuse it directly | **None** |
| Drift corpus | Native Helix corpus of (intended edit, drifted-LLM-rendering) pairs exercising each of the 4 strategies; assert correct strategy selection + ambiguity refusal. Re-derive natively to keep it MIT/Apache-free | **None** |
| Wiring into the harness | Add as a `bench/runners/<mode>` surface emitting `result.v2.json`; aggregate via the existing `bench/aggregator` (BCa bootstrap + pass@k already hand-rolled, stdlib-only) | **None** |

### 2D. Baseline comparison tooling

| Question | Answer | Confidence |
|---|---|---|
| Need `golang.org/x/perf/benchstat`? | **No.** PROJECT.md's historical "benchstat gate" was **removed** when the bench harness went local-only at v1.9 (Phase 50). `go.mod` has no benchstat dep, and `bench/aggregator` already ships a **hand-rolled BCa bootstrap + pass@k + per-language slicing**. The committed baseline is a `result.v2.json` / `BENCH-RESULTS.md` artifact, compared by the existing aggregator — not benchstat. | HIGH (verified `go.mod` + `bench/aggregator/`) |
| Any new vendoring lib? | **No.** Vendoring is `git`-driven (existing `clone.go` shallow-clone-at-SHA) + a `go:generate`/`make` snapshot step + SPDX headers. No new module. | HIGH |
| Microbench/Go benchmark lib? | **No.** `testing.B.Loop` + `make bench-micro` (local-only rule) already covers it; benches stay non-CI-gated; `HELIX_BIN` must be set or smoke tests are false-green (known guard — the harness must enforce it). | HIGH |

---

## Recommended Stack (delta only)

### New Go dependencies
**NONE.** Every capability is served by existing modules.

| Already in `go.mod` (reused) | Version | v2.1 use |
|---|---|---|
| `github.com/anthropics/anthropic-sdk-go` | v1.35.0 | LLM-behavioral adoption score + judge scorer |
| `github.com/modelcontextprotocol/go-sdk` | v1.5.0 | tool-registry typed args drive per-verb reference generation |
| `chromem-go` (via `bench/ragindex`) | (in tree) | optional embedding baseline for RepoMap eval |
| stdlib `embed` | go 1.25.1 | multi-file skill bundle (`SKILL.md` + `reference.md`) |

### New file-format / convention artifacts (the real work)
| Artifact | Location | Format | Why |
|---|---|---|---|
| Bundled skill reference | `internal/cli/skills/helix/reference.md` (+ optional per-capability files) | Markdown, linked from `SKILL.md` | Progressive disclosure; per-verb depth can't fit the 1,536-char cap |
| Multi-file embed | `internal/cli/skill.go` `//go:embed skills/helix/*` → `embed.FS`; `installSkill` walks + copies | Go | ships `reference.md` alongside `SKILL.md` |
| Codex instruction file + hook | `helix setup codex` writes `AGENTS.md` + `~/.codex/hooks.json` (`type:"command"` → `helix nudge`) | Markdown + JSON | Codex `PreToolUse` mirrors Claude's envelope — reuse `nudge.go` |
| Gemini instruction file | `helix setup gemini-cli` writes `GEMINI.md` | Markdown | No hook surface; context-file steering only |
| Vendored fixtures | `bench/datasets/aider-polyglot/fixtures/<lang>/exercises/practice/<name>/` | exercism tree + `.meta/config.json` | committed offline baseline |
| SPDX/NOTICE | per-track `NOTICE` + `SPDX-License-Identifier: MIT` | text | **MIT** redistribution compliance |
| Committed baseline | `bench/reports/.../BENCH-RESULTS.md` + `result.v2.json` | existing schema | local-only baseline per project rule |

---

## Alternatives Considered

| Decision point | Recommended | Alternative | Why not |
|---|---|---|---|
| Per-verb reference delivery | Bundled `reference.md` (progressive disclosure) | Cram into SKILL.md body | 500-line body guidance + 1,536-char idle cap make this the documented, lower-idle-cost path |
| Codex steering | Reuse `helix nudge` via Codex `PreToolUse` hook | New Codex-specific steering engine | Codex hook envelope == Claude's (`additionalContext`); duplication is waste |
| Stats for baseline | Existing hand-rolled BCa bootstrap | `golang.org/x/perf/benchstat` | benchstat gate was deliberately removed at v1.9 (local-only rule); aggregator already does it |
| Fixture delivery | Vendor a pinned subset with SPDX | Keep clone-only | Milestone explicitly wants a *committed* baseline + vendored fixtures that work offline |
| Edit-format corpus | Native Helix drift corpus | Vendor aider tool's Apache-2.0 edit-format fixtures | Avoids mixing Apache-2.0 into an otherwise MIT-fixture tree; keeps `editsim` reuse clean |
| RepoMap metrics | Pure-Go recall@k/MRR/nDCG leaf evaluator | An IR-metrics library | Violates the stdlib-only leaf-evaluator invariant for ~90 LOC of trivial math |

---

## Installation (no `go get` needed)

```bash
# No new modules. Verify the tree builds + the existing gates pass:
go build ./cmd/helix
go vet ./...
go test ./...

# Bench surfaces are HELIX_BIN-gated (false-green guard) — local only, never CI:
make build
HELIX_BIN="$(pwd)/helix" go test ./bench/...
HELIX_BIN="$(pwd)/helix" make bench-micro   # if touching microbenches

# Fixture license gate (extend to cover the vendored tree):
make verify-licenses
```

---

## What NOT to add / what already exists (explicit overlap resolution)

**Do NOT re-build (already ships):**
- The polyglot **edit driver** — `bench/datasets/aider-polyglot/{clone,loader,pin}.go`
  already implements pinned-SHA clone, `.meta/config.json` mapping, the 2-attempt
  + 180s + stderr-reprompt protocol, the anti-tamper pristine-test restore, and
  the per-language native test argv. v2.1 only **vendors a snapshot** + adds the
  two *new* surfaces (RepoMap eval, edit-format).
- A second **steering engine** — `internal/cli/nudge.go` already classifies
  grep/sed/cat/find/Read and emits the advisory envelope; Codex reuses it.
- The v2.0 **terse SKILL.md** — keep it as the idle-cost frontmatter; v2.1
  *adds* `reference.md`, it does not rewrite the skill.
- **benchstat** / any new stats lib — `bench/aggregator` has BCa bootstrap +
  pass@k; the benchstat CI gate was removed at v1.9.
- **CM-ES / edit-similarity** scorer — `bench/evaluators/editsim` exists.
- Any **LLM client / judge** — `anthropic-sdk-go` + `test/oracle/{llm,judge}`
  (build-tag gated) already do tool-selection + judge scoring.
- A **YAML library** for the skill frontmatter — the stdlib splitter in
  `skill.go` handles it; zero-dep invariant must hold.

**Genuinely new (must build):**
- Bundled `reference.md` (+ multi-file `embed.FS` switch in `skill.go`/`installSkill`).
- `helix setup codex` (writes `AGENTS.md` + `~/.codex/hooks.json`) and
  `helix setup gemini-cli` (writes `GEMINI.md`); generic/IDE gets `AGENTS.md`.
- Vendored fixture snapshot + **MIT** SPDX/NOTICE headers + extended
  `make verify-licenses`.
- RepoMap-eval leaf evaluator (recall@k/MRR/nDCG) + a `bench/runners/<mode>` surface.
- Edit-format/fuzzy drift corpus + runner reusing `editsim`.
- Deterministic adoption-contract tests (reference completeness + per-runtime
  install golden + nudge-fires) and an opt-in multi-task LLM adoption scorecard.

---

## Confidence Assessment

| Area | Confidence | Notes |
|---|---|---|
| SKILL.md conventions (1,536 cap, <500 lines, progressive disclosure) | HIGH | Live Claude Code docs + in-tree SKILL-04 assertion agree |
| Codex `AGENTS.md` + `PreToolUse` hook (32 KiB cap, `type:command` only) | HIGH | developers.openai.com/codex/hooks |
| Gemini `GEMINI.md` (no hook) | HIGH | geminicli.com docs |
| Fixture licensing = MIT (not Apache-2.0) | HIGH | In-tree LICENSE-AUDIT.md, byte-verified |
| No new Go deps required | HIGH | `go.mod` + `bench/` + `test/oracle/` inspected directly |
| "Vendor vs clone" recommendation | MEDIUM | Milestone intent inferred ("committed baseline", "vendor fixtures") — confirm in requirements |
| Native edit-format corpus over vendoring aider Apache-2.0 fixtures | MEDIUM | Recommendation to keep license tree clean; not a hard constraint |

## Sources

- [Claude Code — Skills](https://code.claude.com/docs/en/skills) — frontmatter fields, 1,536-char cap, <500-line guidance, progressive disclosure, bundled reference files [HIGH]
- [OpenAI Codex — Custom instructions (AGENTS.md)](https://developers.openai.com/codex/guides/agents-md) — discovery order, 32 KiB cap, override/fallback filenames [HIGH]
- [OpenAI Codex — Hooks](https://developers.openai.com/codex/hooks) — `PreToolUse` event, `additionalContext`/`permissionDecision`, `type:"command"`-only, hooks.json location [HIGH]
- [Gemini CLI — GEMINI.md context files](https://geminicli.com/docs/cli/gemini-md/) and [configuration](https://geminicli.com/docs/reference/configuration/) — context file location, `context.fileName`, custom `.toml` commands, no PreToolUse hook [HIGH]
- [Aider-AI/aider LICENSE.txt](https://github.com/Aider-AI/aider/blob/main/LICENSE.txt) — Apache-2.0 (the tool repo) [HIGH]
- [Aider-AI/polyglot-benchmark](https://github.com/Aider-AI/polyglot-benchmark) — exercism redistribution, per-language `exercises/practice/<name>/` layout [HIGH]
- In-tree: `bench/datasets/aider-polyglot/{clone,loader,pin}.go`, `LICENSE-AUDIT.md` (MIT, byte-verified), `bench/aggregator/{bootstrap,passk}.go`, `bench/evaluators/editsim`, `bench/ragindex` (chromem-go), `internal/cli/{nudge.go,skill.go,setup_clients.go}`, `test/oracle/{llm,judge}`, `go.mod` [HIGH — direct inspection]

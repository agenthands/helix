# Architecture Research

**Domain:** Integration architecture for a v2.1 milestone (agent-adoption layer + Aider-derived bench validation) ON TOP OF a mature Go-native CLI-first code-intelligence platform (single binary + persistent daemon).
**Milestone:** v2.1 Agent Adoption & Aider-Derived Validation
**Researched:** 2026-06-22
**Confidence:** HIGH — every integration point below was read from real in-tree source (`internal/cli/{skill,nudge,setup_clients}.go`, `cmd/docgen/main.go`, `internal/kernel/help/help.go`, `internal/cli/verbs_gen.go`, `bench/datasets/aider-polyglot/loader.go`, `bench/runtime/{result,cell,mode_resolver}.go`, `bench/evaluators/editsim/editsim.go`, `Makefile`). This is an INTEGRATION map, not a domain survey: it states where the new code attaches, what is NEW vs MODIFIED, the data-flow deltas, and a dependency-honoring build order.

> **Framing for the roadmapper:** v2.1 adds NO new architectural layer and NO new Go dependency. Both thrusts are *leaf additions and small edits* against existing seams: Thrust 1 attaches to the `internal/cli/` skill+nudge+setup surface and reuses `cmd/docgen`/`get_tool_help`/`test/oracle`; Thrust 2 attaches to the `bench/` stack (the v1.12 aider-polyglot loader, `bench/runtime` cell spine, `bench/evaluators/*` leaves, `bench/runners/<mode>` filesystem-table). The single structural code change in the entire milestone is switching `internal/cli/skill.go` from an embedded `string` to an `embed.FS` so the skill can ship `reference.md` alongside `SKILL.md`.

---

## Standard Architecture

### System Overview — where v2.1 attaches (★ = NEW, ◆ = MODIFIED, · = reused unchanged)

```
┌──────────────────────────────────────────────────────────────────────────┐
│  THRUST 1 — ADOPTION LAYER  (internal/cli/, cmd/, test/oracle/)           │
├──────────────────────────────────────────────────────────────────────────┤
│  ★ cmd/helix-refgen ──reads──> · skill.ToolProviders() / help.ExtractParam│
│   (per-verb reference generator,        Docs()  (tool registry, same       │
│    --check drift gate, mirrors          source as cmd/docgen + get_tool_help)│
│    cmd/docgen)                                                              │
│        │ writes                                                            │
│        ▼                                                                   │
│  ★ internal/cli/skills/helix/reference.md  (+ per-capability files)        │
│  ◆ internal/cli/skill.go   string ──► embed.FS  (multi-file install)       │
│  · internal/cli/skills/helix/SKILL.md   (terse idle-cost tier, unchanged)  │
│        │ installed by                                                      │
│        ▼                                                                   │
│  ◆ internal/cli/setup_clients.go                                           │
│     · ClaudeCodeRegistrar  ──► installs SKILL.md + reference.md (was 1 file)│
│     ★ codex / gemini-cli / generic registrars: teardown-only ──► ALSO write│
│       AGENTS.md / GEMINI.md  (per-agent instruction file; no skill engine) │
│  ◆ internal/cli/nudge.go   broaden classifyBashTarget (awk/head/tail/pipe);│
│       ★ reuse emitAdvisory envelope for a Codex hooks.json handler          │
│        │ asserted by                                                       │
│        ▼                                                                   │
│  ★ adoption-contract tests (internal/cli/*_test.go):                       │
│     reference ⊇ VerbToolNames();  nudge-fires;  per-agent install goldens  │
│  ★ test/oracle/llm adoption scorecard  (build-tag llm/llmjudge, opt-in)    │
└──────────────────────────────────────────────────────────────────────────┘

┌──────────────────────────────────────────────────────────────────────────┐
│  THRUST 2 — AIDER-DERIVED VALIDATION  (bench/)                            │
├──────────────────────────────────────────────────────────────────────────┤
│  ★ bench/datasets/aider-polyglot/fixtures/<lang>/...  (VENDORED subset     │
│       + per-track MIT NOTICE/SPDX)                                          │
│  · loader.go RunExercise / restorePristineTests  (REUSED VERBATIM)        │
│  ★ EDIT-verb AgentFn  (drives replace-symbol-body / fuzzy-edit / ...       │
│       via the warm daemon)  ──plugs into──> RunExercise's AgentFn seam     │
│        │ run by                                                            │
│        ▼                                                                   │
│  ★ bench/runners/aider_edit/MODE.md   (filesystem-table mode, 0 Go change) │
│  ◆ bench/runtime/result.go  + additive open key `edit_format_applied`      │
│       (*bool, omitempty — mirrors swebench_* keys; NO schema v3 bump)       │
│                                                                            │
│  ★ bench/evaluators/repomapeval/   (recall@k / MRR / nDCG leaf, stdlib)    │
│       ──measures──> · get-repo-map / get-context (internal/repomap)        │
│  ★ bench/datasets/repomap-gold/    (hand-labeled relevance corpus)         │
│  ★ bench/evaluators/fuzzyrobust/   (drift corpus runner, reuses editsim.ES)│
│       ──measures──> · internal/fuzzy 4-strategy cascade + refusal           │
│                                                                            │
│  · bench/aggregator (BCa/pass@k)  ◆ ByLanguage already exists; consumes    │
│       result.v2 rows for committed baseline                                │
│  ★ bench/reports/<run>/BENCH-RESULTS.md  (committed local baseline)        │
│  ◆ Makefile  verify-licenses extended to vendored tree; bench targets       │
│       HELIX_BIN-guarded (fail-not-skip)                                     │
└──────────────────────────────────────────────────────────────────────────┘
```

### Component Responsibilities

| Component | Responsibility | New / Modified / Reused |
|-----------|----------------|-------------------------|
| `cmd/helix-refgen` (or `cmd/docgen` extension) | Generate `reference.md` from `skill.ToolProviders()` + `help.ExtractParamDocs`; `--check` drift gate | **NEW** (or modify `cmd/docgen` to emit a 2nd artifact) |
| `internal/cli/skills/helix/reference.md` | Per-verb synopsis/args/output/example, progressive-disclosure tier | **NEW** (generated, committed) |
| `internal/cli/skill.go` | Embed + install the skill bundle | **MODIFIED** — `string` → `embed.FS`, `installSkill` walks+copies all files |
| `internal/cli/setup_clients.go` | Per-client install | **MODIFIED** — Claude path copies reference too; codex/gemini/generic flip teardown-only → also-write instruction file |
| `internal/cli/nudge.go` | PreToolUse steering | **MODIFIED** — broaden `classifyBashTarget`; envelope reused for Codex hooks.json |
| adoption-contract tests (`internal/cli/*_test.go`) | Deterministic CI gate: reference completeness, nudge-fires, per-agent install goldens | **NEW** (extend `nudge_test.go`, `verbs_gen_test.go` pattern) |
| `test/oracle/llm` adoption scorecard | Opt-in LLM-behavioral "does a model pick helix" score | **NEW oracle test** in existing build-tag-gated harness |
| `bench/datasets/aider-polyglot/loader.go` `RunExercise` | 2-attempt + pristine-test-restore protocol | **REUSED VERBATIM** — do NOT touch WR-01 anti-tamper |
| EDIT-verb `AgentFn` | Route the model's edit through `helix` verbs against the warm daemon | **NEW** — supplies the loader's existing `AgentFn` seam |
| `bench/datasets/aider-polyglot/fixtures/` | Vendored exercism subset + MIT SPDX/NOTICE | **NEW** (data) |
| `bench/runners/aider_edit/MODE.md` | Filesystem-table mode binding | **NEW** (1 dir, 0 Go change — like Phase 80) |
| `bench/runtime/result.go` | `result.v2` builder | **MODIFIED** — add `edit_format_applied *bool` additive open key |
| `bench/evaluators/repomapeval/` | recall@k/MRR/nDCG ranking-quality leaf | **NEW** (stdlib-only leaf) |
| `bench/datasets/repomap-gold/` | Hand-labeled "relevant symbols for task T" corpus | **NEW** (data) |
| `bench/evaluators/fuzzyrobust/` | Drift-corpus runner; asserts strategy selection + refusal; scores with `editsim.ES` | **NEW** (leaf, reuses `editsim`) |
| `bench/aggregator` | BCa bootstrap / pass@k / per-language rollup → baseline | **REUSED** (already has `ByLanguage`) |

---

## Recommended Project Structure (delta only — what lands where)

```
cmd/
├── docgen/                       ◆ option A: extend to also emit reference.md
└── helix-refgen/                 ★ option B (recommended): dedicated generator
    ├── main.go                       (blank-imports == docgen/daemon for registry parity)
    └── main_test.go

internal/cli/
├── skill.go                      ◆ embeddedSkillMD string → //go:embed skills/helix/* embed.FS
├── skills/helix/
│   ├── SKILL.md                  · terse idle-cost tier (unchanged)
│   ├── reference.md              ★ generated per-verb reference (committed)
│   └── reference-edit.md ...     ★ optional per-capability splits
├── setup_clients.go              ◆ Claude copies bundle; codex/gemini/generic write instruction file
├── setup_agents.go               ★ AGENTS.md / GEMINI.md writers + Codex hooks.json writer (new file)
├── nudge.go                      ◆ broaden classifyBashTarget; Codex-envelope reuse
├── reference_contract_test.go    ★ reference ⊇ VerbToolNames() drift gate
├── setup_agents_test.go          ★ per-agent install goldens
└── nudge_test.go                 ◆ add broadened-shape nudge-fires cases

test/oracle/llm/
└── adoption_scorecard_test.go    ★ opt-in choice-rate / fallback-rate (build tags llm,llmjudge)

bench/
├── datasets/
│   ├── aider-polyglot/
│   │   ├── loader.go             · RunExercise REUSED VERBATIM (WR-01 untouched)
│   │   ├── fixtures/<lang>/...   ★ VENDORED exercism subset
│   │   ├── NOTICE / *.SPDX       ★ MIT attribution per track
│   │   └── VENDOR-MANIFEST.md    ★ deterministic vendored-exercise selection
│   └── repomap-gold/             ★ hand-labeled relevance corpus + task specs
├── runners/
│   └── aider_edit/MODE.md        ★ filesystem-table mode (0 Go change)
├── runtime/
│   ├── result.go                 ◆ + edit_format_applied open key
│   └── aider_edit_agent.go       ★ EDIT-verb AgentFn (daemon-dialing)
├── evaluators/
│   ├── editsim/                  · ES() REUSED by fuzzyrobust
│   ├── repomapeval/              ★ recall@k / MRR / nDCG (stdlib leaf)
│   └── fuzzyrobust/              ★ drift-corpus runner (leaf, reuses editsim)
└── reports/<run>/BENCH-RESULTS.md ★ committed local baseline

Makefile                          ◆ verify-licenses → vendored tree; bench targets HELIX_BIN-guarded
```

### Structure Rationale

- **`cmd/helix-refgen` over hand-writing:** The 50 verbs are FROZEN and the registry (`skill.ToolProviders()` → `tool.Name`, `tool.Description`, `tool.InputSchema`) is the single source `cmd/docgen` and `get_tool_help` already read. Generating `reference.md` from the same source makes the completeness contract test trivial (`reference ⊇ VerbToolNames()`) and structurally prevents drift. `cmd/docgen/main.go:110-134` is the exact plumbing to clone; `help.ExtractParamDocs` (`internal/kernel/help/help.go:21`) already turns an `InputSchema` into typed `ParamDoc`s — the per-verb args section is `FormatHelp`'s output, no new parsing.
- **Bundled `reference.md` (progressive disclosure), not a fatter SKILL.md:** STACK.md confirms the 1,536-char idle cap and the <500-line body guidance. The terse `SKILL.md` stays the idle tier; `reference.md` loads on demand. This is why `skill.go` MUST move to `embed.FS` — the only structural code change.
- **Leaf evaluators under `bench/evaluators/`:** `editsim` is the proven precedent: stdlib-only, no cross-package reach, unit-tested against paper examples. `repomapeval` and `fuzzyrobust` follow it. The `vet-ablation-leakage` analyzer already forbids `bench/runners → lspool|semantic/store`; new leaves must respect it.
- **Filesystem-as-table mode (`bench/runners/aider_edit/MODE.md`):** Phase 80 grew 1→6 modes with ZERO resolver Go change by dropping in `MODE.md` dirs (`mode_resolver.go:5-9`). The aider edit surface is one more dir.

---

## Architectural Patterns (the seams v2.1 plugs into)

### Pattern 1: Registry-as-source generation with a `--check` drift gate

**What:** `cmd/docgen` imports all tool-providing packages (blank imports, `cmd/docgen/main.go:32-46`), calls `skill.ToolProviders()`, and renders markdown between `<!-- BEGIN -->/<!-- END -->` markers; `--check` exits 1 if the file would change (CI gate).
**When to use:** Any committed artifact derived from the 50 frozen verbs — exactly `reference.md`.
**Trade-off:** Must keep the generator's blank-import set == the daemon's (the documented "blank-import parity rule", `cmd/docgen/main.go:23-31`) or the generated set diverges from the runtime set. `helix-refgen` inherits this constraint verbatim.

**Example:**
```go
// helix-refgen reuses the docgen registry walk:
for _, tp := range skill.ToolProviders() {
    for _, tool := range tp.Tools() {
        verb := strings.ReplaceAll(tool.Name, "_", "-")      // frozen mechanical mapping
        params := help.ExtractParamDocs(tool.InputSchema)    // typed args from schema
        section := help.FormatHelp(verb, tool.Description, params, "")
        // ... emit reference.md section ...
    }
}
```

### Pattern 2: `embed.FS` skill bundle + path-contained atomic install

**What:** `installSkill` (`skill.go:131`) atomically writes the embedded skill (temp+rename) into a `withinSkillRoot`-contained target dir. Today it embeds ONE file as a `string`.
**When to use:** Shipping `reference.md` alongside `SKILL.md`.
**Trade-off:** The `embed.FS` switch ripples into `EmbeddedSkillBody()` (still returns just `SKILL.md`), the SKILL-04 idle-cost assertion (reads `SKILL.md` only), and `installSkill` (now walks the FS). All three are small, localized edits; the path-traversal guard (`withinSkillRoot`/`lexicalSkillRoot`) is unchanged.

**Example:**
```go
//go:embed skills/helix/*
var embeddedSkillFS embed.FS

func installSkill(targetDir string) error {
    if !withinSkillRoot(targetDir) { /* unchanged guard */ }
    // walk embeddedSkillFS, write each entry atomically (temp+rename)
}
```

### Pattern 3: PreToolUse advisory envelope, reused across Claude + Codex

**What:** `nudge.go` emits `{"hookSpecificOutput":{"hookEventName":"PreToolUse","additionalContext":...}}` at exit 0 (advisory, never blocks — `nudge.go:123-149`). Codex's PreToolUse hook (STACK.md, HIGH-confidence) uses the SAME camelCase `additionalContext` envelope.
**When to use:** Codex steering — write a `~/.codex/hooks.json` (`type:"command"` → `helix nudge`) and reuse `emitAdvisory`.
**Trade-off:** Gemini/IDE/generic have NO hook surface — steering there is instruction-file-only (`GEMINI.md`/`AGENTS.md`). Do not build a per-agent steering engine; the one nudge command serves both hook-capable runtimes.

### Pattern 4: Injected `AgentFn`/`TestFn` seam in `RunExercise` (verb-agnostic loader)

**What:** `RunExercise(ctx, ex, workDir, runTests TestFn, agent AgentFn)` (`loader.go:230`) is verb-agnostic — `AgentFn` is "whatever drives the edit". The loader already does pinned-clone, config-map, 2-attempt reprompt, and the WR-01 pristine-test restore.
**When to use:** The v2.1 polyglot edit bench supplies an `AgentFn` that routes the model's diff through `helix replace-symbol-body`/`fuzzy-edit`/`replace-in-file`/`insert-*` against the warm daemon.
**Trade-off:** The `AgentFn` must dial the daemon (the v2.0 one-shot `helix <verb>` path) — it is NOT a leaf; it lives in `bench/runtime` (or a sibling) where daemon-dialing is allowed, not in the `aiderpolyglot` leaf package (stdlib-only). The pristine-test restore is load-bearing; do not move it into the AgentFn.

### Pattern 5: Additive open `result.v2` key (no schema v3 bump)

**What:** `result.go` carries open provenance keys (`embedder_id`, `language`, `container_id`, `swebench_*`) as `omitempty` (pointer for booleans so a literal `false` survives). `additionalProperties` stays OPEN; `schema_version` stays `"v2"` (`result.go:126-139, 220-229`).
**When to use:** The "edit-format-applied-correctly" signal — `edit_format_applied *bool` mirrors `swebench_raw_resolved` exactly (a `*bool` so an applied=false is preserved, not dropped).
**Trade-off:** None structurally; this is the project's established additive contract. Do NOT add it to `required` and do NOT bump to v3.

---

## Data Flow

### New flow A — per-verb reference generation + install

```
skill.ToolProviders() ──► helix-refgen ──► reference.md (committed)
        (tool.Name, .Description, .InputSchema)        │
                                                       │ embed.FS
helix setup claude-code ──► installSkill walks bundle ──► <.claude>/skills/helix/{SKILL.md,reference.md}
helix setup codex       ──► AGENTS.md + ~/.codex/hooks.json(→ helix nudge)
helix setup gemini-cli  ──► GEMINI.md
        ▲
   make check / CI: helix-refgen --check  +  reference ⊇ VerbToolNames() test  (BLOCKS merge)
```

### New flow B — polyglot edit bench (reuses RunExercise)

```
vendored fixtures ──► loadExercise (.meta/config.json map)
        │
        ▼
RunExercise(ctx, ex, workDir, realTestFn, EDIT-verb AgentFn)   [REUSED VERBATIM]
   attempt i: AgentFn drives `helix replace-symbol-body|fuzzy-edit|...` against warm daemon
              │
              ▼ restorePristineTests (WR-01)  ──►  nativeTestCommand (pytest/cargo --include-ignored/...)
   pass/fail ──► BuildResult{ outcome, language, edit_format_applied:&bool }  ──► result.v2.json
        │
        ▼
bench/aggregator (BCa, pass@k, ByLanguage) ──► bench/reports/<run>/BENCH-RESULTS.md (committed baseline)
```

### New flow C — RepoMap-quality + fuzzy-robustness evals

```
repomap-gold corpus (task T, relevant symbols) ──► get-repo-map / get-context (internal/repomap)
        │                                                   │ ranked output
        ▼                                                   ▼
repomapeval leaf: recall@k / MRR / nDCG + budget-fit invariant ──► result.v2 ──► aggregator

drift corpus (intended edit, drifted rendering) ──► fuzzy-edit / replace-in-file (internal/fuzzy)
        │                                                   │ strategy used + refusal
        ▼                                                   ▼
fuzzyrobust leaf: assert strategy selected + ambiguity REFUSED; score with editsim.ES ──► result.v2
```

### Key invariants the data flow MUST preserve

1. **HELIX_BIN guard (fail-not-skip):** bench smoke is false-green without `HELIX_BIN` (MEMORY: helix-bench-smoke-false-green). The new edit/repomap/fuzzy runners must FAIL or REFUSE when `HELIX_BIN` is unset, never silently SKIP into a green. The committed baseline is captured `HELIX_BIN`-gated, local-only.
2. **WR-01 anti-tamper:** the graded test file is restored pristine before each grade. The EDIT-verb AgentFn must not bypass `restorePristineTests`.
3. **Leaf discipline:** `repomapeval`/`fuzzyrobust` import stdlib (+ `editsim`) only — no `internal/kernel`, `internal/semantic`, or `bench/runtime`. The daemon-dialing AgentFn lives OUTSIDE the leaf.
4. **Generator parity:** `helix-refgen`'s blank-import set must equal the daemon's (else the reference covers the wrong tool set).

---

## Scaling Considerations

| Scale | Architecture Adjustments |
|-------|--------------------------|
| Vendored fixture subset (tens of exercises) | Vendor only the exercises actually exercised (VENDOR-MANIFEST.md), not all 6 full tracks — keeps the tree small and the MIT NOTICE auditable |
| Full 225-task live polyglot run | Local/`HELIX_BIN`-gated only; NEVER a CI gate (network + 6 toolchains). Hermetic vendored subset is the CI proof (Phase 85 precedent) |
| RepoMap gold corpus growth | Hand-labeling is the cost driver; start with a small curated set per language; recall@k is O(k), nDCG O(n log n) — math is not the bottleneck |
| LLM adoption scorecard | Opt-in, never blocks; nondeterminism stays out of the merge gate (v1.4 precedent) |

### Scaling Priorities

1. **First bottleneck:** human curation of the repomap-gold and fuzzy-drift corpora — gate these features (P2) behind the P1 substrate so the milestone isn't blocked on labeling.
2. **Second bottleneck:** vendored-tree size / license audit surface — bound by the VENDOR-MANIFEST subset + the extended `make verify-licenses` hard-fail.

---

## Anti-Patterns

### Anti-Pattern 1: Hand-writing the per-verb reference
**What people do:** Author 50 verbs of args/examples by hand in `reference.md`.
**Why it's wrong:** Drifts from the frozen registry the moment a description changes; the completeness test becomes a stale duplicate.
**Do this instead:** Generate from `skill.ToolProviders()` + `help.ExtractParamDocs` via `helix-refgen --check`, identical to `cmd/docgen`.

### Anti-Pattern 2: Rebuilding the aider polyglot adapter
**What people do:** Treat "add the aider benchmark" as new harness work.
**Why it's wrong:** The v1.12 loader (clone + config-map + 2-attempt + WR-01 anti-tamper + native test argv) already exists and is correct; rebuilding risks regressing WR-01.
**Do this instead:** Reuse `RunExercise` verbatim; only supply the EDIT-verb `AgentFn`, vendor fixtures, add the `edit_format_applied` field, and commit a baseline.

### Anti-Pattern 3: A per-agent skill engine for Codex/Gemini/IDE
**What people do:** Build a bespoke skill runtime per non-Claude agent.
**Why it's wrong:** Those agents read a markdown instruction file (AGENTS.md/GEMINI.md), not Claude Agent Skills. High cost, no return.
**Do this instead:** One shared generated reference + a thin per-agent instruction file; reuse the single `helix nudge` for the only other hook-capable runtime (Codex).

### Anti-Pattern 4: Bumping `result.v2` to v3 for the edit-format signal
**What people do:** Add a required field / new schema version.
**Why it's wrong:** Breaks byte-compat of existing artifacts; the project's contract is additive-minor open keys.
**Do this instead:** `edit_format_applied *bool` with `omitempty`, `additionalProperties` open, `schema_version` stays `"v2"` (mirror `swebench_raw_resolved`).

### Anti-Pattern 5: A deny/block PreToolUse hook to "force" adoption
**What people do:** exit 2 on grep/sed/cat.
**Why it's wrong:** grep/sed/cat are legitimately correct for prose/logs/config/unknown-symbol discovery; blocking trains the model to fight the tool. `nudge.go` deliberately exits 0.
**Do this instead:** Keep advisory exit-0; broaden detection only.

---

## Integration Points

### Internal Boundaries

| Boundary | Communication | Notes |
|----------|---------------|-------|
| `helix-refgen` ↔ tool registry | `skill.ToolProviders()` + `help.ExtractParamDocs` | Same source as `cmd/docgen`/`get_tool_help`; keep blank-import parity with daemon |
| `skill.go` ↔ `setup_clients.go` | `installSkill(targetDir)` walks `embed.FS` | Claude/Claude-Desktop registrars copy the bundle; path-traversal guard unchanged |
| `setup_clients.go` ↔ non-Claude agents | write `AGENTS.md`/`GEMINI.md` (+ Codex `hooks.json`) | Flip teardown-only → also-write; Codex hooks.json points at `helix nudge` |
| `nudge.go` ↔ Codex hook | shared `additionalContext` exit-0 envelope | One steering engine, two runtimes |
| EDIT-verb `AgentFn` ↔ warm daemon | one-shot `helix <verb>` dial (v2.0 path) | Lives in `bench/runtime` (daemon-dialing allowed), NOT the stdlib leaf |
| `aiderpolyglot.RunExercise` ↔ AgentFn/TestFn | injected func seams | REUSED VERBATIM; WR-01 restore untouched |
| `repomapeval`/`fuzzyrobust` ↔ kernel engines | measure `get-repo-map`/`get-context` & `internal/fuzzy` outputs | Leaves import stdlib + `editsim` only; `vet-ablation-leakage` forbids kernel imports from `bench/runners` |
| new runners ↔ `bench/runtime` | `BuildResult` → `result.v2.json` → `bench/aggregator` | Additive `edit_format_applied` key; aggregator's `ByLanguage` already exists |
| Makefile gates | `verify-licenses` (vendored tree), `bench`/`bench-quick` (HELIX_BIN) | License gate already exists for the cloned tracks; extend to vendored fixtures |

### External Services

| Service | Integration Pattern | Notes |
|---------|---------------------|-------|
| Aider-AI/polyglot-benchmark | pinned-SHA clone (`pin.go` `7e0611e7…`) → snapshot into `fixtures/` | Exercism content is **MIT** (in-tree byte-verified LICENSE-AUDIT.md), NOT Apache-2.0 |
| Anthropic / DeepSeek LLM | `anthropic-sdk-go v1.35.0` (already a dep) via `test/oracle/{llm,judge}` | Opt-in adoption scorecard; build-tag gated; never blocks merge |
| Claude Code / Codex / Gemini hook & instruction surfaces | file writes (SKILL.md/reference.md, AGENTS.md, GEMINI.md, hooks.json) | No new dependency; conventions verified in STACK.md (HIGH) |

---

## Suggested Build Order (dependency-honoring — roadmap phases from 97)

The order encodes three hard dependencies the question calls out: **reference → contract-test**, **vendor → baseline**, **output/corpus → eval**. Thrust 1 and Thrust 2 are independent and could interleave; within each, order is fixed.

**Thrust 1 (adoption):**
1. **`embed.FS` switch + `helix-refgen` + generated `reference.md`** — the substrate everything else asserts/installs. (`skill.go` MODIFIED, `helix-refgen` NEW, `reference.md` NEW.) *Depends on: nothing new.*
2. **Deterministic adoption-contract tests** — reference ⊇ `VerbToolNames()`, nudge-fires (broadened), idle-cost cap. *Depends on: 1 (reference must exist to assert completeness).* BLOCKS merge.
3. **Multi-agent instruction files + Codex hook** — `setup_clients.go`/`setup_agents.go` write AGENTS.md/GEMINI.md, Codex hooks.json reuses `nudge`. *Depends on: 1 (shared reference is the substrate) + the nudge broadening.* Add per-agent install goldens.
4. **(P2) LLM-behavioral adoption scorecard** — `test/oracle/llm`, opt-in. *Depends on: 1 (loads the skill body) + 2 (deterministic layer green first).*

**Thrust 2 (validation):**
5. **Vendor fixtures + MIT SPDX/NOTICE + extended `verify-licenses`** — committed offline data. *Depends on: nothing new (reuses `pin.go`).* Must precede the baseline.
6. **EDIT-verb AgentFn + `aider_edit` MODE.md + `edit_format_applied` key** — wires `RunExercise` to helix verbs. *Depends on: 5 (vendored fixtures) + the result.v2 additive key.*
7. **Committed polyglot baseline** — HELIX_BIN-gated local capture → `BENCH-RESULTS.md`. *Depends on: 5 + 6 (must run the wired bench offline).*
8. **(P2) RepoMap-gold corpus → `repomapeval` leaf → baseline** — corpus before eval. *Depends on: corpus authored first.*
9. **(P2) Fuzzy-drift corpus → `fuzzyrobust` leaf (reuses `editsim.ES`) → baseline** — corpus before eval. *Depends on: corpus authored first.*

**Cross-cutting ordering rule for every bench phase:** the HELIX_BIN guard (fail-not-skip) and the leaf-import boundary (`vet-ablation-leakage`) are pre-existing gates the new code must satisfy, not new work — verify them in each bench phase's exit criteria.

---

## Sources

- In-tree source (read directly, HIGH): `internal/cli/skill.go`, `internal/cli/nudge.go`, `internal/cli/setup_clients.go`, `internal/cli/verbs_gen.go`, `cmd/docgen/main.go`, `internal/kernel/help/help.go`, `bench/datasets/aider-polyglot/loader.go`, `bench/runtime/result.go`, `bench/runtime/cell.go`, `bench/runners/mode_resolver.go`, `bench/evaluators/editsim/editsim.go`, `Makefile`
- `.planning/research/STACK.md`, `.planning/research/FEATURES.md` (this cycle — established findings)
- `.planning/PROJECT.md` (v2.1 milestone section + v1.12 bench phase history)
- MEMORY: helix-bench-smoke-false-green (HELIX_BIN fail-not-skip guard)

---
*Architecture integration research for: Helix v2.1 — Agent Adoption & Aider-Derived Validation*
*Researched: 2026-06-22*

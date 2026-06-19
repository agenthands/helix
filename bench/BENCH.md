# Helix Bench Stack

The `bench/` tree is the v1.12 milestone benchmark stack — the artifact that produces
Helix's **headline, publishable** capability numbers across external benchmarks (Aider
Polyglot, SWE-bench, Multi-SWE-bench, Terminal-Bench) and the internal ToolBench-Go
capability suite. It is a net-new tree introduced in Phase 75 and is built up by Phases
75–89.

`bench/` is a sibling of, and **independent from**, `eval/` (the in-process PR-gate
wiring smoke). See the "eval ↔ bench separation" note below (INFRA-03).

## Directory layout (six dirs: five runtime + one contract)

`tree -d -L 2 bench/` lists exactly six top-level directories. Five are **runtime** dirs
(they hold executable adapters, dataset fixtures, mode definitions, language toolchain
wiring, evaluators, and generated reports). One is the **contract** dir (`schema/`), which
holds only the versioned result-JSON schema every runtime dir reads from and writes into —
it carries no executable runtime artifacts.

| Dir | Kind | What it holds | Lands in |
|-----|------|---------------|----------|
| `bench/datasets/` | **runtime** | Per-benchmark dataset adapters + fixtures (internal ToolBench-Go seeded first; external SWE-bench / Aider / Terminal-Bench adapters added later) | 75 (scaffold), 78+ |
| `bench/runners/` | **runtime** | Per-mode run definitions (`<mode>/MODE.md`); the `cmd/helix-bench run` matrix driver writes here | 76+ |
| `bench/languages/` | **runtime** | Per-language toolchain wiring (pytest, go test, cargo, etc.) used by the runners | 78+ |
| `bench/evaluators/` | **runtime** | Scorers + the result aggregator that round-trips `result.v2.json` | 79+ |
| `bench/reports/` | **runtime** | Generated, reproducible reports (leaderboard, per-language, cost) | 89 |
| `bench/schema/` | **contract** | The versioned `result.v2.schema.json` and the single `fairness_contract` source of truth — **contract-only, no runtime artifacts** (D-07) | 75 (Plan 03) |

Phase 75 only stands up the empty skeleton (each dir tracked via a `.gitkeep`) and the
`schema/` contract document (Plan 03). Plan 04 fills `runners/`; Plan 05 fills
`datasets/`. Downstream phases (76–89) write into the stable layout above.

## Ablation modes (`bench/runners/<mode>/MODE.md`) — the five-of-six matrix

Each bench arm is a `bench/runners/<mode>/MODE.md` definition (ABLATE-01). The
mode→profile resolver (`bench/runners/mode_resolver.go`) is table-driven: it
reads the two-key (`mode` + `profile`) frontmatter of `MODE.md`, so adding a
mode is purely a matter of dropping in a new directory — no Go change (D-05).
Phase 80 grows the matrix from the single Phase 77 seed (`your_agent_full`) to
six modes:

| Mode | Profile | Notes |
|------|---------|-------|
| `your_agent_full` | `bench-full` | full Helix tool surface (Phase 77 seed) |
| `baseline_plain` | `baseline` | zero-Helix-tools control arm (see below) |
| `no_lsp` | `bench-no-lsp` | LSP subsystem disabled (ABLATE-05) |
| `no_structured_edit` | `bench-no-structured-edit` | structured-edit tools excluded (ABLATE-07) |
| `your_agent_no_semantic` | `bench-no-semantic` | semantic-store tools excluded; **deferred guarantee** (see below) |
| `baseline_rag` | `baseline` (placeholder) | registered **fail-closed stub**, no row until Phase 83 |

**`baseline_plain` — empty Helix inventory, no new YAML.** `baseline_plain`
reuses the existing `baseline.yaml` profile (D-01, ABLATE-03 — there is
deliberately **no** new `bench-*` YAML for it). It exposes an **empty** Helix
tool inventory: the agent sees only the shell / grep / read / edit / test
capabilities its own runtime exposes natively. That empty inventory is
enforced by the **profile filter** (the `baseline` profile's empty tool
lists), not by any bench-side code. The daemon still spawns for trace
symmetry, so baseline_plain's run shape matches the other arms.

**`your_agent_no_semantic` — `ablation_status: guarantee_pending_phase_81`.**
This arm emits a **real** result row, but the row is marked **partial** via
`ablation_status: guarantee_pending_phase_81`. The kernel-level
`disable_semantic_subsystem` flag — the zero-DuckDB guarantee, ABLATE-06 —
lands in **Phase 81**, not this phase. Until then `bench-no-semantic` is
tool-filter-only (Phase 76 D-11/D-12): the 10 semantic-store tools are
excluded from the agent surface, but the daemon may still read the semantic
store via back-channel paths. The `guarantee_pending_phase_81` status flags
the row as not-yet-a-clean no_semantic measurement; the clean zero-DuckDB
number arrives once Phase 81 wires the kernel flag with its config-gate E2E
test.

**`baseline_rag` — fail-closed stub, no row until Phase 83.** `baseline_rag`
is a registered fail-closed stub. Its `MODE.md` carries a placeholder
`profile: baseline` so the resolver parses it, but the cell wiring fail-closes
the mode **before** any daemon spawn (detected by mode name, not a frontmatter
marker — keeping the resolver change-free). The real RAG arm (standalone
`cmd/helix-bench-rag` + chromem-go embedding index, ABLATE-04) is **deferred to
Phase 83**; this mode produces **no result row** this phase.

## Operator prerequisites (operator-side, NOT Go build deps)

Running the full bench stack requires tooling that is **NOT** needed to build the Helix
binary. The Helix binary remains a single CGO Go binary with no Python/Docker runtime
dependency (see `CLAUDE.md`). The prerequisites below are **operator-side** — only an
operator who actually *runs benchmarks* needs them; they are never linked into `helix`.

- **Python 3.11+** — upstream benchmark harnesses (SWE-bench, Aider Polyglot driver) and
  `pytest --json-report` for the Python language tier are Python-based.
- **Docker Engine** — SWE-bench / Multi-SWE-bench / Terminal-Bench execute each task in a
  per-task container; the container runtime (CONTAINER-*) lands before those adapters.
- **Per-language toolchains** — each language tier under `bench/languages/` shells to that
  language's native test/build tooling (Go toolchain, `cargo`, `pytest`, a JDK, etc.).
  Only the tiers you run need their toolchain installed.

## eval ↔ bench separation note (INFRA-03)

`eval/` and `bench/` are **independent siblings**. They share **no code and no pricing
file** (D-15). `eval/` stays the in-process PR-gate wiring smoke (daemon startup, profile
loading, scorer, reporter — see `eval/EVAL.md`); `bench/` is the milestone artifact for
headline, publishable claims.

For the v1.12 milestone this boundary is **prose-enforced only** — there is intentionally
**no `vet-noeval2bench` analyzer** this milestone (D-08; deferred until a real cross-import
leak appears, matching the `internal/lint/` precedent). Do not add `bench/ → eval/` or
`eval/ → bench/` code imports. The only cross-link between the two trees is a reciprocal
documentation pointer: this note links to `eval/EVAL.md`, and `eval/EVAL.md` carries a
one-paragraph reciprocal pointer back to `bench/BENCH.md`. That reciprocal paragraph is the
**single permitted change** to `eval/` under BENCH-01 (which otherwise stays byte-identical
to pre-milestone HEAD).

## `make bench` name collision — RESOLVED (Phase 77, BENCH-05)

**Resolution (Phase 77, RESEARCH Pitfall 1 / Open Question 1):** the original `make bench`
target (the Phase 64 Go microbenchmark suite at `./test/bench/...`) was **renamed to
`make bench-micro`**. The recipe is preserved verbatim — only the target name changed —
and `make bench-baseline` still captures a local baseline from that same microbench recipe.

The `bench` target name now belongs to the v1.12 milestone bench stack:

| Target | Invokes | Purpose |
|--------|---------|---------|
| `make bench-micro` | `go test -short -bench=. ... ./test/bench/...` | the original Phase 64 Go microbenchmark suite (formerly `make bench`) |
| `make bench-baseline` | same microbench recipe, teed to a gitignored baseline | local microbench baseline capture |
| `make bench` | `go run ./cmd/helix-bench run --benchmarks=$(SUITE)` | the milestone bench driver (BENCH-05) |
| `make bench SUITE=<suite>` | `... run --benchmarks=<suite>` | the `bench-<suite>` parameterization (default `internal-toolbench`) |
| `make bench-quick` | `go build ./cmd/helix` THEN `go run ./cmd/helix-bench run --languages=go --modes=your_agent_full --tasks=IT-go-patch-apply-1 --agent=scripted` | the hermetic scripted CI smoke gate (≤90s, ≥1 task succeeds) |

`make bench-quick` builds the `helix` daemon binary **first** (the subprocess-daemon /
forwarder-drive path SKIPs if `helix` is absent — RESEARCH build-sequencing note), then runs
the scripted `your_agent_full` smoke on the single `IT-go-patch-apply-1` seed task. Like `eval-quick`
it is **local-only, no-network, no-API-key** (the scripted agent replays a hard-coded MCP call
sequence; it never calls a real LLM — D-01). `make bench` (full driver) is local/nightly, never
a PR gate (project rule: benchmarks local-only).

> Migration note: anyone who called `make bench` for the Go microbenchmark must now call
> `make bench-micro`.

## `result.v2.json` provenance key names (stable contract for Phase 79)

The `result.v2.json` schema (`bench/schema/result.v2.schema.json`) requires only
`schema_version` and leaves `additionalProperties` **open** — so the provenance keys the
runtime emits are not pinned by the schema. Phase 77 fixes the following **snake_case** key
names as the stable contract so the Phase 79 aggregator/scorers read them without renaming
(RESEARCH Open Question 3):

| Key | Type | Meaning |
|-----|------|---------|
| `schema_version` | string | result schema version; currently `"v2"` (the one schema-required field) |
| `outcome` | string | task outcome resolved by `trace.Merge` (budget breach > non-zero verify exit → `"failed"`, else `"success"`) |
| `fairness` | object | the fairness block (mode overrides) from `runners.DefaultContract` / the resolved mode |
| `tokens_input` / `tokens_output` | int | token accounting (0 under the scripted gate; populated by the real-agent path) |
| `trace_ref` | string | filesystem path to the merged 2-leg `trace.json` for this cell |
| `model_id` | string | the model identifier attributed to the run (e.g. the scripted gate's placeholder sonnet id) |

These names are emitted today by the Plan 04 `result.v2` builder and verified by
`go test ./bench/runtime/ -run ResultV2Valid`. **Do not rename them in Phase 79** — the
aggregator consumes them as-is. New metrics (e.g. `edit_locality`, `regression_rate`, `pass@k`)
land as additional open properties alongside these, never by repurposing an existing key.

## Companion docs

- `bench/PROVIDERS.md` — per-provider LLM TOS attestation (machine-parseable frontmatter;
  parsed by `make verify-tos` from Plan 05).
- `bench/LICENSES.md` — per-dataset license audit scaffold (each external dataset adapter
  appends its row).

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

## ⚠️ `make bench` name collision (deferred to Phase 77)

**Flag:** there is an existing `make bench` target (`Makefile`) that runs the Phase 64
Go microbenchmark suite at `./test/bench/...`. A future Phase 77 (BENCH-05) wants
`make bench` to mean `cmd/helix-bench run` (this milestone's bench stack). These two
meanings collide on the same target name.

Phase 75 **only documents** this collision — it does **not** rename either target. Phase 77
must resolve it (rename one of the two; e.g. keep `make bench` for the existing microbench
and use `make bench-run` / `make helix-bench` for the milestone driver, or vice versa).
Until Phase 77 resolves it, `make bench` continues to run the existing microbench suite
unchanged. (RESEARCH Pitfall 1; settled "deferred to Phase 77" in RESEARCH Open Questions Q1.)

## Companion docs

- `bench/PROVIDERS.md` — per-provider LLM TOS attestation (machine-parseable frontmatter;
  parsed by `make verify-tos` from Plan 05).
- `bench/LICENSES.md` — per-dataset license audit scaffold (each external dataset adapter
  appends its row).

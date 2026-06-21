# Phase 85: Aider Polyglot Adapter + 7 Remaining Per-Language Runners - Context

**Gathered:** 2026-06-21
**Status:** Ready for planning
**Mode:** Auto-generated (discuss skipped via workflow.skip_discuss)

<domain>
## Phase Boundary

Cheapest external benchmark first — Aider Polyglot's 225 Exercism tasks across 6 languages requires no Docker and no upstream Python harness, just per-language test runners. Land the adapter AND simultaneously light up the remaining 7 ToolBench languages (Python, TypeScript, JavaScript, Java, C#, C++, Rust) since the per-language toolchain image work is the same dependency.

**Requirements:** ADAPTER-AIDER-01, TOOLBENCH-03, TOOLBENCH-04, TOOLBENCH-05, TOOLBENCH-06, TOOLBENCH-07, TOOLBENCH-08, TOOLBENCH-09

**Success Criteria (what must be TRUE):**

1. Aider Polyglot full run completes via `dataset-loader-only` adapter (shallow git clone of `Aider-AI/polyglot-benchmark` at pinned sha); 2-attempt protocol with stderr re-prompt; per-language pass-rate matches published sanity benchmarks for the pinned model.
2. All 7 remaining languages have a `bench/languages/<L>/runner.go` implementing `LanguageRunner`: Python (`pytest --json-report`, ≥ 8/10 capabilities), TypeScript (`vitest --reporter=json`, ≥ 8/10 + tsserver diagnostics), JavaScript (`jest --json`, ≥ 8/10 + eslint diagnostics), Java (`mvn test`, ≥ 8/10 + jdtls semantic view), C# (`dotnet test --logger trx`, ≥ 6/10), C++ (`cmake/ctest`, ≥ 6/10 + clangd semantic view), Rust (`cargo test --message-format=json`, ≥ 8/10 + rust-analyzer semantic view).
3. Pre-baked per-language toolchain images (offline-resolved deps via `mvn -o`, `cargo --offline`, `pnpm install --offline --frozen-lockfile`, `pip install --no-index --find-links=…`) ship; tests run with `--network=none`; non-hermetic tasks are flagged in the dataset.
4. Per-track Exercism license audit lands in `bench/datasets/aider-polyglot/LICENSE-AUDIT.md` (per-track sha256 + redistribution clause excerpt); `make verify-licenses` is green.

**Depends on:** Phase 78 (Go LanguageRunner is the template), Phase 79 (evaluators consume per-language test output), Phase 82 (aggregator handles per-language slicing)

</domain>

<decisions>
## Implementation Decisions

### Claude's Discretion
All implementation choices are at Claude's discretion — discuss phase was skipped per user setting. Use the ROADMAP phase goal, success criteria, and codebase conventions to guide decisions.

- The 7 per-language runners follow the Phase 78 Go `LanguageRunner` interface/template verbatim — research must locate that interface + the Go runner as the template to clone.
- Each runner shells out to the language's native test runner with a JSON/structured reporter (pytest/vitest/jest/mvn/dotnet/ctest/cargo) and parses results; semantic-view capabilities use the existing LSP wiring where applicable.
- **Environment reality:** toolchains present in this env — python3, node/npm, dotnet, g++, cargo/rustc, go; **javac is ABSENT (JRE-only)**. Per-language live test execution and the container `--network=none` runs (SC#3, which leans on Phase 84 container infra) must be gated on toolchain/engine availability and SKIP cleanly when absent — with hermetic siblings (golden reporter-output fixtures parsed without invoking the real toolchain) so each runner's parse/capability logic is unit-tested regardless. Plain `go test ./...` must not be false-green.
- The Aider Polyglot adapter is dataset-loader-only (shallow git clone at pinned sha) — gate the live clone/run on network; provide a small committed fixture task set for hermetic adapter tests.
- License audit + `make verify-licenses` is a hard gate following the Phase 75 `make validate-cost-table` / `make verify-tos` pattern.

</decisions>

<code_context>
## Existing Code Insights

Codebase context will be gathered during plan-phase research. Key anchors:
- Phase 78 Go `LanguageRunner` interface + Go runner (the template for the 7 runners) and the 10 capability test classes.
- Phase 79 evaluators (consume per-language test output → result.v2 metrics).
- Phase 82 aggregator per-language slicing.
- Phase 84 `bench/container/` engine + `--network=none` / image cache (for SC#3 hermetic toolchain runs).
- Phase 75 `make verify-tos` / `make validate-cost-table` hard-gate pattern (for `make verify-licenses`).
- `$HELIX_CACHE_DIR` cache convention; pinned-sha shallow-clone pattern (mirror Phase 83 corpus handling).

</code_context>

<specifics>
## Specific Ideas

No additional requirements beyond ADAPTER-AIDER-01 + TOOLBENCH-03..09 and the four success criteria above — discuss phase skipped. Refer to the ROADMAP phase description and success criteria.

</specifics>

<deferred>
## Deferred Ideas

None — discuss phase skipped.

</deferred>

# Phase 80: Five-of-Six Ablation Runners + Fairness Enforcement - Research

**Researched:** 2026-06-19
**Domain:** Go bench-harness wiring — mode resolution, fairness contract enforcement, cross-mode delta computation
**Confidence:** HIGH (every claim grounded in a verified file:line in this repo)

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** `baseline_plain` STILL spawns a Helix daemon with the existing `baseline.yaml` profile (zero Helix tools via the empty-lists mechanism — `skills:[] tools:[] exclude_tools:[]`, proven by `TestBaselineExposesZeroHelixTools`). It does NOT fork the cell path or skip the daemon — one code path for all modes, so the daemon-tap leg keeps the same 2-leg trace shape. `baseline_plain` resolves via a new `bench/runners/baseline_plain/MODE.md` with `profile: baseline` (NOT a new `bench-*` YAML — ABLATE-03 forbids one). Document the empty-inventory decision in `bench/BENCH.md`.
- **D-02:** ASYMMETRIC resolution of the two deferred modes. `no_semantic` RUNS end-to-end via the existing `bench-no-semantic.yaml` profile and produces a real, schema-valid `result.v2.json` row tagged with a deferred-guarantee marker (D-03). `baseline_rag` is a REGISTERED FAIL-CLOSED STUB: it gets a `bench/runners/baseline_rag/MODE.md` + resolver entry, but its runner returns a clear `deferred to Phase 83` status and produces **NO result row**. Smoke assertion: 4 real rows (`your_agent_full`, `baseline_plain`, `no_lsp`, `no_structured_edit`) + 1 marked-partial row (`no_semantic`) + `baseline_rag` registered-and-fail-closed (no row). "Five-of-six" = five modes have a runnable runner this phase.
- **D-03:** Belt-and-suspenders deferred marker for `no_semantic`: a machine-checkable `ablation_status` field (e.g. `guarantee_pending_phase_81`) in the result row AND prose in MODE.md. Schema impact: `result.v2.schema.json` is additive-only with top-level `additionalProperties` OPEN, so `ablation_status` lands as a new OPTIONAL field — NO schema major bump. Real modes either omit it or set it to `enforced`. Phase 81 flips the field to `enforced`.
- **D-04:** Two layers. (1) **Startup gate:** each runner calls `runners.DefaultContract.Validate()` at startup and treats a non-nil return as FATAL. The cell does NOT call `Validate()` today — Phase 80 wires it in. (2) **CI contract test:** asserts every runner's *effective* `(model_id, temperature, max_tokens, system_prompt_hash, retry_policy, cache_policy)` equals `DefaultContract` (modulo justified `Overrides[]` entries carrying non-empty `WaiverReason` + `ApprovedBy`).
- **D-05:** Minimal in-phase post-run delta pass — NOT the full aggregator. After all modes for a task complete, compute exactly the three deltas (`your_agent_full − baseline_plain`, `full − no_lsp`, `full − no_structured_edit`) and surface them in the per-mode result rows. Single-run, three-fixed-deltas only. The full aggregator/BCa/pass@k/variance gate/leaderboard stay Phase 82.

### Claude's Discretion
- Full ABLATE-01 `MODE.md` frontmatter schema beyond `mode` + `profile`. Keep it minimal and table-driven; the resolver's strict `KnownFields(true)` parser means any added key MUST be reflected in `modeFrontmatter`. Candidate fields: an ablation-target / disabled-subsystem note, a delta-baseline pointer. Do NOT break the existing `your_agent_full/MODE.md`.
- Exact `ablation_status` field name / enum values (D-03) — pick a clear, greppable convention; Phase 81 consumes it.
- Whether the startup `Validate()` call is unconditional or gated on the real `claude` agent (D-04) — the CI contract test MUST be unconditional regardless.
- Exact shape/location of the delta helper and whether deltas are written into the row or row + a sibling `deltas.json` (D-05) — as long as they surface in the per-mode rows.
- The per-mode directory name for no_semantic: prefer `bench/runners/your_agent_no_semantic/MODE.md` (Phase 81 criterion #4 references that exact path).

### Deferred Ideas (OUT OF SCOPE)
- Kernel `disable_semantic_subsystem` flag (ABLATE-06, the honest no_semantic ablation) → Phase 81.
- Real `baseline_rag` standalone `cmd/helix-bench-rag` binary + chromem-go vector store (ABLATE-04) → Phase 83.
- Multi-run aggregator, BCa bootstrap, pass@k, >5% variance gate, cost rollup, leaderboard → Phase 82.
- Container runtime / per-language runners / external benchmarks → Phases 84–88.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| ABLATE-01 | 6 modes implemented (`baseline_plain`, `baseline_rag`, `your_agent_full`, `no_lsp`, `no_semantic`, `no_structured_edit`); each runs end-to-end on a smoke task; mode definition lives in `bench/runners/<mode>/MODE.md`. Acceptance: each mode runs end-to-end on a smoke task. | Resolver is filesystem-as-table (`mode_resolver.go:83-106`); 5 new `MODE.md` dirs drop in with zero resolver Go change. `your_agent_full/MODE.md` is the seed template. Asymmetric handling (D-02): `baseline_rag` fail-closed (no row), `no_semantic` partial row. |
| ABLATE-03 | `baseline_plain` reuses existing `baseline.yaml` (Phase 67) — no new YAML, no Helix tools exposed. Acceptance: tool inventory for `baseline_plain` is empty except for shell/grep/read/edit/test from the agent runtime itself. | `baseline.yaml` verified at `internal/profile/profiles/baseline.yaml:29-31` (`skills:[] tools:[] exclude_tools:[]`); empty-inventory proven by `TestBaselineExposesZeroHelixTools` (`internal/profile/baseline_test.go`). `baseline_plain/MODE.md` → `profile: baseline`. |
</phase_requirements>

## Summary

Phase 80 is **pure growth on a complete substrate**. Every load-bearing component already exists and was verified in this session: the table-driven mode resolver (`bench/runners/mode_resolver.go`), the fairness contract struct with a working `Validate()` (`bench/runners/fairness_contract.go`), the end-to-end cell pipeline (`bench/runtime/cell.go` `RunCell`), the matrix dispatcher (`bench/runtime/matrix.go`), the result.v2 builder + validate-on-write gate (`bench/runtime/result.go`), the open-`additionalProperties` schema (`bench/schema/result.v2.schema.json`), the four ablation profiles (`internal/profile/profiles/bench-*.yaml`), and the zero-tools `baseline.yaml`. All dependency phases shipped what CONTEXT claims (76 profiles, 77 cell+resolver+seed mode, 78 Go ToolBench corpus = 10 tasks, 79 evaluators = coordinator + 5 graders).

The work is: (1) write 5 new `MODE.md` directories (no resolver change), (2) add a fail-closed control point in `RunCell` so `baseline_rag` produces NO row, (3) set the additive `ablation_status` field on the `no_semantic` row, (4) wire `DefaultContract.Validate()` into runner startup as a fatal gate, (5) add an unconditional CI contract test asserting effective config == `DefaultContract`, and (6) add a minimal post-run 3-delta pass at the matrix layer (the only place that sees all modes for a task).

**The single highest-risk finding (HIGH confidence):** the fairness contract's `(model_id, temperature, max_tokens, system_prompt_hash, retry_policy, cache_policy)` are **NOT currently applied to the real claude agent at all**. `internal/eval/agent/claude.go:76-89` `buildArgv` passes ONLY `--max-turns` (from `MaxToolCalls`) — there is no `--model`, no temperature, no `--max-tokens`, no `--append-system-prompt`, no retry/cache flag. So "effective config" for the D-04 CI contract test has **no live producer to read from today**. The planner must decide whether the contract test asserts against the *projected* config in `result.v2.json` (which sources `model_id` from `DefaultContract.ModelID` via `BuildResult`, `result.go:152`) or whether Phase 80 must also thread the contract fields into the claude argv. This is the central open question (see Open Questions Q1).

**Primary recommendation:** Treat this phase as 4 fully-real modes + 1 partial + 1 stub. Add ONE optional `disabled_subsystem` (or `ablation_target`) string to `modeFrontmatter` for documentation/greppability (matched by a `KnownFields(true)`-safe struct field), wire `Validate()` into `RunCell` step (1) (right after profile resolution), put the fail-closed `baseline_rag` short-circuit immediately after resolution, set `ablation_status` in `BuildResult` driven by a new `ResultInput.AblationStatus`, and add the delta pass as a new post-`RunMatrix` step that reads the just-written `result.v2.json` rows per task and writes deltas back (or to a sibling). Keep the CI contract test asserting the *projected* effective config (the values that actually land in `result.v2.json`), and document in the test that the live-agent enforcement of those fields is Phase 80+/future work.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Mode → profile resolution | Bench runners (`bench/runners/mode_resolver.go`) | Filesystem (`MODE.md` dirs) | The filesystem IS the table; resolver reads frontmatter, no Go map. |
| Fairness contract source | Bench runners (`bench/runners/fairness_contract.go`) | — | Single compile-time `DefaultContract` var; every runner reads it. |
| Startup fairness gate | Bench runtime (`bench/runtime/cell.go` `RunCell`) | Bench runners (`Validate()`) | Gate must fire per-cell before the daemon/agent runs; `Validate()` is the pure, testable predicate. |
| Result row emission / fail-close | Bench runtime (`bench/runtime/cell.go` `RunCell`) | Result builder (`result.go`) | `RunCell` is the only place that decides to call `BuildResult` + `writeDurable`; the fail-closed short-circuit lives here. |
| Cross-mode delta pass | Bench runtime matrix (`bench/runtime/matrix.go`) | Bench runners (a new delta helper) | The matrix layer is the ONLY place that sees all modes for a task; the cell sees one mode. |
| CI contract test | Bench runtime / runners test | Result builder (the projected config) | Static assertion over `DefaultContract` + what `BuildResult` projects. |
| Empty tool inventory | Profile filter (`internal/profile`/`internal/mcp` middleware) | `baseline.yaml` | Empty-lists mechanism; enforced by `ProfileFilterMiddleware`, not by the bench code. |

## Standard Stack

This is an internal-substrate phase. There are NO new external packages to install. All work uses Go stdlib + already-vendored deps verified present in this repo:

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `gopkg.in/yaml.v3` | already vendored | MODE.md frontmatter strict decode (`KnownFields(true)`) | Already the resolver's decoder (`mode_resolver.go:28,118-119`). |
| `github.com/santhosh-tekuri/jsonschema/v6` | already vendored | result.v2 validate-on-write gate | Already used by `Validate()` (`result.go:18,185-208`). |
| `github.com/spf13/cobra` | v1.9.1 (CLAUDE.md) | `helix-bench run` subcommand tree | Already the CLI framework. |
| `encoding/json` (stdlib) | — | result row read-back for the delta pass | The matrix already uses it (`matrix.go:18`). |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| New optional `modeFrontmatter` field | Hard-coded Go ablation-target map | A map re-introduces the exact anti-pattern the resolver was built to avoid (`mode_resolver.go:6-12`). Keep it frontmatter-driven. |
| Delta pass at matrix layer | Delta pass inside `RunCell` | The cell sees only one mode; it physically cannot compute a cross-mode delta. Matrix is the correct tier. |
| Writing deltas back into each row | Sibling `deltas.json` only | Criterion #4 says deltas "surface in the per-mode result rows" — a sibling-only artifact does not satisfy that. Prefer write-back (or both). |

**Installation:** None. `go build ./cmd/helix && go build ./cmd/helix-bench` (per CLAUDE.md), `go vet ./...`, `go test ./...`.

**Version verification:** No external packages added in this phase — the Package Legitimacy Audit and Environment Availability sections are N/A beyond the existing Go toolchain (confirmed: this repo builds today).

## Package Legitimacy Audit

> N/A — Phase 80 installs no external packages. All dependencies are Go stdlib or already-vendored modules verified in use in this repo.

## Verified Substrate (current code reality — grounds every plan)

### 1. Mode resolver — `bench/runners/mode_resolver.go` [VERIFIED: read in session]
- `ResolveProfile(mode string) (string, error)` (line 72) and `ResolveProfileFromRoot(root, mode string) (string, error)` (line 83) — signatures CONFIRMED. `RunCell` calls them at `cell.go:250-253`.
- `modeFrontmatter` struct (lines 35-38) has exactly TWO fields: `Mode string` (yaml:"mode"), `Profile string` (yaml:"profile").
- `parseFrontmatter` uses `dec.KnownFields(true)` (line 119) — CONFIRMED strict. **Any new frontmatter key (e.g. `disabled_subsystem`, `ablation_target`) MUST be added to `modeFrontmatter` or every `MODE.md` carrying it fails to parse.** So CONTEXT's "5 new MODE.md dirs drop in with zero Go change" is true ONLY if the new dirs use exactly `mode` + `profile`. The moment Claude's-discretion adds a documentation field, the struct changes (one-line field add) and ALL existing+new `MODE.md` files may carry it (optional — yaml zero-values an absent key, which `KnownFields` permits).
- `validateModeName` (lines 54-62) rejects path traversal BEFORE any FS join.
- Test `TestModeResolverYourAgentFull` asserts `ResolveProfile("your_agent_full") == "bench-full"` (`mode_resolver_test.go:23-30`); `TestModeResolverFromRootStrictFrontmatter` proves an unknown key is rejected (lines 65-88).

### 2. Fairness contract — `bench/runners/fairness_contract.go` [VERIFIED: read in session]
- `DefaultContract` (lines 84-100): `ModelID: "claude-sonnet-4-5-20250929"`, `Temperature: 0.0`, `MaxTokens: 8192`, `SystemPromptHash: "7873294a…"`, `Retry: RetryPolicy{MaxRetries:3, BackoffMs:1000}`, `Cache: CachePolicy{Mode:"ephemeral_5m"}`, `Overrides: map[string]ModeOverride{}`.
- `func (c FairnessContract) Validate() error` (lines 107-114): returns non-nil iff any override has an empty/whitespace `WaiverReason`. CONFIRMED the cell does NOT call it (no caller in `bench/` outside tests — verified by grep: only `fairness_contract_test.go` calls `.Validate()`).
- `FairnessContract` fields (lines 66-74): `ModelID, Temperature, MaxTokens, SystemPromptHash, Retry, Cache, Overrides`. **This is exactly the D-04 field set** `(model_id, temperature, max_tokens, system_prompt_hash, retry_policy, cache_policy)`.
- `ModeOverride` (lines 56-61): `MaxTokens *int`, `Temperature *float64`, `WaiverReason string`, `ApprovedBy string` — pointer fields distinguish "override present" from "inherit". CONFIRMED matches CONTEXT.
- `DeprecationGate(today time.Time, deprecationAt string) error` (lines 123-132) exists (injected clock). Not required by Phase 80 criteria but available.
- `TestDefaultContractValidates` (`fairness_contract_test.go:152-156`) already asserts the committed contract passes `Validate()` — so a startup gate over `DefaultContract` will never fatal in CI under the current contract.

### 3. Cell pipeline — `bench/runtime/cell.go` `RunCell` [VERIFIED: read in session]
End-to-end path confirmed (lines 214-554), numbered in the source:
1. Validate path segments (218-232) → 2. `cellDurablePaths` keys `<OutDir>/<task>/<mode>/<run_index>/result.v2.json` (159-167, CONFIRMED) → **(1) resolve mode→profile (247-256)** → (2) sandbox (258-290) → (3) write cell config + `StartDaemon` (292-332) → (4) drive agent scripted|claude (334-398) → (5) kill daemon + PID-gated tap (400-413) → (6) outcome via LanguageRunner.RunTests or verify.sh (415-455) → (7) synth CC leg (457-458) → (8) 2-leg merge (460-475) → (8b) coordinator.Grade (482-515) → **(9) `BuildResult` + `Validate` (517-537)** → **(10) `writeDurable` result + trace (539-549)** → (11) cleanup-on-success (551-552).

**Attach points for Phase 80:**
- **D-04 startup `Validate()` gate:** insert immediately after step (1) profile resolution (~line 256) or at the very top of `RunCell` (before sandbox). It is pure and fast — calling it unconditionally costs nothing. The `cfg.Agent` field (`cell.go:93`, values `""|"scripted"|"claude"`) is available if the planner chooses to gate enforcement on `--agent=claude`. **Recommendation: call `Validate()` unconditionally at the top of `RunCell`; it is the CI-cheap fail-closed guard. The CI contract test is the separate always-on guarantee.**
- **D-02 baseline_rag fail-close (no row):** the row is emitted unconditionally today at steps (9)-(10). The cleanest control point is right after profile resolution: if `cfg.Mode == "baseline_rag"` (or, better, if the resolved profile / a `MODE.md` marker signals a stub), return a `CellResult` with a `Deferred`/`SkippedReason` signal BEFORE the sandbox/daemon spawn — producing NO `result.v2.json`. The matrix's `runOneCell` (`matrix.go:234-273`) sets `Success = err==nil && VerifyExitCode==0 && ResultValid`; a fail-closed stub must NOT be counted as a success — give it a distinct outcome flag so the smoke can assert "registered + no row".
- **D-03 ablation_status on no_semantic row:** set at step (9). `BuildResult` (`result.go:121-162`) takes a `ResultInput`; add an `AblationStatus string` field to `ResultInput` + `resultDoc` (omitempty), and have `RunCell` pass `guarantee_pending_phase_81` when `cfg.Mode == "your_agent_no_semantic"`. Schema permits it (open `additionalProperties`).
- **D-05 delta pass:** NOT in `RunCell` (the cell sees one mode). See matrix finding below.

### 4. Result builder + schema — `bench/runtime/result.go` + `bench/schema/result.v2.schema.json` [VERIFIED: read in session]
- Schema top-level: `"required": ["schema_version"]` and **NO top-level `additionalProperties: false`** (verified — the object at line 7 has no `additionalProperties` key, so it defaults to OPEN). The `$comment` (line 6) explicitly states "additionalProperties is left OPEN at the top level so adding a field never requires a schema bump." → **`ablation_status` lands as a new OPTIONAL top-level field with NO schema change required** (D-03 CONFIRMED). Adding it to the schema's `properties` for documentation is optional and still additive.
- `mode` field exists (schema lines 18-21); `fairness.overrides[]` exists (lines 51-76) with `mode`, `waiver_reason`, `approved_by` item props.
- `BuildResult` (`result.go:121-162`) sources `model_id` from `in.Fairness.ModelID` (line 152) and `fairness.overrides` from `fairnessBlock(in.Fairness)` (line 149). `RunCell` passes `Fairness: runners.DefaultContract` (`cell.go:527`). **This is what the CI contract test can assert against** — the projected effective config in the row.
- `Validate(resultBytes)` (lines 185-208) is the validate-on-write gate. Any new field must keep the doc schema-valid (open additionalProperties means it will).
- `resultDoc` (lines 90-114) is the typed doc; `ResultInput` (lines 28-69) is the builder input. Both need a small additive change for `ablation_status`.

### 5. Matrix dispatcher — `bench/runtime/matrix.go` [VERIFIED: read in session]
- `ExpandMatrix(benchmarks, languages, modes, tasks)` (line 104) produces the cartesian product; ordering is benchmarks×languages×modes×tasks (line 101 comment, 140-148).
- `RunMatrix(ctx, cells, parallel, cfg)` (line 160) → `dispatch` (174) runs cells concurrently under a semaphore; `runOneCell` (234) maps a `Cell` to a `CellConfig` and calls `RunCell`.
- **Critical for D-05:** there is currently NO "all modes for a task complete" barrier. Cells run independently and concurrently; `Summary.Outcomes` (line 81) holds all `CellOutcome`s AFTER `wg.Wait()` (line 212). **The delta pass must run as a post-`RunMatrix` step (in `cmd/helix-bench`'s `runBench`, after line 236) OR as a new function in `matrix.go` invoked after dispatch** — grouping the written rows by task, finding the 4 mode rows, computing the 3 deltas, and writing them back. The rows are on disk at `<OutDir>/<task>/<mode>/<run_index>/result.v2.json` (`cellDurablePaths`); the delta pass reads them via `encoding/json`, or reads from `Summary.Outcomes[].Result.ResultPath`.

### 6. Profiles [VERIFIED: read/grep in session]
- `internal/profile/profiles/baseline.yaml`: `skills:[] tools:[] exclude_tools:[]` (lines 29-31), `name: baseline` (line 17). Empty-inventory mechanism documented in-file (lines 1-16) and proven by `TestBaselineExposesZeroHelixTools` (`internal/profile/baseline_test.go`, confirmed present).
- `bench-full.yaml`, `bench-no-lsp.yaml`, `bench-no-semantic.yaml`, `bench-no-structured-edit.yaml` all present (verified via `ls`).
- `bench-no-lsp.yaml:37` carries `disable_lsp_subsystem: true`; `bench-no-structured-edit.yaml:45` carries `disable_structured_edit_subsystem: true` (kernel flags, ABLATE-05/07, Phase 76).
- `bench-no-semantic.yaml` (read in full): tool-filter-only — excludes the 10 semantic-store tools (lines 38-49), keeps `get_repo_map`/`get_context` (tree-sitter fallback), sets **NO kernel flag** (line 8 comment confirms `disable_semantic_subsystem` is Phase 81). This is exactly why the `no_semantic` row needs `ablation_status: guarantee_pending_phase_81`.

### 7. Fairness test + system prompt [VERIFIED: read in session]
- `bench/runners/system_prompt.txt` present (623 bytes). `TestSystemPromptHashMatches` (`fairness_contract_test.go:138-148`) recomputes `sha256(system_prompt.txt)` and asserts it equals `DefaultContract.SystemPromptHash` — the drift gate. The new CI contract test sits alongside in this same package (or in `bench/runtime`).
- No existing test asserts effective config == `DefaultContract` for a runner (grep found none) — the D-04 CI contract test is genuinely new.

### 8. Dependency phases shipped what CONTEXT claims [VERIFIED: ls in session]
- **Phase 76 profiles:** all 4 `bench-*.yaml` present. ✓
- **Phase 77 cell+resolver+seed:** `cell.go`, `mode_resolver.go`, `your_agent_full/MODE.md` (frontmatter `mode: your_agent_full` / `profile: bench-full`) present. ✓
- **Phase 78 Go ToolBench corpus:** `bench/datasets/internal-toolbench/go/` has 10 task dirs (IT-go-call-graph-1, context-min-1, dependency-graph-1, failure-handling-1, fuzzy-search-1, incremental-update-1, lsp-diagnostics-1, patch-apply-1, rename-safety-1, semantic-view-1). ✓
- **Phase 79 evaluators:** `bench/evaluators/` has `coordinator/`, `metrics.go` (the 19-field nullable `Metrics` struct, verified), `patch_validator/`, `regression_checker/`, `test_runner/`, `token_meter/`, `tool_trace_analyzer/`. `coordinator.Grade` is wired into `cell.go:508-515`. ✓

## The Fairness "Effective Config" Gap (HIGHEST-VALUE FINDING)

[VERIFIED: read `internal/eval/agent/claude.go:74-89` + `bench/runtime/subprocess/claude.go`]

The real claude agent is driven by `internal/eval/agent.Agent.Run`. Its `buildArgv` (`claude.go:76-89`) emits ONLY:
```
--print --bare --strict-mcp-config --mcp-config <path> --output-format stream-json --verbose --include-partial-messages --max-turns <MaxToolCalls> <prompt>
```
There is **no `--model`, no temperature flag, no `--max-tokens`, no `--append-system-prompt`/system-prompt flag, no retry/cache flag**. `BudgetParams` (`claude.go:22-23`) carries only `MaxToolCalls`. `bench/runtime/subprocess/claude.go` (`StartClaude`) passes only `MaxToolCalls` through.

**Implication for D-04's CI contract test:** the contract's `(model_id, temperature, max_tokens, system_prompt_hash, retry_policy, cache_policy)` have **no live producer** on the agent path today. The only place those values are "effective" is the *projected* `result.v2.json` (where `model_id` comes from `DefaultContract.ModelID`, `result.go:152`). The other five fields are not yet emitted anywhere.

**This is the load-bearing decision for the planner (Open Q1).** Two viable scopes:
- **(A) Assert the projected config (recommended for the Phase 80 boundary):** the CI contract test asserts that every runner, when it builds a result, projects `model_id == DefaultContract.ModelID` and that the contract itself `Validate()`s. This is unconditional, hermetic, and satisfies "no runner silently drifts from the contract" for the fields that ARE projected. Document explicitly that live-agent enforcement of temperature/max_tokens/system_prompt is future work (it depends on the claude CLI flag surface, which Phase 80 should not invent).
- **(B) Thread the contract into the claude argv:** extend `BudgetParams`/`buildArgv` to set `--model DefaultContract.ModelID` and (if the CLI supports them) temperature/max-tokens/append-system-prompt, then assert the constructed argv equals the contract. This is larger, touches `internal/eval/agent` (shared with the eval harness), and risks scope-creep beyond "growth." **Verify the actual `claude` CLI flag surface before committing to (B)** — this repo's `claude.go` deliberately keeps argv minimal.

Either way, **the CI contract test must be unconditional** (D-04) and must not require a live model.

## Architecture Patterns

### System Architecture Diagram

```
helix-bench run --modes=<m1,m2,...> --tasks=<t1,...> --agent=scripted|claude
        │
        ▼
runBench (cmd/helix-bench/main.go)
        │  ExpandMatrix(benchmarks × languages × modes × tasks)
        ▼
RunMatrix (matrix.go)  ──sem-bounded──►  runOneCell ──► RunCell (one cell = one mode)
        │                                                     │
        │                                          (1) ResolveProfile(mode) ──reads──► bench/runners/<mode>/MODE.md
        │                                                     │
        │                                          ┌──────────┴───────────┐
        │                                          │  [NEW D-04] Validate() fatal gate
        │                                          │  [NEW D-02] if baseline_rag → fail-closed, NO row, return
        │                                          ▼
        │                                   sandbox → StartDaemon(profile) → drive agent → kill+tap → merge → Grade
        │                                          │
        │                                   (9) BuildResult(Fairness=DefaultContract,
        │                                          [NEW D-03] AblationStatus for no_semantic) → Validate → writeDurable
        │                                          ▼
        │                                   <OutDir>/<task>/<mode>/<run_index>/result.v2.json
        ▼
[NEW D-05] post-run delta pass (after RunMatrix / over Summary.Outcomes):
        group rows by task → read the 4 real-mode rows → compute 3 deltas
        (full−baseline_plain, full−no_lsp, full−no_structured_edit)
        → write deltas back into per-mode rows (+ optional sibling deltas.json)

[NEW D-04] CI contract test (unconditional, package test):
        for every registered mode → assert projected effective config == DefaultContract
        (model_id projected today; temperature/max_tokens/system_prompt_hash/retry/cache
         per Open Q1 scope decision)
```

### Recommended Project Structure (new/changed files)
```
bench/runners/
├── baseline_plain/MODE.md          # NEW — profile: baseline (D-01)
├── no_lsp/MODE.md                  # NEW — profile: bench-no-lsp
├── no_structured_edit/MODE.md      # NEW — profile: bench-no-structured-edit
├── your_agent_no_semantic/MODE.md  # NEW — profile: bench-no-semantic, prose: deferred (D-03)
├── baseline_rag/MODE.md            # NEW — stub marker, fail-closed (D-02)
├── mode_resolver.go                # CHANGE iff a new frontmatter field is added (Claude's discretion)
├── fairness_contract.go            # unchanged (Validate already exists)
├── contract_test.go                # NEW — unconditional CI effective-config == DefaultContract (D-04)
└── deltas.go (or bench/runtime/)   # NEW — small 3-delta helper (D-05)
bench/runtime/
├── cell.go                         # CHANGE — wire Validate() gate, baseline_rag fail-close, ablation_status
├── result.go                       # CHANGE — additive ResultInput.AblationStatus + resultDoc field
└── matrix.go (or cmd/helix-bench)  # CHANGE — invoke post-run delta pass over Summary.Outcomes
bench/schema/result.v2.schema.json  # OPTIONAL CHANGE — document ablation_status (additive, no bump)
bench/BENCH.md                      # CHANGE — baseline_plain empty inventory + no_semantic deferred guarantee
internal/profile/profiles/          # unchanged (all profiles exist)
```

### Pattern 1: New MODE.md (table-driven, KnownFields-safe)
**What:** Each new mode is a directory with a `MODE.md` whose frontmatter the resolver reads.
**When to use:** Every one of the 5 new modes.
**Example (matches the seed `your_agent_full/MODE.md` exactly):**
```markdown
---
mode: no_lsp
profile: bench-no-lsp
---

# no_lsp
The LSP-disabled ablation arm. Resolves to bench-no-lsp (disable_lsp_subsystem).
```
If Claude's discretion adds a documentation field, it MUST be reflected in `modeFrontmatter` (e.g. add `DisabledSubsystem string `yaml:"disabled_subsystem"``) — otherwise `KnownFields(true)` rejects it. Keep the field OPTIONAL (yaml zero-value when absent is fine).

### Pattern 2: Additive result field (no schema bump)
**What:** `ablation_status` rides as an open additional property.
**Example:**
```go
// result.go — ResultInput + resultDoc
AblationStatus string `json:"ablation_status,omitempty"` // omitempty: real modes omit it
```
`RunCell` sets `AblationStatus: "guarantee_pending_phase_81"` only for `your_agent_no_semantic`. Schema is open at top level, so `Validate()` passes unchanged.

### Pattern 3: Fail-closed stub (no row)
**What:** `baseline_rag` returns early before sandbox/daemon, emitting NO `result.v2.json`.
**Where:** `RunCell` immediately after profile resolution (cell.go ~line 256), or detected via a `MODE.md` stub marker so the resolver/runner can branch without a hard-coded mode-name string.
**Smoke contract:** assert the `baseline_rag` cell produced no `result.v2.json` AND surfaced a clear "deferred to Phase 83" status — distinct from both success and infra-error.

### Anti-Patterns to Avoid
- **Hard-coding a Go `map[mode]profile`** — the resolver was explicitly built to avoid this (`mode_resolver.go:6-12`). Add `MODE.md` dirs, not map entries.
- **Computing deltas inside `RunCell`** — the cell sees one mode; a cross-mode delta there is impossible. Matrix/post-run only.
- **Bumping the schema to v3 for `ablation_status`** — top-level `additionalProperties` is OPEN; additive fields never bump (schema `$comment`, D-03).
- **Counting the `baseline_rag` stub as a succeeded cell** — `runOneCell` success is `err==nil && VerifyExitCode==0 && ResultValid` (`matrix.go:272`); a fail-closed stub must carry a distinct outcome so it is neither a success nor an infra error.
- **Re-rolling the claude argv / credential allowlist in bench** — `subprocess/claude.go` delegates to `internal/eval/agent` deliberately (T-77-11). If Open Q1 scope (B) is chosen, extend the eval agent, do not fork it.
- **Growing the delta pass into the Phase 82 aggregator** — single-run, 3 fixed deltas only (D-05).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Mode→profile lookup | A new Go map or switch | The existing `ResolveProfile` reading `MODE.md` | Filesystem-as-table is the established pattern (`mode_resolver.go`). |
| Result schema validation | A bespoke field checker | `runtime.Validate(bytes)` (`result.go:185`) | Already compiles the embedded schema offline. |
| Fairness predicate | A new override-checker | `DefaultContract.Validate()` (`fairness_contract.go:107`) | Already implemented + tested. |
| System-prompt drift detection | A new hash check | `TestSystemPromptHashMatches` (`fairness_contract_test.go:138`) | Already the drift gate. |
| Empty tool inventory for baseline_plain | A bench-side tool stripper | `baseline.yaml` empty-lists + `ProfileFilterMiddleware` | Enforced by the profile filter (D-01); proven by `TestBaselineExposesZeroHelixTools`. |
| Per-cell result path | A custom path joiner | `cellDurablePaths` (`cell.go:159`) | Already V5-validated and run-index-keyed. |

**Key insight:** Phase 80 has essentially nothing to invent — the danger is *re-inventing* substrate that already exists. Every "build" instinct should map to a "call the existing thing" action.

## Common Pitfalls

### Pitfall 1: KnownFields(true) rejects any unmodeled frontmatter key
**What goes wrong:** Adding a `disabled_subsystem`/`delta_baseline` key to a new `MODE.md` without adding the field to `modeFrontmatter` makes the resolver hard-fail that mode at parse time.
**Why it happens:** `dec.KnownFields(true)` (`mode_resolver.go:119`) is strict by design (fail-closed on malformed frontmatter).
**How to avoid:** If any key beyond `mode`/`profile` is added, add the corresponding struct field FIRST and keep it optional. Re-run `go test ./bench/runners/` (the strict-frontmatter test catches drift).
**Warning signs:** `mode_resolver: mode "X": parse frontmatter: ... field not found in type`.

### Pitfall 2: baseline_rag stub silently counted as success or as infra error
**What goes wrong:** If `baseline_rag` returns a normal `CellResult` (success) it inflates the leaderboard with a non-existent arm; if it returns an infra error it pollutes the per-cell error stream.
**Why it happens:** `runOneCell` (`matrix.go:272`) has only two outcomes today: success or `Err`.
**How to avoid:** Add a third outcome (e.g. `CellOutcome.Deferred bool` or a `SkippedReason string`); the smoke asserts the stub is "registered + deferred + no row," distinct from both.
**Warning signs:** `summary.Succeeded` includes a `baseline_rag` cell, or a `result.v2.json` exists under `<task>/baseline_rag/`.

### Pitfall 3: Delta pass run before all modes for a task complete
**What goes wrong:** Reading mode rows before the matrix finishes yields missing/partial deltas.
**Why it happens:** `RunMatrix` dispatches cells concurrently (`matrix.go:186-211`); there is no per-task barrier.
**How to avoid:** Run the delta pass strictly AFTER `RunMatrix` returns (`wg.Wait()` has completed). Group `Summary.Outcomes` by task; only compute the 3 deltas when all 4 real-mode rows for that task exist; skip tasks missing a mode (and say so).
**Warning signs:** deltas computed against a nil/absent baseline row; NaN/garbage delta values.

### Pitfall 4: Treating the deferred no_semantic row as a clean measurement
**What goes wrong:** The Phase 82 aggregator can't distinguish the partial `no_semantic` row from a real one, producing a false "no_semantic" claim.
**Why it happens:** `bench-no-semantic.yaml` is tool-filter-only; the daemon can still read the semantic store via back-channel (SemanticLookup / `SetEnrichFn` / RankFiles) until Phase 81's kernel flag (`bench-no-semantic.yaml:7-8`).
**How to avoid:** Set `ablation_status: guarantee_pending_phase_81` on the row (D-03) AND document the deferral in `your_agent_no_semantic/MODE.md` prose + `bench/BENCH.md`. Phase 81 flips it to `enforced`.
**Warning signs:** a `no_semantic` row with no `ablation_status` field; the aggregator reading it as honest.

### Pitfall 5: Asserting a fairness "effective config" that no producer sets
**What goes wrong:** A CI contract test that asserts the live claude argv carries `temperature`/`max_tokens`/`system_prompt` fails (or passes vacuously) because `buildArgv` sets none of them (`internal/eval/agent/claude.go:76-89`).
**Why it happens:** The agent path threads only `MaxToolCalls`; the contract fields are projected only into `result.v2.json` (`model_id`).
**How to avoid:** Resolve Open Q1 first. Scope (A): assert the projected config (`model_id == DefaultContract.ModelID`, contract `Validate()`s) and document the rest as future enforcement. Scope (B): thread the fields into the argv and assert the argv — larger, touches the shared eval agent.
**Warning signs:** a contract test that "passes" without ever reading a real effective value; or one that fails on temperature/max_tokens with no obvious producer.

### Pitfall 6 (from `.planning/research/PITFALLS.md`): daemon-tap PID cross-talk & token-counting source
**What goes wrong:** New modes' trace legs cross-contaminate, or token metrics get sourced from the wrong counter.
**How to avoid:** The cell already PID-gates the tap (`cell.go:328-413`, `RejectedForeignPid`) and sources tokens from the provider usage block via `UsagePresent` (`cell.go:497-503`). New modes inherit this for free as long as they flow through the unmodified `RunCell` spine — do NOT fork the path for any mode (D-01).

## Code Examples

### Resolving + gating in RunCell (the D-04 + D-02 attach point)
```go
// Source: bench/runtime/cell.go:247-256 (existing) — insert NEW gates here.
// (1) Resolve mode -> profile (D-05, existing).
profileName, err = runners.ResolveProfile(cfg.Mode) // or ...FromRoot
if err != nil { return res, fmt.Errorf(...) }

// [NEW D-04] fail-closed fairness gate — pure, unconditional, CI-cheap.
if verr := runners.DefaultContract.Validate(); verr != nil {
    return res, fmt.Errorf("bench/runtime: fairness contract invalid: %w", verr)
}

// [NEW D-02] baseline_rag is a registered fail-closed stub — NO row.
// (Prefer detecting a MODE.md stub marker over a hard-coded name.)
if isDeferredStub(cfg.Mode /* or resolved marker */) {
    res.Deferred = true // new field
    res.DeferredReason = "baseline_rag: real RAG arm deferred to Phase 83 (ABLATE-04)"
    return res, nil     // no sandbox, no daemon, no BuildResult, no writeDurable
}
```

### Additive ablation_status (D-03)
```go
// Source: bench/runtime/result.go:28-69 (ResultInput) + 90-114 (resultDoc) — add one field each.
type ResultInput struct {
    // ... existing fields ...
    AblationStatus string // "" for honest modes; "guarantee_pending_phase_81" for no_semantic
}
type resultDoc struct {
    // ... existing fields ...
    AblationStatus string `json:"ablation_status,omitempty"`
}
// In RunCell step (9): set AblationStatus when cfg.Mode == "your_agent_no_semantic".
```

### Minimal 3-delta pass (D-05, post-RunMatrix)
```go
// Source: runs AFTER bench/runtime.RunMatrix returns (matrix.go) — over Summary.Outcomes.
// For each task: locate the your_agent_full, baseline_plain, no_lsp, no_structured_edit rows;
// compute exactly 3 deltas on the comparable metrics; write them back into each mode row
// (and/or a sibling deltas.json). Skip tasks missing any of the 4 modes (log + continue).
// MUST stay single-run, 3-fixed-deltas — NOT the Phase 82 aggregator.
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Hard-coded mode→profile map (`internal/eval/runner.profileForMode`) | Filesystem-as-table `MODE.md` resolver | Phase 77 | New modes add no Go (unless a new frontmatter key). |
| Single `your_agent_full` seed mode | 6-mode matrix | Phase 80 (this) | Resolver unchanged; 5 new dirs. |
| Fairness contract defined but uncalled | `Validate()` wired into runner startup | Phase 80 (this) | Fail-closed unfair-benchmark guard. |

**Deprecated/outdated:** none relevant to this phase — the substrate is current (Phases 75-79, all 2026-06).

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | The `claude` CLI does not expose `--temperature`/`--max-tokens`/`--append-system-prompt` flags that the bench should set | Effective Config Gap, Open Q1 | If it does, scope (B) becomes cheaper and the contract test could assert the live argv. Planner should verify the installed `claude` CLI flag surface before locking Q1. (This is the only `[ASSUMED]` claim — derived from `buildArgv` setting none of them, NOT from confirming the CLI lacks them.) |

**All other claims in this research are `[VERIFIED: read in session]` against a real file:line in this repo.**

## Open Questions

1. **D-04 — what is the "effective config" the CI contract test asserts, and is `Validate()` unconditional?**
   - What we know: `DefaultContract` holds all 6 fields; `BuildResult` projects only `model_id` into the row (`result.go:152`); the claude argv sets none of temperature/max_tokens/system_prompt (`internal/eval/agent/claude.go:76-89`); `Validate()` is pure and CI-cheap.
   - What's unclear: whether Phase 80 should (A) assert the projected config + document the rest as future, or (B) thread the contract fields into the agent argv (touches shared `internal/eval/agent`).
   - Recommendation: **Scope (A) for the Phase 80 boundary** — assert `model_id == DefaultContract.ModelID` + contract `Validate()`s, unconditionally; document live-agent enforcement of the other fields as out-of-phase. Call `Validate()` UNCONDITIONALLY at the top of `RunCell` (cheap, fail-closed). Confirm the `claude` CLI flag surface (A1) before considering (B).

2. **D-05 — write deltas back into rows, into a sibling `deltas.json`, or both?**
   - What we know: criterion #4 requires deltas to "surface in the per-mode result rows"; rows are written atomically by `writeDurable` (`cell.go:668`); the delta pass runs after the rows exist.
   - What's unclear: whether re-opening + rewriting each row is preferred over a sibling artifact.
   - Recommendation: write back into each per-mode row (satisfies criterion #4 literally) under a new open property (e.g. `ablation_deltas`), re-validating via `Validate()`. Optionally ALSO emit a sibling `<OutDir>/<task>/deltas.json` for convenience. Keep the helper tiny.

3. **D-02 — how does a runner signal "fail-closed stub" cleanly?**
   - What we know: `RunCell` emits a row unconditionally today; `runOneCell` has only success/err outcomes.
   - Recommendation: detect a stub marker (prefer a `MODE.md` frontmatter flag over a hard-coded `"baseline_rag"` string, so the convention generalizes), short-circuit in `RunCell` before sandbox, and add a `Deferred`/`SkippedReason` outcome the matrix + smoke can assert.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | build/test | ✓ | repo builds today | — |
| `helix` binary | per-cell daemon (scripted smoke) | built by `go build ./cmd/helix` | — | `make bench-quick` builds it first (`bench/BENCH.md:79-83`) |
| `claude` CLI | `--agent=claude` only | not required for CI | — | scripted agent is the hermetic gate; claude path is wired-not-gating (`StartClaude` returns `ErrClaudeNotFound` cleanly) |

**Missing dependencies with no fallback:** none — the phase's CI gate is hermetic and scripted (no model, no network).
**Missing dependencies with fallback:** `claude` CLI (local-only, opt-in).

## Validation Architecture

> Nyquist validation ENABLED for this phase.

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go standard `testing` (`go test`) |
| Config file | none (Go convention) |
| Quick run command | `go test ./bench/runners/ ./bench/runtime/ -count=1` |
| Full suite command | `go test ./... && go vet ./...` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| ABLATE-01 | Each new mode resolves to its profile via MODE.md | unit | `go test ./bench/runners/ -run TestModeResolver -count=1` | ❌ Wave 0 (extend existing resolver test for the 5 new modes) |
| ABLATE-01 | 4 real rows + 1 partial (no_semantic) + 1 fail-closed (baseline_rag, no row) on a smoke task | integration | `go test ./bench/runtime/ -run TestRunCell -count=1` (extend) + a multi-mode smoke | ❌ Wave 0 |
| ABLATE-03 | baseline_plain → profile baseline; empty Helix inventory | unit | `go test ./internal/profile/ -run TestBaselineExposesZeroHelixTools -count=1` (exists) + a resolver assertion for `baseline_plain` | ⚠️ baseline test exists; mode resolution assertion is Wave 0 |
| FAIR (D-04) | Startup `Validate()` is fatal on a non-nil contract | unit | `go test ./bench/runtime/ -run TestFairnessGate -count=1` | ❌ Wave 0 |
| FAIR (D-04) | Every runner's effective config == `DefaultContract` (unconditional) | unit (contract) | `go test ./bench/runners/ -run TestEffectiveConfigMatchesContract -count=1` | ❌ Wave 0 |
| D-03 | no_semantic row carries `ablation_status: guarantee_pending_phase_81`; schema still valid | unit | `go test ./bench/runtime/ -run TestAblationStatus -count=1` | ❌ Wave 0 |
| D-05 | 3 deltas compute correctly + surface in per-mode rows | unit | `go test ./bench/runtime/ -run TestAblationDeltas -count=1` | ❌ Wave 0 |
| (regression) | system-prompt drift gate | unit | `go test ./bench/runners/ -run TestSystemPromptHashMatches -count=1` (exists) | ✅ |

### Sampling Rate
- **Per task commit:** `go test ./bench/runners/ ./bench/runtime/ -count=1 && go vet ./bench/...`
- **Per wave merge:** `go test ./bench/... ./internal/profile/... -count=1`
- **Phase gate:** `go test ./... && go vet ./...` green; `make bench-quick` exits 0 (hermetic scripted smoke); a multi-mode scripted smoke produces 4 real rows + 1 partial + 0 baseline_rag rows.

### Natural Test Boundaries (Nyquist)
1. **Startup fairness gate** — pure `Validate()` over `DefaultContract`; unit-testable with a synthetic bad-override contract (mirror `TestEmptyWaiverReasonFatal`).
2. **CI contract test** — static, unconditional, no model: assert projected effective config == `DefaultContract` for every registered mode.
3. **4-real + 1-partial + 1-fail-closed smoke** — drive the scripted agent across all 6 modes on one seed task; assert row count, `ablation_status` on no_semantic, and no row + deferred status for baseline_rag.
4. **3-delta computation** — feed 4 synthetic mode rows; assert exactly the 3 deltas with correct arithmetic; assert they land in the per-mode rows.

### Wave 0 Gaps
- [ ] `bench/runners/contract_test.go` — unconditional effective-config == `DefaultContract` (D-04)
- [ ] `bench/runtime/cell_test.go` (extend) — fairness gate fatal; baseline_rag no-row; no_semantic `ablation_status`
- [ ] `bench/runners/mode_resolver_test.go` (extend) — resolve the 5 new modes to their profiles
- [ ] `bench/runtime/` delta test — 3-delta arithmetic + row write-back
- [ ] Multi-mode scripted smoke harness (4 real + 1 partial + 1 fail-closed)
- [ ] Framework install: none — Go `testing` is built-in.

## Security Domain

> `security_enforcement`: no explicit `false` found, but this is a Go-native internal bench-harness phase with NO new external input surface, NO auth/session/crypto, and NO network in the CI gate. ASVS categories are evaluated below and are almost entirely N/A.

### Applicable ASVS Categories
| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | No auth surface in bench harness. |
| V3 Session Management | no | No sessions. |
| V4 Access Control | no | Local CLI tool. |
| V5 Input Validation | yes (existing) | Path-segment validation already enforced: `validateModeName` (`mode_resolver.go:54`), `validateCellKey`/`validatePathSegment`/`validateRunIndexSegment` (`cell.go`). New modes/paths MUST flow through these — they already do (RunCell validates task/mode/benchmark/language before any join). |
| V6 Cryptography | no (display-only) | `WaiverReason`/`ApprovedBy` are free-text attestation, never exec'd/shelled (`fairness_contract.go:19-22`, T-75-09). `SystemPromptHash` is sha256 for drift detection, not a security control. Do NOT hand-roll any crypto. |

### Known Threat Patterns for this stack
| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Path traversal via a malicious mode/task name | Tampering | Existing `validateModeName`/`validatePathSegment` BEFORE any `filepath.Join` (already enforced; new modes inherit it). |
| Untrusted `claude` argv / cred leak in subprocess | Info disclosure | Reuse `internal/eval/agent` strict env allowlist (T-77-11); do NOT re-roll argv in bench. |
| Stub mode silently scored as real | Tampering (data integrity) | Distinct deferred outcome for baseline_rag (D-02); `ablation_status` marker for no_semantic (D-03). |

## Sources

### Primary (HIGH confidence)
- `bench/runners/mode_resolver.go` (read) — resolver signatures, `modeFrontmatter`, `KnownFields(true)`.
- `bench/runners/fairness_contract.go` (read) — `DefaultContract`, `Validate()`, field set, `ModeOverride`.
- `bench/runtime/cell.go` (read) — `RunCell` end-to-end path, attach points, result path keying.
- `bench/runtime/result.go` (read) — `BuildResult`, `ResultInput`, `resultDoc`, `Validate()`, `model_id` projection.
- `bench/runtime/matrix.go` (read) — `ExpandMatrix`/`RunMatrix`/`runOneCell`; no per-task barrier.
- `bench/schema/result.v2.schema.json` (read) — top-level `additionalProperties` OPEN; `mode`, `fairness.overrides[]`.
- `bench/runners/fairness_contract_test.go` (read) — existing gates; no effective-config test exists.
- `bench/runners/your_agent_full/MODE.md` (read) — seed frontmatter template.
- `internal/profile/profiles/baseline.yaml` + `bench-no-semantic.yaml` (read), `bench-no-lsp.yaml`/`bench-no-structured-edit.yaml` kernel-flag lines (grep).
- `internal/eval/agent/claude.go` (read) — `buildArgv` sets only `--max-turns`; the effective-config gap.
- `bench/runtime/subprocess/claude.go` (read) — `StartClaude` threads only `MaxToolCalls`.
- `cmd/helix-bench/main.go` (read) — `runBench`, `--modes`, post-`RunMatrix` insertion point.
- `bench/BENCH.md` (read) — where to document; `make bench-quick` build sequencing.
- `bench/evaluators/metrics.go` (grep) — 19-field nullable `Metrics`; ls of corpus + evaluators confirms Phases 78/79.
- `.planning/phases/80-.../80-CONTEXT.md`, `.planning/REQUIREMENTS.md` (read) — locked decisions, ABLATE-01/03.

### Secondary (MEDIUM confidence)
- `.planning/research/PITFALLS.md` (referenced via CONTEXT) — daemon-tap PID cross-talk, token-counting source.

### Tertiary (LOW confidence)
- A1: claude CLI flag surface — assumed from `buildArgv`, not confirmed against the installed CLI.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new packages; all deps verified in use.
- Architecture/attach points: HIGH — every component read at the source-line level this session.
- Pitfalls: HIGH — derived directly from the verified code paths.
- Fairness effective-config scope (Open Q1): MEDIUM — the gap is verified HIGH; the resolution depends on a scope decision + the A1 CLI-flag assumption.

**Research date:** 2026-06-19
**Valid until:** 2026-07-19 (stable internal substrate; revalidate if Phases 78-79 artifacts move).

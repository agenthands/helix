---
phase: 67
plan: 01
type: tdd
wave: 0
depends_on: []
autonomous: true
requirements: [EVAL-02, EVAL-03, EVAL-04, EVAL-05, EVAL-06]
files_modified:
  - internal/profile/profiles/baseline.yaml
  - internal/profile/baseline_test.go
  - internal/profile/loader.go
  - eval/EVAL.md
  - eval/corpus/.gitkeep
  - eval/gen/.gitkeep
  - eval/fixtures/.gitkeep
  - eval/reports/.gitkeep
  - eval/.gitignore
  - internal/eval/runner/doc.go
  - internal/eval/sandbox/doc.go
  - internal/eval/agent/doc.go
  - internal/eval/trace/doc.go
  - internal/eval/score/doc.go
  - internal/eval/judge/doc.go
  - internal/eval/report/doc.go
  - internal/eval/budget/types.go
  - internal/eval/budget/types_test.go
  - internal/eval/pipeline.go
  - cmd/helix-eval/main.go
  - Makefile
tags: [eval, scaffolding, profile, baseline]

must_haves:
  truths:
    - "internal/profile/profiles/baseline.yaml is loadable by the existing profile loader"
    - "Loading baseline + applying ProfileFilterMiddleware to tools/list returns ZERO Helix tools (Assumption A1 verified)"
    - "eval/ top-level tree (corpus/ gen/ fixtures/ reports/) exists and is git-tracked"
    - "internal/eval/* subpackages exist as importable Go packages with stub interfaces only"
    - "internal/eval/budget/types.go declares BreachReason struct + String() method (Wave-0 cross-plan type, consumed by Plans 02 and 03)"
    - "cmd/helix-eval builds (`go build ./cmd/helix-eval`)"
    - "make eval-quick and make eval targets exist and exit non-zero with NotImplemented (until later waves)"
    - "eval/EVAL.md contains the Provider Retention Attestation block with retrieval date 2026-05-10"
  artifacts:
    - path: "internal/profile/profiles/baseline.yaml"
      provides: "Sixth profile that strips Helix tools entirely"
      contains: "name: baseline"
    - path: "internal/profile/baseline_test.go"
      provides: "Assumption A1 assertion: empty skills+tools = zero exposed tools"
    - path: "eval/EVAL.md"
      provides: "TOS attestation per EVAL-06"
      contains: "Provider Retention Attestation"
    - path: "internal/eval/budget/types.go"
      provides: "Wave-0 cross-plan BreachReason type contract — consumed by Plan 02 watchdog AND Plan 03 trace merger"
      exports: ["BreachReason", "(BreachReason).String"]
    - path: "cmd/helix-eval/main.go"
      provides: "cobra entrypoint binary"
    - path: "Makefile"
      provides: "eval and eval-quick targets"
  key_links:
    - from: "internal/profile/loader.go"
      to: "internal/profile/profiles/baseline.yaml"
      via: "embed.FS"
      pattern: "baseline\\.yaml"
    - from: "Makefile"
      to: "cmd/helix-eval/main.go"
      via: "go run ./cmd/helix-eval"
      pattern: "eval-quick:"
---

<objective>
Wave 0 scaffolding. Land the `baseline` profile (Assumption A1 verified by RED→GREEN test BEFORE the harness depends on it), the `eval/` top-level tree, the `internal/eval/*` package skeletons, the `cmd/helix-eval` cobra entrypoint, the Makefile targets, and the `eval/EVAL.md` TOS attestation.

Purpose: Phase 67 D-04 says corpus is per-task-dir; D-06 says heuristic scoring; EVAL-06 says synthetic-only with ZDR attestation. This plan creates the scaffolding ALL later plans depend on. Crucially, it proves Assumption A1 — "empty `skills: []` + `tools: []` produces zero exposed tools" — before the runner relies on it.

Output: New profile YAML + assertion test, `eval/` skeleton, `internal/eval/*` package skeletons, cmd binary, Makefile glue, `EVAL.md` attestation.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/ROADMAP.md
@.planning/STATE.md
@.planning/phases/67-evaluation-harness/67-CONTEXT.md
@.planning/phases/67-evaluation-harness/67-RESEARCH.md
@.planning/phases/67-evaluation-harness/67-VALIDATION.md

@internal/profile/loader.go
@internal/profile/profiles/full.yaml
@internal/profile/profiles/ci-bot.yaml
@internal/mcp/middleware.go

<interfaces>
<!-- Existing profile loader contract (extracted from internal/profile/loader.go) -->

The loader uses `embed.FS` to read all `profiles/*.yaml` at startup. To add `baseline.yaml`, the only requirement is dropping the file into `internal/profile/profiles/`; `embed.FS` glob discovers it automatically.

ProfileFilterMiddleware on `tools/list` consults the active profile's allow-list; an empty `skills: []` plus `tools: []` is the mechanism this plan asserts produces a zero-tool result.

If the assertion test FAILS (A1 wrong), the plan must extend `internal/profile/profile.go` with a `disable_all_tools: true` flag and patch the middleware. Default expectation per RESEARCH §"Baseline Profile": empty lists work as-is.
</interfaces>
</context>

<tasks>

<task type="auto" tdd="true">
  <name>Task 1: Author baseline.yaml + RED→GREEN assumption A1 test</name>
  <files>
    internal/profile/profiles/baseline.yaml,
    internal/profile/baseline_test.go,
    internal/profile/loader.go (only if A1 fails)
  </files>
  <behavior>
    - Test 1 (RED first): TestBaselineProfileLoads — load profile registry; assert "baseline" is present.
    - Test 2 (RED first): TestBaselineExposesZeroHelixTools — load baseline profile; run the same `tools/list` filter ProfileFilterMiddleware applies; assert resulting tool count == 0.
    - Test 3: TestBaselineGuardrailsOff — assert baseline.GuardrailsConfig.Enforcement == "off".
    - Test 4: TestBaselineDescriptionMentionsEval — substring assert on description so reviewers cannot accidentally repurpose this profile.
  </behavior>
  <action>
    Write `internal/profile/profiles/baseline.yaml` per RESEARCH §"Baseline Profile" verbatim (D-02 implementation). Use `name: baseline`, empty `skills: []`, empty `tools: []`, empty `exclude_tools: []`, `default_mode: edit`, `single_project: false`, `guardrails.enforcement: off`, the four `allowed_mode_transitions` entries.

    Write `internal/profile/baseline_test.go` with the four tests above. Use the same loader entrypoint the existing 5 profiles' tests use (mirror pattern from `loader.go` callers — the existing test files in this package show the idiom).

    RUN the tests. If TestBaselineExposesZeroHelixTools FAILS, Assumption A1 was wrong:
      - Add `disable_all_tools: true` field to ProfileSpec in `internal/profile/profile.go`.
      - Patch `internal/mcp/middleware.go` ProfileFilterMiddleware to short-circuit `tools/list` to `[]` when `disable_all_tools` is set.
      - Set `disable_all_tools: true` on baseline.yaml.
      - Re-run tests until GREEN.

    Document the chosen mechanism (empty-lists vs. disable-flag) in the commit message and in a comment at the top of baseline.yaml so later phases know what the profile contract is.
  </action>
  <verify>
    <automated>go test ./internal/profile -run "TestBaseline" -count=1 -race</automated>
  </verify>
  <done>
    All four baseline tests pass. `go vet ./internal/profile/...` clean. Commit message records whether A1 held or required the disable_all_tools fallback.
  </done>
</task>

<task type="auto">
  <name>Task 2: eval/ top-level tree + EVAL.md TOS attestation + .gitignore</name>
  <files>
    eval/EVAL.md,
    eval/corpus/.gitkeep,
    eval/gen/.gitkeep,
    eval/fixtures/.gitkeep,
    eval/reports/.gitkeep,
    eval/.gitignore
  </files>
  <action>
    Create `eval/` top-level tree per RESEARCH §"File-System Layout":
    - `eval/corpus/` (per-task dirs go here — empty in this wave; populated in Wave 2)
    - `eval/gen/` (programmatic generators — empty)
    - `eval/fixtures/` (in-process eval-quick seeds — empty)
    - `eval/reports/` (run-id subdirs go here — empty)

    `eval/.gitignore` must ignore `reports/<run-id>/` while keeping `reports/.gitkeep`. Concretely:
    ```
    /reports/*
    !/reports/.gitkeep
    ```

    `eval/EVAL.md` must include verbatim:
    1. Phase-67 charter paragraph (one sentence: "Helix eval harness — synthetic corpus only by default; commercial-LLM API calls run with retention-zero where the provider supports it.").
    2. Provider Retention Attestation block from RESEARCH §"TOS Attestation Format" — copy the markdown block as-is, including all three Anthropic-doc citation links and the retrieval date `2026-05-10`. Include the "Future Providers (placeholder)" table.
    3. The eval-run policy bullets (synthetic default / `HELIX_EVAL_ZDR_VERIFIED=1` for non-OSS / runner refuses non-synthetic without env-var).
    4. The M5 caveat paragraph ("ZDR is account-level, NOT per-request. There is no `X-ZDR-Required: true` header.").
    5. A loud `eval-quick is harness-validation-only` note (Pitfall 6 mitigation): explicit text "eval-quick uses a scripted agent and does NOT measure real Claude Code agent behavior. Use `make eval` for behavior."
    6. Operator checklist for ZDR (re-verify quarterly, signed DPA, env-var is human-attestation only — Pitfall 7 mitigation).
  </action>
  <verify>
    <automated>test -f eval/EVAL.md && grep -q "Provider Retention Attestation" eval/EVAL.md && grep -q "HELIX_EVAL_ZDR_VERIFIED" eval/EVAL.md && grep -q "INFORMATIONAL" eval/EVAL.md && test -d eval/corpus && test -d eval/gen && test -d eval/fixtures && test -d eval/reports</automated>
  </verify>
  <done>
    `eval/` tree created; `EVAL.md` covers all six attestation sections; `.gitignore` ignores generated reports while keeping the directory tracked.
  </done>
</task>

<task type="auto" tdd="true">
  <name>Task 3: internal/eval/* package skeletons + cmd/helix-eval cobra entrypoint + Makefile targets</name>
  <files>
    internal/eval/runner/doc.go,
    internal/eval/sandbox/doc.go,
    internal/eval/agent/doc.go,
    internal/eval/trace/doc.go,
    internal/eval/score/doc.go,
    internal/eval/judge/doc.go,
    internal/eval/report/doc.go,
    internal/eval/budget/types.go,
    internal/eval/budget/types_test.go,
    internal/eval/pipeline.go,
    cmd/helix-eval/main.go,
    cmd/helix-eval/main_test.go,
    Makefile
  </files>
  <behavior>
    - Test 1: TestHelixEvalCommandHelp — `helix-eval --help` exits 0 and mentions "eval" and "eval-quick" subcommands.
    - Test 2: TestHelixEvalRunNotImplemented — `helix-eval run --quick --corpus /tmp/empty` exits non-zero with "not yet implemented" until later waves wire it.
    - Test 3: TestPipelineExportsEvalPhases — `internal/eval/pipeline.go` exports a `Phases` symbol that equals `pipelines.EvalPhases` from `internal/phasegraph/pipelines/eval.go` (re-exported reference, not copied).
    - Test 4 (RED): TestBreachReasonShape — `budget.BreachReason` struct has fields `Axis string`, `Limit int64`, `Observed int64`; zero-value is valid; round-trips via JSON.
    - Test 5 (RED): TestBreachReasonString — `BreachReason{Axis:"seconds", Limit:300, Observed:312}.String()` returns exactly `"failed-with-cause: budget_seconds"` (D-08 wording verbatim).
  </behavior>
  <action>
    Each `internal/eval/<sub>/doc.go` is a one-paragraph `// Package <name> ...` doc comment matching the role described in RESEARCH §"File-System Layout". No code yet — just the package declaration and doc. This locks the import boundaries so later waves can't accidentally invent new package names.

    **EXCEPTION — `internal/eval/budget/`:** Instead of `doc.go`, ship `internal/eval/budget/types.go` in Wave 0 declaring the **`BreachReason` type** that downstream Wave-1 plans (Plan 02 watchdog, Plan 03 trace merger) consume. This is the cross-plan interface contract; it MUST land in Wave 0 so Plan 02 and Plan 03 can run as parallel-disjoint Wave-1 work without one importing from the other's behavior code. Concretely:
    ```go
    // Package budget declares the cross-cutting BreachReason type used by
    // the watchdog (Plan 02) and the trace merger (Plan 03). Behavior
    // (watchdog enforcement, YAML loading) lands in Plan 02's budget.go.
    package budget

    // BreachReason describes which D-08 axis tripped a budget breach.
    type BreachReason struct {
        Axis     string `json:"axis"`     // "input_tokens" | "output_tokens" | "seconds" | "tool_calls"
        Limit    int64  `json:"limit"`    // configured cap that was breached
        Observed int64  `json:"observed"` // measured value at breach
    }

    // String returns the D-08 outcome wording: "failed-with-cause: budget_<axis>".
    func (b BreachReason) String() string { return "failed-with-cause: budget_" + b.Axis }
    ```
    `internal/eval/budget/types_test.go` covers TestBreachReasonShape + TestBreachReasonString from the behavior block.

    `internal/eval/pipeline.go`: re-export the existing `pipelines.EvalPhases` from `internal/phasegraph/pipelines/eval.go` as `var Phases = pipelines.EvalPhases`. This is the seam later waves wrap with real `Run` bodies (per DAG-02 contract — fill noopRun, do not invent a parallel orchestrator).

    `cmd/helix-eval/main.go`: cobra `rootCmd` with two subcommands:
    - `helix-eval run` (flags: `--corpus PATH`, `--mode MODE` repeatable, `--run-id ID`, `--quick`, `--judge-model MODEL`, `--no-judge`, `--out PATH`). Body: returns "not yet implemented (Phase 67 Wave 1+)" sentinel until later waves wire it.
    - `helix-eval validate-rules` (flag: `--corpus PATH`). Body: returns "not yet implemented" sentinel until Wave 2 lands the DSL.
    Use `cobra` (already in repo per cmd/helix). The `--quick` flag selects the in-process path (Wave 4); without `--quick`, subprocess path is used.

    `cmd/helix-eval/main_test.go`: the three tests above. For TestHelixEvalCommandHelp use `cobra.Command.SetOut + Execute` rather than spawning a subprocess (faster, deterministic).

    `Makefile`: add two targets at the bottom of the file (do NOT modify existing targets):
    ```
    .PHONY: eval eval-quick

    # eval-quick: in-process scripted-agent harness validation. <30s wall.
    # Runs on every PR. NOT real-agent behavior measurement (see eval/EVAL.md).
    eval-quick:
    \tgo run ./cmd/helix-eval run --quick --corpus eval/fixtures --out eval/reports

    # eval: full out-of-process matrix. Local-only; nightly CI only.
    # NEVER add this to PR-gating workflows (project rule: benchmarks local-only).
    eval:
    \tgo run ./cmd/helix-eval run --corpus eval/corpus --mode baseline --mode native --mode semantic --mode semantic_guarded --out eval/reports
    ```
    The leading TAB is required for Make.
  </action>
  <verify>
    <automated>go build ./cmd/helix-eval ./internal/eval/... && go vet ./cmd/helix-eval ./internal/eval/... && go test ./cmd/helix-eval -run "TestHelixEval|TestPipelineExportsEvalPhases" -count=1 && go test ./internal/eval/budget -run "TestBreachReason" -count=1 -race && grep -E "^eval-quick:" Makefile && grep -E "^eval:" Makefile</automated>
  </verify>
  <done>
    Skeleton compiles; cobra binary builds; Makefile targets present; pipeline.go re-exports phasegraph EvalPhases without copying. All three tests green.
  </done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| Profile YAML → ProfileFilterMiddleware | A bad profile YAML could expose tools to a mode that should not have them; assertion test is the gate. |
| `eval/EVAL.md` content → operator behavior | Operator-checklist gate is human-trust-only; ZDR env-var is a circuit-breaker, not verification. |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-67-01 | T (Tampering) | `internal/profile/profiles/baseline.yaml` | mitigate | Assertion test in `internal/profile/baseline_test.go` runs in every PR; if a future change exposes any Helix tool through baseline, the test fails. Per A1 verification: if empty-lists semantics are not enough, fall back to a `disable_all_tools: true` flag and middleware short-circuit. |
| T-67-06 | I (Information Disclosure) | `eval/EVAL.md` ZDR attestation | mitigate | Operator-checklist documents quarterly re-verification cadence; runner gates non-synthetic corpora behind `HELIX_EVAL_ZDR_VERIFIED=1` (Wave 1 implements the gate; Wave 0 documents the gate's existence). Pitfall 7 explicitly accepted: env-var is human-trust, not verification. |

Block on: HIGH severity. T-67-01 is HIGH (could silently restore Helix tools to baseline → invalidates eval comparison). T-67-06 is MEDIUM (operator gate, not technical). All threats have mitigations in this plan.
</threat_model>

<verification>
- baseline.yaml is the sixth file in `internal/profile/profiles/` and `go test ./internal/profile -run TestBaseline` is green.
- The chosen A1-mechanism (empty-lists OR disable_all_tools) is documented in commit message + baseline.yaml header comment.
- `eval/` has corpus/, gen/, fixtures/, reports/ with `.gitkeep` files.
- `EVAL.md` has the six required sections.
- `make eval-quick` and `make eval` are present in Makefile, both produce a "not yet implemented" exit until Wave 1+.
- `go vet ./...` and `go test ./...` are clean (no regressions in existing packages).
</verification>

<success_criteria>
- [ ] `internal/profile/profiles/baseline.yaml` exists; `TestBaseline*` pass.
- [ ] Assumption A1 is verified (or reset via disable_all_tools mechanism, documented).
- [ ] `eval/` top-level tree exists; `EVAL.md` has all six sections; `.gitignore` ignores reports.
- [ ] `internal/eval/*` package skeletons compile.
- [ ] `cmd/helix-eval` builds; `--help` works; `run` returns sentinel.
- [ ] `Makefile` has `eval` and `eval-quick` targets.
- [ ] `internal/eval/pipeline.go` re-exports `pipelines.EvalPhases`.
- [ ] `go vet ./... && go test ./...` clean.
</success_criteria>

<dependencies>
- External: none beyond Go toolchain. `claude` CLI is NOT needed for this wave.
- Internal: depends on existing `internal/profile/loader.go`, `internal/mcp/middleware.go` (ProfileFilterMiddleware), `internal/phasegraph/pipelines/eval.go` (EvalPhases — already declared with noopRun).
- Plans: none (this is Wave 0).
</dependencies>

<output>
After completion, create `.planning/phases/67-evaluation-harness/67-01-SUMMARY.md` recording: which A1 mechanism was used (empty-lists vs disable_all_tools), the exact `baseline.yaml` contents committed, and any deviations from RESEARCH.
</output>

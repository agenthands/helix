---
phase: 67
plan: 06b
type: tdd
wave: 4
depends_on: [67-04, 67-06a]
autonomous: true
requirements: [EVAL-03, EVAL-05]
files_modified:
  - eval/fixtures/quick-public-api-001/task.md
  - eval/fixtures/quick-public-api-001/expected_tools.yaml
  - eval/fixtures/quick-public-api-001/budget.yaml
  - eval/fixtures/quick-public-api-001/scripted_agent.yaml
  - eval/fixtures/quick-public-api-001/verify.sh
  - eval/fixtures/quick-public-api-001/repo/main.go
  - eval/fixtures/quick-public-api-001/repo/go.mod
  - eval/fixtures/quick-large-edit-001/task.md
  - eval/fixtures/quick-large-edit-001/expected_tools.yaml
  - eval/fixtures/quick-large-edit-001/budget.yaml
  - eval/fixtures/quick-large-edit-001/scripted_agent.yaml
  - eval/fixtures/quick-large-edit-001/verify.sh
  - eval/fixtures/quick-large-edit-001/repo/main.go
  - eval/fixtures/quick-large-edit-001/repo/go.mod
  - eval/fixtures/quick-security-001/task.md
  - eval/fixtures/quick-security-001/expected_tools.yaml
  - eval/fixtures/quick-security-001/budget.yaml
  - eval/fixtures/quick-security-001/scripted_agent.yaml
  - eval/fixtures/quick-security-001/verify.sh
  - eval/fixtures/quick-security-001/repo/main.go
  - eval/fixtures/quick-security-001/repo/go.mod
  - eval/fixtures/quick-rename-ts-001/task.md
  - eval/fixtures/quick-rename-ts-001/expected_tools.yaml
  - eval/fixtures/quick-rename-ts-001/budget.yaml
  - eval/fixtures/quick-rename-ts-001/scripted_agent.yaml
  - eval/fixtures/quick-rename-ts-001/verify.sh
  - eval/fixtures/quick-rename-ts-001/repo/index.ts
  - eval/fixtures/quick-rename-ts-001/repo/package.json
  - eval/fixtures/quick-rename-ts-001/repo/tsconfig.json
  - eval/fixtures/quick-delete-ts-001/task.md
  - eval/fixtures/quick-delete-ts-001/expected_tools.yaml
  - eval/fixtures/quick-delete-ts-001/budget.yaml
  - eval/fixtures/quick-delete-ts-001/scripted_agent.yaml
  - eval/fixtures/quick-delete-ts-001/verify.sh
  - eval/fixtures/quick-delete-ts-001/repo/index.ts
  - eval/fixtures/quick-delete-ts-001/repo/package.json
  - eval/fixtures/quick-delete-ts-001/repo/tsconfig.json
  - eval/fixtures/quick-rename-py-001/task.md
  - eval/fixtures/quick-rename-py-001/expected_tools.yaml
  - eval/fixtures/quick-rename-py-001/budget.yaml
  - eval/fixtures/quick-rename-py-001/scripted_agent.yaml
  - eval/fixtures/quick-rename-py-001/verify.sh
  - eval/fixtures/quick-rename-py-001/repo/main.py
  - eval/fixtures/quick-public-api-py-001/task.md
  - eval/fixtures/quick-public-api-py-001/expected_tools.yaml
  - eval/fixtures/quick-public-api-py-001/budget.yaml
  - eval/fixtures/quick-public-api-py-001/scripted_agent.yaml
  - eval/fixtures/quick-public-api-py-001/verify.sh
  - eval/fixtures/quick-public-api-py-001/repo/main.py
  - internal/eval/runner/inprocess_fixtures_test.go
tags: [eval, in-process, eval-quick, fixtures, eval-05]

must_haves:
  truths:
    - "eval/fixtures/ contains 10 quick fixtures total (2 from 06a + 8 from this plan)"
    - "All 5 EVAL-05 families are represented: rename, delete, public_api, large_edit, security"
    - "Coverage spans Go (first-class), TypeScript (first-class), Python (best-effort)"
    - "Combined make eval-quick run completes in <30s wall (D-05 budget) across the 10-fixture set × 4 modes"
    - "Each fixture vet/typecheck/lint-clean for its language"
    - "Each fixture's expected_tools.yaml maps to the family's heuristic rule(s) from Plan 04"
  artifacts:
    - path: "eval/fixtures/quick-public-api-001"
      provides: "Go public-API edit fixture (analyze_blast_radius before edit)"
    - path: "eval/fixtures/quick-large-edit-001"
      provides: "Go large-edit fixture (replace_symbol_body / fuzzy_edit)"
    - path: "eval/fixtures/quick-security-001"
      provides: "Go security fixture (find_taint_sinks pattern OR safe_delete on auth-bearing symbol)"
    - path: "eval/fixtures/quick-rename-ts-001 + quick-delete-ts-001"
      provides: "TypeScript coverage for rename + delete families"
    - path: "eval/fixtures/quick-rename-py-001 + quick-public-api-py-001"
      provides: "Python coverage for rename + public-API families"
  key_links:
    - from: "eval/fixtures/quick-*/expected_tools.yaml"
      to: "internal/eval/score/rules.go (Plan 04)"
      via: "expect_sequence rules that the scorer evaluates"
      pattern: "expect_sequence|expect_set|expect_receipts"
    - from: "internal/eval/runner/inprocess_fixtures_test.go"
      to: "internal/eval/runner/inprocess.go (Plan 06a)"
      via: "RunQuick invoked over the full 10-fixture set"
      pattern: "RunQuick"
---

<objective>
Wave 4 (part 2 of 2): expand the eval-quick fixture set from 2 (Plan 06a reference fixtures) to **10 total** fixtures so D-05 — "make eval-quick: ~10 in-process tasks, target <30s wall-time, in CI on every PR" — is delivered as locked.

D-05 is LOCKED in CONTEXT.md. EVAL-05 is LOCKED in REQUIREMENTS.md and explicitly enumerates the rename / delete / public-API / large-edit / security families. The 2-fixture set in 06a only exercises 2 of 5 families and only Go; this plan completes the breadth.

**Coverage matrix (8 new fixtures):**

| Family       | Go                          | TypeScript                  | Python                          |
|--------------|-----------------------------|-----------------------------|---------------------------------|
| rename       | (06a: quick-rename-001)     | quick-rename-ts-001         | quick-rename-py-001             |
| delete       | (06a: quick-delete-001)     | quick-delete-ts-001         | —                               |
| public_api   | quick-public-api-001        | —                           | quick-public-api-py-001         |
| large_edit   | quick-large-edit-001        | —                           | —                               |
| security     | quick-security-001          | —                           | —                               |

Total = 2 (06a) + 8 (this plan) = **10**, satisfying D-05's "~10 in-process tasks".

**Constraint:** Combined 10-fixture run must still complete <30s wall across 4 modes. Plan 06a's `TestRunQuickWallTimeUnder30Seconds` set headroom on the 2-fixture set (target <10s); this plan's job is to keep each new fixture small (single-file repos where possible) so the 10-fixture run stays under 30s.

Output: 8 new quick fixtures + a full-matrix RunQuick test that asserts 10 fixtures × 4 modes <30s.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/phases/67-evaluation-harness/67-CONTEXT.md
@.planning/phases/67-evaluation-harness/67-RESEARCH.md
@.planning/phases/67-evaluation-harness/67-06a-eval-quick-inprocess-PLAN.md
@.planning/phases/67-evaluation-harness/67-04-scorer-dsl-and-seed-corpus-PLAN.md

<interfaces>
This plan adds DATA only (fixtures) plus one integration test. The fixture format and scripted-agent contract are already defined by Plan 06a:

- `task.md` — prompt + intent
- `repo/` — single-file or minimal-file project for the fixture's language
- `verify.sh` — exit 0 = pass; assertions on diff/grep/vet/tsc/python -m py_compile
- `expected_tools.yaml` — heuristic patterns scored by `internal/eval/score` (Plan 04)
- `budget.yaml` — per-task overrides; default to D-08 caps if absent
- `scripted_agent.yaml` — the hard-coded MCP tool-call sequence ScriptedAgent dispatches (Plan 06a)

The full 10-fixture set is read by `RunQuick(opts.FixturesDir = "eval/fixtures")` from Plan 06a; no code changes are needed in `internal/eval/runner/inprocess.go`. This plan only adds fixture data + one whole-matrix wall-time integration test.
</interfaces>
</context>

<tasks>

<task type="auto">
  <name>Task 1: 5 Go + TS + Python fixtures covering remaining EVAL-05 families</name>
  <files>
    eval/fixtures/quick-public-api-001/...,
    eval/fixtures/quick-large-edit-001/...,
    eval/fixtures/quick-security-001/...,
    eval/fixtures/quick-rename-ts-001/...,
    eval/fixtures/quick-delete-ts-001/...,
    eval/fixtures/quick-rename-py-001/...,
    eval/fixtures/quick-public-api-py-001/...
  </files>
  <action>
    Author 7 fixtures (the 8th is the security fixture in Task 2 — separated for clarity since security has a tighter scoping conversation). Each fixture follows the Plan 06a fixture format. **Keep repos minimal — ideally single-file** — so each fixture's daemon-side warm-up + scripted-agent dispatch stays under 2-3s wall.

    **quick-public-api-001 (Go, public_api family):**
    - repo/go.mod: `module example.com/quick-public-api-001` + `go 1.22`
    - repo/main.go: a public type `Greeter` with method `Hello() string` consumed by `main()`. Task: change return type from `string` to `(string, error)`.
    - scripted_agent.yaml: `[{tool: analyze_blast_radius, args: {symbol: Greeter.Hello}}, {tool: replace_symbol_body, args: {...}}]`
    - expected_tools.yaml: `expect_sequence: [{id: blast-radius-before-public-edit, score: 1, pattern: [{tool: analyze_blast_radius}, {tool: replace_symbol_body}]}]`
    - verify.sh: `set -e; cd repo; grep -q "Hello() (string, error)" main.go; go vet ./...; go build ./...`

    **quick-large-edit-001 (Go, large_edit family):**
    - Single-file repo with a 30-line function. Task: rewrite the function body using fuzzy_edit / replace_symbol_body.
    - scripted_agent.yaml: `[{tool: replace_symbol_body, args: {symbol: Process, body: <new body>}}]` OR `[{tool: fuzzy_edit, args: {...}}]`.
    - expected_tools.yaml: `+1 replace_symbol_body for body-rewrite`, `-1 replace_in_file with whole-function regex`.
    - verify.sh: vet-clean + grep for new behavior.

    **quick-rename-ts-001 (TypeScript, rename family):**
    - repo/package.json: `{"name":"quick-rename-ts-001","private":true,"devDependencies":{"typescript":"5.x"}}` (NO npm install needed; tsc fallback to globally installed if available; verify.sh skips tsc gracefully if not on PATH).
    - repo/tsconfig.json: minimal `{"compilerOptions":{"target":"ES2022","strict":true,"noEmit":true}}`
    - repo/index.ts: `export class Foo { hello(): string { return "hi" } }; const f = new Foo(); f.hello();`
    - Task + scripted_agent: rename `Foo` → `Bar` via `find_references` then `rename_symbol`.
    - verify.sh: `set -e; cd repo; grep -q "class Bar" index.ts; ! grep -q "class Foo" index.ts; if command -v tsc >/dev/null; then tsc --noEmit; fi`

    **quick-delete-ts-001 (TypeScript, delete family):**
    - repo with one unused exported function `legacyHelper` and one in-use function `currentHelper`. Task: safe-delete `legacyHelper`.
    - scripted_agent: `[{tool: find_references, args: {symbol: legacyHelper}}, {tool: safe_delete_symbol, args: {symbol: legacyHelper}}]`
    - expected_tools.yaml: `+1 safe_delete_symbol with receipts after find_references` (Plan 04 starter rule).

    **quick-rename-py-001 (Python, rename family, best-effort tier):**
    - repo/main.py only (no requirements.txt; stdlib only). Class `Foo` with method `hello`; rename to `Bar`.
    - scripted_agent: `[{tool: find_references, args: {symbol: Foo}}, {tool: rename_symbol, args: {old_name: Foo, new_name: Bar}}]`
    - verify.sh: `set -e; cd repo; grep -q "class Bar" main.py; ! grep -q "class Foo" main.py; python3 -m py_compile main.py`

    **quick-public-api-py-001 (Python, public_api family):**
    - repo/main.py with public function `compute(x: int) -> int` consumed by an `if __name__ == "__main__":` block. Task: change return type to `tuple[int, str]`.
    - scripted_agent: `[{tool: analyze_blast_radius, args: {symbol: compute}}, {tool: replace_symbol_body, args: {...}}]`
    - verify.sh: `set -e; cd repo; python3 -m py_compile main.py; grep -q "Tuple\[int, str\]\|tuple\[int, str\]" main.py`

    For each fixture: `chmod +x verify.sh`. Each `budget.yaml` sets `max_seconds: 5` (tight per-task budget so 10 × 5s × 4 modes is bounded; per-mode daemon reuse from 06a keeps wall-time well under 30s).

    Each `expected_tools.yaml` MUST be valid against the Plan 04 DSL (KnownFields strict) — `go run ./cmd/helix-eval validate-rules --corpus eval/fixtures` must pass after this task.
  </action>
  <verify>
    <automated>find eval/fixtures/quick-public-api-001 eval/fixtures/quick-large-edit-001 eval/fixtures/quick-rename-ts-001 eval/fixtures/quick-delete-ts-001 eval/fixtures/quick-rename-py-001 eval/fixtures/quick-public-api-py-001 -name verify.sh -type f -exec test -x {} \; && go run ./cmd/helix-eval validate-rules --corpus eval/fixtures && cd eval/fixtures/quick-public-api-001/repo && go vet ./... && cd - >/dev/null && cd eval/fixtures/quick-large-edit-001/repo && go vet ./... && cd - >/dev/null && python3 -m py_compile eval/fixtures/quick-rename-py-001/repo/main.py && python3 -m py_compile eval/fixtures/quick-public-api-py-001/repo/main.py</automated>
  </verify>
  <done>
    7 fixtures created (5 in this task + the 2nd Go-rename/delete pair from 06a still exists); validate-rules clean; Go fixtures vet-clean; Python fixtures py_compile-clean; verify.sh files executable.
  </done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: quick-security-001 fixture + 10-fixture wall-time integration test</name>
  <files>
    eval/fixtures/quick-security-001/task.md,
    eval/fixtures/quick-security-001/expected_tools.yaml,
    eval/fixtures/quick-security-001/budget.yaml,
    eval/fixtures/quick-security-001/scripted_agent.yaml,
    eval/fixtures/quick-security-001/verify.sh,
    eval/fixtures/quick-security-001/repo/main.go,
    eval/fixtures/quick-security-001/repo/go.mod,
    internal/eval/runner/inprocess_fixtures_test.go
  </files>
  <behavior>
    - Test 1 (RED): TestRunQuickFullFixtureSetWallTime — RunQuick over the 10-fixture set × 4 modes completes in <30s wall (D-05 budget). Use `testing.Short()` to skip on under-resourced runners; assert with `time.Since(start) < 30*time.Second` plus a 10% slack.
    - Test 2 (RED): TestRunQuickFullFixtureSetCoversAllFamilies — assert that across the 10 fixtures, each of the 5 EVAL-05 families (rename, delete, public_api, large_edit, security) has at least one fixture; assert language coverage includes Go, TypeScript, and Python.
    - Test 3 (RED): TestQuickSecurityFixtureLoadable — fixture loads via score.LoadRules + LoadScript without error.
  </behavior>
  <action>
    **quick-security-001 (Go, security family):**
    - repo/go.mod + minimal repo/main.go containing a function that constructs a shell command from user input via `exec.Command("sh", "-c", userInput)`. Task: replace with safer `exec.Command("name", arg1, arg2)` form (no shell interpolation).
    - scripted_agent.yaml: `[{tool: find_references, args: {symbol: handleRequest}}, {tool: replace_symbol_body, args: {symbol: handleRequest, body: <safe form>}}]`. (We do NOT depend on `find_taint_sinks` MCP tool here because Helix-Go does not ship a security capability today per project memory; the heuristic rule scores on "edit performed via replace_symbol_body after find_references on the affected symbol", not on a security-tool call.)
    - expected_tools.yaml: `expect_sequence: [{id: security-edit-with-references, score: 1, pattern: [{tool: find_references}, {tool: replace_symbol_body}]}]` PLUS `expect_set: [{id: avoid-shell-grep, anti: true, score: -1, tools: [replace_in_file]}]`.
    - verify.sh: `set -e; cd repo; ! grep -q '"sh", "-c"' main.go; go vet ./...; go build ./...`

    **`internal/eval/runner/inprocess_fixtures_test.go`:**
    - TestRunQuickFullFixtureSetWallTime: discovers all `eval/fixtures/quick-*` dirs, calls `RunQuick(ctx, QuickOpts{FixturesDir:"eval/fixtures", Modes:[]string{"baseline","native","semantic","semantic_guarded"}, ...})`, asserts wall-time <30s. The test uses `testing.Short()` skip-shortcut and a `t.Parallel()`-disabled top-level so the timing measurement is meaningful.
    - TestRunQuickFullFixtureSetCoversAllFamilies: walks `eval/fixtures/quick-*/expected_tools.yaml`, parses `task_kind` field, asserts the set of seen task_kinds == {"rename","delete","public_api","large_edit","security"}; walks repos and checks file extensions to assert {".go", ".ts", ".py"} all appear at least once.
    - TestQuickSecurityFixtureLoadable: mirrors the Plan 06a pattern.

    NOTE: This test depends on Plan 06a's RunQuick + 06a's 2 reference fixtures + this plan's 8 new fixtures. It is the GATE that proves D-05 is satisfied.
  </action>
  <verify>
    <automated>cd eval/fixtures/quick-security-001/repo && go vet ./... && cd - >/dev/null && go run ./cmd/helix-eval validate-rules --corpus eval/fixtures && go test ./internal/eval/runner -run "TestRunQuickFullFixtureSet|TestQuickSecurityFixtureLoadable" -count=1 -race -timeout 60s</automated>
  </verify>
  <done>
    Security fixture vet-clean and DSL-valid; full 10-fixture wall-time test green under 30s on CI runner; all 5 EVAL-05 families covered; Go/TS/Python coverage asserted by test.
  </done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| Fixture YAML/scripts → ScriptedAgent dispatch | Trusted (in-repo, PR-reviewed); no LLM in the loop. |
| Fixture coverage assertion → CI gate | Trusted; the integration test is the contract that D-05 ~10-task breadth holds. |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-67-Pitfall-6 | (Misadvertised coverage) | additional fixtures could re-suggest eval-quick measures real-agent behavior | mitigate | Inherits 06a's four-layer Pitfall-6 defense (banner, gate, EVAL.md, Makefile comment); this plan adds NO new agent-behavior claim — every fixture is scripted. |
| T-67-Pitfall-7 | (Budget overrun) | 10-fixture × 4-mode run could exceed D-05's 30s budget | mitigate | TestRunQuickFullFixtureSetWallTime asserts <30s with 10% slack; if it red-lights, the fix is fixture-shrink + per-mode daemon reuse from 06a — NOT D-05 scope reduction. |
| T-67-Pitfall-8 | (Coverage drift) | future fixture edits could silently drop a family | mitigate | TestRunQuickFullFixtureSetCoversAllFamilies asserts all 5 EVAL-05 families AND Go/TS/Python coverage are always present; deletion of any family fails the test. |

Block on: HIGH severity. T-67-Pitfall-7 (budget) and T-67-Pitfall-8 (coverage drift) are HIGH because either failure invalidates D-05/EVAL-05 — both have asserting tests in this plan.
</threat_model>

<verification>
- 8 new fixtures exist; total `find eval/fixtures -type d -name 'quick-*'` count == 10.
- Every fixture's `expected_tools.yaml` validates under Plan 04 DSL (`validate-rules` clean).
- Go fixtures vet-clean; Python fixtures py_compile-clean.
- TestRunQuickFullFixtureSetWallTime green: 10 × 4 modes < 30s.
- TestRunQuickFullFixtureSetCoversAllFamilies green: all 5 EVAL-05 families + Go/TS/Python.
- `make eval-quick` end-to-end runs in <30s on CI with the 10-fixture set.
</verification>

<success_criteria>
- [ ] 8 new fixtures: quick-public-api-001, quick-large-edit-001, quick-security-001, quick-rename-ts-001, quick-delete-ts-001, quick-rename-py-001, quick-public-api-py-001 + 1 reserve under "Task 2 security" — adjust if family already covered.
- [ ] All 5 EVAL-05 families have ≥1 fixture in the eval-quick set.
- [ ] Go, TypeScript, Python all represented.
- [ ] Wall-time test green: 10 fixtures × 4 modes < 30s.
- [ ] `validate-rules --corpus eval/fixtures` clean.
</success_criteria>

<dependencies>
- Plans: 67-04 (DSL + scorer + validate-rules subcommand), 67-06a (RunQuick + ScriptedAgent + LoadScript + 2 reference fixtures).
- External: `python3` on PATH for py_compile checks; TypeScript fixtures gracefully skip tsc if `tsc` not on PATH.
</dependencies>

<output>
After completion, create `.planning/phases/67-evaluation-harness/67-06b-SUMMARY.md` recording: per-fixture wall-time breakdown, total 10-fixture × 4-mode run wall-time on CI, any per-mode daemon-reuse tweaks needed to fit the budget.
</output>

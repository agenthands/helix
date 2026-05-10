---
phase: 67
plan: 04
type: tdd
wave: 2
depends_on: [67-01, 67-03]
autonomous: true
requirements: [EVAL-01, EVAL-04, EVAL-05]
files_modified:
  - internal/eval/score/rules.go
  - internal/eval/score/rules_test.go
  - internal/eval/score/score.go
  - internal/eval/score/score_test.go
  - internal/eval/score/starter_rules_test.go
  - eval/corpus/go-rename-public-001/task.md
  - eval/corpus/go-rename-public-001/expected_tools.yaml
  - eval/corpus/go-rename-public-001/budget.yaml
  - eval/corpus/go-rename-public-001/verify.sh
  - eval/corpus/go-rename-public-001/repo/main.go
  - eval/corpus/go-rename-public-001/repo/go.mod
  - eval/corpus/go-delete-symbol-001/task.md
  - eval/corpus/go-delete-symbol-001/expected_tools.yaml
  - eval/corpus/go-delete-symbol-001/budget.yaml
  - eval/corpus/go-delete-symbol-001/verify.sh
  - eval/corpus/go-delete-symbol-001/repo/main.go
  - eval/corpus/go-delete-symbol-001/repo/go.mod
  - eval/corpus/go-public-api-001/task.md
  - eval/corpus/go-public-api-001/expected_tools.yaml
  - eval/corpus/go-public-api-001/budget.yaml
  - eval/corpus/go-public-api-001/verify.sh
  - eval/corpus/go-public-api-001/repo/main.go
  - eval/corpus/go-public-api-001/repo/go.mod
  - eval/corpus/ts-rename-001/task.md
  - eval/corpus/ts-rename-001/expected_tools.yaml
  - eval/corpus/ts-rename-001/budget.yaml
  - eval/corpus/ts-rename-001/verify.sh
  - eval/corpus/ts-rename-001/repo/index.ts
  - eval/corpus/ts-rename-001/repo/package.json
  - eval/corpus/ts-delete-001/task.md
  - eval/corpus/ts-delete-001/expected_tools.yaml
  - eval/corpus/ts-delete-001/budget.yaml
  - eval/corpus/ts-delete-001/verify.sh
  - eval/corpus/ts-delete-001/repo/index.ts
  - eval/corpus/ts-delete-001/repo/package.json
  - eval/corpus/py-rename-001/task.md
  - eval/corpus/py-rename-001/expected_tools.yaml
  - eval/corpus/py-rename-001/budget.yaml
  - eval/corpus/py-rename-001/verify.sh
  - eval/corpus/py-rename-001/repo/main.py
  - eval/corpus/py-public-api-001/task.md
  - eval/corpus/py-public-api-001/expected_tools.yaml
  - eval/corpus/py-public-api-001/budget.yaml
  - eval/corpus/py-public-api-001/verify.sh
  - eval/corpus/py-public-api-001/repo/main.py
  - eval/corpus/go-large-edit-001/task.md
  - eval/corpus/go-large-edit-001/expected_tools.yaml
  - eval/corpus/go-large-edit-001/budget.yaml
  - eval/corpus/go-large-edit-001/verify.sh
  - eval/corpus/go-large-edit-001/repo/main.go
  - eval/corpus/go-large-edit-001/repo/go.mod
  - eval/corpus/go-security-001/task.md
  - eval/corpus/go-security-001/expected_tools.yaml
  - eval/corpus/go-security-001/budget.yaml
  - eval/corpus/go-security-001/verify.sh
  - eval/corpus/go-security-001/repo/main.go
  - eval/corpus/go-security-001/repo/go.mod
  - eval/corpus/ts-public-api-001/task.md
  - eval/corpus/ts-public-api-001/expected_tools.yaml
  - eval/corpus/ts-public-api-001/budget.yaml
  - eval/corpus/ts-public-api-001/verify.sh
  - eval/corpus/ts-public-api-001/repo/index.ts
  - eval/corpus/ts-public-api-001/repo/package.json
tags: [eval, scorer, corpus, heuristic, dsl]

must_haves:
  truths:
    - "expected_tools.yaml DSL parses with KnownFields(true); typos fail loudly"
    - "Heuristic scorer applies +1/-1 deltas per RESEARCH §'Heuristic Rule DSL' semantics"
    - "Rename family rules score +1 for find_references→rename_symbol; -1 for grep+replace_in_file"
    - "Delete family rules score +1 for safe_delete_symbol with non-empty receipts; -1 for delete_file without prior find_references"
    - "Public-API family rules score +1 for analyze_blast_radius before public-symbol edit; -1 for replace_symbol_body on exported symbol without blast-radius"
    - "10 hand-authored seed tasks cover Go + TypeScript + Python; rename + delete + public-API + large-edit + security families"
    - "helix-eval validate-rules walks every task in eval/corpus/ and fails on any malformed expected_tools.yaml"
  artifacts:
    - path: "internal/eval/score/rules.go"
      provides: "DSL parser + matcher (sequence, set, receipts)"
      exports: ["Rules", "LoadRules", "Score", "Apply", "ExpectSequence", "ForbidSequence", "ExpectSet", "ForbidSet", "ReceiptRule"]
    - path: "eval/corpus/"
      provides: "10 seed tasks across rename / delete / public-API / large-edit / security families"
  key_links:
    - from: "internal/eval/score/score.go"
      to: "internal/eval/trace/schema.go"
      via: "MergedTrace consumption"
      pattern: "trace\\.MergedTrace|trace\\.Event"
    - from: "internal/eval/score/rules.go"
      to: "yaml.v3 KnownFields(true)"
      via: "strict YAML parsing"
      pattern: "KnownFields"
---

<objective>
Wave 2: heuristic scorer + DSL parser + 10 seed tasks. Implements EVAL-05 (rename/delete/public-API right-tool scoring +1/-1) on top of the merged trace schema from Plan 03. Hand-authored seed corpus covers Go + TypeScript + Python.

Purpose: EVAL-05 is the keystone behavioral signal: did the agent use the canonical Helix tool, or did it fall back to grep + manual edit? Heuristic scoring is CI-actionable per D-06; LLM judge (Plan 06) is informational only.

Output: `internal/eval/score` with DSL+scorer; 10 task directories under `eval/corpus/` shaped per D-04; CLI subcommand `helix-eval validate-rules` lights up.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/phases/67-evaluation-harness/67-CONTEXT.md
@.planning/phases/67-evaluation-harness/67-RESEARCH.md
@internal/eval/trace/schema.go

<interfaces>
<!-- DSL shape from RESEARCH §"Heuristic Rule DSL" verbatim -->
expected_tools.yaml:
- task_kind: rename | delete | public_api | large_edit | security (informational)
- expect_sequence: [{id, score:+N, pattern:[{tool, args_match{key:val|key_regex:val}}]}]
- expect_set: [{id, score:+N, tools:[...]}]
- forbid_sequence: [...] (negative on match)
- forbid_set: [{id, score:-N, when{tool_used}, require_prior{any_of:[...]}}]
- receipts: [{id, score:+N, when{tool_used}, require{receipts_non_empty}}]

<!-- MergedTrace consumption surface (from Plan 03) -->
trace.MergedTrace.Events: []Event with Tool, Outcome, ArgsSummary, ToolUseID, etc.
Iterating events filtered by Source=="daemon" && Kind==KindToolCall yields the tool-call timeline; each carries Tool, Outcome, ArgsSummary (a flattened JSON arg summary).
</interfaces>
</context>

<tasks>

<task type="auto" tdd="true">
  <name>Task 1: Rule DSL parser with strict KnownFields + schema validation</name>
  <files>
    internal/eval/score/rules.go,
    internal/eval/score/rules_test.go
  </files>
  <behavior>
    - Test 1 (RED): TestLoadRulesAllConstructs — parse a fixture yaml containing every construct (expect_sequence, expect_set, forbid_sequence, forbid_set, receipts) and assert all fields are populated correctly.
    - Test 2 (RED): TestLoadRulesUnknownKeyFails — yaml with `expectedSequence` (typo) returns a non-nil error mentioning the unknown key (KnownFields(true)).
    - Test 3 (RED): TestLoadRulesMissingRequired — a rule missing `id` returns an error referencing the path.
    - Test 4 (RED): TestLoadRulesArgsMatchRegex — `args_match.symbol_regex: "^[A-Z][a-zA-Z0-9_]+$"` parses into a compiled regex (parse error if regex is invalid).
    - Test 5 (RED): TestLoadRulesNegativeScoreNormalized — `forbid_sequence` rule with `score: -1` round-trips identically; the matcher still applies the same negative delta.
    - Test 6 (RED): TestValidateRulesCommand — `helix-eval validate-rules --corpus eval/corpus` walks all task dirs, parses every expected_tools.yaml, returns 0 if all parse and a non-zero exit code listing the offending file otherwise. (This wires the cobra subcommand stub from Plan 01.)
  </behavior>
  <action>
    Implement `internal/eval/score/rules.go`:
    ```go
    type Rules struct {
        TaskKind string `yaml:"task_kind"`
        ExpectSequence []SequenceRule `yaml:"expect_sequence"`
        ForbidSequence []SequenceRule `yaml:"forbid_sequence"`
        ExpectSet []SetRule `yaml:"expect_set"`
        ForbidSet []ForbidSetRule `yaml:"forbid_set"`
        Receipts []ReceiptRule `yaml:"receipts"`
    }
    type SequenceRule struct{ ID string; Score int; Pattern []PatternStep }
    type PatternStep struct{ Tool string; ArgsMatch map[string]string; ArgsMatchRegex map[string]*regexp.Regexp }
    type SetRule struct{ ID string; Score int; Tools []string }
    type ForbidSetRule struct{ ID string; Score int; When struct{ ToolUsed string }; RequirePrior struct{ AnyOf []string } }
    type ReceiptRule struct{ ID string; Score int; When struct{ ToolUsed string }; Require struct{ ReceiptsNonEmpty bool } }
    ```
    `LoadRules(path string) (Rules, error)` uses `yaml.NewDecoder(f).KnownFields(true).Decode(&r)`. After decode:
    - Each rule's `id` MUST be non-empty.
    - For SequenceRule, every PatternStep `tool` MUST be non-empty.
    - For ForbidSetRule, `When.ToolUsed` MUST be non-empty.
    - For ReceiptRule similarly.
    - For `args_match` keys ending in `_regex`, compile into `ArgsMatchRegex`; on bad regex, return error.

    Wire `helix-eval validate-rules` (stub from Plan 01): walk `<corpusDir>/*/expected_tools.yaml`, call `LoadRules`, accumulate errors; print and exit 1 if any error.
  </action>
  <verify>
    <automated>go test ./internal/eval/score -run "TestLoadRules|TestValidateRulesCommand" -count=1 -race</automated>
  </verify>
  <done>
    Six DSL-parser tests pass; `helix-eval validate-rules` runs end-to-end on a fixture corpus directory; race-clean.
  </done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: Scorer — apply Rules over a MergedTrace; emit Score with id-keyed deltas</name>
  <files>
    internal/eval/score/score.go,
    internal/eval/score/score_test.go,
    internal/eval/score/starter_rules_test.go
  </files>
  <behavior>
    - Test 1 (RED): TestScoreExpectSequenceMatches — given a trace with daemon events [find_references, rename_symbol] and rule `expect_sequence: rename-after-references` → score includes `{rename-after-references: +1}`.
    - Test 2 (RED): TestScoreExpectSequenceWithIntervening — same, but trace has [find_references, get_definition, rename_symbol] → still matches (intervening events allowed per RESEARCH §"Matcher Semantics").
    - Test 3 (RED): TestScoreExpectSequenceArgsMatch — pattern requires `args_match.symbol: "AuthMiddleware"`; only matches when args_summary contains "AuthMiddleware" substring.
    - Test 4 (RED): TestScoreForbidSequenceMatches — trace [search_for_pattern, replace_in_file] + forbid rule rename-by-grep → score includes `{rename-by-grep: -1}`.
    - Test 5 (RED): TestScoreForbidSetRequirePriorMissing — trace [delete_file] (no prior find_references) + ForbidSet `delete-without-references` → `{delete-without-references: -1}`.
    - Test 6 (RED): TestScoreForbidSetRequirePriorPresent — trace [find_references, delete_file] + same rule → no negative score.
    - Test 7 (RED): TestScoreReceiptsRule — daemon tool_call event for `safe_delete_symbol` whose args_summary contains `receipts:["abc","def"]` matches `receipts_non_empty: true` and scores +1.
    - Test 8 (RED): TestScoreExpectSetTools — set rule `[rename_symbol, verify_edit]` matches when both tools fire (any order).
    - Test 9 (starter rules — table-driven): TestStarterRulesRename — load `eval/corpus/go-rename-public-001/expected_tools.yaml` against a synthetic trace; assert expected score deltas.
    - Test 10 (starter rules): TestStarterRulesDelete — same for go-delete-symbol-001.
    - Test 11 (starter rules): TestStarterRulesPublicAPI — same for go-public-api-001.
  </behavior>
  <action>
    `internal/eval/score/score.go`:
    ```go
    type Delta struct{ RuleID string; Score int }
    type Score struct{ Total int; Deltas []Delta }
    func Apply(t trace.MergedTrace, r Rules) Score
    ```
    Implementation:
    1. Build `toolCallEvents := []Event` filtering Source=="daemon" && Kind==KindToolCall.
    2. For each ExpectSequence rule, run `matchSubsequence(toolCallEvents, rule.Pattern)`; on hit, append +Score delta.
    3. For each ForbidSequence rule, same; on hit, append the (already-negative) Score delta.
    4. For each ExpectSet rule, check every tool in `Tools` appears at least once.
    5. For each ForbidSetRule, find first event with Tool==When.ToolUsed; check no prior event has Tool ∈ RequirePrior.AnyOf; if so, append negative delta.
    6. For each ReceiptRule, find first event with Tool==When.ToolUsed; if Require.ReceiptsNonEmpty, parse `receipts: [...]` from `args_summary` (substring `receipts:[` followed by at least one quoted ID); on match, append positive delta.

    `matchSubsequence`: greedy left-to-right; for each PatternStep, scan forward in toolCallEvents until a matching event is found; if exhausted, return false. ArgsMatch checks substring on `args_summary`; ArgsMatchRegex applies the precompiled regex.

    `Score.Total` = sum of all `Delta.Score`.

    Starter rule tests: each loads a YAML file from `eval/corpus/<task>/expected_tools.yaml` and a synthetic trace (constructed in-test from Event literals — no I/O). Assert specific deltas. These tests are how Wave 2 proves the DSL semantics match the rule examples in CONTEXT.md D-06.
  </action>
  <verify>
    <automated>go test ./internal/eval/score -count=1 -race</automated>
  </verify>
  <done>
    Eleven scoring tests pass (8 DSL + 3 starter-rule). Race-clean. `Apply` is deterministic — same trace + same rules → byte-identical Score.
  </done>
</task>

<task type="auto">
  <name>Task 3: 10 hand-authored seed tasks (Go + TypeScript + Python; rename / delete / public-API / large-edit / security)</name>
  <files>
    eval/corpus/go-rename-public-001/{task.md,expected_tools.yaml,budget.yaml,verify.sh,repo/main.go,repo/go.mod},
    eval/corpus/go-delete-symbol-001/{task.md,expected_tools.yaml,budget.yaml,verify.sh,repo/main.go,repo/go.mod},
    eval/corpus/go-public-api-001/{task.md,expected_tools.yaml,budget.yaml,verify.sh,repo/main.go,repo/go.mod},
    eval/corpus/go-large-edit-001/{task.md,expected_tools.yaml,budget.yaml,verify.sh,repo/main.go,repo/go.mod},
    eval/corpus/go-security-001/{task.md,expected_tools.yaml,budget.yaml,verify.sh,repo/main.go,repo/go.mod},
    eval/corpus/ts-rename-001/{task.md,expected_tools.yaml,budget.yaml,verify.sh,repo/index.ts,repo/package.json},
    eval/corpus/ts-delete-001/{task.md,expected_tools.yaml,budget.yaml,verify.sh,repo/index.ts,repo/package.json},
    eval/corpus/ts-public-api-001/{task.md,expected_tools.yaml,budget.yaml,verify.sh,repo/index.ts,repo/package.json},
    eval/corpus/py-rename-001/{task.md,expected_tools.yaml,budget.yaml,verify.sh,repo/main.py},
    eval/corpus/py-public-api-001/{task.md,expected_tools.yaml,budget.yaml,verify.sh,repo/main.py}
  </files>
  <action>
    Author 10 task directories per D-04 layout. Coverage matrix:

    | task-id | Lang | Family | Family Rule Coverage |
    |---------|------|--------|----------------------|
    | go-rename-public-001 | Go | rename | +rename-after-references; -rename-by-grep |
    | go-delete-symbol-001 | Go | delete | +safe-delete-with-receipts; -delete-without-references |
    | go-public-api-001 | Go | public_api | +blast-radius-before-public-edit; -replace-symbol-body-without-blast-radius |
    | go-large-edit-001 | Go | large_edit | +get_context-before-replace_symbol_body; -replace_in_file-on-large-region |
    | go-security-001 | Go | security | +verify_edit-after-security-edit; -no-diagnostics-after-edit |
    | ts-rename-001 | TS | rename | (same shape as go-rename-public-001) |
    | ts-delete-001 | TS | delete | (same shape) |
    | ts-public-api-001 | TS | public_api | (same shape) |
    | py-rename-001 | Python | rename | (same shape) |
    | py-public-api-001 | Python | public_api | (same shape) |

    For EACH task:
    1. **task.md** — 1-3 sentences describing the user-visible intent. Example for go-rename-public-001: "Rename the public function `AuthMiddleware` to `AuthGuard` across the package. Update all callers. Tests must continue to pass."
    2. **repo/** — minimal compileable fixture. For Go: `go.mod` with `module example.com/<task-id>` and Go 1.22; one `main.go` with the symbol to rename/delete/etc. For TS: `package.json` with `{"type":"module"}` (no deps) and one `index.ts`. For Python: one `main.py` (no requirements file).
    3. **verify.sh** — bash script (chmod +x; assert via test in Task 4 below). Exit 0 = pass.
       - Go: `cd repo && go vet ./... && go test ./... && grep -q "AuthGuard" main.go && ! grep -q "AuthMiddleware" main.go`
       - TS: `cd repo && node --experimental-strip-types index.ts && grep ...`
       - Python: `cd repo && python3 main.py && grep ...`
       Each verify.sh asserts the renamed/deleted/etc. state AND that no leftover references to the original symbol remain.
    4. **expected_tools.yaml** — rules per RESEARCH §"Heuristic Rule DSL" + §"Starter Rule Coverage (EVAL-05)". Mandatory entries:
       - rename-family: `expect_sequence` (find_references → rename_symbol); `forbid_sequence` (search_for_pattern → replace_in_file).
       - delete-family: `receipts` (safe_delete_symbol with receipts_non_empty); `forbid_set` (delete_file requires prior find_references OR analyze_blast_radius).
       - public_api: `expect_sequence` (analyze_blast_radius → replace_symbol_body); `forbid_set` (replace_symbol_body on exported requires prior analyze_blast_radius).
       - large_edit: `expect_sequence` (get_context|get_repo_map → replace_symbol_body); `forbid_sequence` (replace_in_file with very long find_regex).
       - security: `expect_sequence` (rename_symbol|replace_symbol_body → verify_edit); `expect_set` (get_diagnostics post-edit).
    5. **budget.yaml** — D-08 four-axis caps. Use defaults for most; override `max_tool_calls: 50` for narrow tasks like rename, `max_seconds: 600` for large_edit. Always declare all four keys for clarity.

    Make every verify.sh executable in the commit. Repos must be compilable so the agent has something real to manipulate.

    Constraint: every fixture is SYNTHETIC (no copy from real-world repos), satisfies EVAL-06.
  </action>
  <verify>
    <automated>find eval/corpus -name verify.sh -type f -exec test -x {} \; && go run ./cmd/helix-eval validate-rules --corpus eval/corpus && cd eval/corpus/go-rename-public-001/repo && go vet ./... && cd - >/dev/null && cd eval/corpus/go-delete-symbol-001/repo && go vet ./... && cd - >/dev/null && cd eval/corpus/go-public-api-001/repo && go vet ./... && cd - >/dev/null && cd eval/corpus/go-large-edit-001/repo && go vet ./... && cd - >/dev/null && cd eval/corpus/go-security-001/repo && go vet ./...</automated>
  </verify>
  <done>
    Ten task dirs created; every expected_tools.yaml parses; every Go fixture vets clean; every verify.sh is executable; matrix covers Go+TS+Python and rename/delete/public-API/large-edit/security.
  </done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| Task author → eval runner | Semi-trusted: corpus tasks ship in-repo and are PR-reviewed, but `verify.sh` runs as the eval user. |
| `expected_tools.yaml` author → scorer | Trusted (in-repo); typos caught by `validate-rules` CI gate. |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-67-01 | E (Elevation of Privilege) | malicious task fixture executes `verify.sh` with eval-user privileges | mitigate | All corpus content is in-repo and PR-reviewed. CI does NOT pull external corpora (per EVAL-06 synthetic-only). `verify.sh` runs in per-task `repo/` tmpdir (Plan 02 sandbox); no host-env passthrough beyond the documented allowlist. ZDR-gate (Plan 06) refuses non-synthetic corpora without `HELIX_EVAL_ZDR_VERIFIED=1`. |
| T-67-Pitfall-6 | I (Info Disclosure) | scoring DSL allows wide regex which could DoS scorer with catastrophic backtracking | mitigate | Compile each `_regex` at LoadRules time so a bad pattern is rejected at corpus-author time, not at eval-run time. Reject patterns with `(?:` followed by `*`/`+` if simple lint detects exponential shape (defer to Go's regexp/syntax — RE2 has no catastrophic backtracking, so this risk is structurally bounded). |

Block on: HIGH severity. T-67-01 is HIGH; mitigated via PR-review + sandbox cwd + env-allowlist (Plan 02) + ZDR-gate (Plan 06). T-67-Pitfall-6 is LOW (Go's RE2 engine has no exponential blowup).
</threat_model>

<verification>
- DSL parses with KnownFields(true); typos fail loudly (TestLoadRulesUnknownKeyFails).
- Scorer is deterministic (Apply returns identical Score for identical input).
- 10 seed tasks; all 5 EVAL-05 families covered; all 3 languages covered.
- Every Go fixture passes `go vet`.
- `helix-eval validate-rules eval/corpus` exits 0.
</verification>

<success_criteria>
- [ ] `internal/eval/score/rules.go` + tests green.
- [ ] `internal/eval/score/score.go` + tests green (8 + 3 starter).
- [ ] 10 task directories with all 5 files each (task.md, expected_tools.yaml, budget.yaml, verify.sh+x, repo/).
- [ ] Coverage matrix verified: Go+TS+Python × rename/delete/public-API/large-edit/security.
- [ ] `helix-eval validate-rules` exits 0 on the corpus.
- [ ] `go vet ./internal/eval/...` clean; `go test ./internal/eval/score -count=1 -race` green.
</success_criteria>

<dependencies>
- Plans: 67-01 (corpus tree, helix-eval cobra stub), 67-03 (trace.MergedTrace + Event types).
- External: none.
</dependencies>

<output>
After completion, create `.planning/phases/67-evaluation-harness/67-04-SUMMARY.md` recording: the final 10 task IDs, the coverage matrix actually shipped, any DSL extensions added beyond the RESEARCH spec.
</output>

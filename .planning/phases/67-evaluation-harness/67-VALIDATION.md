---
phase: 67
slug: evaluation-harness
status: planned
nyquist_compliant: true
wave_0_complete: false
created: 2026-05-10
---

# Phase 67 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Two test surfaces apply: (a) **harness self-tests** validate the runner, scorer, reporters, and DSL parser; (b) **eval runs** validate the SUT (helix daemon) against the corpus.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | `go test` (stdlib) for harness self-tests; `make eval-quick` for in-process SUT smoke; `make eval` for full subprocess matrix |
| **Config file** | none — Go stdlib testing; eval modes configured via in-memory overlays (Plan 06) and per-mode `helix_config.yml` (Plan 05) |
| **Quick run command** | `go test ./eval/... ./internal/eval/... ./cmd/helix-eval/...` |
| **Full suite command** | `go vet ./... && go test ./... && make eval-quick` |
| **Estimated runtime** | harness self-tests ~30s; `eval-quick` <30s wall; `make eval` ~30 min total (4 modes × 50–100 tasks) |

Notes:
- `make eval` is **never** a Wave-0..Wave-N gate inside CI for PRs; nightly only (per memory: benchmarks/local-only). PR gate is `eval-quick` only. CI grep gate (Plan 07) fails the build if any workflow file references `tool_behavior_judge`.
- Harness self-tests cover: subprocess sandbox isolation, trace merger event ordering + pid-gating + path-prefix invariant, heuristic DSL parser/matcher, baseline-profile zero-tools assertion (Assumption A1), reporter JSON schemas, judge exit-code isolation.

---

## Sampling Rate

- **After every task commit:** `go test ./eval/... ./internal/eval/... ./cmd/helix-eval/...` (harness self-tests touched by the commit)
- **After every plan wave:** `go vet ./... && go test ./...`
- **After Wave 1 (sandbox+agent+trace):** `make eval-quick` must pass on a 1-task fixture (no claude CLI required)
- **After Wave 4 (in-process eval-quick):** `make eval-quick` covers all 4 modes against the 10-task fixture set (Plan 06a's 2 reference fixtures + Plan 06b's 8 expansion fixtures spanning all 5 EVAL-05 families × Go/TS/Python)
- **Before `/gsd-verify-work`:** Full suite green + `make eval-quick` green; one local `make eval` run recorded in `eval/reports/<run-id>/` (NOT a CI artifact per memory: benchmarks local-only)
- **Max feedback latency:** 30 seconds for harness self-tests; 30 seconds for eval-quick

---

## Per-Task Verification Map

> Each plan task is mirrored here with its automated command and threat ref.
> Threat refs ("T-67-NN") tie back to the PLAN.md `<threat_model>` block in the named plan.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 67-01-T1 | 01 | 0 | EVAL-02 | T-67-01 | Baseline profile exposes zero Helix tools (Assumption A1) | unit | `go test ./internal/profile -run "TestBaseline" -count=1 -race` | ❌ W0 | ⬜ pending |
| 67-01-T2 | 01 | 0 | EVAL-06 | T-67-06 | EVAL.md attestation block exists with retrieval date + ZDR env-var contract | check | `test -f eval/EVAL.md && grep -q "Provider Retention Attestation" eval/EVAL.md && grep -q "HELIX_EVAL_ZDR_VERIFIED" eval/EVAL.md && grep -q "INFORMATIONAL" eval/EVAL.md` | ❌ W0 | ⬜ pending |
| 67-01-T3 | 01 | 0 | EVAL-03/04 | — | helix-eval binary builds; Makefile targets exist; pipeline.go re-exports phasegraph EvalPhases; budget.BreachReason Wave-0 type contract declared (consumed by Plans 02 and 03) | integration | `go build ./cmd/helix-eval ./internal/eval/... && go vet ./cmd/helix-eval ./internal/eval/... && go test ./cmd/helix-eval -run "TestHelixEval\|TestPipelineExportsEvalPhases" -count=1 && go test ./internal/eval/budget -run "TestBreachReason" -count=1 -race && grep -E "^eval-quick:" Makefile && grep -E "^eval:" Makefile` | ❌ W0 | ⬜ pending |
| 67-02-T1 | 02 | 1 | EVAL-02 | T-67-01, T-67-04 | Per-(task,mode) sandbox isolates HOME/socket/repo; refuses symlinked tmpdir | integration | `go test ./internal/eval/sandbox -run TestSandbox -count=1 -race` | ❌ W0 | ⬜ pending |
| 67-02-T2 | 02 | 1 | EVAL-02 | T-67-Pitfall-1 | claude CLI invoked with `--bare --strict-mcp-config`; env scrubbed to allowlist | integration | `go test ./internal/eval/agent -count=1 -race` | ❌ W0 | ⬜ pending |
| 67-02-T3 | 02 | 1 | EVAL-01 | — | Budget watchdog enforces D-08 four-axis caps; breach surfaces `failed-with-cause: budget_<axis>` | unit | `go test ./internal/eval/budget -count=1 -race` | ❌ W0 | ⬜ pending |
| 67-03-T1 | 03 | 1 | EVAL-01 | — | MergedTrace schema matches RESEARCH §"Merged trace.json Shape" exactly; outcome enum closed | unit | `go test ./internal/eval/trace -run "TestEvent|TestMergedTrace|TestOutcome|TestEventKindClosed" -count=1 -race` | ❌ W0 | ⬜ pending |
| 67-03-T2 | 03 | 1 | EVAL-01/02 | T-67-04 | Daemon JSONL pid-gate rejects foreign-pid events; CC stream-json parsed into typed events | unit | `go test ./internal/eval/trace -run "TestTap" -count=1 -race` | ❌ W0 | ⬜ pending |
| 67-03-T3 | 03 | 1 | EVAL-01 | T-67-02 | Wall-clock merge with daemon-wins-on-tie; path-prefix invariant rejects patches outside repo | unit | `go test ./internal/eval/trace -run "TestMerge" -count=1 -race` | ❌ W0 | ⬜ pending |
| 67-04-T1 | 04 | 2 | EVAL-05 | T-67-Pitfall-6 | DSL parses with KnownFields(true); typos fail loudly | unit | `go test ./internal/eval/score -run "TestLoadRules|TestValidateRulesCommand" -count=1 -race` | ❌ W0 | ⬜ pending |
| 67-04-T2 | 04 | 2 | EVAL-05 | — | Heuristic scorer applies +1/-1 deltas per RESEARCH §"Matcher Semantics" | unit | `go test ./internal/eval/score -count=1 -race` | ❌ W0 | ⬜ pending |
| 67-04-T3 | 04 | 2 | EVAL-04/05 | T-67-01 | 10 seed tasks cover Go+TS+Python × rename/delete/public-API/large-edit/security; all Go fixtures vet-clean | integration | `find eval/corpus -name verify.sh -type f -exec test -x {} \; && go run ./cmd/helix-eval validate-rules --corpus eval/corpus && cd eval/corpus/go-rename-public-001/repo && go vet ./... && cd - >/dev/null && cd eval/corpus/go-delete-symbol-001/repo && go vet ./... && cd - >/dev/null && cd eval/corpus/go-public-api-001/repo && go vet ./... && cd - >/dev/null && cd eval/corpus/go-large-edit-001/repo && go vet ./... && cd - >/dev/null && cd eval/corpus/go-security-001/repo && go vet ./...` | ❌ W0 | ⬜ pending |
| 67-05-T1 | 05 | 3 | EVAL-01/06 | T-67-06 | ZDR gate refuses external corpora without `HELIX_EVAL_ZDR_VERIFIED=1`; EvalResult JSON shape matches EVAL-01 | unit | `go test ./internal/eval/runner -run "TestZDRGate" -count=1 -race && go test ./internal/eval/report -run "TestEvalResult|TestWriteResult" -count=1 -race` | ❌ W0 | ⬜ pending |
| 67-05-T2 | 05 | 3 | EVAL-01/02 | T-67-03 | Runner fills phasegraph EvalPhases Run bodies (DAG-02); per-(task,mode) artifacts produced | integration | `go test ./internal/eval/runner -count=1 -race && go vet ./internal/eval/runner ./internal/eval/pipeline` | ❌ W0 | ⬜ pending |
| 67-05-T3 | 05 | 3 | EVAL-04 | T-67-Pitfall-1 | All 5 EVAL-04 reports + run_metadata.json emitted; env KEYS captured (never values) | unit | `go test ./internal/eval/report ./cmd/helix-eval -count=1 -race && go vet ./internal/eval/report ./cmd/helix-eval` | ❌ W0 | ⬜ pending |
| 67-06a-T1 | 06a | 4 | EVAL-03 | — | Scripted-agent dispatch + 2 reference quick fixtures (rename, delete) vet-clean | integration | `go test ./internal/eval/runner -run "TestLoadScript\|TestScriptedAgent\|TestQuickRenameFixtureLoadable\|TestQuickDeleteFixtureLoadable" -count=1 -race && cd eval/fixtures/quick-rename-001/repo && go vet ./... && cd - >/dev/null && cd eval/fixtures/quick-delete-001/repo && go vet ./...` | ❌ W0 | ⬜ pending |
| 67-06a-T2 | 06a | 4 | EVAL-03 | T-67-Pitfall-6 | RunQuick boots in-process daemon over bufconn; harness-validation banner emitted; success-flag gated; daemon reused across fixtures within a mode; <30s wall on 2-fixture set with headroom for 06b expansion | integration | `go test ./internal/eval/runner -run "TestRunQuick" -count=1 -race && go vet ./internal/eval/runner ./cmd/helix-eval && time make eval-quick` | ❌ W0 | ⬜ pending |
| 67-06b-T1 | 06b | 4 | EVAL-03/05 | — | 7 new fixtures (public-api Go, large-edit Go, rename TS, delete TS, rename Py, public-api Py) vet/typecheck/py_compile-clean; DSL-valid | integration | `find eval/fixtures/quick-public-api-001 eval/fixtures/quick-large-edit-001 eval/fixtures/quick-rename-ts-001 eval/fixtures/quick-delete-ts-001 eval/fixtures/quick-rename-py-001 eval/fixtures/quick-public-api-py-001 -name verify.sh -type f -exec test -x {} \; && go run ./cmd/helix-eval validate-rules --corpus eval/fixtures && cd eval/fixtures/quick-public-api-001/repo && go vet ./... && cd - >/dev/null && cd eval/fixtures/quick-large-edit-001/repo && go vet ./... && cd - >/dev/null && python3 -m py_compile eval/fixtures/quick-rename-py-001/repo/main.py && python3 -m py_compile eval/fixtures/quick-public-api-py-001/repo/main.py` | ❌ W0 | ⬜ pending |
| 67-06b-T2 | 06b | 4 | EVAL-03/05 | T-67-Pitfall-7, T-67-Pitfall-8 | quick-security-001 fixture vet/DSL-clean; full 10-fixture wall-time test green under 30s; all 5 EVAL-05 families covered (rename/delete/public_api/large_edit/security); Go+TS+Python coverage asserted | integration | `cd eval/fixtures/quick-security-001/repo && go vet ./... && cd - >/dev/null && go run ./cmd/helix-eval validate-rules --corpus eval/fixtures && go test ./internal/eval/runner -run "TestRunQuickFullFixtureSet\|TestQuickSecurityFixtureLoadable" -count=1 -race -timeout 60s` | ❌ W0 | ⬜ pending |
| 67-07-T1 | 07 | 5 | EVAL-07 | T-67-Pitfall-1 | Anthropic API client retries 429/5xx; never logs API key; rubric covers 5 EVAL-05 families | unit | `go test ./internal/eval/judge -run "TestClient|TestRubric" -count=1 -race && grep -q "rename" internal/eval/judge/prompts/rubric.md && grep -q "delete" internal/eval/judge/prompts/rubric.md && grep -q "public_api" internal/eval/judge/prompts/rubric.md && grep -q "large_edit" internal/eval/judge/prompts/rubric.md && grep -q "security" internal/eval/judge/prompts/rubric.md` | ❌ W0 | ⬜ pending |
| 67-07-T2 | 07 | 5 | EVAL-07 | T-67-05, T-67-Pitfall-8 | Judge sees trace only (no patch, no task body); errors swallowed in Run signature; exit code never affected | unit | `go test ./internal/eval/judge -count=1 -race && go test ./internal/eval/report -run "TestEvalReportMarkdownIncludesJudgeSection" -count=1 && go test ./cmd/helix-eval -run "TestRunMatrixExitCodeIgnoresJudge" -count=1` | ❌ W0 | ⬜ pending |
| 67-07-T3 | 07 | 5 | EVAL-07 | T-67-Pitfall-8 | CI runs eval-quick only; grep gate forbids judge ref in workflows; EVAL.md operator checklist | check | `grep -E "make eval-quick\|eval-quick:" .github/workflows/ci.yml && ! grep -E "^\s*run: make eval$" .github/workflows/ci.yml && grep -q "tool_behavior_judge" .github/workflows/ci.yml && grep -q "ZDR Operator Checklist" eval/EVAL.md && grep -q "INFORMATIONAL" eval/EVAL.md` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/profile/profiles/baseline.yaml` — exists + tools-stripped assertion test (Assumption A1) — Plan 01
- [ ] `eval/` skeleton (corpus/, gen/, fixtures/, reports/) — directory layout in place — Plan 01
- [ ] `internal/eval/` package skeleton — runner, sandbox, agent, trace, score, judge, report subpackages with stub doc.go files — Plan 01
- [ ] `internal/eval/budget/types.go` — Wave-0 BreachReason type contract (consumed by Plans 02 and 03 as parallel-disjoint Wave-1 work) — Plan 01
- [ ] `cmd/helix-eval/` — main.go skeleton + cobra commands stubbed — Plan 01
- [ ] `Makefile` — `eval` and `eval-quick` targets stubbed (calling `go run ./cmd/helix-eval`) — Plan 01
- [ ] `eval/EVAL.md` — TOS attestation skeleton (filled at planning time per EVAL-06) — Plan 01
- [ ] `internal/eval/sandbox/sandbox_test.go` — env isolation + UDS socket assertions — Plan 02
- [ ] `internal/eval/trace/merge_test.go` — merger event-ordering + pid-gate + path-prefix fixtures — Plan 03
- [ ] `internal/eval/score/rules_test.go` — DSL parser fixtures — Plan 04

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| ZDR / retention-zero attestation accuracy | EVAL-06 | Anthropic ZDR is account-level, not per-request — no API surface to assert against | Reviewer reads `eval/EVAL.md`, follows the linked Anthropic privacy doc, confirms each provider entry has a retrieval date and a verified setting; CI lint asserts only that `HELIX_EVAL_ZDR_VERIFIED=1` env var was set when commercial-provider modes ran |
| LLM judge prompt quality | EVAL-05, EVAL-07 | Judge output is informational; quality is a human-review concern | Reviewer reads 10 sample `tool_behavior_judge.json` outputs; rubric matches EVAL-05 task families |
| Cross-version CC drift | D-03 | Records-not-pins by design | When CI runs `make eval`, reviewer checks `eval_report.json` `cc_version` field is non-empty and consistent within the run |
| Real-agent behavior on `make eval` | EVAL-01..EVAL-05 | LLM-driven; non-deterministic; not CI-gated per project memory rule | Reviewer runs `make eval` locally pre-release; inspects `eval/reports/<run-id>/eval_report.md` mode-comparison table; archives report alongside release notes |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 30s for harness self-tests
- [x] `make eval` excluded from any PR-gating CI workflow (memory rule: benchmarks local-only)
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** pending plan checker

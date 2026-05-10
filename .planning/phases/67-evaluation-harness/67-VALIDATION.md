---
phase: 67
slug: evaluation-harness
status: draft
nyquist_compliant: false
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
| **Config file** | none — Go stdlib testing; eval modes configured via `eval/config/*.yaml` |
| **Quick run command** | `go test ./eval/... ./internal/eval/... ./cmd/helix-eval/...` |
| **Full suite command** | `go vet ./... && go test ./... && make eval-quick` |
| **Estimated runtime** | harness self-tests ~30s; `eval-quick` <30s wall; `make eval` ~30 min total (4 modes × 50–100 tasks) |

Notes:
- `make eval` is **never** a Wave-0..Wave-N gate inside CI for PRs; nightly only (per memory: benchmarks/local-only). PR gate is `eval-quick` only.
- Harness self-tests cover: subprocess sandbox isolation, trace merger event ordering, heuristic DSL parser/matcher, baseline-profile zero-tools assertion (Assumption A1), reporter JSON schemas.

---

## Sampling Rate

- **After every task commit:** `go test ./eval/... ./internal/eval/... ./cmd/helix-eval/...` (harness self-tests touched by the commit)
- **After every plan wave:** `go vet ./... && go test ./...`
- **After Wave 1 (sandbox+agent+trace):** `make eval-quick` must pass on a 1-task fixture
- **After Wave 4 (in-process eval-quick):** `make eval-quick` covers all 4 modes against a 1–2 task fixture
- **Before `/gsd-verify-work`:** Full suite green + `make eval-quick` green; one local `make eval` run recorded in `EVAL.md` artifacts (NOT a CI artifact per memory: benchmarks local-only)
- **Max feedback latency:** 30 seconds for harness self-tests; 30 seconds for eval-quick

---

## Per-Task Verification Map

> Filled in by planner. Each plan task gets a row with the automated command that proves the behavior.
> Threat refs ("T-67-NN") tie back to the PLAN.md `<threat_model>` block.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 67-NN-NN | NN | N | EVAL-XX | T-67-NN / — | {expected secure behavior or "N/A"} | unit / integration / eval-fixture | `{command}` | ✅ / ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/profile/profiles/baseline.yaml` — exists + tools-stripped assertion test (Assumption A1)
- [ ] `eval/` skeleton (corpus/, gen/, config/, judge/) — directory layout in place
- [ ] `internal/eval/` package skeleton — runner, sandbox, trace, scorer, judge, report subpackages with stub interfaces
- [ ] `cmd/helix-eval/` — main.go skeleton + cobra commands stubbed
- [ ] `Makefile` — `eval` and `eval-quick` targets stubbed (calling `go run ./cmd/helix-eval`)
- [ ] `eval/EVAL.md` — TOS attestation skeleton (filled at planning time per EVAL-06)
- [ ] `internal/eval/sandbox/sandbox_test.go` — env isolation + UDS socket assertions
- [ ] `internal/eval/trace/merge_test.go` — merger event-ordering fixtures
- [ ] `internal/eval/scorer/dsl_test.go` — DSL parser fixtures

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| ZDR / retention-zero attestation accuracy | EVAL-06 | Anthropic ZDR is account-level, not per-request — no API surface to assert against | Reviewer reads `eval/EVAL.md`, follows the linked Anthropic privacy doc, confirms each provider entry has a retrieval date and a verified setting; CI lint asserts only that `HELIX_EVAL_ZDR_VERIFIED=1` env var was set when commercial-provider modes ran |
| LLM judge prompt quality | EVAL-05, EVAL-07 | Judge output is informational; quality is a human-review concern | Reviewer reads 10 sample `tool_behavior_judge.json` outputs; rubric matches EVAL-05 task families |
| Cross-version CC drift | D-03 | Records-not-pins by design | When CI runs `make eval`, reviewer checks `eval_report.json` `cc_version` field is non-empty and consistent within the run |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s for harness self-tests
- [ ] `make eval` excluded from any PR-gating CI workflow (memory rule: benchmarks local-only)
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending

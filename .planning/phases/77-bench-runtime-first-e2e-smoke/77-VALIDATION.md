---
phase: 77
slug: bench-runtime-first-e2e-smoke
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-06-17
---

# Phase 77 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Derived from `77-RESEARCH.md` § Validation Architecture.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go standard `testing` + `github.com/stretchr/testify` (require/assert) + `jsonschema/v6` (schema validation) |
| **Config file** | none (`go test`) |
| **Quick run command** | `go test ./bench/... ./cmd/helix-bench/...` |
| **Full suite command** | `go test ./...` (then `go vet ./...` and `gofmt -w .` per CLAUDE.md) |
| **Estimated runtime** | quick ~10–30 s; `make bench-quick` smoke ≤ 90 s; single-task `helix-bench run` ≤ 30 s |

---

## Sampling Rate

- **After every task commit:** Run `go test ./bench/... ./cmd/helix-bench/... && go vet ./...`
- **After every plan wave:** Run `go test ./...`
- **Before `/gsd-verify-work`:** Full suite green + `make bench-quick` exits 0 ≤ 90 s + single-task smoke ≤ 30 s + `gofmt -w .` clean
- **Max feedback latency:** ~30 seconds (quick command)

### Sampling/observability boundary (the "Nyquist rate" of this smoke)

Asserting ONLY the verify exit code **aliases** real failures. The smoke MUST also assert all three,
or a green run can hide a broken tap, leaked spans, or an invalid result:

1. **2-leg merged trace / zero orphan spans** — `MergedTrace.ToolCallSummary.Total >= 1` AND the CC
   (agent-tap) leg is present (exit-code-only would pass even if the agent-tap leg silently dropped).
2. **Zero PID cross-talk** — `DaemonTapResult.RejectedForeignPid == 0` and no foreign tool names in
   the merged trace (exit-code-only would not detect a neighbor cell's leakage).
3. **result.v2 schema validity** — validate the emitted doc against `bench/schema/result.v2.schema.json`
   (exit-code-only would pass on a malformed/empty result file).

---

## Per-Task Verification Map

> Task IDs populate after planning (Step 8). Requirement→test-type mapping below is the contract the
> planner's `<acceptance_criteria>` and the nyquist-auditor must satisfy.

| Decision/Req | Behavior | Test Type | Automated Command | File (Wave 0) |
|--------------|----------|-----------|-------------------|---------------|
| D-05 | mode `your_agent_full` → profile `bench-full` via MODE.md resolver | unit | `go test ./bench/runners/ -run ModeResolver` | `bench/runners/mode_resolver_test.go` |
| D-04 | result.v2 builder emits schema-valid doc (outcome+fairness+tokens+trace-ref) | unit | `go test ./bench/runtime/ -run ResultV2Valid` | `bench/runtime/result_test.go` |
| D-08 | cell key/path `<run_id>/<task>/<mode>`; durable vs ephemeral split; preserve-on-failure | unit | `go test ./bench/runtime/ -run CellLayout` | `bench/runtime/cell_test.go` |
| D-02 | synthesize `CCTapResult` from `[]StepResult`; `Merge` yields 2-leg trace | unit | `go test ./bench/runtime/ -run SynthCCTap` | `bench/runtime/cell_test.go` |
| BENCH-04 | subprocess daemon spawn + forwarder drive + PID-gated tap → `MergedTrace` | integration | `go test ./bench/runtime/ -run DaemonTap` (mirror eval's; SKIP if no `helix` on PATH) | `bench/runtime/daemon_tap_integration_test.go` |
| METRIC-06 / criterion #4 | zero PID cross-talk on parallel cells: `RejectedForeignPid == 0` | integration | `go test ./bench/runtime/ -run CrossCell` | `bench/runtime/cross_cell_test.go` |
| D-03 | seed scripted task: real MCP edit makes `go test` pass (verify exit 0) | integration | `go test ./bench/datasets/... -run SeedTaskVerify` or via `helix-bench run` | `bench/datasets/toolbench-go/<task>/` |
| Criterion #1 | `helix-bench run --benchmarks=toolbench-go --modes=your_agent_full --tasks=<one>` ≤ 30 s, schema-valid | e2e/smoke | timed `helix-bench run …`; assert exit 0 + result.v2 validates | `cmd/helix-bench/run_cmd_test.go` |
| BENCH-05 / criterion #2 | `make bench-quick` exit 0, ≥ 1 task succeeds, ≤ 90 s | e2e/smoke | `time make bench-quick` (CI) | Makefile target |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `bench/runners/mode_resolver_test.go` — D-05 mode→profile resolver
- [ ] `bench/runtime/result_test.go` — D-04 result.v2 build + schema-validate
- [ ] `bench/runtime/cell_test.go` — D-08 cell layout + D-02 CCTap synth (unit)
- [ ] `bench/runtime/daemon_tap_integration_test.go` — BENCH-04 (mirror `internal/eval/runner/daemon_tap_integration_test.go`; SKIP if no `helix`)
- [ ] `bench/runtime/cross_cell_test.go` — METRIC-06 / criterion #4 parallel PID cross-talk
- [ ] `cmd/helix-bench/run_cmd_test.go` — flag plumbing + exit semantics (mirror `cmd/helix-eval/run_cmd_test.go`)
- [ ] Seed fixture `bench/datasets/toolbench-go/<task>/` with a verify-passing scripted edit
- [ ] Framework install: **none** — `testing`, `testify`, `jsonschema/v6` all present in `go.mod`

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Real `claude` CLI agent path produces the same `CCTapResult` shape | D-01 (wired, not gating) | Needs `claude` CLI + `$ANTHROPIC_API_KEY` + network; excluded from hermetic CI | `helix-bench run --benchmarks=toolbench-go --modes=your_agent_full --tasks=<one> --agent=claude` locally; confirm exit 0 + 2-leg trace |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending

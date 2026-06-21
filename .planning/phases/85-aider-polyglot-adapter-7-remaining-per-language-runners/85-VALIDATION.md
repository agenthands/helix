---
phase: 85
slug: aider-polyglot-adapter-7-remaining-per-language-runners
status: planned
nyquist_compliant: true
wave_0_complete: false
created: 2026-06-21
---

# Phase 85 — Validation Strategy

> Seeded from the "Validation Architecture" section of 85-RESEARCH.md. The planner fills the
> Per-Task Verification Map from the four success criteria. The governing rule (MEMORY
> helix-bench-smoke-false-green + Phase 81 vacuous-gate lesson): every per-language runner's
> parse/capability logic MUST have a HERMETIC golden-fixture test (no subprocess) as the SOLE
> authoritative proof; live toolchain runs and `--network=none` container runs SKIP cleanly when
> the toolchain/engine is absent. No live test may be the sole proof of any success criterion.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (golden reporter-output fixtures per language) |
| **Config file** | none — standard Go toolchain |
| **Quick run command** | `go vet ./bench/... && go test ./bench/languages/... ./bench/adapters/... ./bench/aggregator/... ./bench/runtime/...` |
| **Full suite command** | `go test ./...` (live per-language + container `--network=none` + pinned-sha clone tests SKIP unless toolchain/engine/network present) |
| **Estimated runtime** | ~60–150 seconds (hermetic); live layers gated/skipped |

---

## Sampling Rate

- **After every task commit:** Run the quick run command for touched packages
- **After every plan wave:** Run the full suite command
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** ~150 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| (planner fills from RESEARCH Validation Architecture) | | | ADAPTER-AIDER-01 / TOOLBENCH-03..09 | unit(golden) | | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] Each `bench/languages/<L>/runner.go` (Python/TS/JS/Java/C#/C++/Rust) has a HERMETIC parser test driven by a committed golden reporter fixture (pytest-json / vitest-json / jest-json / surefire-xml / TRX / ctest-junit / cargo libtest-text) — proves pass/fail row extraction with NO live toolchain
- [ ] Capability ≥N/10 gate test per runner (from task.json capability fields, via coverage.go)
- [ ] Additive `language` field on result.v2 (`result.go`) + schema; aggregator per-language slice test (golden)
- [ ] Aider Polyglot dataset-loader-only adapter: hermetic fixture-task-set test (parse + 2-attempt stderr-reprompt protocol) — live pinned-sha clone gated on network
- [ ] `--network=none` container run seam (`container.Run`) argv-asserted hermetically (live exec gated on engine)
- [ ] `make verify-licenses` hard-gate (clones verify_tos.go injected-`today` strict-decode pattern) + LICENSE-AUDIT.md per-track sha256

---

## Manual-Only / Toolchain-Gated Verifications

| Behavior | Requirement | Why Gated | Test Instructions |
|----------|-------------|-----------|-------------------|
| Live per-language test execution (pytest/vitest/jest/mvn/dotnet/ctest/cargo) | TOOLBENCH-03..09 | Needs the language toolchain (javac/mvn absent here; npx/dotnet/g++/cargo present); gated, SKIPs when absent | Run each runner against a real Exercism exercise on a host with the toolchain |
| Aider Polyglot full 225-task run + per-language sanity pass-rate | ADAPTER-AIDER-01 | Needs network clone + the pinned model + all 6 toolchains | Run the adapter end-to-end on a fully-provisioned host; compare per-language pass-rate to the committed baseline within tolerance |
| `--network=none` hermetic container task runs | TOOLBENCH (SC#3) | Needs a container engine + pre-baked toolchain images | Run a task through `container.Run(--network=none)` against a baked image |

*All runner parse/capability logic, the adapter protocol, the language-slice aggregation, and the license gate have hermetic automated verification via golden fixtures + injected seams.*

---

## Validation Sign-Off

- [ ] Every runner has a hermetic golden-fixture parser test (sole authoritative proof)
- [ ] No live/toolchain-gated test is the sole proof of a success criterion
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] No watch-mode flags
- [ ] Feedback latency < 150s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** planned

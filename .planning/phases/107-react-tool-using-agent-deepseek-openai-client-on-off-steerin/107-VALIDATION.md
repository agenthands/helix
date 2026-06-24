---
phase: 107
slug: react-tool-using-agent-deepseek-openai-client-on-off-steerin
status: approved
nyquist_compliant: true
wave_0_complete: false
created: 2026-06-24
---

# Phase 107 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | pytest (dev-time, in `tools/dspy-tune/` venv only — NOT `go test`) |
| **Config file** | none — flat, fixture-free convention matching existing `test_parity.py`/`test_degenerate.py`/`test_split.py` (no new pytest config, no `conftest.py`) |
| **Quick run command** | `cd tools/dspy-tune && python -m pytest test_agent.py -q` |
| **Full suite command** | `cd tools/dspy-tune && python -m pytest -q` + `make vet` + `git diff --exit-code go.mod` |
| **Estimated runtime** | ~5 seconds (hermetic, no network, keys unset) |

---

## Sampling Rate

- **After every task commit:** Run quick command (hermetic agent tests, no network/keys)
- **After every plan wave:** Run full suite (pytest + `make vet` + `git diff go.mod`)
- **Before `/gsd-verify-work`:** Full suite green AND boundary checks green
- **Max feedback latency:** ~5 seconds

---

## Per-Task Verification Map

All hermetic tests live in the single flat module `tools/dspy-tune/test_agent.py` (7 named test functions, fake LLM + fake `helix`, no network, keys unset).

| Task ID | Plan | Wave | Requirement | Named test(s) | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|---------------|-----------------|-----------|-------------------|-------------|--------|
| 107-01-01 (RED) | 01 | 1 | AGENT-01/02/03 | all 7 (authored RED) | anti-vacuity pair authored before code | unit | `python -m pytest test_agent.py -q` | ❌ W0 | ⬜ pending |
| 107-01-02 (GREEN llm/tools) | 01 | 1 | AGENT-02 | `test_missing_key_raises`, `test_model_config_var`, `test_provider_fallback` | fixed-argv `subprocess.run([...])`, no `shell=True`; loud-fail RAISE on missing key | unit | `python -m pytest test_agent.py -q` | ❌ W0 | ⬜ pending |
| 107-01-03 (GREEN react) | 01 | 1 | AGENT-01, AGENT-03 | `test_loop_emits_and_observes`, `test_no_progress_terminates`, `test_steering_on_off_omission`, `test_degenerate_always_grep_scores_zero` | bounded loop + deterministic termination reason; OFF provably omits sentinel; degenerate scores 0 | unit | `python -m pytest test_agent.py -q` | ❌ W0 | ⬜ pending |
| 107-01-* (boundary) | 01 | 1 | ADOPT-04 (cross) | — | no Go edge; quarantine boundary intact; no `shell=True` | boundary | `make vet && git diff --exit-code go.mod` | ✅ | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `tools/dspy-tune/test_agent.py` — flat hermetic fake-LLM + fake-`helix` harness with inline fakes (no `conftest.py`, no new pytest config; matches existing `test_parity.py`/`test_degenerate.py` convention)
- [ ] `openai==2.43.0` added to `tools/dspy-tune/requirements.txt` (dev venv only)

*The hermetic tests construct NO `openai` client (fakes are duck-typed/injected), so they run with `openai` absent and keys unset.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Live DeepSeek `deepseek-v4-flash` first-attempt tool-calling reliability | AGENT-02 | Requires a real API key + network; not hermetic | One live smoke call at implementation time; confirm fallback path to OpenAI on DeepSeek error |

*All non-live phase behaviors have automated hermetic verification.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 5s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending

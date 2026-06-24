---
phase: 107
slug: react-tool-using-agent-deepseek-openai-client-on-off-steerin
status: draft
nyquist_compliant: false
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
| **Config file** | `tools/dspy-tune/pyproject.toml` or `pytest.ini` (Wave 0 adds if absent) |
| **Quick run command** | `cd tools/dspy-tune && python -m pytest agent/test_agent.py -q` |
| **Full suite command** | `cd tools/dspy-tune && python -m pytest -q` + `make vet` + `git diff --exit-code go.mod` |
| **Estimated runtime** | ~5 seconds (hermetic, no network) |

---

## Sampling Rate

- **After every task commit:** Run quick command (hermetic agent tests, no network/keys)
- **After every plan wave:** Run full suite (pytest + `make vet` + `git diff go.mod`)
- **Before `/gsd-verify-work`:** Full suite green AND boundary checks green
- **Max feedback latency:** ~5 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 107-01-01 | 01 | 1 | AGENT-02 | — | fixed-argv `subprocess.run([...])`, no `shell=True`; loud fail on missing key | unit | `python -m pytest agent/test_llm.py -q` | ❌ W0 | ⬜ pending |
| 107-01-02 | 01 | 1 | AGENT-01 | — | bounded loop, deterministic termination, transcript written | unit | `python -m pytest agent/test_react.py -q` | ❌ W0 | ⬜ pending |
| 107-01-03 | 01 | 1 | AGENT-03 | — | OFF prompt provably omits steering sentinel | unit | `python -m pytest agent/test_agent.py -q` | ❌ W0 | ⬜ pending |
| 107-01-04 | 01 | 1 | AGENT-01/02/03 | — | anti-vacuity: degenerate always-grep agent scores 0; hermetic (no net/keys) | unit | `python -m pytest agent/test_agent.py -q` | ❌ W0 | ⬜ pending |
| 107-01-05 | 01 | 1 | ADOPT-04 (cross) | — | no Go edge; quarantine boundary intact | boundary | `make vet && git diff --exit-code go.mod` | ✅ | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `tools/dspy-tune/agent/test_agent.py` — hermetic fake-LLM + fake-`helix` harness (no network, keys unset)
- [ ] `tools/dspy-tune/agent/conftest.py` — shared fixtures (fake LLM client, fake `helix` shim on PATH, temp transcript dir)
- [ ] pytest config in `tools/dspy-tune/` if no framework config detected
- [ ] `openai==2.43.0` added to `tools/dspy-tune/requirements.txt` (dev venv only)

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

# Helix Evaluation Harness

Helix eval harness — synthetic corpus only by default; commercial-LLM API calls run with retention-zero where the provider supports it.

> **eval ↔ bench separation (INFRA-03).** The v1.12 milestone benchmark stack lives independently under `bench/` (see `bench/BENCH.md`) and is a separate sibling tree from this `eval/` harness. `eval/` stays the in-process PR-gate wiring smoke; `bench/` is the milestone artifact for headline, publishable claims. The two trees share **no code and no pricing file**, and the boundary is **prose-enforced** for this milestone (no analyzer; D-08). This paragraph is the reciprocal cross-link required by INFRA-03 and is the single permitted change to `eval/` under BENCH-01.

## Provider Retention Attestation

**Verified at:** 2026-05-10 (Phase 67 planning)
**Re-verification cadence:** Every minor Helix release (quarterly minimum)

### Anthropic API (used by `claude` CLI in eval modes)

| Attribute | Value | Source |
|-----------|-------|--------|
| Default API log retention | 7 days (since 2025-09-14) | https://privacy.claude.com/en/articles/8956058 |
| Zero Data Retention (ZDR) availability | Enterprise opt-in via DPA | https://platform.claude.com/docs/en/build-with-claude/api-and-data-retention |
| ZDR enforcement mechanism | Account-level (no per-request header); enforced by Anthropic when DPA signed | https://platform.claude.com/docs/en/build-with-claude/api-and-data-retention |
| Claude Code ZDR | Available via Claude for Enterprise | https://code.claude.com/docs/en/zero-data-retention |
| Retrieved on | 2026-05-10 |

> **INFORMATIONAL:** The Anthropic API retains call data for 7 days by default. Zero Data Retention (ZDR) is an enterprise-tier feature that requires a signed Data Processing Agreement (DPA) with Anthropic. Eval runs that process proprietary corpora MUST use an API key covered by an active ZDR DPA.

### Future Providers (placeholder)

| Provider | Status | TOS link | Retrieved |
|----------|--------|----------|-----------|
| OpenAI (Codex) | Not in scope (deferred) | — | — |
| Google (Gemini CLI) | Not in scope (deferred) | — | — |

## Eval-Run Policy

- **Default mode (no enterprise account configured):** runs against synthetic corpus only; OSS Helix repo permitted as secondary source.
- **Enterprise-ZDR mode (env var `HELIX_EVAL_ZDR_VERIFIED=1`):** may run against private corpora. Operator attests they hold a signed Anthropic DPA covering the API key in use. Helix performs no header-based verification (none exists per Anthropic docs).
- **Eval runner refuses to start** against non-synthetic, non-OSS-Helix corpora unless `HELIX_EVAL_ZDR_VERIFIED=1` is set.

## ZDR Caveat (M5 Pitfall Mitigation)

**ZDR is account-level, NOT per-request. There is no `X-ZDR-Required: true` header.**

ZDR status is established when your organization signs a DPA with Anthropic. Once the DPA is in place, ALL API calls from your organization's API keys have ZDR applied — you cannot selectively enable or disable it per request. The `HELIX_EVAL_ZDR_VERIFIED=1` environment variable is a HUMAN ATTESTATION gate; it does not contact Anthropic to verify account ZDR status.

Operators MUST verify their account ZDR status directly via the Anthropic console or DPA documentation before setting this env var.

## eval-quick vs eval: Critical Distinction

> **WARNING:** `eval-quick` uses a scripted agent and does NOT measure real Claude Code agent behavior. Use `make eval` for behavior.

| Target | What it proves | When to run |
|--------|---------------|-------------|
| `make eval-quick` | Harness wiring (daemon startup, profile loading, scorer, reporter) — not agent behavior | Every PR (CI gate, <30s wall) |
| `make eval` | Real Claude Code agent behavior on 50–100 tasks across 4 modes | Nightly / pre-release (NOT PR-gating; see project rule: benchmarks local-only) |

`make eval-quick` uses an in-process scripted agent that issues hard-coded MCP tool calls. It validates the harness infrastructure, not what a real language model does. Never report `eval-quick` results as agent-behavior measurements.

## Operator ZDR Checklist

Before setting `HELIX_EVAL_ZDR_VERIFIED=1` and running eval against a non-synthetic corpus:

- [ ] **Re-verify ZDR quarterly.** Anthropic's retention policy is subject to change. Check https://platform.claude.com/docs/en/build-with-claude/api-and-data-retention before each eval cycle.
- [ ] **Confirm signed DPA is in effect.** ZDR requires a Data Processing Agreement between your organization and Anthropic. Verbal or informal agreements do not activate ZDR.
- [ ] **Verify the API key is covered.** ZDR applies at the organization level; confirm the specific API key used in `ANTHROPIC_API_KEY` belongs to the ZDR-enabled organization.
- [ ] **Understand that `HELIX_EVAL_ZDR_VERIFIED=1` is human-attestation only.** Setting this variable is your attestation that the above checks have been completed. Helix does not verify ZDR status programmatically.
- [ ] **Treat synthetic corpus runs as the default.** When in doubt, run against `eval/corpus/` (Helix OSS fixtures) only.

## EVAL-07: LLM Judge Informational Status

The LLM judge (`tool_behavior_judge.json`) is **INFORMATIONAL ONLY** and must **never** gate merges. Four layers of mitigation enforce this:

### Four-Layer EVAL-07 Mitigation

1. **Separate JSON file** — Judge output is written to `tool_behavior_judge.json`, separate from `tool_behavior.json` (heuristic CI gate). The two files are never combined.

2. **INFORMATIONAL `__readme` boilerplate** — Every `tool_behavior_judge.json` (even stubs) contains `"__readme": "INFORMATIONAL — DO NOT USE FOR CI GATING"` at the top level. This is asserted by `TestJudgeOutputBoilerplate`.

3. **Judge errors swallowed in Run signature** — `judge.Run(...)` returns `Output` only (no error). A complete Anthropic API outage produces a stub file with `"judge_failed": true` and exits 0. The runner exit code is computed exclusively from per-task heuristic results (see EVAL-07 comment block in `cmd/helix-eval/main.go`).

4. **CI grep gate** — The `go-test.yml` workflow includes a `forbid judge in CI` step that `grep`s all workflow files for `tool_behavior_judge` references (excluding itself). Any future PR that wires the judge into a CI gate will fail this check automatically.

### Quick Troubleshooting

**If a CI failure mentions `tool_behavior_judge`:**
The grep gate caught a regression. A PR has added a reference to `tool_behavior_judge.json` in a GitHub Actions workflow file. Revert the workflow change. The judge output is never CI-actionable.

**If the judge section is missing from `eval_report.md`:**
The judge was not run. Either `--no-judge` was set (default for `make eval`), or `ANTHROPIC_API_KEY` was not set. This is expected in CI. Run `make eval` locally with `ANTHROPIC_API_KEY` set to include judge output.

**If `judge_failed: true` in `tool_behavior_judge.json`:**
The Anthropic API was unreachable or returned an error. The `eval_report.md` will note this. This does not affect the eval exit code.

## ZDR Operator Checklist (per quarter)

- [ ] Anthropic DPA still active for the API key in use
- [ ] `HELIX_EVAL_ZDR_VERIFIED=1` ONLY when point above is true
- [ ] No external corpora committed to `eval/corpus/` without explicit code review

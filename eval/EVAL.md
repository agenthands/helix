# Helix Evaluation Harness

Helix eval harness — synthetic corpus only by default; commercial-LLM API calls run with retention-zero where the provider supports it.

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

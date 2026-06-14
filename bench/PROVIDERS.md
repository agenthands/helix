# Bench Provider TOS Attestation

This file is the **per-provider Terms-of-Service attestation** surface for the v1.12 bench
stack (INFRA-01). Each commercial-LLM provider that the bench runners may call has one
section below. Each section opens with a `---`-delimited **YAML frontmatter block** carrying
a fixed field set (D-14), followed by human-readable prose notes modeled on `eval/EVAL.md`.

`make verify-tos` (Plan 05) parses the frontmatter block of every provider section with a
**strict decoder** (unknown keys hard-fail) and enforces that `attested_on` is fresh
(within the 90-day gate). It checks **only** that the date is fresh and the frontmatter
parses — it does **NOT** and **CANNOT** verify that `benchmarking_permitted` /
`publish_permitted` are factually true against the live provider TOS. That legal
determination is a **manual, human-only verification** (VALIDATION.md Manual-Only row); a
blocking human checkpoint in this plan confirms the flags against live provider terms.

## Frontmatter field set (D-14)

Each provider block carries EXACTLY these six keys:

| Key | Type | Meaning |
|-----|------|---------|
| `provider` | string | Provider / API name |
| `tos_url` | string | Canonical URL of the Terms of Service the attestation is made against |
| `attested_by` | string | Maintainer who made the attestation |
| `attested_on` | date (`YYYY-MM-DD`, Go layout `2006-01-02`) | Date the attestation was made / last re-verified |
| `benchmarking_permitted` | bool | The live TOS permits running benchmarks against this provider |
| `publish_permitted` | bool | The live TOS permits publishing the resulting benchmark numbers |

## Display-only safety note

The `benchmarking_permitted`, `publish_permitted`, `tos_url`, `attested_by`, and all
free-text fields in this file are **DISPLAY / ATTESTATION ONLY**. They are parsed for
display and freshness validation and are **never** `exec`'d, never passed to a shell, and
never used as a command or query sink. There is no injection surface here (T-75-04: accept).

---

## Anthropic API (`claude` CLI, `your_agent_*` modes)

---
provider: Anthropic
tos_url: https://www.anthropic.com/legal/commercial-terms
attested_by: helix-maintainers
attested_on: 2026-06-14
benchmarking_permitted: true
publish_permitted: true
---

Anthropic is the primary provider for the v1.12 bench stack (the `claude` CLI drives the
`your_agent_*` modes). Retention and ZDR specifics are documented in `eval/EVAL.md`
(Provider Retention Attestation, verified 2026-05-10): the API retains call data 7 days by
default, with account-level Zero Data Retention available under a signed DPA.

> **ATTESTATION (requires human confirmation against live TOS):** As of `attested_on`, the
> Anthropic Commercial Terms of Service are attested to permit benchmarking
> (`benchmarking_permitted: true`) and publishing of resulting benchmark numbers
> (`publish_permitted: true`). These flags are a maintainer attestation and MUST be
> confirmed against the live TOS at `tos_url` before relying on them — `make verify-tos`
> only checks freshness, not legal accuracy.

---

## Future providers (placeholder — not in scope for v1.12)

The providers below are **placeholders**, deferred and not exercised by the v1.12 bench
stack. Their flags are set conservatively to `false` so that no benchmark run treats them as
permitted until a maintainer attests against their live TOS and flips the flags.

---
provider: OpenAI
tos_url: https://openai.com/policies/terms-of-use
attested_by: helix-maintainers
attested_on: 2026-06-14
benchmarking_permitted: false
publish_permitted: false
---

OpenAI (Codex / `gpt-*`) is **not in scope** for v1.12 (deferred). The flags are `false`
pending a future maintainer attestation against the live OpenAI Terms of Use.

---
provider: Google
tos_url: https://ai.google.dev/gemini-api/terms
attested_by: helix-maintainers
attested_on: 2026-06-14
benchmarking_permitted: false
publish_permitted: false
---

Google (Gemini CLI / `gemini-*`) is **not in scope** for v1.12 (deferred). The flags are
`false` pending a future maintainer attestation against the live Gemini API Additional
Terms of Service.

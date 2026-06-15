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
determination is a **manual, human-only verification** (VALIDATION.md Manual-Only row).

> ⚠️ **PERMISSIVE DEFAULTS — TOS VERIFICATION DEFERRED.** Every provider block below
> (Anthropic, OpenAI, Google, and local/self-hosted models) currently sets
> `benchmarking_permitted: true` and `publish_permitted: true` as **permissive defaults**,
> chosen deliberately to unblock the provider-independent benchmark system. **These flags
> are NOT yet verified against the live provider Terms of Service.** They are maintainer
> placeholders, NOT independently confirmed legal attestations. Real, per-provider TOS
> verification (visiting each `tos_url` and confirming the live terms) is **explicitly
> deferred to a future phase**. Do not rely on these flags as evidence that a given
> provider's TOS permits benchmarking or publishing until that verification phase lands and
> flips them to verified attestations.

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
attested_on: 2026-06-15
benchmarking_permitted: true
publish_permitted: true
---

Anthropic is the primary provider for the v1.12 bench stack (the `claude` CLI drives the
`your_agent_*` modes). Retention and ZDR specifics are documented in `eval/EVAL.md`
(Provider Retention Attestation, verified 2026-05-10): the API retains call data 7 days by
default, with account-level Zero Data Retention available under a signed DPA.

> **PERMISSIVE DEFAULT (not yet verified against live TOS):** `benchmarking_permitted: true`
> and `publish_permitted: true` are set as permissive defaults to unblock the
> provider-independent bench system. They have NOT been confirmed against the live Anthropic
> Commercial Terms of Service at `tos_url`; that verification is deferred to a future phase.
> `make verify-tos` only checks freshness, not legal accuracy.

---

## Additional providers

The providers below carry the same **permissive defaults** as Anthropic so the
provider-independent bench system treats every provider uniformly. Their flags are NOT yet
verified against live TOS (see the top-of-file note); real per-provider verification is
deferred to a future phase.

---
provider: OpenAI
tos_url: https://openai.com/policies/terms-of-use
attested_by: helix-maintainers
attested_on: 2026-06-15
benchmarking_permitted: true
publish_permitted: true
---

OpenAI (Codex / `gpt-*`). `benchmarking_permitted: true` and `publish_permitted: true` are
**permissive defaults** to unblock the provider-independent bench system; they have NOT been
confirmed against the live OpenAI Terms of Use at `tos_url`. That verification is deferred to
a future phase.

---
provider: Google
tos_url: https://ai.google.dev/gemini-api/terms
attested_by: helix-maintainers
attested_on: 2026-06-15
benchmarking_permitted: true
publish_permitted: true
---

Google (Gemini CLI / `gemini-*`). `benchmarking_permitted: true` and `publish_permitted: true`
are **permissive defaults** to unblock the provider-independent bench system; they have NOT
been confirmed against the live Gemini API Additional Terms of Service at `tos_url`. That
verification is deferred to a future phase.

---
provider: Local / self-hosted
tos_url: local
attested_by: helix-maintainers
attested_on: 2026-06-15
benchmarking_permitted: true
publish_permitted: true
---

Local / self-hosted models (e.g. Ollama, llama.cpp, vLLM, or any model the operator runs on
their own hardware). These have **no external API Terms of Service** — the operator controls
both the model and the data — so `benchmarking_permitted: true` and `publish_permitted: true`
are the natural defaults rather than placeholders. The `tos_url: local` sentinel marks that
no external TOS applies. As with every block in this file, these flags are permissive
defaults and `make verify-tos` checks only frontmatter freshness, not legal accuracy.

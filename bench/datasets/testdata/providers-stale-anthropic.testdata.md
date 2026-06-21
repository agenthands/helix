# STALE-ANTHROPIC FIXTURE — for verify-tos HARD-FAIL tests ONLY (CR-01 regression).
#
# This reproduces the ORIGINAL CR-01 bug shape: a standalone Markdown
# horizontal-rule `---` appears BEFORE the first real attestation block, which
# the old sequential fence-pairing parser used to consume — letting a stale
# Anthropic block slip through the gate. The Anthropic block here carries a
# stale `attested_on` (>90d before the injected test clock 2026-06-15), so
# verifyTOS() MUST return a non-nil error. OpenAI is kept fresh to prove the
# failure is the Anthropic block specifically, not a blanket reject.

## Display-only safety note

Some prose with a trailing horizontal rule below, exactly like the real file.

---

## Anthropic API

---
provider: Anthropic
tos_url: https://www.anthropic.com/legal/commercial-terms
attested_by: helix-maintainers
attested_on: 2020-01-01
benchmarking_permitted: true
publish_permitted: true
---

Stale Anthropic attestation (>90 days old relative to the injected test clock).

---
provider: OpenAI
tos_url: https://openai.com/policies/terms-of-use
attested_by: helix-maintainers
attested_on: 2026-06-15
benchmarking_permitted: true
publish_permitted: true
---

Fresh OpenAI attestation.

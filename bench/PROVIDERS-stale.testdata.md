# STALE FIXTURE — for verify-tos HARD-FAIL tests ONLY

This is NOT a real attestation surface. It carries a single provider block whose
`attested_on` is more than 90 days before the injected test clock (2026-06-15),
so `verifyTOS()` must return a non-nil error (D-16).

---
provider: Anthropic
tos_url: https://www.anthropic.com/legal/commercial-terms
attested_by: helix-maintainers
attested_on: 2026-01-01
benchmarking_permitted: true
publish_permitted: true
---

Stale attestation (>90 days old relative to the injected test clock).

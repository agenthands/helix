# Research — DeepSeek model ids, pricing, OpenAI-compatible surface (as of 2026-06-24)

**Bottom line:** Pin `DSPY_LM_MODEL=deepseek-v4-flash` explicitly now. The legacy `deepseek-chat`/`deepseek-reasoner` aliases retire **2026-07-24 15:59 UTC** (mid-milestone risk), and `deepseek-reasoner` does **not** support function calling — it would silently break the ReAct tool loop. Pinning the explicit `deepseek-v4-*` id makes the cutover a one-line change and is already future-proof.

## Model ids & cutover
- Live ids: `deepseek-v4-flash`, `deepseek-v4-pro`. Aliases `deepseek-chat`/`deepseek-reasoner` deprecated 2026-07-24; today they still resolve (→ non-thinking / thinking modes of v4-flash).
- **Pin explicit `deepseek-v4-flash`** (default) / `deepseek-v4-pro` (only where stronger reasoning needed).

## Pricing (per 1M tokens, official)
| Model | input cache-hit | input cache-miss | output |
|---|---|---|---|
| `deepseek-v4-flash` | $0.0028 | $0.14 | $0.28 |
| `deepseek-v4-pro`   | $0.003625 | $0.435 | $0.87 |
- No off-peak discount documented for V4. **Cache-hit input is ~50× cheaper than cache-miss** → a stable system-prompt prefix across GEPA candidates is the biggest cost lever.

## OpenAI-compatible surface
- `base_url="https://api.deepseek.com"` (append `/v1` only if a 404 appears; beta features at `/beta`). Works with `openai==2.43.0`.
- Both V4 models support standard `tools` function calling + JSON output. Context 1M tokens, max output up to ~384K (verify exact figure before relying on it for truncation/budget).
- **Caveat:** legacy `deepseek-reasoner` alias has NO function calling — do not use it for the tool-using agent; use `deepseek-v4-pro` (thinking via `extra_body={"thinking":{"type":"enabled"}}`) if reasoning+tools are needed.

## Rate limits
- DeepSeek limits by **concurrency** (not RPM/TPM): v4-flash = 2500, v4-pro = 500 concurrent; over-limit → HTTP 429. Cap in-flight concurrency below the limit and enable retry-with-backoff (DSPy/LiteLLM retries).

## Sources
- https://api-docs.deepseek.com/quick_start/pricing
- https://api-docs.deepseek.com/ (base_url)
- https://api-docs.deepseek.com/guides/function_calling
- https://api-docs.deepseek.com/guides/reasoning_model
- https://api-docs.deepseek.com/quick_start/rate_limit

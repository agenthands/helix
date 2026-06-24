"""Provider-agnostic chat LLM: DeepSeek primary, OpenAI fallback.

DEV-TIME / OFFLINE ONLY. This module lives under tools/ and is NEVER linked
into the helix binary, `helix setup`, go.mod, or the default `go test ./...`
path. One `openai` client wraps BOTH providers — DeepSeek is OpenAI
wire-compatible (`base_url=https://api.deepseek.com`), so a single SDK covers
the primary and the fallback.

LOUD-FAIL contract (AGENT-02): this is the explicit INVERSION of
`optimize.py`'s quiet `return 0` on a missing key (optimize.py:48-62). A real
run with the selected provider's API key absent RAISES RuntimeError — it never
silently produces an empty result. A quiet skip-and-exit-0 may live ONLY in an
optional `__main__` wrapper, never inside `__init__`/`chat()` used by `run()`.

Keys are read from the environment ONLY and are never logged, returned, or
written to any transcript.
"""

import os

# Guarded import: the hermetic test suite injects duck-typed fakes and
# constructs NO real client, so this module must import even when `openai` is
# not installed in the dev venv. The module-level `OpenAI` symbol is also the
# documented fallback injection seam (the test monkeypatches `agent.llm.OpenAI`).
try:  # pragma: no cover - trivial import guard
    from openai import OpenAI
except ImportError:  # pragma: no cover
    OpenAI = None

# Config vars (alias-safe). `deepseek-chat`/`deepseek-reasoner` retire
# 2026-07-24 15:59 UTC -> default to the explicit successor id.
DEFAULT_MODEL = "deepseek-v4-flash"
DEFAULT_FALLBACK_MODEL = "gpt-4.1-mini"

_DEEPSEEK_BASE_URL = "https://api.deepseek.com"

_PROVIDER_KEY_ENV = {
    "deepseek": "DEEPSEEK_API_KEY",
    "openai": "OPENAI_API_KEY",
}


class LLM:
    """A minimal chat client. `chat(messages, tools)` returns the first choice's
    message object (duck-typed: `.content`, `.tool_calls`, `.model_dump(...)`)."""

    def __init__(self, provider="deepseek", model=None, fallback_model=None):
        self.provider = provider
        # Model precedence: explicit arg > AGENT_MODEL env > DEFAULT_MODEL.
        self.model = model or os.environ.get("AGENT_MODEL") or DEFAULT_MODEL
        self.fallback_model = (
            fallback_model
            or os.environ.get("AGENT_FALLBACK_MODEL")
            or DEFAULT_FALLBACK_MODEL
        )

        key_env = _PROVIDER_KEY_ENV.get(provider, "DEEPSEEK_API_KEY")
        api_key = os.environ.get(key_env)
        # LOUD-FAIL: no silent return-0 on a missing key (inverts optimize.py).
        if not api_key:
            raise RuntimeError(
                f"{key_env} is not set — cannot run the {provider} agent. "
                f"Set it in the dev environment (it is never read by any Go code). "
                f"This is a hard failure by design (no silent skip)."
            )

        base_url = _DEEPSEEK_BASE_URL if provider == "deepseek" else None
        # Construct the primary client only if the SDK is importable. Client
        # construction is lazy/offline (no network until .create()). When the
        # SDK is absent (hermetic test venv), `primary` is None and the test
        # injects its own fake via the `llm.primary` seam.
        self.primary = OpenAI(api_key=api_key, base_url=base_url) if OpenAI else None

    def chat(self, messages, tools=None):
        """Call the primary provider; on ANY primary failure, fall back to OpenAI.

        The fallback client is built through the module-level `OpenAI` symbol so
        the test can monkeypatch `agent.llm.OpenAI` to force the except->fallback
        branch without a real network client (WARNING-4 injection seam).
        """
        try:
            if self.primary is None:
                raise RuntimeError("primary client unavailable")
            resp = self.primary.chat.completions.create(
                model=self.model, messages=messages, tools=tools
            )
            # Expose token usage for cost metering (attribution.MeteredLLM reads
            # `.last_usage` since the returned message carries no usage block).
            self.last_usage = getattr(resp, "usage", None)
            return resp.choices[0].message
        except Exception as primary_err:
            fallback_key = os.environ.get("OPENAI_API_KEY")
            if not fallback_key:
                raise RuntimeError(
                    f"primary provider failed and OPENAI_API_KEY is not set for "
                    f"fallback: {primary_err}"
                ) from primary_err
            if OpenAI is None:
                raise RuntimeError(
                    "openai SDK unavailable — cannot build fallback client"
                ) from primary_err
            fallback = OpenAI(api_key=fallback_key)
            resp = fallback.chat.completions.create(
                model=self.fallback_model, messages=messages, tools=tools
            )
            self.last_usage = getattr(resp, "usage", None)
            return resp.choices[0].message

from __future__ import annotations

import os
from typing import Protocol


class LLMProvider(Protocol):
    async def complete(self, prompt: str) -> str:
        ...


class MockProvider:
    """Deterministic mock provider for CI and fast repeatable tests."""

    async def complete(self, prompt: str) -> str:
        return (
            "Evidence indicates recurrent thermostat failure under high load. "
            "Maintenance manual §4.2 recommends immediate thermostat replacement (SP-COOL-9981) "
            "and secondary cooling circuit flush."
        )


class OllamaProvider:
    """Local Ollama provider for cost-free local dev loop."""

    def __init__(self, base_url: str = "http://127.0.0.1:11434", model: str = "qwen2.5:0.5b"):
        self.base_url = base_url
        self.model = model

    async def complete(self, prompt: str) -> str:
        import httpx

        async with httpx.AsyncClient(timeout=30.0) as client:
            resp = await client.post(
                f"{self.base_url}/api/generate",
                json={"model": self.model, "prompt": prompt, "stream": False},
            )
            resp.raise_for_status()
            return resp.json().get("response", "")


class AnthropicProvider:
    """Hosted Anthropic Claude provider for production/demo mode."""

    def __init__(self, api_key: str | None = None, model: str = "claude-3-5-sonnet-latest"):
        self.api_key = api_key or os.getenv("ANTHROPIC_API_KEY", "")
        self.model = model

    async def complete(self, prompt: str) -> str:
        from anthropic import AsyncAnthropic

        client = AsyncAnthropic(api_key=self.api_key)
        resp = await client.messages.create(
            model=self.model,
            max_tokens=1024,
            messages=[{"role": "user", "content": prompt}],
        )
        first = resp.content[0] if resp.content else None
        return getattr(first, "text", "") if first is not None else ""


def get_provider() -> LLMProvider:
    provider_type = os.getenv("LLM_PROVIDER", "mock").lower()
    if provider_type == "ollama":
        return OllamaProvider(
            base_url=os.getenv("OLLAMA_BASE_URL", "http://127.0.0.1:11434"),
            model=os.getenv("OLLAMA_MODEL", "qwen2.5:0.5b"),
        )
    elif provider_type == "anthropic":
        return AnthropicProvider()
    return MockProvider()

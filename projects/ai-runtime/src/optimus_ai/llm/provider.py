from __future__ import annotations

import os
from dataclasses import dataclass
from typing import Protocol

from pydantic_ai.models import Model
from pydantic_ai.models.test import TestModel


class LLMProvider(Protocol):
    """Protocol for direct string-based completions (backward compatibility & utilities)."""

    async def complete(self, prompt: str) -> str: ...


@dataclass
class LLMSettings:
    """Configuration settings for LLM and Pydantic AI model providers."""

    provider: str = "mock"
    model_name: str | None = None
    api_key: str | None = None
    base_url: str | None = None

    @classmethod
    def from_env(cls) -> LLMSettings:
        provider = os.getenv("LLM_PROVIDER", "mock").lower()

        # Model resolution with provider-specific fallback
        generic_model = os.getenv("LLM_MODEL")
        generic_key = os.getenv("LLM_API_KEY")
        generic_url = os.getenv("LLM_BASE_URL")

        if provider == "anthropic":
            model = generic_model or os.getenv("ANTHROPIC_MODEL", "claude-3-5-sonnet-latest")
            key = generic_key or os.getenv("ANTHROPIC_API_KEY")
            base_url = generic_url or os.getenv("ANTHROPIC_BASE_URL")
        elif provider == "openai":
            model = generic_model or os.getenv("OPENAI_MODEL", "gpt-4o")
            key = generic_key or os.getenv("OPENAI_API_KEY")
            base_url = generic_url or os.getenv("OPENAI_BASE_URL")
        elif provider == "ollama":
            model = generic_model or os.getenv("OLLAMA_MODEL", "qwen2.5:0.5b")
            key = generic_key or os.getenv("OLLAMA_API_KEY", "ollama")
            base_url = generic_url or os.getenv("OLLAMA_BASE_URL", "http://127.0.0.1:11434/v1")
        else:
            provider = "mock"
            model = generic_model or "mock-model"
            key = generic_key or "mock-key"
            base_url = generic_url

        return cls(
            provider=provider,
            model_name=model,
            api_key=key,
            base_url=base_url,
        )


def get_model(settings: LLMSettings | None = None) -> Model:
    """Return a configured pydantic_ai.models.Model based on settings or environment variables."""
    cfg = settings or LLMSettings.from_env()

    if cfg.provider == "anthropic":
        from pydantic_ai.models.anthropic import AnthropicModel
        from pydantic_ai.providers.anthropic import AnthropicProvider

        anthropic_provider = AnthropicProvider(
            api_key=cfg.api_key,
            base_url=cfg.base_url,
        )
        return AnthropicModel(
            model_name=cfg.model_name or "claude-3-5-sonnet-latest",  # type: ignore[arg-type]
            provider=anthropic_provider,
        )

    if cfg.provider == "openai":
        from pydantic_ai.models.openai import OpenAIChatModel
        from pydantic_ai.providers.openai import OpenAIProvider

        openai_provider = OpenAIProvider(
            api_key=cfg.api_key,
            base_url=cfg.base_url,
        )
        return OpenAIChatModel(
            model_name=cfg.model_name or "gpt-4o",  # type: ignore[arg-type]
            provider=openai_provider,
        )

    if cfg.provider == "ollama":
        from pydantic_ai.models.openai import OpenAIChatModel
        from pydantic_ai.providers.openai import OpenAIProvider

        # Ollama exposes OpenAI-compatible endpoint at /v1
        base_url = cfg.base_url or "http://127.0.0.1:11434/v1"
        if not base_url.endswith("/v1") and not base_url.endswith("/v1/"):
            base_url = f"{base_url.rstrip('/')}/v1"

        ollama_provider = OpenAIProvider(
            api_key=cfg.api_key or "ollama",
            base_url=base_url,
        )
        return OpenAIChatModel(
            model_name=cfg.model_name or "qwen2.5:0.5b",  # type: ignore[arg-type]
            provider=ollama_provider,
        )

    # Deterministic TestModel for fast, cost-free tests & CI
    return TestModel(
        custom_output_text=(
            "Evidence indicates recurrent thermostat failure under high load. "
            "Maintenance manual §4.2 recommends immediate thermostat replacement (SP-COOL-9981) "
            "and secondary cooling circuit flush."
        )
    )


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

    def __init__(
        self,
        api_key: str | None = None,
        model: str = "claude-3-5-sonnet-latest",
        base_url: str | None = None,
    ):
        self.api_key = api_key or os.getenv("ANTHROPIC_API_KEY", "")
        self.model = model
        self.base_url = base_url or os.getenv("ANTHROPIC_BASE_URL")

    async def complete(self, prompt: str) -> str:
        from anthropic import AsyncAnthropic

        client = AsyncAnthropic(api_key=self.api_key, base_url=self.base_url)
        resp = await client.messages.create(
            model=self.model,
            max_tokens=1024,
            messages=[{"role": "user", "content": prompt}],
        )
        first = resp.content[0] if resp.content else None
        return getattr(first, "text", "") if first is not None else ""


class OpenAIProvider:
    """Hosted OpenAI provider for production/demo mode."""

    def __init__(
        self,
        api_key: str | None = None,
        model: str = "gpt-4o",
        base_url: str | None = None,
    ):
        self.api_key = api_key or os.getenv("OPENAI_API_KEY", "")
        self.model = model
        self.base_url = base_url or os.getenv("OPENAI_BASE_URL")

    async def complete(self, prompt: str) -> str:
        from openai import AsyncOpenAI

        client = AsyncOpenAI(api_key=self.api_key, base_url=self.base_url)
        resp = await client.chat.completions.create(
            model=self.model,
            messages=[{"role": "user", "content": prompt}],
        )
        first = resp.choices[0] if resp.choices else None
        return first.message.content or "" if first and first.message else ""


def get_provider(settings: LLMSettings | None = None) -> LLMProvider:
    cfg = settings or LLMSettings.from_env()
    if cfg.provider == "ollama":
        return OllamaProvider(
            base_url=cfg.base_url or "http://127.0.0.1:11434",
            model=cfg.model_name or "qwen2.5:0.5b",
        )
    if cfg.provider == "anthropic":
        return AnthropicProvider(
            api_key=cfg.api_key,
            model=cfg.model_name or "claude-3-5-sonnet-latest",
            base_url=cfg.base_url,
        )
    if cfg.provider == "openai":
        return OpenAIProvider(
            api_key=cfg.api_key,
            model=cfg.model_name or "gpt-4o",
            base_url=cfg.base_url,
        )
    return MockProvider()

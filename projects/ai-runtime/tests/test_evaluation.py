from __future__ import annotations

import pytest

from optimus_ai.evaluation import (
    GOLDEN_DATASET,
    EvaluationHarness,
)
from optimus_ai.llm.provider import LLMSettings, get_model


def test_golden_dataset_integrity():
    assert len(GOLDEN_DATASET) >= 10, "Golden dataset must contain at least 10 realistic enterprise scenarios"
    for scenario in GOLDEN_DATASET:
        assert scenario.scenario_id, "Scenario ID cannot be empty"
        assert scenario.tenant_id == "acme", "Tenant ID must be set"
        assert scenario.asset_id, "Asset ID must be set"
        assert len(scenario.expected_tools) >= 3, "Expected tools must cover EAM, PLM, and ERP"
        assert scenario.expected_spare_part_id.startswith("SP-"), "Spare part ID must follow convention"
        assert scenario.expected_min_failures >= 1, "Expected min failures must be positive"


@pytest.mark.asyncio
async def test_evaluation_harness_primary_pump_scenario():
    harness = EvaluationHarness()
    p104_scenario = next(s for s in GOLDEN_DATASET if s.scenario_id == "P104-OVERHEAT")

    result = await harness.run_scenario(p104_scenario)

    assert result.passed is True, f"Primary scenario P-104 failed: {result.failures}"
    assert len(result.failures) == 0
    assert result.evidence is not None
    assert result.evidence.asset_id == "P-104"
    assert result.evidence.tenant_id == "acme"
    assert result.evidence.failures_last_30_days >= 4
    assert result.evidence.spare_part_id == "SP-COOL-9981"
    assert result.evidence.spare_part_in_stock is True
    assert len(result.evidence.tool_calls) == 3
    assert result.duration_ms > 0


@pytest.mark.asyncio
async def test_evaluation_harness_full_golden_suite():
    harness = EvaluationHarness()
    report = await harness.run_all()

    assert report.total_scenarios == len(GOLDEN_DATASET)
    assert report.passed_count == len(GOLDEN_DATASET)
    assert report.failed_count == 0
    assert report.pass_rate == 100.0

    summary_text = report.summary()
    assert "OPTIMUS AI EVALUATION REPORT" in summary_text
    assert "10/10 PASSED" in summary_text
    assert "100.0%" in summary_text
    assert "[PASS] P104-OVERHEAT" in summary_text
    assert "[PASS] T200-VIBRATION" in summary_text


@pytest.mark.asyncio
async def test_evaluation_harness_detects_missing_tool():
    harness = EvaluationHarness()
    scenario = GOLDEN_DATASET[0]

    # Create a scenario expecting a non-existent tool
    modified_scenario = type(scenario)(
        scenario_id="TEST-FAIL-TOOL",
        name="Test Tool Failure Detection",
        tenant_id=scenario.tenant_id,
        asset_id=scenario.asset_id,
        symptom=scenario.symptom,
        expected_tools=["non_existent.tool_call"],
        expected_min_failures=scenario.expected_min_failures,
        expected_spare_part_id=scenario.expected_spare_part_id,
        expected_spare_in_stock=scenario.expected_spare_in_stock,
        expected_severity=scenario.expected_severity,
        expected_safety_risk=scenario.expected_safety_risk,
        expected_field_visit=scenario.expected_field_visit,
        mock_history=scenario.mock_history,
        mock_plm=scenario.mock_plm,
        mock_inventory=scenario.mock_inventory,
    )

    result = await harness.run_scenario(modified_scenario)
    assert result.passed is False
    assert any("Missing expected MCP tool call" in f for f in result.failures)


@pytest.mark.asyncio
async def test_evaluation_harness_detects_spare_part_mismatch():
    harness = EvaluationHarness()
    scenario = GOLDEN_DATASET[0]

    # Scenario expecting wrong spare part
    modified_scenario = type(scenario)(
        scenario_id="TEST-FAIL-PART",
        name="Test Part Mismatch Detection",
        tenant_id=scenario.tenant_id,
        asset_id=scenario.asset_id,
        symptom=scenario.symptom,
        expected_tools=scenario.expected_tools,
        expected_min_failures=scenario.expected_min_failures,
        expected_spare_part_id="SP-WRONG-9999",
        expected_spare_in_stock=scenario.expected_spare_in_stock,
        expected_severity=scenario.expected_severity,
        expected_safety_risk=scenario.expected_safety_risk,
        expected_field_visit=scenario.expected_field_visit,
        mock_history=scenario.mock_history,
        mock_plm=scenario.mock_plm,
        mock_inventory=scenario.mock_inventory,
    )

    result = await harness.run_scenario(modified_scenario)
    assert result.passed is False
    assert any("Spare part ID mismatch" in f for f in result.failures)


def test_llm_settings_anthropic_env_vars(monkeypatch):
    monkeypatch.setenv("LLM_PROVIDER", "anthropic")
    monkeypatch.setenv("ANTHROPIC_API_KEY", "sk-ant-custom-key-12345")
    monkeypatch.setenv("ANTHROPIC_MODEL", "claude-3-5-haiku-20241022")
    monkeypatch.setenv("ANTHROPIC_BASE_URL", "https://custom.anthropic.endpoint/v1")

    settings = LLMSettings.from_env()
    assert settings.provider == "anthropic"
    assert settings.api_key == "sk-ant-custom-key-12345"
    assert settings.model_name == "claude-3-5-haiku-20241022"
    assert settings.base_url == "https://custom.anthropic.endpoint/v1"

    model = get_model(settings)
    assert model.model_name == "claude-3-5-haiku-20241022"


def test_llm_settings_openai_env_vars(monkeypatch):
    monkeypatch.setenv("LLM_PROVIDER", "openai")
    monkeypatch.setenv("OPENAI_API_KEY", "sk-openai-custom-key-67890")
    monkeypatch.setenv("OPENAI_MODEL", "gpt-4o-mini")
    monkeypatch.setenv("OPENAI_BASE_URL", "https://api.openai.com/v1")

    settings = LLMSettings.from_env()
    assert settings.provider == "openai"
    assert settings.api_key == "sk-openai-custom-key-67890"
    assert settings.model_name == "gpt-4o-mini"
    assert settings.base_url == "https://api.openai.com/v1"

    model = get_model(settings)
    assert model.model_name == "gpt-4o-mini"


def test_llm_settings_ollama_env_vars(monkeypatch):
    monkeypatch.setenv("LLM_PROVIDER", "ollama")
    monkeypatch.setenv("OLLAMA_MODEL", "qwen2.5:1.5b")
    monkeypatch.setenv("OLLAMA_BASE_URL", "http://192.168.1.100:11434")

    settings = LLMSettings.from_env()
    assert settings.provider == "ollama"
    assert settings.model_name == "qwen2.5:1.5b"
    assert settings.base_url == "http://192.168.1.100:11434"

    model = get_model(settings)
    assert model.model_name == "qwen2.5:1.5b"


def test_llm_settings_generic_overrides(monkeypatch):
    monkeypatch.setenv("LLM_PROVIDER", "openai")
    monkeypatch.setenv("OPENAI_API_KEY", "key-specific")
    monkeypatch.setenv("LLM_API_KEY", "key-override")
    monkeypatch.setenv("LLM_MODEL", "model-override")

    settings = LLMSettings.from_env()
    assert settings.api_key == "key-override"
    assert settings.model_name == "model-override"

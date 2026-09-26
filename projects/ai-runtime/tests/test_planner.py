import pytest

from optimus_ai.agents.planner import PlannerAgent
from optimus_ai.llm.provider import MockProvider


@pytest.mark.asyncio
async def test_planner_investigation_flow():
    planner = PlannerAgent(llm_provider=MockProvider())
    evidence = await planner.investigate(
        tenant_id="acme",
        asset_id="P-104",
        symptom="repeated overheating",
    )

    # Assert structured typed outcomes
    assert evidence.asset_id == "P-104"
    assert evidence.tenant_id == "acme"
    assert evidence.failures_last_30_days >= 4
    assert "PLM-COOL-4021" in evidence.plm_findings or "cooling" in evidence.plm_findings.lower()
    assert evidence.spare_part_id == "SP-COOL-9981"
    assert evidence.spare_part_in_stock is True
    assert len(evidence.recommendation) > 0

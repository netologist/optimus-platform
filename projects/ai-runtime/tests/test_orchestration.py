from __future__ import annotations

import pytest

from optimus_ai.orchestration import (
    InvestigationOrchestrator,
    ProgressEvent,
    ProgressReporter,
)


@pytest.mark.asyncio
async def test_progress_reporter_in_memory_subscription():
    reporter = ProgressReporter()
    received_events: list[ProgressEvent] = []

    reporter.subscribe(lambda e: received_events.append(e))

    await reporter.emit(
        tenant_id="acme",
        asset_id="P-104",
        stage="querying_eam",
        message="Querying maintenance history",
        metadata={"failures": 4},
    )

    assert len(received_events) == 1
    ev = received_events[0]
    assert ev.tenant_id == "acme"
    assert ev.asset_id == "P-104"
    assert ev.stage == "querying_eam"
    assert ev.metadata["failures"] == 4


@pytest.mark.asyncio
async def test_investigation_orchestrator_flow():
    reporter = ProgressReporter()
    stages_recorded: list[str] = []
    reporter.subscribe(lambda e: stages_recorded.append(e.stage))

    orchestrator = InvestigationOrchestrator(progress_reporter=reporter)

    result = await orchestrator.execute_investigation(
        tenant_id="acme",
        asset_id="P-104",
        symptom="recurrent overheating",
    )

    # Assert structured evidence gathered
    assert result["asset_id"] == "P-104"
    assert result["tenant_id"] == "acme"
    assert result["failures_last_30_days"] >= 4
    assert "PLM-COOL-4021" in result["plm_findings"] or "cooling" in result["plm_findings"].lower()
    assert result["spare_part_id"] == "SP-COOL-9981"
    assert result["spare_part_in_stock"] is True
    assert "eam.get_maintenance_history" in result["tool_calls"]
    assert "plm.search_documents" in result["tool_calls"]
    assert "erp.get_inventory" in result["tool_calls"]

    # Assert progress stages traversed
    assert "discovering_tools" in stages_recorded
    assert "querying_eam" in stages_recorded
    assert "searching_plm_rag" in stages_recorded
    assert "checking_erp_inventory" in stages_recorded
    assert "investigation_completed" in stages_recorded

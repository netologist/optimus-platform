from __future__ import annotations

import time
from dataclasses import dataclass, field
from typing import Any

from ..agents.planner import EvidenceContext, PlannerAgent
from ..llm.provider import LLMProvider, MockProvider
from ..mcp.client import MCPClient
from .dataset import GOLDEN_DATASET, GoldenScenario


@dataclass
class EvaluationResult:
    """Individual scenario evaluation outcome with structured assertion records."""

    scenario_id: str
    scenario_name: str
    passed: bool
    failures: list[str] = field(default_factory=list)
    evidence: EvidenceContext | None = None
    duration_ms: float = 0.0


@dataclass
class EvaluationReport:
    """Consolidated test report summarizing all golden scenario evaluations."""

    total_scenarios: int
    passed_count: int
    failed_count: int
    pass_rate: float
    duration_total_ms: float
    results: list[EvaluationResult] = field(default_factory=list)

    def summary(self) -> str:
        lines = [
            "=" * 70,
            f"OPTIMUS AI EVALUATION REPORT: {self.passed_count}/{self.total_scenarios} PASSED ({self.pass_rate:.1f}%)",
            f"Total Duration: {self.duration_total_ms:.2f}ms",
            "=" * 70,
        ]
        for res in self.results:
            status = "PASS" if res.passed else "FAIL"
            lines.append(f"[{status}] {res.scenario_id}: {res.scenario_name} ({res.duration_ms:.1f}ms)")
            if not res.passed:
                for fail in res.failures:
                    lines.append(f"       -> Assertion failure: {fail}")
        lines.append("=" * 70)
        return "\n".join(lines)


class MockHarnessMCPClient(MCPClient):
    """Deterministic MCP client configured with golden scenario responses."""

    def __init__(self, scenario: GoldenScenario):
        super().__init__()
        self.scenario = scenario

    async def discover_tools(self, tenant_id: str) -> list[dict[str, Any]]:
        return [
            {"name": "eam.get_maintenance_history", "endpoint": "http://127.0.0.1:8080"},
            {"name": "plm.search_documents", "endpoint": "http://127.0.0.1:8080"},
            {"name": "erp.get_inventory", "endpoint": "http://127.0.0.1:8080"},
        ]

    async def call_tool(
        self, endpoint: str, tool_name: str, arguments: dict[str, Any]
    ) -> dict[str, Any]:
        if tool_name == "eam.get_maintenance_history":
            return self.scenario.mock_history or {
                "failures_last_30_days": self.scenario.expected_min_failures,
                "asset_id": self.scenario.asset_id,
            }
        if tool_name == "plm.search_documents":
            doc = self.scenario.mock_plm or {
                "document_id": "PLM-DOC-100",
                "section": "§1.0",
                "content": "Failure analysis",
                "spare_part": self.scenario.expected_spare_part_id,
            }
            return {"results": [doc]}
        if tool_name == "erp.get_inventory":
            inv = self.scenario.mock_inventory or {
                "spare_part_id": self.scenario.expected_spare_part_id,
                "in_stock": 1 if self.scenario.expected_spare_in_stock else 0,
            }
            return inv
        return {}


class EvaluationHarness:
    """Evaluation harness running golden scenario suites with structural assertions (ADR-011)."""

    def __init__(self, llm_provider: LLMProvider | None = None):
        self.llm_provider = llm_provider or MockProvider()

    async def run_scenario(
        self,
        scenario: GoldenScenario,
        planner: PlannerAgent | None = None,
    ) -> EvaluationResult:
        start_time = time.perf_counter()
        failures: list[str] = []

        mcp_client = MockHarnessMCPClient(scenario)
        agent = planner or PlannerAgent(
            mcp_client=mcp_client,
            llm_provider=self.llm_provider,
        )

        try:
            evidence = await agent.investigate(
                tenant_id=scenario.tenant_id,
                asset_id=scenario.asset_id,
                symptom=scenario.symptom,
            )
        except Exception as e:
            duration_ms = (time.perf_counter() - start_time) * 1000.0
            return EvaluationResult(
                scenario_id=scenario.scenario_id,
                scenario_name=scenario.name,
                passed=False,
                failures=[f"Agent investigation raised exception: {e}"],
                evidence=None,
                duration_ms=duration_ms,
            )

        # 1. Structural tool call assertion (all expected tools were invoked)
        for expected_tool in scenario.expected_tools:
            if expected_tool not in evidence.tool_calls:
                failures.append(
                    f"Missing expected MCP tool call '{expected_tool}'. Observed calls: {evidence.tool_calls}"
                )

        # 2. Tenant isolation assertion
        if evidence.tenant_id != scenario.tenant_id:
            failures.append(
                f"Tenant ID mismatch: expected '{scenario.tenant_id}', got '{evidence.tenant_id}'"
            )

        # 3. Asset ID assertion
        if evidence.asset_id != scenario.asset_id:
            failures.append(
                f"Asset ID mismatch: expected '{scenario.asset_id}', got '{evidence.asset_id}'"
            )

        # 4. Failures threshold assertion
        if evidence.failures_last_30_days < scenario.expected_min_failures:
            failures.append(
                f"Failure count below expected threshold: expected >= {scenario.expected_min_failures}, got {evidence.failures_last_30_days}"
            )

        # 5. Spare part identification assertion
        if evidence.spare_part_id != scenario.expected_spare_part_id:
            failures.append(
                f"Spare part ID mismatch: expected '{scenario.expected_spare_part_id}', got '{evidence.spare_part_id}'"
            )

        # 6. Inventory availability assertion
        if evidence.spare_part_in_stock != scenario.expected_spare_in_stock:
            failures.append(
                f"Spare part stock status mismatch: expected {scenario.expected_spare_in_stock}, got {evidence.spare_part_in_stock}"
            )

        # 7. Non-empty recommendation assertion
        if not evidence.recommendation or not evidence.recommendation.strip():
            failures.append("Recommendation is empty or blank.")

        duration_ms = (time.perf_counter() - start_time) * 1000.0
        return EvaluationResult(
            scenario_id=scenario.scenario_id,
            scenario_name=scenario.name,
            passed=(len(failures) == 0),
            failures=failures,
            evidence=evidence,
            duration_ms=duration_ms,
        )

    async def run_all(
        self,
        scenarios: list[GoldenScenario] | None = None,
        planner: PlannerAgent | None = None,
    ) -> EvaluationReport:
        suite = scenarios or GOLDEN_DATASET
        results: list[EvaluationResult] = []
        start_total = time.perf_counter()

        for scenario in suite:
            res = await self.run_scenario(scenario, planner=planner)
            results.append(res)

        total_duration_ms = (time.perf_counter() - start_total) * 1000.0
        passed_count = sum(1 for r in results if r.passed)
        failed_count = len(results) - passed_count
        pass_rate = (passed_count / len(results) * 100.0) if results else 0.0

        return EvaluationReport(
            total_scenarios=len(results),
            passed_count=passed_count,
            failed_count=failed_count,
            pass_rate=pass_rate,
            duration_total_ms=total_duration_ms,
            results=results,
        )

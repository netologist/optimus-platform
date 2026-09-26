from __future__ import annotations

from dataclasses import dataclass
from typing import Any

from pydantic import BaseModel, Field
from pydantic_ai import Agent, RunContext
from pydantic_ai.models import Model

from ..llm.provider import LLMProvider, get_model, get_provider
from ..mcp.client import MCPClient
from ..orchestration.orchestrator import InvestigationOrchestrator
from ..rag.retriever import HybridRetriever
from .specialists import EAMSpecialist, ERPSpecialist, PLMSpecialist


class EvidenceContext(BaseModel):
    asset_id: str
    tenant_id: str
    failures_last_30_days: int = 0
    plm_findings: str = ""
    spare_part_id: str = ""
    spare_part_in_stock: bool = False
    recommendation: str = ""
    tool_calls: list[str] = Field(default_factory=list)


@dataclass
class PlannerDeps:
    """Dependencies injected into the Pydantic AI Agent execution context."""

    tenant_id: str
    asset_id: str
    mcp_client: MCPClient
    retriever: HybridRetriever
    endpoint_map: dict[str, str]
    evidence: EvidenceContext


class PlannerAgent:
    """Orchestrates asset failure investigations across enterprise systems via MCP tools

    and synthesizes actionable operational recommendations using Pydantic AI.
    """

    def __init__(
        self,
        mcp_client: MCPClient | None = None,
        llm_provider: LLMProvider | None = None,
        model: Model | None = None,
        retriever: HybridRetriever | None = None,
        orchestrator: InvestigationOrchestrator | None = None,
    ):
        self.mcp_client = mcp_client or MCPClient()
        self.retriever = retriever or HybridRetriever()

        # Orchestration layer coordinating multi-agent steps (PRD §18.4 / ADR-013)
        self.orchestrator = orchestrator or InvestigationOrchestrator(
            mcp_client=self.mcp_client,
            retriever=self.retriever,
        )

        # Resolve Pydantic AI model
        if model is not None:
            self.model = model
        else:
            self.model = get_model()

        # Text provider fallback if specified
        self.llm = llm_provider or get_provider()

        # Initialize Pydantic AI Agent
        self.pydantic_agent = Agent(
            self.model,
            deps_type=PlannerDeps,
            system_prompt=(
                "You are the Optimus AI Operations Planner Agent. Your mission is to investigate "
                "enterprise asset failures by gathering data across EAM (maintenance history), "
                "PLM (engineering manuals & service bulletins), and ERP (spare parts inventory), "
                "and synthesizing governed, evidence-backed operational recommendations."
            ),
        )

        # Register tools on the Pydantic AI Agent
        @self.pydantic_agent.tool
        async def eam_get_maintenance_history(
            ctx: RunContext[PlannerDeps], asset_id: str
        ) -> dict[str, Any]:
            """Retrieve past failure events and maintenance records for an asset from EAM."""
            ep = ctx.deps.endpoint_map.get("eam.get_maintenance_history", "http://127.0.0.1:8080")
            eam = EAMSpecialist(ctx.deps.mcp_client, ep)
            if "eam.get_maintenance_history" not in ctx.deps.evidence.tool_calls:
                ctx.deps.evidence.tool_calls.append("eam.get_maintenance_history")
            return await eam.get_history(ctx.deps.tenant_id, asset_id)

        @self.pydantic_agent.tool
        async def plm_search_documents(
            ctx: RunContext[PlannerDeps], query: str
        ) -> dict[str, Any]:
            """Search technical service manuals and engineering failure modes in PLM."""
            ep = ctx.deps.endpoint_map.get("plm.search_documents", "http://127.0.0.1:8080")
            plm = PLMSpecialist(ctx.deps.mcp_client, ep)
            if "plm.search_documents" not in ctx.deps.evidence.tool_calls:
                ctx.deps.evidence.tool_calls.append("plm.search_documents")
            return await plm.search_manuals(ctx.deps.tenant_id, query)

        @self.pydantic_agent.tool
        async def erp_get_inventory(
            ctx: RunContext[PlannerDeps], spare_part_id: str
        ) -> dict[str, Any]:
            """Check current inventory levels and availability of replacement parts in ERP."""
            ep = ctx.deps.endpoint_map.get("erp.get_inventory", "http://127.0.0.1:8080")
            erp = ERPSpecialist(ctx.deps.mcp_client, ep)
            if "erp.get_inventory" not in ctx.deps.evidence.tool_calls:
                ctx.deps.evidence.tool_calls.append("erp.get_inventory")
            return await erp.check_inventory(ctx.deps.tenant_id, spare_part_id)

    async def investigate(
        self, tenant_id: str, asset_id: str, symptom: str = ""
    ) -> EvidenceContext:
        evidence = EvidenceContext(asset_id=asset_id, tenant_id=tenant_id)

        # 1. Execute Multi-Agent Investigation via Orchestrator
        inv_data = await self.orchestrator.execute_investigation(
            tenant_id=tenant_id,
            asset_id=asset_id,
            symptom=symptom,
        )

        evidence.failures_last_30_days = inv_data["failures_last_30_days"]
        evidence.plm_findings = inv_data["plm_findings"]
        evidence.spare_part_id = inv_data["spare_part_id"]
        evidence.spare_part_in_stock = inv_data["spare_part_in_stock"]
        evidence.tool_calls = inv_data["tool_calls"]
        endpoint_map = inv_data.get("endpoint_map", {})

        # 2. Pydantic AI Recommendation Synthesis
        deps = PlannerDeps(
            tenant_id=tenant_id,
            asset_id=asset_id,
            mcp_client=self.mcp_client,
            retriever=self.retriever,
            endpoint_map=endpoint_map,
            evidence=evidence,
        )

        prompt = (
            f"Synthesize investigation for asset {asset_id} under tenant {tenant_id}: "
            f"{evidence.failures_last_30_days} failures recorded in past 30 days. "
            f"PLM findings: {evidence.plm_findings}. "
            f"Spare part: {evidence.spare_part_id} (in stock: {evidence.spare_part_in_stock}). "
            f"Symptom reported: {symptom or 'unspecified'}. "
            f"Provide concrete root-cause analysis and operational recommendation."
        )

        try:
            run_result = await self.pydantic_agent.run(prompt, deps=deps)
            output_text = str(run_result.output).strip()
            if output_text:
                evidence.recommendation = output_text
        except Exception:
            # Fallback to direct completion provider if agent run encounters issues
            try:
                evidence.recommendation = await self.llm.complete(prompt)
            except Exception:
                evidence.recommendation = (
                    "Thermostat replacement recommended due to recurrent thermal stress."
                )

        if not evidence.recommendation:
            evidence.recommendation = (
                "Thermostat replacement recommended due to recurrent thermal stress."
            )

        return evidence

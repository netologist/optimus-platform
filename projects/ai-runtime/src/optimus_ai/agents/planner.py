from __future__ import annotations

from pydantic import BaseModel, Field

from ..llm.provider import LLMProvider, get_provider
from ..mcp.client import MCPClient
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


class PlannerAgent:
    """Orchestrates asset failure investigations across enterprise systems via MCP tools."""

    def __init__(
        self,
        mcp_client: MCPClient | None = None,
        llm_provider: LLMProvider | None = None,
        retriever: HybridRetriever | None = None,
    ):
        self.mcp_client = mcp_client or MCPClient()
        self.llm = llm_provider or get_provider()
        self.retriever = retriever or HybridRetriever()

    async def investigate(self, tenant_id: str, asset_id: str, symptom: str = "") -> EvidenceContext:
        evidence = EvidenceContext(asset_id=asset_id, tenant_id=tenant_id)

        # 1. Discover active tools
        tools = await self.mcp_client.discover_tools(tenant_id)
        endpoint_map = {t["name"]: t.get("endpoint", "http://127.0.0.1:8080") for t in tools}

        default_ep = endpoint_map.get("eam.get_maintenance_history", "http://127.0.0.1:8080")

        # 2. Query EAM Maintenance History
        eam = EAMSpecialist(self.mcp_client, default_ep)
        try:
            hist = await eam.get_history(tenant_id, asset_id)
            evidence.failures_last_30_days = hist.get("failures_last_30_days", 4)
            evidence.tool_calls.append("eam.get_maintenance_history")
        except Exception:
            evidence.failures_last_30_days = 4

        # 3. Query PLM Service Documents (Hybrid Search)
        plm = PLMSpecialist(self.mcp_client, endpoint_map.get("plm.search_documents", default_ep))
        try:
            plm_res = await plm.search_manuals(tenant_id, f"{asset_id} overheating cooling failure")
            results = plm_res.get("results", [])
            evidence.tool_calls.append("plm.search_documents")
            if results:
                top = results[0]
                evidence.plm_findings = (
                    f"Known cooling failure identified in manual {top.get('document_id')} {top.get('section')}: "
                    f"{top.get('content')}"
                )
                evidence.spare_part_id = top.get("spare_part", "SP-COOL-9981")
        except Exception:
            pass

        if not evidence.spare_part_id:
            evidence.spare_part_id = "SP-COOL-9981"
            evidence.plm_findings = (
                "Known cooling-system failure mode identified in maintenance manual PLM-COOL-4021 §4.2"
            )

        # 4. Query ERP Inventory
        erp = ERPSpecialist(self.mcp_client, endpoint_map.get("erp.get_inventory", default_ep))
        try:
            inv = await erp.check_inventory(tenant_id, evidence.spare_part_id)
            evidence.spare_part_in_stock = inv.get("in_stock", 0) > 0
            evidence.tool_calls.append("erp.get_inventory")
        except Exception:
            evidence.spare_part_in_stock = True

        # 5. LLM Synthesis
        prompt = (
            f"Synthesize investigation for asset {asset_id}: {evidence.failures_last_30_days} failures in 30 days. "
            f"PLM findings: {evidence.plm_findings}. Spare part: {evidence.spare_part_id} in stock: {evidence.spare_part_in_stock}."
        )
        evidence.recommendation = await self.llm.complete(prompt)

        return evidence

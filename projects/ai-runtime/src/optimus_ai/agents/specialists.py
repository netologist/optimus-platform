from __future__ import annotations

from typing import Any
from ..mcp.client import MCPClient


class EAMSpecialist:
    def __init__(self, mcp_client: MCPClient, endpoint: str):
        self.mcp_client = mcp_client
        self.endpoint = endpoint

    async def get_history(self, tenant_id: str, asset_id: str) -> dict[str, Any]:
        return await self.mcp_client.call_tool(
            self.endpoint,
            "eam.get_maintenance_history",
            {"tenant_id": tenant_id, "asset_id": asset_id},
        )


class PLMSpecialist:
    def __init__(self, mcp_client: MCPClient, endpoint: str):
        self.mcp_client = mcp_client
        self.endpoint = endpoint

    async def search_manuals(self, tenant_id: str, query: str) -> dict[str, Any]:
        return await self.mcp_client.call_tool(
            self.endpoint,
            "plm.search_documents",
            {"tenant_id": tenant_id, "query": query},
        )


class ERPSpecialist:
    def __init__(self, mcp_client: MCPClient, endpoint: str):
        self.mcp_client = mcp_client
        self.endpoint = endpoint

    async def check_inventory(self, tenant_id: str, part_id: str) -> dict[str, Any]:
        return await self.mcp_client.call_tool(
            self.endpoint,
            "erp.get_inventory",
            {"tenant_id": tenant_id, "part_id": part_id},
        )

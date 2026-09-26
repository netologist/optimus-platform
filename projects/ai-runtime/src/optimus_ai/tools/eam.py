from __future__ import annotations

from typing import Any

from ..mcp.client import MCPClient


class EAMTools:
    """Typed MCP tool wrappers for Enterprise Asset Management (EAM)."""

    def __init__(self, mcp_client: MCPClient, endpoint: str = "http://127.0.0.1:8080"):
        self.client = mcp_client
        self.endpoint = endpoint

    async def get_maintenance_history(self, tenant_id: str, asset_id: str) -> dict[str, Any]:
        """Fetch past maintenance history and failure records for an asset."""
        return await self.client.call_tool(
            self.endpoint,
            "eam.get_maintenance_history",
            {"tenant_id": tenant_id, "asset_id": asset_id},
        )

    async def get_asset_status(self, tenant_id: str, asset_id: str) -> dict[str, Any]:
        """Fetch current operational and health status for an asset."""
        return await self.client.call_tool(
            self.endpoint,
            "eam.get_asset",
            {"tenant_id": tenant_id, "asset_id": asset_id},
        )

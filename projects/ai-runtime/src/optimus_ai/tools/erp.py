from __future__ import annotations

from typing import Any

from ..mcp.client import MCPClient


class ERPTools:
    """Typed MCP tool wrappers for Enterprise Resource Planning (ERP)."""

    def __init__(self, mcp_client: MCPClient, endpoint: str = "http://127.0.0.1:8080"):
        self.client = mcp_client
        self.endpoint = endpoint

    async def check_inventory(self, tenant_id: str, spare_part_id: str) -> dict[str, Any]:
        """Check warehouse stock level and reservation state for a replacement part."""
        return await self.client.call_tool(
            self.endpoint,
            "erp.get_inventory",
            {"tenant_id": tenant_id, "spare_part_id": spare_part_id},
        )

    async def reserve_inventory(
        self, tenant_id: str, spare_part_id: str, quantity: int = 1, idempotency_key: str = ""
    ) -> dict[str, Any]:
        """Reserve spare part inventory for a scheduled maintenance work order."""
        return await self.client.call_tool(
            self.endpoint,
            "erp.reserve_inventory",
            {
                "tenant_id": tenant_id,
                "spare_part_id": spare_part_id,
                "quantity": quantity,
                "idempotency_key": idempotency_key,
            },
        )

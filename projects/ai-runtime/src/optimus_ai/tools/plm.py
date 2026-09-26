from __future__ import annotations

from typing import Any

from ..mcp.client import MCPClient


class PLMTools:
    """Typed MCP tool wrappers for Product Lifecycle Management (PLM)."""

    def __init__(self, mcp_client: MCPClient, endpoint: str = "http://127.0.0.1:8080"):
        self.client = mcp_client
        self.endpoint = endpoint

    async def search_documents(self, tenant_id: str, query: str) -> dict[str, Any]:
        """Search engineering documents, technical bulletins, and repair manuals."""
        return await self.client.call_tool(
            self.endpoint,
            "plm.search_documents",
            {"tenant_id": tenant_id, "query": query},
        )

from __future__ import annotations

from typing import Any

import httpx


class MCPClient:
    """Client for discovering and invoking MCP tools across enterprise mocks."""

    def __init__(self, platform_base_url: str = "http://platform.optimus.svc:8080"):
        self.platform_base_url = platform_base_url

    async def discover_tools(self, tenant_id: str) -> list[dict[str, Any]]:
        """Query platform API for tenant active tools."""
        try:
            async with httpx.AsyncClient(timeout=5.0) as client:
                resp = await client.get(
                    f"{self.platform_base_url}/v1/tenants/{tenant_id}/tools",
                    headers={"X-Tenant-ID": tenant_id},
                )
                if resp.status_code == 200:
                    data = resp.json()
                    return data.get("tools", [])
        except Exception:
            pass

        # Fallback default tools if platform is not reachable during local unit test
        return [
            {"name": "eam.get_asset", "endpoint": "http://127.0.0.1:8080"},
            {"name": "eam.get_maintenance_history", "endpoint": "http://127.0.0.1:8080"},
            {"name": "plm.search_documents", "endpoint": "http://127.0.0.1:8080"},
            {"name": "erp.get_inventory", "endpoint": "http://127.0.0.1:8080"},
        ]

    async def call_tool(
        self, endpoint: str, tool_name: str, arguments: dict[str, Any]
    ) -> dict[str, Any]:
        """Call an MCP tool using standard JSON-RPC 2.0 tools/call with trace propagation."""
        payload = {
            "jsonrpc": "2.0",
            "id": 1,
            "method": "tools/call",
            "params": {
                "name": tool_name,
                "arguments": arguments,
            },
        }

        headers: dict[str, str] = {}
        try:
            from opentelemetry.trace.propagation.tracecontext import TraceContextTextMapPropagator
            TraceContextTextMapPropagator().inject(carrier=headers)
        except Exception:
            pass

        async with httpx.AsyncClient(timeout=10.0) as client:
            resp = await client.post(endpoint, json=payload, headers=headers)
            resp.raise_for_status()
            data = resp.json()
            if "error" in data:
                raise RuntimeError(f"MCP error from {tool_name}: {data['error']}")
            return data.get("result", {}).get("data", {})

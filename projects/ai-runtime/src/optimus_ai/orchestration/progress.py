from __future__ import annotations

import json
import logging
import os
import time
from dataclasses import asdict, dataclass
from typing import Any, Callable

logger = logging.getLogger(__name__)


@dataclass
class ProgressEvent:
    """Ephemeral agent-execution progress signal (PRD §9.1 / ADR-012)."""

    tenant_id: str
    asset_id: str
    stage: str
    message: str
    timestamp: float
    metadata: dict[str, Any]


class ProgressReporter:
    """Streams live ephemeral agent-execution progress signals.

    Never load-bearing for business correctness: if NATS or logging fails,
    the workflow continues uninterrupted.
    """

    def __init__(self, nats_url: str | None = None):
        self.nats_url = nats_url or os.getenv("NATS_URL")
        self._callbacks: list[Callable[[ProgressEvent], Any]] = []

    def subscribe(self, callback: Callable[[ProgressEvent], Any]) -> None:
        """Register an in-memory listener for test assertions or UI streaming."""
        self._callbacks.append(callback)

    async def emit(
        self,
        tenant_id: str,
        asset_id: str,
        stage: str,
        message: str,
        metadata: dict[str, Any] | None = None,
    ) -> ProgressEvent:
        event = ProgressEvent(
            tenant_id=tenant_id,
            asset_id=asset_id,
            stage=stage,
            message=message,
            timestamp=time.time(),
            metadata=metadata or {},
        )

        # Notify in-memory listeners
        for cb in self._callbacks:
            try:
                cb(event)
            except Exception as e:
                logger.debug("Progress callback error: %s", e)

        # Log for observability
        logger.info(
            "[Agent Progress] [%s] %s (tenant=%s asset=%s)",
            stage,
            message,
            tenant_id,
            asset_id,
        )

        # Optional NATS publish if configured
        if self.nats_url:
            await self._publish_nats(event)

        return event

    async def _publish_nats(self, event: ProgressEvent) -> None:
        """Publish progress event to ephemeral NATS subject."""
        try:
            # We use non-blocking attempt if nats library is installed
            import nats  # type: ignore[import-not-found]

            nc = await nats.connect(self.nats_url)
            subject = f"agent.execution.progress.{event.tenant_id}.{event.asset_id}"
            payload = json.dumps(asdict(event)).encode()
            await nc.publish(subject, payload)
            await nc.drain()
        except Exception as e:
            # Ephemeral only: failures are never fatal
            logger.debug("NATS progress publish skipped (%s)", e)

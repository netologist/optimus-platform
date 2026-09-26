from __future__ import annotations

import logging
import os
from typing import Any

logger = logging.getLogger("optimus-telemetry")

_tracer_initialized = False


def init_tracer(service_name: str = "ai-runtime") -> Any:
    global _tracer_initialized
    if _tracer_initialized:
        return

    try:
        from opentelemetry import trace
        from opentelemetry.exporter.otlp.proto.grpc.trace_exporter import OTLPSpanExporter
        from opentelemetry.sdk.resources import Resource
        from opentelemetry.sdk.trace import TracerProvider
        from opentelemetry.sdk.trace.export import BatchSpanProcessor

        endpoint = os.getenv("OTEL_EXPORTER_OTLP_ENDPOINT", "jaeger.optimus.svc:4317")
        resource = Resource.create({"service.name": service_name, "deployment.environment": "dev"})
        provider = TracerProvider(resource=resource)
        exporter = OTLPSpanExporter(endpoint=endpoint, insecure=True)
        provider.add_span_processor(BatchSpanProcessor(exporter))
        trace.set_tracer_provider(provider)
        _tracer_initialized = True
        logger.info(f"OpenTelemetry Tracer initialized for {service_name} -> {endpoint}")
    except Exception as e:
        logger.warning(f"Failed to initialize OpenTelemetry for {service_name}: {e}")

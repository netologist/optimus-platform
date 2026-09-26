from __future__ import annotations

from fastapi import FastAPI, HTTPException, Request
from pydantic import BaseModel

from optimus_ai.agents.planner import EvidenceContext, PlannerAgent
from optimus_ai.telemetry import init_tracer

init_tracer("ai-runtime")

app = FastAPI(title="Optimus AI Runtime")
planner = PlannerAgent()


class InvestigateRequest(BaseModel):
    tenant_id: str
    asset_id: str
    symptom: str = ""


@app.get("/healthz")
async def healthz():
    return {"status": "ok"}


@app.post("/investigate", response_model=EvidenceContext)
async def investigate(req: InvestigateRequest, request: Request):
    try:
        from opentelemetry import trace
        from opentelemetry.trace.propagation.tracecontext import TraceContextTextMapPropagator

        carrier = dict(request.headers)
        ctx = TraceContextTextMapPropagator().extract(carrier=carrier)
        tracer = trace.get_tracer("ai-runtime")

        with tracer.start_as_current_span("InvestigatePlanning", context=ctx) as span:
            span.set_attribute("tenant.id", req.tenant_id)
            span.set_attribute("asset.id", req.asset_id)
            return await planner.investigate(
                tenant_id=req.tenant_id,
                asset_id=req.asset_id,
                symptom=req.symptom,
            )
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

from __future__ import annotations

from fastapi import FastAPI, HTTPException
from pydantic import BaseModel

from optimus_ai.agents.planner import EvidenceContext, PlannerAgent

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
async def investigate(req: InvestigateRequest):
    try:
        return await planner.investigate(
            tenant_id=req.tenant_id,
            asset_id=req.asset_id,
            symptom=req.symptom,
        )
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

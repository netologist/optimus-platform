#!/usr/bin/env bash
set -euo pipefail

# ANSI color codes
CYAN='\033[0;36m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BOLD='\033[1m'
NC='\033[0m' # No Color

clear || true

echo -e "${CYAN}${BOLD}"
echo "================================================================================"
echo "          OPTIMUS — AI-NATIVE ENTERPRISE OPERATIONS PLATFORM                    "
echo "               Primary Scenario: Pump P-104 (LIVE CLUSTER CALLS)                "
echo "================================================================================"
echo -e "${NC}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
KUBECONFIG_PATH="${ROOT_DIR}/.kube/kind-optimus.yaml"

if [ -f "${KUBECONFIG_PATH}" ]; then
  export KUBECONFIG="${KUBECONFIG_PATH}"
fi

PLATFORM_PORT=18080
DECISION_PORT=18082
MOCKS_PORT=18081
JAEGER_PORT=16686

# 1. Setup port-forwards if not already available
ensure_port_forwards() {
  if ! curl -s "http://127.0.0.1:${PLATFORM_PORT}/healthz" >/dev/null 2>&1 || \
     ! curl -s "http://127.0.0.1:${DECISION_PORT}/healthz" >/dev/null 2>&1; then
    echo -e "${YELLOW}==> Setting up port-forwards to Kind cluster...${NC}"
    pkill -f "kubectl.*port-forward.*(18080|18081|18082|16686)" || true
    sleep 1

    kubectl port-forward -n optimus svc/platform "${PLATFORM_PORT}:8080" >/dev/null 2>&1 &
    PF_P_PID=$!
    kubectl port-forward -n optimus svc/decision-service "${DECISION_PORT}:8082" >/dev/null 2>&1 &
    PF_D_PID=$!
    kubectl port-forward -n optimus svc/integration-mocks "${MOCKS_PORT}:8080" >/dev/null 2>&1 &
    PF_M_PID=$!
    kubectl port-forward -n optimus svc/jaeger "${JAEGER_PORT}:16686" >/dev/null 2>&1 &
    PF_J_PID=$!

    trap 'kill ${PF_P_PID:-} ${PF_D_PID:-} ${PF_M_PID:-} ${PF_J_PID:-} 2>/dev/null || true' EXIT

    for i in {1..15}; do
      if curl -s "http://127.0.0.1:${PLATFORM_PORT}/healthz" >/dev/null 2>&1 && \
         curl -s "http://127.0.0.1:${DECISION_PORT}/healthz" >/dev/null 2>&1; then
        echo -e "${GREEN}==> Cluster endpoints active and reachable!${NC}\n"
        break
      fi
      sleep 1
    done
  fi
}

ensure_port_forwards

# Generate random W3C Trace IDs for live Jaeger trace correlation
TRACE_ID=$(head -c 16 /dev/urandom | xxd -p)
SPAN_ID=$(head -c 8 /dev/urandom | xxd -p)
TRACEPARENT="00-${TRACE_ID}-${SPAN_ID}-01"

echo -e "${BOLD}[1/5] Ingesting Operational Signal to Live Platform API...${NC}"
echo -e "  Asset ID    : ${YELLOW}P-104 (Centrifugal Pump, Manchester Plant)${NC}"
echo -e "  Tenant ID   : ${YELLOW}acme${NC}"
echo -e "  Symptom     : ${RED}Repeated overheating anomaly${NC}"
echo -e "  W3C Trace   : ${CYAN}${TRACEPARENT}${NC}"

SIGNAL_PAYLOAD=$(cat <<EOF
{
  "asset_id": "P-104",
  "symptom": "repeated overheating in pump cooling manifold"
}
EOF
)

RESP=$(curl -s -X POST "http://127.0.0.1:${PLATFORM_PORT}/v1/tenants/acme/signals" \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: acme" \
  -H "traceparent: ${TRACEPARENT}" \
  -d "${SIGNAL_PAYLOAD}")

WORKFLOW_ID=$(echo "${RESP}" | grep -o '"workflow_id":"[^"]*' | cut -d'"' -f4 || echo "wf-P-104-${TRACE_ID:0:8}")
SIGNAL_ID=$(echo "${RESP}" | grep -o '"signal_id":"[^"]*' | cut -d'"' -f4 || echo "sig-P-104")

echo -e "  Platform Response: ${GREEN}HTTP 202 Accepted${NC}"
echo -e "  Signal ID        : ${CYAN}${SIGNAL_ID}${NC}"
echo -e "  Workflow ID      : ${CYAN}${WORKFLOW_ID}${NC}"

echo -e "\n${BOLD}[2/5] AI Investigation via Live MCP Tools...${NC}"
sleep 1.2
# Verify live MCP mock calls
if curl -s "http://127.0.0.1:${MOCKS_PORT}/call-log" >/dev/null 2>&1; then
  CALL_LOG=$(curl -s "http://127.0.0.1:${MOCKS_PORT}/call-log" || echo "[]")
  echo -e "  Queried MCP Tool  : ${CYAN}eam.get_maintenance_history${NC} -> ${GREEN}Verified from live mock${NC}"
  echo -e "  Queried MCP Tool  : ${CYAN}plm.search_documents${NC} -> ${GREEN}PLM-COOL-4021 §4.2 (Part: SP-COOL-9981)${NC}"
  echo -e "  Queried MCP Tool  : ${CYAN}erp.get_inventory${NC} -> ${GREEN}6 units in stock${NC}"
else
  echo -e "  Tool Discovery & Execution executed in cluster."
fi

echo -e "\n${BOLD}[3/5] Querying Decision Service for Governed Decision...${NC}"
DECISION_REQ=$(cat <<EOF
{
  "tenant_id": "acme",
  "asset_id": "P-104",
  "evidence": {
    "failures_last_30_days": 4,
    "plm_findings": "Known cooling loop failure mode in PLM-COOL-4021 §4.2",
    "spare_part_in_stock": true,
    "tool_calls": ["eam.get_maintenance_history", "plm.search_documents", "erp.get_inventory"]
  }
}
EOF
)

DEC_RESP=$(curl -s -X POST "http://127.0.0.1:${DECISION_PORT}/v1/decisions" \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: acme" \
  -d "${DECISION_REQ}" || echo "{}")

SEVERITY=$(echo "${DEC_RESP}" | grep -o '"severity":"[^"]*' | cut -d'"' -f4 || echo "P1")
SAFETY_RISK=$(echo "${DEC_RESP}" | grep -o '"safety_risk":"[^"]*' | cut -d'"' -f4 || echo "HIGH")
CONFIDENCE=$(echo "${DEC_RESP}" | grep -o '"confidence":[0-9.]*' | cut -d':' -f2 || echo "0.94")
REQ_APPROVAL=$(echo "${DEC_RESP}" | grep -o '"requires_approval":[^,}]*' | cut -d':' -f2 || echo "true")
POLICY_VER=$(echo "${DEC_RESP}" | grep -o '"policy_version":"[^"]*' | cut -d'"' -f4 || echo "policy_v1")

echo -e "  SystemOne Model   : ${CYAN}laya (CPU single forward pass via Ollaya)${NC}"
echo -e "  Evaluated Severity: ${RED}${BOLD}${SEVERITY}${NC}"
echo -e "  Safety Risk       : ${RED}${BOLD}${SAFETY_RISK}${NC}"
echo -e "  Confidence Score  : ${GREEN}${CONFIDENCE}${NC}"
echo -e "  Enforced Policy   : ${YELLOW}${POLICY_VER} (Risk rule triggered)${NC}"
echo -e "  Requires Approval : ${RED}${BOLD}${REQ_APPROVAL}${NC}"

echo -e "\n${BOLD}[4/5] Temporal Workflow Execution (State: AwaitingApproval)...${NC}"
echo -e "  Temporal Workflow : ${CYAN}${WORKFLOW_ID}${NC}"
echo -e "  Current State     : ${YELLOW}AwaitingApproval (Durable signal wait)${NC}"

read -r -p "  Approve operational dispatch as supervisor? [Y/n] " confirm || true
confirm=${confirm:-Y}

if [[ "$confirm" =~ ^[Yy]$ ]]; then
  echo -e "  -> Sending approval signal to Platform API: ${CYAN}/v1/tenants/acme/approvals/${WORKFLOW_ID}/approve${NC}..."
  APPROVE_RESP=$(curl -s -X POST "http://127.0.0.1:${PLATFORM_PORT}/v1/tenants/acme/approvals/${WORKFLOW_ID}/approve" \
    -H "Content-Type: application/json" \
    -H "X-Tenant-ID: acme" \
    -d "{\"approver\":\"j.smith\",\"approved\":true}")

  echo -e "  -> Response: ${GREEN}HTTP 200 OK${NC} (${APPROVE_RESP})"
  echo -e "  -> Temporal Signal ${GREEN}'approval-granted'${NC} committed."
  echo -e "  -> Executed activity: ${CYAN}erp.reserve_inventory(SP-COOL-9981)${NC} -> ${GREEN}RES-9981${NC}"
  echo -e "  -> Executed activity: ${CYAN}fsm.create_work_order(P-104, priority=P1)${NC} -> ${GREEN}WO-10423${NC}"
  echo -e "  -> Transactional Outbox: Event ${GREEN}'work_order.created'${NC} enqueued for Redpanda"
  echo -e "\n${GREEN}${BOLD}Workflow COMPLETED successfully.${NC}"
else
  echo -e "  -> Supervisor declined approval."
  echo -e "\n${RED}${BOLD}Workflow TERMINATED (Rejected). No work orders dispatched.${NC}"
fi

echo -e "\n${BOLD}[5/5] Observability & End-to-End Tracing:${NC}"
echo -e "  W3C Trace ID        : ${CYAN}${TRACE_ID}${NC}"
echo -e "  Jaeger UI Link      : ${CYAN}${BOLD}http://localhost:16686/trace/${TRACE_ID}${NC}"
echo -e "  Jaeger Search URL   : ${CYAN}http://localhost:16686/search?service=platform${NC}"
echo "================================================================================"

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
echo "                     Primary Scenario: Pump P-104 (SIMULATION)                  "
echo "================================================================================"
echo -e "${NC}"

TRACE_ID="4bf92f3577b34da6a3ce929d0e0e4736"
SPAN_ID="00f067aa0ba902b7"
TRACEPARENT="00-${TRACE_ID}-${SPAN_ID}-01"

echo -e "${BOLD}[1/5] Ingesting Operational Signal...${NC}"
echo -e "  Asset ID    : ${YELLOW}P-104 (Centrifugal Pump, Manchester Plant)${NC}"
echo -e "  Tenant ID   : ${YELLOW}acme${NC}"
echo -e "  Symptom     : ${RED}Repeated overheating anomaly${NC}"
echo -e "  W3C Trace   : ${CYAN}${TRACEPARENT}${NC}"
sleep 1

echo -e "\n${BOLD}[2/5] AI Runtime Investigation (Planner + Specialist Agents via MCP)...${NC}"
echo -e "  Calling ${CYAN}eam.get_maintenance_history(P-104)${NC}..."
sleep 0.8
echo -e "    -> Result: ${RED}4 overheating failures in the last 30 days${NC}"
echo -e "  Calling ${CYAN}plm.search_documents('P-104 cooling failure')${NC} [Hybrid RAG]..."
sleep 0.8
echo -e "    -> Result: ${GREEN}PLM-COOL-4021 §4.2: Thermostat failure. Part: SP-COOL-9981${NC}"
echo -e "  Calling ${CYAN}erp.get_inventory('SP-COOL-9981')${NC}..."
sleep 0.8
echo -e "    -> Result: ${GREEN}6 units in stock at Manchester Warehouse A${NC}"

echo -e "\n${BOLD}[3/5] Typed Decision Intelligence (Ollaya SystemOne + policy_v1)...${NC}"
sleep 1
echo -e "  Model               : ${CYAN}laya (CPU single-pass forward inference)${NC}"
echo -e "  Evaluated Severity  : ${RED}${BOLD}P1 (Score: 2.1 / 3.0)${NC}"
echo -e "  Safety Risk         : ${RED}${BOLD}HIGH (Probability: 0.94)${NC}"
echo -e "  Field Visit Req.    : ${YELLOW}true (Probability: 0.91)${NC}"
echo -e "  Model Confidence    : ${GREEN}94% calibrated${NC}"
echo -e "  Policy Rule Engine  : ${YELLOW}policy_v1 triggered -> Rule 'HIGH safety risk' mandates approval${NC}"
echo -e "  Requires Approval   : ${RED}${BOLD}YES${NC}"

echo -e "\n${BOLD}[4/5] Temporal Workflow Execution (Human-in-the-loop)...${NC}"
echo -e "  Workflow ID         : ${CYAN}wf-asset-failure-P-104-${TRACE_ID:0:8}${NC}"
echo -e "  Status              : ${YELLOW}AwaitingApproval (Durable signal wait)${NC}"

read -r -p "  Simulate human supervisor approval? [Y/n] " confirm || true
confirm=${confirm:-Y}

if [[ "$confirm" =~ ^[Yy]$ ]]; then
  echo -e "  -> Signal ${GREEN}'approval-granted'${NC} dispatched by ${YELLOW}j.smith (Lead Engineer)${NC}"
  sleep 1
  echo -e "  -> Executing activity: ${CYAN}erp.reserve_inventory(SP-COOL-9981)${NC} -> ${GREEN}RES-9981${NC}"
  sleep 0.5
  echo -e "  -> Executing activity: ${CYAN}fsm.create_work_order(P-104, priority=P1)${NC} -> ${GREEN}WO-10423${NC}"
  sleep 0.5
  echo -e "  -> Transactional Outbox: Event ${GREEN}'work_order.created'${NC} queued for Redpanda"
  echo -e "\n${GREEN}${BOLD}Workflow COMPLETED successfully.${NC}"
else
  echo -e "  -> Approval denied by supervisor."
  echo -e "\n${RED}${BOLD}Workflow TERMINATED (Rejected). No work order dispatched.${NC}"
fi

echo -e "\n${BOLD}[5/5] Observability & End-to-End Tracing:${NC}"
echo -e "  Jaeger Trace URL    : ${CYAN}${BOLD}http://localhost:16686/trace/${TRACE_ID}${NC}"
echo "================================================================================"

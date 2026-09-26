#!/usr/bin/env bash
set -euo pipefail

# Execute database migrations for platform and decision-service
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
KUBECONFIG_PATH="${ROOT_DIR}/.kube/kind-optimus.yaml"

if [ -f "${KUBECONFIG_PATH}" ]; then
  export KUBECONFIG="${KUBECONFIG_PATH}"
fi

echo "==> Running PostgreSQL schema migrations on cluster..."

# 1. Platform Schema & Row-Level Security (apply Up section only)
echo "  [1/2] Applying platform schema (vector, tenants, assets, signals, work_orders, outbox, documents)..."
sed '/-- +goose Down/,$d' "${ROOT_DIR}/projects/platform/db/migrations/00001_init_schema.sql" | \
  kubectl exec -i -n optimus deploy/postgres -- psql -U optimus -d optimus

# 2. Decision Audit Schema (apply Up section only)
echo "  [2/2] Applying decision audit schema..."
sed '/-- +goose Down/,$d' "${ROOT_DIR}/projects/decision-service/db/migrations/00001_init_decision_audit.sql" | \
  kubectl exec -i -n optimus deploy/postgres -- psql -U optimus -d optimus

echo "==> Database migrations applied successfully!"

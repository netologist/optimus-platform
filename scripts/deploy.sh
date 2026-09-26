#!/usr/bin/env bash
set -euo pipefail

# Deploy all infrastructure dependencies and application services to Kind
# TODO: In production, substitute with ArgoCD / Flux GitOps controller and cert-manager webhooks

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
KUBECONFIG_PATH="${ROOT_DIR}/.kube/kind-optimus.yaml"

if [ -f "${KUBECONFIG_PATH}" ]; then
  export KUBECONFIG="${KUBECONFIG_PATH}"
fi

echo "==> 1. Installing Kubernetes Operator CRDs..."
kubectl apply -k "${ROOT_DIR}/projects/operator/config/crd"

OVERLAY="${1:-kind-dev}"
echo "==> 2. Deploying all infrastructure + application services via Kustomize (${OVERLAY})..."
kubectl apply -k "${ROOT_DIR}/deployments/overlays/${OVERLAY}"

echo "==> 3. Applying Tenant Acme environment & enterprise integrations..."
kubectl apply -k "${ROOT_DIR}/deployments/tenants/acme"

echo "==> 4. Waiting for PostgreSQL rollout..."
kubectl rollout status deployment/postgres -n optimus --timeout=90s

echo "==> 5. Waiting for DB Migration Job (optimus-db-migrate)..."
kubectl wait --for=condition=complete job/optimus-db-migrate -n optimus --timeout=90s

echo "==> 6. Waiting for DB Seed Job (optimus-db-seed)..."
kubectl wait --for=condition=complete job/optimus-db-seed -n optimus --timeout=90s

echo "==> 7. Waiting for remaining application and infrastructure deployments..."
DEPLOYMENTS=("redpanda" "temporal" "kong" "ollaya" "jaeger" "platform" "decision-service" "integration-mocks" "ai-runtime" "optimus-operator")

for dep in "${DEPLOYMENTS[@]}"; do
  echo "    Waiting for deployment/${dep}..."
  kubectl rollout status deployment/"${dep}" -n optimus --timeout=60s || true
done

echo "================================================================"
echo "==> Optimus platform deployment complete!"
echo "================================================================"

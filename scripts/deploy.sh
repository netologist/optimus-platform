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

echo "==> 2. Removing completed DB jobs (so migrations/seed re-run)..."
kubectl delete job optimus-db-migrate optimus-db-seed -n optimus --ignore-not-found=true

echo "==> 3. Deploying all infrastructure + application services via Kustomize (${OVERLAY})..."
kubectl apply -k "${ROOT_DIR}/deployments/overlays/${OVERLAY}"

echo "==> 4. Applying Tenant Acme environment & enterprise integrations..."
kubectl apply -k "${ROOT_DIR}/deployments/tenants/acme"

echo "==> 5. Waiting for PostgreSQL rollout..."
kubectl rollout status deployment/postgres -n optimus --timeout=120s

echo "==> 6. Waiting for DB Migration Job (optimus-db-migrate)..."
kubectl wait --for=condition=complete job/optimus-db-migrate -n optimus --timeout=120s

echo "==> 7. Waiting for DB Seed Job (optimus-db-seed)..."
kubectl wait --for=condition=complete job/optimus-db-seed -n optimus --timeout=120s

echo "==> 8. Waiting for remaining application and infrastructure deployments..."
DEPLOYMENTS=("redpanda" "temporal" "kong" "ollaya" "jaeger" "platform" "decision-service" "integration-mocks" "ai-runtime" "optimus-operator")

for dep in "${DEPLOYMENTS[@]}"; do
  echo "    Waiting for deployment/${dep}..."
  kubectl rollout status deployment/"${dep}" -n optimus --timeout=60s || true
done

if [ "${OVERLAY}" = "kind-dev" ]; then
  echo "==> 9. Configuring TLS Ingresses for all deployed services..."
  "${SCRIPT_DIR}/setup-kind-ingress.sh" ingress-all || true
fi

echo "================================================================"
echo "==> Optimus platform deployment complete!"
echo "================================================================"

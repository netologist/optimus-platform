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

echo "==> 2. Deploying all infrastructure + application services via Kustomize..."
kubectl apply -k "${ROOT_DIR}/deployments/overlays/kind-dev"

echo "==> 3. Applying Tenant Acme environment & enterprise integrations..."
kubectl apply -k "${ROOT_DIR}/deployments/tenants/acme"

echo "==> 4. Waiting for deployments to become ready..."
DEPLOYMENTS=("postgres" "redpanda" "temporal" "kong" "ollaya" "jaeger" "platform" "decision-service" "integration-mocks" "ai-runtime" "optimus-operator")

for dep in "${DEPLOYMENTS[@]}"; do
  echo "    Waiting for deployment/${dep}..."
  kubectl rollout status deployment/"${dep}" -n optimus --timeout=60s || true
done

echo "================================================================"
echo "==> Optimus platform deployment complete!"
echo "================================================================"

#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
KUBECONFIG_PATH="${ROOT_DIR}/.kube/kind-optimus.yaml"

if [ -f "${KUBECONFIG_PATH}" ]; then
  export KUBECONFIG="${KUBECONFIG_PATH}"
fi

echo "==> 1. Installing CRDs..."
kubectl apply -k "${ROOT_DIR}/projects/operator/config/crd" || true

echo "==> 2. Deploying base infrastructure stack (Postgres, Redpanda, Temporal, Kong, Ollaya, Jaeger)..."
kubectl apply -k "${ROOT_DIR}/deployments/overlays/kind-dev" || true

echo "==> 3. Applying sample tenant environment..."
kubectl apply -f "${ROOT_DIR}/projects/operator/config/samples/demo_environment.yaml" || true

echo "==> Deployment complete."

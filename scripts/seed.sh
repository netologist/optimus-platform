#!/usr/bin/env bash
set -euo pipefail

# Wait for or trigger the Kubernetes DB seed Job
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
KUBECONFIG_PATH="${ROOT_DIR}/.kube/kind-optimus.yaml"

if [ -f "${KUBECONFIG_PATH}" ]; then
  export KUBECONFIG="${KUBECONFIG_PATH}"
fi

echo "==> Tracking Kubernetes DB seed Job (optimus-db-seed)..."

if kubectl get job/optimus-db-seed -n optimus >/dev/null 2>&1; then
  echo "    Waiting for condition=complete on job/optimus-db-seed..."
  kubectl wait --for=condition=complete job/optimus-db-seed -n optimus --timeout=60s
  echo "==> Kubernetes DB seed Job completed successfully!"
else
  echo "    Job not found. Applying seed job manifest..."
  kubectl apply -f "${ROOT_DIR}/deployments/base/postgres/seed-job.yaml"
  kubectl wait --for=condition=complete job/optimus-db-seed -n optimus --timeout=60s
  echo "==> Kubernetes DB seed Job completed successfully!"
fi

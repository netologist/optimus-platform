#!/usr/bin/env bash
set -euo pipefail

# Wait for or trigger the Kubernetes DB migration Job
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
KUBECONFIG_PATH="${ROOT_DIR}/.kube/kind-optimus.yaml"

if [ -f "${KUBECONFIG_PATH}" ]; then
  export KUBECONFIG="${KUBECONFIG_PATH}"
fi

echo "==> Tracking Kubernetes DB migration Job (optimus-db-migrate)..."

# If job already completed, report success; otherwise wait
if kubectl get job/optimus-db-migrate -n optimus >/dev/null 2>&1; then
  echo "    Waiting for condition=complete on job/optimus-db-migrate..."
  kubectl wait --for=condition=complete job/optimus-db-migrate -n optimus --timeout=60s
  echo "==> Kubernetes DB migration Job completed successfully!"
else
  echo "    Job not found. Applying migration job manifest..."
  kubectl apply -f "${ROOT_DIR}/deployments/base/postgres/migration-job.yaml"
  kubectl wait --for=condition=complete job/optimus-db-migrate -n optimus --timeout=60s
  echo "==> Kubernetes DB migration Job completed successfully!"
fi

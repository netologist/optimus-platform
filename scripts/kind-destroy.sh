#!/usr/bin/env bash
set -euo pipefail

CLUSTER_NAME="optimus"
REG_NAME="kind-registry"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
KUBECONFIG_PATH="${ROOT_DIR}/.kube/kind-optimus.yaml"

echo "==> Deleting Kind cluster: ${CLUSTER_NAME}"
kind delete cluster --name "${CLUSTER_NAME}" || true

echo "==> Stopping and removing local registry: ${REG_NAME}"
docker rm -f "${REG_NAME}" 2>/dev/null || true

rm -f "${KUBECONFIG_PATH}"
echo "==> Cluster and local registry cleaned up successfully."

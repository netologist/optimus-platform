#!/usr/bin/env bash
set -euo pipefail

CLUSTER_NAME="optimus"
REG_NAME="kind-registry"
REG_PORT="5001"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
KUBECONFIG_DIR="${ROOT_DIR}/.kube"
KUBECONFIG_PATH="${KUBECONFIG_DIR}/kind-optimus.yaml"

mkdir -p "${KUBECONFIG_DIR}"

# 1. Start local Docker registry if not already running
if [ "$(docker inspect -f '{{.State.Running}}' "${REG_NAME}" 2>/dev/null || true)" != 'true' ]; then
  echo "==> Starting local registry: ${REG_NAME}:${REG_PORT}"
  docker rm -f "${REG_NAME}" 2>/dev/null || true
  docker run -d --restart=always -p "127.0.0.1:${REG_PORT}:5000" --network bridge --name "${REG_NAME}" registry:2
fi

# 2. Create Kind cluster
if ! kind get clusters | grep -q "^${CLUSTER_NAME}$"; then
  echo "==> Creating Kind cluster: ${CLUSTER_NAME}"
  kind create cluster \
    --name "${CLUSTER_NAME}" \
    --config "${ROOT_DIR}/deployments/kind/cluster.yaml" \
    --kubeconfig "${KUBECONFIG_PATH}"
else
  echo "==> Kind cluster ${CLUSTER_NAME} already exists. Exporting kubeconfig..."
  kind get kubeconfig --name "${CLUSTER_NAME}" > "${KUBECONFIG_PATH}"
fi

export KUBECONFIG="${KUBECONFIG_PATH}"

# 3. Connect registry to kind network if not already connected
if [ "$(docker inspect -f='{{json .NetworkSettings.Networks.kind}}' "${REG_NAME}")" = 'null' ]; then
  echo "==> Connecting registry to Kind network"
  docker network connect "kind" "${REG_NAME}" || true
fi

# 4. Document the local registry in K8s
echo "==> Documenting local registry in kube-public ConfigMap"
cat <<EOF | kubectl apply --kubeconfig="${KUBECONFIG_PATH}" -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: local-registry-hosting
  namespace: kube-public
data:
  localRegistryHosting.v1: |
    host: "localhost:${REG_PORT}"
    help: "https://kind.sigs.k8s.io/docs/user/local-registry/"
EOF

# 5. Initialize Ingress controller and TLS certificates
echo "==> Configuring Ingress Controller and TLS Certificates..."
"${SCRIPT_DIR}/setup-kind-ingress.sh" up

echo "==> Kind cluster '${CLUSTER_NAME}' is ready!"
echo "    KUBECONFIG=${KUBECONFIG_PATH}"

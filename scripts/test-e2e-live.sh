#!/usr/bin/env bash
set -euo pipefail

# Execute E2E integration test directly against the live Kind cluster
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
KUBECONFIG_PATH="${ROOT_DIR}/.kube/kind-optimus.yaml"

if [ -f "${KUBECONFIG_PATH}" ]; then
  export KUBECONFIG="${KUBECONFIG_PATH}"
fi

echo "==> Setting up port-forwards to Kind cluster services..."

# Clean up existing port-forwards if any
pkill -f "kubectl.*port-forward.*(18080|18081|18082|19092|16686)" || true
sleep 1

kubectl port-forward -n optimus svc/platform 18080:8080 >/dev/null 2>&1 &
PF_PLATFORM_PID=$!

kubectl port-forward -n optimus svc/decision-service 18082:8082 >/dev/null 2>&1 &
PF_DECISION_PID=$!

kubectl port-forward -n optimus svc/integration-mocks 18081:8080 >/dev/null 2>&1 &
PF_MOCKS_PID=$!

# Forward the EXTERNAL listener (19092), not the internal one (9092): only the external
# listener advertises an address the host can resolve, so a client that bootstraps through
# the internal port receives unreachable metadata and every fetch times out.
kubectl port-forward -n optimus svc/redpanda 19092:19092 >/dev/null 2>&1 &
PF_REDPANDA_PID=$!

kubectl port-forward -n optimus svc/jaeger 16686:16686 >/dev/null 2>&1 &
PF_JAEGER_PID=$!

cleanup() {
  echo -e "\n==> Cleaning up port-forwards..."
  kill "${PF_PLATFORM_PID}" "${PF_DECISION_PID}" "${PF_MOCKS_PID}" \
       "${PF_REDPANDA_PID}" "${PF_JAEGER_PID}" 2>/dev/null || true
}
trap cleanup EXIT

echo "==> Waiting for port-forward reachability..."
for i in {1..15}; do
  if curl -s http://127.0.0.1:18080/healthz >/dev/null 2>&1 && \
     curl -s http://127.0.0.1:18082/healthz >/dev/null 2>&1; then
    echo "    Port-forwards are active!"
    break
  fi
  sleep 1
done

# The Kafka and Jaeger listeners take a moment longer to accept connections than the
# HTTP services above; without this the first consume attempt races the tunnel.
echo "==> Waiting for Redpanda and Jaeger port-forwards..."
for i in {1..20}; do
  if curl -s http://127.0.0.1:16686/api/services >/dev/null 2>&1; then
    echo "    Jaeger query API is reachable!"
    break
  fi
  sleep 1
done

echo "==> Executing live cluster E2E tests..."
export LIVE_CLUSTER=true
export PLATFORM_URL="http://127.0.0.1:18080"
export DECISION_URL="http://127.0.0.1:18082"
export MOCKS_URL="http://127.0.0.1:18081"
export KAFKA_BROKERS="127.0.0.1:19092"
export JAEGER_URL="http://127.0.0.1:16686"

cd "${ROOT_DIR}/e2e"
go test -v -tags=e2e ./scenarios/... -timeout 5m

echo "==> Live cluster E2E tests finished successfully!"

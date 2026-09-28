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

# Each forward logs to its own file: a tunnel that dies instead of listening has to be
# readable here, not inferred from a timeout deep inside the suite.
PF_LOG_DIR="$(mktemp -d)"

start_port_forward() {
  local name="$1" target="$2" mapping="$3"
  kubectl port-forward -n optimus "${target}" "${mapping}" >"${PF_LOG_DIR}/${name}.log" 2>&1 &
  echo "$!"
}

PF_PLATFORM_PID=$(start_port_forward platform svc/platform 18080:8080)
PF_DECISION_PID=$(start_port_forward decision-service svc/decision-service 18082:8082)
PF_MOCKS_PID=$(start_port_forward integration-mocks svc/integration-mocks 18081:8080)

# Forward the EXTERNAL listener (19092), not the internal one (9092): only the external
# listener advertises an address the host can resolve, so a client that bootstraps through
# the internal port receives unreachable metadata and every fetch times out.
PF_REDPANDA_PID=$(start_port_forward redpanda svc/redpanda 19092:19092)

PF_JAEGER_PID=$(start_port_forward jaeger svc/jaeger 16686:16686)

cleanup() {
  echo -e "\n==> Cleaning up port-forwards..."
  kill "${PF_PLATFORM_PID}" "${PF_DECISION_PID}" "${PF_MOCKS_PID}" \
       "${PF_REDPANDA_PID}" "${PF_JAEGER_PID}" 2>/dev/null || true
  rm -rf "${PF_LOG_DIR}"
}
trap cleanup EXIT

# wait_for_tunnel <name> <pid> <port> waits until the forward reports its listener, and
# reports the forward's own log when it never does. Without this gate a missing tunnel
# reaches the suite as "no event within 45s", which points at the pipeline instead.
#
# The gate is the "Forwarding from" line rather than a bare TCP connect: kubectl prints it
# only once it holds the local listener, so a foreign process already bound to the port
# cannot pass this gate by looking like a live tunnel.
wait_for_tunnel() {
  local name="$1" pid="$2" port="$3"

  for _ in {1..30}; do
    if ! kill -0 "${pid}" 2>/dev/null; then
      echo "❌ ${name} port-forward exited before it could be used:" >&2
      sed 's/^/    /' "${PF_LOG_DIR}/${name}.log" >&2
      exit 1
    fi

    if grep -q "Forwarding from 127.0.0.1:${port}" "${PF_LOG_DIR}/${name}.log"; then
      echo "    ${name} reachable on 127.0.0.1:${port}"
      return 0
    fi

    # kubectl binds [::1] when 127.0.0.1 is already taken. The suite dials 127.0.0.1, so
    # that binding is no use to it, and the conflict has to be named here instead of
    # turning into a timeout.
    if grep -q "Forwarding from .*:${port}" "${PF_LOG_DIR}/${name}.log"; then
      echo "❌ ${name} could not bind 127.0.0.1:${port} — another process owns that port:" >&2
      sed 's/^/    /' "${PF_LOG_DIR}/${name}.log" >&2
      exit 1
    fi

    sleep 1
  done

  echo "❌ ${name} port-forward never forwarded anything on 127.0.0.1:${port}:" >&2
  sed 's/^/    /' "${PF_LOG_DIR}/${name}.log" >&2
  exit 1
}

echo "==> Waiting for the port-forwards to accept connections..."
wait_for_tunnel platform "${PF_PLATFORM_PID}" 18080
wait_for_tunnel decision-service "${PF_DECISION_PID}" 18082
wait_for_tunnel integration-mocks "${PF_MOCKS_PID}" 18081
wait_for_tunnel redpanda "${PF_REDPANDA_PID}" 19092
wait_for_tunnel jaeger "${PF_JAEGER_PID}" 16686

echo "==> Waiting for the forwarded services to answer..."
# A listening tunnel only proves the forward came up; these prove it reaches the service.
SERVICES_READY=false
for _ in {1..15}; do
  if curl -s http://127.0.0.1:18080/healthz >/dev/null 2>&1 && \
     curl -s http://127.0.0.1:18082/healthz >/dev/null 2>&1 && \
     curl -s http://127.0.0.1:16686/api/services >/dev/null 2>&1; then
    echo "    platform, decision-service and jaeger answer through their tunnels"
    SERVICES_READY=true
    break
  fi
  sleep 1
done

if [ "${SERVICES_READY}" != "true" ]; then
  echo "❌ the forwarded services never answered; refusing to run the suite against a half-open environment" >&2
  exit 1
fi

echo "==> Executing live cluster E2E tests..."
export LIVE_CLUSTER=true
export PLATFORM_URL="http://127.0.0.1:18080"
export DECISION_URL="http://127.0.0.1:18082"
export MOCKS_URL="http://127.0.0.1:18081"
export KAFKA_BROKERS="127.0.0.1:19092"
export JAEGER_URL="http://127.0.0.1:16686"

cd "${ROOT_DIR}/e2e"
# -count=1 is load-bearing: without it a previous PASS is replayed from the Go build
# cache and the suite never touches the cluster, so a dead cluster still reports green.
go test -v -count=1 -tags=e2e ./scenarios/... -timeout 5m

echo "==> Live cluster E2E tests finished successfully!"

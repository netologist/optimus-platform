#!/usr/bin/env bash
set -euo pipefail

REGISTRY="localhost:5001/optimus"
TAG="dev"

echo "==> Building Optimus container images..."

SERVICES=("platform" "decision-service" "integration-mocks" "operator")

for svc in "${SERVICES[@]}"; do
  echo "==> Building ${svc} -> ${REGISTRY}/${svc}:${TAG}"
  # If Dockerfile exists, build, otherwise skip
  if [ -f "projects/${svc}/Dockerfile" ]; then
    docker build -t "${REGISTRY}/${svc}:${TAG}" -f "projects/${svc}/Dockerfile" "projects/${svc}"
    docker push "${REGISTRY}/${svc}:${TAG}" || true
  fi
done

echo "==> Image build completed."

#!/usr/bin/env bash
set -euo pipefail

# Build and push all Optimus container images to the local Kind registry
# TODO: In production, enable multi-arch buildx (linux/amd64, linux/arm64), SBOM generation (syft), and image signing (cosign)

REGISTRY="localhost:5001/optimus"
TAG="dev"

echo "==> Building Optimus container images for local Kind cluster..."

SERVICES=("platform" "decision-service" "integration-mocks" "operator" "ai-runtime")

FAILED=()

for svc in "${SERVICES[@]}"; do
  echo "----------------------------------------------------------------"
  echo "==> Building ${svc} -> ${REGISTRY}/${svc}:${TAG}"
  echo "----------------------------------------------------------------"
  if docker build -t "${REGISTRY}/${svc}:${TAG}" -f "projects/${svc}/Dockerfile" "projects/${svc}" \
    && docker push "${REGISTRY}/${svc}:${TAG}"; then
    echo "==> ${svc} OK"
  else
    echo "==> WARNING: ${svc} build/push failed — continuing with remaining services"
    FAILED+=("${svc}")
  fi
done

echo "----------------------------------------------------------------"
echo "==> Building ollaya mock -> localhost:5001/ollaya-dev/ollaya:latest"
echo "----------------------------------------------------------------"
if docker build -t "localhost:5001/ollaya-dev/ollaya:latest" -t "${REGISTRY}/ollaya:${TAG}" \
     -f "projects/ollaya/Dockerfile" "projects/ollaya" \
  && docker push "localhost:5001/ollaya-dev/ollaya:latest" \
  && docker push "${REGISTRY}/ollaya:${TAG}"; then
  echo "==> ollaya OK"
else
  echo "==> WARNING: ollaya build/push failed"
  FAILED+=("ollaya")
fi

echo "================================================================"
if [ ${#FAILED[@]} -eq 0 ]; then
  echo "==> All 6 application images built and pushed successfully!"
else
  echo "==> Completed with failures: ${FAILED[*]}"
  echo "    Re-run 'mise run build' after fixing network/dependencies."
  exit 1
fi
echo "================================================================"

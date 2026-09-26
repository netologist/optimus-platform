#!/usr/bin/env bash
set -euo pipefail

# Build and push all Optimus container images to the local Kind registry
# TODO: In production, enable multi-arch buildx (linux/amd64, linux/arm64), SBOM generation (syft), and image signing (cosign)

REGISTRY="localhost:5001/optimus"
TAG="dev"

echo "==> Building Optimus container images for local Kind cluster..."

SERVICES=("platform" "decision-service" "integration-mocks" "operator" "ai-runtime")

for svc in "${SERVICES[@]}"; do
  echo "----------------------------------------------------------------"
  echo "==> Building ${svc} -> ${REGISTRY}/${svc}:${TAG}"
  echo "----------------------------------------------------------------"
  docker build -t "${REGISTRY}/${svc}:${TAG}" -f "projects/${svc}/Dockerfile" "projects/${svc}"
  echo "==> Pushing ${REGISTRY}/${svc}:${TAG} to local registry..."
  docker push "${REGISTRY}/${svc}:${TAG}"
done

echo "----------------------------------------------------------------"
echo "==> Building ollaya mock -> localhost:5001/ollaya-dev/ollaya:latest"
echo "----------------------------------------------------------------"
docker build -t "localhost:5001/ollaya-dev/ollaya:latest" -t "${REGISTRY}/ollaya:${TAG}" -f "projects/ollaya/Dockerfile" "projects/ollaya"
echo "==> Pushing ollaya to local registry..."
docker push "localhost:5001/ollaya-dev/ollaya:latest"
docker push "${REGISTRY}/ollaya:${TAG}"

echo "================================================================"
echo "==> All 6 application images built and pushed successfully!"
echo "================================================================"

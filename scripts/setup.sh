#!/usr/bin/env bash
set -euo pipefail

# One-shot master setup script for the Optimus platform
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

echo "================================================================================"
echo "          OPTIMUS — ONE-SHOT PLATFORM BOOTSTRAP & SETUP                         "
echo "================================================================================"

# 1. Start Kind cluster and local registry mirror
echo -e "\n==> [1/3] Initializing Kind Cluster & Local Registry..."
"${SCRIPT_DIR}/kind-create.sh"

export KUBECONFIG="${ROOT_DIR}/.kube/kind-optimus.yaml"

# 2. Build and push container images
echo -e "\n==> [2/3] Building and Pushing Application Container Images..."
"${SCRIPT_DIR}/build.sh"

# 3. Deploy Kubernetes infrastructure, DB jobs, and application services
echo -e "\n==> [3/3] Deploying Infrastructure, DB Jobs, and Services via Kustomize..."
"${SCRIPT_DIR}/deploy.sh" "kind-dev"

echo -e "\n================================================================================"
echo "  Optimus Platform is 100% READY!"
echo "  Run tests : mise run e2e:live"
echo "  Run demo  : mise run demo"
echo "================================================================================"
